package main

import (
	"context"
	"log"
	"os"

	"github.com/snookish/ledger"
)

func main() {
	ctx := context.Background()
	dir := os.Getenv("LEDGER_DIR")
	if dir == "" {
		dir = "/tmp/ledger-rotation"
	}
	_ = os.RemoveAll(dir)

	wal, err := ledger.Open(ctx, ledger.WithDir(dir), ledger.WithSegmentSize(64*1024))
	if err != nil {
		log.Fatalf("open failed: %v", err)
	}

	payload := make([]byte, 16*1024)
	for idx := range payload {
		payload[idx] = 'x'
	}

	for idx := range 10 {
		lsn, err := wal.Append(ctx, payload)
		if err != nil {
			log.Fatalf("append %d failed: %v", idx, err)
		}
		log.Printf("append %d lsn %d", idx, lsn)
	}

	if err := wal.Close(ctx); err != nil {
		log.Fatalf("close failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("read dir failed: %v", err)
	}
	log.Printf("segments before trim: %d", len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		log.Printf(" %s %d bytes", entry.Name(), info.Size())
	}

	// Trim the oldest segment after it is consumed
	trimWal, err := ledger.Open(ctx, ledger.WithDir(dir))
	if err != nil {
		log.Fatalf("open for trim failed: %v", err)
	}
	if err := trimWal.TrimOldest(); err != nil {
		log.Fatalf("trim failed: %v", err)
	}
	if err := trimWal.Close(ctx); err != nil {
		log.Fatalf("close trim wal failed: %v", err)
	}

	after, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("read after trim failed: %v", err)
	}
	log.Printf("segments after trim: %d", len(after))

	reader := ledger.NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		log.Fatalf("read after trim failed: %v", err)
	}
	log.Printf("read after trim %d records", len(out))
}

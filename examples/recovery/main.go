package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/snookish/ledger"
)

func main() {
	ctx := context.Background()
	dir := os.Getenv("LEDGER_DIR")
	if dir == "" {
		dir = "/tmp/ledger-recovery"
	}
	_ = os.RemoveAll(dir)

	wal, err := ledger.Open(ctx, ledger.WithDir(dir), ledger.WithSyncer(ledger.NewNoopSyncer()))
	if err != nil {
		log.Fatalf("open failed: %v", err)
	}
	for _, msg := range []string{"a", "b", "c"} {
		if _, err := wal.Append(ctx, []byte(msg)); err != nil {
			log.Fatalf("append failed: %v", err)
		}
	}
	wal.Close(ctx)

	// Simulate a crash that truncates the last record
	path := filepath.Join(dir, "00000001.log")
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}
	log.Printf("size before truncate %d", len(data))
	// Truncate inside the last record's payload (header at 16, payload at 23)
	if err := os.Truncate(path, 23); err != nil {
		log.Fatalf("truncate failed: %v", err)
	}
	truncData, _ := os.ReadFile(path)
	log.Printf("size after truncate %d", len(truncData))

	reader := ledger.NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}
	log.Printf("recovered %d records (want 2)", len(out))
	for idx, rec := range out {
		log.Printf("%d: %s", idx, string(rec))
	}

	// Torn sector: flip a byte in the first record
	_ = os.WriteFile(path, data, 0644)
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("open for torn failed: %v", err)
	}
	_, _ = file.Seek(7, 0)
	_, _ = file.Write([]byte("X"))
	_ = file.Close()

	out, _ = reader.ReadAll(ctx)
	log.Printf("after torn, recovered %d records (want 0)", len(out))
}

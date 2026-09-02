package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/snookish/ledger"
)

func main() {
	ctx := context.Background()
	dir := os.Getenv("LEDGER_DIR")
	if dir == "" {
		dir = "/tmp/ledger-concurrent"
	}
	_ = os.RemoveAll(dir)

	wal, err := ledger.Open(ctx, ledger.WithDir(dir))
	if err != nil {
		log.Fatalf("open failed: %v", err)
	}

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for idx := range workers {
		go func(workerID int) {
			defer wg.Done()
			payload := fmt.Appendf(nil, `{"worker":%d,"msg":"hello"}`, workerID)
			lsn, err := wal.Append(ctx, payload)
			if err != nil {
				log.Printf("worker %d append failed: %v", workerID, err)
				return
			}
			log.Printf("worker %d lsn %d", workerID, lsn)
		}(idx)
	}
	wg.Wait()

	if err := wal.Close(ctx); err != nil {
		log.Fatalf("close failed: %v", err)
	}

	reader := ledger.NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}
	log.Printf("concurrent read %d records", len(out))
}

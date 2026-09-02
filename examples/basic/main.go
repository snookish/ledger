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
		dir = "/tmp/ledger-basic"
	}
	_ = os.RemoveAll(dir)

	wal, err := ledger.Open(ctx, ledger.WithDir(dir))
	if err != nil {
		log.Fatalf("open failed: %v", err)
	}

	records := [][]byte{
		[]byte(`{"id":"ord_1","type":"order_created","amount":14900}`),
		[]byte(`{"id":"pay_1","type":"payment_captured","order_id":"ord_1"}`),
	}

	for idx, rec := range records {
		lsn, err := wal.Append(ctx, rec)
		if err != nil {
			log.Fatalf("append %d failed: %v", idx, err)
		}
		log.Printf("append %d lsn %d", idx, lsn)
	}

	if err := wal.Close(ctx); err != nil {
		log.Fatalf("close failed: %v", err)
	}

	// Read back with strict recovery
	reader := ledger.NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}
	log.Printf("read %d records", len(out))
	for idx, rec := range out {
		log.Printf("%d: %s", idx, string(rec))
	}
}

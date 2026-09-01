package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/snookish/ledger"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	ctx := context.Background()
	dir := os.Getenv("LEDGER_DIR")
	if dir == "" {
		dir = os.Getenv("WAL_DIR")
	}
	if dir == "" {
		dir = "/tmp/ledger-demo"
	}

	logger.Printf("ledger dir: %s", dir)

	w, err := ledger.Open(ctx, ledger.WithDir(dir))
	if err != nil {
		logger.Fatalf("open failed: %v", err)
	}
	defer func() {
		if err := w.Close(ctx); err != nil {
			logger.Printf("close failed: %v", err)
		} else {
			logger.Printf("ledger closed")
		}
	}()

	smallRecords := [][]byte{
		[]byte(`{"id":"ord_01H8X","type":"order_created","amount":14900,"currency":"USD","customer":"cust_42"}`),
		[]byte(`{"id":"pay_01H8Y","type":"payment_captured","order_id":"ord_01H8X","amount":14900,"method":"card"}`),
	}

	// Big batch ~61KB JSON, spans 2 blocks (32761 + ~28KB).
	bigBatch := buildBigBatch(500)

	records := append(smallRecords, bigBatch)

	for idx, rec := range records {
		lsn, err := w.Append(ctx, rec)
		if err != nil {
			logger.Fatalf("append %d failed: %v", idx, err)
		}
		logger.Printf("append %d -> lsn %d (%d bytes)", idx, lsn, len(rec))
	}

	logger.Printf("Done, segment files:")
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		info, _ := entry.Info()
		logger.Printf("  %s  %d bytes", entry.Name(), info.Size())
	}

	// Rotation demo with realistic small segments so we actually roll.
	logger.Printf("--- Rotation Demo ---")
	rotationDir := dir + "-rotate"
	_ = os.RemoveAll(rotationDir)

	rotationWal, err := ledger.Open(ctx, ledger.WithDir(rotationDir), ledger.WithSegmentSize(64<<10))
	if err != nil {
		logger.Fatalf("open rotation wal failed: %v", err)
	}

	for i := range 10 {
		payload := buildBigBatch(80)
		lsn, err := rotationWal.Append(ctx, payload)
		if err != nil {
			logger.Fatalf("rotation append %d failed: %v", i, err)
		}
		logger.Printf("rotation append %d -> lsn %d (%d bytes)", i, lsn, len(payload))
	}

	if err := rotationWal.Close(ctx); err != nil {
		logger.Printf("rotation close failed: %v", err)
	}

	logger.Printf("Rotation segment files in %s:", rotationDir)
	rotationEntries, _ := os.ReadDir(rotationDir)
	for _, entry := range rotationEntries {
		info, _ := entry.Info()
		logger.Printf("  %s  %d bytes", entry.Name(), info.Size())
	}
}

func buildBigBatch(count int) []byte {
	type order struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
		Amount   int    `json:"amount"`
		Currency string `json:"currency"`
		Note     string `json:"note"`
	}

	orders := make([]order, count)
	for i := range orders {
		orders[i] = order{
			ID:       fmt.Sprintf("ord_%05d", i),
			Customer: fmt.Sprintf("cust_%04d", i%100),
			Amount:   10000 + i*10,
			Currency: "USD",
			Note:     "realistic order for block spanning test",
		}
	}

	data, _ := json.Marshal(map[string]any{
		"type":   "order_batch_big",
		"count":  count,
		"orders": orders,
	})

	return data
}

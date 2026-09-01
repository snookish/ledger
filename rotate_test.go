package ledger

import (
	"context"
	"os"
	"testing"
)

func TestRotateOnSegmentLimit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Small segment so we can trigger rotation quickly in test.
	wal, err := Open(ctx, WithDir(dir), WithSegmentSize(64<<10), WithSyncer(NewNoopSyncer()))
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close(ctx)

	payload := make([]byte, 16<<10)
	for i := range payload {
		payload[i] = byte('a' + i%26)
	}

	// Keep appending until we roll over.
	for i := range 11 {
		if _, err := wal.Append(ctx, payload); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	segments, err := listSegments(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(segments) < 2 {
		t.Fatalf("want at least 2 segments, got %d", len(segments))
	}

	// Both files should exist and have data.
	for _, segPath := range segments {
		info, err := os.Stat(segPath)
		if err != nil {
			t.Fatalf("stat %s: %v", segPath, err)
		}
		if info.Size() == 0 {
			t.Fatalf("segment %s is empty", segPath)
		}
	}
}

package ledger

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReaderReadAll(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	wal, err := Open(ctx, WithDir(dir), WithSyncer(NewNoopSyncer()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wal.Append(ctx, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if _, err := wal.Append(ctx, []byte("world")); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(ctx); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || string(out[0]) != "hello" || string(out[1]) != "world" {
		t.Fatalf("want hello world, got %v", out)
	}
}

func TestReaderTruncate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	wal, err := Open(ctx, WithDir(dir), WithSyncer(NewNoopSyncer()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wal.Append(ctx, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := wal.Append(ctx, []byte("b")); err != nil {
		t.Fatal(err)
	}
	if _, err := wal.Append(ctx, []byte("c")); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(ctx); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "00000001.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Truncate(path, 23); err != nil {
		t.Fatal(err)
	}
	reader := NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("truncate payload want 2, got %d", len(out))
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Truncate(path, 22); err != nil {
		t.Fatal(err)
	}
	out, err = reader.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("truncate header want 2, got %d", len(out))
	}
}

func TestReaderTorn(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	wal, err := Open(ctx, WithDir(dir), WithSyncer(NewNoopSyncer()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wal.Append(ctx, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(ctx); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "00000001.log")
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(7, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("X")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("torn want 0, got %d", len(out))
	}
}

func TestReaderFragment(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	wal, err := Open(ctx, WithDir(dir), WithSyncer(NewNoopSyncer()))
	if err != nil {
		t.Fatal(err)
	}
	big := make([]byte, 40000)
	for idx := range big {
		big[idx] = byte('a' + idx%26)
	}
	if _, err := wal.Append(ctx, big); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(ctx); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || len(out[0]) != 40000 {
		t.Fatalf("fragment want 40000, got %d", len(out[0]))
	}
}

func TestReaderRotate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	wal, err := Open(ctx, WithDir(dir), WithSegmentSize(64*1024), WithSyncer(NewNoopSyncer()))
	if err != nil {
		t.Fatal(err)
	}
	big := make([]byte, 16*1024)
	for idx := range big {
		big[idx] = 'x'
	}
	for range 10 {
		if _, err := wal.Append(ctx, big); err != nil {
			t.Fatal(err)
		}
	}
	if err := wal.Close(ctx); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(dir)
	out, err := reader.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 10 {
		t.Fatalf("rotate want 10, got %d", len(out))
	}
}

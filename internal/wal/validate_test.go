package wal

import (
	"context"
	"os"
	"testing"
)

func TestValidateDir(t *testing.T) {
	ctx := context.Background()
	if _, err := Open(ctx, WithDir("relative/path")); err != ErrBadDir {
		t.Fatalf("want ErrBadDir for relative, got %v", err)
	}
	if _, err := Open(ctx, WithDir(".")); err != ErrBadDir {
		t.Fatalf("want ErrBadDir for dot, got %v", err)
	}
	if _, err := Open(ctx, WithDir("   ")); err != ErrBadDir {
		t.Fatalf("want ErrBadDir for blank, got %v", err)
	}
	if _, err := Open(ctx, WithDir("/tmp\x00/bad")); err != ErrBadDir {
		t.Fatalf("want ErrBadDir for null, got %v", err)
	}
	// file not dir
	dir := t.TempDir()
	filePath := dir + "/file"
	os.WriteFile(filePath, []byte("x"), 0o644)
	if _, err := Open(ctx, WithDir(filePath)); err != ErrBadDir {
		t.Fatalf("want ErrBadDir for file not dir, got %v", err)
	}
	// valid absolute dir should work
	valid := t.TempDir()
	wal, err := Open(ctx, WithDir(valid))
	if err != nil {
		t.Fatalf("valid dir should not error, got %v", err)
	}
	wal.Close(ctx)
}

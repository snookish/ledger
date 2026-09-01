package wal

import (
	"context"
	"errors"
)

// WAL is the log. It owns the segment files in dir.
type WAL struct {
	dir  string
	opts Options
}

// Open creates or reopens the log in dir.
func Open(dir string, opts ...Option) (*WAL, error) {
	o := applyOptions(opts...)
	_ = o
	return nil, errors.New("not implemented")
}

// Append adds one record.
func (w *WAL) Append(ctx context.Context, b []byte) (uint64, error) {
	return 0, errors.New("not implemented")
}

// Close flushes and closes the log.
func (w *WAL) Close(ctx context.Context) error {
	return errors.New("not implemented")
}

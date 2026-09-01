package ledger

import (
	"context"
	"errors"
	"os"
	"sync"
)

// Common errors for callers to check with errors.Is.
var (
	ErrClosed  = errors.New("ledger: closed")
	ErrCorrupt = errors.New("ledger: corrupt record")
)

// WAL is the log. It owns the segment files in dir.
type WAL struct {
	dir        string
	opts       Options
	mu         sync.Mutex
	file       *os.File
	writer     *blockWriter
	manager    *segmentManager
	curSegment int
	lsn        uint64
	closed     bool
}

// Open creates or reopens the log. Dir defaults to /var/lib/ledger.
func Open(ctx context.Context, opts ...Option) (*WAL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	options := applyOptions(opts...)
	if err := options.Validate(); err != nil {
		return nil, err
	}
	dir := options.Dir

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	manager := newSegmentManager(dir)
	file, segmentNum, blockOffset, err := manager.openLast()
	if err != nil {
		return nil, err
	}

	return &WAL{
		dir:        dir,
		opts:       options,
		file:       file,
		writer:     newBlockWriter(file, blockOffset),
		curSegment: segmentNum,
		manager:    manager,
	}, nil
}

// Append adds one record and returns its LSN.
func (wal *WAL) Append(ctx context.Context, payload []byte) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	wal.mu.Lock()
	defer wal.mu.Unlock()

	if wal.closed {
		return 0, ErrClosed
	}

	lsn, err := wal.appendLocked(payload)
	if err != nil {
		return 0, err
	}

	if err := wal.opts.Syncer.Sync(wal.file); err != nil {
		return 0, err
	}

	return lsn, nil
}

// Close flushes and closes the log. It respects context cancellation.
func (wal *WAL) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	wal.mu.Lock()
	defer wal.mu.Unlock()

	if wal.closed {
		return nil
	}

	if wal.file != nil {
		if wal.writer.Offset() != 0 {
			_ = wal.writer.padBlock()
		}
		_ = wal.opts.Syncer.Sync(wal.file)
		wal.file.Close()
	}

	wal.closed = true
	return nil
}

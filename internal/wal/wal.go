package wal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Common errors for callers to check with errors.Is.
var (
	ErrClosed  = errors.New("wal: closed")
	ErrCorrupt = errors.New("wal: corrupt record")
)

// WAL is the log. It owns the segment files in dir.
type WAL struct {
	dir         string
	opts        Options
	mu          sync.Mutex
	file        *os.File
	blockOffset int
	curSegment  int
	lsn         uint64
	closed      bool
}

// Open creates or reopens the log. Dir defaults to /var/lib/ledger.
func Open(ctx context.Context, opts ...Option) (*WAL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	options := applyOptions(opts...)
	dir := options.Dir

	if err := validateDir(dir); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	segments, err := listSegments(dir)
	if err != nil {
		return nil, err
	}

	var (
		segmentNum  int
		blockOffset int
		file        *os.File
	)

	if len(segments) == 0 {
		segmentNum = 1
		file, err = openSegmentForAppend(dir, segmentNum)
		if err != nil {
			return nil, err
		}
		blockOffset = 0
	} else {
		lastSegment := segments[len(segments)-1]
		baseName := filepath.Base(lastSegment)
		parsedNum, ok := parseSegmentName(baseName)
		if !ok {
			return nil, ErrCorrupt
		}
		segmentNum = parsedNum

		file, err = os.OpenFile(lastSegment, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}

		info, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, err
		}
		blockOffset = int(info.Size() % int64(BlockSize))
	}

	return &WAL{
		dir:         dir,
		opts:        options,
		file:        file,
		blockOffset: blockOffset,
		curSegment:  segmentNum,
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
		if wal.blockOffset != 0 {
			_ = wal.padBlock()
		}
		_ = wal.opts.Syncer.Sync(wal.file)
		wal.file.Close()
	}

	wal.closed = true
	return nil
}

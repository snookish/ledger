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
	reqCh      chan *appendReq
	doneCh     chan struct{}
	groupDone  chan struct{}
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

	wal := &WAL{
		dir:        dir,
		opts:       options,
		file:       file,
		writer:     newBlockWriter(file, blockOffset),
		curSegment: segmentNum,
		manager:    manager,
	}

	if options.BatchTimeout > 0 {
		wal.reqCh = make(chan *appendReq, options.MaxBatchCount*2)
		wal.doneCh = make(chan struct{})
		wal.groupDone = make(chan struct{})
		go wal.runGroupCommit()
	}

	return wal, nil
}

// Append adds one record and returns its LSN.
func (wal *WAL) Append(ctx context.Context, payload []byte) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if wal.reqCh != nil {
		wal.mu.Lock()
		closed := wal.closed
		wal.mu.Unlock()
		if closed {
			return 0, ErrClosed
		}
		req := &appendReq{
			payload: payload,
			done:    make(chan *appendResult, 1),
		}
		select {
		case wal.reqCh <- req:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
		select {
		case res := <-req.done:
			return res.lsn, res.err
		case <-ctx.Done():
			return 0, ctx.Err()
		}
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

	if wal.reqCh != nil {
		close(wal.doneCh)
		<-wal.groupDone
		wal.mu.Lock()
		defer wal.mu.Unlock()
		if wal.closed {
			return nil
		}
		wal.closed = true
		if wal.file != nil {
			if wal.writer.Offset() != 0 {
				if err := wal.writer.padBlock(); err != nil {
					return err
				}
			}
			if err := wal.opts.Syncer.Sync(wal.file); err != nil {
				return err
			}
			if err := wal.file.Close(); err != nil {
				return err
			}
		}
		return nil
	}

	wal.mu.Lock()
	defer wal.mu.Unlock()

	if wal.closed {
		return nil
	}

	wal.closed = true
	if wal.file != nil {
		if wal.writer.Offset() != 0 {
			if err := wal.writer.padBlock(); err != nil {
				return err
			}
		}
		if err := wal.opts.Syncer.Sync(wal.file); err != nil {
			return err
		}
		if err := wal.file.Close(); err != nil {
			return err
		}
	}
	return nil
}

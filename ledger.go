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
//
// When batching is enabled (BatchTimeout > 0) this takes the group-commit
// path: the request is enqueued to runGroupCommit and shares one fsync with
// other concurrent Appends. When batching is disabled it writes and syncs
// directly. The enqueue is bounded (MaxBatchCount*2) so a slow batch does not
// grow without limit.
func (wal *WAL) Append(ctx context.Context, payload []byte) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if wal.reqCh != nil {
		// Check closed before enqueue so we do not add work after Close
		// has started shutting down the batch loop.
		wal.mu.Lock()
		if wal.closed {
			wal.mu.Unlock()
			return 0, ErrClosed
		}
		wal.mu.Unlock()

		req := &appendReq{
			payload: payload,
			done:    make(chan *appendResult, 1),
		}

		// Wait for queue space, but also notice if Close shuts down the
		// group loop or the caller cancels while queued.
		select {
		case wal.reqCh <- req:
		case <-wal.doneCh:
			return 0, ErrClosed
		case <-ctx.Done():
			return 0, ctx.Err()
		}

		// Wait for the batch leader to write and sync. If Close wins
		// the race we return ErrClosed; if ctx is canceled we return
		// ctx.Err(). In either case the payload may still be flushed
		// after we return because it is already enqueued.
		select {
		case res := <-req.done:
			return res.lsn, res.err
		case <-wal.doneCh:
			return 0, ErrClosed
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
	}

	wal.mu.Lock()
	defer wal.mu.Unlock()

	if wal.closed {
		return nil
	}

	wal.closed = true
	return wal.closeLocked()
}

func (wal *WAL) closeLocked() error {
	if wal.file == nil {
		return nil
	}
	if wal.writer.Offset() != 0 {
		if err := wal.writer.padBlock(); err != nil {
			return err
		}
	}
	if err := wal.opts.Syncer.Sync(wal.file); err != nil {
		return err
	}
	return wal.file.Close()
}

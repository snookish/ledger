package ledger

import "time"

type appendReq struct {
	payload []byte
	done    chan *appendResult // buffer 1 so flushBatch never blocks on wake up
}

type appendResult struct {
	lsn uint64
	err error
}

// runGroupCommit collects concurrent Appends and does one fsync per batch.
// It waits for BatchTimeout or until MaxBatchCount / MaxBatchBytes is hit.
func (wal *WAL) runGroupCommit() {
	timer := time.NewTimer(wal.opts.BatchTimeout)
	defer timer.Stop()

	var batchBytes int
	var batch []*appendReq

	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wal.opts.BatchTimeout)
	}

	flush := func() {
		wal.flushBatch(batch)
		batch = nil
		batchBytes = 0
	}

	for {
		select {
		case req := <-wal.reqCh:
			batch = append(batch, req)
			batchBytes += len(req.payload) + HeaderSize
			if len(batch) >= wal.opts.MaxBatchCount || batchBytes >= wal.opts.MaxBatchBytes {
				flush()
				resetTimer()
			}
		case <-timer.C:
			flush()
			timer.Reset(wal.opts.BatchTimeout)
		case <-wal.doneCh:
			flush()
			close(wal.groupDone)
			return
		}
	}
}

// flushBatch writes every payload in the batch under mu, does one Sync,
// and wakes each waiter. Batch may be empty (timer fired with no work)
// in which case this is a no-op.
func (wal *WAL) flushBatch(batch []*appendReq) {
	if len(batch) == 0 {
		return
	}

	wal.mu.Lock()
	var firstErr error
	results := make([]*appendResult, len(batch))

	for idx, req := range batch {
		if firstErr != nil {
			// Preserve ordering: earlier failure fails the rest
			// without touching the file.
			results[idx] = &appendResult{err: firstErr}
			continue
		}
		lsn, err := wal.appendLocked(req.payload)
		if err != nil {
			firstErr = err
		}
		results[idx] = &appendResult{lsn: lsn, err: err}
	}

	// One fsync for the whole batch. If any appendLocked failed we
	// already have firstErr and skip.
	if firstErr == nil {
		if err := wal.opts.Syncer.Sync(wal.file); err != nil {
			firstErr = err
		}
	}
	wal.mu.Unlock()

	// Wake each caller. If firstErr is set (write or sync) every
	// previously successful entry gets that error so the batch is
	// all-or-nothing from the caller's view.
	for idx, req := range batch {
		res := results[idx]
		if firstErr != nil && res.err == nil {
			res.err = firstErr
		}
		select {
		case req.done <- res:
		default:
		}
	}
}

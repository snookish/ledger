package ledger

import "time"

type appendReq struct {
	payload []byte
	done    chan *appendResult
}

type appendResult struct {
	lsn uint64
	err error
}

// runGroupCommit collects concurrent Appends and does one fsync per batch.
// It waits for BatchTimeout, or until MaxBatchCount / MaxBatchBytes is hit.
func (wal *WAL) runGroupCommit() {
	timer := time.NewTimer(wal.opts.BatchTimeout)
	defer timer.Stop()

	var batchBytes int
	var batch []*appendReq

	for {
		select {
		case req := <-wal.reqCh:
			batch = append(batch, req)
			batchBytes += len(req.payload) + HeaderSize
			// Batch is full, flush now instead of waiting
			if len(batch) >= wal.opts.MaxBatchCount || batchBytes >= wal.opts.MaxBatchBytes {
				wal.flushBatch(batch)
				batch = nil
				batchBytes = 0
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(wal.opts.BatchTimeout)
			}
		case <-timer.C:
			// Timeout, flush what we have
			wal.flushBatch(batch)
			batch = nil
			batchBytes = 0
			timer.Reset(wal.opts.BatchTimeout)
		case <-wal.doneCh:
			// Shutting down, flush last batch
			wal.flushBatch(batch)
			close(wal.groupDone)
			return
		}
	}
}

// flushBatch writes every payload in the batch, does one Sync and wakes callers.
func (wal *WAL) flushBatch(batch []*appendReq) {
	if len(batch) == 0 {
		return
	}
	wal.mu.Lock()
	var firstErr error
	results := make([]*appendResult, len(batch))
	for idx, req := range batch {
		lsn, err := wal.appendLocked(req.payload)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		results[idx] = &appendResult{lsn: lsn, err: err}
	}

	// One fsync for the whole batch
	if firstErr == nil {
		if err := wal.opts.Syncer.Sync(wal.file); err != nil {
			firstErr = err
		}
	}
	wal.mu.Unlock()

	// Wake everyone, same error for the batch if sync failed
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

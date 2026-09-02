package ledger

import "os"

// TrimOldest removes the oldest segment if more than one exists.
// Caller should call only after the consumer has safely persisted that prefix.
func (wal *WAL) TrimOldest() error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	segments, err := wal.manager.list()
	if err != nil {
		return err
	}
	if len(segments) <= 1 {
		return nil
	}

	// Never delete the current segment
	oldest := segments[0]
	current := segmentPath(wal.dir, wal.curSegment)
	if oldest == current {
		return nil
	}

	if err := os.Remove(oldest); err != nil {
		return err
	}
	return fsyncDir(wal.dir)
}

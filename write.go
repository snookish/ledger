package ledger

// appendLocked writes the payload, splitting it across blocks if needed.
// LSN is only incremented on success so a failed write does not leave a gap.
func (wal *WAL) appendLocked(payload []byte) (uint64, error) {
	nextLSN := wal.lsn + 1

	if len(payload) == 0 {
		if err := wal.writer.ensureHeaderSpace(); err != nil {
			return 0, err
		}
		if err := wal.maybeRotate(int64(HeaderSize)); err != nil {
			return 0, err
		}
		if err := wal.writer.writeFragment(KindFull, payload); err != nil {
			return 0, err
		}
		wal.lsn = nextLSN
		return nextLSN, nil
	}

	remainingPayload := payload
	isFirstFragment := true

	for len(remainingPayload) > 0 {
		if err := wal.writer.ensureHeaderSpace(); err != nil {
			return 0, err
		}

		if err := wal.maybeRotate(int64(len(remainingPayload) + HeaderSize)); err != nil {
			return 0, err
		}

		chunkSize := wal.writer.availablePayload()
		isLast := len(remainingPayload) <= chunkSize
		kind := wal.writer.chooseKind(isFirstFragment, isLast)
		if isLast {
			if err := wal.writer.writeFragment(kind, remainingPayload); err != nil {
				return 0, err
			}
			break
		}

		if err := wal.writer.writeFragment(kind, remainingPayload[:chunkSize]); err != nil {
			return 0, err
		}

		remainingPayload = remainingPayload[chunkSize:]
		isFirstFragment = false
	}

	wal.lsn = nextLSN
	return nextLSN, nil
}

// maybeRotate rolls to a new segment if the current one would get too big.
func (wal *WAL) maybeRotate(needBytes int64) error {
	nextFile, nextSegment, err := wal.manager.rotateIfNeeded(
		wal.file, wal.writer, wal.opts.Syncer, wal.opts.SegmentSize, needBytes,
	)
	if err != nil {
		return err
	}
	if nextFile == nil {
		return nil
	}
	wal.file = nextFile
	wal.writer.Reset(nextFile, 0)
	wal.curSegment = nextSegment
	return nil
}

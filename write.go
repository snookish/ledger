package ledger

// appendLocked writes the payload, splitting it across blocks if needed.
// Caller must hold mu.
func (wal *WAL) appendLocked(payload []byte) (uint64, error) {
	wal.lsn++
	lsn := wal.lsn

	if len(payload) == 0 {
		if err := wal.writer.ensureHeaderSpace(); err != nil {
			return 0, err
		}
		if err := wal.maybeRotate(int64(HeaderSize)); err != nil {
			return 0, err
		}
		if err := wal.writer.ensureHeaderSpace(); err != nil {
			return 0, err
		}
		if err := wal.writer.writeFragment(KindFull, payload); err != nil {
			return 0, err
		}
		return lsn, nil
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

		if err := wal.writer.ensureHeaderSpace(); err != nil {
			return 0, err
		}

		chunkSize := wal.writer.availablePayload()
		if len(remainingPayload) <= chunkSize {
			kind := wal.writer.chooseKind(isFirstFragment, len(payload), len(remainingPayload))
			if err := wal.writer.writeFragment(kind, remainingPayload); err != nil {
				return 0, err
			}
			break
		}

		chunk := remainingPayload[:chunkSize]
		kind := KindFirst
		if !isFirstFragment {
			kind = KindMiddle
		}

		if err := wal.writer.writeFragment(kind, chunk); err != nil {
			return 0, err
		}

		remainingPayload = remainingPayload[chunkSize:]
		isFirstFragment = false
	}

	return lsn, nil
}

// maybeRotate rolls to a new segment if the current one would get too big.
func (wal *WAL) maybeRotate(needBytes int64) error {
	info, err := wal.file.Stat()
	if err != nil {
		return err
	}

	if info.Size()+needBytes <= wal.opts.SegmentSize {
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

	if err := wal.file.Close(); err != nil {
		return err
	}

	nextFile, nextSegment, err := wal.manager.createNext()
	if err != nil {
		return err
	}

	wal.file = nextFile
	wal.writer.Reset(nextFile, 0)
	wal.curSegment = nextSegment

	return nil
}

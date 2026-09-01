package wal

// appendLocked writes the payload, splitting it across blocks if needed.
func (wal *WAL) appendLocked(payload []byte) (uint64, error) {
	wal.lsn++
	lsn := wal.lsn

	if len(payload) == 0 {
		if err := wal.writeFragment(KindFull, payload); err != nil {
			return 0, err
		}
		return lsn, nil
	}

	remainingPayload := payload
	isFirstFragment := true

	for len(remainingPayload) > 0 {
		blockLeft := BlockSize - wal.blockOffset
		if blockLeft < HeaderSize {
			if err := wal.padBlock(); err != nil {
				return 0, err
			}
			blockLeft = BlockSize
		}

		if err := wal.maybeRotate(int64(len(remainingPayload) + HeaderSize)); err != nil {
			return 0, err
		}

		blockLeft = BlockSize - wal.blockOffset
		if blockLeft < HeaderSize {
			if err := wal.padBlock(); err != nil {
				return 0, err
			}
			blockLeft = BlockSize
		}

		chunkSize := min(blockLeft-HeaderSize, MaxPayloadPerFragment)
		if len(remainingPayload) <= chunkSize {
			kind := wal.chooseKind(isFirstFragment, len(payload), len(remainingPayload))
			if err := wal.writeFragment(kind, remainingPayload); err != nil {
				return 0, err
			}
			break
		}

		chunk := remainingPayload[:chunkSize]
		kind := KindFirst
		if !isFirstFragment {
			kind = KindMiddle
		}

		if err := wal.writeFragment(kind, chunk); err != nil {
			return 0, err
		}

		remainingPayload = remainingPayload[chunkSize:]
		isFirstFragment = false
	}

	return lsn, nil
}

func (wal *WAL) chooseKind(isFirst bool, totalLen, remainingLen int) RecordKind {
	if isFirst && remainingLen == totalLen {
		return KindFull
	}
	if isFirst {
		return KindFirst
	}
	return KindLast
}

func (wal *WAL) writeFragment(kind RecordKind, payload []byte) error {
	header := EncodeHeader(len(payload), kind, payload)

	if _, err := wal.file.Write(header[:]); err != nil {
		return err
	}

	if len(payload) > 0 {
		if _, err := wal.file.Write(payload); err != nil {
			return err
		}
	}

	wal.blockOffset += HeaderSize + len(payload)
	if wal.blockOffset == BlockSize {
		wal.blockOffset = 0
	}

	return nil
}

func (wal *WAL) padBlock() error {
	left := BlockSize - wal.blockOffset
	if left == 0 {
		return nil
	}

	zeros := make([]byte, left)
	if _, err := wal.file.Write(zeros); err != nil {
		return err
	}

	wal.blockOffset = 0
	return nil
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

	if wal.blockOffset != 0 {
		if err := wal.padBlock(); err != nil {
			return err
		}
	}

	if err := wal.opts.Syncer.Sync(wal.file); err != nil {
		return err
	}

	if err := wal.file.Close(); err != nil {
		return err
	}

	nextSegment := wal.curSegment + 1
	nextFile, err := openSegmentForAppend(wal.dir, nextSegment)
	if err != nil {
		return err
	}

	wal.file = nextFile
	wal.curSegment = nextSegment
	wal.blockOffset = 0

	return nil
}

package ledger

import "os"

// One block is 32KB, records never cross blocks.
type blockWriter struct {
	file   *os.File
	offset int
}

func newBlockWriter(file *os.File, offset int) *blockWriter {
	return &blockWriter{file: file, offset: offset}
}

func (writer *blockWriter) Offset() int {
	return writer.offset
}

func (writer *blockWriter) Reset(file *os.File, offset int) {
	writer.file = file
	writer.offset = offset
}

// writeFragment writes one header + payload and moves the offset.
func (writer *blockWriter) writeFragment(kind RecordKind, payload []byte) error {
	header := EncodeHeader(len(payload), kind, payload)

	if _, err := writer.file.Write(header[:]); err != nil {
		return err
	}

	if len(payload) > 0 {
		if _, err := writer.file.Write(payload); err != nil {
			return err
		}
	}

	writer.offset += HeaderSize + len(payload)
	if writer.offset == BlockSize {
		writer.offset = 0
	}

	return nil
}

// padBlock fills the rest of the block with zeros.
func (writer *blockWriter) padBlock() error {
	left := BlockSize - writer.offset
	if left == 0 {
		return nil
	}

	zeros := make([]byte, left)
	if _, err := writer.file.Write(zeros); err != nil {
		return err
	}

	writer.offset = 0
	return nil
}

// chooseKind picks Full, First, Middle, or Last for a chunk.
func (writer *blockWriter) chooseKind(isFirst bool, totalLen, remainingLen int) RecordKind {
	if isFirst && remainingLen == totalLen {
		return KindFull
	}
	if isFirst {
		return KindFirst
	}
	return KindLast
}

package ledger

import (
	"encoding/binary"
	"hash/crc32"
)

// BlockSize is one block in the log file. Records never cross blocks.
const BlockSize = 32768

// HeaderSize is the header before each fragment.
const HeaderSize = 7

// MaxPayloadPerFragment is the biggest payload that fits in one fragment.
const MaxPayloadPerFragment = BlockSize - HeaderSize

// RecordKind is the kind of a fragment.
type RecordKind uint8

const (
	KindZero   RecordKind = iota // KindZero is padding at the end of a block.
	KindFull                     // KindFull is a record that fits in one fragment.
	KindFirst                    // KindFirst is the first piece of a split record.
	KindMiddle                   // KindMiddle is a middle piece of a split record.
	KindLast                     // KindLast is the last piece of a split record.
)

// maskDelta keeps a zero file from looking like a valid record.
const maskDelta = uint32(0xa282ead8)

// crcTable is the Castagnoli table for fast CRC.
var crcTable = crc32.MakeTable(crc32.Castagnoli)

// Mask hides the real CRC so zeros on disk don't look valid.
func Mask(crc uint32) uint32 {
	return ((crc >> 15) | (crc << 17)) + maskDelta
}

// Unmask restores a masked CRC.
func Unmask(masked uint32) uint32 {
	rot := masked - maskDelta
	return ((rot >> 17) | (rot << 15))
}

// calcCRC is the checksum for a fragment. It covers kind + payload.
func calcCRC(kind RecordKind, payload []byte) uint32 {
	crc := crc32.Checksum([]byte{byte(kind)}, crcTable)
	crc = crc32.Update(crc, crcTable, payload)
	return Mask(crc)
}

// EncodeHeader builds the 7 byte header for a fragment.
func EncodeHeader(payloadLen int, kind RecordKind, payload []byte) [HeaderSize]byte {
	var h [HeaderSize]byte
	crc := calcCRC(kind, payload)
	binary.LittleEndian.PutUint32(h[0:4], crc)
	binary.LittleEndian.PutUint16(h[4:6], uint16(payloadLen))
	h[6] = byte(kind)
	return h
}

// DecodeHeader reads a header. It returns false if there are too few bytes.
func DecodeHeader(b []byte) (crc uint32, length uint16, kind RecordKind, ok bool) {
	if len(b) < HeaderSize {
		return 0, 0, 0, false
	}
	crc = binary.LittleEndian.Uint32(b[0:4])
	length = binary.LittleEndian.Uint16(b[4:6])
	kind = RecordKind(b[6])
	return crc, length, kind, true
}

// VerifyHeader checks if the CRC matches kind and payload.
func VerifyHeader(crc uint32, kind RecordKind, payload []byte) bool {
	expected := calcCRC(kind, payload)
	return expected == crc
}

// NeedsFragment says if the payload must be split to fit.
func NeedsFragment(blockRemaining, payloadLen int) bool {
	if payloadLen > MaxPayloadPerFragment {
		return true
	}
	return blockRemaining < HeaderSize+payloadLen
}

// FragmentCount counts how many fragments a record will need.
func FragmentCount(payloadLen int, blockRemaining int) int {
	if payloadLen == 0 {
		return 1
	}

	var count int
	remaining := payloadLen
	firstSpace := max(blockRemaining-HeaderSize, 0)

	// Use what is left in the current block first.
	if firstSpace > 0 {
		if remaining <= firstSpace {
			return 1
		}
		remaining -= firstSpace
		count++
	}

	// Remaining pieces each fill a full block.
	for remaining > 0 {
		if remaining <= MaxPayloadPerFragment {
			count++
			break
		}
		remaining -= MaxPayloadPerFragment
		count++
	}

	return count
}

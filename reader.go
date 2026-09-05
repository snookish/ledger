package ledger

import (
	"context"
	"os"
)

// Reader reads the log back.
type Reader struct {
	dir    string
	verify bool // If true we stop at first bad record, if false we skip it
}

func NewReader(dir string) *Reader {
	return &Reader{dir: dir, verify: true}
}

// WithVerify says if we should check the checksum. Turn it off to salvage what we can.
func (reader *Reader) WithVerify(verify bool) *Reader {
	reader.verify = verify
	return reader
}

// ReadAll returns all the complete records in order.
func (reader *Reader) ReadAll(ctx context.Context) ([][]byte, error) {
	var out [][]byte
	if err := reader.Replay(ctx, func(payload []byte) error {
		cp := make([]byte, len(payload))
		copy(cp, payload)
		out = append(out, cp)
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// Replay walks each record in order and calls fn.
func (reader *Reader) Replay(ctx context.Context, fn func([]byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	segments, err := listSegments(reader.dir)
	if err != nil {
		return err
	}

	var fragBuffer []byte
	fragActive := false

	for _, segPath := range segments {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := os.ReadFile(segPath)
		if err != nil {
			return err
		}

		for blockStart := 0; blockStart < len(data); blockStart += BlockSize {
			if err := ctx.Err(); err != nil {
				return err
			}
			blockEnd := min(blockStart+BlockSize, len(data))
			block := data[blockStart:blockEnd]
			pos := 0

		posLoop:
			for pos+HeaderSize <= len(block) {
				headerBytes := block[pos : pos+HeaderSize]

				// Rest of the block is just zeros, nothing more here
				isZeroHeader := headerBytes[0] == 0 && headerBytes[1] == 0 &&
					headerBytes[2] == 0 && headerBytes[3] == 0 &&
					headerBytes[4] == 0 && headerBytes[5] == 0 && headerBytes[6] == 0
				if isZeroHeader {
					break posLoop
				}

				crc, length, kind, ok := DecodeHeader(headerBytes)
				if !ok {
					// We ran off the end of the file mid-header, so stop here
					return nil
				}

				// Make sure we recognize this kind
				switch kind {
				case KindFull, KindFirst, KindMiddle, KindLast:
					// Good, we know how to handle it
				case KindZero:
					// Just padding at the end of a block
					break posLoop
				default:
					// We have never seen this kind before, log is broken here
					if reader.verify {
						return nil
					}
					break posLoop
				}

				// Payload should fit comfortably in what is left of the block
				if int(length) > len(block)-pos-HeaderSize {
					// File was cut off or torn in the middle of a record
					if reader.verify {
						return nil
					}
					break posLoop
				}

				payload := block[pos+HeaderSize : pos+HeaderSize+int(length)]

				// Checksum is over kind and payload, so any torn write will not match
				if !VerifyHeader(crc, kind, payload) {
					if reader.verify {
						return nil
					}
					break posLoop
				}

				// Now we stitch the pieces of a big record back together
				switch kind {
				case KindFull:
					switch {
					case fragActive && reader.verify:
						return nil
					case fragActive:
						fragActive = false
					}
					if err := fn(payload); err != nil {
						return err
					}
				case KindFirst:
					if fragActive && reader.verify {
						return nil
					}
					// Previous fragment was incomplete, drop it and start new
					fragBuffer = append([]byte(nil), payload...)
					fragActive = true
				case KindMiddle:
					switch {
					case !fragActive && reader.verify:
						return nil
					case !fragActive:
						break posLoop
					default:
						fragBuffer = append(fragBuffer, payload...)
					}
				case KindLast:
					switch {
					case !fragActive && reader.verify:
						return nil
					case !fragActive:
						break posLoop
					default:
						fragBuffer = append(fragBuffer, payload...)
						complete := fragBuffer
						fragBuffer = nil
						fragActive = false
						if err := fn(complete); err != nil {
							return err
						}
					}
				}

				pos += HeaderSize + int(length)
			}
		}
	}

	return nil
}

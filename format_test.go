package ledger

import (
	"testing"
)

// Mask and Unmask cancel each other.
func TestMaskUnmask(t *testing.T) {
	values := []uint32{0, 1, 0xa282ead8, 0xffffffff, 12345}
	for _, v := range values {
		masked := Mask(v)
		got := Unmask(masked)
		if got != v {
			t.Fatalf("Mask/Unmask failed for %d: got %d", v, got)
		}
	}
}

// A zero file must not look valid.
func TestMaskZeroNotZero(t *testing.T) {
	if Mask(0) == 0 {
		t.Fatal("Mask(0) should not be zero")
	}
}

// Encode then decode should give back the same header.
func TestEncodeDecodeHeader(t *testing.T) {
	payload := []byte("hello")
	h := EncodeHeader(len(payload), KindFull, payload)
	crc, length, kind, ok := DecodeHeader(h[:])
	if !ok {
		t.Fatal("DecodeHeader failed")
	}
	if length != uint16(len(payload)) {
		t.Fatalf("length mismatch: got %d want %d", length, len(payload))
	}
	if kind != KindFull {
		t.Fatalf("kind mismatch: got %d want %d", kind, KindFull)
	}
	if !VerifyHeader(crc, kind, payload) {
		t.Fatal("valid payload should verify")
	}
	bad := []byte("hellx")
	if VerifyHeader(crc, kind, bad) {
		t.Fatal("altered payload should not verify")
	}
	if VerifyHeader(crc, KindFirst, payload) {
		t.Fatal("wrong kind should not verify")
	}
}

// Decode needs 7 bytes.
func TestDecodeHeaderShort(t *testing.T) {
	short := make([]byte, HeaderSize-1)
	if _, _, _, ok := DecodeHeader(short); ok {
		t.Fatal("should fail for short input")
	}
}

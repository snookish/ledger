package wal

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

// NeedsFragment checks if a payload fits in the current block.
func TestNeedsFragment(t *testing.T) {
	if !NeedsFragment(10, 10) {
		t.Fatal("should need fragment when not enough space")
	}
	if NeedsFragment(BlockSize, 10) {
		t.Fatal("should not need fragment when block is empty and payload fits")
	}
	if !NeedsFragment(BlockSize, MaxPayloadPerFragment+1) {
		t.Fatal("should need fragment when payload is too large for any block")
	}
}

// FragmentCount returns how many pieces a record needs.
func TestFragmentCount(t *testing.T) {
	if n := FragmentCount(100, BlockSize); n != 1 {
		t.Fatalf("got %d, want 1", n)
	}
	if n := FragmentCount(0, BlockSize); n != 1 {
		t.Fatalf("got %d, want 1", n)
	}
	large := MaxPayloadPerFragment*2 + 100
	if n := FragmentCount(large, BlockSize); n != 3 {
		t.Fatalf("got %d, want 3 for large payload", n)
	}
	if n := FragmentCount(100, HeaderSize+50); n != 2 {
		t.Fatalf("got %d, want 2 when block nearly full", n)
	}
	// Fits exactly in the space left.
	if n := FragmentCount(50, HeaderSize+50+7); n != 1 {
		t.Fatalf("got %d, want 1 for exact fit", n)
	}
	// No space left at all, needs a new block.
	if n := FragmentCount(10, 0); n != 1 {
		t.Fatalf("got %d, want 1 when no space left", n)
	}
}

package hashw

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func TestU64IndependentFraming(t *testing.T) {
	w := New()
	w.U64(0x0102030405060708)
	raw := []byte{4, 0, 0, 0, 8, 1, 2, 3, 4, 5, 6, 7, 8}
	expected := sha256.Sum256(raw)
	if !bytes.Equal(w.Sum(), expected[:]) {
		t.Fatal("U64 framing differs from independent byte vector")
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], 0x0102030405060708)
	if !bytes.Equal(b[:], raw[5:]) {
		t.Fatal("bad vector")
	}
}

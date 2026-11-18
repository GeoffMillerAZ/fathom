// Package hashw provides the framed SHA-256 writer for canonical keys.
// Each write carries a kind tag, a big-endian length, and a payload.
// The frame distinguishes value boundaries, strings from raw bytes,
// and empty writes from omitted writes.
package hashw

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

// Kind tags. Values are part of the key format: reordering or reusing
// one is a silent rekey of every fleet, which is what the keyEpoch — not
// an edit here — is for.
const (
	tagStr   byte = 0x01
	tagU32   byte = 0x02
	tagBytes byte = 0x03
)

// Writer accumulates framed writes over sha256. The zero value is not
// usable; New is the constructor.
type Writer struct {
	h hash.Hash
}

// New returns a Writer over a fresh sha256 state.
func New() *Writer {
	return &Writer{h: sha256.New()}
}

// frame writes one tagged, length-prefixed payload. The length prefix
// is what makes write boundaries part of the digest.
func (w *Writer) frame(tag byte, payload []byte) {
	var hdr [5]byte
	hdr[0] = tag
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(payload)))
	w.h.Write(hdr[:])
	w.h.Write(payload)
}

// Str hashes one string value.
func (w *Writer) Str(s string) {
	w.frame(tagStr, []byte(s))
}

// U32 hashes one fixed-width integer (epochs, mode bits).
func (w *Writer) U32(v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	w.frame(tagU32, b[:])
}

// Bytes hashes one raw byte value (content digests).
func (w *Writer) Bytes(b []byte) {
	w.frame(tagBytes, b)
}

// Sum returns the sha256 of everything written so far. It does not
// consume the writer: further writes extend the same sequence.
func (w *Writer) Sum() []byte {
	return w.h.Sum(nil)
}

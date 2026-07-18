package eventstore

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// hashPayload returns the lowercase hex-encoded SHA-256 digest of b.
// Returns the digest of an empty input when b is nil or empty.
func hashPayload(b []byte) string {
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}

// hashEntry computes the canonical entry Hash over:
//
//	Seq (big-endian uint64, 8 bytes)
//	Kind (UTF-8 bytes)
//	NUL separator (1 byte)
//	PayloadHash (ASCII hex bytes, 64 bytes)
//	NUL separator (1 byte)
//	PrevHash (ASCII hex bytes, 64 bytes)
//	NUL separator (1 byte)
//	RecordedAtNano (big-endian int64, 8 bytes — Unix nano UTC)
//
// Using Unix nanoseconds (not RFC3339) for the timestamp avoids sub-second
// rounding and makes the canonical form a deterministic byte sequence that is
// trivially machine-verifiable without string parsing.
func hashEntry(seq uint64, kind, payloadHash, prevHash string, recordedAtNano int64) string {
	h := sha256.New()

	var seqBuf [8]byte
	binary.BigEndian.PutUint64(seqBuf[:], seq)
	h.Write(seqBuf[:])

	h.Write([]byte(kind))
	h.Write([]byte{0})

	h.Write([]byte(payloadHash))
	h.Write([]byte{0})

	h.Write([]byte(prevHash))
	h.Write([]byte{0})

	var tsBuf [8]byte
	// Safe cast: Unix nano fits in int64 for any foreseeable date.
	//nolint:gosec
	binary.BigEndian.PutUint64(tsBuf[:], uint64(recordedAtNano))
	h.Write(tsBuf[:])

	return hex.EncodeToString(h.Sum(nil))
}

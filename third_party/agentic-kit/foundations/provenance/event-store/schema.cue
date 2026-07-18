// Package eventstore defines the CUE-first contract for the tamper-evident,
// append-only, hash-chained event store. Go types in types.go and store.go
// mirror these definitions; any drift is a bug.
package eventstore

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Entry is a single record in the hash-chained event log. Every entry
// commits to its predecessor via PrevHash, forming a tamper-evident chain
// that can be verified without trusting any external state.
#Entry: {
	// Seq is the 0-based position of this entry in the chain.
	Seq: >=0

	// Kind is the event kind discriminator. Must be non-empty.
	Kind: _nonEmpty

	// PayloadHash is the lowercase hex-encoded SHA-256 digest of the raw
	// payload bytes associated with this entry.
	PayloadHash: _nonEmpty

	// PrevHash is the Hash of the immediately preceding entry. For the
	// genesis entry (Seq == 0) this is a string of 64 ASCII zero characters.
	PrevHash: string

	// Hash is the lowercase hex-encoded SHA-256 digest computed over the
	// canonical concatenation of (Seq, Kind, PayloadHash, PrevHash, RecordedAt).
	Hash: _nonEmpty

	// RecordedAt is the RFC3339-formatted UTC wall-clock time at which the
	// entry was appended to the store. Note: in the canonical hash (Hash field),
	// RecordedAt is encoded as a big-endian int64 Unix nanosecond timestamp —
	// not as an RFC3339 string — to avoid sub-second rounding ambiguity. This
	// CUE field carries the human-readable form; the hashed form is UnixNano.
	RecordedAt: _nonEmpty
}

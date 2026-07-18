// Package eventstore provides an append-only, hash-chained event store. It is
// the system-of-record foundation for legal chain-of-custody: every entry
// commits to its predecessor, enabling offline chain verification without
// trusting any projection or secondary store.
//
// Threat model: Verify detects any edit by a party who cannot recompute
// SHA-256, and any reordering, middle-drop, or Seq-gap. It does NOT detect
// authorized rewrites by a holder who recomputes the chain (e.g. backdating
// RecordedAt then recomputing all hashes), nor tail truncation (dropping the
// last N entries yields a valid shorter chain). Detecting those attacks
// requires anchoring Head().Hash and the entry count in an external trusted
// store (e.g. a transparency log or notary service).
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema. store.go provides the Store implementation;
// hash.go provides the canonical hashing helpers.
//
// Concurrency: Store is safe for concurrent use via sync.RWMutex.
package eventstore

import (
	"fmt"
	"time"
)

// GenesisPrevHash is the PrevHash value for the first entry in a chain (Seq 0).
// It is a string of 64 ASCII zero characters, representing a zeroed SHA-256 digest.
const GenesisPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Entry is a single record in the hash-chained event log.
//
// CUE mirror: #Entry in schema.cue.
type Entry struct {
	// Seq is the 0-based position of this entry in the chain.
	Seq uint64

	// Kind is the event kind discriminator. Must be non-empty.
	Kind string

	// Payload is the raw bytes associated with this event. It is not stored in
	// the CUE schema (binary is opaque to CUE), but its SHA-256 digest is
	// recorded in PayloadHash.
	Payload []byte

	// PayloadHash is the lowercase hex-encoded SHA-256 digest of Payload.
	PayloadHash string

	// PrevHash is the Hash of the immediately preceding entry. For Seq 0
	// it equals GenesisPrevHash.
	PrevHash string

	// Hash is the lowercase hex-encoded SHA-256 digest over the canonical
	// concatenation:
	//   Seq (big-endian uint64) || Kind (UTF-8) || NUL ||
	//   PayloadHash (ASCII hex) || NUL ||
	//   PrevHash (ASCII hex)    || NUL ||
	//   RecordedAt (Unix nano, big-endian int64)
	//
	// Using Unix nano (not RFC3339) for the timestamp component avoids
	// sub-second rounding ambiguity and makes the canonical form a
	// fixed-width byte sequence that is trivially machine-verifiable.
	Hash string

	// RecordedAt is the UTC wall-clock time at which the entry was appended.
	RecordedAt time.Time
}

// Validate returns an error if any invariant of the Entry is violated.
//
// Checked invariants:
//   - Kind must be non-empty.
//   - PayloadHash must be non-empty.
//   - Hash must be non-empty.
//   - PrevHash must be non-empty (GenesisPrevHash for Seq 0; prior Hash otherwise).
//   - RecordedAt must be non-zero.
func (e Entry) Validate() error {
	if e.Kind == "" {
		return fmt.Errorf("eventstore: entry seq=%d: Kind must not be empty", e.Seq)
	}
	if e.PayloadHash == "" {
		return fmt.Errorf("eventstore: entry seq=%d: PayloadHash must not be empty", e.Seq)
	}
	if e.Hash == "" {
		return fmt.Errorf("eventstore: entry seq=%d: Hash must not be empty", e.Seq)
	}
	if e.PrevHash == "" {
		return fmt.Errorf("eventstore: entry seq=%d: PrevHash must not be empty", e.Seq)
	}
	if e.RecordedAt.IsZero() {
		return fmt.Errorf("eventstore: entry seq=%d: RecordedAt must not be zero", e.Seq)
	}
	return nil
}

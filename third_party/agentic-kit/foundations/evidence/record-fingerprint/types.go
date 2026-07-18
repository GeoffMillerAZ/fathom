// Package recordfingerprint provides a content-hash-based record deduplication
// primitive. It detects whether a re-ingested record is new, unchanged, or
// changed relative to the last observed version — without relying on record
// IDs alone.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency contract: Store is safe for concurrent use; all methods are
// guarded by a sync.RWMutex. Fingerprint is a plain value type safe to read
// concurrently without additional synchronization.
package recordfingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"
)

// ChangeKind classifies the result of an Observe or ObserveHash call.
//
// CUE mirror: #ChangeKind in schema.cue.
type ChangeKind string

const (
	// ChangeNew means the record ID has never been seen before.
	ChangeNew ChangeKind = "new"
	// ChangeUnchanged means the record ID was seen before with the same content hash.
	ChangeUnchanged ChangeKind = "unchanged"
	// ChangeChanged means the record ID was seen before but the content hash differs.
	ChangeChanged ChangeKind = "changed"
)

// sha256HexRE matches exactly 64 lowercase hexadecimal characters — the only
// format produced by HashContent and accepted by this package.
var sha256HexRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Fingerprint captures the last-seen content hash for a record ID.
//
// CUE mirror: #Fingerprint in schema.cue.
type Fingerprint struct {
	// RecordID is the stable identifier of the record. Must be non-empty.
	RecordID string `json:"RecordID"`

	// ContentHash is the sha256 hex digest of the record's content bytes.
	// Must be exactly 64 lowercase hex characters.
	ContentHash string `json:"ContentHash"`

	// SeenAt is the time this fingerprint was first or last stored (see Store
	// semantics). Must be non-zero; serialised as RFC3339.
	SeenAt time.Time `json:"SeenAt"`
}

// Validate returns an error if any field is invalid.
// Rules:
//   - RecordID must be non-empty.
//   - ContentHash must be exactly 64 lowercase hex characters (SHA-256 digest).
//   - SeenAt must be a non-zero time.
func (f Fingerprint) Validate() error {
	if f.RecordID == "" {
		return fmt.Errorf("recordfingerprint: RecordID must not be empty")
	}
	if !sha256HexRE.MatchString(f.ContentHash) {
		return fmt.Errorf("recordfingerprint: ContentHash must be a 64-char lowercase hex SHA-256 digest, got %q", f.ContentHash)
	}
	if f.SeenAt.IsZero() {
		return fmt.Errorf("recordfingerprint: SeenAt must be a non-zero time")
	}
	return nil
}

// HashContent returns the sha256 hex digest of b. It is the canonical helper
// for computing ContentHash values consistent with this package.
func HashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Store tracks the last-seen fingerprint for each record ID. The zero value is
// not usable; create one with NewStore.
type Store struct {
	mu   sync.RWMutex
	byID map[string]Fingerprint
	now  func() time.Time
}

// NewStore returns an empty, ready-to-use Store with a real wall-clock.
// Pass a custom now function to control time in tests.
func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		byID: make(map[string]Fingerprint),
		now:  now,
	}
}

// Observe hashes content and compares it against the stored fingerprint for
// recordID:
//   - If recordID is unknown → ChangeNew; stores the fingerprint.
//   - If recordID is known with the same hash → ChangeUnchanged; stored
//     fingerprint is NOT updated (SeenAt is preserved from first observation).
//   - If recordID is known with a different hash → ChangeChanged; stored
//     fingerprint is updated with the new hash and current time.
//
// Returns the kind, the current stored fingerprint after the call, and any
// validation error. On error the returned ChangeKind and Fingerprint are zero.
func (s *Store) Observe(recordID string, content []byte) (ChangeKind, Fingerprint, error) {
	return s.ObserveHash(recordID, HashContent(content))
}

// ObserveHash behaves like Observe but accepts a precomputed content hash.
// Use this when the content has already been hashed upstream to avoid redundant
// sha256 work.
//
// Returns an error if recordID is empty or contentHash is not a valid 64-char
// lowercase hex SHA-256 digest. On error the returned ChangeKind and Fingerprint
// are zero values and nothing is stored.
func (s *Store) ObserveHash(recordID, contentHash string) (ChangeKind, Fingerprint, error) {
	// Validate inputs before acquiring the lock so that invalid calls never
	// corrupt the store's state.
	if recordID == "" {
		return "", Fingerprint{}, fmt.Errorf("recordfingerprint: RecordID must not be empty")
	}
	if !sha256HexRE.MatchString(contentHash) {
		return "", Fingerprint{}, fmt.Errorf("recordfingerprint: ContentHash must be a 64-char lowercase hex SHA-256 digest, got %q", contentHash)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, found := s.byID[recordID]
	if !found {
		fp := Fingerprint{
			RecordID:    recordID,
			ContentHash: contentHash,
			SeenAt:      s.now(),
		}
		s.byID[recordID] = fp
		return ChangeNew, fp, nil
	}

	if existing.ContentHash == contentHash {
		return ChangeUnchanged, existing, nil
	}

	// Hash differs: update stored fingerprint with new hash and current time.
	fp := Fingerprint{
		RecordID:    recordID,
		ContentHash: contentHash,
		SeenAt:      s.now(),
	}
	s.byID[recordID] = fp
	return ChangeChanged, fp, nil
}

// Get returns the stored fingerprint for recordID and whether it was found.
func (s *Store) Get(recordID string) (Fingerprint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fp, ok := s.byID[recordID]
	return fp, ok
}

// All returns a snapshot of all stored fingerprints sorted by RecordID.
// The order is deterministic across calls.
func (s *Store) All() []Fingerprint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Fingerprint, 0, len(s.byID))
	for _, fp := range s.byID {
		out = append(out, fp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RecordID < out[j].RecordID
	})
	return out
}

// Len returns the number of distinct record IDs tracked by the Store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}

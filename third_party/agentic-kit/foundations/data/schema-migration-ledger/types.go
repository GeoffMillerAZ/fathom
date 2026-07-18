// Package schemamigrationledger provides an append-only ledger of schema
// migrations applied to the data layer. Entries are deterministically
// replayable: the sequence of Append calls always produces the same ledger.
//
// The Go types mirror the CUE definitions in schema.cue.
// CUE is authoritative for data shape; Go is authoritative for behaviour.
//
// Concurrency contract: Ledger is safe for concurrent use; all methods are
// guarded by a sync.RWMutex. MigrationRecord is a plain value type safe to
// read concurrently without additional synchronization.
package schemamigrationledger

import (
	"errors"
	"fmt"
	"sync"
)

// MigrationRecord represents a single schema migration that has been applied
// to the data layer. It mirrors #MigrationRecord in schema.cue.
type MigrationRecord struct {
	// ID is a non-empty, unique migration identifier (e.g. "0001_add_users").
	ID string

	// From is the schema version before this migration was applied.
	// May be empty for the very first migration in a ledger.
	From string

	// To is the schema version after this migration was applied. Must be
	// non-empty.
	To string

	// AppliedAt is the Unix timestamp (seconds) when the migration was applied.
	AppliedAt int64
}

// validate checks the record against the same invariants enforced by CUE.
func (r MigrationRecord) validate() error {
	if r.ID == "" {
		return errors.New("schemamigrationledger: MigrationRecord.ID must not be empty")
	}
	if r.To == "" {
		return errors.New("schemamigrationledger: MigrationRecord.To must not be empty")
	}
	if r.AppliedAt <= 0 {
		return fmt.Errorf("schemamigrationledger: MigrationRecord.AppliedAt must be positive, got %d", r.AppliedAt)
	}
	return nil
}

// Ledger is an append-only, in-memory ledger of schema migrations.
// The zero value is not usable; create one with NewLedger.
type Ledger struct {
	mu      sync.RWMutex
	entries []MigrationRecord
	byID    map[string]struct{}
}

// NewLedger returns an empty, ready-to-use Ledger.
func NewLedger() *Ledger {
	return &Ledger{
		byID: make(map[string]struct{}),
	}
}

// Append validates r and appends it to the ledger.
// Returns an error if:
//   - r is invalid (empty ID, empty To, or non-positive AppliedAt), or
//   - a record with the same ID has already been appended.
//
// Append is safe for concurrent use.
func (l *Ledger) Append(r MigrationRecord) error {
	if err := r.validate(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.byID[r.ID]; exists {
		return fmt.Errorf("schemamigrationledger: migration %q has already been applied", r.ID)
	}
	l.entries = append(l.entries, r)
	l.byID[r.ID] = struct{}{}
	return nil
}

// Applied returns a snapshot of all migration records in the order they were
// appended. The slice is a copy — callers may not mutate the ledger through it.
func (l *Ledger) Applied() []MigrationRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]MigrationRecord, len(l.entries))
	copy(out, l.entries)
	return out
}

// Current returns the schema version produced by the most recently appended
// migration (i.e. the To field of the last entry), or an empty string if the
// ledger is empty.
func (l *Ledger) Current() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) == 0 {
		return ""
	}
	return l.entries[len(l.entries)-1].To
}

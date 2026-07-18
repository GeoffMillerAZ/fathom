// Package coverage implements the completeness ledger for the investigation
// engine. For every source it tracks how many records are known, scanned,
// failed, and pending, making "known unknowns" a queryable first-class fact
// and ensuring no silent truncation of unscanned tails.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Relationship to convergence-oracle: convergence-oracle answers "have we
// found enough?" (quality/agreement of findings); coverage answers "have we
// looked everywhere?" (completeness of source ingestion). They are
// complementary halves of the investigation prime directive.
//
// Resume checkpoints: SourceCoverage.Cursor and SourceCoverage.LastSync are
// the canonical per-source resume-checkpoint mechanism in the kit. An
// ingester persists its opaque resume cursor and last-sync time on the
// source's coverage entry and reads them back to continue incremental
// ingestion — checkpointing is part of the completeness ledger, not a
// separate concern. (The former foundations/communications/
// source-sync-checkpoint package was retired in favour of these fields; the
// only proven workload stored its cursors here.)
//
// Concurrency contract: Tracker is safe for concurrent use; all methods are
// guarded by a sync.RWMutex. SourceCoverage and Summary are plain value types
// safe to read concurrently.
package coverage

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// SourceCoverage records the ingestion state for a single source. All count
// fields must satisfy: RecordsScanned + RecordsFailed + RecordsPending <=
// RecordsKnown.
//
// CUE mirror: #SourceCoverage in schema.cue.
type SourceCoverage struct {
	// SourceID uniquely identifies the source. Must be non-empty.
	SourceID string

	// RecordsKnown is the total number of records the source advertises.
	RecordsKnown int

	// RecordsScanned is the number of records fully ingested.
	RecordsScanned int

	// RecordsFailed is the number of records attempted but errored.
	RecordsFailed int

	// RecordsPending is the number of known records not yet scanned.
	RecordsPending int

	// LastSync is the timestamp of the last synchronisation attempt.
	// Zero value means no sync has occurred.
	LastSync time.Time

	// Cursor is an opaque resume cursor for incremental ingestion.
	// Empty string means no cursor is held.
	Cursor string
}

// Validate returns an error if the SourceCoverage is malformed.
//
// Rules:
//   - SourceID must be non-empty.
//   - All count fields must be >= 0.
//   - RecordsScanned + RecordsFailed + RecordsPending must be <= RecordsKnown.
func (c SourceCoverage) Validate() error {
	if c.SourceID == "" {
		return fmt.Errorf("coverage: SourceID must not be empty")
	}
	if c.RecordsKnown < 0 {
		return fmt.Errorf("coverage: RecordsKnown must be >= 0, got %d", c.RecordsKnown)
	}
	if c.RecordsScanned < 0 {
		return fmt.Errorf("coverage: RecordsScanned must be >= 0, got %d", c.RecordsScanned)
	}
	if c.RecordsFailed < 0 {
		return fmt.Errorf("coverage: RecordsFailed must be >= 0, got %d", c.RecordsFailed)
	}
	if c.RecordsPending < 0 {
		return fmt.Errorf("coverage: RecordsPending must be >= 0, got %d", c.RecordsPending)
	}
	if c.RecordsScanned+c.RecordsFailed+c.RecordsPending > c.RecordsKnown {
		return fmt.Errorf(
			"coverage: RecordsScanned(%d) + RecordsFailed(%d) + RecordsPending(%d) = %d exceeds RecordsKnown(%d)",
			c.RecordsScanned, c.RecordsFailed, c.RecordsPending,
			c.RecordsScanned+c.RecordsFailed+c.RecordsPending,
			c.RecordsKnown,
		)
	}
	return nil
}

// Ratio returns the fraction of known records that have been scanned.
// Returns 0 when RecordsKnown is 0 to avoid division by zero.
func (c SourceCoverage) Ratio() float64 {
	if c.RecordsKnown == 0 {
		return 0
	}
	return float64(c.RecordsScanned) / float64(c.RecordsKnown)
}

// KnownUnknowns returns the number of records the system knows about but has
// not yet successfully scanned or definitively failed. This is the primary
// "unscanned tail" metric that the no-silent-truncation invariant requires to
// be surfaced explicitly.
//
// KnownUnknowns = RecordsKnown - RecordsScanned - RecordsFailed
//
// Clamped at 0; a well-formed (Validate-passing) coverage never underflows,
// but direct construction might.
func (c SourceCoverage) KnownUnknowns() int {
	v := c.RecordsKnown - c.RecordsScanned - c.RecordsFailed
	if v < 0 {
		return 0
	}
	return v
}

// Complete reports whether the source is fully processed: all known records
// have been scanned, none are pending, none have failed, and at least one
// record is known.
func (c SourceCoverage) Complete() bool {
	return c.RecordsKnown > 0 &&
		c.RecordsPending == 0 &&
		c.RecordsFailed == 0 &&
		c.RecordsScanned == c.RecordsKnown
}

// Overall is the aggregate coverage across all sources in a Tracker.
type Overall struct {
	// Known is the sum of RecordsKnown across all sources.
	Known int

	// Scanned is the sum of RecordsScanned across all sources.
	Scanned int

	// Failed is the sum of RecordsFailed across all sources.
	Failed int

	// Pending is the sum of RecordsPending across all sources.
	Pending int

	// Ratio is Scanned/Known (0 when Known == 0).
	Ratio float64

	// Complete is true when every source reports Complete().
	Complete bool
}

// Tracker holds SourceCoverage entries keyed by SourceID. It is safe for
// concurrent use.
type Tracker struct {
	mu       sync.RWMutex
	bySource map[string]SourceCoverage
}

// NewTracker returns an empty, ready-to-use Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		bySource: make(map[string]SourceCoverage),
	}
}

// Set validates c and stores it in the Tracker, replacing any existing entry
// for the same SourceID. Returns an error if Validate() fails.
func (t *Tracker) Set(c SourceCoverage) error {
	if err := c.Validate(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bySource[c.SourceID] = c
	return nil
}

// Get returns the SourceCoverage for the given sourceID and whether it was
// found.
func (t *Tracker) Get(sourceID string) (SourceCoverage, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	c, ok := t.bySource[sourceID]
	return c, ok
}

// All returns all SourceCoverage entries sorted by SourceID for determinism.
func (t *Tracker) All() []SourceCoverage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]SourceCoverage, 0, len(t.bySource))
	for _, c := range t.bySource {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SourceID < out[j].SourceID
	})
	return out
}

// Overall aggregates coverage across all sources in the Tracker. Ratio is
// totalScanned/totalKnown (0 when totalKnown == 0). Complete is true only
// when every source is individually Complete().
func (t *Tracker) Overall() Overall {
	all := t.All()
	var known, scanned, failed, pending int
	complete := len(all) > 0
	for _, c := range all {
		known += c.RecordsKnown
		scanned += c.RecordsScanned
		failed += c.RecordsFailed
		pending += c.RecordsPending
		if !c.Complete() {
			complete = false
		}
	}
	ratio := 0.0
	if known > 0 {
		ratio = float64(scanned) / float64(known)
	}
	return Overall{
		Known:    known,
		Scanned:  scanned,
		Failed:   failed,
		Pending:  pending,
		Ratio:    ratio,
		Complete: complete,
	}
}

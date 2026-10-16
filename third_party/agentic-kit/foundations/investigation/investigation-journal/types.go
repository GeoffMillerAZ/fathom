// Package investigationjournal provides an append-only journal of investigation
// steps. Every meaningful action taken during an investigation is recorded as
// an Entry so the investigation is fully replayable.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// # Hash-chaining and Tier-1 ledger
//
// This package is a higher-level projection layer, not Throughline's Tier-1
// append-only ledger. Per-entry hash-chaining and content-addressed identity
// (PrevHash pointers, chain-verification) are the responsibility of the
// provenance/event-store foundation, which provides a replayable,
// hash-chained backing store. Callers that require tamper-evidence should
// persist journal entries via event-store and reconstruct from it on reload.
//
// # Persistence
//
// The journal is in-memory; durability is opt-in via an entry sink. Attach a
// sink with AttachSink and every successful Append/AppendTraced is delivered
// to it synchronously, in journal order, before the entry is added to the
// in-memory log (see AttachSink for the exact ordering contract). JSONLSink
// adapts a persistence/jsonl-log Writer into such a sink; LoadJSONL reads a
// JSONL stream back into entries and NewFromEntries rebuilds a Journal from
// them — together they form the journal→JSONL→journal round trip.
//
// Concurrency: a Journal is NOT safe for concurrent use — callers must
// serialize access. It must not be copied after first use.
package investigationjournal

import (
	"fmt"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// Entry is a single, immutable record in the journal.
//
// CUE mirror: #Entry in schema.cue.
type Entry struct {
	// Step is the name or label of the investigation step (e.g.
	// "hypothesis-formed", "evidence-collected", "conclusion-reached").
	// Must be non-empty.
	Step string

	// Note is a human-readable description of what happened at this step.
	// Must be non-empty.
	Note string

	// At is the wall-clock instant when this entry was recorded.
	// Must be non-zero.
	At time.Time

	// Author records who or what produced this entry. Optional for
	// system/internal callers that record deterministic state (e.g. DAD loop
	// phase transitions). When set, Author must be valid per
	// aiprovenance.Author.Validate(). Use AppendTraced for the fully-provenanced
	// path — it requires a valid Author and injects the journal clock.
	Author *aiprovenance.Author

	// SpanRefs holds IDs of evidence spans that this entry references. May be
	// nil. When present the slice is a defensive copy — mutations do not affect
	// the journal's internal state.
	SpanRefs []string
}

// Validate returns an error if the Entry violates any invariant:
//   - Step must be non-empty.
//   - Note must be non-empty.
//   - At must be non-zero.
//   - Author, when non-nil, must pass aiprovenance.Author.Validate().
func (e Entry) Validate() error {
	if e.Step == "" {
		return fmt.Errorf("investigationjournal: Step must not be empty")
	}
	if e.Note == "" {
		return fmt.Errorf("investigationjournal: Note must not be empty")
	}
	if e.At.IsZero() {
		return fmt.Errorf("investigationjournal: At must not be zero")
	}
	if e.Author != nil {
		if err := e.Author.Validate(); err != nil {
			return fmt.Errorf("investigationjournal: invalid Author: %w", err)
		}
	}
	return nil
}

// TracedEntry is the input type for AppendTraced. It requires an Author so
// that every fully-provenanced journal entry is traceable to its producer.
type TracedEntry struct {
	// Step is the name or label of the investigation step. Must be non-empty.
	Step string

	// Note is a human-readable description of what happened. Must be non-empty.
	Note string

	// Author is the human or AI producer of this entry. Must be a valid
	// aiprovenance.Author (i.e. Author.Validate() must return nil).
	Author aiprovenance.Author

	// SpanRefs holds IDs of evidence spans referenced by this entry. May be nil.
	SpanRefs []string
}

// Journal is an append-only sequence of investigation entries. The zero value
// is ready to use with wall-clock timestamps. For deterministic tests construct
// via NewJournalWithClock.
//
// A Journal must not be copied after first use.
type Journal struct {
	entries []Entry
	// now is the time source for AppendTraced. When nil, time.Now is used.
	now func() time.Time
	// sink, when non-nil, is invoked synchronously with every successfully
	// validated entry before it is added to entries. See AttachSink.
	sink func(Entry) error
}

// NewJournalWithClock returns a Journal that uses the supplied clock function
// for AppendTraced timestamps instead of time.Now. This enables deterministic
// tests without mocking global state.
func NewJournalWithClock(clock func() time.Time) *Journal {
	return &Journal{now: clock}
}

// clock returns the effective time source.
func (j *Journal) clock() func() time.Time {
	if j.now != nil {
		return j.now
	}
	return time.Now
}

// AttachSink registers fn as the journal's entry sink. Pass nil to detach.
//
// Once attached, the sink is invoked synchronously on every successful
// Append and AppendTraced, with a defensive copy of the entry (the sink may
// retain or mutate it freely without affecting journal state).
//
// Ordering contract:
//   - The sink observes entries in exactly journal order — it runs after
//     validation succeeds and before the entry is added to the in-memory log.
//   - If the sink returns an error, the error propagates (wrapped) from
//     Append/AppendTraced and the journal is unchanged: the in-memory view
//     never runs ahead of the sink. A durable backing log may run ahead by at
//     most the single failed — possibly torn — write, which jsonl-log's
//     partial-line recovery surfaces on reload.
//   - AppendUnchecked bypasses the sink (see its doc).
//
// AttachSink is typically called once, immediately after construction (or
// after NewFromEntries when resuming from a persisted log). It is not safe to
// call concurrently with appends — the Journal is single-goroutine by
// contract.
func (j *Journal) AttachSink(fn func(Entry) error) {
	j.sink = fn
}

// commit delivers e to the attached sink (if any) and, on sink success,
// appends it to the in-memory log. e must already be validated and owned by
// the journal (defensively copied); the sink receives its own copy.
func (j *Journal) commit(e Entry) error {
	if j.sink != nil {
		if err := j.sink(e.clone()); err != nil {
			return fmt.Errorf("investigationjournal: sink: %w", err)
		}
	}
	j.entries = append(j.entries, e)
	return nil
}

// Append adds e to the journal. It returns an error if:
//   - Step is empty.
//   - Note is empty.
//   - At is zero.
//   - Author is non-nil and fails Author.Validate().
//   - An attached sink (see AttachSink) rejects the entry — in which case the
//     journal is unchanged and the sink's error is returned wrapped.
//
// Append is additive-compatible: nil Author is accepted to support
// system/internal callers (e.g. DAD loop, governed-investigation scenario) that
// record deterministic state without a provenance actor. For the
// fully-provenanced path use AppendTraced, which requires a valid Author and
// injects the journal clock.
func (j *Journal) Append(e Entry) error {
	if err := e.Validate(); err != nil {
		return err
	}
	return j.commit(e.clone())
}

// AppendUnchecked adds e to the journal without any validation. It exists
// solely for migration shims or test helpers that need to inject pre-validated
// entries. Prefer Append or AppendTraced in all production code.
//
// AppendUnchecked does NOT enforce the At/Author invariants — callers are
// responsible for correctness.
//
// AppendUnchecked also bypasses any attached sink (see AttachSink): it has no
// error return, so a sink failure could not propagate and would have to be
// silently dropped — which would violate the sink's never-silent contract.
// Entries injected this way are therefore NOT persisted.
func (j *Journal) AppendUnchecked(e Entry) {
	j.entries = append(j.entries, e.clone())
}

// AppendTraced adds a fully-provenanced entry to the journal. It:
//  1. Validates Author via Author.Validate() — an invalid Author is rejected.
//  2. Validates Step and Note are non-empty.
//  3. Sets At from the journal's clock (injected via NewJournalWithClock, or
//     time.Now() for zero-value journals).
//  4. Stores a defensive copy of SpanRefs (nil in → nil stored).
//
// AppendTraced is the preferred entry point for any step where the producer
// (human or AI) is known — it enforces Throughline invariant 1 (provenance
// mandatory) at the point of write.
//
// Like Append, AppendTraced delivers the entry to any attached sink (see
// AttachSink) before adding it to the in-memory log; a sink error propagates
// wrapped and leaves the journal unchanged.
func (j *Journal) AppendTraced(te TracedEntry) error {
	if te.Step == "" {
		return fmt.Errorf("investigationjournal: Step must not be empty")
	}
	if te.Note == "" {
		return fmt.Errorf("investigationjournal: Note must not be empty")
	}
	if err := te.Author.Validate(); err != nil {
		return fmt.Errorf("investigationjournal: invalid Author: %w", err)
	}

	authorCopy := te.Author.Clone()
	var spansCopy []string
	if te.SpanRefs != nil {
		spansCopy = make([]string, len(te.SpanRefs))
		copy(spansCopy, te.SpanRefs)
	}

	e := Entry{
		Step:     te.Step,
		Note:     te.Note,
		At:       j.clock()(),
		Author:   &authorCopy,
		SpanRefs: spansCopy,
	}
	return j.commit(e)
}

// Entries returns a snapshot of all journal entries in append order. The
// returned slice is a defensive copy — mutations do not affect the journal.
func (j *Journal) Entries() []Entry {
	if len(j.entries) == 0 {
		return nil
	}
	out := make([]Entry, len(j.entries))
	for i, e := range j.entries {
		out[i] = e.clone()
	}
	return out
}

// ReplayAsOf returns a defensive-copy snapshot of all entries with At <= t, in
// append order. It answers the query "as of date X, what had been logged?".
//
// Returns nil when no entries satisfy the predicate.
//
// Note: for tamper-evident replay over a verified hash chain, persist journal
// entries through the provenance/event-store foundation and reconstruct from
// there. This method replays from the in-memory projection only.
func (j *Journal) ReplayAsOf(t time.Time) []Entry {
	var out []Entry
	for _, e := range j.entries {
		if !e.At.After(t) {
			out = append(out, e.clone())
		}
	}
	return out
}

// clone returns a deep copy of e. The returned value shares no pointer-based
// state with the original — SpanRefs and Author are both copied defensively.
func (e Entry) clone() Entry {
	c := e
	if e.Author != nil {
		cp := e.Author.Clone()
		c.Author = &cp
	}
	if e.SpanRefs != nil {
		spans := make([]string, len(e.SpanRefs))
		copy(spans, e.SpanRefs)
		c.SpanRefs = spans
	}
	return c
}

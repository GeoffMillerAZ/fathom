package lead

import (
	"fmt"
	"sync"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// legalTransitions maps (currentStatus, nextStatus) → true for all legal
// state-changing transitions.
//
//	open     → pursuing
//	pursuing → closed | abandoned | open
//	closed   → (terminal)
//	abandoned → (terminal)
var legalTransitions = map[LeadStatus]map[LeadStatus]bool{
	StatusOpen: {
		StatusPursuing: true,
	},
	StatusPursuing: {
		StatusClosed:    true,
		StatusAbandoned: true,
		StatusOpen:      true,
	},
	// StatusClosed and StatusAbandoned are forward-terminal: no outgoing edges.
}

// LeadLog is an append-only, in-memory history of Lead state transitions. Each
// call to Open or Transition appends a new LeadEntry to the log; no existing
// entry is ever mutated. The log is safe for concurrent use.
//
// The clock is injected at construction time so that all RecordedAt timestamps
// are deterministic in tests. Production code should use NewLeadLog().
type LeadLog struct {
	mu      sync.RWMutex
	entries []LeadEntry
	heads   map[string]int // leadID → index of most-recent entry in entries
	clock   func() time.Time
}

// NewLeadLog returns a LeadLog that uses time.Now().UTC() as its clock.
func NewLeadLog() *LeadLog {
	return &LeadLog{
		heads: make(map[string]int),
		clock: func() time.Time { return time.Now().UTC() },
	}
}

// NewLeadLogWithClock returns a LeadLog that uses the supplied clock function.
// Inject a deterministic clock in tests to produce reproducible RecordedAt
// values. Panics if clock is nil.
func NewLeadLogWithClock(clock func() time.Time) *LeadLog {
	if clock == nil {
		panic("lead: NewLeadLogWithClock: clock must not be nil")
	}
	return &LeadLog{
		heads: make(map[string]int),
		clock: clock,
	}
}

// Open validates l and appends the initial open entry to the log. Returns an
// error — without appending — if Validate() fails or if a lead with the same
// ID already exists.
func (lg *LeadLog) Open(l Lead) error {
	if err := l.Validate(); err != nil {
		return fmt.Errorf("lead: Open: %w", err)
	}

	lg.mu.Lock()
	defer lg.mu.Unlock()

	if _, exists := lg.heads[l.ID]; exists {
		return fmt.Errorf("lead: Open: lead %q already exists", l.ID)
	}

	entry := LeadEntry{
		Lead:             l.clone(),
		TransitionAuthor: l.Author.Clone(),
		RecordedAt:       lg.clock(),
	}
	idx := len(lg.entries)
	lg.entries = append(lg.entries, entry)
	lg.heads[l.ID] = idx
	return nil
}

// Transition validates the requested status change for the lead identified by
// leadID, then appends a new LeadEntry. Returns ErrNotFound if the lead does
// not exist, or a descriptive error for illegal transitions. On any error,
// the log is not modified.
//
// outcome is recorded in the Lead snapshot's Outcome field; it may be empty
// for transitions that do not produce a final finding (e.g. open→pursuing).
func (lg *LeadLog) Transition(
	leadID string,
	next LeadStatus,
	author aiprovenance.Author,
	outcome string,
) error {
	if err := author.Validate(); err != nil {
		return fmt.Errorf("lead: Transition: author invalid: %w", err)
	}

	lg.mu.Lock()
	defer lg.mu.Unlock()

	headIdx, exists := lg.heads[leadID]
	if !exists {
		return fmt.Errorf("lead: Transition: %w: %q", ErrNotFound, leadID)
	}

	current := lg.entries[headIdx].Lead
	edges, stateKnown := legalTransitions[current.Status]
	if !stateKnown || !edges[next] {
		return fmt.Errorf(
			"lead: Transition: illegal transition for lead %q: %q → %q",
			leadID, current.Status, next,
		)
	}

	updated := current.clone()
	updated.Status = next
	updated.Outcome = outcome
	updated.UpdatedAt = lg.clock()

	entry := LeadEntry{
		Lead:             updated,
		TransitionAuthor: author.Clone(),
		RecordedAt:       updated.UpdatedAt,
	}
	idx := len(lg.entries)
	lg.entries = append(lg.entries, entry)
	lg.heads[leadID] = idx
	return nil
}

// History returns a defensive copy of all LeadEntry values in append order.
// The caller may modify the returned slice without affecting the log's internal
// state.
func (lg *LeadLog) History() []LeadEntry {
	lg.mu.RLock()
	defer lg.mu.RUnlock()

	out := make([]LeadEntry, len(lg.entries))
	for i, e := range lg.entries {
		out[i] = LeadEntry{
			Lead:             e.Lead.clone(),
			TransitionAuthor: e.TransitionAuthor.Clone(),
			RecordedAt:       e.RecordedAt,
		}
	}
	return out
}

// Head returns a defensive copy of the most recent LeadEntry for leadID and
// true, or a zero LeadEntry and false if the lead does not exist.
func (lg *LeadLog) Head(leadID string) (LeadEntry, bool) {
	lg.mu.RLock()
	defer lg.mu.RUnlock()

	idx, ok := lg.heads[leadID]
	if !ok {
		return LeadEntry{}, false
	}
	e := lg.entries[idx]
	return LeadEntry{
		Lead:             e.Lead.clone(),
		TransitionAuthor: e.TransitionAuthor.Clone(),
		RecordedAt:       e.RecordedAt,
	}, true
}

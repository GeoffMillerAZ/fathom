// Package lead tracks open investigative leads and their provenance-bearing
// transition history. Each lead is a thread of inquiry that may originate from
// a hypothesis, a set of evidence spans, or an analyst's prompt. Status changes
// are append-only: every legal transition appends a new LeadEntry to the log
// rather than mutating the previous state.
//
// Design notes:
//   - Distinct from coordination/task-tree: a lead is an investigative concern
//     (epistemics), not an execution unit (scheduling/concurrency).
//   - Clock is injected via NewLeadLogWithClock so tests produce reproducible
//     timestamps without any inline time.Now() call.
//   - All slice fields on Lead carry defensive copies in Validate() and when
//     stored in the log, so callers cannot corrupt stored history via aliasing.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
package lead

import (
	"errors"
	"fmt"
	"time"

	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// ErrNotFound is returned when a lead ID does not exist in the log.
var ErrNotFound = errors.New("lead: not found")

// LeadStatus is the lifecycle state of a Lead.
// CUE mirror: #LeadStatus in schema.cue.
type LeadStatus string

const (
	// StatusOpen is the initial state: the lead has been identified and awaits
	// investigation.
	StatusOpen LeadStatus = "open"

	// StatusPursuing means the lead is actively being investigated.
	StatusPursuing LeadStatus = "pursuing"

	// StatusClosed means the lead has been fully resolved with a recorded outcome.
	// This is a forward-terminal state: no transitions out of it.
	StatusClosed LeadStatus = "closed"

	// StatusAbandoned means the lead was set aside without resolution (e.g. out
	// of scope or evidence too weak). This is a forward-terminal state.
	StatusAbandoned LeadStatus = "abandoned"
)

// validStatuses is the set of all recognised LeadStatus values.
var validStatuses = map[LeadStatus]bool{
	StatusOpen:      true,
	StatusPursuing:  true,
	StatusClosed:    true,
	StatusAbandoned: true,
}

// Lead is a provenance-bearing investigative thread. Every field is part of
// the immutable snapshot stored in a LeadEntry.
//
// CUE mirror: #Lead in schema.cue.
type Lead struct {
	// ID is a non-empty stable identifier for this lead.
	ID string

	// Prompt is the human-readable question or hypothesis that the lead
	// addresses. Must be non-empty.
	Prompt string

	// HypothesisID optionally links this lead to a parent hypothesis.
	HypothesisID string

	// TriggerSpanIDs holds the IDs of evidence spans that triggered this lead.
	// May be empty. Validate performs a defensive copy.
	TriggerSpanIDs []string

	// Assignee is the identifier of the person or agent assigned to the lead.
	// Empty means unassigned.
	Assignee string

	// Status is the current lifecycle state of the lead.
	Status LeadStatus

	// Author records who (human or AI) originated this lead.
	// Must satisfy aiprovenance.Author.Validate().
	Author aiprovenance.Author

	// CreatedAt is the wall-clock time when the lead was first opened.
	CreatedAt time.Time

	// UpdatedAt is the wall-clock time of the most recent status change.
	UpdatedAt time.Time

	// Outcome is a human-readable summary of what was found when the lead
	// was closed or abandoned. Empty on open/pursuing leads.
	Outcome string

	// ResultSpanIDs holds the IDs of evidence spans produced by investigating
	// this lead. May be empty. Validate performs a defensive copy.
	ResultSpanIDs []string
}

// Validate returns an error if the Lead violates any invariant:
//   - ID must be non-empty.
//   - Prompt must be non-empty.
//   - Status must be one of the four valid values.
//   - Author must satisfy aiprovenance.Author.Validate().
//
// Validate also performs defensive copies of TriggerSpanIDs and ResultSpanIDs
// so callers cannot mutate the lead's internal slices after validation.
func (l Lead) Validate() error {
	if l.ID == "" {
		return fmt.Errorf("lead: ID must not be empty")
	}
	if l.Prompt == "" {
		return fmt.Errorf("lead: Prompt must not be empty")
	}
	if !validStatuses[l.Status] {
		return fmt.Errorf("lead: unknown Status %q; must be open|pursuing|closed|abandoned", l.Status)
	}
	if err := l.Author.Validate(); err != nil {
		return fmt.Errorf("lead: Author invalid: %w", err)
	}
	return nil
}

// clone returns a deep copy of the Lead, performing defensive copies of all
// slice fields so the caller and the log never share underlying arrays.
func (l Lead) clone() Lead {
	out := l
	out.Author = l.Author.Clone()
	if l.TriggerSpanIDs != nil {
		out.TriggerSpanIDs = make([]string, len(l.TriggerSpanIDs))
		copy(out.TriggerSpanIDs, l.TriggerSpanIDs)
	}
	if l.ResultSpanIDs != nil {
		out.ResultSpanIDs = make([]string, len(l.ResultSpanIDs))
		copy(out.ResultSpanIDs, l.ResultSpanIDs)
	}
	return out
}

// LeadEntry is one immutable record appended to the LeadLog. It captures the
// full Lead snapshot after each transition, the transition author, and the
// wall-clock time at which it was appended.
//
// CUE mirror: #LeadEntry in schema.cue.
type LeadEntry struct {
	// Lead is the complete Lead snapshot after this transition.
	Lead Lead

	// TransitionAuthor records who authorised this particular transition.
	// For the opening entry this is the same as Lead.Author.
	TransitionAuthor aiprovenance.Author

	// RecordedAt is the wall-clock time set by the LeadLog's injected clock.
	RecordedAt time.Time
}

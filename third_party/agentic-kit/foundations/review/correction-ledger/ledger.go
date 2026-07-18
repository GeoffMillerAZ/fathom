package correctionledger

import (
	"fmt"
	"sync"
	"time"
)

// Ledger is an append-only record of every finding lifecycle event.
// Past events are never mutated or deleted; the current state of any finding
// is derived by replaying the event sequence via Replay.
//
// # Constructors
//
// Use New() for production code and NewWithClock() for tests that require a
// deterministic timestamp. The zero value of Ledger is usable via the legacy
// OpenFindingOrPanic method but cannot participate in error-returning operations
// — prefer New() for all new code.
//
// # Clock injection
//
// Throughline invariant 3 demands deterministic replay: given the same event
// sequence, Replay must always produce the same state. A hard-wired wall clock
// makes replay non-deterministic because timestamps are generated at call time
// rather than being stored in the event. NewWithClock injects a deterministic
// clock for tests and replay harnesses.
type Ledger struct {
	mu     sync.RWMutex
	events []LedgerEvent
	now    func() time.Time
	// knownIDs tracks FindingIDs that have been opened, enabling O(1) duplicate
	// and phantom-reference detection without rescanning the event log.
	knownIDs map[FindingID]struct{}
}

// New returns a new, empty Ledger that uses time.Now().UTC() as its clock.
// Prefer New() over the zero value for all new code; the zero-value Ledger
// lacks the injectable clock and error-returning methods.
func New() *Ledger {
	return &Ledger{
		now:      func() time.Time { return time.Now().UTC() },
		knownIDs: make(map[FindingID]struct{}),
	}
}

// NewWithClock returns a new, empty Ledger that uses the supplied clock
// function. Inject a deterministic clock in tests to produce reproducible
// timestamps and enable true replay-determinism testing.
//
// Panics if clock is nil.
func NewWithClock(clock func() time.Time) *Ledger {
	if clock == nil {
		panic("correctionledger: NewWithClock: clock must not be nil")
	}
	return &Ledger{
		now:      clock,
		knownIDs: make(map[FindingID]struct{}),
	}
}

// clock returns the ledger's clock function, falling back to time.Now().UTC()
// for zero-value Ledger instances (backward compatibility with code that
// constructs a Ledger without New()).
func (l *Ledger) clock() func() time.Time {
	if l.now == nil {
		return func() time.Time { return time.Now().UTC() }
	}
	return l.now
}

// ensureKnownIDsLocked initialises the knownIDs set lazily for zero-value
// Ledger instances, rebuilding it from the event log to stay consistent.
// Must be called with l.mu held for writing.
func (l *Ledger) ensureKnownIDsLocked() {
	if l.knownIDs != nil {
		return
	}
	l.knownIDs = make(map[FindingID]struct{})
	for _, ev := range l.events {
		if ev.Kind == LedgerEventKindFindingOpened && ev.FindingOpened != nil {
			l.knownIDs[ev.FindingOpened.ID] = struct{}{}
		}
	}
}

// Append records a new event at the end of the ledger. The OccurredAt
// timestamp is set to the current UTC time (via the ledger's clock) if it is
// zero, so callers do not need to set it explicitly.
//
// Append never modifies past events.
//
// Callers that hold l.mu.Lock() must call appendLocked instead to avoid
// deadlock; Append is for external callers only.
func (l *Ledger) Append(event LedgerEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.appendLocked(event)
}

// appendLocked is the mutex-free inner body of Append. Must be called with
// l.mu held for writing.
func (l *Ledger) appendLocked(event LedgerEvent) {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = l.clock()()
	}
	l.events = append(l.events, event)
}

// Events returns a defensive deep copy of the ledger event sequence.
// Pointer fields (FindingOpened, Transition, Correction, Dismissal) are
// individually copied so that callers cannot mutate internal ledger state by
// writing through the returned pointers.
//
// The returned slice itself is also a copy, so appending to it or replacing
// elements does not affect the internal state.
func (l *Ledger) Events() []LedgerEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]LedgerEvent, len(l.events))
	for i, ev := range l.events {
		out[i] = ev // copy value fields (Kind, OccurredAt)
		if ev.FindingOpened != nil {
			cp := *ev.FindingOpened
			out[i].FindingOpened = &cp
		}
		if ev.Transition != nil {
			cp := *ev.Transition
			out[i].Transition = &cp
		}
		if ev.Correction != nil {
			cp := *ev.Correction
			if ev.Correction.Author != nil {
				authorCP := *ev.Correction.Author
				cp.Author = &authorCP
			}
			out[i].Correction = &cp
		}
		if ev.Dismissal != nil {
			cp := *ev.Dismissal
			if ev.Dismissal.Author != nil {
				authorCP := *ev.Dismissal.Author
				cp.Author = &authorCP
			}
			out[i].Dismissal = &cp
		}
	}
	return out
}

// Replay reconstructs the current state of every known finding by replaying
// the ledger event sequence from the beginning. Given the same event sequence,
// Replay is deterministic and always produces the same result.
//
// The returned map is keyed by FindingID. Findings not yet in the ledger will
// not appear in the map.
func (l *Ledger) Replay() map[FindingID]Finding {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state := make(map[FindingID]Finding)

	for _, ev := range l.events {
		switch ev.Kind {
		case LedgerEventKindFindingOpened:
			if ev.FindingOpened == nil {
				continue
			}
			f := *ev.FindingOpened
			state[f.ID] = f

		case LedgerEventKindTransition:
			if ev.Transition == nil {
				continue
			}
			t := ev.Transition
			f, ok := state[t.FindingID]
			if !ok {
				// Transition for unknown finding — skip; ledger may be partial.
				continue
			}
			f.State = t.ToState
			f.UpdatedAt = ev.OccurredAt
			state[t.FindingID] = f

		case LedgerEventKindCorrectionAdded:
			// Corrections do not change finding state directly; the
			// corresponding Transition event handles the state change.
			// We record the event for auditability only.

		case LedgerEventKindDismissalAdded:
			// Dismissals do not mutate the underlying fact or finding state;
			// the corresponding Transition event handles the state change.
		}
	}

	return state
}

// OpenFinding creates a new Finding in the open state and appends a
// LedgerEventKindFindingOpened event. The finding's CreatedAt and UpdatedAt
// are both set to the current UTC time via the ledger's clock.
//
// Returns an error if id is empty or if a finding with the same id has already
// been opened in this ledger (duplicate IDs are a data-integrity error for
// legal chain-of-custody purposes).
func (l *Ledger) OpenFinding(id FindingID, description string) (Finding, error) {
	if id == "" {
		return Finding{}, fmt.Errorf("correctionledger: FindingID must not be empty")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.ensureKnownIDsLocked()
	if _, exists := l.knownIDs[id]; exists {
		return Finding{}, fmt.Errorf("correctionledger: finding %q already exists in the ledger; duplicate IDs are a data-integrity error", id)
	}

	now := l.clock()()
	f := Finding{
		ID:          id,
		State:       StateOpen,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	l.appendLocked(LedgerEvent{
		Kind:          LedgerEventKindFindingOpened,
		OccurredAt:    now,
		FindingOpened: &f,
	})
	l.knownIDs[id] = struct{}{}
	return f, nil
}

// OpenFindingOrPanic is a zero-value-compatible wrapper around OpenFinding
// that panics on error. It exists for backward compatibility with code written
// before OpenFinding gained an error return (e.g. zero-value Ledger usage in
// governed-prose-kit). New code should use New() + OpenFinding and handle the
// error explicitly.
func (l *Ledger) OpenFindingOrPanic(id FindingID, description string) Finding {
	f, err := l.OpenFinding(id, description)
	if err != nil {
		panic(err)
	}
	return f
}

// ApplyTransition validates the FSM transition, records it in the ledger,
// and returns the updated Finding. An error is returned for illegal transitions.
//
// The transition's UpdatedAt timestamp is set via the ledger's clock so that
// replay produces deterministic timestamps when a fixed clock is injected.
func (l *Ledger) ApplyTransition(f Finding, event Event) (Finding, error) {
	updated, err := l.transition(f, event)
	if err != nil {
		return Finding{}, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.appendLocked(LedgerEvent{
		Kind:       LedgerEventKindTransition,
		OccurredAt: updated.UpdatedAt,
		Transition: &TransitionEvent{
			FindingID: f.ID,
			Event:     event,
			FromState: f.State,
			ToState:   updated.State,
		},
	})
	return updated, nil
}

// transition applies event to the given finding using the ledger's clock for
// the UpdatedAt timestamp, returning the updated Finding. This is the
// clock-aware internal variant of the standalone Transition function.
func (l *Ledger) transition(f Finding, event Event) (Finding, error) {
	return transitionWithClock(f, event, l.clock())
}

// RecordCorrection appends a Correction event to the ledger after validating
// that all required fields are non-empty (mirroring schema.cue constraints)
// and that the FindingID refers to a finding that was previously opened.
//
// Note: RecordCorrection does NOT advance the finding's FSM state. The caller
// must call ApplyTransition with EventCorrect to record the state change.
func (l *Ledger) RecordCorrection(c Correction) error {
	if err := c.validate(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.ensureKnownIDsLocked()
	if _, exists := l.knownIDs[c.FindingID]; !exists {
		return fmt.Errorf("correctionledger: correction references unknown FindingID %q; open the finding first", c.FindingID)
	}
	if c.CorrectedAt.IsZero() {
		c.CorrectedAt = l.clock()()
	}
	l.appendLocked(LedgerEvent{
		Kind:       LedgerEventKindCorrectionAdded,
		OccurredAt: c.CorrectedAt,
		Correction: &c,
	})
	return nil
}

// RecordDismissal appends a Dismissal event to the ledger after validating
// that all required fields are non-empty (mirroring schema.cue constraints)
// and that the FindingID refers to a finding that was previously opened.
//
// A Dismissal NEVER mutates the underlying fact.
//
// Note: RecordDismissal does NOT advance the finding's FSM state. The caller
// must call ApplyTransition with EventDismiss to record the state change.
//
// The return type was changed from void to error to surface validation failures
// and phantom-ID errors. Callers that previously ignored the return value
// should now check it.
func (l *Ledger) RecordDismissal(d Dismissal) error {
	if err := d.validate(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.ensureKnownIDsLocked()
	if _, exists := l.knownIDs[d.FindingID]; !exists {
		return fmt.Errorf("correctionledger: dismissal references unknown FindingID %q; open the finding first", d.FindingID)
	}
	if d.DismissedAt.IsZero() {
		d.DismissedAt = l.clock()()
	}
	l.appendLocked(LedgerEvent{
		Kind:       LedgerEventKindDismissalAdded,
		OccurredAt: d.DismissedAt,
		Dismissal:  &d,
	})
	return nil
}

package intaketriage

import (
	"fmt"
	"time"
)

// FSM is the intake-triage finite state machine for a single proposal.
// It enforces human-only transitions, reversible exclusion, and an
// append-only transition history.
//
// Zero value is NOT valid; construct via NewFSM.
type FSM struct {
	proposalID      string
	state           ProposalState
	exclusionReason string
	clock           func() time.Time
	history         []TransitionEvent
	suggestions     []Suggestion
}

// NewFSM constructs an FSM for proposalID, starting in StateProposed.
// clock is the injected time source for transition timestamps; it must
// not be nil.
func NewFSM(proposalID string, clock func() time.Time) (*FSM, error) {
	if proposalID == "" {
		return nil, fmt.Errorf("intaketriage: NewFSM: proposalID must not be empty")
	}
	if clock == nil {
		return nil, fmt.Errorf("intaketriage: NewFSM: clock must not be nil")
	}
	return &FSM{
		proposalID: proposalID,
		state:      StateProposed,
		clock:      clock,
	}, nil
}

// ResumeFSM reconstructs an FSM for a proposal previously persisted at any
// lifecycle state, including terminals. It is the hydration seam for
// persistence-backed consumers (ledger/read-model stores) whose proposals do
// not start at StateProposed in memory.
//
// state must be one of the four recognised ProposalStates. exclusionReason
// must be non-empty iff state==StateExcluded: hydrating StateExcluded without
// a reason returns ErrExclusionReasonRequired (a persisted exclusion always
// carries one), and a non-empty reason with any other state is rejected
// because the FSM invariant is that ExclusionReason is cleared outside
// StateExcluded.
//
// Terminal-state hydration (StateImported, StateExcluded) is explicitly
// allowed: consumers hydrate terminals for read and guard purposes (inspecting
// State/ExclusionReason, or letting the FSM reject a stale decision). A
// subsequent Decide on a hydrated terminal returns the same transition error
// it would on a live FSM; Restore on a hydrated StateExcluded works as usual.
//
// History-since-resume semantics: the returned FSM's History is empty and
// records only transitions made after this call. ResumeFSM does NOT replay
// prior transitions — the persisted ledger remains the source of truth for
// history before the resume point.
func ResumeFSM(proposalID string, state ProposalState, exclusionReason string, clock func() time.Time) (*FSM, error) {
	if proposalID == "" {
		return nil, fmt.Errorf("intaketriage: ResumeFSM: proposalID must not be empty")
	}
	if clock == nil {
		return nil, fmt.Errorf("intaketriage: ResumeFSM: clock must not be nil")
	}
	if err := validateState(state); err != nil {
		return nil, fmt.Errorf("intaketriage: ResumeFSM: %w", err)
	}
	if state == StateExcluded && exclusionReason == "" {
		return nil, ErrExclusionReasonRequired
	}
	if state != StateExcluded && exclusionReason != "" {
		return nil, fmt.Errorf("intaketriage: ResumeFSM: exclusionReason must be empty when state is %q (only StateExcluded carries a reason)", state)
	}
	return &FSM{
		proposalID:      proposalID,
		state:           state,
		exclusionReason: exclusionReason,
		clock:           clock,
	}, nil
}

// State returns the current lifecycle state of the proposal.
func (f *FSM) State() ProposalState { return f.state }

// ExclusionReason returns the reason recorded when the proposal was last
// excluded, or empty if the proposal has never been excluded or has been
// restored.
func (f *FSM) ExclusionReason() string { return f.exclusionReason }

// History returns a defensive copy of the append-only transition history.
func (f *FSM) History() []TransitionEvent {
	if len(f.history) == 0 {
		return nil
	}
	out := make([]TransitionEvent, len(f.history))
	copy(out, f.history)
	return out
}

// Suggestions returns a defensive copy of all AI suggestions recorded so far.
func (f *FSM) Suggestions() []Suggestion {
	if len(f.suggestions) == 0 {
		return nil
	}
	out := make([]Suggestion, len(f.suggestions))
	copy(out, f.suggestions)
	return out
}

// Decide records a human state transition. The to state must be one of
// StateImported, StateExcluded, or StateDeferred. When to==StateExcluded,
// reason must not be empty.
//
// Decide returns ErrAIActorForbidden if by.Kind==ActorKindAI.
// Decide returns ErrExclusionReasonRequired if to==StateExcluded and reason
// is empty.
//
// Valid source states:
//   - StateProposed → StateImported | StateExcluded | StateDeferred
//   - StateDeferred → StateImported | StateExcluded
//
// Attempting to transition from a terminal state (StateImported, StateExcluded)
// other than via Restore returns an error.
func (f *FSM) Decide(to ProposalState, reason string, by Actor) error {
	if err := by.Validate(); err != nil {
		return err
	}
	if by.Kind == ActorKindAI {
		return ErrAIActorForbidden
	}
	if to == StateExcluded && reason == "" {
		return ErrExclusionReasonRequired
	}
	if err := f.guardTransition(to); err != nil {
		return err
	}

	prev := f.state
	f.state = to
	if to == StateExcluded {
		f.exclusionReason = reason
	} else {
		f.exclusionReason = ""
	}
	f.history = append(f.history, TransitionEvent{
		ProposalID: f.proposalID,
		From:       prev,
		To:         to,
		By:         by,
		Reason:     reason,
		At:         f.clock(),
	})
	return nil
}

// Restore moves an excluded proposal back to StateProposed, clearing the
// exclusion reason. It returns ErrNotExcluded if the proposal is not
// currently in StateExcluded.
//
// Restore returns ErrAIActorForbidden if by.Kind==ActorKindAI.
func (f *FSM) Restore(by Actor) error {
	if err := by.Validate(); err != nil {
		return err
	}
	if by.Kind == ActorKindAI {
		return ErrAIActorForbidden
	}
	if f.state != StateExcluded {
		return ErrNotExcluded
	}
	prev := f.state
	f.state = StateProposed
	f.exclusionReason = ""
	f.history = append(f.history, TransitionEvent{
		ProposalID: f.proposalID,
		From:       prev,
		To:         StateProposed,
		By:         by,
		At:         f.clock(),
	})
	return nil
}

// Suggest records an AI-generated recommendation without changing state.
// Suggestions are informational only; the FSM never acts on them.
// by must be an AI actor (ActorKindAI); a human actor returns an error.
func (f *FSM) Suggest(proposedState ProposalState, rationale string, confidence float64, by Actor) error {
	if err := by.Validate(); err != nil {
		return err
	}
	if by.Kind != ActorKindAI {
		return fmt.Errorf("intaketriage: Suggest: only AI actors may submit suggestions; use Decide for human transitions")
	}
	if err := validateState(proposedState); err != nil {
		return fmt.Errorf("intaketriage: Suggest: invalid proposedState: %w", err)
	}
	f.suggestions = append(f.suggestions, Suggestion{
		ProposedState: proposedState,
		Rationale:     rationale,
		Confidence:    confidence,
		By:            by,
		At:            f.clock(),
	})
	return nil
}

// guardTransition checks that the requested (current→to) transition is valid.
func (f *FSM) guardTransition(to ProposalState) error {
	if err := validateState(to); err != nil {
		return fmt.Errorf("intaketriage: Decide: %w", err)
	}
	switch f.state {
	case StateProposed:
		// proposed → imported | excluded | deferred
		if to == StateProposed {
			return fmt.Errorf("intaketriage: Decide: transition from %q to %q is not valid (already proposed)", f.state, to)
		}
	case StateDeferred:
		// deferred → imported | excluded (but not back to proposed directly)
		if to == StateProposed || to == StateDeferred {
			return fmt.Errorf("intaketriage: Decide: transition from %q to %q is not valid", f.state, to)
		}
	case StateImported:
		return fmt.Errorf("intaketriage: Decide: proposal is already imported; no further transitions allowed (terminal state)")
	case StateExcluded:
		return fmt.Errorf("intaketriage: Decide: proposal is excluded; use Restore to move it back to proposed first")
	default:
		return fmt.Errorf("intaketriage: Decide: unknown current state %q", f.state)
	}
	return nil
}

// validateState returns an error if s is not a recognized ProposalState.
func validateState(s ProposalState) error {
	switch s {
	case StateProposed, StateImported, StateExcluded, StateDeferred:
		return nil
	default:
		return fmt.Errorf("unknown ProposalState %q", s)
	}
}

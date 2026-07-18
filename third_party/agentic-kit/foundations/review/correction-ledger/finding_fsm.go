package correctionledger

import (
	"fmt"
	"time"
)

// transitionWithClock applies event to the given finding using the supplied
// clock for the UpdatedAt timestamp. This is the shared implementation used
// both by the standalone Transition function and by Ledger.ApplyTransition.
func transitionWithClock(f Finding, event Event, now func() time.Time) (Finding, error) {
	transitions, stateKnown := legalTransitions[f.State]
	if !stateKnown {
		return Finding{}, fmt.Errorf(
			"correctionledger: finding %q is in terminal state %q and cannot be transitioned (event %q rejected)",
			f.ID, f.State, event,
		)
	}
	nextState, ok := transitions[event]
	if !ok {
		return Finding{}, fmt.Errorf(
			"correctionledger: illegal transition for finding %q: %q -(%q)→ (no such edge)",
			f.ID, f.State, event,
		)
	}
	updated := f
	updated.State = nextState
	updated.UpdatedAt = now()
	return updated, nil
}

// legalTransitions maps (currentState, event) → nextState for all legal
// state transitions in the Finding FSM.
//
//	open  → claim      → reviewing
//	reviewing → correct → corrected  (forward-terminal)
//	reviewing → dismiss → dismissed  (forward-terminal)
//	reviewing → defer   → deferred
//	deferred  → reopen  → reviewing
var legalTransitions = map[State]map[Event]State{
	StateOpen: {
		EventClaim: StateReviewing,
	},
	StateReviewing: {
		EventCorrect: StateCorrected,
		EventDismiss: StateDismissed,
		EventDefer:   StateDeferred,
	},
	StateDeferred: {
		EventReopen: StateReviewing,
	},
	// StateCorrected and StateDismissed are forward-terminal:
	// they have no outgoing transitions.
}

// Transition applies event to the given finding and returns the updated
// Finding with its new state, leaving the original value unchanged.
//
// An error is returned for any illegal transition, including:
//   - open → corrected (must go through reviewing)
//   - corrected → open (forward-terminal)
//   - dismissed → open (forward-terminal)
//   - any unrecognised (state, event) pair
//
// Note: Transition is a stateless helper that stamps UpdatedAt with
// time.Now().UTC(). When using a Ledger with an injected clock, prefer
// Ledger.ApplyTransition, which uses the ledger's clock for deterministic
// replay timestamps.
func Transition(f Finding, event Event) (Finding, error) {
	return transitionWithClock(f, event, func() time.Time { return time.Now().UTC() })
}

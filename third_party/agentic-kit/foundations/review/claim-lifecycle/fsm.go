package claimlifecycle

import (
	"fmt"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// legalTransitions maps (currentState, eventKind) → nextState for all legal
// FSM state-changing transitions. Content events (proposed, assertion_linked,
// evidence_linked) do not appear here; their legality is checked separately
// in Transition by state guards.
//
//	proposed → supported|refuted|marked_uncertain
//	supported|refuted|uncertain → reopened → proposed
//	supported|refuted → superseded
//	any state → recanted
var legalTransitions = map[ClaimState]map[ClaimEventKind]ClaimState{
	StateProposed: {
		KindSupported:       StateSupported,
		KindRefuted:         StateRefuted,
		KindMarkedUncertain: StateUncertain,
		KindRecanted:        StateRecanted,
	},
	StateSupported: {
		KindReopened:   StateProposed,
		KindSuperseded: StateSuperseded,
		KindRecanted:   StateRecanted,
	},
	StateRefuted: {
		KindReopened:   StateProposed,
		KindSuperseded: StateSuperseded,
		KindRecanted:   StateRecanted,
	},
	StateUncertain: {
		KindReopened: StateProposed,
		KindRecanted: StateRecanted,
	},
	// StateSuperseded and StateRecanted are forward-terminal: they have no
	// outgoing state-changing transitions. The recanted catch-all below handles
	// the "any → recanted" rule by explicit entry in every non-terminal state.
}

// contentEventStates lists the states in which content events (assertion_linked,
// evidence_linked) may be appended. They are allowed while the claim is open
// (proposed, supported, refuted, uncertain) but not once it is terminal.
var contentEventStates = map[ClaimState]bool{
	StateProposed:  true,
	StateSupported: true,
	StateRefuted:   true,
	StateUncertain: true,
}

// TransitionPayload bundles the optional kind-specific payload for Transition.
// Exactly the fields relevant to the requested event kind need to be set.
type TransitionPayload struct {
	// ProposedText is the claim text used when appending a KindProposed event.
	ProposedText string

	// AssertionLinked is the payload for a KindAssertionLinked event.
	AssertionLinked *AssertionLinkedPayload

	// EvidenceLinked is the payload for a KindEvidenceLinked event.
	EvidenceLinked *EvidenceLinkedPayload
}

// Transition checks whether appending eventKind is legal given the current
// projected state of claimID, then appends a ClaimEvent to ledger. It returns
// an error — without appending — for any illegal transition.
//
// Legal state-changing transitions are defined in legalTransitions. Content
// events (KindProposed, KindAssertionLinked, KindEvidenceLinked) are allowed
// when the claim is in an open state (proposed/supported/refuted/uncertain).
//
// KindProposed is special: it is required as the first event to open a claim
// (projected state "not found" is treated as "may propose"). After that, it is
// also allowed while the claim is in an open state, so the text can be amended.
func Transition(
	ledger *ClaimLedger,
	claimID string,
	eventKind ClaimEventKind,
	author aiprovenance.Author,
	payload TransitionPayload,
) error {
	// Determine current state by projecting.
	events, err := ledger.Events()
	if err != nil {
		return fmt.Errorf("claimlifecycle: Transition: replay: %w", err)
	}

	view, projectErr := Project(claimID, events)

	// KindProposed is the claim-opening event; it is legal even when the
	// claim does not yet exist in the ledger.
	if eventKind == KindProposed {
		if projectErr != nil {
			// Claim not found — first event is always legal.
		} else if !contentEventStates[view.State] {
			return fmt.Errorf(
				"claimlifecycle: Transition: claim %q in terminal state %q cannot receive proposed event",
				claimID, view.State,
			)
		}
		return ledger.Append(ClaimEvent{
			Kind:     KindProposed,
			ClaimID:  claimID,
			Author:   author,
			Proposed: &ProposedPayload{Text: payload.ProposedText},
		})
	}

	// For all other events, the claim must already exist.
	if projectErr != nil {
		return fmt.Errorf("claimlifecycle: Transition: %w", projectErr)
	}

	currentState := view.State

	// Content events: check open-state guard.
	if eventKind == KindAssertionLinked || eventKind == KindEvidenceLinked {
		if !contentEventStates[currentState] {
			return fmt.Errorf(
				"claimlifecycle: Transition: claim %q in terminal state %q cannot receive content event %q",
				claimID, currentState, eventKind,
			)
		}
		ev := ClaimEvent{
			Kind:            eventKind,
			ClaimID:         claimID,
			Author:          author,
			AssertionLinked: payload.AssertionLinked,
			EvidenceLinked:  payload.EvidenceLinked,
		}
		return ledger.Append(ev)
	}

	// State-changing transitions: look up in the FSM table.
	edges, stateKnown := legalTransitions[currentState]
	if !stateKnown {
		return fmt.Errorf(
			"claimlifecycle: Transition: claim %q is in forward-terminal state %q (event %q rejected)",
			claimID, currentState, eventKind,
		)
	}
	if _, ok := edges[eventKind]; !ok {
		return fmt.Errorf(
			"claimlifecycle: Transition: illegal transition for claim %q: %q -(%q)→ (no such edge)",
			claimID, currentState, eventKind,
		)
	}

	return ledger.Append(ClaimEvent{
		Kind:    eventKind,
		ClaimID: claimID,
		Author:  author,
	})
}

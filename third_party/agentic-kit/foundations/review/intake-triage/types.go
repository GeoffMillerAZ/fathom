// Package intaketriage implements a generic human-only intake FSM extracted
// from throughline's app/acquire service. It manages the lifecycle of search
// proposals: proposed → imported | excluded(reason) | deferred, with
// reversible exclusion and a hard AI-actor guard on all transitions.
//
// # Human-only enforcement
//
// Every state transition must be initiated by a human actor (ActorKindHuman).
// An AI actor may suggest transitions (e.g. by calling Suggest), but the FSM
// rejects any Decide call from an AI actor with ErrAIActorForbidden. This
// mirrors the acquisition doctrine in throughline's doc-35: human oversight
// is mandatory for intake decisions; an AI suggesting relevance scores is
// informational only.
//
// # Reversible exclusion
//
// Excluded proposals may be restored to StateProposed via Restore. All other
// terminal states (StateImported, StateDeferred) are not reversible by this
// FSM — re-importing requires a new proposal. The reversal preserves the full
// transition history; the ExclusionReason is cleared on restore.
//
// # Append-only history
//
// Every transition is recorded in an append-only TransitionEvent slice on the
// FSM. Callers can use the history for audit trails and ledger replay.
//
// # Concurrency contract
//
// FSM is NOT safe for concurrent use. Callers must synchronise externally
// (e.g. hold a per-proposal lock) when sharing an FSM across goroutines.
// The typical pattern is one FSM per proposal, used from a single goroutine.
//
// CUE contracts live in schema.cue; this file and intaketriage.go are the Go
// counterpart and must stay in sync.
package intaketriage

import (
	"errors"
	"fmt"
	"time"
)

// ActorKind discriminates human from AI actors.
type ActorKind string

const (
	// ActorKindHuman is a human actor. Only human actors may call Decide.
	ActorKindHuman ActorKind = "human"
	// ActorKindAI is an AI actor. AI actors may only call Suggest.
	ActorKindAI ActorKind = "ai"
)

// Actor identifies the agent initiating a state transition or suggestion.
type Actor struct {
	// Kind discriminates human from AI.
	Kind ActorKind
	// ID is a stable identifier (e.g. "human:web", "ai:planner").
	ID string
}

// Validate returns an error if the Actor is not well-formed.
func (a Actor) Validate() error {
	if a.Kind != ActorKindHuman && a.Kind != ActorKindAI {
		return fmt.Errorf("intaketriage: Actor.Kind must be %q or %q, got %q", ActorKindHuman, ActorKindAI, a.Kind)
	}
	if a.ID == "" {
		return fmt.Errorf("intaketriage: Actor.ID must not be empty")
	}
	return nil
}

// ProposalState is the lifecycle state of a triage proposal.
type ProposalState string

const (
	// StateProposed is the initial state: the item is surfaced for human review.
	StateProposed ProposalState = "proposed"
	// StateImported means a human decided to import this item.
	StateImported ProposalState = "imported"
	// StateExcluded means a human decided this item is not relevant.
	// An ExclusionReason is required. Exclusions are reversible via Restore.
	StateExcluded ProposalState = "excluded"
	// StateDeferred means a human decided to revisit this item later.
	StateDeferred ProposalState = "deferred"
)

// Suggestion is an AI-generated recommendation for a proposal's state.
// It is purely informational: the FSM records it but never acts on it.
type Suggestion struct {
	// ProposedState is the state the AI recommends.
	ProposedState ProposalState
	// Rationale is the AI's reasoning.
	Rationale string
	// Confidence is a 0–1 score; callers may display it to the human reviewer.
	Confidence float64
	// By is the AI actor that generated this suggestion.
	By Actor
	// At is the wall-clock time of the suggestion (injected clock).
	At time.Time
}

// TransitionEvent records one FSM state transition in the append-only history.
type TransitionEvent struct {
	// ProposalID is the ID of the proposal that transitioned.
	ProposalID string
	// From is the state before the transition.
	From ProposalState
	// To is the state after the transition.
	To ProposalState
	// By is the human actor who initiated the transition.
	By Actor
	// Reason is required when To==StateExcluded; empty otherwise.
	Reason string
	// At is the wall-clock time of the transition (injected clock).
	At time.Time
}

// ErrAIActorForbidden is returned when an AI actor attempts to call Decide.
// AI actors may only call Suggest; decisions require human actors.
var ErrAIActorForbidden = errors.New("intaketriage: AI actor may not make intake decisions; only human actors may call Decide")

// ErrExclusionReasonRequired is returned when Decide is called with
// StateExcluded but an empty reason.
var ErrExclusionReasonRequired = errors.New("intaketriage: exclusion reason must not be empty when deciding StateExcluded")

// ErrNotExcluded is returned by Restore when the proposal is not in
// StateExcluded.
var ErrNotExcluded = errors.New("intaketriage: Restore requires the proposal to be in StateExcluded")

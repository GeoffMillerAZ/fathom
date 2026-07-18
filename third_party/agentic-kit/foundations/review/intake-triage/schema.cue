// Package intaketriage defines CUE contracts for the intake-triage foundation.
// Go types in types.go and intaketriage.go are the Go counterpart; drift is a bug.
package intaketriage

// #ProposalState is the lifecycle state of a triage item.
#ProposalState: "proposed" | "imported" | "excluded" | "deferred"

// #ActorKind discriminates human from AI actors. Human-only transitions are
// enforced by the FSM; this enum makes the constraint explicit in the schema.
#ActorKind: "human" | "ai"

// #Actor identifies the agent that initiates a state transition.
#Actor: {
	Kind: #ActorKind
	// ID is a stable identifier for the actor (e.g. "human:web", "ai:planner").
	ID: string & !=""
}

// #Proposal is a triage item surfaced for human review.
#Proposal: {
	// ID uniquely identifies this proposal.
	ID: string & !=""

	// State is the current lifecycle state.
	State: #ProposalState

	// ExclusionReason is required when State == "excluded".
	ExclusionReason?: string

	// DecidedBy is the actor that made the last state transition.
	DecidedBy?: #Actor
}

// #TransitionEvent records one state transition in the append-only history.
#TransitionEvent: {
	ProposalID: string & !=""
	From:       #ProposalState
	To:         #ProposalState
	By:         #Actor
	// Reason is required when To == "excluded".
	Reason?: string
}

// Package workflowengine defines the CUE schema for WorkflowDefinition and
// related types. These constraints are evaluated with `cue vet ./...`.
//
// Retry and timeout policies are intentionally absent: the engine never
// retries actions or expires states, so the contract carries no inert fields.
// Callers own retry/timeout at the side-effect site (see the Go package doc).
package workflowengine

// #WorkflowState declares a single state.
#WorkflowState: {
	id:          string & !=""
	terminal:    bool
	description: string
}

// #WorkflowTransition declares an edge between two states fired by an event.
#WorkflowTransition: {
	from:    string & !=""
	to:      string & !=""
	on:      string & !=""
	guards:  [...string]
	actions: [...string]
}

// #WorkflowPhase groups states into a higher-level lifecycle phase.
#WorkflowPhase: {
	id:          string & !=""
	states:      [...string]
	entry_event: string
}

// #WorkflowDefinition is the declarative description of a workflow.
// id, version, initial_state, and at least one state are required.
#WorkflowDefinition: {
	id:            string & !=""
	version:       string & !=""
	initial_state: string & !=""
	states: [_, ...#WorkflowState]
	transitions: [...#WorkflowTransition]
	phases?: [...#WorkflowPhase]
	config?: {[string]: string}
}

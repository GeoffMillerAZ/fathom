// Package workflowengine provides an FSM workflow engine for agentic-kit.
//
// A WorkflowDefinition declares states and transitions; a WorkflowEngine
// validates definitions, advances instances, and replays history without
// re-running side effects. Guards and actions are registered by name so
// definitions remain JSON-serializable and reconstructable from storage.
//
// Retry and timeout are intentionally out of scope for the engine: it never
// retries a failed action and never expires a state. Callers own those
// policies at the point where the side effect happens — e.g. wrap model calls
// in llmclient.WithRetry, or drive state expiry from the caller's own clock —
// because only the caller knows which effects are safe to repeat.
//
// A WorkflowEngine is safe for concurrent use once its Definition and the
// exported Guards/Actions maps are fully populated; populating those maps is
// not synchronized and must happen before the engine is shared.
package workflowengine

import (
	"context"
	"time"
)

// GuardFunc evaluates whether a transition may fire for the given instance.
// Returning false halts the transition and surfaces ErrGuardFailed.
type GuardFunc func(ctx context.Context, inst WorkflowInstance) bool

// ActionFunc performs a side effect during a transition. Returning a non-nil
// error halts the transition and surfaces ErrActionFailed; instance state is
// not mutated by the engine in that case.
type ActionFunc func(ctx context.Context, inst WorkflowInstance) error

// WorkflowState declares a single state in a workflow definition.
type WorkflowState struct {
	ID          string `json:"id"`
	Terminal    bool   `json:"terminal"`
	Description string `json:"description"`
}

// WorkflowTransition declares an edge between two states fired by an event.
type WorkflowTransition struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	On      string   `json:"on"`
	Guards  []string `json:"guards"`
	Actions []string `json:"actions"`
}

// WorkflowPhase groups states into a higher-level lifecycle phase.
type WorkflowPhase struct {
	ID         string   `json:"id"`
	States     []string `json:"states"`
	EntryEvent string   `json:"entry_event"`
}

// GuardEval captures the outcome of a single guard evaluation.
type GuardEval struct {
	Guard  string `json:"guard"`
	Result bool   `json:"result"`
}

// TransitionRecord records a single advance step on a WorkflowInstance.
type TransitionRecord struct {
	From         string      `json:"from"`
	To           string      `json:"to"`
	Event        string      `json:"event"`
	GuardsEval   []GuardEval `json:"guards_eval"`
	ActionsFired []string    `json:"actions_fired"`
	Timestamp    string      `json:"timestamp"`
}

// WorkflowDefinition is the declarative description of a workflow.
type WorkflowDefinition struct {
	ID           string               `json:"id"`
	Version      string               `json:"version"`
	InitialState string               `json:"initial_state"`
	States       []WorkflowState      `json:"states"`
	Transitions  []WorkflowTransition `json:"transitions"`
	Phases       []WorkflowPhase      `json:"phases,omitempty"`
	Config       map[string]string    `json:"config,omitempty"`
}

// WorkflowInstance is a live or historical run of a WorkflowDefinition.
type WorkflowInstance struct {
	InstanceID   string             `json:"instance_id"`
	WorkflowID   string             `json:"workflow_id"`
	CurrentState string             `json:"current_state"`
	History      []TransitionRecord `json:"history"`
	Context      map[string]string  `json:"context"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
}

// WorkflowEngine orchestrates state-machine progression for instances of a
// single WorkflowDefinition. Guards and Actions are dispatched by name.
type WorkflowEngine struct {
	Definition WorkflowDefinition
	Guards     map[string]GuardFunc
	Actions    map[string]ActionFunc
	// Now optionally overrides time.Now for deterministic tests.
	Now func() time.Time
}

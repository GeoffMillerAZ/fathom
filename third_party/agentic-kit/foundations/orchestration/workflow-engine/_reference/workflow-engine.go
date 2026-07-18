// ANNOTATED COPY – foundations/orchestration/workflow-engine/engine.go
// 337 lines, 4 branches. The DAG state-machine executor used by the workflow-cu

/*
WHY IT'S COMPLEX

A state machine engine has to enforce several invariants simultaneously: no t
from a terminal state, all guard and action names must resolve to registered
instances must be cloned (not mutated) on advance, and history replay must be
verifiable without re-running side effects. Each of those is a separate corre
and they interact: a failed guard lookup during audit replay, a missing guard
immutability, a mutated instance breaks concurrent callers.

WHAT IT DOES

WorkflowEngine is a pure Go state machine. It takes a WorkflowDefinition (decl
states, transitions, phases, retry policy, timeouts) and a set of registered
and ActionFuncs. It then provides:

   1. NewEngine(def) – constructs the engine and validates the definition struct
      Structural validation catches: missing IDs, duplicate states, transitions
      terminal states, unknown state references in timeouts and phases.

   2. Validate() – extends structural validation to confirm every guard and actio
      referenced in a transition is actually registered. Called before first use.

   3. Advance(ctx, inst, event) – the hot path.
      – Finds the transition matching (inst.CurrentState, event) – first match wi
      – Runs all guards in order; if any returns false, halts with ErrGuardFaile
      – Clones the instance (immutable input contract).
      – Runs all actions in order against the clone; if any errors, halts with E
      – Appends a TransitionRecord to clone.History with guard evaluations and fi
      – Returns the new instance. Input is never mutated.

   4. ReplayHistory(records) – structural replay only. Verifies that each Transi
      chains from/to correctly against the definition without re-running guards o
      Used for audit and recovery scenarios.

KEY DESIGN DECISIONS

   – GuardFunc and ActionFunc are registered by name (string), not embedded in t
     definition struct. This keeps the definition JSON-serializable and lets the
     engine be reconstructed from storage without re-embedding closures.

   – The engine uses a Now func() time.Time field (defaulting to time.Now) so te
     can inject deterministic timestamps. This is why TransitionRecord.Timestamp
     is deterministic in tests without any mocking framework.

   – cloneInstance makes a shallow-deep copy: the outer struct is value-copied,
     but History and Context slices/maps get new backing arrays. This prevents
     the next Advance call from seeing mutations from a previous one.

   – findTransition returns the first matching (From, On) pair. The definition ca
     have multiple transitions from the same state on different events, but not
     two on the same event – that would be ambiguous and the structural validato
     does not currently catch duplicates (a known limitation).

WHAT THE BRIDGE ADDS
──────────────────────────────────────────────────────────────────
WorkflowRunnerBridge (in agentic-cue-runtime/bridges.go) wraps this engine and
Advance on each RunStage transition, mapping stage constants to event strings.
This is how a governed LLM run loop drives a workflow without the workflow kno
anything about LLMs.
*/

package workflowengine

import (
	"context"
	"fmt"
	"time"
)

// GuardFunc evaluates whether a transition may fire for the given instance.
// Returning false halts the transition and surfaces ErrGuardFailed.
type GuardFunc func(ctx context.Context, inst WorkflowInstance) bool

// ActionFunc performs a side effect during a transition. Returning a non-nil
// error halts the transition and surfaces ErrActionFailed; the instance stat
// is not mutated by the engine in that case.
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

// BackoffStrategy enumerates retry backoff variants.
type BackoffStrategy string

const (
	BackoffNone        BackoffStrategy = "none"
	BackoffLinear      BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
)

// RetryPolicy controls how transient failures during actions are retried.
type RetryPolicy struct {
	MaxAttempts    int             `json:"max_attempts"`
	Backoff        BackoffStrategy `json:"backoff"`
	BackoffBaseSec int             `json:"backoff_base_sec"`
	OnExhaustion   string          `json:"on_exhaustion"`
}

// TimeoutPolicy bounds how long a state may remain current.
type TimeoutPolicy struct {
	StateID    string `json:"state_id"`
	TimeoutSec int    `json:"timeout_sec"`
	OnTimeout  string `json:"on_timeout"`
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
	Retry        *RetryPolicy         `json:"retry,omitempty"`
	Timeouts     []TimeoutPolicy      `json:"timeouts,omitempty"`
	Config       map[string]string    `json:"config,omitempty"`
}

// WorkflowInstance is a live or historical run of a WorkflowDefinition.
type WorkflowInstance struct {
	InstanceID   string            `json:"instance_id"`
	WorkflowID   string            `json:"workflow_id"`
	CurrentState string            `json:"current_state"`
	History      []TransitionRecord `json:"history"`
	Context      map[string]string  `json:"context"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

// WorkflowEngine orchestrates state-machine progression for instances of a
// single WorkflowDefinition. Guards and Actions are dispatched by name,
// looking up registered functions.
type WorkflowEngine struct {
	Definition WorkflowDefinition
	Guards     map[string]GuardFunc
	Actions    map[string]ActionFunc
	// Now optionally overrides time.Now for deterministic tests.
	Now func() time.Time
}

// NewEngine constructs a WorkflowEngine from a definition and validates the
// definition structurally. Guards and Actions maps are initialised empty;
// register entries with the Guards / Actions fields or via a Registry.
func NewEngine(def WorkflowDefinition) (*WorkflowEngine, error) {
	e := &WorkflowEngine{
		Definition: def,
		Guards:     make(map[string]GuardFunc),
		Actions:    make(map[string]ActionFunc),
	}
	if err := validateDefinitionStructure(def); err != nil {
		return nil, err
	}
	return e, nil
}

// Validate checks that the engine's definition is internally consistent and
// that every guard and action referenced by a transition has a registered
// implementation. It returns ErrInvalidDefinition with wrapped detail when
// any check fails.
func (e *WorkflowEngine) Validate() error {
	if err := validateDefinitionStructure(e.Definition); err != nil {
		return err
	}
	for _, tr := range e.Definition.Transitions {
		for _, g := range tr.Guards {
			if _, ok := e.Guards[g]; !ok {
				return fmt.Errorf("%w: guard %q referenced by transition %s->%s on %s",
					ErrInvalidDefinition, g, tr.From, tr.To, tr.On)
			}
		}
		for _, a := range tr.Actions {
			if _, ok := e.Actions[a]; !ok {
				return fmt.Errorf("%w: action %q referenced by transition %s->%s on %s",
					ErrInvalidDefinition, a, tr.From, tr.To, tr.On)
			}
		}
	}
	return nil
}

// Advance attempts to fire the transition matching (inst.CurrentState, event)
// On success it returns a new WorkflowInstance with the updated current state
// an appended TransitionRecord, and refreshed UpdatedAt. On failure it retur
// nil and one of ErrNoTransition, ErrGuardFailed, ErrActionFailed wrapped wi
// detail. The input instance is not mutated.
func (e *WorkflowEngine) Advance(ctx context.Context, inst WorkflowInstance, event string) (*WorkflowInstance, error) {
	tr, ok := e.findTransition(inst.CurrentState, event)
	if !ok {
		return nil, fmt.Errorf("%w: from state %q on event %q", ErrNoTransition, inst.CurrentState, event)
	}

	guardEvals := make([]GuardEval, 0, len(tr.Guards))
	for _, name := range tr.Guards {
		fn, exists := e.Guards[name]
		if !exists {
			return nil, fmt.Errorf("%w: guard %q not registered", ErrInvalidDefinition, name)
		}
		result := fn(ctx, inst)
		guardEvals = append(guardEvals, GuardEval{Guard: name, Result: result})
		if !result {
			return nil, fmt.Errorf("%w: guard %q rejected transition %s->%s on %s",
				ErrGuardFailed, name, tr.From, tr.To, tr.On)
		}
	}

	next := cloneInstance(inst)

	fired := make([]string, 0, len(tr.Actions))
	for _, name := range tr.Actions {
		fn, exists := e.Actions[name]
		if !exists {
			return nil, fmt.Errorf("%w: action %q not registered", ErrInvalidDefinition, name)
		}
	// [scan gap: lines 254-262 not captured]
		Event:        tr.On,
		GuardsEval:   guardEvals,
		ActionsFired: fired,
		Timestamp:    ts,
	})
	next.UpdatedAt = ts
	return &next, nil
}

// ReplayHistory verifies that the supplied transition records form a sequence
// reachable in the current definition starting at InitialState. It does not
// re-run guards or actions; it checks structural validity only. Returns nil
// when every record corresponds to a declared transition and the chained
// from/to states form a valid path.
func (e *WorkflowEngine) ReplayHistory(records []TransitionRecord) error {
	if err := validateDefinitionStructure(e.Definition); err != nil {
		return err
	}
	cur := e.Definition.InitialState
	for i, rec := range records {
		if rec.From != cur {
			return fmt.Errorf("%w: record %d expected from=%q, got %q",
				ErrInvalidDefinition, i, cur, rec.From)
		}
		tr, ok := e.findTransition(rec.From, rec.Event)
		if !ok {
			return fmt.Errorf("%w: record %d no transition from %q on %q",
				ErrInvalidDefinition, i, rec.From, rec.Event)
		}
		if tr.To != rec.To {
			return fmt.Errorf("%w: record %d expected to=%q, got %q",
				ErrInvalidDefinition, i, tr.To, rec.To)
		}
		cur = rec.To
	}
	return nil
}

func (e *WorkflowEngine) findTransition(from, event string) (WorkflowTransition, bool) {
	for _, tr := range e.Definition.Transitions {
		if tr.From == from && tr.On == event {
			return tr, true
		}
	}
	return WorkflowTransition{}, false
}

func (e *WorkflowEngine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func validateDefinitionStructure(def WorkflowDefinition) error {
	if def.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidDefinition)
	}
	if def.Version == "" {
		return fmt.Errorf("%w: version is required", ErrInvalidDefinition)
	}
	if def.InitialState == "" {
		return fmt.Errorf("%w: initial_state is required", ErrInvalidDefinition)
	}
	if len(def.States) == 0 {
		return fmt.Errorf("%w: at least one state is required", ErrInvalidDefinition)
	}

	stateByID := make(map[string]WorkflowState, len(def.States))
	for _, s := range def.States {
		if s.ID == "" {
			return fmt.Errorf("%w: state with empty id", ErrInvalidDefinition)
		}
		if _, dup := stateByID[s.ID]; dup {
			return fmt.Errorf("%w: duplicate state id %q", ErrInvalidDefinition, s.ID)
		}
		stateByID[s.ID] = s
	}
	if _, ok := stateByID[def.InitialState]; !ok {
		return fmt.Errorf("%w: initial_state %q not declared in states",
			ErrInvalidDefinition, def.InitialState)
	}

	for i, tr := range def.Transitions {
		if tr.From == "" || tr.To == "" || tr.On == "" {
			return fmt.Errorf("%w: transition %d missing from/to/on", ErrInvalidDefinition, i)
		}
		fromState, ok := stateByID[tr.From]
		if !ok {
			return fmt.Errorf("%w: transition %d from=%q not declared", ErrInvalidDefinition, i, tr.From)
		}
		if _, ok := stateByID[tr.To]; !ok {
			return fmt.Errorf("%w: transition %d to=%q not declared", ErrInvalidDefinition, i, tr.To)
		}
		if fromState.Terminal {
			return fmt.Errorf("%w: transition %d originates from terminal state %q",
				ErrInvalidDefinition, i, tr.From)
		}
	}

	for i, ph := range def.Phases {
		if ph.ID == "" {
			return fmt.Errorf("%w: phase %d missing id", ErrInvalidDefinition, i)
		}
		for _, sid := range ph.States {
			if _, ok := stateByID[sid]; !ok {
				return fmt.Errorf("%w: phase %q references unknown state %q",
					ErrInvalidDefinition, ph.ID, sid)
			}
		}
	}

	for i, to := range def.Timeouts {
		if _, ok := stateByID[to.StateID]; !ok {
			return fmt.Errorf("%w: timeout %d references unknown state %q",
				ErrInvalidDefinition, i, to.StateID)
		}
	}
	return nil
}

func cloneInstance(in WorkflowInstance) WorkflowInstance {
	out := in
	if in.History != nil {
		out.History = make([]TransitionRecord, len(in.History))
		copy(out.History, in.History)
	}
	if in.Context != nil {
		out.Context = make(map[string]string, len(in.Context))
		for k, v := range in.Context {
			out.Context[k] = v
		}
	}
	return out
}

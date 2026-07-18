package workflowengine

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors returned by the engine. Callers use errors.Is for matching.
var (
	// ErrInvalidDefinition is returned when a WorkflowDefinition fails structural
	// or registration validation.
	ErrInvalidDefinition = errors.New("invalid workflow definition")

	// ErrNoTransition is returned when no transition matches (currentState, event).
	ErrNoTransition = errors.New("no matching transition")

	// ErrGuardFailed is returned when a guard rejects the transition.
	ErrGuardFailed = errors.New("guard failed")

	// ErrActionFailed is returned when an action returns a non-nil error.
	ErrActionFailed = errors.New("action failed")
)

// NewEngine constructs a WorkflowEngine from a definition and validates the
// definition structurally. Guards and Actions maps are initialised empty.
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
// implementation.
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

// Advance attempts to fire the transition matching (inst.CurrentState, event).
// On success it returns a new WorkflowInstance with the updated current state,
// an appended TransitionRecord, and refreshed UpdatedAt. The input instance is
// not mutated.
func (e *WorkflowEngine) Advance(ctx context.Context, inst WorkflowInstance, event string) (*WorkflowInstance, error) {
	tr, ok := e.findTransition(inst.CurrentState, event)
	if !ok {
		return nil, fmt.Errorf("%w: from state %q on event %q", ErrNoTransition, inst.CurrentState, event)
	}

	// Evaluate all guards before cloning the instance so the input is never touched.
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

	// Clone before running actions so partial action failures do not mutate the
	// caller's instance.
	next := cloneInstance(inst)

	fired := make([]string, 0, len(tr.Actions))
	for _, name := range tr.Actions {
		fn, exists := e.Actions[name]
		if !exists {
			return nil, fmt.Errorf("%w: action %q not registered", ErrInvalidDefinition, name)
		}
		if err := fn(ctx, next); err != nil {
			return nil, fmt.Errorf("%w: action %q: %v", ErrActionFailed, name, err)
		}
		fired = append(fired, name)
	}

	ts := e.now().UTC().Format(time.RFC3339)
	next.CurrentState = tr.To
	next.History = append(next.History, TransitionRecord{
		From:         tr.From,
		To:           tr.To,
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
// re-run guards or actions; it checks structural validity only.
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

// Step is an alias for Advance that matches the interface described in the
// design spec (Engine.Step(instance, event)).
func (e *WorkflowEngine) Step(inst WorkflowInstance, event string) (*WorkflowInstance, error) {
	return e.Advance(context.Background(), inst, event)
}

// NewInstance creates a fresh WorkflowInstance for this engine's definition.
func (e *WorkflowEngine) NewInstance(instanceID string) WorkflowInstance {
	ts := e.now().UTC().Format(time.RFC3339)
	return WorkflowInstance{
		InstanceID:   instanceID,
		WorkflowID:   e.Definition.ID,
		CurrentState: e.Definition.InitialState,
		History:      nil,
		Context:      make(map[string]string),
		CreatedAt:    ts,
		UpdatedAt:    ts,
	}
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

// validateDefinitionStructure performs structural validation of a
// WorkflowDefinition: required fields, unique state IDs, valid transition
// references, no outgoing edges from terminal states, and phase state
// references.
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

	return nil
}

// cloneInstance makes a shallow-deep copy: outer struct is value-copied,
// History and Context get new backing arrays.
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

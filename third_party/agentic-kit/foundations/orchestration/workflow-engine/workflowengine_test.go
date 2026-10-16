package workflowengine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	we "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/orchestration/workflow-engine"
)

// ---- helpers ---------------------------------------------------------------

// simpleDef returns a minimal three-state workflow for testing.
//
//	pending → (start) → running → (finish) → done[terminal]
//	running → (fail)  → failed[terminal]
func simpleDef() we.WorkflowDefinition {
	return we.WorkflowDefinition{
		ID:           "simple",
		Version:      "1.0.0",
		InitialState: "pending",
		States: []we.WorkflowState{
			{ID: "pending", Terminal: false, Description: "Pending"},
			{ID: "running", Terminal: false, Description: "Running"},
			{ID: "done", Terminal: true, Description: "Done"},
			{ID: "failed", Terminal: true, Description: "Failed"},
		},
		Transitions: []we.WorkflowTransition{
			{From: "pending", To: "running", On: "start"},
			{From: "running", To: "done", On: "finish"},
			{From: "running", To: "failed", On: "fail"},
		},
	}
}

func newEngine(t *testing.T) *we.WorkflowEngine {
	t.Helper()
	e, err := we.NewEngine(simpleDef())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	// Inject deterministic clock.
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	e.Now = func() time.Time { return epoch }
	return e
}

// ---- NewEngine validation --------------------------------------------------

func TestNewEngine_InvalidDefinition(t *testing.T) {
	cases := []struct {
		name string
		def  we.WorkflowDefinition
	}{
		{
			name: "empty_id",
			def: we.WorkflowDefinition{
				ID: "", Version: "1.0", InitialState: "s",
				States: []we.WorkflowState{{ID: "s"}},
			},
		},
		{
			name: "empty_version",
			def: we.WorkflowDefinition{
				ID: "x", Version: "", InitialState: "s",
				States: []we.WorkflowState{{ID: "s"}},
			},
		},
		{
			name: "missing_initial_state",
			def: we.WorkflowDefinition{
				ID: "x", Version: "1.0", InitialState: "missing",
				States: []we.WorkflowState{{ID: "s"}},
			},
		},
		{
			name: "no_states",
			def: we.WorkflowDefinition{
				ID: "x", Version: "1.0", InitialState: "s",
				States: nil,
			},
		},
		{
			name: "duplicate_state",
			def: we.WorkflowDefinition{
				ID: "x", Version: "1.0", InitialState: "s",
				States: []we.WorkflowState{{ID: "s"}, {ID: "s"}},
			},
		},
		{
			name: "transition_from_terminal",
			def: we.WorkflowDefinition{
				ID: "x", Version: "1.0", InitialState: "s",
				States: []we.WorkflowState{{ID: "s"}, {ID: "t", Terminal: true}},
				Transitions: []we.WorkflowTransition{
					{From: "t", To: "s", On: "back"},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := we.NewEngine(tc.def)
			if !errors.Is(err, we.ErrInvalidDefinition) {
				t.Errorf("expected ErrInvalidDefinition, got %v", err)
			}
		})
	}
}

// ---- Validate with unregistered guards/actions ----------------------------

func TestValidate_UnregisteredGuard(t *testing.T) {
	def := simpleDef()
	def.Transitions[0].Guards = []string{"unregistered-guard"}
	e, err := we.NewEngine(def)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	err = e.Validate()
	if !errors.Is(err, we.ErrInvalidDefinition) {
		t.Errorf("expected ErrInvalidDefinition for unregistered guard, got %v", err)
	}
}

func TestValidate_UnregisteredAction(t *testing.T) {
	def := simpleDef()
	def.Transitions[0].Actions = []string{"unregistered-action"}
	e, err := we.NewEngine(def)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	err = e.Validate()
	if !errors.Is(err, we.ErrInvalidDefinition) {
		t.Errorf("expected ErrInvalidDefinition for unregistered action, got %v", err)
	}
}

func TestValidate_RegisteredGuardAndAction(t *testing.T) {
	def := simpleDef()
	def.Transitions[0].Guards = []string{"canStart"}
	def.Transitions[0].Actions = []string{"logStart"}
	e, err := we.NewEngine(def)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	e.Guards["canStart"] = func(_ context.Context, _ we.WorkflowInstance) bool { return true }
	e.Actions["logStart"] = func(_ context.Context, _ we.WorkflowInstance) error { return nil }

	if err := e.Validate(); err != nil {
		t.Errorf("unexpected Validate error: %v", err)
	}
}

// ---- Advance: happy path ---------------------------------------------------

func TestAdvance_HappyPath(t *testing.T) {
	e := newEngine(t)
	inst := e.NewInstance("inst-1")

	next, err := e.Advance(context.Background(), inst, "start")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if next.CurrentState != "running" {
		t.Errorf("expected running, got %q", next.CurrentState)
	}
	if len(next.History) != 1 {
		t.Errorf("expected 1 history record, got %d", len(next.History))
	}
	rec := next.History[0]
	if rec.From != "pending" || rec.To != "running" || rec.Event != "start" {
		t.Errorf("unexpected record: %+v", rec)
	}
}

func TestAdvance_ChainedTransitions(t *testing.T) {
	e := newEngine(t)
	inst := e.NewInstance("chain-1")

	inst2, err := e.Advance(context.Background(), inst, "start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	inst3, err := e.Advance(context.Background(), *inst2, "finish")
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if inst3.CurrentState != "done" {
		t.Errorf("expected done, got %q", inst3.CurrentState)
	}
	if len(inst3.History) != 2 {
		t.Errorf("expected 2 history records, got %d", len(inst3.History))
	}
}

// ---- Advance: input not mutated -------------------------------------------

func TestAdvance_InputNotMutated(t *testing.T) {
	e := newEngine(t)
	orig := e.NewInstance("immut-1")
	origState := orig.CurrentState
	origHistLen := len(orig.History)

	_, err := e.Advance(context.Background(), orig, "start")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if orig.CurrentState != origState {
		t.Errorf("input instance mutated: CurrentState changed from %q to %q", origState, orig.CurrentState)
	}
	if len(orig.History) != origHistLen {
		t.Errorf("input instance mutated: History length changed")
	}
}

// ---- Advance: no matching transition --------------------------------------

func TestAdvance_NoTransition(t *testing.T) {
	e := newEngine(t)
	inst := e.NewInstance("notrans-1")

	_, err := e.Advance(context.Background(), inst, "finish")
	if !errors.Is(err, we.ErrNoTransition) {
		t.Errorf("expected ErrNoTransition, got %v", err)
	}
}

// ---- Advance: guard failure -----------------------------------------------

func TestAdvance_GuardFailed(t *testing.T) {
	def := simpleDef()
	def.Transitions[0].Guards = []string{"denyAll"}
	e, err := we.NewEngine(def)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	e.Guards["denyAll"] = func(_ context.Context, _ we.WorkflowInstance) bool { return false }

	inst := e.NewInstance("guard-1")
	_, gerr := e.Advance(context.Background(), inst, "start")
	if !errors.Is(gerr, we.ErrGuardFailed) {
		t.Errorf("expected ErrGuardFailed, got %v", gerr)
	}
	// Original instance must remain unchanged.
	if inst.CurrentState != "pending" {
		t.Errorf("instance mutated after guard failure")
	}
}

// ---- Advance: action failure ----------------------------------------------

func TestAdvance_ActionFailed(t *testing.T) {
	def := simpleDef()
	def.Transitions[0].Actions = []string{"boom"}
	e, err := we.NewEngine(def)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	e.Actions["boom"] = func(_ context.Context, _ we.WorkflowInstance) error {
		return errors.New("boom")
	}

	inst := e.NewInstance("action-1")
	_, aerr := e.Advance(context.Background(), inst, "start")
	if !errors.Is(aerr, we.ErrActionFailed) {
		t.Errorf("expected ErrActionFailed, got %v", aerr)
	}
}

// ---- Step alias -----------------------------------------------------------

func TestStep_Alias(t *testing.T) {
	e := newEngine(t)
	inst := e.NewInstance("step-1")

	next, err := e.Step(inst, "start")
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if next.CurrentState != "running" {
		t.Errorf("expected running, got %q", next.CurrentState)
	}
}

// ---- ReplayHistory --------------------------------------------------------

func TestReplayHistory_Valid(t *testing.T) {
	e := newEngine(t)
	inst := e.NewInstance("replay-1")

	inst2, _ := e.Advance(context.Background(), inst, "start")
	inst3, _ := e.Advance(context.Background(), *inst2, "finish")

	if err := e.ReplayHistory(inst3.History); err != nil {
		t.Errorf("ReplayHistory unexpected error: %v", err)
	}
}

func TestReplayHistory_InvalidChain(t *testing.T) {
	e := newEngine(t)
	records := []we.TransitionRecord{
		{From: "pending", To: "running", Event: "start", Timestamp: "2024-01-01T00:00:00Z"},
		{From: "pending", To: "done", Event: "finish", Timestamp: "2024-01-01T00:00:01Z"}, // wrong From
	}
	err := e.ReplayHistory(records)
	if !errors.Is(err, we.ErrInvalidDefinition) {
		t.Errorf("expected ErrInvalidDefinition, got %v", err)
	}
}

// ---- Deterministic timestamp ----------------------------------------------

func TestAdvance_DeterministicTimestamp(t *testing.T) {
	e := newEngine(t)
	inst := e.NewInstance("ts-1")

	next1, _ := e.Advance(context.Background(), inst, "start")
	next2, _ := e.Advance(context.Background(), inst, "start")

	if next1.UpdatedAt != next2.UpdatedAt {
		t.Errorf("timestamps differ: %q vs %q", next1.UpdatedAt, next2.UpdatedAt)
	}
}

// ---- Terminal state blocks advance ----------------------------------------

func TestAdvance_TerminalStateRejected(t *testing.T) {
	e := newEngine(t)
	inst := e.NewInstance("term-1")
	inst.CurrentState = "done"

	_, err := e.Advance(context.Background(), inst, "finish")
	if !errors.Is(err, we.ErrNoTransition) {
		t.Errorf("expected ErrNoTransition from terminal state, got %v", err)
	}
}

// ---- Guard evaluations recorded in history --------------------------------

func TestAdvance_GuardEvalsInHistory(t *testing.T) {
	def := simpleDef()
	def.Transitions[0].Guards = []string{"allowStart"}
	e, err := we.NewEngine(def)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	e.Guards["allowStart"] = func(_ context.Context, _ we.WorkflowInstance) bool { return true }
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	e.Now = func() time.Time { return epoch }

	inst := e.NewInstance("guard-eval-1")
	next, err := e.Advance(context.Background(), inst, "start")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	rec := next.History[0]
	if len(rec.GuardsEval) != 1 {
		t.Fatalf("expected 1 guard eval, got %d", len(rec.GuardsEval))
	}
	if rec.GuardsEval[0].Guard != "allowStart" || !rec.GuardsEval[0].Result {
		t.Errorf("unexpected guard eval: %+v", rec.GuardsEval[0])
	}
}

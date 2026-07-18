package commandcenter_test

import (
	"testing"
	"time"

	workflowengine "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/workflow-engine"
	telem "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
	commandcenter "github.com/geoffmilleraz/agentic-kit/kits/command-center"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// twoStateWorkflow returns a simple workflow with two states and one transition.
//   - initial: "pending"
//   - terminal: "done"
//   - transition: pending →(approve)→ done
func twoStateWorkflow() workflowengine.WorkflowDefinition {
	return workflowengine.WorkflowDefinition{
		ID:           "simple-wf",
		Version:      "1.0",
		InitialState: "pending",
		States: []workflowengine.WorkflowState{
			{ID: "pending", Terminal: false, Description: "awaiting approval"},
			{ID: "done", Terminal: true, Description: "approved and complete"},
		},
		Transitions: []workflowengine.WorkflowTransition{
			{From: "pending", To: "done", On: "approve"},
		},
	}
}

// threeStateWorkflow returns a workflow with three states and two transitions.
//   - initial → active →(start)→ running →(finish)→ complete (terminal)
func threeStateWorkflow() workflowengine.WorkflowDefinition {
	return workflowengine.WorkflowDefinition{
		ID:           "three-state-wf",
		Version:      "1.0",
		InitialState: "active",
		States: []workflowengine.WorkflowState{
			{ID: "active", Terminal: false},
			{ID: "running", Terminal: false},
			{ID: "complete", Terminal: true},
		},
		Transitions: []workflowengine.WorkflowTransition{
			{From: "active", To: "running", On: "start"},
			{From: "running", To: "complete", On: "finish"},
		},
	}
}

// newCC constructs a CommandCenter backed by a MemorySink and fails on error.
func newCC(t *testing.T, s sink.Sink) *commandcenter.CommandCenter {
	t.Helper()
	cc, err := commandcenter.New(s)
	if err != nil {
		t.Fatalf("commandcenter.New: %v", err)
	}
	return cc
}

// eventsOfType returns all events from ms whose "event_type" attribute matches.
func eventsOfType(events []telem.Event, eventType string) []telem.Event {
	var out []telem.Event
	for _, e := range events {
		if e.Attributes["event_type"] == eventType {
			out = append(out, e)
		}
	}
	return out
}

// ── Test 1: New rejects nil sink ──────────────────────────────────────────────

func TestNew_RejectsNilSink(t *testing.T) {
	t.Parallel()
	_, err := commandcenter.New(nil)
	if err == nil {
		t.Fatal("expected error for nil sink; got nil")
	}
}

// ── Test 2: Submit → instance is reachable ────────────────────────────────────

func TestSubmit_InstanceReachable(t *testing.T) {
	t.Parallel()
	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	id, err := cc.Submit(commandcenter.WorkflowSpec{
		Definition: twoStateWorkflow(),
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	inst, ok := cc.Instance(id)
	if !ok {
		t.Fatalf("Instance(%q) not found after Submit", id)
	}
	if inst.CurrentState != "pending" {
		t.Errorf("initial state = %q; want %q", inst.CurrentState, "pending")
	}
}

// ── Test 3: Submit emits a workflow_submitted telemetry event ─────────────────

func TestSubmit_EmitsTelemetryEvent(t *testing.T) {
	t.Parallel()
	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	id, err := cc.Submit(commandcenter.WorkflowSpec{Definition: twoStateWorkflow()})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	events := ms.Events()
	submitted := eventsOfType(events, "workflow_submitted")
	if len(submitted) != 1 {
		t.Fatalf("expected 1 workflow_submitted event; got %d", len(submitted))
	}
	if submitted[0].Attributes["instance_id"] != id {
		t.Errorf("event instance_id = %q; want %q", submitted[0].Attributes["instance_id"], id)
	}
}

// ── Test 4: Advance transitions state ─────────────────────────────────────────

func TestAdvance_TransitionsState(t *testing.T) {
	t.Parallel()
	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	id, _ := cc.Submit(commandcenter.WorkflowSpec{Definition: twoStateWorkflow()})

	if err := cc.Advance(id, "approve"); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	inst, _ := cc.Instance(id)
	if inst.CurrentState != "done" {
		t.Errorf("state after advance = %q; want %q", inst.CurrentState, "done")
	}
}

// ── Test 5: Advance emits a workflow_advanced telemetry event ─────────────────

func TestAdvance_EmitsTelemetryEvent(t *testing.T) {
	t.Parallel()
	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	id, _ := cc.Submit(commandcenter.WorkflowSpec{Definition: twoStateWorkflow()})
	ms.Reset() // clear the submit event

	if err := cc.Advance(id, "approve"); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	events := ms.Events()
	advanced := eventsOfType(events, "workflow_advanced")
	if len(advanced) != 1 {
		t.Fatalf("expected 1 workflow_advanced event; got %d", len(advanced))
	}
	if advanced[0].Attributes["instance_id"] != id {
		t.Errorf("event instance_id = %q; want %q", advanced[0].Attributes["instance_id"], id)
	}
	if advanced[0].Attributes["event"] != "approve" {
		t.Errorf("event.event = %q; want %q", advanced[0].Attributes["event"], "approve")
	}
}

// ── Test 6: Primary end-to-end: Submit → Advance → event-emitted ──────────────
// This is the canonical kit test: a workflow is submitted, advanced, and we
// confirm both the state transition and the telemetry event were produced.

func TestSubmitAdvanceEventEmitted(t *testing.T) {
	t.Parallel()

	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	// Step 1 — Submit.
	id, err := cc.Submit(commandcenter.WorkflowSpec{
		Definition: twoStateWorkflow(),
		Context:    map[string]string{"requested_by": "operator-1"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Step 2 — Advance.
	if err := cc.Advance(id, "approve"); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	// Step 3 — Verify event emitted.
	events := ms.Events()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 telemetry events; got %d", len(events))
	}

	submitted := eventsOfType(events, "workflow_submitted")
	advanced := eventsOfType(events, "workflow_advanced")

	if len(submitted) != 1 {
		t.Errorf("workflow_submitted events = %d; want 1", len(submitted))
	}
	if len(advanced) != 1 {
		t.Errorf("workflow_advanced events = %d; want 1", len(advanced))
	}

	// Verify final state.
	inst, ok := cc.Instance(id)
	if !ok {
		t.Fatalf("instance %q not found", id)
	}
	if inst.CurrentState != "done" {
		t.Errorf("final state = %q; want %q", inst.CurrentState, "done")
	}
}

// ── Test 7: Pending returns review items for terminal transitions ─────────────

func TestPending_ReturnsTerminalReviewItems(t *testing.T) {
	t.Parallel()
	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	id, _ := cc.Submit(commandcenter.WorkflowSpec{Definition: twoStateWorkflow()})

	// Before advancing, no review items.
	pending := cc.Pending()
	if len(pending) != 0 {
		t.Errorf("pending before advance = %d; want 0", len(pending))
	}

	// Advance to terminal state.
	if err := cc.Advance(id, "approve"); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	pending = cc.Pending()
	if len(pending) != 1 {
		t.Fatalf("pending after terminal advance = %d; want 1", len(pending))
	}
	if pending[0].InstanceID != id {
		t.Errorf("pending[0].InstanceID = %q; want %q", pending[0].InstanceID, id)
	}
}

// ── Test 8: Multiple workflows are tracked independently ─────────────────────

func TestMultipleWorkflows(t *testing.T) {
	t.Parallel()
	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	id1, err1 := cc.Submit(commandcenter.WorkflowSpec{Definition: twoStateWorkflow()})
	id2, err2 := cc.Submit(commandcenter.WorkflowSpec{Definition: threeStateWorkflow()})
	if err1 != nil || err2 != nil {
		t.Fatalf("Submit errors: %v, %v", err1, err2)
	}
	if id1 == id2 {
		t.Errorf("expected distinct instance IDs; both are %q", id1)
	}

	// Advance wf1 to terminal.
	if err := cc.Advance(id1, "approve"); err != nil {
		t.Fatalf("Advance(wf1): %v", err)
	}
	// Advance wf2 only to first intermediate state.
	if err := cc.Advance(id2, "start"); err != nil {
		t.Fatalf("Advance(wf2, start): %v", err)
	}

	inst1, _ := cc.Instance(id1)
	inst2, _ := cc.Instance(id2)

	if inst1.CurrentState != "done" {
		t.Errorf("wf1 state = %q; want %q", inst1.CurrentState, "done")
	}
	if inst2.CurrentState != "running" {
		t.Errorf("wf2 state = %q; want %q", inst2.CurrentState, "running")
	}

	// Only wf1 (terminal) should appear in Pending.
	pending := cc.Pending()
	if len(pending) != 1 {
		t.Errorf("pending = %d; want 1 (only wf1 reached terminal)", len(pending))
	}
}

// ── Test 9: Advance on unknown instance returns error ─────────────────────────

func TestAdvance_UnknownInstanceReturnsError(t *testing.T) {
	t.Parallel()
	ms := &sink.MemorySink{}
	cc := newCC(t, ms)

	err := cc.Advance("nonexistent-instance", "approve")
	if err == nil {
		t.Fatal("expected error for unknown instance; got nil")
	}
}

// ── Test 10: Determinism — same spec, same sequence → same events ─────────────

func TestDeterminism(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	run := func() []telem.Event {
		ms := &sink.MemorySink{}
		cc, _ := commandcenter.New(ms)
		_ = fixedTime // referenced for determinism intent
		id, _ := cc.Submit(commandcenter.WorkflowSpec{Definition: twoStateWorkflow()})
		_ = cc.Advance(id, "approve")
		return ms.Events()
	}

	ev1 := run()
	ev2 := run()

	if len(ev1) != len(ev2) {
		t.Errorf("non-deterministic event count: %d vs %d", len(ev1), len(ev2))
	}
	for i := range ev1 {
		if ev1[i].Kind != ev2[i].Kind {
			t.Errorf("event[%d].Kind: %q vs %q", i, ev1[i].Kind, ev2[i].Kind)
		}
		if ev1[i].Source != ev2[i].Source {
			t.Errorf("event[%d].Source: %q vs %q", i, ev1[i].Source, ev2[i].Source)
		}
		if ev1[i].Attributes["event_type"] != ev2[i].Attributes["event_type"] {
			t.Errorf("event[%d].event_type: %q vs %q", i,
				ev1[i].Attributes["event_type"], ev2[i].Attributes["event_type"])
		}
	}
}

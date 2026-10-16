package agentharness_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/sink"
	agentharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/agent-harness"
)

// countingTool returns a tool with the given declared effect whose handler
// counts its invocations — the proof of whether the gate let it run.
func countingTool(name string, eff *toolregistry.Effect, calls *int32) toolregistry.Tool {
	return toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        name,
			Description: "test tool " + name,
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		Effect: eff,
		Handler: func(context.Context, json.RawMessage) (string, error) {
			atomic.AddInt32(calls, 1)
			return name + "-result", nil
		},
	}
}

// oneToolThenDone scripts a model that calls name once and then finishes.
func oneToolThenDone(name string) *llmclient.MockClient {
	return llmclient.NewMockClient(
		toolUseEntry("tu-1", name, json.RawMessage(`{"id":"x"}`), llmclient.Usage{}),
		textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}),
	)
}

// toolResultsIn returns every tool_result block in the transcript.
func toolResultsIn(msgs []llmclient.Message) []llmclient.ContentBlock {
	var out []llmclient.ContentBlock
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == llmclient.BlockToolResult {
				out = append(out, b)
			}
		}
	}
	return out
}

func approveAll(by string) agentharness.Approver {
	return agentharness.ApproverFunc(func(_ context.Context, _ agentharness.ApprovalRequest) (agentharness.Approval, error) {
		return agentharness.Approval{Approved: true, ApprovedBy: by}, nil
	})
}

func TestEffectStrictUndeclaredIsDeniedExactlyOnce(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("alpha")
	reg := newRegistry(t, countingTool("alpha", nil, &calls)) // nil = undeclared
	ms := &sink.MemorySink{}

	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{Client: mock, Registry: reg, Sink: ms, Now: fixedClock}, // no Approver
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v (a gate denial is not an error)", err)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed — the loop continues after a denial", res.Stop)
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("handler ran %d times, want 0: an undeclared tool must not dispatch under Strict without approval", n)
	}
	denied := eventsWithStep(ms.Events(), agentharness.StepGateDenied)
	if len(denied) != 1 {
		t.Fatalf("gate_denied events = %d, want exactly 1", len(denied))
	}
	attrs := denied[0].Attributes
	if attrs[agentharness.AttrToolName] != "alpha" || attrs[agentharness.AttrEffect] != agentharness.EffectUndeclared {
		t.Errorf("gate_denied attrs = %v, want tool_name=alpha effect=undeclared", attrs)
	}
	if !strings.Contains(attrs[agentharness.AttrReason], "no Approver wired") {
		t.Errorf("gate_denied reason = %q, want it to name the missing approver", attrs[agentharness.AttrReason])
	}
	results := toolResultsIn(res.Messages)
	if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "denied by effect gate") {
		t.Errorf("tool results = %+v, want one is_error effect-gate denial", results)
	}
	if res.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1 (the denied call still produced a tool_result)", res.ToolCalls)
	}
}

func TestEffectStrictReadOnlyDispatches(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("reader")
	reg := newRegistry(t, countingTool("reader", &toolregistry.Effect{ReadOnlyHint: true}, &calls))
	ms := &sink.MemorySink{}
	approverCalls := int32(0)

	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{
			Client: mock, Registry: reg, Sink: ms, Now: fixedClock,
			Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
				atomic.AddInt32(&approverCalls, 1)
				return agentharness.Approval{}, nil
			}),
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("handler ran %d times, want 1: a read-only tool dispatches under Strict", n)
	}
	if n := atomic.LoadInt32(&approverCalls); n != 0 {
		t.Errorf("approver asked %d times, want 0 for a read-only tool", n)
	}
	if gate := eventsWithStep(ms.Events(), agentharness.StepGateDenied); len(gate) != 0 {
		t.Errorf("gate_denied events = %d, want 0", len(gate))
	}
	if results := toolResultsIn(res.Messages); len(results) != 1 || results[0].IsError {
		t.Errorf("tool results = %+v, want one non-error result", results)
	}
}

func TestEffectStrictExplicitAdditiveDispatches(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("appender")
	reg := newRegistry(t, countingTool("appender", &toolregistry.Effect{}, &calls)) // declared: additive, not destructive
	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := h.Run(context.Background(), "go"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("handler ran %d times, want 1: an explicit all-false Effect is a declaration taken at its word", n)
	}
}

func TestEffectStrictDestructiveApproved(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("deleter")
	reg := newRegistry(t, countingTool("deleter", &toolregistry.Effect{DestructiveHint: true}, &calls))
	ms := &sink.MemorySink{}
	var got agentharness.ApprovalRequest

	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{
			Client: mock, Registry: reg, Sink: ms, Now: fixedClock,
			Approver: agentharness.ApproverFunc(func(_ context.Context, req agentharness.ApprovalRequest) (agentharness.Approval, error) {
				got = req
				return agentharness.Approval{Approved: true, ApprovedBy: "reviewer-1", Reason: "ticket 42"}, nil
			}),
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("handler ran %d times, want 1 after approval", n)
	}
	if got.Call.ToolName != "deleter" || got.Call.ToolUseID != "tu-1" || string(got.Call.Input) != `{"id":"x"}` {
		t.Errorf("ApprovalRequest.Call = %+v, want the pending call", got.Call)
	}
	if got.Effect == nil || !got.Effect.DestructiveHint {
		t.Errorf("ApprovalRequest.Effect = %+v, want the declared destructive effect", got.Effect)
	}
	if !got.Preview.OK || !strings.Contains(got.Preview.Summary, `"deleter"`) || !strings.Contains(got.Preview.Summary, "destructive") || !strings.Contains(got.Preview.Summary, `{"id":"x"}`) {
		t.Errorf("Preview = %+v, want an OK card naming the tool, the effect and the input", got.Preview)
	}
	approved := eventsWithStep(ms.Events(), agentharness.StepGateApproved)
	if len(approved) != 1 {
		t.Fatalf("gate_approved events = %d, want 1", len(approved))
	}
	if a := approved[0].Attributes; a[agentharness.AttrApprovedBy] != "reviewer-1" || a[agentharness.AttrEffect] != agentharness.EffectDestructive || a[agentharness.AttrReason] != "ticket 42" {
		t.Errorf("gate_approved attrs = %v", a)
	}
	if denied := eventsWithStep(ms.Events(), agentharness.StepGateDenied); len(denied) != 0 {
		t.Errorf("gate_denied events = %d, want 0", len(denied))
	}
	if results := toolResultsIn(res.Messages); len(results) != 1 || results[0].IsError || results[0].Content != "deleter-result" {
		t.Errorf("tool results = %+v, want the real result", results)
	}
}

func TestEffectStrictDestructiveDenied(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("deleter")
	reg := newRegistry(t, countingTool("deleter", &toolregistry.Effect{DestructiveHint: true}, &calls))
	ms := &sink.MemorySink{}

	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{
			Client: mock, Registry: reg, Sink: ms, Now: fixedClock,
			Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
				return agentharness.Approval{Approved: false, Reason: "not during the freeze"}, nil
			}),
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("handler ran %d times, want 0 after a denial", n)
	}
	denied := eventsWithStep(ms.Events(), agentharness.StepGateDenied)
	if len(denied) != 1 || denied[0].Attributes[agentharness.AttrReason] != "not during the freeze" {
		t.Errorf("gate_denied events = %+v, want exactly one carrying the approver's reason", denied)
	}
	results := toolResultsIn(res.Messages)
	if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "not during the freeze") {
		t.Errorf("tool results = %+v, want the denial reason surfaced to the model", results)
	}
}

func TestEffectStrictApproverErrorIsDenial(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("deleter")
	reg := newRegistry(t, countingTool("deleter", &toolregistry.Effect{DestructiveHint: true}, &calls))
	ms := &sink.MemorySink{}

	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{
			Client: mock, Registry: reg, Sink: ms, Now: fixedClock,
			Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
				return agentharness.Approval{Approved: true}, errors.New("approval channel down")
			}),
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v (an approver failure denies the call, it does not stop the run)", err)
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("handler ran %d times, want 0: an approver error is a denial even when it says Approved", n)
	}
	if denied := eventsWithStep(ms.Events(), agentharness.StepGateDenied); len(denied) != 1 || !strings.Contains(denied[0].Attributes[agentharness.AttrReason], "approval channel down") {
		t.Errorf("gate_denied events = %+v, want one carrying the approver error", denied)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed", res.Stop)
	}
}

func TestEffectStrictUnknownToolIsNotGated(t *testing.T) {
	mock := oneToolThenDone("ghost")
	reg := newRegistry(t)
	ms := &sink.MemorySink{}
	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{Client: mock, Registry: reg, Sink: ms, Now: fixedClock},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gate := eventsWithStep(ms.Events(), agentharness.StepGateDenied); len(gate) != 0 {
		t.Errorf("gate_denied events = %d, want 0 for an unregistered tool (nothing can happen; Dispatch reports it)", len(gate))
	}
	if results := toolResultsIn(res.Messages); len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "unknown tool") {
		t.Errorf("tool results = %+v, want the registry's unknown-tool error", results)
	}
}

func TestEffectPermissiveIsUnchanged(t *testing.T) {
	for _, policy := range []agentharness.EffectPolicy{"", agentharness.EffectPermissive} {
		t.Run(string(policy), func(t *testing.T) {
			var calls int32
			mock := oneToolThenDone("alpha")
			reg := newRegistry(t, countingTool("alpha", nil, &calls))
			ms := &sink.MemorySink{}
			approverCalls := int32(0)
			h, err := agentharness.New(
				agentharness.Config{MaxTurns: 4, EffectPolicy: policy},
				agentharness.Deps{
					Client: mock, Registry: reg, Sink: ms, Now: fixedClock,
					Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
						atomic.AddInt32(&approverCalls, 1)
						return agentharness.Approval{}, nil
					}),
				},
			)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := h.Run(context.Background(), "go"); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if n := atomic.LoadInt32(&calls); n != 1 {
				t.Errorf("handler ran %d times, want 1 under permissive", n)
			}
			if n := atomic.LoadInt32(&approverCalls); n != 0 {
				t.Errorf("approver asked %d times, want 0 under permissive", n)
			}
			for _, step := range []string{agentharness.StepGateDenied, agentharness.StepGateApproved} {
				if got := eventsWithStep(ms.Events(), step); len(got) != 0 {
					t.Errorf("%s events = %d, want 0 under permissive", step, len(got))
				}
			}
		})
	}
}

func TestEffectPolicyValidation(t *testing.T) {
	mock := llmclient.NewMockClient()
	_, err := agentharness.New(agentharness.Config{MaxTurns: 1, EffectPolicy: "lenient"}, agentharness.Deps{Client: mock})
	if err == nil || !strings.Contains(err.Error(), "EffectPolicy") {
		t.Errorf("New with EffectPolicy=lenient: err = %v, want a validation error naming EffectPolicy", err)
	}
}

func TestEffectGateRunsAfterPreDispatchHook(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("deleter")
	reg := newRegistry(t, countingTool("deleter", &toolregistry.Effect{DestructiveHint: true}, &calls))
	ms := &sink.MemorySink{}
	approverCalls := int32(0)

	h, err := agentharness.New(
		agentharness.Config{
			MaxTurns:     4,
			EffectPolicy: agentharness.EffectStrict,
			Hooks: &agentharness.HookConfig{
				PreDispatch: func(context.Context, agentharness.ToolCallView) (bool, string, error) {
					return false, "hook says no", nil
				},
			},
		},
		agentharness.Deps{
			Client: mock, Registry: reg, Sink: ms, Now: fixedClock,
			Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
				atomic.AddInt32(&approverCalls, 1)
				return agentharness.Approval{Approved: true}, nil
			}),
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := atomic.LoadInt32(&approverCalls); n != 0 {
		t.Errorf("approver asked %d times, want 0: a PreDispatch denial never reaches the gate", n)
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("handler ran %d times, want 0", n)
	}
	if results := toolResultsIn(res.Messages); len(results) != 1 || !strings.Contains(results[0].Content, "denied by policy: hook says no") {
		t.Errorf("tool results = %+v, want the hook's denial", results)
	}
}

func TestEffectStrictParallelDispatchApprovesSequentially(t *testing.T) {
	var calls int32
	mock := llmclient.NewMockClient(
		llmclient.MockEntry{Response: llmclient.Response{
			Content: []llmclient.ContentBlock{
				llmclient.ToolUseBlock("tu-1", "deleter", json.RawMessage(`{"id":"1"}`)),
				llmclient.ToolUseBlock("tu-2", "deleter", json.RawMessage(`{"id":"2"}`)),
			},
			StopReason: llmclient.StopToolUse,
		}},
		textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	reg := newRegistry(t, countingTool("deleter", &toolregistry.Effect{DestructiveHint: true}, &calls))
	var order []string
	inFlight := int32(0)
	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict, ParallelTools: 2},
		agentharness.Deps{
			Client: mock, Registry: reg, Now: fixedClock,
			Approver: agentharness.ApproverFunc(func(_ context.Context, req agentharness.ApprovalRequest) (agentharness.Approval, error) {
				if atomic.AddInt32(&inFlight, 1) != 1 {
					t.Error("approvals overlapped; they must be sequential")
				}
				order = append(order, req.Call.ToolUseID)
				atomic.AddInt32(&inFlight, -1)
				return agentharness.Approval{Approved: req.Call.ToolUseID == "tu-2", Reason: "only the second"}, nil
			}),
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(order, ",") != "tu-1,tu-2" {
		t.Errorf("approval order = %v, want tu-1,tu-2 (call order)", order)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("handler ran %d times, want 1 (only the approved call)", n)
	}
	results := toolResultsIn(res.Messages)
	if len(results) != 2 || !results[0].IsError || results[1].IsError {
		t.Errorf("tool results = %+v, want [denied, dispatched] in tool_use order", results)
	}
}

func TestEffectStrictStreamEmitsGateDenied(t *testing.T) {
	var calls int32
	mock := oneToolThenDone("alpha")
	reg := newRegistry(t, countingTool("alpha", nil, &calls))
	h, err := agentharness.New(
		agentharness.Config{MaxTurns: 4, EffectPolicy: agentharness.EffectStrict},
		agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ch, err := h.RunStream(context.Background(), "go")
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	var gate []agentharness.HarnessEvent
	var final *agentharness.RunResult
	for ev := range ch {
		switch ev.Kind {
		case agentharness.HarnessEventGateDenied:
			gate = append(gate, ev)
		case agentharness.HarnessEventResult:
			final = ev.Result
		}
	}
	if len(gate) != 1 {
		t.Fatalf("gate_denied stream events = %d, want 1", len(gate))
	}
	if gate[0].ToolName != "alpha" || gate[0].ToolUseID != "tu-1" || gate[0].Effect != agentharness.EffectUndeclared || gate[0].Reason == "" {
		t.Errorf("gate_denied event = %+v", gate[0])
	}
	if final == nil || final.Stop != agentharness.StopCompleted {
		t.Errorf("final = %+v, want a completed result", final)
	}
}

func TestRequireApprovalDirect(t *testing.T) {
	reg := newRegistry(t, countingTool("deleter", &toolregistry.Effect{DestructiveHint: true}, new(int32)))
	call := agentharness.ToolCallView{Turn: 3, ToolUseID: "tu-9", ToolName: "deleter", Input: json.RawMessage(`{"id":"9"}`)}

	t.Run("no approver is a denial, not an error", func(t *testing.T) {
		h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, agentharness.Deps{Client: llmclient.NewMockClient(), Registry: reg})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		a, err := h.RequireApproval(context.Background(), call)
		if err != nil || a.Approved || !strings.Contains(a.Reason, "no Approver wired") {
			t.Errorf("RequireApproval = %+v, %v; want a reasoned denial and nil error", a, err)
		}
	})

	t.Run("empty denial reason is filled", func(t *testing.T) {
		h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, agentharness.Deps{
			Client: llmclient.NewMockClient(), Registry: reg,
			Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
				return agentharness.Approval{}, nil
			}),
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		a, err := h.RequireApproval(context.Background(), call)
		if err != nil || a.Approved || a.Reason == "" {
			t.Errorf("RequireApproval = %+v, %v; want a denial with a generic reason", a, err)
		}
	})

	t.Run("approver error is returned and denies", func(t *testing.T) {
		boom := errors.New("boom")
		h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, agentharness.Deps{
			Client: llmclient.NewMockClient(), Registry: reg,
			Approver: agentharness.ApproverFunc(func(context.Context, agentharness.ApprovalRequest) (agentharness.Approval, error) {
				return agentharness.Approval{Approved: true}, boom
			}),
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		a, err := h.RequireApproval(context.Background(), call)
		if !errors.Is(err, boom) || a.Approved {
			t.Errorf("RequireApproval = %+v, %v; want the error back and Approved=false", a, err)
		}
	})

	t.Run("the request carries the effect and a hitl preview", func(t *testing.T) {
		var got agentharness.ApprovalRequest
		h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, agentharness.Deps{
			Client: llmclient.NewMockClient(), Registry: reg,
			Approver: agentharness.ApproverFunc(func(_ context.Context, req agentharness.ApprovalRequest) (agentharness.Approval, error) {
				got = req
				return agentharness.Approval{Approved: true, ApprovedBy: "me"}, nil
			}),
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if a, err := h.RequireApproval(context.Background(), call); err != nil || !a.Approved {
			t.Fatalf("RequireApproval = %+v, %v", a, err)
		}
		if got.Call.Turn != 3 || got.Effect == nil || !got.Effect.DestructiveHint {
			t.Errorf("request = %+v, want the call and its declared effect", got)
		}
		if !got.Preview.OK || got.Preview.Err != "" || len(got.Preview.Changes) != 0 {
			t.Errorf("preview = %+v, want OK with no Changes (the harness cannot dry-run a tool)", got.Preview)
		}
	})
}

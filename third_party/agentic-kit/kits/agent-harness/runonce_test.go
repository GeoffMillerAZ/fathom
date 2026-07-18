package agentharness_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/agent/conversation"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
	agentharness "github.com/geoffmilleraz/agentic-kit/kits/agent-harness"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// newRunOnceHarness builds a Harness with no Registry (RunOnce path).
func newRunOnceHarness(t *testing.T, client llmclient.LLMClient, deps agentharness.Deps) *agentharness.Harness {
	t.Helper()
	deps.Client = client
	h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

// simpleReq is a minimal single-message request.
func simpleReq(userText string) llmclient.Request {
	return llmclient.Request{
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock(userText)}},
		},
	}
}

// ─── nil-Registry construction ────────────────────────────────────────────────

// TestNilRegistryNew verifies New succeeds with a nil Registry.
func TestNilRegistryNew(t *testing.T) {
	client := llmclient.NewMockClient()
	h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, agentharness.Deps{
		Client: client,
		// Registry intentionally nil
	})
	if err != nil {
		t.Fatalf("New with nil Registry: %v", err)
	}
	if h == nil {
		t.Fatal("New returned nil Harness")
	}
}

// TestNilRegistryRunErrors verifies that Run, RunConversation, RunFromCheckpoint,
// RunStream, RunStreamFromCheckpoint, RunBlocks, and RunStreamBlocks all return
// a clear error naming RunOnce when Registry is nil.
func TestNilRegistryRunErrors(t *testing.T) {
	client := llmclient.NewMockClient(textEntry("hi", llmclient.StopEndTurn, llmclient.Usage{}))
	h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, agentharness.Deps{Client: client})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	blocks := []llmclient.ContentBlock{llmclient.TextBlock("hi")}

	t.Run("Run", func(t *testing.T) {
		_, err := h.Run(ctx, "hi")
		assertRegistryError(t, err)
	})
	t.Run("RunConversation", func(t *testing.T) {
		conv := conversation.New(conversation.Options{})
		if err := conv.AppendUserText("hi"); err != nil {
			t.Fatalf("AppendUserText: %v", err)
		}
		_, err := h.RunConversation(ctx, conv)
		assertRegistryError(t, err)
	})
	t.Run("RunFromCheckpoint", func(t *testing.T) {
		cp := &agentharness.Checkpoint{Version: 1, ConvSnapshot: []byte(`{"version":1,"messages":[]}`)}
		_, err := h.RunFromCheckpoint(ctx, cp)
		assertRegistryError(t, err)
	})
	t.Run("RunStream", func(t *testing.T) {
		_, err := h.RunStream(ctx, "hi")
		assertStreamRegistryError(t, err)
	})
	t.Run("RunStreamFromCheckpoint", func(t *testing.T) {
		cp := &agentharness.Checkpoint{Version: 1, ConvSnapshot: []byte(`{"version":1,"messages":[]}`)}
		_, err := h.RunStreamFromCheckpoint(ctx, cp)
		assertStreamRegistryError(t, err)
	})
	t.Run("RunBlocks", func(t *testing.T) {
		_, err := h.RunBlocks(ctx, blocks)
		assertRegistryError(t, err)
	})
	t.Run("RunStreamBlocks", func(t *testing.T) {
		_, err := h.RunStreamBlocks(ctx, blocks)
		assertStreamRegistryError(t, err)
	})
}

func assertRegistryError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error for nil Registry, got nil")
	}
	if !strings.Contains(err.Error(), "RunOnce") {
		t.Errorf("error %q does not name RunOnce", err)
	}
}

func assertStreamRegistryError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error for nil Registry, got nil")
	}
	if !strings.Contains(err.Error(), "RunOnce") {
		t.Errorf("error %q does not name RunOnce", err)
	}
}

// ─── RunOnce happy path ───────────────────────────────────────────────────────

func TestRunOnceHappyPath(t *testing.T) {
	usage := llmclient.Usage{InputTokens: 20, OutputTokens: 10}
	mock := llmclient.NewMockClient(textEntry("the answer", llmclient.StopEndTurn, usage))
	ms := &sink.MemorySink{}
	clock := advancingClock(time.Millisecond)
	journal := investigationjournal.NewJournalWithClock(clock)

	h := newRunOnceHarness(t, mock, agentharness.Deps{
		Sink:    ms,
		Journal: journal,
		Now:     clock,
	})

	req := simpleReq("what is 2+2?")
	res, err := h.RunOnce(context.Background(), req)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Stop and shape.
	if res.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed", res.Stop)
	}
	if res.FinalText != "the answer" {
		t.Errorf("FinalText = %q, want %q", res.FinalText, "the answer")
	}
	if res.Turns != 1 {
		t.Errorf("Turns = %d, want 1", res.Turns)
	}
	if res.ToolCalls != 0 {
		t.Errorf("ToolCalls = %d, want 0", res.ToolCalls)
	}
	if res.Usage != usage {
		t.Errorf("Usage = %+v, want %+v", res.Usage, usage)
	}
	if res.Final == nil {
		t.Error("Final checkpoint is nil")
	}

	// Messages = the exchange: user + assistant.
	if len(res.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(res.Messages))
	}
	if res.Messages[0].Role != llmclient.RoleUser {
		t.Errorf("Messages[0].Role = %q, want user", res.Messages[0].Role)
	}
	if res.Messages[1].Role != llmclient.RoleAssistant {
		t.Errorf("Messages[1].Role = %q, want assistant", res.Messages[1].Role)
	}

	// Exactly one model call was made.
	if mock.CallCount() != 1 {
		t.Errorf("model calls = %d, want 1", mock.CallCount())
	}
	// The client received the request as-is.
	calls := mock.Calls()
	if len(calls[0].Request.Messages) != 1 {
		t.Errorf("request Messages len = %d, want 1", len(calls[0].Request.Messages))
	}

	// Telemetry: one turn event.
	events := eventsWithStep(ms.Events(), agentharness.StepTurn)
	if len(events) != 1 {
		t.Fatalf("turn events = %d, want 1", len(events))
	}
	if events[0].Attributes[agentharness.AttrTurn] != "0" {
		t.Errorf("turn attr = %q, want 0", events[0].Attributes[agentharness.AttrTurn])
	}
	if events[0].Attributes[agentharness.AttrStopReason] != "end_turn" {
		t.Errorf("stop_reason attr = %q, want end_turn", events[0].Attributes[agentharness.AttrStopReason])
	}

	// Journal: run-started, turn-completed, run-ended.
	entries := journal.Entries()
	wantSteps := []string{"run-started", "turn-completed", "run-ended"}
	if len(entries) != len(wantSteps) {
		t.Fatalf("journal entries = %d (%+v), want %d", len(entries), entries, len(wantSteps))
	}
	for i, e := range entries {
		if e.Step != wantSteps[i] {
			t.Errorf("entry %d step = %q, want %q", i, e.Step, wantSteps[i])
		}
	}
}

// TestRunOnceStopSequence verifies stop_sequence is also a completion.
func TestRunOnceStopSequence(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("stopped", llmclient.StopSequenceHit, llmclient.Usage{}))
	h := newRunOnceHarness(t, mock, agentharness.Deps{Now: fixedClock})
	res, err := h.RunOnce(context.Background(), simpleReq("go"))
	if err != nil || res.Stop != agentharness.StopCompleted {
		t.Fatalf("RunOnce = (%+v, %v), want completed/nil", res, err)
	}
}

// ─── RunOnce budget denial ────────────────────────────────────────────────────

func TestRunOnceBudgetDenial(t *testing.T) {
	clock := advancingClock(time.Millisecond)
	guard := budgetguard.New(budgetguard.Config{
		MaxReqPerMin: 0, // deny everything
		MaxReqPerDay: 100,
		MaxUSDPerDay: 100,
	}, clock)
	mock := llmclient.NewMockClient(textEntry("never", llmclient.StopEndTurn, llmclient.Usage{}))
	ms := &sink.MemorySink{}

	h := newRunOnceHarness(t, mock, agentharness.Deps{
		Budget: guard,
		Sink:   ms,
		Now:    clock,
	})

	res, err := h.RunOnce(context.Background(), simpleReq("hello"))
	if err != nil {
		t.Fatalf("RunOnce budget denial should be a stop, not an error: %v", err)
	}
	if res.Stop != agentharness.StopBudget {
		t.Errorf("Stop = %q, want budget", res.Stop)
	}
	if res.Turns != 0 {
		t.Errorf("Turns = %d, want 0 (model never called)", res.Turns)
	}
	if mock.CallCount() != 0 {
		t.Errorf("model calls = %d, want 0", mock.CallCount())
	}

	// Budget event emitted.
	budgetEvents := eventsWithStep(ms.Events(), agentharness.StepBudget)
	if len(budgetEvents) != 1 {
		t.Fatalf("budget events = %d, want 1", len(budgetEvents))
	}
}

// ─── RunOnce PreModelCall hook ────────────────────────────────────────────────

func TestRunOncePreModelCallHookFires(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("ok", llmclient.StopEndTurn, llmclient.Usage{}))
	var hookCalled bool

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 1,
		Hooks: &agentharness.HookConfig{
			PreModelCall: func(ctx context.Context, v agentharness.PreModelCallView) error {
				hookCalled = true
				return nil
			},
		},
	}, agentharness.Deps{Client: mock, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := h.RunOnce(context.Background(), simpleReq("ping"))
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed", res.Stop)
	}
	if !hookCalled {
		t.Error("PreModelCall hook was not called")
	}
}

func TestRunOncePreModelCallHookError(t *testing.T) {
	hookErr := errors.New("policy denied")
	mock := llmclient.NewMockClient(textEntry("never", llmclient.StopEndTurn, llmclient.Usage{}))

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 1,
		Hooks: &agentharness.HookConfig{
			PreModelCall: func(ctx context.Context, v agentharness.PreModelCallView) error {
				return hookErr
			},
		},
	}, agentharness.Deps{Client: mock, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := h.RunOnce(context.Background(), simpleReq("ping"))
	if err == nil {
		t.Fatal("RunOnce: error = nil, want hook error")
	}
	if !errors.Is(err, hookErr) {
		t.Errorf("error = %v, want it to wrap hookErr", err)
	}
	if res.Stop != agentharness.StopHook {
		t.Errorf("Stop = %q, want hook", res.Stop)
	}
	if mock.CallCount() != 0 {
		t.Errorf("model calls = %d, want 0 (hook fires before call)", mock.CallCount())
	}
}

// ─── RunOnce infrastructure error ────────────────────────────────────────────

func TestRunOnceClientError(t *testing.T) {
	boom := errors.New("provider exploded")
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(boom))

	h := newRunOnceHarness(t, mock, agentharness.Deps{Now: fixedClock})
	res, err := h.RunOnce(context.Background(), simpleReq("go"))
	if err == nil {
		t.Fatal("RunOnce: error = nil, want client error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want it to wrap boom", err)
	}
	if res.Stop != agentharness.StopError {
		t.Errorf("Stop = %q, want error", res.Stop)
	}
}

// ─── RunOnceStream event sequence ────────────────────────────────────────────

func TestRunOnceStreamEventSequence(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("streamed answer", llmclient.StopEndTurn,
		llmclient.Usage{InputTokens: 5, OutputTokens: 3}))

	h := newRunOnceHarness(t, mock, agentharness.Deps{Now: advancingClock(time.Millisecond)})

	ch, err := h.RunOnceStream(context.Background(), simpleReq("stream this"))
	if err != nil {
		t.Fatalf("RunOnceStream: %v", err)
	}

	var (
		kinds      []agentharness.HarnessEventKind
		textDeltas []string
		terminal   *agentharness.HarnessEvent
	)
	for e := range ch {
		kinds = append(kinds, e.Kind)
		if e.Kind == agentharness.HarnessEventTextDelta {
			textDeltas = append(textDeltas, e.TextDelta)
		}
		if e.Kind == agentharness.HarnessEventResult {
			ev := e
			terminal = &ev
		}
	}

	// Terminal event must be present and carry a complete result.
	if terminal == nil {
		t.Fatal("no terminal HarnessEventResult received")
	}
	if terminal.Err != nil {
		t.Errorf("terminal Err = %v, want nil", terminal.Err)
	}
	if terminal.Result == nil {
		t.Fatal("terminal Result is nil")
	}
	if terminal.Result.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed", terminal.Result.Stop)
	}
	if terminal.Result.Turns != 1 {
		t.Errorf("Turns = %d, want 1", terminal.Result.Turns)
	}
	if terminal.Result.FinalText != "streamed answer" {
		t.Errorf("FinalText = %q, want %q", terminal.Result.FinalText, "streamed answer")
	}
	if terminal.Result.Final == nil {
		t.Error("Final checkpoint is nil")
	}

	// Text deltas must reconstruct the full text.
	if got := strings.Join(textDeltas, ""); got != "streamed answer" {
		t.Errorf("text deltas = %q, want %q", got, "streamed answer")
	}

	// TurnStart and TurnComplete must appear before Result.
	var sawTurnStart, sawTurnComplete bool
	for _, k := range kinds {
		switch k {
		case agentharness.HarnessEventTurnStart:
			sawTurnStart = true
		case agentharness.HarnessEventTurnComplete:
			sawTurnComplete = true
		}
	}
	if !sawTurnStart {
		t.Error("HarnessEventTurnStart not emitted")
	}
	if !sawTurnComplete {
		t.Error("HarnessEventTurnComplete not emitted")
	}
	// Last event must be Result.
	if kinds[len(kinds)-1] != agentharness.HarnessEventResult {
		t.Errorf("last event = %q, want result", kinds[len(kinds)-1])
	}
}

// TestRunOnceStreamBudgetDenial verifies budget denial flows through the stream.
func TestRunOnceStreamBudgetDenial(t *testing.T) {
	clock := advancingClock(time.Millisecond)
	guard := budgetguard.New(budgetguard.Config{
		MaxReqPerMin: 0,
		MaxReqPerDay: 100,
		MaxUSDPerDay: 100,
	}, clock)
	mock := llmclient.NewMockClient()

	h := newRunOnceHarness(t, mock, agentharness.Deps{Budget: guard, Now: clock})

	ch, err := h.RunOnceStream(context.Background(), simpleReq("hi"))
	if err != nil {
		t.Fatalf("RunOnceStream: %v", err)
	}

	var terminal *agentharness.HarnessEvent
	var sawBudgetDenied bool
	for e := range ch {
		if e.Kind == agentharness.HarnessEventBudgetDenied {
			sawBudgetDenied = true
		}
		if e.Kind == agentharness.HarnessEventResult {
			ev := e
			terminal = &ev
		}
	}
	if !sawBudgetDenied {
		t.Error("HarnessEventBudgetDenied not emitted")
	}
	if terminal == nil || terminal.Result == nil {
		t.Fatal("no terminal event")
	}
	if terminal.Result.Stop != agentharness.StopBudget {
		t.Errorf("Stop = %q, want budget", terminal.Result.Stop)
	}
}

// TestRunOnceTraceCorrelation verifies the injected trace is stamped on events.
func TestRunOnceTraceCorrelation(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("ok", llmclient.StopEndTurn, llmclient.Usage{}))
	ms := &sink.MemorySink{}

	h := newRunOnceHarness(t, mock, agentharness.Deps{Sink: ms, Now: fixedClock})

	tc := event.TraceContext{RunID: "trace-abc", SpanID: "span-xyz"}
	ctx := event.WithTrace(context.Background(), tc)

	if _, err := h.RunOnce(ctx, simpleReq("go")); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	events := ms.Events()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Attributes[event.AttrRunID] != "trace-abc" {
		t.Errorf("run_id = %q, want trace-abc", events[0].Attributes[event.AttrRunID])
	}
}

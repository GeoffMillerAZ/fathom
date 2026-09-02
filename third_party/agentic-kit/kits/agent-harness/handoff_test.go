package agentharness_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
	agentharness "github.com/geoffmilleraz/agentic-kit/kits/agent-harness"
)

func TestHandoffOnMaxTurns(t *testing.T) {
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", json.RawMessage(`{}`), llmclient.Usage{}),
		toolUseEntry("tu-2", "beta", json.RawMessage(`{}`), llmclient.Usage{}),
		toolUseEntry("tu-3", "alpha", json.RawMessage(`{}`), llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "a"), echoTool("beta", "b"))
	ms := &sink.MemorySink{}
	h, err := agentharness.New(agentharness.Config{MaxTurns: 3},
		agentharness.Deps{Client: mock, Registry: reg, Sink: ms, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != agentharness.StopMaxTurns {
		t.Fatalf("Stop = %q, want max_turns", res.Stop)
	}
	if res.Handoff == nil {
		t.Fatal("RunResult.Handoff = nil on max_turns, want a record")
	}
	ho := *res.Handoff
	want := agentharness.Handoff{
		StopCause:     agentharness.StopMaxTurns,
		Turns:         3,
		LastTool:      "alpha",
		CheckpointRef: agentharness.CheckpointRef(res.Final),
		Tried:         []string{"alpha", "beta"},
		Summary:       "run stopped on max_turns after 3 turn(s); last tool alpha; tried [alpha, beta]; resume from " + agentharness.CheckpointRef(res.Final),
	}
	if !reflect.DeepEqual(ho, want) {
		t.Errorf("Handoff =\n%+v\nwant\n%+v", ho, want)
	}
	if !strings.HasPrefix(ho.CheckpointRef, "checkpoint:v1:turn3:") || len(ho.CheckpointRef) != len("checkpoint:v1:turn3:")+16 {
		t.Errorf("CheckpointRef = %q, want checkpoint:v1:turn3:<16 hex>", ho.CheckpointRef)
	}
	if got := agentharness.HandoffFor(res); !reflect.DeepEqual(got, ho) {
		t.Errorf("HandoffFor(result) = %+v, want the attached record %+v (pure and deterministic)", got, ho)
	}

	events := eventsWithStep(ms.Events(), agentharness.StepHandoff)
	if len(events) != 1 {
		t.Fatalf("handoff events = %d, want exactly 1", len(events))
	}
	a := events[0].Attributes
	if a[agentharness.AttrStopCause] != "max_turns" || a[agentharness.AttrTurns] != "3" || a[agentharness.AttrLastTool] != "alpha" || a[agentharness.AttrTried] != "alpha,beta" || a[agentharness.AttrCheckpointRef] != ho.CheckpointRef {
		t.Errorf("handoff event attrs = %v", a)
	}
}

func TestHandoffOnBudget(t *testing.T) {
	clock := advancingClock(time.Millisecond)
	guard := budgetguard.New(budgetguard.Config{MaxReqPerMin: 100, MaxReqPerDay: 100, MaxUSDPerDay: 0.015}, clock) // one 0.01 charge fits
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", json.RawMessage(`{}`), llmclient.Usage{}),
		toolUseEntry("tu-2", "alpha", json.RawMessage(`{}`), llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "a"))
	ms := &sink.MemorySink{}
	h, err := agentharness.New(agentharness.Config{MaxTurns: 10, EstimatedUSDPerCall: 0.01},
		agentharness.Deps{Client: mock, Registry: reg, Budget: guard, Sink: ms, Now: clock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "spend")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != agentharness.StopBudget || res.Handoff == nil {
		t.Fatalf("Stop = %q, Handoff = %+v; want budget with a record", res.Stop, res.Handoff)
	}
	if res.Handoff.Turns != 1 || res.Handoff.LastTool != "alpha" || res.Handoff.StopCause != agentharness.StopBudget {
		t.Errorf("Handoff = %+v, want turns=1 last_tool=alpha stop=budget", res.Handoff)
	}
	if n := len(eventsWithStep(ms.Events(), agentharness.StepHandoff)); n != 1 {
		t.Errorf("handoff events = %d, want 1", n)
	}
}

func TestHandoffOnMaxTokensNoTools(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("trunc", llmclient.StopMaxTokens, llmclient.Usage{}))
	reg := newRegistry(t)
	h, err := agentharness.New(agentharness.Config{MaxTurns: 3},
		agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != agentharness.StopMaxTokens || res.Handoff == nil {
		t.Fatalf("Stop = %q, Handoff = %+v; want max_tokens with a record", res.Stop, res.Handoff)
	}
	if res.Handoff.LastTool != "" || res.Handoff.Tried != nil || !strings.Contains(res.Handoff.Summary, "no tool was called") {
		t.Errorf("Handoff = %+v, want no tools and a summary saying so", res.Handoff)
	}
}

func TestNoHandoffOnOtherStops(t *testing.T) {
	tests := []struct {
		name  string
		entry llmclient.MockEntry
		want  agentharness.StopCause
	}{
		{name: "completed", entry: textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}), want: agentharness.StopCompleted},
		{name: "refusal", entry: textEntry("no", llmclient.StopRefusal, llmclient.Usage{}), want: agentharness.StopRefusal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := llmclient.NewMockClient(tc.entry)
			ms := &sink.MemorySink{}
			h, err := agentharness.New(agentharness.Config{MaxTurns: 3},
				agentharness.Deps{Client: mock, Registry: newRegistry(t), Sink: ms, Now: fixedClock})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			res, err := h.Run(context.Background(), "go")
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Stop != tc.want {
				t.Fatalf("Stop = %q, want %q", res.Stop, tc.want)
			}
			if res.Handoff != nil {
				t.Errorf("Handoff = %+v, want nil on %s", res.Handoff, tc.want)
			}
			if n := len(eventsWithStep(ms.Events(), agentharness.StepHandoff)); n != 0 {
				t.Errorf("handoff events = %d, want 0", n)
			}
		})
	}
}

func TestHandoffStreamAndRunOnce(t *testing.T) {
	t.Run("stream emits the handoff before the terminal event", func(t *testing.T) {
		mock := llmclient.NewMockClient(toolUseEntry("tu-1", "alpha", json.RawMessage(`{}`), llmclient.Usage{}))
		h, err := agentharness.New(agentharness.Config{MaxTurns: 1},
			agentharness.Deps{Client: mock, Registry: newRegistry(t, echoTool("alpha", "a")), Now: fixedClock})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ch, err := h.RunStream(context.Background(), "go")
		if err != nil {
			t.Fatalf("RunStream: %v", err)
		}
		var kinds []agentharness.HarnessEventKind
		var ho *agentharness.Handoff
		for ev := range ch {
			kinds = append(kinds, ev.Kind)
			if ev.Kind == agentharness.HarnessEventHandoff {
				ho = ev.Handoff
			}
		}
		if ho == nil || ho.StopCause != agentharness.StopMaxTurns || ho.LastTool != "alpha" {
			t.Fatalf("handoff stream event = %+v, want a max_turns record naming alpha", ho)
		}
		if kinds[len(kinds)-1] != agentharness.HarnessEventResult || kinds[len(kinds)-2] != agentharness.HarnessEventHandoff {
			t.Errorf("event kinds = %v, want handoff immediately before result", kinds)
		}
	})

	t.Run("RunOnce on max_tokens carries a handoff", func(t *testing.T) {
		mock := llmclient.NewMockClient(textEntry("trunc", llmclient.StopMaxTokens, llmclient.Usage{}))
		h, err := agentharness.New(agentharness.Config{MaxTurns: 1}, agentharness.Deps{Client: mock, Now: fixedClock})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		res, err := h.RunOnce(context.Background(), llmclient.Request{Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("hi")}},
		}})
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if res.Stop != agentharness.StopMaxTokens || res.Handoff == nil || res.Handoff.Turns != 1 {
			t.Errorf("RunOnce result = stop %q handoff %+v; want max_tokens with turns=1", res.Stop, res.Handoff)
		}
	})
}

func TestHandoffJSONAndCheckpointRef(t *testing.T) {
	ho := agentharness.Handoff{StopCause: agentharness.StopBudget, Turns: 2, LastTool: "x", CheckpointRef: "checkpoint:v1:turn2:0123456789abcdef", Tried: []string{"x"}, Summary: "s"}
	b, err := json.Marshal(ho)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"stop_cause":"budget","turns":2,"last_tool":"x","checkpoint_ref":"checkpoint:v1:turn2:0123456789abcdef","tried":["x"],"summary":"s"}`
	if string(b) != want {
		t.Errorf("json = %s\nwant %s", b, want)
	}
	if got := agentharness.CheckpointRef(nil); got != "" {
		t.Errorf("CheckpointRef(nil) = %q, want empty", got)
	}
	cp := &agentharness.Checkpoint{Version: 1, TurnCursor: 2, ConvSnapshot: []byte(`{"version":1,"messages":[]}`)}
	if a, b := agentharness.CheckpointRef(cp), agentharness.CheckpointRef(cp); a != b || !strings.HasPrefix(a, "checkpoint:v1:turn2:") {
		t.Errorf("CheckpointRef unstable or malformed: %q vs %q", a, b)
	}
}

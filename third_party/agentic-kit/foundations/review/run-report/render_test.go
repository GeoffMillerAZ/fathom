package runreport_test

import (
	"context"
	"strings"
	"testing"
	"time"

	investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	runreport "github.com/geoffmilleraz/agentic-kit/foundations/review/run-report"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
	agentharness "github.com/geoffmilleraz/agentic-kit/kits/agent-harness"
)

// ─── clock helpers ────────────────────────────────────────────────────────────

var epoch = time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// ─── Render — nil / empty input degrades gracefully ──────────────────────────

func TestRender_EmptyInput(t *testing.T) {
	got, err := runreport.Render(runreport.Input{})
	if err != nil {
		t.Fatalf("Render: unexpected error: %v", err)
	}
	s := string(got)
	// All sections must be present.
	for _, want := range []string{
		"# Run report",
		"## Summary",
		"## Agent activity",
		"## Turn timeline",
		"## Flags",
		"## Cost",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing section %q in output", want)
		}
	}
	// Absent data produces "not captured", never fabricated values.
	if !strings.Contains(s, "not captured") {
		t.Error("expected 'not captured' for missing sections")
	}
	if !strings.Contains(s, "(no trace ID)") {
		t.Error("expected '(no trace ID)' when RunID is empty")
	}
}

// ─── Render — golden test (byte-stable) ──────────────────────────────────────

func TestRender_Golden(t *testing.T) {
	start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	finish := time.Date(2025, 6, 15, 10, 2, 30, 0, time.UTC)

	j := investigationjournal.NewJournalWithClock(fixedClock(start))
	mustAppend := func(step, note string) {
		t.Helper()
		if err := j.Append(investigationjournal.Entry{
			Step: step,
			Note: note,
			At:   start,
		}); err != nil {
			t.Fatalf("journal.Append: %v", err)
		}
	}
	mustAppend("run-started", "user asked: what is 2+2?")
	mustAppend("turn-completed", "turn 0: stop_reason=end_turn tools=[]")
	mustAppend("run-ended", "stop=completed turns=1 tool_calls=0")

	// Build telemetry events that mirror what the harness would emit.
	turnEvent := event.Event{
		ID:        "ev-turn-0",
		Kind:      event.KindInfo,
		Source:    agentharness.TelemetrySource,
		Timestamp: start,
		Attributes: map[string]string{
			agentharness.AttrStep:       agentharness.StepTurn,
			agentharness.AttrTurn:       "0",
			agentharness.AttrStopReason: "end_turn",
			agentharness.AttrDurationMS: "1234",
			agentharness.AttrToolNames:  "",
			event.AttrRunID:             "aabbccdd",
			event.AttrSpanID:            "11223344",
		},
	}

	in := runreport.Input{
		RunID:      "aabbccdd",
		StartedAt:  start,
		FinishedAt: finish,
		Model:      "claude-3-5-sonnet",
		StopCause:  "completed",
		Turns:      1,
		ToolCalls:  0,
		Journal:    j,
		Events:     []event.Event{turnEvent},
		CostReport: "cost report\nby label:\n  \"\": calls=1 input=10 output=20 cache_read=0 cache_write=0 usd=0.000030\ntotal: calls=1 input=10 output=20 cache_read=0 cache_write=0 usd=0.000030\n",
	}

	got, err := runreport.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Call twice — must be byte-identical.
	got2, err := runreport.Render(in)
	if err != nil {
		t.Fatalf("Render (2nd call): %v", err)
	}
	if string(got) != string(got2) {
		t.Error("Render is not byte-stable: two calls with identical input produced different output")
	}

	s := string(got)
	t.Logf("--- rendered output ---\n%s", s)

	// Structural checks (golden anchors).
	checks := []struct{ name, want string }{
		{"run id", "aabbccdd"},
		{"start time", "2025-06-15T10:00:00Z"},
		{"finish time", "2025-06-15T10:02:30Z"},
		{"duration", "2m30s"},
		{"model", "claude-3-5-sonnet"},
		{"stop cause", "completed"},
		{"turns", "| 1 |"},
		{"tool calls", "| 0 |"},
		{"journal step", "run-started"},
		{"journal note", "user asked: what is 2+2?"},
		{"turn row turn 0", "| 0 |"},
		{"stop reason", "end_turn"},
		{"duration ms", "1234"},
		{"cost block", "calls=1"},
		{"no flags", "no flags raised"},
	}
	for _, c := range checks {
		if !strings.Contains(s, c.want) {
			t.Errorf("golden check %q: %q not found in output", c.name, c.want)
		}
	}
}

// ─── Render — injection flags section ────────────────────────────────────────

func TestRender_InjectionFlags(t *testing.T) {
	injEvent := event.Event{
		ID:        "ev-inj-0",
		Kind:      event.KindError,
		Source:    agentharness.TelemetrySource,
		Timestamp: epoch,
		Attributes: map[string]string{
			agentharness.AttrStep:      agentharness.StepInjection,
			agentharness.AttrTurn:      "1",
			agentharness.AttrAction:    "block",
			agentharness.AttrSeverity:  "high",
			agentharness.AttrRuleIDs:   "prompt-leak",
			agentharness.AttrToolUseID: "tu-abc",
		},
	}

	got, err := runreport.Render(runreport.Input{Events: []event.Event{injEvent}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(got)
	for _, want := range []string{"Injection detections", "block", "high", "prompt-leak", "tu-abc"} {
		if !strings.Contains(s, want) {
			t.Errorf("injection flags: %q not found", want)
		}
	}
}

// ─── Render — no journal ──────────────────────────────────────────────────────

func TestRender_NoJournal(t *testing.T) {
	got, err := runreport.Render(runreport.Input{Turns: 2})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), "not captured") {
		t.Error("expected 'not captured' in agent activity section when journal is nil")
	}
}

// ─── Harness integration — build a real run, render it ───────────────────────

func TestRender_HarnessIntegration(t *testing.T) {
	now := epoch
	clock := func() time.Time { now = now.Add(time.Millisecond * 100); return now }

	mem := &sink.MemorySink{}
	j := investigationjournal.NewJournalWithClock(clock)

	client := llmclient.NewMockClient(
		llmclient.MockEntryOK("The answer is 4."),
	)

	h, err := agentharness.New(
		agentharness.Config{
			MaxTurns: 5,
			Model:    "mock-model",
		},
		agentharness.Deps{
			Client:  client,
			Sink:    mem,
			Journal: j,
			Now:     clock,
		},
	)
	if err != nil {
		t.Fatalf("agentharness.New: %v", err)
	}

	ctx := context.Background()
	result, err := h.RunOnce(ctx, llmclient.Request{
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("what is 2+2?")}},
		},
	})
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	in := runreport.Input{
		RunID:      "integration-test",
		StartedAt:  epoch,
		FinishedAt: now,
		Model:      "mock-model",
		StopCause:  string(result.Stop),
		Turns:      result.Turns,
		ToolCalls:  result.ToolCalls,
		Journal:    j,
		Events:     mem.Events(),
	}

	got, err := runreport.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(got)
	t.Logf("--- harness integration output ---\n%s", s)

	// The harness journals "run-started" and "run-ended" as steps.
	for _, want := range []string{"run-started", "run-ended"} {
		if !strings.Contains(s, want) {
			t.Errorf("harness integration: journal step %q not found in report", want)
		}
	}
	// StopCause must appear in summary.
	if !strings.Contains(s, "completed") {
		t.Error("expected 'completed' stop cause in report")
	}
}

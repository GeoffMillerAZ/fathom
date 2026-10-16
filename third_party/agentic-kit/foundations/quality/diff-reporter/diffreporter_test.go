package diffreporter

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	llmclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	evaljournal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-journal"
)

var epoch = time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

func makeRecord(runID, scenID string, verdict evaljournal.Verdict, score *float64) evaljournal.Record {
	r := evaljournal.Record{
		RunID:        runID,
		ScenarioID:   scenID,
		Producer:     evaljournal.Producer{Model: "m"},
		Judge:        evaljournal.Judge{Model: "j", Family: "f"},
		Verdict:      verdict,
		Decisive:     verdict != evaljournal.VerdictAbstain,
		Rationale:    "ok",
		EvidenceRefs: []string{},
		OccurredAt:   epoch,
	}
	if r.Verdict == evaljournal.VerdictAbstain || r.Verdict == evaljournal.VerdictError {
		r.Rationale = ""
		r.Decisive = false
	}
	r.Score = score
	return r
}

func ptr(f float64) *float64 { return &f }

// --- Diff ---

func TestDiff_empty(t *testing.T) {
	d := Diff(nil, nil, DiffOptions{})
	if len(d.ScoreDeltas) != 0 || len(d.VerdictFlips) != 0 {
		t.Errorf("expected empty delta, got %+v", d)
	}
	if d.Regressions != 0 || d.Improvements != 0 {
		t.Errorf("expected zero counts, got %+v", d)
	}
}

func TestDiff_noChange(t *testing.T) {
	base := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	cand := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	d := Diff(base, cand, DiffOptions{})
	if d.Regressions != 0 || d.Improvements != 0 {
		t.Errorf("no-change: expected 0/0, got %d/%d", d.Regressions, d.Improvements)
	}
	if len(d.VerdictFlips) != 0 {
		t.Errorf("expected no flips, got %v", d.VerdictFlips)
	}
}

func TestDiff_verdictFlipIsRegression(t *testing.T) {
	base := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	cand := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictFail, ptr(0.2))}
	d := Diff(base, cand, DiffOptions{})
	if d.Regressions != 1 {
		t.Errorf("expected 1 regression (flip), got %d", d.Regressions)
	}
	if len(d.VerdictFlips) != 1 {
		t.Errorf("expected 1 flip, got %d", len(d.VerdictFlips))
	}
}

func TestDiff_scoreImprovementCounted(t *testing.T) {
	base := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.5))}
	cand := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	d := Diff(base, cand, DiffOptions{})
	if d.Improvements != 1 {
		t.Errorf("expected 1 improvement, got %d", d.Improvements)
	}
}

func TestDiff_scoreDropBelowThresholdNotRegression(t *testing.T) {
	base := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	cand := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.85))}
	d := Diff(base, cand, DiffOptions{ScoreDropThreshold: 0.1})
	// drop is 0.05, threshold is 0.1 — not a regression
	if d.Regressions != 0 {
		t.Errorf("expected 0 regressions, got %d", d.Regressions)
	}
}

func TestDiff_scoreDropAboveThresholdIsRegression(t *testing.T) {
	base := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	cand := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.7))}
	d := Diff(base, cand, DiffOptions{ScoreDropThreshold: 0.1})
	// drop is 0.2 >= threshold 0.1 → regression
	if d.Regressions != 1 {
		t.Errorf("expected 1 regression (score drop), got %d", d.Regressions)
	}
}

func TestDiff_lastRecordWins(t *testing.T) {
	// Two records with the same key; the last should win.
	base := []evaljournal.Record{
		makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.5)),
		makeRecord("concern-a", "s1", evaljournal.VerdictFail, ptr(0.2)),
	}
	cand := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	d := Diff(base, cand, DiffOptions{})
	// base last = fail, cand = pass → flip → regression
	if d.Regressions != 1 {
		t.Errorf("expected 1 regression (flip from fail to pass), got %d", d.Regressions)
	}
}

func TestDiff_sortedOutput(t *testing.T) {
	base := []evaljournal.Record{
		makeRecord("concern-b", "s2", evaljournal.VerdictPass, ptr(0.7)),
		makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.8)),
	}
	cand := []evaljournal.Record{
		makeRecord("concern-b", "s2", evaljournal.VerdictFail, ptr(0.3)),
		makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9)),
	}
	d := Diff(base, cand, DiffOptions{})
	// Output must be sorted: (s1, concern-a) before (s2, concern-b)
	if len(d.ScoreDeltas) < 2 {
		t.Fatalf("expected 2 score deltas, got %d", len(d.ScoreDeltas))
	}
	if d.ScoreDeltas[0].ScenarioID > d.ScoreDeltas[1].ScenarioID {
		t.Errorf("score_deltas not sorted: %s > %s", d.ScoreDeltas[0].ScenarioID, d.ScoreDeltas[1].ScenarioID)
	}
}

// --- Golden: byte-stable marshal ---

func TestDiff_byteStable(t *testing.T) {
	base := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictPass, ptr(0.9))}
	cand := []evaljournal.Record{makeRecord("concern-a", "s1", evaljournal.VerdictFail, ptr(0.4))}
	d1 := Diff(base, cand, DiffOptions{})
	d2 := Diff(base, cand, DiffOptions{})
	b1, _ := json.Marshal(d1)
	b2, _ := json.Marshal(d2)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("Diff marshal not byte-stable:\n%s\n%s", b1, b2)
	}
}

// --- TrajectoryDiff ---

func toolUseMsg(name string, input json.RawMessage) llmclient.Message {
	return llmclient.Message{
		Role:    llmclient.RoleAssistant,
		Content: []llmclient.ContentBlock{llmclient.ToolUseBlock("tool-id", name, input)},
	}
}

func textMsg(role llmclient.Role, text string) llmclient.Message {
	return llmclient.Message{
		Role:    role,
		Content: []llmclient.ContentBlock{llmclient.TextBlock(text)},
	}
}

func TestTrajectoryDiff_identical(t *testing.T) {
	input := json.RawMessage(`{"path":"/foo.go"}`)
	msgs := []llmclient.Message{
		textMsg(llmclient.RoleUser, "analyze"),
		toolUseMsg("read_file", input),
		textMsg(llmclient.RoleUser, "done"),
	}
	d := TrajectoryDiff(msgs, msgs)
	if d.CallsChanged || d.StopChanged || d.TurnsChanged {
		t.Errorf("expected no changes: %+v", d)
	}
}

func TestTrajectoryDiff_callsChanged(t *testing.T) {
	baseInput := json.RawMessage(`{"path":"/a.go"}`)
	candInput := json.RawMessage(`{"path":"/b.go"}`)
	base := []llmclient.Message{toolUseMsg("read_file", baseInput)}
	cand := []llmclient.Message{toolUseMsg("read_file", candInput)}
	d := TrajectoryDiff(base, cand)
	if !d.CallsChanged {
		t.Error("expected CallsChanged=true for different args")
	}
}

func TestTrajectoryDiff_differentToolName(t *testing.T) {
	input := json.RawMessage(`{"path":"/foo.go"}`)
	base := []llmclient.Message{toolUseMsg("read_file", input)}
	cand := []llmclient.Message{toolUseMsg("write_file", input)}
	d := TrajectoryDiff(base, cand)
	if !d.CallsChanged {
		t.Error("expected CallsChanged=true for different tool name")
	}
}

func TestTrajectoryDiff_turnsChanged(t *testing.T) {
	base := []llmclient.Message{textMsg(llmclient.RoleUser, "hello")}
	cand := []llmclient.Message{
		textMsg(llmclient.RoleUser, "hello"),
		textMsg(llmclient.RoleAssistant, "world"),
	}
	d := TrajectoryDiff(base, cand)
	if !d.TurnsChanged {
		t.Error("expected TurnsChanged=true")
	}
	if d.BaseTurns != 1 || d.CandTurns != 2 {
		t.Errorf("expected BaseTurns=1 CandTurns=2, got %d/%d", d.BaseTurns, d.CandTurns)
	}
}

func TestTrajectoryDiff_stopReasonInferred(t *testing.T) {
	input := json.RawMessage(`{"path":"/foo.go"}`)
	// base ends with tool_use → StopToolUse
	// cand ends with text → StopEndTurn
	base := []llmclient.Message{toolUseMsg("read_file", input)}
	cand := []llmclient.Message{textMsg(llmclient.RoleAssistant, "done")}
	d := TrajectoryDiff(base, cand)
	if d.BaseStop != string(llmclient.StopToolUse) {
		t.Errorf("BaseStop: got %q, want %q", d.BaseStop, llmclient.StopToolUse)
	}
	if d.CandStop != string(llmclient.StopEndTurn) {
		t.Errorf("CandStop: got %q, want %q", d.CandStop, llmclient.StopEndTurn)
	}
	if !d.StopChanged {
		t.Error("expected StopChanged=true")
	}
}

func TestTrajectoryDiff_emptyTrajectories(t *testing.T) {
	d := TrajectoryDiff(nil, nil)
	if d.CallsChanged || d.StopChanged || d.TurnsChanged {
		t.Errorf("empty trajectories: expected no changes: %+v", d)
	}
	if d.BaseTurns != 0 || d.CandTurns != 0 {
		t.Errorf("expected 0 turns, got %d/%d", d.BaseTurns, d.CandTurns)
	}
}

func TestTrajectoryDiff_byteStable(t *testing.T) {
	input := json.RawMessage(`{"key":"value"}`)
	base := []llmclient.Message{toolUseMsg("search", input)}
	cand := []llmclient.Message{toolUseMsg("search", input), textMsg(llmclient.RoleAssistant, "done")}
	d1 := TrajectoryDiff(base, cand)
	d2 := TrajectoryDiff(base, cand)
	b1, _ := json.Marshal(d1)
	b2, _ := json.Marshal(d2)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("TrajectoryDiff not byte-stable:\n%s\n%s", b1, b2)
	}
}

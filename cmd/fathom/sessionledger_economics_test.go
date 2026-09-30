package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// writeTranscript writes lines as a Claude Code transcript and returns its path.
func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// ledgerRowMap marshals e the way appendSessionLedger does and decodes it
// back as a generic object, so a test reads the row a reader would.
func ledgerRowMap(t *testing.T, e sessionLedgerEntry) map[string]any {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(b, &row); err != nil {
		t.Fatal(err)
	}
	return row
}

// PE-13w: the Stop hook's ledger row carries the whole-session tool-call
// count from the same fold that fills its token and cache fields, and the
// field is absent (never a zero) when no transcript was parsed (law 12).
func TestSessionLedgerEntryCarriesToolCalls(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	path := writeTranscript(t,
		`{"type":"user","timestamp":"2026-09-29T11:00:00Z"}`,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:05Z","message":{"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":20},"content":[{"type":"tool_use","id":"a","name":"Read","input":{"file_path":"x.go"}},{"type":"tool_use","id":"b","name":"Bash"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:09Z","message":{"content":[{"type":"text"},{"type":"tool_use","id":"c","name":"Edit"}]}}`,
	)
	e := buildSessionLedgerEntry("sess-tc", "fathom", transcriptSummaryFrom(path), nil, now)
	if e.ToolCalls == nil || *e.ToolCalls != 3 {
		t.Fatalf("ToolCalls = %v, want 3 (every tool_use block, any name)", e.ToolCalls)
	}
	row := ledgerRowMap(t, e)
	if row["tool_calls"] != float64(3) {
		t.Errorf("row tool_calls = %v, want 3", row["tool_calls"])
	}
	if row["tokens_in"] != float64(10) || row["tokens_cache_read"] != float64(100) {
		t.Errorf("the token and cache fields ride the same fold: %v", row)
	}

	// A parsed transcript with no tool call is a real zero, recorded.
	quiet := writeTranscript(t, `{"type":"assistant","timestamp":"2026-09-29T11:00:05Z","message":{"content":[{"type":"text"}]}}`)
	z := buildSessionLedgerEntry("sess-quiet", "fathom", transcriptSummaryFrom(quiet), nil, now)
	if z.ToolCalls == nil || *z.ToolCalls != 0 {
		t.Errorf("a parsed transcript with no tool_use records 0, got %v", z.ToolCalls)
	}

	// No transcript parsed: the field is absent, never a fabricated zero.
	for name, ts := range map[string]transcriptSummary{
		"no path":    transcriptSummaryFrom(""),
		"unreadable": transcriptSummaryFrom(filepath.Join(t.TempDir(), "missing.jsonl")),
	} {
		e := buildSessionLedgerEntry("sess-none", "fathom", ts, nil, now)
		if e.ToolCalls != nil {
			t.Errorf("%s: ToolCalls = %d, want nil", name, *e.ToolCalls)
		}
		if _, present := ledgerRowMap(t, e)["tool_calls"]; present {
			t.Errorf("%s: tool_calls must be absent when no transcript was parsed", name)
		}
	}

	// The product path: the Stop hook's own append carries it.
	ledger := withTempSessionLedger(t)
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())
	recordSessionLedgerEntry("claude-code", "sess-hook", path, t.TempDir(), now, nil)
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["tool_calls"] != float64(3) {
		t.Errorf("the Stop hook's row tool_calls = %v, want 3", got["tool_calls"])
	}
}

// WI-9w: the Stop hook records the harness (the codec's id) and the model
// (the last assistant message's), sessionEffortsOf and agentSessionsOf carry
// them, and two sessions with different (model, harness) land in two
// ModelScorecard cells instead of one "unknown" cell. A row that recorded
// neither stays "unknown", named, never guessed.
func TestScorecardSplitsByRecordedModelAndHarness(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

	// The product path records both fields.
	ledger := withTempSessionLedger(t)
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())
	path := writeTranscript(t,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:05Z","message":{"model":"claude-sonnet-4-5","content":[{"type":"text"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:09Z","message":{"model":"<synthetic>","content":[{"type":"text"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:10Z","message":{"model":"claude-opus-4-5","content":[{"type":"text"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:11Z","message":{"model":"<synthetic>","content":[{"type":"text"}]}}`,
	)
	recordSessionLedgerEntry("claude-code", "sess-rec", path, t.TempDir(), now, nil)
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	if rec["harness"] != "claude-code" || rec["model"] != "claude-opus-4-5" {
		t.Errorf("recorded harness %v model %v, want claude-code and the last real assistant model", rec["harness"], rec["model"])
	}

	// Two sessions, two identities, one repository.
	rows := []string{
		`{"session_id":"s-a","repo":"svc","at":"2026-09-10T11:00:00Z","duration_ms":3600000,"tokens_in":100,"tokens_out":10,"harness":"claude-code","model":"claude-opus-4-5"}`,
		`{"session_id":"s-b","repo":"svc","at":"2026-09-11T11:00:00Z","duration_ms":3600000,"tokens_in":200,"tokens_out":20,"harness":"codex","model":"gpt-5-codex"}`,
		`{"session_id":"s-c","repo":"svc","at":"2026-09-12T11:00:00Z","duration_ms":3600000,"tokens_in":300,"tokens_out":30}`,
	}
	m := observatoryModel{ledger: foldSessionLedger([]byte(strings.Join(rows, "\n") + "\n"))}
	efforts := sessionEffortsOf(m)
	if len(efforts) != 3 || efforts[0].Harness != "claude-code" || efforts[0].Model != "claude-opus-4-5" ||
		efforts[1].Harness != "codex" || efforts[1].Model != "gpt-5-codex" || efforts[2].Harness != "" || efforts[2].Model != "" {
		t.Fatalf("sessionEffortsOf = %+v", efforts)
	}
	h := console.History{}
	prove := func(id, at string) {
		h.Rows = append(h.Rows, console.RowHistory{Service: "svc", Plan: "p.md", ID: id, Readings: []console.RowReading{
			{At: "2026-09-01T00:00:00Z", State: console.HistOpen}, {At: at, State: console.HistProven, Weight: 1}}})
		h.Readings = append(h.Readings, console.PlanReading{At: at, Root: "/src/svc", Service: "svc", Plan: "p.md"})
	}
	prove("R-1", "2026-09-10T10:30:00Z")
	prove("R-2", "2026-09-11T10:30:00Z")
	prove("R-3", "2026-09-12T10:30:00Z")
	join := console.JoinSessionProof(context.Background(), h, agentSessionsOf(m), console.HistoryFilter{Service: "svc"},
		func(context.Context, string, string) (string, bool) { return "", false })
	if len(join.Outcomes) != 3 {
		t.Fatalf("join outcomes = %+v (unjoined %+v)", join.Outcomes, join.Unjoined)
	}
	sc := console.ModelScorecard(console.EffortPerProvenChange(join, h, efforts, nil, console.NewZones(nil, nil)))
	cells := map[string]bool{}
	for _, c := range sc.Cells {
		cells[c.Harness+"/"+c.Model] = true
	}
	for _, want := range []string{"claude-code/claude-opus-4-5", "codex/gpt-5-codex", "unknown/unknown"} {
		if !cells[want] {
			t.Errorf("scorecard cells %v lack %s", cells, want)
		}
	}
	if len(sc.Cells) != 3 {
		t.Errorf("want three cells (two recorded identities and the unrecorded one), got %d", len(sc.Cells))
	}
}

// PE-14: under `stop-hook --harness codex` the ledger row is folded from
// the Codex rollout (transcript_path) by the harness port's CodexSessionCost,
// into the same token, cache, tool_calls and model fields a Claude Code row
// carries; the fold's state rides the session's one offset stamp, so a
// second Stop reads only what the rollout gained.
func TestStopHookLedgerFoldsCodexRollout(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "harness", "testdata", "rollouts", "codex", "modern.sanitized.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	rollout := filepath.Join(t.TempDir(), "rollout-2026-09-29T11-00-00-sess.jsonl")
	// The first Stop sees only the first half of the rollout's lines.
	lines := strings.SplitAfter(string(raw), "\n")
	half := strings.Join(lines[:len(lines)/2], "")
	if err := os.WriteFile(rollout, []byte(half), 0o600); err != nil {
		t.Fatal(err)
	}
	ledger := withTempSessionLedger(t)
	stamps := t.TempDir()
	t.Setenv("FATHOM_SESSION_STAMPS", stamps)
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_CHANNELS", "")
	resetObsCache()

	stop := func() map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"session_id": "codex-sess", "cwd": repo, "transcript_path": rollout, "stop_hook_active": false})
		restore := pipeStdin(t, string(body))
		var out, errb bytes.Buffer
		code := cmdStopHook([]string{"--harness", "codex", "--url", "http://127.0.0.1:1"}, &out, &errb)
		restore()
		if code != 0 {
			t.Fatalf("stop-hook --harness codex exit %d: %s", code, errb.String())
		}
		data, err := os.ReadFile(ledger)
		if err != nil {
			t.Fatal(err)
		}
		rows := strings.Split(strings.TrimSpace(string(data)), "\n")
		var row map[string]any
		if err := json.Unmarshal([]byte(rows[len(rows)-1]), &row); err != nil {
			t.Fatal(err)
		}
		return row
	}

	first := stop()
	if first["harness"] != "codex" {
		t.Errorf("harness = %v, want codex", first["harness"])
	}
	stampFiles, _ := filepath.Glob(filepath.Join(stamps, "transcript-*.json"))
	if len(stampFiles) != 1 {
		t.Fatalf("the Codex fold keeps its state in the session's one offset stamp, got %v", stampFiles)
	}
	var st transcriptOffsetStamp
	if b, err := os.ReadFile(stampFiles[0]); err != nil || json.Unmarshal(b, &st) != nil || st.Codex == nil || st.Codex.Offset == 0 {
		t.Fatalf("the stamp carries the CodexCostFold: %+v %v", st, err)
	}

	// The rest of the rollout arrives; the next Stop folds on from the stamp.
	if err := os.WriteFile(rollout, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	row := stop()
	for k, want := range map[string]any{
		"harness": "codex", "model": "gpt-5.6-sol",
		"tokens_in": float64(67196), "tokens_out": float64(3049), "tokens_cache_read": float64(635136), "tokens_cache_create": float64(0),
		"tokens_total": float64(67196 + 3049 + 635136), "tool_calls": float64(14),
	} {
		if row[k] != want {
			t.Errorf("codex row %s = %v, want %v", k, row[k], want)
		}
	}
	if row["tokens_note"] != "" {
		t.Errorf("tokens_note = %q, want empty beside real token fields", row["tokens_note"])
	}
	if n, _ := row["transcript_schema_note"].(string); !strings.Contains(n, "Codex rollout") {
		t.Errorf("the schema note names the Codex rollout, got %q", n)
	}

	// An unreadable rollout records no figure, never a zero.
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())
	rollout = filepath.Join(t.TempDir(), "missing.jsonl")
	gone := stop()
	for _, k := range []string{"tokens_in", "tool_calls", "model"} {
		if _, present := gone[k]; present {
			t.Errorf("an unreadable rollout must leave %s absent: %v", k, gone)
		}
	}
	if gone["tokens_note"] == "" {
		t.Error("an unreadable rollout names why tokens are absent")
	}
}

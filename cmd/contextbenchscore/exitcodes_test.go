package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// A three-task corpus with a committed criterion: strictly more successes,
// tokens <= 1x, floor 3, and a leak rule over the none arm.
const gatedTasksJSON = `{
  "version": 1,
  "machine_criterion": {"id": "H2-1", "require_strictly_more_successes": true, "max_token_ratio": 1.0, "min_trials": 3,
    "leak_rule": {"arm": "none", "min_passes": 2, "of_trials": 3}},
  "registered_criteria": [{"id": "H2-2", "require_strictly_more_successes": true, "max_token_ratio": 1.0, "min_trials": 3, "scoring": "precision",
    "leak_rule": {"arm": "none", "min_passes": 2, "of_trials": 3}}],
  "tasks": [
    {"id": "T1", "prompt": "p1", "ground_truth": ["a"], "scoring_rule": "exact-match"},
    {"id": "T2", "prompt": "p2", "ground_truth": ["b"], "scoring_rule": "exact-match"},
    {"id": "T3", "prompt": "p3", "ground_truth": ["c"], "scoring_rule": "exact-match", "requires": ["compiled-layer"]}
  ]
}`

func writeGatedCorpus(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "tasks.json")
	if err := os.WriteFile(path, []byte(gatedTasksJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeCells writes one ArmResult file per cell into dir.
func writeCells(t *testing.T, dir string, cells []contextbench.ArmResult) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range cells {
		b, _ := json.Marshal(c)
		name := c.TaskID + "-" + c.Arm + "-t" + string(rune('0'+c.Trial)) + ".json"
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func cell(task, arm string, trial int, ok bool, tokens int64) contextbench.ArmResult {
	return contextbench.ArmResult{TaskID: task, Arm: arm, Trial: trial, Success: ok, TotalTokens: tokens, ModelID: "claude-x", CostUSD: 0.5}
}

// threeTrials emits trials 1..3 of a cell with the same outcome.
func threeTrials(task, arm string, ok bool, tokens int64) []contextbench.ArmResult {
	var out []contextbench.ArmResult
	for i := 1; i <= 3; i++ {
		out = append(out, cell(task, arm, i, ok, tokens))
	}
	return out
}

func concat(groups ...[]contextbench.ArmResult) []contextbench.ArmResult {
	var out []contextbench.ArmResult
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// TestCmdLedger_ExitCodes pins the ledger's exit-code contract (PB-6):
// 0 pass, 1 fail, 3 indeterminate — and that a leaked task leaves the
// verdict while staying on the row.
func TestCmdLedger_ExitCodes(t *testing.T) {
	cases := []struct {
		name  string
		cells []contextbench.ArmResult
		want  int
	}{
		{"pass", concat(
			threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
			threeTrials("T2", "fathom", true, 100), threeTrials("T2", "grep", true, 200),
		), 0},
		{"fail: equal success", concat(
			threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", true, 200),
			threeTrials("T2", "fathom", true, 100), threeTrials("T2", "grep", true, 200),
		), 1},
		{"fail: token premium", concat(
			threeTrials("T1", "fathom", true, 300), threeTrials("T1", "grep", false, 200),
			threeTrials("T2", "fathom", true, 300), threeTrials("T2", "grep", true, 200),
		), 1},
		{"indeterminate: below the floor", concat(
			[]contextbench.ArmResult{cell("T1", "fathom", 1, true, 100), cell("T1", "grep", 1, false, 200)},
			threeTrials("T2", "fathom", true, 100), threeTrials("T2", "grep", true, 200),
		), 3},
		{"indeterminate: a task in one arm only", concat(
			threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
			threeTrials("T2", "fathom", true, 100),
		), 3},
		{"pass: the leaked task is excluded, the rest wins", concat(
			threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200), threeTrials("T1", "none", false, 50),
			threeTrials("T2", "fathom", true, 100), threeTrials("T2", "grep", true, 200), threeTrials("T2", "none", true, 50),
		), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			tasks := writeGatedCorpus(t, dir)
			results := filepath.Join(dir, "results")
			writeCells(t, results, c.cells)
			ledger := filepath.Join(dir, "ledger.jsonl")
			rc := run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", ledger, "-run-id", "r1", "-commit", "abc", "-arms", "fathom grep", "-repeat", "3", "-env", "compiled-layer"})
			if rc != c.want {
				t.Fatalf("exit %d, want %d", rc, c.want)
			}
			rows, err := contextbench.LoadLedger(ledger)
			if err != nil || len(rows) != 1 {
				t.Fatalf("one row must be appended whatever the verdict: %v %d", err, len(rows))
			}
			r := rows[0]
			if r.CorpusSHA256 == "" || r.ModelID != "claude-x" || r.CostUSD == 0 || len(r.PerTask) == 0 {
				t.Errorf("row must carry corpus hash, model id, cost, per-task outcomes: %+v", r)
			}
			// PB-10: the reliability figures ride on every paired row,
			// whatever the verdict — they are reported, never gating.
			if r.Reliability == nil || r.CISuccessDiff == nil || r.CISuccessDiff.Tasks != r.Tasks || r.BootstrapSeed == 0 {
				t.Errorf("row must carry ci_success_diff over its paired tasks and the seed: %+v", r.Reliability)
			}
			// With -env compiled-layer T3 was intended and never ran: 2 arms ×
			// 3 trials lost, plus whatever the case left out.
			if r.CellsLost < 6 {
				t.Errorf("cells_lost = %d, want >= 6 (T3 never ran)", r.CellsLost)
			}
		})
	}
}

func TestCmdLedger_LeakedTaskLeavesTheVerdictAndStaysOnTheRow(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	results := filepath.Join(dir, "results")
	// T2 is answered from memory by the none arm (3/3); without exclusion
	// fathom's win on T1 alone would be 3 vs 0 successes — with T2 both arms
	// pass, so the strictly-more clause still holds either way. What must
	// change is the row: T2 named as leaked, per_task without it.
	writeCells(t, results, concat(
		threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200), threeTrials("T1", "none", false, 50),
		threeTrials("T2", "fathom", true, 100), threeTrials("T2", "grep", true, 200), threeTrials("T2", "none", true, 50),
	))
	ledger := filepath.Join(dir, "ledger.jsonl")
	if rc := run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", ledger, "-run-id", "r1", "-commit", "abc"}); rc != 0 {
		t.Fatalf("exit %d", rc)
	}
	rows, _ := contextbench.LoadLedger(ledger)
	r := rows[0]
	if len(r.LeakedTasks) != 1 || r.LeakedTasks[0] != "T2" {
		t.Fatalf("leaked_tasks = %v, want [T2]", r.LeakedTasks)
	}
	if r.Tasks != 1 || len(r.PerTask) != 1 || r.PerTask[0].TaskID != "T1" {
		t.Errorf("the leaked task must leave the paired verdict: %+v", r)
	}
	// Without -env, T3 (requires compiled-layer) was never intended: not lost.
	if r.CellsLost != 0 {
		t.Errorf("cells_lost = %d, want 0 — a task the environment could not run was never intended", r.CellsLost)
	}
}

func TestCmdLedger_NamedCriterion(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	results := filepath.Join(dir, "results")
	writeCells(t, results, concat(threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200)))
	ledger := filepath.Join(dir, "ledger.jsonl")
	if rc := run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", ledger, "-run-id", "r1", "-commit", "abc", "-criterion", "H2-2"}); rc != 0 {
		t.Fatalf("exit %d", rc)
	}
	rows, _ := contextbench.LoadLedger(ledger)
	if rows[0].Evaluation.CriterionID != "H2-2" {
		t.Errorf("criterion_id = %q, want H2-2", rows[0].Evaluation.CriterionID)
	}
	if rc := run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", ledger, "-run-id", "r2", "-commit", "abc", "-criterion", "H7-7"}); rc != 2 {
		t.Errorf("an unregistered criterion is a usage error (2), got %d", rc)
	}
}

// TestCmdRegress_ExitCodes pins the regression gate's exit-code contract
// (PB-13): 0 no regression, 1 a task fell, 3 not comparable.
func TestCmdRegress_ExitCodes(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	ledger := filepath.Join(dir, "ledger.jsonl")

	// One row: nothing to compare against.
	writeCells(t, filepath.Join(dir, "a"), concat(
		threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
		threeTrials("T2", "fathom", true, 100), threeTrials("T2", "grep", false, 200),
	))
	if rc := run([]string{"ledger", "-tasks", tasks, "-results", filepath.Join(dir, "a"), "-ledger", ledger, "-run-id", "r1", "-commit", "abc"}); rc != 0 {
		t.Fatalf("seed ledger: exit %d", rc)
	}
	if rc := run([]string{"regress", "-ledger", ledger}); rc != 3 {
		t.Fatalf("a first run on a corpus has nothing to regress against: want 3, got %d", rc)
	}

	// Second row, same corpus, nothing fell.
	writeCells(t, filepath.Join(dir, "b"), concat(
		threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
		threeTrials("T2", "fathom", true, 100), threeTrials("T2", "grep", false, 200),
	))
	run([]string{"ledger", "-tasks", tasks, "-results", filepath.Join(dir, "b"), "-ledger", ledger, "-run-id", "r2", "-commit", "abd"})
	if rc := run([]string{"regress", "-ledger", ledger}); rc != 0 {
		t.Fatalf("no task fell: want 0, got %d", rc)
	}

	// Third row: T2 fell 3/3 -> 0/3 on the treatment arm.
	writeCells(t, filepath.Join(dir, "c"), concat(
		threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200),
		threeTrials("T2", "fathom", false, 100), threeTrials("T2", "grep", false, 200),
	))
	run([]string{"ledger", "-tasks", tasks, "-results", filepath.Join(dir, "c"), "-ledger", ledger, "-run-id", "r3", "-commit", "abe"})
	if rc := run([]string{"regress", "-ledger", ledger}); rc != 1 {
		t.Fatalf("T2 fell: want 1, got %d", rc)
	}
	if rc := run([]string{"regress", "-ledger", ledger, "-baseline", "r1"}); rc != 1 {
		t.Fatalf("explicit baseline r1: want 1, got %d", rc)
	}
	if rc := run([]string{"regress", "-ledger", filepath.Join(dir, "absent.jsonl")}); rc != 3 {
		t.Fatalf("an empty ledger is not comparable: want 3, got %d", rc)
	}
	// The subcommand itself, called directly: the same contract, and the
	// JSON form of the report.
	if rc := cmdRegress([]string{"-ledger", ledger, "-json"}); rc != 1 {
		t.Fatalf("cmdRegress direct: want 1, got %d", rc)
	}
	if rc := cmdRegress([]string{"-bogus"}); rc != 2 {
		t.Fatalf("a bad flag is a usage error: want 2, got %d", rc)
	}
}

func TestCmdTasks_FiltersByRequires(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	// Capture stdout.
	r, w, _ := os.Pipe()
	saved := os.Stdout
	os.Stdout = w
	rc := cmdTasks([]string{"-tasks", tasks})
	w.Close()
	os.Stdout = saved
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if rc != 0 {
		t.Fatalf("exit %d", rc)
	}
	// id NUL prompt NUL min_trials NUL per task (RC-18) — 3 fields/task, so
	// 2 tasks yield 6 NULs (7 parts, the trailing one empty).
	parts := bytes.Split(buf.Bytes(), []byte{0})
	if len(parts) != 7 || string(parts[0]) != "T1" || string(parts[3]) != "T2" {
		t.Fatalf("without compiled-layer only T1 and T2 run: %q", buf.String())
	}

	r, w, _ = os.Pipe()
	os.Stdout = w
	cmdTasks([]string{"-tasks", tasks, "-env", "compiled-layer"})
	w.Close()
	os.Stdout = saved
	buf.Reset()
	buf.ReadFrom(r)
	if parts := bytes.Split(buf.Bytes(), []byte{0}); len(parts) != 10 || string(parts[6]) != "T3" {
		t.Fatalf("with compiled-layer T3 runs too: %q", buf.String())
	}
}

func TestCmdScore_PrecisionAndTranscript(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	claudeJSON := filepath.Join(dir, "claude.json")
	os.WriteFile(claudeJSON, []byte("{\"result\":\"```\\na\\n```\",\"total_cost_usd\":0.25,\"modelUsage\":{\"claude-x\":{}},\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}"), 0o644)
	transcript := filepath.Join(dir, "stream.jsonl")
	os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"t"}],"usage":{"input_tokens":7,"output_tokens":1}}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"a"}]}}
`), 0o644)
	out := filepath.Join(dir, "out.json")
	rc := run([]string{"score", "-tasks", tasks, "-task-id", "T1", "-arm", "fathom", "-claude-json", claudeJSON, "-out", out, "-trial", "2", "-scoring", "precision", "-transcript", transcript})
	if rc != 0 {
		t.Fatalf("exit %d", rc)
	}
	raw, _ := os.ReadFile(out)
	var ar contextbench.ArmResult
	if err := json.Unmarshal(raw, &ar); err != nil {
		t.Fatal(err)
	}
	if !ar.Success || ar.Scoring != "precision" || ar.Precision != 1 || ar.Recall != 1 || ar.Predicted != 1 {
		t.Errorf("precision scoring not recorded: %+v", ar)
	}
	if ar.ModelID != "claude-x" || ar.CostUSD != 0.25 || ar.Trial != 2 {
		t.Errorf("model id / cost / trial not recorded: %+v", ar)
	}
	if ar.TokensToFirstGoldHit != 8 || ar.ToolCalls != 1 {
		t.Errorf("transcript figures not recorded: %+v", ar)
	}
	if rc := run([]string{"score", "-tasks", tasks, "-task-id", "T1", "-arm", "fathom", "-claude-json", claudeJSON, "-out", out, "-scoring", "bogus"}); rc != 2 {
		t.Errorf("unknown scoring is a usage error, got %d", rc)
	}
}

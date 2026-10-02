package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangOracleCallsFromTool (LB-F204, O264): beside -scip, -calls-from
// tool takes L2 from the tool-oracle's calls at the tool's own tier (B here)
// while the index still grades definitions (A), typed edges and imports; the
// index's call counts are not in the reading. Without the flag the calls file
// is still refused beside the index's own call truth.
func TestCmdLangOracleCallsFromTool(t *testing.T) {
	subject := t.TempDir()
	for _, f := range []string{"greeter.ts", "main.ts", "math.ts"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "internal", "scip", "testdata", "tsfixture", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subject, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scip := filepath.Join("..", "..", "internal", "scip", "testdata", "tsfixture", "index-no-display.scip")
	queue := filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")
	oracle := filepath.Join(t.TempDir(), "calls.json")
	// run -> greet is real; run -> nothing is a pair the tool invents; a call
	// in a file fathom does not read is out of scope.
	if err := os.WriteFile(oracle, []byte(`{"tool":"fake-cg","version":"1","tier":"B","licence":"EPL-2.0","calls":[
 {"path":"main.ts","from":"run","to":"greet"},{"path":"main.ts","from":"run","to":"nothing"},{"path":"gone.ts","from":"a","to":"b"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := []string{"-dialect", "typescript", "-subject", subject, "-subject-name", "fixture/ts", "-scip", scip, "-scip-defs", "-tool-oracle", oracle,
		"-date", "2026-10-02", "-commit", "abc1234", "-queue", queue}

	var stdout, stderr bytes.Buffer
	out := filepath.Join(t.TempDir(), "typescript.json")
	if code := cmdLangOracle(append(append([]string(nil), base...), "-out", out), &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "calls given alongside another calls oracle") {
		t.Fatalf("without -calls-from the calls file must be refused beside -scip (exit %d): %s", code, stderr.String())
	}

	stderr.Reset()
	out = filepath.Join(t.TempDir(), "typescript.json")
	if code := cmdLangOracle(append(append([]string(nil), base...), "-calls-from", "tool", "-out", out), &stdout, &stderr); code != 0 {
		t.Fatalf("-calls-from tool exit %d: %s", code, stderr.String())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if r.MetricTiers[langbench.MCallRecall] != "B" || r.MetricTiers[langbench.MCallPrecision] != "B" {
		t.Errorf("call metrics are the tool's tier B: %v", r.MetricTiers)
	}
	if r.MetricTiers[langbench.MDefMapping] != "A" || r.MetricTiers[langbench.MTypedRecall] != "A" {
		t.Errorf("definition mapping and typed edges stay the index's tier A: %v", r.MetricTiers)
	}
	if r.Counts[langbench.CntCallTruth] != 2 || r.Counts[langbench.CntCallTruthFound] != 1 {
		t.Errorf("call truth is the tool's two in-scope pairs, one found: %v", r.Counts)
	}
	if v := r.Metrics[langbench.MCallRecall]; v == nil || *v != 0.5 {
		t.Errorf("call_recall = %v, want 0.5", v)
	}
	var tools []string
	for _, o := range r.Oracles {
		tools = append(tools, o.Tool+":"+o.Tier)
	}
	if !strings.Contains(strings.Join(tools, " "), "fake-cg:B") {
		t.Errorf("oracles = %v", tools)
	}

	stderr.Reset()
	if code := cmdLangOracle([]string{"-dialect", "typescript", "-subject", subject, "-calls-from", "tool", "-out", out}, &stdout, &stderr); code != 2 {
		t.Errorf("-calls-from tool without -scip or a tool oracle: exit %d, want 2", code)
	}
}

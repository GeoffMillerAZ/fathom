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

// TestCmdLangParity (LB-F18, design §11 step 2): a port's dialect and the
// incumbent it retires, measured on the same subject against the same
// oracle; per-metric delta, the SourceKind diff, and the merge rule as the
// exit code.
func TestCmdLangParity(t *testing.T) {
	subject := t.TempDir()
	// Padding keeps the doc engine's throughput honest under -race: the
	// tenancy floor is 1 MB/s, and a few hundred bytes would be timed in
	// microseconds of fixed cost.
	pad := strings.Repeat("# a comment line that the TOML decoder skips quickly\n", 1200)
	src := "[project]\nname = \"demo\"\nversion = \"1.0\"\n\n[project.scripts]\ndemo = \"demo.cli:main\"\n\n[tool.black.sub]\nx = 1\n" + pad
	if err := os.WriteFile(filepath.Join(subject, "pyproject.toml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	if err := os.WriteFile(oracle, []byte(`{"tool":"tomllib","version":"3","tier":"C","definitions":[
		{"path":"pyproject.toml","name":"demo","line":1,"kind":"package"},
		{"path":"pyproject.toml","name":"demo","line":6,"kind":"route"},
		{"path":"pyproject.toml","name":"black","line":8,"kind":"section"},
		{"path":"vendor/other/pyproject.toml","name":"ghost","line":1,"kind":"package"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join("..", "..", "internal", "extractor", "dialects", "python-packaging", "dialect.json")
	base := []string{"-dialect", "python-packaging", "-manifest", manifest, "-incumbent", "tomlextract",
		"-subject", "fixture=" + subject, "-tool-oracle", "fixture=" + oracle, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}

	report := filepath.Join(t.TempDir(), "parity.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangParity(append(append([]string(nil), base...), "-target", "L1", "-out", report), &stdout, &stderr); code != 0 {
		t.Fatalf("merge case exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	t.Log(stdout.String())
	var p langbench.Parity
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if !p.Merge || p.DialectLevel != "L1" || p.Incumbent != "tomlextract" {
		t.Fatalf("parity = %+v", p)
	}
	var recall *langbench.MetricDelta
	for i := range p.Metrics {
		if p.Metrics[i].Metric == langbench.MSymRecall {
			recall = &p.Metrics[i]
		}
	}
	if recall == nil || recall.Dialect == nil || *recall.Dialect != 1 || recall.Delta == nil || *recall.Delta <= 0 || !recall.Gated {
		t.Fatalf("sym_recall delta = %+v", recall)
	}
	if len(p.KindsGained) == 0 || len(p.KindsLost) == 0 {
		t.Errorf("kinds diff empty: lost %v gained %v", p.KindsLost, p.KindsGained)
	}
	if !strings.Contains(stdout.String(), "verdict: MERGE") || !strings.Contains(stdout.String(), "kinds lost:") {
		t.Errorf("report text:\n%s", stdout.String())
	}

	// The same measurement against a target the dialect does not reach is
	// refused by exit code.
	stdout.Reset()
	if code := cmdLangParity(append(append([]string(nil), base...), "-target", "L3"), &stdout, &stderr); code != 1 {
		t.Fatalf("unmet target exit %d, want 1\n%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "target L3 not met") {
		t.Errorf("refusal does not say why:\n%s", stdout.String())
	}

	// Usage and operational errors exit 2.
	for name, args := range map[string][]string{
		"unknown incumbent": {"-dialect", "python-packaging", "-manifest", manifest, "-incumbent", "nope", "-subject", subject},
		"no dialect.json":   {"-dialect", "go", "-subject", subject},
		"no subject":        {"-dialect", "python-packaging", "-manifest", manifest},
		"oracle unnamed":    {"-dialect", "python-packaging", "-manifest", manifest, "-subject", "a=" + subject, "-subject", "b=" + subject, "-tool-oracle", oracle},
	} {
		if code := cmdLangParity(args, &stdout, &stderr); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
}

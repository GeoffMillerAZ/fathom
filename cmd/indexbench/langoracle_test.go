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

// fakeCtags writes an executable that answers --version as universal-ctags
// and otherwise prints tags — the tier-B oracle, hermetic.
func fakeCtags(t *testing.T, dir, tags string) string {
	t.Helper()
	tagsPath := filepath.Join(dir, "tags.json")
	if err := os.WriteFile(tagsPath, []byte(tags), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'Universal Ctags 6.2.1(fake), Copyright'; exit 0; fi\ncat '" + tagsPath + "'\n"
	p := filepath.Join(dir, "ctags")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCmdLangOracle (LB-F23, design §10): in-process extraction over the
// connector's file list, the ctags (B) and SCIP (A) oracles, §1.4 matching,
// tenancy, and one merged reading per dialect written two-space indented.
func TestCmdLangOracle(t *testing.T) {
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
	// Not a TypeScript file: never read, never scored.
	if err := os.WriteFile(filepath.Join(subject, "notes.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	ctags := fakeCtags(t, tools, strings.Join([]string{
		`{"_type": "tag", "name": "Greeter", "path": "greeter.ts", "line": 3, "kind": "class", "end": 8}`,
		`{"_type": "tag", "name": "greet", "path": "greeter.ts", "line": 4, "kind": "method", "scope": "Greeter", "end": 7}`,
		`{"_type": "tag", "name": "run", "path": "main.ts", "line": 3, "kind": "function", "end": 6}`,
		`{"_type": "tag", "name": "add", "path": "math.ts", "line": 1, "kind": "function", "end": 3}`,
		`{"_type": "tag", "name": "g", "path": "main.ts", "line": 4, "kind": "variable"}`,
	}, "\n")+"\n")
	scip := filepath.Join("..", "..", "internal", "scip", "testdata", "tsfixture", "index-no-display.scip")
	out := filepath.Join(t.TempDir(), "typescript.json")

	args := []string{"-dialect", "typescript", "-subject", subject, "-subject-name", "fixture/ts",
		"-ctags", ctags, "-scip", scip, "-out", out, "-date", "2026-09-27", "-commit", "abc1234", "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\n  \"level_met\": \"") {
		t.Errorf("reading is not two-space indented with level_met on one line:\n%s", raw)
	}
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("written reading does not validate: %v", err)
	}
	if r.Dialect != "typescript" || r.TargetLevel != "L4" || len(r.Subjects) != 1 || r.Subjects[0].Files != 3 {
		t.Fatalf("reading header wrong: %+v", r)
	}
	for _, m := range []string{langbench.MSymRecall, langbench.MSymPrecision, langbench.MCallRecall, langbench.MDefMapping} {
		if r.Metrics[m] == nil {
			t.Errorf("%s not measured", m)
		}
	}
	if r.MetricTiers[langbench.MSymRecall] != "B" || r.MetricTiers[langbench.MCallRecall] != "A" {
		t.Errorf("tiers = %v", r.MetricTiers)
	}
	if got := *r.Metrics[langbench.MSymRecall]; got != 1 {
		t.Errorf("sym_recall = %v, want 1 (4 of 4 definitions)", got)
	}
	if r.ExcludedByKind["ctags:variable"] != 1 {
		t.Errorf("excluded = %v", r.ExcludedByKind)
	}
	if r.Tenancy.PeakModulePages == 0 {
		t.Error("peak_module_pages not recorded for a wasm grammar")
	}

	// Re-measuring the same subject replaces it; a second subject merges.
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("rerun exit %d: %s", code, stderr.String())
	}
	args2 := append([]string(nil), args...)
	args2[5] = "fixture/ts2"
	args2 = append(args2[:8], args2[10:]...) // drop -scip for the second subject
	if code := cmdLangOracle(args2, &stdout, &stderr); code != 0 {
		t.Fatalf("second subject exit %d: %s", code, stderr.String())
	}
	raw, _ = os.ReadFile(out)
	_ = json.Unmarshal(raw, &r)
	if len(r.Subjects) != 2 {
		t.Fatalf("subjects = %d, want 2", len(r.Subjects))
	}

	// An absent ctags is disclosed, never faked: L1 unmeasured.
	out2 := filepath.Join(t.TempDir(), "ts.json")
	stderr.Reset()
	if code := cmdLangOracle([]string{"-dialect", "typescript", "-subject", subject, "-ctags", filepath.Join(tools, "nope"), "-out", out2}, &stdout, &stderr); code != 0 {
		t.Fatalf("no-ctags exit %d: %s", code, stderr.String())
	}
	raw, _ = os.ReadFile(out2)
	var r2 langbench.Reading
	_ = json.Unmarshal(raw, &r2)
	if r2.Metrics[langbench.MSymRecall] != nil || r2.LevelMet != langbench.LevelNone || len(r2.Oracles) == 0 || !strings.HasPrefix(r2.Oracles[0].Status, "unavailable") {
		t.Errorf("absent ctags: metrics %v level %s oracles %+v", r2.Metrics, r2.LevelMet, r2.Oracles)
	}

	// An injected grammar first met after the warm-up is re-timed warm,
	// never charged to throughput as a one-time compile.
	htmlSubj := t.TempDir()
	_ = os.WriteFile(filepath.Join(htmlSubj, "a.html"), []byte("<html><body><div id=\"a\"></div></body></html>\n"), 0o644)
	_ = os.WriteFile(filepath.Join(htmlSubj, "b.html"), []byte("<html><body><div id=\"b\"></div><script>function f() { return 1 }</script></body></html>\n"), 0o644)
	out3 := filepath.Join(t.TempDir(), "html.json")
	if code := cmdLangOracle([]string{"-dialect", "html", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "html", "dialect.json"),
		"-subject", htmlSubj, "-ctags", ctags, "-out", out3}, &stdout, &stderr); code != 0 {
		t.Fatalf("html exit %d: %s", code, stderr.String())
	}
	raw, _ = os.ReadFile(out3)
	if !strings.Contains(string(raw), "timed on a second, warm extraction") {
		t.Errorf("an injected grammar's first file was not re-timed:\n%s", raw)
	}

	// Usage errors.
	if code := cmdLangOracle([]string{"-dialect", "cobol", "-subject", subject, "-out", out2}, &stdout, &stderr); code != 2 {
		t.Errorf("unknown dialect exit %d, want 2", code)
	}
	empty := t.TempDir()
	if code := cmdLangOracle([]string{"-dialect", "typescript", "-subject", empty, "-ctags", ctags, "-out", out2}, &stdout, &stderr); code != 1 {
		t.Errorf("subject without dialect files exit %d, want 1", code)
	}
}

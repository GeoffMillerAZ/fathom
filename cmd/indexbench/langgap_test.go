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

func TestCmdLangGapJSON(t *testing.T) {
	root, r := loopFixture(t)
	r.Subjects[0].Counts["range_ok"] = 70
	langbench.Regrade(&r)
	b, _ := langbench.MarshalReading(r)
	path := filepath.Join(root, "docs/bench/readings/lang/sample.json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := mainRun([]string{"lang-gap", "-root", root, "-id", "sample", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d: %s", code, &errb)
	}
	var report langbench.GapReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Gaps) != 1 || report.Gaps[0].Class != "range mismatch" || report.Gaps[0].Count != 30 {
		t.Fatalf("%s", &out)
	}
	out.Reset()
	if code := mainRun([]string{"lang-gap", "-root", root, "-id", "sample"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "§13.2") || !strings.Contains(out.String(), "locations unavailable") {
		t.Fatalf("exit %d: %s %s", code, &out, &errb)
	}
}

func TestCmdLangNextNearMiss(t *testing.T) {
	root, r := loopFixture(t)
	r.Subjects[0].Counts["range_ok"] = 74
	langbench.Regrade(&r)
	b, _ := langbench.MarshalReading(r)
	if err := os.WriteFile(filepath.Join(root, "docs/bench/readings/lang/sample.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := mainRun([]string{"lang-next", "-root", root, "--near-miss", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d: %s", code, &errb)
	}
	var rows []langbench.NearMiss
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Metric != "range_agreement" || rows[0].GrammarCommit != r.Grammar.Commit {
		t.Fatalf("%s", &out)
	}
	out.Reset()
	if code := mainRun([]string{"lang-next", "-root", root, "--near-miss", "-margin", "0.001"}, &out, &errb); code != 0 || out.Len() != 0 {
		t.Fatalf("exit=%d %s %s", code, &out, &errb)
	}
	out.Reset()
	if code := mainRun([]string{"lang-next", "-root", root, "--near-miss", "-margin", "NaN"}, &out, &errb); code != 2 {
		t.Fatalf("invalid margin exit=%d", code)
	}
}

// Older refusal measurements predate files_with_error in Reading. Preserve
// their explicit counted measurement and its measured pin, with provenance.
func TestLangNearMissRefusalMeasurement(t *testing.T) {
	root, _ := loopFixture(t)
	plan := "- [-] Sample {id: LB-Q-sample-rest, disposition: refused, reason: \"no grammar under the 5% line: 6 of the 100 calibration files (6%: fixture) parse with an error under example/tree-sitter-sample abc1234\", trigger: \"a newer grammar\"}\n"
	if err := os.WriteFile(filepath.Join(root, "docs/plans/language-breadth.md"), []byte(plan), 0644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := cmdLangNearMiss(root, .01, true, &out, &errb); code != 0 {
		t.Fatalf("exit=%d: %s", code, &errb)
	}
	if !strings.Contains(out.String(), "parse_error_rate") || !strings.Contains(out.String(), "abc1234") || !strings.Contains(out.String(), "LB-Q-sample-rest") {
		t.Fatalf("%s", &out)
	}
}

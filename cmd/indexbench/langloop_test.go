package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

func loopFixture(t *testing.T) (string, langbench.Reading) {
	t.Helper()
	root := t.TempDir()
	b, err := os.ReadFile("../../internal/langbench/testdata/gap-reading.json")
	if err != nil {
		t.Fatal(err)
	}
	var r langbench.Reading
	if err = json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	r.Subjects[0].Counts = map[string]int{"oracle_defs": 100, "fathom_defs": 100, "matched": 100, "kind_equal": 100, "range_eligible": 100, "range_ok": 80, "callables": 100, "callables_signed": 100}
	r.Subjects[0].Families = map[string]string{"l1": "A"}
	r.Subjects[0].KindIntersection = []string{"class"}
	r.Subjects[0].Tenancy = &langbench.SubjectTenancy{ExtractMS: 100, ExtractCPUMS: 100, MaxFileMS: 1, PeakModulePages: 1}
	langbench.Regrade(&r)
	dir := filepath.Join(root, "docs/bench/readings/lang")
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	b, err = langbench.MarshalReading(r)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "sample.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
	q := []langbench.QueueEntry{{ID: "sample", Name: "Sample", Kind: "language", TargetLevel: "L1", Engine: "tags", GrammarSource: "sample", Licence: "MIT", Oracle: []string{"ctags"}, CalibrationSubjects: []string{"sample"}, PlanRow: "LB-Q-sample"}}
	b, _ = json.Marshal(q)
	if err = os.MkdirAll(filepath.Join(root, "docs/plans"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "docs/plans/language-breadth.queue.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
	return root, r
}

func TestLangLoopNoWriteAndRegression(t *testing.T) {
	for _, regress := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "regression"}[regress], func(t *testing.T) {
			root, r := loopFixture(t)
			path := filepath.Join(root, "docs/bench/readings/lang/sample.json")
			before, _ := os.ReadFile(path)
			var stages []string
			ops := langLoopOps{
				measure: func(e langbench.QueueEntry, subject, work string, w io.Writer) (string, error) {
					stages = append(stages, "probe+oracle")
					if regress {
						r.Subjects[0].Counts["range_ok"] = 70
					}
					b, _ := langbench.MarshalReading(r)
					p := filepath.Join(work, "reading.json")
					return p, os.WriteFile(p, b, 0644)
				},
				tracker: func(root string, r langbench.Reading) ([]byte, error) {
					stages = append(stages, "tracker")
					return []byte("tracker\n"), nil
				},
			}
			var out, errb bytes.Buffer
			code := runLangLoop([]string{"-root", root, "sample"}, &out, &errb, ops)
			want := 0
			if regress {
				want = 1
			}
			if code != want {
				t.Fatalf("code=%d want=%d: %s %s", code, want, &out, &errb)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("default wrote committed reading")
			}
			if _, err := os.Stat(filepath.Join(root, "docs/languages.md")); !os.IsNotExist(err) {
				t.Fatal("default wrote tracker")
			}
			if strings.Join(stages, ",") != "probe+oracle,tracker" || !strings.Contains(out.String(), "L1") || !strings.Contains(out.String(), "delta") {
				t.Fatalf("stages=%v output=%s", stages, &out)
			}
			out.Reset()
			errb.Reset()
			if code = runLangLoop([]string{"-root", root, "-write", "sample"}, &out, &errb, ops); code != want {
				t.Fatalf("write code=%d: %s", code, &errb)
			}
			after, _ = os.ReadFile(path)
			if regress && !bytes.Equal(before, after) {
				t.Fatal("regression was committed")
			}
			if !regress {
				if b, err := os.ReadFile(filepath.Join(root, "docs/languages.md")); err != nil || string(b) != "tracker\n" {
					t.Fatalf("tracker write: %q %v", b, err)
				}
			}
		})
	}
}

func TestLangLoopTask(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task not installed")
	}
	root, r := loopFixture(t)
	candidate := filepath.Join(t.TempDir(), "reading.json")
	before, _ := os.ReadFile(filepath.Join(root, "docs/bench/readings/lang/sample.json"))
	run := func() ([]byte, error) {
		cmd := exec.Command("task", "--silent", "lang:loop", "--", "sample")
		cmd.Dir = "../.."
		cmd.Env = append(os.Environ(), "LANG_ROOT="+root, "READING="+candidate, "WRITE=0", "SUBJECT=", "ORACLE_CONFIG=")
		return cmd.CombinedOutput()
	}
	if err := os.WriteFile(candidate, before, 0644); err != nil {
		t.Fatal(err)
	}
	var directOut, directErr bytes.Buffer
	if code := mainRun([]string{"lang-loop", "-root", root, "-reading", candidate, "sample"}, &directOut, &directErr); code != 0 {
		t.Fatalf("CLI exit %d: %s", code, &directErr)
	}
	if out, err := run(); err != nil {
		t.Fatalf("unchanged: %v\n%s", err, out)
	}
	r.Subjects[0].Counts["range_ok"] = 70
	b, _ := langbench.MarshalReading(r)
	if err := os.WriteFile(candidate, b, 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := run(); err == nil || !bytes.Contains(out, []byte("regression:")) {
		t.Fatalf("regression: %v\n%s", err, out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "docs/bench/readings/lang/sample.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("task wrote committed reading")
	}
}

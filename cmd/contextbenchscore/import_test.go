package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

const publiclocTestdata = "../../internal/contextbench/publicloc/testdata"

func importArgs(out string, extra ...string) []string {
	args := []string{
		"import",
		"-multi-swe", filepath.Join(publiclocTestdata, "multi-swe-go.jsonl"), "-multi-swe-revision", "fixture",
		"-swe-explore", filepath.Join(publiclocTestdata, "swe-explore-go.jsonl"), "-swe-explore-revision", "fixture",
		"-checkouts", filepath.Join(publiclocTestdata, "checkouts"),
		"-callers-dir", filepath.Join(publiclocTestdata, "callers"),
		"-out", out,
	}
	return append(args, extra...)
}

func TestCmdImport_DerivesALoadableCorpusDeterministically(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	if rc := run(importArgs(first)); rc != 0 {
		t.Fatalf("import exited %d", rc)
	}
	if rc := run(importArgs(second)); rc != 0 {
		t.Fatalf("second import exited %d", rc)
	}
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("two imports of the same pins produced different bytes")
	}

	corpus, err := publicloc.LoadCorpus(first)
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(corpus.Tasks) != 14 || !corpus.Selection.PreRegistered {
		t.Errorf("tasks=%d pre_registered=%v", len(corpus.Tasks), corpus.Selection.PreRegistered)
	}
	for _, p := range corpus.Sources {
		if p.License == "" || p.URL == "" || p.SHA256 == "" || p.Revision != "fixture" {
			t.Errorf("pin incomplete: %+v", p)
		}
	}
	tasks, err := contextbench.LoadTasks(first)
	if err != nil {
		t.Fatalf("runner cannot load the corpus: %v", err)
	}
	if len(tasks) != 14 {
		t.Errorf("runner loaded %d tasks", len(tasks))
	}
	if crit, err := contextbench.LoadCriterion(first); err != nil || crit.ID != "H2-2" {
		t.Errorf("criterion: %+v, %v", crit, err)
	}
}

func TestCmdImport_UnregisteredNAndOmittedCallers(t *testing.T) {
	out := filepath.Join(t.TempDir(), "tasks.json")
	if rc := run(importArgs(out, "-n", "3")); rc != 2 {
		t.Errorf("-n 3 without -unregistered-n: rc=%d, want 2", rc)
	}
	if rc := run(importArgs(out, "-n", "3", "-unregistered-n")); rc != 0 {
		t.Fatalf("-n 3 -unregistered-n: rc=%d", rc)
	}
	corpus, err := publicloc.LoadCorpus(out)
	if err != nil {
		t.Fatal(err)
	}
	if corpus.Selection.PreRegistered || corpus.Selection.Selected != 3 || corpus.Selection.ExcludedBeyondN != 2 {
		t.Errorf("selection = %+v", corpus.Selection)
	}

	// No -callers-dir: file and function tasks only, omission recorded.
	args := []string{
		"import",
		"-multi-swe", filepath.Join(publiclocTestdata, "multi-swe-go.jsonl"),
		"-checkouts", filepath.Join(publiclocTestdata, "checkouts"),
		"-out", out,
	}
	if rc := run(args); rc != 0 {
		t.Fatalf("import without callers: rc=%d", rc)
	}
	corpus, err = publicloc.LoadCorpus(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range corpus.Tasks {
		if task.Class == "find_callers" {
			t.Errorf("find_callers derived without loctruth output: %s", task.ID)
		}
	}
	if len(corpus.Instances) == 0 || len(corpus.Instances[0].Omitted) == 0 {
		t.Errorf("omission not recorded: %+v", corpus.Instances)
	}
}

func TestCmdImport_SelectOnlyAndClasses(t *testing.T) {
	out := filepath.Join(t.TempDir(), "selected.json")
	args := []string{
		"import", "-select-only",
		"-multi-swe", filepath.Join(publiclocTestdata, "multi-swe-go.jsonl"),
		"-swe-explore", filepath.Join(publiclocTestdata, "swe-explore-go.jsonl"),
		"-out", out,
	}
	if rc := run(args); rc != 0 {
		t.Fatalf("-select-only: rc=%d", rc)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var sel struct {
		Selection publicloc.SelectionReport `json:"selection"`
		Instances []struct {
			InstanceID string `json:"instance_id"`
			RepoURL    string `json:"repo_url"`
			BaseCommit string `json:"base_commit"`
			Checkout   string `json:"checkout"`
			Service    string `json:"service"`
		} `json:"instances"`
	}
	if err := json.Unmarshal(raw, &sel); err != nil {
		t.Fatal(err)
	}
	if sel.Selection.Selected != 5 || len(sel.Instances) != 5 || sel.Instances[0].Checkout != "example-org__gadgets-1" || sel.Instances[0].RepoURL == "" || sel.Instances[0].Service == "" {
		t.Errorf("selection list = %+v", sel)
	}

	corpusPath := filepath.Join(t.TempDir(), "tasks.json")
	if rc := run(importArgs(corpusPath, "-classes", "file_localization,find_callers")); rc != 0 {
		t.Fatalf("-classes: rc=%d", rc)
	}
	corpus, err := publicloc.LoadCorpus(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Tasks) != 9 || !reflect.DeepEqual(corpus.Selection.Classes, []string{"file_localization", "find_callers"}) {
		t.Errorf("classes filter: %d tasks, classes=%v", len(corpus.Tasks), corpus.Selection.Classes)
	}
	for _, task := range corpus.Tasks {
		if task.Class == "function_localization" {
			t.Errorf("filtered class present: %s", task.ID)
		}
	}
	if corpus.Instances[0].Tasks != 2 {
		t.Errorf("per-instance task count not recomputed: %+v", corpus.Instances[0])
	}
	if rc := run(importArgs(corpusPath, "-classes", "bogus")); rc != 2 {
		t.Errorf("unknown class: rc=%d, want 2", rc)
	}
}

func TestCmdImport_FailsLoudly(t *testing.T) {
	out := filepath.Join(t.TempDir(), "tasks.json")
	// Called directly as well as through run's dispatch: several packages
	// declare a `run`, and the verify gate's reachability must see this
	// test reach cmdImport without resolving through that shared name.
	if rc := cmdImport([]string{"-out", out, "-checkouts", "x"}); rc != 2 {
		t.Errorf("no source (direct): rc=%d, want 2", rc)
	}
	if rc := run([]string{"import", "-out", out, "-checkouts", "x"}); rc != 2 {
		t.Errorf("no source: rc=%d, want 2", rc)
	}
	if rc := run([]string{"import", "-multi-swe", "x.jsonl"}); rc != 2 {
		t.Errorf("no -checkouts/-out: rc=%d, want 2", rc)
	}
	if rc := run(importArgs(out, "-checkouts", t.TempDir())); rc != 1 {
		t.Errorf("missing checkouts: rc=%d, want 1", rc)
	}
	if rc := run(importArgs(out, "-callers-dir", t.TempDir())); rc != 1 {
		t.Errorf("missing loctruth output: rc=%d, want 1", rc)
	}
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	if err := os.WriteFile(bad, []byte(`{"org":"o","repo":"r","number":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := run([]string{"import", "-multi-swe", bad, "-checkouts", t.TempDir(), "-out", out}); rc != 1 {
		t.Errorf("malformed snapshot: rc=%d, want 1", rc)
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("a failed import wrote a corpus")
	}
}

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitFixtureSubject creates a local, self-contained go module git repo (one
// commit) with a passing and a failing test, so cmdRun's oracle step has
// something real (but tiny, local, no network) to run.
func gitFixtureSubject(t *testing.T) (repoPath, commit string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("go.mod", "module scenariobenchsubject\n\ngo 1.21\n")
	write("thing.go", "package thing\n\nfunc Add(a, b int) int { return a + b }\n")
	write("thing_test.go", "package thing\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

func writeCorpusFixture(t *testing.T, dir, repoURL, commit string) string {
	t.Helper()
	corpus := `{
  "arms": {
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [{
    "id": "T1",
    "category": "bug_fix",
    "prompt": "fix it",
    "derivation": "synthetic fixture for scenariobench's own tests",
    "subject": {"repo_url": "` + repoURL + `", "base_commit": "` + commit + `"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"fail_to_pass": ["TestPass"]},
    "predictions": {"control": "no-effect (this is the control)"}
  }],
  "criterion": {"id": "C1", "treatment": "control", "control": "control", "min_trials": 1}
}`
	path := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(path, []byte(corpus), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCmdPlan_NoSpend(t *testing.T) {
	dir := t.TempDir()
	repoURL, commit := gitFixtureSubject(t)
	corpusPath := writeCorpusFixture(t, dir, repoURL, commit)
	cellsDir := filepath.Join(dir, "cells")

	rc := run([]string{"plan", "-corpus", corpusPath, "-cells", cellsDir, "-harness-version", "test-1"})
	if rc != 0 {
		t.Fatalf("plan exited %d", rc)
	}
	// plan must not have created any subject clone or session artifact —
	// it inspects the corpus and the cell store only.
	if _, err := os.Stat(filepath.Join(dir, "workspace")); !os.IsNotExist(err) {
		t.Errorf("plan should spend nothing and create no workspace, but found one (err=%v)", err)
	}
}

func TestCmdPlan_MissingFlags(t *testing.T) {
	if rc := run([]string{"plan"}); rc != 2 {
		t.Errorf("plan with no -corpus: exit %d, want 2", rc)
	}
}

func TestCmdRun_FakeHarnessControlArm(t *testing.T) {
	dir := t.TempDir()
	repoURL, commit := gitFixtureSubject(t)
	corpusPath := writeCorpusFixture(t, dir, repoURL, commit)
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	streamsDir := repoRelative(t, "testdata", "scenariobench", "streams")

	rc := run([]string{
		"run",
		"-corpus", corpusPath,
		"-root", root,
		"-cells", cellsDir,
		"-raw-dir", rawDir,
		"-run-id", "run-1",
		"-harness-version", "fake-1",
		"-harness", "fake",
		"-fake-streams-dir", streamsDir,
		"-fake-streams", `{"T1/control":"single-session.jsonl"}`,
	})
	if rc != 0 {
		t.Fatalf("run exited %d, want 0 (the fixture's oracle test passes)", rc)
	}

	entries, err := os.ReadDir(cellsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected at least one cell filed under %s: err=%v entries=%v", cellsDir, err, entries)
	}

	// Running it again must borrow rather than re-run (the second run
	// still exits 0 and needs no fake stream mapping since it never calls
	// the harness).
	rc2 := run([]string{
		"run", "-corpus", corpusPath, "-root", root, "-cells", cellsDir, "-raw-dir", rawDir,
		"-run-id", "run-2", "-harness-version", "fake-1", "-harness", "fake", "-fake-streams-dir", streamsDir,
	})
	if rc2 != 0 {
		t.Fatalf("second run exited %d, want 0 (should borrow)", rc2)
	}
}

func TestCmdScore_RecomputesReadings(t *testing.T) {
	dir := t.TempDir()
	cellPath := filepath.Join(dir, "cell.json")
	raw, _ := json.Marshal(map[string]any{
		"task_id":  "T1",
		"arm":      "control",
		"cell_key": "k1",
		"run_id":   "run-1",
		"sessions": []map[string]any{
			{"index": 0, "role": "single", "total_tokens": 42, "turns": 3, "cost_usd": 0.01, "payload_bytes": 100},
		},
		"oracle":   map[string]any{"passed": true},
		"readings": map[string]any{},
	})
	if err := os.WriteFile(cellPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	rc := run([]string{"score", "-in", cellPath})
	if rc != 0 {
		t.Fatalf("score exited %d", rc)
	}
	out, err := os.ReadFile(cellPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"total_tokens": 42`) {
		t.Errorf("expected recomputed readings to include total_tokens=42: %s", out)
	}
	if !strings.Contains(string(out), `"success": true`) {
		t.Errorf("expected success to be recomputed from oracle.passed: %s", out)
	}
}

func TestCmdScore_MissingIn(t *testing.T) {
	if rc := run([]string{"score"}); rc != 2 {
		t.Errorf("score with no -in: exit %d, want 2", rc)
	}
}

func TestCmdLedger_MissingFlags(t *testing.T) {
	if rc := run([]string{"ledger"}); rc != 2 {
		t.Errorf("ledger with no flags: exit %d, want 2", rc)
	}
}

// repoRelative resolves a path relative to the fathom repo root, walking up
// from the test's working directory (cmd/scenariobench) — cmd tests run
// with cwd set to the package dir, two levels below the repo root.
func repoRelative(t *testing.T, parts ...string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(wd, "..", "..")
	return filepath.Join(append([]string{root}, parts...)...)
}

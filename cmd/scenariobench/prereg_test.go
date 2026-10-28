package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// BX-62 / D2 / D12 / D13: `run -prereg` refuses, in D13's order, a prereg
// outside <top>/docs/bench/prereg/, one LoadPrereg rejects, one that
// differs from HEAD, a dirty tree, a -root that is not the prereg's
// repository, and a stored cell filed under another prereg (or none);
// -rescore-lost is refused outright. Every scratch repository lives in
// t.TempDir(), never this repository, and every git call scrubs the
// environment (the 2026-09-14 hook incident, gitenv.go).

const preregTestID = "localization-v1"

// preregGit runs git in dir with the scrubbed environment, a fixed identity
// and, when date is set, that author and committer date.
func preregGit(t *testing.T, dir, date string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(scenariobench.GitEnv(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
	)
	if date != "" {
		cmd.Env = append(cmd.Env, "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func preregFixtureBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(repoRelative(t, "internal", "benchprep", "testdata", "prereg", preregTestID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// preregWithID returns the fixture prereg re-identified as id, for a second
// valid prereg file.
func preregWithID(t *testing.T, id string) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(preregFixtureBytes(t), &m); err != nil {
		t.Fatal(err)
	}
	m["id"] = id
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func preregWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func preregSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// preregRepoFixture builds a scratch repository holding
// docs/bench/prereg/localization-v1.json, committed at commitDate, with
// workspace/ ignored as in this repository. It returns the repository's
// top level and the prereg's path.
func preregRepoFixture(t *testing.T, commitDate string) (top, prereg string) {
	t.Helper()
	top = filepath.Join(t.TempDir(), "prereg-repo")
	if err := os.MkdirAll(top, 0o755); err != nil {
		t.Fatal(err)
	}
	preregGit(t, top, "", "init", "-q", "-b", "main")
	preregWrite(t, filepath.Join(top, ".gitignore"), []byte("workspace/\n"))
	preregWrite(t, filepath.Join(top, "README.md"), []byte("scratch\n"))
	prereg = filepath.Join(top, "docs", "bench", "prereg", preregTestID+".json")
	preregWrite(t, prereg, preregFixtureBytes(t))
	preregGit(t, top, commitDate, "add", "-A")
	preregGit(t, top, commitDate, "commit", "-q", "-m", "prereg")
	return top, prereg
}

// preregRunFixture is one corpus with two tasks (T1, T2) on the control arm
// and fake streams for both; cells and raw transcripts live outside the
// prereg repository so a run never dirties it.
type preregRunFixture struct {
	dir, corpus, cells, raw, readme, ledger string
}

func newPreregRunFixture(t *testing.T) preregRunFixture {
	t.Helper()
	dir := t.TempDir()
	repoURL, commit := gitFixtureSubject(t)
	f := preregRunFixture{
		dir:    dir,
		corpus: writeTwoTaskCorpusFixture(t, dir, repoURL, commit),
		cells:  filepath.Join(dir, "cells"),
		raw:    filepath.Join(dir, "raw"),
		readme: filepath.Join(dir, "README.md"),
		ledger: filepath.Join(dir, "ledger.jsonl"),
	}
	preregWrite(t, f.readme, []byte("# Bench\n\n## Scenario runs\n\n"+
		scenariobench.ScenarioLedgerBeginMarker+"\n\n"+scenariobench.ScenarioLedgerEndMarker+"\n"))
	return f
}

// run runs the fixture's corpus under root ("" for a root of its own).
func (f preregRunFixture) run(t *testing.T, root, runID string, extra ...string) int {
	t.Helper()
	args := []string{
		"run",
		"-corpus", f.corpus, "-cells", f.cells, "-raw-dir", f.raw,
		"-run-id", runID, "-harness-version", "fake-1", "-harness", "fake",
		"-fake-streams-dir", repoRelative(t, "testdata", "scenariobench", "streams"),
		"-fake-streams", `{"T1/control":"single-session.jsonl","T2/control":"single-session.jsonl"}`,
	}
	if root == "" {
		// A plain run still gets its own root: PrepareSubject clones under
		// <root>/workspace, never this package's directory.
		root = filepath.Join(f.dir, "fathom-root")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return run(append(append(args, "-root", root), extra...))
}

func (f preregRunFixture) fileLedger(t *testing.T, runID string) int {
	t.Helper()
	return run([]string{
		"ledger",
		"-corpus", f.corpus, "-cells", f.cells, "-ledger", f.ledger, "-readme", f.readme,
		"-run-id", runID, "-harness-version", "fake-1",
	})
}

func (f preregRunFixture) storedCells(t *testing.T) map[string]scenariobench.CellResult {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(f.cells, "*", "t*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]scenariobench.CellResult{}
	for _, p := range paths {
		out[p] = readStoredCell(t, p)
	}
	return out
}

// editStoredCells rewrites every stored cell's JSON through edit, keyed by
// task id.
func (f preregRunFixture) editStoredCells(t *testing.T, edit func(task string, m map[string]any)) {
	t.Helper()
	for p, c := range f.storedCells(t) {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		edit(c.TaskID, m)
		out, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// preregRefused runs fn, wants exit 2, and returns its stderr.
func preregRefused(t *testing.T, fn func() int) string {
	t.Helper()
	rc := 0
	stderr := captureStderr(t, func() { rc = fn() })
	if rc != 2 {
		t.Fatalf("exit %d, want 2 (stderr: %s)", rc, stderr)
	}
	if strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("refused as a usage error, not by the check under test: %s", stderr)
	}
	return stderr
}

func assertNoSessionRan(t *testing.T, f preregRunFixture, runID string) {
	t.Helper()
	if entries, err := os.ReadDir(filepath.Join(f.raw, runID)); err == nil && len(entries) > 0 {
		t.Errorf("a refused run must start no session, but %s holds %d entries", filepath.Join(f.raw, runID), len(entries))
	}
}

func TestCmdRun_PreregRefusesAPathOutsideDocsBenchPrereg(t *testing.T) {
	top, _ := preregRepoFixture(t, "")
	outside := filepath.Join(top, preregTestID+".json")
	preregWrite(t, outside, preregFixtureBytes(t))
	preregGit(t, top, "", "add", "-A")
	preregGit(t, top, "", "commit", "-q", "-m", "a prereg outside docs/bench/prereg")
	f := newPreregRunFixture(t)
	stderr := preregRefused(t, func() int { return f.run(t, top, "run-1", "-prereg", outside) })
	if !strings.Contains(stderr, "docs/bench/prereg/") {
		t.Errorf("refusal should name docs/bench/prereg/: %s", stderr)
	}
	assertNoSessionRan(t, f, "run-1")
}

func TestCmdRun_PreregRefusesAFileThatDiffersFromHEAD(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		setup      func(t *testing.T, top, committed string) string
	}{
		{"modified", "modified", func(t *testing.T, top, committed string) string {
			preregWrite(t, committed, append(preregFixtureBytes(t), '\n'))
			return committed
		}},
		{"untracked", "untracked", func(t *testing.T, top, _ string) string {
			p := filepath.Join(top, "docs", "bench", "prereg", "untracked-v1.json")
			preregWrite(t, p, preregWithID(t, "untracked-v1"))
			return p
		}},
		{"ignored", "ignored", func(t *testing.T, top, _ string) string {
			preregWrite(t, filepath.Join(top, ".gitignore"), []byte("workspace/\ndocs/bench/prereg/ignored-v1.json\n"))
			preregGit(t, top, "", "commit", "-q", "-am", "ignore a prereg")
			p := filepath.Join(top, "docs", "bench", "prereg", "ignored-v1.json")
			preregWrite(t, p, preregWithID(t, "ignored-v1"))
			return p
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			top, committed := preregRepoFixture(t, "")
			prereg := tc.setup(t, top, committed)
			f := newPreregRunFixture(t)
			stderr := preregRefused(t, func() int { return f.run(t, top, "run-1", "-prereg", prereg) })
			if !strings.Contains(stderr, tc.want) || !strings.Contains(stderr, "HEAD") {
				t.Errorf("refusal should say the prereg is %s against HEAD: %s", tc.want, stderr)
			}
			if strings.Contains(stderr, "dirty") {
				t.Errorf("the file check runs before the tree check: %s", stderr)
			}
			assertNoSessionRan(t, f, "run-1")
		})
	}
}

func TestCmdRun_PreregRefusesADirtyTree(t *testing.T) {
	top, prereg := preregRepoFixture(t, "")
	preregWrite(t, filepath.Join(top, "README.md"), []byte("edited\n"))
	f := newPreregRunFixture(t)
	stderr := preregRefused(t, func() int { return f.run(t, top, "run-1", "-prereg", prereg) })
	if !strings.Contains(stderr, "dirty") || !strings.Contains(stderr, "README.md") {
		t.Errorf("refusal should name the dirty tree and its path: %s", stderr)
	}
	assertNoSessionRan(t, f, "run-1")
}

func TestCmdRun_PreregRefusesARootOutsideThePreregRepository(t *testing.T) {
	_, prereg := preregRepoFixture(t, "")
	other := filepath.Join(t.TempDir(), "fathom-root")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	f := newPreregRunFixture(t)
	stderr := preregRefused(t, func() int { return f.run(t, other, "run-1", "-prereg", prereg) })
	if !strings.Contains(stderr, "-root") {
		t.Errorf("refusal should name -root: %s", stderr)
	}
	assertNoSessionRan(t, f, "run-1")
}

func TestCmdRun_PreregRefusesRescoreLost(t *testing.T) {
	top, prereg := preregRepoFixture(t, "")
	f := newPreregRunFixture(t)
	stderr := preregRefused(t, func() int { return f.run(t, top, "run-1", "-prereg", prereg, "-rescore-lost") })
	if !strings.Contains(stderr, "-rescore-lost") {
		t.Errorf("refusal should name -rescore-lost: %s", stderr)
	}
}

func TestCmdRun_PreregStampsEveryCell(t *testing.T) {
	top, prereg := preregRepoFixture(t, "")
	f := newPreregRunFixture(t)
	if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
		t.Fatalf("run -prereg exited %d, want 0", rc)
	}
	cells := f.storedCells(t)
	if len(cells) != 2 {
		t.Fatalf("want 2 stored cells, got %d", len(cells))
	}
	want := preregSHA(t, prereg)
	for p, c := range cells {
		if c.PreregSHA256 != want {
			t.Errorf("%s: prereg_sha256 = %q, want %q", p, c.PreregSHA256, want)
		}
	}
}

func TestCmdRun_PreregNeverBorrowsAnotherPreregsCell(t *testing.T) {
	top, prereg := preregRepoFixture(t, "")
	sha := preregSHA(t, prereg)

	t.Run("unstamped", func(t *testing.T) {
		f := newPreregRunFixture(t)
		if rc := f.run(t, "", "run-1"); rc != 0 {
			t.Fatalf("plain run exited %d", rc)
		}
		stderr := preregRefused(t, func() int { return f.run(t, top, "run-2", "-prereg", prereg) })
		for _, want := range []string{"T1/control/t1", "T2/control/t1", "unstamped", "fresh -cells"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("refusal should contain %q: %s", want, stderr)
			}
		}
		assertNoSessionRan(t, f, "run-2")
	})
	t.Run("another prereg", func(t *testing.T) {
		f := newPreregRunFixture(t)
		if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
			t.Fatalf("run -prereg exited %d", rc)
		}
		f.editStoredCells(t, func(task string, m map[string]any) {
			if task == "T2" {
				m["prereg_sha256"] = strings.Repeat("0", 64)
			}
		})
		stderr := preregRefused(t, func() int { return f.run(t, top, "run-1", "-prereg", prereg) })
		if !strings.Contains(stderr, "T2/control/t1") || strings.Contains(stderr, "T1/control/t1") || !strings.Contains(stderr, strings.Repeat("0", 64)) {
			t.Errorf("refusal should name only the foreign cell and its sha: %s", stderr)
		}
	})
	t.Run("same prereg borrows", func(t *testing.T) {
		f := newPreregRunFixture(t)
		if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
			t.Fatalf("run -prereg exited %d", rc)
		}
		if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
			t.Fatalf("a resume under the same prereg borrows its own cells: exit %d", rc)
		}
		for p, c := range f.storedCells(t) {
			if c.PreregSHA256 != sha {
				t.Errorf("%s lost its stamp: %q", p, c.PreregSHA256)
			}
		}
	})
}

func TestCmdLedger_StampsPreregSHAOnEveryRow(t *testing.T) {
	t.Run("stamped", func(t *testing.T) {
		top, prereg := preregRepoFixture(t, "")
		f := newPreregRunFixture(t)
		if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
			t.Fatalf("run -prereg exited %d", rc)
		}
		if rc := f.fileLedger(t, "run-1"); rc == 2 {
			t.Fatalf("ledger exited 2")
		}
		rows, err := scenariobench.LoadScenarioLedger(f.ledger)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 2 {
			t.Fatalf("want 2 rows, got %d", len(rows))
		}
		want := preregSHA(t, prereg)
		for _, r := range rows {
			if r.PreregSHA256 != want {
				t.Errorf("%s/%s: prereg_sha256 = %q, want %q", r.TaskID, r.Arm, r.PreregSHA256, want)
			}
		}
	})
	t.Run("unstamped files as today", func(t *testing.T) {
		f := newPreregRunFixture(t)
		if rc := f.run(t, "", "run-1"); rc != 0 {
			t.Fatalf("plain run exited %d", rc)
		}
		if rc := f.fileLedger(t, "run-1"); rc == 2 {
			t.Fatalf("ledger exited 2")
		}
		raw, err := os.ReadFile(f.ledger)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "prereg_sha256") {
			t.Errorf("an unstamped run must file rows without the field:\n%s", raw)
		}
	})
}

func TestCmdLedger_RefusesCellsUnderMixedPreregs(t *testing.T) {
	top, prereg := preregRepoFixture(t, "")
	sha := preregSHA(t, prereg)
	other := strings.Repeat("0", 64)
	for _, tc := range []struct {
		name string
		edit func(task string, m map[string]any)
		want []string
	}{
		{"two shas", func(task string, m map[string]any) {
			if task == "T2" {
				m["prereg_sha256"] = other
			}
		}, []string{sha, other, "T1/control/t1", "T2/control/t1"}},
		{"stamped beside unstamped", func(task string, m map[string]any) {
			if task == "T2" {
				delete(m, "prereg_sha256")
			}
		}, []string{sha, "unstamped", "T2/control/t1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPreregRunFixture(t)
			if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
				t.Fatalf("run -prereg exited %d", rc)
			}
			f.editStoredCells(t, tc.edit)
			stderr := preregRefused(t, func() int { return f.fileLedger(t, "run-1") })
			for _, want := range tc.want {
				if !strings.Contains(stderr, want) {
					t.Errorf("refusal should contain %q: %s", want, stderr)
				}
			}
			if _, err := os.Stat(f.ledger); !os.IsNotExist(err) {
				t.Errorf("a refused filing must write no ledger (stat: %v)", err)
			}
		})
	}
}

func TestVoidAndBackfillCriteria_KeepPreregStamps(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	sha := strings.Repeat("a", 64)
	if err := scenariobench.AppendScenarioLedger(ledgerPath, scenariobench.ScenarioLedgerRow{
		RunID: "run-1", TaskID: "T1", Arm: "full", Trials: 1, Successes: 1, ModelID: "sonnet", PreregSHA256: sha,
	}); err != nil {
		t.Fatal(err)
	}
	readmePath := scenarioLedgerReadmeFixture(t, dir)
	cellsDir := filepath.Join(dir, "cells")
	key := scenariobench.ScenarioCellKey(scenariobench.CellInvariants{
		CorpusSHA256: "abc123", TaskID: "T1", Arm: "full", ModelID: "sonnet",
		HarnessVersion: "2.1.0", Scoring: "oracle-v1", PromptSubject: "x",
	})
	if err := scenariobench.StoreScenarioCell(cellsDir, scenariobench.CellResult{
		TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-1", Success: true, PreregSHA256: sha,
	}); err != nil {
		t.Fatal(err)
	}
	glob := filepath.Join(dir, "no-such-corpus", "*.json")
	if rc := run([]string{"ledger", "-backfill-criteria", "-ledger", ledgerPath, "-corpus-glob", glob}); rc != 0 {
		t.Fatalf("ledger -backfill-criteria exited %d", rc)
	}
	if rc := run([]string{
		"void", "-ledger", ledgerPath, "-readme", readmePath, "-cells", cellsDir, "-corpus-glob", glob,
		"-run-id", "run-1", "-task", "T1", "-arm", "full", "-reason", "instrument: a test voids this row",
	}); rc != 0 {
		t.Fatalf("void exited %d", rc)
	}
	rows, err := scenariobench.LoadScenarioLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].PreregSHA256 != sha || rows[0].InstrumentDefect == "" {
		t.Fatalf("void must keep the row's prereg stamp: %+v", rows)
	}
	cell, ok, err := scenariobench.BorrowScenarioCell(cellsDir, key, 1)
	if err != nil || !ok {
		t.Fatalf("cell gone after void: ok=%v err=%v", ok, err)
	}
	if cell.PreregSHA256 != sha {
		t.Errorf("void must keep the cell's prereg stamp, got %q", cell.PreregSHA256)
	}
}

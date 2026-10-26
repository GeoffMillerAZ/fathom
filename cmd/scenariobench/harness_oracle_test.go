package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func TestCmdRun_OracleHarness(t *testing.T) {
	dir := t.TempDir()
	repo, commit := gitFixtureSubject(t)
	path := writeCorpusFixture(t, dir, repo, commit)
	corpus, err := scenariobench.LoadScenarioCorpus(path)
	if err != nil {
		t.Fatal(err)
	}
	// Missing and ambiguous source pins must fail before creating run state.
	for _, sources := range [][]scenariobench.SourcePin{nil, {{Name: "one", Revision: "pin"}, {Name: "two", Revision: "pin"}}} {
		corpus.Sources = sources
		data, err := json.Marshal(corpus)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(dir, "refused-root")
		stderr := captureStderr(t, func() {
			captureStdout(t, func() {
				if rc := run([]string{"run", "-corpus", path, "-root", root, "-cells", filepath.Join(dir, "refused-cells"), "-raw-dir", filepath.Join(dir, "refused-raw"), "-run-id", "refused", "-trials", "3", "-harness", "oracle", "-harness-version", "oracle-1"}); rc != 2 {
					t.Errorf("%d pins: exit=%d", len(sources), rc)
				}
			})
		})
		if strings.Count(stderr, "oracle needs exactly one source pin") != 1 {
			t.Errorf("expected one preflight refusal: %s", stderr)
		}
		for _, name := range []string{"refused-root", "refused-cells", "refused-raw"} {
			if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
				t.Errorf("refused run created %s: %v", name, err)
			}
		}
	}
	corpus.Sources = []scenariobench.SourcePin{{Name: "fixture", Revision: "fixture-pin"}}
	corpus.Tasks[0].Category = scenariobench.CategoryLocalization
	corpus.Tasks[0].Oracle = scenariobench.Oracle{Localization: true}
	corpus.Tasks[0].Gold = scenariobench.Gold{Files: []string{"thing.go"}}
	data, _ := json.Marshal(corpus)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if rc := run([]string{"run", "-corpus", path, "-root", filepath.Join(dir, "root"), "-cells", filepath.Join(dir, "cells"), "-raw-dir", filepath.Join(dir, "raw"), "-run-id", "oracle-fixture", "-harness-version", "oracle-1", "-harness", "oracle"}); rc != 0 {
			t.Errorf("exit = %d", rc)
		}
	})
	if !strings.Contains(out, "T1/control/t1: PASS") {
		t.Fatalf("not PASS: %s", out)
	}
}

func TestCmdRun_HelpListsOracleHarness(t *testing.T) {
	help := captureStderr(t, func() { run([]string{"run", "-help"}) })
	if !strings.Contains(help, "oracle (verified gold patch or gold files, no spend)") || !strings.Contains(help, "claude|fake|null|oracle") {
		t.Fatalf("missing help: %s", help)
	}
}

// runOfflineFixture exercises the production cmd layer using only committed bytes.
func runOfflineFixture(t *testing.T, name string, localization bool, wantPass bool) {
	t.Helper()
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("FATHOM_DECIDE", "off")
	dir := t.TempDir()
	repo, _ := gitFixtureSubject(t)
	fixtureRoot := filepath.Join("..", "..", "internal", "scenariobench", "testdata", "dryrun")
	for _, file := range []string{"go.mod", "thing.go", "thing_test.go"} {
		data, err := os.ReadFile(filepath.Join(fixtureRoot, "subject", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, file), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = repo
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("add", ".")
	git("-c", "commit.gpgsign=false", "commit", "-qm", "dry-run fixture")
	path := writeCorpusFixture(t, dir, repo, git("rev-parse", "HEAD"))
	corpus, err := scenariobench.LoadScenarioCorpus(path)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := os.ReadFile(filepath.Join(fixtureRoot, "gold.patch"))
	if err != nil {
		t.Fatal(err)
	}
	tests, err := os.ReadFile(filepath.Join(fixtureRoot, "test.patch"))
	if err != nil {
		t.Fatal(err)
	}
	corpus.Sources = []scenariobench.SourcePin{{Name: "fixture", Revision: "fixture-pin"}}
	corpus.Tasks[0].Gold = scenariobench.Gold{Files: []string{"thing.go"}, PatchSHA: fmt.Sprintf("%x", sha256.Sum256(patch)), TestPatch: string(tests)}
	corpus.Tasks[0].Oracle = scenariobench.Oracle{FailToPass: []string{"TestFix"}, PassToPass: []string{"TestStable"}}
	if localization {
		corpus.Tasks[0].Category = scenariobench.CategoryLocalization
		corpus.Tasks[0].Oracle = scenariobench.Oracle{Localization: true}
	}
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	patchDir := filepath.Join(dir, "patches")
	if err := os.MkdirAll(patchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patchDir, corpus.Tasks[0].Gold.PatchSHA+".patch"), patch, 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		rc := run([]string{"run", "-corpus", path, "-root", filepath.Join(dir, "root"), "-cells", filepath.Join(dir, "cells"), "-raw-dir", filepath.Join(dir, "raw"), "-run-id", "fixture", "-harness-version", name + "-1", "-harness", name, "-gold-patch-dir", patchDir})
		want := 1
		if wantPass {
			want = 0
		}
		if rc != want {
			t.Errorf("exit=%d, want %d", rc, want)
		}
	})
	verdict := "FAIL"
	if wantPass {
		verdict = "PASS"
	}
	if !strings.Contains(out, "T1/control/t1: "+verdict) {
		t.Fatalf("not graded %s: %s", verdict, out)
	}
	cell := readStoredCell(t, findStoredCell(t, filepath.Join(dir, "cells")))
	if cell.InstrumentDefect != "" || cell.Success != wantPass || cell.Readings.TotalTurns != 0 || cell.Readings.TotalCostUSD != 0 {
		t.Fatalf("not a zero-turn measured cell: %+v", cell)
	}
}

func TestCmdRun_OracleHarnessF2(t *testing.T) { runOfflineFixture(t, "oracle", false, true) }

func TestCmdRun_OracleHarnessVersion(t *testing.T) {
	help := captureStderr(t, func() {
		if rc := run([]string{"run", "-corpus", "unused.json", "-run-id", "fixture", "-harness", "oracle", "-harness-version", "null-1"}); rc != 2 {
			t.Errorf("exit=%d", rc)
		}
	})
	if !strings.Contains(help, "-harness oracle requires -harness-version oracle-1") {
		t.Fatalf("mislabeled control not refused: %s", help)
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func TestCmdDryRun(t *testing.T) {
	dir := t.TempDir()
	repo, commit := gitFixtureSubject(t)
	corpusPath := writeCorpusFixture(t, dir, repo, commit)
	sha, err := scenariobench.CorpusSHA256(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	if err := os.MkdirAll(cellsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"null", "oracle", "wrong-patch"} {
		for trial := 1; trial <= 3; trial++ {
			c := scenariobench.CellResult{TaskID: "T1", Arm: "control", Trial: trial, HarnessVersion: name + "-1", CorpusSHA256: sha, Success: name == "oracle", Sessions: []scenariobench.SessionResult{{}}, RunID: "fixture"}
			data, _ := json.Marshal(c)
			if err := os.WriteFile(filepath.Join(cellsDir, name+string(rune('0'+trial))+".json"), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	args := []string{"dryrun", "-corpus", corpusPath, "-cells", cellsDir, "-run-id", "fixture"}
	out := captureStdout(t, func() {
		if rc := run(args); rc != 0 {
			t.Errorf("exit=%d", rc)
		}
	})
	if out != "T1: keep\n" {
		t.Fatalf("output=%q", out)
	}
	out = captureStdout(t, func() {
		if rc := run(append(args, "--json")); rc != 0 {
			t.Errorf("exit=%d", rc)
		}
	})
	var verdict []scenariobench.DryRunTaskVerdict
	if err := json.Unmarshal([]byte(out), &verdict); err != nil || len(verdict) != 1 || !verdict[0].Keep {
		t.Fatalf("JSON=%s: %v", out, err)
	}
	// An absent or incomplete store cannot report a healthy corpus.
	captureStderr(t, func() {
		if rc := run([]string{"dryrun", "-corpus", corpusPath, "-cells", filepath.Join(dir, "absent")}); rc != 2 {
			t.Errorf("absent store exit=%d", rc)
		}
	})
}

func TestCmdDryRunHelp(t *testing.T) {
	help := captureStderr(t, func() { run([]string{"dryrun", "-help"}) })
	for _, want := range []string{"null, oracle and wrong-patch", "3 trials", "-json", "controls may share a run id"} {
		if !strings.Contains(help, want) {
			t.Fatalf("missing %q: %s", want, help)
		}
	}
	usage := captureStderr(t, func() { run(nil) })
	if !strings.Contains(usage, "dryrun") {
		t.Fatalf("not discoverable: %s", usage)
	}
}

func TestCmdDryRunSelectArm(t *testing.T) {
	dir := t.TempDir()
	repo, commit := gitFixtureSubject(t)
	path := writeCorpusFixture(t, dir, repo, commit)
	corpus, err := scenariobench.LoadScenarioCorpus(path)
	if err != nil {
		t.Fatal(err)
	}
	corpus.Arms["wired"] = scenariobench.Arm{Wired: true, Instructions: "none"}
	corpus.Tasks[0].Predictions["wired"] = "not exercised by offline controls"
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := scenariobench.CorpusSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	if err := os.MkdirAll(cellsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"null", "oracle", "wrong-patch"} {
		for trial := 1; trial <= 3; trial++ {
			c := scenariobench.CellResult{TaskID: "T1", Arm: "control", Trial: trial, HarnessVersion: name + "-1", CorpusSHA256: sha, Success: name == "oracle", Sessions: []scenariobench.SessionResult{{}}}
			data, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cellsDir, name+string(rune('0'+trial))+".json"), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	out := captureStdout(t, func() {
		if rc := run([]string{"dryrun", "-corpus", path, "-cells", cellsDir, "-arm", "control"}); rc != 0 {
			t.Errorf("exit=%d", rc)
		}
	})
	if out != "T1: keep\n" {
		t.Fatalf("selected arm: %q", out)
	}
}

func TestCmdDryRunSharedRunID(t *testing.T) {
	dir := t.TempDir()
	repo, commit := gitFixtureSubject(t)
	path := writeCorpusFixture(t, dir, repo, commit)
	corpus, err := scenariobench.LoadScenarioCorpus(path)
	if err != nil {
		t.Fatal(err)
	}
	corpus.Sources = []scenariobench.SourcePin{{Name: "fixture", Revision: "fixture-pin"}}
	corpus.Tasks[0].Category = scenariobench.CategoryLocalization
	corpus.Tasks[0].Oracle = scenariobench.Oracle{Localization: true}
	corpus.Tasks[0].Gold = scenariobench.Gold{Files: []string{"thing.go"}}
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cells := filepath.Join(dir, "cells")
	for _, name := range []string{"null", "oracle", "wrong-patch"} {
		captureStdout(t, func() {
			rc := run([]string{"run", "-corpus", path, "-root", filepath.Join(dir, "root"), "-cells", cells, "-raw-dir", filepath.Join(dir, "raw"), "-trials", "3", "-run-id", "shared", "-harness", name, "-harness-version", name + "-1"})
			want := 1
			if name == "oracle" {
				want = 0
			}
			if rc != want {
				t.Errorf("%s exit=%d, want %d", name, rc, want)
			}
		})
	}
	out := captureStdout(t, func() {
		if rc := run([]string{"dryrun", "-corpus", path, "-cells", cells, "-run-id", "shared"}); rc != 0 {
			t.Errorf("dryrun exit=%d", rc)
		}
	})
	if out != "T1: keep\n" {
		t.Fatalf("shared controls: %q", out)
	}
}

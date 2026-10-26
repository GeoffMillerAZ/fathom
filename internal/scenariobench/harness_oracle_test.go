package scenariobench

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dryFixture(t *testing.T) (Corpus, []byte, string) {
	t.Helper()
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("FATHOM_DECIDE", "off")
	files := map[string]string{}
	for _, name := range []string{"go.mod", "thing.go", "thing_test.go"} {
		b, err := os.ReadFile(filepath.Join("testdata", "dryrun", "subject", name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(b)
	}
	repo, commit := gitFixtureRepoWithFiles(t, files)
	patch, err := os.ReadFile("testdata/dryrun/gold.patch")
	if err != nil {
		t.Fatal(err)
	}
	tests, err := os.ReadFile("testdata/dryrun/test.patch")
	if err != nil {
		t.Fatal(err)
	}
	subject := Subject{RepoURL: repo, BaseCommit: commit}
	sessions := Sessions{Mode: ModeSingle, Count: 1}
	return Corpus{Sources: []SourcePin{{Name: "fixture", Revision: "fixture-pin"}}, Arms: map[string]Arm{"control": {Instructions: "none"}}, Tasks: []Task{
		{ID: "F1", Category: CategoryLocalization, Subject: subject, Sessions: sessions, Oracle: Oracle{Localization: true}, Gold: Gold{Files: []string{"thing.go"}}},
		{ID: "F2", Category: CategoryBugFix, Subject: subject, Sessions: sessions, Oracle: Oracle{FailToPass: []string{"TestFix"}, PassToPass: []string{"TestStable"}}, Gold: Gold{Files: []string{"thing.go"}, PatchSHA: fmt.Sprintf("%x", sha256.Sum256(patch)), TestPatch: string(tests)}},
	}}, patch, repo
}

func runDryFixture(t *testing.T, corpus Corpus, h Harness) RunExternalCorpusResult {
	t.Helper()
	sha, err := CanonicalCorpusSHA256(corpus)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	result, err := RunExternalCorpus(context.Background(), RunExternalCorpusOptions{Root: root, Corpus: corpus, CorpusSHA256: sha, Harness: h, HarnessVersion: h.Version(), RunID: "dry-fixture", RawDir: filepath.Join(root, "raw"), CellsDir: filepath.Join(root, "cells")})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestNewOracleHarness(t *testing.T) {
	corpus, patch, repo := dryFixture(t)
	calls := 0
	cache := filepath.Join(t.TempDir(), "workspace", "gold-patches")
	h := NewOracleHarness(corpus.Sources[0], cache, func(_ context.Context, source SourcePin, id string) ([]byte, error) {
		calls++
		if source != corpus.Sources[0] || id != "F2" {
			t.Fatalf("unexpected fetch: %+v %s", source, id)
		}
		return patch, nil
	})
	if h.Name() != "oracle" || h.Version() != "oracle-1" {
		t.Fatal("wrong identity")
	}
	result := runDryFixture(t, corpus, h)
	if len(result.Lost) != 0 || len(result.Cells) != 2 {
		t.Fatalf("not measured: %+v", result)
	}
	for _, cell := range result.Cells {
		if !cell.Success || !cell.Oracle.Passed || CellLost(cell.Sessions, h) != "" {
			t.Fatalf("oracle did not pass: %+v", cell)
		}
		if len(cell.Sessions) != 1 || cell.Sessions[0].Turns != 0 || cell.Readings.TotalCostUSD != 0 {
			t.Fatal("not zero-turn and free")
		}
		if cell.TaskID == "F1" && (!cell.Oracle.Localization.HitAt1 || len(ParseAnswerFiles(cell.Sessions[0].FinalAnswer)) != 1) {
			t.Fatal("gold file not ranked first (therefore hit@3 != 1)")
		}
		if cell.TaskID == "F2" && (!cell.Oracle.FailToPassOK || !cell.Oracle.PassToPassOK || cell.Oracle.Calibration == nil || cell.Oracle.Calibration.NotMeasurable != "") {
			t.Fatalf("hidden tests did not pass: %+v", cell.Oracle)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d", calls)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache: %v %v", entries, err)
	}
	content, err := os.ReadFile(filepath.Join(repo, "thing.go"))
	if err != nil || strings.Contains(string(content), "return a + b") {
		t.Fatal("gold patch escaped into source repository")
	}
}

func TestOracleHarnessRejectsUnverifiedPatch(t *testing.T) {
	corpus, _, _ := dryFixture(t)
	h := NewOracleHarness(corpus.Sources[0], filepath.Join(t.TempDir(), "workspace", "gold-patches"), func(context.Context, SourcePin, string) ([]byte, error) { return []byte("corrupt"), nil })
	wt := Worktree{Path: t.TempDir()}
	_, err := RunSessions(context.Background(), h, corpus.Tasks[1], "control", corpus.Arms["control"], wt, RunOptions{RawDir: t.TempDir(), Trial: 1})
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("want verification refusal, got %v", err)
	}
}

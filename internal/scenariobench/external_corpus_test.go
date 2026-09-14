package scenariobench

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/indexbench"
)

// fauxtownGitFixtureRepo stages one fauxtown fixture Go service
// (indexbench.StageFauxtownModule) into a fresh local git repository and
// commits it — entirely local, so RunExternalCorpus's own
// PrepareSubject/RunSessions/RunOracle pipeline is testable with no
// network and no real model call (VB-25's own "so the test runs
// offline"), mirroring subject_test.go's gitFixtureRepo for a real
// fixture module rather than a bare marker file.
func fauxtownGitFixtureRepo(t *testing.T, svc string) (repoPath, commit string) {
	t.Helper()
	dir := t.TempDir()
	if _, err := indexbench.StageFauxtownModule(svc, dir); err != nil {
		t.Fatalf("StageFauxtownModule(%s): %v", svc, err)
	}
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	run("add", "-A")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

// lcaTestCorpus builds a single-task, two-arm corpus over a local
// fauxtown ledger-service clone: "fathom" (unwired, but declaring an MCP
// "fathom" server so the instrument-truth gate has a server to check —
// see RunExternalCorpus's own doc comment on why an unwired arm is enough
// here) and "control" (no MCP at all). Both are read-only Localization
// tasks — Gold.Files matches lca/fixture.jsonl's own first record so the
// two stay in sync for a reader comparing them.
func lcaTestCorpus(t *testing.T, repoPath, commit string) Corpus {
	t.Helper()
	task := Task{
		ID:       "fauxtown__ledger-service-1",
		Category: CategoryLocalization,
		Prompt:   "The following issue was filed against fauxtown/ledger-service. Name every file you believe the fix will need to change, most likely first. Do not edit anything.\n\nIssue title: Balance returns nil error for an empty account ID",
		Subject: Subject{
			RepoURL:    repoPath,
			BaseCommit: commit,
			Service:    "lca-fauxtown-ledger-service",
		},
		Sessions:   Sessions{Mode: ModeSingle, Count: 1, MaxTurns: DefaultLCAMaxTurns},
		Oracle:     Oracle{Localization: true},
		Gold:       Gold{Files: []string{"errors.go", "ledger.go"}},
		Derivation: "test fixture for TestRunExternalCorpus",
		Predictions: map[string]string{
			LCAArmFathom:  "wins",
			LCAArmControl: "loses",
		},
	}
	return Corpus{
		Version:     1,
		Description: "RunExternalCorpus dry-mode test corpus",
		Arms: map[string]Arm{
			LCAArmFathom: {
				Wired:        false,
				MCP:          map[string]any{"fathom": map[string]any{"command": "true"}},
				AllowedTools: []string{"Read", "Grep", "Bash"},
				Instructions: "plain",
				Surface:      "fathom",
				Prediction:   "wins",
			},
			LCAArmControl: {
				Wired:        false,
				AllowedTools: []string{"Read", "Grep", "Bash"},
				Instructions: "plain",
				Surface:      "",
				Prediction:   "loses",
			},
		},
		Tasks: []Task{task},
		Criterion: Criterion{
			ID: LCACriterionID, Treatment: LCAArmFathom, Control: LCAArmControl, MinTrials: 1,
			Registered: "test",
		},
	}
}

func TestRunExternalCorpus_DryMode_LedgerRow(t *testing.T) {
	repoPath, commit := fauxtownGitFixtureRepo(t, "ledger-service")
	corpus := lcaTestCorpus(t, repoPath, commit)
	corpusSHA, err := CanonicalCorpusSHA256(corpus)
	if err != nil {
		t.Fatalf("CanonicalCorpusSHA256: %v", err)
	}

	root := t.TempDir()
	cellsDir := filepath.Join(root, "cells")
	rawDir := filepath.Join(root, "raw")
	ledgerPath := filepath.Join(root, "ledger.jsonl")

	harness := &FakeHarness{
		StreamsDir: streamsDir,
		Streams: map[string]string{
			"fauxtown__ledger-service-1/" + LCAArmFathom:  "lca-fathom-hit.jsonl",
			"fauxtown__ledger-service-1/" + LCAArmControl: "lca-control-miss.jsonl",
		},
	}

	opts := RunExternalCorpusOptions{
		Root: root, Corpus: corpus, CorpusSHA256: corpusSHA, Harness: harness,
		RunID: "dry-1", Model: "fake", RawDir: rawDir, CellsDir: cellsDir, Trials: 1,
		ModelID: "fake-model", HarnessVersion: "fake-1", Scoring: "oracle-v1",
		LedgerPath: ledgerPath, Commit: "testcommit",
	}

	result, err := RunExternalCorpus(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunExternalCorpus: %v", err)
	}
	if len(result.Lost) != 0 {
		t.Fatalf("expected no lost cells, got %+v", result.Lost)
	}
	if len(result.Cells) != 2 {
		t.Fatalf("expected 2 cells (one per arm), got %d", len(result.Cells))
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 ledger rows (one per arm), got %d", len(result.Rows))
	}

	rowsByArm := map[string]ScenarioLedgerRow{}
	for _, r := range result.Rows {
		rowsByArm[r.Arm] = r
	}
	fathomRow, ok := rowsByArm[LCAArmFathom]
	if !ok {
		t.Fatal("expected a fathom row")
	}
	controlRow, ok := rowsByArm[LCAArmControl]
	if !ok {
		t.Fatal("expected a control row")
	}

	// Instrument truth: the fathom arm's MCP server reported connected
	// AND was actually called (SurfaceCalls > 0) — never merely wired.
	if fathomRow.SurfaceUnused {
		t.Error("fathom row reports SurfaceUnused, but the fixture transcript calls mcp__fathom__localize")
	}
	if fathomRow.Readings.SurfaceCalls == 0 {
		t.Error("expected fathom row's SurfaceCalls > 0")
	}
	if fathomRow.Readings.HooksFired["PreToolUse"] == 0 {
		t.Error("expected the fathom row to carry the fixture's own PreToolUse hook_response count")
	}

	// The files oracle's own headline numbers (VB-25).
	if fathomRow.Readings.LocalizationN != 1 {
		t.Fatalf("fathom LocalizationN = %d, want 1", fathomRow.Readings.LocalizationN)
	}
	if fathomRow.Readings.LocalizationHitAt1Rate != 1 {
		t.Errorf("fathom LocalizationHitAt1Rate = %v, want 1 (top answer is ledger.go)", fathomRow.Readings.LocalizationHitAt1Rate)
	}
	if fathomRow.Readings.LocalizationHitAt5Rate != 1 {
		t.Errorf("fathom LocalizationHitAt5Rate = %v, want 1", fathomRow.Readings.LocalizationHitAt5Rate)
	}
	if fathomRow.Readings.LocalizationFileRecallMean != 1 {
		t.Errorf("fathom file_recall = %v, want 1 (both gold files named)", fathomRow.Readings.LocalizationFileRecallMean)
	}
	if fathomRow.Successes != 1 {
		t.Errorf("fathom Successes = %d, want 1", fathomRow.Successes)
	}

	if controlRow.Readings.LocalizationN != 1 {
		t.Fatalf("control LocalizationN = %d, want 1", controlRow.Readings.LocalizationN)
	}
	if controlRow.Readings.LocalizationHitAt1Rate != 0 {
		t.Errorf("control LocalizationHitAt1Rate = %v, want 0 (top answer is payment.go)", controlRow.Readings.LocalizationHitAt1Rate)
	}
	if controlRow.Successes != 0 {
		t.Errorf("control Successes = %d, want 0", controlRow.Successes)
	}
	if controlRow.Readings.SurfaceCalls != 0 {
		t.Errorf("control SurfaceCalls = %d, want 0 (no surface declared)", controlRow.Readings.SurfaceCalls)
	}

	// PerTrial carries the per-cell Localization reading verbatim.
	if len(fathomRow.PerTrial) != 1 || fathomRow.PerTrial[0].Localization == nil {
		t.Fatalf("expected fathom row's PerTrial to carry the Localization reading, got %+v", fathomRow.PerTrial)
	}

	// The row was actually filed to the ledger, with the same field names
	// docs/bench/scenarios.md documents.
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("reading ledger: %v", err)
	}
	if !strings.Contains(string(raw), `"localization_hit_at_1":1`) {
		t.Errorf("ledger JSONL missing localization_hit_at_1: %s", raw)
	}
	if !strings.Contains(string(raw), `"file_recall":1`) {
		t.Errorf("ledger JSONL missing file_recall: %s", raw)
	}
	filedRows, err := LoadScenarioLedger(ledgerPath)
	if err != nil {
		t.Fatalf("LoadScenarioLedger: %v", err)
	}
	if len(filedRows) != 2 {
		t.Fatalf("expected 2 filed rows, got %d", len(filedRows))
	}

	// A second run of the SAME corpus/invariants borrows every cell
	// (content-addressed store) rather than re-running, and produces a
	// byte-identical set of readings.
	result2, err := RunExternalCorpus(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunExternalCorpus (2nd): %v", err)
	}
	// The second run's rows differ only in Borrowed provenance
	// (source_run_id); strip that noise by comparing the readings each
	// row carries instead of the whole row.
	if len(result2.Rows) != 2 {
		t.Fatalf("expected 2 rows on the second (borrowing) run, got %d", len(result2.Rows))
	}
	for i := range result.Rows {
		a, err := json.Marshal(result.Rows[i].Readings)
		if err != nil {
			t.Fatalf("marshaling readings: %v", err)
		}
		b, err := json.Marshal(result2.Rows[i].Readings)
		if err != nil {
			t.Fatalf("marshaling readings: %v", err)
		}
		if string(a) != string(b) {
			t.Errorf("row %d readings differ between an original and a borrowed run:\n%s\nvs\n%s", i, a, b)
		}
	}
}

// TestRunExternalCell_SuccessAnswerOnlyExemption is the fix-first
// review's own regression test for runExternalCell's Success wiring:
// before the fix it computed Success from its own inlined copy of the
// any-edits floor (missing the RefusalAware exemption runOneCell's copy
// had, O57 finding 2) instead of the ONE shared SuccessFor. This proves
// that wiring along the REAL path (PrepareSubject + RunSessions +
// RunOracle) with an answer-only task (no edits ever expected, by
// design) — SuccessFor's RefusalAware exemption specifically is covered
// directly and exhaustively by TestSuccessFor (oracle_test.go), since
// exercising ScoreRefusal here would need a refusal-aware oracle's
// detector/git-diff step, which runs against a freshly `checkoutCommitIntoWorktree`d
// worktree whose own history does not contain task.Subject.BaseCommit at
// all (confirmed separately, PRE-EXISTING and unrelated to this review —
// see the report) — not something this test should paper over by
// avoiding forbidden_paths/detectors, since a real refusal-aware corpus
// task always carries both.
func TestRunExternalCell_SuccessAnswerOnlyExemption(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	task := Task{
		ID:         "answer-only-external-1",
		Category:   CategoryOrientation,
		Prompt:     "what is the answer?",
		Subject:    Subject{RepoURL: repoPath, BaseCommit: commit},
		Sessions:   Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 5},
		Derivation: "test fixture for TestRunExternalCell_SuccessAnswerOnlyExemption",
		Oracle: Oracle{
			// answer-token.jsonl's final answer reads "The answer is
			// fusion.New and also golden-test appear here."
			AnswerAllOf: []string{"golden-test"},
		},
	}
	arm := Arm{Wired: false, Instructions: "plain"}
	harness := &FakeHarness{StreamsDir: streamsDir, Default: "answer-token.jsonl"}
	root := t.TempDir()
	inv := CellInvariants{TaskID: task.ID, Arm: "control", ModelID: "fake-model", HarnessVersion: "fake-1"}
	key := ScenarioCellKey(inv)

	cell, err := runExternalCell(context.Background(), root, task, "control", arm, harness, filepath.Join(root, "raw"), "fake", "run-1", 1, inv, key)
	if err != nil {
		t.Fatalf("runExternalCell: %v", err)
	}
	if !cell.Oracle.Passed {
		t.Fatalf("expected oracle.Passed=true (the answer cites the token), got %+v", cell.Oracle)
	}
	if AnyEdits(cell.Sessions) {
		t.Fatalf("expected no edits at all in this fixture, got FilesEdited on %+v", cell.Sessions)
	}
	if !cell.Success {
		t.Error("Success = false, want true: an AnswerOnly task is exempt from the any-edits floor, via the SAME SuccessFor runOneCell and cmdScore now share")
	}
}

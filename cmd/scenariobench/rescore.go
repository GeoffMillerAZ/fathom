// rescore.go — wave N18 defect 2: `scenariobench run -rescore-lost`
// re-scores, from raw session streams a run already spent real tokens
// producing, any cell of this run that has no stored cell record (LOST)
// or whose stored record's own InstrumentDefect matches -rescore-reason —
// by re-parsing those streams through the SAME session parser, detectors
// and oracle the live path uses (scenariobench.RescoreSessionsFromRaw),
// WITHOUT launching `claude` and WITHOUT touching the (shared, long since
// reset and reused by a later cell) subject worktree at all. This file
// never imports a Harness and never calls PrepareSubject/RunSessions — a
// rescore run cannot spend real tokens by construction, not merely by
// convention. A task whose oracle genuinely needs a live worktree
// (scenariobench.NeedsLiveWorktree) is reported not rescorable rather than
// guessed at.
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// errNotRescorable marks a cell rescoreOneCell could not produce a sound
// reading for from raw alone (no complete raw attempt on disk, or an
// oracle shape that needs a live worktree) — reported through
// cellOutcome.lostReason exactly like an ordinary LOST cell, never a hard
// error that would abort the rest of the run.
var errNotRescorable = errors.New("not rescorable")

// rescoreCellForReport is runCellForReport's own rescore-mode sibling:
// same borrow-check/run/store shape, but "run" means reconstructing from
// raw (rescoreOneCell), never launching a session. A cell already stored
// under this exact key is left alone (borrowed) UNLESS rescoreReason is
// set and matches its own InstrumentDefect, in which case it is rescored
// and OVERWRITTEN (scenariobench.OverwriteScenarioCell, like
// `scenariobench void`'s own correction) rather than left alone; a cell
// with no stored record at all is rescored and stored fresh.
func rescoreCellForReport(root string, cp cellPlan, corpus scenariobench.Corpus, cellsDir, rawDir, runID, rescoreReason string) (cellOutcome, error) {
	existing, borrowed, err := scenariobench.BorrowScenarioCell(cellsDir, cp.key, cp.trial)
	if err != nil {
		return cellOutcome{cp: cp}, fmt.Errorf("checking cell store for %s/%s/t%d: %w", cp.task.ID, cp.arm, cp.trial, err)
	}
	if borrowed && (rescoreReason == "" || existing.InstrumentDefect != rescoreReason) {
		// Either it is already a sound reading, or -rescore-reason was not
		// given at all — either way this cell stands as stored.
		return cellOutcome{cp: cp, borrowed: true}, nil
	}

	arm := corpus.Arms[cp.arm]
	result, err := rescoreOneCell(root, cp, arm, rawDir, runID)
	if err != nil {
		if errors.Is(err, errNotRescorable) {
			return cellOutcome{cp: cp, lostReason: err.Error()}, nil
		}
		return cellOutcome{cp: cp}, fmt.Errorf("rescoring %s/%s/t%d: %w", cp.task.ID, cp.arm, cp.trial, err)
	}
	// The SAME gate a live session's own outcome is held to — a cell whose
	// reconstructed sessions still show a genuine defect (a real sandbox
	// escape, a real answer-key leak, an instrument fault) stays LOST,
	// never stored, exactly as it would have live.
	if reason := scenariobench.CellLost(result.Sessions); reason != "" {
		return cellOutcome{cp: cp, lostReason: reason}, nil
	}

	if borrowed {
		result.CellKey = cp.key
		if err := scenariobench.OverwriteScenarioCell(cellsDir, result); err != nil {
			return cellOutcome{cp: cp}, fmt.Errorf("overwriting rescored cell %s/%s/t%d: %w", cp.task.ID, cp.arm, cp.trial, err)
		}
	} else {
		if err := scenariobench.StoreScenarioCell(cellsDir, result); err != nil {
			return cellOutcome{cp: cp}, fmt.Errorf("storing rescored cell %s/%s/t%d: %w", cp.task.ID, cp.arm, cp.trial, err)
		}
	}
	return cellOutcome{cp: cp, result: result}, nil
}

// rescoreOneCell reconstructs and scores one cell entirely from its own
// raw session streams — the rescore-mode sibling of runOneCell (which it
// otherwise mirrors: same Readings/Oracle/Success assembly), except it
// never touches wt at all (there is none) and so never computes the
// readings that need one (PlanCoverage, FalseDone, Resumption,
// ContractDrift, Overlap, RuleServed, RuleServedBeforeEdit,
// ApplyCompileCost) — left at their own nil/zero value, an honest "not
// attempted from raw" rather than a guess. RescoredFromRaw is always true
// on what this returns.
func rescoreOneCell(root string, cp cellPlan, arm scenariobench.Arm, rawDir, runID string) (scenariobench.CellResult, error) {
	if scenariobench.NeedsLiveWorktree(cp.task) {
		return scenariobench.CellResult{}, fmt.Errorf("%w: oracle needs a live worktree (test patch, named tests, a detector, or refusal scoring)", errNotRescorable)
	}

	cellRawBase := scenariobench.CellRawBase(rawDir, runID, cp.task.ID, cp.arm, cp.trial)
	attemptDir, ok, err := scenariobench.LatestCompleteAttempt(cellRawBase)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("finding a complete raw attempt under %s: %w", cellRawBase, err)
	}
	if !ok {
		return scenariobench.CellResult{}, fmt.Errorf("%w: no complete raw session stream under %s", errNotRescorable, cellRawBase)
	}

	// Computed with no I/O (HistoricalWorktreePath/HistoricalCellDir's own
	// doc comment): the cell's own directories exactly as PrepareSubject
	// would have named them, whether or not they still exist — the safe
	// directories SandboxEscapeScan needs (wave N18 defect 1's own fix)
	// and the path FilesRead/FilesEdited relativize against, never read
	// from disk.
	worktreeHint := scenariobench.HistoricalWorktreePath(root, cp.task, arm)
	cellDirHint := scenariobench.HistoricalCellDir(root, cp.task, arm, cp.trial)
	// wave N19 defect 1: the shared subjects/ pool root, alongside this
	// cell's own three directories — see HistoricalSubjectsDir's own doc
	// comment for why a bare listing of it is safe while a specific OTHER
	// cell's worktree under it stays caught.
	subjectsDirHint := scenariobench.HistoricalSubjectsDir(root)
	safeDirs := []string{worktreeHint, cellDirHint, cellRawBase, subjectsDirHint}

	sessions, err := scenariobench.RescoreSessionsFromRaw(attemptDir, cp.task.Subject.ExcludePaths, arm.Wired, worktreeHint, safeDirs)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("%w: %v", errNotRescorable, err)
	}

	// scenariobench.Worktree{} (the zero value): NeedsLiveWorktree already
	// refused every task shape whose RunOracle would otherwise dereference
	// wt for anything (ApplyTestPatch, runOracleTests, runDetector,
	// ScoreRefusal — see RunOracle's own doc comment) — every oracle branch
	// this call can actually reach reads sessions/task only.
	oracle, err := scenariobench.RunOracle(context.Background(), scenariobench.Worktree{}, cp.task, sessions, root)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("running oracle: %w", err)
	}

	readings := scenariobench.AggregateReadings(sessions)
	readings.SurfaceCalls = scenariobench.SurfaceCallCount(readings.ToolCalls, arm.Surface)
	readings.GoldSourceTool, readings.GoldSourceEvidence = scenariobench.GoldSourceTool(sessions, cp.task)
	readings.Violation = cellViolation(oracle)
	readings.Localization = oracle.Localization
	readings.RefusalOutcome = oracle.Outcome

	return scenariobench.CellResult{
		TaskID:          cp.task.ID,
		Arm:             cp.arm,
		Trial:           cp.trial,
		Category:        cp.task.Category,
		Sessions:        sessions,
		Oracle:          oracle,
		Readings:        readings,
		Success:         scenariobench.SuccessFor(cp.task, oracle, sessions),
		SelfSkip:        scenariobench.SelfSkipDetected(sessions),
		CellKey:         cp.key,
		CorpusSHA256:    cp.inv.CorpusSHA256,
		ModelID:         cp.inv.ModelID,
		HarnessVersion:  cp.inv.HarnessVersion,
		ArmSurface:      cp.inv.ArmSurface,
		Surface:         arm.Surface,
		Gold:            cp.task.Gold,
		RunID:           runID,
		WorktreePath:    worktreeHint,
		BaseCommit:      cp.task.Subject.BaseCommit,
		RescoredFromRaw: true,
	}, nil
}

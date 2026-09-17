// rescore.go — wave N18 defect 2: `scenariobench run -rescore-lost`
// re-scores, from raw session streams a run already spent real tokens
// producing, any cell of this run that has no stored cell record (LOST)
// or whose stored record's own InstrumentDefect matches -rescore-reason —
// by re-parsing those streams through the SAME session parser, detectors
// and oracle the live path uses (scenariobench.RescoreSessionsFromRaw),
// WITHOUT launching `claude`. This file never imports a Harness and never
// calls RunSessions — a rescore run cannot spend real tokens by
// construction, not merely by convention.
//
// A task whose oracle NeedsLiveWorktree (a test patch, named tests, a
// detector, or refusal scoring) took the zero-Worktree{} path's own
// "not rescorable" refusal through wave N19; wave N20 gives it a second
// chance — rescoreOneCellRematerialized re-materializes the cell's
// worktree from raw (scenariobench.RematerializeCellWorktree: the SAME
// PrepareSubject a live cell used, plus the cell's own stored agent.diff
// applied on top) and runs the SAME baseline+oracle a live cell runs
// against it, deleting the worktree afterward either way. This is the ONE
// place in this file that calls PrepareSubject — only ever against a
// worktree this file itself deletes again before returning, never one a
// later live run would also touch, so it still cannot spend real tokens: no
// Harness is ever constructed and RunSessions is still never called. A cell
// whose raw dir holds no agent.diff (or a binary diff with no
// untracked-snapshot fallback) is reported not rescorable by
// RematerializeCellWorktree's own reason, never guessed at.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

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
func rescoreCellForReport(ctx context.Context, root string, cp cellPlan, corpus scenariobench.Corpus, cellsDir, rawDir, runID, rescoreReason string) (cellOutcome, error) {
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
	result, err := rescoreOneCell(ctx, root, cp, arm, rawDir, runID)
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
// otherwise mirrors: same Readings/Oracle/Success assembly). A task whose
// oracle NeedsLiveWorktree is handed off to rescoreOneCellRematerialized
// below, which DOES rebuild a real worktree (and so can compute the
// baseline+oracle exactly as a live cell would); every other task takes
// this function's own zero-Worktree{} path and so never computes the
// readings that need one (PlanCoverage, FalseDone, Resumption,
// ContractDrift, Overlap, RuleServed, RuleServedBeforeEdit,
// ApplyCompileCost) — left at their own nil/zero value, an honest "not
// attempted from raw" rather than a guess. RescoredFromRaw is always true
// on what either path returns.
func rescoreOneCell(ctx context.Context, root string, cp cellPlan, arm scenariobench.Arm, rawDir, runID string) (scenariobench.CellResult, error) {
	cellRawBase := scenariobench.CellRawBase(rawDir, runID, cp.task.ID, cp.arm, cp.trial)
	attemptDir, ok, err := scenariobench.LatestCompleteAttempt(cellRawBase)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("finding a complete raw attempt under %s: %w", cellRawBase, err)
	}
	if !ok {
		return scenariobench.CellResult{}, fmt.Errorf("%w: no complete raw session stream under %s", errNotRescorable, cellRawBase)
	}

	if scenariobench.NeedsLiveWorktree(cp.task) {
		return rescoreOneCellRematerialized(ctx, root, cp, arm, attemptDir, runID)
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

// rescoreOneCellRematerialized is rescoreOneCell's sibling for a task whose
// oracle NeedsLiveWorktree: re-materializes the cell's worktree from raw
// (scenariobench.RematerializeCellWorktree — the SAME PrepareSubject a live
// cell used, plus the cell's own stored agent.diff applied on top), then
// runs the baseline detectors and RunOracle exactly as runOneCell's own
// live path does (O57 finding 3: a detector PrepareSubject's own wiring
// left red is voided, never scored, exactly like a live cell), before
// deleting the worktree either way (RematerializeCellWorktree's own
// cleanup) — never leaving one behind between cells (disk-small, "one
// re-materialised worktree at a time" is this wave's own operating
// constraint). Like rescoreOneCell's own zero-Worktree{} path, this never
// computes the SDD readings that need a plan.md this reconstruction has no
// sound way to date (PlanCoverage, FalseDone, Resumption, ContractDrift,
// Overlap, RuleServed) — left at their own zero value. Rematerialized is
// always true on what this returns, alongside RescoredFromRaw.
func rescoreOneCellRematerialized(ctx context.Context, root string, cp cellPlan, arm scenariobench.Arm, attemptDir, runID string) (scenariobench.CellResult, error) {
	wt, cleanup, ok, reason := scenariobench.RematerializeCellWorktree(ctx, root, cp.task, arm, attemptDir)
	defer cleanup()
	if !ok {
		return scenariobench.CellResult{}, fmt.Errorf("%w: %s", errNotRescorable, reason)
	}

	// O57 finding 6: per-trial, not per-(task,arm) — same reasoning as
	// runOneCell's own wt.CellDir reassignment (main.go).
	wt.CellDir = scenariobench.PerTrialCellDir(wt.CellDir, "", cp.trial)
	if err := os.MkdirAll(wt.CellDir, 0o755); err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("creating per-trial cell dir %s: %w", wt.CellDir, err)
	}

	baseline, err := scenariobench.RunBaselineDetectors(ctx, wt, cp.task)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("running baseline detectors: %w", err)
	}

	// wt.Path/wt.CellDir/attemptDir plus the shared subjects/ pool root
	// (wave N19 defect 1) — the same four safe dirs rescoreOneCell's own
	// zero-Worktree{} path hands in, now from the REAL worktree rather than
	// a no-I/O historical guess at where it would have been.
	subjectsDirHint := scenariobench.HistoricalSubjectsDir(root)
	safeDirs := []string{wt.Path, wt.CellDir, attemptDir, subjectsDirHint}

	sessions, err := scenariobench.RescoreSessionsFromRaw(attemptDir, cp.task.Subject.ExcludePaths, arm.Wired, wt.Path, safeDirs)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("%w: %v", errNotRescorable, err)
	}

	oracle, err := scenariobench.RunOracle(ctx, wt, cp.task, sessions, root)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("running oracle: %w", err)
	}
	scenariobench.VoidBaselineRedDetectors(&oracle, baseline, cp.task, sessions)

	readings := scenariobench.AggregateReadings(sessions)
	readings.SurfaceCalls = scenariobench.SurfaceCallCount(readings.ToolCalls, arm.Surface)
	scenariobench.ApplyCompileCost(&readings, wt)
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
		WorktreePath:    wt.Path,
		BaseCommit:      cp.task.Subject.BaseCommit,
		RescoredFromRaw: true,
		Rematerialized:  true,
	}, nil
}

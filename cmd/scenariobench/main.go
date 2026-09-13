// Command scenariobench is the change bench driver (vision 27, ADR-033;
// docs/plans/strengths-bench.md wave 2) over internal/scenariobench.
//
// Subcommands:
//
//	scenariobench plan -corpus <path> [-cells <dir>] [-model-id <id>] [-harness-version <v>] [-scoring <id>] [-fathom-commit <sha>] [-trials N]
//	    Lists every (task, arm, trial) cell the corpus implies, its key, and
//	    whether it would borrow from the cell store or run for real. Spends
//	    nothing: no process is started.
//
//	scenariobench run -corpus <path> -root <fathom repo root> [-cells <dir>] [-raw-dir <dir>] [-model-id <id>] [-harness-version <v>] [-scoring <id>] [-fathom-commit <sha>] [-trials N] [-model <name>] [-only <task id>] [-arm <name>] [-harness claude|fake] [-fake-streams-dir <dir>] [-fake-streams <json>]
//	    For each selected cell not already in the cell store: PrepareSubject,
//	    RunSessions, RunOracle, stores the scored CellResult. `-harness fake`
//	    (with -fake-streams-dir / -fake-streams, a JSON map of lookup key to
//	    stream file name) replays committed transcripts instead of shelling
//	    out to `claude -p` — the only spend-free way to exercise this
//	    subcommand, and what this package's own tests use.
//
//	scenariobench score -in <cell.json> [-out <path>]
//	    Recomputes a stored cell's basic aggregate readings (total tokens,
//	    turns, cost, payload bytes) from its sessions and rewrites it. Free:
//	    it reads what run already captured.
//
//	scenariobench ledger -corpus <path> -cells <dir> -ledger <path.jsonl> -readme <path> -run-id <id> [-commit <sha>] [-arm-surface arm=value ...]
//	    Reads every stored cell for -run-id, groups by (task, arm) into
//	    ScenarioLedgerRow, appends them to -ledger, re-renders -readme's
//	    marker-fenced section, and evaluates the corpus's registered
//	    criterion. Exit 0 pass, 1 fail (a measured refutation — recorded, not
//	    excused), 3 indeterminate (re-run: not enough was measured).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: scenariobench <plan|run|score|ledger> [flags]")
		return 2
	}
	switch args[0] {
	case "plan":
		return cmdPlan(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "score":
		return cmdScore(args[1:])
	case "ledger":
		return cmdLedger(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (want plan, run, score, or ledger)\n", args[0])
		return 2
	}
}

// cellFlags are the invariants every subcommand that touches the cell
// store needs, factored out so plan/run/ledger cannot disagree about what
// a key means.
type cellFlags struct {
	modelID        *string
	harnessVersion *string
	scoring        *string
	fathomCommit   *string
}

func addCellFlags(fs *flag.FlagSet) cellFlags {
	return cellFlags{
		modelID:        fs.String("model-id", "sonnet", "model identity the cell key is built on"),
		harnessVersion: fs.String("harness-version", "", "harness version string the cell key is built on (required; e.g. `claude --version`'s output, or the fake harness's Version())"),
		scoring:        fs.String("scoring", "oracle-v1", "the readings/oracle version identifier the cell key is built on"),
		fathomCommit:   fs.String("fathom-commit", "", "the fathom commit a wired arm's cells key on (default: this tree's HEAD via git)"),
	}
}

func resolveFathomCommit(explicit string) string {
	if explicit != "" {
		return explicit
	}
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func promptSubjectFor(task scenariobench.Task) string {
	svc := task.Subject.Service
	if svc == "" {
		svc = "the subject"
	}
	return "the " + svc + " codebase"
}

// cellPlan is one (task, arm, trial) cell's identity, computed the same
// way by plan/run/ledger.
type cellPlan struct {
	task  scenariobench.Task
	arm   string
	trial int
	inv   scenariobench.CellInvariants
	key   string
	// altKeys are the keys the same cell carries under the OTHER fathom
	// commits named by a comma-separated -fathom-commit: a run whose tree
	// moved between launches for driver-only commits keyed its wired
	// cells on more than one surface; the ledger step may read them all,
	// naming every surface it used. `run` and `plan` key on the first.
	altKeys []string
}

func planCells(c scenariobench.Corpus, cf cellFlags, trials int, corpusSHA string) []cellPlan {
	commits := strings.Split(*cf.fathomCommit, ",")
	fathomCommit := resolveFathomCommit(strings.TrimSpace(commits[0]))
	var altCommits []string
	for _, c := range commits[1:] {
		if c = strings.TrimSpace(c); c != "" {
			altCommits = append(altCommits, c)
		}
	}
	armNames := make([]string, 0, len(c.Arms))
	for name := range c.Arms {
		armNames = append(armNames, name)
	}
	sort.Strings(armNames)

	var out []cellPlan
	for _, task := range c.Tasks {
		for _, armName := range armNames {
			arm := c.Arms[armName]
			surface := scenariobench.ResolveArmSurface(arm, fathomCommit)
			for trial := 1; trial <= trials; trial++ {
				inv := scenariobench.CellInvariants{
					CorpusSHA256:   corpusSHA,
					TaskID:         task.ID,
					Arm:            armName,
					ModelID:        *cf.modelID,
					HarnessVersion: *cf.harnessVersion,
					Scoring:        *cf.scoring,
					PromptSubject:  promptSubjectFor(task),
					ArmSurface:     surface,
				}
				cp := cellPlan{task: task, arm: armName, trial: trial, inv: inv, key: scenariobench.ScenarioCellKey(inv)}
				for _, alt := range altCommits {
					altInv := inv
					altInv.ArmSurface = scenariobench.ResolveArmSurface(arm, alt)
					if altInv.ArmSurface != inv.ArmSurface {
						cp.altKeys = append(cp.altKeys, scenariobench.ScenarioCellKey(altInv))
					}
				}
				out = append(out, cp)
			}
		}
	}
	return out
}

func loadCorpus(path string) (scenariobench.Corpus, string, error) {
	c, err := scenariobench.LoadScenarioCorpus(path)
	if err != nil {
		return scenariobench.Corpus{}, "", err
	}
	sha, err := scenariobench.CorpusSHA256(path)
	if err != nil {
		return scenariobench.Corpus{}, "", err
	}
	return c, sha, nil
}

// ─── plan ────────────────────────────────────────────────────────────────

func cmdPlan(args []string) int {
	fs := newFlagSet("plan")
	corpusPath := fs.String("corpus", "", "path to the scenario corpus JSON (required)")
	cellsDir := fs.String("cells", filepath.Join("workspace", "artifacts", "scenariobench", "cells"), "content-addressed cell store")
	trials := fs.Int("trials", 1, "trials per (task, arm)")
	cf := addCellFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *corpusPath == "" {
		fmt.Fprintln(os.Stderr, "scenariobench plan: -corpus is required")
		return 2
	}
	if *cf.harnessVersion == "" {
		fmt.Fprintln(os.Stderr, "scenariobench plan: -harness-version is required (a cell key over an unknown harness would let an unrelated cell be borrowed)")
		return 2
	}
	corpus, sha, err := loadCorpus(*corpusPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench plan: %v\n", err)
		return 2
	}

	cells := planCells(corpus, cf, *trials, sha)
	var wouldRun, wouldBorrow int
	for _, cp := range cells {
		_, borrowed, err := scenariobench.BorrowScenarioCell(*cellsDir, cp.key, cp.trial)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench plan: checking cell store for %s/%s/t%d: %v\n", cp.task.ID, cp.arm, cp.trial, err)
			return 2
		}
		verb := "would run"
		if borrowed {
			verb = "would borrow"
			wouldBorrow++
		} else {
			wouldRun++
		}
		fmt.Fprintf(os.Stdout, "%s/%s/t%d key=%s %s\n", cp.task.ID, cp.arm, cp.trial, cp.key[:12], verb)
	}
	fmt.Fprintf(os.Stdout, "scenariobench plan: %d cell(s) — %d would run, %d would borrow (no spend)\n", len(cells), wouldRun, wouldBorrow)
	return 0
}

// ─── run ─────────────────────────────────────────────────────────────────

func cmdRun(args []string) int {
	fs := newFlagSet("run")
	corpusPath := fs.String("corpus", "", "path to the scenario corpus JSON (required)")
	root := fs.String("root", "", "this repo's root, where `go build ./cmd/fathom` runs from (required for a wired arm)")
	cellsDir := fs.String("cells", filepath.Join("workspace", "artifacts", "scenariobench", "cells"), "content-addressed cell store")
	rawDir := fs.String("raw-dir", filepath.Join("workspace", "artifacts", "scenariobench", "raw"), "base directory for per-session raw transcripts")
	trials := fs.Int("trials", 1, "trials per (task, arm)")
	model := fs.String("model", "sonnet", "model passed to the harness")
	only := fs.String("only", "", "comma-separated task ids to run (default: every task)")
	onlyArm := fs.String("arm", "", "run only this arm (default: every arm)")
	runID := fs.String("run-id", "", "this run's id (required; stamps CellResult.RunID)")
	harnessName := fs.String("harness", "claude", "harness to run sessions with: claude (real spend) or fake (replays committed transcripts, no spend)")
	fakeStreamsDir := fs.String("fake-streams-dir", "", "-harness fake: directory holding the committed stream fixtures")
	fakeStreams := fs.String("fake-streams", "", "-harness fake: JSON map of \"<task>/<arm>[/<role>]\" to a file name under -fake-streams-dir")
	cf := addCellFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *corpusPath == "" || *runID == "" {
		fmt.Fprintln(os.Stderr, "scenariobench run: -corpus and -run-id are required")
		return 2
	}
	if *cf.harnessVersion == "" {
		fmt.Fprintln(os.Stderr, "scenariobench run: -harness-version is required")
		return 2
	}
	corpus, sha, err := loadCorpus(*corpusPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench run: %v\n", err)
		return 2
	}
	onlyTasks := splitCommaSet(*only)
	cells := planCells(corpus, cf, *trials, sha)

	ctx := context.Background()
	anyFailed := false
	ran := 0
	lost := 0
	absRawDir, _ := filepath.Abs(*rawDir) // claude -p runs in the worktree and resolves --mcp-config against ITS cwd
	for _, cp := range cells {
		if len(onlyTasks) > 0 && !onlyTasks[cp.task.ID] {
			continue
		}
		if *onlyArm != "" && cp.arm != *onlyArm {
			continue
		}
		if _, borrowed, err := scenariobench.BorrowScenarioCell(*cellsDir, cp.key, cp.trial); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench run: checking cell store for %s/%s/t%d: %v\n", cp.task.ID, cp.arm, cp.trial, err)
			return 2
		} else if borrowed {
			fmt.Fprintf(os.Stdout, "%s/%s/t%d: borrowed\n", cp.task.ID, cp.arm, cp.trial)
			continue
		}

		arm := corpus.Arms[cp.arm]
		harness, herr := resolveHarness(*harnessName, *fakeStreamsDir, *fakeStreams, cp.task.ID, cp.arm)
		if herr != nil {
			fmt.Fprintf(os.Stderr, "scenariobench run: %v\n", herr)
			return 2
		}

		result, err := runOneCell(ctx, *root, cp, arm, harness, absRawDir, *model, *runID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench run: %s/%s/t%d: %v\n", cp.task.ID, cp.arm, cp.trial, err)
			return 2
		}
		if reason := scenariobench.CellLost(result.Sessions); reason != "" {
			// A session that never ran is not a reading: it is not stored
			// (a later run would borrow it as data) and the row counts it
			// lost, exactly as context-bench does (guards pilot 1,
			// 2026-09-13: 16 cells whose harness died at startup were
			// stored, then borrowed by the re-run).
			lost++
			fmt.Fprintf(os.Stderr, "%s/%s/t%d: LOST — %s\n", cp.task.ID, cp.arm, cp.trial, reason)
			continue
		}
		if err := scenariobench.StoreScenarioCell(*cellsDir, result); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench run: storing %s/%s/t%d: %v\n", cp.task.ID, cp.arm, cp.trial, err)
			return 2
		}
		ran++
		status := "FAIL"
		if result.Success {
			status = "PASS"
		} else {
			anyFailed = true
		}
		fmt.Fprintf(os.Stdout, "%s/%s/t%d: %s tokens=%d cost=%.4f\n", cp.task.ID, cp.arm, cp.trial, status, result.Readings.TotalTokens, result.Readings.TotalCostUSD)
	}
	fmt.Fprintf(os.Stdout, "scenariobench run: %d cell(s) run, %d lost\n", ran, lost)
	if lost > 0 && ran == 0 {
		return 3
	}
	if anyFailed {
		return 1
	}
	return 0
}

func runOneCell(ctx context.Context, root string, cp cellPlan, arm scenariobench.Arm, harness scenariobench.Harness, rawDir, model, runID string) (scenariobench.CellResult, error) {
	wt, err := scenariobench.PrepareSubject(ctx, cp.task, arm, root)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("preparing subject: %w", err)
	}
	defer scenariobench.StopWorktree(&wt)

	sessionsStart := time.Now()
	sessions, err := scenariobench.RunSessions(ctx, harness, cp.task, cp.arm, arm, wt, scenariobench.RunOptions{
		Trial: cp.trial, Model: model, RawDir: rawDir,
	})
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("running sessions: %w", err)
	}

	// SDD readings capture: plan.md and agent.diff go into wt.CellDir
	// AFTER the sessions and BEFORE the oracle — RunOracle applies the
	// hidden test patch and resets nothing else in the tree, but it is
	// still the last thing to touch it, and these two files must record
	// only what the AGENT produced.
	planPath, planBytes := captureAgentPlan(wt, sessionsStart)
	diffPath := captureAgentDiff(ctx, wt)

	oracle, err := scenariobench.RunOracle(ctx, wt, cp.task, sessions, root)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("running oracle: %w", err)
	}

	readings := scenariobench.AggregateReadings(sessions)
	boxChecks := allBoxChecks(sessions)

	planCoverage := scenariobench.PlanCoverage(planBytes, cp.task.Gold)
	readings.PlanCoverage = &planCoverage

	falseDone := scenariobench.FalseDoneRate(boxChecks, wt.Path, oracle.CoverProfile)
	readings.FalseDone = &falseDone

	resumption := scenariobench.ResumptionReadings(sessions, provenFromFalseDone(sessions, falseDone))
	readings.Resumption = &resumption

	drift := scenariobench.ContractDrift(planBytes, wt.Path, oracle.CoverProfile)
	readings.Drift = &drift

	if cp.task.Sessions.Mode == scenariobench.ModePlannerWorkers {
		overlap := scenariobench.ReexplorationOverlap(sessions)
		readings.Overlap = &overlap
	}

	if cp.task.Rule != nil {
		if turn, ok := firstClaimTurn(sessions); ok {
			readings.RuleServed = scenariobench.RuleServed(hooksLogPath(wt), cp.task.Rule.ID, turn)
		}
	}

	return scenariobench.CellResult{
		TaskID:   cp.task.ID,
		Arm:      cp.arm,
		Trial:    cp.trial,
		Category: cp.task.Category,
		Sessions: sessions,
		Oracle:   oracle,
		Readings: readings,
		// An untouched tree cannot pass (guards pilot 1) — UNLESS the
		// task's oracle is answer-only (SB-32 lookups-vs.json), where
		// nothing was ever meant to be edited.
		Success:        oracle.Passed && (scenariobench.AnswerOnly(cp.task) || scenariobench.AnyEdits(sessions)),
		CellKey:        cp.key,
		CorpusSHA256:   cp.inv.CorpusSHA256,
		ModelID:        cp.inv.ModelID,
		HarnessVersion: cp.inv.HarnessVersion,
		ArmSurface:     cp.inv.ArmSurface,
		Gold:           cp.task.Gold,
		PlanPath:       planPath,
		AgentDiffPath:  diffPath,
		RunID:          runID,
		StartedAt:      "", // TODO(SB-19 follow-up): inject a clock; not needed for wave 2's own tests
		WorktreePath:   wt.Path,
		SubjectClone:   wt.SubjectClone,
		BaseCommit:     cp.task.Subject.BaseCommit,
	}, nil
}

// allBoxChecks flattens every session's BoxChecks, in session order.
func allBoxChecks(sessions []scenariobench.SessionResult) []scenariobench.BoxCheck {
	var out []scenariobench.BoxCheck
	for _, s := range sessions {
		out = append(out, s.BoxChecks...)
	}
	return out
}

// firstClaimTurn is the earliest Turn among every session's BoxChecks — the
// turn RuleServed compares an obligation's own first-served turn against.
// ok is false when no session ever checked a box, in which case RuleServed
// is not called at all: "did the rule reach the session before its first
// claim" is not a question without a claim.
func firstClaimTurn(sessions []scenariobench.SessionResult) (turn int, ok bool) {
	for _, bc := range allBoxChecks(sessions) {
		if !ok || bc.Turn < turn {
			turn, ok = bc.Turn, true
		}
	}
	return turn, ok
}

// provenFromFalseDone derives ResumptionReadings' "proven" map from
// FalseDoneRate's own verdict (claimed − refuted): a box-checked row is
// proven when FalseDoneRate did NOT refute it against the final tree — the
// caller's ground truth, never re-derived by ResumptionReadings itself.
func provenFromFalseDone(sessions []scenariobench.SessionResult, fd scenariobench.FalseDoneReading) map[string]bool {
	refuted := make(map[string]bool, len(fd.RefutedRows))
	for _, row := range fd.RefutedRows {
		label := row
		if i := strings.Index(row, ": "); i >= 0 {
			label = row[:i]
		}
		refuted[label] = true
	}
	proven := map[string]bool{}
	for _, bc := range allBoxChecks(sessions) {
		if bc.RowID == "" {
			continue
		}
		if _, seen := proven[bc.RowID]; !seen {
			proven[bc.RowID] = !refuted[bc.RowID]
		}
	}
	return proven
}

// hooksLogPath is the edit-hook beat log RuleServed should read for this
// cell: $FATHOM_HOOKS_LOG when the harness's own environment (or this
// process's) set one, else a best-effort guess under the worktree itself.
// Neither PrepareSubject nor RunSessions scopes FATHOM_HOOKS_LOG per cell
// today (a wired arm's own session env would need to point it at a
// cell-local file) — RuleServed already treats an unreadable/absent log as
// not_measurable (nil), so a wrong guess here costs nothing but a false
// "not served" rather than a false "served".
func hooksLogPath(wt scenariobench.Worktree) string {
	if p := os.Getenv("FATHOM_HOOKS_LOG"); p != "" {
		return p
	}
	return filepath.Join(wt.Path, ".fathom", "hooks.jsonl")
}

// captureAgentPlan copies the agent's own scratch plan into
// wt.CellDir/plan.md — the first (sorted) docs/plans/*.md file in the
// worktree modified AFTER sessionsStart (so a subject that already ships
// its own docs/plans/*.md, this repo included, is never mistaken for the
// agent's plan), else NOTES.md at the worktree root. Returns ("", nil)
// when neither exists — a task whose agent wrote no plan at all, which
// PlanCoverage/ContractDrift report as not_measurable, never as zero.
func captureAgentPlan(wt scenariobench.Worktree, sessionsStart time.Time) (path string, data []byte) {
	src := findAgentPlanFile(wt.Path, sessionsStart)
	if src == "" {
		return "", nil
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return "", nil
	}
	dest := filepath.Join(wt.CellDir, "plan.md")
	if err := os.WriteFile(dest, raw, 0o644); err != nil {
		return "", raw
	}
	return dest, raw
}

func findAgentPlanFile(worktree string, sessionsStart time.Time) string {
	plansDir := filepath.Join(worktree, "docs", "plans")
	if entries, err := os.ReadDir(plansDir); err == nil {
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			p := filepath.Join(plansDir, name)
			if info, err := os.Stat(p); err == nil && info.ModTime().After(sessionsStart) {
				return p
			}
		}
	}
	notes := filepath.Join(worktree, "NOTES.md")
	if _, err := os.Stat(notes); err == nil {
		return notes
	}
	return ""
}

// captureAgentDiff writes wt.CellDir/agent.diff: `git add -N .` (so an
// untracked file the agent created shows up as a new-file hunk, not as
// nothing) followed by `git diff` against the index, covering everything
// the sessions changed since PrepareSubject reset the tree to base_commit.
// Best-effort: an error returns "" rather than failing the cell — a
// captured diff is a convenience for later re-scoring, not a precondition
// for the readings computed here (which read the live worktree directly).
func captureAgentDiff(ctx context.Context, wt scenariobench.Worktree) string {
	add := exec.CommandContext(ctx, "git", "add", "-N", ".")
	add.Dir = wt.Path
	if _, err := add.CombinedOutput(); err != nil {
		return ""
	}
	diff := exec.CommandContext(ctx, "git", "diff")
	diff.Dir = wt.Path
	out, err := diff.Output()
	if err != nil {
		return ""
	}
	dest := filepath.Join(wt.CellDir, "agent.diff")
	if err := os.WriteFile(dest, out, 0o644); err != nil {
		return ""
	}
	return dest
}

// reconstructFinalTree rebuilds cell's final tree when its shared
// WorktreePath no longer exists on disk (a later cell sharing the same
// subject/base_commit/arm reset it out from under this one): a detached
// temp worktree off the persisted SubjectClone at BaseCommit, with the
// cell's own AgentDiffPath applied via `git apply` — the same shape
// PrepareSubject/captureAgentDiff produced at run time, rebuilt from what
// was captured rather than by re-running anything. ok is false whenever
// reconstruction cannot be attempted or fails at any step (SubjectClone or
// BaseCommit missing — an older CellResult from before these fields
// existed — the clone gone, the commit unreachable, or the diff failing to
// apply); the caller then falls back to its ordinary not_measurable rather
// than trust a wrong tree. The returned cleanup removes the temp worktree
// and MUST be called once the caller is done reading tree.
func reconstructFinalTree(cell *scenariobench.CellResult) (tree string, cleanup func(), ok bool) {
	noop := func() {}
	if cell.SubjectClone == "" || cell.BaseCommit == "" {
		return "", noop, false
	}
	if _, err := os.Stat(filepath.Join(cell.SubjectClone, ".git")); err != nil {
		return "", noop, false
	}
	tmp, err := os.MkdirTemp("", "scenariobench-rescore-")
	if err != nil {
		return "", noop, false
	}
	remove := func() {
		_ = exec.Command("git", "-C", cell.SubjectClone, "worktree", "remove", "--force", tmp).Run()
		_ = os.RemoveAll(tmp)
	}
	if _, err := exec.Command("git", "-C", cell.SubjectClone, "worktree", "add", "--detach", tmp, cell.BaseCommit).CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmp)
		return "", noop, false
	}
	if cell.AgentDiffPath != "" {
		if info, err := os.Stat(cell.AgentDiffPath); err == nil && info.Size() > 0 {
			apply := exec.Command("git", "apply", cell.AgentDiffPath)
			apply.Dir = tmp
			if _, err := apply.CombinedOutput(); err != nil {
				remove()
				return "", noop, false
			}
		}
	}
	return tmp, remove, true
}

// rescoreSDDReadings recomputes cell's SDD readings (PlanCoverage,
// FalseDone, Resumption, Drift) from what was captured at run time —
// PlanPath/AgentDiffPath and cell.Gold, both persisted on the CellResult —
// never by re-preparing the subject. PlanCoverage never needed a tree at
// all. FalseDone/ContractDrift's "final tree" search does: cell.WorktreePath
// is used when it still exists on disk; else reconstructFinalTree rebuilds
// it from SubjectClone/BaseCommit/AgentDiffPath — the worktree a cell
// recorded is a SHARED, REUSED path (PrepareSubject keys it on
// subject+base_commit+arm, not on task or trial), so a later cell sharing
// it may already have reset it out from under this one by the time score
// runs. Only when reconstruction also cannot be attempted do both readings
// report an explicit not_measurable rather than a false zero.
func rescoreSDDReadings(cell *scenariobench.CellResult) {
	var planBytes []byte
	if cell.PlanPath != "" {
		if raw, err := os.ReadFile(cell.PlanPath); err == nil {
			planBytes = raw
		}
	}
	planCoverage := scenariobench.PlanCoverage(planBytes, cell.Gold)
	cell.Readings.PlanCoverage = &planCoverage

	tree := cell.WorktreePath
	if tree != "" {
		if _, err := os.Stat(tree); err != nil {
			tree = ""
		}
	}
	if tree == "" {
		if recon, cleanup, ok := reconstructFinalTree(cell); ok {
			defer cleanup()
			tree = recon
		}
	}
	boxChecks := allBoxChecks(cell.Sessions)
	falseDone := falseDoneOrNotMeasurable(boxChecks, tree, cell.Oracle.CoverProfile)
	cell.Readings.FalseDone = &falseDone

	resumption := scenariobench.ResumptionReadings(cell.Sessions, provenFromFalseDone(cell.Sessions, falseDone))
	cell.Readings.Resumption = &resumption

	drift := driftOrNotMeasurable(planBytes, tree, cell.Oracle.CoverProfile)
	cell.Readings.Drift = &drift
}

const noWorktreeForRescore = "no worktree available to re-check the final tree (it was reused by a later cell sharing the same subject/base_commit/arm)"

func falseDoneOrNotMeasurable(checks []scenariobench.BoxCheck, tree, coverprofile string) scenariobench.FalseDoneReading {
	if tree == "" {
		return scenariobench.FalseDoneReading{Claimed: len(checks), NotMeasurable: noWorktreeForRescore}
	}
	return scenariobench.FalseDoneRate(checks, tree, coverprofile)
}

func driftOrNotMeasurable(planMD []byte, tree, coverprofile string) scenariobench.DriftReading {
	if tree == "" {
		return scenariobench.DriftReading{NotMeasurable: noWorktreeForRescore}
	}
	return scenariobench.ContractDrift(planMD, tree, coverprofile)
}

func resolveHarness(name, streamsDir, streamsJSON, taskID, armName string) (scenariobench.Harness, error) {
	switch name {
	case "claude":
		return scenariobench.ClaudeHarness{}, nil
	case "fake":
		if streamsDir == "" {
			return nil, fmt.Errorf("-harness fake requires -fake-streams-dir")
		}
		streams := map[string]string{}
		if streamsJSON != "" {
			if err := json.Unmarshal([]byte(streamsJSON), &streams); err != nil {
				return nil, fmt.Errorf("parsing -fake-streams: %w", err)
			}
		}
		return &scenariobench.FakeHarness{StreamsDir: streamsDir, Streams: streams, TaskID: taskID, ArmName: armName}, nil
	default:
		return nil, fmt.Errorf("unknown -harness %q (want claude or fake)", name)
	}
}

// ─── score ───────────────────────────────────────────────────────────────

func cmdScore(args []string) int {
	fs := newFlagSet("score")
	in := fs.String("in", "", "path to a stored CellResult JSON (required)")
	out := fs.String("out", "", "path to write the recomputed CellResult (default: overwrite -in)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" {
		fmt.Fprintln(os.Stderr, "scenariobench score: -in is required")
		return 2
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench score: %v\n", err)
		return 2
	}
	var cell scenariobench.CellResult
	if err := json.Unmarshal(raw, &cell); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench score: parsing %s: %v\n", *in, err)
		return 2
	}
	cell.Readings = scenariobench.AggregateReadings(cell.Sessions)
	cell.Success = cell.Oracle.Passed && scenariobench.AnyEdits(cell.Sessions)
	rescoreSDDReadings(&cell)

	dest := *out
	if dest == "" {
		dest = *in
	}
	outRaw, err := json.MarshalIndent(cell, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench score: %v\n", err)
		return 2
	}
	if err := os.WriteFile(dest, append(outRaw, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench score: writing %s: %v\n", dest, err)
		return 2
	}
	fmt.Fprintf(os.Stdout, "scenariobench score: rewrote %s (tokens=%d)\n", dest, cell.Readings.TotalTokens)
	return 0
}

// ─── ledger ──────────────────────────────────────────────────────────────

func cmdLedger(args []string) int {
	fs := newFlagSet("ledger")
	corpusPath := fs.String("corpus", "", "path to the scenario corpus JSON (required)")
	cellsDir := fs.String("cells", filepath.Join("workspace", "artifacts", "scenariobench", "cells"), "content-addressed cell store")
	ledgerPath := fs.String("ledger", filepath.Join("docs", "bench", "scenarios-ledger.jsonl"), "committed JSONL ledger")
	readmePath := fs.String("readme", filepath.Join("docs", "bench", "README.md"), "README carrying the marker-fenced scenario ledger table")
	runID := fs.String("run-id", "", "the run whose cells to file (required)")
	commit := fs.String("commit", "", "repo HEAD this run measured (default: git rev-parse --short HEAD)")
	trials := fs.Int("trials", 1, "trials per (task, arm) this run intended")
	cf := addCellFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *corpusPath == "" || *runID == "" {
		fmt.Fprintln(os.Stderr, "scenariobench ledger: -corpus and -run-id are required")
		return 2
	}
	if *cf.harnessVersion == "" {
		fmt.Fprintln(os.Stderr, "scenariobench ledger: -harness-version is required")
		return 2
	}
	corpus, sha, err := loadCorpus(*corpusPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
		return 2
	}
	generatedAt := ""
	resolvedCommit := *commit
	if resolvedCommit == "" {
		resolvedCommit = resolveFathomCommit("")
	}

	cells := planCells(corpus, cf, *trials, sha)
	type key struct{ task, arm string }
	byTaskArm := map[key][]scenariobench.CellResult{}
	for _, cp := range cells {
		cell, ok, err := scenariobench.BorrowScenarioCell(*cellsDir, cp.key, cp.trial)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench ledger: reading cell %s/%s/t%d: %v\n", cp.task.ID, cp.arm, cp.trial, err)
			return 2
		}
		for _, alt := range cp.altKeys {
			if ok {
				break
			}
			cell, ok, err = scenariobench.BorrowScenarioCell(*cellsDir, alt, cp.trial)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
				return 2
			}
		}
		if !ok || cell.RunID != *runID {
			continue
		}
		k := key{cp.task.ID, cp.arm}
		byTaskArm[k] = append(byTaskArm[k], cell)
	}

	var rows []scenariobench.ScenarioLedgerRow
	var allCells []scenariobench.CellResult
	keys := make([]key, 0, len(byTaskArm))
	for k := range byTaskArm {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].task != keys[j].task {
			return keys[i].task < keys[j].task
		}
		return keys[i].arm < keys[j].arm
	})
	taskByID := map[string]scenariobench.Task{}
	for _, t := range corpus.Tasks {
		taskByID[t.ID] = t
	}
	for _, k := range keys {
		group := byTaskArm[k]
		allCells = append(allCells, group...)
		row := scenariobench.ScenarioLedgerRow{
			RunID:          *runID,
			GeneratedAt:    generatedAt,
			Commit:         resolvedCommit,
			CorpusSHA256:   sha,
			TaskID:         k.task,
			Category:       taskByID[k.task].Category,
			Arm:            k.arm,
			ModelID:        *cf.modelID,
			HarnessVersion: *cf.harnessVersion,
		}
		for _, cell := range group {
			row.Trials++
			if cell.Success {
				row.Successes++
			}
			row.Readings.TotalTokens += cell.Readings.TotalTokens
			row.Readings.TotalTurns += cell.Readings.TotalTurns
			row.Readings.TotalCostUSD += cell.Readings.TotalCostUSD
			row.Readings.PayloadBytes += cell.Readings.PayloadBytes
			row.CostUSD += cell.Readings.TotalCostUSD
		}
		row.Prediction = taskByID[k.task].Predictions[k.arm]
		rows = append(rows, row)
	}
	rowBorrowed := scenariobench.BorrowedScenarioCellsOf(allCells)
	for i := range rows {
		for _, b := range rowBorrowed {
			if b.TaskID == rows[i].TaskID && b.Arm == rows[i].Arm {
				rows[i].Borrowed = append(rows[i].Borrowed, b)
			}
		}
	}

	for _, row := range rows {
		if err := scenariobench.AppendScenarioLedger(*ledgerPath, row); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
			return 2
		}
	}

	all, err := scenariobench.LoadScenarioLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
		return 2
	}
	if err := scenariobench.RewriteScenarioLedgerSection(*readmePath, scenariobench.RenderScenarioLedger(all)); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench ledger: rendering %s: %v\n", *readmePath, err)
		return 2
	}

	ev := corpus.Criterion.Evaluate(rows)
	fmt.Fprintf(os.Stdout, "scenariobench ledger: filed %d row(s); criterion %s: %s (%s)\n", len(rows), ev.CriterionID, ev.Verdict, ev.Reason)
	switch ev.Verdict {
	case scenariobench.ScenarioVerdictPass:
		return 0
	case scenariobench.ScenarioVerdictIndeterminate:
		return 3
	default:
		return 1
	}
}

// ─── flag helpers ────────────────────────────────────────────────────────

func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

func splitCommaSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out[f] = true
		}
	}
	return out
}

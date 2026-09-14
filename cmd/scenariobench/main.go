// Command scenariobench is the change bench driver (vision 27, ADR-033;
// docs/plans/strengths-bench.md wave 2) over internal/scenariobench.
//
// Subcommands:
//
//	scenariobench plan -corpus <path|lca> [-dataset <path.jsonl>] [-cells <dir>] [-model-id <id>] [-harness-version <v>] [-scoring <id>] [-fathom-commit <sha>] [-trials N]
//	    Lists every (task, arm, trial) cell the corpus implies, its key, and
//	    whether it would borrow from the cell store or run for real. Spends
//	    nothing: no process is started. -corpus lca (VB-25, vision 29 Wave
//	    G) builds the corpus in memory from -dataset — the operator's own
//	    downloaded Long Code Arena bug-localization JSONL, never fetched
//	    automatically (docs/bench/scenarios.md "External corpus (LCA bug
//	    localization)") — instead of reading a committed corpus file.
//
//	scenariobench run -corpus <path|lca> [-dataset <path.jsonl>] -root <fathom repo root> [-cells <dir>] [-raw-dir <dir>] [-model-id <id>] [-harness-version <v>] [-scoring <id>] [-fathom-commit <sha>] [-trials N] [-model <name>] [-only <task id>] [-arm <name>] [-harness claude|fake] [-fake-streams-dir <dir>] [-fake-streams <json>] [-parallel-arms]
//	    For each selected cell not already in the cell store: PrepareSubject,
//	    RunSessions, RunOracle, stores the scored CellResult. `-harness fake`
//	    (with -fake-streams-dir / -fake-streams, a JSON map of lookup key to
//	    stream file name) replays committed transcripts instead of shelling
//	    out to `claude -p` — the only spend-free way to exercise this
//	    subcommand (the "dry cell", docs/bench/scenarios.md), and what this
//	    package's own tests use. -corpus lca is the same sentinel plan
//	    documents above; internal/scenariobench.RunExternalCorpus is the
//	    same pipeline as a package-level call, used directly by this
//	    package's own tests.
//
//	scenariobench score -in <cell.json> [-out <path>] [-corpus <path>]
//	    Recomputes a stored cell's basic aggregate readings (total tokens,
//	    turns, cost, payload bytes) from its sessions and rewrites it. Free:
//	    it reads what run already captured. -corpus additionally recomputes
//	    gold_source_tool against the task's own gold; without it the stored
//	    value is left exactly as it was (never overwritten with
//	    not_measured), and a stderr line says so.
//
//	scenariobench ledger -corpus <path|lca> [-dataset <path.jsonl>] -cells <dir> -ledger <path.jsonl> -readme <path> -run-id <id> [-commit <sha>] [-corpus-glob <glob>] [-arm-surface arm=value ...] [-trials N]
//	    Reads every stored cell for -run-id, groups by (task, arm) into
//	    ScenarioLedgerRow, appends them to -ledger, re-renders -readme's
//	    marker-fenced section — a "Where fathom stands" verdict block
//	    (RC-35, scenariobench.RenderVerdict) above the table, built from
//	    every corpus under -corpus-glob — and evaluates the corpus's
//	    registered criterion. -trials, left unset, defaults to the corpus's
//	    own Criterion.MinTrials; whatever it resolves to is only a floor —
//	    a cell key with more trials actually stored is still counted in
//	    full. Exit 0 pass, 1 fail (a measured refutation — recorded, not
//	    excused), 3 indeterminate (re-run: not enough was measured).
//
//	scenariobench void -run-id <id> -task <id> -arm <name> -reason <text> [-cells <dir>] [-ledger <path.jsonl>] [-readme <path>] [-corpus-glob <glob>]
//	    Marks a FILED ledger row's own InstrumentDefect (and every stored
//	    cell record filed under that same run/task/arm) with -reason —
//	    ADR-034 decision 2's own retroactive voiding, for an instrument
//	    defect discovered only after the numbers were already read — then
//	    re-renders -readme's marker-fenced section exactly as `ledger`
//	    does. Refuses, changing nothing, when no ledger row matches
//	    (run_id, task_id, arm): voiding a row that was never filed would
//	    invent a defect claim for a reading that does not exist.
package main

import (
	"context"
	"encoding/json"
	"errors"
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
	// A bench driver never wants the repository of a git hook it may be
	// running under: scrub what git exports to hooks before any subprocess
	// (internal/scenariobench/gitenv.go explains the 2026-09-14 incident).
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Setenv("LEFTHOOK", "0")
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: scenariobench <plan|run|score|ledger|void> [flags]")
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
	case "void":
		return cmdVoid(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (want plan, run, score, ledger, or void)\n", args[0])
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
		// RC-15: a task the corpus marked dropped (an oracle that cannot be
		// won from the task text — SB-cli__cli-10009's fail_to_pass names
		// tests of a package absent at base) never becomes a cell at all —
		// plan/run/ledger all build their cell list from planCells, so
		// none of them ever sees it. `scenariobench score` operates on one
		// already-stored CellResult with no corpus in hand at all, so a
		// dropped task simply never has a cell for it to rescore.
		if task.Disposition == "dropped" {
			continue
		}
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

// batchCellPlans groups cells so a batch never carries two trials of the
// SAME arm — only distinct arms of the same (task, trial) may run
// concurrently under -parallel-arms, since PrepareSubject keys a worktree
// on (subject, base_commit, ARM) with no trial component: two trials of
// one arm sharing a batch would run PrepareSubject/RunSessions
// concurrently against the very same worktree, scratch daemon store, and
// cell dir. planCells emits cells arm-major within a task (every trial of
// arm 1, then every trial of arm 2, ...), so cells sharing a (task,
// trial) key are not generally contiguous in cells' own order — this
// groups by that key directly, in FIRST-SEEN order of the key, so a
// batch's own printed/stored order stays deterministic regardless of
// input order.
func batchCellPlans(cells []cellPlan) [][]cellPlan {
	type taskTrial struct {
		taskID string
		trial  int
	}
	var order []taskTrial
	byKey := map[taskTrial][]cellPlan{}
	for _, cp := range cells {
		key := taskTrial{cp.task.ID, cp.trial}
		if _, ok := byKey[key]; !ok {
			order = append(order, key)
		}
		byKey[key] = append(byKey[key], cp)
	}
	out := make([][]cellPlan, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
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

// externalCorpusSentinel is the -corpus value that switches plan/run/
// ledger from loading a committed corpus FILE to building one in memory
// from an operator-supplied -dataset (VB-25, vision 29 Wave G): Long Code
// Arena's bug-localization track. Only one external corpus is wired so
// far; a second would get its own sentinel, never overload this one.
const externalCorpusSentinel = "lca"

// loadCorpusOrLCA resolves -corpus/-dataset into a Corpus and its
// SHA256, exactly as loadCorpus does for a committed JSON file — except
// when corpusPath is externalCorpusSentinel ("lca"): datasetPath (the
// operator's OWN downloaded Long Code Arena bug-localization JSONL —
// never fetched automatically, docs/bench/scenarios.md "External corpus
// (LCA bug localization)") is loaded via scenariobench.LoadLCARecords and
// converted via scenariobench.BuildLCACorpus, hashed over its own
// canonical JSON bytes (scenariobench.CanonicalCorpusSHA256) so a cell
// key stays stable across identical dataset content regardless of where
// it happens to sit on disk.
func loadCorpusOrLCA(corpusPath, datasetPath string) (scenariobench.Corpus, string, error) {
	if corpusPath != externalCorpusSentinel {
		return loadCorpus(corpusPath)
	}
	if datasetPath == "" {
		return scenariobench.Corpus{}, "", fmt.Errorf("-corpus %s requires -dataset <path.jsonl> (the operator's own downloaded Long Code Arena bug-localization snapshot — never fetched automatically)", externalCorpusSentinel)
	}
	records, err := scenariobench.LoadLCARecords(datasetPath)
	if err != nil {
		return scenariobench.Corpus{}, "", err
	}
	corpus, err := scenariobench.BuildLCACorpus(records, datasetPath)
	if err != nil {
		return scenariobench.Corpus{}, "", err
	}
	sha, err := scenariobench.CanonicalCorpusSHA256(corpus)
	if err != nil {
		return scenariobench.Corpus{}, "", err
	}
	return corpus, sha, nil
}

const datasetFlagUsage = "for -corpus " + externalCorpusSentinel + ": path to the operator's own downloaded Long Code Arena bug-localization JSONL (never fetched automatically — docs/bench/scenarios.md \"External corpus (LCA bug localization)\")"

// criteriaBySHA256 loads every corpus matching glob and returns its
// registered Criterion keyed by the corpus's own SHA256 — the same key a
// ScenarioLedgerRow carries as CorpusSHA256 (RC-35), so RenderVerdict can
// name each run's treatment/control without a hardcoded run→corpus table.
// An empty match (a repo with no corpora yet) is not an error.
func criteriaBySHA256(glob string) (map[string]scenariobench.Criterion, error) {
	paths, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("globbing %q: %w", glob, err)
	}
	out := make(map[string]scenariobench.Criterion, len(paths))
	for _, p := range paths {
		c, sha, err := loadCorpus(p)
		if err != nil {
			return nil, fmt.Errorf("loading corpus %s: %w", p, err)
		}
		out[sha] = c.Criterion
	}
	return out, nil
}

// ─── plan ────────────────────────────────────────────────────────────────

func cmdPlan(args []string) int {
	fs := newFlagSet("plan")
	corpusPath := fs.String("corpus", "", "path to the scenario corpus JSON, or \""+externalCorpusSentinel+"\" to build one from -dataset (required)")
	datasetPath := fs.String("dataset", "", datasetFlagUsage)
	cellsDir := fs.String("cells", filepath.Join("workspace", "artifacts", "scenariobench", "cells"), "content-addressed cell store")
	trials := fs.Int("trials", 1, "trials per (task, arm)")
	escalate := fs.Bool("escalate", false, "RC-14: instead of listing cells, read -ledger and list the n=2 tasks whose treatment/control delta sits within -margin of the corpus criterion's own thresholds — candidates to re-run at n>=3")
	ledgerPath := fs.String("ledger", filepath.Join("docs", "bench", "scenarios-ledger.jsonl"), "-escalate: the ledger JSONL to read")
	margin := fs.Float64("margin", 0.1, "-escalate: how close a delta must sit to the criterion's own threshold to flag a trial escalation")
	runIDFilter := fs.String("run-id", "", "-escalate: restrict to this run id (default: every row in the ledger)")
	cf := addCellFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *corpusPath == "" {
		fmt.Fprintln(os.Stderr, "scenariobench plan: -corpus is required")
		return 2
	}
	corpus, sha, err := loadCorpusOrLCA(*corpusPath, *datasetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench plan: %v\n", err)
		return 2
	}

	if *escalate {
		return cmdPlanEscalate(*ledgerPath, *runIDFilter, corpus.Criterion, *margin)
	}

	if *cf.harnessVersion == "" {
		fmt.Fprintln(os.Stderr, "scenariobench plan: -harness-version is required (a cell key over an unknown harness would let an unrelated cell be borrowed)")
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

// cmdPlanEscalate is RC-14's `scenariobench plan --escalate` path: no cell
// store, no corpus cell listing — just EscalationPlan over ledgerPath's
// rows (optionally narrowed to one run-id), so a screen-then-escalate
// decision costs a ledger read, never a spend.
func cmdPlanEscalate(ledgerPath, runIDFilter string, criterion scenariobench.Criterion, margin float64) int {
	rows, err := scenariobench.LoadScenarioLedger(ledgerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench plan --escalate: %v\n", err)
		return 2
	}
	if runIDFilter != "" {
		filtered := rows[:0]
		for _, r := range rows {
			if r.RunID == runIDFilter {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}

	cells, skipped := scenariobench.EscalationPlan(rows, criterion, margin)
	if len(cells) == 0 {
		fmt.Fprintf(os.Stdout, "scenariobench plan --escalate: no n=2 cell sits within %.2f of %s's thresholds", margin, criterion.ID)
		if skipped > 0 {
			fmt.Fprintf(os.Stdout, " (%d voided instrument_defect row(s) skipped)", skipped)
		}
		fmt.Fprintln(os.Stdout)
		return 0
	}
	for _, c := range cells {
		fmt.Fprintf(os.Stdout, "%s: n=%d success_delta=%.3f tokens_ratio=%.3f — %s\n", c.TaskID, c.Trials, c.SuccessDelta, c.TokensRatio, c.Reason)
	}
	fmt.Fprintf(os.Stdout, "scenariobench plan --escalate: %d cell(s) flagged for n>=3", len(cells))
	if skipped > 0 {
		fmt.Fprintf(os.Stdout, " (%d voided instrument_defect row(s) skipped, never a basis for the recommendation)", skipped)
	}
	fmt.Fprintln(os.Stdout)
	return 0
}

// ─── run ─────────────────────────────────────────────────────────────────

func cmdRun(args []string) int {
	fs := newFlagSet("run")
	corpusPath := fs.String("corpus", "", "path to the scenario corpus JSON, or \""+externalCorpusSentinel+"\" to build one from -dataset (required)")
	datasetPath := fs.String("dataset", "", datasetFlagUsage)
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
	parallelArms := fs.Bool("parallel-arms", false, "run one task's arms concurrently (RC-16) — each cell prepares its own worktree and, when wired, its own scratch daemon on an independently allocated port, so nothing is shared between them; default off")
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
	corpus, sha, err := loadCorpusOrLCA(*corpusPath, *datasetPath)
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

	var selected []cellPlan
	for _, cp := range cells {
		if len(onlyTasks) > 0 && !onlyTasks[cp.task.ID] {
			continue
		}
		if *onlyArm != "" && cp.arm != *onlyArm {
			continue
		}
		selected = append(selected, cp)
	}

	// Batched by (task, trial): batchCellPlans groups cells so a batch
	// never carries two trials of the SAME arm — PrepareSubject keys a
	// worktree on (subject, base_commit, ARM) with no trial component, so
	// two trials of one arm running concurrently under -parallel-arms
	// would race on the very same worktree/scratch-store/cell dir (only
	// DIFFERENT arms are safe to run concurrently — RunArmsConcurrently's
	// own doc comment). RunArmsConcurrently runs a batch's cells in
	// parallel when -parallel-arms asks for it — each prepares its own
	// worktree and, when wired, its own scratch daemon on an
	// independently allocated port, so nothing is shared between them —
	// and outcomes are always processed in the batch's ORIGINAL order, so
	// the ledger's printed/stored order never depends on which goroutine
	// finished first.
	for _, batch := range batchCellPlans(selected) {
		fns := make([]func() (cellOutcome, error), len(batch))
		for k, cp := range batch {
			cp := cp
			fns[k] = func() (cellOutcome, error) {
				return runCellForReport(ctx, *root, cp, corpus, *harnessName, *fakeStreamsDir, *fakeStreams, *cellsDir, absRawDir, *model, *runID)
			}
		}

		var outcomes []cellOutcome
		var errs []error
		if *parallelArms && len(fns) > 1 {
			outcomes, errs = scenariobench.RunArmsConcurrently(fns)
		} else {
			outcomes = make([]cellOutcome, len(fns))
			errs = make([]error, len(fns))
			for k, fn := range fns {
				outcomes[k], errs[k] = fn()
			}
		}

		for k, oc := range outcomes {
			if errs[k] != nil {
				fmt.Fprintf(os.Stderr, "scenariobench run: %v\n", errs[k])
				return 2
			}
			switch {
			case oc.borrowed:
				fmt.Fprintf(os.Stdout, "%s/%s/t%d: borrowed\n", oc.cp.task.ID, oc.cp.arm, oc.cp.trial)
			case oc.lostReason != "":
				// A session that never ran is not a reading: it is not
				// stored (a later run would borrow it as data) and the row
				// counts it lost, exactly as context-bench does (guards
				// pilot 1, 2026-09-13: 16 cells whose harness died at
				// startup were stored, then borrowed by the re-run).
				lost++
				fmt.Fprintf(os.Stderr, "%s/%s/t%d: LOST — %s\n", oc.cp.task.ID, oc.cp.arm, oc.cp.trial, oc.lostReason)
				if strings.Contains(oc.lostReason, "operation not permitted") {
					// A wedged volume verdict sticks to this process: every
					// later cell would be LOST the same way (vs-pilot-2 lost
					// 28 in a row, 2026-09-13). Stop; a relaunch borrows every
					// stored cell and re-runs only the lost ones.
					fmt.Fprintln(os.Stderr, "scenariobench run: the volume refused a read or write (a wedged TCC verdict) — stopping; reset it and relaunch with the same -run-id")
					return 4
				}
			default:
				ran++
				status := "FAIL"
				if oc.result.Success {
					status = "PASS"
				} else {
					anyFailed = true
				}
				fmt.Fprintf(os.Stdout, "%s/%s/t%d: %s tokens=%d cost=%.4f\n", oc.cp.task.ID, oc.cp.arm, oc.cp.trial, status, oc.result.Readings.TotalTokens, oc.result.Readings.TotalCostUSD)
			}
		}
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

// cellOutcome is what running (or borrowing) one cellPlan produced —
// runCellForReport's own result type, so a caller feeding it through
// scenariobench.RunArmsConcurrently (RC-16) or a plain sequential loop
// prints/stores/counts it identically either way.
// errSubjectPrep marks a PrepareSubject failure so runCellForReport can
// turn it into a LOST cell instead of aborting the run.
var errSubjectPrep = errors.New("preparing subject")

// ledgerClock is cmdLedger's own injected clock (internal/CLAUDE.md's
// determinism contract: inject clocks, never call time.Now() directly in
// logic a test needs to pin) — GeneratedAt on every row cmdLedger files
// (O55 item 5). Tests override it for a deterministic timestamp; production
// leaves it as time.Now.
var ledgerClock = time.Now

type cellOutcome struct {
	cp         cellPlan
	borrowed   bool
	lostReason string
	result     scenariobench.CellResult
}

// runCellForReport is cmdRun's per-cell unit of work: borrow-check, then
// (on a miss) resolve the harness, run the cell, apply the LOST gate, and
// store it — returning the outcome as data rather than printing anything
// itself, so the caller can print cells in a fixed order even when several
// ran concurrently (RC-16).
func runCellForReport(ctx context.Context, root string, cp cellPlan, corpus scenariobench.Corpus, harnessName, fakeStreamsDir, fakeStreams, cellsDir, rawDir, model, runID string) (cellOutcome, error) {
	if _, borrowed, err := scenariobench.BorrowScenarioCell(cellsDir, cp.key, cp.trial); err != nil {
		return cellOutcome{cp: cp}, fmt.Errorf("checking cell store for %s/%s/t%d: %w", cp.task.ID, cp.arm, cp.trial, err)
	} else if borrowed {
		return cellOutcome{cp: cp, borrowed: true}, nil
	}

	arm := corpus.Arms[cp.arm]
	harness, herr := resolveHarness(harnessName, fakeStreamsDir, fakeStreams, cp.task.ID, cp.arm)
	if herr != nil {
		return cellOutcome{cp: cp}, herr
	}

	result, err := runOneCell(ctx, root, cp, arm, harness, rawDir, model, runID)
	if err != nil && errors.Is(err, errSubjectPrep) {
		// A subject that could not be prepared (a fetch refused by the
		// volume wedge, a daemon that never came up) is an instrument fault
		// of this cell: LOST, never stored, and the run goes on — one task's
		// clone must not abort a hundred other cells (vs-pilot-2, 2026-09-13).
		return cellOutcome{cp: cp, lostReason: "instrument: " + err.Error()}, nil
	}
	if err != nil {
		return cellOutcome{cp: cp}, fmt.Errorf("%s/%s/t%d: %w", cp.task.ID, cp.arm, cp.trial, err)
	}
	if reason := scenariobench.CellLost(result.Sessions); reason != "" {
		return cellOutcome{cp: cp, lostReason: reason}, nil
	}
	if err := scenariobench.StoreScenarioCell(cellsDir, result); err != nil {
		return cellOutcome{cp: cp}, fmt.Errorf("storing %s/%s/t%d: %w", cp.task.ID, cp.arm, cp.trial, err)
	}
	return cellOutcome{cp: cp, result: result}, nil
}

func runOneCell(ctx context.Context, root string, cp cellPlan, arm scenariobench.Arm, harness scenariobench.Harness, rawDir, model, runID string) (scenariobench.CellResult, error) {
	wt, err := scenariobench.PrepareSubject(ctx, cp.task, arm, root)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("%w: %v", errSubjectPrep, err)
	}
	defer scenariobench.StopWorktree(&wt)

	// The clean room (RC-6) is Claude Code's own --restricted mode
	// (buildClaudeArgs) plus --settings <wt.Arm.SettingsPath> — both
	// derived from req/wt.Arm inside RunSessions/buildClaudeArgs, nothing
	// to wire here. FATHOM_THREADS (per cell) is still merged into the
	// session env by RunSessions itself, keyed on wt.CellDir.
	sessionsStart := time.Now()
	sessions, err := scenariobench.RunSessions(ctx, harness, cp.task, cp.arm, arm, wt, scenariobench.RunOptions{
		RunID: runID,
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
	diffPath, instrumentFilesExcluded := captureAgentDiff(ctx, wt)

	oracle, err := scenariobench.RunOracle(ctx, wt, cp.task, sessions, root)
	if err != nil {
		return scenariobench.CellResult{}, fmt.Errorf("running oracle: %w", err)
	}

	readings := scenariobench.AggregateReadings(sessions)
	readings.SurfaceCalls = scenariobench.SurfaceCallCount(readings.ToolCalls, arm.Surface)
	scenariobench.ApplyCompileCost(&readings, wt)
	boxChecks := allBoxChecks(sessions)

	planCoverage := scenariobench.PlanCoverage(planBytes, cp.task.Gold)
	readings.PlanCoverage = &planCoverage

	falseDone := scenariobench.FalseDoneRate(boxChecks, wt.Path, oracle.CoverProfile)
	readings.FalseDone = &falseDone

	resumption := scenariobench.ResumptionReadings(sessions, provenFromFalseDone(sessions, falseDone))
	readings.Resumption = &resumption

	drift := scenariobench.ContractDrift(planBytes, wt.Path, oracle.CoverProfile)
	readings.Drift = &drift

	var rowsAssigned, rowsUnassigned *int
	if cp.task.Sessions.Mode == scenariobench.ModePlannerWorkers {
		overlap := scenariobench.ReexplorationOverlap(sessions)
		readings.Overlap = &overlap

		// O55 item 3: rows_assigned counts the worker sessions RunSessions'
		// reassignment loop actually ran (every session but the planner);
		// rows_unassigned re-reads the plan file AFTER sessions finished —
		// whatever RunSessions' own maxTotal cap (or an errored worker)
		// left unchecked.
		assigned := 0
		for _, s := range sessions {
			if s.Role == "worker" {
				assigned++
			}
		}
		rowsAssigned = &assigned
		if n, err := scenariobench.RemainingPlanRows(wt.Path); err == nil {
			rowsUnassigned = &n
		}
	}

	if cp.task.Rule != nil {
		if turn, ok := firstClaimTurn(sessions); ok {
			readings.RuleServed = scenariobench.RuleServed(hooksLogPath(wt), cp.task.Rule.ID, turn)
		}
		// RC-12 (vision 28 §2.4, F12): the direct transcript reading,
		// against the task's own gold files (what the accepted change
		// touched is exactly what a guards task's rule governs). Left nil
		// (not measurable) when the task carries no gold files to check
		// against — never a guessed governed path.
		readings.RuleServedBeforeEdit = ruleServedBeforeEditForCell(sessions, cp.task.Gold.Files)
	}
	readings.Violation = cellViolation(oracle)

	readings.GoldSourceTool, readings.GoldSourceEvidence = scenariobench.GoldSourceTool(sessions, cp.task)
	// VB-25: carry the oracle's own localization verdict (set only for a
	// task with Oracle.Localization) onto the cell's Readings, exactly as
	// PlanCoverage/FalseDone/Drift already are above.
	readings.Localization = oracle.Localization

	return scenariobench.CellResult{
		TaskID:   cp.task.ID,
		Arm:      cp.arm,
		Trial:    cp.trial,
		Category: cp.task.Category,
		Sessions: sessions,
		Oracle:   oracle,
		Readings: readings,
		// An untouched tree cannot pass (guards pilot 1) — UNLESS the
		// task's oracle is answer-only (SB-32 lookups-vs.json) or
		// localization-only (VB-25, the LCA bug-localization track),
		// where nothing was ever meant to be edited.
		Success:                 oracle.Passed && (scenariobench.AnswerOnly(cp.task) || scenariobench.LocalizationOnly(cp.task) || scenariobench.AnyEdits(sessions)),
		CellKey:                 cp.key,
		CorpusSHA256:            cp.inv.CorpusSHA256,
		ModelID:                 cp.inv.ModelID,
		HarnessVersion:          cp.inv.HarnessVersion,
		ArmSurface:              cp.inv.ArmSurface,
		Surface:                 arm.Surface,
		Gold:                    cp.task.Gold,
		PlanPath:                planPath,
		AgentDiffPath:           diffPath,
		RunID:                   runID,
		StartedAt:               "", // TODO(SB-19 follow-up): inject a clock; not needed for wave 2's own tests
		WorktreePath:            wt.Path,
		SubjectClone:            wt.SubjectClone,
		BaseCommit:              cp.task.Subject.BaseCommit,
		RowsAssigned:            rowsAssigned,
		RowsUnassigned:          rowsUnassigned,
		InstrumentFilesExcluded: instrumentFilesExcluded,
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

// ruleServedBeforeEditForCell is RC-12's per-cell wiring over
// RuleServedBeforeEdit: sessions run in chronological order, so the FIRST
// session where the reading is measurable (it edited a governedPath) is
// the one whose verdict answers the cell's own question. nil (not
// measurable) when governedPaths is empty, or no session ever edited one.
func ruleServedBeforeEditForCell(sessions []scenariobench.SessionResult, governedPaths []string) *bool {
	if len(governedPaths) == 0 {
		return nil
	}
	for _, s := range sessions {
		if got := scenariobench.RuleServedBeforeEdit(s, governedPaths); got != nil {
			return got
		}
	}
	return nil
}

// cellViolation is RC-12: the rule detectors' own outcome, already computed
// by RunOracle, carried onto the row — true when ANY detector reported
// Violated, nil when the task registered no detector at all (a task with
// no rule to violate answers no question, not a false "no violation").
func cellViolation(oracle scenariobench.OracleResult) *bool {
	if len(oracle.Detectors) == 0 {
		return nil
	}
	violated := false
	for _, d := range oracle.Detectors {
		if d.Violated {
			violated = true
			break
		}
	}
	return &violated
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
// the sessions changed since PrepareSubject reset the tree to base_commit
// — MINUS whatever PrepareSubject's own instrument wrote (O55 item 4:
// `fathom init`'s wiring — the CLAUDE.md/AGENTS.md stanza, everything
// internal/wiring's ledger records, .fathomignore, anything under
// workspace/ — is never the agent's own work and must never be credited to
// it in agent.diff or any byte count derived from it). Returns the
// captured diff's path ("" on any error — a captured diff is a convenience
// for later re-scoring, not a precondition for the readings computed here,
// which read the live worktree directly) and how many whole-file sections
// were excluded, for CellResult.InstrumentFilesExcluded.
func captureAgentDiff(ctx context.Context, wt scenariobench.Worktree) (path string, instrumentFilesExcluded int) {
	add := exec.CommandContext(ctx, "git", "add", "-N", ".")
	add.Dir = wt.Path
	if _, err := add.CombinedOutput(); err != nil {
		return "", 0
	}
	diff := exec.CommandContext(ctx, "git", "diff")
	diff.Dir = wt.Path
	out, err := diff.Output()
	if err != nil {
		return "", 0
	}
	// Region-aware first (CLAUDE.md/AGENTS.md's own fathom stanza — never
	// ledger-tracked, O55 item 4 fix-first review), then the ledger/
	// workspace-driven whole-file exclusion; the two act on disjoint files,
	// so order between them does not matter, and their counts simply add.
	var regionsExcluded int
	out, regionsExcluded = scenariobench.StripInstrumentRegions(ctx, wt, out)
	var filesExcluded int
	if owned, err := scenariobench.InstrumentOwnedPaths(wt); err == nil {
		// workspace/ is THIS repo's own convention, never assumed of an
		// arbitrary subject (O55 item 4, fix-first review) — only a
		// workspace/ path the SUBJECT's own .gitignore actually ignores
		// is excluded; GitIgnoredWorkspacePaths queries just the
		// workspace/ paths this diff names.
		gitIgnored := scenariobench.GitIgnoredWorkspacePaths(ctx, wt, out)
		out, filesExcluded = scenariobench.FilterInstrumentOwnedDiff(out, owned, gitIgnored)
	}
	instrumentFilesExcluded = regionsExcluded + filesExcluded
	dest := filepath.Join(wt.CellDir, "agent.diff")
	if err := os.WriteFile(dest, out, 0o644); err != nil {
		return "", 0
	}
	return dest, instrumentFilesExcluded
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
	corpusPath := fs.String("corpus", "", "path to the scenario corpus JSON — recomputes gold_source_tool against the task's own gold; without it, the stored value is left untouched (never overwritten with not_measured)")
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
	// AggregateReadings recomputes Readings from scratch (sessions only) —
	// the compile-cost reading (RC-10) comes from a `fathom compile` run
	// outside any session, so it survives the replacement only if carried
	// across by hand, USD rolled back into the recomputed total exactly as
	// ApplyCompileCost would from a live Worktree (which score never has).
	// GoldSourceTool/GoldSourceEvidence survive the same way — AggregateReadings
	// does not touch them at all, so recomputing needs the corpus (below) and
	// NOT recomputing must still preserve whatever was already stored, never
	// silently reset it to empty (which the ledger's own sentinel reads as
	// GoldSourceNotMeasured — a false regression, not an honest not-measured).
	compileCost := cell.Readings.CompileCost
	compileCostUSD := cell.Readings.CompileCostUSD
	goldSourceTool := cell.Readings.GoldSourceTool
	goldSourceEvidence := cell.Readings.GoldSourceEvidence
	cell.Readings = scenariobench.AggregateReadings(cell.Sessions)
	cell.Readings.CompileCost = compileCost
	cell.Readings.CompileCostUSD = compileCostUSD
	if compileCost != nil {
		cell.Readings.TotalCostUSD += compileCostUSD
	}
	if *corpusPath == "" {
		fmt.Fprintln(os.Stderr, "scenariobench score: gold_source_tool: not recomputed (no -corpus)")
		cell.Readings.GoldSourceTool = goldSourceTool
		cell.Readings.GoldSourceEvidence = goldSourceEvidence
	} else {
		corpus, _, err := loadCorpus(*corpusPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench score: %v\n", err)
			return 2
		}
		task, found := scenariobench.Task{}, false
		for _, t := range corpus.Tasks {
			if t.ID == cell.TaskID {
				task, found = t, true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "scenariobench score: task %q not found in %s — gold_source_tool not recomputed\n", cell.TaskID, *corpusPath)
			cell.Readings.GoldSourceTool = goldSourceTool
			cell.Readings.GoldSourceEvidence = goldSourceEvidence
		} else {
			cell.Readings.GoldSourceTool, cell.Readings.GoldSourceEvidence = scenariobench.GoldSourceTool(cell.Sessions, task)
		}
	}
	cell.Readings.SurfaceCalls = scenariobench.SurfaceCallCount(cell.Readings.ToolCalls, cell.Surface)
	if cell.Surface != "" && cell.Readings.SurfaceCalls == 0 {
		fmt.Fprintf(os.Stderr, "scenariobench score: %s/%s/t%d: surface %q unused — zero %s* tool calls across the cell\n",
			cell.TaskID, cell.Arm, cell.Trial, cell.Surface, scenariobench.SurfaceToolPrefix(cell.Surface))
	}
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

// borrowRunMismatchError builds cmdLedger's -strict-borrow refusal message
// (O55 item 5): a stored cell record exists at key/trial, but names a
// different run than -run-id expects — usually -cells pointed at the wrong
// store, or -run-id was mistyped, not "this cell just wasn't part of this
// run" (that case is !ok, handled separately). Names the store it actually
// resolved (storeRoot, CellResult.StoreRoot — BorrowScenarioCell's own
// cellsDir) so the operator has something to check.
func borrowRunMismatchError(taskID, arm string, trial int, key, storeRoot, gotRunID, wantRunID string) error {
	return fmt.Errorf("cell %s/%s/t%d (key %s) in store %s was filed under run %q, not the expected run %q — refusing to borrow (pass -strict-borrow=false to allow across-run borrowing)",
		taskID, arm, trial, key, storeRoot, gotRunID, wantRunID)
}

func cmdLedger(args []string) int {
	fs := newFlagSet("ledger")
	corpusPath := fs.String("corpus", "", "path to the scenario corpus JSON, or \""+externalCorpusSentinel+"\" to build one from -dataset (required)")
	datasetPath := fs.String("dataset", "", datasetFlagUsage)
	cellsDir := fs.String("cells", filepath.Join("workspace", "artifacts", "scenariobench", "cells"), "content-addressed cell store")
	ledgerPath := fs.String("ledger", filepath.Join("docs", "bench", "scenarios-ledger.jsonl"), "committed JSONL ledger")
	readmePath := fs.String("readme", filepath.Join("docs", "bench", "README.md"), "README carrying the marker-fenced scenario ledger table")
	corpusGlob := fs.String("corpus-glob", filepath.Join("testdata", "scenariobench", "*.json"), "every corpus whose criterion the verdict block may cite, keyed by corpus SHA256")
	runID := fs.String("run-id", "", "the run whose cells to file (required)")
	commit := fs.String("commit", "", "repo HEAD this run measured (default: git rev-parse --short HEAD)")
	trials := fs.Int("trials", 0, "trials per (task, arm) this run intended (default: the corpus criterion's own min_trials; a cell key with MORE stored trials than this is still counted in full, never capped)")
	strictBorrow := fs.Bool("strict-borrow", true, "refuse (exit 2) to file a row when a stored cell's own run_id differs from -run-id, rather than silently skipping it — false restores the old silent-skip behavior for a cellsDir deliberately shared across runs")
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
	corpus, sha, err := loadCorpusOrLCA(*corpusPath, *datasetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
		return 2
	}
	// O55 item 5: RFC3339 UTC from the injected clock — every filed row
	// carries WHEN this filing happened, deterministic in tests
	// (ledgerClock), real wall time in production.
	generatedAt := ledgerClock().UTC().Format(time.RFC3339)
	resolvedCommit := *commit
	if resolvedCommit == "" {
		resolvedCommit = resolveFathomCommit("")
	}

	// -trials unset (0) falls back to the corpus's OWN registered floor
	// (Criterion.MinTrials) rather than a bare 1 — a run that intended
	// only 1 trial passes -trials 1 explicitly; leaving it unset means
	// "count what the corpus itself commits to".
	effectiveTrials := *trials
	if effectiveTrials <= 0 {
		effectiveTrials = corpus.Criterion.MinTrials
		if effectiveTrials <= 0 {
			effectiveTrials = 1
		}
	}

	cells := planCells(corpus, cf, effectiveTrials, sha)
	type key struct{ task, arm string }
	byTaskArm := map[key][]scenariobench.CellResult{}
	seenTrial := map[string]map[int]bool{} // primary cell key -> trials already fetched
	primaryKeyOf := map[key]string{}       // (task,arm) -> its own primary (never alt) cell key
	for _, cp := range cells {
		if _, exists := primaryKeyOf[key{cp.task.ID, cp.arm}]; !exists {
			primaryKeyOf[key{cp.task.ID, cp.arm}] = cp.key
		}
		cell, ok, err := scenariobench.BorrowScenarioCell(*cellsDir, cp.key, cp.trial)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench ledger: reading cell %s/%s/t%d: %v\n", cp.task.ID, cp.arm, cp.trial, err)
			return 2
		}
		resolvedKey := cp.key
		if seenTrial[cp.key] == nil {
			seenTrial[cp.key] = map[int]bool{}
		}
		seenTrial[cp.key][cp.trial] = true
		for _, alt := range cp.altKeys {
			if ok {
				break
			}
			cell, ok, err = scenariobench.BorrowScenarioCell(*cellsDir, alt, cp.trial)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
				return 2
			}
			resolvedKey = alt
		}
		if !ok {
			continue
		}
		if cell.RunID != *runID {
			if *strictBorrow {
				fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", borrowRunMismatchError(cp.task.ID, cp.arm, cp.trial, resolvedKey, cell.StoreRoot, cell.RunID, *runID))
				return 2
			}
			continue
		}
		k := key{cp.task.ID, cp.arm}
		byTaskArm[k] = append(byTaskArm[k], cell)
	}

	// A cell key may carry MORE trials on disk than effectiveTrials asked
	// for (a run that stored t1 and t2 while -trials defaulted to 1 for
	// this invocation) — count every one of them, never silently drop the
	// extra (docs/bench/scenarios.md's ledger-row defect, 2026-09-13).
	// Only the PRIMARY key is probed: a cell that only ever matched under
	// an alt fathom-commit key (rare, multi-commit filing) is not
	// re-discovered here beyond what planCells already asked for.
	for k, primaryKey := range primaryKeyOf {
		stored, err := scenariobench.StoredScenarioTrials(*cellsDir, primaryKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
			return 2
		}
		for _, trial := range stored {
			if seenTrial[primaryKey][trial] {
				continue
			}
			cell, ok, err := scenariobench.BorrowScenarioCell(*cellsDir, primaryKey, trial)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scenariobench ledger: reading cell %s/t%d: %v\n", primaryKey, trial, err)
				return 2
			}
			seenTrial[primaryKey][trial] = true
			if !ok {
				continue
			}
			if cell.RunID != *runID {
				if *strictBorrow {
					fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", borrowRunMismatchError(k.task, k.arm, trial, primaryKey, cell.StoreRoot, cell.RunID, *runID))
					return 2
				}
				continue
			}
			byTaskArm[k] = append(byTaskArm[k], cell)
		}
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
		// AggregateRowReadings is the single source of truth for a row's
		// own Readings (every count summed, every ratio averaged with its
		// own n) plus the full per-trial detail nothing else here
		// recomputes by hand — the fix for the row-aggregation defect
		// that dropped every reading but a handful (docs/bench/scenarios.md).
		row.Readings, row.PerTrial = scenariobench.AggregateRowReadings(group)

		goldCounts := map[string]int{}
		goldPassCounts := map[string]int{}
		for _, cell := range group {
			row.Trials++
			if cell.Success {
				row.Successes++
			}
			row.CostUSD += cell.Readings.TotalCostUSD
			// VB-4: a cell scored before this reading existed carries no
			// GoldSourceTool at all — GoldSourceNotMeasured names that,
			// never blended silently into GoldSourceNone ("the agent knew
			// or guessed"), which is itself a finding.
			tool := cell.Readings.GoldSourceTool
			if tool == "" {
				tool = scenariobench.GoldSourceNotMeasured
			}
			goldCounts[tool]++
			if cell.Success {
				goldPassCounts[tool]++
			}
		}
		if row.Trials > 0 {
			row.GoldSourceRate = make(map[string]float64, len(goldCounts))
			for tool, n := range goldCounts {
				row.GoldSourceRate[tool] = float64(n) / float64(row.Trials)
			}
		}
		if len(goldPassCounts) > 0 {
			row.GoldSourcePasses = goldPassCounts
		}
		row.Prediction = taskByID[k.task].Predictions[k.arm]
		surface := corpus.Arms[k.arm].Surface
		row.Readings.SurfaceCalls = scenariobench.SurfaceCallCount(row.Readings.ToolCalls, surface)
		if surface != "" && row.Readings.SurfaceCalls == 0 {
			row.SurfaceUnused = true
		}
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
	// RC-35: the "Where fathom stands" block goes ABOVE the table, built
	// from every corpus under -corpus-glob (keyed by its own SHA256, which
	// each row already carries) — not only the one this invocation loaded
	// — so the block stays informative across the whole committed history,
	// not just the run being filed.
	criteria, err := criteriaBySHA256(*corpusGlob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench ledger: %v\n", err)
		return 2
	}
	body := scenariobench.RenderVerdict(all, criteria) + "\n" + scenariobench.RenderScenarioLedger(all)
	if shapeTable := scenariobench.RenderGoldSourceByCategory(scenariobench.GoldSourceRateByCategory(all)); shapeTable != "" {
		body += "\n" + shapeTable
	}
	if err := scenariobench.RewriteScenarioLedgerSection(*readmePath, body); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench ledger: rendering %s: %v\n", *readmePath, err)
		return 2
	}

	ev := corpus.Criterion.Evaluate(rows)
	trialCounts := make([]string, 0, len(rows))
	for _, row := range rows {
		trialCounts = append(trialCounts, fmt.Sprintf("%s/%s:%d", row.TaskID, row.Arm, row.Trials))
	}
	fmt.Fprintf(os.Stdout, "scenariobench ledger: filed %d row(s) [%s]; criterion %s: %s (%s)\n",
		len(rows), strings.Join(trialCounts, ", "), ev.CriterionID, ev.Verdict, ev.Reason)
	switch ev.Verdict {
	case scenariobench.ScenarioVerdictPass:
		return 0
	case scenariobench.ScenarioVerdictIndeterminate:
		return 3
	default:
		return 1
	}
}

// ─── void ────────────────────────────────────────────────────────────────

// cmdVoid is O53 defect 7: `scenariobench void` marks a FILED ledger
// row's own InstrumentDefect (and every matching stored cell record's)
// after the fact — a defect discovered only once the numbers were
// already read, ADR-034 decision 2's own escape hatch, which had no
// command to drive it before this one. It refuses when the row is
// absent: voiding something that was never filed would silently invent a
// defect claim for a reading that does not exist.
func cmdVoid(args []string) int {
	fs := newFlagSet("void")
	cellsDir := fs.String("cells", filepath.Join("workspace", "artifacts", "scenariobench", "cells"), "content-addressed cell store")
	ledgerPath := fs.String("ledger", filepath.Join("docs", "bench", "scenarios-ledger.jsonl"), "committed JSONL ledger")
	readmePath := fs.String("readme", filepath.Join("docs", "bench", "README.md"), "README carrying the marker-fenced scenario ledger table")
	corpusGlob := fs.String("corpus-glob", filepath.Join("testdata", "scenariobench", "*.json"), "every corpus whose criterion the re-rendered verdict block may cite, keyed by corpus SHA256")
	runID := fs.String("run-id", "", "the run whose row to void (required)")
	taskID := fs.String("task", "", "the task id (required)")
	arm := fs.String("arm", "", "the arm name (required)")
	reason := fs.String("reason", "", "the instrument defect naming why this row is voided (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *runID == "" || *taskID == "" || *arm == "" || *reason == "" {
		fmt.Fprintln(os.Stderr, "scenariobench void: -run-id, -task, -arm, and -reason are required")
		return 2
	}

	if err := scenariobench.VoidScenarioLedgerRow(*ledgerPath, *runID, *taskID, *arm, *reason); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench void: %v\n", err)
		return 2
	}

	// "and the cell record": every stored cell filed under this exact
	// (run_id, task_id, arm) — never only the ledger row's own summary —
	// carries the same InstrumentDefect, so a reader of the cell store
	// directly sees the same fact the ledger does.
	cells, err := scenariobench.FindScenarioCellsByRun(*cellsDir, *taskID, *arm, *runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench void: %v\n", err)
		return 2
	}
	for _, c := range cells {
		c.InstrumentDefect = *reason
		if err := scenariobench.OverwriteScenarioCell(*cellsDir, c); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench void: %v\n", err)
			return 2
		}
	}

	// Re-render the README block exactly as `ledger` does: the SAME
	// three pieces (verdict, table, gold-source-by-category), from the
	// ledger's own current (now-voided) contents.
	rows, err := scenariobench.LoadScenarioLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench void: %v\n", err)
		return 2
	}
	criteria, err := criteriaBySHA256(*corpusGlob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench void: %v\n", err)
		return 2
	}
	body := scenariobench.RenderVerdict(rows, criteria) + "\n" + scenariobench.RenderScenarioLedger(rows)
	if shapeTable := scenariobench.RenderGoldSourceByCategory(scenariobench.GoldSourceRateByCategory(rows)); shapeTable != "" {
		body += "\n" + shapeTable
	}
	if err := scenariobench.RewriteScenarioLedgerSection(*readmePath, body); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench void: rendering %s: %v\n", *readmePath, err)
		return 2
	}

	fmt.Fprintf(os.Stdout, "scenariobench void: voided run %s task %s arm %s (%d cell record(s)): %s\n", *runID, *taskID, *arm, len(cells), *reason)
	return 0
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

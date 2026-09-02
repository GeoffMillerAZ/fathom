// Command contextbenchscore is the thin CLI wrapper scripts/context-bench.sh
// uses to score one claude -p transcript and to assemble the final results
// artifact, over the tested internal/contextbench package (docs/plans/
// falsifiability.md B1-2). All scoring/aggregation logic lives in
// internal/contextbench and is unit-tested there; this binary wires flags
// to that library and writes files. Its exit-code contracts (ledger,
// regress) are pinned by tests in this package.
//
// Subcommands:
//
//	contextbenchscore tasks -tasks <tasks.json> [-env a,b]
//	    Emits the runnable tasks as NUL-separated id/prompt pairs for the
//	    runner, skipping (to stderr) tasks whose `requires` the environment
//	    does not satisfy (RequiresSatisfied, PB-12).
//
//	contextbenchscore score  -tasks <tasks.json> -task-id <id> -arm <name> -claude-json <path> -out <path> [-trial N] [-scoring substring|precision] [-transcript <stream.jsonl>]
//	    Scores one arm's raw `claude -p --output-format json` output against
//	    one task and writes an ArmResult JSON to -out. -scoring precision
//	    applies H2-2's fenced-block rule; -transcript reads a stream-json
//	    transcript for tokens-to-first-gold-hit and tool-call counts.
//
//	contextbenchscore report -tasks <tasks.json> -results <dir of *.json ArmResults> -out-json <path> -out-md <path>
//	    Reads every ArmResult JSON file in -results (non-recursive), builds
//	    the aggregated RunResults artifact, and writes both the JSON results
//	    file and the rendered Markdown summary table.
//
//	contextbenchscore ledger -tasks <tasks.json> -results <dir> -ledger <path.jsonl> -run-id <id> -commit <sha> [-model <m>] [-criterion H2-1] [-treatment fathom] [-control grep] [-arms "fathom grep none"] [-repeat N] [-dirty-tree] [-out-md <path>]
//	    Applies the corpus's committed leak rule, pairs the two arms,
//	    evaluates the named criterion (default: the machine_criterion
//	    block) against the paired summary, appends one row — per-task
//	    outcomes, corpus hash, resolved model id, cost, cells lost — to the
//	    committed bench ledger, and exits 0 for a pass, 1 for a fail, and 3
//	    for indeterminate. The exit code is the gate: a fail is a measured
//	    refutation and a legitimate result to record, while indeterminate
//	    means the run did not measure enough to decide and must be re-run.
//
//	contextbenchscore regress -ledger <path.jsonl> [-baseline <run_id>]
//	    Compares the newest row against the previous row with the same
//	    corpus_sha256 (or the named baseline): a task whose treatment
//	    success rate fell is a regression; flaky tasks are listed, not
//	    failed; added tasks are not regressions. Exit 0 / 1 / 3 as above.
//	    Free: it reads the ledger and runs nothing.
//
//	contextbenchscore backfill ...
//	    Appends one row for a run that predates the ledger, from its plan's
//	    recorded numbers; see cmdBackfill.
//
//	contextbenchscore import -multi-swe <jsonl> -swe-explore <jsonl> -checkouts <dir> [-callers-dir <dir>] -out <path> [-n 35]
//	    Derives the public localization corpus from pinned local snapshots
//	    with no LLM and no network (import.go; docs/bench/public-loc.md).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: contextbenchscore <tasks|score|report|ledger|regress|backfill|import> [flags]")
		return 2
	}
	switch args[0] {
	case "tasks":
		return cmdTasks(args[1:])
	case "score":
		return cmdScore(args[1:])
	case "report":
		return cmdReport(args[1:])
	case "ledger":
		return cmdLedger(args[1:])
	case "regress":
		return cmdRegress(args[1:])
	case "backfill":
		return cmdBackfill(args[1:])
	case "import":
		return cmdImport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (want tasks, score, report, ledger, regress, backfill, or import)\n", args[0])
		return 2
	}
}

// splitList splits a comma- or space-separated flag value into its
// non-empty items.
func splitList(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// cmdTasks emits the runnable subset of a corpus for the shell runner:
// id NUL prompt NUL per task, in corpus order. Tasks whose `requires` the
// environment (-env) does not cover are named on stderr and skipped —
// never scored as misses.
func cmdTasks(args []string) int {
	fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to the corpus JSON")
	env := fs.String("env", "", "comma-separated capabilities the environment provides (e.g. compiled-layer)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tasksPath == "" {
		fmt.Fprintln(os.Stderr, "tasks: -tasks is required")
		return 2
	}
	tasks, err := contextbench.LoadTasks(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tasks: %v\n", err)
		return 1
	}
	have := splitList(*env)
	for _, t := range tasks {
		if !contextbench.RequiresSatisfied(t, have) {
			fmt.Fprintf(os.Stderr, "tasks: skipping %s — requires %v, env provides %v\n", t.ID, t.Requires, have)
			continue
		}
		fmt.Printf("%s\x00%s\x00", t.ID, t.Prompt)
	}
	return 0
}

func cmdScore(args []string) int {
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to tasks.json")
	taskID := fs.String("task-id", "", "task id to score")
	arm := fs.String("arm", "", "arm name (e.g. fathom, grep, none)")
	claudeJSONPath := fs.String("claude-json", "", "path to captured `claude -p --output-format json` output")
	outPath := fs.String("out", "", "path to write the scored ArmResult JSON")
	trial := fs.Int("trial", 0, "1-based repetition index within this run (0 = single-trial run)")
	scoring := fs.String("scoring", contextbench.ScoringSubstring, "substring (H2-1) or precision (H2-2: fenced answer block within max_predicted)")
	transcript := fs.String("transcript", "", "optional stream-json transcript; fills tokens_to_first_gold_hit and tool_calls")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tasksPath == "" || *taskID == "" || *arm == "" || *claudeJSONPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "score: -tasks, -task-id, -arm, -claude-json, and -out are all required")
		return 2
	}

	tasks, err := contextbench.LoadTasks(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "score: %v\n", err)
		return 1
	}
	var task *contextbench.Task
	for i := range tasks {
		if tasks[i].ID == *taskID {
			task = &tasks[i]
			break
		}
	}
	if task == nil {
		fmt.Fprintf(os.Stderr, "score: task %q not found in %s\n", *taskID, *tasksPath)
		return 1
	}

	raw, err := os.ReadFile(*claudeJSONPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "score: reading %s: %v\n", *claudeJSONPath, err)
		return 1
	}

	var ar contextbench.ArmResult
	switch *scoring {
	case contextbench.ScoringSubstring:
		ar, err = contextbench.ArmResultFromClaudeJSON(*task, *arm, raw)
	case contextbench.ScoringPrecision:
		ar, err = contextbench.ArmResultFromClaudeJSONWithPrecision(*task, *arm, raw)
	default:
		fmt.Fprintf(os.Stderr, "score: unknown -scoring %q (want substring or precision)\n", *scoring)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "score: %v\n", err)
		return 1
	}
	ar.Trial = *trial
	if *transcript != "" {
		tr, err := os.ReadFile(*transcript)
		if err != nil {
			fmt.Fprintf(os.Stderr, "score: reading transcript %s: %v\n", *transcript, err)
			return 1
		}
		hit := contextbench.FirstGoldHit(tr, task.GroundTruth)
		ar.ToolCalls = hit.ToolCalls
		if hit.Found {
			ar.TokensToFirstGoldHit = hit.TokensToFirstHit
		}
	}

	out, err := json.MarshalIndent(ar, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "score: marshaling result: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "score: writing %s: %v\n", *outPath, err)
		return 1
	}

	status := "FAIL"
	if ar.Success {
		status = "PASS"
	}
	fmt.Printf("%s %s/%s: %s (tokens=%d wall_ms=%d)\n", status, ar.TaskID, ar.Arm, status, ar.TotalTokens, ar.WallTimeMS)
	return 0
}

func cmdReport(args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to tasks.json (used only to record provenance in the results artifact)")
	resultsDir := fs.String("results", "", "directory of per-task-per-arm ArmResult JSON files (non-recursive)")
	outJSON := fs.String("out-json", "", "path to write the aggregated results JSON")
	outMD := fs.String("out-md", "", "path to write the rendered Markdown summary table")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *resultsDir == "" || *outJSON == "" || *outMD == "" {
		fmt.Fprintln(os.Stderr, "report: -results, -out-json, and -out-md are all required")
		return 2
	}

	results, err := loadResults(*resultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "report: %v\n", err)
		return 1
	}

	run := contextbench.NewRunResults(time.Now().UTC().Format(time.RFC3339), *tasksPath, results)

	outBytes, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "report: marshaling results: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*outJSON, outBytes, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "report: writing %s: %v\n", *outJSON, err)
		return 1
	}

	md := contextbench.RenderMarkdown(results)
	if err := os.WriteFile(*outMD, []byte(md), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "report: writing %s: %v\n", *outMD, err)
		return 1
	}

	fmt.Printf("wrote %s and %s (%d results)\n", *outJSON, *outMD, len(results))
	return 0
}

// loadResults reads every ArmResult JSON file directly under dir, in sorted
// filename order so aggregation is deterministic.
func loadResults(dir string) ([]contextbench.ArmResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var results []contextbench.ArmResult
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		var ar contextbench.ArmResult
		if err := json.Unmarshal(raw, &ar); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		results = append(results, ar)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no ArmResult *.json files found in %s", dir)
	}
	return results, nil
}

// cmdLedger pairs the arms, evaluates the committed criterion, and appends
// the run to the bench ledger. Exit codes: 0 pass, 1 fail, 3 indeterminate,
// 2 usage/IO. Indeterminate is deliberately NOT folded into fail — they
// demand different responses (re-run vs. accept the refutation).
func cmdLedger(args []string) int {
	fs := flag.NewFlagSet("ledger", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to the corpus JSON (carries the committed criteria)")
	resultsDir := fs.String("results", "", "directory of per-task-per-arm-per-trial ArmResult JSON files")
	ledgerPath := fs.String("ledger", "docs/bench/ledger.jsonl", "path to the committed JSONL bench ledger")
	runID := fs.String("run-id", "", "run identifier (the runner's timestamp)")
	commit := fs.String("commit", "", "repo HEAD the run measured")
	model := fs.String("model", "", "model alias both arms ran on")
	criterionID := fs.String("criterion", "", "registered criterion id to evaluate (default: the machine_criterion block)")
	treatment := fs.String("treatment", "fathom", "treatment arm name")
	control := fs.String("control", "grep", "control arm name")
	arms := fs.String("arms", "", "every arm the run intended (space/comma separated), for the cells_lost count; default: treatment and control")
	repeat := fs.Int("repeat", 1, "trials per cell the run intended, for the cells_lost count")
	env := fs.String("env", "", "capabilities the run's environment provided; tasks whose requires it does not cover were never intended and are not counted lost")
	dirty := fs.Bool("dirty-tree", false, "the working tree had uncommitted changes when the run measured it")
	outMD := fs.String("out-md", "", "optional path to render the whole ledger as a Markdown table")
	notes := fs.String("notes", "", "optional free-text note recorded with the row")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tasksPath == "" || *resultsDir == "" || *runID == "" || *commit == "" {
		fmt.Fprintln(os.Stderr, "ledger: -tasks, -results, -run-id, and -commit are all required")
		return 2
	}

	var criterion contextbench.Criterion
	var err error
	if *criterionID == "" {
		criterion, err = contextbench.LoadCriterion(*tasksPath)
	} else {
		criterion, err = contextbench.LoadCriterionByID(*tasksPath, *criterionID)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
		return 2
	}
	tasks, err := contextbench.LoadTasks(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
		return 2
	}
	corpusSHA, err := contextbench.CorpusSHA256(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
		return 2
	}
	results, err := loadResults(*resultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
		return 2
	}

	intended := splitList(*arms)
	if len(intended) == 0 {
		intended = []string{*treatment, *control}
	}
	have := splitList(*env)
	var runnable []contextbench.Task
	for _, t := range tasks {
		if contextbench.RequiresSatisfied(t, have) {
			runnable = append(runnable, t)
		}
	}
	cellsLost := contextbench.CellsLost(runnable, intended, *repeat, results)

	var leaked []string
	if criterion.LeakRule != nil {
		leaked = contextbench.LeakedTasks(results, *criterion.LeakRule)
		results = contextbench.WithoutTasks(results, leaked)
	}

	paired := contextbench.Pair(results, *treatment, *control)
	ev := criterion.Evaluate(paired)
	row := contextbench.NewLedgerRow(*runID, time.Now().UTC().Format(time.RFC3339), *commit, *tasksPath, *model, paired, ev)
	row.Notes = *notes
	row.PerTask = paired.PerTask
	row.CorpusSHA256 = corpusSHA
	row.DirtyTree = *dirty
	row.CellsLost = cellsLost
	row.LeakedTasks = leaked
	// Reported, never gating (PB-10): the verdict above is already fixed;
	// the figures describe how much evidence stands behind it. A run with
	// no paired task has nothing to resample and carries none.
	if paired.Tasks > 0 {
		var rcfg contextbench.ReliabilityConfig
		if criterion.Reliability != nil {
			rcfg = *criterion.Reliability
		}
		rel, err := contextbench.ComputeReliability(paired, rcfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
			return 2
		}
		row.Reliability = &rel
	}
	ids := map[string]bool{}
	for _, r := range results {
		if r.ModelID != "" {
			ids[r.ModelID] = true
		}
		row.CostUSD += r.CostUSD
	}
	var modelIDs []string
	for id := range ids {
		modelIDs = append(modelIDs, id)
	}
	sort.Strings(modelIDs)
	row.ModelID = strings.Join(modelIDs, ",")

	if err := contextbench.AppendLedger(*ledgerPath, row); err != nil {
		fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
		return 2
	}
	if *outMD != "" {
		rows, err := contextbench.LoadLedger(*ledgerPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
			return 2
		}
		if err := os.WriteFile(*outMD, []byte(contextbench.RenderLedgerMarkdown(rows)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "ledger: writing %s: %v\n", *outMD, err)
			return 2
		}
	}

	fmt.Printf("%s [%s] %s: %s\n", strings.ToUpper(string(ev.Verdict)), criterion.ID, *runID, ev.Reason)
	fmt.Printf("  tasks=%d min_trials=%d success=%d/%d tokens agg=%.3fx median=%.3fx absent=%d flaky=%d leaked=%d cells_lost=%d cost_usd=%.2f model_id=%s\n",
		paired.Tasks, paired.MinTrials, paired.TreatmentSuccesses, paired.ControlSuccesses,
		paired.AggregateTokenRatio, paired.MedianTokenRatio, len(paired.Absent), len(paired.FlakyTasks),
		len(leaked), cellsLost, row.CostUSD, row.ModelID)
	fmt.Printf("  appended to %s\n", *ledgerPath)

	return exitFor(ev.Verdict)
}

// cmdRegress is the free regression gate over the committed ledger (PB-13,
// BC-16). Exit codes: 0 no regression, 1 at least one task fell, 3 the
// rows could not be compared, 2 usage/IO.
func cmdRegress(args []string) int {
	fs := flag.NewFlagSet("regress", flag.ContinueOnError)
	ledgerPath := fs.String("ledger", "docs/bench/ledger.jsonl", "path to the committed JSONL bench ledger")
	baseline := fs.String("baseline", "", "run_id to compare the newest row against (default: the previous row with the same corpus_sha256)")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rows, err := contextbench.LoadLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "regress: %v\n", err)
		return 2
	}
	rep := contextbench.Regress(rows, *baseline)
	if *asJSON {
		out, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Printf("%s regress %s vs %s: %s\n", strings.ToUpper(string(rep.Verdict)), rep.CurrentRunID, orNone(rep.BaselineRunID), rep.Reason)
		for _, r := range rep.Regressions {
			fmt.Printf("  REGRESSION %s: %.2f (%d trials) -> %.2f (%d trials)\n", r.TaskID, r.BaselineRate, r.BaselineTrials, r.CurrentRate, r.CurrentTrials)
		}
		if len(rep.Flaky) > 0 {
			fmt.Printf("  flaky (listed, not failed): %s\n", strings.Join(rep.Flaky, " "))
		}
		if len(rep.Added) > 0 {
			fmt.Printf("  added since baseline (not regressions): %s\n", strings.Join(rep.Added, " "))
		}
		if len(rep.Removed) > 0 {
			fmt.Printf("  removed since baseline: %s\n", strings.Join(rep.Removed, " "))
		}
	}
	return exitFor(rep.Verdict)
}

func orNone(s string) string {
	if s == "" {
		return "(no baseline)"
	}
	return s
}

// exitFor maps a tri-state verdict to the bench's exit-code contract.
func exitFor(v contextbench.Verdict) int {
	switch v {
	case contextbench.VerdictPass:
		return 0
	case contextbench.VerdictFail:
		return 1
	default:
		return 3
	}
}

// cmdBackfill appends one row for a run that predates the ledger, from the
// numbers its plan recorded (PB-2). The row is evaluated under the corpus's
// CURRENT criterion — an n=1 run reads indeterminate under the replication
// floor — and -notes must say it was backfilled. Exit 0 on append; the
// verdict is not the exit code here because a backfill is a record, not a
// measurement being gated.
func cmdBackfill(args []string) int {
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to the corpus JSON (carries the committed machine_criterion)")
	ledgerPath := fs.String("ledger", "docs/bench/ledger.jsonl", "path to the committed JSONL bench ledger")
	runID := fs.String("run-id", "", "run identifier (the original run's timestamp)")
	commit := fs.String("commit", "", "repo HEAD the run measured")
	model := fs.String("model", "", "model both arms ran on")
	tasks := fs.Int("task-count", 0, "paired task count")
	tSucc := fs.Int("treatment-successes", 0, "treatment successes (n=1: one per task at most)")
	cSucc := fs.Int("control-successes", 0, "control successes")
	ratio := fs.Float64("aggregate-token-ratio", 0, "treatment/control aggregate token ratio as recorded")
	notes := fs.String("notes", "", "must contain the word 'backfilled' and name the record it came from")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tasksPath == "" || *runID == "" || *commit == "" || *tasks <= 0 || !strings.Contains(*notes, "backfilled") {
		fmt.Fprintln(os.Stderr, "backfill: -tasks, -run-id, -commit, -task-count > 0, and -notes containing 'backfilled' are all required")
		return 2
	}
	criterion, err := contextbench.LoadCriterion(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill: %v\n", err)
		return 2
	}
	row := contextbench.BackfillLedgerRow(criterion, *runID, time.Now().UTC().Format(time.RFC3339), *commit, *tasksPath, *model, *tasks, *tSucc, *cSucc, *ratio, *notes)
	if err := contextbench.AppendLedger(*ledgerPath, row); err != nil {
		fmt.Fprintf(os.Stderr, "backfill: %v\n", err)
		return 2
	}
	fmt.Printf("%s [%s] %s: %s\n  appended to %s\n", strings.ToUpper(string(row.Evaluation.Verdict)), criterion.ID, *runID, row.Evaluation.Reason, *ledgerPath)
	return 0
}

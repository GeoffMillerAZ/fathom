// Command contextbenchscore is the thin CLI wrapper scripts/context-bench.sh
// uses to score one claude -p transcript and to assemble the final results
// artifact, over the tested internal/contextbench package (docs/plans/
// falsifiability.md B1-2). All scoring/aggregation logic lives in
// internal/contextbench and is unit-tested there; this binary only wires
// flags to that library and writes files, so it is deliberately not itself
// unit-tested beyond a build check (see B1-2's skip_test note in the plan).
//
// Subcommands:
//
//	contextbenchscore score  -tasks <tasks.json> -task-id <id> -arm <name> -claude-json <path> -out <path>
//	    Scores one arm's raw `claude -p --output-format json` output against
//	    one task and writes an ArmResult JSON to -out.
//
//	contextbenchscore report -tasks <tasks.json> -results <dir of *.json ArmResults> -out-json <path> -out-md <path>
//	    Reads every ArmResult JSON file in -results (non-recursive), builds
//	    the aggregated RunResults artifact, and writes both the JSON results
//	    file and the rendered Markdown summary table.
//
//	contextbenchscore ledger -tasks <tasks.json> -results <dir> -ledger <path.jsonl> -run-id <id> -commit <sha> [-model <m>] [-treatment fathom] [-control grep] [-out-md <path>]
//	    Pairs the two arms, evaluates the corpus's committed
//	    machine_criterion against the paired summary, appends one row to the
//	    committed bench ledger, and exits 0 for a pass, 1 for a fail, and 3
//	    for indeterminate. The exit code is the gate: a fail is a measured
//	    refutation and a legitimate result to record, while indeterminate
//	    means the run did not measure enough to decide and must be re-run.
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
		fmt.Fprintln(os.Stderr, "usage: contextbenchscore <score|report|ledger|import> [flags]")
		return 2
	}
	switch args[0] {
	case "score":
		return cmdScore(args[1:])
	case "report":
		return cmdReport(args[1:])
	case "ledger":
		return cmdLedger(args[1:])
	case "import":
		return cmdImport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (want score, report, ledger, or import)\n", args[0])
		return 2
	}
}

func cmdScore(args []string) int {
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	tasksPath := fs.String("tasks", "", "path to tasks.json")
	taskID := fs.String("task-id", "", "task id to score")
	arm := fs.String("arm", "", "arm name (e.g. fathom, grep)")
	claudeJSONPath := fs.String("claude-json", "", "path to captured `claude -p --output-format json` output")
	outPath := fs.String("out", "", "path to write the scored ArmResult JSON")
	trial := fs.Int("trial", 0, "1-based repetition index within this run (0 = single-trial run)")
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

	ar, err := contextbench.ArmResultFromClaudeJSON(*task, *arm, raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "score: %v\n", err)
		return 1
	}
	ar.Trial = *trial

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

	entries, err := os.ReadDir(*resultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "report: reading %s: %v\n", *resultsDir, err)
		return 1
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names) // deterministic aggregation order

	var results []contextbench.ArmResult
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(*resultsDir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "report: reading %s: %v\n", name, err)
			return 1
		}
		var ar contextbench.ArmResult
		if err := json.Unmarshal(raw, &ar); err != nil {
			fmt.Fprintf(os.Stderr, "report: parsing %s: %v\n", name, err)
			return 1
		}
		results = append(results, ar)
	}
	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "report: no ArmResult *.json files found in %s\n", *resultsDir)
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
	tasksPath := fs.String("tasks", "", "path to the corpus JSON (carries the committed machine_criterion)")
	resultsDir := fs.String("results", "", "directory of per-task-per-arm-per-trial ArmResult JSON files")
	ledgerPath := fs.String("ledger", "docs/bench/ledger.jsonl", "path to the committed JSONL bench ledger")
	runID := fs.String("run-id", "", "run identifier (the runner's timestamp)")
	commit := fs.String("commit", "", "repo HEAD the run measured")
	model := fs.String("model", "", "model both arms ran on")
	treatment := fs.String("treatment", "fathom", "treatment arm name")
	control := fs.String("control", "grep", "control arm name")
	outMD := fs.String("out-md", "", "optional path to render the whole ledger as a Markdown table")
	notes := fs.String("notes", "", "optional free-text note recorded with the row")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tasksPath == "" || *resultsDir == "" || *runID == "" || *commit == "" {
		fmt.Fprintln(os.Stderr, "ledger: -tasks, -results, -run-id, and -commit are all required")
		return 2
	}

	criterion, err := contextbench.LoadCriterion(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
		return 2
	}
	results, err := loadResults(*resultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
		return 2
	}

	paired := contextbench.Pair(results, *treatment, *control)
	ev := criterion.Evaluate(paired)
	row := contextbench.NewLedgerRow(*runID, time.Now().UTC().Format(time.RFC3339), *commit, *tasksPath, *model, paired, ev)
	row.Notes = *notes

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
	fmt.Printf("  tasks=%d min_trials=%d success=%d/%d tokens agg=%.3fx median=%.3fx absent=%d flaky=%d\n",
		paired.Tasks, paired.MinTrials, paired.TreatmentSuccesses, paired.ControlSuccesses,
		paired.AggregateTokenRatio, paired.MedianTokenRatio, len(paired.Absent), len(paired.FlakyTasks))
	fmt.Printf("  appended to %s\n", *ledgerPath)

	switch ev.Verdict {
	case contextbench.VerdictPass:
		return 0
	case contextbench.VerdictFail:
		return 1
	default:
		return 3
	}
}

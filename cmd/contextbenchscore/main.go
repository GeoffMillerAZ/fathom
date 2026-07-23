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
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: contextbenchscore <score|report> [flags]")
		return 2
	}
	switch args[0] {
	case "score":
		return cmdScore(args[1:])
	case "report":
		return cmdReport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (want score or report)\n", args[0])
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

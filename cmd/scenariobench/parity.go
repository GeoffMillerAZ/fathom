package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func init() { offlineCommands["parity"] = cmdParity }

// cmdParity checks that a corpus's arms differ only where they declare it
// (BX-60, scenariobench.CheckArmParity). It renders every arm into its own
// scratch copy of -tree; it starts no daemon and executes no Setup line.
//
// Exit codes: 0 when every arm is PASS, 1 when any arm is FAIL, 2 when an arm
// is INCOMPLETE and none FAIL, and 2 for a usage error, a load error, an
// unknown baseline or task, or any other CheckArmParity refusal.
func cmdParity(args []string) int {
	flags := newFlagSet("parity")
	corpusPath := flags.String("corpus", "", "path to the scenario corpus JSON (required)")
	baseline := flags.String("baseline", "", "arm every other arm is compared to (default: the corpus criterion's control)")
	taskID := flags.String("task", "", "restrict to this task id (default: every task not dropped)")
	tree := flags.String("tree", "", "local tree each arm's scratch copy starts from (default: an empty directory); .git, symlinks and non-regular files are not copied")
	asJSON := flags.Bool("json", false, "print the ParityReport as indented JSON instead of one line per arm")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "scenariobench parity: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *corpusPath == "" {
		fmt.Fprintln(os.Stderr, "scenariobench parity: -corpus is required")
		return 2
	}
	corpus, _, err := loadCorpus(*corpusPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench parity: %v\n", err)
		return 2
	}
	report, err := scenariobench.CheckArmParity(corpus, scenariobench.ParityOptions{Baseline: *baseline, TaskID: *taskID, Tree: *tree})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench parity: %v\n", err)
		return 2
	}
	if len(report.Skipped) > 0 {
		fmt.Fprintf(os.Stderr, "scenariobench parity: %d tree entries not copied: %s\n", len(report.Skipped), strings.Join(report.Skipped, ", "))
	}
	if *asJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench parity: %v\n", err)
			return 2
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("parity: baseline %s, %d tasks, tree sha256 %s\n", report.Baseline, len(report.Tasks), report.TreeSHA256)
		for _, arm := range report.Arms {
			switch arm.Verdict {
			case scenariobench.ParityFail:
				line := fmt.Sprintf("%s: FAIL %s", arm.Arm, strings.Join(arm.Undeclared, ", "))
				if len(arm.Declared) > 0 {
					line += " (declared: " + strings.Join(arm.Declared, ", ") + ")"
				}
				fmt.Println(line)
			case scenariobench.ParityIncomplete:
				fmt.Printf("%s: INCOMPLETE %s: %s\n", arm.Arm, strings.Join(arm.Unrendered, ", "), arm.RenderError)
			default:
				fmt.Printf("%s: %s\n", arm.Arm, arm.Verdict)
			}
		}
	}
	exit := 0
	for _, arm := range report.Arms {
		switch arm.Verdict {
		case scenariobench.ParityFail:
			return 1
		case scenariobench.ParityIncomplete:
			exit = 2
		}
	}
	return exit
}

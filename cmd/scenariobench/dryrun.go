package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func init() { offlineCommands["dryrun"] = cmdDryRun }

// cmdDryRun reads existing cells. Running controls remains the run verb's job.
func cmdDryRun(args []string) int {
	flags := newFlagSet("dryrun")
	corpusPath := flags.String("corpus", "", "scenario corpus JSON (required)")
	cellsDir := flags.String("cells", filepath.Join("workspace", "artifacts", "scenariobench", "cells"), "stored null, oracle and wrong-patch cells, 3 trials each")
	arm := flags.String("arm", "", "restrict to this corpus arm (use an unwired arm for offline controls)")
	runID := flags.String("run-id", "", "restrict to this run id (otherwise duplicate trials are refused)")
	asJSON := flags.Bool("json", false, "print sorted DryRunVerdict as JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *corpusPath == "" {
		fmt.Fprintln(os.Stderr, "scenariobench dryrun: -corpus is required")
		return 2
	}
	corpus, sha, err := loadCorpusOrLCA(*corpusPath, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench dryrun: %v\n", err)
		return 2
	}
	if *arm != "" {
		selected, ok := corpus.Arms[*arm]
		if !ok {
			fmt.Fprintf(os.Stderr, "scenariobench dryrun: unknown arm %q\n", *arm)
			return 2
		}
		corpus.Arms = map[string]scenariobench.Arm{*arm: selected}
	}
	var cells []scenariobench.CellResult
	err = filepath.WalkDir(*cellsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var cell scenariobench.CellResult
		if err := json.Unmarshal(raw, &cell); err != nil {
			return fmt.Errorf("reading cell %s: %w", path, err)
		}
		if cell.CorpusSHA256 == sha && (*runID == "" || cell.RunID == *runID) {
			cells = append(cells, cell)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench dryrun: %v\n", err)
		return 2
	}
	verdicts, err := scenariobench.DryRunVerdict(corpus, cells)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench dryrun: %v\n", err)
		return 2
	}
	if *asJSON {
		if err := json.NewEncoder(os.Stdout).Encode(verdicts); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench dryrun: %v\n", err)
			return 2
		}
	} else {
		for _, v := range verdicts {
			if v.Keep {
				fmt.Printf("%s: keep\n", v.TaskID)
			} else {
				fmt.Printf("%s: drop (%s)\n", v.TaskID, v.Reason)
			}
		}
	}
	return 0
}

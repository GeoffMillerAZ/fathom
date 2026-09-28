package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// cmdLangRegrade is `indexbench lang-regrade <reading.json>...`: after a
// harness rule changes (a threshold, not_measured, the required-kind
// mapping), each committed reading is recomputed from its recorded
// subjects' raw counts — no extraction, no oracle — and rewritten, with a
// note naming the harness commit that regraded it. A reading whose
// extraction or matching changed must be re-measured with lang-oracle
// instead; regrading never stands in for that.
func cmdLangRegrade(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-regrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	commit := fs.String("commit", "", "harness commit regrading (default: git rev-parse --short HEAD)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: indexbench lang-regrade [-commit <sha>] <reading.json>...")
		return 2
	}
	if *commit == "" {
		*commit = gitOut(context.Background(), ".", "rev-parse", "--short", "HEAD")
	}
	for _, path := range fs.Args() {
		r, err := loadReading(path)
		if err != nil {
			fmt.Fprintln(stderr, "lang-regrade:", err)
			return 1
		}
		if spec, _, err := resolveBenchSpec(r.Dialect, ""); err == nil {
			r.RequiredKinds = spec.Manifest.RequiredCanonical()
		}
		before := r.LevelMet
		langbench.Regrade(&r)
		r.Notes = mergeNotes(r.Notes, []string{fmt.Sprintf("regraded at %s from the recorded subject counts (harness rules only; extraction as measured at %s)", *commit, r.FathomCommit)})
		if err := r.Validate(); err != nil {
			fmt.Fprintln(stderr, "lang-regrade: reading refused:", err)
			return 1
		}
		b, err := langbench.MarshalReading(r)
		if err == nil {
			err = os.WriteFile(path, b, 0o644)
		}
		if err != nil {
			fmt.Fprintln(stderr, "lang-regrade:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: level_met %s -> %s grade %+v\n", r.Dialect, before, r.LevelMet, r.Grade)
	}
	return 0
}

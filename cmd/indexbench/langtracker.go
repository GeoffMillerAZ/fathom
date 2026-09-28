package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// cmdLangTracker is `indexbench lang-tracker > docs/languages.md` (LB-F17,
// design §12): the tracker rendered from the queue, the registered
// dialects and the committed readings.
func cmdLangTracker(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-tracker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	in, err := langbench.TrackerInputFromRepo(*root)
	if err != nil {
		fmt.Fprintln(stderr, "lang-tracker:", err)
		return 1
	}
	doc := langbench.RenderTracker(in)
	if errs := langbench.CheckTrackerAgainstReadings(doc, in.Readings); len(errs) > 0 {
		fmt.Fprintln(stderr, "lang-tracker:", errs)
		return 1
	}
	fmt.Fprint(stdout, doc)
	return 0
}

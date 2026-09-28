package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// cmdLangNext is `indexbench lang-next` (design §13.0 step 2 and §15,
// owner decision D6): the open queue items in queue order — rows neither
// [x] nor [-] — each "eligible" or "blocked: <rows>". A dependency on
// another queue item is satisfied by that item's committed reading at L1
// or better, not only by its row. Without -all, only eligible items print.
func cmdLangNext(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-next", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	all := fs.Bool("all", false, "also list blocked items and what blocks them")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	q, err := langbench.LoadQueue(filepath.Join(*root, "docs", "plans", "language-breadth.queue.json"))
	if err != nil {
		fmt.Fprintln(stderr, "lang-next:", err)
		return 1
	}
	md, err := os.ReadFile(filepath.Join(*root, "docs", "plans", "language-breadth.md"))
	if err != nil {
		fmt.Fprintln(stderr, "lang-next:", err)
		return 1
	}
	plan := verify.ParseMarkdownPlan("fathom", md)
	done := map[string]bool{}
	for _, t := range plan.Tasks {
		done[t.ID] = t.Claimed || t.Skipped
	}
	blocked := langbench.QueueEligibility(q, plan, langbench.ReadingLevels(filepath.Join(*root, "docs", "bench", "readings", "lang")))
	for _, e := range q {
		if done[e.PlanRow] {
			continue
		}
		switch b := blocked[e.ID]; {
		case len(b) == 0:
			fmt.Fprintf(stdout, "%s\teligible\n", e.ID)
		case *all:
			fmt.Fprintf(stdout, "%s\tblocked: %s\n", e.ID, strings.Join(b, ","))
		}
	}
	return 0
}

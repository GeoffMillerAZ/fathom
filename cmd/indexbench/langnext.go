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
// owner decision D6, G2-6): the open queue items in queue order — rows
// neither [x] nor [-] — each "eligible", "blocked: <rows>" or "needs
// re-planning: <rows>". A dependency is met only by an [x] row: a deferred
// [-] one blocks and a refused one needs re-planning. A dependency on another
// queue item is also satisfied by that item's committed reading at L1 or
// better, unless the item is refused. Without -all, only eligible items print.
func cmdLangNext(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-next", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	all := fs.Bool("all", false, "also list blocked items, items needing re-planning, and what holds them")
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
	blocks := langbench.QueueEligibility(q, plan, langbench.ReadingLevels(filepath.Join(*root, "docs", "bench", "readings", "lang")))
	for _, l := range langNextLines(q, done, blocks, *all) {
		fmt.Fprintln(stdout, l)
	}
	return 0
}

// langNextLines renders lang-next: every queue item whose own row is open,
// in queue order; all adds the ones held by a dependency, re-planning first.
func langNextLines(q []langbench.QueueEntry, done map[string]bool, blocks map[string]langbench.QueueBlock, all bool) []string {
	var out []string
	for _, e := range q {
		if done[e.PlanRow] {
			continue
		}
		b := blocks[e.ID]
		if b.Eligible() {
			out = append(out, e.ID+"\teligible")
			continue
		}
		if !all {
			continue
		}
		var why []string
		if len(b.Replan) > 0 {
			why = append(why, "needs re-planning: "+strings.Join(b.Replan, ","))
		}
		if len(b.Blocked) > 0 {
			why = append(why, "blocked: "+strings.Join(b.Blocked, ","))
		}
		out = append(out, e.ID+"\t"+strings.Join(why, "; "))
	}
	return out
}

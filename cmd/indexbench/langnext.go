package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// cmdLangNext is `indexbench lang-next` (design §13.0 step 2 and §15,
// owner decision D6, G2-6): the open queue items in queue order — rows
// neither [x] nor [-] — each "eligible", "waits: <-rest row> ...", "blocked: <rows>" or "needs
// re-planning: <rows>". A dependency is met only by an [x] row: a deferred
// [-] one blocks and a refused one needs re-planning. A dependency on another
// queue item is also satisfied by that item's committed reading at L1 or
// better, unless the item is refused. Without -all, only eligible items print.
func cmdLangNext(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-next", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	all := fs.Bool("all", false, "also list blocked items, items needing re-planning, and what holds them")
	near := fs.Bool("near-miss", false, "list measured first-level blockers within -margin of their bar, including refused items")
	margin := fs.Float64("margin", .01, "absolute ratio margin (0.01 is one percentage point)")
	jsonOut := fs.Bool("json", false, "emit near-miss rows as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *near {
		return cmdLangNearMiss(*root, *margin, *jsonOut, stdout, stderr)
	}
	q, err := langbench.LoadQueue(filepath.Join(*root, "docs", "plans", "language-breadth.queue.json"))
	if err != nil {
		fmt.Fprintln(stderr, "lang-next:", err)
		return 1
	}
	plan, err := langbench.LoadBreadthPlan(*root)
	if err != nil {
		fmt.Fprintln(stderr, "lang-next:", err)
		return 1
	}
	level := langbench.ReadingLevels(filepath.Join(*root, "docs", "bench", "readings", "lang"))
	blocks := langbench.QueueEligibility(q, plan, level)
	for _, l := range langNextLines(q, plan, level, blocks, *all) {
		fmt.Fprintln(stdout, l)
	}
	return 0
}

// langNextLines renders lang-next: every queue item whose own row is open,
// in queue order. An item is "eligible" when its needs are met and it has no
// deferred -rest row (a partial reading adds where it stands). An item with a
// deferred -rest row "waits", whatever its reading, and prints only with all,
// which also adds the items held by a dependency, re-planning first.
func langNextLines(q []langbench.QueueEntry, plan verify.Plan, level func(string) (string, bool), blocks map[string]langbench.QueueBlock, all bool) []string {
	rows := map[string]verify.Task{}
	for _, t := range plan.Tasks {
		rows[t.ID] = t
	}
	var out []string
	for _, e := range q {
		if own, ok := rows[e.PlanRow]; ok && (own.Claimed || own.Skipped) {
			continue
		}
		b := blocks[e.ID]
		if b.Eligible() {
			line, waits := langNextEligible(e, rows, level)
			if waits && !all {
				continue
			}
			out = append(out, e.ID+"\t"+line)
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

// langNextEligible is the status text of an item whose needs are met, and
// whether a deferred -rest row holds it instead. A deferred -rest row holds
// the item whatever its reading (none, absent, partial or at target): its
// trigger, not the queue, says when the item is next startable (O1394).
func langNextEligible(e langbench.QueueEntry, rows map[string]verify.Task, lvl func(string) (string, bool)) (string, bool) {
	met, has := lvl(e.ID)
	partial := has && slices.Contains(langbench.Levels, met)
	if !partial {
		met = "none"
	}
	rest, ok := rows[e.PlanRow+"-rest"]
	if ok && rest.Skipped && (rest.Disposition == "" || rest.Disposition == "deferred") {
		trigger := []rune(rest.Trigger)
		if len(trigger) > 120 {
			trigger = trigger[:120]
		}
		return fmt.Sprintf("waits: %s (at %s of %s): %s", rest.ID, met, e.TargetLevel, string(trigger)), true
	}
	if !partial || slices.Index(langbench.Levels, met) >= slices.Index(langbench.Levels, e.TargetLevel) {
		return "eligible", false
	}
	return fmt.Sprintf("eligible (at %s of %s)", met, e.TargetLevel), false
}

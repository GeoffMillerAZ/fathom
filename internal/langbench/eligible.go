package langbench

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// QueueBlock is what keeps one queue entry from being eligible: Blocked are
// depends_on rows still open or deferred (G2-6: deferred is not done), Replan
// are rows refused — they will not be built, so the entry needs re-planning
// rather than waiting. Both empty: eligible.
type QueueBlock struct {
	Blocked []string
	Replan  []string
}

// Eligible reports whether nothing blocks the entry.
func (b QueueBlock) Eligible() bool { return len(b.Blocked) == 0 && len(b.Replan) == 0 }

// QueueEligibility is §13.0 step 2 under owner decision D6 (2026-09-28) and
// G2-6 (2026-09-29): for every queue entry, the depends_on rows still
// blocking it. A row is satisfied only when it is [x]. A [-] row is not: a
// refused one (any disposition but deferred) puts the entry in Replan, a
// deferred one (or one with no disposition) keeps it Blocked. A dependency on
// another queue item (LB-Q-<id>: cpp on c, scss/less on css) is also
// satisfied when that item has a committed reading at L1 or better, even while
// its own row stays open for a higher target or is deferred — unless it is
// refused. level returns an item's committed level_met and whether a reading
// exists. The same rule as `fathom plans next` (depSatisfied, needsReplan),
// read from the authored checkboxes: this function recomputes no verdict.
func QueueEligibility(q []QueueEntry, plan verify.Plan, level func(id string) (string, bool)) map[string]QueueBlock {
	rows := map[string]verify.Task{}
	for _, t := range plan.Tasks {
		rows[t.ID] = t
	}
	queued := map[string]bool{}
	for _, e := range q {
		queued[e.ID] = true
	}
	out := map[string]QueueBlock{}
	for _, e := range q {
		var b QueueBlock
		for _, d := range e.DependsOn {
			t, ok := rows[d]
			if ok && t.Claimed && !t.Skipped {
				continue
			}
			if ok && refused(t) {
				b.Replan = append(b.Replan, d)
				continue
			}
			if id, isQ := strings.CutPrefix(d, "LB-Q-"); isQ && queued[id] {
				if l, ok := level(id); ok && slices.Contains(Levels, l) {
					continue
				}
			}
			b.Blocked = append(b.Blocked, d)
		}
		slices.Sort(b.Blocked)
		slices.Sort(b.Replan)
		out[e.ID] = b
	}
	return out
}

// refused reports whether a [-] row will not be built: any disposition but
// deferred (G2-6; cmd/fathom's needsReplan reads the same).
func refused(t verify.Task) bool {
	return t.Skipped && t.Disposition != "" && t.Disposition != "deferred"
}

// ReadingLevels returns a level function over the committed readings in
// dir (docs/bench/readings/lang): an item's level_met, and whether its
// reading exists, decodes and validates.
func ReadingLevels(dir string) func(id string) (string, bool) {
	return func(id string) (string, bool) {
		r, err := LoadReading(filepath.Join(dir, id+".json"))
		if err != nil {
			return "", false // absent, or refused by Validate: never trusted
		}
		return r.LevelMet, true
	}
}

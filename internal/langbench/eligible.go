package langbench

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// QueueEligibility is §13.0 step 2 under owner decision D6 (2026-09-28):
// for every queue entry, the depends_on rows still blocking it (nil when
// eligible). A row is satisfied when it is [x] or [-]; a dependency on
// another queue item (LB-Q-<id>: cpp on c, scss/less on css) is also
// satisfied when that item has a committed reading at L1 or better, even
// while its own row stays open for a higher target. level returns an
// item's committed level_met and whether a reading exists.
func QueueEligibility(q []QueueEntry, plan verify.Plan, level func(id string) (string, bool)) map[string][]string {
	rows := map[string]verify.Task{}
	for _, t := range plan.Tasks {
		rows[t.ID] = t
	}
	queued := map[string]bool{}
	for _, e := range q {
		queued[e.ID] = true
	}
	out := map[string][]string{}
	for _, e := range q {
		var blocked []string
		for _, d := range e.DependsOn {
			if t, ok := rows[d]; ok && (t.Claimed || t.Skipped) {
				continue
			}
			if id, ok := strings.CutPrefix(d, "LB-Q-"); ok && queued[id] {
				if l, ok := level(id); ok && slices.Contains(Levels, l) {
					continue
				}
			}
			blocked = append(blocked, d)
		}
		slices.Sort(blocked)
		out[e.ID] = blocked
	}
	return out
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

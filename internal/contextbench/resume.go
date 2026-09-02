package contextbench

import "sort"

// Resume (PB-4). A --repeat 3 home run is 78 `claude -p` calls; one
// watchdog kill or one empty envelope used to force a full re-run because
// the runner had no way to fill in the missing cell. `--resume <run-dir>`
// re-uses a run directory, skips every cell whose scored file exists, and
// scores only the gaps; MergeResumedResults is the union the ledger step
// then sees, and CellsLost says how many intended cells never scored — a
// number that belongs in the row, not only in the terminal.

// MergeResumedResults returns the union of prior and fresh keyed by
// (task, arm, trial). A cell present in both takes the fresh reading —
// the cell was re-scored on purpose. Output is sorted by (task, arm,
// trial), so the same cells in any file order merge identically.
func MergeResumedResults(prior, fresh []ArmResult) []ArmResult {
	type key struct {
		task, arm string
		trial     int
	}
	merged := map[key]ArmResult{}
	for _, r := range prior {
		merged[key{r.TaskID, r.Arm, r.Trial}] = r
	}
	for _, r := range fresh {
		merged[key{r.TaskID, r.Arm, r.Trial}] = r
	}
	out := make([]ArmResult, 0, len(merged))
	for _, r := range merged {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskID != out[j].TaskID {
			return out[i].TaskID < out[j].TaskID
		}
		if out[i].Arm != out[j].Arm {
			return out[i].Arm < out[j].Arm
		}
		return out[i].Trial < out[j].Trial
	})
	return out
}

// CellsLost counts the (task, arm, trial) cells a run intended — every
// task in tasks × every arm × repeat trials — that results does not
// carry. A missing Trial (0, single-trial artifacts) counts as trial 1.
// Only tasks in tasks are counted, so a leaked or filtered task that was
// never intended is not "lost".
func CellsLost(tasks []Task, arms []string, repeat int, results []ArmResult) int {
	if repeat < 1 {
		repeat = 1
	}
	type key struct {
		task, arm string
		trial     int
	}
	have := map[key]bool{}
	for _, r := range results {
		t := r.Trial
		if t == 0 {
			t = 1
		}
		have[key{r.TaskID, r.Arm, t}] = true
	}
	lost := 0
	for _, task := range tasks {
		for _, arm := range arms {
			for t := 1; t <= repeat; t++ {
				if !have[key{task.ID, arm, t}] {
					lost++
				}
			}
		}
	}
	return lost
}

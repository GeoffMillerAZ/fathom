package contextbench

import "sort"

// The memorisation floor (PB-9; docs/vision/26-proof-credibility.md §2.2).
//
// Frontier models name the buggy file from issue text alone a large share
// of the time on benchmarks they have seen. A task both arms can answer
// from memory measures nothing about discovery. So the runner has a third
// arm, "none" — every tool disallowed, no MCP servers — and the corpus
// commits a LeakRule beside its criterion BEFORE the run: a task the none
// arm passes in at least MinPasses trials is `leaked`, excluded from the
// verdict, and listed in the ledger row so the exclusion is visible.

// LeakRule is the committed rule for calling a task leaked.
type LeakRule struct {
	// Arm is the no-tools arm's name ("none").
	Arm string `json:"arm"`
	// MinPasses is how many successful trials in Arm mark a task leaked.
	MinPasses int `json:"min_passes"`
	// OfTrials documents the trial count the rule was registered against
	// (informational: MinPasses is what LeakedTasks reads).
	OfTrials int `json:"of_trials,omitempty"`
}

// LeakedTasks returns, sorted, the ids of tasks whose Arm cell passed at
// least rule.MinPasses times. A rule with no arm or a non-positive
// threshold marks nothing — an absent rule is not a permissive one, it is
// simply no rule.
func LeakedTasks(results []ArmResult, rule LeakRule) []string {
	if rule.Arm == "" || rule.MinPasses <= 0 {
		return nil
	}
	var out []string
	for _, st := range StatsByTaskArm(results) {
		if st.Arm == rule.Arm && st.Successes >= rule.MinPasses {
			out = append(out, st.TaskID)
		}
	}
	sort.Strings(out)
	return out
}

// WithoutTasks returns results with every cell of the named tasks removed,
// in the input order. It is how leaked tasks leave the verdict: the rows
// are dropped from what Pair sees, and the ids are recorded on the ledger
// row so the drop cannot go unnoticed.
func WithoutTasks(results []ArmResult, ids []string) []ArmResult {
	skip := make(map[string]bool, len(ids))
	for _, id := range ids {
		skip[id] = true
	}
	out := make([]ArmResult, 0, len(results))
	for _, r := range results {
		if !skip[r.TaskID] {
			out = append(out, r)
		}
	}
	return out
}

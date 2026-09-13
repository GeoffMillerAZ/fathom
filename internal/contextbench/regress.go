package contextbench

import (
	"fmt"
	"sort"
)

// The regression gate (PB-13, BC-16; docs/vision/26-proof-credibility.md
// §2.4). Semantics mirror agentic-kit's eval-runner Regression rather
// than inventing a third convention:
//
//   - the baseline is a committed record, the current run is compared
//     against it per scenario — here per task, on the treatment arm's
//     success rate;
//   - a pair regresses when the current reading is worse than the
//     baseline's; anything added since the baseline is not a regression by
//     definition; anything removed is listed;
//   - no comparable pairs at all means nothing regressed — stated
//     explicitly, because an empty check set is otherwise a failure;
//   - a baseline that cannot be trusted is fatal (there: a torn journal
//     tail; here: no row with the same corpus identity, or a row with no
//     per-task data) — the gate does not compare against a guess.
//
// One departure, and it is the bench's own rule: a task that was flaky in
// either row (neither always passed nor always failed) is LISTED, never
// failed. At n=3 a 3/3 → 2/3 step is inside the noise the replication
// floor exists to reveal; calling it a regression would make the gate cry
// wolf on every run and be ignored by the second.

// TaskRegression is one task whose treatment success rate fell.
type TaskRegression struct {
	TaskID         string  `json:"task_id"`
	BaselineRate   float64 `json:"baseline_rate"`
	CurrentRate    float64 `json:"current_rate"`
	BaselineTrials int     `json:"baseline_trials"`
	CurrentTrials  int     `json:"current_trials"`
}

// RegressionReport is Regress's verdict. Verdict is pass when nothing
// regressed, fail when at least one task did, indeterminate when the two
// rows could not be compared.
type RegressionReport struct {
	Verdict       Verdict          `json:"verdict"`
	Reason        string           `json:"reason"`
	CurrentRunID  string           `json:"current_run_id"`
	BaselineRunID string           `json:"baseline_run_id,omitempty"`
	CorpusSHA256  string           `json:"corpus_sha256,omitempty"`
	Compared      int              `json:"compared"`
	Regressions   []TaskRegression `json:"regressions,omitempty"`
	Flaky         []string         `json:"flaky,omitempty"`
	Added         []string         `json:"added,omitempty"`
	Removed       []string         `json:"removed,omitempty"`
}

// Regress compares the newest ledger row against a baseline: the row
// named by baselineID, or — when baselineID is empty — the most recent
// earlier row with the same corpus_sha256. All output slices are sorted.
func Regress(rows []LedgerRow, baselineID string) RegressionReport {
	if len(rows) == 0 {
		return RegressionReport{Verdict: VerdictIndeterminate, Reason: "the ledger has no rows"}
	}
	cur := rows[len(rows)-1]
	rep := RegressionReport{CurrentRunID: cur.RunID, CorpusSHA256: cur.CorpusSHA256}
	if cur.CorpusSHA256 == "" {
		rep.Verdict, rep.Reason = VerdictIndeterminate, fmt.Sprintf("newest row %s records no corpus_sha256 — nothing can be known to have scored the same questions", cur.RunID)
		return rep
	}
	if len(cur.PerTask) == 0 {
		rep.Verdict, rep.Reason = VerdictIndeterminate, fmt.Sprintf("newest row %s carries no per_task outcomes to compare", cur.RunID)
		return rep
	}

	var base *LedgerRow
	for i := len(rows) - 2; i >= 0; i-- {
		r := rows[i]
		if baselineID != "" {
			if r.RunID == baselineID {
				base = &rows[i]
				break
			}
			continue
		}
		if r.RunID == cur.RunID {
			// The same run scored under a second criterion (H2-1 as
			// history beside H2-3) carries identical per_task outcomes;
			// comparing a run with itself would read as "no regression".
			continue
		}
		if r.CorpusSHA256 == cur.CorpusSHA256 {
			base = &rows[i]
			break
		}
	}
	switch {
	case base == nil && baselineID != "":
		rep.Verdict, rep.Reason = VerdictIndeterminate, fmt.Sprintf("no earlier row with run_id %q", baselineID)
		return rep
	case base == nil:
		rep.Verdict, rep.Reason = VerdictIndeterminate, fmt.Sprintf("no earlier row scored corpus %s — the first run on a corpus has nothing to regress against", short(cur.CorpusSHA256))
		return rep
	case base.CorpusSHA256 != cur.CorpusSHA256:
		rep.BaselineRunID = base.RunID
		rep.Verdict, rep.Reason = VerdictIndeterminate, fmt.Sprintf("baseline %s scored corpus %s, newest %s — different questions are not comparable", base.RunID, short(base.CorpusSHA256), short(cur.CorpusSHA256))
		return rep
	case len(base.PerTask) == 0:
		rep.BaselineRunID = base.RunID
		rep.Verdict, rep.Reason = VerdictIndeterminate, fmt.Sprintf("baseline %s carries no per_task outcomes", base.RunID)
		return rep
	}
	rep.BaselineRunID = base.RunID

	baseBy := make(map[string]PairedTask, len(base.PerTask))
	for _, p := range base.PerTask {
		baseBy[p.TaskID] = p
	}
	curBy := make(map[string]PairedTask, len(cur.PerTask))
	for _, p := range cur.PerTask {
		curBy[p.TaskID] = p
	}
	for id := range baseBy {
		if _, ok := curBy[id]; !ok {
			rep.Removed = append(rep.Removed, id)
		}
	}
	for _, c := range cur.PerTask {
		b, ok := baseBy[c.TaskID]
		if !ok {
			rep.Added = append(rep.Added, c.TaskID)
			continue
		}
		rep.Compared++
		if flakyCell(b.TreatmentSuccesses, b.TreatmentTrials) || flakyCell(c.TreatmentSuccesses, c.TreatmentTrials) {
			rep.Flaky = append(rep.Flaky, c.TaskID)
			continue
		}
		br, cr := rate(b.TreatmentSuccesses, b.TreatmentTrials), rate(c.TreatmentSuccesses, c.TreatmentTrials)
		if cr < br {
			rep.Regressions = append(rep.Regressions, TaskRegression{
				TaskID: c.TaskID, BaselineRate: br, CurrentRate: cr,
				BaselineTrials: b.TreatmentTrials, CurrentTrials: c.TreatmentTrials,
			})
		}
	}
	sort.Strings(rep.Added)
	sort.Strings(rep.Removed)
	sort.Strings(rep.Flaky)
	sort.Slice(rep.Regressions, func(i, j int) bool { return rep.Regressions[i].TaskID < rep.Regressions[j].TaskID })

	switch {
	case len(rep.Regressions) > 0:
		rep.Verdict = VerdictFail
		rep.Reason = fmt.Sprintf("%d task(s) fell on the treatment arm vs %s (%d compared, %d flaky listed, %d added, %d removed)",
			len(rep.Regressions), base.RunID, rep.Compared, len(rep.Flaky), len(rep.Added), len(rep.Removed))
	case rep.Compared == 0:
		rep.Verdict = VerdictPass
		rep.Reason = fmt.Sprintf("no task in common with %s — nothing to regress (%d added, %d removed)", base.RunID, len(rep.Added), len(rep.Removed))
	default:
		rep.Verdict = VerdictPass
		rep.Reason = fmt.Sprintf("no task fell on the treatment arm vs %s (%d compared, %d flaky listed, %d added, %d removed)",
			base.RunID, rep.Compared, len(rep.Flaky), len(rep.Added), len(rep.Removed))
	}
	return rep
}

func flakyCell(successes, trials int) bool { return successes > 0 && successes < trials }

func rate(successes, trials int) float64 {
	if trials == 0 {
		return 0
	}
	return float64(successes) / float64(trials)
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

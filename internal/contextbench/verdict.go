// verdict.go — RC-35 (ADR-034 decision 2; vision 28 §2.5/§2.7), the home
// bench's own "Where fathom stands" block, rendered above
// RenderLedgerMarkdown's table by cmd/benchledgerrender — the Go-side half
// of scripts/bench-ledger-render.sh.
//
// A home LedgerRow is already one aggregated run (treatment vs control
// across the whole corpus), unlike the scenario ledger's per-(task, arm)
// rows (internal/scenariobench.renderVerdict), so this needs none of that
// file's per-arm grouping — one paragraph per row is the whole job.
package contextbench

import (
	"fmt"
	"strings"
)

// VerdictHeading opens the block, matching
// internal/scenariobench.VerdictHeading's own wording so both ledgers'
// renders read the same way.
const VerdictHeading = "### Where fathom stands"

// RenderVerdict renders rows (oldest first, the same order LoadLedger and
// RenderLedgerMarkdown use) as one plain-language paragraph per run.
func RenderVerdict(rows []LedgerRow) string {
	var b strings.Builder
	b.WriteString(VerdictHeading)
	b.WriteString("\n\n")
	if len(rows) == 0 {
		b.WriteString("_No runs recorded yet. `task bench-home` produces the first row (it spends real tokens)._\n")
		return b.String()
	}
	for _, r := range rows {
		verb := "matched"
		tRate, cRate := successRate(r.TreatmentSuccesses, r.TreatmentTrials), successRate(r.ControlSuccesses, r.ControlTrials)
		switch {
		case tRate > cRate:
			verb = "beat"
		case tRate < cRate:
			verb = "lost to"
		}
		// RC-35 fix: the registered criterion, not the raw success rates,
		// decides whether the row may claim a beat/loss at all — an
		// indeterminate row (e.g. below the replication floor) has not
		// measured enough to say either.
		verdictPhrase := fmt.Sprintf("%s %s", verb, r.Control)
		if r.Evaluation.Verdict == VerdictIndeterminate {
			verdictPhrase = "did not run enough trials to decide against " + r.Control
		}
		tokenPart := "an unmeasured"
		if r.AggregateTokenRatio > 0 {
			tokenPart = fmt.Sprintf("%.3f×", r.AggregateTokenRatio)
		}

		fmt.Fprintf(&b, "**%s** — %s vs %s: success %d/%d vs %d/%d",
			r.RunID, r.Treatment, r.Control,
			r.TreatmentSuccesses, r.TreatmentTrials, r.ControlSuccesses, r.ControlTrials)
		if r.AggregateTokenRatio > 0 {
			fmt.Fprintf(&b, ", tokens %.3f×", r.AggregateTokenRatio)
		}
		if r.AggregateCostRatio > 0 {
			fmt.Fprintf(&b, ", cost %.3f×", r.AggregateCostRatio)
		}
		b.WriteString(".\n")
		fmt.Fprintf(&b, "On %s, fathom's arm %s on success (%d v %d) at %s the tokens; registered criterion %s: %s (%s).\n\n",
			r.RunID, verdictPhrase, r.TreatmentSuccesses, r.ControlSuccesses, tokenPart,
			r.Evaluation.CriterionID, r.Evaluation.Verdict, r.Evaluation.Reason)
	}
	return b.String()
}

// successRate is trials-safe: an unmeasured arm (0 trials) reads as 0, not
// a division panic or a NaN comparison.
func successRate(successes, trials int) float64 {
	if trials == 0 {
		return 0
	}
	return float64(successes) / float64(trials)
}

// verdict.go — RC-35 (ADR-034 decision 2; vision 28 §2.5 "evidence anchors
// the gate verifies", §2.7 "the verdict in plain language"). renderVerdict
// turns committed scenario-ledger rows into a "Where fathom stands" block:
// per run, treatment vs control successes, token and cost ratio, surface
// call totals, and a one-sentence plain-language verdict — placed ABOVE the
// rendered tables, in both `scenariobench ledger` (RenderVerdict, this
// file) and the home bench's render (internal/contextbench.RenderVerdict,
// a separate implementation over that package's own LedgerRow — a home row
// is already one aggregated run, so it needs none of the per-arm grouping
// this file does).
//
// A voided row (InstrumentDefect set, RC-36) is excluded from every
// arithmetic here and named on its own line instead: a voided reading is
// not proof of anything, but its absence from the numbers must stay
// visible, never silent.
package scenariobench

import (
	"fmt"
	"sort"
	"strings"
)

// VerdictHeading opens the block every caller renders, so the scenario
// ledger's hook and the home bench's render frame it identically.
const VerdictHeading = "### Where fathom stands"

// RenderVerdict is renderVerdict's exported face for callers outside this
// package (cmd/scenariobench's `ledger` hook).
func RenderVerdict(rows []ScenarioLedgerRow, criteria map[string]Criterion) string {
	return renderVerdict(rows, criteria)
}

// renderVerdict renders one paragraph per run present in rows (sorted by
// run id, for a deterministic render), using criteria — keyed by the
// corpus SHA256 a row's own CorpusSHA256 names — to know which arm is the
// treatment and which is the control for that run. A run whose rows carry
// a CorpusSHA256 absent from criteria (or missing the named arm entirely)
// still lists its arms' raw totals — never dropped, never guessed at.
func renderVerdict(rows []ScenarioLedgerRow, criteria map[string]Criterion) string {
	var b strings.Builder
	b.WriteString(VerdictHeading)
	b.WriteString("\n\n")
	if len(rows) == 0 {
		b.WriteString("_No runs recorded yet._\n")
		return b.String()
	}

	type runGroup struct {
		sound  []ScenarioLedgerRow
		voided []ScenarioLedgerRow
	}
	byRun := map[string]*runGroup{}
	for _, r := range rows {
		g, ok := byRun[r.RunID]
		if !ok {
			g = &runGroup{}
			byRun[r.RunID] = g
		}
		if r.InstrumentDefect != "" {
			g.voided = append(g.voided, r)
		} else {
			g.sound = append(g.sound, r)
		}
	}
	runIDs := make([]string, 0, len(byRun))
	for id := range byRun {
		runIDs = append(runIDs, id)
	}
	sort.Strings(runIDs)

	for _, runID := range runIDs {
		g := byRun[runID]
		b.WriteString(renderRunVerdict(runID, g.sound, g.voided, criteria))
	}
	return b.String()
}

// armTotals is one arm's aggregate over a run's SOUND (non-voided) rows.
type armTotals struct {
	trials       int
	successes    int
	tokens       int64
	costUSD      float64
	surfaceCalls int
}

func renderRunVerdict(runID string, sound, voided []ScenarioLedgerRow, criteria map[string]Criterion) string {
	byArm := map[string]*armTotals{}
	var sha string
	for _, r := range sound {
		if sha == "" {
			sha = r.CorpusSHA256
		}
		a, ok := byArm[r.Arm]
		if !ok {
			a = &armTotals{}
			byArm[r.Arm] = a
		}
		a.trials += r.Trials
		a.successes += r.Successes
		a.tokens += r.Readings.TotalTokens
		a.costUSD += r.CostUSD
		a.surfaceCalls += r.Readings.SurfaceCalls
	}
	arms := make([]string, 0, len(byArm))
	for a := range byArm {
		arms = append(arms, a)
	}
	sort.Strings(arms)

	var b strings.Builder
	fmt.Fprintf(&b, "**%s**", runID)

	crit, known := criteria[sha]
	t, tOK := byArm[crit.Treatment]
	c, cOK := byArm[crit.Control]
	switch {
	case len(arms) == 0:
		b.WriteString(": every row is voided — nothing sound to measure.\n")
	case known && tOK && cOK:
		fmt.Fprintf(&b, " — %s vs %s: success %d/%d vs %d/%d",
			crit.Treatment, crit.Control, t.successes, t.trials, c.successes, c.trials)
		if t.tokens > 0 && c.tokens > 0 {
			fmt.Fprintf(&b, ", tokens %.3f×", float64(t.tokens)/float64(c.tokens))
		}
		if t.costUSD > 0 && c.costUSD > 0 {
			fmt.Fprintf(&b, ", cost %.3f×", t.costUSD/c.costUSD)
		}
		b.WriteString(".\n")
		b.WriteString(surfaceCallsLine(arms, byArm))
		b.WriteString(plainLanguageLine(runID, crit, t, c))
	default:
		if sha == "" {
			b.WriteString(" (voided or no sound row named a corpus)")
		} else if !known {
			b.WriteString(" (no registered criterion for this corpus)")
		} else {
			b.WriteString(fmt.Sprintf(" (criterion names %s/%s, not among this run's arms)", crit.Treatment, crit.Control))
		}
		b.WriteString(":\n")
		b.WriteString(rawArmsLine(arms, byArm))
	}

	if len(voided) > 0 {
		ids := make([]string, 0, len(voided))
		for _, r := range voided {
			ids = append(ids, fmt.Sprintf("%s/%s", r.TaskID, r.Arm))
		}
		sort.Strings(ids)
		fmt.Fprintf(&b, "Voided: %d row(s) carry `instrument_defect` and are excluded from the numbers above — %s.\n", len(voided), strings.Join(ids, ", "))
	}
	b.WriteString("\n")
	return b.String()
}

// surfaceCallsLine reports every arm's summed surface_calls, in sorted arm
// order.
func surfaceCallsLine(arms []string, byArm map[string]*armTotals) string {
	parts := make([]string, 0, len(arms))
	for _, a := range arms {
		parts = append(parts, fmt.Sprintf("%s=%d", a, byArm[a].surfaceCalls))
	}
	return "Surface calls: " + strings.Join(parts, ", ") + ".\n"
}

// rawArmsLine is the fallback rendering when the run's registered
// treatment/control cannot be resolved among its own arms: every arm's raw
// totals, named, rather than a comparison invented from a guess.
func rawArmsLine(arms []string, byArm map[string]*armTotals) string {
	parts := make([]string, 0, len(arms))
	for _, a := range arms {
		t := byArm[a]
		parts = append(parts, fmt.Sprintf("%s %d/%d (surface calls %d)", a, t.successes, t.trials, t.surfaceCalls))
	}
	return strings.Join(parts, "; ") + ".\n"
}

// plainLanguageLine is vision 28 §2.7: the verdict in plain language,
// before any ratio — "On <run>, fathom's arm matched/beat/lost to <control>
// on success (a v b) at <x>× the tokens; the tools were called N times."
func plainLanguageLine(runID string, crit Criterion, t, c *armTotals) string {
	verdict := "matched"
	tRate, cRate := rateOf(t.successes, t.trials), rateOf(c.successes, c.trials)
	switch {
	case tRate > cRate:
		verdict = "beat"
	case tRate < cRate:
		verdict = "lost to"
	}
	tokenPart := "an unmeasured"
	if t.tokens > 0 && c.tokens > 0 {
		tokenPart = fmt.Sprintf("%.3f×", float64(t.tokens)/float64(c.tokens))
	}
	return fmt.Sprintf("On %s, fathom's arm %s %s on success (%d v %d) at %s the tokens; the tools were called %d time(s).\n",
		runID, verdict, crit.Control, t.successes, c.successes, tokenPart, t.surfaceCalls)
}

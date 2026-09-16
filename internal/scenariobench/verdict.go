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

// criterionByID scans criteria's VALUES (never its keys — a corpus rewrite
// changes the sha key but not the criterion's own ID) for one whose ID
// equals id, returning it and true on the first match. Deterministic
// despite Go's randomized map iteration: every Criterion registered under
// a given ID in one criteria set is the same corpus's own registration, so
// which key it was found under does not matter — only that IDs collide is
// a corpus-authoring bug this function does not need to detect.
func criterionByID(criteria map[string]Criterion, id string) (Criterion, bool) {
	for _, c := range criteria {
		if c.ID == id {
			return c, true
		}
	}
	return Criterion{}, false
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
	var sha, critID string
	for _, r := range sound {
		if sha == "" {
			sha = r.CorpusSHA256
		}
		if critID == "" {
			critID = r.CriterionID
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

	// The row's own CriterionID (stamped at filing time) wins over the
	// sha-keyed lookup: a corpus rewrite after filing changes CorpusSHA256
	// but not the criterion's own ID, so a criterion still registered
	// under SOME sha in criteria (criterionByID scans criteria's values,
	// not its keys) keeps an old run citable. A row filed before
	// CriterionID existed carries "" and falls straight through to the
	// sha lookup, unchanged.
	crit, known := criteria[sha]
	if critID != "" {
		if c, ok := criterionByID(criteria, critID); ok {
			crit, known = c, true
		}
	}
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
		b.WriteString(plainLanguageLine(runID, crit, crit.Evaluate(sound), t, c))
		b.WriteString(goldSourceLine(sound, crit.Treatment))
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
// on success (a v b) at <x>× the tokens; the tools were called N times." —
// followed by the registered criterion's own verdict on the same line.
//
// The beat/lost-to/matched verb comes from the raw success rates, but a
// verb alone does not get to claim a win or a loss: ev (crit.Evaluate over
// this run's own sound rows) is the pre-registered criterion's judgment,
// and an indeterminate run — most commonly below the replication floor,
// the n=1 guard — has not measured enough to say either, so the verb is
// replaced rather than left to imply a result the criterion refused to
// certify.
func plainLanguageLine(runID string, crit Criterion, ev ScenarioEvaluation, t, c *armTotals) string {
	verb := "matched"
	tRate, cRate := rateOf(t.successes, t.trials), rateOf(c.successes, c.trials)
	switch {
	case tRate > cRate:
		verb = "beat"
	case tRate < cRate:
		verb = "lost to"
	}
	verdictPhrase := fmt.Sprintf("%s %s", verb, crit.Control)
	if ev.Verdict == ScenarioVerdictIndeterminate {
		verdictPhrase = "did not run enough trials to decide against " + crit.Control
	}
	tokenPart := "an unmeasured"
	if t.tokens > 0 && c.tokens > 0 {
		tokenPart = fmt.Sprintf("%.3f×", float64(t.tokens)/float64(c.tokens))
	}
	return fmt.Sprintf("On %s, fathom's arm %s on success (%d v %d) at %s the tokens; the tools were called %d time(s). Registered criterion %s: %s (%s).\n",
		runID, verdictPhrase, t.successes, c.successes, tokenPart, t.surfaceCalls,
		ev.CriterionID, ev.Verdict, ev.Reason)
}

// goldSourceLine is ADR-035 decision 5's verdict-block receipt: among the
// TREATMENT arm's own rows in this run, which tool earned the most PASSES
// (GoldSourcePasses) — "a cell passes the task; a tool passes only when
// it was the gold source." Empty when no row recorded a GoldSourcePasses
// reading at all (rows filed before VB-4).
func goldSourceLine(rows []ScenarioLedgerRow, treatment string) string {
	counts := map[string]int{}
	total := 0
	seen := false
	for _, r := range rows {
		if r.Arm != treatment {
			continue
		}
		total += r.Successes
		if len(r.GoldSourcePasses) > 0 {
			seen = true
			for tool, n := range r.GoldSourcePasses {
				counts[tool] += n
			}
		}
	}
	if !seen || total == 0 {
		return ""
	}
	tools := make([]string, 0, len(counts))
	for t := range counts {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		if counts[tools[i]] != counts[tools[j]] {
			return counts[tools[i]] > counts[tools[j]]
		}
		return tools[i] < tools[j]
	})
	top := tools[0]
	return fmt.Sprintf("gold source: %s earned %d of %d passes.\n", top, counts[top], total)
}

package main

// intenttriggers.go — CX-21 (docs/plans/change-and-compliance.md; design
// docs/design/change-and-compliance.md §5 step 3): the instruments a
// computable revisit or promote trigger may name, read when the operator
// runs `fathom intent audit` (or opens the Intent tab, which runs the same
// audit read-only). Nothing here runs on a schedule: the daemon never
// evaluates a trigger on its own (CX-X5).
//
// The vocabulary:
//
//	arch_violations          the newest ingest-time reading's violations,
//	                         summed over its deny rules (CX-20)
//	arch_violations:<rule>   one rule's count in that reading ("a -> b")
//	plans_failing            claimed plan rows the verify ledger refutes at
//	                         HEAD (measured-stale readings excluded)
//
// A reading that cannot be taken (no store, no reading recorded, a rule the
// reading does not carry, no verify-ledger row) is an error, so the trigger
// reads "unevaluated" with the reason — absent is never 0.

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Instrument names (the vocabulary above).
const (
	instrumentArchViolations = "arch_violations"
	instrumentPlansFailing   = "plans_failing"
)

// intentInstruments returns the audit's instrument reader for service.
// st is an open store (the console's); when nil, storePath is opened on the
// first arch_violations read, only if the file exists. rows are the
// verify-ledger readings the audit already loaded.
func intentInstruments(ctx context.Context, st store.Store, storePath, service string, rows []intent.PlanRowState) intent.InstrumentReader {
	var (
		once     sync.Once
		trend    []store.ArchViolationReading
		trendErr error
	)
	loadTrend := func() {
		if service == "" {
			trendErr = fmt.Errorf("no service resolved for this repository")
			return
		}
		s := st
		if s == nil {
			if storePath == "" {
				trendErr = fmt.Errorf("no store to read (pass --store)")
				return
			}
			if _, err := os.Stat(storePath); err != nil {
				trendErr = fmt.Errorf("store %s: %w", storePath, err)
				return
			}
			opened, err := store.Open(storePath)
			if err != nil {
				trendErr = fmt.Errorf("open store %s: %w", storePath, err)
				return
			}
			defer opened.Close()
			s = opened
		}
		trend, trendErr = s.ArchViolationTrend(ctx, service)
	}
	return func(name string) (intent.InstrumentReading, bool, error) {
		switch {
		case name == instrumentPlansFailing:
			return plansFailingReading(rows)
		case name == instrumentArchViolations || strings.HasPrefix(name, instrumentArchViolations+":"):
			once.Do(loadTrend)
			if trendErr != nil {
				return intent.InstrumentReading{}, true, trendErr
			}
			return archViolationsReading(service, trend, strings.TrimPrefix(strings.TrimPrefix(name, instrumentArchViolations), ":"))
		}
		return intent.InstrumentReading{}, false, nil
	}
}

// normalizeRuleKey spells a rule the way readings key it ("a->b" and
// "a  ->  b" both read "a -> b").
func normalizeRuleKey(s string) string {
	from, to, ok := strings.Cut(s, "->")
	if !ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(from) + " -> " + strings.TrimSpace(to)
}

// archViolationsReading reads the newest reading in trend: every rule
// summed when rule is "", else that rule's count.
func archViolationsReading(service string, trend []store.ArchViolationReading, rule string) (intent.InstrumentReading, bool, error) {
	latest := latestArchReading(trend)
	if len(latest) == 0 {
		return intent.InstrumentReading{}, true, fmt.Errorf("no violation reading recorded for %s — the repository has no .fathom/rules/archconformance.cue, or it has not been ingested since it gained one", service)
	}
	when := fmt.Sprintf("the ingest of %s, %s", shortCommit(latest[0].Commit), time.Unix(0, latest[0].RecordedAt).UTC().Format("2006-01-02"))
	if rule == "" {
		total := 0
		for _, r := range latest {
			total += r.Count
		}
		return intent.InstrumentReading{Value: float64(total), Reading: fmt.Sprintf("%d violation(s) over %d rule(s) at %s", total, len(latest), when)}, true, nil
	}
	key := normalizeRuleKey(rule)
	for _, r := range latest {
		if r.Rule == key {
			return intent.InstrumentReading{Value: float64(r.Count), Reading: fmt.Sprintf("%d violation(s) of %s at %s", r.Count, key, when)}, true, nil
		}
	}
	return intent.InstrumentReading{}, true, fmt.Errorf("rule %q is not in the newest reading (%s)", key, when)
}

// latestArchReading returns the rows of the newest reading (trend is sorted
// by seq then rule).
func latestArchReading(trend []store.ArchViolationReading) []store.ArchViolationReading {
	if len(trend) == 0 {
		return nil
	}
	last := trend[len(trend)-1].Seq
	i := len(trend)
	for i > 0 && trend[i-1].Seq == last {
		i--
	}
	return trend[i:]
}

// plansFailingReading counts the claimed rows the verify ledger refutes.
func plansFailingReading(rows []intent.PlanRowState) (intent.InstrumentReading, bool, error) {
	if len(rows) == 0 {
		return intent.InstrumentReading{}, true, fmt.Errorf("no verify-ledger reading for this repository at HEAD")
	}
	failing, measured, newest := 0, 0, ""
	for _, r := range rows {
		if r.MeasuredStale {
			continue
		}
		measured++
		if r.Failing {
			failing++
		}
		if r.MeasuredAt > newest {
			newest = r.MeasuredAt
		}
	}
	if measured == 0 {
		return intent.InstrumentReading{}, true, fmt.Errorf("every verify-ledger reading is measured-stale")
	}
	return intent.InstrumentReading{Value: float64(failing), Reading: fmt.Sprintf("%d failing of %d measured plan row(s), verify ledger %s", failing, measured, newest)}, true, nil
}

// renderIntentTriggers prints the audit's trigger verdicts: fired first,
// then not fired, then the unevaluated (computable ones with their reason,
// prose ones counted).
func renderIntentTriggers(w io.Writer, ts []intent.TriggerVerdict) {
	by := map[string][]intent.TriggerVerdict{}
	for _, t := range ts {
		by[t.State] = append(by[t.State], t)
	}
	prose := 0
	for _, t := range by[intent.TriggerUnevaluated] {
		if t.Instrument == "" {
			prose++
		}
	}
	fmt.Fprintf(w, "\ntriggers: %d revisit/promote condition(s) — %d fired, %d not fired, %d unevaluated (%d prose), %d unevaluable annotation(s)\n",
		len(ts)-len(by[intent.TriggerUnevaluable]), len(by[intent.TriggerFired]), len(by[intent.TriggerHolding]), len(by[intent.TriggerUnevaluated]), prose, len(by[intent.TriggerUnevaluable]))
	for _, st := range []string{intent.TriggerFired, intent.TriggerHolding} {
		for _, t := range by[st] {
			fmt.Fprintf(w, "  %s %s %s — %s %s: %s\n      reading: %s\n      at %s:%d\n", strings.ToUpper(st), t.Record, t.Label, t.Instrument, t.Threshold, clipText(t.Text, 120), t.Reading, t.File, t.Line)
		}
	}
	for _, t := range by[intent.TriggerUnevaluated] {
		if t.Instrument == "" {
			continue
		}
		fmt.Fprintf(w, "  UNEVALUATED %s %s — %s %s: %s\n      at %s:%d\n", t.Record, t.Label, t.Instrument, t.Threshold, t.Reason, t.File, t.Line)
	}
	if prose > 0 {
		fmt.Fprintf(w, "  %d prose trigger(s) stay human-judged — add {instrument: <name>, threshold: \"<op> <number>\"} to make one computable (instruments: %s, %s[:<rule>], %s)\n",
			prose, instrumentArchViolations, instrumentArchViolations, instrumentPlansFailing)
	}
}

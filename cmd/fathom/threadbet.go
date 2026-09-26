package main

// threadbet.go — the vision-21 pre-registered bet, evaluated as code.
//
// The claim under test: a session opened WITH a thread brief after
// startup//clear reaches its first edit within the warm-up band of resumed
// sessions, at a lower all-in per-turn cost. The registered kill rides in
// the check's own output: at n≥20 fresh units and a failed band, the
// verdict text says the injection is to be removed. The instrument carries
// its consequence, so the kill cannot quietly become a memory.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// threadBetMinFreshUnits is the registered floor before any verdict.
const threadBetMinFreshUnits = 20

// threadBetReport is the whole evaluation: counts, medians with their
// denominators, the feature's own measured cost, and the verdict in words.
type threadBetReport struct {
	FreshUnits   int `json:"fresh_units"`
	ResumedUnits int `json:"resumed_units"`
	// Warm-up: median assistant turns to the first file-changing call.
	FreshWarmupMed   *int `json:"fresh_warmup_median,omitempty"`
	ResumedWarmupMed *int `json:"resumed_warmup_median,omitempty"`
	// All-in per-turn: (in + out + cache_read/10) / turns. The /10 is a
	// PRICING ASSUMPTION (cache reads bill ~10% of input), named on every
	// surface that shows the figure — an assumption, not a measurement.
	FreshAllInMed   *int `json:"fresh_allin_median,omitempty"`
	ResumedAllInMed *int `json:"resumed_allin_median,omitempty"`
	// BriefBytesTotal is what continuity has actually SPENT — a ledger
	// sum, not an estimate (the owner's token rule).
	BriefBytesTotal int `json:"brief_bytes_total"`
	// IsolatedSpendRows counts rows written inside a `fathom assess` judge
	// (IR-4): never a cohort unit, and their brief bytes are not in
	// BriefBytesTotal. IsolatedSpendNote names the exclusion, and Detail
	// carries it, so every surface that renders the bet (the brief, the
	// flywheel) says it — counted AND named.
	IsolatedSpendRows int    `json:"isolated_spend_rows,omitempty"`
	IsolatedSpendNote string `json:"isolated_spend_note,omitempty"`
	Verdict           string `json:"verdict"` // collecting | met | not-met
	Detail            string `json:"detail"`
}

// threadBetCheck folds deduped ledger rows into the two cohorts.
//
// Cohorts: fresh-with-brief = session_source startup|clear AND a brief was
// injected; resumed = session_source resume. Rows without a source label
// (pre-stamp binaries) belong to NEITHER cohort — unknown is not a cohort
// (law 3). The ≥10-turn floor applies as in the arm comparison.
func threadBetCheck(rows []sessionLedgerEntry) threadBetReport {
	rep := threadBetReport{}
	var freshWarm, resWarm, freshAllIn, resAllIn []int
	allIn := func(e sessionLedgerEntry) (int, bool) {
		if e.TokensIn == nil || e.TokensOut == nil || e.TurnCount == nil || *e.TurnCount < abMinTurnsPerUnit {
			return 0, false
		}
		cache := 0
		if e.TokensCacheRead != nil {
			cache = *e.TokensCacheRead / 10
		}
		return (*e.TokensIn + *e.TokensOut + cache) / *e.TurnCount, true
	}
	for _, e := range rows {
		if spendIsolatedRow(e) {
			// IR-4 (pre-registered 2026-09-25): judging spend is never a
			// bet unit, and its brief bytes were never continuity's spend.
			rep.IsolatedSpendRows++
			continue
		}
		rep.BriefBytesTotal += e.BriefBytes
		switch {
		case (e.SessionSource == "startup" || e.SessionSource == "clear") && e.BriefInjected:
			if v, ok := allIn(e); ok {
				rep.FreshUnits++
				freshAllIn = append(freshAllIn, v)
				if e.FirstEditTurn != nil {
					freshWarm = append(freshWarm, *e.FirstEditTurn)
				}
			}
		case e.SessionSource == "resume":
			if v, ok := allIn(e); ok {
				rep.ResumedUnits++
				resAllIn = append(resAllIn, v)
				if e.FirstEditTurn != nil {
					resWarm = append(resWarm, *e.FirstEditTurn)
				}
			}
		}
	}
	med := func(xs []int) *int {
		if v, ok := medianInt(xs); ok {
			return &v
		}
		return nil
	}
	rep.FreshWarmupMed, rep.ResumedWarmupMed = med(freshWarm), med(resWarm)
	rep.FreshAllInMed, rep.ResumedAllInMed = med(freshAllIn), med(resAllIn)

	switch {
	case rep.FreshUnits < threadBetMinFreshUnits:
		rep.Verdict = "collecting"
		rep.Detail = fmt.Sprintf("%d of %d fresh-with-brief unit(s); %d resumed baseline unit(s). "+
			"No verdict below the registered floor.", rep.FreshUnits, threadBetMinFreshUnits, rep.ResumedUnits)
	case rep.ResumedUnits == 0 || rep.ResumedWarmupMed == nil || rep.FreshWarmupMed == nil ||
		rep.ResumedAllInMed == nil || rep.FreshAllInMed == nil:
		rep.Verdict = "collecting"
		rep.Detail = "the fresh floor is met but a baseline median is still missing — named, not guessed"
	case *rep.FreshWarmupMed <= *rep.ResumedWarmupMed && *rep.FreshAllInMed < *rep.ResumedAllInMed:
		rep.Verdict = "met"
		rep.Detail = fmt.Sprintf("fresh-with-brief warms up as fast (%d vs %d turns to first edit) at lower "+
			"all-in cost (%s vs %s tokens/turn, cache reads weighted at the 10%% pricing assumption)",
			*rep.FreshWarmupMed, *rep.ResumedWarmupMed,
			fmtThousands(*rep.FreshAllInMed), fmtThousands(*rep.ResumedAllInMed))
	default:
		rep.Verdict = "not-met"
		rep.Detail = fmt.Sprintf("the registered band failed at n=%d (warm-up %v vs %v; all-in %v vs %v) — "+
			"per the registered kill, the SessionStart injection is to be REMOVED; a continuity feature that "+
			"does not pay its ~600 tokens is context pollution with a good conscience",
			rep.FreshUnits, derefOr(rep.FreshWarmupMed), derefOr(rep.ResumedWarmupMed),
			derefOr(rep.FreshAllInMed), derefOr(rep.ResumedAllInMed))
	}
	if rep.IsolatedSpendRows > 0 {
		rep.IsolatedSpendNote = fmtThousands(rep.IsolatedSpendRows) + " session(s) ran inside a `fathom assess` judge (spend_label set) — " +
			"the cost of judging a unit, so never a bet unit and never in the brief-byte sum (IR-4, pre-registered 2026-09-25)"
		rep.Detail = strings.TrimSuffix(rep.Detail, ".") + fmt.Sprintf("; %s session(s) excluded as judging spend (IR-4)", fmtThousands(rep.IsolatedSpendRows))
	}
	return rep
}

func derefOr(p *int) any {
	if p == nil {
		return "—"
	}
	return *p
}

// dedupedLedgerRows is the shared session-level fold (one row per session,
// last stop wins) for consumers outside the impact model.
func dedupedLedgerRows() []sessionLedgerEntry {
	data, err := readFileMaybe(sessionLedgerPath())
	if err != nil {
		return nil
	}
	latest := map[string]sessionLedgerEntry{}
	var order []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e sessionLedgerEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		key := e.SessionID
		if key == "" {
			key = "\x00anon:" + e.At + ":" + e.Repo
		}
		if _, seen := latest[key]; !seen {
			order = append(order, key)
		}
		latest[key] = e
	}
	out := make([]sessionLedgerEntry, 0, len(order))
	for _, k := range order {
		out = append(out, latest[k])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

func readFileMaybe(p string) ([]byte, error) { return os.ReadFile(p) }

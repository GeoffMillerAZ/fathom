package console

// ruleoutcomes.go — the reader half of SR-61 (surface reckoning F-27/U4,
// design §6): the edit hook records served, top_changed and still_served per
// claim id (cmd/fathom's ruleoutcome_track.go, into the GL-10 signal log); this is
// what `fathom obligations` and the serving explainer show of it, beside the
// rules they list. A pure fold over rows the caller already decoded — the
// counts and the class are semantic.RuleEffectiveness's own (GL-11), so the
// console and the learning loop can never disagree about what a rule's
// record is.
//
// Absent is never 0 (vision 35 law 3): no log yet is absent with its reason,
// an unreadable log is an error, and a log with a malformed row is a lower
// bound. A claim missing from a log that IS there is a measured zero — the
// hook had its chance and did not serve it.

import (
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// RuleOutcomeWindowDays is the window the counts are read over: GL-11's
// 30-day effectiveness window, the only one design doc §5's ratio classes
// speak of.
const RuleOutcomeWindowDays = 30

// ruleOutcomeNeverTriggeredDays is GL-11's wider window: a rule is never
// triggered only when it went unserved for this long, so a log younger than
// it cannot say so.
const ruleOutcomeNeverTriggeredDays = 90

// RuleOutcomeRead is what the caller learned reading the signal log: the
// raw material for the truth state, decoupled from where the file lives.
type RuleOutcomeRead struct {
	// Present is false when no signal log exists (no hook has ever recorded
	// a served rule on this machine).
	Present bool
	// Err names a log that exists and could not be read.
	Err error
	// Rows is the rows decoded; Skipped the malformed rows left out.
	Rows, Skipped int
	// Since is the earliest row's timestamp; zero when there is none.
	Since time.Time
}

// RuleOutcomeCounts is one served rule's hook record over the window.
type RuleOutcomeCounts struct {
	ClaimID string `json:"claim_id"`
	Served  int    `json:"served"`
	// TopChanged and StillServed (JF-2; Complied and Violated before) are
	// the edit hook's top_changed and still_served events: another rule
	// topped the file's served set this turn, or the same rule still did on
	// a repeat edit. They count scope churn and persistence, not edits that
	// complied with a rule or not.
	TopChanged   int `json:"top_changed"`
	StillServed  int `json:"still_served"`
	Blocked      int `json:"blocked"`
	Refuted      int `json:"refuted"`
	Corroborated int `json:"corroborated"`
	// Class is GL-11's class (ignored, effective, contested, never_triggered)
	// when one applies and the log is old enough to support it; a class is a
	// proposal for a human, never an action.
	Class  string `json:"class,omitempty"`
	Remedy string `json:"remedy,omitempty"`
}

// RuleOutcomeSet is the counts for the rules a call listed, in that order.
type RuleOutcomeSet struct {
	WindowDays int                 `json:"window_days"`
	Rules      []RuleOutcomeCounts `json:"rules"`
}

// ByClaim returns the counts for claimID.
func (s RuleOutcomeSet) ByClaim(claimID string) (RuleOutcomeCounts, bool) {
	for _, r := range s.Rules {
		if r.ClaimID == claimID {
			return r, true
		}
	}
	return RuleOutcomeCounts{}, false
}

const ruleOutcomeSource = "the edit hook's rule-outcome signal log (served, top_changed, still_served per claim id)"

// RuleOutcomesView folds the decoded signal-log events into the counts for
// claimIDs (the rules a call listed; blanks — a raw-fallback row has no
// claim — are skipped). now is injected.
func RuleOutcomesView(claimIDs []string, events []semantic.RuleOutcomeEvent, read RuleOutcomeRead, now time.Time) Value[RuleOutcomeSet] {
	switch {
	case read.Err != nil:
		return Errored[RuleOutcomeSet]("the rule-outcome log is unreadable: " + read.Err.Error()).WithSource(ruleOutcomeSource)
	case !read.Present:
		return Absent[RuleOutcomeSet]("no edit hook has recorded a served rule on this machine yet, so no rule has a record").WithSource(ruleOutcomeSource)
	}
	ids := make([]string, 0, len(claimIDs))
	seen := map[string]bool{}
	for _, id := range claimIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return Absent[RuleOutcomeSet]("no compiled rule is served here, so there is no rule record to count").WithSource(ruleOutcomeSource)
	}

	results := map[string]semantic.RuleEffectivenessResult{}
	for _, r := range semantic.RuleEffectiveness(ids, events, now) {
		results[r.RuleID] = r
	}
	spansNeverTriggered := !read.Since.IsZero() && !read.Since.After(now.AddDate(0, 0, -ruleOutcomeNeverTriggeredDays))
	set := RuleOutcomeSet{WindowDays: RuleOutcomeWindowDays, Rules: make([]RuleOutcomeCounts, 0, len(ids))}
	for _, id := range ids {
		r := results[id]
		c := RuleOutcomeCounts{
			ClaimID: id, Served: r.Served30, TopChanged: r.TopChanged30, StillServed: r.StillServed30,
			Blocked: r.Blocked30, Refuted: r.Refuted30, Corroborated: r.Corroborated30,
			Class: string(r.Class), Remedy: r.Remedy,
		}
		if r.Class == semantic.RuleClassNeverTriggered && !spansNeverTriggered {
			// The log began inside the 90-day window: "never served in 90
			// days" is a claim it cannot make yet.
			c.Class, c.Remedy = "", ""
		}
		set.Rules = append(set.Rules, c)
	}

	v := Present(set, ruleOutcomeSource)
	v.Denominator = fmt.Sprintf("the last %d days of %d logged rule events (log since %s)", RuleOutcomeWindowDays, read.Rows, sinceLabel(read.Since))
	if read.Skipped > 0 {
		return Partial(set, fmt.Sprintf("%d malformed row(s) in the log were left out, so these counts are a lower bound", read.Skipped)).
			WithSource(ruleOutcomeSource).WithDenominator(v.Denominator)
	}
	return v
}

func sinceLabel(t time.Time) string {
	if t.IsZero() {
		return "its first row"
	}
	return t.UTC().Format("2006-01-02")
}

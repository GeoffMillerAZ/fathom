// ruleeffectiveness.go — GL-11 (docs/plans/governance-learning.md, design
// doc §5 "The signal source and rule effectiveness (U4)"): a rule served a
// hundred times and ignored a hundred and first time carries a signal
// fathom never computed before this. RuleEffectiveness is the PROJECTION
// over GL-10's signal log (cmd/fathom's recordRuleOutcome, read back by its
// caller as RuleOutcomeEvent rows) — per rule, counts and rates over a
// window, with n, classified into one of design doc §5's four classes,
// each carrying a proposed remedy. A remedy is a PROPOSAL only: nothing
// here writes a rules file or flips a claim's status (GL-X1/GL-X2 —
// promotion and retirement under published thresholds is GL-13's own,
// separate, announced step).
//
// Determinism (internal/CLAUDE.md): now is injected by every caller, never
// read from time.Now() here; output is sorted by RuleID.
package semantic

import (
	"sort"
	"time"
)

// Rule outcome event kinds (design doc §5's table plus the pre-existing
// human claim verdicts, GL-10). Mirrors cmd/fathom's ruleOutcome* constants
// by literal value — internal/semantic cannot import cmd/fathom (import
// direction), so the two sides agree on the wire vocabulary rather than
// sharing a Go identifier.
const (
	RuleEventServed       = "served"
	RuleEventComplied     = "complied"
	RuleEventViolated     = "violated"
	RuleEventBlocked      = "blocked"
	RuleEventRefuted      = "refuted"
	RuleEventCorroborated = "corroborated"
)

// RuleOutcomeEvent is one row from GL-10's signal log: which rule, what
// happened, when.
type RuleOutcomeEvent struct {
	RuleID string
	Event  string
	At     time.Time

	// Session (GL-13, design doc §6) is the identity that produced the
	// event — the harness session id, or "human:cli" for a human running
	// the CLI with no harness session exported (cmd/fathom's
	// claimVerdictIdentity). "" for an event whose writer predates this
	// field, or a served/complied/violated/blocked row an edit/PreToolUse
	// hook wrote before that wiring carries a session (GL-10's own scope).
	// applyLearning (GL-13) is this field's only reader: RuleEffectiveness
	// itself (GL-11) classifies by counts and rates alone and never looks
	// at WHO produced an event, so adding this field changes nothing about
	// its classification.
	Session string
}

// RuleEffectivenessClass is one of design doc §5's four classes.
// RuleClassNone means neither a ratio class (ignored/effective — these
// need n >= ruleEffectivenessMinN served in the 30-day window) nor
// never_triggered (zero served in 90 days) nor contested (>=3 refutes)
// applies: genuinely not yet classifiable, never defaulted to a named
// class just to fill the field.
type RuleEffectivenessClass string

const (
	RuleClassNone           RuleEffectivenessClass = ""
	RuleClassIgnored        RuleEffectivenessClass = "ignored"
	RuleClassNeverTriggered RuleEffectivenessClass = "never_triggered"
	RuleClassEffective      RuleEffectivenessClass = "effective"
	RuleClassContested      RuleEffectivenessClass = "contested"
)

// ruleEffectivenessMinN is the served-count floor (design doc §5 table
// header: "30-day window, n at least 20 served") below which the ratio
// classes (ignored, effective) never fire — a rule served twice that was
// violated once is not "50% ignored," it is unmeasured.
const ruleEffectivenessMinN = 20

// ruleContestedMinRefutes / ruleIgnoredMinRatio / ruleEffectiveMinRatio are
// design doc §5's table thresholds, named so classifyRule reads like the
// table rather than a wall of literals.
const (
	ruleContestedMinRefutes = 3
	ruleIgnoredMinRatio     = 0.5
	ruleEffectiveMinRatio   = 0.9
)

// ruleEffectivenessWindow / ruleNeverTriggeredWindow are the two windows
// design doc §5's table uses: 30 days for the ratio and contested classes,
// 90 days for "never triggered" (its own row explicitly overrides the
// table header's default window).
const (
	ruleEffectivenessWindow  = 30 * 24 * time.Hour
	ruleNeverTriggeredWindow = 90 * 24 * time.Hour
)

// RuleEffectivenessResult is one rule's computed classification: counts
// over the relevant window, the class (if any applies) and its proposed
// remedy.
type RuleEffectivenessResult struct {
	RuleID string

	// Served30/Complied30/Violated30/Blocked30/Refuted30/Corroborated30 are
	// counts within the 30-day window ending at the caller's now.
	Served30       int
	Complied30     int
	Violated30     int
	Blocked30      int
	Refuted30      int
	Corroborated30 int
	// Served90 is the served count over the wider 90-day window
	// never_triggered reads.
	Served90 int

	// Class is "" (RuleClassNone) when no class's condition is met.
	Class RuleEffectivenessClass
	// Remedy is "" exactly when Class is RuleClassNone.
	Remedy string
}

// RuleEffectiveness projects events into one RuleEffectivenessResult per
// rule ID: every id in ruleIDs (so a rule with ZERO events still reads as
// never_triggered rather than silently disappearing — design doc §5's
// whole point for that class), plus any rule ID events mentions that
// ruleIDs left out (an event that already happened is never dropped for a
// caller's incomplete rule list). now is the injected clock
// (internal/CLAUDE.md's determinism contract): production callers pass
// time.Now(), tests pass a fixed instant. Output is sorted by RuleID.
func RuleEffectiveness(ruleIDs []string, events []RuleOutcomeEvent, now time.Time) []RuleEffectivenessResult {
	known := make(map[string]bool, len(ruleIDs))
	for _, id := range ruleIDs {
		known[id] = true
	}
	for _, e := range events {
		known[e.RuleID] = true
	}

	out := make([]RuleEffectivenessResult, 0, len(known))
	for id := range known {
		out = append(out, computeRuleEffectiveness(id, events, now))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RuleID < out[j].RuleID })
	return out
}

// computeRuleEffectiveness folds every event for ruleID into one result.
func computeRuleEffectiveness(ruleID string, events []RuleOutcomeEvent, now time.Time) RuleEffectivenessResult {
	r := RuleEffectivenessResult{RuleID: ruleID}
	cutoff30 := now.Add(-ruleEffectivenessWindow)
	cutoff90 := now.Add(-ruleNeverTriggeredWindow)
	for _, e := range events {
		if e.RuleID != ruleID || e.At.After(now) {
			continue
		}
		if !e.At.Before(cutoff90) && e.Event == RuleEventServed {
			r.Served90++
		}
		if e.At.Before(cutoff30) {
			continue
		}
		switch e.Event {
		case RuleEventServed:
			r.Served30++
		case RuleEventComplied:
			r.Complied30++
		case RuleEventViolated:
			r.Violated30++
		case RuleEventBlocked:
			r.Blocked30++
		case RuleEventRefuted:
			r.Refuted30++
		case RuleEventCorroborated:
			r.Corroborated30++
		}
	}
	r.Class, r.Remedy = classifyRule(r)
	return r
}

// classifyRule applies design doc §5's table, in the table's own row
// order: ignored, never_triggered, effective, contested. The ratio classes
// (ignored, effective) require Served30 >= ruleEffectivenessMinN, so they
// can never overlap never_triggered's Served90 == 0 condition (n=0 in the
// 90-day superset implies n=0 in the 30-day window too). ignored is
// checked before effective only because the table lists it first; a rule
// cannot satisfy both (a violated/served ratio >= 0.5 and a complied/
// served ratio >= 0.9 cannot both hold when complied+violated <= served).
func classifyRule(r RuleEffectivenessResult) (RuleEffectivenessClass, string) {
	switch {
	case r.Served30 >= ruleEffectivenessMinN && r.Blocked30 == 0 &&
		float64(r.Violated30)/float64(r.Served30) >= ruleIgnoredMinRatio:
		return RuleClassIgnored, "move to a hook (deny) or rewrite with the violating example"
	case r.Served90 == 0:
		return RuleClassNeverTriggered, "prune or re-scope"
	case r.Served30 >= ruleEffectivenessMinN &&
		float64(r.Complied30)/float64(r.Served30) >= ruleEffectiveMinRatio:
		return RuleClassEffective, "keep"
	case r.Refuted30 >= ruleContestedMinRefutes:
		return RuleClassContested, "human review"
	default:
		return RuleClassNone, ""
	}
}

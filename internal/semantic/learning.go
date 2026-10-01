// learning.go — GL-13 (docs/plans/governance-learning.md, design doc
// docs/design/governance-learning.md §6 "Rules that learn"): promotion
// (candidate → active) and retirement (active → retired) of a compiled
// claim's Lifecycle under GL-12's published thresholds, fed by GL-10/
// GL-11's unified signal vocabulary (RuleOutcomeEvent; RuleID == ClaimID
// for every row a claim verdict or an edit-hook decision produced — see
// cmd/fathom's recordRuleOutcome call sites).
//
// Retirement closes the claim's bi-temporal validity window (ValidTo is
// set to the caller's now) rather than deleting the row — the same "flip
// a status, never remove a claim" discipline core.ClaimStatus's own doc
// comment states, extended to the orthogonal Lifecycle axis. GL-X1
// refuses silent learning: every transition this file makes is returned
// as a LearningDecision carrying the signals that moved it, for GL-14's
// learningAnnouncements to announce — applyLearning itself writes nothing
// and announces nothing; it only decides.
//
// Determinism (internal/AGENTS.md): now is the injected clock, never
// time.Now() here; output claims are returned in the SAME order as the
// input (a caller's own PutClaims ordering is undisturbed), and decisions
// are sorted by ClaimID.
package semantic

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// LearningDecision is one promotion or retirement applyLearning made,
// carrying the signals that justified it — GL-14's learningAnnouncements
// reads these verbatim rather than recomputing them, so the announcement
// can never drift from the decision that was actually made.
type LearningDecision struct {
	ClaimID string
	From    core.ClaimLifecycle
	To      core.ClaimLifecycle
	// Reason is a human-readable citation of the signals below, e.g.
	// "3 corroborations from 2 sessions, 0 refutes over 30d".
	Reason string

	// Corroborations/DistinctSessions/Refutes/Served are the counts the
	// decision was measured against — promotion's are windowed
	// (thresholds.PromoteWindowDays), a refute-retirement's are all-time,
	// and a never-triggered retirement's Served is windowed
	// (thresholds.RetireNeverTriggeredDays). Never all populated at once;
	// see the Reason string for which ones apply to THIS decision.
	Corroborations   int
	DistinctSessions int
	Refutes          int
	Served           int
}

// applyLearning applies design doc §6's promotion/retirement rule to
// claims, given events (the unified GL-10/GL-11 signal — served/top_changed/
// still_served/blocked/refuted/corroborated) and thresholds (GL-12). now is
// the injected clock.
//
// Promotion (candidate → active): corroborations reach
// thresholds.PromoteCorroborations from at least
// thresholds.PromoteMinSessions DISTINCT sessions, both counted within the
// trailing thresholds.PromoteWindowDays, AND refutes are 0 in that same
// window (design doc §6). A corroboration event with no session identity
// (Session=="", a legacy or test-constructed row) still counts toward the
// raw total but can never itself supply a distinct session.
//
// Retirement (active → retired): EITHER refutes reach
// thresholds.RetireRefutes with refutes greater than corroborations
// (all-time counts — design doc §6 names no window for this condition,
// and LearningThresholds carries none), OR the claim goes unserved for
// thresholds.RetireNeverTriggeredDays. The never-triggered condition is
// judged only once the claim itself is at least that old (by ValidFrom;
// a zero ValidFrom is treated as unconstrained/old-enough, mirroring
// core.SemanticClaim.ValidAt's own "zero means unconstrained" convention)
// — a claim promoted moments ago has not yet had a fair chance to be
// served, and design doc §6's "never triggered for 180 days" describes a
// claim's whole unserved history, not an empty trailing window on a
// brand-new one.
//
// A retired claim is terminal: applyLearning never re-examines one (no
// decision, no ValidTo change) — retirement is a one-time event.
func applyLearning(claims []core.SemanticClaim, events []RuleOutcomeEvent, thresholds LearningThresholds, now time.Time) ([]core.SemanticClaim, []LearningDecision) {
	byClaim := make(map[string][]RuleOutcomeEvent, len(claims))
	for _, e := range events {
		if e.At.After(now) {
			continue // never trust a future-dated event
		}
		byClaim[e.RuleID] = append(byClaim[e.RuleID], e)
	}

	promoteCutoff := now.Add(-time.Duration(thresholds.PromoteWindowDays) * 24 * time.Hour)
	neverTriggeredCutoff := now.Add(-time.Duration(thresholds.RetireNeverTriggeredDays) * 24 * time.Hour)

	out := make([]core.SemanticClaim, len(claims))
	var decisions []LearningDecision
	for i, c := range claims {
		out[i] = c
		rows := byClaim[c.ClaimID]

		switch c.Lifecycle.EffectiveLifecycle() {
		case core.LifecycleCandidate:
			corrob, sessions, refutes := windowedVerdictSignal(rows, promoteCutoff)
			if corrob >= thresholds.PromoteCorroborations &&
				len(sessions) >= thresholds.PromoteMinSessions &&
				refutes == 0 {
				out[i].Lifecycle = core.LifecycleActive
				decisions = append(decisions, LearningDecision{
					ClaimID: c.ClaimID, From: core.LifecycleCandidate, To: core.LifecycleActive,
					Reason: fmt.Sprintf("%d corroborations from %d distinct sessions, 0 refutes over %dd",
						corrob, len(sessions), thresholds.PromoteWindowDays),
					Corroborations: corrob, DistinctSessions: len(sessions),
				})
			}

		case core.LifecycleActive:
			corrobAll, refutesAll := allTimeVerdictCounts(rows)
			switch {
			case refutesAll >= thresholds.RetireRefutes && refutesAll > corrobAll:
				out[i].Lifecycle = core.LifecycleRetired
				out[i].ValidTo = now
				decisions = append(decisions, LearningDecision{
					ClaimID: c.ClaimID, From: core.LifecycleActive, To: core.LifecycleRetired,
					Reason:         fmt.Sprintf("%d refutes > %d corroborations", refutesAll, corrobAll),
					Refutes:        refutesAll,
					Corroborations: corrobAll,
				})
			case oldEnoughToJudge(c.ValidFrom, now, thresholds.RetireNeverTriggeredDays) &&
				servedSince(rows, neverTriggeredCutoff) == 0:
				out[i].Lifecycle = core.LifecycleRetired
				out[i].ValidTo = now
				decisions = append(decisions, LearningDecision{
					ClaimID: c.ClaimID, From: core.LifecycleActive, To: core.LifecycleRetired,
					Reason: fmt.Sprintf("never triggered for %dd", thresholds.RetireNeverTriggeredDays),
					Served: 0,
				})
			}

		case core.LifecycleRetired:
			// Terminal — no further transition.
		}
	}

	sort.Slice(decisions, func(i, j int) bool { return decisions[i].ClaimID < decisions[j].ClaimID })
	return out, decisions
}

// ApplyLearning is applyLearning's exported face: `fathom claim learn`
// (GL-13, cmd/fathom/claim_learn.go) is its product caller, and persists
// and announces what it decides.
func ApplyLearning(claims []core.SemanticClaim, events []RuleOutcomeEvent, thresholds LearningThresholds, now time.Time) ([]core.SemanticClaim, []LearningDecision) {
	return applyLearning(claims, events, thresholds, now)
}

// CloseClaimValidity is closeClaimValidity's exported face (GL-15, the one
// claim-ending path): `fathom claim learn --apply` closes a retired
// claim's validity window through it, never deleting the row.
func CloseClaimValidity(ctx context.Context, st store.Store, serviceID, claimID string, status core.ClaimStatus, at time.Time) error {
	return closeClaimValidity(ctx, st, serviceID, claimID, status, at)
}

// windowedVerdictSignal folds rows within [cutoff, now] into promotion's
// three numbers: the raw corroboration count, the set of distinct
// corroborating sessions, and the raw refute count.
func windowedVerdictSignal(rows []RuleOutcomeEvent, cutoff time.Time) (corroborations int, sessions map[string]bool, refutes int) {
	sessions = map[string]bool{}
	for _, e := range rows {
		if e.At.Before(cutoff) {
			continue
		}
		switch e.Event {
		case RuleEventCorroborated:
			corroborations++
			if e.Session != "" {
				sessions[e.Session] = true
			}
		case RuleEventRefuted:
			refutes++
		}
	}
	return corroborations, sessions, refutes
}

// allTimeVerdictCounts folds EVERY row (no window) into retirement's
// refute-vs-corroboration comparison — design doc §6 names no window for
// this condition, unlike promotion's explicit 30-day one.
func allTimeVerdictCounts(rows []RuleOutcomeEvent) (corroborations, refutes int) {
	for _, e := range rows {
		switch e.Event {
		case RuleEventCorroborated:
			corroborations++
		case RuleEventRefuted:
			refutes++
		}
	}
	return corroborations, refutes
}

// servedSince counts "served" rows at or after cutoff.
func servedSince(rows []RuleOutcomeEvent, cutoff time.Time) int {
	n := 0
	for _, e := range rows {
		if e.Event == RuleEventServed && !e.At.Before(cutoff) {
			n++
		}
	}
	return n
}

// oldEnoughToJudge reports whether a claim minted at validFrom has existed
// for at least windowDays as of now — the guard that keeps a
// freshly-promoted claim from retiring on its first learning pass for
// having (unsurprisingly) zero served events yet. A zero validFrom (a
// claim whose mint time is unknown — the same additive-migration gap
// core.SemanticClaim.ValidAt's own doc comment names) is treated as
// unconstrained: old enough.
func oldEnoughToJudge(validFrom, now time.Time, windowDays int) bool {
	if validFrom.IsZero() {
		return true
	}
	return !now.Before(validFrom.Add(time.Duration(windowDays) * 24 * time.Hour))
}

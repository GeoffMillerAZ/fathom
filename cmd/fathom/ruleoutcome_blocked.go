// ruleoutcome_blocked.go — PR-10 (docs/plans/served-not-kept.md): the
// PreToolUse enforce deny (enforce.go, edithook_pre.go) is the one place a
// `blocked` outcome can happen, and ruleOutcomeBlocked sat declared in
// ruleoutcome.go with no writer. recordBlockedOutcome is that writer.
//
// THE KEY. Every other row in the ledger is keyed by a claim id
// (trackRuleOutcomeAt: the served rule's core.SemanticClaim.ClaimID, a
// content-addressed sha256 minted when the intent corpus is compiled), and
// internal/semantic.RuleEffectiveness groups by that key. An enforced
// violation has no claim id the hook can reach: a guard's identity is its
// Note (guardspec's guardRuleID — a guard has no id field), a content check
// names an invariant (I19, I28-secrets, I31, I36) and a layer rule its own
// id. Deriving the claim behind any of those needs the store (or the daemon
// that holds it) — and the pre-hook's enforcement path is daemon-free by
// contract (I53; enforce.go's own header). So a blocked row is keyed by the
// violation's OWN identity behind an `enforce:` qualifier. A claim id is
// 64 hex characters and never contains a colon, so the two key spaces
// cannot collide (enforceOutcomeID; TestRecordBlockedOutcomeRefusesAClaimShapedKey).
//
// THE GAP THIS LEAVES (O163). Blocked rows do not join to the served /
// top_changed / still_served rows of the same rule, because those carry a claim id
// and these carry an enforce: key; the effectiveness view counts them as
// "rule ids in the signal log that are not claims" and says nothing more.
// That is harmless today: PR-D6 (the deferred GL-11 reclassification) is
// written to EXCLUDE enforce:true rules from effectiveness, since a rule
// that denies has no `ignored` to measure, and the prefix is the key that
// exclusion reads.
package main

import "strings"

// enforceOutcomePrefix qualifies a blocked row's key so it can never equal
// a claim id.
const enforceOutcomePrefix = "enforce:"

// enforceOutcomeID is the ledger key for a denied violation:
// enforce:<check id>:<rule id>. The check id is a plain identifier
// (forbidden_path, org_token, import_direction, ...), so the rule id — which
// may itself contain colons, a guard Note is prose — is everything after the
// second colon.
func enforceOutcomeID(checkID, ruleID string) string {
	if ruleID == "" {
		return "" // recordRuleOutcome drops a blank id; an unattributable block is no row
	}
	if checkID == "" {
		checkID = "unknown"
	}
	return enforceOutcomePrefix + checkID + ":" + ruleID
}

// OutcomeID is the ledger key for this decision's first violation.
func (d enforceDecision) OutcomeID() string {
	return enforceOutcomeID(d.CheckID, d.RuleID)
}

// recordBlockedOutcome appends one `blocked` row for an enforce deny the
// codec actually emitted. ruleID is enforceDecision.OutcomeID(); an id that
// is not enforce:-qualified (a claim id, say) is dropped rather than
// recorded, so a blocked row can never be attributed to a claim it has no
// business naming. Like every ledger write it is silent on failure: a
// ledger that cannot be written must never break the deny it describes.
func recordBlockedOutcome(ruleID, session, file string) {
	if !strings.HasPrefix(ruleID, enforceOutcomePrefix) {
		return
	}
	recordRuleOutcome(ruleID, ruleOutcomeBlocked, session, file)
}

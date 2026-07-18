package scoring

import (
	"math"
	"strings"
)

// NormalizeResult maps uppercase result strings to canonical lowercase.
// Anything unrecognized → "partial" (conservative default).
func NormalizeResult(str string) string {
	switch strings.TrimSpace(str) {
	case "PASS":
		return "pass"
	case "FAIL":
		return "fail"
	case "PARTIAL":
		return "partial"
	case "CONTRADICTS", "CONTRADICTION":
		return "contradicts"
	default:
		return "partial"
	}
}

// NormalizeEvidenceTier maps non-standard LLM-produced evidence tier strings
// to the canonical Tier vocabulary: TierVerified, TierPreScan, TierDocClaimed,
// TierSMEStated, TierAssumed. Two passes: an exact alias-map lookup
// (case-insensitive), then a substring fuzzy match. Falls back to TierAssumed
// for anything unrecognizable; canonical values pass through unchanged, so the
// function is idempotent. Call it before TierRank / TierCeiling when the input
// did not originate from this package's constants.
func NormalizeEvidenceTier(tier string) Tier {
	normalized := map[string]Tier{
		"code_verified":   TierVerified,
		"code_reviewed":   TierVerified,
		"code_seen":       TierVerified,
		"code":            TierVerified,
		"direct":          TierVerified,
		"observed":        TierVerified,
		"config_found":    TierVerified,
		"config_verified": TierVerified,
		"sme-verified":    TierVerified,
		"sme_verified":    TierVerified,
		"red":             TierPreScan,
		"doc_claimed":     TierDocClaimed,
		"documentation":   TierDocClaimed,
		"sme_stated":      TierSMEStated,
	}

	if mapped, ok := normalized[strings.ToLower(tier)]; ok {
		return mapped
	}

	lower := strings.ToLower(tier)
	switch {
	case strings.Contains(lower, "verif") || strings.Contains(lower, "code") ||
		strings.Contains(lower, "direct") || strings.Contains(lower, "config"):
		return TierVerified
	case strings.Contains(lower, "scan") || strings.Contains(lower, "infer"):
		return TierPreScan
	case strings.Contains(lower, "doc") || strings.Contains(lower, "claim"):
		return TierDocClaimed
	case strings.Contains(lower, "sme") || strings.Contains(lower, "expert"):
		return TierSMEStated
	case strings.Contains(lower, "assum"):
		return TierAssumed
	}

	validTiers := map[Tier]bool{
		TierVerified: true, TierPreScan: true, TierDocClaimed: true,
		TierSMEStated: true, TierAssumed: true,
	}
	if validTiers[tier] {
		return tier
	}
	return TierAssumed
}

// ApplyCeiling enforces evidence-tier confidence caps on an existing
// confidence value: TierDocClaimed and TierSMEStated cap at 2; TierAssumed
// caps at 1. TierVerified and TierPreScan are uncapped by this function
// (their ceilings, 5 and 4, are reported by TierCeiling). Tier matching is
// case-insensitive on the canonical hyphenated spellings.
func ApplyCeiling(confidence int, tier Tier) int {
	switch strings.ToLower(tier) {
	case TierDocClaimed, TierSMEStated:
		if confidence > 2 {
			return 2
		}
	case TierAssumed:
		if confidence > 1 {
			return 1
		}
	}
	return confidence
}

// TierRank returns the ordinal TRUST-ORDERING rank of an evidence tier
// (higher = more trusted): TierVerified → 5, TierPreScan → 4,
// TierDocClaimed → 3, TierSMEStated → 2, TierAssumed and anything
// unrecognized → 0. Use it to compare two pieces of evidence and pick the
// better-sourced one.
//
// Ranks are positions on the tier ladder, NOT confidence values. In
// particular TierRank(TierDocClaimed) = 3 while
// TierCeiling(TierDocClaimed) = 2 — that asymmetry is intentional, not a
// bug. Rank says a documentation claim outranks an SME statement when
// choosing between sources; ceiling says neither unverified source ever
// justifies confidence above 2. The two scales merely coincide for verified
// (5/5) and pre-scan (4/4).
//
// Accepts the canonical hyphenated spellings plus underscore variants
// (pre_scan, doc_claimed, sme_stated); for arbitrary LLM output, run
// NormalizeEvidenceTier first.
func TierRank(tier Tier) int {
	switch tier {
	case TierVerified:
		return 5
	case TierPreScan, "pre_scan":
		return 4
	case TierDocClaimed, "doc_claimed":
		return 3
	case TierSMEStated, "sme_stated":
		return 2
	default:
		return 0
	}
}

// TierCeiling returns the MAXIMUM CONFIDENCE (0–5 scale) permitted for an
// evidence tier: TierVerified → 5, TierPreScan → 4, TierDocClaimed → 2,
// TierSMEStated → 2, TierAssumed and anything unrecognized → 1.
//
// This is a confidence cap, not an ordering — see TierRank for why
// doc-claimed is ranked 3 but capped at 2. Doc-claimed and sme-stated share
// the same ceiling on purpose: both are unverified hearsay, and the platform
// refuses to award confidence 3+ to any claim that has not at least been
// pre-scanned. ApplyCeiling enforces the same caps on an existing confidence
// value, and DeriveConfidenceFromTier derives confidence from this ceiling.
//
// Accepts the canonical hyphenated spellings plus underscore variants; for
// arbitrary LLM output, run NormalizeEvidenceTier first.
func TierCeiling(tier Tier) int {
	switch tier {
	case TierVerified:
		return 5
	case TierPreScan, "pre_scan":
		return 4
	case TierDocClaimed, "doc_claimed", TierSMEStated, "sme_stated":
		return 2
	default:
		return 1
	}
}

// DeriveConfidenceFromTier computes confidence from evidence tier × corroboration.
// Corroboration means ≥2 distinct file paths; the same file cited twice is not independent.
// Corroborated → TierCeiling(tier). Not corroborated → ceiling - 1 (min 1).
func DeriveConfidenceFromTier(tier Tier, corroborated bool) int {
	ceiling := TierCeiling(tier)
	if corroborated {
		return ceiling
	}
	if ceiling > 1 {
		return ceiling - 1
	}
	return 1
}

// DeriveDisposition maps investigation state + confidence + result + evidence gap
// to the 7-value disposition taxonomy:
// pass / lean_pass / mixed / lean_fail / fail / contradiction / not_applicable.
//
// Key rules:
//   - contradiction result → always "contradiction"
//   - evidence_gap result → "lean_fail"
//   - needs_sources or exploring+confidence=0 → "not_applicable"
//   - truncated (not converged/abandoned/needs_sources/exploring) → "lean_fail"
//   - pass + confidence≥4 → "pass"; pass + confidence<4 → "lean_pass"
//   - fail + evidence gap → "lean_fail"; fail without → "fail"
//   - partial + evidence gap → "lean_fail"; partial + confidence≥3 → "mixed"; else → "lean_fail"
func DeriveDisposition(result string, confidence int, state string, hasEvidenceGap bool) string {
	if result == "contradiction" || result == "contradicts" {
		return "contradiction"
	}
	if result == "evidence_gap" {
		return "lean_fail"
	}
	if state == "needs_sources" || (state == "exploring" && confidence == 0) {
		return "not_applicable"
	}
	if state != "converged" && state != "abandoned" && state != "needs_sources" && state != "exploring" {
		return "lean_fail"
	}
	switch result {
	case "pass":
		if confidence >= 4 {
			return "pass"
		}
		return "lean_pass"
	case "fail":
		if hasEvidenceGap {
			return "lean_fail"
		}
		return "fail"
	default: // partial and any other normalised value
		if hasEvidenceGap {
			return "lean_fail"
		}
		if confidence >= 3 {
			return "mixed"
		}
		return "lean_fail"
	}
}

// DeriveCompleteness returns "conclusive" or "evidence_gap".
// Returns "evidence_gap" when: hasEvidenceGap is true, state is abandoned,
// state is not converged (truncated), or confidence ≤ 2 even when converged.
// Returns "conclusive" only when fully converged with confidence ≥ 3.
func DeriveCompleteness(state string, confidence int, hasEvidenceGap bool) string {
	if hasEvidenceGap {
		return "evidence_gap"
	}
	if state == "abandoned" {
		return "evidence_gap"
	}
	if state != "converged" {
		return "evidence_gap"
	}
	if confidence <= 2 {
		return "evidence_gap"
	}
	return "conclusive"
}

// DeriveCoverage returns a 0..1 coverage score rounded to 2 decimal places.
// When criteria are present, uses Σ(weight of met criteria), capped at 1.0.
// Falls back to a confidence-proxy when criteria are absent (pre-G2 runs):
//   - pass  → confidence/5
//   - fail  → 0
//   - other → confidence/5 × 0.6
func DeriveCoverage(confidence int, result string, criteria []Criterion) float64 {
	if len(criteria) > 0 {
		var total float64
		for _, c := range criteria {
			if c.Met {
				total += c.Weight
			}
		}
		if total > 1.0 {
			total = 1.0
		}
		return math.Round(total*100) / 100
	}
	if result == "pass" {
		return float64(confidence) / 5.0
	}
	if result == "fail" {
		return 0.0
	}
	return math.Round(float64(confidence)/5.0*0.6*100) / 100
}

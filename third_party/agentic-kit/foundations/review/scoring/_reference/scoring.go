// ANNOTATED COPY - foundations/review/scoring/scoring.go
// 232 lines, 50 branches. Pure deterministic scoring - the highest density file

// WHY IT'S COMPLEX
//
// Scoring is conceptually simple (numbers in, numbers out) but the complexity c
// the semantic rules it encodes. Each function here implements a specific decis
// the platform's investigation standard - which tier caps what confidence, whi
// follows from what combination of result + confidence + state + evidence-gap, h
// falls back to a coverage proxy. Rules that took months to learn that can't c
//
// This file is shared across materializer, coordinator, probes, and agents - it
// single canonical implementation of these rules. Getting any branch wrong crip
// systematically scoring errors across every consumer.

// WHAT IT DOES

// 1. NormalizeResult(string) → string
//    Maps LLM-produced uppercase variants ("PASS", "FAIL", "PARTIAL", "CONTRADI
//    to canonical lowercase. Anything unrecognized → "partial" (conservative de

// 2. NormalizeEvidenceTier(tier string) → string
//    Two-pass normalization: exact map lookup first (handles "code_verified",
//    "doc_claimed", "sme_stated" etc.), then substring fuzzy match.
//    Falls back to "assumed" for anything unrecognizable.
//    The five canonical tiers: verified > pre-scan > doc-claimed > sme-stated > assumed

// 3. ApplyCeiling(confidence int, tier string) → int
//    doc-claimed and sme-stated cap at 2. assumed caps at 1.
//    verified and pre-scan are uncapped (their ceilings are 5 and 4 via TierCeil
//    This enforces: you can't claim confidence 5 for something you only read in a doc.

// 4. TierRank / TierCeiling
//    TierRank: numeric ordering for comparison (verified=4, pre-scan=3, doc-claimed=2,
//    TierCeiling: max permitted confidence per tier (5, 4, 2, 2, 1).

// 5. DeriveConfidenceFromTier(tier, corroborated bool) → int
//    Corroboration = ≥2 DISTINCT file paths. Same file cited twice is not independent.
//    Corroborated → ceiling. Not corroborated → ceiling - 1 (min 1).

// 6. DeriveDisposition → 6-value taxonomy
//    The most branch-dense function. Inputs: result (pass/fail/partial/contradiction/
//    confidence (0-5), state (converged/abandoned/needs_sources/exploring/...), hasEvidenceGap.
//    Outputs: pass / lean_pass / mixed / lean_fail / fail / contradiction / not_applicable

//    Key rules:
//    - contradiction → always "contradiction"
//    - evidence_gap result → "lean_fail" (not fail - wrong/missing evidence ≠ confirmed fail)
//    - needs_sources or exploring+confidence=0 → "not_applicable"
//    - truncated (not converged, not abandoned, not terminal) → "lean_fail"
//    - pass + confidence≥4 → "pass"; pass + confidence<4 → "lean_pass"
//    - fail + evidence gap → "lean_fail"; fail without → "fail"
//    - partial + evidence gap → "lean_fail"; partial + confidence≥3 → "mixed"; else → "lean_fail"

// 7. DeriveCompleteness → "conclusive" or "evidence_gap"
//    evidence_gap when: hasEvidenceGap, abandoned, not converged (truncated), or
//    conclusive only when fully converged with confidence≥3.

// 8. DeriveCoverage → float64 0..1
//    Primary: weighted sum of met criteria (Σ weight where Met=true), capped at 1.0.
//    Fallback (pre-G2 runs with no criteria): confidence-proxy.
//      pass → confidence/5; fail → 0; partial → confidence/5 × 0.6.
//    Rounded to 2 decimal places.

// WHY THE DISPOSITION RULES ARE SUBTLE
//
// "evidence_gap" result is not the same as "hasEvidenceGap" flag. The result means
// the investigation returned an evidence-gap signal (mis-routed concern, wrong data).
// The flag means any evidence gap was detected during the investigation. Both push
// toward "lean_fail" not "fail" - the platform encodes that you shouldn't call
// something definitively failing if the evidence itself is questionable.
//
// The "truncated" check (state not in {converged, abandoned, needs_sources, exploring})
// handles investigations that were cut off by budget or deadline - they get "lean_fail"
// not "partial", because the incomplete investigation is explicitly inconclusive.
/*

 */

// Package scoring provides pure, deterministic scoring and evidence-tier hel
// shared across materializer, coordinator, probes, and agents packages.
package scoring

import (
	"math"
	"strings"
)

// Criterion is the minimal view scoring needs from a materialized criterion.
type Criterion struct {
	Met    bool
	Weight float64
}

// NormalizeResult maps the uppercase result strings to canonical lowercase c
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
// canonical values: verified, pre-scan, doc-claimed, sme-stated, assumed.
func NormalizeEvidenceTier(tier string) string {
	normalized := map[string]string{
		"code_verified":   "verified",
		"code_reviewed":   "verified",
		"code_seen":       "verified",
		"code":            "verified",
		"direct":          "verified",
		"observed":        "verified",
		"config_found":    "verified",
		"config_verified": "verified",
		"sme-verified":    "verified",
		"sme_verified":    "verified",
		"red":             "pre-scan",
		"doc_claimed":     "doc-claimed",
		"documentation":   "doc-claimed",
		"sme_stated":      "sme-stated",
	}

	if mapped, ok := normalized[strings.ToLower(tier)]; ok {
		return mapped
	}

	lower := strings.ToLower(tier)
	switch {
	case strings.Contains(lower, "verif") || strings.Contains(lower, "code") ||
		strings.Contains(lower, "direct") || strings.Contains(lower, "config"):
		return "verified"
	case strings.Contains(lower, "scan") || strings.Contains(lower, "infer"):
		return "pre-scan"
	case strings.Contains(lower, "doc") || strings.Contains(lower, "claim"):
		return "doc-claimed"
	case strings.Contains(lower, "sme") || strings.Contains(lower, "expert"):
		return "sme-stated"
	case strings.Contains(lower, "assum"):
		return "assumed"
	}

	validTiers := map[string]bool{
		"verified": true, "pre-scan": true, "doc-claimed": true,
		"sme-stated": true, "assumed": true,
	}
	if validTiers[tier] {
		return tier
	}
	return "assumed"
}

// ApplyCeiling enforces evidence-tier confidence caps.
// doc-claimed and sme-stated cap at 2; assumed caps at 1.
func ApplyCeiling(confidence int, tier string) int {
	switch strings.ToLower(tier) {
	case "doc-claimed", "sme-stated":
		return 2
	case "assumed":
		if confidence > 1 {
			return 1
		}
	}
	return confidence
}

// TierRank returns a numeric rank for evidence tier comparison (higher = bet
func TierRank(tier string) int {
	switch tier {
	case "verified":
		return 5
	case "pre-scan", "pre_scan":
		return 4
	case "doc-claimed", "doc_claimed":
		return 3
	case "sme-stated", "sme_stated":
		return 2
	default:
		return 0
	}
}

// TierCeiling returns the maximum confidence permitted for an evidence tier.
// verified → 5 / pre-scan → 4 / doc-claimed → 2 / sme-stated → 2 / assumed →
func TierCeiling(tier string) int {
	switch tier {
	case "verified":
		return 5
	case "pre-scan", "pre_scan":
		return 4
	case "doc-claimed", "doc_claimed", "sme-stated", "sme_stated":
		return 2
	default:
		return 1
	}
}

// DeriveConfidenceFromTier computes confidence from evidence tier × corroboration.
// Corroboration = ≥2 distinct file paths; same file cited twice is not independent.
func DeriveConfidenceFromTier(tier string, corroborated bool) int {
	ceiling := TierCeiling(tier)
	if corroborated {
		return ceiling
	}
	if ceiling > 1 {
		return ceiling - 1
	}
	return 1
}

// DeriveDisposition maps investigation state + confidence + result + evidence gap p
// to the 6-value disposition taxonomy.
func DeriveDisposition(result string, confidence int, state string, hasEvidenceGap bool) string {
	if result == "contradiction" {
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
	default: // partial
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
func DeriveCompleteness(state string, confidence int, hasEvidenceGap bool) string {
	if hasEvidenceGap {
		return "evidence_gap"
	}
	if state == "abandoned" {
		return "evidence_gap"
	}
	if state != "converged" && state != "abandoned" {
		return "evidence_gap"
	}
	if state == "converged" && confidence <= 2 {
		return "evidence_gap"
	}
	return "conclusive"
}

// DeriveCoverage returns a 0..1 coverage score.
// When criteria are present, uses Σ(weight of met criteria).
// Falls back to confidence-proxy when criteria are absent (pre-G2 runs).
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

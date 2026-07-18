// Package scoring provides pure, deterministic scoring and evidence-tier
// helpers for review pipelines: normalizing LLM-produced result and tier
// strings into a canonical vocabulary, ranking evidence tiers, capping
// confidence by tier, and deriving disposition / completeness / coverage
// from investigation outcomes.
//
// Composition: behavioral-only (no CUE schema). The package operates on plain
// values supplied by callers and carries no configuration or org-specific
// catalog of its own.
//
// Concurrency: every function is pure and stateless; the package is safe for
// unrestricted concurrent use.
package scoring

// Tier names an evidence tier. It is a type ALIAS for string (note the "=";
// it is not a defined type), so every existing string-typed call site keeps
// compiling unchanged — the alias exists to document intent at API
// boundaries and to anchor the exported Tier* constants.
//
// Canonical values, strongest to weakest:
//
//	TierVerified > TierPreScan > TierDocClaimed > TierSMEStated > TierAssumed
//
// Two DIFFERENT numeric scales are defined over this vocabulary — do not
// conflate them:
//
//   - TierRank: ordinal trust ordering used to compare two pieces of
//     evidence (which source wins). Ranks are positions, not confidence.
//   - TierCeiling: maximum confidence (0–5) a claim may carry given its
//     tier (how sure you are allowed to be).
//
// They diverge at doc-claimed: TierRank(TierDocClaimed) = 3 while
// TierCeiling(TierDocClaimed) = 2. See those functions for why.
//
// Use NormalizeEvidenceTier to fold arbitrary LLM-produced strings into this
// vocabulary before ranking or capping.
type Tier = string

// Canonical evidence tiers, strongest to weakest. These are the exact strings
// produced by NormalizeEvidenceTier and accepted by TierRank / TierCeiling /
// ApplyCeiling / DeriveConfidenceFromTier.
const (
	// TierVerified: directly observed — code read, config inspected,
	// behavior seen first-hand.
	TierVerified Tier = "verified"

	// TierPreScan: produced by an automated pre-scan or inference pass,
	// not yet directly verified.
	TierPreScan Tier = "pre-scan"

	// TierDocClaimed: asserted by documentation; not independently verified.
	TierDocClaimed Tier = "doc-claimed"

	// TierSMEStated: asserted by a subject-matter expert; not independently
	// verified.
	TierSMEStated Tier = "sme-stated"

	// TierAssumed: no supporting evidence. Also the fallback that
	// NormalizeEvidenceTier returns for anything unrecognizable.
	TierAssumed Tier = "assumed"
)

// Criterion is the minimal view scoring needs of an evaluated criterion:
// whether it was met and its weight in the coverage sum.
type Criterion struct {
	Met    bool
	Weight float64
}

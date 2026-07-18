// Package dlpevaluator evaluates text against configurable DLP (Data Loss
// Prevention) rules and produces a leakage-risk Report. Rules are data —
// callers supply a []DLPRule (from CUE or programmatically) rather than
// relying on hard-coded org-specific patterns. The evaluator is fully
// org-agnostic.
//
// Concurrency: Evaluator values are safe for concurrent use once constructed.
package dlpevaluator

// RiskBand classifies the leakage risk level of a finding or a full Report.
// Values form a strictly ordered set: None < Low < Medium < High.
type RiskBand string

const (
	// RiskNone means no leakage risk was detected.
	RiskNone RiskBand = "none"
	// RiskLow means a low leakage risk was detected.
	RiskLow RiskBand = "low"
	// RiskMedium means a medium leakage risk was detected.
	RiskMedium RiskBand = "medium"
	// RiskHigh means a high leakage risk was detected.
	RiskHigh RiskBand = "high"
)

// riskRank maps each RiskBand to a numeric rank for comparison.
var riskRank = map[RiskBand]int{
	RiskNone:   0,
	RiskLow:    1,
	RiskMedium: 2,
	RiskHigh:   3,
}

// RiskGTE returns true if a is greater than or equal to b.
func RiskGTE(a, b RiskBand) bool {
	return riskRank[a] >= riskRank[b]
}

// DLPRule is a single detection rule. Pattern must be a valid Go regexp.
// RiskBand, ID, and Description are loaded from data (e.g. CUE) — they are
// never hard-coded with org-specific literals inside this package.
type DLPRule struct {
	// ID is a unique, stable identifier for the rule (e.g. "DLP-001").
	ID string

	// Pattern is a Go regexp matched against the evaluated text.
	Pattern string

	// RiskBand is the leakage risk level assigned when this rule fires.
	RiskBand RiskBand

	// Description is a human-readable explanation of what this rule detects.
	Description string
}

// Finding is a single rule firing against a piece of text.
type Finding struct {
	// RuleID is the ID of the rule that fired.
	RuleID string

	// RiskBand is the risk level of the matched rule.
	RiskBand RiskBand

	// Excerpt is a short extract of the text that triggered the match
	// (at most 120 bytes, truncated with "…" if longer).
	Excerpt string
}

// Report is the outcome of an Evaluate call.
type Report struct {
	// Score is a normalised leakage-risk score in [0.0, 1.0].
	// 0.0 means no risk; 1.0 means the highest possible risk band fired.
	Score float64

	// Findings contains one entry per rule that fired.
	Findings []Finding

	// MaxRisk is the highest risk band among all findings,
	// or RiskNone if no rules fired.
	MaxRisk RiskBand
}

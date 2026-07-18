// Package injectiondetector provides prompt-injection and instruction-override
// detection for untrusted content. Rules and severities are data-driven —
// callers supply a []Rule (from CUE or programmatically) rather than relying
// on hard-coded org-specific patterns.
//
// Concurrency: Detector is safe for concurrent use after construction.
package injectiondetector

// Severity represents the threat level of a detection match.
// Values form a strictly ordered set: Low < Medium < High < Critical.
type Severity string

const (
	// SeverityLow is the lowest threat level.
	SeverityLow Severity = "low"
	// SeverityMedium is a moderate threat level.
	SeverityMedium Severity = "medium"
	// SeverityHigh is a serious threat level.
	SeverityHigh Severity = "high"
	// SeverityCritical is the highest threat level.
	SeverityCritical Severity = "critical"
)

// severityRank maps each Severity to a numeric rank for comparison.
// Higher numbers mean higher severity.
var severityRank = map[Severity]int{
	SeverityLow:      1,
	SeverityMedium:   2,
	SeverityHigh:     3,
	SeverityCritical: 4,
}

// SeverityGTE returns true if a is greater than or equal to b.
func SeverityGTE(a, b Severity) bool {
	return severityRank[a] >= severityRank[b]
}

// Rule is a single detection rule. Pattern must be a valid Go regexp.
// Severity, ID, and Description are loaded from data (e.g. CUE) — they are
// never hard-coded with org-specific literals inside this package.
type Rule struct {
	// ID is a unique, stable identifier for the rule (e.g. "INJ-001").
	ID string

	// Pattern is a Go regexp matched case-insensitively against the content.
	Pattern string

	// Severity is the threat level assigned when this rule fires.
	Severity Severity

	// Description is a human-readable explanation of the threat.
	Description string
}

// Match is a single rule firing against a piece of content.
type Match struct {
	// RuleID is the ID of the rule that fired.
	RuleID string

	// Severity is the severity of the matched rule.
	Severity Severity

	// Excerpt is a short extract of the content that triggered the match
	// (at most 120 bytes, truncated with "…" if longer).
	Excerpt string
}

// Result is the outcome of a Scan call.
type Result struct {
	// Flagged is true when at least one rule matched.
	Flagged bool

	// Matches contains one entry per rule that fired. Multiple rules may fire
	// on the same content.
	Matches []Match

	// MaxSeverity is the highest severity among all matches, or "" if Flagged
	// is false.
	MaxSeverity Severity
}

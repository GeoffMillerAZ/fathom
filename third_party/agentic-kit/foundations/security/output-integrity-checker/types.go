// Package outputintegritychecker verifies that an LLM output satisfies a
// configurable integrity policy: required disclaimers are present and banned
// tokens are absent. Policy is fully data-driven — no org-specific literals
// are hard-coded in this package.
//
// Concurrency: Checker is safe for concurrent use after construction.
package outputintegritychecker

// IntegrityPolicy defines what an output must and must not contain.
type IntegrityPolicy struct {
	// Name identifies the policy (e.g. "generic-v1").
	Name string

	// Version is a semver-style string (e.g. "1.0.0").
	Version string

	// RequiredDisclaimers is the list of substrings that must appear in the
	// output. Each is a plain string (case-insensitive containment check).
	RequiredDisclaimers []RequiredDisclaimer

	// BannedTokens is the list of patterns that must NOT appear in the output.
	// Each Pattern is a Go regexp.
	BannedTokens []BannedToken
}

// RequiredDisclaimer defines a substring that must be present in the output.
type RequiredDisclaimer struct {
	// ID is a unique, stable identifier (e.g. "RD-001").
	ID string

	// Substring is the required text. Checked case-insensitively.
	Substring string

	// Description explains why this disclaimer is required.
	Description string
}

// BannedToken defines a pattern that must not appear in the output.
type BannedToken struct {
	// ID is a unique, stable identifier (e.g. "BT-001").
	ID string

	// Pattern is a Go regexp matched against the output.
	Pattern string

	// Description explains why this token is banned.
	Description string
}

// Violation records a single policy violation found in the output.
type Violation struct {
	// RuleID is the ID of the RequiredDisclaimer or BannedToken that fired.
	RuleID string

	// Kind is either "missing_disclaimer" or "banned_token".
	Kind string

	// Detail is a human-readable explanation of the violation.
	Detail string

	// Excerpt is a short extract from the output relevant to the violation
	// (for banned tokens: the matched text; for missing disclaimers: empty).
	Excerpt string
}

// Report is the result of Check.
type Report struct {
	// Passed is true when no violations were found.
	Passed bool

	// Violations contains one entry per policy rule that was violated.
	Violations []Violation
}

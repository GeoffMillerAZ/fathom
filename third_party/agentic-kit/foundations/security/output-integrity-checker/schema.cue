// Package outputintegritychecker defines CUE contracts for configurable output
// integrity policies. Required disclaimers and banned tokens are pure data —
// no org-specific literals are defined in this schema.
package outputintegritychecker

// #RequiredDisclaimer defines a substring that must appear in an output.
#RequiredDisclaimer: {
	// ID is a unique, stable identifier (e.g. "RD-001").
	ID: string & !=""

	// Substring is the required text, checked case-insensitively.
	Substring: string & !=""

	// Description explains why this disclaimer is required.
	Description: string & !=""
}

// #BannedToken defines a pattern that must not appear in an output.
#BannedToken: {
	// ID is a unique, stable identifier (e.g. "BT-001").
	ID: string & !=""

	// Pattern is a Go regexp matched against the output.
	Pattern: string & !=""

	// Description explains why this token is banned.
	Description: string & !=""
}

// #IntegrityPolicy defines what an output must and must not contain.
#IntegrityPolicy: {
	// Name identifies the policy (e.g. "generic-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// RequiredDisclaimers is the list of substrings that must appear.
	RequiredDisclaimers: [...#RequiredDisclaimer]

	// BannedTokens is the list of patterns that must not appear.
	BannedTokens: [...#BannedToken]
}

// #Violation records a single policy violation.
#Violation: {
	// RuleID is the ID of the rule that was violated.
	RuleID: string & !=""

	// Kind is either "missing_disclaimer" or "banned_token".
	Kind: "missing_disclaimer" | "banned_token"

	// Detail is a human-readable explanation.
	Detail: string & !=""

	// Excerpt is a short extract relevant to the violation (may be empty for
	// missing-disclaimer violations).
	Excerpt: string
}

// #Report is the result of an integrity check.
#Report: {
	// Passed is true when no violations were found.
	Passed: bool

	// Violations contains one entry per violated rule.
	Violations: [...#Violation]
}

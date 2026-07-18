// Package redactionexecutor defines CUE contracts for the redaction policy
// and its constituent rules. Rules and replacements are data — they are
// supplied by callers (from CUE, configuration, or programmatically) rather
// than hard-coded inside the package. This makes the executor org-agnostic.
package redactionexecutor

// #RedactionRule describes a single redaction rule.
// Pattern must be a valid Go regexp. Replacement is the literal string that
// is substituted for every match.
#RedactionRule: {
	// ID is a unique, stable identifier for the rule (e.g. "REDACT-PII-001").
	ID: string & !=""

	// Pattern is a Go regexp expression matched against the input text.
	Pattern: string & !=""

	// Replacement is the string substituted for every match (e.g. "[REDACTED]").
	// An empty replacement is allowed (it removes matched text).
	Replacement: string
}

// #RedactionPolicy is a named, versioned collection of redaction rules.
// Applying a policy redacts all matches for every rule in the policy.
#RedactionPolicy: {
	// Name identifies the policy (e.g. "pii-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// Rules is the list of redaction rules. At least one rule is required.
	Rules: [#RedactionRule, ...#RedactionRule]
}

// Package prellmdlpgate defines CUE contracts for pre-LLM DLP redaction
// policies. All patterns and labels are configurable data — no org-specific
// literals are defined in this schema.
//
// Policies validated by this schema describe best-effort secret-SHAPE
// redaction: rules only catch text matching their patterns. Encoded, split,
// or novel-format secrets pass through — a policy is a seatbelt, not a
// guarantee.
package prellmdlpgate

// #RedactRule is a single configurable DLP redaction rule.
#RedactRule: {
	// ID is a unique, stable identifier for the rule (e.g. "DLP-001").
	ID: string & !=""

	// Pattern is a Go regexp expression matched against the text.
	Pattern: string & !=""

	// Label is the replacement token inserted where redacted text appeared.
	Label: string & !=""

	// Description is a human-readable explanation of what this rule redacts.
	Description: string & !=""
}

// #RedactPolicy is a named, versioned collection of RedactRules.
#RedactPolicy: {
	// Name identifies the policy (e.g. "generic-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// Rules is the list of redaction rules. At least one rule is required.
	Rules: [#RedactRule, ...#RedactRule]
}

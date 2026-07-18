// Package prellmdlpgate scans and redacts secret-SHAPED content from text
// segments before they are sent to an LLM.
//
// Honesty contract: this is best-effort, regexp-based redaction of strings
// that look like credentials — it is NOT DLP-as-guarantee. Base64/hex-encoded
// secrets, secrets split across multiple segments, novel token formats, and
// anything without a recognizable shape pass straight through. Treat the gate
// as a seatbelt that reduces accidental leakage, never as authorization to
// send untrusted or sensitive text to an external model.
//
// Composition: RedactRule/RedactPolicy mirror #RedactRule/#RedactPolicy in
// schema.cue; all patterns and labels are configurable data — no org-specific
// literals are hard-coded in this package. DefaultPolicy and ExtendedPolicy
// ship generic catalogs; MustDefaultGate is the infallible constructor for
// the shipped default.
//
// Concurrency: a Gate is immutable after NewGate and safe for unrestricted
// concurrent use.
package prellmdlpgate

// RedactRule is a single DLP redaction rule. Pattern must be a valid Go regexp.
type RedactRule struct {
	// ID is a unique, stable identifier for the rule (e.g. "DLP-001").
	ID string

	// Pattern is a Go regexp matched against the text. The ENTIRE match is
	// replaced with Label; capture groups get no special treatment.
	Pattern string

	// Label is the replacement token inserted in place of the redacted text
	// (e.g. "[REDACTED:CARD]"). Must not be empty. Avoid "$" in labels: the
	// replacement is applied with Regexp.ReplaceAllString, which expands
	// $-references.
	Label string

	// Description is a human-readable explanation of what this rule redacts.
	Description string
}

// RedactPolicy is a named, versioned collection of RedactRules.
type RedactPolicy struct {
	// Name identifies the policy (e.g. "generic-v1").
	Name string

	// Version is a semver-style string (e.g. "1.0.0").
	Version string

	// Rules is the ordered list of redaction rules.
	Rules []RedactRule
}

// Finding records one match of a RedactRule against the input text.
type Finding struct {
	// RuleID is the ID of the rule that matched.
	RuleID string

	// Label is the redaction label that replaced the match.
	Label string

	// Excerpt is a short extract of the original text that was redacted
	// (at most 120 bytes, truncated with "…" if longer).
	Excerpt string
}

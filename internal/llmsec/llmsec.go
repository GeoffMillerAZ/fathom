// Package llmsec provides LLM security helpers for sextant: DLP-based text
// redaction to strip credentials and other sensitive content from symbol data
// before it is forwarded to any language model.
//
// The implementation is a thin wrapper over agentic-kit's pre-llm-dlp-gate
// package so the policy and regex logic stay in one canonical place.
package llmsec

import (
	prellmdlpgate "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

// Posture controls how the Redactor behaves.
type Posture struct {
	// Passthrough disables all redaction when true. Use only in development
	// environments where no sensitive data can reach the pipeline.
	Passthrough bool
}

// DefaultPosture returns the recommended production posture: DLP enabled,
// using the generic default policy from agentic-kit.
func DefaultPosture() Posture {
	return Posture{Passthrough: false}
}

// Redactor scrubs sensitive content from text using a compiled DLP gate.
// Create one with NewRedactor; the zero value is not usable.
type Redactor struct {
	posture Posture
	gate    *prellmdlpgate.Gate // nil when posture.Passthrough == true
}

// NewRedactor constructs a Redactor for the given Posture.
// When posture.Passthrough is false the agentic-kit default DLP policy is
// compiled; any pattern compilation failure panics because it indicates a
// programmer error in the shipped policy, not a runtime input error.
func NewRedactor(posture Posture) *Redactor {
	r := &Redactor{posture: posture}
	if posture.Passthrough {
		return r
	}
	g, err := prellmdlpgate.NewGate(prellmdlpgate.DefaultPolicy())
	if err != nil {
		// DefaultPolicy is shipped with the binary and must compile cleanly.
		panic("llmsec: failed to compile default DLP policy: " + err.Error())
	}
	r.gate = g
	return r
}

// Redact scans text for sensitive patterns and replaces them with labelled
// placeholders. It returns the (possibly modified) text and a boolean that is
// true when at least one substitution was made.
//
// When the Posture is Passthrough the original text is returned unchanged and
// changed is always false.
func (r *Redactor) Redact(text string) (out string, changed bool) {
	if r.posture.Passthrough || r.gate == nil {
		return text, false
	}
	redacted, findings := r.gate.Scan(text)
	return redacted, len(findings) > 0
}

// Package llmsec provides LLM security helpers for fathom: DLP-based text
// redaction to strip credentials and other sensitive content from symbol data
// before it is forwarded to any language model.
//
// The implementation is a thin wrapper over agentic-kit's pre-llm-dlp-gate
// package so the policy and regex logic stay in one canonical place.
package llmsec

import (
	"strings"

	prellmdlpgate "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

// Posture controls how the Redactor behaves.
type Posture struct {
	// Passthrough disables all redaction when true. Use only in development
	// environments where no sensitive data can reach the pipeline.
	Passthrough bool
	// Extended compiles the kit's ExtendedPolicy — the generic policy plus
	// high-precision credential shapes (DLP-101..106: PEM private keys, AWS
	// access key IDs, GitHub, Slack and Google tokens, JWTs) — instead of
	// the generic one. For callers that send arbitrary source text off-host.
	Extended bool
}

// DefaultPosture returns the recommended production posture: DLP enabled,
// using the generic default policy from agentic-kit.
func DefaultPosture() Posture {
	return Posture{Passthrough: false}
}

// ExtendedPosture is DefaultPosture with the kit's extended credential
// catalog: the posture for a lane whose state is raw source (commit
// patches, working-tree diffs), where a committed key is the likely leak.
func ExtendedPosture() Posture {
	return Posture{Extended: true}
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
	policy := prellmdlpgate.DefaultPolicy()
	if posture.Extended {
		policy = prellmdlpgate.ExtendedPolicy()
	}
	g, err := prellmdlpgate.NewGate(policy)
	if err != nil {
		// Both policies are shipped with the binary and must compile cleanly.
		panic("llmsec: failed to compile DLP policy " + policy.Name + ": " + err.Error())
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

// RedactCount is Redact plus how many spans the pass replaced: for each
// rule that fired, the occurrences of its label the pass added (a label
// already present in the input is not counted again), and never fewer
// than the number of rules that fired.
func (r *Redactor) RedactCount(text string) (out string, n int) {
	if r.posture.Passthrough || r.gate == nil {
		return text, 0
	}
	redacted, findings := r.gate.Scan(text)
	for _, f := range findings {
		if d := strings.Count(redacted, f.Label) - strings.Count(text, f.Label); d > 0 {
			n += d
		}
	}
	if n < len(findings) {
		n = len(findings)
	}
	return redacted, n
}

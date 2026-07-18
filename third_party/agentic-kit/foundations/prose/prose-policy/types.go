// Package prosepolicy defines DLP posture levels and the data-driven policy
// evaluation engine for prose content governance.
//
// The posture levels are configurable and org-agnostic: no hard-coded
// organisation rules appear here. Callers configure a Policy with a desired
// Posture and a set of PostureRules that describe what terms trigger redaction
// at each level. Evaluate then applies those rules to a prose segment,
// returning a Decision that records whether the segment is allowed as-is and
// which terms (if any) must be redacted.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Boundary vs foundations/security: prosepolicy is configurable PROSE posture
// EVALUATION — Evaluate returns a Decision (allowed or not, plus the terms
// that must be redacted) but never rewrites text. Pre-LLM redaction EXECUTION
// lives in foundations/security: security/pre-llm-dlp-gate scans segments and
// redacts secret-shaped content before they reach an LLM, and
// security/redaction-executor applies a compiled RedactionPolicy's regexp
// rules to text. Use prosepolicy to decide the DLP posture of prose
// artifacts; use the security packages to actually strip sensitive content
// on the pre-LLM path.
//
// Concurrency: values are immutable and safe for concurrent use.
package prosepolicy

import (
	"fmt"
	"strings"
)

// Posture is the DLP posture level for a policy. Each level represents a
// progressively more restrictive stance on what content may appear in prose.
//
// CUE mirror: #Posture in schema.cue.
type Posture string

const (
	// PostureOpen permits all content — no automated redaction is applied.
	PostureOpen Posture = "open"

	// PostureInternal permits internal content but may redact terms that
	// should not leave the organisation.
	PostureInternal Posture = "internal"

	// PostureRestricted applies tighter controls — terms that should not
	// appear in restricted-audience documents are redacted.
	PostureRestricted Posture = "restricted"

	// PostureSecret applies the strictest controls — only the minimum
	// necessary content may appear; all configured sensitive terms are
	// redacted.
	PostureSecret Posture = "secret"
)

// validPostures is the authoritative set of valid Posture values.
var validPostures = map[Posture]struct{}{
	PostureOpen:       {},
	PostureInternal:   {},
	PostureRestricted: {},
	PostureSecret:     {},
}

// PostureRule describes a set of terms that are prohibited at or above a given
// posture threshold. When the active Policy.Posture is >= the rule's
// Threshold, any term in Terms found in a segment's text is redacted.
//
// CUE mirror: #PostureRule in schema.cue.
type PostureRule struct {
	// Threshold is the minimum posture level at which this rule activates.
	// Must be a valid Posture value.
	Threshold Posture

	// Terms is the list of literal substrings that must be redacted when this
	// rule is active. Each term must be non-empty.
	Terms []string
}

// Policy holds the active DLP posture and the rules that govern evaluation.
// It is constructed by callers with domain-specific rules — no org-specific
// defaults are baked in.
//
// CUE mirror: #Policy in schema.cue.
type Policy struct {
	// Posture is the active DLP posture level. Must be a valid Posture value.
	Posture Posture

	// Rules is the set of posture rules consulted during evaluation.
	// May be empty (no automated redaction beyond posture-level allow/deny).
	Rules []PostureRule
}

// Validate returns an error if the policy or any of its rules is invalid.
func (p Policy) Validate() error {
	if _, ok := validPostures[p.Posture]; !ok {
		return fmt.Errorf("prosepolicy: unknown Posture %q", p.Posture)
	}
	for i, r := range p.Rules {
		if _, ok := validPostures[r.Threshold]; !ok {
			return fmt.Errorf("prosepolicy: Rules[%d].Threshold: unknown Posture %q", i, r.Threshold)
		}
		for j, term := range r.Terms {
			if term == "" {
				return fmt.Errorf("prosepolicy: Rules[%d].Terms[%d] must not be empty", i, j)
			}
		}
	}
	return nil
}

// Decision is the result of evaluating a prose segment against a Policy.
//
// CUE mirror: #Decision in schema.cue.
type Decision struct {
	// Allowed is true when the segment may be published under the active
	// posture without requiring manual review. It is false when one or more
	// redactions were triggered.
	Allowed bool

	// Redactions is the list of terms that were found in the segment text and
	// must be removed or replaced before publication.
	Redactions []string
}

// postureOrder maps Posture values to a comparable ordinal so that ">=" logic
// can be expressed without string comparisons.
var postureOrder = map[Posture]int{
	PostureOpen:       0,
	PostureInternal:   1,
	PostureRestricted: 2,
	PostureSecret:     3,
}

// Evaluate inspects segmentText against the receiver Policy and returns a
// Decision. Rules whose Threshold is less than or equal to the active Posture
// are applied; any matching Terms are collected as Redactions. The Decision is
// Allowed when Redactions is empty.
//
// Evaluate does not mutate segmentText — redaction (replacement) is left to
// the caller, informed by the returned Redactions list.
func (p Policy) Evaluate(segmentText string) Decision {
	activeLevel := postureOrder[p.Posture]

	var redactions []string
	seen := make(map[string]struct{})

	for _, rule := range p.Rules {
		ruleLevel := postureOrder[rule.Threshold]
		if activeLevel < ruleLevel {
			// This rule only activates at a stricter posture than the active one.
			continue
		}
		for _, term := range rule.Terms {
			if _, already := seen[term]; already {
				continue
			}
			if strings.Contains(segmentText, term) {
				redactions = append(redactions, term)
				seen[term] = struct{}{}
			}
		}
	}

	return Decision{
		Allowed:    len(redactions) == 0,
		Redactions: redactions,
	}
}

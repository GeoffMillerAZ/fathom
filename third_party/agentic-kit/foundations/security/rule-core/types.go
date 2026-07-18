// Package rulecore is the shared regex-rule engine extracted from the security
// and quality foundations. It provides:
//
//   - Pattern compilation with a uniform error format.
//   - First-match scanning (one Finding per rule, in rule order).
//   - Literal-safe fixpoint replacement ($ never expanded; capped at maxPasses).
//   - UTF-8-safe excerpt extraction (capped at ExcerptMaxBytes).
//   - Ordered-string comparison without forcing a shared type on callers.
//
// The four engines that delegate here (injection-detector, redaction-executor,
// pre-llm-dlp-gate, dlp-evaluator) keep their own exported types and APIs
// unchanged; rulecore is an internal implementation detail of each.
//
// Concurrency: CompiledSet is immutable after Compile and safe for concurrent
// use by multiple goroutines.
package rulecore

import "regexp"

// ExcerptMaxBytes is the maximum byte length of an excerpt returned by
// ScanFirst. Excerpts that exceed this length are truncated to a valid UTF-8
// boundary and suffixed with "…".
const ExcerptMaxBytes = 120

// PatternSpec is the minimum a rule-core caller must provide to Compile. The
// ID is used in error messages and in the Findings returned by ScanFirst.
type PatternSpec struct {
	// ID is a unique, stable identifier for the rule (e.g. "INJ-001").
	ID string

	// Pattern is a Go regexp. Any valid Go regexp is accepted.
	Pattern string
}

// Finding is a single rule match returned by ScanFirst.
type Finding struct {
	// RuleID is the PatternSpec.ID of the rule that matched.
	RuleID string

	// Start is the byte offset of the match start in the scanned text.
	Start int

	// End is the byte offset of the match end in the scanned text.
	End int

	// Excerpt is a UTF-8-safe extract of the matched region, capped at
	// ExcerptMaxBytes bytes. Truncated matches are suffixed with "…".
	Excerpt string
}

// CompiledSet is an immutable, concurrency-safe set of compiled regexp rules.
// Create one with Compile; the zero value is not usable.
type CompiledSet struct {
	ids  []string         // parallel to regs
	regs []*regexp.Regexp // compiled patterns in spec order
}

// Ordinal supports ordered-string comparison using an externally supplied
// ranking. It is used by calling packages to compare typed constants (e.g.
// Severity, RiskBand, SensitivityLevel) without duplicating the rank map
// logic inside rulecore or forcing a shared enum type.
//
// An Ordinal is safe for concurrent use; it holds only an immutable map.
type Ordinal struct {
	rank map[string]int
}

// NewOrdinal returns an Ordinal whose ordering is defined by levelsLowToHigh.
// The first element is the lowest rank (0); each subsequent element has a
// higher rank. Duplicate entries take the last-seen position.
func NewOrdinal(levelsLowToHigh []string) Ordinal {
	m := make(map[string]int, len(levelsLowToHigh))
	for i, s := range levelsLowToHigh {
		m[s] = i
	}
	return Ordinal{rank: m}
}

// GTE returns true if a is greater than or equal to b in the ordinal's
// ordering. Unknown strings map to rank 0 (the minimum).
func (o Ordinal) GTE(a, b string) bool {
	return o.rank[a] >= o.rank[b]
}

// Max returns the highest-ranked element among levels. Returns "" if levels is
// empty. Unknown strings map to rank 0.
func (o Ordinal) Max(levels []string) string {
	if len(levels) == 0 {
		return ""
	}
	best := levels[0]
	for _, s := range levels[1:] {
		if o.rank[s] > o.rank[best] {
			best = s
		}
	}
	return best
}

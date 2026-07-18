package rulecore

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Compile compiles the patterns from specs and returns an immutable CompiledSet.
// Returns an error on the first invalid pattern, with message format:
//
//	rulecore: rule "ID" has invalid pattern "pat": <underlying error>
func Compile(specs []PatternSpec) (*CompiledSet, error) {
	ids := make([]string, len(specs))
	regs := make([]*regexp.Regexp, len(specs))
	for i, s := range specs {
		re, err := regexp.Compile(s.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rulecore: rule %q has invalid pattern %q: %w", s.ID, s.Pattern, err)
		}
		ids[i] = s.ID
		regs[i] = re
	}
	return &CompiledSet{ids: ids, regs: regs}, nil
}

// ScanFirst returns one Finding per compiled pattern that matches text, in
// rule order. At most one Finding per rule is returned (the first match). Rules
// that do not match produce no Finding.
//
// ScanFirst is safe to call concurrently on the same CompiledSet.
func (s *CompiledSet) ScanFirst(text string) []Finding {
	var out []Finding
	for i, re := range s.regs {
		loc := re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		out = append(out, Finding{
			RuleID:  s.ids[i],
			Start:   loc[0],
			End:     loc[1],
			Excerpt: extractExcerpt(text, loc[0], loc[1]),
		})
	}
	return out
}

// ReplaceFixpoint applies literal replacements to text, iterating until the
// output stabilises (a textual fixpoint) or maxPasses sweeps have been
// executed. Returns (result, converged):
//
//   - converged == true if the fixpoint was reached within maxPasses.
//   - converged == false if the cap was hit; result is the last sweep's output.
//
// The replacements map keys rule IDs to their replacement strings. Patterns
// whose ID does not appear in replacements are ignored on each sweep.
// ReplaceAllLiteralString semantics are used: "$" in replacement strings is
// never expanded as a backreference.
//
// ReplaceFixpoint is safe to call concurrently on the same CompiledSet.
func (s *CompiledSet) ReplaceFixpoint(text string, replacements map[string]string, maxPasses int) (string, bool) {
	result := text
	for range maxPasses {
		next := result
		for i, re := range s.regs {
			repl, ok := replacements[s.ids[i]]
			if !ok {
				continue
			}
			next = re.ReplaceAllLiteralString(next, repl)
		}
		if next == result {
			return result, true
		}
		result = next
	}
	return result, false
}

// extractExcerpt returns a UTF-8-safe substring of s[start:end], capped at
// ExcerptMaxBytes bytes, truncated with "…" when shortened.
func extractExcerpt(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	excerpt := s[start:end]
	if len(excerpt) <= ExcerptMaxBytes {
		return excerpt
	}
	truncated := excerpt[:ExcerptMaxBytes]
	for !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "…"
}

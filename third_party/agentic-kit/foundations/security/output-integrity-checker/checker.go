package outputintegritychecker

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const excerptMaxBytes = 120

// Checker verifies output against a compiled IntegrityPolicy.
// Create one with NewChecker; the zero value is not usable.
type Checker struct {
	policy         IntegrityPolicy
	compiledBanned []*regexp.Regexp
}

// NewChecker compiles each BannedToken's Pattern and returns a ready Checker.
// Returns an error if any pattern fails to compile.
func NewChecker(policy IntegrityPolicy) (*Checker, error) {
	compiled := make([]*regexp.Regexp, len(policy.BannedTokens))
	for i, bt := range policy.BannedTokens {
		re, err := regexp.Compile(bt.Pattern)
		if err != nil {
			return nil, fmt.Errorf("outputintegritychecker: banned token %q has invalid pattern %q: %w", bt.ID, bt.Pattern, err)
		}
		compiled[i] = re
	}
	return &Checker{policy: policy, compiledBanned: compiled}, nil
}

// Check verifies that output satisfies the policy. It returns a Report
// describing whether the output passed or which rules were violated.
// Results are deterministic for a given (output, policy) pair.
func (c *Checker) Check(output string) Report {
	var violations []Violation
	lower := strings.ToLower(output)

	// Check required disclaimers (case-insensitive containment).
	for _, rd := range c.policy.RequiredDisclaimers {
		if !strings.Contains(lower, strings.ToLower(rd.Substring)) {
			violations = append(violations, Violation{
				RuleID:  rd.ID,
				Kind:    "missing_disclaimer",
				Detail:  fmt.Sprintf("required disclaimer %q not found: %s", rd.Substring, rd.Description),
				Excerpt: "",
			})
		}
	}

	// Check banned tokens.
	for i, re := range c.compiledBanned {
		bt := c.policy.BannedTokens[i]
		loc := re.FindStringIndex(output)
		if loc != nil {
			excerpt := extractExcerpt(output, loc[0], loc[1])
			violations = append(violations, Violation{
				RuleID:  bt.ID,
				Kind:    "banned_token",
				Detail:  fmt.Sprintf("banned token matched by rule %q: %s", bt.ID, bt.Description),
				Excerpt: excerpt,
			})
		}
	}

	return Report{
		Passed:     len(violations) == 0,
		Violations: violations,
	}
}

// Check is a package-level convenience function. It builds a temporary Checker
// from the provided policy and checks the output.
// Returns an error if any banned token pattern fails to compile.
func Check(output string, policy IntegrityPolicy) (Report, error) {
	c, err := NewChecker(policy)
	if err != nil {
		return Report{}, err
	}
	return c.Check(output), nil
}

// extractExcerpt returns a UTF-8-safe excerpt of s from start to end,
// capped at excerptMaxBytes, truncated with "…" when shortened.
func extractExcerpt(s string, start, end int) string {
	if end > len(s) {
		end = len(s)
	}
	if start < 0 {
		start = 0
	}
	excerpt := s[start:end]
	if len(excerpt) <= excerptMaxBytes {
		return excerpt
	}
	truncated := excerpt[:excerptMaxBytes]
	for !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "…"
}

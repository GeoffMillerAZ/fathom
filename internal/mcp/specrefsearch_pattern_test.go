package mcp

import (
	"regexp"
	"testing"
)

// TestSpecRefSearchPattern pins SR-27's per-family regex reconstruction:
// each of sdd.DefaultSpecRefPattern's three citation shapes gets a
// pattern that matches ITS OWN shape and rejects an incidental substring
// (a longer/related ID must not false-positive).
func TestSpecRefSearchPattern(t *testing.T) {
	cases := []struct {
		id      string
		match   string
		noMatch string
		caseIns bool
	}{
		{id: "US-042", match: "see [US-042] here", noMatch: "see [US-0421] here", caseIns: false},
		{id: "M18.2", match: "ships in M18.2 today", noMatch: "ships in M18.20 today", caseIns: true},
		{id: "doc 31", match: "see doc   31 for detail", noMatch: "see doc 310 for detail", caseIns: false},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			pattern, caseSensitive := specRefSearchPattern(c.id)
			if caseSensitive == c.caseIns {
				t.Errorf("specRefSearchPattern(%q) caseSensitive = %v, want %v", c.id, caseSensitive, !c.caseIns)
			}
			reSrc := pattern
			if !caseSensitive {
				reSrc = "(?i)" + reSrc
			}
			re, err := regexp.Compile(reSrc)
			if err != nil {
				t.Fatalf("specRefSearchPattern(%q) = %q, does not compile: %v", c.id, pattern, err)
			}
			if !re.MatchString(c.match) {
				t.Errorf("pattern %q must match %q", pattern, c.match)
			}
			if re.MatchString(c.noMatch) {
				t.Errorf("pattern %q must not match %q (would false-positive on a longer ID)", pattern, c.noMatch)
			}
		})
	}

	// Milestone case-insensitivity: the extractor accepts "m18.2" too.
	pattern, caseSensitive := specRefSearchPattern("M18.2")
	reSrc := pattern
	if !caseSensitive {
		reSrc = "(?i)" + reSrc
	}
	re := regexp.MustCompile(reSrc)
	if !re.MatchString("see m18.2 lowercase") {
		t.Errorf("milestone pattern %q must match the lowercase form too", pattern)
	}
}

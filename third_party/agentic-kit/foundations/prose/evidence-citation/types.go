// Package evidencecitation renders and parses citation strings for
// evidence references — the full round trip every grounded-answer consumer
// needs. Every claim in governed prose is citation-linked, not merely
// vaguely evidence-backed; this package provides the canonical rendering
// (Cite) and its inverse (Parser.Parse), plus the validation hook
// (Validate) that rejects fabricated citations and rewrite helpers
// (Strip/Replace) for display text.
//
// The Cite function sorts refs before rendering so the output is stable
// regardless of the order the caller provides them — required for
// deterministic document hashing and reproducible audit trails. Parse
// recognizes both Cite's "[ref1; ref2]" form and the "[tag:id]" bracket-tag
// form models are typically prompted to emit; the tag vocabulary (e.g.
// "span", "authority") is caller data, never a kit constant.
//
// Honest non-guarantees: parsing identifies WELL-FORMED tags only — it
// cannot detect citations the model omitted or mangled, and it cannot tell
// a real ID from an invented one (that is Validate's job, against a
// caller-supplied resolver). Strip/Replace rewrite only blocks the parser
// recognizes.
//
// Concurrency: everything here is pure — no clock, no I/O, no shared
// mutable state. All functions and Parser methods are safe for concurrent
// use.
//
// CUE contracts live in schema.cue; the Go types must stay in sync with
// that schema.
package evidencecitation

import (
	"fmt"
	"sort"
	"strings"
)

// Cite renders a deterministic citation string from the provided evidence
// reference identifiers. The output format is "[ref1; ref2; ...]" with refs
// sorted lexicographically.
//
// Rules:
//   - Returns an empty string when refs is nil or empty.
//   - Duplicate refs are deduplicated before rendering.
//   - Empty string elements in refs are ignored.
//
// Examples:
//
//	Cite(nil)                          → ""
//	Cite([]string{})                   → ""
//	Cite([]string{"span-b", "span-a"}) → "[span-a; span-b]"
//	Cite([]string{"span-a", "span-a"}) → "[span-a]"
func Cite(refs []string) string {
	if len(refs) == 0 {
		return ""
	}

	// Deduplicate and filter empty strings.
	seen := make(map[string]struct{}, len(refs))
	unique := refs[:0:0] // new slice backed by fresh memory
	for _, r := range refs {
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		unique = append(unique, r)
	}

	if len(unique) == 0 {
		return ""
	}

	sort.Strings(unique)
	return fmt.Sprintf("[%s]", strings.Join(unique, "; "))
}

package view

import "github.com/geoffmilleraz/fathom/pkg/lang"

// This file holds the containment rule of the package, in one place, so every
// helper that nests symbols (Enclosing and Touched)
// applies the same one.
//
// A symbol has a known span when EndLine >= Line > 0. The span is the lines
// Line to EndLine inclusive. A symbol without one (EndLine 0, or below Line, or
// Line not positive) has an unknown span and never contains anything.

// known reports whether s has a known span.
func known(s lang.Symbol) bool { return s.Line > 0 && s.EndLine >= s.Line }

// before is the order of listings: Line, then EndLine descending (the
// wider span first), then Key. Under it a container sorts before what it holds.
func before(a, b lang.Symbol) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.EndLine != b.EndLine {
		return a.EndLine > b.EndLine
	}
	return a.Key < b.Key
}

// covers reports whether the known span of s holds every line of start..end.
func covers(s lang.Symbol, start, end int) bool {
	return known(s) && s.Line <= start && end <= s.EndLine
}

// moreInner reports whether a is a more inner container than b: the latest
// Line, then the earliest EndLine, then the largest Key. It is the reverse of
// before, so the innermost symbol is always the last in the one listing order
// and, among equal spans, the first of them is the outer one (ESCALATE-27 R4).
func moreInner(a, b lang.Symbol) bool {
	if a.Line != b.Line {
		return a.Line > b.Line
	}
	if a.EndLine != b.EndLine {
		return a.EndLine < b.EndLine
	}
	return a.Key > b.Key
}

package view

import (
	"cmp"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// This file holds the containment rule of the package, in one place, so every
// helper that nests symbols (Enclosing, Touched and Outline) applies the same one.
//
// A symbol has a known span when EndLine >= Line > 0. The span is the lines
// Line to EndLine inclusive. A symbol without one (EndLine 0, or below Line, or
// Line not positive) has an unknown span and never contains anything.

// known reports whether s has a known span.
func known(s lang.Symbol) bool { return s.Line > 0 && s.EndLine >= s.Line }

// before is the order of listings: Line, then EndLine descending (the
// wider span first), then Key. Keys are not unique in real reads (one-line
// TypeScript overloads share Key and span), so the order is made total by the
// remaining fields: Kind, Canonical, Name, Signature, Package, Tier. Under it a
// container sorts before what it holds, and no input order changes a listing.
func before(a, b lang.Symbol) bool { return order(a, b) < 0 }

// order is the three-way form of before.
func order(a, b lang.Symbol) int {
	return cmp.Or(
		cmp.Compare(a.Line, b.Line),
		cmp.Compare(b.EndLine, a.EndLine),
		cmp.Compare(a.Key, b.Key),
		cmp.Compare(a.Kind, b.Kind),
		cmp.Compare(a.Canonical, b.Canonical),
		cmp.Compare(a.Name, b.Name),
		cmp.Compare(a.Signature, b.Signature),
		cmp.Compare(a.Package, b.Package),
		cmp.Compare(a.Tier, b.Tier),
	)
}

// covers reports whether the known span of s holds every line of start..end.
func covers(s lang.Symbol, start, end int) bool {
	return known(s) && s.Line <= start && end <= s.EndLine
}

// moreInner reports whether a is a more inner container than b: the latest
// Line, then the earliest EndLine, then the later in the total order of before
// (the largest Key first). It is the exact reverse of before, so the innermost
// symbol is always the last in the one listing order and, among equal spans, the
// first of them is the outer one (ESCALATE-27 R4).
func moreInner(a, b lang.Symbol) bool { return order(a, b) > 0 }

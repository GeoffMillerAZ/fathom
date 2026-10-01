package decision

import (
	"fmt"
	"math"
	"slices"
)

// scoreLevelCountCeiling stands in for the level count of an integral score
// range wider than any real limit, so a range like [0, 1e30] neither
// overflows int nor slips under a declared maximum.
const scoreLevelCountCeiling = math.MaxInt32

// outsideDeclaredLimits reports whether q lies outside the limits c declares,
// and if so why: the reason names provider and the limit, and is the string
// an adapter records in Response.Invalid for q's ID without sending q at all.
// It is the generic half of "an adapter's vendor limits are the adapter's
// business, not the core's" (DP-13): Validate checks only the structure every
// Decider shares, and an adapter with limits of its own passes its declared
// Capabilities here, so it refuses cleanly instead of failing the batch or
// sending a question the provider will reject.
//
// It checks, in this order:
//
//   - the question's kind, against c.Kinds. A declared Capabilities with no
//     Kinds answers no kind, as Capabilities.Kinds documents; the alias
//     "noul" is compared as KindPredicate on both sides.
//   - a KindChoice question's option count, against c.MaxOptions.
//   - a KindScore question's level count, against c.MinScoreLevels and
//     c.MaxScoreLevels. The count is len(q.Levels) when Levels is set, else
//     the width of an integral [Min, Max] range (Max-Min+1, the levels an
//     adapter synthesizes from it). A range with a fractional bound has no
//     level count, so the level limits do not judge it; the adapter renders
//     it or refuses it on its own terms.
//
// A zero limit is no limit on that side, and c.Declared false declares
// nothing, so no question is ever outside it. It does not re-run Validate's
// structural rules.
func outsideDeclaredLimits(provider string, c Capabilities, q Question) (reason string, outside bool) {
	if !c.Declared {
		return "", false
	}
	kind := canonicalKind(q.Kind)
	if !slices.ContainsFunc(c.Kinds, func(k Kind) bool { return canonicalKind(k) == kind }) {
		return fmt.Sprintf("question kind %q is not answered by the %s provider", kind, provider), true
	}
	switch kind {
	case KindChoice:
		if c.MaxOptions > 0 && len(q.Options) > c.MaxOptions {
			return fmt.Sprintf("choice has %d options, but the %s provider allows at most %d", len(q.Options), provider, c.MaxOptions), true
		}
	case KindScore:
		n, counted := scoreLevelCount(q)
		if !counted {
			return "", false
		}
		if c.MinScoreLevels > 0 && n < c.MinScoreLevels {
			return fmt.Sprintf("score has %d %s, but the %s provider needs at least %d", n, pluralLevels(n), provider, c.MinScoreLevels), true
		}
		if c.MaxScoreLevels > 0 && n > c.MaxScoreLevels {
			return fmt.Sprintf("score has %d %s, but the %s provider allows at most %d", n, pluralLevels(n), provider, c.MaxScoreLevels), true
		}
	}
	return "", false
}

// scoreLevelCount is how many ordered levels q, a KindScore question, has:
// len(q.Levels) when Levels is set, else the width of its integral
// [Min, Max] range. counted is false for a range with a fractional
// bound, which has no level count. A range wider than any
// real limit reports scoreLevelCountCeiling.
func scoreLevelCount(q Question) (n int, counted bool) {
	if len(q.Levels) > 0 {
		return len(q.Levels), true
	}
	if q.Min != math.Trunc(q.Min) || q.Max != math.Trunc(q.Max) {
		return 0, false
	}
	span := q.Max - q.Min
	if math.IsNaN(span) || span < 0 {
		return 0, false
	}
	if span >= scoreLevelCountCeiling {
		return scoreLevelCountCeiling, true
	}
	return int(span) + 1, true
}

// pluralLevels is "level" for one and "levels" otherwise.
func pluralLevels(n int) string {
	if n == 1 {
		return "level"
	}
	return "levels"
}

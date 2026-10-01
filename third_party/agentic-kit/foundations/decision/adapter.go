package decision

// The two functions below are the core's support for adapters written in
// their own packages: foundations/decision/llm and
// foundations/decision/typesafe use them, and a Decider written outside the
// kit needs the same two answers to treat the alias "noul" and its own
// declared limits the way the shipped adapters do.

// CanonicalKind returns the canonical Kind for k: the decode alias "noul"
// becomes KindPredicate, and every other value is returned unchanged. An
// adapter switches on CanonicalKind(q.Kind), so a Kind built without
// ParseKind is still answered as its canonical kind.
func CanonicalKind(k Kind) Kind {
	return canonicalKind(k)
}

// OutsideDeclaredLimits reports whether q lies outside the limits c
// declares, and if so why: the reason names provider and the limit, and is
// the string an adapter records in Response.Invalid for q's ID without
// sending q at all. An adapter with limits of its own passes its declared
// Capabilities, so it refuses an out-of-limit question cleanly instead of
// failing the batch or sending a question its provider will reject.
//
// It checks q's kind against c.Kinds (the alias "noul" compared as
// KindPredicate), a KindChoice question's option count against
// c.MaxOptions, and a KindScore question's level count against
// c.MinScoreLevels and c.MaxScoreLevels. The level count is len(q.Levels)
// when Levels is set, else the width of an integral [Min, Max] range; a
// range with a fractional bound has no level count and is not judged. A
// zero limit is no limit on that side, and c.Declared false declares
// nothing, so no question is outside it. It does not re-run Validate.
func OutsideDeclaredLimits(provider string, c Capabilities, q Question) (reason string, outside bool) {
	return outsideDeclaredLimits(provider, c, q)
}

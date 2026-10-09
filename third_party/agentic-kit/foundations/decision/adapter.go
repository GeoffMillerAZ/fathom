package decision

import "fmt"

// The functions below are the core's support for adapters written in their
// own packages: foundations/decision/llm, foundations/decision/typesafe and
// foundations/decision/openai use them, and a Decider written outside the
// kit needs the same answers to treat the alias "noul", its own declared
// limits and a Request's images the way the shipped adapters do.

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

// UnsupportedImages reports whether req carries Images that a Decider
// declaring c cannot send, and if so why: the reason names provider and the
// image count, and is the string an adapter records in Response.Invalid for
// EVERY question of req, without calling its provider. Answering a question
// about an image without the image would answer a different question, so a
// text-only adapter refuses the whole Request instead (a Cascade then
// escalates each question to a stage that declares Images).
//
// It is false when req carries no Images, when c declares Images, and when
// c.Declared is false (an undeclared Decider declares nothing, as with
// OutsideDeclaredLimits).
func UnsupportedImages(provider string, c Capabilities, req Request) (reason string, unsupported bool) {
	if len(req.Images) == 0 || !c.Declared || c.Images {
		return "", false
	}
	return fmt.Sprintf("%s takes no image input; the request carries %d image(s)", provider, len(req.Images)), true
}

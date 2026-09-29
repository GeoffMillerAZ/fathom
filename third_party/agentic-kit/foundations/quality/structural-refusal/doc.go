// Package structuralrefusal refuses an affirmative verdict that cites no
// evidence by rewriting the verdict itself — a "meets" with nothing behind it
// becomes "unknown" IN THE VALUE — so the refusal survives every renderer,
// store and loader downstream. The principle it carries: a refusal that lives
// in a renderer is a refusal one renderer can forget. A warning field beside a
// "meets" is read as a "meets"; the only refusal that survives a rendering
// layer nobody has written yet is one that changes the value.
//
// Reason to exist: the first consumer (named in docs/compat.md) produced
// per-requirement verdicts whose single most useful sentence — "you match
// requirement X" — was also the easiest to invent, because until its facts
// had addresses there was no token a model could cite even if it wanted to.
// This is the shape that makes the claim falsifiable: a verdict, the evidence
// behind it, the verbatim span a reader can check, and the reason a claimed
// verdict was refused. Its headline number is UnknownRate — the share of
// judgments with no citable evidence either way — which replaces any blended
// score, because blending is what lets an evidence-thin answer score the
// same as an evidenced one.
//
// Three decisions that are not obvious:
//
//   - Only the affirmative claim carries a burden of proof. "does-not-meet"
//     needs no evidence, deliberately: an absence has nothing to cite, and
//     demanding a citation for "I have not done this" pushes the answer
//     toward the claim that CAN be evidenced.
//   - Uncitable evidence is discarded BEFORE it is counted. Without that
//     order, evidence: [{}] satisfies "has evidence" and the refusal never
//     fires — the check would be counting list length, not evidence.
//   - Normalize is applied on READ (UnknownRate), not only on write. New is
//     the only way to BUILD a judgment in Go, but a loader bypasses every
//     constructor: a hand-edited store saying verdict: "meets" with an empty
//     evidence list must not count as a match because it reached the store
//     through the loader instead of through Go. schema.cue states the same
//     rule (a "meets" carries evidence), so a store decoded against it
//     refuses the row at load, and Normalize catches what was never decoded
//     that way.
//
// Shape contract: schema.cue is authoritative; Judgment, Evidence and Rate
// mirror #Judgment, #Evidence and #Rate, and any drift is a bug. What counts
// as a resolvable evidence kind is the consumer's vocabulary — its schema
// narrows #Evidence.kind; this package requires only that a citation name a
// kind and an id at all.
//
// Concurrency: every value is plain data and every function is pure; the
// package holds no state and is safe for unrestricted concurrent use.
//
// Non-guarantees (stated as plainly as the guarantees — VISION belief 6):
//   - it does not resolve evidence: an id naming a kind and a value the
//     consumer's store has never heard of is citable here and false there.
//     Resolution is the consumer's, and it is the second half of honesty;
//   - it does not judge: Refuse never turns "unknown" into anything else and
//     never turns "meets" into "does-not-meet";
//   - it is not a security boundary of any kind.
//
// Verified against: go 1.26, cue v0.16.1.
package structuralrefusal

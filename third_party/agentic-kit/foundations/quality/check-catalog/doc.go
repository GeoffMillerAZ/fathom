// Package checkcatalog makes a gate's checks enumerable: every check is a
// catalog row with an id, a severity and — the field that matters most — a
// declared means, deterministic or model, so "how much of this gate needs no
// model" is a printed number rather than a claim. Reconcile holds the catalog
// and the implementations to each other in both directions: a check that runs
// without a row is an undocumented gate, and a row with no check is a promise
// the code does not keep. A consumer's bidirectional test is one call and one
// assertion.
//
// Reason to exist: the first consumer (named in docs/compat.md) had good gates
// — a parse self-test, numeric traceability, a retraction registry, keyword
// coverage — scattered across four packages with four report shapes, so
// nothing could answer "is this artifact ready" except by running everything
// and reading prose. Making the checks rows fixed that, and the two structural
// rules Reconcile enforces are what keep the rows honest:
//
//	a check not in the catalog does not run
//	a catalog row with no implementation fails the build
//
// Severity uses a published four-tier parser-vendor taxonomy (fatal / major /
// data / improvement) rather than an invented one, because a scale nobody
// else uses is a scale nobody calibrates. Blocking is decided by the
// severity, once, in Severity.Blocking — a check choosing its own blocking
// behaviour is how a gate quietly stops gating.
//
// Two more rules the type carries. A fatal check must be deterministic (in
// schema.cue and in Row.Validate): a model deciding whether an artifact is
// readable at all is exactly the inversion "a model is never the last line of
// defense" forbids. And a check that cannot run must say so — return Skipped,
// never an empty slice — because a skipped check that looks like a passed one
// is the quiet failure a gate exists to prevent; Run treats a nil Fn the same
// way.
//
// Shape contract: schema.cue is authoritative; Row mirrors #Check and any
// drift between the two is a bug (CUE owns the shape, Go owns the behavior).
// Loading rows out of CUE is the consumer's job — this package is stdlib only
// and decodes nothing.
//
// Determinism: Run executes checks in id order and reads no clock, so a
// report diffs cleanly between runs.
//
// Concurrency: every value here is plain data and every function is pure; the
// package holds no state and is safe for unrestricted concurrent use. A
// Check's Fn is the consumer's, and so is its concurrency contract.
//
// Non-guarantees (stated as plainly as the guarantees — VISION belief 6):
//   - it does not load, parse or validate CUE; the consumer decodes its
//     catalog and hands Rows in;
//   - it cannot tell a check that returned no findings because the input was
//     clean from one that returned none because it silently could not run —
//     only the Skipped convention distinguishes them, and only where it is
//     used;
//   - it is not a security boundary of any kind.
//
// Verified against: go 1.26, cue v0.16.1.
package checkcatalog

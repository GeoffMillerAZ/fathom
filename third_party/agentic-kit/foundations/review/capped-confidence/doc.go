// Package cappedconfidence bounds a score by the ceiling its evidence
// permits, from a table the caller loads out of CUE, and renders a capped
// score as a band with its reason — never as the point value. A judgment can
// be no more confident than the worst evidence it rests on: a 92 computed
// from a line split on a delimiter is not a 92, it is an upper bound of 50
// wearing a 92's clothes, and printing the point value is what makes it
// unfalsifiable.
//
// Reason to exist: the first consumer (named in docs/compat.md) found that
// nothing downstream of ingestion knew where a fact came from, so a record
// read from the owner's own system and one guessed out of scraped markup
// could produce the same number. Its fix was a ceiling per evidence class,
// pinned in a CUE data file so re-tuning a ceiling is a reviewable diff
// rather than a silent recalibration inside a scorer, with the rationale
// (why) mandatory because a judgement nobody can re-derive is one nobody can
// revise. This package is that fix with the table left to the caller:
// classes come from configuration, never from Go, and an undeclared class
// gets no default — it is precisely the case where a guess would do the most
// damage.
//
// Three states, and the third is the point. Supported: the raw stands.
// Capped: the raw exceeded its ceiling and Render says "≤ceiling — class
// evidence", keeping Raw verbatim so the overstatement stays visible.
// Unscored: no ceiling could be established — no table, an undeclared class,
// a raw that is not a number — and Render says so instead of returning a
// number, because a check that cannot run must never look clean. The zero
// Score is unscored: a value that never went through Cap has nothing
// bounding it.
//
// Relation to foundations/review/scoring: ApplyCeiling there caps an integer
// confidence by a fixed five-tier vocabulary whose ceilings live in Go, and
// returns only the capped number. This package is the table-driven form —
// the classes and ceilings are data, the raw survives, and "could not cap" is
// a state rather than a pass-through. It lands beside ApplyCeiling, not in
// place of it: callers that need the vocabulary in data, the band, or an
// honest unscored state migrate; callers content with the fixed tiers need
// not.
//
// Shape contract: schema.cue is authoritative; Config, Ceiling and Score
// mirror #Table, #Ceiling and #Score, and any drift is a bug. Cap clamps DOWN
// only: a ceiling is a maximum, and flooring a negative raw here would let
// "≤max" render for a value that was never a score.
//
// Concurrency: a *Table is immutable after NewTable and safe for
// unrestricted concurrent use; every other function is pure.
//
// Non-guarantees (stated as plainly as the guarantees — VISION belief 6):
//   - it loads no CUE or JSON; the caller decodes its table and hands it to
//     NewTable, which re-checks every #Table constraint so a bypassed schema
//     still fails closed;
//   - it does not floor, round or range-check the raw beyond "is a number";
//     enforcement at the write boundary is the caller's;
//   - a Score assembled by a struct literal is only as honest as its author —
//     Cap and Unscored are the constructors that carry the claim.
//
// Verified against: go 1.26, cue v0.16.1.
package cappedconfidence

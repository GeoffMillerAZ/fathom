// Package fuzzy provides pure-logic fuzzy matching and ranking for
// chat, thread, and search UX. No I/O; no external dependencies.
//
// # Matching
//
// Match scores a needle against a haystack using a case-insensitive
// subsequence match. The score is computed from weighted components:
//
//   - +2 per matched rune
//   - +3 bonus when a match starts at a word boundary (start of string,
//     or the rune immediately follows a space or ASCII punctuation)
//   - +2 bonus for each continuation of a contiguous run (second rune
//     onward in an unbroken matched sequence)
//   - -1 penalty per gap rune between matched runes (cumulative; floor 0)
//
// See the exported constants for the exact weight values.
//
// # Multi-term
//
// MatchTerms splits the needle on whitespace and requires every term to
// match (AND semantics). Score is the sum of per-term scores; positions
// are the sorted, deduplicated union of per-term positions.
//
// # Ranking
//
// Rank applies MatchTerms semantics to a slice of candidates and returns
// matching results sorted by score descending, then by original index
// ascending (stable, deterministic). An empty or whitespace-only needle
// returns all candidates in input order with score zero.
//
// Concurrency: all functions are pure and safe for concurrent use.
package fuzzy

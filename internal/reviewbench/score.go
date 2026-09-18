// Score reproduces the 2026-09-17 "strict re-score" section of
// docs/bench/readings/vb24-review-historical-prs.md exactly (cited inline
// below), scoring against the review engine's OWN structured Go values
// (review.ChangedSymbolReview / impact.TestReach) rather than re-parsing
// rendered Markdown — the class of bug ("CALLER_RE… never matched a single
// nested-caller line") that hid the original pass's real result.
package reviewbench

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

// HitKind classifies how (or whether) a gold entry was matched — the same
// four buckets the strict re-score's per-pair table uses.
type HitKind string

const (
	// HitOwnSymbol: gold names a (file, func) INSIDE a file A's own diff
	// touched — trivially present in fathom's file-level "Changed symbols"
	// listing, no call-graph inference required (the strict re-score's
	// point: this is not evidence of index inference).
	HitOwnSymbol HitKind = "own_symbol"
	// HitCaller: gold names a (file, func) OUTSIDE A's changed files that
	// fathom's caller listing (direct, or transitive when depth > 1) names
	// as a caller of a symbol A's diff changed — genuine call-graph
	// inference.
	HitCaller HitKind = "caller"
	// HitAffectedTest: gold names a pre-existing Test* function (outside
	// A's changed files) that fathom's test-impact composition reports as
	// reaching a symbol A's diff changed — genuine reachability inference.
	HitAffectedTest HitKind = "affected_test"
	// HitMiss: no match under any rule above.
	HitMiss HitKind = "miss"
)

// GoldMatch is one gold entry's scoring outcome.
type GoldMatch struct {
	Gold GoldEntry
	Kind HitKind
	// Depth is the caller depth the match was found at (1 = direct caller,
	// 2 = a caller of a caller, …) — 0 for HitOwnSymbol/HitAffectedTest/HitMiss.
	Depth int
	// Tier is the matched caller's/test's edge_confidence
	// ("resolved"/"heuristic") — "" for HitOwnSymbol/HitMiss.
	Tier string
}

// PairScore is one pair's full scoring outcome: every gold entry's match,
// plus the pair-level strict-new-hit verdict the row's `>= 30%` bar is
// measured against.
type PairScore struct {
	Pair    Pair
	Gold    []GoldEntry
	Matches []GoldMatch
}

// StrictNewHit reports whether this pair counts as a "strict-new hit" —
// docs/bench/readings/vb24-review-historical-prs.md's actual bar: at least
// one gold entry matched as HitCaller or HitAffectedTest (own-symbol
// matches, however many, never qualify a pair on their own).
func (p PairScore) StrictNewHit() bool {
	for _, m := range p.Matches {
		if m.Kind == HitCaller || m.Kind == HitAffectedTest {
			return true
		}
	}
	return false
}

// AnyHit reports whether this pair had at least one match of any kind
// (the ORIGINAL, superseded scoring rule — reported alongside StrictNewHit
// for context, never as the row's own bar).
func (p PairScore) AnyHit() bool {
	return len(p.Matches) > 0
}

func leafName(symbolName string) string {
	if i := strings.LastIndex(symbolName, "."); i >= 0 {
		return symbolName[i+1:]
	}
	return symbolName
}

func inSet(set map[string]bool, s string) bool { return set[s] }

// ScorePair scores one pair's gold entries against the review engine's
// changed-symbol/caller detail and test-impact composition — the SAME
// "Changed symbols" + "Test gap"/"Affected tests" data
// `renderGitHubReview` sources (github.go), read directly instead of
// through its rendered Markdown.
//
// changedFiles is the actual `git diff --name-only` result the review ran
// over (A's own changed files) — the boundary "outside A's own diff"
// matches against.
func ScorePair(pair Pair, changedFiles []string, changedSymbols []review.ChangedSymbolReview, testGap impact.TestGapResult) PairScore {
	gold := ExtractGold(pair.PRB)
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}

	// ownByFile: every symbol name (leaf) defined in a changed file —
	// file-level, exactly what ChangedSymbolReviews itself enumerates.
	ownByFile := map[string]map[string]bool{}
	// callersByFile: every caller's (leaf name, depth, tier), file-scoped,
	// regardless of which top-level changed symbol it calls — a maintainer
	// reading the rendered comment sees all of them, so scoring does too.
	type callerHit struct {
		depth int
		tier  string
	}
	callersByFile := map[string]map[string]callerHit{}

	for _, cs := range changedSymbols {
		if ownByFile[cs.File] == nil {
			ownByFile[cs.File] = map[string]bool{}
		}
		ownByFile[cs.File][leafName(symbolLeaf(cs.Symbol))] = true
		for _, c := range cs.Callers {
			if callersByFile[c.File] == nil {
				callersByFile[c.File] = map[string]callerHit{}
			}
			name := leafName(symbolLeaf(c.Symbol))
			depth := c.Depth
			if depth == 0 {
				depth = 1
			}
			existing, ok := callersByFile[c.File][name]
			// Keep the SHALLOWEST depth / best (resolved-first) tier a
			// gold name was reachable at — a maintainer reading the
			// comment sees the best evidence for a repeated name, not an
			// arbitrary one.
			if !ok || depth < existing.depth || (depth == existing.depth && string(c.Tier) == "resolved") {
				callersByFile[c.File][name] = callerHit{depth: depth, tier: string(c.Tier)}
			}
		}
	}

	testsByFile := map[string]map[string]bool{}
	for _, t := range testGap.AffectedTests {
		if testsByFile[t.File] == nil {
			testsByFile[t.File] = map[string]bool{}
		}
		testsByFile[t.File][leafName(symbolLeaf(t.Test))] = true
	}

	matches := make([]GoldMatch, 0, len(gold))
	for _, g := range gold {
		m := GoldMatch{Gold: g, Kind: HitMiss}
		if own := ownByFile[g.File]; own != nil && own[g.Func] && changed[g.File] {
			m.Kind = HitOwnSymbol
		} else if callers := callersByFile[g.File]; callers != nil {
			if hit, ok := callers[g.Func]; ok && !changed[g.File] {
				m.Kind = HitCaller
				m.Depth = hit.depth
				m.Tier = hit.tier
			}
		}
		if m.Kind == HitMiss {
			if tests := testsByFile[g.File]; tests != nil && tests[g.Func] && !changed[g.File] {
				m.Kind = HitAffectedTest
			}
		}
		matches = append(matches, m)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Gold.File != matches[j].Gold.File {
			return matches[i].Gold.File < matches[j].Gold.File
		}
		return matches[i].Gold.Func < matches[j].Gold.Func
	})

	return PairScore{Pair: pair, Gold: gold, Matches: matches}
}

// symbolLeaf strips the "service::pkg." key prefix a ChangedSymbolReview's
// Symbol/Callers carry (core.SymbolKey form), leaving "Type.Method" or
// "Name" — leafName then takes the final "."-segment.
func symbolLeaf(key string) string {
	if i := strings.Index(key, "::"); i >= 0 {
		key = key[i+2:]
	}
	// key is now "dir/path.pkg.Name" or "dir/path.pkg.Type.Method" (a
	// directory-qualified package glued to the dotted name — see
	// core.QualifiedPackage/SymbolKey). A directory or package segment
	// never itself contains ".", so the first "." AFTER the last "/"
	// starts the dotted-name portion (Name, or Type.Method).
	slash := strings.LastIndex(key, "/")
	rest := key
	if slash >= 0 {
		rest = key[slash+1:]
	}
	if dot := strings.Index(rest, "."); dot >= 0 {
		return rest[dot+1:]
	}
	return rest
}

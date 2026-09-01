// Package search is app-kit's retrieval core.
//
// It holds exactly one thing: reciprocal rank fusion, the arithmetic that
// turns several already-ranked candidate-id lists into one fused order. RRF
// is the only piece of retrieval this package claims is the SAME calculation
// wherever it runs, because it is the only piece with no opinion about what a
// document is, how it was tokenized, what a match score means, or who is
// allowed to see it — fusing two ranked lists is identical work whether they
// came from a TailwindPlus component's BM25+vector arms
// (internal/twplus/index.SearchHybrid) or from a classification-banded
// document corpus's retrievers (platform-engineering's portal/internal/search,
// which re-exports this package's names so its own callers are unchanged).
//
// What deliberately does NOT live here: tokenization (a component's class
// names — "rounded-md", "dark:", "aria-checked:" — need hyphen/colon-aware
// splitting; a document's prose does not, and the two corpora's tokenizers
// stay independent for exactly that reason); BM25 itself (both consumers
// reach it through SQLite FTS5's built-in bm25(), which is not Go code to
// extract — and the portal additionally has a hand-rolled, band-local scorer
// where every statistic — df, N, avgdl — is computed within one
// classification band on purpose, per ADR-0051: baking a corpus-global
// statistic into a package shared with a corpus that has no access control at
// all would be exactly the kind of leak a refactor reintroduces without
// anyone reading it); and access control (this package never sees a document,
// only opaque ids a caller already retrieved from something that already
// decided who may see them). Promoting any of those into one API would force
// one consumer to route around whatever the other needed, which is worse than
// two implementations — so only the part that really is one calculation
// moved.
package search

import (
	"cmp"
	"sort"
)

// DefaultRRFK is the reciprocal-rank-fusion constant: large enough that the
// difference between rank 1 and rank 2 does not swamp a second retriever's
// opinion, which is the entire point of fusing. 60 is the value the original
// RRF paper (Cormack, Clarke & Buettcher, 2009) settled on, and it is the
// value both of this package's two consumers had independently chosen before
// this package existed.
const DefaultRRFK = 60.0

// Ranked is one fused result: an id plus the fused score and the per-list rank
// that produced it.
//
// Ranks is kept rather than discarded because an unexplained ranking is a
// black box: a caller that wants to say WHY a result ranked where it did (the
// portal's ports.Hit.Why) needs Ranks[i], the 1-based rank of ID in lists[i],
// or 0 if that retriever did not return it. Length always equals len(lists).
//
// T is constrained to cmp.Ordered rather than plain comparable because the
// determinism rule below needs a total order to break ties on, and every
// known id type — a document ref string, a SQLite variant rowid — already has
// one.
type Ranked[T cmp.Ordered] struct {
	ID    T
	Score float64
	Ranks []int
}

// FuseRRF fuses N ranked id lists by reciprocal rank: an id's score is the sum
// of 1/(k+rank) over every list that returned it, rank counted from 1.
//
// Determinism: ties break on id, ascending. Two retrievers that disagree by a
// float rounding step must not reorder results between runs, because a golden
// query set (or a screenshot in a plan) would then be untrustworthy.
//
// A k of zero or less is meaningless (it makes the top rank's contribution
// infinite or negative) and is replaced by DefaultRRFK rather than rejected:
// this is a ranking function, not a validator, and it must always produce an
// order.
//
// A retriever that repeats an id within its own list must not be paid twice
// for it; the best (lowest) rank in that list is the one that counts. The
// zero value of T ("" for a string id, 0 for an integer rowid) is dropped
// rather than ranked, on the theory that an adapter emits the zero value only
// to mean "no result here", never a real candidate — true of both of this
// package's consumers, whose id spaces (non-empty document refs; SQLite
// rowids, which start at 1) never legitimately produce it.
func FuseRRF[T cmp.Ordered](lists [][]T, k float64) []Ranked[T] {
	if k <= 0 {
		k = DefaultRRFK
	}
	var zero T
	n := len(lists)
	pos := make(map[T]int, 64)
	out := make([]Ranked[T], 0, 64)

	for li, list := range lists {
		seen := make(map[T]bool, len(list))
		for r, id := range list {
			if id == zero || seen[id] {
				continue
			}
			seen[id] = true
			i, ok := pos[id]
			if !ok {
				i = len(out)
				pos[id] = i
				out = append(out, Ranked[T]{ID: id, Ranks: make([]int, n)})
			}
			out[i].Ranks[li] = r + 1
			out[i].Score += 1 / (k + float64(r+1))
		}
	}

	// out is built in first-encounter order, which is a deterministic
	// function of the input, and sorted by a total order — so no map
	// iteration can leak into the result.
	sort.Slice(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return out[a].ID < out[b].ID
	})
	return out
}

// IDs projects a fused result to its id order, which is what a caller needs
// once the documents behind the ids have already been resolved.
func IDs[T cmp.Ordered](rs []Ranked[T]) []T {
	out := make([]T, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

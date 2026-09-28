package docindex

// rank.go — RankDocs (KR-4, docs/design/knowledge-retrieval.md §5): BM25
// candidates from store.QueryDocUnits (heading weighted 3:1 — KR-1's
// column-order choice makes this literal), reordered by reciprocal rank
// fusion (k = 60) against the concern ranker's own order — units doc_links
// (KR-6) ties to the caller's concernSymbols, most-linked first
// (store.DocUnitsLinkedToTargets).
//
// RankDocs never admits a unit the text query itself did not match: the
// candidate universe is exactly the BM25 hit set, and the concern-link
// signal only re-orders it — a unit with no BM25 hit has no snippet() to
// carry and would not be a text search result whatever its link count.
// KR-4 depends only on KR-8 (not KR-6): before any doc_links row exists,
// DocUnitsLinkedToTargets returns none and RankDocs degrades to pure BM25
// order, exactly as if concernSymbols were always empty.

import (
	"context"
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// rrfK is reciprocal rank fusion's k constant (§5: "k = 60").
const rrfK = 60.0

// rankDocsCandidatePool bounds how many BM25 hits RankDocs pulls before
// fusing and truncating to the caller's limit — wide enough that a unit
// ranked outside the top `limit` by BM25 alone can still surface through a
// strong concern-link signal, bounded so a call never asks docs_fts for an
// unbounded result.
const rankDocsCandidatePool = 200

// RankedDoc is one fused `search kind: docs` result: doc_units' metadata,
// the query's snippet() excerpt, the fused RRF score (higher is better),
// and Bytes — the receipt's rendered byte count (§5), an estimate of what
// showing this result costs, not the FTS index's own storage footprint.
type RankedDoc struct {
	UnitID          string
	Path            string
	LineStart       int
	LineEnd         int
	HeadingPath     string
	Kind            string
	StaleByLocation bool
	Snippet         string
	Score           float64
	Bytes           int
}

// RankDocs answers one `search kind: docs` query (§5). st must be a
// *store.SQLiteStore with a populated doc index (docindex.go's doc comment:
// docs_fts is SQLite-only) — a nil st or a query error is returned as an
// error, never a silent empty result (I38). limit <= 0 defaults to 50 (§5).
func RankDocs(ctx context.Context, st *store.SQLiteStore, serviceID, query string, concernSymbols []string, limit int) ([]RankedDoc, error) {
	if st == nil {
		return nil, fmt.Errorf("RankDocs: nil store")
	}
	if limit <= 0 {
		limit = 50
	}
	pool := rankDocsCandidatePool
	if pool < limit {
		pool = limit
	}

	hits, err := st.QueryDocUnits(ctx, serviceID, query, pool)
	if err != nil {
		return nil, fmt.Errorf("RankDocs: %w", err)
	}
	if len(hits) == 0 {
		return nil, nil
	}

	linkMatches, err := st.DocUnitsLinkedToTargets(ctx, serviceID, concernSymbols)
	if err != nil {
		return nil, fmt.Errorf("RankDocs: concern links: %w", err)
	}
	concernRank := make(map[string]int, len(linkMatches)) // unit_id -> 1-based rank
	for i, m := range linkMatches {
		concernRank[m.UnitID] = i + 1
	}

	type scored struct {
		hit   store.DocSearchHit
		score float64
	}
	scoredHits := make([]scored, len(hits))
	for i, h := range hits {
		s := 1.0 / (rrfK + float64(i+1)) // BM25 rank component, 1-based
		if r, ok := concernRank[h.UnitID]; ok {
			s += 1.0 / (rrfK + float64(r))
		}
		scoredHits[i] = scored{hit: h, score: s}
	}
	// Stable sort by score descending, tie-broken by unit_id ascending —
	// determinism (I2): two equal-score hits never depend on QueryDocUnits'
	// own (already-deterministic) tie order surviving the resort.
	sort.SliceStable(scoredHits, func(i, j int) bool {
		if scoredHits[i].score != scoredHits[j].score {
			return scoredHits[i].score > scoredHits[j].score
		}
		return scoredHits[i].hit.UnitID < scoredHits[j].hit.UnitID
	})
	if len(scoredHits) > limit {
		scoredHits = scoredHits[:limit]
	}

	out := make([]RankedDoc, len(scoredHits))
	for i, sc := range scoredHits {
		out[i] = RankedDoc{
			UnitID:          sc.hit.UnitID,
			Path:            sc.hit.Path,
			LineStart:       sc.hit.LineStart,
			LineEnd:         sc.hit.LineEnd,
			HeadingPath:     sc.hit.HeadingPath,
			Kind:            sc.hit.Kind,
			StaleByLocation: sc.hit.StaleByLocation,
			Snippet:         sc.hit.Snippet,
			Score:           sc.score,
			Bytes:           len(sc.hit.Snippet) + len(sc.hit.HeadingPath) + len(sc.hit.Path),
		}
	}
	return out, nil
}

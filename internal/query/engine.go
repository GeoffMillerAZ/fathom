// Package query implements the Layer 3 knowledge query engine for fathom.
// It reads observations from a store.Store and produces typed query results
// including staleness reports and contradiction detection.
//
// Evidence tier ceilings are enforced via the agentic-kit scoring foundation
// (ADR-007): a doc-claimed observation never outranks a verified one at equal
// keyword overlap.
//
// Optional hybrid search: when a *vector.SymbolIndexer is wired in via WithVector and
// Query.SemanticWt > 0, each result's final score blends keyword overlap and
// cosine similarity: final = (1-SemanticWt)*keyword + SemanticWt*semantic.
// When no Indexer is configured the engine behaves exactly as before (keyword-only).
package query

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/review/scoring"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/vector"
)

const (
	defaultTopN   = 20
	defaultMaxAge = 24 * time.Hour
)

// Engine is the knowledge query engine. It is safe for concurrent use.
type Engine struct {
	store   store.Store
	now     func() time.Time
	maxAge  time.Duration
	indexer *vector.SymbolIndexer // nil → keyword-only mode
}

// Option configures an Engine.
type Option func(*Engine)

// WithNow overrides the clock used for staleness checks. Useful in tests.
func WithNow(fn func() time.Time) Option {
	return func(e *Engine) { e.now = fn }
}

// WithMaxAge sets the age threshold beyond which a source is considered stale.
func WithMaxAge(d time.Duration) Option {
	return func(e *Engine) { e.maxAge = d }
}

// WithVector wires a vector.SymbolIndexer into the engine. When non-nil and
// Query.SemanticWt > 0 the engine blends keyword and vector-similarity scores.
// The caller is responsible for indexing symbols before querying.
func WithVector(idx *vector.SymbolIndexer) Option {
	return func(e *Engine) { e.indexer = idx }
}

// New creates an Engine backed by the given store. Options may override
// defaults (clock: time.Now, maxAge: 24h).
func New(s store.Store, opts ...Option) *Engine {
	e := &Engine{
		store:  s,
		now:    time.Now,
		maxAge: defaultMaxAge,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Query executes a concern-scoped knowledge query. ConcernID and ServiceID are
// both required (ADR-006). The result includes ranked SymbolNodes, a staleness
// report, and any detected contradictions.
func (e *Engine) Query(ctx context.Context, q core.Query) (core.QueryResult, error) {
	if q.ConcernID == "" {
		return core.QueryResult{}, errors.New("query: ConcernID is required (ADR-006)")
	}
	if q.ServiceID == "" {
		return core.QueryResult{}, errors.New("query: ServiceID is required")
	}

	obs, err := e.store.QueryObservations(ctx, q.ServiceID, q.ConcernID)
	if err != nil {
		return core.QueryResult{}, err
	}

	// Unscoped-store fallback: a plain ingest writes observations without
	// concern tags, so a concern-scoped read can come back empty even though
	// the service has data. Re-read unscoped and rank by keywords, treating
	// the concern itself as a keyword when none were given. Zero-score
	// observations are dropped so the fallback never fabricates relevance —
	// an empty result stays an honest empty result. Doing this here (not in
	// each surface) keeps CLI, MCP, and the Go client behaviour identical.
	if len(obs) == 0 {
		all, allErr := e.store.QueryObservations(ctx, q.ServiceID, "")
		if allErr != nil {
			return core.QueryResult{}, allErr
		}
		if len(all) > 0 {
			if len(q.Keywords) == 0 {
				q.Keywords = []string{q.ConcernID}
			}
			obs = filterByKeywords(all, q.Keywords)
		}
	}

	// Filter by kind when specified.
	if len(q.Kinds) > 0 {
		obs = filterByKind(obs, q.Kinds)
	}

	topN := q.TopN
	if topN <= 0 {
		topN = defaultTopN
	}

	var nodes []core.SymbolNode

	// Hybrid path: blend keyword score with vector-similarity when configured.
	if e.indexer != nil && q.SemanticWt > 0 {
		var err error
		nodes, err = e.hybridRank(ctx, obs, q.Keywords, q.SemanticWt, topN)
		if err != nil {
			return core.QueryResult{}, err
		}
	} else {
		// Keyword-only path (original behaviour).
		nodes = scoreAndRank(obs, q.Keywords)
		if len(nodes) > topN {
			nodes = nodes[:topN]
		}
	}

	staleness := e.buildStalenessReport(obs)
	contradictions := detectContradictions(obs)

	return core.QueryResult{
		Nodes:          nodes,
		Staleness:      staleness,
		Contradictions: contradictions,
	}, nil
}

// hybridRank scores observations by blending keyword overlap and cosine similarity,
// then returns the top-N nodes. It builds the query text from all keywords joined
// by spaces and searches the Indexer for vector scores, then combines:
//
//	final = (1-semanticWt)*normKeyword + semanticWt*cosineSimilarity
//
// Cosine similarity is in [−1,1]; we normalise the keyword score into [0,1] by
// dividing by len(keywords) (clamped to 1). When keyword list is empty the keyword
// component is 0 for all observations.
func (e *Engine) hybridRank(ctx context.Context, obs []core.Observation, keywords []string, semanticWt float64, topN int) ([]core.SymbolNode, error) {
	// Build a query text for the vector search.
	queryText := strings.Join(keywords, " ")
	if queryText == "" {
		queryText = " " // embed a whitespace token so the search still runs
	}

	// Search the index for the topN nearest symbols.
	hits, err := e.indexer.SearchText(ctx, queryText, topN*2) // over-fetch for blending
	if err != nil {
		return nil, err
	}

	// Build a lookup: symbol name → best cosine score.
	semScore := make(map[string]float64, len(hits))
	for _, h := range hits {
		if prev, ok := semScore[h.Symbol]; !ok || h.Score > prev {
			semScore[h.Symbol] = h.Score
		}
	}

	// Normalise keyword denominator.
	kwDenom := float64(len(keywords))
	if kwDenom < 1 {
		kwDenom = 1
	}

	type hybridScored struct {
		node  core.SymbolNode
		score float64
		tier  int
	}

	items := make([]hybridScored, len(obs))
	for i, o := range obs {
		ks := float64(keywordScore(o, keywords)) / kwDenom
		ss := semScore[o.Symbol] // 0 if not in index
		// Map cosine [-1,1] → [0,1].
		normSS := (ss + 1) / 2
		final := (1-semanticWt)*ks + semanticWt*normSS
		items[i] = hybridScored{
			node:  observationToNode(o),
			score: final,
			tier:  scoring.TierRank(string(o.EvidenceTier)),
		}
	}

	// Sort by (score DESC, tier DESC, name ASC, file ASC) for determinism.
	sort.SliceStable(items, func(i, j int) bool {
		si := items[i].score*1000 + float64(items[i].tier)
		sj := items[j].score*1000 + float64(items[j].tier)
		if si != sj {
			return si > sj
		}
		if items[i].node.Name != items[j].node.Name {
			return items[i].node.Name < items[j].node.Name
		}
		return items[i].node.FilePath < items[j].node.FilePath
	})

	if len(items) > topN {
		items = items[:topN]
	}

	nodes := make([]core.SymbolNode, len(items))
	for i, it := range items {
		nodes[i] = it.node
	}
	return nodes, nil
}

// FindCallers returns all SymbolNodes for the given serviceID whose Calls list
// contains the given symbol. Matching is by bare name or SymbolKey suffix
// (e.g. "ValidateToken" matches "pkg/auth.ValidateToken").
func (e *Engine) FindCallers(ctx context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	// Query all observations for the service (no concern filter).
	obs, err := e.store.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return nil, err
	}

	var nodes []core.SymbolNode
	for _, o := range obs {
		if callsSymbol(o.Calls, symbol) {
			nodes = append(nodes, observationToNode(o))
		}
	}
	// Stable sort by symbol name for determinism.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// TypedEdges returns every typed Go call edge (ADR-021 §3) stored for
// serviceID — direct, interface-dispatch (one row per implementer), and
// unresolved. Nil for a non-Go service, or a Go service whose typed pass
// hasn't run yet.
func (e *Engine) TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error) {
	return e.store.TypedEdges(ctx, serviceID)
}

// FindSymbol returns all SymbolNodes for the given serviceID whose Symbol field
// matches name exactly, or whose SymbolKey (SourceRef + "." + Symbol) has name
// as a suffix.
func (e *Engine) FindSymbol(ctx context.Context, serviceID, name string) ([]core.SymbolNode, error) {
	obs, err := e.store.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return nil, err
	}

	var nodes []core.SymbolNode
	for _, o := range obs {
		if symbolMatches(o, name) {
			nodes = append(nodes, observationToNode(o))
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// ─── internal helpers ────────────────────────────────────────────────────────

// scored wraps an observation with its computed rank for sorting.
type scored struct {
	node         core.SymbolNode
	keywordScore int
	tierRank     int
}

// scoreAndRank converts observations to SymbolNodes and sorts them:
//  1. Combined score = keywordOverlap*10 + tierRank (higher = better).
//  2. Tie-break by Symbol name (alphabetical, ascending) for determinism.
func scoreAndRank(obs []core.Observation, keywords []string) []core.SymbolNode {
	items := make([]scored, len(obs))
	for i, o := range obs {
		ks := keywordScore(o, keywords)
		tr := scoring.TierRank(string(o.EvidenceTier))
		items[i] = scored{
			node:         observationToNode(o),
			keywordScore: ks,
			tierRank:     tr,
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		si := items[i].keywordScore*10 + items[i].tierRank
		sj := items[j].keywordScore*10 + items[j].tierRank
		if si != sj {
			return si > sj
		}
		// Deterministic tie-break: name then source file.
		if items[i].node.Name != items[j].node.Name {
			return items[i].node.Name < items[j].node.Name
		}
		return items[i].node.FilePath < items[j].node.FilePath
	})

	nodes := make([]core.SymbolNode, len(items))
	for i, it := range items {
		nodes[i] = it.node
	}
	return nodes
}

// keywordScore counts how many keywords appear (case-insensitive) in the
// concatenation of the observation's Symbol and Claim fields.
func keywordScore(o core.Observation, keywords []string) int {
	if len(keywords) == 0 {
		return 0
	}
	haystack := strings.ToLower(o.Symbol + " " + o.Claim)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(haystack, strings.ToLower(kw)) {
			count++
		}
	}
	return count
}

// observationToNode converts an Observation to the SymbolNode surface type by
// delegating to the canonical reconstruction (core.Observation.ToSymbolNode).
// It must carry Package: without it SymbolNode.Key() is malformed
// ("svc::Name" instead of "svc::pkg.Name") and downstream call-graph lookups
// — e.g. verify's package-filtered test_reaches — cannot resolve the node.
func observationToNode(o core.Observation) core.SymbolNode {
	return o.ToSymbolNode()
}

// filterByKeywords returns only observations with a positive keywordScore,
// i.e. at least one keyword appears in the Symbol or Claim text. Used by the
// unscoped-store fallback so it never returns unrelated symbols.
func filterByKeywords(obs []core.Observation, keywords []string) []core.Observation {
	out := obs[:0:0]
	for _, o := range obs {
		if keywordScore(o, keywords) > 0 {
			out = append(out, o)
		}
	}
	return out
}

// filterByKind returns only observations whose SourceKind is in the allow list.
func filterByKind(obs []core.Observation, kinds []core.SourceKind) []core.Observation {
	set := make(map[core.SourceKind]struct{}, len(kinds))
	for _, k := range kinds {
		set[k] = struct{}{}
	}
	out := obs[:0:0] // empty slice sharing no backing array
	for _, o := range obs {
		if _, ok := set[o.SourceKind]; ok {
			out = append(out, o)
		}
	}
	return out
}

// maxStaleExamples caps the SourceStatus rows a query-side StalenessReport
// carries. The report rides on every answer; when a whole index went stale it
// used to list every source (117 lines around an 8-node result). The full
// source list and commit drift are the staleness_report tool's job.
const maxStaleExamples = 3

// buildStalenessReport builds a StalenessReport from the (already filtered)
// observations. One source per distinct SourceRef; uses the latest IngestedAt
// across all observations for that ref.
//
// The report is constant-size: Sources holds at most maxStaleExamples stale
// entries (oldest first); StaleSources/TotalSources/FreshSources and
// OldestIngestedAt carry the full picture.
func (e *Engine) buildStalenessReport(obs []core.Observation) core.StalenessReport {
	// Track latest IngestedAt per source ref.
	latest := map[string]time.Time{}
	for _, o := range obs {
		if t, ok := latest[o.SourceRef]; !ok || o.IngestedAt.After(t) {
			latest[o.SourceRef] = o.IngestedAt
		}
	}

	now := e.now()
	totalSources := len(latest)
	var staleEntries []core.SourceStatus
	var oldest time.Time

	// Sort the keys for deterministic output.
	refs := make([]string, 0, len(latest))
	for ref := range latest {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	for _, ref := range refs {
		ingested := latest[ref]
		if oldest.IsZero() || ingested.Before(oldest) {
			oldest = ingested
		}
		if now.Sub(ingested) > e.maxAge {
			staleEntries = append(staleEntries, core.SourceStatus{
				SourceRef:    ref,
				LastIngested: ingested,
				Stale:        true,
			})
		}
	}

	staleSources := len(staleEntries)
	// Oldest first; ties break on the already-sorted SourceRef for determinism.
	sort.SliceStable(staleEntries, func(i, j int) bool {
		return staleEntries[i].LastIngested.Before(staleEntries[j].LastIngested)
	})
	if len(staleEntries) > maxStaleExamples {
		staleEntries = staleEntries[:maxStaleExamples]
	}

	return core.StalenessReport{
		Sources:             staleEntries,
		StaleSources:        staleSources,
		TotalSources:        totalSources,
		FreshSources:        totalSources - staleSources,
		OldestIngestedAt:    oldest,
		ReingestRecommended: staleSources > 0,
	}
}

// detectContradictions finds observations for the same Symbol with differing
// Claims. Returns at most one Contradiction per symbol pair (first two
// disagreeing claims).
func detectContradictions(obs []core.Observation) []core.Contradiction {
	// Group observations by symbol.
	bySymbol := map[string][]core.Observation{}
	for _, o := range obs {
		bySymbol[o.Symbol] = append(bySymbol[o.Symbol], o)
	}

	// Sort symbols for determinism.
	symbols := make([]string, 0, len(bySymbol))
	for sym := range bySymbol {
		symbols = append(symbols, sym)
	}
	sort.Strings(symbols)

	var contradictions []core.Contradiction
	for _, sym := range symbols {
		group := bySymbol[sym]
		if len(group) < 2 {
			continue
		}
		// Find the first pair with differing claims.
		found := false
		for i := 0; i < len(group) && !found; i++ {
			for j := i + 1; j < len(group) && !found; j++ {
				if group[i].Claim != group[j].Claim {
					contradictions = append(contradictions, core.Contradiction{
						Symbol: sym,
						A:      group[i],
						B:      group[j],
					})
					found = true
				}
			}
		}
	}
	return contradictions
}

// callsSymbol reports whether any entry in the calls slice references symbol
// either exactly or as a SymbolKey suffix (e.g. "pkg/auth.ValidateToken"
// matches bare name "ValidateToken").
func callsSymbol(calls []string, symbol string) bool {
	for _, c := range calls {
		if c == symbol {
			return true
		}
		// suffix match: "pkg/auth.ValidateToken" ends with ".ValidateToken"
		if strings.HasSuffix(c, "."+symbol) {
			return true
		}
	}
	return false
}

// symbolMatches returns true when the observation's Symbol equals name exactly,
// or when the query name is a suffix of the stored symbol (handling "Type.Method"
// stored symbols matched by bare "Method" queries), or when the stored symbol
// is a suffix of the query name (handling "pkg.Symbol" queries against "Symbol").
func symbolMatches(o core.Observation, name string) bool {
	if o.Symbol == name {
		return true
	}
	// Bare query name matches a stored "Type.Method" symbol: e.g. query "CloseCase"
	// matches stored "Registry.CloseCase".
	if strings.HasSuffix(o.Symbol, "."+name) {
		return true
	}
	// Qualified query name matches stored bare symbol: e.g. query "auth.ValidateToken"
	// matches stored "ValidateToken" via the ".Symbol" suffix.
	if strings.HasSuffix(name, "."+o.Symbol) {
		return true
	}
	return false
}

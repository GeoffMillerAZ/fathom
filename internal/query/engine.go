// Package query implements the Layer 3 knowledge query engine for fathom.
// It reads observations from a store.Store and produces typed query results
// including staleness reports and contradiction detection.
//
// Evidence tier ceilings are enforced via the agentic-kit scoring foundation
// (ADR-007): a doc-claimed observation never outranks a verified one at equal
// keyword overlap.
//
// Ranking is keyword overlap blended with evidence tier (scoreAndRank) —
// there is no vector or embedding path. ADR-041 decision 2 deleted the prior
// optional hybrid-search branch (internal/vector, WithVector) as dead code no
// production caller wired; revisit only through vision 31 §5's kill
// criterion (docs/vision/31-substrate.md).
package query

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/review/scoring"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	defaultTopN   = 20
	defaultMaxAge = 24 * time.Hour
)

// Engine is the knowledge query engine. It is safe for concurrent use.
type Engine struct {
	store  store.Store
	now    func() time.Time
	maxAge time.Duration
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
	namesModule := namesModuleSymbol(q)
	obs = withoutModuleSymbols(obs, namesModule)

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
			obs = filterByKeywords(withoutModuleSymbols(all, namesModule), q.Keywords)
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

	nodes := scoreAndRank(obs, q.Keywords)
	if len(nodes) > topN {
		nodes = nodes[:topN]
	}

	staleness := e.buildStalenessReport(obs)
	contradictions := detectContradictions(obs)

	return core.QueryResult{
		Nodes:          nodes,
		Staleness:      staleness,
		Contradictions: contradictions,
	}, nil
}

// FindCallers returns all SymbolNodes for the given serviceID whose Calls list
// contains the given symbol. Matching is by bare name or SymbolKey suffix
// (e.g. "ValidateToken" matches "pkg/auth.ValidateToken").
//
// RT-85: the candidates come from the store's call index
// (ObservationsCalling) — only a service whose index is not complete loads
// its whole observation set.
func (e *Engine) FindCallers(ctx context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	obs, indexed, err := e.store.ObservationsCalling(ctx, serviceID, symbol)
	if err != nil {
		return nil, err
	}
	if !indexed {
		return e.findCallersFull(ctx, serviceID, symbol)
	}
	return callersFrom(obs, symbol), nil
}

// findCallersFull is FindCallers over the service's whole observation set.
func (e *Engine) findCallersFull(ctx context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	obs, err := e.store.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return nil, err
	}
	return callersFrom(obs, symbol), nil
}

func callersFrom(obs []core.Observation, symbol string) []core.SymbolNode {

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
	return nodes
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
//
// finding [8] (DG-10 plan-row spurious-match class): name is normalized
// through normalizeReceiverSyntax FIRST, so a plan contract written in
// ordinary Go doc syntax for a pointer/value-receiver method --
// "(*Pipeline).TypedEdgeStatus" -- can exact-match the extractor's own
// stored form ("Pipeline.TypedEdgeStatus", never parens or a pointer
// star) instead of falling through to symbolMatches' loose bidirectional
// suffix rule and only ever matching an unrelated same-named TYPE. When
// that normalization actually did something (raw was genuinely
// receiver-qualified), a resulting GoMethod match is sorted ahead of a
// same-named GoType match -- the two are otherwise indistinguishable by
// name alone, and a bare type declaration is never what "(*T).M" syntax
// means. Ordinary (non-parenthesized) queries are completely unaffected:
// normalizeReceiverSyntax is a no-op for them, and the existing
// alphabetical-by-Name ordering is unchanged.
//
// RT-85: the candidates are a point lookup over the (service_id, symbol)
// index (ObservationsForSymbol), never the service's whole observation set.
func (e *Engine) FindSymbol(ctx context.Context, serviceID, name string) ([]core.SymbolNode, error) {
	qname := normalizeReceiverSyntax(name)
	obs, err := e.store.ObservationsForSymbol(ctx, serviceID, qname)
	if err != nil {
		return nil, err
	}
	return findSymbolIn(obs, name, qname), nil
}

// findSymbolFull is FindSymbol over the service's whole observation set.
func (e *Engine) findSymbolFull(ctx context.Context, serviceID, name string) ([]core.SymbolNode, error) {
	obs, err := e.store.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return nil, err
	}
	return findSymbolIn(obs, name, normalizeReceiverSyntax(name)), nil
}

func findSymbolIn(obs []core.Observation, name, qname string) []core.SymbolNode {
	receiverQualified := qname != name

	var nodes []core.SymbolNode
	for _, o := range obs {
		if symbolMatches(o, qname) {
			nodes = append(nodes, observationToNode(o))
		}
	}

	if receiverQualified {
		// "(*T).Method" / "(T).Method" syntax unambiguously names a METHOD --
		// never a type, function, or anything else that happens to share the
		// bare name via symbolMatches' loose bidirectional suffix rule
		// (finding [8], DG-10's plan-row spurious-match class). Narrow down
		// to just the method match(es) when at least one exists, so a caller
		// building a canonical key from this result (verify's package-scoped
		// distinctKeys narrowing, in particular) sees exactly one distinct
		// symbol -- the method -- instead of the method AND an unrelated
		// same-named type, which left the canonical key ambiguous and the
		// downstream call-graph reachability check (test_reaches/edge_wired)
		// silently resolving nothing. If no method matched at all (e.g. a
		// genuinely absent one), the full match set is left as-is so
		// symbol_present still reports "not found" rather than silently
		// reporting an unrelated same-named symbol as present.
		var methods []core.SymbolNode
		for _, n := range nodes {
			if n.Kind == core.KindGoMethod {
				methods = append(methods, n)
			}
		}
		if len(methods) > 0 {
			nodes = methods
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes
}

// normalizeReceiverSyntax strips Go's doc-style "(*T).Method" / "(T).Method"
// pointer/value-receiver wrapping down to the plain "T.Method" form the Go
// extractor actually stores (receiverTypeName never keeps the pointer star
// or parens, internal/extractor/goextract.go) -- without this, a plan
// contract written in ordinary Go doc syntax for a pointer-receiver method
// (the idiomatic way to name one) can never exact-match its own stored
// Symbol, and silently falls back to matching an unrelated same-named type
// or function via symbolMatches' loose bidirectional suffix rule (finding
// [8]). Returns name unchanged when it does not look like this form, so
// every other query (the overwhelming majority) is completely unaffected.
func normalizeReceiverSyntax(name string) string {
	if !strings.HasPrefix(name, "(") {
		return name
	}
	close := strings.Index(name, ")")
	if close < 0 {
		return name
	}
	recv := strings.TrimPrefix(name[1:close], "*")
	rest := name[close+1:]
	if recv == "" || !strings.HasPrefix(rest, ".") {
		return name
	}
	return recv + rest
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

// namesModuleSymbol reports whether q's concern is a file's module symbol's
// name (K18, core.ModuleSymbolName) exactly: the one query a module symbol
// ranks for. Keywords never qualify, because callers derive them: find_symbol's
// last did_you_mean lowercases the name it missed, so "<MODULE>" (a spelling
// its case-insensitive tier no longer answers with a module symbol) would
// come back as the keyword "<module>".
func namesModuleSymbol(q core.Query) bool {
	return q.ConcernID == core.ModuleSymbolName
}

// withoutModuleSymbols drops every file's module symbol from obs unless keep
// (the query named one; namesModuleSymbol). Its name contains "module" and
// sorts first on the name tie-break ('<' precedes every letter), so before
// wave h2 a concern "module" on flask 3.1.3 ranked module symbols only and a
// concern-tagged file's module symbol led its concern (O741). The rule is
// the name (core.SymbolNode.IsModuleSymbol), never a kind list, and it is
// applied on read: nothing stored changes.
func withoutModuleSymbols(obs []core.Observation, keep bool) []core.Observation {
	if keep {
		return obs
	}
	out := obs[:0:0]
	for _, o := range obs {
		if !observationToNode(o).IsModuleSymbol() {
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
		// A module symbol's name is shared by every file of its dialect, so
		// the qualifier is the only thing that tells them apart (O743): it
		// must name this file. An ordinary name keeps the suffix rule alone.
		if o.Symbol == core.ModuleSymbolName {
			return moduleQualifierNames(o, strings.TrimSuffix(name, "."+o.Symbol))
		}
		return true
	}
	return false
}

// moduleQualifierNames reports whether qualifier names the file of the module
// symbol o: it ends, at a "." or "/" boundary, the symbol's package or the
// file's stem path (its source path without the extension), in either
// "pkg.sub" or "pkg/sub" spelling. Kind-generic: the name alone marks a
// module symbol (core.IsModuleSymbol), so every dialect whose manifest turns
// module_symbol on inherits it.
func moduleQualifierNames(o core.Observation, qualifier string) bool {
	if qualifier == "" {
		return false
	}
	stem := strings.TrimSuffix(o.SourceRef, path.Ext(o.SourceRef))
	slashed := strings.ReplaceAll(qualifier, ".", "/")
	for _, cand := range []string{o.Package, strings.ReplaceAll(o.Package, ".", "/"), stem} {
		if cand == "" {
			continue
		}
		for _, q := range []string{qualifier, slashed} {
			if cand == q || strings.HasSuffix(cand, "/"+q) || strings.HasSuffix(cand, "."+q) {
				return true
			}
		}
	}
	return false
}

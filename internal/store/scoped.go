package store

// scoped.go — the READ chokepoint (docs/design/scope-and-purge.md §2.2),
// rebuilt after the V-round rework.
//
// The filter is not a decorator a constructor installs; it is PART of the
// backends' own read methods. Every exported constructor in this package —
// Open, BackendForPath, NewSQLiteStore, NewMemoryStore, MountReadOnly,
// OpenPlatform (which builds its per-service stores through
// BackendForPath) — therefore returns a store whose content-bearing reads
// filter, and a constructor that could return an unfiltered store cannot
// be written without failing TestEveryStoreConstructorIsScoped, which
// derives the constructor list from this package's own source (the V3
// rework: the previous build hand-listed two constructors and missed
// MountReadOnly, so eleven shipped surfaces bypassed the filter while the
// "one chokepoint" test passed).
//
// Suppression state is re-read ON EVERY CALL through a scope.Source, which
// stat()s the file and reloads on (mtime, size) change — so the daemon's
// long-lived handle sees a new suppression on the NEXT query, not on the
// next restart (the V5 rework: the previous build read state once at
// store-open, which made the feature inert in the only deployment that
// exists). The state path comes exclusively from $FATHOM_SUPPRESSIONS
// (scope.EnvVar), which the fathom binary's main() defaults to
// ~/.fathom/suppressions.jsonl; nothing here reads the home directory, so
// `go test ./...` on any machine is hermetic unless a test opts in (the V6
// rework: no init()-armed globals, no machine-dependent gate).
//
// The gate filters READS ONLY, and only the content-bearing ones — the
// classification lives in internal/scope's TestSuppressionCoversEverySurface,
// which enumerates every Store method and requires each to be filtered or
// exempt-with-a-written-reason. Writes, deletes and the ingest hash-gate
// pass through untouched: suppression is reversible and must never change
// what gets indexed. Purge reads raw SQL directly and is deliberately NOT
// filtered — a read-side filter in front of a delete would leave the
// suppressed rows undeleted while the run reported success over them.

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
)

// scopeGate carries the per-store suppression source. The zero value is
// unusable; construct via newScopeGate at every store constructor.
type scopeGate struct {
	src *scope.Source
}

// newScopeGate resolves the suppression file from the environment
// (scope.EnvVar). An unset variable yields a permanently disarmed gate —
// the hermetic default for tests and library consumers.
func newScopeGate() scopeGate {
	return scopeGate{src: scope.NewSource(scope.PathFromEnv(), nil)}
}

// filterScoped is the shared body of every filtered read: snapshot the
// suppression state (stat + reload-on-change), apply, record the
// disclosure into the caller's context collector. It never turns a
// successful read into an error: a malformed suppression file fails OPEN
// (scope.Source.Err is the loud surface for that).
func filterScoped[T any](ctx context.Context, g scopeGate, service string, items []T, err error, refOf func(T) string) ([]T, error) {
	if err != nil || g.src == nil {
		return items, err
	}
	set, now := g.src.Snapshot()
	kept, d := scope.ApplySuppressions(set, service, now, items, refOf)
	scope.Record(ctx, service, d)
	return kept, nil
}

// claimLinkRef extracts the path a claim link cites, through the same
// core.ParseCiteRef grammar the citation was written with. A non-citation
// rel (scopes, scope-fp) carries no path and is never filtered.
func claimLinkRef(l core.ClaimLink) string {
	if p, _, _, ok := core.ParseCiteRef(l.TargetRef); ok {
		return p
	}
	return ""
}

// ---- SQLiteStore: the six content-bearing reads ----

// QueryObservations filters by observations.source_ref.
func (s *SQLiteStore) QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
	obs, err := s.queryObservationsRaw(ctx, serviceID, concernID)
	return filterScoped(ctx, s.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
}

// ListSourceRefs filters the ref list itself. fusion's orphan reconcile
// reads this; filtering can only make it MORE conservative — a suppressed
// ref is never considered an orphan, and a reversible hide must never
// cause a delete.
func (s *SQLiteStore) ListSourceRefs(ctx context.Context, serviceID string) ([]string, error) {
	refs, err := s.listSourceRefsRaw(ctx, serviceID)
	return filterScoped(ctx, s.gate, serviceID, refs, err, func(r string) string { return r })
}

// KnowledgeCards filters by knowledge_cards.file — the card has no
// source_ref column at all, which is exactly the class of row a single
// ref-list filter would miss (§3.3).
func (s *SQLiteStore) KnowledgeCards(ctx context.Context, serviceID string) ([]KnowledgeCard, error) {
	cards, err := s.knowledgeCardsRaw(ctx, serviceID)
	return filterScoped(ctx, s.gate, serviceID, cards, err, func(c KnowledgeCard) string { return c.File })
}

// DocSummaries filters by doc_summaries.source_ref.
func (s *SQLiteStore) DocSummaries(ctx context.Context, serviceID string) ([]DocSummary, error) {
	docs, err := s.docSummariesRaw(ctx, serviceID)
	return filterScoped(ctx, s.gate, serviceID, docs, err, func(d DocSummary) string { return d.SourceRef })
}

// TypedEdges filters by typed_edges.from_file.
func (s *SQLiteStore) TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error) {
	edges, err := s.typedEdgesRaw(ctx, serviceID)
	return filterScoped(ctx, s.gate, serviceID, edges, err, func(e core.Edge) string { return e.FromFile })
}

// ClaimLinks filters by the path inside claim_links.target_ref.
func (s *SQLiteStore) ClaimLinks(ctx context.Context, serviceID string, claimIDs []string) ([]core.ClaimLink, error) {
	links, err := s.claimLinksRaw(ctx, serviceID, claimIDs)
	return filterScoped(ctx, s.gate, serviceID, links, err, claimLinkRef)
}

// CertifyDeclarationsForService filters by certify_declarations.file (vision
// 33 stage 0, TC-5) — a declared marker names the FILE its Test* function
// lives in, the same path-anchored shape as a claim_links citation.
func (s *SQLiteStore) CertifyDeclarationsForService(ctx context.Context, serviceID string) ([]CertifyDeclaration, error) {
	decls, err := s.certifyDeclarationsForServiceRaw(ctx, serviceID)
	return filterScoped(ctx, s.gate, serviceID, decls, err, func(d CertifyDeclaration) string { return d.File })
}

// SourceKindCounts filters by observations.source_ref (RT-144).
func (s *SQLiteStore) SourceKindCounts(ctx context.Context, serviceID string) ([]SourceKindCount, error) {
	rows, err := s.sourceKindCountsRaw(ctx, serviceID)
	return filterScoped(ctx, s.gate, serviceID, rows, err, func(r SourceKindCount) string { return r.SourceRef })
}

// ---- MemoryStore: the same six (+ CertifyDeclarationsForService) ----

// QueryObservations filters by observations.source_ref.
func (m *MemoryStore) QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error) {
	obs, err := m.queryObservationsRaw(ctx, serviceID, concernID)
	return filterScoped(ctx, m.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
}

// ListSourceRefs filters the ref list itself.
func (m *MemoryStore) ListSourceRefs(ctx context.Context, serviceID string) ([]string, error) {
	refs, err := m.listSourceRefsRaw(ctx, serviceID)
	return filterScoped(ctx, m.gate, serviceID, refs, err, func(r string) string { return r })
}

// KnowledgeCards filters by knowledge_cards.file.
func (m *MemoryStore) KnowledgeCards(ctx context.Context, serviceID string) ([]KnowledgeCard, error) {
	cards, err := m.knowledgeCardsRaw(ctx, serviceID)
	return filterScoped(ctx, m.gate, serviceID, cards, err, func(c KnowledgeCard) string { return c.File })
}

// DocSummaries filters by doc_summaries.source_ref.
func (m *MemoryStore) DocSummaries(ctx context.Context, serviceID string) ([]DocSummary, error) {
	docs, err := m.docSummariesRaw(ctx, serviceID)
	return filterScoped(ctx, m.gate, serviceID, docs, err, func(d DocSummary) string { return d.SourceRef })
}

// TypedEdges filters by typed_edges.from_file.
func (m *MemoryStore) TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error) {
	edges, err := m.typedEdgesRaw(ctx, serviceID)
	return filterScoped(ctx, m.gate, serviceID, edges, err, func(e core.Edge) string { return e.FromFile })
}

// ClaimLinks filters by the path inside claim_links.target_ref.
func (m *MemoryStore) ClaimLinks(ctx context.Context, serviceID string, claimIDs []string) ([]core.ClaimLink, error) {
	links, err := m.claimLinksRaw(ctx, serviceID, claimIDs)
	return filterScoped(ctx, m.gate, serviceID, links, err, claimLinkRef)
}

// CertifyDeclarationsForService filters by certify_declarations.file — see
// the SQLiteStore twin's doc comment.
func (m *MemoryStore) CertifyDeclarationsForService(ctx context.Context, serviceID string) ([]CertifyDeclaration, error) {
	decls, err := m.certifyDeclarationsForServiceRaw(ctx, serviceID)
	return filterScoped(ctx, m.gate, serviceID, decls, err, func(d CertifyDeclaration) string { return d.File })
}

// ---- TypedEdgeStatus (DG-10): both backends ----

// TypedEdgeStatus filters the persisted status's DiscoveryGapFiles, its one
// path-bearing field, so a suppressed file never surfaces through
// list_services, staleness_report or /statusz. Counts stay as recorded; the
// drop is disclosed like every other filtered read.
func (s *SQLiteStore) TypedEdgeStatus(ctx context.Context, serviceID string) (TypedEdgeStatusRecord, bool, error) {
	rec, ok, err := s.typedEdgeStatusRaw(ctx, serviceID)
	return filterTypedEdgeStatus(ctx, s.gate, serviceID, rec, ok, err)
}

// TypedEdgeStatus filters DiscoveryGapFiles; see the SQLiteStore method.
func (m *MemoryStore) TypedEdgeStatus(ctx context.Context, serviceID string) (TypedEdgeStatusRecord, bool, error) {
	rec, ok, err := m.typedEdgeStatusRaw(ctx, serviceID)
	return filterTypedEdgeStatus(ctx, m.gate, serviceID, rec, ok, err)
}

func filterTypedEdgeStatus(ctx context.Context, g scopeGate, serviceID string, rec TypedEdgeStatusRecord, ok bool, err error) (TypedEdgeStatusRecord, bool, error) {
	if err != nil || !ok {
		return rec, ok, err
	}
	rec.DiscoveryGapFiles, err = filterScoped(ctx, g, serviceID, rec.DiscoveryGapFiles, nil, func(f string) string { return f })
	return rec, ok, err
}

// SourceKindCounts filters by observations.source_ref (RT-144).
func (m *MemoryStore) SourceKindCounts(ctx context.Context, serviceID string) ([]SourceKindCount, error) {
	rows, err := m.sourceKindCountsRaw(ctx, serviceID)
	return filterScoped(ctx, m.gate, serviceID, rows, err, func(r SourceKindCount) string { return r.SourceRef })
}

// SourcePackages filters by the one source_ref it reads (RT-27).
func (s *SQLiteStore) SourcePackages(ctx context.Context, serviceID, sourceRef string) ([]string, error) {
	pkgs, err := s.sourcePackagesRaw(ctx, serviceID, sourceRef)
	return filterSourcePackages(ctx, s.gate, serviceID, sourceRef, pkgs, err)
}

// SourcePackages filters by the one source_ref it reads (RT-27).
func (m *MemoryStore) SourcePackages(ctx context.Context, serviceID, sourceRef string) ([]string, error) {
	pkgs, err := m.sourcePackagesRaw(ctx, serviceID, sourceRef)
	return filterSourcePackages(ctx, m.gate, serviceID, sourceRef, pkgs, err)
}

// filterSourcePackages drops every package when sourceRef itself is
// suppressed — the rows all share that one anchor.
func filterSourcePackages(ctx context.Context, g scopeGate, serviceID, sourceRef string, pkgs []string, err error) ([]string, error) {
	if err != nil || len(pkgs) == 0 {
		return pkgs, err
	}
	kept, ferr := filterScoped(ctx, g, serviceID, []string{sourceRef}, nil, func(r string) string { return r })
	if ferr != nil || len(kept) == 0 {
		return nil, ferr
	}
	return pkgs, nil
}

// PackageObservations filters by observations.source_ref (RT-27).
func (s *SQLiteStore) PackageObservations(ctx context.Context, serviceID string, pkgs []string) ([]core.Observation, error) {
	obs, err := s.packageObservationsRaw(ctx, serviceID, pkgs)
	return filterScoped(ctx, s.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
}

// PackageObservations filters by observations.source_ref (RT-27).
func (m *MemoryStore) PackageObservations(ctx context.Context, serviceID string, pkgs []string) ([]core.Observation, error) {
	obs, err := m.packageObservationsRaw(ctx, serviceID, pkgs)
	return filterScoped(ctx, m.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
}

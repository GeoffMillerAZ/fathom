package mcp

// graph_patch.go — SU-4 (docs/plans/substrate.md §A, ADR-041 decision 3):
// the daemon's memoized call graphs are keyed by the store watermark
// (IndexedCommit@IndexedAt), and a single-file re-ingest (SU-5, POST
// /refresh-file) replaces one file's observations WITHOUT moving that
// watermark. Before this wiring the memo therefore kept serving the
// pre-refresh graph under the same key: the pre-edit hook refreshed the
// file, and the next find_callers on one of its symbols answered from the
// stale graph (O99). Patching in place keeps the memo — no full Build,
// 13-25 s on this repository — AND makes the refreshed package visible.

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// patchScope is service's graph-store scope at the backend's current
// watermark.
func (s *Server) patchScope(service string) memo.Scope {
	//fathom:tenancy-allow bg-ctx scope lookup for a cache patch after a refresh job, called through a context-free interface; a point query on the watermark
	return navScopeNoEnsure(context.Background(), s.backend, service)
}

// HasCachedGraph reports whether the snapshot or any graph is memoized for
// service — the refresh path asks it before paying to list the package's
// nodes; a snapshot alone must be patched too (the iterating tools read
// it under the watermark a single-file refresh does not move).
func (s *Server) HasCachedGraph(service string) bool {
	return memo.HasGraphs(s.patchScope(service))
}

// PatchPackageGraph replaces package pkg's nodes (and, when typed is
// non-nil, its typed edges) in the graph store's snapshot and graphs for
// service (memo.PatchPackage): the syntax and syntax+typed graphs are
// patched copy-on-write, the resolved-only graph dropped and rebuilt on
// its next use. A no-op when nothing is cached for service.
func (s *Server) PatchPackageGraph(service, pkg string, nodes []core.SymbolNode, typed []core.Edge) {
	memo.PatchPackage(s.patchScope(service), pkg, nodes, typed)
}

// DropCachedGraphs forgets every memoized graph for service — the honest
// fallback when a refresh cannot name the package to patch (a file whose
// fresh extraction produced no symbols): the next query rebuilds; a stale
// graph is never served.
func (s *Server) DropCachedGraphs(service string) {
	memo.DropScope(s.patchScope(service))
}

// evictServiceCaches drops service's memo entries once an ingest of it may
// have written (§5: eager eviction on WatermarkBumped). A backend that
// names no store caches nothing to evict.
func evictServiceCaches(backend any, service string) {
	if id, ok := backend.(storeIdentifier); ok && id.StoreID() != "" {
		memo.EvictStoreService(id.StoreID(), service)
	}
}

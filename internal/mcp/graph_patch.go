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

import "github.com/geoffmilleraz/fathom/internal/core"

// The two cache keys loadNavGraphs and verify_plan memoize under.
func graphKeyBoth(service string) string     { return service + "\x00both" }
func graphKeyResolved(service string) string { return service + "\x00resolved" }

// HasCachedGraph reports whether any graph is memoized for service — the
// refresh path asks it before paying to list the package's nodes.
func (s *Server) HasCachedGraph(service string) bool {
	return s.graphCache.Has(graphKeyBoth(service)) || s.graphCache.Has(graphKeyResolved(service))
}

// PatchPackageGraph replaces package pkg's nodes (and, when typed is
// non-nil, its typed edges) in the memoized syntax+typed graph. The
// resolved-only graph is dropped rather than patched: ReplacePackage
// re-derives the package's syntax edges from Calls, which the resolved graph
// must never carry — it rebuilds on its next use (BuildResolved, the cheaper
// build). A no-op when nothing is cached for service.
func (s *Server) PatchPackageGraph(service, pkg string, nodes []core.SymbolNode, typed []core.Edge) {
	s.graphCache.InvalidatePackage(graphKeyBoth(service), pkg, nodes, typed)
	s.graphCache.Invalidate(graphKeyResolved(service))
}

// DropCachedGraphs forgets every memoized graph for service — the honest
// fallback when a refresh cannot name the package to patch (a file whose
// fresh extraction produced no symbols): the next query rebuilds; a stale
// graph is never served.
func (s *Server) DropCachedGraphs(service string) {
	s.graphCache.Invalidate(graphKeyBoth(service))
	s.graphCache.Invalidate(graphKeyResolved(service))
}

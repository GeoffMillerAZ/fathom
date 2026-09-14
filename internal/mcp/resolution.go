// Package mcp — the VB-19 receipt envelope (docs/vision/29-verbs-not-
// lookups.md §3 wave E; ADR-035 decision 4): every agent-surface answer
// that returns edges or edge-derived facts (references, find_callers,
// blast_radius, symbol_context, safe_delete_check, rename_impact,
// test_impact, review_diff) reports the resolution-tier mix it rests on
// plus the index commit it was answered from, so a stranger can decide
// whether to bother re-deriving it (VB-20, `fathom receipt verify`) before
// spending the git-archive-and-go/types cost.
//
// resolution always reflects exactly the facts a response actually
// returned — never a wider set a budget or elision line trimmed away
// (the same discipline blastBucketsOf's `include` filter already
// established for by_depth, depth.go).
//
// index_commit has no snapshot isolation with the rows beside it: each
// handler computes its rows first and calls indexCommitFor (or
// uniformIndexCommit) after, as a separate ServiceMeta read — a concurrent
// ingest landing in between can make index_commit name a commit NEWER
// than what the rows actually reflect. Narrow in practice (both reads
// happen within one request), and a receipt naming a too-new commit fails
// `fathom receipt verify` loudly (a real mismatch) rather than silently,
// but it is not a guarantee — a future revision could snapshot the
// watermark once per call, before the rows are computed, if this is ever
// observed to matter.
package mcp

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// resolutionMix is the envelope's `resolution: {typed: n, heuristic: m}`
// field. Always present, never omitted: a {0,0} mix on an empty answer is
// itself the honest fact ("no edge-derived rows here"), not something to
// hide behind omitempty. No `external` count — ResolutionTierOf never
// returns that tier today (ADR-021 §R6), so adding an always-zero third
// key would be decoration, not evidence.
type resolutionMix struct {
	Typed     int `json:"typed"`
	Heuristic int `json:"heuristic"`
}

// addConfidence tallies one already-computed core.EdgeConfidence value —
// the resolved→typed, heuristic→heuristic mapping core.ResolutionTierOf
// performs from an Edge's Kind, applied directly to a confidence value a
// caller already holds (callerDepths/blastBucketsOf's depthInfo, safe_
// delete_check's tierOf, …) rather than reconstructing a synthetic Edge
// just to re-derive what is already known.
func (m *resolutionMix) addConfidence(c core.EdgeConfidence) {
	if c == core.EdgeConfidenceResolved {
		m.Typed++
		return
	}
	m.Heuristic++
}

// addKind tallies one core.EdgeKind (callgraph.EdgeInfo.Kind) via
// core.ResolutionTierOf, for callers that only have the raw Kind on hand
// (CallersDetail/CalleesDetail rows) rather than a precomputed confidence.
func (m *resolutionMix) addKind(k core.EdgeKind) {
	if core.ResolutionTierOf(core.Edge{Kind: k}).Kind == core.TierTyped {
		m.Typed++
		return
	}
	m.Heuristic++
}

// resolutionFromDepthBuckets sums the per-depth resolved/heuristic counts
// depth.go's withDepthBuckets already computed (find_callers, references)
// into one envelope-level mix. Depth-0 rows (the definition/target itself,
// never an edge) contribute nothing on either side, by construction.
func resolutionFromDepthBuckets(buckets []depthBucket) resolutionMix {
	var m resolutionMix
	for _, b := range buckets {
		m.Typed += b.Resolved
		m.Heuristic += b.Heuristic
	}
	return m
}

// resolutionFromBlastBuckets is resolutionFromDepthBuckets for
// blast_radius' by_depth shape (blastBucket carries the same two counts).
func resolutionFromBlastBuckets(buckets []blastBucket) resolutionMix {
	var m resolutionMix
	for _, b := range buckets {
		m.Typed += b.Resolved
		m.Heuristic += b.Heuristic
	}
	return m
}

// uniformIndexCommit is indexCommitFor for a fleet-wide answer (references)
// that scans several services: it reports one commit only when every named
// service resolves to the same non-empty commit — a mixed-commit scan has
// no single honest watermark, mirroring references' own occurrence_service
// uniform-lift (renderReferenceOccurrence). uniform is false (and commit
// "") when services is empty, any service can't name a commit, or two
// disagree.
func uniformIndexCommit(ctx context.Context, backend QueryInterface, services []string) (commit string, uniform bool) {
	for i, svc := range services {
		ic := indexCommitFor(ctx, backend, svc)
		if ic == "" {
			return "", false
		}
		if i == 0 {
			commit = ic
			continue
		}
		if ic != commit {
			return "", false
		}
	}
	return commit, commit != ""
}

// indexCommitFor returns the daemon's watermark for service (ServiceMeta's
// indexed_commit) — the VB-19 envelope's `index_commit`, a receipt's
// anchor to the commit `fathom receipt verify` (VB-20) re-derives against.
// "" when the backend cannot name one (no ServiceMeta support, or the
// service was never ingested) — every caller omits the field in that case,
// exactly as safe_delete_check already does.
func indexCommitFor(ctx context.Context, backend QueryInterface, service string) string {
	_, indexCommit, found, err := backend.ServiceMeta(ctx, service)
	if err != nil || !found {
		return ""
	}
	return indexCommit
}

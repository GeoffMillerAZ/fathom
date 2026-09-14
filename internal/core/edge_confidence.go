package core

// EdgeConfidence classifies how much a reader may trust a call edge
// (docs/vision/26-proof-credibility.md §3, docs/plans/blast-depth.md BD-1):
// the two words the blast-radius tools speak per edge.
type EdgeConfidence string

const (
	// EdgeConfidenceResolved: the Go typed pass (internal/fusion.
	// EnrichTypedEdges, ADR-021 §3) resolved this edge with compiler-grade
	// type information — a direct call, one implementer of an
	// interface-dispatched call, or a function value.
	EdgeConfidenceResolved EdgeConfidence = "resolved"
	// EdgeConfidenceHeuristic: a syntax-only edge — built from
	// Observation.Calls by the extractor, never confirmed by the typed pass
	// (either the pass has not run for the service, or the call site
	// resolved to EdgeUnresolved and the resolver explicitly gave up).
	EdgeConfidenceHeuristic EdgeConfidence = "heuristic"
)

// EdgeConfidenceOf reads an edge's confidence from the provenance the
// fusion pass already records on it: Edge.Kind is "" for every syntax-only
// producer and is set only by the typed enrichment pass, so the
// classification is a lookup, never a guess. EdgeUnresolved is heuristic
// too — it records that the typed resolver could NOT settle the target,
// which is weaker evidence than a resolved edge, not stronger. Any other
// non-empty Kind (including kinds added after this function) came from the
// typed pass and is resolved.
func EdgeConfidenceOf(e Edge) EdgeConfidence {
	switch e.Kind {
	case "", EdgeUnresolved:
		return EdgeConfidenceHeuristic
	default:
		return EdgeConfidenceResolved
	}
}

// CrossRepoEdgeConfidenceOf reports a CrossRepoEdge's confidence, mirroring
// EdgeConfidenceOf's two-word vocabulary for the cross-repo platform graph
// (VB-30). Every CrossRepoEdgeServiceRef edge is EdgeConfidenceHeuristic: a
// normalized (method, path template) string match across repositories,
// never confirmed by a compiler-grade resolver. The original
// CrossRepoEdgeCall edges (platformlink.BuildFromSymbols, the zero-value
// Kind) are ALSO heuristic today — that linker joins by bare "pkg.Name"
// string equality across repos, which the Go typed pass never verifies
// across a service boundary. The switch stays explicit (rather than a bare
// `return EdgeConfidenceHeuristic`) so a future typed cross-repo resolver
// has a branch to land in without a caller-visible change.
func CrossRepoEdgeConfidenceOf(e CrossRepoEdge) EdgeConfidence {
	switch e.Kind {
	default:
		return EdgeConfidenceHeuristic
	}
}

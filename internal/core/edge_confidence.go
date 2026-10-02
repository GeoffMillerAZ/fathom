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
// producer and is set only by a typed/resolved enrichment pass, so the
// classification is a lookup, never a guess. EdgeUnresolved is heuristic
// too — it records that a resolver could NOT settle the target, which is
// weaker evidence than a resolved edge, not stronger. EdgeFileResolved
// (ADR-041 decision 5, SU-11) is ALSO always heuristic, explicitly — it
// carries no compiler/type-checker confirmation, only import-graph plus
// name-in-scope matching, and must never be promoted by falling into the
// "any other Kind is resolved" default below just because it is non-empty.
// EdgeTypeRef (wave lg) is heuristic for the same reason: it labels a
// syntax-only edge to a Go type, a reference no typed pass confirmed.
// Any OTHER non-empty Kind (including kinds added after this function,
// besides the three named exceptions) came from a typed pass and is
// resolved.
func EdgeConfidenceOf(e Edge) EdgeConfidence {
	switch e.Kind {
	case "", EdgeUnresolved, EdgeFileResolved, EdgeTypeRef:
		return EdgeConfidenceHeuristic
	default:
		return EdgeConfidenceResolved
	}
}

// CrossRepoEdgeConfidenceOf reports a CrossRepoEdge's confidence, mirroring
// EdgeConfidenceOf's two-word vocabulary for the cross-repo platform graph
// (VB-30). Every kind produced today is heuristic: CrossRepoEdgeServiceRef
// is a normalized (method, path template) string match across
// repositories, and CrossRepoEdgeCall (platformlink.BuildFromSymbols, the
// zero-value Kind) joins by bare "pkg.Name" string equality across repos —
// neither is ever confirmed by a compiler-grade resolver. A future typed
// cross-repo resolver adds a branch here (and to CrossRepoEdgeKind) when it
// exists; until then there is none to take.
func CrossRepoEdgeConfidenceOf(e CrossRepoEdge) EdgeConfidence {
	return EdgeConfidenceHeuristic
}

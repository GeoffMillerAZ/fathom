package core

// ResolutionTierKind is the closed set of evidence classes a call/reference
// edge can carry (VB-19, docs/vision/29-verbs-not-lookups.md §3 wave E;
// ADR-035 decision 4): "typed" (the Go typed pass, internal/fusion.
// EnrichTypedEdges, resolved it with compiler-grade type information),
// "heuristic" (a syntax/name-based match — every tree-sitter language today,
// plus the ambiguous-callee fallback for Go), or "external" (an edge
// imported from another indexer's output — reserved by ADR-021/ADR-035 for
// a future `ingest --scip`-style import; nothing produces it yet).
type ResolutionTierKind string

const (
	TierTyped     ResolutionTierKind = "typed"
	TierHeuristic ResolutionTierKind = "heuristic"
	TierExternal  ResolutionTierKind = "external"
)

// ResolutionTier is one edge's resolution tier. Indexer/Version are set only
// when Kind == TierExternal — String() renders that case as
// "external(<indexer>@<version>)", matching the plan's own naming
// (docs/plans/verbs.md VB-19). The zero value is TierHeuristic (never a
// silent "typed" default), and the struct never grows a fifth field-free
// kind — the enum is closed at these three.
type ResolutionTier struct {
	Kind    ResolutionTierKind
	Indexer string
	Version string
}

// String renders the tier the way every envelope and receipt names it:
// "typed", "heuristic", or "external(<indexer>@<version>)".
func (t ResolutionTier) String() string {
	switch t.Kind {
	case TierExternal:
		return "external(" + t.Indexer + "@" + t.Version + ")"
	case TierTyped:
		return string(TierTyped)
	default:
		return string(TierHeuristic)
	}
}

// ExternalResolutionTier constructs the reserved "external(<indexer>@
// <version>)" tier for an edge imported from another indexer's output.
// Nothing in this repository produces one today (ADR-021 §R6, ADR-035
// decision 4) — the constructor exists so the enum is genuinely closed
// (every non-typed, non-heuristic tier is spelled this way) rather than
// an unreachable doc comment.
func ExternalResolutionTier(indexer, version string) ResolutionTier {
	return ResolutionTier{Kind: TierExternal, Indexer: indexer, Version: version}
}

// resolutionTierOf derives edge's resolution tier from the provenance the
// fusion pass already recorded on it (Edge.Kind) — the same lookup
// EdgeConfidenceOf already performs, mapped onto the three-way tier rather
// than duplicated: EdgeConfidenceResolved becomes typed,
// EdgeConfidenceHeuristic becomes heuristic. No edge produced by this
// repository's extractors or typed pass is ever "external" — that tier is
// reserved for an imported index (ADR-021 §R6) and is never returned here.
func resolutionTierOf(edge Edge) ResolutionTier {
	if EdgeConfidenceOf(edge) == EdgeConfidenceResolved {
		return ResolutionTier{Kind: TierTyped}
	}
	return ResolutionTier{Kind: TierHeuristic}
}

// ResolutionTierOf is resolutionTierOf's exported entry point — every
// consumer outside internal/core (the agent-surface envelopes, VB-19) calls
// this rather than duplicating the Edge.Kind → tier mapping.
func ResolutionTierOf(edge Edge) ResolutionTier {
	return resolutionTierOf(edge)
}

package langbench

// routes.go — LB-F98 (wave px): route references scored at a tool format's
// L3. A route pair is a cross-file reference (route entity to the provider
// entity its target host names), so it is scored into the xref family — the
// union with any cross_references the same subject's oracle scored there —
// and, for disclosure, into its own route_* counts.

// The route counts a subject carries beside the xref family's.
const (
	CntRouteTruth        = "route_truth"
	CntRouteFathom       = "route_fathom"
	CntRouteTruthFound   = "route_truth_found"
	CntRouteFathomInTrue = "route_fathom_in_truth"
	// CntRouteUnresolvedPrefix + reason counts fathom's unresolved route
	// references; CntRouteOracleUnresolvedPrefix + reason the oracle's.
	CntRouteUnresolvedPrefix       = "route_unresolved_"
	CntRouteOracleUnresolvedPrefix = "route_oracle_unresolved_"
)

// ScoreRouteRefs counts route truth against found (entity granularity,
// names compared whole) into counts' xref family and its route_* counts.
func ScoreRouteRefs(truth, found []XrefPair, counts map[string]int) {
	own := map[string]int{}
	ScoreXrefs(truth, found, own)
	for xref, route := range map[string]string{
		CntXrefTruth: CntRouteTruth, CntXrefFathom: CntRouteFathom,
		CntXrefTruthFound: CntRouteTruthFound, CntXrefFathomInTrue: CntRouteFathomInTrue,
	} {
		counts[xref] += own[xref]
		counts[route] += own[xref]
	}
}

// WeakerTier is the weaker of two oracle tiers (A strongest); an empty tier
// is absent, so the other stands.
func WeakerTier(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case tierRank[b] < tierRank[a]:
		return b
	}
	return a
}

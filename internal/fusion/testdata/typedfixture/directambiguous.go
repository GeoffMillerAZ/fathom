package typedfixture

// CallMemStoreDirect calls PutClaims directly on a concrete *MemStore — not
// through the Store interface. DiskStore declares a method of the exact
// same name, so the syntax-only resolver's ambiguous-suffix fallback
// (internal/analysis/callgraph/graph.go's resolveAmbiguousCallees,
// byDotSuffix count > 1) refuses this edge outright: it will not guess
// between MemStore.PutClaims and DiskStore.PutClaims. Only the typed
// pass's precise type-checked resolution (classifySelection, which sees
// m's static type is *MemStore) can wire it — this is O40's exact
// reproduction shape: a direct call on a concrete receiver whose method
// name is shared by another receiver of the same interface.
func CallMemStoreDirect(m *MemStore) error {
	return m.PutClaims("direct")
}

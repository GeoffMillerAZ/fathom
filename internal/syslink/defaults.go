package syslink

// DefaultRegistry returns a fresh registry holding the wave-2 sources of
// docs/design/system-graph.md §5, keyed by ADR-048 dialect id. It builds a
// new registry on every call, so no package-level mutable state exists.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	must := func(dialect string, s ContractSource) {
		if err := r.RegisterContractSource(dialect, s); err != nil {
			panic(err) // a duplicate here is a programming error, never input-dependent
		}
	}
	for _, d := range []string{"go", "javascript", "typescript", "tsx", "python"} {
		must(d, routeSource{})
	}
	must("go", goClientSource{})
	return r
}

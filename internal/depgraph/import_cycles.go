package depgraph

// ImportCycles detects import cycles in a package import graph.
//
// pkgImports maps a package path to the list of packages it imports.  The
// function returns the strongly-connected components that form cycles,
// represented as sorted slices of package paths.  The outer slice is sorted
// by the first element of each cycle for determinism.
//
// Packages that only appear as imports (not as keys in pkgImports) are treated
// as leaf nodes with no outbound imports.
func ImportCycles(pkgImports map[string][]string) [][]string {
	// Build a Graph from the import map and delegate to Graph.Cycles().
	g := NewGraph()
	for pkg, imports := range pkgImports {
		g.AddNode(pkg)
		for _, imp := range imports {
			g.AddEdge(pkg, imp)
		}
	}
	return g.Cycles()
}

package depgraph

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// DepGraph takes an ecosystem and root and returns units,
// edges and lock digests out. The resolver whose Ecosystem() matches is
// selected (first match wins when two claim one name); an unknown
// ecosystem refuses loudly, naming the known set, because a typo that
// resolves to an empty graph is silent under-keying.
//
// Graph.LockDigests aggregates every unit's lock entries under the key
// "<ecosystem>/<unit-name>/<entry-name>" — exactly two joins on "/", and
// since unit names may themselves contain slashes, consumers treat the
// aggregate key as opaque and use Unit.LockDigests when they need the
// parts. Cross-ecosystem edges are declared, not resolved here: a
// single-ecosystem call returns no edges (ResolveDeclaredEdges is the
// seam that populates them).
func DepGraph(ctx context.Context, ecosystem string, root fs.FS, p Profile, resolvers ...Resolver) (Graph, error) {
	var selected Resolver
	known := make([]string, 0, len(resolvers))
	for _, r := range resolvers {
		eco := r.Ecosystem()
		known = append(known, eco)
		if selected == nil && eco == ecosystem {
			selected = r
		}
	}
	if selected == nil {
		sort.Strings(known)
		return Graph{}, fmt.Errorf("depgraph: no resolver for ecosystem %q (known: %s)", ecosystem, strings.Join(known, ", "))
	}

	units, err := selected.Units(ctx, root, p)
	if err != nil {
		return Graph{}, fmt.Errorf("depgraph: resolve %s units: %w", ecosystem, err)
	}

	locks := make(map[string]string)
	for _, u := range units {
		for name, digest := range u.LockDigests {
			locks[u.Ecosystem+"/"+u.Name+"/"+name] = digest
		}
	}
	return Graph{Units: units, Edges: []Edge{}, LockDigests: locks}, nil
}

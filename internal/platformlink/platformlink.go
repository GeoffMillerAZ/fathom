// Package platformlink builds the cross-repo platform graph by walking Go source
// files across multiple service repos, extracting exported symbols and their
// outbound call references, then joining calls that resolve to exported symbols
// in a different service into directed core.CrossRepoEdge records.
//
// BuildPlatformGraph is a pure function: identical inputs always produce an
// identical, sorted graph with no shared state or network access.
package platformlink

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

// Graph holds the directed cross-repo call edges for a fleet of services.
// All slices returned by its methods are sorted for determinism.
type Graph struct {
	edges []core.CrossRepoEdge
}

// Edges returns all cross-repo edges in the graph, sorted deterministically.
func (g *Graph) Edges() []core.CrossRepoEdge {
	out := make([]core.CrossRepoEdge, len(g.edges))
	copy(out, g.edges)
	return out
}

// Callers returns all edges whose ToKey matches the given symbol name (bare
// name, e.g. "Greet"). Matching is by bare name so callers can look up a
// symbol without knowing the full qualified key.
func (g *Graph) Callers(toKey string) []core.CrossRepoEdge {
	var out []core.CrossRepoEdge
	for _, e := range g.edges {
		if e.ToKey == toKey {
			out = append(out, e)
		}
	}
	return out
}

// BlastRadius returns the sorted, deduplicated list of service IDs that have
// at least one edge pointing at a symbol exported by the given service.
func (g *Graph) BlastRadius(service string) []string {
	seen := map[string]bool{}
	for _, e := range g.edges {
		if e.ToService == service {
			seen[e.FromService] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	svcs := make([]string, 0, len(seen))
	for s := range seen {
		svcs = append(svcs, s)
	}
	sort.Strings(svcs)
	return svcs
}

// symbolRecord is an internal index entry for a single exported symbol.
type symbolRecord struct {
	service  string
	pkg      string // package name (last path element of the import path)
	name     string // bare symbol name, e.g. "Greet"
	filePath string // service-relative file path
}

// BuildPlatformGraph walks all Go source files in each repo, extracts exported
// symbols and their cross-package call references, and joins references that
// resolve to an exported symbol in a different service into cross-repo edges.
//
// repos maps serviceID → absolute local path. The function is pure: same repos
// map → identical output (sorted edges, no randomness, no side effects).
func BuildPlatformGraph(repos map[string]string) (*Graph, error) {
	if len(repos) == 0 {
		return &Graph{}, nil
	}

	ge := extractor.NewGoExtractor()

	// Phase 1: extract all symbols from every service.
	// exported index: key = "pkgName.SymbolName" → []symbolRecord
	// We use pkgName (last directory element) as the package identifier because
	// cross-package calls in Go source look like "alpha.Greet" where "alpha" is
	// the package name declared in the file.
	type perService struct {
		nodes    []core.SymbolNode
		rootPath string
	}
	allServices := make(map[string]*perService, len(repos))

	// Sorted service IDs for deterministic processing order.
	serviceIDs := make([]string, 0, len(repos))
	for svcID := range repos {
		serviceIDs = append(serviceIDs, svcID)
	}
	sort.Strings(serviceIDs)

	for _, svcID := range serviceIDs {
		rootPath := repos[svcID]
		ps := &perService{rootPath: rootPath}

		err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip hidden, vendor, and testdata directories.
				name := d.Name()
				if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Skip generated files.
			if strings.HasSuffix(path, ".pb.go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}

			relPath, relErr := filepath.Rel(rootPath, path)
			if relErr != nil {
				return relErr
			}

			nodes, extractErr := ge.Extract(svcID, relPath, src)
			if extractErr != nil {
				// Non-fatal: skip unparseable files.
				return nil
			}
			ps.nodes = append(ps.nodes, nodes...)
			return nil
		})
		if err != nil {
			return nil, err
		}

		allServices[svcID] = ps
	}

	// Phase 2: build a cross-service exported symbol index.
	// Maps "pkgName.SymbolName" → symbolRecord for all exported symbols across
	// all services. We only index exported names (ast.IsExported = starts with
	// upper-case), which is the default from GoExtractor.
	exportIndex := map[string]symbolRecord{}

	for _, svcID := range serviceIDs {
		ps := allServices[svcID]
		for _, node := range ps.nodes {
			if !isExported(node.Name) {
				continue
			}
			// node.Package is either the Go package name (for funcs/types) or the
			// receiver type name (for methods). For cross-repo matching we index
			// both "pkgName.SymbolName" and bare "SymbolName" so callers can find
			// symbols even when the local import alias differs.
			qualKey := node.Package + "." + node.Name
			exportIndex[qualKey] = symbolRecord{
				service:  svcID,
				pkg:      node.Package,
				name:     node.Name,
				filePath: node.FilePath,
			}
		}
	}

	// Phase 3: for each call edge, check whether the callee resolves to a symbol
	// in a *different* service. If so, emit a CrossRepoEdge.
	edgeSet := map[core.CrossRepoEdge]bool{}

	for _, svcID := range serviceIDs {
		ps := allServices[svcID]
		for _, node := range ps.nodes {
			for _, callKey := range node.Calls {
				// callKey from GoExtractor is either:
				//   "service::pkg.Name"  (intra-service, already resolved)
				//   "pkg.Name"           (unresolved selector call — potential cross-repo)
				//   "bare"               (unresolved simple call)

				// We only care about selector calls ("pkg.Name") that are not
				// already resolved to the same service.
				if strings.Contains(callKey, "::") {
					// Intra-service resolved key — skip.
					continue
				}
				if !strings.Contains(callKey, ".") {
					// Bare builtin/local name — skip.
					continue
				}

				// callKey looks like "alpha.Greet"
				target, ok := exportIndex[callKey]
				if !ok {
					continue
				}
				// Only emit cross-service edges.
				if target.service == svcID {
					continue
				}

				// Derive bare symbol name from callKey (the part after the dot).
				parts := strings.SplitN(callKey, ".", 2)
				bareSymbol := parts[1]

				edge := core.CrossRepoEdge{
					FromService: svcID,
					FromFile:    node.FilePath,
					ToService:   target.service,
					ToKey:       bareSymbol,
					ToFile:      target.filePath,
				}
				edgeSet[edge] = true
			}
		}
	}

	// Phase 4: collect and sort edges for determinism.
	edges := make([]core.CrossRepoEdge, 0, len(edgeSet))
	for e := range edgeSet {
		edges = append(edges, e)
	}
	sortEdges(edges)

	return &Graph{edges: edges}, nil
}

// sortEdges sorts a slice of CrossRepoEdge in a stable, deterministic order.
func sortEdges(edges []core.CrossRepoEdge) {
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.FromService != b.FromService {
			return a.FromService < b.FromService
		}
		if a.FromFile != b.FromFile {
			return a.FromFile < b.FromFile
		}
		if a.ToService != b.ToService {
			return a.ToService < b.ToService
		}
		if a.ToKey != b.ToKey {
			return a.ToKey < b.ToKey
		}
		return a.ToFile < b.ToFile
	})
}

// isExported reports whether name is an exported Go identifier.
func isExported(name string) bool {
	if name == "" {
		return false
	}
	// Methods are stored as "ReceiverType.MethodName"; check the method part.
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

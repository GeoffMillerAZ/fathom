// Package platformlink builds the cross-repo platform graph from extracted
// symbol nodes across a fleet of service repos: it joins each symbol's outbound
// call references to symbols defined in a *different* service, producing
// directed core.CrossRepoEdge records.
//
// It is language-agnostic. BuildFromSymbols operates on already-extracted
// []core.SymbolNode — typically read back from the store, where every
// extractor's symbols and call edges now live — so cross-repo blast radius
// works for every language whose extractor populates node.Calls, not only Go.
// BuildPlatformGraph is a convenience that walks Go source on disk and feeds the
// same core; the canonical multi-language entry point is
// pkg/platform.BuildFromStore.
//
// All builders are pure: identical inputs always produce an identical, sorted
// graph with no shared state or network access.
package platformlink

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
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

// symbolRecord is an internal index entry for a single defined symbol.
type symbolRecord struct {
	service  string
	pkg      string // package / namespace / receiver of the symbol
	name     string // bare symbol name, e.g. "Greet"
	filePath string // service-relative file path
}

// BuildFromSymbols builds the cross-repo graph from already-extracted symbol
// nodes grouped by service ID. This is the language-agnostic core: it works for
// any language whose extractor populates node.Package / node.Name / node.Calls
// (read back from the store), not just Go.
//
// Matching is qualified and conservative. A caller's outbound call "pkg.Name"
// links to every symbol in a *different* service whose Package=="pkg" and
// Name=="Name". Two forms of callee are intentionally NOT cross-linked:
//
//   - intra-service-resolved keys ("service::pkg.Name"), which are same-service
//     by construction; and
//   - bare, unqualified callees ("Name" with no package), because bare names
//     collide fleet-wide and cross-linking them would manufacture false edges.
//
// The targeted cross_repo_callers MCP tool handles bare-name lookups precisely
// when a caller actually wants them. A call "pkg.Name" that matches symbols in
// several services fans out to all of them — that is the true blast radius.
//
// Pure and deterministic: same input → identical sorted edges.
func BuildFromSymbols(nodesByService map[string][]core.SymbolNode) *Graph {
	if len(nodesByService) == 0 {
		return &Graph{}
	}

	// Deterministic service processing order.
	serviceIDs := make([]string, 0, len(nodesByService))
	for svcID := range nodesByService {
		serviceIDs = append(serviceIDs, svcID)
	}
	sort.Strings(serviceIDs)

	// Phase 1: build the cross-service symbol index, keyed by "Package.Name".
	// Multiple services may define the same qualified key; keep them all so a
	// call fans out to every defining service. No Go-capitalization gate —
	// visibility is language-specific and not parsed here, and cross-service
	// calls only reference reachable symbols in practice.
	exportIndex := map[string][]symbolRecord{}
	for _, svcID := range serviceIDs {
		for _, node := range nodesByService[svcID] {
			if node.Name == "" || node.Package == "" {
				continue
			}
			qualKey := node.Package + "." + node.Name
			exportIndex[qualKey] = append(exportIndex[qualKey], symbolRecord{
				service:  svcID,
				pkg:      node.Package,
				name:     node.Name,
				filePath: node.FilePath,
			})
		}
	}

	// Phase 2: for each qualifying outbound call that resolves to a symbol in a
	// different service, emit a cross-repo edge.
	edgeSet := map[core.CrossRepoEdge]bool{}
	for _, svcID := range serviceIDs {
		for _, node := range nodesByService[svcID] {
			for _, callKey := range node.Calls {
				// Intra-service resolved keys ("service::pkg.Name") are same-service.
				if strings.Contains(callKey, "::") {
					continue
				}
				// Require a package qualifier; bare names are too ambiguous fleet-wide.
				if !strings.Contains(callKey, ".") {
					continue
				}
				for _, target := range exportIndex[callKey] {
					if target.service == svcID {
						continue
					}
					edgeSet[core.CrossRepoEdge{
						FromService: svcID,
						FromFile:    node.FilePath,
						ToService:   target.service,
						ToKey:       target.name,
						ToFile:      target.filePath,
					}] = true
				}
			}
		}
	}

	// Phase 3: collect and sort edges for determinism.
	edges := make([]core.CrossRepoEdge, 0, len(edgeSet))
	for e := range edgeSet {
		edges = append(edges, e)
	}
	sortEdges(edges)

	return &Graph{edges: edges}
}

// BuildPlatformGraph walks all Go source files in each repo, extracts symbols
// and their cross-package call references, and joins references that resolve to
// a symbol in a different service into cross-repo edges. It is a convenience
// wrapper over BuildFromSymbols for the Go-on-disk case; the store-backed,
// multi-language path is pkg/platform.BuildFromStore.
//
// repos maps serviceID → absolute local path. The function is pure: same repos
// map → identical output (sorted edges, no randomness, no side effects).
func BuildPlatformGraph(repos map[string]string) (*Graph, error) {
	if len(repos) == 0 {
		return &Graph{}, nil
	}

	ge := extractor.NewGoExtractor()

	// Sorted service IDs for deterministic processing order.
	serviceIDs := make([]string, 0, len(repos))
	for svcID := range repos {
		serviceIDs = append(serviceIDs, svcID)
	}
	sort.Strings(serviceIDs)

	nodesByService := make(map[string][]core.SymbolNode, len(repos))
	for _, svcID := range serviceIDs {
		rootPath := repos[svcID]
		var nodes []core.SymbolNode

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
			// Skip generated and test files.
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

			extracted, extractErr := ge.Extract(svcID, relPath, src)
			if extractErr != nil {
				// Non-fatal: skip unparseable files.
				return nil
			}
			nodes = append(nodes, extracted...)
			return nil
		})
		if err != nil {
			return nil, err
		}

		nodesByService[svcID] = nodes
	}

	return BuildFromSymbols(nodesByService), nil
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

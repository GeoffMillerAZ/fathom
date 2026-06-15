// Package platform is the public API for sextant's cross-repo platform graph.
// It re-exports the internal platformlink results for external consumers,
// mirroring the sextant_platform_callers and cross_repo_blast_radius MCP tools.
//
// Usage:
//
//	g, err := platform.Build(map[string]string{
//	    "payment-service": "/path/to/payment",
//	    "order-service":   "/path/to/orders",
//	})
//	callers := platform.PlatformCallers(g, "ProcessPayment")
//	deps    := platform.CrossRepoBlastRadius(g, "payment-service")
package platform

import (
	"context"
	"fmt"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/platformlink"
	"github.com/geoffmilleraz/sextant/internal/store"
)

// Graph is the cross-repo platform graph. Use Build to obtain one.
type Graph struct {
	inner *platformlink.Graph
}

// Build walks the Go source in each repo and returns the platform graph.
// repos maps serviceID → absolute local path on disk.
// The function is pure: same repos map → identical output.
func Build(repos map[string]string) (*Graph, error) {
	inner, err := platformlink.BuildPlatformGraph(repos)
	if err != nil {
		return nil, err
	}
	return &Graph{inner: inner}, nil
}

// PlatformCallers returns all cross-repo edges where the callee symbol name
// matches the given symbol (bare name, e.g. "ProcessPayment"). This mirrors
// the sextant_platform_callers MCP tool.
func PlatformCallers(g *Graph, symbol string) []core.CrossRepoEdge {
	return g.inner.Callers(symbol)
}

// CrossRepoBlastRadius returns the sorted list of services that depend on
// (i.e. call exported symbols from) the given service. This mirrors the
// cross_repo_blast_radius MCP tool.
func CrossRepoBlastRadius(g *Graph, service string) []string {
	return g.inner.BlastRadius(service)
}

// BuildFromStore builds the cross-repo platform graph from evidence already
// ingested into the store, across the given services. Unlike Build — which
// walks Go source on disk — this path is language-agnostic: it links the
// symbols and call edges of every language whose extractor populated them at
// ingest time (Python, TS, Java, Rust, …), not just Go.
//
// When serviceIDs is empty, every service in the store is used. The result is
// deterministic for a fixed store state.
func BuildFromStore(ctx context.Context, st store.Store, serviceIDs []string) (*Graph, error) {
	if st == nil {
		return nil, fmt.Errorf("platform.BuildFromStore: nil store")
	}
	if len(serviceIDs) == 0 {
		ids, err := st.ListServiceIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("platform.BuildFromStore: list services: %w", err)
		}
		serviceIDs = ids
	}

	nodesByService := make(map[string][]core.SymbolNode, len(serviceIDs))
	for _, svc := range serviceIDs {
		obs, err := st.QueryObservations(ctx, svc, "")
		if err != nil {
			return nil, fmt.Errorf("platform.BuildFromStore: query %q: %w", svc, err)
		}
		nodes := make([]core.SymbolNode, 0, len(obs))
		for _, o := range obs {
			nodes = append(nodes, o.ToSymbolNode())
		}
		nodesByService[svc] = nodes
	}

	return &Graph{inner: platformlink.BuildFromSymbols(nodesByService)}, nil
}

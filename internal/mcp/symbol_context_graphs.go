package mcp

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/memo"
)

// loadSymbolContextGraphsAt loads exactly the graph symbol_context reads.
// GraphForTyped preserves its uncached syntax fallback on typed read errors.
func loadSymbolContextGraphsAt(ctx context.Context, backend QueryInterface, service string, sc memo.Scope) (*navGraphs, error) {
	snap, err := serviceSnapshotAt(ctx, backend, service, sc)
	if err != nil {
		return nil, err
	}
	g := &navGraphs{Nodes: snap.Nodes, Snapshot: snap}
	g.Both, g.Typed, g.TypedErr, err = memo.GraphForTyped(ctx, snap, memo.GraphBoth, typedLoader(backend, service))
	if err != nil {
		return nil, err
	}
	return g, nil
}

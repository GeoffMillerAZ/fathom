package mcp

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// symbolContextGraphs holds only the navigation inputs symbol_context reads.
// Nodes and Both are shared memo values and must remain read-only.
type symbolContextGraphs struct {
	Nodes    []core.SymbolNode
	Both     *callgraph.Graph
	Typed    []core.Edge
	TypedErr error
}

// loadSymbolContextGraphsAt loads exactly the graph symbol_context reads.
// Syntax and Resolved stay cold; a later reader pays for their builds.
// GraphForTyped preserves its uncached syntax fallback on typed read errors.
func loadSymbolContextGraphsAt(ctx context.Context, backend QueryInterface, service string, sc memo.Scope) (*symbolContextGraphs, error) {
	snap, err := serviceSnapshotAt(ctx, backend, service, sc)
	if err != nil {
		return nil, err
	}
	g := &symbolContextGraphs{Nodes: snap.Nodes}
	g.Both, g.Typed, g.TypedErr, err = memo.GraphForTyped(ctx, snap, memo.GraphBoth, typedLoader(backend, service))
	if err != nil {
		return nil, err
	}
	return g, nil
}

package main

// console_graphstore.go — the console reads the daemon's one symbol
// snapshot and graph store (internal/memo, RT-74, RT-75): the same
// (store, service, watermark) scope the MCP tools key on, so a project
// page and a find_callers call share one load and one graph instead of
// each keeping its own.

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// consoleScope is service's graph-store scope in c's store.
func consoleScope(ctx context.Context, c *client.Client, service string) memo.Scope {
	wm, err := c.StoreWatermark(ctx, service)
	if err != nil {
		wm = ""
	}
	if tok := c.SuppressionToken(service); wm != "" && tok != "" {
		wm += "|suppressed:" + tok // the MCP tools' key (resolved_nav.go navScope)
	}
	return memo.Scope{Store: c.StoreID(), Service: service, Watermark: wm}
}

// consoleSnapshot is service's symbol snapshot.
func consoleSnapshot(ctx context.Context, c *client.Client, service string) (*memo.Snapshot, error) {
	snap, err := memo.SnapshotForExtra(ctx, consoleScope(ctx, c, service), func(l context.Context) ([]core.SymbolNode, any, error) {
		cctx, col := scope.WithCollector(l)
		nodes, err := c.ListAllSymbols(cctx, service)
		return nodes, col.For(service), err
	})
	if err != nil {
		return nil, err
	}
	if d, ok := snap.Extra.(*scope.Disclosure); ok {
		scope.Record(ctx, service, d)
	}
	return snap, nil
}

// consoleTypedLoader loads service's typed edges (an error reads as none,
// as the MCP tools read it).
func consoleTypedLoader(c *client.Client, service string) memo.TypedLoader {
	return func(l context.Context) ([]core.Edge, error) {
		edges, _ := c.TypedEdges(l, service)
		return edges, nil
	}
}

// consoleBothGraph is service's snapshot, typed edges and syntax+typed
// graph — the graph find_callers and the verify engine walk.
func consoleBothGraph(ctx context.Context, c *client.Client, service string) (*memo.Snapshot, []core.Edge, *callgraph.Graph, error) {
	snap, err := consoleSnapshot(ctx, c, service)
	if err != nil {
		return nil, nil, nil, err
	}
	typed := consoleTypedLoader(c, service)
	edges, err := memo.TypedFor(ctx, snap.Scope, typed)
	if err != nil {
		return nil, nil, nil, err
	}
	g, err := memo.GraphFor(ctx, snap, memo.GraphBoth, typed)
	if err != nil {
		return nil, nil, nil, err
	}
	return snap, edges, g, nil
}

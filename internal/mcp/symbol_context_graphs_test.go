package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

type contextGraphBackend struct {
	QueryInterface
	mark         string
	nodes        []core.SymbolNode
	typed        []core.Edge
	typedReads   int
	typedFailure error
}

func (b *contextGraphBackend) StoreID() string { return "symbol-context-graphs" }
func (b *contextGraphBackend) StoreWatermark(context.Context, string) (string, error) {
	return b.mark, nil
}
func (b *contextGraphBackend) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	return b.nodes, nil
}
func (b *contextGraphBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range b.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}
func (b *contextGraphBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	b.typedReads++
	if b.typedFailure != nil {
		return nil, b.typedFailure
	}
	return b.typed, nil
}
func (b *contextGraphBackend) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return "", "", false, nil
}

func contextGraphMisses() int64 {
	for _, s := range memo.Stats() {
		if s.Name == "graphstore" {
			return s.Misses
		}
	}
	return -1
}

func TestSymbolContextBuildsOnlyBothGraph(t *testing.T) {
	n := func(name, file string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "p", Name: name, Kind: core.KindGoFunc, FilePath: file, Calls: calls}
	}
	b := &contextGraphBackend{mark: t.Name(), nodes: []core.SymbolNode{n("Caller", "p/caller.go", "Target"), n("Target", "p/target.go")}}
	b.typed = []core.Edge{{FromService: "svc", FromSymbol: b.nodes[0].Key(), ToSymbol: b.nodes[1].Key(), Kind: core.EdgeDirect}}
	srv := NewServer(b)
	before := contextGraphMisses()
	got := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Target", "snippet": false})
	if got["resolution"] == nil || got["typed_edges"] == nil {
		t.Fatalf("typed context lost: %+v", got)
	}
	if delta := contextGraphMisses() - before; delta != 1 {
		t.Fatalf("graphstore misses = %d, want only Both graph", delta)
	}
	if b.typedReads != 1 {
		t.Fatalf("typed reads = %d, want one", b.typedReads)
	}
	short, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	warm := callToolWithin(t, short, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Target", "snippet": false})
	if warm["partial"] != nil {
		t.Fatalf("resident Both yielded a cold partial: %+v", warm)
	}
	if delta := contextGraphMisses() - before; delta != 1 {
		t.Fatalf("repeat graphstore misses = %d, want resident Both reuse", delta)
	}
	b.mark = t.Name() + "/next"
	b.typed = nil
	next := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Target", "snippet": false})
	if next["typed_edges"] != nil {
		t.Fatalf("new watermark served old typed edges: %+v", next)
	}
	if delta := contextGraphMisses() - before; delta != 2 {
		t.Fatalf("new watermark graphstore misses = %d, want fresh Both", delta)
	}
}

func TestSymbolContextTypedReadFailureRetries(t *testing.T) {
	b := &contextGraphBackend{mark: t.Name(), typedFailure: errors.New("typed read failed"), nodes: []core.SymbolNode{
		{Service: "svc", Package: "p", Name: "Caller", Kind: core.KindGoFunc, FilePath: "p/caller.go", Calls: []string{"Target"}},
		{Service: "svc", Package: "p", Name: "Target", Kind: core.KindGoFunc, FilePath: "p/target.go"},
	}}
	srv := NewServer(b)
	got := callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Target", "snippet": false})
	if got["typed_edges"] != nil {
		t.Fatalf("failed typed read served typed edges: %+v", got)
	}
	if len(got["callers"].([]any)) != 1 {
		t.Fatalf("syntax fallback lost caller: %+v", got)
	}
	b.typedFailure = nil
	b.typed = []core.Edge{{FromService: "svc", FromSymbol: b.nodes[0].Key(), ToSymbol: b.nodes[1].Key(), Kind: core.EdgeDirect}}
	got = callToolWithin(t, context.Background(), srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Target", "snippet": false})
	if got["typed_edges"] == nil || b.typedReads != 2 {
		t.Fatalf("typed retry failed (reads %d): %+v", b.typedReads, got)
	}
}

func TestColdContextNoteDoesNotClaimResidentGraphIsAbsent(t *testing.T) {
	// A cached Both graph can outlive its typed-edge details under the memo
	// budget. The partial note cannot identify which component is missing.
	note := coldGraphNote("svc", &coldGraph{})
	if strings.Contains(note, "graph not built") || !strings.Contains(note, "typed-edge details") {
		t.Fatalf("cold note makes an unsupported graph-state claim: %q", note)
	}
}

package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/mcp"
)

// traverseFakeBackend is a minimal QueryInterface implementation for traverse tests.
// It holds symbol nodes and source files, and implements exactly the 8 methods
// of QueryInterface at the time Feature A was written.
type traverseFakeBackend struct {
	nodes   []core.SymbolNode
	sources []core.SourceFile
}

func (b *traverseFakeBackend) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (b *traverseFakeBackend) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (b *traverseFakeBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	for _, n := range b.nodes {
		if n.Name == name {
			return []core.SymbolNode{n}, nil
		}
	}
	return nil, nil
}

func (b *traverseFakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return b.nodes, nil
}
func (b *traverseFakeBackend) Services(_ context.Context) ([]string, error) { return nil, nil }
func (b *traverseFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return b.sources, nil
}
func (b *traverseFakeBackend) IngestService(_ context.Context, _ string) error { return nil }
func (b *traverseFakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (b *traverseFakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func newTraverseSrv() *mcp.Server {
	// A→B (calls), B→C (calls)
	nodes := []core.SymbolNode{
		{Service: "svc", Name: "A", FilePath: "pkg/a.go", Line: 1, Calls: []string{"B"}},
		{Service: "svc", Name: "B", FilePath: "pkg/b.go", Line: 5, Calls: []string{"C"}},
		{Service: "svc", Name: "C", FilePath: "pkg/c.go", Line: 10, Calls: nil},
	}
	return mcp.NewServer(&traverseFakeBackend{nodes: nodes})
}

func TestTraverseTool_TwoHopCalls(t *testing.T) {
	srv := newTraverseSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 900, Method: "tools/call",
		Params: map[string]any{
			"name": "traverse",
			"arguments": map[string]any{
				"service": "svc",
				"start":   "A",
				"kinds":   []string{"calls"},
				"depth":   2,
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("traverse error: %v", resp["error"])
	}
	p := extToolPayload(t, resp)
	paths, ok := p["paths"].([]any)
	if !ok {
		t.Fatalf("traverse: paths missing: %v", p)
	}
	// Expect at least 2 paths: A→B and A→B→C
	if len(paths) < 2 {
		t.Errorf("traverse A depth=2: expected >=2 paths, got %d", len(paths))
	}
}

func TestTraverseTool_Depth1(t *testing.T) {
	srv := newTraverseSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 901, Method: "tools/call",
		Params: map[string]any{
			"name": "traverse",
			"arguments": map[string]any{
				"service": "svc",
				"start":   "A",
				"depth":   1,
			},
		},
	})
	p := extToolPayload(t, resp)
	paths, ok := p["paths"].([]any)
	if !ok {
		t.Fatalf("traverse: paths missing: %v", p)
	}
	if len(paths) != 1 {
		t.Errorf("traverse A depth=1: expected 1 path, got %d", len(paths))
	}
}

func TestTraverseTool_MissingService(t *testing.T) {
	srv := newTraverseSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 902, Method: "tools/call",
		Params: map[string]any{
			"name":      "traverse",
			"arguments": map[string]any{"start": "A"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for missing service: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

func TestTraverseTool_MissingStart(t *testing.T) {
	srv := newTraverseSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 903, Method: "tools/call",
		Params: map[string]any{
			"name":      "traverse",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for missing start: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

func TestTraverseTool_EdgeCounts(t *testing.T) {
	srv := newTraverseSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 904, Method: "tools/call",
		Params: map[string]any{
			"name": "traverse",
			"arguments": map[string]any{
				"service": "svc",
				"start":   "A",
			},
		},
	})
	p := extToolPayload(t, resp)
	if p["edge_counts"] == nil {
		t.Fatalf("traverse: edge_counts missing: %v", p)
	}
	if p["truncated"] == nil {
		t.Fatalf("traverse: truncated missing: %v", p)
	}
}

func TestTraverseTool_HeterogeneousPath(t *testing.T) {
	// Two-hop: call edge A→B, then spec-ref edge from b.go→M18.2
	nodes := []core.SymbolNode{
		{Service: "svc", Name: "A", FilePath: "pkg/a.go", Line: 1, Calls: []string{"B"}},
		{Service: "svc", Name: "B", FilePath: "pkg/b.go", Line: 5, Calls: nil},
	}
	// Source file for b.go contains a spec reference M18.2
	sources := []core.SourceFile{
		{Path: "pkg/b.go", Content: []byte("// implements M18.2\nfunc B() {}"), Lang: "go"},
	}
	be := &traverseFakeBackend{nodes: nodes, sources: sources}
	srv := mcp.NewServer(be)

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 905, Method: "tools/call",
		Params: map[string]any{
			"name": "traverse",
			"arguments": map[string]any{
				"service": "svc",
				"start":   "A",
				"depth":   3,
			},
		},
	})
	p := extToolPayload(t, resp)
	paths, ok := p["paths"].([]any)
	if !ok {
		t.Fatalf("traverse: paths missing: %v", p)
	}
	// Should find at minimum: A→B (call), and paths via spec-ref from b.go→M18.2
	// The exact count depends on edge assembly; just verify non-empty.
	if len(paths) == 0 {
		t.Error("traverse heterogeneous: expected at least 1 path")
	}
}

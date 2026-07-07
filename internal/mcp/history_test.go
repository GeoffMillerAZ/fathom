package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// historyFakeBackend implements QueryInterface with preset commits.
type historyFakeBackend struct {
	commits []mcp.CommitInfo
	nodes   []core.SymbolNode
}

func (b *historyFakeBackend) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (b *historyFakeBackend) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (b *historyFakeBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	for _, n := range b.nodes {
		if n.Name == name {
			return []core.SymbolNode{n}, nil
		}
	}
	return nil, nil
}

func (b *historyFakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return b.nodes, nil
}
func (b *historyFakeBackend) Services(_ context.Context) ([]string, error) { return nil, nil }
func (b *historyFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}
func (b *historyFakeBackend) IngestService(_ context.Context, _ string) error { return nil }
func (b *historyFakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (b *historyFakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return b.commits, nil
}

var _ mcp.QueryInterface = (*historyFakeBackend)(nil)

func historyFixture() []mcp.CommitInfo {
	t1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)
	t2 := time.Date(2024, 1, 14, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)
	t3 := time.Date(2024, 1, 13, 8, 0, 0, 0, time.UTC).Format(time.RFC3339)
	return []mcp.CommitInfo{
		{
			Hash: "abc12345", Date: t1, Author: "Alice", Message: "Fix auth M18.2",
			Files: []string{"pkg/auth/auth.go", "pkg/auth/token.go"}, FilesTouchedCount: 2, SpecIDs: []string{"M18.2"},
		},
		{
			Hash: "def67890", Date: t2, Author: "Bob", Message: "Refactor utils",
			Files: []string{"pkg/util/util.go"}, FilesTouchedCount: 1, SpecIDs: nil,
		},
		{
			Hash: "fed11111", Date: t3, Author: "Carol", Message: "Add config M18.2",
			Files: []string{"pkg/config/config.go"}, FilesTouchedCount: 1, SpecIDs: []string{"M18.2"},
		},
	}
}

func newHistorySrv() *mcp.Server {
	return mcp.NewServer(&historyFakeBackend{
		commits: historyFixture(),
		nodes: []core.SymbolNode{
			{Service: "svc", Name: "ValidateToken", FilePath: "pkg/auth/token.go"},
		},
	})
}

func TestHistory_All(t *testing.T) {
	srv := newHistorySrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1100, Method: "tools/call",
		Params: map[string]any{
			"name":      "history",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	commits, ok := p["commits"].([]any)
	if !ok {
		t.Fatalf("commits missing: %v", p)
	}
	if len(commits) != 3 {
		t.Errorf("expected 3 commits, got %d", len(commits))
	}
}

func TestHistory_FilterBySpec(t *testing.T) {
	srv := newHistorySrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1101, Method: "tools/call",
		Params: map[string]any{
			"name":      "history",
			"arguments": map[string]any{"service": "svc", "spec": "M18.2"},
		},
	})
	p := extToolPayload(t, resp)
	commits, ok := p["commits"].([]any)
	if !ok {
		t.Fatalf("commits missing: %v", p)
	}
	if len(commits) != 2 {
		t.Errorf("spec filter M18.2: expected 2 commits, got %d", len(commits))
	}
}

func TestHistory_FilterByFile(t *testing.T) {
	srv := newHistorySrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1102, Method: "tools/call",
		Params: map[string]any{
			"name":      "history",
			"arguments": map[string]any{"service": "svc", "file": "token.go"},
		},
	})
	p := extToolPayload(t, resp)
	commits, ok := p["commits"].([]any)
	if !ok {
		t.Fatalf("commits missing: %v", p)
	}
	if len(commits) != 1 {
		t.Errorf("file filter token.go: expected 1 commit, got %d", len(commits))
	}
}

func TestHistory_Limit(t *testing.T) {
	srv := newHistorySrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1103, Method: "tools/call",
		Params: map[string]any{
			"name":      "history",
			"arguments": map[string]any{"service": "svc", "limit": 1},
		},
	})
	p := extToolPayload(t, resp)
	commits, ok := p["commits"].([]any)
	if !ok {
		t.Fatalf("commits missing: %v", p)
	}
	if len(commits) != 1 {
		t.Errorf("limit=1: expected 1 commit, got %d", len(commits))
	}
}

func TestHistory_FilterBySymbol(t *testing.T) {
	srv := newHistorySrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1104, Method: "tools/call",
		Params: map[string]any{
			"name":      "history",
			"arguments": map[string]any{"service": "svc", "symbol": "ValidateToken"},
		},
	})
	p := extToolPayload(t, resp)
	commits, ok := p["commits"].([]any)
	if !ok {
		t.Fatalf("commits missing: %v", p)
	}
	// ValidateToken -> pkg/auth/token.go -> matches commit abc12345
	if len(commits) != 1 {
		t.Errorf("symbol filter ValidateToken: expected 1 commit, got %d", len(commits))
	}
}

func TestHistory_MissingService(t *testing.T) {
	srv := newHistorySrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1105, Method: "tools/call",
		Params: map[string]any{
			"name":      "history",
			"arguments": map[string]any{},
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

// Package mcp_test — TDD test for SR-43 (F-24, design §5): query_symbols
// opens with a one-line answer-first summary and a lower default top-N.
package mcp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// querySymbolsCapturingBackend records the core.Query query_symbols builds
// (in particular TopN, which the mcp layer never re-enforces itself — the
// backend does the ranking/limiting) so the default it passes when the
// caller omits "top" is observable from a test.
type querySymbolsCapturingBackend struct {
	extendedFakeBackend
	lastQuery core.Query
	nodes     []core.SymbolNode
}

func (b *querySymbolsCapturingBackend) QuerySymbols(_ context.Context, q core.Query) (core.QueryResult, error) {
	b.lastQuery = q
	return core.QueryResult{Nodes: b.nodes}, nil
}

func TestQuerySymbolsAnswerFirst(t *testing.T) {
	be := &querySymbolsCapturingBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		nodes: []core.SymbolNode{
			{Service: "svc", Name: "HandleLogin", Kind: core.KindGoFunc, FilePath: "pkg/auth.go", Line: 10},
		},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "query_symbols",
			"arguments": map[string]any{"service": "svc", "concern": "login"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("query_symbols error: %v", resp["error"])
	}
	text := resp["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.HasPrefix(text, `{"summary":`) {
		t.Fatalf("query_symbols answer must open with the summary field first, got: %s", text)
	}

	// SR-43: default top-N is lower than the old 20.
	if be.lastQuery.TopN <= 0 || be.lastQuery.TopN >= 20 {
		t.Fatalf("default TopN = %d, want > 0 and < 20 (cut from the old default)", be.lastQuery.TopN)
	}
}

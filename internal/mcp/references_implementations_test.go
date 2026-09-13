// Package mcp_test — RC-21 (docs/plans/receipts.md; docs/vision/28-receipts.md
// §3.4/F9): references on an interface method lists the implementing types
// first. Reuses the Writer/FileWriter/NopWriter fixture from
// find_symbol_implementations_test.go (SB-5).
package mcp_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func writerImplBackend() *implFakeBackend {
	return &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: writerFixtureNodes(), services: []string{"svc"}},
		sources:             []core.SourceFile{{Path: "sample.go", Content: []byte(writerInterfaceSource), Lang: "go"}},
	}
}

func referencesImplementations(t *testing.T, be *implFakeBackend, query string) map[string]any {
	t.Helper()
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 5000, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	return xtraPayload(t, resp)
}

func TestReferences_InterfaceMethod_DottedQuery_ListsImplementersFirst(t *testing.T) {
	p := referencesImplementations(t, writerImplBackend(), "Writer.Write")
	impls, ok := p["implementations"].([]any)
	if !ok || len(impls) != 2 {
		t.Fatalf("expected 2 implementations, got %v (%v)", p["implementations"], p)
	}
	names := []string{impls[0].(map[string]any)["name"].(string), impls[1].(map[string]any)["name"].(string)}
	if names[0] != "FileWriter.Write" || names[1] != "NopWriter.Write" {
		t.Errorf("expected sorted [FileWriter.Write, NopWriter.Write], got %v", names)
	}
	first := impls[0].(map[string]any)
	if first["file_path"] != "sample.go" {
		t.Errorf("expected file_path=sample.go, got %v", first["file_path"])
	}
	// FileWriter's OWN Write definition sits at line 10 (writerFixtureNodes),
	// not FileWriter's type declaration at line 8 — the concrete site, not
	// the type site.
	if line, _ := first["line"].(float64); int(line) != 10 {
		t.Errorf("expected FileWriter.Write's own line 10, got %v", first["line"])
	}
}

func TestReferences_InterfaceMethod_BareQuery_Unambiguous(t *testing.T) {
	p := referencesImplementations(t, writerImplBackend(), "Write")
	impls, ok := p["implementations"].([]any)
	if !ok || len(impls) != 2 {
		t.Fatalf("expected 2 implementations for bare 'Write', got %v (%v)", p["implementations"], p)
	}
}

func TestReferences_NonInterfaceMethod_NoImplementationsField(t *testing.T) {
	p := referencesImplementations(t, writerImplBackend(), "FileWriter")
	if _, has := p["implementations"]; has {
		t.Errorf("expected no implementations field for a non-interface-method query, got %v", p["implementations"])
	}
}

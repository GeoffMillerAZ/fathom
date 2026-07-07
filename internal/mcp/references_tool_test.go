// Package mcp_test — TDD tests for the references MCP tool.
package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── referencesFakeBackend ────────────────────────────────────────────────────

// referencesFakeBackend supplies two services with real source content and a
// symbol whose name equals the query, so all three assertions can be driven
// from a single backend.
type referencesFakeBackend struct {
	extendedFakeBackend
	sourcesByService map[string][]core.SourceFile
}

func (r *referencesFakeBackend) Sources(_ context.Context, svc string) ([]core.SourceFile, error) {
	return r.sourcesByService[svc], nil
}

// compile-time check.
var _ mcp.QueryInterface = (*referencesFakeBackend)(nil)

// newReferencesSrv returns a Server backed by a two-service fake that contains
// the query token "FindMe" in multiple places.
func newReferencesSrv() (*mcp.Server, string) {
	const query = "FindMe"

	srcA1 := core.SourceFile{
		Path: "pkg/alpha.go",
		// Line 1: no match; Line 2: match; Line 3: match
		Content: []byte("package alpha\n\nfunc FindMe() {}\n// also FindMe here\n"),
		Lang:    "go",
	}
	srcA2 := core.SourceFile{
		Path:    "pkg/other.go",
		Content: []byte("package alpha\n// unrelated\n"),
		Lang:    "go",
	}
	srcB1 := core.SourceFile{
		Path:    "docs/guide.md",
		Content: []byte("# Guide\n\nSee FindMe for details.\n"),
		Lang:    "md",
	}

	be := &referencesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes: []core.SymbolNode{
				{
					Service:  "svcA",
					Package:  "alpha",
					Name:     query, // exact match → definition
					Kind:     core.KindGoFunc,
					FilePath: "pkg/alpha.go",
					Line:     3,
					Tier:     core.TierVerified,
				},
				{
					Service:  "svcA",
					Package:  "alpha",
					Name:     "Other",
					Kind:     core.KindGoFunc,
					FilePath: "pkg/other.go",
					Line:     2,
					Tier:     core.TierVerified,
				},
			},
			services: []string{"svcA", "svcB"},
		},
		sourcesByService: map[string][]core.SourceFile{
			"svcA": {srcA1, srcA2},
			"svcB": {srcB1},
		},
	}

	return mcp.NewServer(be), query
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestReferences_OccurrencesAcrossServices(t *testing.T) {
	srv, query := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2000, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query},
		},
	})
	p := xtraPayload(t, resp)

	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing or wrong type: %v", p)
	}
	// svcA/pkg/alpha.go has 2 matches; svcB/docs/guide.md has 1 match → 3 total
	if len(occs) < 3 {
		t.Errorf("expected at least 3 occurrences across 2 services, got %d", len(occs))
	}

	// Every occurrence must be from one of the expected services and have tier pre-scan.
	services := map[string]bool{}
	for _, raw := range occs {
		occ := raw.(map[string]any)
		svc, _ := occ["service"].(string)
		services[svc] = true
		tier, _ := occ["tier"].(string)
		if tier != "pre-scan" {
			t.Errorf("occurrence tier should be pre-scan, got %q", tier)
		}
		if occ["file"] == nil {
			t.Error("occurrence missing file")
		}
		if occ["line"] == nil {
			t.Error("occurrence missing line")
		}
	}
	if !services["svcA"] {
		t.Error("expected occurrences from svcA")
	}
	if !services["svcB"] {
		t.Error("expected occurrences from svcB")
	}
}

func TestReferences_DefinitionsEntry(t *testing.T) {
	srv, query := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2001, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query},
		},
	})
	p := xtraPayload(t, resp)

	defs, ok := p["definitions"].([]any)
	if !ok {
		t.Fatalf("definitions missing or wrong type: %v", p)
	}
	if len(defs) == 0 {
		t.Fatal("expected at least one definition entry for exact-name match")
	}
	// The symbol "FindMe" in svcA must appear.
	found := false
	for _, raw := range defs {
		def := raw.(map[string]any)
		if def["symbol"] == query && def["service"] == "svcA" {
			found = true
			if def["file"] == nil {
				t.Error("definition missing file")
			}
			if def["tier"] == nil {
				t.Error("definition missing tier")
			}
		}
	}
	if !found {
		t.Errorf("definition for symbol %q in svcA not found in %v", query, defs)
	}
}

func TestReferences_MaxCap_Truncated(t *testing.T) {
	srv, query := newReferencesSrv()
	// Set max=1 so the first occurrence fills the cap.
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2002, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query, "max": 1},
		},
	})
	p := xtraPayload(t, resp)

	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing: %v", p)
	}
	if len(occs) > 1 {
		t.Errorf("expected at most 1 occurrence (cap=1), got %d", len(occs))
	}
	truncated, _ := p["truncated"].(bool)
	if !truncated {
		t.Error("expected truncated=true when cap is exceeded")
	}
}

func TestReferences_ServiceFilter(t *testing.T) {
	srv, query := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2003, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": query, "services": []string{"svcB"}},
		},
	})
	p := xtraPayload(t, resp)

	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing: %v", p)
	}
	for _, raw := range occs {
		occ := raw.(map[string]any)
		if occ["service"] != "svcB" {
			t.Errorf("expected only svcB occurrences, got service=%v", occ["service"])
		}
	}
}

func TestReferences_MissingQuery(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2004, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error code, got %v", code)
	}
}

func TestReferences_BadRegex(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2005, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": "[invalid(", "regex": true},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error code for bad regex, got %v", code)
	}
}

func TestReferences_WordMatch(t *testing.T) {
	// "FindMe" should match but "FindMeNot" should not when word=true
	f := core.SourceFile{
		Path:    "pkg/x.go",
		Content: []byte("FindMe is here\nFindMeNot is also here\n"),
		Lang:    "go",
	}
	be := &referencesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes:    nil,
			services: []string{"svc"},
		},
		sourcesByService: map[string][]core.SourceFile{
			"svc": {f},
		},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2006, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": "FindMe", "word": true},
		},
	})
	p := xtraPayload(t, resp)
	occs, ok := p["occurrences"].([]any)
	if !ok {
		t.Fatalf("occurrences missing: %v", p)
	}
	// Only line 1 should match.
	if len(occs) != 1 {
		t.Errorf("word match: expected 1 occurrence (FindMe), got %d: %v", len(occs), occs)
	}
	if len(occs) == 1 {
		occ := occs[0].(map[string]any)
		if occ["line"].(float64) != 1 {
			t.Errorf("expected line 1, got %v", occ["line"])
		}
	}
}

func TestReferences_InToolsList(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2010, Method: "tools/list", Params: map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("tools/list error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)
	found := false
	for _, raw := range tools {
		if raw.(map[string]any)["name"] == "references" {
			found = true
			break
		}
	}
	if !found {
		t.Error("references tool not found in tools/list")
	}
}

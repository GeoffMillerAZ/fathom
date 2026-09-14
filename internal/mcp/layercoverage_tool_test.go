package mcp_test

// layercoverage_tool_test.go — O54 deliverable 2, end to end through the
// MCP surface: layer_coverage rides every obligations response the
// backend can report it for, and the queried file's nearest uncompiled
// guide earns a note naming the resume command.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// coverageFakeBackend adds layerCoverageBackend to semanticFakeBackend.
type coverageFakeBackend struct {
	semanticFakeBackend
	coverage      semantic.LayerCoverage
	coverageFound bool
	coverageErr   error
}

func (c *coverageFakeBackend) ObligationsLayerCoverage(_ context.Context, _ string) (semantic.LayerCoverage, bool, error) {
	return c.coverage, c.coverageFound, c.coverageErr
}

func TestObligationsTool_LayerCoverage_PresentWhenReported(t *testing.T) {
	fake := &coverageFakeBackend{
		semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "internal/extractor/goextract.go"},
		},
		coverage: semantic.LayerCoverage{
			SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true,
			UncompiledScopes: []string{"ADR.md", "internal/extractor/CLAUDE.md"},
		},
		coverageFound: true,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "internal/extractor/goextract.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	cov, ok := payload["layer_coverage"].(map[string]any)
	if !ok {
		t.Fatalf("payload[layer_coverage] missing or wrong shape: %#v", payload["layer_coverage"])
	}
	if cov["sections_total"] != float64(60) || cov["sections_compiled"] != float64(18) || cov["budget_stopped"] != true {
		t.Errorf("layer_coverage = %v, want {60 18 true ...}", cov)
	}
	wantNote := "internal/extractor/CLAUDE.md governs this path and was not compiled: budget stopped at 18/60 sections; run `fathom compile --layer obligations --continue`"
	if payload["note"] != wantNote {
		t.Errorf("payload[note] = %v, want %q", payload["note"], wantNote)
	}
}

// TestObligationsTool_LayerCoverage_NoNoteWhenFileGovernedByCompiledGuide:
// the queried file's own nearest guide already compiled (not in
// UncompiledScopes) — layer_coverage still rides the response, but no note
// is fabricated for a file whose governance IS complete.
func TestObligationsTool_LayerCoverage_NoNoteWhenFileGovernedByCompiledGuide(t *testing.T) {
	fake := &coverageFakeBackend{
		semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "cmd/fathom/compile.go"},
		},
		coverage: semantic.LayerCoverage{
			SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true,
			UncompiledScopes: []string{"ADR.md"},
		},
		coverageFound: true,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "cmd/fathom/compile.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if _, ok := payload["layer_coverage"]; !ok {
		t.Errorf("payload[layer_coverage] missing, want present (a compile_runs row exists)")
	}
	if _, ok := payload["note"]; ok {
		t.Errorf("payload[note] = %v, want absent — ADR.md's root tail is not this file's nearest guide", payload["note"])
	}
}

// TestObligationsTool_LayerCoverage_AbsentWhenBackendCannotReportIt: a
// backend that never implements layerCoverageBackend, or reports
// found:false (never compiled), carries no layer_coverage at all — an
// honest absence, never a fabricated full-coverage claim.
func TestObligationsTool_LayerCoverage_AbsentWhenBackendCannotReportIt(t *testing.T) {
	fake := &semanticFakeBackend{obligations: semantic.Obligations{Service: "svc", File: "a.go"}}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": "a.go"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	if _, ok := payload["layer_coverage"]; ok {
		t.Errorf("payload[layer_coverage] = %v, want absent", payload["layer_coverage"])
	}
}

// TestObligationsTool_LayerCoverage_RootScopeNeverGetsAFileNote: a
// root-scope call (no file) never fabricates a per-file coverage note,
// even when the layer is partially compiled.
func TestObligationsTool_LayerCoverage_RootScopeNeverGetsAFileNote(t *testing.T) {
	fake := &coverageFakeBackend{
		semanticFakeBackend: semanticFakeBackend{obligations: semantic.Obligations{Service: "svc"}},
		coverage: semantic.LayerCoverage{
			SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true,
			UncompiledScopes: []string{"ADR.md", "internal/CLAUDE.md"},
		},
		coverageFound: true,
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	payload := extractPayload(t, resp)
	wantRootNote := "Root-scope obligations only (repo-root/CLAUDE.md-level rules) — pass file for file-scoped rules."
	if payload["note"] != wantRootNote {
		t.Errorf("payload[note] = %v, want the root-scope note %q unchanged", payload["note"], wantRootNote)
	}
}

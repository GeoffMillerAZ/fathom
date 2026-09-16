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
// backend that never implements layerCoverageBackend at all (an older
// module, a minimal test fake with no ObligationsLayerCoverage method)
// carries no layer_coverage at all — an honest absence, never a fabricated
// full-coverage claim. A backend that DOES implement it but reports
// found:false (a real store with no compile_runs row for the service) is
// the distinct O60 case covered by
// TestObligationsTool_LayerCoverage_UnknownWhenNoCompileRunsRow below —
// that one now DOES carry a layer_coverage key, status "unknown", so a
// stranger reading the envelope can tell a complete layer from an
// unrecorded one instead of the two looking identical.
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

// TestObligationsTool_LayerCoverage_UnknownWhenNoCompileRunsRow (O60): the
// obligations layer compiled fine (rules serve) but the store has no
// compile_runs row for the service — semantic.ObligationsLayerCoverage's
// own honest found:false, e.g. a layer compiled before compile_runs
// tracking existed. Before this fix the envelope carried NO layer_coverage
// key at all in this case, identical to a backend that cannot report
// coverage — observed live on the operator daemon: an obligations response
// with keys compiled_at_watermark, displaced, file, layer, note, root,
// rules, service and no layer_coverage anywhere. A stranger could not tell
// a complete layer from an unrecorded one. Now the key is always present
// when the backend implements layerCoverageBackend: real numbers when
// found, {"status":"unknown", "note":…} when not.
func TestObligationsTool_LayerCoverage_UnknownWhenNoCompileRunsRow(t *testing.T) {
	fake := &coverageFakeBackend{
		semanticFakeBackend: semanticFakeBackend{
			obligations: semantic.Obligations{Service: "svc", File: "internal/extractor/goextract.go"},
		},
		coverageFound: false, // no compile_runs row — not the same as an error
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
	if cov["status"] != "unknown" {
		t.Errorf("layer_coverage[status] = %v, want %q", cov["status"], "unknown")
	}
	wantNote := "layer compiled before coverage was recorded; recompile to record coverage"
	if cov["note"] != wantNote {
		t.Errorf("layer_coverage[note] = %v, want %q", cov["note"], wantNote)
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

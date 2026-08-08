// Package mcp_test — TDD tests for the extra analyzer tools wired by
// registerExtraTools.
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

// newExtraSrv returns a Server backed by extendedFakeBackend with a fixed
// set of symbol nodes useful for testing the extra tools.
func newExtraSrv() *mcp.Server {
	return mcp.NewServer(&extendedFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "Alpha", Kind: core.KindGoFunc, FilePath: "pkg/alpha.go", Line: 10, Calls: []string{"svc::pkg.Beta"}, Complexity: 5, Signature: "func Alpha()"},
			{Service: "svc", Package: "pkg", Name: "Beta", Kind: core.KindGoFunc, FilePath: "pkg/beta.go", Line: 20, Calls: nil, Complexity: 2, Signature: "func Beta()"},
			{Service: "svc", Package: "pkg", Name: "MyStruct", Kind: core.KindGoType, FilePath: "pkg/types.go", Line: 1, Complexity: 0, Signature: "type MyStruct struct"},
			{Service: "svc", Package: "pkg", Name: "TestAlpha", Kind: core.KindGoFunc, FilePath: "pkg/alpha_test.go", Line: 1, Calls: []string{"svc::pkg.Alpha"}, Complexity: 1},
			{Service: "svc", Package: "pkg", Name: "UndocumentedFunc", Kind: core.KindGoFunc, FilePath: "pkg/x.go", Line: 1},
		},
		services: []string{"svc"},
	})
}

// xtraPayload is a test helper that extracts the JSON payload from a
// roundtrip response, failing the test on any error.
func xtraPayload(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	if resp["error"] != nil {
		t.Fatalf("unexpected RPC error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result field missing: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("result.content missing: %v", result)
	}
	text, ok := content[0].(map[string]any)["text"].(string)
	if !ok {
		t.Fatalf("content[0].text missing: %v", content)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v (text=%q)", err, text)
	}
	return payload
}

// xtraError returns the error object from a roundtrip response, failing if
// there was no error.
func xtraError(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error but got result: %v", resp)
	}
	return errObj
}

// ─── blast_radius ─────────────────────────────────────────────────────────────

func TestBlastRadius_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1001, Method: "tools/call",
		Params: map[string]any{
			"name":      "blast_radius",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/alpha.go"}},
		},
	})
	p := xtraPayload(t, resp)
	if p["symbols"] == nil {
		t.Error("blast_radius: symbols missing from payload")
	}
	if p["tests"] == nil {
		t.Error("blast_radius: tests missing from payload")
	}
}

func TestBlastRadius_MissingChangedFiles(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1002, Method: "tools/call",
		Params: map[string]any{
			"name":      "blast_radius",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error, got code %v", code)
	}
}

func TestBlastRadius_MissingService(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1003, Method: "tools/call",
		Params: map[string]any{
			"name":      "blast_radius",
			"arguments": map[string]any{"changed_files": []string{"pkg/alpha.go"}},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error, got code %v", code)
	}
}

// ─── diff_impact ──────────────────────────────────────────────────────────────

func TestDiffImpact_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1010, Method: "tools/call",
		Params: map[string]any{
			"name":      "diff_impact",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/beta.go"}},
		},
	})
	p := xtraPayload(t, resp)
	if _, ok := p["symbols"]; !ok {
		t.Error("diff_impact: symbols missing")
	}
}

// ─── spec_coverage ────────────────────────────────────────────────────────────

func TestSpecCoverage_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1020, Method: "tools/call",
		Params: map[string]any{
			"name": "spec_coverage",
			"arguments": map[string]any{
				"service":  "svc",
				"spec_ids": []string{"[US-001]", "[EP-002]"},
			},
		},
	})
	p := xtraPayload(t, resp)
	if p["coverage"] == nil {
		t.Error("spec_coverage: coverage missing from payload")
	}
	if p["uncovered"] == nil {
		t.Error("spec_coverage: uncovered missing from payload")
	}
}

// TestSpecCoverage_MissingSpecIDs — DELIBERATE behavior change (P1-8,
// docs/plans/pulled-tools-excellence.md): spec_coverage declares spec_ids
// required in its InputSchema, but its handler never enforced that — a
// spec_ids-less call used to run and return an empty coverage matrix (the
// "orphan-listing" use the C4-5 audit classified as legitimate). Dispatch-
// level required-field validation now makes the declared contract real:
// the call refuses with ErrCodeParams. The old orphan-listing behavior was
// an undocumented side effect of the handler's leniency, not a promised
// contract; a caller who wants it back should get a schema change (spec_ids
// optional, documented), not a validation hole. This test pins the
// refusal so the change stays deliberate, never incidental.
func TestSpecCoverage_MissingSpecIDs(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1021, Method: "tools/call",
		Params: map[string]any{
			"name":      "spec_coverage",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v (spec_coverage declares spec_ids required)", code, mcp.ErrCodeParams)
	}
}

// TestSpecStatus_MissingSpecIDs pins the same deliberate P1-8 change for
// spec_status (the other spec tool whose handler never enforced its
// declared spec_ids requirement — see TestSpecCoverage_MissingSpecIDs for
// the full rationale).
func TestSpecStatus_MissingSpecIDs(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1022, Method: "tools/call",
		Params: map[string]any{
			"name":      "spec_status",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v (spec_status declares spec_ids required)", code, mcp.ErrCodeParams)
	}
}

// ─── spec_status ─────────────────────────────────────────────────────────────

func TestSpecStatus_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1030, Method: "tools/call",
		Params: map[string]any{
			"name": "spec_status",
			"arguments": map[string]any{
				"service":  "svc",
				"spec_ids": []string{"[US-001]"},
			},
		},
	})
	p := xtraPayload(t, resp)
	if p["statuses"] == nil {
		t.Error("spec_status: statuses missing")
	}
}

// ─── traceability ─────────────────────────────────────────────────────────────

func TestTraceability_Basic(t *testing.T) {
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes: []core.SymbolNode{{Service: "svc", Name: "Foo", Kind: core.KindGoFunc}},
		},
		sources: []core.SourceFile{
			{Path: "pkg/x.go", Content: []byte("// [US-001] implements user story"), Lang: "go"},
		},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1040, Method: "tools/call",
		Params: map[string]any{
			"name":      "traceability",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["refs"] == nil {
		t.Error("traceability: refs missing")
	}
}

func TestTraceability_MissingService(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1041, Method: "tools/call",
		Params: map[string]any{
			"name":      "traceability",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── propagate_concern ────────────────────────────────────────────────────────

func TestPropagateConcern_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1050, Method: "tools/call",
		Params: map[string]any{
			"name":      "propagate_concern",
			"arguments": map[string]any{"service": "svc", "concern": "Alpha", "seed": "svc::pkg.Alpha"},
		},
	})
	p := xtraPayload(t, resp)
	if p["symbols"] == nil {
		t.Error("propagate_concern: symbols missing")
	}
}

func TestPropagateConcern_MissingConcern(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1051, Method: "tools/call",
		Params: map[string]any{
			"name":      "propagate_concern",
			"arguments": map[string]any{"service": "svc", "seed": "Alpha"},
		},
	})
	xtraError(t, resp)
}

// ─── evidence_summary ─────────────────────────────────────────────────────────

func TestEvidenceSummary_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1060, Method: "tools/call",
		Params: map[string]any{
			"name":      "evidence_summary",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["summary"] == nil {
		t.Error("evidence_summary: summary missing")
	}
}

// ─── contradiction_scan ───────────────────────────────────────────────────────

func TestContradictionScan_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1070, Method: "tools/call",
		Params: map[string]any{
			"name":      "contradiction_scan",
			"arguments": map[string]any{"service": "svc", "concern": "any"},
		},
	})
	p := xtraPayload(t, resp)
	if p["contradictions"] == nil {
		t.Error("contradiction_scan: contradictions missing")
	}
}

func TestContradictionScan_MissingConcern(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1071, Method: "tools/call",
		Params: map[string]any{
			"name":      "contradiction_scan",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	xtraError(t, resp)
}

// ─── arch_conformance ─────────────────────────────────────────────────────────

func TestArchConformance_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1080, Method: "tools/call",
		Params: map[string]any{
			"name": "arch_conformance",
			"arguments": map[string]any{
				"service": "svc",
				"rules": []map[string]any{
					{"from_pkg": "pkg", "to_pkg": "other", "allow": false},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	if p["violations"] == nil {
		t.Error("arch_conformance: violations missing")
	}
}

func TestArchConformance_MissingService(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1081, Method: "tools/call",
		Params: map[string]any{
			"name":      "arch_conformance",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── arch_conformance: C4-3 rule_source / checked states ──────────────────────

// TestArchConformance_RuleSource_Inline proves that when the caller passes
// inline rules, rule_source is "inline" and checked is true — the existing
// (pre-C4) contract, now made explicit.
func TestArchConformance_RuleSource_Inline(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1082, Method: "tools/call",
		Params: map[string]any{
			"name": "arch_conformance",
			"arguments": map[string]any{
				"service": "svc",
				"rules": []map[string]any{
					{"from_pkg": "pkg", "to_pkg": "other", "allow": false},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	if p["rule_source"] != "inline" {
		t.Errorf("rule_source = %v, want %q", p["rule_source"], "inline")
	}
	if p["checked"] != true {
		t.Errorf("checked = %v, want true", p["checked"])
	}
	if _, hasReason := p["reason"]; hasReason {
		t.Errorf("reason should be absent when checked, got %v", p["reason"])
	}
}

// TestArchConformance_RuleSource_Project proves that when the caller passes
// no rules but the service's corpus root has a valid
// .fathom/rules/archconformance.cue, arch_conformance loads and uses it,
// reporting rule_source "project" and checked true — and that its
// violations reflect the loaded rules, not an empty pass-through.
func TestArchConformance_RuleSource_Project(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "pkg", to_pkg: "other", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	srv := mcp.NewServer(&extendedFakeBackend{
		nodes:    extFixtureNodes(),
		services: []string{"svc"},
		svcPath:  corpusRoot,
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1083, Method: "tools/call",
		Params: map[string]any{
			"name":      "arch_conformance",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["rule_source"] != "project" {
		t.Errorf("rule_source = %v, want %q", p["rule_source"], "project")
	}
	if p["checked"] != true {
		t.Errorf("checked = %v, want true", p["checked"])
	}
	if _, hasReason := p["reason"]; hasReason {
		t.Errorf("reason should be absent when checked, got %v", p["reason"])
	}
	// extFixtureNodes has A (pkg "pkg") calling B (pkg "pkg") — same
	// package, so the loaded pkg->other deny rule does not fire; assert the
	// tool actually ran the loaded rule (count is a number, not absent).
	if _, ok := p["count"].(float64); !ok {
		t.Errorf("count missing or wrong type: %v", p["count"])
	}
}

// TestArchConformance_RuleSource_Project_CatchesViolation proves the loaded
// project rules are actually enforced against real edges, not just present
// in the response.
func TestArchConformance_RuleSource_Project_CatchesViolation(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// extFixtureNodes' A calls "svc::pkg.B" verbatim (Calls: []string{"svc::pkg.B"}).
	// Post-P1-4, archConformanceEdges resolves that already-service-
	// resolved callee to node B and rebuilds BOTH sides as declaring-
	// package keys — FromSymbol="pkg.A", ToSymbol="pkg.B" (not the raw,
	// service-prefix-contaminated "svc::pkg.B" the pre-fix edge builder
	// left unresolved, whose pkgOf-derived prefix was "svc::pkg", not a
	// real package name any rule author would write) — so a plain
	// same-package deny rule now matches this edge.
	ruleFile := `
rules: [
	{from_pkg: "pkg", to_pkg: "pkg", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	srv := mcp.NewServer(&extendedFakeBackend{
		nodes:    extFixtureNodes(),
		services: []string{"svc"},
		svcPath:  corpusRoot,
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1084, Method: "tools/call",
		Params: map[string]any{
			"name":      "arch_conformance",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	count, ok := p["count"].(float64)
	if !ok || count == 0 {
		t.Fatalf("expected at least one violation from loaded project rules, got count=%v violations=%v", p["count"], p["violations"])
	}
}

// TestArchConformance_RuleSource_None_NoInlineNoProject proves that when
// the caller passes no rules and no project rule file exists,
// arch_conformance reports rule_source "none", checked false, and a reason
// — never a clean-shaped {violations:[],count:0} indistinguishable from a
// real scan (the confirmed C4-5 honesty bug this row fixes).
func TestArchConformance_RuleSource_None_NoInlineNoProject(t *testing.T) {
	corpusRoot := t.TempDir() // no .fathom/rules/archconformance.cue written
	srv := mcp.NewServer(&extendedFakeBackend{
		nodes:    extFixtureNodes(),
		services: []string{"svc"},
		svcPath:  corpusRoot,
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1085, Method: "tools/call",
		Params: map[string]any{
			"name":      "arch_conformance",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["rule_source"] != "none" {
		t.Errorf("rule_source = %v, want %q", p["rule_source"], "none")
	}
	if p["checked"] != false {
		t.Errorf("checked = %v, want false", p["checked"])
	}
	reason, _ := p["reason"].(string)
	if reason == "" {
		t.Errorf("reason should be a non-empty explanation when checked is false, got %v", p["reason"])
	}
	if count, ok := p["count"].(float64); !ok || count != 0 {
		t.Errorf("count = %v, want 0", p["count"])
	}
}

// TestArchConformance_RuleSource_None_NoServiceMeta proves the same "none"
// honesty contract holds when the backend has no ServiceMeta path at all
// (extendedFakeBackend's default zero value) — not just when a corpus root
// exists but is empty of rules.
func TestArchConformance_RuleSource_None_NoServiceMeta(t *testing.T) {
	srv := newExtraSrv() // default extendedFakeBackend: svcPath == ""
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1086, Method: "tools/call",
		Params: map[string]any{
			"name":      "arch_conformance",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["rule_source"] != "none" {
		t.Errorf("rule_source = %v, want %q", p["rule_source"], "none")
	}
	if p["checked"] != false {
		t.Errorf("checked = %v, want false", p["checked"])
	}
}

// TestArchConformance_RelativeServiceMetaPath_NotChecked reproduces the
// critic's live exploit of the pre-fix code: a service whose ServiceMeta
// path is recorded RELATIVE (".", the pre-O10 meta class). Before the
// filepath.IsAbs guard, projectrules.Load resolved that against the
// daemon's CWD and loaded whatever repo the daemon happened to run in —
// answering rule_source:"project", checked:true with a FOREIGN rule pack
// (this very repo's own .fathom/rules/archconformance.cue, when the test
// runs from the repo tree). The guard must turn that into an honest
// rule_source:"none", checked:false + re-ingest reason — never a foreign
// "project" answer and never an RPC error.
func TestArchConformance_RelativeServiceMetaPath_NotChecked(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{
		nodes:    extFixtureNodes(),
		services: []string{"svc"},
		svcPath:  ".", // relative — must never resolve against the process CWD
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1088, Method: "tools/call",
		Params: map[string]any{
			"name":      "arch_conformance",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["rule_source"] != "none" {
		t.Errorf("rule_source = %v, want %q (a relative corpus root must never load a project pack)", p["rule_source"], "none")
	}
	if p["checked"] != false {
		t.Errorf("checked = %v, want false", p["checked"])
	}
	reason, _ := p["reason"].(string)
	if !strings.Contains(reason, "re-ingest") {
		t.Errorf("reason should tell the caller to re-ingest the service, got %q", reason)
	}
	if count, ok := p["count"].(float64); !ok || count != 0 {
		t.Errorf("count = %v, want 0", p["count"])
	}
}

// TestArchConformance_MalformedProjectRules_LoudError proves a malformed
// project rule file is a loud RPC error, never silently treated as "no
// rules" (which would be indistinguishable from an intentionally absent
// file).
func TestArchConformance_MalformedProjectRules_LoudError(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte("not valid { cue :::"), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	srv := mcp.NewServer(&extendedFakeBackend{
		nodes:    extFixtureNodes(),
		services: []string{"svc"},
		svcPath:  corpusRoot,
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1087, Method: "tools/call",
		Params: map[string]any{
			"name":      "arch_conformance",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	xtraError(t, resp)
}

// ─── arch_conformance: P1-4 method-edge normalization (closes C4-4) ──────────

// TestArchConformance_MethodEdgeNormalization_CatchesViolation proves the
// disclosed gap in schema/rules/archconformance/schema.cue #Rule is closed:
// a METHOD-origin edge whose target resolves through a receiver-VARIABLE-
// qualified call (never package-qualified in the raw Calls text) is now
// caught by a plain package-named deny rule.
//
// Fixture: package "internal" declares type Handler (Kind=GoType,
// Package="internal") with method Handler.Serve (Kind=GoMethod,
// Package="Handler" — the receiver type, per goextract's documented
// override — Calls=["h.Forbidden"], the raw form the Go extractor records
// for a call through a receiver variable typed as another package's
// Client). Package "external" declares type Client (Package="external")
// with method Client.Forbidden (Package="Client"). Pre-P1-4, the edge
// builder produced FromSymbol="Handler.Handler.Serve" (pkgOf ->
// "Handler.Handler", not a real package) and left ToSymbol="h.Forbidden"
// unresolved (pkgOf -> "h") — so a rule naming the real packages
// ("internal" -> "external") could never match. Post-P1-4,
// archConformanceEdges resolves both sides to their declaring packages.
func TestArchConformance_MethodEdgeNormalization_CatchesViolation(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "internal", Name: "Handler", Kind: core.KindGoType, FilePath: "internal/handler.go", Line: 1},
		{
			Service: "svc", Package: "Handler", Name: "Handler.Serve", Kind: core.KindGoMethod,
			FilePath: "internal/handler.go", Line: 10, Calls: []string{"h.Forbidden"},
		},
		{Service: "svc", Package: "external", Name: "Client", Kind: core.KindGoType, FilePath: "external/client.go", Line: 1},
		{
			Service: "svc", Package: "Client", Name: "Client.Forbidden", Kind: core.KindGoMethod,
			FilePath: "external/client.go", Line: 5,
		},
	}
	srv := mcp.NewServer(&extendedFakeBackend{nodes: nodes, services: []string{"svc"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1088, Method: "tools/call",
		Params: map[string]any{
			"name": "arch_conformance",
			"arguments": map[string]any{
				"service": "svc",
				"rules": []map[string]any{
					{"from_pkg": "internal", "to_pkg": "external", "allow": false},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	count, _ := p["count"].(float64)
	if count == 0 {
		t.Fatalf("expected the method-origin edge (internal -> external) to be caught, got count=%v violations=%v", p["count"], p["violations"])
	}
	violations, _ := p["violations"].([]any)
	found := false
	for _, raw := range violations {
		v, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if v["from"] == "internal.Serve" && v["to"] == "external.Forbidden" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a violation from=%q to=%q, got %v", "internal.Serve", "external.Forbidden", violations)
	}
}

// TestArchConformance_MethodEdgeNormalization_UnresolvedCalleeStaysRaw
// proves an external/unresolvable callee (never a node in this service) is
// left exactly as before — no regression for edges that already couldn't
// be resolved to a declaring package.
func TestArchConformance_MethodEdgeNormalization_UnresolvedCalleeStaysRaw(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "internal", Name: "Handler", Kind: core.KindGoType, FilePath: "internal/handler.go", Line: 1},
		{
			Service: "svc", Package: "Handler", Name: "Handler.Serve", Kind: core.KindGoMethod,
			FilePath: "internal/handler.go", Line: 10, Calls: []string{"fmt.Println"},
		},
	}
	srv := mcp.NewServer(&extendedFakeBackend{nodes: nodes, services: []string{"svc"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1089, Method: "tools/call",
		Params: map[string]any{
			"name": "arch_conformance",
			"arguments": map[string]any{
				"service": "svc",
				"rules": []map[string]any{
					{"from_pkg": "internal", "to_pkg": "fmt", "allow": false},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	violations, _ := p["violations"].([]any)
	found := false
	for _, raw := range violations {
		v, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if v["from"] == "internal.Serve" && v["to"] == "fmt.Println" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the unresolved stdlib callee fmt.Println (raw, unchanged) to match to_pkg %q, got %v", "fmt", violations)
	}
}

// ─── doc_coverage ─────────────────────────────────────────────────────────────

func TestDocCoverage_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1090, Method: "tools/call",
		Params: map[string]any{
			"name":      "doc_coverage",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["pct"] == nil {
		t.Error("doc_coverage: pct missing")
	}
}

// ─── stale_docs ───────────────────────────────────────────────────────────────

func TestStaleDocs_Basic(t *testing.T) {
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "Foo"}}},
		sources: []core.SourceFile{
			{Path: "docs/README.md", Content: []byte("See `NonExistentSymbol` for details."), Lang: "md"},
		},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1100, Method: "tools/call",
		Params: map[string]any{
			"name":      "stale_docs",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["drifts"] == nil {
		t.Error("stale_docs: drifts missing")
	}
	if p["count"] == nil {
		t.Error("stale_docs: count missing")
	}
}

// ─── naming ───────────────────────────────────────────────────────────────────

func TestNaming_Basic(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "bad_name", Kind: core.KindGoFunc},
		},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1110, Method: "tools/call",
		Params: map[string]any{
			"name":      "naming",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["issues"] == nil {
		t.Error("naming: issues missing")
	}
}

// ─── struct_tags ──────────────────────────────────────────────────────────────

func TestStructTags_Basic(t *testing.T) {
	src := `package pkg
type Req struct {
    Name string
}`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "Req"}}},
		sources:             []core.SourceFile{{Path: "pkg/req.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1120, Method: "tools/call",
		Params: map[string]any{
			"name":      "struct_tags",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["issues"] == nil {
		t.Error("struct_tags: issues missing")
	}
}

// ─── err_audit ────────────────────────────────────────────────────────────────

func TestErrAudit_Basic(t *testing.T) {
	src := `package pkg
import "errors"
func F() error {
    err := errors.New("oops")
    _ = err
    return nil
}`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "F"}}},
		sources:             []core.SourceFile{{Path: "pkg/f.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1130, Method: "tools/call",
		Params: map[string]any{
			"name":      "err_audit",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["findings"] == nil {
		t.Error("err_audit: findings missing")
	}
}

// ─── sql_map ──────────────────────────────────────────────────────────────────

func TestSQLMap_Basic(t *testing.T) {
	src := `SELECT * FROM users WHERE id = $1`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "F"}}},
		sources:             []core.SourceFile{{Path: "pkg/q.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1140, Method: "tools/call",
		Params: map[string]any{
			"name":      "sql_map",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["tables"] == nil {
		t.Error("sql_map: tables missing")
	}
}

// ─── field_usage ──────────────────────────────────────────────────────────────

func TestFieldUsage_Basic(t *testing.T) {
	src := `package pkg
type MyStruct struct {
    Foo string
    Bar int
}
func Use(m MyStruct) string {
    return m.Foo
}`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "MyStruct", Kind: core.KindGoType}}},
		sources:             []core.SourceFile{{Path: "pkg/my.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1150, Method: "tools/call",
		Params: map[string]any{
			"name":      "field_usage",
			"arguments": map[string]any{"service": "svc", "struct_name": "MyStruct"},
		},
	})
	p := xtraPayload(t, resp)
	if p["fields"] == nil {
		t.Error("field_usage: fields missing")
	}
}

func TestFieldUsage_MissingStructName(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1151, Method: "tools/call",
		Params: map[string]any{
			"name":      "field_usage",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	xtraError(t, resp)
}

// ─── clones ───────────────────────────────────────────────────────────────────

func TestClones_Basic(t *testing.T) {
	dup := `package pkg
func dup() int { return 42 }`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "dup"}}},
		sources: []core.SourceFile{
			{Path: "pkg/a.go", Content: []byte(dup), Lang: "go"},
			{Path: "pkg/b.go", Content: []byte(dup), Lang: "go"},
		},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1160, Method: "tools/call",
		Params: map[string]any{
			"name":      "clones",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["groups"] == nil {
		t.Error("clones: groups missing")
	}
}

// ─── migration_targets ────────────────────────────────────────────────────────

func TestMigrationTargets_Basic(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "io/ioutil", Name: "ReadFile", Kind: core.KindGoFunc},
		},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1170, Method: "tools/call",
		Params: map[string]any{
			"name":      "migration_targets",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["targets"] == nil {
		t.Error("migration_targets: targets missing")
	}
}

// ─── get_go_symbols ───────────────────────────────────────────────────────────

func TestGetGoSymbols_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1180, Method: "tools/call",
		Params: map[string]any{
			"name":      "get_go_symbols",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["symbols"] == nil {
		t.Error("get_go_symbols: symbols missing")
	}
}

func TestGetGoSymbols_KindFilter(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1181, Method: "tools/call",
		Params: map[string]any{
			"name":      "get_go_symbols",
			"arguments": map[string]any{"service": "svc", "kinds": []string{"GoType"}},
		},
	})
	p := xtraPayload(t, resp)
	syms, ok := p["symbols"].([]any)
	if !ok {
		t.Fatal("symbols not an array")
	}
	for _, s := range syms {
		sm := s.(map[string]any)
		if sm["kind"] != "GoType" {
			t.Errorf("got non-GoType symbol with kind=%v", sm["kind"])
		}
	}
}

// ─── api_diff ─────────────────────────────────────────────────────────────────

func TestAPIDiff_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1190, Method: "tools/call",
		Params: map[string]any{
			"name":      "api_diff",
			"arguments": map[string]any{"service_a": "svc", "service_b": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["added"] == nil || p["removed"] == nil || p["changed"] == nil {
		t.Error("api_diff: missing added/removed/changed fields")
	}
}

func TestAPIDiff_MissingServiceB(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1191, Method: "tools/call",
		Params: map[string]any{
			"name":      "api_diff",
			"arguments": map[string]any{"service_a": "svc"},
		},
	})
	xtraError(t, resp)
}

// ─── dep_drift ────────────────────────────────────────────────────────────────

func TestDepDrift_Basic(t *testing.T) {
	gomod := `module example.com
go 1.21
require github.com/foo/bar v1.2.3
`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svcA", "svcB"}},
		sources: []core.SourceFile{
			{Path: "go.mod", Content: []byte(gomod), Lang: "go"},
		},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1200, Method: "tools/call",
		Params: map[string]any{
			"name":      "dep_drift",
			"arguments": map[string]any{"service_a": "svcA", "service_b": "svcB"},
		},
	})
	p := xtraPayload(t, resp)
	if p["drift"] == nil {
		t.Error("dep_drift: drift missing")
	}
}

func TestDepDrift_MissingServiceB(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1201, Method: "tools/call",
		Params: map[string]any{
			"name":      "dep_drift",
			"arguments": map[string]any{"service_a": "svcA"},
		},
	})
	xtraError(t, resp)
}

// ─── package_deps ─────────────────────────────────────────────────────────────

func TestPackageDeps_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1210, Method: "tools/call",
		Params: map[string]any{
			"name":      "package_deps",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["package_deps"] == nil {
		t.Error("package_deps: package_deps missing")
	}
}

// ─── hotspots ─────────────────────────────────────────────────────────────────

func TestHotspots_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1220, Method: "tools/call",
		Params: map[string]any{
			"name":      "hotspots",
			"arguments": map[string]any{"service": "svc", "top_n": 3},
		},
	})
	p := xtraPayload(t, resp)
	if p["hotspots"] == nil {
		t.Error("hotspots: hotspots missing")
	}
	hs, ok := p["hotspots"].([]any)
	if !ok {
		t.Fatal("hotspots is not array")
	}
	if len(hs) > 3 {
		t.Errorf("expected at most 3 hotspots, got %d", len(hs))
	}
}

func TestHotspots_MissingService(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1221, Method: "tools/call",
		Params: map[string]any{
			"name":      "hotspots",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── license ──────────────────────────────────────────────────────────────────

func TestLicense_MIT(t *testing.T) {
	mitText := `MIT License

Copyright (c) 2024 Example

Permission is hereby granted, free of charge, to any person obtaining a copy...`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{},
		sources:             []core.SourceFile{{Path: "LICENSE", Content: []byte(mitText), Lang: ""}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1230, Method: "tools/call",
		Params: map[string]any{
			"name":      "license",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["spdx"] == nil {
		t.Error("license: spdx missing")
	}
}

func TestLicense_NoFile(t *testing.T) {
	srv := newExtraSrv() // no LICENSE file in extendedFakeBackend (Sources returns nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1231, Method: "tools/call",
		Params: map[string]any{
			"name":      "license",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["spdx"] != "UNKNOWN" {
		t.Errorf("expected UNKNOWN, got %v", p["spdx"])
	}
}

// ─── license_policy ───────────────────────────────────────────────────────────

func TestLicensePolicy_Allowed(t *testing.T) {
	mitText := `MIT License
Permission is hereby granted`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{},
		sources:             []core.SourceFile{{Path: "LICENSE", Content: []byte(mitText), Lang: ""}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1240, Method: "tools/call",
		Params: map[string]any{
			"name":      "license_policy",
			"arguments": map[string]any{"service": "svc", "allowed": []string{"MIT", "Apache-2.0"}},
		},
	})
	p := xtraPayload(t, resp)
	if p["ok"] == nil {
		t.Error("license_policy: ok missing")
	}
}

// TestLicensePolicy_MissingAllowed: license_policy declares "allowed"
// required in its InputSchema but never checked it itself (C4-5's honesty
// audit) — a call omitting it used to run anyway (a "previously-lenient
// caller", P1-8's dispatch-level required-field gate; see
// TestDispatch_PreviouslyLenientTool_NowRefuses for the same case asserted
// directly against dispatch). Deliberately tightened here rather than left
// straddling both possible outcomes: it must now ALWAYS be an
// ErrCodeParams, never a lenient {ok:false} result.
func TestLicensePolicy_MissingAllowed(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1241, Method: "tools/call",
		Params: map[string]any{
			"name":      "license_policy",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── security_rollup ──────────────────────────────────────────────────────────

func TestSecurityRollup_Basic(t *testing.T) {
	src := `package pkg
const token = "ghp_abc123faketoken"
`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{},
		sources:             []core.SourceFile{{Path: "pkg/creds.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1250, Method: "tools/call",
		Params: map[string]any{
			"name":      "security_rollup",
			"arguments": map[string]any{"service": "svc", "allowed_licenses": []string{"MIT"}},
		},
	})
	p := xtraPayload(t, resp)
	if p["secrets"] == nil {
		t.Error("security_rollup: secrets missing")
	}
	if p["license"] == nil {
		t.Error("security_rollup: license missing")
	}
}

// ─── ingest ───────────────────────────────────────────────────────────────────

func TestIngest_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1260, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["ok"] != true {
		t.Errorf("ingest: expected ok=true, got %v", p)
	}
}

func TestIngest_MissingService(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1261, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── cross_repo_blast_radius ──────────────────────────────────────────────────

func TestCrossRepoBlastRadius_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1270, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_blast_radius",
			"arguments": map[string]any{"symbol": "Alpha"},
		},
	})
	p := xtraPayload(t, resp)
	if p["hits"] == nil {
		t.Error("cross_repo_blast_radius: hits missing")
	}
}

func TestCrossRepoBlastRadius_MissingSymbol(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1271, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_blast_radius",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── repo_map ─────────────────────────────────────────────────────────────────

func TestRepoMap_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1280, Method: "tools/call",
		Params: map[string]any{
			"name":      "repo_map",
			"arguments": map[string]any{"service": "svc", "budget": 500},
		},
	})
	p := xtraPayload(t, resp)
	if p["map"] == nil {
		t.Error("repo_map: map missing")
	}
}

// ─── pack_context ─────────────────────────────────────────────────────────────

func TestPackContext_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1290, Method: "tools/call",
		Params: map[string]any{
			"name":      "pack_context",
			"arguments": map[string]any{"service": "svc", "concern": "any", "budget": 500},
		},
	})
	p := xtraPayload(t, resp)
	if p["included"] == nil {
		t.Error("pack_context: included missing")
	}
}

// ─── estimate_cost ────────────────────────────────────────────────────────────

func TestEstimateCost_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1300, Method: "tools/call",
		Params: map[string]any{
			"name":      "estimate_cost",
			"arguments": map[string]any{"text": "hello world this is a test"},
		},
	})
	p := xtraPayload(t, resp)
	if p["tokens"] == nil {
		t.Error("estimate_cost: tokens missing")
	}
	if p["usd"] == nil {
		t.Error("estimate_cost: usd missing")
	}
}

func TestEstimateCost_MissingText(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1301, Method: "tools/call",
		Params: map[string]any{
			"name":      "estimate_cost",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── cost_report ──────────────────────────────────────────────────────────────

func TestCostReport_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1310, Method: "tools/call",
		Params: map[string]any{
			"name":      "cost_report",
			"arguments": map[string]any{},
		},
	})
	p := xtraPayload(t, resp)
	if p["tools"] == nil {
		t.Error("cost_report: tools missing")
	}
	if p["total_tokens"] == nil {
		t.Error("cost_report: total_tokens missing")
	}
}

// ─── batch ────────────────────────────────────────────────────────────────────

func TestBatch_Basic(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1320, Method: "tools/call",
		Params: map[string]any{
			"name": "batch",
			"arguments": map[string]any{
				"calls": []map[string]any{
					{"name": "hotspots", "arguments": map[string]any{"service": "svc", "top_n": 5}},
					{"name": "doc_coverage", "arguments": map[string]any{"service": "svc"}},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	results, ok := p["results"].([]any)
	if !ok {
		t.Fatalf("batch: results missing or not array")
	}
	if len(results) != 2 {
		t.Errorf("batch: expected 2 results, got %d", len(results))
	}
}

func TestBatch_UnknownTool(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1321, Method: "tools/call",
		Params: map[string]any{
			"name": "batch",
			"arguments": map[string]any{
				"calls": []map[string]any{
					{"name": "no_such_tool", "arguments": map[string]any{}},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	results := p["results"].([]any)
	r := results[0].(map[string]any)
	if r["error"] == nil {
		t.Error("batch: expected per-call error for unknown tool")
	}
}

func TestBatch_MissingCalls(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1322, Method: "tools/call",
		Params: map[string]any{
			"name":      "batch",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── proto_contract ───────────────────────────────────────────────────────────

func TestProtoContract_Basic(t *testing.T) {
	protoSrc := `syntax = "proto3";
package hello;
service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply);
}
message HelloRequest { string name = 1; }
message HelloReply { string message = 1; }
`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{},
		sources:             []core.SourceFile{{Path: "proto/hello.proto", Content: []byte(protoSrc), Lang: "proto"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1330, Method: "tools/call",
		Params: map[string]any{
			"name":      "proto_contract",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	contracts, ok := p["contracts"].([]any)
	if !ok {
		t.Fatalf("proto_contract: contracts missing or not array")
	}
	if len(contracts) == 0 {
		t.Error("proto_contract: expected at least one contract")
	}
}

// ─── proto_conformance ────────────────────────────────────────────────────────

func TestProtoConformance_Basic(t *testing.T) {
	protoSrc := `syntax = "proto3";
package hello;
service greeter {
  rpc sayHello (hello_request) returns (hello_reply);
}
message hello_request { string Name = 1; }
message hello_reply { string Message = 2; }
`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{},
		sources:             []core.SourceFile{{Path: "proto/bad.proto", Content: []byte(protoSrc), Lang: "proto"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1340, Method: "tools/call",
		Params: map[string]any{
			"name":      "proto_conformance",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	if p["issues"] == nil {
		t.Error("proto_conformance: issues missing")
	}
}

// ─── schema_export ────────────────────────────────────────────────────────────

func TestSchemaExport_JSON(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1350, Method: "tools/call",
		Params: map[string]any{
			"name":      "schema_export",
			"arguments": map[string]any{"service": "svc", "format": "json"},
		},
	})
	p := xtraPayload(t, resp)
	if p["output"] == nil {
		t.Error("schema_export(json): output missing")
	}
	// Verify output is valid JSON.
	out, ok := p["output"].(string)
	if !ok {
		t.Fatal("output is not string")
	}
	var js any
	if err := json.Unmarshal([]byte(out), &js); err != nil {
		t.Errorf("schema_export JSON output is not valid JSON: %v", err)
	}
}

func TestSchemaExport_DOT(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1351, Method: "tools/call",
		Params: map[string]any{
			"name":      "schema_export",
			"arguments": map[string]any{"service": "svc", "format": "dot"},
		},
	})
	p := xtraPayload(t, resp)
	out, ok := p["output"].(string)
	if !ok {
		t.Fatal("schema_export(dot): output not string")
	}
	if !containsStr(out, "digraph") {
		t.Errorf("DOT output does not contain 'digraph': %q", out)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ─── tools list coverage ──────────────────────────────────────────────────────

// TestExtraToolsInList verifies that a representative set of the new tools
// appears in tools/list when no profile filter is active.
func TestExtraToolsInList(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 9000, Method: "tools/list", Params: map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("tools/list error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	toolsList := result["tools"].([]any)
	names := make(map[string]bool, len(toolsList))
	for _, raw := range toolsList {
		names[raw.(map[string]any)["name"].(string)] = true
	}

	want := []string{
		"blast_radius", "diff_impact", "spec_coverage", "spec_status",
		"traceability", "propagate_concern", "evidence_summary", "contradiction_scan",
		"arch_conformance", "doc_coverage", "stale_docs", "naming",
		"struct_tags", "err_audit", "sql_map", "field_usage",
		"clones", "migration_targets", "get_go_symbols", "api_diff",
		"dep_drift", "package_deps", "hotspots",
		"license", "license_policy", "security_rollup", "ingest",
		"cross_repo_blast_radius",
		"repo_map", "pack_context",
		"estimate_cost", "cost_report", "batch",
		"proto_contract", "proto_conformance",
		"schema_export",
		"spec_refs", "brief",
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("extra tool %q not found in tools/list", name)
		}
	}
}

// ─── spec_refs ────────────────────────────────────────────────────────────────

func TestSpecRefs_Basic(t *testing.T) {
	src := `// implements [US-042] and M18.2
// see also doc 31
func Handler() {}
`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "Handler"}}},
		sources:             []core.SourceFile{{Path: "pkg/handler.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1400, Method: "tools/call",
		Params: map[string]any{
			"name":      "spec_refs",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := xtraPayload(t, resp)
	specs, ok := p["specs"].(map[string]any)
	if !ok {
		t.Fatalf("spec_refs: specs missing or wrong type: %v", p)
	}
	if specs["US-042"] == nil {
		t.Error("spec_refs: expected US-042 in specs")
	}
	if specs["M18.2"] == nil {
		t.Error("spec_refs: expected M18.2 in specs")
	}
	if specs["doc 31"] == nil {
		t.Error("spec_refs: expected 'doc 31' in specs")
	}
	if p["total_refs"] == nil {
		t.Error("spec_refs: total_refs missing")
	}
	if p["files_scanned"] == nil {
		t.Error("spec_refs: files_scanned missing")
	}
}

func TestSpecRefs_SpecFilter(t *testing.T) {
	src := `// [US-042] is here
// [US-099] is also here
`
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: []core.SymbolNode{{Service: "svc", Name: "X"}}},
		sources:             []core.SourceFile{{Path: "pkg/x.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1401, Method: "tools/call",
		Params: map[string]any{
			"name":      "spec_refs",
			"arguments": map[string]any{"service": "svc", "spec": "US-042"},
		},
	})
	p := xtraPayload(t, resp)
	specs, ok := p["specs"].(map[string]any)
	if !ok {
		t.Fatalf("spec_refs: specs missing: %v", p)
	}
	if specs["US-042"] == nil {
		t.Error("spec_refs: filtered US-042 missing from result")
	}
	if specs["US-099"] != nil {
		t.Error("spec_refs: US-099 must be filtered out")
	}
}

func TestSpecRefs_MissingService(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1402, Method: "tools/call",
		Params: map[string]any{
			"name":      "spec_refs",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

func TestSpecRefs_InvalidPattern(t *testing.T) {
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{},
		sources:             []core.SourceFile{{Path: "f.go", Content: []byte("x"), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1403, Method: "tools/call",
		Params: map[string]any{
			"name":      "spec_refs",
			"arguments": map[string]any{"service": "svc", "pattern": "[invalid("},
		},
	})
	errObj := xtraError(t, resp)
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("expected params error, got code %v", code)
	}
}

func TestSpecRefs_LineNumbers(t *testing.T) {
	src := "line1\n[US-001]\nline3\n[US-001]\n"
	srv := mcp.NewServer(&sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{},
		sources:             []core.SourceFile{{Path: "f.go", Content: []byte(src), Lang: "go"}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1404, Method: "tools/call",
		Params: map[string]any{
			"name":      "spec_refs",
			"arguments": map[string]any{"service": "svc", "spec": "US-001"},
		},
	})
	p := xtraPayload(t, resp)
	specs := p["specs"].(map[string]any)
	hits, ok := specs["US-001"].([]any)
	if !ok {
		t.Fatalf("US-001 hits missing: %v", specs)
	}
	if len(hits) != 2 {
		t.Errorf("expected 2 hits for US-001, got %d", len(hits))
	}
	lines := []float64{}
	for _, h := range hits {
		hm := h.(map[string]any)
		lines = append(lines, hm["line"].(float64))
	}
	if lines[0] != 2 || lines[1] != 4 {
		t.Errorf("expected lines [2,4], got %v", lines)
	}
}

// ─── brief ────────────────────────────────────────────────────────────────────

// briefFakeBackend is a sourcesFakeBackend that also handles FindSymbol and
// QuerySymbols for the brief tool tests.
type briefFakeBackend struct {
	sourcesFakeBackend
	symbolMap   map[string][]core.SymbolNode // name → nodes
	queryResult core.QueryResult

	// obligationsByFile is D1's (docs/plans/expert-depth.md) claims-attach
	// fixture: keyed by the exact FilePath a resolved symbol/concern node
	// carries, so a test can control which claims match which path without
	// reimplementing scope-glob matching (that's obligationsBackend.
	// Obligations' job — brief only reuses it).
	obligationsByFile map[string]semantic.Obligations
}

func (b *briefFakeBackend) FindSymbol(_ context.Context, _ string, name string) ([]core.SymbolNode, error) {
	if nodes, ok := b.symbolMap[name]; ok {
		return nodes, nil
	}
	return nil, nil
}

func (b *briefFakeBackend) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return b.queryResult, nil
}

// Obligations implements obligationsBackend (D1): brief reuses the exact
// same served-claim shape (Rule/Src/Tier/Status) the obligations tool
// serves, so a fake keyed only by file path is enough — no new matching
// logic to fake.
func (b *briefFakeBackend) Obligations(_ context.Context, _, file string, _ int) (semantic.Obligations, error) {
	return b.obligationsByFile[file], nil
}

func (b *briefFakeBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool {
	return false
}

var _ mcp.QueryInterface = (*briefFakeBackend)(nil)

func TestBrief_SymbolsAndConcern(t *testing.T) {
	be := &briefFakeBackend{
		sourcesFakeBackend: sourcesFakeBackend{
			extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		},
		symbolMap: map[string][]core.SymbolNode{
			"Handler": {{Name: "Handler", Kind: core.KindGoFunc, FilePath: "pkg/h.go", Line: 10, Signature: "func Handler()"}},
		},
		queryResult: core.QueryResult{
			Nodes: []core.SymbolNode{
				{Name: "Middleware", Kind: core.KindGoFunc, FilePath: "pkg/mw.go", Line: 5, Signature: "func Middleware()"},
			},
		},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1500, Method: "tools/call",
		Params: map[string]any{
			"name": "brief",
			"arguments": map[string]any{
				"service": "svc",
				"symbols": []string{"Handler"},
				"concern": "auth",
			},
		},
	})
	p := xtraPayload(t, resp)
	brief, ok := p["brief"].(string)
	if !ok {
		t.Fatalf("brief: field missing: %v", p)
	}
	if !strings.Contains(brief, "Handler") {
		t.Error("brief: expected Handler in brief text")
	}
	if !strings.Contains(brief, "Middleware") {
		t.Error("brief: expected concern hit Middleware in brief text")
	}
}

// ─── D1: brief attaches governed semantic claims ───────────────────────────
// (docs/plans/expert-depth.md D1-1/D1-2)

// TestBrief_AttachesFreshClaims: a fresh claim scoped to a resolved symbol's
// file is appended verbatim (citation + tier), with no state label (fresh is
// the unflagged default), and claims_attached is set on the result.
func TestBrief_AttachesFreshClaims(t *testing.T) {
	be := &briefFakeBackend{
		sourcesFakeBackend: sourcesFakeBackend{
			extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		},
		symbolMap: map[string][]core.SymbolNode{
			"Handler": {{Name: "Handler", Kind: core.KindGoFunc, FilePath: "pkg/h.go", Line: 10, Signature: "func Handler()"}},
		},
		obligationsByFile: map[string]semantic.Obligations{
			"pkg/h.go": {
				Service: "svc", File: "pkg/h.go",
				Rules: []semantic.Obligation{
					{Rule: "Sort all exported slice results.", Src: "CLAUDE.md#L12-L12", Tier: "doc-claimed"},
				},
			},
		},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1510, Method: "tools/call",
		Params: map[string]any{
			"name":      "brief",
			"arguments": map[string]any{"service": "svc", "symbols": []string{"Handler"}},
		},
	})
	p := xtraPayload(t, resp)
	brief, ok := p["brief"].(string)
	if !ok {
		t.Fatalf("brief: field missing: %v", p)
	}
	if !strings.Contains(brief, "Sort all exported slice results.") {
		t.Errorf("brief: expected the fresh claim's rule text, got: %q", brief)
	}
	if !strings.Contains(brief, "CLAUDE.md#L12-L12") {
		t.Errorf("brief: expected the claim's citation, got: %q", brief)
	}
	if !strings.Contains(brief, "doc-claimed") {
		t.Errorf("brief: expected the claim's tier, got: %q", brief)
	}
	if strings.Contains(brief, "[stale]") || strings.Contains(brief, "[contradicted]") {
		t.Errorf("brief: a fresh claim must not carry a state label, got: %q", brief)
	}
	if n, ok := p["claims_attached"].(float64); !ok || n != 1 {
		t.Errorf("claims_attached = %v, want 1", p["claims_attached"])
	}
}

// TestBrief_LabelsStaleClaims: a non-fresh claim is still served, but its
// state must be visibly labeled — never a flagged claim served silently.
func TestBrief_LabelsStaleClaims(t *testing.T) {
	be := &briefFakeBackend{
		sourcesFakeBackend: sourcesFakeBackend{
			extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		},
		symbolMap: map[string][]core.SymbolNode{
			"Handler": {{Name: "Handler", Kind: core.KindGoFunc, FilePath: "pkg/h.go", Line: 10, Signature: "func Handler()"}},
		},
		obligationsByFile: map[string]semantic.Obligations{
			"pkg/h.go": {
				Service: "svc", File: "pkg/h.go",
				Rules: []semantic.Obligation{
					{Rule: "Run tests before committing.", Src: "CLAUDE.md#L8-L8", Tier: "doc-claimed", Status: "stale"},
				},
			},
		},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1511, Method: "tools/call",
		Params: map[string]any{
			"name":      "brief",
			"arguments": map[string]any{"service": "svc", "symbols": []string{"Handler"}},
		},
	})
	p := xtraPayload(t, resp)
	brief, ok := p["brief"].(string)
	if !ok {
		t.Fatalf("brief: field missing: %v", p)
	}
	if !strings.Contains(brief, "Run tests before committing.") {
		t.Errorf("brief: expected the stale claim's rule text, got: %q", brief)
	}
	if !strings.Contains(brief, "[stale]") {
		t.Errorf("brief: expected the stale claim's state labeled, got: %q", brief)
	}
	if n, ok := p["claims_attached"].(float64); !ok || n != 1 {
		t.Errorf("claims_attached = %v, want 1", p["claims_attached"])
	}
}

// briefClaimsBackend builds the standard D1 fixture: one resolvable symbol
// in pkg/h.go plus one governed claim scoped to it (rule text injectable so
// the truncation case can blow the char budget).
func briefClaimsBackend(rule string) *briefFakeBackend {
	return &briefFakeBackend{
		sourcesFakeBackend: sourcesFakeBackend{
			extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		},
		symbolMap: map[string][]core.SymbolNode{
			"Handler": {{Name: "Handler", Kind: core.KindGoFunc, FilePath: "pkg/h.go", Line: 10, Signature: "func Handler()"}},
		},
		obligationsByFile: map[string]semantic.Obligations{
			"pkg/h.go": {
				Service: "svc", File: "pkg/h.go",
				Rules: []semantic.Obligation{
					{Rule: rule, Src: "CLAUDE.md#L12-L12", Tier: "doc-claimed"},
				},
			},
		},
	}
}

// callLogEntryFor parses the JSONL buffer and returns the first entry whose
// tool and via match. Fails the test when absent.
func callLogEntryFor(t *testing.T, buf *bytes.Buffer, tool, via string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("call-log line not JSON: %v (line=%q)", err, line)
		}
		gotVia, _ := entry["via"].(string)
		if entry["tool"] == tool && gotVia == via {
			return entry
		}
	}
	t.Fatalf("no call-log entry for tool=%q via=%q in: %s", tool, via, buf.String())
	return nil
}

// TestBrief_ClaimsAttachedReachesCallLog is D1-2's real evidence
// (docs/plans/expert-depth.md): the claims-block hit-rate instrument is the
// JSONL call log, not the tool result — so the assertion must be on the
// LOGGED row. Covers all three write paths: the top-level dispatch
// (loggedToolsCall, which reads the field back out of the wrapped MCP
// content envelope), the batch sub-call path (logSub, which reads the raw
// handler result map), and the truncated-brief path (whose result shape
// used to drop the field, logging a claims-serving truncated brief as
// claims_attached:0 and silently poisoning D1's 30-day kill clock).
func TestBrief_ClaimsAttachedReachesCallLog(t *testing.T) {
	briefArgs := map[string]any{"service": "svc", "symbols": []string{"Handler"}}

	t.Run("top_level", func(t *testing.T) {
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(briefClaimsBackend("Sort all exported slice results."), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1520, Method: "tools/call",
			Params: map[string]any{"name": "brief", "arguments": briefArgs},
		})
		if _, ok := xtraPayload(t, resp)["claims_attached"].(float64); !ok {
			t.Fatal("result payload missing claims_attached (fixture broken?)")
		}
		entry := callLogEntryFor(t, &buf, "brief", "")
		if entry["claims_attached"] != float64(1) {
			t.Errorf("logged claims_attached = %v, want 1 (entry: %v)", entry["claims_attached"], entry)
		}
	})

	t.Run("via_batch", func(t *testing.T) {
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(briefClaimsBackend("Sort all exported slice results."), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1521, Method: "tools/call",
			Params: map[string]any{
				"name": "batch",
				"arguments": map[string]any{
					"calls": []map[string]any{{"name": "brief", "arguments": briefArgs}},
				},
			},
		})
		if resp["error"] != nil {
			t.Fatalf("unexpected error: %v", resp["error"])
		}
		entry := callLogEntryFor(t, &buf, "brief", "batch")
		if entry["claims_attached"] != float64(1) {
			t.Errorf("batch sub-call logged claims_attached = %v, want 1 (entry: %v)", entry["claims_attached"], entry)
		}
	})

	t.Run("truncated", func(t *testing.T) {
		// A 500-char rule blows the budget INSIDE the claims section: the
		// header + symbol line (~130 bytes) fit under charBudget (=budget*4
		// = 200), the claims block does not — exercising the truncation
		// return specifically, not an earlier-section truncation where zero
		// claims were genuinely attached.
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(briefClaimsBackend(strings.Repeat("x", 500)), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1522, Method: "tools/call",
			Params: map[string]any{
				"name":      "brief",
				"arguments": map[string]any{"service": "svc", "symbols": []string{"Handler"}, "budget": 50},
			},
		})
		p := xtraPayload(t, resp)
		brief, _ := p["brief"].(string)
		if !strings.Contains(brief, "truncated") {
			t.Fatalf("expected a truncated brief (fixture sizing broke), got: %q", brief)
		}
		entry := callLogEntryFor(t, &buf, "brief", "")
		if entry["claims_attached"] != float64(1) {
			t.Errorf("truncated brief logged claims_attached = %v, want 1 (entry: %v)", entry["claims_attached"], entry)
		}
	})
}

// TestBrief_NoClaims_Unchanged: a backend that serves obligations but has no
// claims scoped to the resolved paths must leave the brief output IDENTICAL
// to the pre-D1 shape — no empty "Governing claims" section, no
// claims_attached field.
func TestBrief_NoClaims_Unchanged(t *testing.T) {
	be := &briefFakeBackend{
		sourcesFakeBackend: sourcesFakeBackend{
			extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		},
		symbolMap: map[string][]core.SymbolNode{
			"Handler": {{Name: "Handler", Kind: core.KindGoFunc, FilePath: "pkg/h.go", Line: 10, Signature: "func Handler()"}},
		},
		// obligationsByFile intentionally nil/empty: no claims for pkg/h.go.
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1512, Method: "tools/call",
		Params: map[string]any{
			"name":      "brief",
			"arguments": map[string]any{"service": "svc", "symbols": []string{"Handler"}},
		},
	})
	p := xtraPayload(t, resp)
	brief, ok := p["brief"].(string)
	if !ok {
		t.Fatalf("brief: field missing: %v", p)
	}
	if strings.Contains(brief, "Governing claims") {
		t.Errorf("brief: expected no claims section when nothing matches, got: %q", brief)
	}
	if _, present := p["claims_attached"]; present {
		t.Errorf("claims_attached should be absent when zero claims attached, got: %v", p["claims_attached"])
	}
}

func TestBrief_NotFoundSymbol(t *testing.T) {
	be := &briefFakeBackend{
		sourcesFakeBackend: sourcesFakeBackend{
			extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		},
		symbolMap: map[string][]core.SymbolNode{},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1501, Method: "tools/call",
		Params: map[string]any{
			"name": "brief",
			"arguments": map[string]any{
				"service": "svc",
				"symbols": []string{"MissingSymbol"},
			},
		},
	})
	p := xtraPayload(t, resp)
	brief, ok := p["brief"].(string)
	if !ok {
		t.Fatalf("brief: field missing: %v", p)
	}
	if !strings.Contains(brief, "NOT FOUND") {
		t.Errorf("brief: expected NOT FOUND for missing symbol, got: %q", brief)
	}
}

func TestBrief_BudgetTruncation(t *testing.T) {
	// Create a symbol with a long signature that forces budget overflow.
	longSig := strings.Repeat("x", 500)
	be := &briefFakeBackend{
		sourcesFakeBackend: sourcesFakeBackend{
			extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		},
		symbolMap: map[string][]core.SymbolNode{
			"BigSym": {{Name: "BigSym", Kind: core.KindGoFunc, FilePath: "pkg/big.go", Line: 1, Signature: longSig}},
		},
	}
	srv := mcp.NewServer(be)
	// Use budget=1 to force truncation (charBudget = 4 chars).
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1502, Method: "tools/call",
		Params: map[string]any{
			"name": "brief",
			"arguments": map[string]any{
				"service": "svc",
				"symbols": []string{"BigSym"},
				"budget":  1,
			},
		},
	})
	p := xtraPayload(t, resp)
	brief, ok := p["brief"].(string)
	if !ok {
		t.Fatalf("brief: field missing: %v", p)
	}
	if !strings.Contains(brief, "truncated") {
		t.Errorf("brief: expected truncation notice, got: %q", brief)
	}
}

func TestBrief_MissingService(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1503, Method: "tools/call",
		Params: map[string]any{
			"name":      "brief",
			"arguments": map[string]any{},
		},
	})
	xtraError(t, resp)
}

// ─── ingest hygiene (IO-2 / O1 fix d) ────────────────────────────────────────

// hygieneFakeBackend implements ingest-by-path with per-service recorded
// roots, so the hygiene tests can seed a registry shaped like the live
// incidents (the ../geoffmiller-cloud twin, the crossplane-platform nested
// root) and prove the daemon now refuses them at the door.
type hygieneFakeBackend struct {
	extendedFakeBackend
	paths      map[string]string // service → recorded corpus root
	ingestedAt map[string]string // service → path IngestServiceAt received
}

func (h *hygieneFakeBackend) ServiceMeta(_ context.Context, serviceID string) (string, string, bool, error) {
	p, ok := h.paths[serviceID]
	return p, "", ok, nil
}

func (h *hygieneFakeBackend) IngestServiceAt(_ context.Context, serviceID, path string) error {
	if h.ingestedAt == nil {
		h.ingestedAt = map[string]string{}
	}
	h.ingestedAt[serviceID] = path
	return nil
}

func TestIngestHygiene(t *testing.T) {
	mustDir := func(t *testing.T, parts ...string) string {
		t.Helper()
		p := filepath.Join(parts...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	canon := func(t *testing.T, p string) string {
		t.Helper()
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	newSrv := func(paths map[string]string, services []string) (*mcp.Server, *hygieneFakeBackend) {
		h := &hygieneFakeBackend{paths: paths}
		h.services = services
		return mcp.NewServer(h), h
	}
	errMsg := func(t *testing.T, resp map[string]any) string {
		t.Helper()
		e := xtraError(t, resp)
		msg, _ := e["message"].(string)
		return msg
	}
	call := func(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
		t.Helper()
		return roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1290, Method: "tools/call",
			Params: map[string]any{"name": "ingest", "arguments": args},
		})
	}

	t.Run("path_like_service_id_refused_even_for_refresh", func(t *testing.T) {
		srv, _ := newSrv(nil, nil)
		resp := call(t, srv, map[string]any{"service": "../geoffmiller-cloud"})
		if e := errMsg(t, resp); !strings.Contains(e, "logical name") {
			t.Errorf("refusal should teach the rule, got %q", e)
		}
	})

	t.Run("relative_onboarding_path_refused", func(t *testing.T) {
		srv, _ := newSrv(nil, nil)
		resp := call(t, srv, map[string]any{"service": "newsvc", "path": "relative/root"})
		if e := errMsg(t, resp); !strings.Contains(e, "absolute") {
			t.Errorf("refusal should name absoluteness, got %q", e)
		}
	})

	t.Run("duplicate_root_refused_naming_claimant", func(t *testing.T) {
		root := t.TempDir()
		dirA := mustDir(t, root, "repo-a")
		srv, _ := newSrv(map[string]string{"claimant": dirA}, []string{"claimant"})
		resp := call(t, srv, map[string]any{"service": "twin", "path": dirA})
		if e := errMsg(t, resp); !strings.Contains(e, "claimant") {
			t.Errorf("refusal must name the claiming service, got %q", e)
		}
	})

	t.Run("nested_root_refused_both_directions", func(t *testing.T) {
		root := t.TempDir()
		parent := mustDir(t, root, "parent")
		child := mustDir(t, root, "parent", "sub", "module")
		srv, _ := newSrv(map[string]string{"parent-svc": parent}, []string{"parent-svc"})
		if e := errMsg(t, call(t, srv, map[string]any{"service": "child-svc", "path": child})); !strings.Contains(e, "parent-svc") {
			t.Errorf("child-inside-parent refusal must name the enclosing service, got %q", e)
		}
		srv2, _ := newSrv(map[string]string{"child-svc": child}, []string{"child-svc"})
		if e := errMsg(t, call(t, srv2, map[string]any{"service": "parent-svc", "path": parent})); !strings.Contains(e, "child-svc") {
			t.Errorf("parent-over-child refusal must name the nested service, got %q", e)
		}
	})

	t.Run("clean_onboard_canonicalizes", func(t *testing.T) {
		root := t.TempDir()
		real := mustDir(t, root, "real-repo")
		link := filepath.Join(root, "link-repo")
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		srv, h := newSrv(nil, nil)
		resp := call(t, srv, map[string]any{"service": "fresh", "path": link})
		p := xtraPayload(t, resp)
		if p["ok"] != true {
			t.Fatalf("clean onboard refused: %v", p)
		}
		if got, want := h.ingestedAt["fresh"], canon(t, real); got != want {
			t.Errorf("IngestServiceAt received %q, want the canonicalized root %q (the /Users-symlink registration class)", got, want)
		}
	})
}

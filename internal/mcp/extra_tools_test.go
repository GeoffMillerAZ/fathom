// Package mcp_test — TDD tests for the extra analyzer tools wired by
// registerExtraTools.
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
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

// TestRepoMap_ElisionLine is RC-19's proof for repo_map: alongside the
// existing structured "elided" block, a truncated map now also carries the
// uniform one-line "elision" text the other four newly-budgeted tools use.
func TestRepoMap_ElisionLine(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1281, Method: "tools/call",
		Params: map[string]any{
			"name":      "repo_map",
			"arguments": map[string]any{"service": "svc", "budget": 5},
		},
	})
	p := xtraPayload(t, resp)
	if p["elided"] == nil {
		t.Fatalf("repo_map: elided missing though a tiny budget was forced: %v", p)
	}
	line, ok := p["elision"].(string)
	if !ok || line == "" {
		t.Fatalf("repo_map: elision missing: %v", p)
	}
	if !strings.HasPrefix(line, "elided: ") || !strings.Contains(line, "refine with budget") {
		t.Errorf("repo_map: elision = %q, want the RC-19 shape", line)
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

// TestPackContext_SentenceConcernRefused pins J3(d) (app-kit O28):
// pack_context used to silently return an empty pack (budget spent,
// included:[], dropped:0 — indistinguishable from "no evidence for a real
// concern") when handed a natural-language sentence instead of a concern
// ID. It now refuses loudly with a params error naming the problem, before
// ever running the futile query.
func TestPackContext_SentenceConcernRefused(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1291, Method: "tools/call",
		Params: map[string]any{
			"name":      "pack_context",
			"arguments": map[string]any{"service": "svc", "concern": "G4 adoption improvement propagation second consumer reviewed"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for a sentence concern, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "not a concern ID") || !strings.Contains(msg, "whitespace") {
		t.Errorf("message = %q, want it to name the whitespace problem", msg)
	}
}

// TestPackContext_HyphenatedConcernIDStillAccepted proves the whitespace
// check does not over-refuse: a normal, single-token, hyphenated concern ID
// still runs the query.
func TestPackContext_HyphenatedConcernIDStillAccepted(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1292, Method: "tools/call",
		Params: map[string]any{
			"name":      "pack_context",
			"arguments": map[string]any{"service": "svc", "concern": "tailwind-build", "budget": 500},
		},
	})
	p := xtraPayload(t, resp)
	if p["included"] == nil {
		t.Error("pack_context: included missing")
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
					{"name": "complexity", "arguments": map[string]any{"service": "svc", "top_n": 5}},
					{"name": "evidence_summary", "arguments": map[string]any{"service": "svc"}},
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
		"blast_radius", "spec_coverage",
		"traceability", "evidence_summary", "contradiction_scan",
		"arch_conformance", "stale_docs", "api_diff",
		"ingest",
		"repo_map", "pack_context",
		"batch",
		"proto_conformance",
		"spec_refs",
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("extra tool %q not found in tools/list", name)
		}
	}

	// SR-15 (surface reckoning F-53, refined): the MCP `brief` tool
	// (worker pre-pack, registerBrief) is unregistered — 0 lifetime calls
	// and overlaps pack_context.
	if names["brief"] {
		t.Error("brief tool is still registered — SR-15 unregisters it")
	}
	// SR-10 (surface reckoning F-17/F-18): dep_drift and
	// cross_repo_blast_radius are unregistered — 0 lifetime calls.
	if names["dep_drift"] {
		t.Error("dep_drift tool is still registered — SR-10 unregisters it")
	}
	if names["cross_repo_blast_radius"] {
		t.Error("cross_repo_blast_radius tool is still registered — SR-10 unregisters it")
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

// callLogEntryFor parses the JSONL buffer and returns the first entry whose
// tool and via match. Fails the test when absent. Shared by review_tool_test.go
// and verify_plan_tool_test.go (SR-15 removed its original brief-tool caller,
// TestBrief_ClaimsAttachedReachesCallLog, but this helper outlived it).
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

// ─── ingest hygiene (IO-2 / O1 fix d) ────────────────────────────────────────

// hygieneFakeBackend implements ingest-by-path with per-service recorded
// roots, so the hygiene tests can seed a registry shaped like the live
// incidents (the ../orbit-service twin, a nested-root twin) and prove the
// daemon now refuses them at the door.
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
		resp := call(t, srv, map[string]any{"service": "../orbit-service"})
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

// ─── remove_service / rename_service (IO-3) ──────────────────────────────────

// surgeryFakeBackend adds the registry-surgery capability over the hygiene
// fake, recording calls so the tests assert delegation, not just plumbing.
type surgeryFakeBackend struct {
	hygieneFakeBackend
	removed   []string
	renamed   [][2]string
	renameErr error
}

func (s *surgeryFakeBackend) RemoveService(_ context.Context, serviceID string) (bool, error) {
	s.removed = append(s.removed, serviceID)
	_, existed := s.paths[serviceID]
	return existed, nil
}

func (s *surgeryFakeBackend) RenameService(_ context.Context, oldID, newID string) error {
	if s.renameErr != nil {
		return s.renameErr
	}
	s.renamed = append(s.renamed, [2]string{oldID, newID})
	return nil
}

func TestRemoveRenameService(t *testing.T) {
	newSrv := func(paths map[string]string) (*mcp.Server, *surgeryFakeBackend) {
		b := &surgeryFakeBackend{}
		b.paths = paths
		return mcp.NewServer(b), b
	}
	call := func(t *testing.T, srv *mcp.Server, tool string, args map[string]any) map[string]any {
		t.Helper()
		return roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1300, Method: "tools/call",
			Params: map[string]any{"name": tool, "arguments": args},
		})
	}

	t.Run("remove_accepts_malformed_historical_ids", func(t *testing.T) {
		srv, b := newSrv(map[string]string{"../orbit-service": "/idx/orbit"})
		p := xtraPayload(t, call(t, srv, "remove_service", map[string]any{"service": "../orbit-service"}))
		if p["removed"] != true {
			t.Fatalf("want removed=true for the twin, got %v", p)
		}
		if len(b.removed) != 1 || b.removed[0] != "../orbit-service" {
			t.Errorf("delegation mismatch: %v", b.removed)
		}
	})

	t.Run("remove_unknown_is_idempotent", func(t *testing.T) {
		srv, _ := newSrv(nil)
		p := xtraPayload(t, call(t, srv, "remove_service", map[string]any{"service": "ghost"}))
		if p["removed"] != false {
			t.Fatalf("want removed=false, got %v", p)
		}
	})

	t.Run("rename_requires_valid_target", func(t *testing.T) {
		srv, _ := newSrv(nil)
		resp := call(t, srv, "rename_service", map[string]any{"from": "../orbit-service", "to": "../still-bad"})
		e := xtraError(t, resp)
		msg, _ := e["message"].(string)
		if !strings.Contains(msg, "logical name") && !strings.Contains(msg, "valid") {
			t.Errorf("target validation should teach the rule, got %q", msg)
		}
	})

	t.Run("rename_surfaces_typed_store_refusals", func(t *testing.T) {
		srv, b := newSrv(nil)
		b.renameErr = fmt.Errorf("service %q already has rows", "taken")
		resp := call(t, srv, "rename_service", map[string]any{"from": "old", "to": "taken"})
		e := xtraError(t, resp)
		if msg, _ := e["message"].(string); !strings.Contains(msg, "taken") {
			t.Errorf("store refusal should pass through, got %q", msg)
		}
	})

	t.Run("rename_happy_path_delegates", func(t *testing.T) {
		srv, b := newSrv(map[string]string{"../orbit-service": "/idx/orbit"})
		p := xtraPayload(t, call(t, srv, "rename_service", map[string]any{"from": "../orbit-service", "to": "orbit-service-x"}))
		if p["ok"] != true {
			t.Fatalf("want ok=true, got %v", p)
		}
		if len(b.renamed) != 1 || b.renamed[0] != [2]string{"../orbit-service", "orbit-service-x"} {
			t.Errorf("delegation mismatch: %v", b.renamed)
		}
	})
}

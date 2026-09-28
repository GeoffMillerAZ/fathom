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

// spec_coverage was unregistered by SR-27 (surface reckoning F-35):
// review_diff's engine already runs the identical check in-process
// (runSpecCoverage, internal/analysis/review/engine.go) via the same
// sdd package this tool called directly — no logic lost.
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

// arch_conformance was unregistered by SR-27 (surface reckoning F-35):
// review_diff's engine already runs the identical check in-process
// (runArch, internal/analysis/review/engine.go, TestRun_Arch_
// ReportsViolationsWithLineCitation and its C4-3/P1-4 siblings in
// review_test.go) via the same archconformance/projectrules packages
// this tool called directly — no logic lost, one caller instead of two.
// stale_docs was unregistered by SR-28 (surface reckoning F-56): its scan
// now runs in-process as staleness_report's doc_drift section
// (TestStalenessReportDocDrift, internal/mcp/new_tools_test.go).

// api_diff was unregistered by SR-26 (surface reckoning F-20): the
// calibrated API delta stays in conserve's surface plane
// (internal/conserve/surface.go), which reuses internal/analysis/apisurface
// directly — this MCP tool had no caller left to serve.

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

// repoMapEntrypointFixture builds n exported HTTP-handler-shaped functions
// plus one "main", for repo_map's entrypoints-header proof (SR-22, F-10):
// the same symbolnav.Entrypoints detector the retired `entrypoints` tool
// used, capped at 10 with a "more" count.
func repoMapEntrypointFixture(n int) []core.SymbolNode {
	nodes := make([]core.SymbolNode, n+1)
	nodes[0] = core.SymbolNode{Service: "svc", Package: "pkg", Name: "main", Kind: core.KindGoFunc, FilePath: "cmd/main.go", Line: 1, Signature: "func()"}
	for i := 0; i < n; i++ {
		nodes[i+1] = core.SymbolNode{
			Service: "svc", Package: "pkg", Name: fmt.Sprintf("Handler%04d", i),
			Kind: core.KindGoFunc, FilePath: "pkg/handlers.go", Line: i + 2,
			Signature: "func(http.ResponseWriter, *http.Request)",
		}
	}
	return nodes
}

// TestRepoMapHeaderCarriesEntrypoints (SR-22, F-10): repo_map's response
// carries an "entrypoints" header — the same detector the unregistered
// `entrypoints` tool used — capped at 10 rows, with "entrypoints_more"
// naming how many were cut.
func TestRepoMapHeaderCarriesEntrypoints(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{nodes: repoMapEntrypointFixture(12), services: []string{"svc"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1282, Method: "tools/call",
		Params: map[string]any{
			"name":      "repo_map",
			"arguments": map[string]any{"service": "svc", "budget": 4000},
		},
	})
	p := xtraPayload(t, resp)
	eps, ok := p["entrypoints"].([]any)
	if !ok || len(eps) != 10 {
		t.Fatalf("entrypoints = %v, want 10 capped rows", p["entrypoints"])
	}
	more, ok := p["entrypoints_more"].(float64)
	if !ok || int(more) != 3 {
		t.Fatalf("entrypoints_more = %v, want 3 (13 total - 10 capped)", p["entrypoints_more"])
	}
}

// TestRepoMapHeaderOmitsEntrypointsWhenNone: a service with no likely
// entrypoint (the existing repo_map fixtures) never gets a fabricated
// header — the field is absent, not an empty list pretending to be one.
func TestRepoMapHeaderOmitsEntrypointsWhenNone(t *testing.T) {
	srv := newExtraSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1283, Method: "tools/call",
		Params: map[string]any{
			"name":      "repo_map",
			"arguments": map[string]any{"service": "svc", "budget": 500},
		},
	})
	p := xtraPayload(t, resp)
	if _, ok := p["entrypoints"]; ok {
		t.Errorf("entrypoints present though the fixture has none: %v", p["entrypoints"])
	}
	if _, ok := p["entrypoints_more"]; ok {
		t.Errorf("entrypoints_more present though the fixture has none: %v", p["entrypoints_more"])
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
		"blast_radius",
		"traceability", "evidence_summary", "contradiction_scan",
		"ingest",
		"repo_map", "pack_context",
		"batch",
		"proto_conformance",
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("extra tool %q not found in tools/list", name)
		}
	}

	// SR-27 (surface reckoning F-35): arch_conformance, spec_coverage and
	// spec_refs are unregistered — review_diff's engine already ran the
	// first two in-process, and obligations' additive spec_refs section
	// covers the third.
	for _, name := range []string{"arch_conformance", "spec_coverage", "spec_refs"} {
		if names[name] {
			t.Errorf("reckoning-removed tool %q is still in tools/list", name)
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

// spec_refs was unregistered by SR-27 (surface reckoning F-35): the
// edit hook's one real consumer (306 calls) now reads the same
// information from obligations' additive spec_refs section
// (TestObligationsSpecRefs_*, internal/mcp/semantic_tools_test.go)
// instead of a second RPC.
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

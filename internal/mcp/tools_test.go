// Package mcp_test — additional tool-level tests.
package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// compile-time check: fakeBackend must implement QueryInterface.
var _ mcp.QueryInterface = (*fakeBackend)(nil)

// ─── fake backend ────────────────────────────────────────────────────────────

type fakeNode struct {
	name string
	kind string
	file string
}

type fakeQueryResult struct {
	nodes []fakeNode
}

type fakeBackend struct {
	queryResult fakeQueryResult
	callers     []fakeNode
	symbol      []fakeNode
	listAll     []fakeNode
}

func (f *fakeBackend) QuerySymbols(_ context.Context, q core.Query) (core.QueryResult, error) {
	nodes := make([]core.SymbolNode, len(f.queryResult.nodes))
	for i, n := range f.queryResult.nodes {
		nodes[i] = core.SymbolNode{
			Service:  q.ServiceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	return core.QueryResult{Nodes: nodes}, nil
}

func (f *fakeBackend) FindCallers(_ context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, len(f.callers))
	for i, n := range f.callers {
		nodes[i] = core.SymbolNode{
			Service:  serviceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	_ = symbol
	return nodes, nil
}

func (f *fakeBackend) FindSymbol(_ context.Context, serviceID, name string) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, len(f.symbol))
	for i, n := range f.symbol {
		nodes[i] = core.SymbolNode{
			Service:  serviceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	_ = name
	return nodes, nil
}

func (f *fakeBackend) ListAllSymbols(_ context.Context, serviceID string) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, len(f.listAll))
	for i, n := range f.listAll {
		nodes[i] = core.SymbolNode{
			Service:  serviceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	return nodes, nil
}

func (f *fakeBackend) Services(_ context.Context) ([]string, error) {
	return nil, nil
}

func (f *fakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}

func (f *fakeBackend) IngestService(_ context.Context, _ string) error { return nil }

func (f *fakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *fakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (f *fakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return nil, nil
}

// ─── find_callers ────────────────────────────────────────────────────────────

func TestFindCallers(t *testing.T) {
	fake := &fakeBackend{
		callers: []fakeNode{
			{name: "ProcessPayment", kind: "GoFunc", file: "payment/payment.go"},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      30,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "find_callers",
			"arguments": map[string]any{
				"service": "payment-service",
				"symbol":  "ValidateToken",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	callers, ok := payload["callers"].([]any)
	if !ok {
		t.Fatalf("payload.callers missing: %v", payload)
	}
	if len(callers) != 1 {
		t.Errorf("callers count = %d, want 1", len(callers))
	}
}

// TestFindCallersCallerUnder100Bytes (SB-47, docs/plans/strengths-bench.md;
// mirrors SB-1's TestReferencesOccurrenceUnder100Bytes): every rendered
// caller — the compact {loc, c} wire shape renderCallerEntry produces —
// marshals at or under 100 bytes, even over a realistically deep, long
// caller set (the plan's own repro: a 38-caller answer for a real symbol
// used to cost 14 KB before this row).
func TestFindCallersCallerUnder100Bytes(t *testing.T) {
	const n = 38
	fake := &fakeBackend{}
	for i := 0; i < n; i++ {
		fake.callers = append(fake.callers, fakeNode{
			name: "someCallerFunctionName",
			kind: "GoFunc",
			file: "internal/cmd/fathom/serve/handlers/dispatch.go",
		})
	}
	srv := mcp.NewServer(fake)
	payload := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "runWithStdin"})

	callers, ok := payload["callers"].([]any)
	if !ok || len(callers) != n {
		t.Fatalf("callers = %v, want %d entries", payload["callers"], n)
	}
	var total int
	for _, raw := range callers {
		b, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("marshal caller: %v", err)
		}
		if len(b) > 100 {
			t.Errorf("caller %s is %d bytes, want <= 100", b, len(b))
		}
		if _, hasSig := raw.(map[string]any)["sig"]; hasSig {
			t.Errorf("caller %s carries \"sig\" without with_signatures: true", b)
		}
		total += len(b)
	}
	// The plan's own bar: this shape must cost well under 4 KB for the
	// answer that used to cost 14 KB.
	if total >= 4000 {
		t.Errorf("callers array totals %d bytes, want < 4000 (SB-47's target)", total)
	}
	if payload["caller_service"] != "svc" {
		t.Errorf("caller_service = %v, want svc (uniform — one service per call)", payload["caller_service"])
	}
}

// findCallersSigFake is a minimal QueryInterface whose caller carries a
// real signature, for TestFindCallersWithSignatures.
type findCallersSigFake struct {
	fakeBackend
}

func (f *findCallersSigFake) FindCallers(_ context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	return []core.SymbolNode{{
		Service: serviceID, Name: "Caller", Kind: core.KindGoFunc,
		FilePath: "pkg/caller.go", Line: 12,
		Signature: "func Caller(ctx context.Context, arg string) error",
	}}, nil
}

// TestFindCallersWithSignatures: with_signatures: true adds "sig" to each
// compact row; its absence is the default (SB-47).
func TestFindCallersWithSignatures(t *testing.T) {
	srv := mcp.NewServer(&findCallersSigFake{})

	without := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})
	rows := without["callers"].([]any)
	if len(rows) != 1 {
		t.Fatalf("callers = %v, want 1", rows)
	}
	if _, hasSig := rows[0].(map[string]any)["sig"]; hasSig {
		t.Errorf("caller %v carries \"sig\" by default, want absent", rows[0])
	}

	with := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target", "with_signatures": true})
	rows = with["callers"].([]any)
	if sig, _ := rows[0].(map[string]any)["sig"].(string); sig != "func Caller(ctx context.Context, arg string) error" {
		t.Errorf("caller sig = %q, want the full signature", sig)
	}
}

// manyCallersFake returns a caller-configurable QueryInterface: FindCallers
// always answers with the fixed `callers` slice (Service filled in per
// call), regardless of the requested symbol — RC-19/RC-20 proof only cares
// about the SHAPE of a large fan-in, not resolving a real one.
type manyCallersFake struct {
	fakeBackend
	callers []core.SymbolNode
}

func (f *manyCallersFake) FindCallers(_ context.Context, serviceID, _ string) ([]core.SymbolNode, error) {
	out := make([]core.SymbolNode, len(f.callers))
	for i, n := range f.callers {
		n.Service = serviceID
		out[i] = n
	}
	return out, nil
}

// manyCallersAcrossPackages builds n synthetic callers spread evenly across
// numPkgs packages, each with a realistic file path/name so the rendered
// wire size is representative.
func manyCallersAcrossPackages(n, numPkgs int) []core.SymbolNode {
	nodes := make([]core.SymbolNode, n)
	for i := 0; i < n; i++ {
		pkg := fmt.Sprintf("internal/service/pkg%02d", i%numPkgs)
		nodes[i] = core.SymbolNode{
			Package:  pkg,
			Name:     fmt.Sprintf("handleRequest%04d", i),
			Kind:     core.KindGoFunc,
			FilePath: fmt.Sprintf("%s/handler.go", pkg),
			Line:     i + 1,
		}
	}
	return nodes
}

// TestFindCallers_BudgetElision is RC-19's proof for find_callers: a fan-in
// too large for one response (but nowhere near scope_required's much higher
// threshold) is truncated to a whole-row prefix plus a one-line elision
// naming the refinement.
func TestFindCallers_BudgetElision(t *testing.T) {
	fake := &manyCallersFake{callers: manyCallersAcrossPackages(250, 12)}
	srv := mcp.NewServer(fake)
	payload := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})

	if payload["scope_required"] == true {
		t.Fatalf("scope_required unexpectedly true for a 250-row fan-in: %v", payload)
	}
	callers, ok := payload["callers"].([]any)
	if !ok || len(callers) == 0 || len(callers) >= 250 {
		t.Fatalf("callers = %d rows, want a nonzero partial prefix < 250", len(callers))
	}
	line, ok := payload["elision"].(string)
	if !ok || line == "" {
		t.Fatalf("elision missing: %v", payload)
	}
	if !strings.HasPrefix(line, "elided: ") || !strings.Contains(line, "refine with a package-qualified symbol (pkg.Name)") {
		t.Errorf("elision = %q, want the RC-19 shape", line)
	}
}

// TestFindCallers_ScopeRequired is RC-20's golden test for find_callers: a
// generic name's fan-in so large that no page of rows would help returns
// the count and top packages instead of any rows at all.
func TestFindCallers_ScopeRequired(t *testing.T) {
	const n, numPkgs = 1500, 12
	fake := &manyCallersFake{callers: manyCallersAcrossPackages(n, numPkgs)}
	srv := mcp.NewServer(fake)
	payload := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "String"})

	if payload["scope_required"] != true {
		t.Fatalf("scope_required = %v, want true for a %d-row fan-in", payload["scope_required"], n)
	}
	callers, ok := payload["callers"].([]any)
	if !ok || len(callers) != 0 {
		t.Errorf("callers = %v, want empty when scope_required", payload["callers"])
	}
	if count, _ := payload["count"].(float64); int(count) != n {
		t.Errorf("count = %v, want %d", payload["count"], n)
	}
	tops, ok := payload["top_packages"].([]any)
	if !ok || len(tops) != 8 {
		t.Fatalf("top_packages = %v, want 8 entries", payload["top_packages"])
	}
	// Every package carries the same count (n/numPkgs); sorted ties break
	// on package name ascending — pkg00 must lead.
	first := tops[0].(map[string]any)
	if first["package"] != "internal/service/pkg00" {
		t.Errorf("top_packages[0].package = %v, want pkg00 (tie-break by name)", first["package"])
	}
	if msg, _ := payload["message"].(string); msg == "" {
		t.Error("message missing for scope_required response")
	}
}

// ─── find_symbol ─────────────────────────────────────────────────────────────

func TestFindSymbol(t *testing.T) {
	fake := &fakeBackend{
		symbol: []fakeNode{
			{name: "ChargeRequest", kind: "GoType", file: "payment/types.go"},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      40,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "find_symbol",
			"arguments": map[string]any{
				"service": "payment-service",
				"name":    "ChargeRequest",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["symbols"] == nil {
		t.Error("payload.symbols missing")
	}
}

// ─── missing required param → -32602 ─────────────────────────────────────────

func TestQuerySymbolsMissingService(t *testing.T) {
	fake := &fakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      50,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "query_symbols",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	code := errObj["code"].(float64)
	if code != -32602 {
		t.Errorf("error.code = %v, want -32602", code)
	}
}

// ─── set_profile → unknown profile → -32602 ──────────────────────────────────

// ─── tools/call with unknown tool name → -32601 ───────────────────────────────

func TestCallUnknownTool(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      70,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "no_such_tool",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	code := errObj["code"].(float64)
	if code != -32601 {
		t.Errorf("error.code = %v, want -32601", code)
	}
}

// ─── investigation profile: find_callers must appear ─────────────────────────

func TestInvestigationProfileIncludesFindCallers(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})

	// HF-5 deleted /rpc/investigation (ADR-014 amendment 2). Two facts
	// survive and both matter: find_callers' group classification, and its
	// presence on the curated agent surface — the path agents actually use.
	if got := srv.GroupOfForTest("find_callers"); got != mcp.GroupInvestigation {
		t.Errorf("find_callers registered in group %q, want %q", got, mcp.GroupInvestigation)
	}
	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceAgent, true)
	resp := scopedRoundtrip(t, srv, ctx, request{JSONRPC: "2.0", ID: 1, Method: "tools/list"})
	if resp["error"] != nil {
		t.Fatalf("tools/list(agent): %v", resp["error"])
	}
	found := false
	for _, raw := range resp["result"].(map[string]any)["tools"].([]any) {
		if raw.(map[string]any)["name"] == "find_callers" {
			found = true
		}
	}
	if !found {
		t.Error("find_callers must stay on the agent surface — it is the investigation hot loop")
	}
}

// Package mcp_test — P1-1 (additive "symbol" alias on the hot-tool quartet)
// and P1-8 (dispatch-level required-field validation, and its interaction
// with P1-1's aliases) tests. docs/plans/pulled-tools-excellence.md.
package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// rpcErrCode extracts the JSON-RPC error code from a roundtrip response, or
// fails the test if the response has no error.
func rpcErrCode(t *testing.T, resp map[string]any) float64 {
	t.Helper()
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected an RPC error, got result: %v", resp)
	}
	code, _ := errObj["code"].(float64)
	return code
}

// ─── find_symbol: "symbol" alias for "name" ───────────────────────────────────

func TestFindSymbolAlias_NativeNameWorks(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{symbol: []fakeNode{{name: "ChargeRequest", kind: "GoType", file: "payment/types.go"}}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "ChargeRequest"}},
	})
	p := findSymbolPayload(t, resp)
	if p["matched_via"] != "exact" {
		t.Errorf("matched_via = %v, want exact", p["matched_via"])
	}
}

func TestFindSymbolAlias_SymbolAliasWorks(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{symbol: []fakeNode{{name: "ChargeRequest", kind: "GoType", file: "payment/types.go"}}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		// "symbol" only — no "name" at all. Pre-P1-1 this failed dispatch
		// validation ("name required"); it must now succeed identically.
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "symbol": "ChargeRequest"}},
	})
	p := findSymbolPayload(t, resp)
	if p["matched_via"] != "exact" {
		t.Errorf("matched_via = %v, want exact", p["matched_via"])
	}
}

func TestFindSymbolAlias_ConflictingValuesErrorsLoudly(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{symbol: []fakeNode{{name: "ChargeRequest", kind: "GoType", file: "payment/types.go"}}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "ChargeRequest", "symbol": "SomethingElse"}},
	})
	if code := rpcErrCode(t, resp); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
	errObj := resp["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !containsAll(msg, "name", "symbol") {
		t.Errorf("error message should name both conflicting params, got %q", msg)
	}
}

func TestFindSymbolAlias_SameValueNoConflict(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{symbol: []fakeNode{{name: "ChargeRequest", kind: "GoType", file: "payment/types.go"}}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "ChargeRequest", "symbol": "ChargeRequest"}},
	})
	p := findSymbolPayload(t, resp)
	if p["matched_via"] != "exact" {
		t.Errorf("matched_via = %v, want exact (agreeing alias+native should not error)", p["matched_via"])
	}
}

// ─── query_symbols: "symbol" alias for "concern" ──────────────────────────────

func TestQuerySymbolsAlias_NativeConcernWorks(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{queryResult: fakeQueryResult{nodes: []fakeNode{{name: "Login", kind: "GoFunc", file: "auth/login.go"}}}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{"name": "query_symbols", "arguments": map[string]any{"service": "svc", "concern": "authentication"}},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestQuerySymbolsAlias_SymbolAliasWorks(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{queryResult: fakeQueryResult{nodes: []fakeNode{{name: "Login", kind: "GoFunc", file: "auth/login.go"}}}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		// "symbol" only — no "concern" at all.
		Params: map[string]any{"name": "query_symbols", "arguments": map[string]any{"service": "svc", "symbol": "authentication"}},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestQuerySymbolsAlias_ConflictingValuesErrorsLoudly(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/call",
		Params: map[string]any{"name": "query_symbols", "arguments": map[string]any{"service": "svc", "concern": "authentication", "symbol": "billing"}},
	})
	if code := rpcErrCode(t, resp); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
	errObj := resp["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !containsAll(msg, "concern", "symbol") {
		t.Errorf("error message should name both conflicting params, got %q", msg)
	}
}

// ─── references: "symbol" alias for "query" ───────────────────────────────────

func TestReferencesAlias_NativeQueryWorks(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/call",
		Params: map[string]any{"name": "references", "arguments": map[string]any{"query": "ProcessPayment"}},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestReferencesAlias_SymbolAliasWorks(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		// "symbol" only — no "query" at all.
		Params: map[string]any{"name": "references", "arguments": map[string]any{"symbol": "ProcessPayment"}},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestReferencesAlias_ConflictingValuesErrorsLoudly(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: map[string]any{"name": "references", "arguments": map[string]any{"query": "ProcessPayment", "symbol": "ValidateToken"}},
	})
	if code := rpcErrCode(t, resp); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
	errObj := resp["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !containsAll(msg, "query", "symbol") {
		t.Errorf("error message should name both conflicting params, got %q", msg)
	}
}

// ─── P1-8: dispatch-level required-field validation ───────────────────────────

// TestDispatch_PreviouslyLenientTool_NowRefuses proves spec_coverage — which
// declares "spec_ids" required but never checked it itself (C4-5's audit) —
// is now rejected by dispatch BEFORE the handler runs, when "spec_ids" is
// omitted.
func TestDispatch_PreviouslyLenientTool_NowRefuses(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 11, Method: "tools/call",
		Params: map[string]any{"name": "spec_coverage", "arguments": map[string]any{"service": "svc"}},
	})
	if code := rpcErrCode(t, resp); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v (spec_coverage declares \"spec_ids\" required)", code, mcp.ErrCodeParams)
	}
}

// TestDispatch_AliasCallPassesRequiredValidation proves the P1-1/P1-8
// interaction: find_symbol called with ONLY the "symbol" alias (no "name")
// passes dispatch-level required-field validation instead of being
// rejected for "missing" name.
func TestDispatch_AliasCallPassesRequiredValidation(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{symbol: []fakeNode{{name: "ChargeRequest", kind: "GoType", file: "payment/types.go"}}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 12, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "symbol": "ChargeRequest"}},
	})
	if resp["error"] != nil {
		t.Fatalf("alias-only call should pass required validation, got error: %v", resp["error"])
	}
}

// TestDispatch_MissingRequiredField_UnaliasedTool proves the baseline
// dispatch behavior for a tool with no alias at all: a genuinely missing
// required field is still rejected.
func TestDispatch_MissingRequiredField_UnaliasedTool(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 13, Method: "tools/call",
		Params: map[string]any{"name": "find_callers", "arguments": map[string]any{"service": "svc"}},
	})
	if code := rpcErrCode(t, resp); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v (find_callers requires \"symbol\")", code, mcp.ErrCodeParams)
	}
}

// TestBatch_SubCallRequiredValidation_RefusesWithoutFailingBatch closes
// the batch bypass the critic pass found: batch sub-calls invoke
// te.handler directly, skipping handleToolsCallWithCtx's validateRequired
// gate — so batch{calls:[{name:"spec_coverage",arguments:{service}}]}
// reached the exact previously-lenient handler P1-8 claims to close off.
// The batch loop now validates each sub-call before its handler runs. The
// failing sub-call errors into ITS OWN batchResult; sibling sub-calls
// still run (per batch's own contract: per-call errors never abort the
// batch).
func TestBatch_SubCallRequiredValidation_RefusesWithoutFailingBatch(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 14, Method: "tools/call",
		Params: map[string]any{
			"name": "batch",
			"arguments": map[string]any{
				"calls": []map[string]any{
					// Missing "spec_ids" (declared required, handler never checked it).
					{"name": "spec_coverage", "arguments": map[string]any{"service": "svc"}},
					// A valid sibling that must still run.
					{"name": "outline", "arguments": map[string]any{"service": "svc", "file": "pkg/a.go"}},
				},
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("the batch itself must not fail on a sub-call validation error: %v", resp["error"])
	}
	content := resp["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload struct {
		Results []struct {
			Name   string `json:"name"`
			Result any    `json:"result"`
			Error  string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload parse: %v (text=%q)", err, text)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(payload.Results))
	}
	lp := payload.Results[0]
	if lp.Name != "spec_coverage" || lp.Error == "" {
		t.Errorf("spec_coverage sub-call should refuse (missing required \"spec_ids\"), got %+v", lp)
	}
	if lp.Error != "" && !containsAll(lp.Error, "spec_ids") {
		t.Errorf("refusal should name the missing parameter, got %q", lp.Error)
	}
	sib := payload.Results[1]
	if sib.Name != "outline" || sib.Error != "" || sib.Result == nil {
		t.Errorf("sibling outline sub-call should still run and succeed, got %+v", sib)
	}
}

// containsAll reports whether s contains every one of subs as a substring.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

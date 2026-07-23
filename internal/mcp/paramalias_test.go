// Package mcp_test — P1-1 (additive "symbol" alias on the hot-tool quartet)
// and P1-8 (dispatch-level required-field validation, and its interaction
// with P1-1's aliases) tests. docs/plans/pulled-tools-excellence.md.
package mcp_test

import (
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

// TestDispatch_PreviouslyLenientTool_NowRefuses proves license_policy — which
// declares "allowed" required but never checked it itself (C4-5's audit) —
// is now rejected by dispatch BEFORE the handler runs, when "allowed" is
// omitted.
func TestDispatch_PreviouslyLenientTool_NowRefuses(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 11, Method: "tools/call",
		Params: map[string]any{"name": "license_policy", "arguments": map[string]any{"service": "svc"}},
	})
	if code := rpcErrCode(t, resp); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v (license_policy declares \"allowed\" required)", code, mcp.ErrCodeParams)
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

// containsAll reports whether s contains every one of subs as a substring.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

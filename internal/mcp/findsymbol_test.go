// Tiered find_symbol tests.
//
// Dogfood journal ask #3: find_symbol was exact-match only, so it "punishes
// not knowing the exact identifier — which is exactly when you reach for it".
// The tool now degrades deterministically: exact/key-suffix →
// case-insensitive → substring → did_you_mean suggestions (edit-distance,
// else concern-ranked).
package mcp_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// findSymbolPayload decodes the MCP content envelope of a find_symbol result.
func findSymbolPayload(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	if resp["error"] != nil {
		t.Fatalf("find_symbol error: %v", resp["error"])
	}
	content := resp["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload parse: %v (text=%q)", err, text)
	}
	return payload
}

func callFindSymbol(t *testing.T, srv *mcp.Server, name string) map[string]any {
	t.Helper()
	return findSymbolPayload(t, roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 40, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": name}},
	}))
}

func symbolNames(t *testing.T, payload map[string]any) []string {
	t.Helper()
	raw, _ := payload["symbols"].([]any)
	names := make([]string, len(raw))
	for i, s := range raw {
		names[i] = s.(map[string]any)["name"].(string)
	}
	return names
}

func TestFindSymbolExactMatch(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{
		symbol: []fakeNode{{name: "ValidateToken", kind: "GoFunc", file: "auth/token.go"}},
	})
	payload := callFindSymbol(t, srv, "ValidateToken")
	if payload["matched_via"] != "exact" {
		t.Errorf("matched_via = %v, want exact", payload["matched_via"])
	}
	if names := symbolNames(t, payload); len(names) != 1 || names[0] != "ValidateToken" {
		t.Errorf("symbols = %v, want [ValidateToken]", names)
	}
	if payload["did_you_mean"] != nil {
		t.Errorf("exact match should carry no did_you_mean, got %v", payload["did_you_mean"])
	}
}

func TestFindSymbolCaseInsensitiveFallback(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{
		// Exact tier misses (symbol empty); the registry knows the real spelling.
		listAll: []fakeNode{
			{name: "ValidateToken", kind: "GoFunc", file: "auth/token.go"},
			{name: "TokenStore", kind: "GoType", file: "auth/store.go"},
		},
	})
	payload := callFindSymbol(t, srv, "validatetoken")
	if payload["matched_via"] != "case_insensitive" {
		t.Errorf("matched_via = %v, want case_insensitive", payload["matched_via"])
	}
	if names := symbolNames(t, payload); len(names) != 1 || names[0] != "ValidateToken" {
		t.Errorf("symbols = %v, want [ValidateToken]", names)
	}
}

func TestFindSymbolCaseInsensitiveKeySuffix(t *testing.T) {
	// "serveHTTP" (the journal's literal miss) should find "Server.ServeHTTP"
	// via the case-folded key-suffix rule.
	srv := mcp.NewServer(&fakeBackend{
		listAll: []fakeNode{
			{name: "Server.ServeHTTP", kind: "GoFunc", file: "http.go"},
		},
	})
	payload := callFindSymbol(t, srv, "serveHTTP")
	if payload["matched_via"] != "case_insensitive" {
		t.Errorf("matched_via = %v, want case_insensitive", payload["matched_via"])
	}
	if names := symbolNames(t, payload); len(names) != 1 || names[0] != "Server.ServeHTTP" {
		t.Errorf("symbols = %v, want [Server.ServeHTTP]", names)
	}
}

func TestFindSymbolSubstringFallback(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{
		listAll: []fakeNode{
			{name: "ValidateToken", kind: "GoFunc", file: "auth/token.go"},
			{name: "TokenStore", kind: "GoType", file: "auth/store.go"},
			{name: "ChargeCard", kind: "GoFunc", file: "billing/card.go"},
		},
	})
	payload := callFindSymbol(t, srv, "token")
	if payload["matched_via"] != "substring" {
		t.Errorf("matched_via = %v, want substring", payload["matched_via"])
	}
	// Deterministic order: shorter (tighter) names first, then alphabetical.
	names := symbolNames(t, payload)
	want := []string{"TokenStore", "ValidateToken"}
	if len(names) != len(want) {
		t.Fatalf("symbols = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("symbols[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestFindSymbolFuzzySuggestionsForTypo(t *testing.T) {
	// The typo tier must work from the symbol list alone: queryResult is left
	// EMPTY so a passing test proves edit-distance did the work, not the
	// concern-ranked fallback (which a fake backend would answer regardless).
	srv := mcp.NewServer(&fakeBackend{
		listAll: []fakeNode{
			{name: "Authentication", kind: "GoType", file: "auth/auth.go"},
			{name: "Server.ServeHTTP", kind: "GoFunc", file: "http.go"},
			{name: "ChargeCard", kind: "GoFunc", file: "billing/card.go"},
		},
	})

	payload := callFindSymbol(t, srv, "authentiction") // 1 edit from Authentication
	if payload["matched_via"] != "none" {
		t.Errorf("matched_via = %v, want none (suggestions are not matches)", payload["matched_via"])
	}
	dym, _ := payload["did_you_mean"].([]any)
	if len(dym) != 1 {
		t.Fatalf("did_you_mean = %v, want exactly [Authentication]", payload["did_you_mean"])
	}
	if got := dym[0].(map[string]any)["name"]; got != "Authentication" {
		t.Errorf("did_you_mean[0] = %v, want Authentication", got)
	}

	// Typo against a method's last segment: "servehtp" → Server.ServeHTTP.
	payload = callFindSymbol(t, srv, "servehtp")
	dym, _ = payload["did_you_mean"].([]any)
	if len(dym) != 1 || dym[0].(map[string]any)["name"] != "Server.ServeHTTP" {
		t.Fatalf("did_you_mean = %v, want [Server.ServeHTTP]", payload["did_you_mean"])
	}
}

func TestFindSymbolRankedSuggestionsOnMiss(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{
		listAll: []fakeNode{
			{name: "ChargeCard", kind: "GoFunc", file: "billing/card.go"},
		},
		queryResult: fakeQueryResult{nodes: []fakeNode{
			{name: "ValidateToken", kind: "GoFunc", file: "auth/token.go"},
			{name: "RefreshSession", kind: "GoFunc", file: "auth/session.go"},
		}},
	})
	// No fuzzy candidate exists (nothing within edit distance of the query),
	// so the concern-ranked fallback supplies the suggestions.
	payload := callFindSymbol(t, srv, "authentiction")
	if payload["matched_via"] != "none" {
		t.Errorf("matched_via = %v, want none", payload["matched_via"])
	}
	if raw, _ := payload["symbols"].([]any); len(raw) != 0 {
		t.Errorf("symbols should be empty on a full miss, got %v", raw)
	}
	dym, _ := payload["did_you_mean"].([]any)
	if len(dym) != 2 {
		t.Fatalf("did_you_mean = %v, want 2 ranked suggestions", payload["did_you_mean"])
	}
	first := dym[0].(map[string]any)
	if first["name"] != "ValidateToken" {
		t.Errorf("did_you_mean[0] = %v, want ValidateToken", first["name"])
	}
	if first["file_path"] == nil {
		t.Error("suggestions should carry file_path")
	}
}

// ─── P1-3: case-insensitive tier affinity ranking (closes O12(c)) ────────────

// TestFindSymbolCaseInsensitive_PackageAffinity_HeartbeatReproduction is the
// bench reproduction from docs/dogfood-observations.md O12(c):
// find_symbol(agentic-kit, "heartbeat") returned ONLY the coincidentally
// alphabetized worker-heartbeat GoType, hiding the telemetry/heartbeat
// package's own exported functions (never literally named "heartbeat", so
// the name-only tier could never surface them). The fixed tier returns
// BOTH — a name match (the Heartbeat type) AND a package match (every
// telemetry/heartbeat export) — group-ranked, name matches first.
func TestFindSymbolCaseInsensitive_PackageAffinity_HeartbeatReproduction(t *testing.T) {
	nodes := []core.SymbolNode{
		// worker-heartbeat: a type literally named "Heartbeat" — matches by NAME.
		{Service: "agentic-kit", Package: "worker-heartbeat", Name: "Heartbeat", Kind: core.KindGoType, FilePath: "foundations/coordination/worker-heartbeat/types.go"},
		// telemetry/heartbeat: declares `package heartbeat` — its exports match
		// by PACKAGE only; none of them is itself named "heartbeat".
		{Service: "agentic-kit", Package: "heartbeat", Name: "Load", Kind: core.KindGoFunc, FilePath: "foundations/telemetry/heartbeat/heartbeat.go"},
		{Service: "agentic-kit", Package: "heartbeat", Name: "CheckLiveness", Kind: core.KindGoFunc, FilePath: "foundations/telemetry/heartbeat/heartbeat.go"},
		{Service: "agentic-kit", Package: "heartbeat", Name: "Rollup", Kind: core.KindGoFunc, FilePath: "foundations/telemetry/heartbeat/heartbeat.go"},
	}
	srv := mcp.NewServer(&extendedFakeBackend{nodes: nodes, services: []string{"agentic-kit"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 41, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "agentic-kit", "name": "heartbeat"}},
	})
	payload := findSymbolPayload(t, resp)
	if payload["matched_via"] != "case_insensitive" {
		t.Fatalf("matched_via = %v, want case_insensitive", payload["matched_via"])
	}
	names := symbolNames(t, payload)
	// Group 0 (name match) first: Heartbeat. Group 1 (package match) next,
	// alphabetical: CheckLiveness, Load, Rollup.
	want := []string{"Heartbeat", "CheckLiveness", "Load", "Rollup"}
	if len(names) != len(want) {
		t.Fatalf("symbols = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("symbols[%d] = %q, want %q", i, names[i], want[i])
		}
	}
	// The specific regression: telemetry/heartbeat's CheckLiveness must be
	// present, not hidden behind worker-heartbeat's same-named-query type.
	found := false
	for _, n := range names {
		if n == "CheckLiveness" {
			found = true
		}
	}
	if !found {
		t.Error("case-insensitive tier hid telemetry/heartbeat's CheckLiveness behind worker-heartbeat's Heartbeat type")
	}
}

// TestFindSymbolCaseInsensitive_NameMatchOnly_Unaffected proves a plain
// name-only case-insensitive match (no package with a matching name in the
// node set) is unaffected by the package-match group: single result,
// unchanged shape.
func TestFindSymbolCaseInsensitive_NameMatchOnly_Unaffected(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{
		listAll: []fakeNode{
			{name: "ValidateToken", kind: "GoFunc", file: "auth/token.go"},
			{name: "TokenStore", kind: "GoType", file: "auth/store.go"},
		},
	})
	payload := callFindSymbol(t, srv, "validatetoken")
	if payload["matched_via"] != "case_insensitive" {
		t.Errorf("matched_via = %v, want case_insensitive", payload["matched_via"])
	}
	if names := symbolNames(t, payload); len(names) != 1 || names[0] != "ValidateToken" {
		t.Errorf("symbols = %v, want [ValidateToken]", names)
	}
}

// TestFindSymbolCaseInsensitive_PackageMatchCapsAt20 pins J3(b) (app-kit
// O22): a query naming a large package used to return EVERY one of its
// exported symbols through the case-insensitive tier's package-match
// group, unbounded — the O22 "package-like find_symbol query floods
// context without result budget" defect. It now caps at 20 with the
// standard elision disclosure, exactly like the substring tier already
// does.
func TestFindSymbolCaseInsensitive_PackageMatchCapsAt20(t *testing.T) {
	var nodes []core.SymbolNode
	for i := 0; i < 25; i++ {
		nodes = append(nodes, core.SymbolNode{
			Service: "svc", Package: "bigpkg", Name: fmt.Sprintf("Func%02d", i),
			Kind: core.KindGoFunc, FilePath: "bigpkg/x.go", Line: i + 1,
		})
	}
	srv := mcp.NewServer(&extendedFakeBackend{nodes: nodes, services: []string{"svc"}})
	payload := callFindSymbol(t, srv, "bigpkg")
	if payload["matched_via"] != "case_insensitive" {
		t.Fatalf("matched_via = %v, want case_insensitive", payload["matched_via"])
	}
	names := symbolNames(t, payload)
	if len(names) != 20 {
		t.Fatalf("case-insensitive package matches = %d, want capped at 20", len(names))
	}
	if payload["truncated"] != true {
		t.Errorf("truncated = %v, want true", payload["truncated"])
	}
	elided, ok := payload["elided"].(map[string]any)
	if !ok {
		t.Fatalf("elided block missing: %v", payload)
	}
	if of, _ := elided["of"].(float64); of != 25 {
		t.Errorf("elided.of = %v, want 25", elided["of"])
	}
	if cnt, _ := elided["count"].(float64); cnt != 5 {
		t.Errorf("elided.count = %v, want 5", elided["count"])
	}
}

func TestFindSymbolSubstringCapIsDeterministic(t *testing.T) {
	// 25 matching names; the substring tier caps at 20 and reports truncation.
	var nodes []fakeNode
	for _, c := range "abcdefghijklmnopqrstuvwxy" {
		nodes = append(nodes, fakeNode{name: "Tok" + strings.Repeat(string(c), 2), kind: "GoFunc", file: "f.go"})
	}
	srv := mcp.NewServer(&fakeBackend{listAll: nodes})
	payload := callFindSymbol(t, srv, "tok")
	names := symbolNames(t, payload)
	if len(names) != 20 {
		t.Fatalf("substring matches = %d, want capped at 20", len(names))
	}
	if payload["truncated"] != true {
		t.Errorf("truncated = %v, want true", payload["truncated"])
	}
	if names[0] != "Tokaa" {
		t.Errorf("first match = %q, want Tokaa (alphabetical within equal length)", names[0])
	}
}

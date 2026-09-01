package expertchat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// wrapAsToolResult marshals result to JSON and wraps it in the MCP content
// envelope fathom's real dispatch.go actually produces
// (handleToolsCallWithCtx: `{"content":[{"type":"text","text":"<json>"}]}`)
// — the shape every httptest fixture in this file must reproduce, since it
// is the contract HTTPRetriever now decodes against.
func wrapAsToolResult(t *testing.T, result any) json.RawMessage {
	t.Helper()
	inner, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("wrapAsToolResult: marshal inner result: %v", err)
	}
	envelope, err := json.Marshal(mcpToolResult{Content: []mcpContentItem{{Type: "text", Text: string(inner)}}})
	if err != nil {
		t.Fatalf("wrapAsToolResult: marshal envelope: %v", err)
	}
	return envelope
}

// decodeToolsCallRequest decodes an inbound HTTP body as a JSON-RPC
// tools/call request, asserting the envelope shape HTTPRetriever must send:
// method "tools/call", params {name, arguments}.
func decodeToolsCallRequest(t *testing.T, r *http.Request) (id int, args moduleQueryArguments) {
	t.Helper()
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Fatalf("server: decode request: %v", err)
	}
	if req.Method != "tools/call" {
		t.Fatalf("server: got method %q, want %q (fathom's dispatch.go recognizes exactly initialize/tools list/tools call — no bespoke verbs)", req.Method, "tools/call")
	}
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("server: decode tools/call params: %v", err)
	}
	if params.Name != moduleQueryToolName {
		t.Fatalf("server: got tool name %q, want %q", params.Name, moduleQueryToolName)
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		t.Fatalf("server: decode tool arguments: %v", err)
	}
	return req.ID, args
}

// serveModuleQuery spins up an httptest server that decodes exactly one
// tools/call request, hands its arguments to build, and responds with the
// module_query v0 result build returns (envelope and all). It is the shared
// fixture plumbing every test below uses so each test body only states the
// module_query-shaped result it cares about.
func serveModuleQuery(t *testing.T, build func(args moduleQueryArguments) modulesQueryResult) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, args := decodeToolsCallRequest(t, r)
		result := build(args)
		resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: wrapAsToolResult(t, result)}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("server: encode response: %v", err)
		}
	}))
}

// TestHTTPRetriever_HappyPath serves a fixed module_query v0 response,
// envelope and all, from an httptest server (no real network) and asserts
// the decoded, sorted result — one EvidenceItem per claim, inheriting its
// containing module result's module/watermark.
func TestHTTPRetriever_HappyPath(t *testing.T) {
	srv := serveModuleQuery(t, func(args moduleQueryArguments) modulesQueryResult {
		if args.Text != "how does X work?" {
			t.Fatalf("server: got text %q", args.Text)
		}
		return modulesQueryResult{Results: []wireModuleResult{
			// Deliberately out of sorted order — the client must sort.
			{
				Module: "zeta", Service: "svc-z", Watermark: "wm-z",
				Claims: []wireClaim{
					{ClaimID: "c1", Text: "t1", Src: "s1", Anchor: "a1", Tier: "verified", Status: StatusFresh, Kind: "finding"},
				},
			},
			{
				Module: "alpha", Service: "svc-a", Watermark: "wm-a",
				Claims: []wireClaim{
					{ClaimID: "c2", Text: "t2", Src: "s2", Anchor: "a2", Tier: "doc-claimed", Status: StatusStale, Kind: "finding"},
				},
			},
		}}
	})
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	items, err := r.Retrieve(t.Context(), "how does X work?", 0)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Module != "alpha" || items[1].Module != "zeta" {
		t.Fatalf("items not sorted by module: %+v", items)
	}
	if items[0].Watermark != "wm-a" || items[0].SourceRef != "s2" || items[0].Anchor != "a2" {
		t.Fatalf("claim did not inherit its containing module result's fields correctly: %+v", items[0])
	}
	if items[0].ModuleVersion != "" {
		t.Fatalf("ModuleVersion = %q, want empty (manifest loader unbuilt)", items[0].ModuleVersion)
	}
}

// TestHTTPRetriever_EmptyStatusNormalizedToFresh proves the module_query v0
// serving semantics ("" means fresh, not unknown) are honored at decode
// time — Citable()/filterFresh downstream must see StatusFresh, never "".
func TestHTTPRetriever_EmptyStatusNormalizedToFresh(t *testing.T) {
	srv := serveModuleQuery(t, func(moduleQueryArguments) modulesQueryResult {
		return modulesQueryResult{Results: []wireModuleResult{
			{Module: "m", Watermark: "wm", Claims: []wireClaim{
				{ClaimID: "c1", Text: "t", Src: "s", Anchor: "a", Tier: "verified", Status: ""},
			}},
		}}
	})
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	items, err := r.Retrieve(t.Context(), "q", 0)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Status != StatusFresh {
		t.Fatalf("Status = %q, want %q (empty wire status means fresh)", items[0].Status, StatusFresh)
	}
	if !items[0].Citable() {
		t.Fatal("an empty-status claim, normalized to fresh, must be Citable()")
	}
}

// TestHTTPRetriever_EmptyAnchorAllowed proves an empty Anchor (legacy
// hash-only claims, per fathom's own store format) decodes cleanly and does
// not make the item unsafe to cite — validCiteIdentity only requires
// Module/ClaimID/Tier, never Anchor.
func TestHTTPRetriever_EmptyAnchorAllowed(t *testing.T) {
	srv := serveModuleQuery(t, func(moduleQueryArguments) modulesQueryResult {
		return modulesQueryResult{Results: []wireModuleResult{
			{Module: "m", Watermark: "wm", Claims: []wireClaim{
				{ClaimID: "c1", Text: "t", Src: "s", Anchor: "", Tier: "verified", Status: StatusFresh},
			}},
		}}
	})
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	items, err := r.Retrieve(t.Context(), "q", 0)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(items) != 1 || items[0].Anchor != "" {
		t.Fatalf("items = %+v, want one item with an empty Anchor", items)
	}
	if !validCiteIdentity(items[0]) {
		t.Fatal("an item with an empty Anchor must still be a valid cite identity")
	}
}

// TestHTTPRetriever_NodesAndStalenessIgnored proves the informational
// nodes/staleness fields decode without error and never become
// EvidenceItems — symbols are not claims.
func TestHTTPRetriever_NodesAndStalenessIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := decodeToolsCallRequest(t, r)
		raw := `{"results":[{"module":"m","watermark":"wm",` +
			`"claims":[{"claim_id":"c1","text":"t","src":"s","anchor":"a","tier":"verified","status":"fresh"}],` +
			`"nodes":[{"symbol":"Foo","kind":"func"}],` +
			`"staleness":{"drifted":false}}]}`
		envelope, _ := json.Marshal(mcpToolResult{Content: []mcpContentItem{{Type: "text", Text: raw}}})
		resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: envelope}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	items, err := r.Retrieve(t.Context(), "q", 0)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(items) != 1 || items[0].ClaimID != "c1" {
		t.Fatalf("items = %+v, want exactly the one real claim (nodes must never become evidence)", items)
	}
}

// TestHTTPRetriever_PartialModuleDegradation_SkipsErroredModule proves a
// per-module "error" (one federated module unreachable while others answer)
// is skipped — excluded from the result — WITHOUT failing the whole
// Retrieve call.
func TestHTTPRetriever_PartialModuleDegradation_SkipsErroredModule(t *testing.T) {
	srv := serveModuleQuery(t, func(moduleQueryArguments) modulesQueryResult {
		return modulesQueryResult{Results: []wireModuleResult{
			{Module: "ok", Watermark: "wm", Claims: []wireClaim{
				{ClaimID: "c1", Text: "t", Src: "s", Anchor: "a", Tier: "verified", Status: StatusFresh},
			}},
			{Module: "down", Error: "connection refused"},
		}}
	})
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	items, err := r.Retrieve(t.Context(), "q", 0)
	if err != nil {
		t.Fatalf("Retrieve: %v (a partial degradation must not fail the whole call)", err)
	}
	if len(items) != 1 || items[0].Module != "ok" {
		t.Fatalf("items = %+v, want exactly the one claim from the healthy module", items)
	}
}

// TestHTTPRetriever_AllModulesErrored_Fails proves that when EVERY queried
// module reports an error, Retrieve fails loudly rather than silently
// returning an empty result indistinguishable from "the query legitimately
// matched nothing."
func TestHTTPRetriever_AllModulesErrored_Fails(t *testing.T) {
	srv := serveModuleQuery(t, func(moduleQueryArguments) modulesQueryResult {
		return modulesQueryResult{Results: []wireModuleResult{
			{Module: "a", Error: "timeout"},
			{Module: "b", Error: "connection refused"},
		}}
	})
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	_, err := r.Retrieve(t.Context(), "q", 0)
	if err == nil {
		t.Fatal("expected an error when every queried module errored")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error %v does not surface a module's error detail", err)
	}
}

// TestHTTPRetriever_RPCError proves an in-band JSON-RPC error (e.g. unknown
// tool — what a real fathom daemon returns for a method/tool it does not
// recognize) becomes a Go error, not a silently-empty result.
func TestHTTPRetriever_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "tool not found: " + moduleQueryToolName}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	_, err := r.Retrieve(t.Context(), "q", 0)
	if err == nil {
		t.Fatal("expected an error from an RPC error response")
	}
	if !strings.Contains(err.Error(), "tool not found") {
		t.Fatalf("error %v does not surface the RPC error message", err)
	}
}

// TestHTTPRetriever_ToolLevelIsError proves an in-band MCP tool error
// (isError:true inside the content envelope, as opposed to a JSON-RPC-level
// error) also becomes a Go error, not a silently-decoded empty result.
func TestHTTPRetriever_ToolLevelIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := decodeToolsCallRequest(t, r)
		envelope, _ := json.Marshal(mcpToolResult{
			Content: []mcpContentItem{{Type: "text", Text: "module not mounted"}},
			IsError: true,
		})
		resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: envelope}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	_, err := r.Retrieve(t.Context(), "q", 0)
	if err == nil {
		t.Fatal("expected an error for isError:true")
	}
	if !strings.Contains(err.Error(), "module not mounted") {
		t.Fatalf("error %v does not surface the tool error text", err)
	}
}

// TestHTTPRetriever_MissingContentEnvelope proves a malformed/empty content
// array is a loud error, never silently treated as "zero items."
func TestHTTPRetriever_MissingContentEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := decodeToolsCallRequest(t, r)
		envelope, _ := json.Marshal(mcpToolResult{})
		resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: envelope}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	if _, err := r.Retrieve(t.Context(), "q", 0); err == nil {
		t.Fatal("expected an error for a missing content envelope")
	}
}

// TestHTTPRetriever_HTTPErrorStatus proves a non-200 becomes a Go error.
func TestHTTPRetriever_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	if _, err := r.Retrieve(t.Context(), "q", 0); err == nil {
		t.Fatal("expected an error for HTTP 500")
	}
}

// TestHTTPRetriever_RequiresURL proves the retriever fails closed rather
// than silently calling a default host.
func TestHTTPRetriever_RequiresURL(t *testing.T) {
	r := &HTTPRetriever{}
	if _, err := r.Retrieve(t.Context(), "q", 0); err == nil {
		t.Fatal("expected an error when URL is empty")
	}
}

// TestHTTPRetriever_AppliesLimit proves the client-side limit truncation
// after sort, even if the server ignores Max.
func TestHTTPRetriever_AppliesLimit(t *testing.T) {
	srv := serveModuleQuery(t, func(moduleQueryArguments) modulesQueryResult {
		return modulesQueryResult{Results: []wireModuleResult{
			{Module: "a", Watermark: "wm", Claims: []wireClaim{{ClaimID: "c1", Tier: "verified", Status: StatusFresh}}},
			{Module: "b", Watermark: "wm", Claims: []wireClaim{{ClaimID: "c2", Tier: "verified", Status: StatusFresh}}},
			{Module: "c", Watermark: "wm", Claims: []wireClaim{{ClaimID: "c3", Tier: "verified", Status: StatusFresh}}},
		}}
	})
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL}
	items, err := r.Retrieve(t.Context(), "q", 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (limit applied)", len(items))
	}
}

// TestHTTPRetriever_ScopesToModuleAndPassesMax proves the client sends
// Module/Max through to the tool arguments verbatim (the pinned module_query
// v0 field names, not the pre-reconciliation "query"/"limit").
func TestHTTPRetriever_ScopesToModuleAndPassesMax(t *testing.T) {
	srv := serveModuleQuery(t, func(args moduleQueryArguments) modulesQueryResult {
		if args.Module != "agentic-kit" {
			t.Fatalf("server: got module %q, want %q", args.Module, "agentic-kit")
		}
		if args.Max != 5 {
			t.Fatalf("server: got max %d, want 5", args.Max)
		}
		return modulesQueryResult{}
	})
	defer srv.Close()

	r := &HTTPRetriever{URL: srv.URL, Module: "agentic-kit"}
	if _, err := r.Retrieve(t.Context(), "q", 5); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
}

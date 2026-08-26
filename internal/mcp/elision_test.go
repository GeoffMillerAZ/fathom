package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── BA-1/BA-5 fixture backend ──────────────────────────────────────────────

// elisionFakeBackend is a minimal QueryInterface whose lists are large
// enough to force truncation in every truncating tool, and whose
// FindSymbol always misses (so find_symbol degrades to the substring tier).
type elisionFakeBackend struct {
	nodes   []core.SymbolNode
	sources []core.SourceFile
}

func (f *elisionFakeBackend) QuerySymbols(_ context.Context, q core.Query) (core.QueryResult, error) {
	return core.QueryResult{Nodes: f.nodes}, nil
}

func (f *elisionFakeBackend) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *elisionFakeBackend) FindSymbol(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil // always miss: forces the substring-tier degrade path
}

func (f *elisionFakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return f.nodes, nil
}

func (f *elisionFakeBackend) Services(_ context.Context) ([]string, error) {
	return []string{"svc"}, nil
}

func (f *elisionFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return f.sources, nil
}
func (f *elisionFakeBackend) IngestService(_ context.Context, _ string) error { return nil }
func (f *elisionFakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *elisionFakeBackend) ListCommits(_ context.Context, _ string) ([]CommitInfo, error) {
	return nil, nil
}

func (f *elisionFakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return nil, nil
}

// manyNodes builds n synthetic symbols, all named so a "handler" substring
// query matches every one of them but caseFoldMatches matches none (no
// node is named exactly "handler" nor lives in a "handler" package).
func manyNodes(n int) []core.SymbolNode {
	nodes := make([]core.SymbolNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = core.SymbolNode{
			Service:   "svc",
			Package:   fmt.Sprintf("pkg%d", i),
			Name:      fmt.Sprintf("Handler%03d", i),
			Kind:      core.KindGoFunc,
			FilePath:  fmt.Sprintf("pkg%d/file.go", i),
			Line:      i + 1,
			Signature: "func()",
			Calls:     []string{fmt.Sprintf("pkg%d.Callee", i)},
		}
	}
	return nodes
}

// manySources builds one source file with n long lines each mentioning
// needle, so references.Find returns n occurrences whose combined JSON
// size exceeds the "references" budget default well before maxOccs (200).
func manySources(needle string, n int) []core.SourceFile {
	content := ""
	for i := 0; i < n; i++ {
		content += fmt.Sprintf("// line %03d padding padding padding padding padding %s call\n", i, needle)
	}
	return []core.SourceFile{{Path: "big.go", Content: []byte(content), Lang: "go"}}
}

func elisionTestServer() *Server {
	backend := &elisionFakeBackend{
		nodes:   manyNodes(60),
		sources: manySources("Needle", 80),
	}
	return NewServer(backend)
}

func callTool(t *testing.T, srv *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	req := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, hasResp := srv.HandleRequest(context.Background(), raw)
	if !hasResp {
		t.Fatalf("%s: no response", name)
	}
	var env map[string]any
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("%s: decode envelope: %v (raw=%s)", name, err, resp)
	}
	if env["error"] != nil {
		t.Fatalf("%s: rpc error: %v", name, env["error"])
	}
	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("%s: result missing: %v", name, env)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("%s: content missing: %v", name, result)
	}
	first := content[0].(map[string]any)
	var payload map[string]any
	if err := json.Unmarshal([]byte(first["text"].(string)), &payload); err != nil {
		t.Fatalf("%s: decode payload: %v", name, err)
	}
	return payload
}

// assertElided checks the disclosure block's shape (BA-1): count, of,
// ordering and how_to_get_more all present and non-degenerate.
func assertElided(t *testing.T, tool string, payload map[string]any) {
	t.Helper()
	raw, ok := payload["elided"]
	if !ok {
		t.Fatalf("%s: response carries no \"elided\" block though truncation was forced: %v", tool, payload)
	}
	el, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("%s: elided is not an object: %v", tool, raw)
	}
	count, _ := el["count"].(float64)
	of, _ := el["of"].(float64)
	ordering, _ := el["ordering"].(string)
	howToGetMore, _ := el["how_to_get_more"].(string)
	if count <= 0 {
		t.Errorf("%s: elided.count = %v, want > 0", tool, el["count"])
	}
	if of <= count {
		t.Errorf("%s: elided.of = %v, want > count (%v)", tool, el["of"], count)
	}
	if ordering == "" {
		t.Errorf("%s: elided.ordering is empty", tool)
	}
	if howToGetMore == "" {
		t.Errorf("%s: elided.how_to_get_more is empty", tool)
	}
}

// truncatingTools names every tool BA-1 mechanically identified as an
// existing clamp/slice site: query_symbols and references already truncated
// via fitToBudget; repo_map and pack_context already budget-truncate via
// internal/context/packer (repo_map's own disclosure was the defect BA-1
// fixes: previously NO count was reported at all); find_symbol's
// substring-tier fallback already capped at maxSubstringMatches. Every
// other registered tool is exempt because it returns a bounded, non-slicing
// result (a single lookup, one file's clamped byte/line range — a
// continuous read, not a countable "N of M" list — a boolean, a fixed-shape
// report, …).
//
// Two exemptions worth naming explicitly because they DO truncate today but
// do not fit elisionFor's item-count contract:
//   - read_source clamps a byte/line RANGE within one file (ClampedEOF/Note
//     already disclose it) — there is no "total items" to count.
//   - brief truncates prose to a char budget with its own inline notice —
//     continuous text, no discrete list either.
//
// obligations' `displaced`/`displaced_repo`/`displaced_module` fields are a
// pre-existing, differently-shaped disclosure for a shared cross-layer cap;
// left as-is rather than reshaped into `elided` here.
var truncatingTools = map[string]bool{
	"memory_search": true, // proven live in memory_tools_test.go (TestMemorySearchOverTheServer)
	"query_symbols": true,
	"references":    true,
	"repo_map":      true,
	"pack_context":  true,
	"find_symbol":   true,
}

// nonTruncatingTools is every other tool the registry currently serves,
// named explicitly (not derived as "everything else") so that a genuinely
// NEW tool — absent from both lists — fails TestEveryTruncatingToolDiscloses
// loudly instead of silently passing as exempt.
var nonTruncatingTools = map[string]bool{
	"find_callers": true, "rename_impact": true, "reachable": true,
	"test_impact": true, "dead_code": true, "find_callees": true,
	"symbol_context": true, "outline": true, "entrypoints": true,
	"complexity": true, "god_object": true, "community": true,
	"secret_scan": true, "shell_out": true, "http_client_calls": true,
	"cloc": true, "read_source": true, "capabilities": true,
	"server_stats": true, "staleness_report": true, "list_sources": true,
	"blast_radius": true, "diff_impact": true, "spec_coverage": true,
	"traceability": true, "evidence_summary": true, "contradiction_scan": true,
	"verify_plan": true, "review_diff": true, "findings_near": true,
	"arch_conformance": true, "stale_docs": true, "api_diff": true,
	"dep_drift": true, "ingest": true, "remove_service": true,
	"rename_service": true, "cross_repo_blast_radius": true, "batch": true,
	"proto_conformance": true, "spec_refs": true, "brief": true,
	"traverse": true, "history": true, "obligations": true,
	"compile_layer": true, "list_services": true, "fleet_drift": true,
	"cross_repo_callers": true,
}

// TestEveryTruncatingToolDiscloses is BA-1's registry-iteration test: every
// registered tool must be explicitly classified as truncating (and proven,
// live, to emit `elided` when forced) or explicitly exempt — a tool in
// neither list fails loudly, so a new truncating tool cannot be added
// without wiring elisionFor.
func TestEveryTruncatingToolDiscloses(t *testing.T) {
	srv := elisionTestServer()
	for _, te := range srv.tools {
		name := te.desc.Name
		switch {
		case truncatingTools[name]:
			// proven live below
		case nonTruncatingTools[name]:
			// exempt — does not slice/limit a countable result list
		default:
			t.Errorf("tool %q is registered but categorized as neither truncating nor exempt — classify it in internal/mcp/elision_test.go (BA-1)", name)
		}
	}
	// Every name in truncatingTools must actually be registered — a typo or
	// a renamed tool here would otherwise pass by never being reached.
	registered := map[string]bool{}
	for _, te := range srv.tools {
		registered[te.desc.Name] = true
	}
	for name := range truncatingTools {
		if !registered[name] {
			t.Errorf("truncatingTools names %q but the registry does not serve it", name)
		}
	}

	assertElided(t, "query_symbols", callTool(t, srv, "query_symbols", map[string]any{"service": "svc", "concern": "handler", "top": 60}))
	assertElided(t, "references", callTool(t, srv, "references", map[string]any{"query": "Needle"}))
	assertElided(t, "repo_map", callTool(t, srv, "repo_map", map[string]any{"service": "svc", "budget": 50}))
	assertElided(t, "pack_context", callTool(t, srv, "pack_context", map[string]any{"service": "svc", "concern": "handler", "budget": 50}))
	assertElided(t, "find_symbol", callTool(t, srv, "find_symbol", map[string]any{"service": "svc", "name": "handler"}))
}

// ─── BA-2: the arithmetic is checked at construction ────────────────────────

func TestElisionArithmetic(t *testing.T) {
	cases := []struct {
		returned, total int
		wantNil         bool
	}{
		{returned: 12, total: 47, wantNil: false},
		{returned: 0, total: 5, wantNil: false},
		{returned: 5, total: 5, wantNil: true}, // complete answer: no block
		{returned: 5, total: 3, wantNil: true}, // defensive: returned>total treated as complete
		{returned: 0, total: 0, wantNil: true},
	}
	for _, c := range cases {
		el := elisionFor(c.returned, c.total, "some order", "re-call bigger")
		if c.wantNil {
			if el != nil {
				t.Errorf("elisionFor(%d,%d) = %+v, want nil", c.returned, c.total, el)
			}
			continue
		}
		if el == nil {
			t.Fatalf("elisionFor(%d,%d) = nil, want a block", c.returned, c.total)
		}
		if el.Count+c.returned != c.total {
			t.Errorf("elisionFor(%d,%d): count(%d) + returned(%d) = %d, want %d",
				c.returned, c.total, el.Count, c.returned, el.Count+c.returned, c.total)
		}
		if el.Of != c.total {
			t.Errorf("elisionFor(%d,%d): of = %d, want %d", c.returned, c.total, el.Of, c.total)
		}
	}
}

// ─── BA-3: ordering is deterministic and nested ─────────────────────────────

func TestBudgetPrefixProperty(t *testing.T) {
	items := make([]budItem, 40)
	for i := range items {
		items[i] = budItem{S: fmt.Sprintf("item-%02d-xxxxxxxxxxxxxxxx", i)}
	}
	small, elSmall := applyBudget(items, 300, "insertion order", "widen the budget")
	large, elLarge := applyBudget(items, 3000, "insertion order", "widen the budget")

	if len(small) == 0 || len(small) >= len(items) {
		t.Fatalf("small budget kept %d of %d items, want a nonzero partial prefix", len(small), len(items))
	}
	if len(large) != len(items) {
		t.Fatalf("large budget kept %d of %d items, want all", len(large), len(items))
	}
	if elSmall == nil {
		t.Fatal("small budget: expected an elided block")
	}
	if elLarge != nil {
		t.Fatalf("large budget: expected no elided block, got %+v", elLarge)
	}
	if len(small) > len(large) {
		t.Fatalf("small budget kept MORE items (%d) than large budget (%d)", len(small), len(large))
	}
	for i, it := range small {
		if it != large[i] {
			t.Fatalf("small budget's result at index %d (%+v) is not a prefix of the large budget's (%+v)", i, it, large[i])
		}
	}
}

// ─── BA-4: a budget is a target, never a refusal ────────────────────────────

func TestBudgetNeverRefuses(t *testing.T) {
	items := []budItem{{S: "a string longer than one char"}, {S: "second"}}
	got, el := applyBudget(items, 1, "insertion order", "widen the budget")
	if len(got) == 0 {
		t.Fatal("applyBudget with an impossibly small budget refused (returned zero items) — a budget is a target, never a hard cap")
	}
	if len(got) != 1 {
		t.Fatalf("got %d items, want exactly the one minimum-useful item", len(got))
	}
	if el == nil {
		t.Fatal("expected an elided block noting the budget was exceeded")
	}
	if el.HowToGetMore == "" {
		t.Error("elided.how_to_get_more must say what to do about the exceeded budget")
	}
}

// ─── BA-5: per-tool defaults resolve project → user → built-in ─────────────

func TestBudgetDefaultsFromRegistry(t *testing.T) {
	// No configuration at either scope: the design's measured default.
	if got := budgetDefaults("query_symbols", budgetSettings{}); got != 1500*charsPerToken {
		t.Errorf("query_symbols built-in default = %d, want %d", got, 1500*charsPerToken)
	}
	if got := budgetDefaults("references", budgetSettings{}); got != 1500*charsPerToken {
		t.Errorf("references built-in default = %d, want %d", got, 1500*charsPerToken)
	}
	if got, ok := budgetDefaultTokens("repo_map", budgetSettings{}); !ok || got != 2500 {
		t.Errorf("repo_map built-in default = %d/%v, want 2500/true", got, ok)
	}
	if got, ok := budgetDefaultTokens("pack_context", budgetSettings{}); !ok || got != 4000 {
		t.Errorf("pack_context built-in default = %d/%v, want 4000/true", got, ok)
	}
	// A tool with no declared class resolves to 0 (caller's own default
	// applies) rather than fabricating one.
	if got, ok := budgetDefaultTokens("find_symbol", budgetSettings{}); ok || got != 0 {
		t.Errorf("find_symbol (no declared class) = %d/%v, want 0/false", got, ok)
	}

	// User scope may set a default; project scope overrides it — law 13's
	// most-specific-wins precedence, mirroring capability.Resolution.For.
	userOnly := budgetSettings{User: map[string]int{"list": 900}}
	if got := budgetDefaults("query_symbols", userOnly); got != 900*charsPerToken {
		t.Errorf("user-scope override = %d, want %d", got, 900*charsPerToken)
	}
	both := budgetSettings{
		User:    map[string]int{"list": 900},
		Project: map[string]int{"list": 3000},
	}
	if got := budgetDefaults("query_symbols", both); got != 3000*charsPerToken {
		t.Errorf("project must win over user: got %d, want %d", got, 3000*charsPerToken)
	}
}

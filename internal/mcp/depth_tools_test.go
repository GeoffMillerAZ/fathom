package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// graphBackend is a fakeBackend with real call edges: Target ← Direct1
// (syntax only) and Target ← Direct2 (syntax + typed EdgeDirect); Deep
// calls Direct1; TestDeep calls Deep. Sources carry one mention of Target
// inside Direct2 (line 12) and one in a doc file (BD-2 fixtures).
type graphBackend struct {
	*fakeBackend
	nodes []core.SymbolNode
	typed []core.Edge
}

func newGraphBackend() *graphBackend {
	n := func(name, file string, line int, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "pkg", Name: name, Kind: core.KindGoFunc, FilePath: file, Line: line, Calls: calls, Tier: core.TierVerified}
	}
	return &graphBackend{
		fakeBackend: &fakeBackend{},
		nodes: []core.SymbolNode{
			n("Target", "a.go", 3),
			n("Direct1", "b.go", 3, "svc::pkg.Target"),
			n("Direct2", "b.go", 10, "svc::pkg.Target"),
			n("Deep", "c.go", 3, "svc::pkg.Direct1"),
			n("TestDeep", "c_test.go", 3, "svc::pkg.Deep"),
		},
		typed: []core.Edge{{FromService: "svc", FromSymbol: "svc::pkg.Direct2", ToSymbol: "svc::pkg.Target", Kind: core.EdgeDirect}},
	}
}

func (g *graphBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return g.nodes, nil
}

func (g *graphBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return g.typed, nil
}

func (g *graphBackend) Services(_ context.Context) ([]string, error) { return []string{"svc"}, nil }

func (g *graphBackend) FindCallers(_ context.Context, _, symbol string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range g.nodes {
		for _, c := range n.Calls {
			if c == "svc::pkg."+symbol {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

func (g *graphBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return []core.SourceFile{
		{Path: "b.go", Content: []byte("package pkg\n\nfunc Direct1() {\n\tTarget()\n}\n\n\n\n\nfunc Direct2() {\n\tx := 1\n\tTarget()\n}\n")},
		{Path: "README.md", Content: []byte("Call Target first.\n")},
	}, nil
}

func callTool(t *testing.T, srv *mcp.Server, name string, args map[string]any) map[string]any {
	t.Helper()
	resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": name, "arguments": args}})
	if resp["error"] != nil {
		t.Fatalf("%s error: %v", name, resp["error"])
	}
	text := resp["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	return payload
}

// BD-2: find_callers rows carry depth + edge_confidence, by_depth summarises
// them, and depth > 1 adds transitive_callers.
//
// Every call here passes include_heuristic: true. What BD-2 pins is the
// LABEL — that a syntax-only caller and a typed-pass caller are told apart
// in the response — and that is only observable when both are present.
// BP-26 changed which set the DEFAULT returns, not what the rows say; the
// default's own behaviour is pinned by TestFindCallersResolvedByDefault.
func TestFindCallersDepthAndConfidence(t *testing.T) {
	srv := mcp.NewServer(newGraphBackend())
	payload := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target", "include_heuristic": true})
	callers := payload["callers"].([]any)
	if len(callers) != 2 {
		t.Fatalf("callers = %v, want Direct1 and Direct2", callers)
	}
	byName := map[string]map[string]any{}
	for _, c := range callers {
		m := c.(map[string]any)
		byName[m["name"].(string)] = m
	}
	if byName["Direct1"]["depth"] != float64(1) || byName["Direct1"]["edge_confidence"] != "heuristic" {
		t.Errorf("Direct1 = %v: want depth 1, heuristic (syntax-only edge)", byName["Direct1"])
	}
	if byName["Direct2"]["edge_confidence"] != "resolved" {
		t.Errorf("Direct2 = %v: want resolved (typed pass confirmed the edge)", byName["Direct2"])
	}
	buckets := payload["by_depth"].([]any)
	if len(buckets) != 1 || buckets[0].(map[string]any)["resolved"] != float64(1) || buckets[0].(map[string]any)["heuristic"] != float64(1) {
		t.Errorf("by_depth = %v: want one depth-1 bucket, 1 resolved + 1 heuristic", buckets)
	}
	if _, ok := payload["transitive_callers"]; ok {
		t.Error("depth 1 (default) must not add transitive_callers")
	}

	deep := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target", "depth": 0, "include_heuristic": true})
	trans := deep["transitive_callers"].([]any)
	if len(trans) != 2 || trans[0].(map[string]any)["key"] != "svc::pkg.Deep" || trans[1].(map[string]any)["depth"] != float64(3) {
		t.Errorf("transitive_callers = %v: want Deep@2 then TestDeep@3", trans)
	}
	if b := deep["by_depth"].([]any); len(b) != 3 {
		t.Errorf("by_depth (unbounded) = %v: want depths 1..3", b)
	}

	// Determinism: identical calls marshal identically.
	a, _ := json.Marshal(callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target", "depth": 0, "include_heuristic": true}))
	b, _ := json.Marshal(deep)
	if string(a) != string(b) {
		t.Fatalf("find_callers not deterministic:\n%s\n%s", a, b)
	}
}

// BD-3: blast_radius (and its diff_impact alias) group the radius by depth.
//
// blast_radius is asked for the unbounded both-kinds walk explicitly
// (BP-27 made resolved/depth-2 the default); diff_impact still IS that
// walk, so it takes no arguments. The bucket shape under the new defaults
// is pinned by TestBlastRadiusResolvedByDefault.
func TestBlastRadiusByDepthField(t *testing.T) {
	srv := mcp.NewServer(newGraphBackend())
	for _, tool := range []string{"blast_radius", "diff_impact"} {
		args := map[string]any{"service": "svc", "changed_files": []string{"a.go"}}
		if tool == "blast_radius" {
			args["include_heuristic"] = true
			args["max_depth"] = 0
			args["max_symbols"] = 0
		}
		payload := callTool(t, srv, tool, args)
		buckets, ok := payload["by_depth"].([]any)
		if !ok || len(buckets) != 4 {
			t.Fatalf("%s by_depth = %v, want depths 0..3", tool, payload["by_depth"])
		}
		d1 := buckets[1].(map[string]any)
		if d1["depth"] != float64(1) || d1["resolved"] != float64(1) || d1["heuristic"] != float64(1) {
			t.Errorf("%s depth-1 bucket = %v", tool, d1)
		}
		if syms := payload["symbols"].([]any); len(syms) != 5 {
			t.Errorf("%s symbols = %v: the flat field must be unchanged", tool, syms)
		}
	}
}

// BD-2: references rows carry depth (0 = definition, 1 = mention) and
// edge_confidence: a mention inside a symbol with a typed edge to the
// definition is resolved; a doc mention is heuristic.
func TestReferencesDepthAndConfidence(t *testing.T) {
	srv := mcp.NewServer(newGraphBackend())
	payload := callTool(t, srv, "references", map[string]any{"query": "Target", "word": true})
	defs := payload["definitions"].([]any)
	if len(defs) != 1 || defs[0].(map[string]any)["depth"] != float64(0) || defs[0].(map[string]any)["edge_confidence"] != "resolved" {
		t.Fatalf("definitions = %v: want Target at depth 0, resolved (verified tier)", defs)
	}
	// SB-1: depth and tier are lifted to the envelope (occurrences are
	// always pre-scan/depth 1); confidence stays per-row, collapsed to one
	// character ("r"/"h").
	if depth, _ := payload["occurrence_depth"].(float64); depth != 1 {
		t.Errorf("occurrence_depth = %v, want 1", payload["occurrence_depth"])
	}
	occs := payload["occurrences"].([]any)
	got := map[string]string{}
	for _, o := range occs {
		m := o.(map[string]any)
		loc, _ := m["loc"].(string)
		got[loc] = m["c"].(string)
	}
	if got["b.go:12"] != "r" {
		t.Errorf("mention inside Direct2 (typed edge to Target) = %q, want r (resolved); all: %v", got["b.go:12"], got)
	}
	if got["b.go:4"] != "h" {
		t.Errorf("mention inside Direct1 (syntax-only edge) = %q, want h (heuristic); all: %v", got["b.go:4"], got)
	}
	if got["README.md:1"] != "h" {
		t.Errorf("doc mention = %q, want h (heuristic); all: %v", got["README.md:1"], got)
	}
	buckets := payload["by_depth"].([]any)
	if len(buckets) != 2 || buckets[0].(map[string]any)["depth"] != float64(0) || buckets[1].(map[string]any)["count"] != float64(len(occs)) {
		t.Errorf("by_depth = %v: want depth 0 (1 def) and depth 1 (%d mentions)", buckets, len(occs))
	}
}

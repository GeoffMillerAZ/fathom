package mcp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── fixture ─────────────────────────────────────────────────────────────────
//
// Three functions in one service:
//
//	Real   — calls Target, and the typed pass CONFIRMED it (EdgeDirect).
//	Ghost  — its Calls list names "Target" too, but the typed pass resolved
//	         that call site to something else, so no typed edge exists. This
//	         is the shape of every false positive the L0 oracle measured:
//	         a bare-name match the syntax pass believed.
//	Target — the callee.
//
// The resolved default must return Real and not Ghost; include_heuristic
// must return both; and the verify engine must keep seeing both regardless.

type navFake struct {
	nodes     []core.SymbolNode
	typed     []core.Edge
	watermark string
	marks     int
}

func newNavFake() *navFake {
	n := func(name, file string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "p", Name: name, Kind: core.KindGoFunc, FilePath: file, Calls: calls}
	}
	f := &navFake{
		nodes: []core.SymbolNode{
			n("Real", "p/real.go", "Target"),
			n("Ghost", "p/ghost.go", "Target"),
			n("Target", "p/target.go"),
			n("TestReal", "p/real_test.go", "Real"),
		},
		watermark: "commit1@1",
	}
	byName := map[string]string{}
	for _, nd := range f.nodes {
		byName[nd.Name] = nd.Key()
	}
	f.typed = []core.Edge{
		{FromService: "svc", FromSymbol: byName["Real"], ToSymbol: byName["Target"], Kind: core.EdgeDirect},
		{FromService: "svc", FromSymbol: byName["TestReal"], ToSymbol: byName["Real"], Kind: core.EdgeDirect},
	}
	return f
}

func (f *navFake) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (f *navFake) FindCallers(_ context.Context, _, symbol string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range f.nodes {
		for _, c := range n.Calls {
			if c == symbol {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

func (f *navFake) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range f.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *navFake) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	return f.nodes, nil
}
func (f *navFake) Services(context.Context) ([]string, error) { return []string{"svc"}, nil }
func (f *navFake) Sources(context.Context, string) ([]core.SourceFile, error) {
	return nil, nil
}
func (f *navFake) IngestService(context.Context, string) error { return nil }
func (f *navFake) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return "", "", false, nil
}
func (f *navFake) ListCommits(context.Context, string) ([]mcp.CommitInfo, error) { return nil, nil }
func (f *navFake) TypedEdges(context.Context, string) ([]core.Edge, error)       { return f.typed, nil }

// StoreWatermark makes the fake eligible for the BP-28 graph memo.
func (f *navFake) StoreWatermark(context.Context, string) (string, error) {
	f.marks++
	return f.watermark, nil
}

func callerNames(t *testing.T, payload map[string]any) []string {
	t.Helper()
	rows, _ := payload["callers"].([]any)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.(map[string]any)["name"].(string))
	}
	return out
}

// ─── BP-26 ───────────────────────────────────────────────────────────────────

// TestFindCallersResolvedByDefault is BP-26: the default answer carries only
// edges the typed pass confirmed, and says so.
func TestFindCallersResolvedByDefault(t *testing.T) {
	srv := mcp.NewServer(newNavFake())
	got := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})

	if got["edge_filter"] != "resolved" {
		t.Errorf("edge_filter = %v, want resolved", got["edge_filter"])
	}
	names := callerNames(t, got)
	if len(names) != 1 || names[0] != "Real" {
		t.Fatalf("callers = %v, want [Real] — the syntax pass's bare-name caller Ghost must not be in the default answer", names)
	}
	rows := got["callers"].([]any)
	if c := rows[0].(map[string]any)["edge_confidence"]; c != "resolved" {
		t.Errorf("edge_confidence = %v, want resolved — BD-2's label must survive BP-26", c)
	}
}

// TestFindCallersIncludeHeuristic: the old behaviour is still reachable, and
// still labeled.
func TestFindCallersIncludeHeuristic(t *testing.T) {
	srv := mcp.NewServer(newNavFake())
	got := callTool(t, srv, "find_callers", map[string]any{
		"service": "svc", "symbol": "Target", "include_heuristic": true,
	})
	if got["edge_filter"] != "all" {
		t.Errorf("edge_filter = %v, want all", got["edge_filter"])
	}
	names := callerNames(t, got)
	if len(names) != 2 {
		t.Fatalf("callers = %v, want both Ghost and Real", names)
	}
	var sawHeuristic bool
	for _, r := range got["callers"].([]any) {
		if r.(map[string]any)["edge_confidence"] == "heuristic" {
			sawHeuristic = true
		}
	}
	if !sawHeuristic {
		t.Error("no row labeled heuristic — include_heuristic returned rows with no way to tell them apart")
	}
}

// TestFindCallersFallsBackWithoutTypedEdges: a service with no typed pass
// must not get an empty answer dressed up as a resolved one.
func TestFindCallersFallsBackWithoutTypedEdges(t *testing.T) {
	f := newNavFake()
	f.typed = nil
	srv := mcp.NewServer(f)
	got := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})
	if got["edge_filter"] != "all" {
		t.Errorf("edge_filter = %v, want all — a corpus with no typed edges cannot answer at resolved confidence", got["edge_filter"])
	}
	if names := callerNames(t, got); len(names) != 2 {
		t.Errorf("callers = %v, want both — the fallback must not silently drop the only evidence there is", names)
	}
}

// ─── BP-27 ───────────────────────────────────────────────────────────────────

// TestBlastRadiusResolvedByDefault is BP-27: resolved edges, a depth cap,
// and an answer that reports its own bounds.
func TestBlastRadiusResolvedByDefault(t *testing.T) {
	srv := mcp.NewServer(newNavFake())
	got := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"p/target.go"},
	})
	if got["edge_filter"] != "resolved" {
		t.Fatalf("edge_filter = %v, want resolved", got["edge_filter"])
	}
	syms, _ := got["symbols"].([]any)
	for _, s := range syms {
		if key, _ := s.(string); key != "" && strings.HasSuffix(key, "Ghost") {
			t.Errorf("symbols include Ghost (%s) — a syntax-only edge walked into the default blast radius", key)
		}
	}
	for _, field := range []string{"total_symbols", "truncated", "depth_capped", "max_depth"} {
		if _, ok := got[field]; !ok {
			t.Errorf("response has no %q — a bounded answer that does not report its bounds reads as a complete one", field)
		}
	}
}

// TestBlastRadiusDepthCapIsReported: the cap must be visible, not silent.
func TestBlastRadiusDepthCapIsReported(t *testing.T) {
	srv := mcp.NewServer(newNavFake())
	got := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"p/target.go"}, "max_depth": 1,
	})
	if got["depth_capped"] != true {
		t.Errorf("depth_capped = %v, want true — TestReal reaches Target at depth 2 and was cut", got["depth_capped"])
	}
	if md, _ := got["max_depth"].(float64); md != 1 {
		t.Errorf("max_depth = %v, want 1", got["max_depth"])
	}
}

// TestBlastRadiusSymbolCapIsReported: a truncated listing says so and keeps
// its full count.
func TestBlastRadiusSymbolCapIsReported(t *testing.T) {
	srv := mcp.NewServer(newNavFake())
	got := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"p/target.go"}, "max_symbols": 1, "max_depth": 0,
	})
	if got["truncated"] != true {
		t.Fatalf("truncated = %v, want true", got["truncated"])
	}
	total, _ := got["total_symbols"].(float64)
	if int(total) <= len(got["symbols"].([]any)) {
		t.Errorf("total_symbols = %v but %d symbols listed — the full count must survive the cap",
			got["total_symbols"], len(got["symbols"].([]any)))
	}
}

// ─── BP-28 ───────────────────────────────────────────────────────────────────

// TestNavGraphMemoIsWatermarkKeyed: two nav calls on an unchanged index
// consult the watermark and reuse the graph; a re-ingest changes it.
func TestNavGraphMemoIsWatermarkKeyed(t *testing.T) {
	f := newNavFake()
	srv := mcp.NewServer(f)
	callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})
	first := f.marks
	if first == 0 {
		t.Fatal("the handler never asked for a store watermark — the memo cannot be keyed on one")
	}
	callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})
	if f.marks <= first {
		t.Error("the second call did not consult the watermark; a memo that never re-checks would survive an ingest")
	}
	// A re-ingest moves the watermark; the answer must still be correct.
	f.watermark = "commit2@2"
	got := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})
	if names := callerNames(t, got); len(names) != 1 || names[0] != "Real" {
		t.Errorf("after a re-ingest callers = %v, want [Real]", names)
	}
}

// ─── the line that must NOT move ─────────────────────────────────────────────

// TestVerifyReachabilityUsesBothEdgeKinds is the guard on BP-26/BP-27's
// blast radius: the agent-facing defaults changed, the verify gate's
// reachability did not. Ghost reaches Target through a syntax-only edge and
// verify_plan must still say so — otherwise a plan row that verified
// yesterday silently fails today because a DISPLAY default moved.
func TestVerifyReachabilityUsesBothEdgeKinds(t *testing.T) {
	srv := mcp.NewServer(newNavFake())
	got := callTool(t, srv, "verify_plan", map[string]any{
		"service": "svc",
		"tasks": []any{map[string]any{
			"id":             "R-1",
			"claimed":        true,
			"symbol":         "Target",
			"reachable_from": "Ghost",
		}},
	})
	tasks, _ := got["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("verify_plan returned %d tasks, want 1: %v", len(tasks), got)
	}
	var edgeWired map[string]any
	for _, c := range tasks[0].(map[string]any)["criteria"].([]any) {
		if m := c.(map[string]any); m["name"] == "edge_wired" {
			edgeWired = m
		}
	}
	if edgeWired == nil {
		t.Fatalf("no edge_wired criterion: %v", tasks[0])
	}
	if edgeWired["status"] != "pass" {
		t.Fatalf("edge_wired = %v, want pass — Ghost reaches Target only through a SYNTAX edge, and the verify engine must still walk those. If this fails, a display default changed a gate verdict.", edgeWired)
	}
}

// TestReferencesDefinitionsExactByDefault is BP-26's references half: an
// index symbol whose name merely CONTAINS the query is not a definition of
// it, and is opt-in.
func TestReferencesDefinitionsExactByDefault(t *testing.T) {
	f := newNavFake()
	f.nodes = append(f.nodes, core.SymbolNode{
		Service: "svc", Package: "p", Name: "TargetHelper", Kind: core.KindGoFunc, FilePath: "p/helper.go",
	})
	srv := mcp.NewServer(f)

	got := callTool(t, srv, "references", map[string]any{"service": "svc", "query": "Target", "services": []string{"svc"}})
	if got["edge_filter"] != "resolved" {
		t.Errorf("edge_filter = %v, want resolved", got["edge_filter"])
	}
	for _, d := range got["definitions"].([]any) {
		if name := d.(map[string]any)["symbol"]; name == "TargetHelper" {
			t.Errorf("TargetHelper listed as a definition of Target — a containment hit is not a reference")
		}
	}

	wide := callTool(t, srv, "references", map[string]any{
		"service": "svc", "query": "Target", "services": []string{"svc"}, "include_heuristic": true,
	})
	if wide["edge_filter"] != "all" {
		t.Errorf("edge_filter = %v, want all", wide["edge_filter"])
	}
	var sawHelper bool
	for _, d := range wide["definitions"].([]any) {
		if d.(map[string]any)["symbol"] == "TargetHelper" {
			sawHelper = true
		}
	}
	if !sawHelper {
		t.Error("include_heuristic did not restore the substring definition match")
	}
}

// TestFindCallersAmbiguousNameStaysResolved is BP-31 at the tool surface:
// before it, a symbol name two packages share resolved to nothing in the
// resolved graph, resolvedCallersOnly reported "cannot answer", and
// find_callers silently returned the SYNTAX answer — the imprecise answer
// BP-26 exists to stop returning. The L0 oracle lost 19 of 327 true callers
// to exactly this.
func TestFindCallersAmbiguousNameStaysResolved(t *testing.T) {
	f := newNavFake()
	// A second package also declares Target, with its own resolved caller,
	// and a bystander whose Calls name Target without a typed edge.
	extra := []core.SymbolNode{
		{Service: "svc", Package: "q", Name: "Target", Kind: core.KindGoFunc, FilePath: "q/target.go"},
		{Service: "svc", Package: "q", Name: "RealQ", Kind: core.KindGoFunc, FilePath: "q/real.go", Calls: []string{"Target"}},
	}
	f.nodes = append(f.nodes, extra...)
	byName := map[string]string{}
	for _, n := range f.nodes {
		byName[n.Package+"."+n.Name] = n.Key()
	}
	f.typed = append(f.typed, core.Edge{
		FromService: "svc", FromSymbol: byName["q.RealQ"], ToSymbol: byName["q.Target"], Kind: core.EdgeDirect,
	})
	srv := mcp.NewServer(f)

	got := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "Target"})
	if got["edge_filter"] != "resolved" {
		t.Fatalf("edge_filter = %v, want resolved — an ambiguous name must not silently degrade to the syntax answer", got["edge_filter"])
	}
	names := callerNames(t, got)
	want := map[string]bool{"Real": true, "RealQ": true}
	if len(names) != 2 {
		t.Fatalf("callers = %v, want the resolved callers of BOTH Targets", names)
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected caller %q in %v — Ghost has no typed edge and must stay out", n, names)
		}
	}
}

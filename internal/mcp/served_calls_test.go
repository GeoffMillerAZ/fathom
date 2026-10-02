package mcp_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// newServedFake mirrors internal/analysis/callgraph/testdata/servedcalls
// (wave lg, LB-F199/LB-F200): Same calls only dependencies (bytes.Equal, a
// sync.Mutex's Lock) that the syntax pass binds by bare name to the
// service's one Equal and one Lock; Compare calls Store.Equal for real;
// Build calls NewConfig and references the type Config; gen.go is
// build-excluded (no typed edge from it), so its m.Lock edge stays.
func newServedFake() *navFake {
	n := func(pkg, name, file string, kind core.SourceKind, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: pkg, Name: name, Kind: kind, FilePath: file, Line: 1, Calls: calls, Tier: core.TierVerified}
	}
	f := &navFake{
		nodes: []core.SymbolNode{
			n("svc", "Store", "svc/store.go", core.KindGoType),
			n("Store", "Store.Equal", "svc/store.go", core.KindGoMethod),
			n("Store", "Store.Lock", "svc/store.go", core.KindGoMethod),
			n("svc", "Config", "svc/store.go", core.KindGoType),
			n("svc", "NewConfig", "svc/store.go", core.KindGoFunc),
			n("app", "Same", "app/app.go", core.KindGoFunc, "bytes.Equal", "mu.Lock"),
			n("app", "Compare", "app/app.go", core.KindGoFunc, "x.Equal"),
			n("app", "Build", "app/app.go", core.KindGoFunc, "svc.Config", "svc.NewConfig"),
			n("app", "TestBuild", "app/app_test.go", core.KindGoFunc, "Build", "t.Fatal"),
			n("main", "main", "tools/gen/gen.go", core.KindGoFunc, "m.Lock"),
		},
		watermark: "served@1",
	}
	key := map[string]string{}
	for _, nd := range f.nodes {
		key[nd.Name] = nd.Key()
	}
	e := func(from, to, file string, kind core.EdgeKind) core.Edge {
		return core.Edge{FromService: "svc", FromFile: file, FromSymbol: key[from], ToSymbol: key[to], Kind: kind}
	}
	f.typed = []core.Edge{
		e("Build", "NewConfig", "app/app.go", core.EdgeDirect),
		e("Compare", "Store.Equal", "app/app.go", core.EdgeDirect),
		e("Compare", "Store", "app/app.go", core.EdgeInstantiates),
		e("TestBuild", "Build", "app/app_test.go", core.EdgeDirect),
	}
	return f
}

func anyStrings(v any) []string {
	xs, _ := v.([]any)
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		s, _ := x.(string)
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// TestSymbolContextServesCallsNotGuessesOrTypeRefs is wave lg's proof on
// symbol_context: callers and callees read the served graph, where a typed
// file's bare-name guess (Same's bytes.Equal bound to Store.Equal) is gone
// and a type's consumers are references (referenced_by), not callers.
func TestSymbolContextServesCallsNotGuessesOrTypeRefs(t *testing.T) {
	srv := mcp.NewServer(newServedFake())
	ctx := func(sym string) map[string]any {
		return callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": sym, "snippet": false})
	}
	if got := anyStrings(ctx("Store.Equal")["callers"]); !reflect.DeepEqual(got, []string{"svc::app/app.Compare"}) {
		t.Errorf("Store.Equal callers = %v, want only Compare: Same's bytes.Equal is a dependency call, not a call to the service's Equal", got)
	}
	if got := anyStrings(ctx("Store.Lock")["callers"]); !reflect.DeepEqual(got, []string{"svc::tools/gen/main.main"}) {
		t.Errorf("Store.Lock callers = %v, want only gen.go's main (build-excluded: the typed pass never saw its site)", got)
	}
	cfg := ctx("Config")
	if got := anyStrings(cfg["callers"]); len(got) != 0 {
		t.Errorf("Config callers = %v, want none: a type reference is not a call", got)
	}
	if got := anyStrings(cfg["referenced_by"]); !reflect.DeepEqual(got, []string{"svc::app/app.Build"}) {
		t.Errorf("Config referenced_by = %v, want [Build]: a type's consumers are never lost", got)
	}
	build := ctx("Build")
	if got := anyStrings(build["callees"]); !reflect.DeepEqual(got, []string{"svc::svc/svc.NewConfig"}) {
		t.Errorf("Build callees = %v, want [NewConfig]", got)
	}
	if got := anyStrings(build["references"]); !reflect.DeepEqual(got, []string{"svc::svc/svc.Config"}) {
		t.Errorf("Build references = %v, want [Config]", got)
	}
}

// TestBlastRadiusHeuristicKeepsTypeConsumersAsReferences is wave lg's
// proof on blast_radius with include_heuristic: the radius of a change to
// svc/store.go no longer holds Same (its edges were bare-name guesses from
// a file the typed pass resolved), still holds Build (it references
// Config), and names Build as a reference, on the row and in "references".
func TestBlastRadiusHeuristicKeepsTypeConsumersAsReferences(t *testing.T) {
	srv := mcp.NewServer(newServedFake())
	payload := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"svc/store.go"},
		"include_heuristic": true, "max_depth": 1, "max_symbols": 0,
	})
	syms := anyStrings(payload["symbols"])
	want := []string{
		"svc::app/app.Build", "svc::app/app.Compare",
		"svc::svc/Store.Store.Equal", "svc::svc/Store.Store.Lock", "svc::svc/svc.Config", "svc::svc/svc.NewConfig", "svc::svc/svc.Store",
		"svc::tools/gen/main.main",
	}
	if !reflect.DeepEqual(syms, want) {
		t.Errorf("symbols = %v\nwant %v", syms, want)
	}
	if got := anyStrings(payload["references"]); !reflect.DeepEqual(got, []string{"svc::app/app.Build"}) {
		t.Errorf("references = %v, want [Build]", got)
	}
	var buildRow map[string]any
	for _, b := range payload["by_depth"].([]any) {
		for _, r := range b.(map[string]any)["symbols"].([]any) {
			if row := r.(map[string]any); row["key"] == "svc::app/app.Build" {
				buildRow = row
			}
		}
	}
	if buildRow == nil || buildRow["edge_kind"] != "type-ref" || buildRow["edge_confidence"] != "heuristic" {
		t.Errorf("Build's by_depth row = %v, want edge_kind type-ref, edge_confidence heuristic", buildRow)
	}
}

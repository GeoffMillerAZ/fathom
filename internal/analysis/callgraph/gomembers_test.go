package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// K17 (wave lf2, LB-F342/LB-F343): Go's member symbols — a struct field
// (core.KindGoField) and an interface's own method (an Iface.Method
// core.KindGoMethod whose receiver names an interface of its directory) —
// are never a syntax call target: the syntax pass's name-only binding of
// `r.Run` to the one in-service Run, and of a constructor-scoped
// `<import-path>#.Run`, stays what it was before members existed. The typed
// pass reaches a member by its exact key, and a field reference
// (core.EdgeFieldRead/EdgeFieldWrite) is never a call: the callees views
// leave it out, a field's callers view and the dependents and reach walks
// carry it.
func TestGoMembersAreNeverSyntaxTargets(t *testing.T) {
	n := func(name, pkg string, kind core.SourceKind, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "s", Package: pkg, Name: name, Kind: kind, FilePath: "a/a.go", Line: 1, Calls: calls}
	}
	nodes := []core.SymbolNode{
		n("Runner", "a", core.KindGoInterface),
		n("Runner.Run", "Runner", core.KindGoMethod), // the interface's own method
		n("Impl", "a", core.KindGoType),
		n("Impl.Run", "Impl", core.KindGoMethod),
		n("Opts", "a", core.KindGoType),
		n("Opts.Run", "Opts", core.KindGoField), // a func-typed field named like the method
		n("Config", "a", core.KindGoType),
		n("Config.Timeout", "Config", core.KindGoField),
		n("caller", "a", core.KindGoFunc, "r.Run", "example.com/m/a#.Run", "c.Timeout"),
		n("setter", "a", core.KindGoFunc),
	}
	g := callgraph.Build(nodes)
	callees := func(sym string) []string {
		var out []string
		for _, c := range callgraph.CalleesDetail(g, sym) {
			out = append(out, c.Key)
		}
		return out
	}
	if got, want := callees("s::a/a.caller"), []string{"s::a/Impl.Impl.Run"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("caller's syntax callees = %v, want %v: a member made the method's name ambiguous or took the call", got, want)
	}

	g = callgraph.BuildBoth(nodes, []core.Edge{
		{FromService: "s", FromFile: "a/a.go", FromSymbol: "s::a/a.caller", ToSymbol: "s::a/Config.Config.Timeout", Kind: core.EdgeFieldRead},
		{FromService: "s", FromFile: "a/a.go", FromSymbol: "s::a/a.setter", ToSymbol: "s::a/Config.Config.Timeout", Kind: core.EdgeFieldWrite},
		{FromService: "s", FromFile: "a/a.go", FromSymbol: "s::a/a.caller", ToSymbol: "s::a/Impl.Impl.Run", Kind: core.EdgeDirect},
	})
	if got, want := callees("s::a/a.caller"), []string{"s::a/Impl.Impl.Run"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("caller's callees = %v, want %v: a field reference is not a call", got, want)
	}
	field := "s::a/Config.Config.Timeout"
	got := map[string]core.EdgeKind{}
	for _, c := range callgraph.CallersDetail(g, field) {
		got[c.Key] = c.Kind
	}
	want := map[string]core.EdgeKind{"s::a/a.caller": core.EdgeFieldRead, "s::a/a.setter": core.EdgeFieldWrite}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.Timeout's callers = %v, want its reader and writer %v", got, want)
	}
	if refs := callgraph.ReferrersDetail(g, field); len(refs) != 0 {
		t.Fatalf("Config.Timeout's referrers = %v: a field's readers are its callers view, never a second list", refs)
	}
	if deps := callgraph.DependentsDetail(g, field); len(deps) != 2 {
		t.Fatalf("Config.Timeout's dependents = %v, want reader and writer", deps)
	}
	if rn := callgraph.RenameImpact(g, "Config.Timeout"); !reflect.DeepEqual(rn, []string{"s::a/a.caller", "s::a/a.setter"}) {
		t.Fatalf("RenameImpact(Config.Timeout) = %v", rn)
	}
	if deps := callgraph.DependenciesDetail(g, "s::a/a.caller"); len(deps) != 2 {
		t.Fatalf("caller's dependencies = %v, want the call and the field read", deps)
	}
}

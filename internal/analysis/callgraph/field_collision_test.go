package callgraph_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestGoFieldNeverSyntaxCallTarget(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "Owner", Name: "Owner.Run", Kind: core.KindGoField, FilePath: "pkg/owner.go"},
		{Service: "svc", Package: "sample", Name: "Call", Kind: core.KindGoFunc, FilePath: "pkg/call.go", Calls: []string{"Run", "x.Run", "sample.Owner.Run", "svc::pkg/Owner.Owner.Run", "example.com/pkg#.Run"}},
	}
	g := callgraph.Build(nodes)
	if g.EdgeCount() != 0 {
		t.Fatalf("field fabricated %d call edges", g.EdgeCount())
	}
	patched := g.ReplacePackage("sample", nodes[1:], nil)
	if got := patched.EdgeCount(); got != g.EdgeCount() {
		t.Errorf("refresh fabricated %d call edges; Build has %d", got, g.EdgeCount())
	}
}

func TestGoFieldDoesNotHideMethodCandidate(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "Owner", Name: "Owner.Run", Kind: core.KindGoField, FilePath: "pkg/owner.go"},
		{Service: "svc", Package: "Worker", Name: "Worker.Run", Kind: core.KindGoMethod, FilePath: "pkg/worker.go"},
		{Service: "svc", Package: "sample", Name: "Call", Kind: core.KindGoFunc, FilePath: "pkg/call.go", Calls: []string{"x.Run", "example.com/pkg#.Run"}},
	}
	g := callgraph.Build(nodes)
	if g.EdgeCount() != 1 {
		t.Fatalf("method edge count = %d, want 1", g.EdgeCount())
	}
	if got := callgraph.RenameImpact(g, "svc::pkg/Worker.Worker.Run"); len(got) != 1 {
		t.Fatalf("method callers = %v, want one", got)
	}
	patched := g.ReplacePackage("sample", nodes[2:], nil)
	if got := patched.EdgeCount(); got != g.EdgeCount() {
		t.Errorf("refresh has %d call edges; Build has %d", got, g.EdgeCount())
	}
	if got := callgraph.RenameImpact(patched, "svc::pkg/Worker.Worker.Run"); len(got) != 1 {
		t.Errorf("method callers after refresh = %v, want one", got)
	}
}

// A bare method name also suffix-matches a same-named struct field's key;
// the field never shadows the method (core.WithoutShadowingFields), so the
// method still resolves and its callers are found. The field's own
// Owner.Field spelling still resolves the field.
func TestGoFieldDoesNotShadowBareMethodName(t *testing.T) {
	method := core.SymbolNode{Service: "svc", Package: "Recorder", Name: "Recorder.Record", Kind: core.KindGoMethod, FilePath: "internal/decide/record.go"}
	field := core.SymbolNode{Service: "svc", Package: "Decided", Name: "Decided.Record", Kind: core.KindGoField, FilePath: "internal/decide/decided.go"}
	call := core.SymbolNode{Service: "svc", Package: "decide", Name: "Use", Kind: core.KindGoFunc, FilePath: "internal/decide/use.go", Calls: []string{"x.Record"}}
	g := callgraph.Build([]core.SymbolNode{field, method, call})
	if got := callgraph.Canonical(g, "Record"); got != method.Key() {
		t.Fatalf("Canonical(Record) = %q, want the method %q", got, method.Key())
	}
	if got := callgraph.RenameImpact(g, "Record"); len(got) != 1 || got[0] != call.Key() {
		t.Fatalf("RenameImpact(Record) = %v, want the method's caller %s", got, call.Key())
	}
	if got := callgraph.Canonical(g, "Decided.Record"); got != field.Key() {
		t.Fatalf("Canonical(Decided.Record) = %q, want the field %q", got, field.Key())
	}
}

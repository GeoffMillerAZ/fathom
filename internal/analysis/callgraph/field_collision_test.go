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

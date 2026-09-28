package fusion_test

// LB-Q-csharp (design §13.2 step 13): the csharp dialect's imports.scm
// resolves across two fixture files via its manifest's namespace-dir
// strategy — mirroring TestResolveFileEdges_CIncludes' shape.
//
// An aliased using (`using U = MyApp.Util;` then `U.Helper.Assist()`) binds
// a prefix through namespace-dir's whole-import path. The ordinary
// unaliased `using MyApp.Util;` then `Helper.Assist()` is a wildcard import
// (imports.scm, owner decision D7): TestResolveFileEdges_CSharpUsingWildcard.
import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_CSharpUsing(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "csfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "csharp", []string{filepath.Join("MyApp", "Util.cs"), "Program.cs"})

	edges, report := fusion.ResolveFileEdges(context.Background(), "csharp", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 2 {
		t.Errorf("FilesScanned = %d, want 2", report.FilesScanned)
	}
	if report.EdgesResolved != len(edges) {
		t.Errorf("report.EdgesResolved = %d, want %d (len(edges))", report.EdgesResolved, len(edges))
	}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: Kind = %q, want file-resolved", e, e.Kind)
		}
		if core.EdgeConfidenceOf(e) != core.EdgeConfidenceHeuristic {
			t.Errorf("edge %+v: EdgeConfidenceOf = %q, want heuristic", e, core.EdgeConfidenceOf(e))
		}
	}

	want := map[string]string{
		// "using U = MyApp.Util;" then "U.Helper.Assist()": the alias
		// prefix "U." matches the whole-import binding, and "Helper" is
		// Util.cs's sole top-level symbol. Keys spelled out directly
		// (not fkey, which assumes PackagePath == Package, true only at
		// the fixture root — Util.cs's own directory "MyApp/" is part of
		// its PackagePath too, core.SymbolNode.PackagePath, verified
		// directly against this test's own failure before this fix).
		"fixture::MyApp.Consumer.Program.Run": "fixture::MyApp/MyApp.Util.Helper",
	}
	got := fileResolveEdgeSet(edges)
	if len(got) != len(want) {
		t.Fatalf("got %d edges %v, want %d %v", len(got), got, len(want), want)
	}
	for from, to := range want {
		if got[from] != to {
			t.Errorf("edge from %s: got %s, want %s", from, got[from], to)
		}
	}
}

// TestResolveFileEdges_CSharpUsingWildcard (owner decision D7): C#'s
// ordinary `using X;` brings every type of namespace X into scope
// unqualified, so `Y.Z()` resolves — to the member Y.Z when the target
// defines it. The namespace is found by the files that DECLARE it (C#
// namespaces need not mirror directories: Lib/Hammer.cs declares
// MyApp.Tools), then by namespace-dir's directory rule.
func TestResolveFileEdges_CSharpUsingWildcard(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "csfixture"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "csharp", []string{filepath.Join("MyApp", "Util.cs"), filepath.Join("Lib", "Hammer.cs"), "Program.cs", "Program2.cs"})
	edges, report := fusion.ResolveFileEdges(context.Background(), "csharp", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := fileResolveEdgeSetAll(edges)
	want := []string{
		"fixture::MyApp.Consumer.Program.Run -> fixture::MyApp/MyApp.Util.Helper",
		"fixture::MyApp.Consumer.Worker.Go -> fixture::MyApp/MyApp.Util.Helper.Assist",
		"fixture::MyApp.Consumer.Worker.Go -> fixture::Lib/MyApp.Tools.Hammer.Hit",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d edges %v, want %d", len(got), got, len(want))
	}
}

func fileResolveEdgeSetAll(edges []core.Edge) map[string]bool {
	out := map[string]bool{}
	for _, e := range edges {
		out[e.FromSymbol+" -> "+e.ToSymbol] = true
	}
	return out
}

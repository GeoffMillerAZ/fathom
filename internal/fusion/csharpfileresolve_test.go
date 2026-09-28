package fusion_test

// LB-Q-csharp (design §13.2 step 13): the csharp dialect's imports.scm
// resolves across two fixture files via its manifest's namespace-dir
// strategy — mirroring TestResolveFileEdges_CIncludes' shape.
//
// C#'s own "using" semantics do NOT fit namespace-dir's "whole" binding the
// way Java-shaped code does (verified directly, not asserted): a plain
// `using MyApp.Util;` then an UNQUALIFIED `Helper.Assist()` — the ordinary,
// overwhelmingly common C# idiom, since `using` exists precisely to drop
// the namespace prefix — produces zero edges: matchFileResolvedCall's
// "whole" path only matches a call whose text is ITSELF prefixed by the
// import's own LocalName (internal/fusion/fileresolve.go), and namespace-dir
// is not in wholeIsWildcard's closed set (include-path/file-symbol/
// swift-module only) the way a C #include's target's names are. This
// fixture instead uses the one shape namespace-dir's "whole" path DOES
// reach: an ALIASED using (`using U = MyApp.Util;`) followed by an
// alias-qualified call (`U.Helper.Assist()`) — a real, if less common, C#
// pattern (disambiguating a name collision without a full using). This
// proves the wiring (dialect -> imports.scm -> namespace-dir -> file edge)
// end to end, exactly step 13's ask; it does not claim the common
// unqualified case resolves, and docs/plans/language-breadth.md's
// LB-Q-csharp-rest/this dialect's PROVENANCE.md say so plainly. Widening
// namespace-dir's wholeIsWildcard set to also cover a language whose
// whole-import genuinely behaves like a wildcard (C#, and by the same
// argument Java's `import pkg.*;`) is a foundation-level fusion change
// (design §6: "a new strategy is a foundation-level change... never
// something a worker adds") — flagged for Opus, not patched here.
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

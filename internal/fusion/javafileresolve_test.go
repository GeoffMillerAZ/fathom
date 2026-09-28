package fusion_test

// LB-Q-java (design §13.2 step 13): the java dialect's imports.scm parses
// real import statements and resolves them through its manifest's
// namespace-dir strategy (already generically proven for exactly this
// "com.acme.Foo -> file" shape by resolvestrategy_test.go's "ns/java-class"
// case, LB-F11) — mirroring TestResolveFileEdges_CIncludes/_PythonImports'
// shape as far as Java's own grammar allows.
//
// Unlike those languages, `internal/fusion.resolveImports`'s "top level"
// same-file/cross-file CALL matching only ever attributes a call to a
// symbol whose Name has NO "." (fileresolve.go: "TSExtractor's
// qualifyTSDefs only dots a callable's name when its immediate parent is a
// container, so a module-scope function/class/variable always has a bare
// Name"). Every Java method's parent IS a container (Java has no top-level
// functions at all — a method is always "ClassName.method"), so this
// matcher structurally never has a bare-named Java callable to match
// against, for a same-file call OR a cross-file one — not a gap in THIS
// dialect's queries; a foundation property of the shared same-file
// call-capture + top-level-only matching convention (already the
// documented, measured cause of Python's near-zero file_edge_recall,
// LB-Q-python-rest's own reading note; cmd/indexbench/langoracle.go calls
// fusion.ResolveFileEdges directly for the reading's own file_edge_recall/
// precision metrics, so this is the same root cause, not a second one).
// This test proves what DOES work end to end: imports parse and the
// resolver runs to completion without skipping or erroring; it asserts
// zero edges explicitly, as the correct and expected outcome, rather than
// silently accepting whatever came out.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_JavaImports(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "javafixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "java", []string{
		filepath.Join("com", "example", "Util.java"),
		filepath.Join("com", "example", "Main.java"),
	})

	edges, report := fusion.ResolveFileEdges(context.Background(), "java", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 2 {
		t.Errorf("FilesScanned = %d, want 2", report.FilesScanned)
	}
	if report.ImportsParsed == 0 {
		t.Errorf("ImportsParsed = 0, want at least the one `import com.example.Util;` statement")
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

	// Zero call edges is the CORRECT outcome here (see header): every
	// symbol this fixture's calls could possibly attribute to (Main.local,
	// Util.helper) is a qualified ("ClassName.method") name, and
	// resolveImports only ever matches a bare, unqualified top-level name.
	// A non-zero count would mean either the matcher's contract changed
	// (update this test to match) or this dialect is qualifying names
	// unexpectedly (a real regression).
	if len(edges) != 0 {
		t.Errorf("edges = %d, want 0 (every Java symbol here is container-qualified); got %+v", len(edges), edges)
	}

	// Every resolved edge's From/To must be an actually-extracted symbol —
	// this resolver never invents a key.
	known := map[string]bool{}
	for _, s := range symbols {
		known[s.Key()] = true
	}
	for _, e := range edges {
		if !known[e.FromSymbol] {
			t.Errorf("edge From %s is not an extracted symbol", e.FromSymbol)
		}
		if !known[e.ToSymbol] {
			t.Errorf("edge To %s is not an extracted symbol", e.ToSymbol)
		}
	}
}

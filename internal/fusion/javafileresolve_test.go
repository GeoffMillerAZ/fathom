package fusion_test

// LB-Q-java (design §13.2 step 13): the java dialect's imports.scm parses
// real import statements and resolves them through its manifest's
// namespace-dir strategy (already generically proven for exactly this
// "com.acme.Foo -> file" shape by resolvestrategy_test.go's "ns/java-class"
// case, LB-F11) — mirroring TestResolveFileEdges_CIncludes/_PythonImports'
// shape as far as Java's own grammar allows.
//
// A Java method is always container-qualified ("Util.helper"), and its
// call text is only the method's name, so no call resolves by the
// top-level matching the call path makes. Since LB-F74 the import binds its
// class's simple name (`import com.example.Util;` -> Util, imports.scm) and
// the references query reads `Util.helper(3)`'s receiver as the qualified
// reference "Util.helper": run reaches Util's member helper — the one edge
// this fixture has. The same-file call run -> local draws none (a Java
// method is never top level).

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

	got := fileResolveEdgeSetAll(edges)
	want := "fixture::com/example/com.example.Main.run -> fixture::com/example/com.example.Util.helper"
	if len(got) != 1 || !got[want] {
		t.Errorf("edges = %v, want only %s", got, want)
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

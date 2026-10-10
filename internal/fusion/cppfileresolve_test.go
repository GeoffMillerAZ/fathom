package fusion_test

// LB-Q-cpp (design §13.2 step 13): the cpp dialect's imports.scm resolves
// across two fixture files through its manifest's include-path strategy, the
// C dialect's shape (cfileresolve_test.go) over a namespaced header: an
// #include brings the header's top-level names into scope (design §6), and a
// call spelled through the namespace ("util::helper", normalized "util.helper")
// still lands on the header's definition. The angle-bracket system include
// names no in-repo file and adds no edge.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_CppIncludes(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cppfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "cpp", []string{"util.hpp", "main.cpp"})

	edges, report := fusion.ResolveFileEdges(context.Background(), "cpp", dir, files, symbols)
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
		fkey("main", "use"): fkey("util", "util.helper"), // #include "util.hpp" brings util.helper into scope
		// LB-F434: a C++ file is a module (K18), so the #include itself is an
		// edge from main.cpp's module to util.hpp's.
		fkey("main", "<module>"): fkey("util", "<module>"),
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

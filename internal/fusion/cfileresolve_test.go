package fusion_test

// LB-Q-c (design §13.2 step 13): the c dialect's imports.scm resolves
// across two fixture files via its manifest's include-path strategy —
// mirroring TestResolveFileEdges_TypeScriptImports/_PythonImports'
// shape for the C-specific "whole/wildcard" include semantics
// (a #include brings every top-level name of its target into scope,
// design §6) instead of duplicating LB-F11's own strategy-level
// TestResolveStrategies coverage.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_CIncludes(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "c", []string{"util.h", "main.c"})

	edges, report := fusion.ResolveFileEdges(context.Background(), "c", dir, files, symbols)
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
		fkey("main", "use"): fkey("util", "helper"), // #include "util.h" brings helper into scope (include-path is wholeIsWildcard)
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

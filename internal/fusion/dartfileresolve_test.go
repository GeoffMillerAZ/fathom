package fusion_test

// LB-Q-dart (design §13.2 step 13): the dart dialect's imports.scm resolves
// across fixture files through its manifest's namespace-dir strategy —
// `package:acme/...` through the pubspec.yaml named acme (lib/), a bare path
// relative to the importer — and an unaliased import is a wildcard over the
// target library, so a call to a name it defines resolves to that definition.
// A `package:flutter/...` import names a package outside the repository and
// resolves nowhere.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_DartImports(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "dartfixture"))
	if err != nil {
		t.Fatal(err)
	}
	rels := []string{
		filepath.Join("lib", "main.dart"),
		filepath.Join("lib", "src", "model.dart"),
		filepath.Join("lib", "src", "util.dart"),
		filepath.Join("lib", "src", "net.dart"),
	}
	files, symbols := loadFileResolveFixture(t, dir, "dart", rels)

	edges, report := fusion.ResolveFileEdges(context.Background(), "dart", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 4 {
		t.Errorf("FilesScanned = %d, want 4", report.FilesScanned)
	}
	if report.ImportsParsed != 4 {
		t.Errorf("ImportsParsed = %d, want 4", report.ImportsParsed)
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
	// `package:acme/src/model.dart` (pubspec name, lib/) and the relative
	// `src/util.dart` are wildcards: Model (the constructor call), label and
	// format each resolve to their definition. `m.save()` and `net.fetch()`
	// stay unresolved: the tags engine names a call by its last identifier, so
	// a member call carries no receiver to tie to a type or to the `net`
	// prefix, and the `package:flutter/...` import has no file in the repository.
	want := []string{
		"fixture::lib/main.run -> fixture::lib/src/model.Model",
		"fixture::lib/main.run -> fixture::lib/src/model.label",
		"fixture::lib/main.run -> fixture::lib/src/util.format",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d edges %v, want %d %v", len(got), got, len(want), want)
	}
}

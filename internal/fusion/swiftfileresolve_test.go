package fusion_test

// LB-Q-swift (design §13.2 step 13): the swift dialect's imports.scm resolves across fixture files
// through its manifest's swift-module strategy — `import Core` to every file of the SwiftPM target
// named Core (Package.swift's targets, default Sources/<name>), a whole-module import bringing every
// top-level name of the target into scope, and `import struct Net.Client` binding the one name. An
// import of a module outside the repository (`Foundation`, `Missing`) resolves nowhere.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_SwiftImports(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "swiftfixture"))
	if err != nil {
		t.Fatal(err)
	}
	rels := []string{
		"Sources/App/main.swift",
		"Sources/Core/Model.swift",
		"Sources/Core/Util.swift",
		"Sources/Net/Client.swift",
	}
	files, symbols := loadFileResolveFixture(t, dir, "swift", rels)

	edges, report := fusion.ResolveFileEdges(context.Background(), "swift", dir, files, symbols)
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
	// `import Core` is a whole-module import: Model (the initialiser call), label and format each
	// resolve to their definition in the Core target. `import struct Net.Client` binds Client alone.
	// `m.save()` and `.fetch()` stay unresolved (a member call carries no receiver type), and
	// Foundation and Missing name modules with no target in the repository.
	want := []string{
		"fixture::Sources/App/main.run -> fixture::Sources/Core/Model.Model",
		"fixture::Sources/App/main.run -> fixture::Sources/Core/Model.label",
		"fixture::Sources/App/main.run -> fixture::Sources/Core/Util.format",
		"fixture::Sources/App/main.run -> fixture::Sources/Net/Client.Client",
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

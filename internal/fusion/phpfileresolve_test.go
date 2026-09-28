package fusion_test

// LB-Q-php (design §13.2 step 13): the php dialect's imports.scm resolves
// across fixture files through its manifest's namespace-dir strategy with
// composer.json's PSR-4 prefixes.
//
// What resolves, and what does not, is the tags engine's call text, not the
// import: a PHP call site reaches the resolver as the called name alone
// (`Helper::assist()` is "assist", `Util\format('x')` is "format") because
// tree-sitter-php has no node spanning a receiver and its name, so a
// class-qualified call cannot be tied to `use App\Util\Helper;`. A function
// called through a namespace alias can: `use App\Util;` is a wildcard over
// the files declaring that namespace and brings `format` into scope.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_PhpUse(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "phpfixture"))
	if err != nil {
		t.Fatal(err)
	}
	rels := []string{
		filepath.Join("src", "Util", "Helper.php"),
		filepath.Join("src", "Util", "functions.php"),
		filepath.Join("src", "Consumer.php"),
	}
	files, symbols := loadFileResolveFixture(t, dir, "php", rels)

	edges, report := fusion.ResolveFileEdges(context.Background(), "php", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", report.FilesScanned)
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
	want := []string{
		"fixture::src/App.Consumer.run -> fixture::src/Util/App\\Util.format",
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

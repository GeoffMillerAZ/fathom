package fusion_test

// LB-Q-php (design §13.2 step 13): the php dialect's imports.scm resolves
// across fixture files through its manifest's namespace-dir strategy with
// composer.json's PSR-4 prefixes.
//
// What resolves is the tags engine's call text: under calls.qualify (LB-F65)
// a scoped call keeps its scope (`Helper::assist()` is "Helper::assist",
// `self::local()` "self::local") and a member call its receiver
// (`$this->local()` is "this.local"), so `use App\Util\Helper;` (a wildcard,
// D7) and its alias `Assist` bind Helper's member and self/this the enclosing
// class's. A function called through a namespace alias resolves as before:
// `use App\Util;` is a wildcard over the files declaring that namespace and
// brings `format` into scope.

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
		"fixture::src/App.Consumer.run -> fixture::src/Util/App\\Util.Helper.assist",
		"fixture::src/App.Consumer.run -> fixture::src/App.Consumer.local",
		"fixture::src/App.Consumer.viaAlias -> fixture::src/Util/App\\Util.Helper.assist",
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

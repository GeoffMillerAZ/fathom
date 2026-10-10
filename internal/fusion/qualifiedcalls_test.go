package fusion_test

// LB-F65: the file resolver binds a qualified call text — the receiver or
// scope the tags engine keeps under calls.qualify — to what its qualifier
// names: self/cls/this/static (and PHP's self::) to the enclosing class's
// member, never to a same-named top-level; a name a named import binds to
// the member the imported type defines, else the type; and PHP's `::` reads
// like '.', so `Helper::assist()` through `use App\Util\Helper;` is a file
// edge.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_PythonQualifiedCalls(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "pyqualfixture"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "python", []string{"models.py", "service.py"})
	edges, report := fusion.ResolveFileEdges(context.Background(), "python", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	got := fileResolveEdgeSetAll(edges)
	want := []string{
		fkey("models", "Repo.save") + " -> " + fkey("models", "Repo.validate"),
		fkey("models", "Repo.create") + " -> " + fkey("models", "Repo.build"),
		fkey("service", "run") + " -> " + fkey("models", "Repo.create"),
		fkey("service", "run") + " -> " + fkey("models", "Repo"),
		// K18: service's import of models targets models' module symbol
		fkey("service", "<module>") + " -> " + fkey("models", "<module>"),
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
	if bad := fkey("models", "Repo.save") + " -> " + fkey("models", "validate"); got[bad] {
		t.Errorf("self.validate bound to the top-level validate: %s", bad)
	}
	if len(got) != len(want) {
		t.Errorf("got %d edges %v, want %d", len(got), got, len(want))
	}
}

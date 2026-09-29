package fusion

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// TestResolveDocXrefs_Helm pins LB-Q-helm's L3: a chart's dependencies[].name
// resolves (module-source, the package.json convention) to the one chart of
// that name declared in another Chart.yaml; a dependency the repository does
// not hold (redis, postgresql — registries and remote repositories) resolves
// to nothing, and a values.yaml that happens to carry `name` and
// `dependencies` keys is never read as a chart.
func TestResolveDocXrefs_Helm(t *testing.T) {
	x, err := extractor.NewTagsDocExtractor("helm")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	root := filepath.Join("..", "extractor", "dialects", "helm", "testdata")
	var files []DocFile
	for _, rel := range []string{"common/Chart.yaml", "stack/Chart.yaml", "stack/values.yaml", "stack/templates/service.yaml"} {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		r, err := x.ExtractDoc("svc", rel, src)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{"stack/Chart.yaml:stack -depends_on-> common/Chart.yaml:common"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}

	// Two charts of one name leave the reference unresolved rather than guessed.
	dup := append([]DocFile(nil), files...)
	src, _ := os.ReadFile(filepath.Join(root, "common", "Chart.yaml"))
	r, _ := x.ExtractDoc("svc", "vendor/common/Chart.yaml", src)
	dup = append(dup, DocFile{Path: "vendor/common/Chart.yaml", Result: r})
	if got := xrefBrief(ResolveDocXrefs(dup)); len(got) != 0 {
		t.Fatalf("an ambiguous chart name must not resolve: %q", got)
	}
}

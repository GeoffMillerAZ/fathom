package fusion

import (
	"context"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func docFiles(t *testing.T, id string, files map[string]string) []DocFile {
	t.Helper()
	x, err := extractor.NewDocDialectExtractor(id)
	if err != nil {
		t.Fatal(err)
	}
	var out []DocFile
	for _, p := range sortedMapKeys(files) {
		r, err := x.ExtractDoc("svc", p, []byte(files[p]))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, DocFile{Path: p, Result: r})
	}
	return out
}

func sortedMapKeys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	for i := range ks {
		for j := i + 1; j < len(ks); j++ {
			if ks[j] < ks[i] {
				ks[i], ks[j] = ks[j], ks[i]
			}
		}
	}
	return ks
}

func xrefBrief(xs []DocXref) []string {
	var out []string
	for _, x := range xs {
		out = append(out, x.FromFile+":"+x.From+" -"+x.Edge+"-> "+x.ToFile+":"+x.To)
	}
	return out
}

// TestResolveDocXrefs pins LB-F30's resolution half (D4): repo-path and
// repo-root targets resolve to a file (or the one doc file in a named
// directory, or every directory a glob names) and then to that file's
// root entity; a module-source name resolves to the one root entity
// declaring it; a label-selector joins every labelled entity in its scope
// whose labels include the selector; nothing resolving to its own origin
// is kept.
func TestResolveDocXrefs(t *testing.T) {
	t.Run("github-actions repo-root", func(t *testing.T) {
		fs := docFiles(t, "github-actions", map[string]string{
			".github/workflows/ci.yml": "name: CI\non: push\njobs:\n  build:\n    runs-on: x\n    steps:\n      - uses: ./.github/actions/setup\n      - uses: actions/checkout@v4\n  call:\n    uses: ./.github/workflows/reuse.yml\n",
			".github/workflows/reuse.yml": "name: Reuse\non: workflow_call\njobs:\n  r:\n    runs-on: x\n    steps:\n      - run: echo\n",
			".github/actions/setup/action.yml": "name: Setup\nruns:\n  using: composite\n  steps:\n    - uses: ./.github/actions/inner\n",
			".github/actions/inner/action.yaml": "name: Inner\nruns:\n  using: composite\n  steps:\n    - run: echo\n",
		})
		got := xrefBrief(ResolveDocXrefs(fs))
		want := []string{
			".github/actions/setup/action.yml:Setup -uses_action-> .github/actions/inner/action.yaml:Inner",
			".github/workflows/ci.yml:build -uses_action-> .github/actions/setup/action.yml:Setup",
			".github/workflows/ci.yml:call -calls_workflow-> .github/workflows/reuse.yml:Reuse",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q\nwant %q", got, want)
		}
	})
	t.Run("package-json workspaces and dependencies", func(t *testing.T) {
		fs := docFiles(t, "package-json", map[string]string{
			"package.json":            `{"name":"root","workspaces":["packages/*"],"devDependencies":{"left-pad":"1"}}`,
			"packages/a/package.json": `{"name":"@x/a","dependencies":{"@x/b":"*"}}`,
			"packages/b/package.json": `{"name":"@x/b"}`,
		})
		got := xrefBrief(ResolveDocXrefs(fs))
		want := []string{
			"package.json:root -workspace_member-> packages/a/package.json:@x/a",
			"package.json:root -workspace_member-> packages/b/package.json:@x/b",
			"packages/a/package.json:@x/a -depends_on-> packages/b/package.json:@x/b",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q\nwant %q", got, want)
		}
	})
}

// TestDocXrefEdgesReachTheStore pins LB-F30's persistence half: an ingest
// stores every resolved doc-engine cross-file reference as a file-tier
// typed edge (core.EdgeFileResolved, key "doc-xref"), and the intra-file
// references ride on the symbols' Calls as before.
func TestDocXrefEdgesReachTheStore(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"package.json":            `{"name":"root","workspaces":["packages/a"]}`,
		"packages/a/package.json": `{"name":"a","scripts":{"build":"tsc"}}`,
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "dx", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.DocXrefEdges != 1 {
		t.Fatalf("DocXrefEdges = %d, want 1", rep.DocXrefEdges)
	}
	all, _ := st.TypedEdges(ctx, "dx")
	var got []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	want := []string{"package.json|dx::package.root->dx::packages/a/package.a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored doc-xref edges %q, want %q", got, want)
	}
}

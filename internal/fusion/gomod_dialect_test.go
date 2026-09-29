package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// goModCorpus is a small multi-module repository: a root module and a
// workspace over it, a library, a tool module that requires the library, a
// replace to a directory that holds no go.mod, a use of one that does not
// exist, a module under testdata (never read), and the files that are not
// manifests.
func goModCorpus() map[string]string {
	return map[string]string{
		"go.mod": `module example.com/mono

go 1.22

require (
	example.com/lib v0.0.0
	example.com/tools v0.0.0
	github.com/pkg/errors v0.9.1
)

replace example.com/lib => ./lib

replace (
	example.com/tools => ./tools/kit
	example.com/gone => ./gone
)
`,
		"go.work":           "go 1.22\n\nuse (\n\t.\n\t./lib\n\t./tools/kit\n\t./ghost\n)\n",
		"lib/go.mod":        "module example.com/lib\n\ngo 1.22\n\nrequire github.com/pkg/errors v0.9.1\n",
		"tools/kit/go.mod":  "module example.com/tools\n\ngo 1.22\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ../../lib\n",
		"testdata/x/go.mod": "module example.com/lib\n",
		"go.sum":            "github.com/pkg/errors v0.9.1 h1:abc=\n",
		"go.work.sum":       "github.com/pkg/errors v0.9.1 h1:abc=\n",
		"lib/go.mod.bak":    "module example.com/backup\n",
		"lib/lib.go":        "package lib\n",
	}
}

// TestGoModClassification (LB-Q-go-mod, design §5): the manifest claims a
// go.mod and a go.work by exact basename at any depth; their checksum files
// and a backup are not manifests.
func TestGoModClassification(t *testing.T) {
	rules := dialectRules()
	for path, want := range map[string]string{
		"go.mod":             "go-mod",
		"a/b/go.mod":         "go-mod",
		"go.work":            "go-mod",
		"hack/tools/go.work": "go-mod",
		"go.sum":             "",
		"go.work.sum":        "",
		"go.mod.bak":         "",
		"GO.MOD":             "",
		"main.go":            "",
	} {
		v := connector.ClassifyDialect(rules, path, nil)
		if v.Dialect != want {
			t.Errorf("ClassifyDialect(%q) = %q, want %q", path, v.Dialect, want)
		}
	}
	for path, want := range map[string]string{"go.mod": "go-mod", "sub/go.work": "go-mod", "main.go": "go"} {
		if got := connector.LanguageForPath(path); got != want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestResolveDocXrefs_GoMod pins LB-Q-go-mod's L3: a require names the one
// other go.mod declaring that module path, a replace to a directory and a
// go.work use name the go.mod in that directory (a root directory that holds
// both a go.mod and a go.work included), each entity to entity. A module the
// repository does not hold (github.com/pkg/errors), a replace to a directory
// with no go.mod and a use of a directory that does not exist resolve to
// nothing.
func TestResolveDocXrefs_GoMod(t *testing.T) {
	x := extractor.NewGoModulesExtractor()
	sources := goModCorpus()
	var paths []string
	for p := range sources {
		if b := filepath.Base(p); b == "go.mod" || b == "go.work" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var files []DocFile
	for _, rel := range paths {
		if filepath.Base(filepath.Dir(rel)) == "x" { // testdata is never read
			continue
		}
		r, err := x.ExtractDoc("svc", rel, []byte(sources[rel]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"go.mod:example.com/mono -depends_on-> lib/go.mod:example.com/lib",
		"go.mod:example.com/mono -depends_on-> tools/kit/go.mod:example.com/tools",
		"go.mod:replace/example.com/lib -replaced_by-> lib/go.mod:example.com/lib",
		"go.mod:replace/example.com/tools -replaced_by-> tools/kit/go.mod:example.com/tools",
		"go.work:use/. -workspace_member-> go.mod:example.com/mono",
		"go.work:use/./lib -workspace_member-> lib/go.mod:example.com/lib",
		"go.work:use/./tools/kit -workspace_member-> tools/kit/go.mod:example.com/tools",
		"tools/kit/go.mod:example.com/tools -depends_on-> lib/go.mod:example.com/lib",
		"tools/kit/go.mod:replace/example.com/lib -replaced_by-> lib/go.mod:example.com/lib",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestIngest_GoModWorkspace: an ingest extracts every go.mod and go.work
// through the go-mod dialect (a module, its requirements, its replacements, a
// workspace and its uses), tags them "go-mod", never reads a module under
// testdata, a checksum file or a backup, and stores the cross-file
// references that resolve as file-tier typed edges.
func TestIngest_GoModWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, goModCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "gm", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "gm", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		switch o.SourceRef {
		case "testdata/x/go.mod", "go.sum", "go.work.sum", "lib/go.mod.bak":
			t.Errorf("%s|%s: a file that is not read was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	for key, kind := range map[string]string{
		"go.mod|example.com/mono":                  "GoModule",
		"go.mod|example.com/lib":                   "ModuleDep",
		"go.mod|github.com/pkg/errors":             "ModuleDep",
		"go.mod|replace/example.com/lib":           "GoModReplace",
		"go.work|workspace":                        "GoWorkspace",
		"go.work|use/./tools/kit":                  "GoWorkUse",
		"lib/go.mod|example.com/lib":               "GoModule",
		"tools/kit/go.mod|replace/example.com/lib": "GoModReplace",
	} {
		if got[key] != kind {
			t.Errorf("%s = %q, want %q", key, got[key], kind)
		}
	}
	if rep.DocXrefEdges != 9 {
		t.Errorf("DocXrefEdges = %d, want 9 (three uses, two requirements, four replacements)", rep.DocXrefEdges)
	}
	all, _ := st.TypedEdges(ctx, "gm")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"go.mod|gm::go.example.com/mono->gm::lib/go.example.com/lib",
		"go.mod|gm::go.example.com/mono->gm::tools/kit/go.example.com/tools",
		"go.mod|gm::go.replace/example.com/lib->gm::lib/go.example.com/lib",
		"go.mod|gm::go.replace/example.com/tools->gm::tools/kit/go.example.com/tools",
		"go.work|gm::go.use/.->gm::go.example.com/mono",
		"go.work|gm::go.use/./lib->gm::lib/go.example.com/lib",
		"go.work|gm::go.use/./tools/kit->gm::tools/kit/go.example.com/tools",
		"tools/kit/go.mod|gm::tools/kit/go.example.com/tools->gm::lib/go.example.com/lib",
		"tools/kit/go.mod|gm::tools/kit/go.replace/example.com/lib->gm::lib/go.example.com/lib",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
}

// TestReingestFile_GoMod: the one-file refresh reads a go.mod through the
// same extractor an ingest does, so an edit that adds a requirement shows in
// the module's Calls.
func TestReingestFile_GoMod(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, goModCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gm", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "lib/go.mod"
	edited := "module example.com/lib\n\ngo 1.22\n\nrequire (\n\tgithub.com/pkg/errors v0.9.1\n\tgolang.org/x/sync v0.5.0\n)\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "gm", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "gm", "")
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, o := range obs {
		if o.SourceRef == rel && o.Symbol == "example.com/lib" {
			calls = append(calls, o.Calls...)
		}
	}
	sort.Strings(calls)
	if want := []string{"github.com/pkg/errors", "golang.org/x/sync"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("lib calls after ReingestFile = %v, want %v", calls, want)
	}
}

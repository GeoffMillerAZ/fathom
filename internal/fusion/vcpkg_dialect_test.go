package fusion

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestVcpkgClassification (LB-Q-vcpkg, design §5): the dialect claims the two
// vcpkg documents by exact basename at any depth; any other JSON, and a
// differently cased name, is not vcpkg.
func TestVcpkgClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"vcpkg.json", "vcpkg", connector.ClassifiedByBasename},
		{"ports/zlib/vcpkg.json", "vcpkg", connector.ClassifiedByBasename},
		{"vcpkg-configuration.json", "vcpkg", connector.ClassifiedByBasename},
		{"sdk/core/vcpkg/vcpkg.json", "vcpkg", connector.ClassifiedByBasename},
		{"Vcpkg.json", "", connector.ClassifiedNone},
		{"vcpkg.json.bak", "", connector.ClassifiedNone},
		{"versions/baseline.json", "", connector.ClassifiedNone},
		{"package.json", "package-json", connector.ClassifiedByBasename},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}

func vcpkgPort(name string, deps ...string) string {
	s := `{"name": "` + name + `", "version": "1.0", "dependencies": [`
	for i, d := range deps {
		if i > 0 {
			s += ", "
		}
		s += `"` + d + `"`
	}
	return s + "]}\n"
}

func vcpkgCorpus() map[string]string {
	return map[string]string{
		"ports/zlib/vcpkg.json":        vcpkgPort("zlib"),
		"ports/png/vcpkg.json":         vcpkgPort("libpng", "zlib", "vcpkg-cmake"),
		"ports/curl/vcpkg.json":        `{"name":"curl","version":"1","dependencies":[{"name":"zlib","platform":"!windows"},{"name":"libpng","host":true},"absent"],"features":{"ssl":{"description":"tls","dependencies":["openssl-overlay"]}}}`,
		"overlay/openssl/vcpkg.json":   vcpkgPort("openssl-overlay", "zlib"),
		"dup-a/vcpkg.json":             vcpkgPort("dup"),
		"dup-b/vcpkg.json":             vcpkgPort("dup"),
		"app/vcpkg.json":               vcpkgPort("app", "curl", "dup", "fmt"),
		"app/vcpkg-configuration.json": `{"registries":[{"kind":"git","repository":"r","baseline":"b"}],"overlay-ports":["../overlay"]}`,
		"versions/baseline.json":       `{"default": {"zlib": {"baseline": "1", "port-version": 0}}}`,
	}
}

// TestResolveDocXrefs_Vcpkg pins LB-Q-vcpkg's L3: a dependency (plain, host,
// platform-filtered, a feature's) names the one other manifest declaring a
// package of that name, entity to entity, overlay ports included. A name two
// manifests declare (dup: the resolver refuses to guess), one the repository
// does not hold (absent, fmt) resolve to nothing.
func TestResolveDocXrefs_Vcpkg(t *testing.T) {
	x, err := extractor.NewDocDialectExtractor("vcpkg")
	if err != nil {
		t.Fatal(err)
	}
	sources := vcpkgCorpus()
	var paths []string
	for p := range sources {
		if b := filepath.Base(p); b == "vcpkg.json" || b == "vcpkg-configuration.json" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var files []DocFile
	for _, rel := range paths {
		r, err := x.ExtractDoc("svc", rel, []byte(sources[rel]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"app/vcpkg.json:app -depends_on-> ports/curl/vcpkg.json:curl",
		"overlay/openssl/vcpkg.json:openssl-overlay -depends_on-> ports/zlib/vcpkg.json:zlib",
		"ports/curl/vcpkg.json:curl -depends_on-> overlay/openssl/vcpkg.json:openssl-overlay",
		"ports/curl/vcpkg.json:curl -depends_on-> ports/png/vcpkg.json:libpng",
		"ports/curl/vcpkg.json:curl -depends_on-> ports/zlib/vcpkg.json:zlib",
		"ports/png/vcpkg.json:libpng -depends_on-> ports/zlib/vcpkg.json:zlib",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestIngest_Vcpkg: an ingest extracts every vcpkg.json and
// vcpkg-configuration.json through the vcpkg dialect (never the generic JSON
// extractor), leaves other JSON with the generic one, and stores the
// resolved dependency edges as file-tier typed edges.
func TestIngest_Vcpkg(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, vcpkgCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "vc", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "vc", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	var generic []string
	for _, o := range obs {
		switch o.SourceKind {
		case core.KindVcpkgPackage, "VcpkgFeature", "VcpkgRegistry":
			got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		case core.KindJSONKey:
			generic = append(generic, o.SourceRef)
		}
	}
	for _, k := range []string{
		"ports/zlib/vcpkg.json|zlib", "ports/png/vcpkg.json|libpng", "ports/curl/vcpkg.json|curl",
		"ports/curl/vcpkg.json|features.ssl", "app/vcpkg.json|app", "dup-a/vcpkg.json|dup",
		"app/vcpkg-configuration.json|https://example.com/r",
	} {
		if k == "app/vcpkg-configuration.json|https://example.com/r" {
			k = "app/vcpkg-configuration.json|r"
		}
		if got[k] == "" {
			t.Errorf("no vcpkg observation %q (got %v)", k, got)
		}
	}
	for _, g := range generic {
		if b := filepath.Base(g); b == "vcpkg.json" || b == "vcpkg-configuration.json" {
			t.Errorf("%s was read by the generic JSON extractor", g)
		}
	}
	if rep.DocXrefEdges != 6 {
		t.Errorf("DocXrefEdges = %d, want 6", rep.DocXrefEdges)
	}
	all, _ := st.TypedEdges(ctx, "vc")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	want := []string{
		"app/vcpkg.json|vc::app/vcpkg.app->vc::ports/curl/vcpkg.curl",
		"overlay/openssl/vcpkg.json|vc::overlay/openssl/vcpkg.openssl-overlay->vc::ports/zlib/vcpkg.zlib",
		"ports/curl/vcpkg.json|vc::ports/curl/vcpkg.curl->vc::overlay/openssl/vcpkg.openssl-overlay",
		"ports/curl/vcpkg.json|vc::ports/curl/vcpkg.curl->vc::ports/png/vcpkg.libpng",
		"ports/curl/vcpkg.json|vc::ports/curl/vcpkg.curl->vc::ports/zlib/vcpkg.zlib",
		"ports/png/vcpkg.json|vc::ports/png/vcpkg.libpng->vc::ports/zlib/vcpkg.zlib",
	}
	if !reflect.DeepEqual(edges, want) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, want)
	}
}

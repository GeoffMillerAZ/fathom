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

// TestMavenClassification (LB-Q-maven, design §5): the manifest claims a
// POM by its basename alone, at any depth; other XML (a shaded or flattened
// POM a build writes, settings.xml, Ant and Android files) is not a POM.
func TestMavenClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"pom.xml", "maven", connector.ClassifiedByBasename},
		{"core/pom.xml", "maven", connector.ClassifiedByBasename},
		{"a/b/c/pom.xml", "maven", connector.ClassifiedByBasename},
		{"pom-shaded.xml", "", connector.ClassifiedNone},
		{"dependency-reduced-pom.xml", "", connector.ClassifiedNone},
		{".flattened-pom.xml", "", connector.ClassifiedNone},
		{"settings.xml", "", connector.ClassifiedNone},
		{"pom.xml.bak", "", connector.ClassifiedNone},
		{"pom.yaml", "kubernetes", connector.ClassifiedByExtension},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}

func mavenPOM(parent, artifact string, modules []string, deps ...string) string {
	s := "<project>\n  <modelVersion>4.0.0</modelVersion>\n"
	if parent != "" {
		s += "  <parent><groupId>g</groupId><artifactId>" + parent + "</artifactId><version>1</version></parent>\n"
	}
	s += "  <artifactId>" + artifact + "</artifactId>\n"
	if len(modules) > 0 {
		s += "  <modules>\n"
		for _, m := range modules {
			s += "    <module>" + m + "</module>\n"
		}
		s += "  </modules>\n"
	}
	if len(deps) > 0 {
		s += "  <dependencies>\n"
		for _, d := range deps {
			s += "    <dependency><groupId>g</groupId><artifactId>" + d + "</artifactId></dependency>\n"
		}
		s += "  </dependencies>\n"
	}
	return s + "</project>\n"
}

// TestResolveDocXrefs_Maven pins LB-Q-maven's L3: a module names the POM in
// its directory, a parent and a dependency name the one other POM declaring
// that artifactId, entity to entity. An artifact declared in two POMs (a
// name, not a coordinate: the resolver refuses to guess), one the repository
// does not hold (junit) and a module directory with no POM resolve to nothing.
func TestResolveDocXrefs_Maven(t *testing.T) {
	x, err := extractor.NewDocDialectExtractor("maven")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{
		"pom.xml":       mavenPOM("", "shop-parent", []string{"core", "web", "api/", "ghost"}),
		"core/pom.xml":  mavenPOM("shop-parent", "shop-core", nil),
		"web/pom.xml":   mavenPOM("shop-parent", "shop-web", nil, "shop-core", "junit", "dup", "web-self-less"),
		"api/pom.xml":   mavenPOM("shop-parent", "shop-api", nil),
		"dup-a/pom.xml": mavenPOM("shop-parent", "dup", nil),
		"dup-b/pom.xml": mavenPOM("", "dup", nil),
	}
	var files []DocFile
	var paths []string
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		r, err := x.ExtractDoc("svc", rel, []byte(sources[rel]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"api/pom.xml:shop-api -parent-> pom.xml:shop-parent",
		"core/pom.xml:shop-core -parent-> pom.xml:shop-parent",
		"dup-a/pom.xml:dup -parent-> pom.xml:shop-parent",
		"pom.xml:shop-parent -module-> api/pom.xml:shop-api",
		"pom.xml:shop-parent -module-> core/pom.xml:shop-core",
		"pom.xml:shop-parent -module-> web/pom.xml:shop-web",
		"web/pom.xml:shop-web -depends_on-> core/pom.xml:shop-core",
		"web/pom.xml:shop-web -parent-> pom.xml:shop-parent",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func mavenReactorCorpus() map[string]string {
	return map[string]string{
		"pom.xml":          mavenPOM("", "shop-parent", []string{"core", "web"}),
		"core/pom.xml":     mavenPOM("shop-parent", "shop-core", nil),
		"web/pom.xml":      mavenPOM("shop-parent", "shop-web", nil, "shop-core", "junit"),
		"web/pom-war.xml":  mavenPOM("shop-parent", "shop-war", nil),
		"settings.xml":     "<settings><artifactId>x</artifactId></settings>\n",
		"web/notes/pom.md": "# not a pom\n",
	}
}

// TestIngest_MavenReactor: an ingest extracts every pom.xml through the maven
// dialect (the root aggregator and each module), tags it "maven", leaves the
// other XML alone, and stores the cross-file references that resolve — a
// module's directory, a parent and an in-repo dependency — as file-tier typed
// edges. The intra-file references ride on Calls.
func TestIngest_MavenReactor(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, mavenReactorCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "mv", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "mv", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		if o.SourceKind != core.KindMavenArtifact {
			continue
		}
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
	}
	want := map[string]string{
		"pom.xml|shop-parent":    "MavenArtifact",
		"core/pom.xml|shop-core": "MavenArtifact",
		"web/pom.xml|shop-web":   "MavenArtifact",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("maven observations\n got: %v\nwant: %v", got, want)
	}
	for _, o := range obs {
		if o.SourceRef == "settings.xml" || o.SourceRef == "web/pom-war.xml" {
			t.Errorf("%s|%s: an XML file that is not a pom.xml was extracted", o.SourceRef, o.Symbol)
		}
	}
	if rep.DocXrefEdges != 5 {
		t.Errorf("DocXrefEdges = %d, want 5 (two modules, two parents, one dependency)", rep.DocXrefEdges)
	}
	all, _ := st.TypedEdges(ctx, "mv")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"core/pom.xml|mv::core/pom.shop-core->mv::pom.shop-parent",
		"pom.xml|mv::pom.shop-parent->mv::core/pom.shop-core",
		"pom.xml|mv::pom.shop-parent->mv::web/pom.shop-web",
		"web/pom.xml|mv::web/pom.shop-web->mv::core/pom.shop-core",
		"web/pom.xml|mv::web/pom.shop-web->mv::pom.shop-parent",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
}

// TestReingestFile_MavenPOM: the one-file refresh reads a pom.xml through the
// same dialect an ingest does (the basename, not an .xml extension no
// extractor owns), so an edit that adds a dependency shows in Calls.
func TestReingestFile_MavenPOM(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, mavenReactorCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "mv", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "core/pom.xml"
	edited := mavenPOM("shop-parent", "shop-core", nil, "guava")
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "mv", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "mv", "")
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, o := range obs {
		if o.SourceRef == rel && o.Symbol == "shop-core" {
			calls = append(calls, o.Calls...)
		}
	}
	sort.Strings(calls)
	if want := []string{"guava", "shop-parent"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("shop-core calls after ReingestFile = %v, want %v", calls, want)
	}
}

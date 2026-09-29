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

// TestMSBuildClassification (LB-Q-msbuild, design §5): the manifest claims an
// MSBuild file by its extension — a project of any of the three languages and
// the shared props and targets — at any depth; a project's own sidecars
// (App.csproj.user), the solution files and other XML are not MSBuild.
func TestMSBuildClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"App.csproj", "msbuild", connector.ClassifiedByExtension},
		{"src/App/App.csproj", "msbuild", connector.ClassifiedByExtension},
		{"src/Lib/Lib.fsproj", "msbuild", connector.ClassifiedByExtension},
		{"src/Lib/Lib.vbproj", "msbuild", connector.ClassifiedByExtension},
		{"Directory.Build.props", "msbuild", connector.ClassifiedByExtension},
		{"eng/Common.targets", "msbuild", connector.ClassifiedByExtension},
		{"a/b/c/Directory.Packages.props", "msbuild", connector.ClassifiedByExtension},
		{"App.csproj.user", "", connector.ClassifiedNone},
		{"App.sln", "", connector.ClassifiedNone},
		{"App.slnx", "", connector.ClassifiedNone},
		{"packages.config", "", connector.ClassifiedNone},
		{"app.config", "", connector.ClassifiedNone},
		{"App.csproj.bak", "", connector.ClassifiedNone},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}

func msbuildProject(sdk string, packages []string, projectRefs ...string) string {
	s := "<Project"
	if sdk != "" {
		s += ` Sdk="` + sdk + `"`
	}
	s += ">\n  <PropertyGroup>\n    <TargetFramework>net8.0</TargetFramework>\n  </PropertyGroup>\n"
	if len(packages) > 0 {
		s += "  <ItemGroup>\n"
		for _, p := range packages {
			s += `    <PackageReference Include="` + p + `" Version="1.0.0" />` + "\n"
		}
		s += "  </ItemGroup>\n"
	}
	if len(projectRefs) > 0 {
		s += "  <ItemGroup>\n"
		for _, r := range projectRefs {
			s += `    <ProjectReference Include="` + r + `" />` + "\n"
		}
		s += "  </ItemGroup>\n"
	}
	return s + "</Project>\n"
}

func msbuildReactorCorpus() map[string]string {
	return map[string]string{
		"src/App/App.csproj":      msbuildProject("Microsoft.NET.Sdk.Web", []string{"Serilog"}, `..\Core\Core.csproj`, "../Data/Data.fsproj", `..\Ghost\Ghost.csproj`, `$(RepoRoot)tools\Gen\Gen.csproj`, `..\App\App.csproj`),
		"src/Core/Core.csproj":    msbuildProject("Microsoft.NET.Sdk", nil),
		"src/Data/Data.fsproj":    msbuildProject("Microsoft.NET.Sdk", []string{"Dapper"}, `..\Core\Core.csproj`),
		"Directory.Build.props":   "<Project>\n  <ItemGroup>\n    <PackageReference Include=\"StyleCop.Analyzers\" Version=\"1\" />\n  </ItemGroup>\n</Project>\n",
		"eng/Common.targets":      "<Project>\n  <Target Name=\"Stamp\" />\n</Project>\n",
		"src/App/App.csproj.user": "<Project><PropertyGroup><X>1</X></PropertyGroup></Project>\n",
		"src/App/packages.config": "<packages><package id=\"Serilog\" version=\"3\" /></packages>\n",
		"src/App/notes.md":        "# not msbuild\n",
	}
}

// TestResolveDocXrefs_MSBuild pins LB-Q-msbuild's L3: a ProjectReference names
// the project file, written with either separator, and resolves entity to
// entity; one that names a file the repository does not hold (Ghost), one
// written with a property the file cannot expand ($(RepoRoot)) and one that
// names its own file resolve to nothing.
func TestResolveDocXrefs_MSBuild(t *testing.T) {
	x, err := extractor.NewDocDialectExtractor("msbuild")
	if err != nil {
		t.Fatal(err)
	}
	sources := msbuildReactorCorpus()
	var files []DocFile
	var paths []string
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		if rel == "src/App/notes.md" || rel == "src/App/packages.config" || rel == "src/App/App.csproj.user" {
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
		"src/App/App.csproj:App.csproj -project_reference-> src/Core/Core.csproj:Core.csproj",
		"src/App/App.csproj:App.csproj -project_reference-> src/Data/Data.fsproj:Data.fsproj",
		"src/Data/Data.fsproj:Data.fsproj -project_reference-> src/Core/Core.csproj:Core.csproj",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestIngest_MSBuildRepo: an ingest extracts every project, props and targets
// file through the msbuild dialect, tags it "msbuild", leaves a project's
// sidecar, packages.config and everything else alone, and stores the cross-file
// references that resolve — a ProjectReference to a project of the repository —
// as file-tier typed edges. The package references ride on Calls.
func TestIngest_MSBuildRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, msbuildReactorCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "mb", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "mb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	calls := map[string][]string{}
	for _, o := range obs {
		if o.SourceKind != core.KindMSBuildProject {
			continue
		}
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		calls[o.SourceRef] = append(calls[o.SourceRef], o.Calls...)
	}
	want := map[string]string{
		"src/App/App.csproj|App.csproj":               "MSBuildProject",
		"src/Core/Core.csproj|Core.csproj":            "MSBuildProject",
		"src/Data/Data.fsproj|Data.fsproj":            "MSBuildProject",
		"Directory.Build.props|Directory.Build.props": "MSBuildProject",
		"eng/Common.targets|Common.targets":           "MSBuildProject",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("msbuild observations\n got: %v\nwant: %v", got, want)
	}
	for _, o := range obs {
		if o.SourceRef == "src/App/App.csproj.user" || o.SourceRef == "src/App/packages.config" {
			t.Errorf("%s|%s: a file that is not MSBuild was extracted", o.SourceRef, o.Symbol)
		}
	}
	if !reflect.DeepEqual(calls["src/App/App.csproj"], []string{"Serilog"}) || !reflect.DeepEqual(calls["src/Data/Data.fsproj"], []string{"Dapper"}) ||
		!reflect.DeepEqual(calls["Directory.Build.props"], []string{"StyleCop.Analyzers"}) {
		t.Errorf("package references on Calls: %v", calls)
	}
	if rep.DocXrefEdges != 3 {
		t.Errorf("DocXrefEdges = %d, want 3 (App to Core and Data, Data to Core)", rep.DocXrefEdges)
	}
	all, _ := st.TypedEdges(ctx, "mb")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"src/App/App.csproj|mb::src/App/App.App.csproj->mb::src/Core/Core.Core.csproj",
		"src/App/App.csproj|mb::src/App/App.App.csproj->mb::src/Data/Data.Data.fsproj",
		"src/Data/Data.fsproj|mb::src/Data/Data.Data.fsproj->mb::src/Core/Core.Core.csproj",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
}

// TestReingestFile_MSBuildProject: the one-file refresh reads a project through
// the same dialect an ingest does (the extension, not an .xml no extractor
// owns), so an edit that adds a package shows in Calls.
func TestReingestFile_MSBuildProject(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, msbuildReactorCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "mb", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "src/Core/Core.csproj"
	edited := msbuildProject("Microsoft.NET.Sdk", []string{"Polly"})
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "mb", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "mb", "")
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, o := range obs {
		if o.SourceRef == rel && o.Symbol == "Core.csproj" {
			calls = append(calls, o.Calls...)
		}
	}
	sort.Strings(calls)
	if want := []string{"Polly"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("Core.csproj calls after ReingestFile = %v, want %v", calls, want)
	}
}

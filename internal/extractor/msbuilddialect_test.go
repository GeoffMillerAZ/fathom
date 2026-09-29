package extractor

// msbuilddialect_test.go — goldens for the msbuild doc-engine dialect
// (LB-Q-msbuild, design §13.2 steps 10/12/14). The same golden shape as the
// maven, gitlab-ci and cargo dialects: symbols (name/kind/package/line/
// end_line/signature/calls/tier) and the cross-file references the engine
// records, whether or not fusion.ResolveDocXrefs can resolve them (a
// ProjectReference written with a property, one that names a file outside the
// repository). A fixture is read as if it sat in src/App/, so a relative
// ProjectReference resolves against a directory that has siblings.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type msbuildGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type msbuildGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type msbuildGolden struct {
	Symbols   []msbuildGoldenSymbol `json:"symbols"`
	CrossFile []msbuildGoldenCross  `json:"cross_file"`
}

func loadMSBuildExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "msbuild" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(msbuild): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"msbuild\" not found in LoadDialects()")
	return nil
}

// msbuildFixtures lists the testdata files the dialect owns: a golden sits
// beside each as <file>.golden.json.
func msbuildFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "msbuild", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

// TestMSBuildDialectGoldens: every testdata fixture's project, targets and
// cross-file references against a hand-written golden.
func TestMSBuildDialectGoldens(t *testing.T) {
	x := loadMSBuildExtractor(t)
	for _, fx := range msbuildFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			relPath := "src/App/" + filepath.Base(fx)
			res, err := x.ExtractDoc("svc", relPath, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got msbuildGolden
			got.Symbols = []msbuildGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, msbuildGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []msbuildGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, msbuildGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want msbuildGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []msbuildGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []msbuildGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("%s: cross_file\n got: %+v\nwant: %+v", fx, got.CrossFile, want.CrossFile)
			}
		})
	}
}

// TestMSBuildDialectScope: what a project's own top-level items declare is the
// entity's business, nothing a target, a conditional Choose or an Update adds.
// Only a PackageReference that Includes a name is a reference (an Update
// modifies one declared elsewhere), only a Project-level Target is an entity
// (an ItemGroup inside a target is built at run time), and a ProjectReference
// inside a target or a Choose is not the project's own edge.
func TestMSBuildDialectScope(t *testing.T) {
	src := `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <PackageReference Include="direct" Version="1" />
    <PackageReference Update="updated" Version="2" />
    <PackageReference Remove="removed" />
    <ProjectReference Include="..\up\up.csproj" />
  </ItemGroup>
  <Choose>
    <When Condition="'$(A)' == 'b'">
      <ItemGroup>
        <PackageReference Include="chosen" Version="3" />
        <ProjectReference Include="..\chosen\chosen.csproj" />
      </ItemGroup>
    </When>
  </Choose>
  <Target Name="Dynamic">
    <ItemGroup>
      <PackageReference Include="dynamic" Version="4" />
      <ProjectReference Include="..\dynamic\dynamic.csproj" />
    </ItemGroup>
    <Target Name="NotATarget" />
  </Target>
</Project>
`
	x := loadMSBuildExtractor(t)
	res, err := x.ExtractDoc("svc", "a/scope.csproj", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	kinds := map[string]string{}
	for _, n := range res.Nodes {
		got[n.Name] = n.Calls
		kinds[n.Name] = string(n.Kind)
	}
	wantCalls := map[string][]string{"scope.csproj": {"direct"}, "Dynamic": nil}
	if !reflect.DeepEqual(got, wantCalls) {
		t.Errorf("entities and calls\n got: %v\nwant: %v", got, wantCalls)
	}
	if kinds["scope.csproj"] != "MSBuildProject" || kinds["Dynamic"] != "MSBuildTarget" {
		t.Errorf("kinds: %v", kinds)
	}
	var refs []string
	for _, c := range res.CrossFile {
		refs = append(refs, c.Edge+":"+c.Raw)
	}
	if want := []string{`project_reference:..\up\up.csproj`}; !reflect.DeepEqual(refs, want) {
		t.Errorf("cross-file references = %v, want %v", refs, want)
	}
}

// TestMSBuildDialectOneChildForms: an ItemGroup holding one element decodes as
// a map, not a list, and reads exactly as a list does; a project whose items
// are spread over several groups, or written as one group with several
// elements, lands the same references. Two ProjectReferences to one project
// are two records of one edge, a repeated PackageReference one call.
func TestMSBuildDialectOneChildForms(t *testing.T) {
	one := `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup><PackageReference Include="solo" /></ItemGroup>
  <ItemGroup><ProjectReference Include="a\a.csproj" /></ItemGroup>
  <Target Name="only" />
</Project>`
	many := `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <PackageReference Include="x" />
    <PackageReference Include="y" />
    <PackageReference Include="x" Version="2" />
    <ProjectReference Include="a\a.csproj" />
    <ProjectReference Include="a\a.csproj" />
  </ItemGroup>
  <ItemGroup>
    <PackageReference Include="z" />
  </ItemGroup>
</Project>`
	x := loadMSBuildExtractor(t)
	brief := func(src string) (calls, names, cross []string) {
		res, err := x.ExtractDoc("svc", "p/one.csproj", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range res.Nodes {
			calls = append(calls, n.Calls...)
			names = append(names, n.Name)
		}
		for _, c := range res.CrossFile {
			cross = append(cross, c.Edge+":"+c.Raw+"->"+c.Target)
		}
		return
	}
	calls, names, cross := brief(one)
	if want := []string{"solo"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("one: calls %v, want %v", calls, want)
	}
	if want := []string{"one.csproj", "only"}; !reflect.DeepEqual(names, want) {
		t.Errorf("one: names %v, want %v", names, want)
	}
	if want := []string{`project_reference:a\a.csproj->p/a/a.csproj`}; !reflect.DeepEqual(cross, want) {
		t.Errorf("one: cross %v, want %v", cross, want)
	}
	calls, _, cross = brief(many)
	if want := []string{"x", "y", "z"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("many: calls %v, want %v", calls, want)
	}
	if want := []string{`project_reference:a\a.csproj->p/a/a.csproj`, `project_reference:a\a.csproj->p/a/a.csproj`}; !reflect.DeepEqual(cross, want) {
		t.Errorf("many: cross %v, want %v", cross, want)
	}
}

// TestMSBuildDialectEmptyFiles: an MSBuild file that holds nothing is still a
// file of the build (a Directory.Build.targets written empty to stop the parent's
// from being imported is the usual one), so its root is an entity whether the
// decode reads it as a map or as an empty scalar; a file that is not XML, or not
// even a document, yields nothing.
func TestMSBuildDialectEmptyFiles(t *testing.T) {
	x := loadMSBuildExtractor(t)
	for name, c := range map[string]struct{ file, src, want string }{
		"empty root":     {"x.props", `<Project />`, "MSBuildProject x.props@1-1"},
		"comment only":   {"y.targets", "<Project>\n  <!-- none -->\n</Project>\n", "MSBuildProject y.targets@1-3"},
		"scalar root":    {"z.props", `<Project>text</Project>`, "MSBuildProject z.props@1-1"},
		"sdk only":       {"Only.vbproj", `<Project Sdk="Microsoft.NET.Sdk" />`, "MSBuildProject Only.vbproj@1-1 sig=Microsoft.NET.Sdk"},
		"fsharp project": {"Tool.fsproj", "<Project Sdk=\"Microsoft.NET.Sdk\">\n  <PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>\n</Project>\n", "MSBuildProject Tool.fsproj@1-3 sig=net8.0"},
	} {
		res, err := x.ExtractDoc("svc", "a/"+c.file, []byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := brief(res.Nodes); !reflect.DeepEqual(got, []string{c.want}) {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
		if len(res.CrossFile) != 0 {
			t.Errorf("%s: cross-file references %v, want none", name, res.CrossFile)
		}
	}
	for name, src := range map[string]string{
		"empty":   ``,
		"not xml": `Include: x`,
		"text":    `just text`,
	} {
		res, err := x.ExtractDoc("svc", "a/x.props", []byte(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(res.Nodes) != 0 || len(res.CrossFile) != 0 {
			t.Errorf("%s: got %d nodes, %d cross-file refs, want none", name, len(res.Nodes), len(res.CrossFile))
		}
	}
}

package extractor

// docengine_file_test.go — two additions the msbuild dialect needs from the
// doc engine (LB-Q-msbuild, design §4): `$file`, the base name of the file
// being read, and a `\` that a repo-path reference reads as the separator it
// is in a Windows-authored file.

import (
	"reflect"
	"sort"
	"testing"
)

// TestDocEngineFileValue: `$file` is the file's base name, extension included,
// wherever a value expression is read (a name, a signature, a qualifier, a
// reference, a cross-file reference), and an alternation falls back to it when
// the document names nothing. A file whose name is all a document holds still
// has an entity, and two files of one directory (Directory.Build.props and
// Directory.Build.targets) get two names.
func TestDocEngineFileValue(t *testing.T) {
	x := mustDoc(t, `{"decode":"xml","detect":{},"entities":[
  {"select":"$","where":{"is_map":true},"kind":"package","source_kind":"P","name":"$file","signature":".@Sdk | $file"},
  {"select":"$.Target[*]","where":{"is_map":true},"kind":"target","source_kind":"T","name":".@Name","qualify":["$file"]}],
 "references":[{"from":"package","select":"$","value":".@Sdk | $file","to_kind":"package","edge":"self"}],
 "cross_file":[{"select":"$","value":"$file","resolve":"module-source","edge":"self"}]}`)
	src := `<Project Sdk="Microsoft.NET.Sdk">
  <Target Name="Pack" />
</Project>
`
	r, err := x.ExtractDoc("s", "src/App/App.csproj", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"P App.csproj@1-3 sig=Microsoft.NET.Sdk calls=Microsoft.NET.Sdk", "T App.csproj/Pack@2-2"}
	if got := brief(r.Nodes); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if len(r.CrossFile) != 1 || r.CrossFile[0].Raw != "App.csproj" {
		t.Fatalf("cross = %+v", r.CrossFile)
	}
	if !reflect.DeepEqual(r.Roots, []string{"App.csproj"}) {
		t.Fatalf("roots = %v", r.Roots)
	}

	// A document that says nothing of itself still has its file's name; a file
	// with no directory reads the same.
	bare := `<Project><PropertyGroup><A>1</A></PropertyGroup></Project>`
	var names []string
	for _, rel := range []string{"Directory.Build.props", "Directory.Build.targets", "eng/Common.props"} {
		r, err := x.ExtractDoc("s", rel, []byte(bare))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range r.Nodes {
			names = append(names, n.Name+"|"+n.Signature)
		}
	}
	sort.Strings(names)
	wantNames := []string{"Common.props|Common.props", "Directory.Build.props|Directory.Build.props", "Directory.Build.targets|Directory.Build.targets"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("names = %q, want %q", names, wantNames)
	}
}

// TestDocEngineFileValueFirstAlternativeWins: `$file` is an alternative like
// any other, so a document that names itself keeps its own name and only one
// that does not takes its file's.
func TestDocEngineFileValueFirstAlternativeWins(t *testing.T) {
	x := mustDoc(t, `{"decode":"yaml","detect":{},"entities":[
  {"select":"$","where":{"is_map":true},"kind":"package","source_kind":"P","name":".name | $file"}]}`)
	r, _ := x.ExtractDoc("s", "a/Chart.yaml", []byte("name: demo\n"))
	if got, want := brief(r.Nodes), []string{"P demo@1-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("named: got %q want %q", got, want)
	}
	r, _ = x.ExtractDoc("s", "a/Chart.yaml", []byte("other: 1\n"))
	if got, want := brief(r.Nodes), []string{"P Chart.yaml@1-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unnamed: got %q want %q", got, want)
	}
}

// TestDocEngineRepoPathBackslash: a repo-path reference written with `\`
// (MSBuild's ProjectReference, Windows-authored YAML) resolves like its `/`
// spelling, from the referring file's directory or, when it leads with a
// separator, from the repository root; a path that climbs out of the
// repository still does not resolve, and Raw stays as written.
func TestDocEngineRepoPathBackslash(t *testing.T) {
	for _, c := range []struct{ from, raw, want string }{
		{"test/T/T.csproj", `..\..\src\A\A.csproj`, "src/A/A.csproj"},
		{"test/T/T.csproj", `../..\src/A\A.csproj`, "src/A/A.csproj"},
		{"T.csproj", `sub\B.csproj`, "sub/B.csproj"},
		{"a/T.csproj", `\eng\C.props`, "eng/C.props"},
		{"a/T.csproj", `..\..\out.csproj`, ""},
		{"a/T.csproj", `..\a\.\B.csproj`, "a/B.csproj"},
		{"a/T.csproj", `sub/B.csproj`, "a/sub/B.csproj"},
	} {
		if got := resolveDocRef("repo-path", c.from, c.raw); got != c.want {
			t.Errorf("resolveDocRef(repo-path, %q, %q) = %q, want %q", c.from, c.raw, got, c.want)
		}
	}

	x := mustDoc(t, `{"decode":"xml","detect":{},"entities":[
  {"select":"$","where":{"is_map":true},"kind":"package","source_kind":"P","name":"$file"}],
 "cross_file":[{"select":"$.ItemGroup[*].ProjectReference[*]","value":".@Include","resolve":"repo-path","edge":"project_reference"}]}`)
	src := `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <ProjectReference Include="..\..\src\A\A.csproj" />
    <ProjectReference Include="../../src/B/B.csproj" />
  </ItemGroup>
</Project>
`
	r, err := x.ExtractDoc("s", "test/T/T.csproj", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range r.CrossFile {
		got = append(got, c.Raw+" -> "+c.Target)
	}
	want := []string{`..\..\src\A\A.csproj -> src/A/A.csproj`, `../../src/B/B.csproj -> src/B/B.csproj`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

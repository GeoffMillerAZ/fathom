package extractor

// mavendialect_test.go — goldens for the maven doc-engine dialect
// (LB-Q-maven, design §13.2 steps 10/12/14). Same golden shape as the
// gitlab-ci, docker-compose, kubernetes and github-actions dialects: symbols
// (name/kind/package/line/end_line/signature/calls/tier) and the cross-file
// references the engine records, whether or not fusion.ResolveDocXrefs can
// resolve them (a module that names a directory outside the repository, a
// module written with an unresolved property).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type mavenGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type mavenGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type mavenGolden struct {
	Symbols   []mavenGoldenSymbol `json:"symbols"`
	CrossFile []mavenGoldenCross  `json:"cross_file"`
}

func loadMavenExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "maven" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(maven): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"maven\" not found in LoadDialects()")
	return nil
}

func mavenFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "maven", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".xml" {
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

// TestMavenDialectGoldens: every testdata/*.xml fixture's artifact, plugins
// and cross-file references against a hand-written golden. relPath is the
// fixture's own basename, so a repo-path target resolves against the
// repository root.
func TestMavenDialectGoldens(t *testing.T) {
	x := loadMavenExtractor(t)
	for _, fx := range mavenFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			relPath := filepath.Base(fx)
			res, err := x.ExtractDoc("svc", relPath, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got mavenGolden
			got.Symbols = []mavenGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, mavenGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []mavenGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, mavenGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want mavenGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []mavenGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []mavenGoldenCross{}
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

// TestMavenDialectScope: what a pom's own file declares is the entity's
// business, nothing a profile, a plugin or a management section adds. Only
// build/plugins becomes a plugin entity (a pluginManagement entry and the
// same plugin activated below it would share one symbol key), and only the
// project's own <dependencies> are references (a plugin's <dependencies>, a
// profile's and <dependencyManagement>'s are not the project's).
func TestMavenDialectScope(t *testing.T) {
	src := `<project>
  <artifactId>scope</artifactId>
  <dependencies>
    <dependency><artifactId>direct</artifactId></dependency>
  </dependencies>
  <dependencyManagement>
    <dependencies>
      <dependency><artifactId>managed</artifactId></dependency>
    </dependencies>
  </dependencyManagement>
  <build>
    <pluginManagement>
      <plugins><plugin><artifactId>managed-plugin</artifactId></plugin></plugins>
    </pluginManagement>
    <plugins>
      <plugin>
        <artifactId>active-plugin</artifactId>
        <dependencies><dependency><artifactId>plugin-dep</artifactId></dependency></dependencies>
      </plugin>
    </plugins>
  </build>
  <profiles>
    <profile>
      <id>p</id>
      <dependencies><dependency><artifactId>profile-dep</artifactId></dependency></dependencies>
      <build><plugins><plugin><artifactId>profile-plugin</artifactId></plugin></plugins></build>
    </profile>
  </profiles>
</project>
`
	x := loadMavenExtractor(t)
	res, err := x.ExtractDoc("svc", "pom.xml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	kinds := map[string]string{}
	for _, n := range res.Nodes {
		got[n.Name] = n.Calls
		kinds[n.Name] = string(n.Kind)
	}
	wantCalls := map[string][]string{"scope": {"direct"}, "active-plugin": nil}
	if !reflect.DeepEqual(got, wantCalls) {
		t.Errorf("entities and calls\n got: %v\nwant: %v", got, wantCalls)
	}
	if kinds["scope"] != "MavenArtifact" || kinds["active-plugin"] != "MavenPlugin" {
		t.Errorf("kinds: %v", kinds)
	}
	var deps []string
	for _, c := range res.CrossFile {
		if c.Edge == "depends_on" {
			deps = append(deps, c.Raw)
		}
	}
	if !reflect.DeepEqual(deps, []string{"direct"}) {
		t.Errorf("depends_on references = %v, want [direct]", deps)
	}
}

// TestMavenDialectOneChildForms: a <dependencies> or <modules> holding one
// element decodes as a map, not a list, and reads exactly as a list does; a
// parent, a dependency and a module written across several elements all land.
func TestMavenDialectOneChildForms(t *testing.T) {
	one := `<project>
  <artifactId>one</artifactId>
  <parent><artifactId>up</artifactId></parent>
  <modules><module>only</module></modules>
  <dependencies><dependency><artifactId>solo</artifactId></dependency></dependencies>
</project>`
	many := `<project>
  <artifactId>many</artifactId>
  <modules><module>a</module><module>b</module></modules>
  <dependencies>
    <dependency><artifactId>x</artifactId></dependency>
    <dependency><artifactId>y</artifactId></dependency>
    <dependency><artifactId>x</artifactId><classifier>tests</classifier></dependency>
  </dependencies>
</project>`
	x := loadMavenExtractor(t)
	brief := func(src string) (calls []string, cross []string) {
		res, err := x.ExtractDoc("svc", "pom.xml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range res.Nodes {
			calls = append(calls, n.Calls...)
		}
		for _, c := range res.CrossFile {
			cross = append(cross, c.Edge+":"+c.Raw)
		}
		return
	}
	calls, cross := brief(one)
	if want := []string{"solo", "up"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("one: calls %v, want %v", calls, want)
	}
	if want := []string{"parent:up", "module:only", "depends_on:solo"}; !reflect.DeepEqual(cross, want) {
		t.Errorf("one: cross %v, want %v", cross, want)
	}
	calls, cross = brief(many)
	if want := []string{"x", "y"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("many: calls %v, want %v", calls, want)
	}
	if want := []string{"module:a", "module:b", "depends_on:x", "depends_on:y", "depends_on:x"}; !reflect.DeepEqual(sortedCopy(cross), sortedCopy(want)) {
		t.Errorf("many: cross %v, want %v", cross, want)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// TestMavenDialectNeedsAnArtifactId: a file whose root has no artifactId is
// no project, whatever else it holds (its dependencies are not references),
// and a root that is not a map yields nothing.
func TestMavenDialectNeedsAnArtifactId(t *testing.T) {
	x := loadMavenExtractor(t)
	for name, src := range map[string]string{
		"no artifactId": `<project><groupId>g</groupId><dependencies><dependency><artifactId>d</artifactId></dependency></dependencies></project>`,
		"scalar root":   `<project>text</project>`,
		"empty":         ``,
		"not xml":       `artifactId: x`,
	} {
		res, err := x.ExtractDoc("svc", "pom.xml", []byte(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(res.Nodes) != 0 || len(res.CrossFile) != 0 {
			t.Errorf("%s: got %d nodes, %d cross-file refs, want none", name, len(res.Nodes), len(res.CrossFile))
		}
	}
}

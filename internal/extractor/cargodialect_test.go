package extractor

// cargodialect_test.go — goldens for the cargo doc-engine dialect
// (LB-Q-cargo, design §13.2 steps 10/12/14). Same golden shape as the maven,
// python-packaging and package-json dialects: symbols (name/kind/package/
// line/end_line/signature/calls/tier) and the cross-file references the
// engine records, whether or not fusion.ResolveDocXrefs can resolve them (a
// workspace member that climbs out of the repository, a member glob).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type cargoGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type cargoGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type cargoGolden struct {
	Symbols   []cargoGoldenSymbol `json:"symbols"`
	CrossFile []cargoGoldenCross  `json:"cross_file"`
}

func loadCargoExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "cargo" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(cargo): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"cargo\" not found in LoadDialects()")
	return nil
}

func cargoFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "cargo", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
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

// TestCargoDialectGoldens: every testdata/*.toml fixture's entities and
// cross-file references against a hand-written golden. relPath is the
// fixture's own basename, so a repo-path target resolves against the
// repository root.
func TestCargoDialectGoldens(t *testing.T) {
	x := loadCargoExtractor(t)
	for _, fx := range cargoFixtures(t) {
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
			var got cargoGolden
			got.Symbols = []cargoGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, cargoGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []cargoGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, cargoGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want cargoGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []cargoGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []cargoGoldenCross{}
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

// TestCargoDialectScope: what a manifest's own tables declare is the
// entity's business, nothing a workspace table, a patch, a profile or
// metadata adds. Only the package's own [dependencies], [dev-dependencies],
// [build-dependencies] and their [target.*] forms are references; the
// workspace's shared [workspace.dependencies], [patch] and [replace] are
// policy, and [package.metadata] and [profile] are neither.
func TestCargoDialectScope(t *testing.T) {
	src := `[package]
name = "scope"

[dependencies]
direct = "1"

[dev-dependencies]
dev = "1"

[build-dependencies]
build = "1"

[target.'cfg(unix)'.dependencies]
unix = "1"

[target.'cfg(unix)'.build-dependencies]
unix-build = "1"

[workspace.dependencies]
shared = "1"

[patch.crates-io]
patched = { path = "vendor/patched" }

[replace]
"replaced:1.0.0" = { path = "vendor/replaced" }

[package.metadata.deb]
depends = "not-a-dependency"

[profile.release]
lto = true

[badges]
maintenance = { status = "actively-developed" }
`
	x := loadCargoExtractor(t)
	res, err := x.ExtractDoc("svc", "Cargo.toml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	kinds := map[string]string{}
	for _, n := range res.Nodes {
		got[n.Name] = n.Calls
		kinds[n.Name] = string(n.Kind)
	}
	wantCalls := map[string][]string{
		"scope":     {"build", "dev", "direct", "unix", "unix-build"},
		"workspace": nil,
	}
	if !reflect.DeepEqual(got, wantCalls) {
		t.Errorf("entities and calls\n got: %v\nwant: %v", got, wantCalls)
	}
	if kinds["scope"] != "CargoPackage" || kinds["workspace"] != "CargoWorkspace" {
		t.Errorf("kinds: %v", kinds)
	}
	var deps []string
	for _, c := range res.CrossFile {
		deps = append(deps, c.Edge+":"+c.Raw)
	}
	sort.Strings(deps)
	want := []string{"build_depends_on:build", "build_depends_on:unix-build", "depends_on:direct", "depends_on:unix", "dev_depends_on:dev"}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("cross-file references = %v, want %v", deps, want)
	}
}

// TestCargoDialectTargetNamesAreQualified: a target's name is kind-qualified
// (bin/acme), so a binary named like its package (the usual case) and a
// feature named like a dependency keep distinct symbol keys — core.SymbolKey
// carries no kind, and one file's entities are keyed by name alone.
func TestCargoDialectTargetNamesAreQualified(t *testing.T) {
	src := `[package]
name = "acme"

[lib]
name = "acme"

[[bin]]
name = "acme"

[[example]]
name = "acme"

[[test]]
name = "acme"

[[bench]]
name = "acme"

[features]
acme = []
`
	x := loadCargoExtractor(t)
	res, err := x.ExtractDoc("svc", "Cargo.toml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range res.Nodes {
		names = append(names, string(n.Kind)+":"+n.Name)
	}
	want := []string{"CargoPackage:acme", "CargoLib:lib/acme", "CargoBin:bin/acme", "CargoExample:example/acme", "CargoTest:test/acme", "CargoBench:bench/acme", "CargoFeature:acme"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("entities\n got: %v\nwant: %v", names, want)
	}
	seen := map[string]bool{}
	for _, n := range res.Nodes {
		if seen[n.Name] && n.Kind != "CargoFeature" {
			t.Errorf("name %q repeats among the targets", n.Name)
		}
		seen[n.Name] = true
	}
}

// TestCargoDialectNeedsANamedPackage: a [package] table without a name is no
// package and a document that is not TOML yields nothing; either way a
// dependency table alone declares no entity to hang a reference on (its
// cross-file entries are recorded and dropped by the resolver, which finds
// no root entity to start from).
func TestCargoDialectNeedsANamedPackage(t *testing.T) {
	x := loadCargoExtractor(t)
	for name, src := range map[string]string{
		"no name":      "[package]\nversion = \"1\"\n[dependencies]\nserde = \"1\"\n",
		"scalar table": "package = \"acme\"\n[dependencies]\nserde = \"1\"\n",
		"no package":   "[dependencies]\nserde = \"1\"\n",
		"empty":        "",
		"not toml":     "<project><artifactId>x</artifactId></project>",
	} {
		res, err := x.ExtractDoc("svc", "Cargo.toml", []byte(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(res.Nodes) != 0 {
			t.Errorf("%s: got %d nodes, want none", name, len(res.Nodes))
		}
		if len(res.Roots) != 0 {
			t.Errorf("%s: roots %v, want none", name, res.Roots)
		}
		if (name == "empty" || name == "not toml") && len(res.CrossFile) != 0 {
			t.Errorf("%s: got %d cross-file refs, want none", name, len(res.CrossFile))
		}
	}
}

package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestCargoClassification (LB-Q-cargo, design §5): the manifest claims a
// Cargo manifest by its exact basename, at any depth; the rest of the Rust
// toolchain's TOML (cargo's own config, the toolchain pin, rustfmt, clippy
// and cargo-deny configuration), a lockfile and a differently cased name are
// not manifests.
func TestCargoClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"Cargo.toml", "cargo", connector.ClassifiedByBasename},
		{"crates/core/Cargo.toml", "cargo", connector.ClassifiedByBasename},
		{"a/b/c/Cargo.toml", "cargo", connector.ClassifiedByBasename},
		{"cargo.toml", "", connector.ClassifiedNone},
		{"Cargo.lock", "", connector.ClassifiedNone},
		{".cargo/config.toml", "", connector.ClassifiedNone},
		{"rust-toolchain.toml", "", connector.ClassifiedNone},
		{"rustfmt.toml", "", connector.ClassifiedNone},
		{"clippy.toml", "", connector.ClassifiedNone},
		{"deny.toml", "", connector.ClassifiedNone},
		{"pyproject.toml", "python-packaging", connector.ClassifiedByBasename},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}

func cargoWorkspaceRoot(members ...string) string {
	s := "[workspace]\nmembers = ["
	for i, m := range members {
		if i > 0 {
			s += ", "
		}
		s += `"` + m + `"`
	}
	return s + "]\nresolver = \"2\"\n"
}

// cargoManifest is a package manifest; each dependency is "name" for a plain
// [dependencies] entry, "dev:name" for [dev-dependencies] and "build:name" for
// [build-dependencies].
func cargoManifest(name string, deps ...string) string {
	s := "[package]\nname = \"" + name + "\"\nversion = \"0.1.0\"\n"
	sections := map[string]string{}
	for _, d := range deps {
		sec, dep := "dependencies", d
		if strings.HasPrefix(d, "dev:") {
			sec, dep = "dev-dependencies", strings.TrimPrefix(d, "dev:")
		} else if strings.HasPrefix(d, "build:") {
			sec, dep = "build-dependencies", strings.TrimPrefix(d, "build:")
		}
		sections[sec] += dep + " = \"1\"\n"
	}
	for _, sec := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
		if sections[sec] != "" {
			s += "\n[" + sec + "]\n" + sections[sec]
		}
	}
	return s
}

func cargoWorkspaceCorpus() map[string]string {
	return map[string]string{
		"Cargo.toml":              cargoWorkspaceRoot("crates/*", "tools/cli", "ghost"),
		"crates/core/Cargo.toml":  cargoManifest("acme-core"),
		"crates/util/Cargo.toml":  cargoManifest("acme-util", "acme-core", "serde", "dup", "build:acme-gen"),
		"crates/gen/Cargo.toml":   cargoManifest("acme-gen"),
		"crates/empty/README.md":  "# a directory without a manifest\n",
		"tools/cli/Cargo.toml":    cargoManifest("acme-cli", "acme-core", "dev:acme-util"),
		"dup-a/Cargo.toml":        cargoManifest("dup"),
		"dup-b/Cargo.toml":        cargoManifest("dup"),
		"rustfmt.toml":            "max_width = 100\n",
		"rust-toolchain.toml":     "[toolchain]\nchannel = \"stable\"\n",
		"Cargo.lock":              "# generated\nversion = 3\n",
		"crates/core/notes/x.md":  "# not a manifest\n",
		"crates/core/Cargo.toml~": cargoManifest("backup"),
	}
}

// TestResolveDocXrefs_Cargo pins LB-Q-cargo's L3: a workspace's member globs
// and paths name the manifest in each directory, entity to entity (the
// workspace to each member's package), and a dependency names the one other
// manifest declaring that package name. A package declared in two manifests
// (a name, not a source: the resolver refuses to guess), one the repository
// does not hold (serde), a member with no manifest (ghost, crates/empty) and
// a workspace member that is not a package of a manifest resolve to nothing.
func TestResolveDocXrefs_Cargo(t *testing.T) {
	x, err := extractor.NewDocDialectExtractor("cargo")
	if err != nil {
		t.Fatal(err)
	}
	sources := cargoWorkspaceCorpus()
	var files []DocFile
	var paths []string
	for p := range sources {
		if filepath.Base(p) == "Cargo.toml" {
			paths = append(paths, p)
		}
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
		"Cargo.toml:workspace -workspace_member-> crates/core/Cargo.toml:acme-core",
		"Cargo.toml:workspace -workspace_member-> crates/gen/Cargo.toml:acme-gen",
		"Cargo.toml:workspace -workspace_member-> crates/util/Cargo.toml:acme-util",
		"Cargo.toml:workspace -workspace_member-> tools/cli/Cargo.toml:acme-cli",
		"crates/util/Cargo.toml:acme-util -depends_on-> crates/core/Cargo.toml:acme-core",
		"crates/util/Cargo.toml:acme-util -build_depends_on-> crates/gen/Cargo.toml:acme-gen",
		"tools/cli/Cargo.toml:acme-cli -depends_on-> crates/core/Cargo.toml:acme-core",
		"tools/cli/Cargo.toml:acme-cli -dev_depends_on-> crates/util/Cargo.toml:acme-util",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestIngest_CargoWorkspace: an ingest extracts every Cargo.toml through the
// cargo dialect (the virtual root and each member), tags it "cargo", leaves
// the other TOML (rustfmt.toml, rust-toolchain.toml) with the generic
// extractor and a lockfile or a backup file alone, and stores the
// cross-file references that resolve — a member's manifest and an in-repo
// dependency — as file-tier typed edges. The intra-file references ride on
// Calls.
func TestIngest_CargoWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cargoWorkspaceCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "cg", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cg", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	var generic []string
	for _, o := range obs {
		switch o.SourceKind {
		case core.KindCargoPackage, "CargoWorkspace":
			got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		case core.KindTOMLTable, core.KindTOMLKey:
			generic = append(generic, o.SourceRef)
		}
		if o.SourceRef == "Cargo.lock" || o.SourceRef == "crates/core/Cargo.toml~" {
			t.Errorf("%s|%s: a file that is not a Cargo.toml was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	want := map[string]string{
		"Cargo.toml|workspace":             "CargoWorkspace",
		"crates/core/Cargo.toml|acme-core": "CargoPackage",
		"crates/util/Cargo.toml|acme-util": "CargoPackage",
		"crates/gen/Cargo.toml|acme-gen":   "CargoPackage",
		"tools/cli/Cargo.toml|acme-cli":    "CargoPackage",
		"dup-a/Cargo.toml|dup":             "CargoPackage",
		"dup-b/Cargo.toml|dup":             "CargoPackage",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cargo observations\n got: %v\nwant: %v", got, want)
	}
	for _, g := range generic {
		if filepath.Base(g) == "Cargo.toml" {
			t.Errorf("%s was read by the generic TOML extractor", g)
		}
	}
	seenRustfmt := false
	for _, g := range generic {
		if g == "rustfmt.toml" {
			seenRustfmt = true
		}
	}
	if !seenRustfmt {
		t.Errorf("rustfmt.toml lost its generic TOML extraction (generic refs: %v)", generic)
	}
	if rep.DocXrefEdges != 8 {
		t.Errorf("DocXrefEdges = %d, want 8 (four members and four dependencies)", rep.DocXrefEdges)
	}
	all, _ := st.TypedEdges(ctx, "cg")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"Cargo.toml|cg::Cargo.workspace->cg::crates/core/Cargo.acme-core",
		"Cargo.toml|cg::Cargo.workspace->cg::crates/gen/Cargo.acme-gen",
		"Cargo.toml|cg::Cargo.workspace->cg::crates/util/Cargo.acme-util",
		"Cargo.toml|cg::Cargo.workspace->cg::tools/cli/Cargo.acme-cli",
		"crates/util/Cargo.toml|cg::crates/util/Cargo.acme-util->cg::crates/core/Cargo.acme-core",
		"crates/util/Cargo.toml|cg::crates/util/Cargo.acme-util->cg::crates/gen/Cargo.acme-gen",
		"tools/cli/Cargo.toml|cg::tools/cli/Cargo.acme-cli->cg::crates/core/Cargo.acme-core",
		"tools/cli/Cargo.toml|cg::tools/cli/Cargo.acme-cli->cg::crates/util/Cargo.acme-util",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
}

// TestReingestFile_CargoManifest: the one-file refresh reads a Cargo.toml
// through the same dialect an ingest does (the basename, not the ".toml"
// extension the generic extractor owns), so an edit that adds a dependency
// shows in Calls.
func TestReingestFile_CargoManifest(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cargoWorkspaceCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cg", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "crates/core/Cargo.toml"
	edited := cargoManifest("acme-core", "bytes")
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "cg", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cg", "")
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, o := range obs {
		if o.SourceRef == rel && o.Symbol == "acme-core" {
			calls = append(calls, o.Calls...)
		}
	}
	sort.Strings(calls)
	if want := []string{"bytes"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("acme-core calls after ReingestFile = %v, want %v", calls, want)
	}
}

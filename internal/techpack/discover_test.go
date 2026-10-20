package techpack

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// writeFile writes data at dir/rel, creating parents.
func writeFile(t *testing.T, dir, rel, data string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// packCUE is a valid #Pack header file for name at version.
func packCUE(name string, version int) string {
	return "pack: {name: \"" + name + "\", version: " + strconv.Itoa(version) + ", about: \"test pack\"}\n" +
		"generated: [{output: \"{dir}/{name}.gen.go\", source: [\"{dir}/{name}.src\"], regenerate: \"kitgen\"}]\n"
}

// offlineEnv points discovery at a fake Go module cache and a fake CUE
// cache under base, and leaves no network and no go binary reachable: an
// unroutable proxy setting and a PATH holding nothing. Discover must answer
// from disk alone (I56).
func offlineEnv(t *testing.T, base string) (gomodcache, cuecache string) {
	t.Helper()
	gomodcache = filepath.Join(base, "gomodcache")
	cuecache = filepath.Join(base, "cuecache")
	t.Setenv("GOMODCACHE", gomodcache)
	t.Setenv("GOPATH", filepath.Join(base, "gopath-unused"))
	t.Setenv("CUE_CACHE_DIR", cuecache)
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOWORK", filepath.Join(base, "repo", "go.work"))
	t.Setenv("PATH", filepath.Join(base, "empty-bin"))
	return gomodcache, cuecache
}

// TestDiscoverReadsPinsOfflineNeverGoWork is FD-11's anchor: the repository's
// own pins decide which packs exist. A local replace wins over a vendored and
// a cached copy, vendor/ wins over the cache, the cache is read at exactly the
// pinned version (a newer one beside it is never read), a go.work naming
// another pack is ignored, a pinned module absent from disk is disclosed and
// never fetched, a fathom-pack/ without a header is not a pack, and the
// output is sorted and deterministic.
func TestDiscoverReadsPinsOfflineNeverGoWork(t *testing.T) {
	base := t.TempDir()
	gomodcache, cuecache := offlineEnv(t, base)
	repo := filepath.Join(base, "repo")

	writeFile(t, repo, "go.mod", `module example.test/app

go 1.22

require (
	example.test/kitreplace v1.0.0
	example.test/kitvendor v1.2.0
	example.test/KitCache v1.3.0
	example.test/kitabsent v0.9.0
	example.test/kitwork v1.0.0
	example.test/noheader v1.0.0
	example.test/plain v1.0.0 // indirect
)

replace example.test/kitreplace => ../kitreplace
`)
	// go.work is the author's machine, never the pin: it would put another
	// pack on the path for kitwork.
	writeFile(t, repo, "go.work", "go 1.22\n\nuse (\n\t.\n\t../workpack\n)\n\nreplace example.test/kitwork => ../workpack\n")
	writeFile(t, base, "workpack/go.mod", "module example.test/kitwork\n\ngo 1.22\n")
	writeFile(t, base, "workpack/fathom-pack/pack.cue", packCUE("work-pack", 1))

	// The local replace wins over the vendored and cached copies.
	writeFile(t, base, "kitreplace/go.mod", "module example.test/kitreplace\n\ngo 1.22\n")
	writeFile(t, base, "kitreplace/fathom-pack/pack.cue", packCUE("replace-kit", 1))
	writeFile(t, repo, "vendor/example.test/kitreplace/fathom-pack/pack.cue", packCUE("replace-vendored-copy", 1))
	writeFile(t, gomodcache, "example.test/kitreplace@v1.0.0/fathom-pack/pack.cue", packCUE("replace-cached-copy", 1))

	// vendor/ wins over the cache when modules.txt lists the module.
	writeFile(t, repo, "vendor/modules.txt", "# example.test/kitreplace v1.0.0 => ../kitreplace\n## explicit\n# example.test/kitvendor v1.2.0\n## explicit\n")
	writeFile(t, repo, "vendor/example.test/kitvendor/fathom-pack/pack.cue", packCUE("vendor-kit", 2))
	writeFile(t, gomodcache, "example.test/kitvendor@v1.2.0/fathom-pack/pack.cue", packCUE("vendor-cached-copy", 2))

	// The cache at the pinned version only, with module case escaping; the
	// newer version beside it is never read.
	writeFile(t, gomodcache, "example.test/!kit!cache@v1.3.0/fathom-pack/pack.cue", packCUE("cache-kit", 3))
	writeFile(t, gomodcache, "example.test/!kit!cache@v1.3.0/fathom-pack/fixtures/generated.cue", "pairs: []\n")
	writeFile(t, gomodcache, "example.test/!kit!cache@v1.4.0/fathom-pack/pack.cue", packCUE("cache-kit", 4))

	// A fathom-pack/ with no header is not a tech pack; a module with no
	// fathom-pack/ carries none.
	writeFile(t, gomodcache, "example.test/noheader@v1.0.0/fathom-pack/guard-pack.cue", "guards: []\n")
	writeFile(t, gomodcache, "example.test/plain@v1.0.0/go.mod", "module example.test/plain\n")

	// CUE pins: cue.mod/module.cue deps resolve through the CUE cache at
	// exactly the pinned version.
	writeFile(t, repo, "cue.mod/module.cue", "module: \"example.test/app@v0\"\nlanguage: version: \"v0.9.0\"\ndeps: {\n\t\"example.test/cuekit@v0\": {v: \"v0.2.0\"}\n\t\"example.test/cueabsent@v0\": {v: \"v0.1.0\"}\n}\n")
	writeFile(t, cuecache, "mod/extract/example.test/cuekit@v0.2.0/fathom-pack/pack.cue", packCUE("cue-kit", 5))
	writeFile(t, cuecache, "mod/extract/example.test/cuekit@v0.3.0/fathom-pack/pack.cue", packCUE("cue-kit-newer", 6))

	got, err := Discover(repo)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	type row struct {
		Name    string
		Version int
		Module  string
		Pin     string
		Source  Source
		Dir     string
	}
	var rows []row
	for _, p := range got.Packs {
		rows = append(rows, row{p.Name, p.Version, p.Module, p.Pin, p.Source, p.Dir})
		if len(p.Hash) != 64 || strings.Trim(p.Hash, "0123456789abcdef") != "" {
			t.Errorf("pack %s: hash %q is not a sha256 hex digest", p.Name, p.Hash)
		}
	}
	want := []row{
		{"cache-kit", 3, "example.test/KitCache", "v1.3.0", SourceModCache, filepath.Join(gomodcache, "example.test", "!kit!cache@v1.3.0", "fathom-pack")},
		{"cue-kit", 5, "example.test/cuekit", "v0.2.0", SourceCUECache, filepath.Join(cuecache, "mod", "extract", "example.test", "cuekit@v0.2.0", "fathom-pack")},
		{"replace-kit", 1, "example.test/kitreplace", "v1.0.0", SourceReplace, filepath.Join(base, "kitreplace", "fathom-pack")},
		{"vendor-kit", 2, "example.test/kitvendor", "v1.2.0", SourceVendor, filepath.Join(repo, "vendor", "example.test", "kitvendor", "fathom-pack")},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("packs:\n got %+v\nwant %+v", rows, want)
	}

	var missing []string
	for _, m := range got.Missing {
		missing = append(missing, m.String())
	}
	wantMissing := []string{"example.test/cueabsent@v0.1.0", "example.test/kitabsent@v0.9.0", "example.test/kitwork@v1.0.0"}
	if !reflect.DeepEqual(missing, wantMissing) {
		t.Errorf("missing:\n got %v\nwant %v", missing, wantMissing)
	}
	if len(got.Invalid) != 0 {
		t.Errorf("invalid = %+v, want none", got.Invalid)
	}

	// The declarations travel with the pack.
	for _, p := range got.Packs {
		if len(p.Generated) != 1 || p.Generated[0].Regenerate != "kitgen" {
			t.Errorf("pack %s: generated = %+v, want the one kitgen declaration", p.Name, p.Generated)
		}
	}

	// Deterministic: a second run reads the same packs and hashes, and the
	// hash covers every file of the pack, not only its header.
	again, err := Discover(repo)
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("second Discover differs: %v\n got %+v\nwant %+v", err, again, got)
	}
	writeFile(t, gomodcache, "example.test/!kit!cache@v1.3.0/fathom-pack/fixtures/generated.cue", "pairs: [{}]\n")
	moved, err := Discover(repo)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Packs[0].Hash == got.Packs[0].Hash {
		t.Errorf("editing a pack file left its hash %s unchanged", got.Packs[0].Hash)
	}
}

// TestDiscoverDisclosesInvalidPack: a fathom-pack/pack.cue that carries a
// header but fails #Pack (here a kit claiming orientation, the project's
// alone) is named, never served and never silently dropped.
func TestDiscoverDisclosesInvalidPack(t *testing.T) {
	base := t.TempDir()
	gomodcache, _ := offlineEnv(t, base)
	repo := filepath.Join(base, "repo")
	writeFile(t, repo, "go.mod", "module example.test/app\n\ngo 1.22\n\nrequire example.test/badkit v1.0.0\n")
	writeFile(t, gomodcache, "example.test/badkit@v1.0.0/fathom-pack/pack.cue",
		"pack: {name: \"bad-kit\", version: 1, about: \"x\"}\norientation: [{anchor: \"a\", why: \"b\"}]\n")
	got, err := Discover(repo)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Packs) != 0 {
		t.Errorf("packs = %+v, want none (the pack is invalid)", got.Packs)
	}
	if len(got.Invalid) != 1 || got.Invalid[0].Module != "example.test/badkit" || got.Invalid[0].Err == "" {
		t.Errorf("invalid = %+v, want the one bad-kit entry with its error", got.Invalid)
	}
}

// TestDiscoverNoPinsIsEmpty: a directory with no go.mod and no cue.mod is an
// empty discovery, not an error.
func TestDiscoverNoPinsIsEmpty(t *testing.T) {
	base := t.TempDir()
	offlineEnv(t, base)
	got, err := Discover(base)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Packs)+len(got.Missing)+len(got.Invalid) != 0 {
		t.Errorf("got %+v, want an empty discovery", got)
	}
}

package docindex

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// cacheLocator is the test double for the DepCacheLocator port: it maps a
// package's canonical key to a directory a test laid out the way the
// toolchain's own cache would, and answers ErrNotCached for anything else —
// exactly the seam the dependency plane's cache locator (SG-36) fills.
func cacheLocator(dirs map[string]string) DepCacheLocator {
	return func(_ context.Context, ref PkgRef) (string, error) {
		if d, ok := dirs[ref.Key()]; ok {
			return d, nil
		}
		return "", ErrNotCached
	}
}

func findDepUnit(files []depDocFile, path, heading string) (store.DocUnit, bool) {
	for _, f := range files {
		if f.Path != path {
			continue
		}
		for _, u := range f.Units {
			if u.HeadingPath == heading {
				return u, true
			}
		}
	}
	return store.DocUnit{}, false
}

func depHeadings(files []depDocFile) []string {
	var out []string
	for _, f := range files {
		for _, u := range f.Units {
			out = append(out, f.Path+"|"+u.HeadingPath)
		}
	}
	return out
}

func requireDepUnit(t *testing.T, files []depDocFile, path, heading string) store.DocUnit {
	t.Helper()
	u, ok := findDepUnit(files, path, heading)
	if !ok {
		t.Fatalf("no unit %q in %q; have %v", heading, path, depHeadings(files))
	}
	return u
}

func refuteDepUnit(t *testing.T, files []depDocFile, path, heading string) {
	t.Helper()
	if _, ok := findDepUnit(files, path, heading); ok {
		t.Fatalf("unit %q in %q must not be indexed (not public surface); have %v", heading, path, depHeadings(files))
	}
}

const goGizmoSrc = `// Package gizmo frobnicates widgets.
//
// It is the documented entry point.
package gizmo

// Frob turns a widget inside out and returns the result.
func Frob(w Widget) Widget { return w }

// Widget is the thing that gets frobnicated.
type Widget struct{ Name string }

// Spin rotates the widget one full turn.
func (w *Widget) Spin() {}

// hidden is unexported and never public surface.
func hidden() {}

// Limit bounds how many widgets are frobnicated at once.
const Limit = 8

// internalHelper is a method on an unexported receiver.
type helper struct{}

// Do belongs to an unexported type, so it is not public API.
func (h helper) Do() {}

func Undocumented() {}
`

// TestDepDocUnitsFromCache pins KR-11: the doc units of a dependency come
// from the version-exact directory the toolchain's cache already holds (a
// DepCacheLocator resolves it; nothing is fetched, I56) — the package
// README plus doc comments per ecosystem: Go doc comments, .d.ts JSDoc,
// Python docstrings and Rust `///`. Only public surface is read.
func TestDepDocUnitsFromCache(t *testing.T) {
	ctx := context.Background()

	t.Run("golang", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "README.md", "# gizmo\n\nGizmo frobnicates widgets.\n\n## Usage\n\nCall Frob with a widget.\n")
		writeFile(t, dir, "gizmo.go", goGizmoSrc)
		writeFile(t, dir, "gizmo_test.go", "// Package gizmo tests.\npackage gizmo\n\n// TestOnly must never be indexed.\nfunc TestOnly() {}\n")
		writeFile(t, dir, "internal/deep/deep.go", "package deep\n\n// Deep is internal and never public surface.\nfunc Deep() {}\n")
		writeFile(t, dir, "testdata/fx.go", "package fx\n\n// Fixture is test data.\nfunc Fixture() {}\n")
		writeFile(t, dir, "nested/node_modules/x.go", "package x\n\n// Vendored is another package's tree.\nfunc Vendored() {}\n")

		ref := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo", Version: "v1.2.3"}
		files, rep, err := depDocUnits(ctx, cacheLocator(map[string]string{ref.Key(): dir}), ref)
		if err != nil {
			t.Fatalf("depDocUnits: %v", err)
		}
		if rep.Units == 0 || rep.Files == 0 || rep.BytesRead == 0 {
			t.Fatalf("report = %+v, want units, files and bytes counted", rep)
		}

		pkgDoc := requireDepUnit(t, files, "gizmo.go", "package gizmo")
		if !strings.Contains(pkgDoc.Body, "documented entry point") || pkgDoc.Kind != KindDepDoc {
			t.Fatalf("package doc unit = %+v", pkgDoc)
		}
		frob := requireDepUnit(t, files, "gizmo.go", "Frob")
		if !strings.Contains(frob.Body, "func Frob(w Widget) Widget") || !strings.Contains(frob.Body, "inside out") {
			t.Fatalf("Frob body should carry signature and doc, got %q", frob.Body)
		}
		if frob.LineStart != 6 {
			t.Fatalf("Frob LineStart = %d, want 6 (the doc comment's first line)", frob.LineStart)
		}
		requireDepUnit(t, files, "gizmo.go", "Widget")
		requireDepUnit(t, files, "gizmo.go", "Widget.Spin")
		requireDepUnit(t, files, "gizmo.go", "Limit")
		refuteDepUnit(t, files, "gizmo.go", "hidden")
		refuteDepUnit(t, files, "gizmo.go", "helper")
		refuteDepUnit(t, files, "gizmo.go", "helper.Do")
		refuteDepUnit(t, files, "gizmo.go", "Undocumented")
		for _, f := range files {
			if strings.HasSuffix(f.Path, "_test.go") || strings.HasPrefix(f.Path, "internal/") ||
				strings.HasPrefix(f.Path, "testdata/") || strings.Contains(f.Path, "node_modules") {
				t.Fatalf("file %q must not be read: %v", f.Path, depHeadings(files))
			}
		}

		readme := requireDepUnit(t, files, "README.md", "gizmo > Usage")
		if readme.Kind != KindDepReadme || !strings.Contains(readme.Body, "Call Frob") {
			t.Fatalf("README unit = %+v", readme)
		}

		// Determinism (internal/CLAUDE.md): the same cache reads to the same
		// units, ids and order every time.
		again, _, err := depDocUnits(ctx, cacheLocator(map[string]string{ref.Key(): dir}), ref)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(depHeadings(files), "\n") != strings.Join(depHeadings(again), "\n") {
			t.Fatalf("unit order not deterministic:\n%v\n%v", depHeadings(files), depHeadings(again))
		}
		u1 := requireDepUnit(t, again, "gizmo.go", "Frob")
		if u1.UnitID != frob.UnitID || u1.ContentHash != frob.ContentHash || u1.UnitID == "" || u1.ContentHash == "" {
			t.Fatalf("unit identity not stable: %+v vs %+v", frob, u1)
		}
	})

	t.Run("npm", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "README.md", "# leftpad\n\nPads strings on the left.\n")
		writeFile(t, dir, "index.d.ts", strings.Join([]string{
			"/**",
			" * Pads a string on the left up to a length.",
			" * @param s the input string",
			" * @returns the padded string",
			" */",
			"export declare function leftPad(s: string, len: number): string;",
			"",
			"/** The options accepted by the padder. */",
			"export interface PadOptions {",
			"  /** The fill character. */",
			"  fill?: string;",
			"  /** Not part of the public surface. */",
			"  private secret: string;",
			"}",
			"",
			"/** Overload one. */",
			"export declare function pad(a: string): string;",
			"/** Overload two. */",
			"export declare function pad(a: number): string;",
			"",
		}, "\n"))
		writeFile(t, dir, "node_modules/other/index.d.ts", "/** Belongs to another package. */\nexport declare function foreign(): void;\n")

		ref := PkgRef{Ecosystem: "npm", Name: "leftpad", Version: "1.3.0"}
		files, _, err := depDocUnits(ctx, cacheLocator(map[string]string{ref.Key(): dir}), ref)
		if err != nil {
			t.Fatalf("depDocUnits: %v", err)
		}
		lp := requireDepUnit(t, files, "index.d.ts", "leftPad")
		if !strings.Contains(lp.Body, "Pads a string on the left") || !strings.Contains(lp.Body, "@returns the padded string") ||
			!strings.Contains(lp.Body, "export declare function leftPad") {
			t.Fatalf("leftPad body = %q", lp.Body)
		}
		requireDepUnit(t, files, "index.d.ts", "PadOptions")
		requireDepUnit(t, files, "index.d.ts", "fill")
		refuteDepUnit(t, files, "index.d.ts", "secret")
		// Two overloads keep distinct, stable identities (no primary-key clash).
		requireDepUnit(t, files, "index.d.ts", "pad")
		requireDepUnit(t, files, "index.d.ts", "pad (2)")
		for _, f := range files {
			if strings.Contains(f.Path, "node_modules") {
				t.Fatalf("nested node_modules must not be read: %v", depHeadings(files))
			}
		}
		requireDepUnit(t, files, "README.md", "leftpad")
	})

	t.Run("pypi", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "README.rst", "requests-lite\n=============\n\nA tiny HTTP client.\n")
		writeFile(t, dir, "reqlite/__init__.py", strings.Join([]string{
			`"""Tiny HTTP client for scripts."""`,
			"",
			"def fetch(url, timeout=None):",
			`    """Fetch a URL and return the body.`,
			"",
			"    Raises on a non-2xx status.",
			`    """`,
			"    return url",
			"",
			"def _private():",
			`    """Internal helper, never public."""`,
			"",
			"class Session:",
			`    """A reusable connection pool."""`,
			"",
			"    def get(self, url,",
			"            headers=None):",
			`        """Issue a GET request."""`,
			"",
			"    def _sock(self):",
			`        """Private method."""`,
			"",
			"    def __init__(self):",
			`        """Build the pool."""`,
			"",
		}, "\n"))
		// A stub next to a module: the .pyi is the declared surface, the .py is skipped.
		writeFile(t, dir, "reqlite/util.pyi", "def slug(text: str) -> str:\n    \"\"\"Slugify text from the stub.\"\"\"\n")
		writeFile(t, dir, "reqlite/util.py", "def slug(text):\n    \"\"\"Slugify text from the implementation.\"\"\"\n")
		writeFile(t, dir, "tests/test_x.py", "def test_x():\n    \"\"\"A test.\"\"\"\n")

		ref := PkgRef{Ecosystem: "pypi", Name: "requests-lite", Version: "0.4.1"}
		files, _, err := depDocUnits(ctx, cacheLocator(map[string]string{ref.Key(): dir}), ref)
		if err != nil {
			t.Fatalf("depDocUnits: %v", err)
		}
		requireDepUnit(t, files, "reqlite/__init__.py", "module reqlite/__init__.py")
		f := requireDepUnit(t, files, "reqlite/__init__.py", "fetch")
		if !strings.Contains(f.Body, "Fetch a URL and return the body.") || !strings.Contains(f.Body, "Raises on a non-2xx status.") {
			t.Fatalf("fetch body = %q", f.Body)
		}
		requireDepUnit(t, files, "reqlite/__init__.py", "Session")
		requireDepUnit(t, files, "reqlite/__init__.py", "Session.get")
		requireDepUnit(t, files, "reqlite/__init__.py", "Session.__init__")
		refuteDepUnit(t, files, "reqlite/__init__.py", "_private")
		refuteDepUnit(t, files, "reqlite/__init__.py", "Session._sock")
		stub := requireDepUnit(t, files, "reqlite/util.pyi", "slug")
		if !strings.Contains(stub.Body, "from the stub") {
			t.Fatalf("stub body = %q", stub.Body)
		}
		refuteDepUnit(t, files, "reqlite/util.py", "slug")
		for _, f := range files {
			if strings.HasPrefix(f.Path, "tests/") {
				t.Fatalf("tests/ must not be read: %v", depHeadings(files))
			}
		}
		if u := requireDepUnit(t, files, "README.rst", "README"); !strings.Contains(u.Body, "A tiny HTTP client.") || u.Kind != KindDepReadme {
			t.Fatalf("non-Markdown README unit = %+v", u)
		}
	})

	t.Run("cargo", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "README.md", "# adder\n\nAdds numbers.\n")
		writeFile(t, dir, "src/lib.rs", strings.Join([]string{
			"//! Adder is a tiny arithmetic crate.",
			"//!",
			"//! It only adds.",
			"",
			"/// Adds two numbers.",
			"///",
			"/// Overflow panics in debug builds.",
			"pub fn add(a: i32, b: i32) -> i32 { a + b }",
			"",
			"/// A running total.",
			"#[derive(Debug, Clone)]",
			"pub struct Total(pub i32);",
			"",
			"/// Crate-private helper.",
			"pub(crate) fn helper() {}",
			"",
			"/// Private helper.",
			"fn private() {}",
			"",
		}, "\n"))
		writeFile(t, dir, "tests/it.rs", "/// A test.\npub fn t() {}\n")
		writeFile(t, dir, "target/debug/x.rs", "/// Build output.\npub fn built() {}\n")

		ref := PkgRef{Ecosystem: "cargo", Name: "adder", Version: "0.9.0"}
		files, _, err := depDocUnits(ctx, cacheLocator(map[string]string{ref.Key(): dir}), ref)
		if err != nil {
			t.Fatalf("depDocUnits: %v", err)
		}
		crate := requireDepUnit(t, files, "src/lib.rs", "crate")
		if !strings.Contains(crate.Body, "tiny arithmetic crate") || !strings.Contains(crate.Body, "It only adds.") {
			t.Fatalf("crate doc = %q", crate.Body)
		}
		add := requireDepUnit(t, files, "src/lib.rs", "add")
		if !strings.Contains(add.Body, "Adds two numbers.") || !strings.Contains(add.Body, "pub fn add(a: i32, b: i32)") {
			t.Fatalf("add body = %q", add.Body)
		}
		requireDepUnit(t, files, "src/lib.rs", "Total")
		refuteDepUnit(t, files, "src/lib.rs", "helper")
		refuteDepUnit(t, files, "src/lib.rs", "private")
		for _, f := range files {
			if strings.HasPrefix(f.Path, "tests/") || strings.HasPrefix(f.Path, "target/") {
				t.Fatalf("%q must not be read: %v", f.Path, depHeadings(files))
			}
		}
	})
}

// TestDepDocUnitsRefusesWhatItCannotRead pins that a package the cache does
// not hold is refused with the reason (never fetched, I56), an ecosystem
// without a reader is named rather than answered empty (I38), a version-
// less ref cannot be read, and a located directory that has vanished is
// refused as not on disk, never a healthy zero.
func TestDepDocUnitsRefusesWhatItCannotRead(t *testing.T) {
	ctx := context.Background()
	loc := cacheLocator(map[string]string{})

	ref := PkgRef{Ecosystem: "golang", Name: "example.com/absent", Version: "v0.0.1"}
	files, _, err := depDocUnits(ctx, loc, ref)
	if !errors.Is(err, ErrNotCached) || len(files) != 0 {
		t.Fatalf("absent package: files=%v err=%v, want ErrNotCached and no units", files, err)
	}
	if !strings.Contains(err.Error(), "not on disk") || !strings.Contains(err.Error(), "package manager") {
		t.Fatalf("refusal should name the reason and the user's one command, got %q", err)
	}

	for _, eco := range []string{"maven", "nuget"} {
		_, _, err := depDocUnits(ctx, cacheLocator(map[string]string{"pkg:" + eco + "/x/y@1": t.TempDir()}), PkgRef{Ecosystem: eco, Name: "x/y", Version: "1"})
		if !errors.Is(err, ErrDepEcosystemUnsupported) {
			t.Fatalf("%s: err = %v, want ErrDepEcosystemUnsupported", eco, err)
		}
	}

	if _, _, err := depDocUnits(ctx, loc, PkgRef{Ecosystem: "golang", Name: "example.com/x"}); err == nil ||
		!strings.Contains(err.Error(), "version") {
		t.Fatalf("version-less ref: err = %v, want a refusal naming the missing version", err)
	}

	gone := PkgRef{Ecosystem: "golang", Name: "example.com/gone", Version: "v1.0.0"}
	_, _, err = depDocUnits(ctx, cacheLocator(map[string]string{gone.Key(): filepath.Join(t.TempDir(), "does-not-exist")}), gone)
	if !errors.Is(err, ErrNotCached) {
		t.Fatalf("a located directory that has vanished is not on disk: err = %v, want ErrNotCached", err)
	}
}

// TestDepDocUnitsBoundedRead pins the design's §8.3 bounds: a file over the
// per-file cap is skipped and NAMED, and a package that would exceed the
// total source cap stops reading and says so in Partial — never a silent
// trim (I38).
func TestDepDocUnitsBoundedRead(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# big\n\nreadme text\n")
	writeFile(t, dir, "a.go", "package big\n\n// A is small.\nfunc A() {}\n")
	writeFile(t, dir, "b.go", "package big\n\n// B is small.\nfunc B() {}\n"+strings.Repeat("// padding padding padding\n", 200))
	writeFile(t, dir, "c.go", "package big\n\n// C is small.\nfunc C() {}\n")
	ref := PkgRef{Ecosystem: "golang", Name: "example.com/big", Version: "v1.0.0"}
	loc := cacheLocator(map[string]string{ref.Key(): dir})

	// Per-file cap: b.go is bigger than 1 KiB and is skipped, named.
	files, rep, err := depDocUnitsLimited(ctx, loc, ref, depLimits{totalBytes: 1 << 20, fileBytes: 1024, entries: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0].Path != "b.go" || !strings.Contains(rep.Skipped[0].Reason, "per-file cap") {
		t.Fatalf("skipped = %+v, want b.go named with the per-file cap", rep.Skipped)
	}
	requireDepUnit(t, files, "a.go", "A")
	requireDepUnit(t, files, "c.go", "C")
	refuteDepUnit(t, files, "b.go", "B")

	// Total cap: the README is read first, then sources in path order until
	// the cap; the cut is named.
	files, rep, err = depDocUnitsLimited(ctx, loc, ref, depLimits{totalBytes: 130, fileBytes: 1 << 20, entries: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Partial == "" || !strings.Contains(rep.Partial, "cap") {
		t.Fatalf("Partial = %q, want a named cap", rep.Partial)
	}
	requireDepUnit(t, files, "README.md", "big")
	refuteDepUnit(t, files, "c.go", "C")

	// Entry cap: a wide tree stops the walk, named.
	_, rep, err = depDocUnitsLimited(ctx, loc, ref, depLimits{totalBytes: 1 << 20, fileBytes: 1 << 20, entries: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.Partial, "entries") {
		t.Fatalf("Partial = %q, want the entry cap named", rep.Partial)
	}
}

// TestDepDocsNeverDial pins I56 for this package: nothing in docindex may
// import the network or spawn a process — dependency docs come only from
// what the toolchain already placed on disk.
func TestDepDocsNeverDial(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "net" || strings.HasPrefix(p, "net/") || p == "os/exec" {
				t.Fatalf("%s imports %q: docindex must never dial or exec (I56)", n, p)
			}
		}
	}
}

func TestParsePkgRef(t *testing.T) {
	cases := []struct {
		in      string
		want    PkgRef
		wantKey string
		wantErr string
	}{
		{in: "pkg:golang/github.com/x/y@v1.2.3", want: PkgRef{"golang", "github.com/x/y", "v1.2.3"}, wantKey: "pkg:golang/github.com/x/y@v1.2.3"},
		{in: "golang/github.com/x/y@v1.2.3", want: PkgRef{"golang", "github.com/x/y", "v1.2.3"}, wantKey: "pkg:golang/github.com/x/y@v1.2.3"},
		{in: "pkg:NPM/@scope/name@2.0.0", want: PkgRef{"npm", "@scope/name", "2.0.0"}, wantKey: "pkg:npm/@scope/name@2.0.0"},
		{in: "pkg:pypi/requests@2.31.0", want: PkgRef{"pypi", "requests", "2.31.0"}, wantKey: "pkg:pypi/requests@2.31.0"},
		{in: "pkg:golang/github.com/x/y", want: PkgRef{"golang", "github.com/x/y", ""}, wantKey: "pkg:golang/github.com/x/y"},
		{in: "pkg:golang/x@v1#Frob", wantErr: "symbol"},
		{in: "pkg:bogus/x@1", wantErr: "ecosystem"},
		{in: "", wantErr: "empty"},
		{in: "pkg:golang/", wantErr: "ecosystem"},
	}
	for _, c := range cases {
		got, err := ParsePkgRef(c.in)
		if c.wantErr != "" {
			if err == nil || !errors.Is(err, ErrBadPkgKey) || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("ParsePkgRef(%q) err = %v, want ErrBadPkgKey mentioning %q", c.in, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePkgRef(%q): %v", c.in, err)
			continue
		}
		if got != c.want || got.Key() != c.wantKey {
			t.Errorf("ParsePkgRef(%q) = %+v key %q, want %+v key %q", c.in, got, got.Key(), c.want, c.wantKey)
		}
	}
}

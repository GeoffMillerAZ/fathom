package usedset

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// goListDeps runs `go list -deps` of pkgs in root and keeps the packages of
// module mod (its nested modules excluded by the go command itself, which
// resolves them elsewhere).
func goListDeps(t *testing.T, root, mod string, pkgs ...string) []string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list", "-deps"}, pkgs...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly", "GOPROXY=off")
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("go list -deps %v: %v\n%s", pkgs, err, stderr)
	}
	var keep []string
	for _, p := range strings.Fields(string(out)) {
		if p == mod || strings.HasPrefix(p, mod+"/") {
			keep = append(keep, p)
		}
	}
	sort.Strings(keep)
	return keep
}

func goUnits(r Result) []string {
	var out []string
	for _, u := range r.Used {
		if u.Kind == KindGoPackage {
			out = append(out, u.ID)
		}
	}
	sort.Strings(out)
	return out
}

func unitsOf(us []Unit, kind string) map[string]Unit {
	m := map[string]Unit{}
	for _, u := range us {
		if u.Kind == kind {
			m[u.ID] = u
		}
	}
	return m
}

// buildFixture lays out a consumer module that mirrors a kit under
// third_party/kit through a replace, the shape of fathom's own mirrors.
func buildFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n\nrequire example.com/kit v0.0.0\n\nreplace example.com/kit => ./third_party/kit\n")
	write(t, root, "cmd/app/main.go", "package main\n\nimport (\n\t_ \"example.com/app/internal/x\"\n\t_ \"example.com/kit/ui/card\"\n)\n\nfunc main() {}\n")
	write(t, root, "internal/x/x.go", "package x\n\nimport _ \"example.com/kit/a\"\n")
	write(t, root, "internal/x/x_test.go", "package x\n\nimport _ \"example.com/kit/c\"\n")
	write(t, root, "cmd/tool/main.go", "package main\n\nimport _ \"example.com/kit/b\"\n\nfunc main() {}\n")
	write(t, root, "workspace/scratch/main.go", "package main\n\nimport _ \"example.com/kit/c\"\n\nfunc main() {}\n")
	write(t, root, "ui/input.css", "@import \"tailwindcss\" source(none);\n@import \"../third_party/kit/ui/docs/prose.css\";\n@source \"../third_party/kit/ui/card/*.go\";\n@source \"../internal/x/*.go\";\n")

	k := "third_party/kit/"
	write(t, root, k+"go.mod", "module example.com/kit\n\ngo 1.22\n")
	write(t, root, k+"go.sum", "")
	write(t, root, k+"README.md", "# kit\n")
	write(t, root, k+"a/a.go", "package a\n\nimport _ \"example.com/kit/b\"\n")
	write(t, root, k+"a/a_test.go", "package a\n\nimport _ \"example.com/kit/c\"\n")
	write(t, root, k+"a/gen.go", "//go:build ignore\n\npackage main\n\nimport _ \"example.com/kit/c\"\n")
	write(t, root, k+"a/a_plan9.go", "package a\n\nimport _ \"example.com/kit/c\"\n")
	write(t, root, k+"a/testdata/golden.txt", "g\n")
	write(t, root, k+"b/b.go", "package b\n\nimport \"strings\"\n\nvar _ = strings.ToUpper\n")
	write(t, root, k+"c/c.go", "package c\n")
	write(t, root, k+"ui/card/card.go", "package card\n\nimport \"embed\"\n\n//go:embed static/app.js static/img\nvar FS embed.FS\n\n//go:embed \"tmpl/*.html\"\nvar T embed.FS\n")
	write(t, root, k+"ui/card/static/app.js", "import { u } from './util.js';\nexport const a = u;\n")
	write(t, root, k+"ui/card/static/util.js", "export const u = 1;\n")
	write(t, root, k+"ui/card/static/img/logo.svg", "<svg/>\n")
	write(t, root, k+"ui/card/static/img/.hidden", "x\n")
	write(t, root, k+"ui/card/static/unused.js", "export const z = 2;\n")
	write(t, root, k+"ui/card/tmpl/card.html", "<div></div>\n")
	write(t, root, k+"ui/docs/prose.css", ".prose{}\n")
	write(t, root, k+"ui/docs/docs.go", "package docs\n")
	write(t, root, k+"web/orphan.ts", "import './other';\n")
	write(t, root, k+"web/other.ts", "export {};\n")
	// A nested module is another module: never a unit of this one.
	write(t, root, k+"tools/go.mod", "module example.com/kit/tools\n\ngo 1.22\n")
	write(t, root, k+"tools/t.go", "package tools\n")
	return root
}

func TestUsedUnitsGoMatchesListDepsAndAssetsDeclared(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	root := buildFixture(t)
	res, err := UsedUnits(Options{Root: root, Dir: "third_party/kit"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Module != "example.com/kit" {
		t.Fatalf("module = %q", res.Module)
	}
	if strings.Join(res.Entries, ",") != "example.com/app/cmd/app,example.com/app/cmd/tool" {
		t.Errorf("entries = %v (workspace/ is no product entry point)", res.Entries)
	}

	// Go units equal `go list -deps` of the same main packages, restricted
	// to the kit's module.
	want := goListDeps(t, root, "example.com/kit", "./cmd/app", "./cmd/tool")
	if got := goUnits(res); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("go units = %v, want go list -deps = %v", got, want)
	}
	gos := unitsOf(res.Used, KindGoPackage)
	// Breadth first from the sorted entries: the shortest import chain names
	// the reason, so kit/b is cmd/tool's own import, not cmd/app's third hop.
	if u := gos["example.com/kit/b"]; u.Entry != "example.com/app/cmd/tool" || u.FirstImporter != "example.com/app/cmd/tool" {
		t.Errorf("kit/b reason = entry %q, first importer %q; want cmd/tool directly", u.Entry, u.FirstImporter)
	}
	if u := gos["example.com/kit/a"]; u.Entry != "example.com/app/cmd/app" || u.FirstImporter != "example.com/app/internal/x" || u.Dir != "third_party/kit/a" {
		t.Errorf("kit/a = %+v", u)
	}
	// A package keeps every file in its directory, its testdata included:
	// files inside a Go package are never pruned.
	if u := gos["example.com/kit/a"]; u.Files != 5 {
		t.Errorf("kit/a files = %d, want 5 (a.go, a_test.go, gen.go, a_plan9.go, testdata/golden.txt)", u.Files)
	}

	// Declared assets: the go:embed patterns of a reached package (a
	// directory embed skips dot files), the CSS entry's @source and @import
	// into the kit, the module files the replace needs.
	emb := unitsOf(res.Used, KindEmbed)
	for _, f := range []string{"third_party/kit/ui/card/static/app.js", "third_party/kit/ui/card/static/img/logo.svg", "third_party/kit/ui/card/tmpl/card.html"} {
		if u, ok := emb[f]; !ok || u.FirstImporter != "example.com/kit/ui/card" {
			t.Errorf("embed unit %s = %+v, ok %v", f, u, ok)
		}
	}
	if _, ok := emb["third_party/kit/ui/card/static/img/.hidden"]; ok {
		t.Error("a directory embed included a dot file")
	}
	css := unitsOf(res.Used, KindCSSSource)
	if u, ok := css["third_party/kit/ui/card/*.go"]; !ok || u.Entry != "ui/input.css" {
		t.Errorf("css source unit = %+v ok %v (got %v)", u, ok, css)
	}
	if len(css) != 1 {
		t.Errorf("css sources = %v: an @source outside the kit is not a kit unit", css)
	}
	if _, ok := unitsOf(res.Used, KindCSSImport)["third_party/kit/ui/docs/prose.css"]; !ok {
		t.Errorf("the @import of prose.css is not a unit: %+v", res.Used)
	}
	mods := unitsOf(res.Used, KindModuleFile)
	if _, ok := mods["third_party/kit/go.mod"]; !ok {
		t.Errorf("go.mod is not a unit: %v", mods)
	}

	// TS/JS modules by import edges from what is reached: util.js through
	// app.js; orphan.ts and other.ts are reached by nothing.
	js := unitsOf(res.Used, KindJSModule)
	if u, ok := js["third_party/kit/ui/card/static/util.js"]; !ok || u.FirstImporter != "third_party/kit/ui/card/static/app.js" {
		t.Errorf("util.js = %+v ok %v (js units %v)", u, ok, js)
	}

	// The rest is listed unreached: c (only a test, an ignored file and a
	// plan9 file import it), ui/docs as a Go package (its CSS is imported,
	// its Go is not), the orphan TS modules and the unembedded JS.
	un := map[string]bool{}
	for _, u := range res.Unreached {
		un[u.Kind+" "+u.ID] = true
	}
	for _, w := range []string{
		"go-package example.com/kit/c",
		"go-package example.com/kit/ui/docs",
		"js-module third_party/kit/web/orphan.ts",
		"js-module third_party/kit/web/other.ts",
		"js-module third_party/kit/ui/card/static/unused.js",
	} {
		if !un[w] {
			t.Errorf("unreached lacks %q: %v", w, un)
		}
	}
	for k := range un {
		if strings.Contains(k, "kit/tools") {
			t.Errorf("a nested module's package is listed: %s", k)
		}
	}
	if res.Totals.UsedFiles == 0 || res.Totals.UsedFiles >= res.Totals.Files || res.Totals.UsedBytes >= res.Totals.Bytes {
		t.Errorf("totals = %+v", res.Totals)
	}

	// Restricting the entry points restricts the closure.
	only, err := UsedUnits(Options{Root: root, Dir: "third_party/kit", Mains: []string{"cmd/tool"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := goUnits(only); strings.Join(got, ",") != "example.com/kit/b" {
		t.Errorf("cmd/tool alone reaches %v, want kit/b", got)
	}
}

// TestUsedUnitsMatchesListDepsOnThisRepository asserts the same equality on
// fathom's own mirrors: cmd/fathom's closure in app-kit and agentic-kit.
func TestUsedUnitsMatchesListDepsOnThisRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the whole repository")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	for _, c := range []struct{ dir, mod string }{
		{"third_party/app-kit", "github.com/geoffmilleraz/app-kit"},
		{"third_party/agentic-kit", "github.com/geoffmilleraz/agentic-kit"},
	} {
		res, err := UsedUnits(Options{Root: root, Dir: c.dir, Mains: []string{"cmd/fathom"}})
		if err != nil {
			t.Fatal(err)
		}
		want := goListDeps(t, root, c.mod, "./cmd/fathom")
		if got := goUnits(res); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: go units (%d) differ from go list -deps (%d):\n got %v\nwant %v", c.dir, len(got), len(want), got, want)
		}
	}
}

package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestResolveFileEdges_TSJSModuleSymbol (LB-F381, K18): with the typescript and
// javascript module symbol, through fusion's one "javascript" extractor, a
// file's top-level call and a decorator argument written outside the class's
// lines (`@Module({ providers: [X] }) export class …`) are made from the
// file's module symbol; an import of a module — a side-effect import, `import
// * as`, a named import of a barrel, a CommonJS require — is an edge to that
// module's symbol; and a barrel's re-exports (`export * from`, `export { B }
// from`) are edges from the barrel's module symbol to each module it
// re-exports. A name imported through the barrel still binds the definition
// it names, as before.
func TestResolveFileEdges_TSJSModuleSymbol(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/app.service.ts": "export class AppService {\n  hello() {\n    return 1;\n  }\n}\n",
		"src/util.ts":        "export function config() {\n  return {};\n}\n",
		"src/polyfill.ts":    "install();\n",
		"src/a.ts":           "export function fromA() {\n  return 1;\n}\n",
		"src/b.ts":           "export class B {}\n",
		"src/index.ts":       "export * from \"./a\";\nexport { B } from \"./b\";\n",
		"src/app.module.ts": strings.Join([]string{
			`import { Module } from "@nestjs/common";`,
			`import "./polyfill";`,
			`import * as util from "./util";`,
			`import { AppService } from "./app.service";`,
			`import { fromA, B } from "./index";`,
			``,
			`setup(util.config());`,
			``,
			`@Module({ providers: [AppService] })`,
			`export class AppModule {`,
			`  run() {`,
			`    return fromA();`,
			`  }`,
			`}`,
			``,
		}, "\n"),
		"lib/helper.js": "function helper() {\n  return 1;\n}\nmodule.exports = { helper };\n",
		"lib/main.js": strings.Join([]string{
			`const path = require("path");`,
			`const { helper } = require("./helper");`,
			`const h = require("./helper");`,
			``,
			`setup(helper());`,
			``,
		}, "\n"),
	}
	var rels []string
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	srcs, symbols := loadFileResolveFixture(t, dir, "javascript", rels)
	edges, report := fusion.ResolveFileEdges(context.Background(), "javascript", dir, srcs, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	mod := func(pkg string) string { return fkey(pkg, core.ModuleSymbolName) }
	got := map[[2]string]bool{}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: kind %q, want file-resolved", e, e.Kind)
		}
		got[[2]string{e.FromSymbol, e.ToSymbol}] = true
	}
	want := [][2]string{
		{mod("src/app.module"), mod("src/polyfill")},                      // import "./polyfill"
		{mod("src/app.module"), mod("src/util")},                          // import * as util
		{mod("src/app.module"), mod("src/index")},                         // the barrel it imports from
		{mod("src/app.module"), fkey("src/util", "config")},               // the top-level call util.config()
		{mod("src/app.module"), fkey("src/app.service", "AppService")},    // the decorator's argument
		{fkey("src/app.module", "AppModule.run"), fkey("src/a", "fromA")}, // through the barrel, as before
		{mod("src/index"), mod("src/a")},                                  // export * from "./a"
		{mod("src/index"), mod("src/b")},                                  // export { B } from "./b"
		{mod("lib/main"), mod("lib/helper")},                              // require("./helper"), both forms
		{mod("lib/main"), fkey("lib/helper", "helper")},                   // the top-level call helper()
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s -> %s", w[0], w[1])
		}
	}
	// LB-F411 (D1): `import { AppService }` draws the decorator argument's
	// symbol edge into app.service, so its module edge is no longer stored.
	if got[[2]string{mod("src/app.module"), mod("src/app.service")}] {
		t.Errorf("a named import whose file already draws a symbol edge into its target kept its module edge")
	}
	for e := range got {
		if strings.HasSuffix(e[1], "."+core.ModuleSymbolName) && !strings.HasSuffix(e[0], "."+core.ModuleSymbolName) {
			t.Errorf("an edge to a module symbol from inside a definition, with no import there: %s -> %s", e[0], e[1])
		}
	}
	if t.Failed() {
		t.Logf("edges: %v", got)
	}
}

// writeTSJSFixture writes files under a fresh directory and returns it and
// the files' relative paths, sorted.
func writeTSJSFixture(t *testing.T, files map[string]string) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	var rels []string
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	return dir, rels
}

// moduleLeverFiles is LB-F411's fixture: each form of import a TS/JS file
// writes, beside a module of the same file that draws a symbol edge or not.
var moduleLeverFiles = map[string]string{
	"src/a.ts":      "export function f() {\n  return 1;\n}\n",
	"src/b.ts":      "export default function make() {\n  return 2;\n}\n",
	"src/c.ts":      "export function c1() {\n  return 3;\n}\n",
	"src/d.ts":      "export function d1() {\n  return 4;\n}\n",
	"src/e.ts":      "export function unused() {\n  return 5;\n}\n",
	"src/barrel.ts": "export * from \"./a\";\nexport { c1 } from \"./c\";\n",
	"src/main.ts": strings.Join([]string{
		`import { f } from "./a";`,
		`import make from "./b";`,
		`import * as ns from "./c";`,
		`import "./d";`,
		`import { d1 } from "./d";`,
		`import { unused } from "./e";`,
		``,
		`export function run() {`,
		`  return f() + make() + ns.c1() + d1();`,
		`}`,
		``,
	}, "\n"),
	"src/a.test.ts": strings.Join([]string{
		`import { f } from "./a";`,
		``,
		`describe("a", () => {`,
		`  it("returns one", () => {`,
		`    expect(f()).toBe(1);`,
		`  });`,
		`});`,
		``,
	}, "\n"),
	"src/a.spec.ts": strings.Join([]string{
		`import * as a from "./a";`,
		``,
		`describe("a", () => {`,
		`  it("returns one", () => {`,
		`    expect(a.f()).toBe(1);`,
		`  });`,
		`});`,
		``,
	}, "\n"),
	"lib/helper.js": "function helper() {\n  return 1;\n}\nmodule.exports = { helper };\n",
	"lib/main.js": strings.Join([]string{
		`const { helper } = require("./helper");`,
		``,
		`function go() {`,
		`  return helper();`,
		`}`,
		`module.exports = { go };`,
		``,
	}, "\n"),
}

// moduleLeverWant is what LB-F411 keeps and drops, by (from, to) pair.
func moduleLeverWant(key func(pkg, name string) string) (keep, drop [][2]string) {
	mod := func(pkg string) string { return key(pkg, core.ModuleSymbolName) }
	keep = [][2]string{
		{mod("src/main"), mod("src/c")},      // import * as ns, though ns.c1() draws a symbol edge
		{mod("src/main"), mod("src/d")},      // import "./d", though d1() draws one through a named import
		{mod("src/main"), mod("src/e")},      // a named import that draws no symbol edge
		{mod("src/barrel"), mod("src/a")},    // export * from "./a"
		{mod("src/barrel"), mod("src/c")},    // export { c1 } from "./c"
		{mod("src/a.spec"), mod("src/a")},    // import * as a in a test file
		{mod("lib/main"), mod("lib/helper")}, // a CommonJS require, though helper() draws one
		{key("src/main", "run"), key("src/a", "f")},
		{key("src/main", "run"), key("src/b", "make")},
		{key("src/main", "run"), key("src/c", "c1")},
		{key("src/main", "run"), key("src/d", "d1")},
		{key("lib/main", "go"), key("lib/helper", "helper")},
		// A test file's it block calls f in its callback (TQ-27: a describe or
		// it block is a test symbol, named by its quoted titles).
		{key("src/a.test", `"a" "returns one"`), key("src/a", "f")},
		{key("src/a.spec", `"a" "returns one"`), key("src/a", "f")},
	}
	drop = [][2]string{
		{mod("src/main"), mod("src/a")},   // import { f }: run -> f is already an edge into a.ts
		{mod("src/main"), mod("src/b")},   // import make: run -> make is already an edge into b.ts
		{mod("src/a.test"), mod("src/a")}, // import { f } in a test file: its module's f() is
	}
	return keep, drop
}

// TestResolveFileEdges_TSJSModuleEdgeLever (LB-F411, owner ruling D1, O748):
// a named or default ES import of a module whose importing file already
// draws a symbol edge into that module's file stores no module-to-module
// edge; a side-effect import, `import * as`, a barrel re-export, a CommonJS
// require and an import that draws no symbol edge to its file keep theirs,
// and a test file's it block draws its callback's calls.
func TestResolveFileEdges_TSJSModuleEdgeLever(t *testing.T) {
	dir, rels := writeTSJSFixture(t, moduleLeverFiles)
	srcs, symbols := loadFileResolveFixture(t, dir, "javascript", rels)
	edges, report := fusion.ResolveFileEdges(context.Background(), "javascript", dir, srcs, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := map[[2]string]bool{}
	for _, e := range edges {
		got[[2]string{e.FromSymbol, e.ToSymbol}] = true
	}
	keep, drop := moduleLeverWant(fkey)
	for _, w := range keep {
		if !got[w] {
			t.Errorf("missing edge %s -> %s", w[0], w[1])
		}
	}
	for _, w := range drop {
		if got[w] {
			t.Errorf("module edge %s -> %s stored, though the file already draws a symbol edge into its target", w[0], w[1])
		}
	}
	if t.Failed() {
		t.Logf("edges: %v", got)
	}
}

// TestIngest_TSJSModuleEdgeLever (LB-F411): through ingest, the typed edges
// stored are LB-F411's, and a test file's describe and it blocks keep the
// call rows their callbacks write (TQ-27: the rows a selection reaches a TS
// test through).
func TestIngest_TSJSModuleEdgeLever(t *testing.T) {
	dir, _ := writeTSJSFixture(t, moduleLeverFiles)
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "lever-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[[2]string]bool{}
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			got[[2]string{e.FromSymbol, e.ToSymbol}] = true
		}
	}
	key := func(pkg, name string) string { return core.SymbolKey(svc.ID, pkg, name) }
	keep, drop := moduleLeverWant(key)
	for _, w := range keep {
		if !got[w] {
			t.Errorf("missing stored edge %s -> %s", w[0], w[1])
		}
	}
	for _, w := range drop {
		if got[w] {
			t.Errorf("stored module edge %s -> %s, though the file already draws a symbol edge into its target", w[0], w[1])
		}
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[[2]string][]string{}
	for _, o := range obs {
		file, _, _ := strings.Cut(o.SourceRef, ":")
		calls[[2]string{file, o.Symbol}] = o.ToSymbolNode().Calls
	}
	for _, file := range []string{"src/a.test.ts", "src/a.spec.ts"} {
		for block, wants := range map[string][]string{`"a"`: {"describe"}, `"a" "returns one"`: {"it", "expect"}} {
			got := calls[[2]string{file, block}]
			for _, want := range wants {
				if !slices.ContainsFunc(got, func(c string) bool { return c == want || strings.HasSuffix(c, "."+want) }) {
					t.Errorf("%s's %s block call rows %v lack %s", file, block, got, want)
				}
			}
		}
	}
	if t.Failed() {
		t.Logf("edges: %v", got)
	}

	// A re-ingest after one file changed reads every other file's imports
	// back from its stored row (file_imports): the binding form survives it.
	if err := os.WriteFile(filepath.Join(dir, "src", "e.ts"), []byte("export function unused() {\n  return 6;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pl2 := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl2.Close)
	if _, err := pl2.Ingest(ctx, svc); err != nil {
		t.Fatalf("re-Ingest: %v", err)
	}
	again, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	reread := map[[2]string]bool{}
	for _, e := range again {
		reread[[2]string{e.FromSymbol, e.ToSymbol}] = true
	}
	for _, w := range drop {
		if reread[w] {
			t.Errorf("after a re-ingest from stored import rows, module edge %s -> %s is back", w[0], w[1])
		}
	}
	for _, w := range keep {
		if !reread[w] {
			t.Errorf("after a re-ingest from stored import rows, edge %s -> %s is missing", w[0], w[1])
		}
	}
}

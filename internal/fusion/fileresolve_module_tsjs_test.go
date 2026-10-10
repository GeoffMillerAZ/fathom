package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
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
		{mod("src/app.module"), mod("src/app.service")},                   // import { AppService }
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
	for e := range got {
		if strings.HasSuffix(e[1], "."+core.ModuleSymbolName) && !strings.HasSuffix(e[0], "."+core.ModuleSymbolName) {
			t.Errorf("an edge to a module symbol from inside a definition, with no import there: %s -> %s", e[0], e[1])
		}
	}
	if t.Failed() {
		t.Logf("edges: %v", got)
	}
}

package extractor

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// tsModuleSrc is a TypeScript module whose top level calls, imports,
// decorates and registers: the module symbol owns exactly the calls no
// definition encloses or claims.
const tsModuleSrc = `import { Module } from "@nestjs/common";
import "./polyfill";
import * as util from "./util";
import { AppService } from "./app.service";

setup(util.config());

@Module({ providers: [AppService], imports: [register()] })
export class AppModule {
  field = make();
  run() {
    return helper();
  }
}

app.use(router);
if (process.env.DEBUG) {
  enableDebug();
}
export function f() {
  return g();
}
`

// jsModuleSrc is its JavaScript (CommonJS) counterpart.
const jsModuleSrc = `const path = require("path");
const { helper } = require("./helper");
require("./side-effect");

setup(path.join("a", "b"));

class K {
  m() {
    return helper();
  }
}

app.use(router);
for (const p of plugins) {
  register(p);
}
function f() {
  return g();
}
`

// TestModuleSymbolTypeScriptAndJavaScript (LB-F381, K18): with the typescript
// and javascript dialects' module_symbol on, every file of either dialect —
// .ts .tsx .mts .cts .js .jsx .mjs .cjs, through fusion's one "javascript"
// extractor (tsExtractorGroups) and through each dialect's own — has one
// module symbol: named core.ModuleSymbolName, kind JSModule (canonical
// module), line 1 to the file's last line, the file's package, unsigned. It
// owns the calls its top level makes and no definition encloses or claims; a
// decorator's call (its arguments' included) stays the class's, a field
// initializer's the field's (LB-F383), a method's and a function's their own.
func TestModuleSymbolTypeScriptAndJavaScript(t *testing.T) {
	if c, ok := CanonicalKind("JSModule"); !ok || c != "module" {
		t.Fatalf("CanonicalKind(JSModule) = %q, %v; want module", c, ok)
	}
	group, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer group.Close()
	tsOnly, err := NewTSExtractor("typescript")
	if err != nil {
		t.Fatal(err)
	}
	defer tsOnly.Close()
	jsOnly, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer jsOnly.Close()

	tsWant := map[string][]string{
		core.ModuleSymbolName: {"app.use", "enableDebug", "setup", "util.config"},
		"AppModule":           {"Module", "register"},
		"AppModule.field":     {"make"}, // a field's initializer (LB-F383)
		"AppModule.run":       {"helper"},
		"f":                   {"g"},
	}
	jsWant := map[string][]string{
		core.ModuleSymbolName: {"app.use", "path.join", "register", "require", "setup"},
		"K.m":                 {"helper"},
		"f":                   {"g"},
		"path":                {"require"}, // a module-level const's initializer (LB-F382)
	}
	cases := []struct {
		e    *TSExtractor
		path string
		src  string
		want map[string][]string
	}{
		{group, "src/app.module.ts", tsModuleSrc, tsWant},
		{group, "src/app.module.tsx", tsModuleSrc, tsWant},
		{group, "src/app.module.mts", tsModuleSrc, tsWant},
		{group, "src/app.module.cts", tsModuleSrc, tsWant},
		{tsOnly, "src/app.module.ts", tsModuleSrc, tsWant},
		{group, "lib/main.js", jsModuleSrc, jsWant},
		{group, "lib/main.jsx", jsModuleSrc, jsWant},
		{group, "lib/main.mjs", jsModuleSrc, jsWant},
		{group, "lib/main.cjs", jsModuleSrc, jsWant},
		{jsOnly, "lib/main.js", jsModuleSrc, jsWant},
	}
	for _, c := range cases {
		nodes, err := c.e.Extract("svc", c.path, []byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		m := moduleOf(t, nodes)
		lines := bytes.Count([]byte(c.src), []byte("\n"))
		base := filepath.Base(c.path)
		pkg := base[:len(base)-len(filepath.Ext(base))]
		if m.Kind != "JSModule" || m.Line != 1 || m.EndLine != lines || m.Package != pkg || m.Signature != "" {
			t.Errorf("%s: module symbol = %+v, want JSModule, lines 1..%d, package %s, unsigned", c.path, m, lines, pkg)
		}
		got := map[string][]string{}
		for _, n := range nodes {
			if len(n.Calls) > 0 {
				got[n.Name] = n.Calls
			}
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: calls by owner = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestModuleSymbolJSTSIsAnAddition (LB-F381): the typescript and javascript
// goldens change by additions only — every fixture extracts exactly as with
// the switch off, plus its one module symbol at line 1 — and a file with no
// code is a module too (scip-typescript defines one per document).
func TestModuleSymbolJSTSIsAnAddition(t *testing.T) {
	for _, id := range []string{"typescript", "javascript"} {
		on, err := NewTSExtractor(id)
		if err != nil {
			t.Fatal(err)
		}
		off := withoutModuleSymbol(t, id)
		for _, fx := range dialectFixtures(t, id) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			rel := "src/" + filepath.Base(fx)
			with, err := on.Extract("svc", rel, src)
			if err != nil {
				t.Fatal(err)
			}
			without, err := off.Extract("svc", rel, src)
			if err != nil {
				t.Fatal(err)
			}
			if m := moduleOf(t, with); m.Line != 1 {
				t.Errorf("%s: module symbol at line %d, want 1", fx, m.Line)
			}
			rest := slices.DeleteFunc(slices.Clone(with), func(n core.SymbolNode) bool { return n.IsModuleSymbol() })
			// The module takes the calls no definition owned: with the
			// switch off they were dropped, so the rest are unchanged.
			if !reflect.DeepEqual(rest, without) {
				t.Errorf("%s: the module symbol changed the other definitions\n--- with (less the module)\n%+v\n--- without\n%+v", fx, rest, without)
			}
		}
		empty, err := on.Extract("svc", "src/empty"+map[string]string{"typescript": ".ts", "javascript": ".js"}[id], nil)
		if err != nil {
			t.Fatal(err)
		}
		if m := moduleOf(t, empty); m.Line != 1 || m.EndLine != 1 || len(m.Calls) != 0 || len(empty) != 1 {
			t.Errorf("%s: empty file = %+v, want its module symbol alone at line 1", id, empty)
		}
		on.Close()
		off.Close()
	}
}

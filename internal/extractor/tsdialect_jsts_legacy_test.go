package extractor

// tsdialect_jsts_legacy_test.go — LB-Q-javascript-rest and LB-Q-typescript-rest.
// The javascript and typescript dialects left TestTagsDialectParity's legacy
// projection (see migratedTagsDialects) because their queries now emit fewer
// definitions than the pre-manifest engine did: a binding to an arrow function
// or function expression only at module scope (a direct child of the program
// or of an export statement), a method only in a class body, a prototype or
// member assignment only as a module-scope statement, and no object-literal
// member at all (each dialect's tags.scm header carries the measured reason).
// Their goldens are now the full ones TestDialectGoldens checks. These tests
// keep the guarantee that mattered: every symbol the pre-change goldens listed
// (name, kind, first line) is still emitted, except the enumerated dropped set,
// which must no longer be (a React kind, LB-F377, refines a legacy function or
// class: reactRefines). The tables are those goldens' symbol lists as they
// were before the change (written by the pre-manifest engine, PROVENANCE.md,
// plus TypeScript's owner-decision D2 additions); a symbol may be added to the
// fixtures' goldens, never removed from here.

import (
	"path/filepath"
	"testing"
)

type legacyJSTSSymbol struct {
	name, kind string
	line       int
}

var legacyJavaScriptSymbols = map[string][]legacyJSTSSymbol{
	"assign.js": {
		{"View", "JSFunc", 8},
		{"render", "JSMethod", 12},
		{"lookup", "JSMethod", 16},
		{"set", "JSFunc", 22},
		{"use", "JSFunc", 26},
		{"helper", "JSFunc", 28},
	},
	"broken.js": {
		{"good", "JSFunc", 1},
		{"ok", "JSMethod", 9},
	},
	"common.cjs": {
		{"helper", "JSFunc", 7},
	},
	"component.jsx": {
		{"Button", "JSFunc", 3},
		{"Panel", "JSClass", 8},
		{"Panel.render", "JSMethod", 9},
	},
	"module.mjs": {
		{"exported", "JSFunc", 1},
		{"internal", "JSFunc", 5},
		{"arrowExport", "JSFunc", 9},
	},
	"shapes.js": {
		{"Widget", "JSClass", 8},
		{"Widget.constructor", "JSMethod", 11},
		{"Widget.render", "JSMethod", 16},
		{"Widget.#hidden", "JSMethod", 20},
		{"Widget.[Symbol.iterator]", "JSMethod", 24},
		{"Widget.create", "JSMethod", 28},
		{"Widget.value", "JSMethod", 32},
		{"plain", "JSFunc", 37},
		{"gen", "JSFunc", 41},
		{"arrow", "JSFunc", 45},
		{"expr", "JSFunc", 46},
		{"asyncFn", "JSFunc", 63},
	},
}

// droppedJavaScriptSymbols are the pre-change golden symbols that are no
// longer definitions, each a local or an object-literal key scip-typescript
// 0.4.0 never reports as a definition (the residue behind sym_precision .8368).
var droppedJavaScriptSymbols = map[string][]legacyJSTSSymbol{
	// `const handle = () => onClick(label)` inside function Button: a
	// function-local arrow const; its call to onClick is now Button's.
	"component.jsx": {{"handle", "JSFunc", 4}},
	// `method() { return inner(); }` in the module-scope object literal `obj`:
	// an object-literal method.
	"shapes.js": {{"method", "JSMethod", 53}},
}

var legacyTypeScriptSymbols = map[string][]legacyJSTSSymbol{
	"broken.ts": {
		{"good", "JSFunc", 1},
		{"Broken", "TSInterface", 5},
	},
	"module.mts": {
		{"Id", "TSType", 1},
		{"load", "JSFunc", 3},
	},
	"shapes.ts": {
		{"Shape", "TSInterface", 5},
		{"Shape.area", "JSMethod", 6},
		{"Point", "TSType", 10},
		{"Handler", "TSType", 11},
		{"Color", "TSEnum", 13},
		{"Flag", "TSEnum", 18},
		{"Base", "JSClass", 20},
		{"Base.constructor", "JSMethod", 24},
		{"Base.area", "JSMethod", 28},
		{"Base.describe", "JSMethod", 30},
		{"Base.#secret", "JSMethod", 37},
		{"Circle", "JSClass", 43},
		{"Circle.area", "JSMethod", 44},
		{"create", "JSFunc", 49},
		{"arrow", "JSFunc", 53},
		{"inner", "JSFunc", 56},
		{"ambient", "JSFunc", 61},
	},
	"view.tsx": {
		{"Props", "TSInterface", 3},
		{"View", "JSFunc", 7},
		{"Frame", "JSClass", 13},
		{"Frame.render", "JSMethod", 14},
	},
}

var droppedTypeScriptSymbols = map[string][]legacyJSTSSymbol{
	// `const toggle = () => setOpen(!open)` inside function View: a
	// function-local arrow const; its call to setOpen is now View's.
	"view.tsx": {{"toggle", "JSFunc", 9}},
}

// reactRefines maps each React kind (LB-F377) to the kind the same
// definition had before it: a component or a hook is still a function, a
// class component still a class, at the same name and line (their canonical
// kinds are unchanged), so a legacy symbol refined to one is still emitted.
var reactRefines = map[string]string{"JSComponent": "JSFunc", "JSHook": "JSFunc", "JSClassComponent": "JSClass"}

func checkLegacySymbols(t *testing.T, dialect string, keep, dropped map[string][]legacyJSTSSymbol) {
	t.Helper()
	e, err := NewTSExtractor(dialect)
	if err != nil {
		t.Fatalf("NewTSExtractor(%s): %v", dialect, err)
	}
	defer e.Close()
	fixtures := map[string]bool{}
	for f := range keep {
		fixtures[f] = true
	}
	for f := range dropped {
		fixtures[f] = true
	}
	for fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			got := extractGolden(t, e, filepath.Join("dialects", dialect, "testdata", fixture))
			have := map[legacyJSTSSymbol]bool{}
			for _, s := range got.Symbols {
				have[legacyJSTSSymbol{s.Name, s.Kind, s.Line}] = true
				if k, ok := reactRefines[s.Kind]; ok {
					have[legacyJSTSSymbol{s.Name, k, s.Line}] = true
				}
			}
			for _, w := range keep[fixture] {
				if !have[w] {
					t.Errorf("%s: legacy symbol %s %s at line %d is no longer emitted", fixture, w.kind, w.name, w.line)
				}
			}
			for _, d := range dropped[fixture] {
				if have[d] {
					t.Errorf("%s: %s %s at line %d is a local or object-literal key and must not be a definition", fixture, d.kind, d.name, d.line)
				}
			}
		})
	}
}

func TestJavaScriptDialectKeepsLegacySymbols(t *testing.T) {
	checkLegacySymbols(t, "javascript", legacyJavaScriptSymbols, droppedJavaScriptSymbols)
}

func TestTypeScriptDialectKeepsLegacySymbols(t *testing.T) {
	checkLegacySymbols(t, "typescript", legacyTypeScriptSymbols, droppedTypeScriptSymbols)
}

// scopeCase is one scope fixture's contract: the definitions it must emit
// (name and kind), the names that must not be definitions, and calls that
// must be attributed to a named definition.
type scopeCase struct {
	emit    map[string]string   // name -> kind
	notDefs []string            // names that are locals or object-literal keys
	calls   map[string][]string // definition -> calls it must carry
}

func checkScope(t *testing.T, dialect, fixture string, c scopeCase) {
	t.Helper()
	e, err := NewTSExtractor(dialect)
	if err != nil {
		t.Fatalf("NewTSExtractor(%s): %v", dialect, err)
	}
	defer e.Close()
	got := extractGolden(t, e, filepath.Join("dialects", dialect, "testdata", fixture))
	kinds := map[string]string{}
	calls := map[string]map[string]bool{}
	for _, s := range got.Symbols {
		kinds[s.Name] = s.Kind
		calls[s.Name] = map[string]bool{}
		for _, c := range s.Calls {
			calls[s.Name][c] = true
		}
	}
	for name, kind := range c.emit {
		if kinds[name] != kind {
			t.Errorf("%s: %s should be a %s definition, got %q", fixture, name, kind, kinds[name])
		}
	}
	for _, name := range c.notDefs {
		if k, ok := kinds[name]; ok {
			t.Errorf("%s: %s is a local or object-literal key, not a definition (emitted as %s)", fixture, name, k)
		}
	}
	for def, want := range c.calls {
		for _, w := range want {
			if !calls[def][w] {
				t.Errorf("%s: call %s should belong to %s (its calls: %v)", fixture, w, def, calls[def])
			}
		}
	}
	if len(got.Symbols) != len(c.emit) {
		t.Errorf("%s: %d definitions, want exactly %d: %+v", fixture, len(got.Symbols), len(c.emit), got.Symbols)
	}
}

func TestJavaScriptDialectScope(t *testing.T) {
	checkScope(t, "javascript", "scope.js", scopeCase{
		emit: map[string]string{
			"topArrow": "JSFunc", "topExpr": "JSFunc", "topLet": "JSFunc",
			"exportedArrow": "JSFunc", "exportedFn": "JSFunc", "nestedDecl": "JSFunc",
			"Server": "JSFunc", "listen": "JSMethod", "create": "JSFunc", "head": "JSFunc",
			"Router": "JSClass", "Router.route": "JSMethod", "fixture": "JSFunc",
			"<module>": "JSModule", // K18 (LB-F381)
		},
		notDefs: []string{
			"localArrow", "localExpr", "named", "reassigned", "handler", "onload",
			"late", "match", "routes", "index", "show", "edit", "build", "helper",
			"setup", "check", "debugOnly", "hidden",
		},
		calls: map[string][]string{
			"exportedFn":   {"localCall", "otherLocal", "assigned", "localArrow", "localExpr", "nestedDecl"},
			"nestedDecl":   {"nestedCall"},
			"Server":       {"onThis", "loaded", "lateCall"},
			"Router.route": {"compare", "match"},
		},
	})
}

func TestTypeScriptDialectScope(t *testing.T) {
	checkScope(t, "typescript", "scope.ts", scopeCase{
		emit: map[string]string{
			"parse": "JSFunc", "internalArrow": "JSFunc", "legacy": "JSFunc",
			"handle": "JSFunc", "nested": "JSFunc",
			"Service": "JSClass", "Service.lookup": "JSMethod",
			"encode": "JSFunc", "decode": "JSFunc",
			"<module>": "JSModule", // K18 (LB-F381)
		},
		notDefs: []string{
			"toBody", "fallback", "miss", "hiddenHelper", "handlers", "create",
			"remove", "setup", "makeRequest", "check",
		},
		calls: map[string][]string{
			"handle":         {"serialize", "empty", "nested", "toBody", "fallback"},
			"Service.lookup": {"fetchRemote", "miss"},
			"decode":         {"hiddenHelper"},
		},
	})
}

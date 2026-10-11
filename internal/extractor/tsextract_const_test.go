package extractor

import (
	"reflect"
	"sort"
	"testing"
)

// tsConstSrc holds every shape of a module-level binding: a const bound to a
// value (a literal, a call, a member, an object, a class expression), a
// destructured const, an exported one (in a namespace too), a let and a var
// (constants since wave lf7: TestTSModuleLevelBindings), and the bindings that
// are something else or nothing: a function, a component, a local.
const tsConstSrc = `import { x } from "./x";
export const VERSION = "1.0";
const app = express();
export const router = Router();
const { a, b: renamed, ...rest } = require("./cfg");
const [first, second] = pair;
let mutable = 1;
var legacy = 2;
export const handler = () => 1;
const Dialog = Primitive.Root;
const Fancy = memo(() => null);
const Upper = Primitive.something;
const Made = createThing();
const config = { port: 80, start: boot() };
export const $ZodType: core.$constructor<$ZodType> = core.$constructor("$ZodType", (inst) => {
  inst.x = 1;
});
const Shape = class {};
function f() {
  const local = make();
  return local;
}
namespace NS {
  export const inner = 1;
}
`

const jsConstSrc = `const path = require("path");
const { SyncHook, AsyncSeriesHook: Series } = require("tapable");
const RuntimeGlobals = require("./RuntimeGlobals");
const DEFAULT = 5;
let mutable = 1;
var legacy = 2;
const handler = async () => 1;
const Item = memo(() => null);
function f() {
  const local = make();
  return local;
}
module.exports = { f, DEFAULT };
`

// TestModuleLevelConstants (LB-F382): a `const` binding at module scope, or
// exported (in a namespace too), whose value is no function and no React
// component is a definition of kind JSConst (canonical constant, outside every
// oracle's kind intersection but ctags', which tags a JavaScript const as a
// constant): scip-typescript defines each such binding at module scope, and
// def_mapping and typed_recall miss it otherwise (O746). Each name of a
// destructuring const is one. A function-valued const stays a function, a
// component a component; a let and a var are constants too (JavaScript's since
// wave lf5, TestModuleLevelBindings; TypeScript's since wave lf7,
// TestTSModuleLevelBindings) and a local is none in both. The calls its
// initializer makes are the constant's (scip-typescript's const definition has
// no enclosing range, so its mapped definition's lines stand in, D5), never the
// module's.
func TestModuleLevelConstants(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	cases := []struct {
		path, src string
		kinds     map[string]string
		calls     map[string][]string
	}{
		{"src/a.ts", tsConstSrc, map[string]string{
			"VERSION": "JSConst", "app": "JSConst", "router": "JSConst",
			"a": "JSConst", "renamed": "JSConst", "rest": "JSConst",
			"first": "JSConst", "second": "JSConst",
			"Upper": "JSConst", "Made": "JSConst", "config": "JSConst",
			"$ZodType": "JSConst", "Shape": "JSConst", "inner": "JSConst",
			"mutable": "JSConst", "legacy": "JSConst", // TypeScript's let and var too (S3, TestTSModuleLevelBindings)
			"handler": "JSFunc", "Dialog": "JSComponent", "Fancy": "JSComponent", "f": "JSFunc",
			"<module>": "JSModule",
		}, map[string][]string{
			"app": {"express"}, "router": {"Router"}, "Made": {"createThing"},
			"config": {"boot"}, "$ZodType": {"core.$constructor"},
			"Fancy": {"memo"}, "f": {"make"},
			"<module>": {"require"},
		}},
		{"lib/b.js", jsConstSrc, map[string]string{
			"path": "JSConst", "SyncHook": "JSConst", "Series": "JSConst",
			"RuntimeGlobals": "JSConst", "DEFAULT": "JSConst",
			"mutable": "JSConst", "legacy": "JSConst", // JavaScript's let and var too (O749, TestModuleLevelBindings)
			"handler": "JSFunc", "Item": "JSComponent", "f": "JSFunc",
			"<module>": "JSModule",
		}, map[string][]string{
			"path": {"require"}, "RuntimeGlobals": {"require"},
			"Item": {"memo"}, "f": {"make"},
			"<module>": {"require"},
		}},
	}
	for _, c := range cases {
		nodes, err := e.Extract("svc", c.path, []byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		kinds := map[string]string{}
		calls := map[string][]string{}
		for _, n := range nodes {
			if prev, dup := kinds[n.Name]; dup {
				t.Errorf("%s: %s defined twice (%s, %s)", c.path, n.Name, prev, n.Kind)
			}
			kinds[n.Name] = string(n.Kind)
			if len(n.Calls) > 0 {
				calls[n.Name] = n.Calls
			}
		}
		if !reflect.DeepEqual(kinds, c.kinds) {
			var names []string
			for n := range kinds {
				names = append(names, n)
			}
			sort.Strings(names)
			t.Errorf("%s: definitions = %v (%v), want %v", c.path, kinds, names, c.kinds)
		}
		if !reflect.DeepEqual(calls, c.calls) {
			t.Errorf("%s: calls = %q, want %q", c.path, calls, c.calls)
		}
	}
	if k, ok := CanonicalKind("JSConst"); !ok || k != "constant" {
		t.Errorf("CanonicalKind(JSConst) = %q, %v; want constant", k, ok)
	}
}

// TestModuleLevelBindings (wave lf5, item tsjs, O749): in JavaScript a `let` or
// `var` binding at module scope, or exported, each name a destructuring one
// binds, and a CommonJS export assigned a value (`exports.x = value`,
// `module.exports.x = value`) is a JSConst whose initializer's calls are its
// own, as LB-F382 names a const: scip-typescript defines each as a global term
// of the file (express's 468 and webpack's 294 module-level var and let
// bindings read as unmapped definitions). A binding inside a function, a block
// or a loop head is none; one bound to a function, or an export assigned a
// function or a call wrapping one, stays the function (one node, one name, the
// earlier pattern's); a member assigned on another object and
// `module.exports = value` are none. TypeScript's let and var are constants
// too since wave lf7 (owner ruling S3, TestTSModuleLevelBindings).
func TestModuleLevelBindings(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	const src = `var express = require("express");
var app = express();
let count = 0, total = sum();
var { join, resolve: res, ...more } = require("path");
let [first, second] = pair();
let pending;
export let live = start();
export var legacy = 1;
var handler = function () { return app.listen(count); };
exports.version = "1.0.0";
exports.router = express.Router();
module.exports.config = { port: open() };
exports.create = function create() {
  var local = make();
  let inner = 2;
  return local + inner;
};
exports.wrapped = guard(() => handler());
module.exports = { create };
app.settings = build();
if (app) {
  var hoisted = 1;
}
for (var i = 0; i < 2; i++) {
  count += i;
}
function f() {
  let scoped = make();
  var old = make();
  return scoped + old;
}
`
	nodes, err := e.Extract("svc", "lib/app.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	calls := map[string][]string{}
	for _, n := range nodes {
		if prev, dup := kinds[n.Name]; dup {
			t.Errorf("%s defined twice (%s, %s)", n.Name, prev, n.Kind)
		}
		kinds[n.Name] = string(n.Kind)
		if len(n.Calls) > 0 {
			calls[n.Name] = n.Calls
		}
	}
	want := map[string]string{
		"<module>": "JSModule",
		"express":  "JSConst", "app": "JSConst", "count": "JSConst", "total": "JSConst",
		"join": "JSConst", "res": "JSConst", "more": "JSConst", "first": "JSConst", "second": "JSConst",
		"pending": "JSConst", "live": "JSConst", "legacy": "JSConst",
		"version": "JSConst", "router": "JSConst", "config": "JSConst",
		"handler": "JSFunc", "create": "JSFunc", "wrapped": "JSFunc", "f": "JSFunc",
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("definitions = %v\nwant %v", kinds, want)
	}
	wantCalls := map[string][]string{
		"<module>": {"build", "pair", "require"},
		"express":  {"require"}, "app": {"express"}, "total": {"sum"}, "live": {"start"},
		"router": {"express.Router"}, "config": {"open"},
		"handler": {"app.listen"}, "create": {"make"}, "wrapped": {"guard", "handler"}, "f": {"make"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %q\nwant %q", calls, wantCalls)
	}

	ts, err := NewTSExtractor("typescript")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	tsNodes, err := ts.Extract("svc", "src/a.ts", []byte("let mutable = 1;\nvar legacy = 2;\nexport const kept = 3;\n"))
	if err != nil {
		t.Fatal(err)
	}
	var tsNames []string
	for _, n := range tsNodes {
		tsNames = append(tsNames, n.Name)
	}
	sort.Strings(tsNames)
	if !reflect.DeepEqual(tsNames, []string{"<module>", "kept", "legacy", "mutable"}) {
		t.Errorf("typescript definitions = %v, want <module>, kept, legacy and mutable (its let and var are constants since S3)", tsNames)
	}
}

// TestTSModuleLevelBindings (wave lf7, item tsx, owner ruling S3, O901,
// LB-F486): TypeScript's module-level `let` and `var` bindings are definitions,
// as owner ruling M1 made JavaScript's (LB-F470): a `let` or `var` at module
// scope, or exported (in a namespace too), in a .ts or .tsx file, and each name
// a destructuring one binds, is a JSConst whose initializer's calls are its own,
// as LB-F382 names a const: scip-typescript defines each as a global term of the
// file (headlessui's 220 `export let Button = forwardRefWithAs(ButtonFn) as ...`
// bindings read as unmapped definitions, the targets of 8,010 references). One
// bound to a function, a hook or a component stays that definition (one node,
// one name, the earlier pattern's: oneDefPerNode); a binding inside a function,
// a block or a loop head is none, and so is an ambient `declare let`, as an
// ambient const is.
func TestTSModuleLevelBindings(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	cases := []struct {
		path, src string
		kinds     map[string]string
		calls     map[string][]string
	}{
		{"src/a.ts", `import { x } from "./x";
let count = 0, total = sum();
var { join, resolve: res, ...more } = require("path");
let [first, second] = pair();
let pending: number | undefined;
let typed: Map<string, number> = new Map();
export let live = start();
export var legacy = 1;
var handler = function () { return run(count); };
let arrow = (n: number) => n + 1;
let useThing = () => useState(0);
const kept = 3;
if (count) {
  var hoisted = 1;
}
for (var i = 0; i < 2; i++) {
  count += i;
}
for (let j of [1]) {
  count += j;
}
declare let ambient: number;
function f() {
  let scoped = make();
  var old = make();
  return scoped + old;
}
namespace NS {
  export let inner = make();
  let hidden = 1;
}
`, map[string]string{
			"<module>": "JSModule",
			"count":    "JSConst", "total": "JSConst", "join": "JSConst", "res": "JSConst", "more": "JSConst",
			"first": "JSConst", "second": "JSConst", "pending": "JSConst", "typed": "JSConst",
			"live": "JSConst", "legacy": "JSConst", "kept": "JSConst", "inner": "JSConst",
			"handler": "JSFunc", "arrow": "JSFunc", "useThing": "JSHook", "f": "JSFunc",
		}, map[string][]string{
			"<module>": {"pair", "require"},
			"total":    {"sum"}, "typed": {"Map"}, "live": {"start"}, "inner": {"make"},
			"handler": {"run"}, "useThing": {"useState"}, "f": {"make"},
		}},
		{"src/Button.tsx", `import { forwardRefWithAs } from "./render";
function ButtonFn(props: { disabled?: boolean }) {
  return <button disabled={props.disabled} />;
}
export let Button = forwardRefWithAs(ButtonFn) as typeof ButtonFn & { displayName: string };
let DEFAULT_BUTTON_TAG = "button" as const;
let Fancy = memo(() => null);
let Card = () => <div />;
export let Panel = function () { return <section />; };
let Alias = Primitive.Root;
let { Provider, Consumer } = createContext(null);
`, map[string]string{
			"<module>": "JSModule",
			"ButtonFn": "JSComponent", "Button": "JSConst", "DEFAULT_BUTTON_TAG": "JSConst",
			"Fancy": "JSComponent", "Card": "JSComponent", "Panel": "JSComponent", "Alias": "JSComponent",
			"Provider": "JSConst", "Consumer": "JSConst",
		}, map[string][]string{
			"<module>": {"createContext"},
			"Button":   {"forwardRefWithAs"}, "Fancy": {"memo"},
		}},
	}
	for _, c := range cases {
		nodes, err := e.Extract("svc", c.path, []byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		kinds := map[string]string{}
		calls := map[string][]string{}
		for _, n := range nodes {
			if prev, dup := kinds[n.Name]; dup {
				t.Errorf("%s: %s defined twice (%s, %s)", c.path, n.Name, prev, n.Kind)
			}
			kinds[n.Name] = string(n.Kind)
			if len(n.Calls) > 0 {
				calls[n.Name] = n.Calls
			}
		}
		if !reflect.DeepEqual(kinds, c.kinds) {
			t.Errorf("%s: definitions = %v\nwant %v", c.path, kinds, c.kinds)
		}
		if !reflect.DeepEqual(calls, c.calls) {
			t.Errorf("%s: calls = %q\nwant %q", c.path, calls, c.calls)
		}
	}
}

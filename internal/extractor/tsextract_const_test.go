package extractor

import (
	"reflect"
	"sort"
	"testing"
)

// tsConstSrc holds every shape of a module-level binding: a const bound to a
// value (a literal, a call, a member, an object, a class expression), a
// destructured const, an exported one (in a namespace too), and the bindings
// that are something else or nothing: a function, a component, a let, a var,
// a local.
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
// component a component; a let, a var and a local are none. The calls its
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

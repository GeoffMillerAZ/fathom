package extractor

import (
	"reflect"
	"testing"
)

// TestCommonJSExportedFunctions (O721, LB-F384): an export bound by CommonJS
// member assignment is the export's function when its value is a function or
// a call that wraps one — `module.exports.x = async () => {}`, `= async
// function () {}`, `exports.x = wrap(async (e) => …)`, `module.exports.x =
// middy(…)` — named by the export and owning the calls its value makes, so a
// Lambda handler's template binds it (fusion.ResolveLambdaHandlers). An
// exported value that is no function is the export's JSConst (wave lf5, O749:
// TestModuleLevelBindings); a whole-module assignment and a wrapper assigned to
// anything but the module's exports are none. TypeScript's
// `export const handler = withX(…)` is a module-level constant (LB-F382),
// which the binder takes as well.
func TestCommonJSExportedFunctions(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	js := `module.exports.handler = async (event) => {
  return process(event);
};
module.exports.lambdaHandler = async function (event) {
  return run(event);
};
exports.wrapped = metricScope((m) => async (e) => {
  return go(e);
});
module.exports.middy = middy(async (event) => ok(event));
exports.plain = function plain() {
  return 1;
};
exports.config = { a: 1 };
module.exports = { other };
app.cached = memoize(() => 1);
`
	nodes, err := e.Extract("svc", "src/app.js", []byte(js))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	calls := map[string][]string{}
	for _, n := range nodes {
		kinds[n.Name] = string(n.Kind)
		if len(n.Calls) > 0 {
			calls[n.Name] = n.Calls
		}
	}
	wantKinds := map[string]string{
		"<module>": "JSModule", "handler": "JSFunc", "lambdaHandler": "JSFunc",
		"wrapped": "JSFunc", "middy": "JSFunc", "plain": "JSFunc", "config": "JSConst",
	}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("definitions = %v, want %v", kinds, wantKinds)
	}
	wantCalls := map[string][]string{
		"handler": {"process"}, "lambdaHandler": {"run"}, "wrapped": {"go", "metricScope"},
		"middy": {"middy", "ok"}, "<module>": {"memoize"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %q, want %q", calls, wantCalls)
	}

	ts := "export const handler = withDurableExecution(async (event: unknown) => {\n  return step(event);\n});\n"
	nodes, err = e.Extract("svc", "src/app.ts", []byte(ts))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, n.Name+":"+string(n.Kind))
	}
	if want := []string{"<module>:JSModule", "handler:JSConst"}; !reflect.DeepEqual(got, want) {
		t.Errorf("TypeScript wrapped export = %v, want %v", got, want)
	}
}

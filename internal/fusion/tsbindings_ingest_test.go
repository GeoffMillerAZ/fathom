package fusion

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_TSModuleBindings (wave lf7, item tsx, owner ruling S3, O901,
// LB-F486; the product reach of TestTSModuleLevelBindings): `fathom ingest`
// stores a TypeScript file's module-level let and var bindings, in a .ts and a
// .tsx file alike, as JSConst, their initializer's calls their own, and none of
// a function's locals; one bound to a function or a component stays that
// definition.
func TestIngest_TSModuleBindings(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"src/state.ts": "import { make } from \"./make\";\n" +
			"let count = 0;\n" +
			"var registry = make();\n" +
			"export let current: string | null = null;\n" +
			"let { a, b: renamed } = make();\n" +
			"let bump = () => count++;\n" +
			"function f() {\n  let scoped = make();\n  var old = 1;\n  return scoped + old;\n}\n",
		"src/Button.tsx": "import { forwardRefWithAs } from \"./render\";\n" +
			"let DEFAULT_TAG = \"button\" as const;\n" +
			"function ButtonFn(props: { disabled?: boolean }) {\n  return <button disabled={props.disabled} />;\n}\n" +
			"export let Button = forwardRefWithAs(ButtonFn) as typeof ButtonFn;\n" +
			"let Card = () => <div />;\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "tsb", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "tsb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]map[string]string{}
	calls := map[string][]string{}
	for _, o := range obs {
		if o.SourceRef != "src/state.ts" && o.SourceRef != "src/Button.tsx" {
			continue
		}
		if got[o.SourceRef] == nil {
			got[o.SourceRef] = map[string]string{}
		}
		got[o.SourceRef][o.Symbol] = string(o.SourceKind)
		if len(o.Calls) > 0 {
			calls[o.SourceRef+":"+o.Symbol] = o.Calls
		}
	}
	want := map[string]map[string]string{
		"src/state.ts": {
			"<module>": "JSModule", "count": "JSConst", "registry": "JSConst", "current": "JSConst",
			"a": "JSConst", "renamed": "JSConst", "bump": "JSFunc", "f": "JSFunc",
		},
		"src/Button.tsx": {
			"<module>": "JSModule", "DEFAULT_TAG": "JSConst", "ButtonFn": "JSComponent",
			"Button": "JSConst", "Card": "JSComponent",
		},
	}
	if !reflect.DeepEqual(got, want) {
		for f, m := range got {
			var names []string
			for n := range m {
				names = append(names, n)
			}
			sort.Strings(names)
			t.Logf("%s: %v", f, names)
		}
		t.Errorf("stored definitions = %v\nwant %v", got, want)
	}
	if c := calls["src/state.ts:registry"]; len(c) != 1 || c[0] != "make" {
		t.Errorf("registry's calls = %q, want its initializer's [make]", c)
	}
	if c := calls["src/Button.tsx:Button"]; len(c) != 1 || c[0] != "forwardRefWithAs" {
		t.Errorf("Button's calls = %q, want its initializer's [forwardRefWithAs]", c)
	}
}

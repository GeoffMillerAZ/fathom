package extractor

// reactdialect_test.go — LB-F377 (wave c3, item react): React's kinds in the
// typescript and javascript dialects. A capitalized function or module-scope
// arrow/function binding whose body writes JSX is a component (JSComponent),
// as is a capitalized binding to memo, forwardRef, lazy, a styled template or
// a member alias (Primitive.Root); a class extending Component or
// PureComponent is a class component (JSClassComponent); a use[A-Z0-9]
// function that calls a hook is a hook (JSHook); a member of an interface or
// type alias named *Props is a prop (JSProp, Owner.prop); a JSX element whose
// tag is capitalized or a member expression is a call from its enclosing
// definition, and an intrinsic one (<div>) is none. Everything else keeps its
// kind (a capitalized function writing no JSX, a use* function calling no
// hook, a class extending anything else, a non-Props interface's members).

import (
	"path/filepath"
	"slices"
	"sort"
	"testing"
)

type reactCase struct {
	kinds    map[string]string   // every definition: name -> kind
	calls    map[string][]string // definition -> calls it must carry
	notCalls map[string][]string // definition -> calls it must not carry
}

func checkReact(t *testing.T, dialect, fixture string, c reactCase) {
	t.Helper()
	e, err := NewTSExtractor(dialect)
	if err != nil {
		t.Fatalf("NewTSExtractor(%s): %v", dialect, err)
	}
	defer e.Close()
	got := extractGolden(t, e, filepath.Join("dialects", dialect, "testdata", fixture))
	have := map[string]string{}
	calls := map[string][]string{}
	for _, s := range got.Symbols {
		if k, dup := have[s.Name]; dup {
			t.Errorf("%s: %s emitted twice (%s and %s)", fixture, s.Name, k, s.Kind)
		}
		have[s.Name] = s.Kind
		calls[s.Name] = s.Calls
	}
	var names []string
	for n := range c.kinds {
		names = append(names, n)
	}
	for n := range have {
		if _, ok := c.kinds[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if want, got := c.kinds[n], have[n]; want != got {
			t.Errorf("%s: %s: kind %q, want %q", fixture, n, got, want)
		}
	}
	for def, want := range c.calls {
		for _, w := range want {
			if !slices.Contains(calls[def], w) {
				t.Errorf("%s: %s should call %s (its calls: %v)", fixture, def, w, calls[def])
			}
		}
	}
	for def, never := range c.notCalls {
		for _, w := range never {
			if slices.Contains(calls[def], w) {
				t.Errorf("%s: %s must not call %s (its calls: %v)", fixture, def, w, calls[def])
			}
		}
	}
}

func TestTypeScriptDialectReactKinds(t *testing.T) {
	checkReact(t, "typescript", "react.tsx", reactCase{
		kinds: map[string]string{
			"ButtonProps": "TSInterface", "ButtonProps.label": "JSProp", "ButtonProps.onClick": "JSProp",
			"CardProps": "TSType", "CardProps.title": "JSProp", "CardProps.body": "JSProp",
			"LinkProps": "TSType", "LinkProps.href": "JSProp", "LinkProps.external": "JSProp",
			"Theme":  "TSInterface",
			"Button": "JSComponent", "Card": "JSComponent", "Row": "JSComponent", "Link": "JSComponent",
			"Fancy": "JSComponent", "Wrapper": "JSComponent", "Big": "JSComponent", "Img": "JSComponent",
			"Lazy":   "JSComponent",
			"Dialog": "JSComponent",
			"Frame":  "JSClassComponent", "Frame.render": "JSMethod",
			"Pure": "JSClassComponent", "Pure.render": "JSMethod",
			"Store": "JSClass", "Store.get": "JSMethod",
			"Format":    "JSFunc",
			"useToggle": "JSHook", "useTheme": "JSHook",
			"useless": "JSFunc", "userName": "JSFunc",
			"GET":      "JSFunc",   // CONSTANT_CASE writing JSX: an HTTP verb, no component
			"<module>": "JSModule", // K18 (LB-F381): the file's module symbol
			// module-level consts (LB-F382): a context, a CONSTANT_CASE alias
			"ThemeContext": "JSConst", "DROP_HEAD": "JSConst",
		},
		calls: map[string][]string{
			"Button":       {"Icon.Glyph"},
			"Card":         {"Button"},
			"Row":          {"memo"},
			"Link":         {"forwardRef"},
			"Frame.render": {"Card"},
			"useToggle":    {"useState"},
		},
		notCalls: map[string][]string{
			"Button": {"button"},
			"Card":   {"section", "p"},
			"Row":    {"div"},
		},
	})
}

// TestTypeScriptDialectGenericsAreNoJSX: in a .ts file a type argument, a type
// parameter and a type assertion (`<T>x`) are never JSX, so no call comes of
// them; a member of a *Props type alias is still a prop.
func TestTypeScriptDialectGenericsAreNoJSX(t *testing.T) {
	checkReact(t, "typescript", "generics.ts", reactCase{
		kinds: map[string]string{
			"Box": "JSClass", "Box.constructor": "JSMethod", "Box.map": "JSMethod",
			"first": "JSFunc", "identity": "JSFunc",
			"Settings":     "TSInterface",
			"OptionsProps": "TSType", "OptionsProps.verbose": "JSProp",
			"<module>": "JSModule",
		},
		calls: map[string][]string{
			"first":   {"identity"},
			"Box.map": {"Box", "f"},
		},
		notCalls: map[string][]string{
			"first":   {"Record", "T", "unknown", "Array"},
			"Box.map": {"U"},
		},
	})
}

func TestJavaScriptDialectReactKinds(t *testing.T) {
	checkReact(t, "javascript", "react.jsx", reactCase{
		kinds: map[string]string{
			"Header": "JSComponent", "Page": "JSComponent", "Item": "JSComponent", "Input": "JSComponent",
			"Title": "JSComponent",
			"Clock": "JSClassComponent", "Clock.render": "JSMethod",
			"useInterval": "JSHook",
			"<module>":    "JSModule",
			"Parse":       "JSFunc",
		},
		calls: map[string][]string{
			"Header":       {"Icon.Glyph"},
			"Page":         {"Header"},
			"Item":         {"memo"},
			"Clock.render": {"Item"},
			"useInterval":  {"useEffect", "setInterval", "clearInterval"},
		},
		notCalls: map[string][]string{
			"Header": {"h1"},
			"Page":   {"main"},
			"Item":   {"li"},
			"Input":  {"input"},
		},
	})
}

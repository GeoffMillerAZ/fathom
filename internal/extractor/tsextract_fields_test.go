package extractor

import (
	"reflect"
	"testing"
)

// TestMemberFields (LB-F383): a class field, an interface's or an object type
// alias's property, an enum member and a constructor's parameter property
// are definitions, JSField (canonical field, outside the scip oracle's kind
// intersection), named Owner.name by the member pass: scip-typescript defines
// each (`Owner#name.`), and typed_recall misses every reference to one
// otherwise (O746: 15,743 of TypeScript's unmapped references). A *Props
// interface's property stays a prop (its pattern comes first), a method
// signature stays a method, and a property of an object type written
// anywhere else (a parameter's annotation) is none. A field's initializer's
// calls are the field's.
func TestMemberFields(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ts := `export class Registry {
  private _map: Map<string, number> = new Map();
  static readonly MAX = limit();
  #secret = 1;
  constructor(private readonly users: UsersService, public name: string, plain: number) {}
  run(): void {}
}
export interface Internals {
  output: string;
  input?: number;
  parse(x: unknown): void;
}
export type Def = {
  kind: string;
  check(): boolean;
};
export interface ButtonProps {
  label: string;
}
export enum Color {
  Red,
  Green = "green",
}
export function take(opts: { inline: string }): void {}
`
	nodes, err := e.Extract("svc", "src/a.ts", []byte(ts))
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
		"Registry": "JSClass", "Registry._map": "JSField", "Registry.MAX": "JSField", "Registry.#secret": "JSField",
		"Registry.users": "JSField", "Registry.name": "JSField",
		"Registry.constructor": "JSMethod", "Registry.run": "JSMethod",
		"Internals": "TSInterface", "Internals.output": "JSField", "Internals.input": "JSField", "Internals.parse": "JSMethod",
		"Def": "TSType", "Def.kind": "JSField", "Def.check": "JSMethod",
		"ButtonProps": "TSInterface", "ButtonProps.label": "JSProp",
		"Color": "TSEnum", "Color.Red": "JSField", "Color.Green": "JSField",
		"take": "JSFunc",
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("definitions = %v\nwant %v", kinds, want)
	}
	if want := map[string][]string{"Registry._map": {"Map"}, "Registry.MAX": {"limit"}}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls = %q, want %q", calls, want)
	}

	js := "class K {\n  count = start();\n  static #n = 0;\n  run() {\n    return 1;\n  }\n}\n"
	nodes, err = e.Extract("svc", "lib/k.js", []byte(js))
	if err != nil {
		t.Fatal(err)
	}
	kinds = map[string]string{}
	for _, n := range nodes {
		kinds[n.Name] = string(n.Kind)
	}
	if want := map[string]string{"<module>": "JSModule", "K": "JSClass", "K.count": "JSField", "K.#n": "JSField", "K.run": "JSMethod"}; !reflect.DeepEqual(kinds, want) {
		t.Errorf("JavaScript definitions = %v, want %v", kinds, want)
	}
	if k, ok := CanonicalKind("JSField"); !ok || k != "field" {
		t.Errorf("CanonicalKind(JSField) = %q, %v; want field", k, ok)
	}
}

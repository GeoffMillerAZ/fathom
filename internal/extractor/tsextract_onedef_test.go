package extractor

import (
	"reflect"
	"testing"
)

// TestOneDefPerNode (LB-F382): two patterns naming one node by one name make
// one definition, the earlier pattern's (a const bound to an arrow function is
// the function its pattern names, never also a constant); one node named by
// two names (a grouped CSS selector) stays two definitions, two nodes stay
// two, and two of one node, name and kind are left to the member pass.
func TestOneDefPerNode(t *testing.T) {
	defs := []*tsDef{
		{name: "f", kind: "JSConst", start: 0, end: 20, pattern: 40},
		{name: "f", kind: "JSFunc", start: 0, end: 20, pattern: 7},
		{name: ".a", kind: "CSSSelector", start: 30, end: 60, pattern: 2},
		{name: ".b", kind: "CSSSelector", start: 30, end: 60, pattern: 2},
		{name: "g", kind: "JSConst", start: 70, end: 80, pattern: 40},
		{name: "g", kind: "JSConst", start: 90, end: 99, pattern: 40},
		{name: "size", kind: "PythonVar", start: 100, end: 120, pattern: 3},
		{name: "size", kind: "PythonVar", start: 100, end: 120, pattern: 9},
	}
	got := oneDefPerNode(defs)
	var kinds []string
	for _, d := range got {
		kinds = append(kinds, d.name+":"+string(d.kind))
	}
	want := []string{"f:JSFunc", ".a:CSSSelector", ".b:CSSSelector", "g:JSConst", "g:JSConst", "size:PythonVar", "size:PythonVar"}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("oneDefPerNode = %v, want %v", kinds, want)
	}
}

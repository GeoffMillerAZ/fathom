package core

import (
	"reflect"
	"testing"
)

func TestWithoutShadowingFields(t *testing.T) {
	fn := SymbolNode{Service: "s", Package: "capability", Name: "Default", Kind: KindGoFunc, FilePath: "internal/capability/registry.go"}
	field := SymbolNode{Service: "s", Package: "Capability", Name: "Capability.Default", Kind: KindGoField, FilePath: "internal/capability/capability.go"}
	method := SymbolNode{Service: "s", Package: "Recorder", Name: "Recorder.Record", Kind: KindGoMethod, FilePath: "internal/decide/record.go"}
	recField := SymbolNode{Service: "s", Package: "DecidedBy", Name: "DecidedBy.Record", Kind: KindGoField, FilePath: "internal/decide/decidedby.go"}
	typ := SymbolNode{Service: "s", Package: "conserve", Name: "Residue", Kind: KindGoType, FilePath: "internal/conserve/types.go"}
	resField := SymbolNode{Service: "s", Package: "Report", Name: "Report.Residue", Kind: KindGoField, FilePath: "internal/conserve/conserve.go"}
	cases := []struct {
		name  string
		nodes []SymbolNode
		query string
		want  []SymbolNode
	}{
		{"func beside a field", []SymbolNode{field, fn}, "Default", []SymbolNode{fn}},
		{"method beside a field", []SymbolNode{recField, method}, "Record", []SymbolNode{method}},
		{"type beside a field", []SymbolNode{resField, typ}, "Residue", []SymbolNode{typ}},
		{"field only stays", []SymbolNode{field}, "Default", []SymbolNode{field}},
		{"owner-qualified name keeps the field", []SymbolNode{field, fn}, "Capability.Default", []SymbolNode{field, fn}},
		{"key keeps the field", []SymbolNode{field, fn}, field.Key(), []SymbolNode{field, fn}},
		{"no fields unchanged", []SymbolNode{fn, typ}, "Default", []SymbolNode{fn, typ}},
		{"empty", nil, "Default", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WithoutShadowingFields(tc.nodes, tc.query); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("WithoutShadowingFields(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

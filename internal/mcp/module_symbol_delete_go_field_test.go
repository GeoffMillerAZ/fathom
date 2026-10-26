package mcp_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Like the module-symbol regression beside this test, an empty caller set
// cannot prove a field dead. The real-ingest fixture reads T.n in Len, Take
// and Build, but the field-only member pass indexes no reference edges.
func TestBlastRadiusDeleteGoFieldWithReadersIsNotProvenZero(t *testing.T) {
	srv := typeRefServer(t)
	key := (core.SymbolNode{Service: "tfix", Package: "T", Name: "T.n", FilePath: "p/t.go"}).Key()
	found := callTool(t, srv, "find_symbol", map[string]any{"service": "tfix", "name": key})
	fields, _ := found["symbols"].([]any)
	if len(fields) != 1 || fields[0].(map[string]any)["kind"] != string(core.KindGoField) {
		t.Fatalf("fixture field = %v, want one indexed GoField", found)
	}
	for _, symbol := range []string{"T.n", "p/T.T.n", key} {
		t.Run(symbol, func(t *testing.T) {
			resolved := callTool(t, srv, "find_symbol", map[string]any{"service": "tfix", "name": symbol})
			fields, _ := resolved["symbols"].([]any)
			if len(fields) != 1 || fields[0].(map[string]any)["kind"] != string(core.KindGoField) {
				t.Fatalf("find_symbol(%q) = %v, want the fixture field", symbol, resolved)
			}
			got := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": "tfix", "symbol": symbol})
			if got["checked_edges"] != "typed" {
				t.Fatalf("fixture did not exercise typed edges: %v", got)
			}
			rows, ok := got["referents"].([]any)
			if !ok || len(rows) != 0 || got["referent_count"] != float64(0) {
				t.Fatalf("field-only fixture referents = %v, want an explicit empty recorded set", got)
			}
			if got["field_references_indexed"] != false {
				t.Errorf("field reference coverage = %v, want false", got["field_references_indexed"])
			}
			summary, _ := got["summary"].(string)
			if strings.Contains(summary, "proven zero") || !strings.Contains(summary, "field references are not indexed") {
				t.Errorf("summary = %q, want field-reference uncertainty, never proven zero", summary)
			}
		})
	}
}

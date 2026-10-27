package mcp_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func fieldChangeBackend() (*graphBackend, []string) {
	n := func(name, file string, kind core.SourceKind) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "p", Name: name, FilePath: file, Kind: kind}
	}
	target := n("Target", "changed.go", core.KindGoFunc)
	caller := n("Caller", "caller.go", core.KindGoFunc)
	fieldA := n("T.A", "changed.go", core.KindGoField)
	fieldB := n("T.B", "changed.go", core.KindGoField)
	fieldC := n("T.C", "changed.go", core.KindGoField)
	fieldCaller := n("FieldCaller", "field_caller.go", core.KindGoFunc)
	g := &graphBackend{
		fakeBackend: &fakeBackend{},
		nodes: []core.SymbolNode{
			fieldC, caller, fieldB, target, fieldA, fieldCaller, fieldB,
		},
		typed: []core.Edge{
			{FromService: "svc", FromSymbol: caller.Key(), ToSymbol: target.Key(), Kind: core.EdgeDirect},
			// Adversarial edge: a field's referents are not indexed, so this
			// must never make FieldCaller part of the reported radius.
			{FromService: "svc", FromSymbol: fieldCaller.Key(), ToSymbol: fieldA.Key(), Kind: core.EdgeDirect},
		},
	}
	return g, []string{fieldA.Key(), fieldB.Key(), fieldC.Key()}
}

func TestBlastRadiusChangeSeparatesFieldsFromTraversableRadius(t *testing.T) {
	backend, wantFields := fieldChangeBackend()
	srv := mcp.NewServer(backend)
	got := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"changed.go"}, "max_symbols": 2,
	})
	if got["changed_field_count"] != float64(3) {
		t.Errorf("changed_field_count = %v, want 3 unique fields", got["changed_field_count"])
	}
	rows, ok := got["changed_fields"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("changed_fields = %v, want three rows", got["changed_fields"])
	}
	keys := make([]string, 0, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		keys = append(keys, row["key"].(string))
		if row["depth"] != float64(0) || row["changed"] != true || row["note"] != "field references not indexed" {
			t.Errorf("changed field row = %v", row)
		}
	}
	if !reflect.DeepEqual(keys, wantFields) {
		t.Errorf("changed fields = %v, want sorted/deduplicated %v", keys, wantFields)
	}
	if got["total_symbols"] != float64(2) || got["truncated"] != false {
		t.Errorf("traversable radius bounds = %v/%v, want 2/false", got["total_symbols"], got["truncated"])
	}
	symbols := got["symbols"].([]any)
	if !reflect.DeepEqual(symbols, []any{"svc::p.Caller", "svc::p.Target"}) {
		t.Errorf("symbols = %v, want Target and Caller only", symbols)
	}
	buckets := got["by_depth"].([]any)
	if len(buckets) != 2 || buckets[0].(map[string]any)["count"] != float64(1) || buckets[1].(map[string]any)["count"] != float64(1) {
		t.Errorf("by_depth = %v, want one traversable symbol at each depth", buckets)
	}
	if summary, _ := got["summary"].(string); !strings.Contains(summary, "3 changed fields") || !strings.Contains(summary, "field references not indexed") {
		t.Errorf("summary = %q, want explicit field disclosure", summary)
	}

	// More changed fields than the cap must not consume the cap. The cap
	// applies to the same traversable set represented by by_depth.
	capped := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"changed.go"}, "max_symbols": 1,
	})
	cappedSymbols, symbolsOK := capped["symbols"].([]any)
	cappedFields, fieldsOK := capped["changed_fields"].([]any)
	if capped["total_symbols"] != float64(2) || capped["truncated"] != true || !symbolsOK || len(cappedSymbols) != 1 || !fieldsOK || len(cappedFields) != 3 {
		t.Errorf("capped response = %v", capped)
	}
}

func TestBlastRadiusChangeFieldsOnly(t *testing.T) {
	backend, wantFields := fieldChangeBackend()
	backend.nodes = []core.SymbolNode{backend.nodes[0], backend.nodes[2], backend.nodes[4], backend.nodes[6]}
	backend.typed = nil
	srv := mcp.NewServer(backend)
	got := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"changed.go"}, "max_symbols": 1,
	})
	for _, key := range []string{"symbols", "tests", "files", "by_depth"} {
		if rows, ok := got[key].([]any); !ok || len(rows) != 0 {
			t.Errorf("%s = %v, want empty traversable radius", key, got[key])
		}
	}
	if got["total_symbols"] != float64(0) || got["truncated"] != false || got["changed_field_count"] != float64(3) {
		t.Errorf("fields-only counts = total %v, truncated %v, changed fields %v", got["total_symbols"], got["truncated"], got["changed_field_count"])
	}
	rows, ok := got["changed_fields"].([]any)
	if !ok {
		t.Fatalf("changed_fields = %v, want rows", got["changed_fields"])
	}
	var keys []string
	for _, raw := range rows {
		keys = append(keys, raw.(map[string]any)["key"].(string))
	}
	if !reflect.DeepEqual(keys, wantFields) {
		t.Errorf("changed fields = %v, want %v", keys, wantFields)
	}
}

func TestBlastRadiusChangeWithoutFieldsOmitsDisclosure(t *testing.T) {
	got := callTool(t, mcp.NewServer(newGraphBackend()), "blast_radius", map[string]any{
		"service": "svc", "changed_files": []string{"a.go"},
	})
	if _, ok := got["changed_fields"]; ok {
		t.Errorf("changed_fields on field-free result: %v", got["changed_fields"])
	}
	if _, ok := got["changed_field_count"]; ok {
		t.Errorf("changed_field_count on field-free result: %v", got["changed_field_count"])
	}
}

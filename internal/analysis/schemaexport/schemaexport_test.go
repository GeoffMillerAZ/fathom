package schemaexport_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/schemaexport"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── fixtures ─────────────────────────────────────────────────────────────────

func nodes() []core.SymbolNode {
	return []core.SymbolNode{
		{
			Service: "svc", Package: "pkg", Name: "Bravo",
			Kind:  core.KindGoFunc,
			Calls: []string{"svc::pkg.Charlie", "svc::pkg.Alpha"},
		},
		{
			Service: "svc", Package: "pkg", Name: "Alpha",
			Kind:  core.KindGoFunc,
			Calls: []string{"svc::pkg.Bravo"},
		},
		{
			Service: "svc", Package: "pkg", Name: "Charlie",
			Kind:  core.KindGoFunc,
			Calls: nil,
		},
	}
}

func edges() []core.Edge {
	return []core.Edge{
		{FromService: "svc", FromFile: "a.go", FromSymbol: "svc::pkg.Bravo", ToSymbol: "svc::pkg.Charlie"},
		{FromService: "svc", FromFile: "a.go", FromSymbol: "svc::pkg.Alpha", ToSymbol: "svc::pkg.Bravo"},
		{FromService: "svc", FromFile: "a.go", FromSymbol: "svc::pkg.Bravo", ToSymbol: "svc::pkg.Alpha"},
	}
}

// ─── ExportJSON ───────────────────────────────────────────────────────────────

func TestExportJSON_ValidJSON(t *testing.T) {
	out, err := schemaexport.ExportJSON(nodes())
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("ExportJSON output is not valid JSON: %v\n---\n%s", err, out)
	}
}

func TestExportJSON_NodesSorted(t *testing.T) {
	out, err := schemaexport.ExportJSON(nodes())
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}
	var parsed []map[string]interface{}
	json.Unmarshal([]byte(out), &parsed) //nolint:errcheck

	// Collect Name values in output order.
	names := make([]string, len(parsed))
	for i, n := range parsed {
		names[i] = n["Name"].(string)
	}
	// Expected order: Alpha, Bravo, Charlie (sorted by SymbolKey).
	want := []string{"Alpha", "Bravo", "Charlie"}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("ExportJSON node order: got %v, want %v", names, want)
			break
		}
	}
}

func TestExportJSON_Determinism(t *testing.T) {
	// Input in different order.
	ns := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Zeta", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "Alpha", Kind: core.KindGoFunc},
		{Service: "svc", Package: "pkg", Name: "Mu", Kind: core.KindGoFunc},
	}
	a, err1 := schemaexport.ExportJSON(ns)
	b, err2 := schemaexport.ExportJSON(ns)
	if err1 != nil || err2 != nil {
		t.Fatalf("ExportJSON errors: %v, %v", err1, err2)
	}
	if a != b {
		t.Errorf("ExportJSON not deterministic:\n%s\n---\n%s", a, b)
	}
}

func TestExportJSON_InputOrderIndependent(t *testing.T) {
	// Two slices with same nodes in different order must produce identical JSON.
	ns1 := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "B", Kind: core.KindGoFunc},
		{Service: "s", Package: "p", Name: "A", Kind: core.KindGoFunc},
	}
	ns2 := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", Kind: core.KindGoFunc},
		{Service: "s", Package: "p", Name: "B", Kind: core.KindGoFunc},
	}
	out1, _ := schemaexport.ExportJSON(ns1)
	out2, _ := schemaexport.ExportJSON(ns2)
	if out1 != out2 {
		t.Errorf("ExportJSON order-independent failed:\n%s\n---\n%s", out1, out2)
	}
}

func TestExportJSON_Empty(t *testing.T) {
	out, err := schemaexport.ExportJSON(nil)
	if err != nil {
		t.Fatalf("ExportJSON(nil) error: %v", err)
	}
	// Should be "null" or "[]".
	if out != "null" && out != "[]" {
		t.Errorf("ExportJSON(nil) = %q, want 'null' or '[]'", out)
	}
}

func TestExportJSON_CallsSorted(t *testing.T) {
	ns := []core.SymbolNode{
		{
			Service: "s", Package: "p", Name: "F",
			Calls: []string{"s::p.Z", "s::p.A", "s::p.M"},
		},
	}
	out, _ := schemaexport.ExportJSON(ns)
	// Z should appear after M after A in the Calls array.
	posA := strings.Index(out, `"s::p.A"`)
	posM := strings.Index(out, `"s::p.M"`)
	posZ := strings.Index(out, `"s::p.Z"`)
	if posA < 0 || posM < 0 || posZ < 0 {
		t.Fatalf("ExportJSON missing calls in output: %s", out)
	}
	if !(posA < posM && posM < posZ) {
		t.Errorf("ExportJSON Calls not sorted: A@%d M@%d Z@%d in:\n%s", posA, posM, posZ, out)
	}
}

// ─── ExportDOT ────────────────────────────────────────────────────────────────

func TestExportDOT_StartsWithDigraph(t *testing.T) {
	out := schemaexport.ExportDOT(edges())
	if !strings.HasPrefix(out, "digraph fathom {") {
		t.Errorf("ExportDOT missing header; got: %q", out[:min(len(out), 40)])
	}
}

func TestExportDOT_ContainsEdges(t *testing.T) {
	out := schemaexport.ExportDOT(edges())
	for _, e := range edges() {
		if !strings.Contains(out, e.FromSymbol) {
			t.Errorf("ExportDOT missing FromSymbol %q", e.FromSymbol)
		}
		if !strings.Contains(out, e.ToSymbol) {
			t.Errorf("ExportDOT missing ToSymbol %q", e.ToSymbol)
		}
	}
}

func TestExportDOT_Sorted(t *testing.T) {
	out := schemaexport.ExportDOT(edges())
	// Alpha -> Bravo should appear before Bravo -> Alpha and Bravo -> Charlie.
	posAlpha := strings.Index(out, "svc::pkg.Alpha")
	posBravoA := strings.Index(out, "svc::pkg.Bravo -> svc::pkg.Alpha")
	posBravoC := strings.Index(out, "svc::pkg.Bravo -> svc::pkg.Charlie")
	if posAlpha < 0 || posBravoA < 0 || posBravoC < 0 {
		t.Fatalf("ExportDOT missing expected edges:\n%s", out)
	}
	if !(posAlpha < posBravoA && posBravoA < posBravoC) {
		t.Errorf("ExportDOT edges not sorted correctly; posAlpha=%d posBravoA=%d posBravoC=%d\n%s",
			posAlpha, posBravoA, posBravoC, out)
	}
}

func TestExportDOT_Determinism(t *testing.T) {
	a := schemaexport.ExportDOT(edges())
	b := schemaexport.ExportDOT(edges())
	if a != b {
		t.Errorf("ExportDOT not deterministic:\n%s\n---\n%s", a, b)
	}
}

func TestExportDOT_Empty(t *testing.T) {
	out := schemaexport.ExportDOT(nil)
	if out != "digraph fathom {\n}\n" {
		t.Errorf("ExportDOT(nil) = %q, want empty digraph", out)
	}
}

func TestExportDOT_InputOrderIndependent(t *testing.T) {
	e1 := []core.Edge{
		{FromSymbol: "B", ToSymbol: "C"},
		{FromSymbol: "A", ToSymbol: "B"},
	}
	e2 := []core.Edge{
		{FromSymbol: "A", ToSymbol: "B"},
		{FromSymbol: "B", ToSymbol: "C"},
	}
	out1 := schemaexport.ExportDOT(e1)
	out2 := schemaexport.ExportDOT(e2)
	if out1 != out2 {
		t.Errorf("ExportDOT order-independent failed:\n%s\n---\n%s", out1, out2)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

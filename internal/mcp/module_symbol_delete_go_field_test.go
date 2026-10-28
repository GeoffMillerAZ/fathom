package mcp_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// Like the module-symbol regression beside this test, an empty caller set
// cannot prove a field dead. The real-ingest fixture reads T.n in Len, Take
// and Build, but the field-only member pass indexes no reference edges.
func TestBlastRadiusDeleteGoFieldWithReadersIsNotProvenZero(t *testing.T) {
	srv := typeRefServer(t)
	key := (core.SymbolNode{Service: "tfix", Package: "T", Name: "T.n", FilePath: "p/t.go"}).Key()
	innerKey := (core.SymbolNode{Service: "tfix", Package: "Holder", Name: "Holder.Inner", FilePath: "p/use.go"}).Key()
	found := callTool(t, srv, "find_symbol", map[string]any{"service": "tfix", "name": key})
	fields, _ := found["symbols"].([]any)
	if len(fields) != 1 || fields[0].(map[string]any)["kind"] != string(core.KindGoField) {
		t.Fatalf("fixture field = %v, want one indexed GoField", found)
	}
	for _, symbol := range []string{"n", "Inner", "p.T.n", "T.n", "T.T.n", "p/T.T.n", key, "Holder.Inner", innerKey} {
		t.Run(symbol, func(t *testing.T) {
			assertFieldDeleteUncertain(t, srv, "tfix", symbol, true)
		})
	}
}

func fieldMethodCollisionServer(t *testing.T) *mcp.Server {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module cfix\n\ngo 1.22\n",
		"p/p.go": "package p\n\ntype T struct{ n int }\n\nfunc Read(t T) int { return t.n }\n",
		"q/q.go": "package q\n\ntype T struct{}\n\nfunc (T) n() {}\n",
		"r/r.go": "package r\n\ntype S struct{ Name string }\n\ntype U struct{}\n\nfunc (U) Name() string { return \"method\" }\n\nfunc ReadName(u U) string { return u.Name() }\n",
		"s/s.go": "package s\n\nimport \"cfix/r\"\n\nfunc Get(v r.S) string { return v.Name }\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "collision.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "cfix", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return mcp.NewServer(c)
}

func assertFieldDeleteUncertain(t *testing.T, srv *mcp.Server, service, symbol string, emptyReferents bool) map[string]any {
	t.Helper()
	resolved := callTool(t, srv, "find_symbol", map[string]any{"service": service, "name": symbol})
	symbols, _ := resolved["symbols"].([]any)
	var fieldCount int
	for _, row := range symbols {
		if row.(map[string]any)["kind"] == string(core.KindGoField) {
			fieldCount++
		}
	}
	if fieldCount == 0 {
		t.Fatalf("find_symbol(%q) = %v, want at least one GoField", symbol, resolved)
	}
	got := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": service, "symbol": symbol})
	if got["checked_edges"] != "typed" {
		t.Errorf("delete(%q) did not check typed edges: %v", symbol, got)
	}
	if got["field_references_indexed"] != false {
		t.Errorf("delete(%q) field reference coverage = %v, want false", symbol, got["field_references_indexed"])
	}
	summary, _ := got["summary"].(string)
	if strings.Contains(summary, "proven zero") || !strings.Contains(summary, "field references are not indexed") {
		t.Errorf("delete(%q) summary = %q, want field-reference uncertainty", symbol, summary)
	}
	if emptyReferents {
		rows, ok := got["referents"].([]any)
		if !ok || len(rows) != 0 || got["referent_count"] != float64(0) {
			t.Errorf("delete(%q) referents = %v, want an explicit empty recorded set", symbol, got)
		}
	}
	return got
}

func TestBlastRadiusDeleteGoFieldWithMethodNameCollisions(t *testing.T) {
	srv := fieldMethodCollisionServer(t)
	pFieldKey := (core.SymbolNode{Service: "cfix", Package: "T", Name: "T.n", FilePath: "p/p.go"}).Key()
	rFieldKey := (core.SymbolNode{Service: "cfix", Package: "S", Name: "S.Name", FilePath: "r/r.go"}).Key()
	for _, symbol := range []string{"n", "p.T.n", "T.n", "T.T.n", "p/T.T.n", pFieldKey, "Name", "r.S.Name", "S.Name", "S.S.Name", "r/S.S.Name", rFieldKey} {
		t.Run(symbol, func(t *testing.T) {
			got := assertFieldDeleteUncertain(t, srv, "cfix", symbol, false)
			if symbol == "Name" {
				rows, _ := got["referents"].([]any)
				var readName bool
				for _, row := range rows {
					if row.(map[string]any)["symbol"] == "ReadName" {
						readName = true
					}
				}
				if !readName {
					t.Errorf("delete(Name) lost the nonfield caller ReadName: %v", got)
				}
			}
		})
	}
	got := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": "cfix", "symbol": "U.Name"})
	if got["field_references_indexed"] != nil {
		t.Errorf("delete(U.Name) marked a method as a field: %v", got)
	}
	rows, _ := got["referents"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["symbol"] != "ReadName" {
		t.Errorf("delete(U.Name) referents = %v, want ReadName", got)
	}
}

func TestBlastRadiusDeleteGoFieldBeyondLookupDisplayCap(t *testing.T) {
	const service = "capfix"
	st := store.NewMemoryStore()
	var observations []core.Observation
	add := func(node core.SymbolNode) {
		observations = append(observations, core.Observation{
			ServiceID: service, SourceRef: node.FilePath, SourceKind: node.Kind,
			EvidenceTier: core.TierVerified, Symbol: node.Name, Package: node.Package, Line: 1,
		})
	}
	for i := 0; i < 21; i++ {
		owner := fmt.Sprintf("A%02d", i)
		add(core.SymbolNode{Package: owner, Name: owner + ".Name", Kind: core.KindGoMethod, FilePath: "p/" + owner + ".go"})
		add(core.SymbolNode{Package: "p", Name: owner + "Needle", Kind: core.KindGoFunc, FilePath: "p/needles.go"})
	}
	fields := []core.SymbolNode{
		{Service: service, Package: "Z", Name: "Z.Name", Kind: core.KindGoField, FilePath: "p/z.go"},
		{Service: service, Package: "LongOwner", Name: "LongOwner.LongNeedle", Kind: core.KindGoField, FilePath: "p/long.go"},
	}
	for _, field := range fields {
		add(field)
	}
	if err := st.PutObservations(t.Context(), observations); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { _ = c.Close() })
	srv := mcp.NewServer(c)
	for _, tc := range []struct {
		query, tier string
		field       core.SymbolNode
	}{
		{"NAME", "case_insensitive", fields[0]},
		{"Needle", "substring", fields[1]},
	} {
		t.Run(tc.tier, func(t *testing.T) {
			found := callTool(t, srv, "find_symbol", map[string]any{"service": service, "name": tc.query})
			rows, _ := found["symbols"].([]any)
			if found["matched_via"] != tc.tier || found["truncated"] != true || len(rows) != 20 {
				t.Fatalf("find_symbol(%q) = %v, want 20 truncated %s rows", tc.query, found, tc.tier)
			}
			for _, row := range rows {
				if row.(map[string]any)["kind"] == string(core.KindGoField) {
					t.Fatalf("find_symbol(%q) displayed field despite cap: %v", tc.query, found)
				}
			}
			canonical := callTool(t, srv, "find_symbol", map[string]any{"service": service, "name": tc.field.Key()})
			indexed, _ := canonical["symbols"].([]any)
			if len(indexed) != 1 || indexed[0].(map[string]any)["kind"] != string(core.KindGoField) {
				t.Fatalf("find_symbol(%q) = %v, want indexed GoField", tc.field.Key(), canonical)
			}
			got := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": service, "symbol": tc.query})
			if got["field_references_indexed"] != false {
				t.Errorf("delete(%q) field reference coverage = %v, want false", tc.query, got["field_references_indexed"])
			}
			summary, _ := got["summary"].(string)
			if strings.Contains(summary, "proven zero") || !strings.Contains(summary, "field references are not indexed") {
				t.Errorf("delete(%q) summary = %q, want field-reference uncertainty", tc.query, summary)
			}
		})
	}
}

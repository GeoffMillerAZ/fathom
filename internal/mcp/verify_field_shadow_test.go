package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// fieldShadowServer ingests a real Go module where package p declares a
// struct field and a function of the same name (Config.Default beside
// Default, the internal/capability shape), a test that calls the function,
// a field named like an interface (Holder.Store beside Store), and a second
// package q whose own Default leaves the bare name ambiguous in the call
// graph.
func fieldShadowServer(t *testing.T) *mcp.Server {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":      "module sfix\n\ngo 1.22\n",
		"p/p.go":      "package p\n\ntype Config struct{ Default bool }\n\nfunc Default() Config { return Config{Default: true} }\n",
		"p/store.go":  "package p\n\ntype Store interface{ Get() int }\n\ntype Impl struct{}\n\nfunc (Impl) Get() int { return 1 }\n\ntype Holder struct{ Store Store }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestDefaultIsOn(t *testing.T) {\n\tif !Default().Default {\n\t\tt.Fatal(\"off\")\n\t}\n}\n",
		"q/q.go":      "package q\n\nfunc Default() int { return 1 }\n",
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
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "shadow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "sfix", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return mcp.NewServer(c)
}

// TestVerifyPlanGoFieldDoesNotShadowFunc: a {symbol, package} contract
// names the function, never the same-named struct field, so test_reaches
// and edge_wired are asked about the function and pass. An owner-qualified
// contract still names the field.
func TestVerifyPlanGoFieldDoesNotShadowFunc(t *testing.T) {
	srv := fieldShadowServer(t)
	got := callTool(t, srv, "verify_plan", map[string]any{
		"service": "sfix",
		"tasks": []any{
			map[string]any{"id": "F-1", "claimed": true, "symbol": "Default", "package": "p", "reachable_from": "TestDefaultIsOn"},
			map[string]any{"id": "F-2", "claimed": true, "symbol": "Config.Default", "package": "p", "skip_test": true, "skip": "a field has no call edges"},
		},
	})
	tasks, _ := got["tasks"].([]any)
	if len(tasks) != 2 {
		t.Fatalf("verify_plan returned %d tasks, want 2: %v", len(tasks), got)
	}
	byID := map[string]map[string]any{}
	for _, x := range tasks {
		m := x.(map[string]any)
		byID[m["id"].(string)] = m
	}
	f1 := byID["F-1"]
	for _, name := range []string{"symbol_present", "test_reaches", "edge_wired"} {
		if c := shadowCriterion(t, f1, name); c["status"] != "pass" {
			t.Errorf("F-1 %s = %v, want pass through the function", name, c)
		}
	}
	if d, _ := shadowCriterion(t, f1, "symbol_present")["detail"].(string); d != "Default at p/p.go:5" {
		t.Errorf("F-1 symbol_present detail = %q, want the function at p/p.go:5", d)
	}
	if d, _ := shadowCriterion(t, byID["F-2"], "symbol_present")["detail"].(string); d != "Config.Default at p/p.go:3" {
		t.Errorf("F-2 symbol_present detail = %q, want the field at p/p.go:3", d)
	}
}

// TestFindSymbolGoFieldsRankBelowAndNeverShadow: find_symbol's exact tier
// lists every same-named field, after the functions, methods and types
// (C-R14's console order), and a field never hides the one interface a name
// means from SB-5's implementers.
func TestFindSymbolGoFieldsRankBelowAndNeverShadow(t *testing.T) {
	srv := fieldShadowServer(t)
	got := callTool(t, srv, "find_symbol", map[string]any{"service": "sfix", "name": "Default"})
	rows, _ := got["symbols"].([]any)
	var kinds []string
	for _, r := range rows {
		kinds = append(kinds, r.(map[string]any)["kind"].(string))
	}
	if len(kinds) != 3 || kinds[0] != "GoFunc" || kinds[1] != "GoFunc" || kinds[2] != "GoField" {
		t.Fatalf("find_symbol(Default) kinds = %v, want both functions then the field: %v", kinds, got)
	}
	got = callTool(t, srv, "find_symbol", map[string]any{"service": "sfix", "name": "Store"})
	if impls, _ := got["implementations"].([]any); len(impls) == 0 {
		t.Fatalf("find_symbol(Store) = %v, want the interface's implementers despite the field Holder.Store", got)
	}
}

func shadowCriterion(t *testing.T, task map[string]any, name string) map[string]any {
	t.Helper()
	for _, c := range task["criteria"].([]any) {
		if m := c.(map[string]any); m["name"] == name {
			return m
		}
	}
	t.Fatalf("task %v has no %s criterion: %v", task["id"], name, task["criteria"])
	return nil
}

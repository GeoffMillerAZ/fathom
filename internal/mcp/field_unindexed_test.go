package mcp_test

import (
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

// Field references are not indexed (the delete check's rule,
// safe_delete.go): a rename of a Go struct field never reads "0 call
// sites — nothing to rename". The field T.n has three readers (Len, Take,
// Build) the index does not record.
func TestRenameGoFieldWithholdsCallSites(t *testing.T) {
	srv := typeRefServer(t)
	key := (core.SymbolNode{Service: "tfix", Package: "T", Name: "T.n", FilePath: "p/t.go"}).Key()
	for _, symbol := range []string{"T.n", key} {
		t.Run(symbol, func(t *testing.T) {
			got := callTool(t, srv, "blast_radius", map[string]any{"service": "tfix", "kind": "rename", "symbol": symbol})
			if got["field_references_indexed"] != false {
				t.Errorf("rename %s: field_references_indexed = %v, want false: %v", symbol, got["field_references_indexed"], got)
			}
			s, _ := got["summary"].(string)
			if strings.Contains(s, "nothing to rename") || !strings.Contains(s, "field references are not indexed") {
				t.Errorf("rename %s summary = %q, want the call sites named unknown because field references are not indexed", symbol, s)
			}
		})
	}
}

// symbol_context on a Go struct field never answers callers:[] and
// related_tests:[] as if nothing used it.
func TestSymbolContextGoFieldDisclosesUnindexedReferences(t *testing.T) {
	srv := typeRefServer(t)
	got := callTool(t, srv, "symbol_context", map[string]any{"service": "tfix", "symbol": "T.n"})
	if def, _ := got["def"].(map[string]any); def == nil || def["kind"] != string(core.KindGoField) {
		t.Fatalf("symbol_context(T.n) def = %v, want the GoField", got["def"])
	}
	if got["field_references_indexed"] != false {
		t.Errorf("field_references_indexed = %v, want false: %v", got["field_references_indexed"], got)
	}
	for _, k := range []string{"callers", "related_tests"} {
		if s, ok := got[k].(string); !ok || !strings.Contains(s, "field references are not indexed") {
			t.Errorf("%s = %#v, want the not-indexed note in place of an empty list", k, got[k])
		}
	}
}

// A stack frame or failing test names a function, never a struct field: a
// function seed `Run` never seeds the field Opts.Run as a candidate symbol
// (a literal search hit in its file is a different, heuristic seed).
func TestLocalizeFunctionSeedNeverSeedsGoField(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":   "module lfix\n\ngo 1.22\n",
		"p/run.go": "package p\n\n// Run runs.\nfunc Run() int { return 1 }\n",
		"p/opt.go": "package p\n\n// Opts carries a field named like Run.\ntype Opts struct{ Run bool }\n",
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
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "lfix.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "lfix", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	got := callTool(t, mcp.NewServer(c), "localize", map[string]any{"service": "lfix", "failing_test": "Run"})
	cands, _ := got["candidates"].([]any)
	namedRun := false
	for _, x := range cands {
		c := x.(map[string]any)
		if c["symbol"] == "Opts.Run" {
			t.Fatalf("localize seeded the field Opts.Run as a function: candidates %v", cands)
		}
		namedRun = namedRun || (c["file"] == "p/run.go" && c["symbol"] == "Run")
	}
	if !namedRun {
		t.Fatalf("localize candidates %v, want the function Run at p/run.go", cands)
	}
}

// find_callers and test_impact on a Go struct field never answer "no
// callers" or "no tests": field references are not indexed.
func TestCallersAndTestImpactGoFieldWithheld(t *testing.T) {
	srv := typeRefServer(t)
	for _, tool := range []string{"find_callers", "test_impact"} {
		t.Run(tool, func(t *testing.T) {
			got := callTool(t, srv, tool, map[string]any{"service": "tfix", "symbol": "T.n"})
			if got["field_references_indexed"] != false {
				t.Errorf("%s(T.n): field_references_indexed = %v, want false: %v", tool, got["field_references_indexed"], got)
			}
			if s, _ := got["summary"].(string); !strings.Contains(s, "field references are not indexed") {
				t.Errorf("%s(T.n) summary = %q, want the answer named unknown because field references are not indexed", tool, s)
			}
		})
	}
}

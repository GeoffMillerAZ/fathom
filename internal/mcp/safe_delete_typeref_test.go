package mcp_test

// safe_delete_typeref_test.go — O1127, end to end through a real ingest:
// blast_radius kind:"delete" on a Go type lists every symbol that uses the
// type (a parameter, a result, a field type, a composite literal, a
// qualified use from another package), each row carrying the enclosing
// symbol's key, the key find_symbol resolves to that one symbol.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func typeRefServer(t *testing.T) *mcp.Server {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module tfix\n\ngo 1.22\n",
		"p/t.go": "package p\n\n" +
			"// T is the type under deletion.\n" +
			"type T struct{ n int }\n\n" +
			"// Len is T's own method: it goes with T.\n" +
			"func (t T) Len() int { return t.n }\n",
		"p/use.go": "package p\n\n" +
			"// Take takes a T.\n" +
			"func Take(t T) int { return t.n }\n\n" +
			"// Make returns a T.\n" +
			"func Make() *T { return nil }\n\n" +
			"// Holder holds a T in a field.\n" +
			"type Holder struct {\n\tInner T\n}\n\n" +
			"// Build builds one.\n" +
			"func Build() int {\n\tv := T{n: 1}\n\treturn v.n\n}\n\n" +
			"// Unrelated names no T.\n" +
			"func Unrelated(x int) int { return x }\n",
		"q/q.go": "package q\n\n" +
			"import \"tfix/p\"\n\n" +
			"// Wrap takes a p.T.\n" +
			"func Wrap(t p.T) int { return t.Len() }\n",
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
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "typeref.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "tfix", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return mcp.NewServer(c)
}

func TestSafeDelete_TypeReferentsInSignaturesFieldsAndLiterals(t *testing.T) {
	srv := typeRefServer(t)
	got := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": "tfix", "symbol": "T"})
	rows, _ := got["referents"].([]any)
	bySymbol := map[string]map[string]any{}
	for _, r := range rows {
		m := r.(map[string]any)
		bySymbol[m["symbol"].(string)] = m
	}
	for _, want := range []struct{ symbol, file, why string }{
		{"Take", "p/use.go", "a parameter"},
		{"Make", "p/use.go", "a result"},
		{"Holder", "p/use.go", "a field type"},
		{"Build", "p/use.go", "a composite literal"},
		{"Wrap", "q/q.go", "a qualified parameter from another package"},
	} {
		row, ok := bySymbol[want.symbol]
		if !ok {
			t.Errorf("delete(T) referents = %v: want %s (%s)", rows, want.symbol, want.why)
			continue
		}
		if row["file"] != want.file {
			t.Errorf("%s referent file = %v, want %s", want.symbol, row["file"], want.file)
		}
		key, _ := row["key"].(string)
		if key == "" {
			t.Errorf("%s referent carries no key: %v", want.symbol, row)
			continue
		}
		fs := callTool(t, srv, "find_symbol", map[string]any{"service": "tfix", "name": key})
		syms, _ := fs["symbols"].([]any)
		if len(syms) != 1 {
			t.Errorf("find_symbol(%s) = %v, want exactly the one referent symbol", key, fs)
			continue
		}
		s := syms[0].(map[string]any)
		if s["name"] != want.symbol || s["file_path"] != want.file || fmt.Sprint(s["line"]) != fmt.Sprint(row["line"]) {
			t.Errorf("find_symbol(%s) = %v, want %s at %s:%v (the referent row)", key, s, want.symbol, want.file, row["line"])
		}
	}
	for _, never := range []string{"T", "Len", "Unrelated"} {
		if _, ok := bySymbol[never]; ok {
			t.Errorf("delete(T) lists %s: the type itself, its own method and a symbol naming no T are not referents (%v)", never, rows)
		}
	}
	if n, _ := got["referent_count"].(float64); int(n) != len(rows) {
		t.Errorf("referent_count = %v, want %d", got["referent_count"], len(rows))
	}
}

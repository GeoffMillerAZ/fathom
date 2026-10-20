package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// statBackend adds the indexed file_stats rows to the fake backend.
type statBackend struct {
	*extendedFakeBackend
	rows []store.FileStat
}

func (b *statBackend) FileStats(context.Context, string) ([]store.FileStat, bool, error) {
	return b.rows, len(b.rows) > 0, nil
}

func indexedRow(t *testing.T, root, rel string) store.FileStat {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	return store.FileStat{Path: rel, Size: fi.Size(), MtimeNS: fi.ModTime().UnixNano(),
		ContentHash: fusion.ExtractorEpoch + "::" + fusion.FingerprintHash(data)}
}

// O1450: a snippet is served only from the file the index read.
func TestSymbolContextWithholdsSnippetOfChangedFile(t *testing.T) {
	const src = "package pkg\n\nfunc Alpha() {\n\treturn\n}\n"
	cases := []struct {
		name     string
		mutate   func(path string)
		withheld bool
	}{
		{"unchanged", func(string) {}, false},
		{"touched only", func(p string) {
			later := time.Now().Add(time.Hour)
			_ = os.Chtimes(p, later, later)
		}, false},
		{"rewritten longer", func(p string) {
			_ = os.WriteFile(p, []byte("package pkg\n\n// new\n// lines\nfunc Beta(a, b int) {\n}\nfunc Alpha() {\n}\n"), 0o644)
		}, true},
		{"same size other text", func(p string) {
			_ = os.WriteFile(p, []byte(strings.ReplaceAll(src, "Alpha", "Gamma")), 0o644)
			later := time.Now().Add(time.Hour)
			_ = os.Chtimes(p, later, later)
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			path := writeCorpusFile(t, root, "pkg/alpha.go", src)
			row := indexedRow(t, root, "pkg/alpha.go")
			c.mutate(path)
			be := &statBackend{&extendedFakeBackend{
				nodes:    []core.SymbolNode{{Service: "svc", Package: "pkg", Name: "Alpha", Kind: core.KindGoFunc, FilePath: "pkg/alpha.go", Line: 3, Signature: "func Alpha()"}},
				services: []string{"svc"}, svcPath: root,
			}, []store.FileStat{row}}
			resp := roundtrip(t, mcp.NewServer(be), request{
				JSONRPC: "2.0", ID: 3101, Method: "tools/call",
				Params: map[string]any{"name": "symbol_context", "arguments": map[string]any{"service": "svc", "symbol": "Alpha"}},
			})
			p := xtraPayload(t, resp)
			_, hasSnip := p["snippet"]
			why, _ := p["snippet_withheld"].(string)
			if c.withheld {
				if hasSnip || !strings.HasPrefix(why, "file changed since the index") {
					t.Fatalf("want snippet withheld, got snippet=%v withheld=%q", hasSnip, why)
				}
			} else if !hasSnip || why != "" {
				t.Fatalf("want snippet served, got snippet=%v withheld=%q", hasSnip, why)
			}
		})
	}
}

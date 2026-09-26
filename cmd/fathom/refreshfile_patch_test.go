package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// recordingPatcher stands in for the MCP server's graph memo.
type recordingPatcher struct {
	has     bool
	patched []string // packages patched, in order
	nodes   []core.SymbolNode
	dropped int
}

func (p *recordingPatcher) HasCachedGraph(string) bool { return p.has }
func (p *recordingPatcher) PatchPackageGraph(_, pkg string, nodes []core.SymbolNode, _ []core.Edge) {
	p.patched = append(p.patched, pkg)
	p.nodes = append(p.nodes, nodes...)
}
func (p *recordingPatcher) DropCachedGraphs(string) { p.dropped++ }

// TestRefreshFile_PatchesCachedGraph (SU-4): a refreshed file patches the
// memoized graph for its package with the package's CURRENT nodes — the
// changed symbol in, the old one out — only when a graph is memoized at all;
// a file that now defines nothing drops the memo instead of guessing.
func TestRefreshFile_PatchesCachedGraph(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package main\n\nfunc Original() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "refresh-svc", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// Memo present: the package is patched with the fresh node set.
	if err := os.WriteFile(target, []byte("package main\n\nfunc Changed() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &recordingPatcher{has: true}
	out := buildRefreshFileResponse(ctx, c, p, "refresh-svc", "target.go")
	if !out.Refreshed || !out.GraphPatched {
		t.Fatalf("response = %+v, want Refreshed and GraphPatched", out)
	}
	if len(p.patched) != 1 || p.patched[0] != "main" {
		t.Fatalf("patched packages = %v, want [main]", p.patched)
	}
	var sawChanged, sawOriginal bool
	for _, n := range p.nodes {
		sawChanged = sawChanged || n.Name == "Changed"
		sawOriginal = sawOriginal || n.Name == "Original"
	}
	if !sawChanged || sawOriginal {
		t.Fatalf("patched nodes Changed=%v Original=%v, want true/false", sawChanged, sawOriginal)
	}

	// No memo: nothing is listed or patched, and the response says so.
	p2 := &recordingPatcher{has: false}
	out2 := buildRefreshFileResponse(ctx, c, p2, "refresh-svc", "target.go")
	if out2.GraphPatched || len(p2.patched) != 0 || p2.dropped != 0 {
		t.Fatalf("no-memo refresh patched anyway: %+v %+v", out2, p2)
	}

	// The file now defines nothing: no package to patch, memo dropped.
	if err := os.WriteFile(target, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p3 := &recordingPatcher{has: true}
	out3 := buildRefreshFileResponse(ctx, c, p3, "refresh-svc", "target.go")
	if out3.GraphPatched || p3.dropped != 1 || len(p3.patched) != 0 {
		t.Fatalf("empty refresh: response=%+v patcher=%+v, want dropped once and nothing patched", out3, p3)
	}
}

// RT-27 (docs/design/responsible-tenant.md §7.2): /refresh-file stats
// before it reads — a file over the 2 MiB cap is never opened and is
// reported as skipped, not failed — and finds the refreshed file's packages
// by source_ref, never by loading the service's whole symbol table.
func TestRefreshFileStatCapAndPackageLookup(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go", "package main\n\nfunc A() int { return 1 }\n")
	write("b.go", "package main\n\nfunc B() int { return 2 }\n")
	st := &symbolLoadCountingStore{MemoryStore: store.NewMemoryStore()}
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "rt27", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	write("a.go", "package main\n\nfunc A() int { return 1 }\n\nfunc A2() int { return 3 }\n")
	st.loads.Store(0)
	p := &recordingPatcher{has: true}
	out := buildRefreshFileResponse(ctx, c, p, "rt27", "a.go")
	if !out.Refreshed || !out.GraphPatched || out.Error != "" {
		t.Fatalf("response = %+v, want Refreshed and GraphPatched", out)
	}
	if n := st.loads.Load(); n != 0 {
		t.Fatalf("a one-file refresh loaded the whole symbol table %d time(s)", n)
	}
	if len(p.patched) != 1 || p.patched[0] != "main" {
		t.Fatalf("patched packages = %v, want [main]", p.patched)
	}
	names := map[string]bool{}
	for _, n := range p.nodes {
		names[n.Name] = true
	}
	if !names["A"] || !names["A2"] || !names["B"] || len(p.nodes) != 3 {
		t.Fatalf("patched nodes = %v, want the package's complete set A, A2, B", names)
	}

	// Oversize and unreadable: only a stat can have answered.
	big := filepath.Join(dir, "big.go")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(3 << 20); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := os.Chmod(big, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(big, 0o644) })
	out = buildRefreshFileResponse(ctx, c, &recordingPatcher{has: true}, "rt27", "big.go")
	if out.Error != "" || out.Refreshed {
		t.Fatalf("oversize refresh = %+v; want no read attempted (no error) and nothing refreshed", out)
	}
	if !strings.Contains(out.Skipped, "oversize") {
		t.Fatalf("oversize refresh skipped = %q, want it named oversize", out.Skipped)
	}
}

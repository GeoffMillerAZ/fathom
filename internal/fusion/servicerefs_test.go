package fusion_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// RT-89 (§6): an ingest extracts routes and outbound HTTP client calls
// into store rows — every readable file the first time, then only the
// files that changed — and drops a deleted file's rows.
func TestIngestExtractsServiceRefs(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write := func(rel, data string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("srv.go", "package main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.HandleFunc(\"/items\", list)\n}\n\nfunc list(w http.ResponseWriter, r *http.Request) {}\n")
	write("cli.go", "package main\n\nimport \"net/http\"\n\nfunc fetch() {\n\thttp.Get(\"http://catalog/items\")\n}\n")

	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	p.WithGitHead(func(string) string { return "" })
	p.WithGitLog(func(string) string { return "" })
	ingest := func() store.ServiceRefs {
		t.Helper()
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
			t.Fatal(err)
		}
		refs, complete, err := st.ServiceRefsFor(ctx, "svc")
		if err != nil || !complete {
			t.Fatalf("ServiceRefsFor: complete=%v err=%v, want complete after an ingest", complete, err)
		}
		return refs
	}
	refs := ingest()
	if len(refs.Routes) != 1 || refs.Routes[0].File != "srv.go" || refs.Routes[0].Path != "/items" {
		t.Fatalf("routes = %+v, want srv.go's /items", refs.Routes)
	}
	if len(refs.Calls) != 1 || refs.Calls[0].File != "cli.go" {
		t.Fatalf("calls = %+v, want cli.go's GET", refs.Calls)
	}
	// Unchanged: the rows stay.
	if again := ingest(); len(again.Routes) != 1 || len(again.Calls) != 1 {
		t.Fatalf("after an unchanged ingest refs = %+v", again)
	}
	// Deleted: its rows go.
	if err := os.Remove(filepath.Join(root, "cli.go")); err != nil {
		t.Fatal(err)
	}
	if after := ingest(); len(after.Calls) != 0 || len(after.Routes) != 1 {
		t.Fatalf("after deleting cli.go refs = %+v, want its call gone", after)
	}
}

// newRefsFixture is a two-file service (srv.go serves /items, cli.go calls
// catalog) with a pipeline over a fresh SQLite store.
func newRefsFixture(t *testing.T) (root string, st *store.SQLiteStore, p *fusion.Pipeline) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "srv.go"), []byte(srvOneRoute), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cli.go"), []byte("package main\n\nimport \"net/http\"\n\nfunc fetch() {\n\thttp.Get(\"http://catalog/items\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var err error
	st, err = store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	p = fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	p.WithGitHead(func(string) string { return "" })
	p.WithGitLog(func(string) string { return "" })
	return root, st, p
}

const srvOneRoute = "package main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.HandleFunc(\"/items\", list)\n}\n\nfunc list(w http.ResponseWriter, r *http.Request) {}\n"

func routePaths(refs store.ServiceRefs) []string {
	var out []string
	for _, r := range refs.Routes {
		out = append(out, r.File+":"+r.Path)
	}
	sort.Strings(out)
	return out
}

// A per-file refresh (the edit hook's /refresh-file) moves the file's
// fingerprint, so the next ingest hash-gates it: the refresh itself must
// replace that file's routes and calls, or the old ones stay forever.
func TestReingestFile_ReplacesServiceRefs(t *testing.T) {
	ctx := context.Background()
	root, st, p := newRefsFixture(t)
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	two := "package main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.HandleFunc(\"/items\", list)\n\thttp.HandleFunc(\"/orders\", list)\n}\n\nfunc list(w http.ResponseWriter, r *http.Request) {}\n"
	if err := os.WriteFile(filepath.Join(root, "srv.go"), []byte(two), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "svc", root, "srv.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	refs, complete, err := st.ServiceRefsFor(ctx, "svc")
	if err != nil || !complete {
		t.Fatalf("complete=%v err=%v", complete, err)
	}
	if got, want := strings.Join(routePaths(refs), ","), "srv.go:/items,srv.go:/orders"; got != want {
		t.Fatalf("routes after refresh+ingest = %s, want %s", got, want)
	}
	if len(refs.Calls) != 1 {
		t.Fatalf("calls = %+v, want cli.go's call kept", refs.Calls)
	}
}

// A file unreadable for one ingest (the /Volumes EPERM wedge) keeps its
// routes and calls, exactly as pass 3 keeps its observations: once it reads
// again with an unchanged hash it is never pending, so dropped rows would
// never come back.
func TestIngest_TransientUnreadableKeepsServiceRefs(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 000 file")
	}
	ctx := context.Background()
	root, st, p := newRefsFixture(t)
	ingest := func() {
		t.Helper()
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
			var rfe *fusion.ReadFailuresError
			if !errors.As(err, &rfe) {
				t.Fatal(err)
			}
		}
	}
	ingest()
	srv := filepath.Join(root, "srv.go")
	if err := os.Chmod(srv, 0); err != nil {
		t.Fatal(err)
	}
	ingest()
	if err := os.Chmod(srv, 0o644); err != nil {
		t.Fatal(err)
	}
	ingest()
	refs, _, err := st.ServiceRefsFor(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(routePaths(refs), ","); got != "srv.go:/items" {
		t.Fatalf("routes = %q, want srv.go:/items kept across a transient read failure", got)
	}
}

// A service's first extraction while a file is unreadable must not mark
// the rows complete: that file was never extracted, and once it reads
// again unchanged it would never be pending.
func TestIngest_FirstExtractionWithUnreadableStaysIncomplete(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 000 file")
	}
	ctx := context.Background()
	root, st, p := newRefsFixture(t)
	srv := filepath.Join(root, "srv.go")
	if err := os.Chmod(srv, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err == nil {
		t.Fatal("want a read failure")
	}
	if _, complete, _ := st.ServiceRefsFor(ctx, "svc"); complete {
		t.Fatal("service refs marked complete with srv.go never extracted")
	}
	if err := os.Chmod(srv, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	refs, complete, err := st.ServiceRefsFor(ctx, "svc")
	if err != nil || !complete {
		t.Fatalf("complete=%v err=%v", complete, err)
	}
	if got := strings.Join(routePaths(refs), ","); got != "srv.go:/items" {
		t.Fatalf("routes = %q, want srv.go:/items", got)
	}
}

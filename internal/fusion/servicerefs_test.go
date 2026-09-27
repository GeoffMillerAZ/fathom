package fusion_test

import (
	"context"
	"os"
	"path/filepath"
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

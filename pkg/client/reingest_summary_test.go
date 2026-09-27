package client_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestReingestFileKeepsSummaryCountCurrent (review finding, RT-76/RT-79):
// a single-file refresh does not move the watermark the summary row is
// keyed by, so it recounts the row's symbols in place — list_services
// never reports the pre-refresh count as current while the patched
// snapshot shows the new symbols.
func TestReingestFileKeepsSummaryCountCurrent(t *testing.T) {
	for name, open := range map[string]func(t *testing.T) store.Store{
		"memory": func(*testing.T) store.Store { return store.NewMemoryStore() },
		"sqlite": func(t *testing.T) store.Store {
			s, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			return s
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target.go")
			if err := os.WriteFile(target, []byte("package main\n\nfunc A() {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			c := client.New(connector.NewLocalConnector(), open(t))
			t.Cleanup(func() { _ = c.Close() })
			ctx := context.Background()
			if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
				t.Fatal(err)
			}
			before, err := c.ServiceSummary(ctx, "svc")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("package main\n\nfunc A() {}\n\nfunc B() {}\n\nfunc C() {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := c.ReingestFile(ctx, "svc", "target.go"); err != nil {
				t.Fatal(err)
			}
			after, err := c.ServiceSummary(ctx, "svc")
			if err != nil {
				t.Fatal(err)
			}
			if after.SymbolCount != before.SymbolCount+2 {
				t.Fatalf("symbol_count after refresh-file = %d, want %d (before %d)", after.SymbolCount, before.SymbolCount+2, before.SymbolCount)
			}
		})
	}
}

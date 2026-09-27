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

// RT-80 review: "indexed" keeps the meaning verify's exit-2 fail-open and
// init's "no symbols yet" line rely on — a meta row AND at least one
// symbol. A service ingested from an empty tree (or purged) has a meta row
// with indexed_at > 0 and nothing to verify against: not indexed.
func TestIsIndexedRequiresSymbols(t *testing.T) {
	ctx := context.Background()
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	t.Cleanup(func() { _ = c.Close() })

	empty := t.TempDir()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "empty", Path: empty}); err != nil {
		t.Fatalf("Ingest empty: %v", err)
	}
	if ok, _, err := c.IsIndexed(ctx, "empty"); err != nil || ok {
		t.Fatalf("IsIndexed(empty tree) = %v, %v; want false (a meta row with zero symbols is not indexed)", ok, err)
	}

	full := t.TempDir()
	if err := os.WriteFile(filepath.Join(full, "a.go"), []byte("package a\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "Full", Path: full}); err != nil {
		t.Fatalf("Ingest full: %v", err)
	}
	ok, resolved, err := c.IsIndexed(ctx, "full")
	if err != nil || !ok || resolved != "Full" {
		t.Fatalf("IsIndexed(full, case-folded) = %v, %q, %v; want true, \"Full\"", ok, resolved, err)
	}
	if ok, _, _ := c.IsIndexed(ctx, "never"); ok {
		t.Fatal("IsIndexed(never ingested) = true")
	}
}

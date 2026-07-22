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

// TestIngestService_UsesAndPreservesStoredPath is a regression test for the bug
// where IngestService re-ingested from the bare service ID instead of the
// recorded filesystem path — re-walking the wrong directory and clobbering
// ServiceMeta.path to empty. That silently froze the daemon's index, because
// auto-refresh / staleness_report / the ingest tool all depend on that path.
func TestIngestService_UsesAndPreservesStoredPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ms := store.NewMemoryStore()
	// The metadata an initial `ingest --service svc --path <dir>` would record.
	if err := ms.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}

	c := client.New(connector.NewLocalConnector(), ms)
	if err := c.IngestService(ctx, "svc"); err != nil {
		t.Fatalf("IngestService: %v", err)
	}

	// It must re-ingest from the stored path — Foo is now indexed.
	nodes, err := c.FindSymbol(ctx, "svc", "Foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatal("IngestService did not re-ingest from the stored path (Foo not found)")
	}

	// ...and it must PRESERVE the path, not clobber it to the bare ID.
	path, _, found, err := c.ServiceMeta(ctx, "svc")
	if err != nil || !found {
		t.Fatalf("ServiceMeta missing after re-ingest: found=%v err=%v", found, err)
	}
	if path != dir {
		t.Fatalf("ServiceMeta.path was clobbered: got %q, want %q", path, dir)
	}
}

// TestClient_IngestStats is the ADR-021 §2 / TI-6 counterpart: Client exposes
// the pipeline's process-lifetime unclassified/oversize counters so the mcp
// server_stats/ingest tools have something to surface (via the optional
// statsBackend capability in internal/mcp).
func TestClient_IngestStats(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	// Not k8s/compose/openapi-shaped — classifies as unclassified.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("logging:\n  level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())

	before, _ := c.IngestStats()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "statssvc", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	after, _ := c.IngestStats()
	if after-before != 1 {
		t.Errorf("IngestStats() unclassified delta = %d, want 1", after-before)
	}
}

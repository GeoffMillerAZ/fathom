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

// TestIngestServiceResumesKilledFirstIngest pins review finding 1 on RT-109:
// a first ingest killed before its final commit leaves no service_meta,
// only an in_progress ingest_state row naming the corpus root. The
// quarantine remedy's plain `ingest` (service only) must re-ingest from
// that root — not from the bare service ID — and commit its meta.
func TestIngestServiceResumesKilledFirstIngest(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ms := store.NewMemoryStore()
	killed := store.IngestState{ServiceID: "svc", State: store.IngestInProgress, PID: os.Getpid() + 1, StartedAt: 1, Path: dir}
	if err := ms.SetIngestState(ctx, killed); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), ms)
	row, found, err := c.IngestState(ctx, "svc")
	if err != nil || !found || row.Path != dir || row.PID != killed.PID || row.StartedAt != 1 {
		t.Fatalf("IngestState = %+v found=%v err=%v, want the killed row with its path", row, found, err)
	}
	if err := c.IngestService(ctx, "svc"); err != nil {
		t.Fatalf("IngestService: %v", err)
	}
	if nodes, _ := c.FindSymbol(ctx, "svc", "Foo"); len(nodes) == 0 {
		t.Fatal("IngestService did not ingest from the interrupted ingest's path (Foo not found)")
	}
	if path, _, found, _ := c.ServiceMeta(ctx, "svc"); !found || path != dir {
		t.Fatalf("service_meta after the resumed ingest: path=%q found=%v, want %q", path, found, dir)
	}
}

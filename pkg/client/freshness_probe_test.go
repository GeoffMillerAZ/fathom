package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestServiceFreshnessProbeAfterIngest (RT-110, I48): after an ingest of a
// non-git corpus the request path's probe answers exactly the watermark the
// ingest recorded (no refresh loop) and computes no content manifest.
func TestServiceFreshnessProbeAfterIngest(t *testing.T) {
	ctx := context.Background()
	docs := t.TempDir()
	for _, f := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(docs, f), []byte("# "+f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ms := store.NewMemoryStore()
	c := New(connector.NewLocalConnector(), ms)
	t.Cleanup(func() { _ = c.Close() })
	if err := c.IngestServiceAt(ctx, "docs", docs); err != nil {
		t.Fatal(err)
	}
	meta, _, _ := ms.GetServiceMeta(ctx, "docs")
	before := connector.ManifestComputations()
	if p, err := c.ServiceFreshnessProbe(ctx, "docs"); err != nil || p != meta.IndexedCommit {
		t.Fatalf("probe after ingest = %q, %v; want the recorded %q", p, err, meta.IndexedCommit)
	}
	if n := connector.ManifestComputations() - before; n != 0 {
		t.Fatalf("the probe computed %d content manifest(s)", n)
	}
}

package connector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestFreshnessProbeNeverReadsCorpus (RT-110, I48): the request path's
// freshness probe for a non-git corpus never reads a file. It answers the
// content manifest the last ingest recorded while the stats stand, answers a
// stat token (drift — the ingest recomputes the manifest) once they move,
// and re-walks the stats at most once per probe TTL.
func TestFreshnessProbeNeverReadsCorpus(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	for i := 0; i < 50; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.md", i)), []byte(fmt.Sprintf("# %d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Unix(1_000_000, 0)
	prevNow := manifestNow
	t.Cleanup(func() { manifestNow = prevNow })
	manifestNow = func() time.Time { return now }

	lc := NewLocalConnector()
	svc := core.ServiceRef{ID: "d", Path: dir}
	n0 := ManifestComputations()
	w1, err := lc.Watermark(ctx, svc) // the ingest records the content manifest
	if err != nil {
		t.Fatal(err)
	}
	if p, err := lc.FreshnessProbe(ctx, svc); err != nil || p != w1 {
		t.Fatalf("probe over unchanged stats = %q, %v; want the recorded manifest %q", p, err, w1)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "f07.md"), later, later); err != nil {
		t.Fatal(err)
	}
	if p, _ := lc.FreshnessProbe(ctx, svc); p != w1 {
		t.Fatalf("probe within its TTL re-walked the stats: %q", p)
	}
	now = now.Add(probeTTL + time.Second)
	p, err := lc.FreshnessProbe(ctx, svc)
	if err != nil || p == w1 || !strings.HasPrefix(p, "stat:") {
		t.Fatalf("probe after a touch = %q, %v; want a stat: drift token", p, err)
	}
	if got := ManifestComputations() - n0; got != 1 {
		t.Fatalf("the probe recomputed the content manifest (%d computations, want only the ingest's 1)", got)
	}
	// The refresh's ingest recomputes the manifest (same content → same
	// watermark); the probe then answers it again: no refresh loop.
	if w2, _ := lc.Watermark(ctx, svc); w2 != w1 {
		t.Fatalf("same content, manifest moved: %q vs %q", w2, w1)
	}
	now = now.Add(probeTTL + time.Second)
	if p, _ := lc.FreshnessProbe(ctx, svc); p != w1 {
		t.Fatalf("probe after the ingest = %q, want %q", p, w1)
	}
}

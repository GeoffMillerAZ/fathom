package connector

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestStatGatedManifest (RT-110): a non-git corpus's content manifest is
// recomputed only when a file's path, size or mtime moves, and a corpus over
// the stat cap is freshness unknown — remembered for the TTL, never read.
func TestStatGatedManifest(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	for _, f := range []string{"a.md", "b.md", "c.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("# "+f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lc := NewLocalConnector()
	svc := core.ServiceRef{ID: "d", Path: dir}
	n0 := ManifestComputations()
	w1, err := lc.Watermark(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	w2, _ := lc.Watermark(ctx, svc)
	if w1 != w2 || ManifestComputations()-n0 != 1 {
		t.Fatalf("unchanged stats: %q/%q after %d computations; want one", w1, w2, ManifestComputations()-n0)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.md"), later, later); err != nil {
		t.Fatal(err)
	}
	w3, _ := lc.Watermark(ctx, svc)
	if ManifestComputations()-n0 != 2 || w3 != w1 {
		t.Fatalf("a touched file must recompute (same content, same manifest): %d computations, %q", ManifestComputations()-n0, w3)
	}

	prevCap, prevTTL := manifestStatCap, manifestTTL
	t.Cleanup(func() { manifestStatCap, manifestTTL = prevCap, prevTTL })
	manifestStatCap, manifestTTL = 2, time.Hour
	big := NewLocalConnector()
	n1 := ManifestComputations()
	if _, err := big.Watermark(ctx, svc); !errors.Is(err, ErrManifestTooLarge) {
		t.Fatalf("over the cap: %v; want ErrManifestTooLarge", err)
	}
	if _, err := big.Watermark(ctx, svc); !errors.Is(err, ErrManifestTooLarge) {
		t.Fatalf("over the cap, remembered: %v", err)
	}
	if ManifestComputations() != n1 {
		t.Fatal("an over-the-cap corpus was read")
	}
}

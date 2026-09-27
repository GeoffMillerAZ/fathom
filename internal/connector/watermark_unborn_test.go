package connector_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestWatermark_UnbornRepoUsesManifest is RT-110 (ADR-017 amended): a work
// tree git confirms has no commit yet (a fresh `git init`) has no HEAD to
// compare, so its freshness is its content — the manifest, as for a non-git
// corpus — never freshness unknown, which would leave an ingest with no
// watermark at all.
func TestWatermark_UnbornRepoUsesManifest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := gitProofTempDir(t)
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	c := connector.NewLocalConnector()
	svc := core.ServiceRef{ID: "svc", Path: root}
	wm, err := c.Watermark(context.Background(), svc)
	if err != nil || !strings.HasPrefix(wm, "manifest:") {
		t.Fatalf("unborn repo Watermark = %q, %v; want a manifest: token", wm, err)
	}
	probe, err := c.FreshnessProbe(context.Background(), svc)
	if err != nil || probe == "" {
		t.Fatalf("unborn repo FreshnessProbe = %q, %v; want a token", probe, err)
	}
}

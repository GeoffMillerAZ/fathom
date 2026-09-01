package tailwind

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCSSBuild_EndToEnd is the pipeline's conformance gate: the pinned
// Tailwind binary (fetched + checksum-verified on first run) builds real CSS
// from the fixture — Tailwind core, the generated kit theme, and utilities
// scanned out of a .templ file. This is "Tailwind works through the kit",
// observed rather than asserted.
//
// Network is needed once (binary download to the user cache); skipped under
// -short.
func TestCSSBuild_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end CSS build skipped in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	out := filepath.Join(t.TempDir(), "app.css")
	err := Run(
		ctx,
		"-i", filepath.Join("testdata", "fixture", "input.css"),
		"-o", out,
		"--minify",
	)
	if err != nil {
		t.Fatalf("tailwind build: %v", err)
	}

	css, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(css)

	for _, want := range []string{
		// theme variables made it through
		"--ak-color-surface",
		"--ak-color-accent",
		"[data-theme=dark]",
		"prefers-color-scheme:dark",
		// utilities were scanned from the .templ source and resolve
		// through the runtime variables (@theme inline)
		".bg-surface",
		".text-ink",
		".bg-accent",
		"var(--ak-color-accent)",
		// the dark: custom variant compiled
		".dark\\:border-line-strong",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("built CSS missing %q", want)
		}
	}
	if len(got) < 5_000 {
		t.Errorf("built CSS suspiciously small (%d bytes) — preflight/base layers missing?", len(got))
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

const appkitNavigationPreviewDirEnv = "APPKIT_NAVIGATION_PREVIEW_DIR"

var appkitNavigationPreviewActiveKeys = []string{
	"home",
	"fleet",
	"wiring",
	"impact",
	"story",
	"brief",
	"statusz",
}

// TestExportAppkitNavigationPreview exports the real Observatory shell around
// a fixed, inert body for each current-navigation state. It does not invoke
// route handlers, build a live model, start a listener, or read providers.
func TestExportAppkitNavigationPreview(t *testing.T) {
	outDir := os.Getenv(appkitNavigationPreviewDirEnv)
	if outDir == "" {
		t.Skip(appkitNavigationPreviewDirEnv + " is not set")
	}
	requireEmptyAppkitPreviewDir(t, outDir)

	model := syntheticModel()
	for _, theme := range appkitPreviewThemes {
		for _, activeKey := range appkitNavigationPreviewActiveKeys {
			body := `<section aria-labelledby="navigation-fixture-title">` +
				`<h2 id="navigation-fixture-title" class="text-lg font-semibold text-ink">Synthetic navigation state: ` + activeKey + `</h2>` +
				`<p class="mt-2 max-w-3xl text-ink-muted">This inert fixture isolates current-page navigation, skip-link focus, keyboard order, target size, wrapping, and responsive shell behavior. It contains no live Observatory evidence.</p>` +
				`<a class="mt-4 inline-flex min-h-11 items-center rounded-md px-3 py-2 text-accent underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="#fixture-next">Jump to the next fixture section</a>` +
				`<section id="fixture-next" tabindex="-1" class="mt-6 rounded-md border border-line bg-surface-raised p-4"><h3 class="font-semibold text-ink">Fixture destination</h3><p class="mt-2 text-ink-muted">The first content link lands here so focus and scroll behavior can be inspected without route navigation.</p></section>` +
				`</section>`
			page := inertAppkitPreviewHTML(obsPage("Navigation fixture: "+activeKey, activeKey, body, model), theme)
			assertInertAppkitPreview(t, theme, activeKey, page)
			writeAppkitPreviewFile(t, filepath.Join(outDir, theme, activeKey, "index.html"), []byte(page))
		}
		writeAppkitPreviewFile(t, filepath.Join(outDir, theme, "assets", "app.css"), obsAppCSS)
	}

	t.Logf("synthetic app-kit navigation previews exported to %s", outDir)
}

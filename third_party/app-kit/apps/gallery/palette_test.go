package gallery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/palette"
)

func paletteGet(t *testing.T, path string, fragment bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if fragment {
		req.Header.Set(palette.FragmentHeader, "1")
	}
	rec := httptest.NewRecorder()
	New().ServeHTTP(rec, req)
	return rec
}

func TestPaletteSpecimenMountsTriggerDialogAndScript(t *testing.T) {
	rec := paletteGet(t, "/palette?theme=dark", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-theme="dark"`,
		`<a href="/palette/search?theme=dark" data-palette-open="gallery-palette"`,
		`<dialog id="gallery-palette"`,
		`data-palette-action="/palette/search?theme=dark"`,
		`<input type="hidden" name="theme" value="dark">`,
		`data-palette-timeout="5000"`,
		"window.AppKitPalette",
		"interaction.command-palette",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("specimen missing %q", want)
		}
	}
	if strings.Count(body, "<h1") != 1 || strings.Count(body, `<dialog id="gallery-palette"`) != 1 {
		t.Error("specimen needs one h1 and one palette dialog")
	}
}

// The specimen has no header entry (the shared header has a byte budget); the
// Overlays page and the catalog detail's live-specimen link lead to it.
func TestPaletteSpecimenIsReachable(t *testing.T) {
	_, overlays := get(t, New(), "/overlays")
	if !strings.Contains(overlays, `href="/palette"`) {
		t.Error("Overlays page must link to the palette specimen")
	}
	_, detail := get(t, New(), "/catalog/interaction.command-palette?theme=dark")
	if !strings.Contains(detail, `href="/palette?theme=dark"`) {
		t.Error("catalog detail must open the live specimen")
	}
	if strings.Contains(detail, "Observed consumers") {
		t.Error("no consumer has integrated the palette; the detail must not list observed consumers")
	}
}

func TestPaletteSearchServesFragmentOrFullPage(t *testing.T) {
	frag := paletteGet(t, "/palette/search?q=overlay&theme=dark", true)
	if frag.Code != http.StatusOK || !strings.HasPrefix(frag.Body.String(), `<div data-palette-fragment data-palette-state="results"`) {
		t.Fatalf("fragment request: %d %.200s", frag.Code, frag.Body.String())
	}
	if !strings.Contains(frag.Header().Get("Vary"), palette.FragmentHeader) || frag.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("one URL serving two representations must vary on the header: %v", frag.Header())
	}
	fb := frag.Body.String()
	for _, want := range []string{`href="/overlays?theme=dark"`, `href="/catalog/interaction.overlays?theme=dark"`, "<h3", "Gallery pages", "Capabilities"} {
		if !strings.Contains(fb, want) {
			t.Errorf("fragment missing %q", want)
		}
	}
	if strings.Contains(fb, "app-kit Gallery") || strings.Contains(fb, "<html") {
		t.Error("fragment must not carry the page shell")
	}

	page := paletteGet(t, "/palette/search?q=overlay&theme=dark", false)
	pb := page.Body.String()
	if page.Code != http.StatusOK {
		t.Fatalf("page status %d", page.Code)
	}
	for _, want := range []string{
		"app-kit Gallery", `data-theme="dark"`, "<h1", `role="search"`, `value="overlay"`,
		`<input type="hidden" name="theme" value="dark">`, "<h2 ", `href="/overlays?theme=dark"`,
	} {
		if !strings.Contains(pb, want) {
			t.Errorf("no-JS page missing %q", want)
		}
	}
	if strings.Contains(pb, "<h3 class=\"px-3") {
		t.Error("the full page uses h2 group headings beneath its h1")
	}
}

func TestPaletteSearchStates(t *testing.T) {
	for _, tc := range []struct {
		query    string
		fragment bool
		status   int
		wants    []string
	}{
		{"", true, 200, []string{`data-palette-state="idle"`}},
		{"zzz", true, 200, []string{`data-palette-state="empty"`, "No results for “zzz”"}},
		{"partial", true, 200, []string{`data-palette-state="partial"`, "Partial results: "}},
		{"unsearched", true, 200, []string{`data-palette-state="partial"`, `data-palette-count="0"`, `data-palette-announce="No results for “unsearched” (partial)"`, "Matches may exist in sources that were not searched."}},
		{"long", true, 200, []string{"[overflow-wrap:anywhere]", "ReallyLongQualifiedSymbolNameWithoutSpaces"}},
		{"fail", true, 503, []string{"search failed"}},
		{"fail", false, 503, []string{`data-palette-state="error"`, "The synthetic search backend refused this query."}},
		{"zzz", false, 200, []string{`data-palette-state="empty"`, "app-kit Gallery"}},
	} {
		rec := paletteGet(t, "/palette/search?q="+tc.query, tc.fragment)
		if rec.Code != tc.status {
			t.Errorf("q=%q fragment=%v: status %d, want %d", tc.query, tc.fragment, rec.Code, tc.status)
		}
		for _, want := range tc.wants {
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("q=%q fragment=%v missing %q", tc.query, tc.fragment, want)
			}
		}
	}
	if rec := paletteGet(t, "/palette/search/extra", false); rec.Code != http.StatusNotFound {
		t.Errorf("unknown subpath: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/palette/search", nil)
	rec := httptest.NewRecorder()
	New().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", rec.Code)
	}
}

// A client that aborts a slow request (palette.js cancelling a stale query)
// must release the handler instead of holding it for the synthetic delay.
func TestPaletteSlowSearchStopsWhenTheClientAborts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/palette/search?q=slow", nil).WithContext(ctx)
	req.Header.Set(palette.FragmentHeader, "1")
	rec := httptest.NewRecorder()
	start := time.Now()
	New().ServeHTTP(rec, req)
	if elapsed := time.Since(start); elapsed > paletteSlowDelay/2 {
		t.Fatalf("aborted request held the handler for %v", elapsed)
	}
	if rec.Body.Len() != 0 {
		t.Error("an aborted request must not write a fragment")
	}
}

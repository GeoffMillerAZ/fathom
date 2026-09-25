package gallery

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestThemeWorkbenchAllNamedIdentitiesAndSpecimens(t *testing.T) {
	h := New()
	for _, palette := range []string{"appkit", "synthwave", "copper", "lagoon", "orchard", "geoff-indigo", "geoff-synthwave"} {
		for _, specimen := range []string{"tokens", "forms", "components", "dashboard"} {
			for _, scheme := range []string{"", "light", "dark"} {
				q := url.Values{"palette": {palette}, "specimen": {specimen}, "theme": {scheme}, "effect": {"none"}}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", "/themes?"+q.Encode(), nil))
				if w.Code != 200 {
					t.Fatalf("%s: status %d", q.Encode(), w.Code)
				}
				body := w.Body.String()
				if strings.Count(body, "<main") != 1 || !strings.Contains(body, `aria-label="Selected kit specimen"`) || !strings.Contains(body, `action="/themes"`) || !strings.Contains(body, `value="`+palette+`" selected`) || !strings.Contains(body, `value="`+specimen+`" selected`) {
					t.Fatal("selection or actual specimen missing", q.Encode())
				}
				if palette != "appkit" && !strings.Contains(body, `href="/static/themes/`+palette+`.css"`) {
					t.Fatal("selected theme stylesheet absent", palette)
				}
				if palette == "appkit" && strings.Contains(body, `href="/static/themes/`) {
					t.Fatal("default loaded an unwanted palette")
				}
				if strings.Contains(body, "geoff-neon.css") {
					t.Fatal("optional effect ignored none")
				}
				if (palette == "synthwave" || palette == "geoff-indigo" || palette == "geoff-synthwave") && !strings.Contains(body, "dark-only identity") {
					t.Fatal("invented a light brand variant")
				}
				if (palette == "copper" || palette == "lagoon" || palette == "orchard") && strings.Contains(body, "dark-only identity") {
					t.Fatal("light-and-dark identity was incorrectly described as dark-only")
				}
				if w.Header().Get("Set-Cookie") != "" || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("preview saved preferences or allowed stale selection")
				}
				for _, choice := range []string{"light", "dark"} {
					q.Set("theme", choice)
					// html/template escapes ampersands in attributes.
					if !strings.Contains(body, strings.ReplaceAll("/themes?"+q.Encode(), "&", "&amp;")) {
						t.Fatal("scheme link lost palette/specimen/effect")
					}
				}
			}
		}
	}
}

func TestThemeWorkbenchDefaultNeonAndAssetBoundaries(t *testing.T) {
	h := New()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/themes", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Geoff Synthwave") || !strings.Contains(w.Body.String(), "ak-neon-title") || !strings.Contains(w.Body.String(), `href="/static/themes/geoff-neon.css"`) {
		t.Fatal("primary requested identity/effect missing")
	}
	for _, name := range []string{"synthwave", "copper", "lagoon", "orchard", "geoff-indigo", "geoff-synthwave", "geoff-neon"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/static/themes/"+name+".css", nil))
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/css") || strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Fatal("named unversioned asset not revalidatable", name)
		}
	}
	for _, path := range []string{"/themes?palette=unknown", "/themes?palette%7Ctheme=ignored", "/themes?palette=appkit&palette=synthwave", "/themes?effect=animate", "/themes?theme=synthwave", "/themes?specimen=private", "/themes?css=https://example.com/a.css", "/themes?theme=%xx", "/%74hemes"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 400 {
			t.Errorf("invalid selection %s: status %d", path, w.Code)
		}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/themes", nil))
	if w.Code != 405 {
		t.Fatal("workbench unexpectedly accepts writes")
	}
	// Optional palette preview must not silently restyle ordinary Gallery pages.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/catalog", nil))
	if strings.Contains(w.Body.String(), `href="/static/themes/`) {
		t.Fatal("workbench palette leaked into default Gallery")
	}
}

func TestGalleryUtilitiesKeepRuntimeFontAndRadiusBindings(t *testing.T) {
	css, err := assets.ReadFile("ui/app.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"font-family:var(--ak-font-sans)", "border-radius:var(--ak-radius-md)"} {
		if !strings.Contains(string(css), want) {
			t.Errorf("compiled Gallery CSS lacks %s; rebuild gallery CSS", want)
		}
	}
}

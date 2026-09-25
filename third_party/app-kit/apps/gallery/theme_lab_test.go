package gallery

import (
	"os"
	"strings"
	"testing"
)

func TestThemeLabUsesRealGalleryRenderersWithoutChangingOrdinaryPages(t *testing.T) {
	source, err := os.ReadFile("../../cue/tokens/geoff-synthwave/geoff-synthwave.cue")
	if err != nil {
		t.Fatal(err)
	}
	lab, err := NewThemeLab("http://127.0.0.1:8765", source, "synthetic-source-label", "synthetic-local-kit-identity")
	if err != nil {
		t.Fatal(err)
	}
	ordinary := New()
	h := NewWithOptions(Options{ThemeLab: lab})
	if w := referenceRequest(ordinary, "GET", "/theme-lab", nil); w.Code != 404 {
		t.Fatal("lab unexpectedly mounted without source")
	}
	if w := referenceRequest(ordinary, "GET", "/themes", nil); strings.Contains(w.Body.String(), `href="/theme-lab"`) {
		t.Fatal("disabled lab advertised as mounted")
	}
	if w := referenceRequest(h, "GET", "/themes", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `href="/theme-lab"`) {
		t.Fatal("enabled lab missing")
	}
	for _, tc := range []struct {
		id    string
		build func() (string, error)
	}{
		{"dashboard", func() (string, error) { v, e := buildDashboard(); return string(v), e }},
		{"components", func() (string, error) { v, e := buildComponents(); return string(v), e }},
		{"forms", func() (string, error) { v, e := buildForms(); return string(v), e }},
	} {
		expected, err := tc.build()
		if err != nil {
			t.Fatal(err)
		}
		for _, scheme := range []string{"light", "dark"} {
			w := referenceRequest(h, "GET", "/theme-lab/preview?specimen="+tc.id+"&scheme="+scheme, nil)
			if w.Code != 200 || !strings.Contains(w.Body.String(), expected) || !strings.Contains(w.Body.String(), "#f92aad") {
				t.Fatalf("%s/%s not rendered with selected source: %d", tc.id, scheme, w.Code)
			}
			if strings.Count(w.Body.String(), "<main") != 1 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "font-src 'none'") {
				t.Fatal("specimen boundary changed")
			}
		}
	}
	for _, route := range []string{"/dashboard", "/forms", "/catalog"} {
		before := referenceRequest(ordinary, "GET", route, nil)
		after := referenceRequest(h, "GET", route, nil)
		if before.Code != after.Code || before.Body.String() != after.Body.String() {
			t.Fatal("lab changed ordinary app output", route)
		}
	}
}

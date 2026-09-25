package geoffsynthwave

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNeonIsAnOptionalStaticSemanticEffect(t *testing.T) {
	css := NeonCSS()
	for _, want := range []string{".ak-neon-title", "var(--ak-color-ink)", "@media print", "forced-colors: active", "prefers-contrast: more", "text-shadow: none"} {
		if !strings.Contains(css, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, forbidden := range []string{"@keyframes", "animation:", "transition:", "url(", "#ffffff", "h1 {", "body {", "filter:"} {
		if strings.Contains(css, forbidden) {
			t.Errorf("effect escaped its static semantic boundary: %s", forbidden)
		}
	}
	if strings.Contains(CSS(), "text-shadow") {
		t.Fatal("effect became mandatory in palette")
	}
	w := httptest.NewRecorder()
	NeonHandler().ServeHTTP(w, httptest.NewRequest("GET", "/neon.css", nil))
	if w.Code != 200 || w.Body.String() != css || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatal("effect delivery drift")
	}
}

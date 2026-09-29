package geoffsynthwave

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

func sourceTokens(t *testing.T) tokens.Tokens {
	t.Helper()
	source, err := os.ReadFile("../../../../cue/tokens/geoff-synthwave/geoff-synthwave.cue")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestGeneratedCSSMatchesSource(t *testing.T) {
	tok := sourceTokens(t)
	if got, want := CSS(), tokens.EmitThemeCSS(tok); got != want {
		t.Fatal("embedded CSS differs from current CUE source/emitter")
	}
	if tok.Name != "geoff-synthwave" || tok.Fonts.Sans != "Inter, system-ui, sans-serif" {
		t.Fatal("reference identity or committed font declaration changed")
	}
	for _, forbidden := range []string{"@import", "url(", "@font-face", "text-shadow", "@keyframes", "h1 {", "#818cf8", "#7c4dff"} {
		if strings.Contains(CSS(), forbidden) {
			t.Errorf("token-only source palette contains %q", forbidden)
		}
	}
}

func TestRequiredRolesAndDarkOnly(t *testing.T) {
	tok := sourceTokens(t)
	want := map[string]string{
		"surface": "#241b2f", "surface-raised": "#241b2f", "surface-sunken": "#191321", "surface-overlay": "#241b2f",
		"ink": "#ffffff", "ink-muted": "#c7c7c7", "ink-faint": "#c7c7c7", "ink-inverse": "#241b2f",
		"line": "#34464a", "line-strong": "#4b8674",
		"accent": "#f92aad", "accent-hover": "#ff7edb", "accent-ink": "#241b2f", "accent-soft": "#2f152f", "focus": "#72f1b8",
		"positive": "#72f1b8", "positive-soft": "#222930", "caution": "#fede5d", "caution-soft": "#302727",
		"danger": "#f92aad", "danger-soft": "#2f152f", "info": "#36f9f6", "info-soft": "#1c2a36",
		// The site has no modal veil color; Load supplies the kit scrim for a
		// dark-polarity branch (FATHOM-S0-KIT-02), identical in both schemes.
		"scrim": "oklch(0.10 0.010 260)",
	}
	if len(tok.Colors) != len(want) {
		t.Fatal("theme role inventory changed")
	}
	for _, role := range tokens.ColorNames {
		if _, ok := want[role]; !ok {
			t.Errorf("required kit role %q is not explicitly mapped", role)
		}
	}
	for role, value := range want {
		if got := tok.Colors[role]; got.Light != value || got.Dark != value {
			t.Errorf("%s = %+v, want both schemes %s", role, got, value)
		}
		if strings.Count(CSS(), "--ak-color-"+role+": "+value+";") != 4 {
			t.Errorf("%s missing from root/dark/light/OS branches", role)
		}
	}
	if strings.Contains(CSS(), "color-scheme: light;") || strings.Count(CSS(), "color-scheme: dark;") != 4 {
		t.Fatal("dark-only surface polarity changed")
	}
}

func TestChosenContrastPairs(t *testing.T) {
	tok := sourceTokens(t)
	check := func(fg, bg, scheme string, floor float64) {
		t.Helper()
		ratio, err := tokens.ContrastRoles(tok, fg, bg, scheme)
		if err != nil || ratio < floor {
			t.Errorf("%s/%s (%s) = %.2f:1, need %.1f:1; error=%v", fg, bg, scheme, ratio, floor, err)
		}
	}
	for _, scheme := range []string{"light", "dark"} {
		for _, bg := range []string{"surface", "surface-raised", "surface-sunken", "surface-overlay"} {
			for _, fg := range []string{"ink", "ink-muted", "ink-faint", "accent", "accent-hover", "positive", "caution", "danger", "info"} {
				check(fg, bg, scheme, tokens.TextFloor)
			}
			for _, fg := range []string{"line-strong", "focus"} {
				check(fg, bg, scheme, tokens.MeaningFloor)
			}
		}
		for _, bg := range []string{"accent", "accent-hover"} {
			check("accent-ink", bg, scheme, tokens.TextFloor)
		}
		for _, role := range []string{"accent", "positive", "caution", "danger", "info"} {
			check(role, role+"-soft", scheme, tokens.TextFloor)
			check("ink", role+"-soft", scheme, tokens.TextFloor)
		}
	}
	// White-neon text is preserved for the canvas, not reused on bright buttons.
	if ratio, err := tokens.Contrast("#ffffff", tok.Colors["accent"].Dark); err != nil || ratio >= tokens.TextFloor {
		t.Fatal("the documented dark accent-ink adaptation lost its stated basis")
	}
}

func TestCSSHandlerRevalidates(t *testing.T) {
	handler := CSSHandler()
	wantETag := fmt.Sprintf("\"%x\"", sha256.Sum256([]byte(CSS())))
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, "/theme.css", nil))
		if response.Code != http.StatusOK || response.Header().Get("ETag") != wantETag || response.Header().Get("Content-Type") != "text/css; charset=utf-8" || response.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s did not serve current token identity/type/revalidation headers", method)
		}
		if method == http.MethodGet && response.Body.String() != CSS() || method == http.MethodHead && response.Body.Len() != 0 {
			t.Errorf("%s body differs from its method contract", method)
		}
		request := httptest.NewRequest(method, "/theme.css", nil)
		request.Header.Set("If-None-Match", wantETag)
		unchanged := httptest.NewRecorder()
		handler.ServeHTTP(unchanged, request)
		if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 || unchanged.Header().Get("ETag") != wantETag {
			t.Errorf("%s matching validator did not return an empty 304", method)
		}
	}
	stale := httptest.NewRequest(http.MethodGet, "/theme.css", nil)
	stale.Header.Set("If-None-Match", `"old-theme"`)
	changed := httptest.NewRecorder()
	handler.ServeHTTP(changed, stale)
	if changed.Code != http.StatusOK || changed.Body.String() != CSS() {
		t.Fatal("stale validator did not receive current bytes")
	}
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/theme.css", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" || post.Body.String() == CSS() {
		t.Fatal("non-read method was not refused")
	}
}

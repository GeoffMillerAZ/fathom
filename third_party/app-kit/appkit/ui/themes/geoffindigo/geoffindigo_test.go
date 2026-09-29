package geoffindigo

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
	source, err := os.ReadFile("../../../../cue/tokens/geoff-indigo/geoff-indigo.cue")
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
		t.Fatal("embedded CSS differs from the current CUE source/emitter")
	}
	if tok.Name != "geoff-indigo" || tok.Fonts.Sans != "Inter, system-ui, sans-serif" {
		t.Fatal("theme identity or declared font changed")
	}
	for _, forbidden := range []string{"@import", "url(", "@font-face", "<script", "#f92aad"} {
		if strings.Contains(CSS(), forbidden) {
			t.Errorf("token-only current-site stylesheet contains %q", forbidden)
		}
	}
}

func TestRequiredRolesAndDarkOnly(t *testing.T) {
	tok := sourceTokens(t)
	want := map[string]string{
		"surface": "#111827", "surface-raised": "#1f2937", "surface-sunken": "#111827", "surface-overlay": "#1f2937",
		"ink": "#f9fafb", "ink-muted": "#9ca3af", "ink-faint": "#9ca3af", "ink-inverse": "#111827",
		"line": "#374151", "line-strong": "#6b7280",
		"accent": "#818cf8", "accent-hover": "#a5b4fc", "accent-ink": "#111827", "accent-soft": "#19203b", "focus": "#6366f1",
		"positive": "#4ade80", "positive-soft": "#052e16", "caution": "#fcd34d", "caution-soft": "#451a03",
		"danger": "#f87171", "danger-soft": "#450a0a", "info": "#38bdf8", "info-soft": "#082f49",
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
			t.Errorf("%s missing from one of root/dark/light/OS branches", role)
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
	// The source gray-500 label cannot replace semantic faint text on a panel.
	if ratio, err := tokens.Contrast("#6b7280", tok.Colors["surface-raised"].Dark); err != nil || ratio >= tokens.TextFloor {
		t.Fatal("the documented gray-500 text adaptation no longer has its stated basis")
	}
}

func TestCSSHandlerRevalidates(t *testing.T) {
	handler := CSSHandler()
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/theme.css", nil))
	wantETag := fmt.Sprintf("\"%x\"", sha256.Sum256([]byte(CSS())))
	if get.Code != http.StatusOK || get.Body.String() != CSS() || get.Header().Get("ETag") != wantETag {
		t.Fatal("GET did not serve the exact current token artifact with its content identity")
	}
	if get.Header().Get("Content-Type") != "text/css; charset=utf-8" || get.Header().Get("X-Content-Type-Options") != "nosniff" || get.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
		t.Fatal("stylesheet response headers do not preserve revalidation and type boundaries")
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequest(method, "/theme.css", nil)
		request.Header.Set("If-None-Match", wantETag)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotModified || response.Body.Len() != 0 || response.Header().Get("ETag") != wantETag {
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
	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/theme.css", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("ETag") != wantETag {
		t.Fatal("HEAD response differs from the GET identity or includes a body")
	}
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/theme.css", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" || post.Body.String() == CSS() {
		t.Fatal("non-read method was not refused")
	}
}

package themes_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/copper"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/lagoon"
	"github.com/geoffmilleraz/app-kit/appkit/ui/themes/orchard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

type firstPartyTheme struct {
	name, source string
	css          func() string
	handler      func() http.Handler
}

var firstPartyThemes = []firstPartyTheme{
	{name: "copper", source: "../../../cue/tokens/copper/copper.cue", css: copper.CSS, handler: copper.CSSHandler},
	{name: "lagoon", source: "../../../cue/tokens/lagoon/lagoon.cue", css: lagoon.CSS, handler: lagoon.CSSHandler},
	{name: "orchard", source: "../../../cue/tokens/orchard/orchard.cue", css: orchard.CSS, handler: orchard.CSSHandler},
}

func TestFirstPartyThemesRegenerateAndKeepSemanticContrast(t *testing.T) {
	for _, theme := range firstPartyThemes {
		t.Run(theme.name, func(t *testing.T) {
			source, err := os.ReadFile(theme.source)
			if err != nil {
				t.Fatal(err)
			}
			tok, err := tokens.Load(source)
			if err != nil {
				t.Fatal(err)
			}
			if tok.Name != theme.name || theme.css() != tokens.EmitThemeCSS(tok) {
				t.Fatal("embedded CSS differs from its named CUE source or emitter")
			}
			if strings.Contains(theme.css(), "@import") || strings.Contains(theme.css(), "url(") || strings.Contains(theme.css(), "@font-face") {
				t.Fatal("first-party token CSS unexpectedly loads an external asset")
			}

			for _, scheme := range []string{"light", "dark"} {
				for _, surface := range []string{"surface", "surface-raised", "surface-sunken", "surface-overlay"} {
					for _, role := range []string{"ink", "ink-muted", "ink-faint"} {
						checkContrast(t, tok, scheme, role, surface, tokens.TextFloor, "text")
					}
					for _, role := range []string{"accent", "positive", "caution", "danger", "info", "focus", "line-strong"} {
						checkContrast(t, tok, scheme, role, surface, tokens.MeaningFloor, "status/chart/focus")
					}
				}
				for _, control := range []string{"accent", "accent-hover"} {
					checkContrast(t, tok, scheme, "accent-ink", control, tokens.TextFloor, "control label")
				}
				for _, role := range []string{"accent", "positive", "caution", "danger", "info"} {
					checkContrast(t, tok, scheme, role, role+"-soft", tokens.TextFloor, "status text")
					checkContrast(t, tok, scheme, "ink", role+"-soft", tokens.TextFloor, "status context")
				}
			}
		})
	}
}

func checkContrast(t *testing.T, tok tokens.Tokens, scheme, fg, bg string, floor float64, use string) {
	t.Helper()
	ratio, err := tokens.ContrastRoles(tok, fg, bg, scheme)
	if err != nil || ratio < floor {
		t.Errorf("%s %s/%s (%s) = %.2f:1, need %.1f:1; error=%v", use, fg, bg, scheme, ratio, floor, err)
	}
}

func TestFirstPartyThemeHandlersServeRevalidatableEmbeddedCSS(t *testing.T) {
	for _, theme := range firstPartyThemes {
		t.Run(theme.name, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				response := httptest.NewRecorder()
				theme.handler().ServeHTTP(response, httptest.NewRequest(method, "/theme.css", nil))
				if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/css; charset=utf-8" || response.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("ETag") == "" {
					t.Fatalf("%s handler headers/status changed: %d %#v", method, response.Code, response.Header())
				}
				if method == http.MethodGet && response.Body.String() != theme.css() || method == http.MethodHead && response.Body.Len() != 0 {
					t.Fatalf("%s handler body differs from method contract", method)
				}
				cached := httptest.NewRecorder()
				request := httptest.NewRequest(method, "/theme.css", nil)
				request.Header.Set("If-None-Match", response.Header().Get("ETag"))
				theme.handler().ServeHTTP(cached, request)
				if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
					t.Fatalf("%s matching ETag = %d, want empty 304", method, cached.Code)
				}
			}
			refused := httptest.NewRecorder()
			theme.handler().ServeHTTP(refused, httptest.NewRequest(http.MethodPost, "/theme.css", nil))
			if refused.Code != http.StatusMethodNotAllowed || refused.Header().Get("Allow") != "GET, HEAD" || strings.Contains(refused.Body.String(), theme.css()) {
				t.Fatal("non-read request was not safely refused")
			}
		})
	}
}

package synthwave

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

// TestCSSCarriesTheTokenVocabulary — the assertion that matters is not "the
// embed works" but "the embed carries a real token set". A truncated or
// wrong-file embed would still compile and still be non-empty.
func TestCSSCarriesTheTokenVocabulary(t *testing.T) {
	css := CSS()
	if css == "" {
		t.Fatal("CSS() is empty — the //go:embed did not pick up theme.css")
	}
	for _, want := range []string{
		"--ak-color-surface",   // the role every consumer aliases first
		"--ak-color-neon-pink", // synthwave's open role; proves it is THIS token set
		"@theme inline",        // the Tailwind v4 mapping block
	} {
		if !strings.Contains(css, want) {
			t.Errorf("synthwave CSS does not contain %q", want)
		}
	}
}

// TestCSSHasNoGoSideEffects — L1 means plain CSS. If this file ever starts
// templating the stylesheet, the theme stops being consumable by a <link> and
// the layer claim in docs/ui-components.md becomes false.
func TestCSSHasNoGoSideEffects(t *testing.T) {
	if CSS() != CSS() {
		t.Fatal("CSS() is not pure")
	}
	if strings.Contains(CSS(), "%!") || strings.Contains(CSS(), "{{") {
		t.Error("synthwave CSS looks templated; it must be the committed generated file, verbatim")
	}
}

// TestCSSMatchesTheCommittedArtifact — the embedded stylesheet must be exactly
// what `task gen:synthwave` produces from cue/tokens/synthwave/synthwave.cue.
// Nothing gates this today: `task check`'s regeneration-cleanliness diff only
// watches appkit/ui/tokens and appkit/cli/theme/palette_gen.go, so a synthwave
// theme.css that drifts from its generator — e.g. predating the emitter's
// symmetric [data-theme="light"] block (H1) — ships silently. This test is
// the gate `task check` does not run.
func TestCSSMatchesTheCommittedArtifact(t *testing.T) {
	src, err := os.ReadFile("../../../../cue/tokens/synthwave/synthwave.cue")
	if err != nil {
		t.Fatalf("read synthwave.cue: %v", err)
	}
	tok, err := tokens.Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := CSS(), tokens.EmitThemeCSS(tok); got != want {
		t.Error("appkit/ui/themes/synthwave/theme.css is stale relative to cue/tokens/synthwave/synthwave.cue — run `task gen:synthwave`")
	}
}

func TestCSSHandlerServesTheStylesheet(t *testing.T) {
	rec := httptest.NewRecorder()
	CSSHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/theme.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
	if rec.Body.String() != CSS() {
		t.Error("handler body differs from CSS()")
	}
}

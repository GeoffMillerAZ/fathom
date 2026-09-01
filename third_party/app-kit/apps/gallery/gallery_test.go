package gallery

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

func get(t *testing.T, h http.Handler, path string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	return res, rec.Body.String()
}

func TestPagesServe(t *testing.T) {
	h := New()
	for _, p := range allRoutes() {
		res, body := get(t, h, p)
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, res.StatusCode)
		}
		if !strings.Contains(body, "app-kit Gallery") {
			t.Errorf("GET %s missing shell chrome", p)
		}
	}
}

// TestLayoutPageShowsCompositionPrimitives ensures the unified composition
// packages (card, kanban, table) actually appear in the living style guide.
func TestLayoutPageShowsCompositionPrimitives(t *testing.T) {
	_, body := get(t, New(), "/layout")
	for _, want := range []string{
		"card.Card", "kanban.Board", "table.Table", "appkit/ui/router",
		"data-kanban-col", "data-table", "Security deposit return",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout page missing %q", want)
		}
	}
}

// TestAgenticPageShowsHITL ensures the human-in-the-loop surfaces (approval card,
// run trace) and chrome comforts appear in the living style guide.
func TestAgenticPageShowsHITL(t *testing.T) {
	_, body := get(t, New(), "/agentic")
	for _, want := range []string{
		"hitl.ApprovalCard", "hitl.RunTrace", "appkit/ui/chrome",
		"Send the deposit-demand letter", "Awaiting human approval", "aria-current",
		"<details", "Show 3 more elements", "<dl",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("agentic page missing %q", want)
		}
	}
}

// TestFormsPageShowsControls ensures the form vocabulary appears in the style guide.
func TestFormsPageShowsControls(t *testing.T) {
	_, body := get(t, New(), "/forms")
	for _, want := range []string{
		"appkit/ui/form", "Case name", `type="text"`, "<select", "<textarea",
		`type="checkbox"`, "<fieldset", "aria-invalid", "Enter an amount greater than zero.",
		"Save changes", "Delete case",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("forms page missing %q", want)
		}
	}
}

// TestOverlaysPageShowsPrimitives ensures the overlay primitives + script appear.
func TestOverlaysPageShowsPrimitives(t *testing.T) {
	_, body := get(t, New(), "/overlays")
	for _, want := range []string{
		"<dialog id=\"demo-dialog\"", "<dialog id=\"demo-drawer\"",
		"data-overlay-open=\"demo-dialog\"", "AppKitOverlay", "ak-toast-region",
		"Delete this case?", "Evidence provenance",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("overlays page missing %q", want)
		}
	}
}

// TestMotionPageShowsBothStates — the Motion page is the living specimen of
// appkit/ui/motion, so it must actually carry the two-state vocabulary (root +
// state classes), render both end states statically (the no-motion baseline the
// invariant requires), and ship the optional motion.js helper the live toggle
// drives. A page that lost the static state-B specimen or the state classes
// would document the wrong thing.
func TestMotionPageShowsBothStates(t *testing.T) {
	// Pin to buildMotion's own output, not the served page: the shell also
	// carries the nav (which now links "/motion") and every other route's
	// label, so a route-level Contains could keep passing after a specimen were
	// deleted. Asserting against the builder means these strings can only come
	// from the motion specimens themselves.
	content, err := buildMotion()
	if err != nil {
		t.Fatalf("buildMotion: %v", err)
	}
	body := string(content)
	for _, want := range []string{
		"appkit/ui/motion",
		"ak-motion", "ak-state-b", // root + state selector (state B is the static B specimen)
		"ak-when-a", "ak-when-b", // both state layers present
		"ak-fade", "ak-tint", // the interpolation presets
		"AppKitMotion", // the optional motion.js helper is embedded
		"no <code class=\"font-mono\">@source</code> entry", // the L2 contract the page proves
	} {
		if !strings.Contains(body, want) {
			t.Errorf("motion page missing %q", want)
		}
	}
}

func TestUnknownPath404s(t *testing.T) {
	res, _ := get(t, New(), "/nope")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("GET /nope = %d, want 404", res.StatusCode)
	}
}

// TestTokensPageRendersEveryRole ensures the style guide actually documents the
// whole generated vocabulary — every color role, radius, and font shows up as
// its utility. If a token is added in CUE, this page must grow with it.
func TestTokensPageRendersEveryRole(t *testing.T) {
	_, body := get(t, New(), "/tokens")
	for _, r := range tokens.ColorNames {
		if !strings.Contains(body, "bg-"+r) {
			t.Errorf("tokens page missing swatch for role %q", r)
		}
	}
	for _, r := range tokens.RadiusNames {
		if !strings.Contains(body, "rounded-"+r) {
			t.Errorf("tokens page missing radius rounded-%s", r)
		}
	}
	for _, f := range tokens.FontNames {
		if !strings.Contains(body, "font-"+f) {
			t.Errorf("tokens page missing font font-%s", f)
		}
	}
}

func TestComponentsPageRendersEveryViz(t *testing.T) {
	_, body := get(t, New(), "/components")
	for _, name := range []string{"Sparkline", "StackBar", "Donut", "Funnel", "MeterRow"} {
		if !strings.Contains(body, name) {
			t.Errorf("components page missing %s", name)
		}
	}
	// The SVG output must actually be embedded, not just the names.
	if !strings.Contains(body, "<svg") {
		t.Error("components page rendered no SVG")
	}
	if c := strings.Count(body, "role=\"img\""); c < 5 {
		t.Errorf("expected >=5 accessible SVGs (role=img), got %d", c)
	}
}

func TestThemeContract(t *testing.T) {
	h := New()
	// Forced dark.
	_, dark := get(t, h, "/?theme=dark")
	if !strings.Contains(dark, `data-theme="dark"`) {
		t.Error("?theme=dark did not set data-theme=dark")
	}
	// OS-follow: no data-theme attribute.
	_, osFollow := get(t, h, "/")
	if strings.Contains(osFollow, "data-theme=") {
		t.Error("default page should not pin data-theme (OS-follow)")
	}
	// Garbage theme is ignored (treated as OS-follow), never reflected.
	_, junk := get(t, h, "/?theme=evil\"><script>")
	if strings.Contains(junk, "data-theme=\"evil") || strings.Contains(junk, "<script>") {
		t.Error("invalid theme value was not sanitized")
	}
}

func TestCSSServed(t *testing.T) {
	res, body := get(t, New(), "/static/app.css")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/app.css = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("css content-type = %q", ct)
	}
	for _, want := range []string{".bg-accent", "--ak-color-ink", ".rounded-lg"} {
		if !strings.Contains(body, want) {
			t.Errorf("built css missing %q — rebuild with `task gallery:css`", want)
		}
	}
}

// TestInputCSSCoversTokenVocabulary guards the @source inline(...) safelist in
// input.css against drift: every generated token name must be force-generated,
// or the rendered (dynamically-built) utility would be missing from app.css.
func TestInputCSSCoversTokenVocabulary(t *testing.T) {
	src, err := os.ReadFile("ui/input.css")
	if err != nil {
		t.Fatalf("read input.css: %v", err)
	}
	css := string(src)
	for _, r := range tokens.ColorNames {
		// Names appear inside the brace-expansion list, comma-delimited.
		if !strings.Contains(css, r) {
			t.Errorf("input.css @source inline omits color role %q", r)
		}
	}
	for _, r := range tokens.RadiusNames {
		if !strings.Contains(css, r) {
			t.Errorf("input.css @source inline omits radius %q", r)
		}
	}
	for _, f := range tokens.FontNames {
		if !strings.Contains(css, f) {
			t.Errorf("input.css @source inline omits font %q", f)
		}
	}
}

// allRoutes is the single list every whole-Gallery assertion walks. It is
// derived from the nav rather than typed out, so a page added to the catalog
// cannot quietly escape the coverage tests below — which is how a page ships
// with an unsourced figure on it.
func allRoutes() []string {
	out := make([]string, 0, len(nav))
	for _, l := range nav {
		href, _, _ := strings.Cut(l.Href, "?")
		out = append(out, href)
	}
	return out
}

// TestProvenancePageShowsEveryStrength — the Provenance page is the kit's
// public statement of the rule (ADR-0017 §6), so it has to actually contain all
// three enforcement strengths, every Kind label, the draft flag and the
// mechanism waiver. A page that quietly lost the mechanism specimen would leave
// the escape hatch undocumented, and an undocumented escape hatch gets used
// wrong or not at all.
// It asserts against buildProvenance's own output, not the served page. The
// difference is not pedantry: the served page also contains the nav, the theme
// CSS and every other route's label, so a route-level Contains() would keep
// passing if a specimen were deleted and the string happened to survive
// somewhere else in the shell. Pinning to the builder means these strings can
// only come from the specimens themselves.
func TestProvenancePageShowsEveryStrength(t *testing.T) {
	content, err := buildProvenance()
	if err != nil {
		t.Fatalf("buildProvenance: %v", err)
	}
	body := string(content)
	for _, want := range []string{
		"appkit/ui/vizframe",
		"Validate()", "vizframe.Render(", "vizframe.MustRender(",
		">Source<", ">Derived<", ">Estimate<", // the three printed kind labels
		`data-viz-source="published"`, `data-viz-source="derived"`, `data-viz-source="estimate"`,
		`data-viz-draft="true"`, `data-viz-carries="mechanism"`,
		"CNCF Annual Survey 2024",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("provenance page missing %q", want)
		}
	}
	// The refusal specimen must genuinely be empty output, not a picture of
	// emptiness. If Render ever starts emitting a placeholder, this catches it.
	if strings.Contains(body, ">Unsourced figure<") {
		t.Error("the refusal specimen rendered a frame; Render must return \"\" for a figure with no source")
	}
}

// TestEveryGalleryFigureCarriesASource — the kit holding itself to its own rule
// in public before it asks anyone else to. This is a property of a PAGE, not of
// a call: Render returning "" is silent by design, so nothing at the call site
// can tell you a figure went missing, and nothing at the call site can tell you
// a figure shipped with a mechanism waiver it did not deserve.
func TestEveryGalleryFigureCarriesASource(t *testing.T) {
	h := New()
	frames := 0
	for _, route := range allRoutes() {
		_, body := get(t, h, route)
		for _, frame := range vizframes(body) {
			frames++
			mechanism := strings.Contains(frame, `data-viz-carries="mechanism"`)
			sourced := strings.Contains(frame, "data-viz-source=")
			switch {
			case sourced:
				// A source line was rendered; check it is legible rather than
				// merely present (hazard H6).
				if strings.Contains(frame, "text-ink-faint") {
					t.Errorf("%s: a frame sets part of itself in ink-faint (3.89:1 on surface-raised, below the 4.5 text floor)", route)
				}
			case mechanism:
				// Legitimately unsourced: the body states no quantity. The
				// waiver is visible right here in the DOM, which is the point.
			default:
				t.Errorf("%s: a figure carries no source and claims no mechanism waiver:\n  %.240s", route, frame)
			}
		}
	}
	if frames == 0 {
		t.Fatal("no ak-vizframe figures found on any Gallery route — this test is asserting nothing")
	}
}

// vizframes returns each <figure class="ak-vizframe …"> element in body, up to
// its closing tag. Deliberately a scanner rather than an HTML parser: the
// assertion is about a string the kit itself emits, and a parser would add a
// dependency to prove less.
func vizframes(body string) []string {
	const open = `<figure class="ak-vizframe`
	var out []string
	for i := 0; ; {
		j := strings.Index(body[i:], open)
		if j < 0 {
			return out
		}
		start := i + j
		end := strings.Index(body[start:], "</figure>")
		if end < 0 {
			return append(out, body[start:])
		}
		out = append(out, body[start:start+end+len("</figure>")])
		i = start + end
	}
}

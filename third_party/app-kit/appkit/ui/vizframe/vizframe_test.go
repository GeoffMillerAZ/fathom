package vizframe

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

// ---- golden file helpers ---------------------------------------------------
//
// Same contract as appkit/ui/viz (viz_test.go): testdata/<name>.<ext>.golden,
// regenerated with UPDATE_GOLDEN=1 go test ./... . The extension differs
// because a frame is HTML, not SVG.

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".html.golden")
}

// assertGolden compares got against the stored golden file.
// If UPDATE_GOLDEN=1 is set in the environment, it writes/updates the file
// instead of asserting. Run `UPDATE_GOLDEN=1 go test ./...` to regenerate.
func assertGolden(t *testing.T, got, name string) {
	t.Helper()
	path := goldenPath(name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 go test to generate)", path, err)
	}
	if string(want) != got {
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, string(want))
	}
}

// ---- fixtures --------------------------------------------------------------

// bodyFixture is a real viz component's output, so the goldens prove the frame
// composes with the existing package without either one changing.
func bodyFixture() string {
	return viz.Bars([]viz.Bar{
		{Label: "platform", Value: 48, Role: "accent", Suffix: "%"},
		{Label: "data", Value: 31, Role: "info", Suffix: "%"},
		{Label: "security", Value: 21, Role: "positive", Suffix: "%"},
	}, viz.BarsOptions{Max: 100, ShowValue: true, AriaLabel: "spend share by team"})
}

func optsFixture() Options {
	return Options{
		Title:  "Spend share by team",
		Note:   "FY25, all environments",
		Source: Derived("Sum of the FY25 billing export by tag; arithmetic in docs/spend.md."),
	}
}

// ---- goldens ---------------------------------------------------------------

func TestRender_Golden(t *testing.T) {
	assertGolden(t, Render(bodyFixture(), optsFixture()), "frame_default")
}

func TestRender_Draft_Golden(t *testing.T) {
	o := optsFixture()
	o.Source = Estimate("Author's own estimate from three sampled months; not yet reconciled.").Drafted()
	assertGolden(t, Render(bodyFixture(), o), "frame_draft")
}

func TestRender_Published_Golden(t *testing.T) {
	o := optsFixture()
	o.ID = "spend-share"
	o.Source = Published("CNCF Annual Survey 2024, table 6.").Linked("https://example.org/survey-2024")
	assertGolden(t, Render(bodyFixture(), o), "frame_published_linked")
}

func TestRender_Mechanism_Golden(t *testing.T) {
	got := Render(`<svg xmlns="http://www.w3.org/2000/svg" role="img" aria-label="ingest path"><title>ingest path</title></svg>`,
		Options{Title: "Ingest path", Carries: CarriesMechanism})
	assertGolden(t, got, "frame_mechanism")
}

// ---- the guard -------------------------------------------------------------

func TestRender_FigureWithoutSource_RendersNothing(t *testing.T) {
	got := Render(bodyFixture(), Options{Title: "Spend share by team"})
	if got != "" {
		t.Errorf("a figure with no source must not render, got:\n%s", got)
	}
}

func TestRender_MechanismWithoutSource_Renders(t *testing.T) {
	got := Render(`<svg></svg>`, Options{Title: "Ingest path", Carries: CarriesMechanism})
	if !strings.Contains(got, "<figure") {
		t.Errorf("a mechanism diagram may carry no source, got %q", got)
	}
	if strings.Contains(got, "<p ") {
		t.Errorf("mechanism frame emitted a source line it was not given: %s", got)
	}
}

func TestRender_DraftDoesNotExcuseProvenance(t *testing.T) {
	// The point of the draft state: it is an extra warning, not a substitute.
	got := Render(bodyFixture(), Options{Title: "x", Source: Source{Draft: true}})
	if got != "" {
		t.Errorf("draft with no source must not render, got:\n%s", got)
	}
}

func TestValidate_Cases(t *testing.T) {
	cases := []struct {
		name string
		o    Options
		want error
	}{
		{"zero value", Options{}, ErrMissingSource},
		{"figure, no source", Options{Title: "t"}, ErrMissingSource},
		{"mechanism, no source", Options{Carries: CarriesMechanism}, nil},
		{"kind but no text", Options{Source: Source{Kind: KindDerived}}, ErrMissingText},
		{"text but no kind", Options{Source: Source{Text: "somewhere"}}, ErrUnknownKind},
		{"text but bogus kind", Options{Source: Source{Kind: "vibes", Text: "somewhere"}}, ErrUnknownKind},
		{"whitespace text", Options{Source: Source{Kind: KindDerived, Text: "   "}}, ErrMissingText},
		{"mechanism with half a source", Options{Carries: CarriesMechanism, Source: Source{Text: "x"}}, ErrUnknownKind},
		{"published", Options{Source: Published("a report")}, nil},
		{"derived", Options{Source: Derived("this arithmetic")}, nil},
		{"estimate", Options{Source: Estimate("my guess, and I say so")}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.o.Validate(); !errors.Is(got, tc.want) {
				t.Errorf("Validate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMustRender_PanicsOnViolation(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustRender did not panic on a missing source")
		}
		if err, ok := r.(error); !ok || !errors.Is(err, ErrMissingSource) {
			t.Errorf("MustRender panicked with %v, want ErrMissingSource", r)
		}
	}()
	MustRender(bodyFixture(), Options{Title: "x"})
}

func TestMustRender_MatchesRender(t *testing.T) {
	a := Render(bodyFixture(), optsFixture())
	b := MustRender(bodyFixture(), optsFixture())
	if a != b {
		t.Errorf("MustRender and Render disagree on a valid frame")
	}
}

// ---- the kind label cannot be hidden ---------------------------------------

func TestRender_EstimateIsLabelled(t *testing.T) {
	got := Render(bodyFixture(), Options{Source: Estimate("A number I made up responsibly.")})
	if !strings.Contains(got, "Estimate") {
		t.Errorf("an estimate must print the word Estimate: %s", got)
	}
	if !strings.Contains(got, `data-viz-source="estimate"`) {
		t.Errorf("estimate frame missing data-viz-source: %s", got)
	}
}

func TestRender_KindLabels(t *testing.T) {
	cases := map[Kind]string{KindPublished: "Source", KindDerived: "Derived", KindEstimate: "Estimate"}
	for k, want := range cases {
		got := Render(`<svg></svg>`, Options{Source: Source{Kind: k, Text: "the sentence"}})
		if !strings.Contains(got, ">"+want+"<") {
			t.Errorf("kind %q did not render label %q: %s", k, want, got)
		}
	}
}

// ---- safety ----------------------------------------------------------------

func TestRender_Escaping(t *testing.T) {
	got := Render(`<svg></svg>`, Options{
		Title:      `<script>alert(1)</script>`,
		Note:       `<b>bold</b>`,
		DraftLabel: `<i>draft</i>`,
		ID:         `"onload=`,
		AriaLabel:  `<img src=x>`,
		Source:     Derived(`<script>steal()</script>`).Drafted(),
	})
	for _, bad := range []string{"<script>", "<b>", "<i>", "<img ", `"onload=`} {
		if strings.Contains(got, bad) {
			t.Errorf("frame did not escape %q: %s", bad, got)
		}
	}
}

func TestRender_UnsafeHrefDropped(t *testing.T) {
	for _, href := range []string{"javascript:alert(1)", "JavaScript:alert(1)", "data:text/html,<script>", "vbscript:x", " javascript:x"} {
		got := Render(`<svg></svg>`, Options{Source: Published("a report").Linked(href)})
		if strings.Contains(got, "<a ") {
			t.Errorf("frame emitted a link for %q: %s", href, got)
		}
		if !strings.Contains(got, "a report") {
			t.Errorf("dropping the link also dropped the sentence for %q: %s", href, got)
		}
	}
}

func TestRender_SafeHrefKept(t *testing.T) {
	for _, href := range []string{"/docs/spend.md", "#working", "./notes.html", "../notes.html", "https://example.org/x", "mailto:a@example.org"} {
		got := Render(`<svg></svg>`, Options{Source: Published("a report").Linked(href)})
		if !strings.Contains(got, "<a ") {
			t.Errorf("frame dropped a safe href %q: %s", href, got)
		}
	}
}

// ---- composition, determinism, accessibility -------------------------------

func TestRender_BodyPassesThroughUntouched(t *testing.T) {
	body := bodyFixture()
	got := Render(body, optsFixture())
	if !strings.Contains(got, body) {
		t.Errorf("frame altered the viz body it was given")
	}
	// The frame must not disturb the body's accessibility contract.
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("framed viz lost role=img / <title>: %s", got)
	}
}

func TestRender_NoLiteralColors(t *testing.T) {
	got := Render(bodyFixture(), optsFixture())
	// Everything the frame itself adds must be a role utility; the only "#"
	// allowed in the output would come from the body, which has none here.
	frame := strings.Replace(got, bodyFixture(), "", 1)
	for _, bad := range []string{"#", "rgb(", "oklch("} {
		if strings.Contains(frame, bad) {
			t.Errorf("frame emitted a literal color (%q): %s", bad, frame)
		}
	}
}

func TestRender_Determinism(t *testing.T) {
	a := Render(bodyFixture(), optsFixture())
	b := Render(bodyFixture(), optsFixture())
	if a != b {
		t.Errorf("frame non-deterministic")
	}
}

func TestRender_Accessibility(t *testing.T) {
	o := optsFixture()
	o.ID = "spend-share"
	got := Render(bodyFixture(), o)
	for _, want := range []string{"<figure", "<figcaption", `id="spend-share"`, `aria-describedby="spend-share-src"`, `id="spend-share-src"`} {
		if !strings.Contains(got, want) {
			t.Errorf("frame missing %q: %s", want, got)
		}
	}
}

func TestRender_AriaLabelOnlyWithoutTitle(t *testing.T) {
	withTitle := Render(`<svg></svg>`, Options{Title: "T", AriaLabel: "L", Source: Derived("x")})
	if strings.Contains(withTitle, `aria-label=`) {
		t.Errorf("aria-label should defer to the caption when a title is present: %s", withTitle)
	}
	noTitle := Render(`<svg></svg>`, Options{AriaLabel: "L", Source: Derived("x")})
	if !strings.Contains(noTitle, `aria-label="L"`) {
		t.Errorf("aria-label should be emitted when there is no title: %s", noTitle)
	}
}

func TestOptions_ZeroValueDefaultsToStrict(t *testing.T) {
	// Silence must mean the strict case, or the rule leaks.
	var o Options
	if got := o.withDefaults().Carries; got != CarriesFigure {
		t.Errorf("zero Carries = %q, want %q", got, CarriesFigure)
	}
}

// TestSourceLineNeverUsesInkFaint — hazard H6 (docs/theming-hazards.md):
// a caveat set quieter than the claim it qualifies is dishonest. Measured on
// this frame's own bg-surface-raised ground, 2026-08-07: ink 17.46:1 light /
// 14.73:1 dark, ink-muted 7.33 / 6.78, ink-faint 3.89 / 3.73 — below
// tokens.TextFloor (4.5). Nothing in this frame may be set in ink-faint,
// because everything in it is either the claim or the qualification of a claim.
func TestSourceLineNeverUsesInkFaint(t *testing.T) {
	cases := map[string]Options{
		"published": {Title: "T", Source: Published("Publication, table 6.")},
		"derived":   {Title: "T", Note: "N", Source: Derived("Sum of the export.")},
		"estimate":  {Title: "T", Source: Estimate("Author's own figure.")},
		"draft":     {Title: "T", Source: Estimate("Author's own figure.").Drafted()},
		"linked":    {ID: "f", Title: "T", Source: Published("Pub.").Linked("https://example.org")},
		"mechanism": {Title: "T", Carries: CarriesMechanism},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			out := MustRender(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`, o)
			if strings.Contains(out, "ink-faint") {
				t.Errorf("vizframe %s renders ink-faint (3.89:1 light / 3.73:1 dark on surface-raised, below TextFloor 4.5). "+
					"Use ink-muted: smaller is a legible hierarchy, fainter is a legibility cliff. See docs/theming-hazards.md H6", name)
			}
		})
	}
}

// TestSourceLineIsNotQuieterThanTheFigure states the H6 rule the way it will be
// broken next: not by re-adding the exact class, but by reaching for whatever
// the faintest available role is. The source line's role must be the same one
// the frame uses for its own standfirst — one step below the title, no further.
func TestSourceLineIsNotQuieterThanTheFigure(t *testing.T) {
	out := MustRender("<svg/>", Options{Title: "T", Note: "N", Source: Derived("Sum of the export.")})
	i := strings.Index(out, "ak-vizframe-body")
	if i < 0 {
		t.Fatal("no frame body rendered")
	}
	tail := out[i:] // everything after the figure: the source line
	if !strings.Contains(tail, "text-ink-muted") {
		t.Errorf("the source line is not set in ink-muted; got %q", tail)
	}
}

package docs

import (
	"html/template"
	"strings"
	"testing"
)

func TestRenderToC_Empty(t *testing.T) {
	if got := RenderToC(nil); got != "" {
		t.Errorf("RenderToC(nil) = %q, want empty — an empty ToC shell implies content that is not there", got)
	}
}

// TestRenderToC_ExactMarkup pins the boxed-ToC byte contract this package
// promoted from the portal unchanged, so a consumer that already asserted on
// this shape (platform-engineering's own view_test.go did) keeps passing
// against the kit's version with no rewrite.
func TestRenderToC_ExactMarkup(t *testing.T) {
	got := RenderToC([]Heading{
		{Level: 1, Text: "Title", ID: "title"},
		{Level: 2, Text: "First", ID: "first"},
		{Level: 3, Text: "Nested", ID: "nested"},
		{Level: 2, Text: "Second", ID: "second"},
	})
	for _, want := range []string{
		`<nav class="doc-toc" aria-label="Table of contents">`,
		`<li class="doc-toc__item doc-toc__item--h1"><a href="#title">Title</a>`,
		`<li class="doc-toc__item doc-toc__item--h3"><a href="#nested">Nested</a>`,
		`</nav>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderToC(...) = %q, want it to contain %q", got, want)
		}
	}
}

// TestRenderToC_NestsBySkippedLevel proves a skipped level (h3 directly
// under h1, no h2 between) nests under the nearest shallower heading rather
// than being dropped or flattened — the document's structure renders as
// authored.
func TestRenderToC_NestsBySkippedLevel(t *testing.T) {
	got := RenderToC([]Heading{
		{Level: 1, Text: "Top", ID: "top"},
		{Level: 3, Text: "Deep", ID: "deep"},
	})
	// "Deep" must nest inside "Top"'s <li>, not sit beside it in the outer
	// <ol> — i.e. its own <li> opens before Top's closes.
	topOpen := strings.Index(got, `doc-toc__item--h1`)
	deepOpen := strings.Index(got, `doc-toc__item--h3`)
	topClose := strings.Index(got[topOpen:], `</li>`) + topOpen
	if !(topOpen < deepOpen && deepOpen < topClose) {
		t.Errorf("RenderToC(...) = %q, want the h3 nested inside the h1's <li>", got)
	}
}

func TestRenderToC_Escapes(t *testing.T) {
	got := RenderToC([]Heading{{Level: 1, Text: `<script>alert(1)</script>`, ID: `"quoted"`}})
	if strings.Contains(got, "<script>") {
		t.Errorf("RenderToC(...) = %q, heading text rendered unescaped", got)
	}
	if strings.Contains(got, `href="""quoted"""`) {
		t.Errorf("RenderToC(...) = %q, heading id rendered unescaped into an attribute", got)
	}
}

func TestRenderToCPanel_Empty(t *testing.T) {
	if got := RenderToCPanel(nil, Options{}); got != "" {
		t.Errorf("RenderToCPanel(nil, ...) = %q, want empty", got)
	}
}

// Standalone outline links need their own target/focus treatment. A caller's
// page padding does not enlarge a link's clickable box or paint its focus ring.
func TestRenderToCPanel_TargetAndFocusClasses(t *testing.T) {
	got := RenderToCPanel([]Heading{{Level: 2, Text: strings.Repeat("LongLabel", 32), ID: "long"}}, Options{})
	for _, tag := range []string{"summary", "a"} {
		start := strings.Index(got, "<"+tag+" ")
		if start < 0 {
			t.Fatalf("missing %s", tag)
		}
		attrs := got[start : start+strings.IndexByte(got[start:], '>')]
		for _, class := range []string{"min-h-[44px]", "min-w-[44px]", "break-words", "focus-visible:outline-2", "focus-visible:outline-offset-2", "focus-visible:outline-focus"} {
			if !strings.Contains(attrs, class) {
				t.Errorf("%s missing %s", tag, class)
			}
		}
	}
	if !strings.Contains(got, `href="#long"`) || strings.Contains(got, "<script") {
		t.Fatal("outline must retain native fragment navigation")
	}
}

func TestRenderToCRail_Empty(t *testing.T) {
	if got := RenderToCRail(nil, Options{}); got != "" {
		t.Errorf("RenderToCRail(nil, ...) = %q, want empty", got)
	}
}

func TestRenderToCPanel_DefaultLabel(t *testing.T) {
	got := RenderToCPanel([]Heading{{Level: 2, Text: "First", ID: "first"}}, Options{})
	if !strings.Contains(got, ">On this page</summary>") {
		t.Errorf("RenderToCPanel(...) = %q, want the default label \"On this page\"", got)
	}
	if !strings.Contains(got, `<details class="border-y border-line" open>`) {
		t.Errorf("RenderToCPanel(...) = %q, want an always-open native <details>", got)
	}
}

// TestRenderToCPanel_CustomLabel proves a caller can wrap the label in its
// own typography (the portal's tabular-figures instrument register, for
// one) without this package baking that class in as a default every
// consumer inherits — see Options.LabelHTML's doc comment.
func TestRenderToCPanel_CustomLabel(t *testing.T) {
	got := RenderToCPanel(
		[]Heading{{Level: 2, Text: "First", ID: "first"}},
		Options{LabelHTML: template.HTML(`<span class="datum-register">On this page</span>`)},
	)
	if !strings.Contains(got, `<span class="datum-register">On this page</span></summary>`) {
		t.Errorf("RenderToCPanel(...) = %q, want the caller-supplied label markup verbatim", got)
	}
}

func TestRenderToCRail_DefaultLabel(t *testing.T) {
	got := RenderToCRail([]Heading{{Level: 2, Text: "First", ID: "first"}}, Options{})
	if !strings.Contains(got, ">On this page</h2>") {
		t.Errorf("RenderToCRail(...) = %q, want the default label \"On this page\" in an <h2>", got)
	}
	if !strings.Contains(got, `aria-label="Table of contents"`) {
		t.Errorf("RenderToCRail(...) = %q, want an aria-labelled nav", got)
	}
}

// TestIndentClampsBelowH2 proves a level-1 heading reaching either panel
// renderer — a caller that has not pre-filtered its own page title out of
// the outline, unlike the portal's docHeadings — clamps to the table's first
// entry rather than panicking on a negative slice index.
func TestIndentClampsBelowH2(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
	}{
		{"panel", RenderToCPanel([]Heading{{Level: 1, Text: "Top", ID: "top"}}, Options{})},
		{"rail", RenderToCRail([]Heading{{Level: 1, Text: "Top", ID: "top"}}, Options{})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(tc.got, `href="#top"`) {
				t.Fatalf("render did not include the level-1 heading at all: %q", tc.got)
			}
		})
	}
}

// TestIndentDeepensByLevel proves the indent step actually varies with
// level rather than collapsing every heading to one class.
func TestIndentDeepensByLevel(t *testing.T) {
	got := RenderToCPanel([]Heading{
		{Level: 2, Text: "Shallow", ID: "shallow"},
		{Level: 4, Text: "Deep", ID: "deep"},
	}, Options{})
	if !strings.Contains(got, `class="pl-0"`) {
		t.Errorf("RenderToCPanel(...) = %q, want the h2 indented at pl-0", got)
	}
	if !strings.Contains(got, `class="pl-6"`) {
		t.Errorf("RenderToCPanel(...) = %q, want the h4 indented deeper than the h2", got)
	}
}

// TestAnchorsAreGiven proves this package never derives an id from a
// heading's Text — it renders whatever ID it was handed, even one that does
// not look like any slug of Text. A future contributor "helpfully" adding
// slugification here would reintroduce the exact failure the anchors-are-
// declared convention (portal/internal/adapters/gitdocs/anchors.go, P2C-9)
// exists to prevent: every citation dying the day a heading is copy-edited.
// This test is what stops that.
func TestAnchorsAreGiven(t *testing.T) {
	h := Heading{Level: 2, Text: "Why df alone does not close it", ID: "the-two-oracles"}

	toc := RenderToC([]Heading{h})
	if !strings.Contains(toc, `href="#the-two-oracles"`) {
		t.Errorf("RenderToC(...) = %q, want the given id the-two-oracles, not a slug of Text", toc)
	}
	if strings.Contains(toc, "why-df-alone") {
		t.Errorf("RenderToC(...) = %q, appears to have derived an id from Text instead of using the given one", toc)
	}

	panel := RenderToCPanel([]Heading{h}, Options{})
	if !strings.Contains(panel, `href="#the-two-oracles"`) {
		t.Errorf("RenderToCPanel(...) = %q, want the given id", panel)
	}

	rail := RenderToCRail([]Heading{h}, Options{})
	if !strings.Contains(rail, `href="#the-two-oracles"`) {
		t.Errorf("RenderToCRail(...) = %q, want the given id", rail)
	}

	if got := RenderPermalink(h.ID); got != `<a class="doc-heading__anchor" href="#the-two-oracles" aria-label="Permalink to this section">#</a>` {
		t.Errorf("RenderPermalink(%q) = %q, want the given id verbatim", h.ID, got)
	}
}

func TestRenderPermalink_EmptyID(t *testing.T) {
	if got := RenderPermalink(""); got != "" {
		t.Errorf(`RenderPermalink("") = %q, want empty — a heading with no id gets no permalink`, got)
	}
}

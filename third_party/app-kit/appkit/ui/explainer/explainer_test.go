package explainer

import (
	"strings"
	"testing"
)

func sampleDoc() Doc {
	return Doc{
		Codename: "canon",
		Eyebrow:  "CONTROL ENGINE",
		Lead:     `the <b>control law</b>`,
		Oneliner: "intent -> emit -> everywhere",
		Accent:   "#f4b860",
		Roles: []Role{
			{Kind: "THE LAW", Name: "canon", Note: "controls + emission", Current: true},
			{Kind: "THE FACTORY", Name: "PDK", Note: "intent -> Capability"},
		},
		Sections: []Section{{
			Label: "THE MACHINE",
			Title: "author once, emit everywhere",
			Intro: "the <em>spine</em>",
			Body:  Figure(SVG(120, `<text x="10" y="20">x</text>`), "one model"),
		}},
		Foot: `see <a href="canon/README.md">canon</a>`,
	}
}

func TestRender_SelfContainedDocument(t *testing.T) {
	got := Render(sampleDoc())
	for _, want := range []string{
		"<!DOCTYPE html>", "<title>canon</title>", "<style>", "</style>",
		"<h1>canon</h1>",
		"--cyan:#f4b860", // accent threaded into the inlined theme
		`<p class="lead">the <b>control law</b></p>`,                     // Lead is trusted HTML
		`<div class="oneliner">intent -&gt; emit -&gt; everywhere</div>`, // Oneliner escaped
		`<div class="role cur">`,                                         // current role highlighted
		"⤓ Save as PDF",                                                  // the export toolbar is composed in
		"</body>", "</html>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered doc missing %q", want)
		}
	}
	// no external resources (self-contained)
	for _, bad := range []string{"<link", "src=\"http", "href=\"http"} {
		if strings.Contains(got, bad) {
			t.Errorf("doc should be self-contained, found %q", bad)
		}
	}
}

func TestRender_Escaping(t *testing.T) {
	got := Render(Doc{Codename: `<script>x</script>`, Oneliner: `<img src=x>`})
	if strings.Contains(got, "<script>x</script>") {
		t.Errorf("codename not escaped")
	}
	if strings.Contains(got, "<img src=x>") {
		t.Errorf("oneliner not escaped")
	}
}

func TestRender_TitleFallsBackToCodename(t *testing.T) {
	got := Render(Doc{Codename: "Muster"})
	if !strings.Contains(got, "<title>Muster</title>") {
		t.Errorf("title should fall back to codename")
	}
}

func TestRender_DefaultAccent(t *testing.T) {
	got := Render(Doc{Codename: "x"})
	if !strings.Contains(got, "--cyan:"+defaultAccent) {
		t.Errorf("empty accent should default to kit cyan")
	}
}

func TestHelpers(t *testing.T) {
	if got := Value("fast"); got != `<span class="val">✓ fast</span>` {
		t.Errorf("Value: %s", got)
	}
	if got := SVG(90, "<g/>"); !strings.Contains(got, `viewBox="0 0 1080 90"`) || !strings.Contains(got, "<g/>") {
		t.Errorf("SVG: %s", got)
	}
	card := Card("Title", []string{"a", "b"}, "win")
	for _, w := range []string{`<h4>Title</h4>`, "<li>a</li>", "<li>b</li>", `<span class="val">✓ win</span>`} {
		if !strings.Contains(card, w) {
			t.Errorf("Card missing %q: %s", w, card)
		}
	}
	grid := Grid("<x/>", "<y/>")
	if grid != `<div class="grid"><x/><y/></div>` {
		t.Errorf("Grid: %s", grid)
	}
	tbl := Table([]string{"H1", "H2"}, [][]string{{"a", "b"}})
	for _, w := range []string{"<th>H1</th>", "<th>H2</th>", "<td>a</td>", "<td>b</td>"} {
		if !strings.Contains(tbl, w) {
			t.Errorf("Table missing %q", w)
		}
	}
}

func TestHelpers_Escaping(t *testing.T) {
	if strings.Contains(Card("<b>t</b>", []string{"<i>x</i>"}, "<u>v</u>"), "<b>t</b>") {
		t.Errorf("Card should escape plain text")
	}
	if strings.Contains(Table([]string{"<th>"}, [][]string{{"<td>"}}), "<th><th>") {
		t.Errorf("Table should escape cells")
	}
}

func TestRender_Deterministic(t *testing.T) {
	d := sampleDoc()
	if Render(d) != Render(d) {
		t.Errorf("render is non-deterministic")
	}
}

// TestRender_ReducedMotionEscape pins the invariant the package doc claims and
// motion.css enforces for every other kit component: both animated marks stay
// legible with motion off. explainer is the one documented exemption from
// appkit/ui/motion (it is a single self-contained file, so it cannot @import the
// shared layer), which means nothing else guards its escape — a future edit to
// baseCSS could silently drop it. This test is that guard.
//
// It also records the divergence decision: explainer uses `animation:none`,
// where motion.css argues for `1ms`. That is safe *here specifically* because
// both animations are infinite decorative loops with nothing awaiting a
// `transitionend` — the exact case where `none` cannot hang a JS consumer. See
// the package doc's "Motion" section.
func TestRender_ReducedMotionEscape(t *testing.T) {
	out := Render(sampleDoc())

	if !strings.Contains(out, "prefers-reduced-motion:reduce") {
		t.Fatal("no prefers-reduced-motion escape — decorative loops keep animating for readers who asked for less motion")
	}
	// The live dot must read as a green dot without its pulse.
	for _, want := range []string{"border-radius:50%", "background:var(--green)"} {
		if !strings.Contains(out, want) {
			t.Errorf(".live lost its static fallback %q — the dot must be legible with the pulse cancelled", want)
		}
	}
	// The marching-ants dash must read as a dashed stroke without its march.
	if !strings.Contains(out, "stroke-dasharray:6 6") {
		t.Error(".dash lost its static fallback stroke-dasharray:6 6 — the stroke must be legible with the march cancelled")
	}
	// The escape must actually cancel both loops.
	if !strings.Contains(out, ".live{animation:none}") || !strings.Contains(out, ".dash{animation:none}") {
		t.Error("the reduced-motion block does not cancel both decorative loops")
	}
}

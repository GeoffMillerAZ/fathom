package gallery

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// svgRe captures each inline SVG so the L3a contract can be checked on the
// served page, separately from the Gallery's own utility-classed chrome.
var svgRe = regexp.MustCompile(`(?s)<svg\b.*?</svg>`)

// TestGraphPageServesEverySpecimen covers the /graph route, which is linked
// from /components and the analytics.graph catalog preview rather than the
// section nav, so allRoutes-driven tests do not reach it.
func TestGraphPageServesEverySpecimen(t *testing.T) {
	h := New()
	for _, theme := range []string{"", "?theme=light", "?theme=dark"} {
		res, body := get(t, h, "/graph"+theme)
		if res.StatusCode != http.StatusOK || !strings.Contains(body, "app-kit Gallery") {
			t.Fatalf("GET /graph%s = %d", theme, res.StatusCode)
		}
		for _, want := range []string{
			`id="graph-neighbourhood"`, `id="graph-bounded"`, `id="graph-layered"`, `id="graph-given"`, `id="graph-empty"`,
			"Table alternative: store.ResolveEdges neighbourhood",
			"<caption>config.Load callers: nodes</caption>",
			"+9 more",
			"<tr><td>+9 more (Callers)</td><td>config.Load</td><td>Resolved call</td><td>no</td><td>7</td></tr>",
			"<tr><td>+9 more (Callers)</td><td>config.Load</td><td>Heuristic call</td><td>no</td><td>2</td></tr>",
			"1 of 10 supplied nodes is more than 2 steps from the focus",
			"No dependencies recorded",
			"viz: invalid graph input",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("/graph%s missing %q", theme, want)
			}
		}
		svgs := svgRe.FindAllString(body, -1)
		if len(svgs) != 5 {
			t.Fatalf("/graph%s rendered %d SVGs, want 5", theme, len(svgs))
		}
		for _, svg := range svgs {
			if strings.Contains(svg, "class=") || !strings.Contains(svg, `role="img"`) || !strings.Contains(svg, "<desc>") {
				t.Errorf("graph SVG breaks the L3a/accessibility contract: %.200s", svg)
			}
		}
	}
}

// TestGraphFramesDeclareMechanism holds the page to the provenance rule the
// Gallery applies elsewhere: a diagram without quantities says so explicitly.
func TestGraphFramesDeclareMechanism(t *testing.T) {
	_, body := get(t, New(), "/graph")
	frames := vizframes(body)
	if len(frames) != 5 {
		t.Fatalf("found %d vizframes, want 5", len(frames))
	}
	for _, f := range frames {
		if !strings.Contains(f, `data-viz-carries="mechanism"`) {
			t.Errorf("graph frame lacks the mechanism waiver: %.200s", f)
		}
	}
}

// TestGraphTableLinksOwn44pxTargets: the table alternative is the only
// keyboard and assistive-technology route to node destinations, so, like the
// Gallery's other table drill-downs, its links own at least 44px-high hit
// areas rather than borrowing the cell padding. The browser harness measures
// the rendered boxes; this keeps the classes and their built rules in place.
func TestGraphTableLinksOwn44pxTargets(t *testing.T) {
	_, body := get(t, New(), "/graph")
	regions := regexp.MustCompile(`<div class="([^"]*)" tabindex="0" role="region" aria-label="[^"]* table, horizontally scrollable">`).FindAllStringSubmatch(body, -1)
	if len(regions) != 5 {
		t.Fatalf("found %d table regions, want 5", len(regions))
	}
	want := []string{"[&_a]:inline-flex", "[&_a]:min-h-11", "[&_a]:items-center"}
	for _, r := range regions {
		classes := strings.Fields(r[1])
		for _, c := range want {
			if !slices.Contains(classes, c) {
				t.Errorf("table region lacks %s: %s", c, r[1])
			}
		}
	}
	_, css := get(t, New(), "/static/app.css")
	for _, rule := range []string{`.\[\&_a\]\:inline-flex a{display:inline-flex}`, `.\[\&_a\]\:min-h-11 a{min-height:calc(var(--spacing) * 11)}`, `.\[\&_a\]\:items-center a{align-items:center}`} {
		if !strings.Contains(css, rule) {
			t.Errorf("built CSS lacks %s; run task gallery:css", rule)
		}
	}
}

// TestGraphCatalogPageClaimsNoConsumer: Fathom selected analytics.graph but
// has not integrated it, so the capability page must not list it under
// observed consumers.
func TestGraphCatalogPageClaimsNoConsumer(t *testing.T) {
	res, body := get(t, New(), "/catalog/analytics.graph")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /catalog/analytics.graph = %d", res.StatusCode)
	}
	if strings.Contains(body, "Observed consumers") {
		t.Error("a selection is not an observed consumer")
	}
	if !strings.Contains(body, "has not integrated it") {
		t.Error("the page must still disclose the selection and its missing integration")
	}
}

func TestComponentsPageLinksGraphSpecimens(t *testing.T) {
	_, body := get(t, New(), "/components")
	if !strings.Contains(body, `href="/graph"`) {
		t.Error("/components must link the unlisted /graph specimen page")
	}
}

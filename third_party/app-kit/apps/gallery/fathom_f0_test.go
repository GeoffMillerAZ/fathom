package gallery

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Fathom's console wave F0 returned four kit defects (FATHOM-F0-KIT-01..04).
// These pin the served Gallery markup and built CSS the browser check in
// docs/assessments/fixtures/fathom-f0-kit/browser-check.js measures; rendered
// geometry itself is browser evidence, not a go test claim.
func TestFathomF0KitReturnsAreServed(t *testing.T) {
	res, layout := get(t, New(), "/layout")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /layout = %d", res.StatusCode)
	}
	// KIT-01/02: the specimen table's header cells are column headers and its
	// scroll wrapper is the containing block for the hidden Actions label.
	if !strings.Contains(layout, `<div class="relative overflow-x-auto rounded-lg border border-line bg-surface-raised"><table`) {
		t.Error("/layout table lacks the positioned scroll wrapper")
	}
	ths := regexp.MustCompile(`<th\b[^>]*>`).FindAllString(layout, -1)
	if len(ths) == 0 {
		t.Fatal("/layout renders no header cells")
	}
	for _, th := range ths {
		if !strings.Contains(th, `scope="col"`) {
			t.Errorf("/layout header cell without scope=col: %s", th)
		}
	}

	// KIT-03: breadcrumb links carry the target-size and focus utilities.
	_, agentic := get(t, New(), "/agentic")
	crumbs := regexp.MustCompile(`<nav aria-label="Breadcrumb">.*?</nav>`).FindString(agentic)
	if crumbs == "" {
		t.Fatal("/agentic renders no breadcrumb trail")
	}
	if n := strings.Count(crumbs, `class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-sm hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"`); n != 2 {
		t.Errorf("breadcrumb links with target/focus classes = %d, want 2: %s", n, crumbs)
	}

	// KIT-04: the fluid sparkline card fills its container.
	_, components := get(t, New(), "/components")
	if !strings.Contains(components, `<svg xmlns="http://www.w3.org/2000/svg" width="100%" role="img" aria-label="fluid trend" viewBox="0 0 320 48" preserveAspectRatio="xMinYMin meet">`) {
		t.Error("/components lacks the fluid sparkline specimen")
	}

	_, css := get(t, New(), "/static/app.css")
	for _, rule := range []string{
		`.relative{position:relative}`,
		`.min-h-11{min-height:calc(var(--spacing) * 11)}`,
		`.min-w-11{min-width:calc(var(--spacing) * 11)}`,
		`.focus-visible\:outline-focus:focus-visible{outline-color:var(--ak-color-focus)}`,
	} {
		if !strings.Contains(css, rule) {
			t.Errorf("built CSS lacks %s; run task gallery:css", rule)
		}
	}
}

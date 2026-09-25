package gallery

import (
	"strings"
	"testing"
)

// Browser coverage lives in apps/styleguide/browser/components_reflow.mjs.
// This pins the small layout contract, not rendered geometry or accessibility.
func TestComponentFiguresShrinkWithoutDroppingChartsOrCode(t *testing.T) {
	content, err := buildComponents()
	if err != nil {
		t.Fatal(err)
	}
	body := string(content)
	cards := vizCards()
	if got := strings.Count(body, `<figure class="min-w-0 `); got != len(cards) {
		t.Fatalf("shrinkable component figures: got %d, want %d", got, len(cards))
	}
	for _, card := range cards {
		if !strings.Contains(body, string(card.SVG)) {
			t.Errorf("%s: chart markup changed or disappeared", card.Name)
		}
	}
	if strings.Count(body, `<pre class="mt-4 overflow-x-auto `) != len(cards) {
		t.Fatal("every code sample must keep its own scrolling container")
	}
	_, css := get(t, New(), "/static/app.css")
	if !strings.Contains(css, `.min-w-0{min-width:0}`) &&
		!strings.Contains(css, `.min-w-0{min-width:calc(var(--spacing) * 0)}`) {
		t.Fatal("served CSS must implement the figure's shrink contract")
	}
}

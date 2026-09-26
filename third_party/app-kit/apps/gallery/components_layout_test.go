package gallery

import (
	"os"
	"regexp"
	"slices"
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

// components_reflow.mjs (the INC-39 harness) pins the exact figure captions it
// expects in the narrow component preview. It is opt-in, so this keeps its
// list in step with vizCards(): adding a card without updating the harness
// fails here instead of only in the next browser run.
func TestComponentsReflowHarnessPinsEveryCard(t *testing.T) {
	src, err := os.ReadFile("../styleguide/browser/components_reflow.mjs")
	if err != nil {
		t.Fatal(err)
	}
	list := regexp.MustCompile(`(?m)^const captions = \[(.*)\];$`).FindSubmatch(src)
	if list == nil {
		t.Fatal("components_reflow.mjs has no captions list")
	}
	var harness []string
	for _, m := range regexp.MustCompile(`'([^']*)'`).FindAllSubmatch(list[1], -1) {
		harness = append(harness, string(m[1]))
	}
	var cards []string
	for _, card := range vizCards() {
		cards = append(cards, card.Name)
	}
	if !slices.Equal(harness, cards) {
		t.Fatalf("components_reflow.mjs captions (%d) differ from vizCards (%d):\nharness %q\ncards   %q", len(harness), len(cards), harness, cards)
	}
}

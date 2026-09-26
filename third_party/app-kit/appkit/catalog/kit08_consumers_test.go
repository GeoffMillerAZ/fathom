package catalog

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Consumers name apps that use a capability in a published revision; a
// requested or planned use belongs in gaps. The FATHOM-KIT-03/08 records are
// pinned here: Fathom's published observatory calls viz.Sparkline, viz.Gauge,
// vizframe.MustRender and hitl.DiffView, but no app renders heatmap truth
// states yet.
func TestKit08RecordsNameOnlyActualConsumers(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	fathom := []string{"Fathom"}
	want := map[string][]string{
		"viz.heatmap":            {},
		"viz.trend-sparkline":    fathom,
		"viz.composition":        {},
		"viz.scores":             fathom,
		"viz.stages":             {},
		"viz.outcome":            {},
		"viz.provenance-frame":   fathom,
		"work.agent-trace":       fathom,
		"foundation.motion":      {},
		"ops.topology":           {},
		"content.explainer":      {},
		"content.export-toolbar": {},
		"content.details":        {},
	}
	for id, consumers := range want {
		e, ok := c.Get(id)
		if !ok {
			t.Errorf("%s missing", id)
			continue
		}
		if !slices.Equal(e.Consumers, consumers) {
			t.Errorf("%s consumers = %q, want %q", id, e.Consumers, consumers)
		}
	}
	heatmap, _ := c.Get("viz.heatmap")
	if !strings.Contains(strings.Join(heatmap.Gaps, " "), "no app consumes them yet") {
		t.Error("viz.heatmap must say in its gaps that no app consumes truth states yet")
	}
	qualified := regexp.MustCompile(`[()]|(?i)\b(not yet|planned|selected|originating|pending)\b`)
	for _, e := range c.Entries {
		for _, name := range e.Consumers {
			if qualified.MatchString(name) {
				t.Errorf("%s consumer %q is qualified; name an app that uses it, and put intent in gaps", e.ID, name)
			}
		}
	}
}

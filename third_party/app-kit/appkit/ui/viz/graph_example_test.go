package viz_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

// ExampleLayoutEgo lays out one symbol's callers and callees, then renders the
// picture and its table alternative from the same layout.
func ExampleLayoutEgo() {
	nodes := []viz.GraphNode{
		{ID: "api.Handle", Group: "internal/api"},
		{ID: "TestResolve", Group: "internal/store"},
		{ID: "store.Resolve", Group: "internal/store", Href: "/symbols/store.Resolve"},
		{ID: "index.Lookup", Group: "internal/index"},
	}
	edges := []viz.GraphEdge{
		{From: "api.Handle", To: "store.Resolve"},
		{From: "TestResolve", To: "store.Resolve", Emphasis: true},
		{From: "store.Resolve", To: "index.Lookup", Style: viz.GraphEdgeDashed},
	}
	layout, err := viz.LayoutEgo("store.Resolve", nodes, edges, viz.EgoOptions{GraphOptions: viz.GraphOptions{
		AriaLabel:     "store.Resolve neighbourhood",
		SolidLabel:    "Resolved call",
		DashedLabel:   "Heuristic call",
		EmphasisLabel: "Reaches a test",
	}})
	if err != nil {
		panic(err)
	}
	svg, table := layout.SVG(), layout.Table()
	fmt.Println(strings.HasPrefix(svg, "<svg"), strings.Contains(svg, `role="img"`))
	fmt.Println(strings.Contains(table, `<a href="/symbols/store.Resolve">store.Resolve</a>`))
	fmt.Println(len(layout.Notes()))

	_, err = viz.LayoutGraph([]viz.GraphNode{{ID: "a", Href: "javascript:alert(1)"}}, nil, viz.GraphOptions{})
	fmt.Println(errors.Is(err, viz.ErrInvalidGraph))
	// Output:
	// true true
	// true
	// 0
	// true
}

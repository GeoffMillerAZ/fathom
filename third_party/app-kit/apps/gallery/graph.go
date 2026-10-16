package gallery

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

// graph.go is the specimen page for the experimental layered-graph renderer
// (catalog analytics.graph). Every node, edge and route below is synthetic; the
// shapes mirror the originating consumer's code-neighbourhood view without any
// of its data. A diagram states no quantity, so each frame carries the
// mechanism waiver rather than a source line.

type graphSpecimen struct {
	ID, Heading, Note string
	Frame             template.HTML
	Table             template.HTML
	Code              string
}

// graphViewport wraps an SVG in a named, keyboard-scrollable region: a layered
// graph is wider than a phone, and the page must not scroll sideways instead.
func graphViewport(svg, label string) string {
	return `<div class="max-w-full overflow-x-auto rounded-md bg-surface p-3 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="` +
		html.EscapeString(label+" diagram, horizontally scrollable") + `">` + svg + `</div>`
}

// graphTable styles the renderer's class-free table alternative from outside:
// the kit emits plain semantic HTML and the app owns its presentation. Its
// links are the keyboard and assistive-technology route to node destinations,
// so each owns a 44px-high hit area (min-h-11), not just the cell padding.
func graphTable(table, label string) template.HTML {
	return template.HTML(`<details class="mt-3 rounded-md border border-line bg-surface-raised">` +
		`<summary class="g-summary border-0">Table alternative: ` + html.EscapeString(label) + `</summary>` +
		`<div class="max-w-full overflow-x-auto px-3 pb-3 text-sm text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus [&_caption]:py-2 [&_caption]:text-left [&_caption]:font-medium [&_table]:mt-2 [&_table]:w-full [&_table]:border-collapse [&_th]:border-b [&_th]:border-line [&_th]:px-2 [&_th]:py-1.5 [&_th]:text-left [&_th]:font-medium [&_td]:border-b [&_td]:border-line [&_td]:px-2 [&_td]:py-1.5 [&_td]:text-ink-muted [&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center [&_a]:text-accent [&_a]:underline [&_a:focus-visible]:outline-2 [&_a:focus-visible]:outline-offset-2 [&_a:focus-visible]:outline-focus [&_ul]:mt-3 [&_ul]:list-disc [&_ul]:pl-5 [&_li]:text-ink-muted" tabindex="0" role="region" aria-label="` +
		html.EscapeString(label+" table, horizontally scrollable") + `">` + table + `</div></details>`)
}

func graphFrame(id, title string, l viz.GraphLayout) template.HTML {
	note := "Mechanism diagram — no quantity stated."
	if notes := l.Notes(); len(notes) > 0 {
		note = strings.Join(notes, " ")
	}
	return template.HTML(vizframe.MustRender(graphViewport(l.SVG(), title), vizframe.Options{
		ID:      id,
		Title:   title,
		Note:    note,
		Carries: vizframe.CarriesMechanism,
	}))
}

func neighbourhoodFixture() ([]viz.GraphNode, []viz.GraphEdge) {
	nodes := []viz.GraphNode{
		{ID: "cmd.serve", Label: "cmd.serve", Group: "cmd/app"},
		{ID: "api.Handle", Label: "api.Handle", Group: "internal/api", Href: "/graph?symbol=api.Handle#graph-neighbourhood"},
		{ID: "api.Batch", Label: "api.Batch", Group: "internal/api"},
		{ID: "TestResolveEdges", Label: "TestResolveEdges", Group: "internal/store"},
		{ID: "store.ResolveEdges", Label: "store.ResolveEdges", Group: "internal/store", Href: "/graph?symbol=store.ResolveEdges#graph-neighbourhood"},
		{ID: "index.Lookup", Label: "index.Lookup", Group: "internal/index"},
		{ID: "cache.Get", Label: "cache.Get", Group: "internal/cache"},
		{ID: "store.resolveOne", Label: "store.resolveOne", Group: "internal/store"},
		{ID: "disk.ReadSegment", Label: "disk.ReadSegmentWithChecksumVerification", Group: "internal/disk"},
		{ID: "fleet.Sweep", Label: "fleet.Sweep", Group: "internal/fleet"},
	}
	edges := []viz.GraphEdge{
		{From: "cmd.serve", To: "api.Handle"},
		{From: "api.Handle", To: "store.ResolveEdges"},
		{From: "api.Batch", To: "store.ResolveEdges", Style: viz.GraphEdgeDashed},
		{From: "TestResolveEdges", To: "store.ResolveEdges", Emphasis: true},
		{From: "store.ResolveEdges", To: "index.Lookup", Emphasis: true},
		{From: "store.ResolveEdges", To: "cache.Get", Style: viz.GraphEdgeDashed},
		{From: "store.ResolveEdges", To: "store.resolveOne"},
		{From: "store.resolveOne", To: "store.resolveOne"},
		{From: "index.Lookup", To: "disk.ReadSegment", Emphasis: true},
		{From: "disk.ReadSegment", To: "fleet.Sweep"},
	}
	return nodes, edges
}

func fanInFixture() ([]viz.GraphNode, []viz.GraphEdge) {
	nodes := []viz.GraphNode{{ID: "config.Load", Label: "config.Load", Group: "internal/config"}}
	var edges []viz.GraphEdge
	callers := []string{"cmd.doctor", "cmd.serve", "cmd.verify", "cmd.ingest", "cmd.prime", "hooks.Session", "hooks.Edit", "gate.Run", "gate.Push", "fleet.Report", "fleet.Sweep", "plans.Status", "TestLoad", "TestLoadDefaults"}
	for _, c := range callers {
		nodes = append(nodes, viz.GraphNode{ID: c, Label: c})
		e := viz.GraphEdge{From: c, To: "config.Load", Emphasis: strings.HasPrefix(c, "Test")}
		if strings.HasPrefix(c, "hooks.") { // two heuristic calls among the collapsed callers
			e.Style = viz.GraphEdgeDashed
		}
		edges = append(edges, e)
	}
	return nodes, edges
}

func pipelineGraphFixture() ([]viz.GraphNode, []viz.GraphEdge) {
	nodes := []viz.GraphNode{
		{ID: "fetch", Label: "fetch sources"},
		{ID: "parse", Label: "parse"},
		{ID: "extract", Label: "extract symbols", Group: "dialect packs"},
		{ID: "fuse", Label: "fuse edges"},
		{ID: "index", Label: "write index"},
		{ID: "retry", Label: "retry failed"},
		{ID: "lint", Label: "lint config"},
	}
	edges := []viz.GraphEdge{
		{From: "fetch", To: "parse"},
		{From: "parse", To: "extract"},
		{From: "parse", To: "fuse", Style: viz.GraphEdgeDashed},
		{From: "extract", To: "fuse", Emphasis: true},
		{From: "fuse", To: "index", Emphasis: true},
		{From: "lint", To: "index"},
		{From: "fuse", To: "retry"},
		{From: "retry", To: "fetch", Style: viz.GraphEdgeDashed},
	}
	return nodes, edges
}

func buildGraph() (template.HTML, error) {
	var specimens []graphSpecimen

	nn, ne := neighbourhoodFixture()
	ego, err := viz.LayoutEgo("store.ResolveEdges", nn, ne, viz.EgoOptions{GraphOptions: viz.GraphOptions{
		AriaLabel:     "store.ResolveEdges neighbourhood",
		Description:   "Synthetic callers and callees of store.ResolveEdges to depth 2.",
		SolidLabel:    "Resolved call",
		DashedLabel:   "Heuristic call",
		EmphasisLabel: "Reaches a test",
	}})
	if err != nil {
		return "", err
	}
	specimens = append(specimens, graphSpecimen{
		ID:      "graph-neighbourhood",
		Heading: "Ego graph — a symbol's neighbourhood",
		Note:    "The focus sits in the centre column, callers to the left and callees to the right by distance. Dashed means heuristic, thick accent means the path reaches a test; the legend and the table say so in words. Long labels truncate in the box and stay whole in the tooltip and table. The node beyond depth 2 is omitted and counted.",
		Frame:   graphFrame("graph-neighbourhood", "store.ResolveEdges neighbourhood", ego),
		Table:   graphTable(ego.Table(), "store.ResolveEdges neighbourhood"),
		Code: `layout, err := viz.LayoutEgo("store.ResolveEdges", nodes, edges, viz.EgoOptions{
    GraphOptions: viz.GraphOptions{
        AriaLabel:     "store.ResolveEdges neighbourhood",
        SolidLabel:    "Resolved call",
        DashedLabel:   "Heuristic call",
        EmphasisLabel: "Reaches a test",
    },
})
svg, table, notes := layout.SVG(), layout.Table(), layout.Notes()`,
	})

	fn, fe := fanInFixture()
	fan, err := viz.LayoutEgo("config.Load", fn, fe, viz.EgoOptions{Depth: 1, GraphOptions: viz.GraphOptions{
		AriaLabel:     "config.Load callers",
		MaxPerRank:    6,
		SolidLabel:    "Resolved call",
		DashedLabel:   "Heuristic call",
		EmphasisLabel: "Test caller",
	}})
	if err != nil {
		return "", err
	}
	specimens = append(specimens, graphSpecimen{
		ID:      "graph-bounded",
		Heading: "Bounded column — the remainder is aggregated and said",
		Note:    "Fourteen callers exceed MaxPerRank 6: five are drawn (test callers first, then degree, then input order) and the other nine are collapsed into one “+9 more” node. Its seven resolved and two heuristic calls stay separate solid and dashed lines, each counted in its own table row, so the collapse never restyles a call. The frame note, SVG description and table all state the reduction, and the table lists every collapsed caller. The empty callee side is drawn as None, not left out.",
		Frame:   graphFrame("graph-bounded", "config.Load callers", fan),
		Table:   graphTable(fan.Table(), "config.Load callers"),
		Code:    `viz.LayoutEgo("config.Load", nodes, edges, viz.EgoOptions{Depth: 1, GraphOptions: viz.GraphOptions{MaxPerRank: 6, SolidLabel: "Resolved call", DashedLabel: "Heuristic call"}})`,
	})

	pn, pe := pipelineGraphFixture()
	pipe, err := viz.LayoutGraph(pn, pe, viz.GraphOptions{
		AriaLabel:     "indexing pipeline",
		Description:   "Synthetic stages of one ingest run.",
		DashedLabel:   "Optional step",
		EmphasisLabel: "Critical path",
	})
	if err != nil {
		return "", err
	}
	specimens = append(specimens, graphSpecimen{
		ID:      "graph-layered",
		Heading: "Layered graph — longest-path columns",
		Note:    "Sources start at the left and every node sits one column right of its furthest predecessor. Order within a column comes from barycenter sweeps with stable tie-breaks. The retry edge closes a cycle, so it is drawn right to left and counted.",
		Frame:   graphFrame("graph-layered", "Indexing pipeline", pipe),
		Table:   graphTable(pipe.Table(), "indexing pipeline"),
		Code:    `viz.LayoutGraph(nodes, edges, viz.GraphOptions{AriaLabel: "indexing pipeline", DashedLabel: "Optional step", EmphasisLabel: "Critical path"})`,
	})

	given, err := viz.LayoutGraph([]viz.GraphNode{
		{ID: "request", Rank: 0},
		{ID: "policy", Rank: 1, Kind: viz.GraphNodeAggregate, Label: "3 policy checks"},
		{ID: "archive", Rank: 3},
	}, []viz.GraphEdge{{From: "request", To: "policy"}, {From: "policy", To: "archive", Style: viz.GraphEdgeDashed}}, viz.GraphOptions{
		AriaLabel:   "given columns",
		RankMode:    viz.GraphRankGiven,
		RankLabels:  []string{"Inputs", "Checks", "Transforms", "Outputs"},
		DashedLabel: "Deferred",
	})
	if err != nil {
		return "", err
	}
	specimens = append(specimens, graphSpecimen{
		ID:      "graph-given",
		Heading: "Given columns — an empty column stays visible",
		Note:    "RankMode given places nodes in the app's own columns. A labelled column with no nodes says None rather than disappearing, and an app-supplied aggregate node uses the aggregate treatment: a dashed border in the measured ink-muted role, named in the legend.",
		Frame:   graphFrame("graph-given", "Given columns", given),
		Table:   graphTable(given.Table(), "given columns"),
		Code:    `viz.LayoutGraph(nodes, edges, viz.GraphOptions{RankMode: viz.GraphRankGiven, RankLabels: []string{"Inputs", "Checks", "Transforms", "Outputs"}})`,
	})

	empty, err := viz.LayoutGraph(nil, nil, viz.GraphOptions{AriaLabel: "no dependencies", EmptyText: "No dependencies recorded"})
	if err != nil {
		return "", err
	}
	specimens = append(specimens, graphSpecimen{
		ID:      "graph-empty",
		Heading: "Empty — said in words",
		Note:    "No nodes renders an explicit sentence in both the picture and the table, never a blank box.",
		Frame:   graphFrame("graph-empty", "No dependencies", empty),
		Table:   graphTable(empty.Table(), "no dependencies"),
		Code:    `viz.LayoutGraph(nil, nil, viz.GraphOptions{EmptyText: "No dependencies recorded"})`,
	})

	_, refused := viz.LayoutGraph([]viz.GraphNode{{ID: "a", Href: "javascript:alert(1)"}}, nil, viz.GraphOptions{})
	return htmlf("graph", graphHTML, struct {
		Specimens []graphSpecimen
		Refused   string
		Limits    string
	}{specimens, fmt.Sprint(refused), fmt.Sprintf("%d nodes, %d edges, %d columns", viz.GraphMaxNodes, viz.GraphMaxEdges, viz.GraphMaxColumns)})
}

const graphHTML = `
<h1 class="text-2xl font-semibold text-ink">Layered graph</h1>
<p class="mt-2 max-w-3xl text-ink-muted">Experimental <code class="font-mono text-sm text-ink">viz.LayoutGraph</code> and <code class="font-mono text-sm text-ink">viz.LayoutEgo</code> (catalog <code class="font-mono text-sm text-ink">analytics.graph</code>). One validated layout renders two ways: an L3a SVG with no class attributes and colors only through <code class="font-mono text-sm text-ink">var(--ak-color-*)</code>, and a class-free table alternative. Same input, byte-identical output.</p>
<p class="mt-3 max-w-3xl font-medium text-ink">Every node, edge and link on this page is synthetic. The shapes follow Fathom's code-neighbourhood view, the originating consumer; this page is not evidence that Fathom has adopted the renderer.</p>
<p class="mt-3 max-w-3xl text-ink-muted">The SVG is <code class="font-mono text-sm text-ink">role="img"</code> with a title and a description that carries every reduction note. Node links inside the picture are pointer conveniences removed from the tab order; the table alternative carries the same destinations as ordinary links. Inputs are bounded ({{.Limits}}); invalid input is refused before any output.</p>

{{range .Specimens}}
<section class="mt-10" id="{{.ID}}-section">
  <h2 class="text-lg font-medium text-ink">{{.Heading}}</h2>
  <p class="mt-1 max-w-3xl text-sm text-ink-muted">{{.Note}}</p>
  <div class="mt-3 min-w-0">{{.Frame}}</div>
  {{.Table}}
  <div class="mt-3 min-w-0 rounded-lg border border-line bg-surface-sunken p-4">
    <pre class="max-w-full overflow-x-auto font-mono text-xs leading-relaxed text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="{{.Heading}} code example"><code>{{.Code}}</code></pre>
  </div>
</section>
{{end}}

<h2 class="mt-10 text-lg font-medium text-ink">Refusal</h2>
<p class="mt-2 max-w-3xl text-ink-muted">A node link that is not a relative or http(s) reference, a duplicate ID, an unknown edge endpoint or an over-bound input returns an error wrapping <code class="font-mono text-sm text-ink">viz.ErrInvalidGraph</code> and no layout:</p>
<pre class="mt-3 max-w-full overflow-x-auto rounded-lg border border-line bg-surface-sunken p-4 font-mono text-xs text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="Refusal example"><code>{{.Refused}}</code></pre>
`

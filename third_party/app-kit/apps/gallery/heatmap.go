package gallery

import (
	"html/template"
	"math"
	"strconv"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

// heatmapPath is the viz.Heatmap truth-state specimen (FATHOM-KIT-03). It is a
// Components sub-route, so the Components navigation entry stays current.
const heatmapPath = "/components/heatmap"

// heatmapFixtureCell is one synthetic coverage cell. The chart inputs and the
// table alternative are both derived from this single record, so the figure
// and its accessible alternative cannot disagree.
type heatmapFixtureCell struct {
	Value  float64
	State  viz.CellState
	Reason string
}

// heatmapFixture is an invented project × layer coverage matrix: every value
// is made up and no measurement stands behind it.
func heatmapFixture() (projects, layers []string, cells [][]heatmapFixtureCell) {
	projects = []string{"atlas", "beacon", "cobalt", "delta"}
	layers = []string{"dialect", "provider", "platform", "concern"}
	notCompiled := heatmapFixtureCell{State: viz.CellMissing, Reason: "not compiled"}
	cells = [][]heatmapFixtureCell{
		{{Value: 12}, {Value: 0}, {Value: 7}, notCompiled},
		{{Value: 5}, {State: viz.CellUnknown, Reason: "index unreadable"}, {Value: 9}, {Value: 3}},
		{notCompiled, notCompiled, {State: viz.CellError, Reason: "compile failed: exit 2"}, {Value: 0}},
		{{Value: 14}, {Value: 2}, {State: viz.CellUnknown, Reason: "stale since 2026-09-01"}, {State: viz.CellError, Reason: "timeout after 30 s"}},
	}
	return projects, layers, cells
}

// heatmapInputs converts the fixture into viz.Heatmap's rows and States.
// Absent cells carry NaN so the legacy comparison renders them as holes.
func heatmapInputs(cells [][]heatmapFixtureCell) ([][]float64, [][]viz.HeatmapCell) {
	rows := make([][]float64, len(cells))
	states := make([][]viz.HeatmapCell, len(cells))
	for r, row := range cells {
		rows[r] = make([]float64, len(row))
		states[r] = make([]viz.HeatmapCell, len(row))
		for c, cell := range row {
			rows[r][c] = cell.Value
			if cell.State != viz.CellValue {
				rows[r][c] = math.NaN()
			}
			states[r][c] = viz.HeatmapCell{State: cell.State, Reason: cell.Reason}
		}
	}
	return rows, states
}

// heatmapCellText is the table's wording for one cell; it matches the text
// after "row, column: " in the chart's cell title.
func heatmapCellText(cell heatmapFixtureCell) string {
	text := ""
	switch {
	case cell.State == viz.CellValue && cell.Value == 0:
		text = "0 (zero)"
	case cell.State == viz.CellValue:
		text = strconv.FormatFloat(cell.Value, 'f', -1, 64)
	default:
		text = string(cell.State)
	}
	if cell.Reason != "" {
		text += " — " + cell.Reason
	}
	return text
}

type heatmapTableRow struct {
	Project string
	Cells   []string
}

type heatmapData struct {
	Truth, AllMissing, Legacy template.HTML
	Layers                    []string
	Table                     []heatmapTableRow
	TruthCall, MissingCall    string
}

func buildHeatmap() (template.HTML, error) {
	projects, layers, cells := heatmapFixture()
	rows, states := heatmapInputs(cells)
	// Cell 56 keeps each column wide enough for its label: the renderer centers
	// x labels without wrapping, so long labels overlap at the 26px default.
	opts := viz.HeatmapOptions{Cell: 56, XLabels: layers, YLabels: projects, States: states, AriaLabel: "Layer coverage by project, with cell truth states"}
	table := make([]heatmapTableRow, len(cells))
	for r, row := range cells {
		table[r].Project = projects[r]
		for _, cell := range row {
			table[r].Cells = append(table[r].Cells, heatmapCellText(cell))
		}
	}
	nothing := make([][]viz.HeatmapCell, 3)
	for r := range nothing {
		nothing[r] = []viz.HeatmapCell{{State: viz.CellMissing, Reason: "not compiled"}, {State: viz.CellMissing, Reason: "not compiled"}, {State: viz.CellMissing, Reason: "not compiled"}}
	}
	return htmlf("heatmap", heatmapHTML, heatmapData{
		Truth:      template.HTML(viz.Heatmap(rows, opts)),
		AllMissing: template.HTML(viz.Heatmap(nil, viz.HeatmapOptions{Cell: 56, States: nothing, XLabels: layers[:3], YLabels: projects[:3], AriaLabel: "Layer coverage with nothing compiled"})),
		Legacy:     template.HTML(viz.Heatmap(rows, viz.HeatmapOptions{Cell: 56, XLabels: layers, YLabels: projects, AriaLabel: "Layer coverage, legacy rendering without truth states"})),
		Layers:     layers,
		Table:      table,
		TruthCall: `viz.Heatmap(rows, viz.HeatmapOptions{
    Cell: 56, XLabels: layers, YLabels: projects,
    States: [][]viz.HeatmapCell{
        {{}, {}, {}, {State: viz.CellMissing, Reason: "not compiled"}},
        {{}, {State: viz.CellUnknown, Reason: "index unreadable"}, {}, {}},
        …
    },
})`,
		MissingCall: `viz.Heatmap(nil, viz.HeatmapOptions{States: allMissing}) // no ramp fill, no value range`,
	})
}

const heatmapHTML = `
<p class="text-sm text-ink-muted"><a href="/components" class="text-accent underline underline-offset-4">Components</a> / Heatmap truth states</p>
<h1 class="mt-2 text-2xl font-semibold text-ink">Heatmap truth states</h1>
<p class="mt-2 max-w-2xl text-ink-muted"><code class="font-mono text-accent">viz.Heatmap</code> with <code class="font-mono">States</code> (or <code class="font-mono">TruthStates</code>) separates a measured value and a measured zero from three kinds of absence. Missing and unknown cells are hatched, never filled; an error cell carries a danger cross. A zero is a filled ramp cell marked <strong>0</strong>. Every cell has a title with its reason, and the legend names the states present.</p>
<p class="mt-3 max-w-2xl text-sm text-ink-muted">Synthetic fixture: the projects, layers and values are invented. The SVG is L3a (no classes, role colors only); cell titles are pointer-only, so the table below carries the same cells for keyboard and screen-reader use.</p>

<h2 class="mt-8 text-lg font-medium text-ink">Mixed states</h2>
<figure class="mt-4 min-w-0 rounded-lg border border-line bg-surface-raised p-5">
  <figcaption class="font-medium text-ink">Layer coverage by project</figcaption>
  <div class="mt-4 max-w-full overflow-x-auto rounded-md bg-surface p-4 focus-visible:outline-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="Layer coverage chart, horizontally scrollable if needed">{{.Truth}}</div>
  <pre class="mt-4 overflow-x-auto rounded-md bg-surface-sunken p-3 text-xs text-ink-muted"><code class="font-mono">{{.TruthCall}}</code></pre>
</figure>

<h2 class="mt-8 text-lg font-medium text-ink">The same cells as a table</h2>
<div class="mt-3 overflow-x-auto rounded-md border border-line focus-visible:outline-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="Layer coverage table, horizontally scrollable if needed">
  <table id="heatmap-cells" class="w-full text-left text-sm">
    <caption class="sr-only">Layer coverage by project: value, zero, missing, unknown or error, with the reason</caption>
    <thead class="border-b border-line bg-surface-sunken text-ink-muted"><tr><th scope="col" class="px-4 py-3 font-medium">Project</th>{{range .Layers}}<th scope="col" class="px-4 py-3 font-medium">{{.}}</th>{{end}}</tr></thead>
    <tbody>{{range .Table}}<tr class="border-b border-line"><th scope="row" class="px-4 py-3 font-medium text-ink">{{.Project}}</th>{{range .Cells}}<td class="px-4 py-3 text-ink">{{.}}</td>{{end}}</tr>{{end}}</tbody>
  </table>
</div>

<h2 class="mt-8 text-lg font-medium text-ink">Nothing measured</h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">An all-missing grid draws no ramp fill and claims no value range; the legend states the shared reason.</p>
<figure class="mt-4 min-w-0 rounded-lg border border-line bg-surface-raised p-5">
  <figcaption class="font-medium text-ink">Nothing compiled</figcaption>
  <div class="mt-4 max-w-full overflow-x-auto rounded-md bg-surface p-4 focus-visible:outline-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="All-missing chart, horizontally scrollable if needed">{{.AllMissing}}</div>
  <pre class="mt-4 overflow-x-auto rounded-md bg-surface-sunken p-3 text-xs text-ink-muted"><code class="font-mono">{{.MissingCall}}</code></pre>
</figure>

<h2 class="mt-8 text-lg font-medium text-ink">Without truth states</h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">The original rendering, unchanged for existing callers: unknown and error become the same dashed hole as missing, and the zero cells are only faint fills.</p>
<figure class="mt-4 min-w-0 rounded-lg border border-line bg-surface-raised p-5">
  <figcaption class="font-medium text-ink">Legacy rendering of the same cells</figcaption>
  <div class="mt-4 max-w-full overflow-x-auto rounded-md bg-surface p-4 focus-visible:outline-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="Legacy chart, horizontally scrollable if needed">{{.Legacy}}</div>
</figure>`

// heatmapCard is the compact Components-page specimen of the same fixture.
func heatmapCard() vizCard {
	projects, layers, cells := heatmapFixture()
	rows, states := heatmapInputs(cells)
	return vizCard{
		Name: "Heatmap — truth states (value, zero, missing, unknown, error)",
		Call: `viz.Heatmap(rows, viz.HeatmapOptions{States: [][]viz.HeatmapCell{{…, {State: viz.CellMissing, Reason: "not compiled"}}}})`,
		SVG:  template.HTML(viz.Heatmap(rows, viz.HeatmapOptions{Cell: 56, XLabels: layers, YLabels: projects, States: states, AriaLabel: "Layer coverage, truth states"})),
	}
}

package viz

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

// ---- fixtures -------------------------------------------------------------

// pipelineFixture is a small build pipeline: a DAG plus one cycle-closing edge
// (retry → fetch) and one duplicate edge, so longest-path ranking, cycle
// breaking and merging all appear in one golden.
func pipelineFixture() ([]GraphNode, []GraphEdge) {
	nodes := []GraphNode{
		{ID: "fetch", Label: "fetch sources"},
		{ID: "parse", Label: "parse"},
		{ID: "extract", Label: "extract symbols", Group: "dialect packs"},
		{ID: "fuse", Label: "fuse edges"},
		{ID: "index", Label: "write index", Href: "/stages/index"},
		{ID: "retry", Label: "retry"},
		{ID: "lint", Label: "lint config"},
	}
	edges := []GraphEdge{
		{From: "fetch", To: "parse"},
		{From: "parse", To: "extract"},
		{From: "parse", To: "fuse", Style: GraphEdgeDashed},
		{From: "extract", To: "fuse", Emphasis: true},
		{From: "fuse", To: "index", Emphasis: true},
		{From: "lint", To: "index"},
		{From: "fuse", To: "retry"},
		{From: "retry", To: "fetch", Style: GraphEdgeDashed},
		{From: "extract", To: "fuse"}, // duplicate: merged
	}
	return nodes, edges
}

// egoFixture is a synthetic code neighbourhood around store.Resolve.
func egoFixture() ([]GraphNode, []GraphEdge) {
	nodes := []GraphNode{
		{ID: "cli.run", Group: "cmd/app"},
		{ID: "api.Handle", Group: "internal/api", Href: "/symbols/api.Handle"},
		{ID: "api.Batch", Group: "internal/api"},
		{ID: "TestResolve", Group: "internal/store"},
		{ID: "store.Resolve", Group: "internal/store", Href: "/symbols/store.Resolve"},
		{ID: "index.Lookup", Group: "internal/index"},
		{ID: "cache.Get", Group: "internal/cache"},
		{ID: "disk.Read", Group: "internal/disk"},
		{ID: "far.Away", Group: "internal/far"},
		{ID: "unrelated", Group: "internal/other"},
	}
	edges := []GraphEdge{
		{From: "cli.run", To: "api.Handle"},
		{From: "api.Handle", To: "store.Resolve"},
		{From: "api.Batch", To: "store.Resolve", Style: GraphEdgeDashed},
		{From: "TestResolve", To: "store.Resolve", Emphasis: true},
		{From: "store.Resolve", To: "index.Lookup", Emphasis: true},
		{From: "store.Resolve", To: "cache.Get", Style: GraphEdgeDashed},
		{From: "index.Lookup", To: "disk.Read"},
		{From: "disk.Read", To: "far.Away"},
	}
	return nodes, edges
}

func egoOptions() EgoOptions {
	return EgoOptions{GraphOptions: GraphOptions{
		AriaLabel:     "store.Resolve neighbourhood",
		SolidLabel:    "Resolved call",
		DashedLabel:   "Heuristic call",
		EmphasisLabel: "Reaches a test",
	}}
}

// wideFixture has one caller column far over the default bound.
func wideFixture(n int) ([]GraphNode, []GraphEdge) {
	nodes := []GraphNode{{ID: "focus", Label: "focus"}}
	var edges []GraphEdge
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("caller%02d", i)
		nodes = append(nodes, GraphNode{ID: id})
		edges = append(edges, GraphEdge{From: id, To: "focus", Emphasis: i == n-1})
	}
	return nodes, edges
}

func mustLayout(t *testing.T, nodes []GraphNode, edges []GraphEdge, o GraphOptions) GraphLayout {
	t.Helper()
	l, err := LayoutGraph(nodes, edges, o)
	if err != nil {
		t.Fatalf("LayoutGraph: %v", err)
	}
	return l
}

func mustEgo(t *testing.T, focus string, nodes []GraphNode, edges []GraphEdge, o EgoOptions) GraphLayout {
	t.Helper()
	l, err := LayoutEgo(focus, nodes, edges, o)
	if err != nil {
		t.Fatalf("LayoutEgo: %v", err)
	}
	return l
}

// assertGoldenHTML is assertGolden for the table alternative (.html.golden).
func assertGoldenHTML(t *testing.T, got, name string) {
	t.Helper()
	path := filepath.Join("testdata", name+".html.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 go test to generate)", path, err)
	}
	if string(want) != got {
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, string(want))
	}
}

// column returns a node's column and position in the laid-out order.
func column(t *testing.T, l GraphLayout, label string) (col, idx int) {
	t.Helper()
	for c, order := range l.order {
		for i, v := range order {
			if l.nodes[v].label == label {
				return c, i
			}
		}
	}
	t.Fatalf("node %q not laid out", label)
	return -1, -1
}

// ---- golden output --------------------------------------------------------

func TestGraph_Layered_Golden(t *testing.T) {
	nodes, edges := pipelineFixture()
	l := mustLayout(t, nodes, edges, GraphOptions{
		AriaLabel:   "indexing pipeline",
		Description: "Stages of one ingest run.",
		DashedLabel: "Optional",
	})
	assertGolden(t, l.SVG(), "graph_layered")
	assertGoldenHTML(t, l.Table(), "graph_layered_table")
}

func TestGraph_Ego_Golden(t *testing.T) {
	nodes, edges := egoFixture()
	l := mustEgo(t, "store.Resolve", nodes, edges, egoOptions())
	assertGolden(t, l.SVG(), "graph_ego")
	assertGoldenHTML(t, l.Table(), "graph_ego_table")
}

func TestGraph_Bounded_Golden(t *testing.T) {
	nodes, edges := wideFixture(12)
	l := mustEgo(t, "focus", nodes, edges, EgoOptions{GraphOptions: GraphOptions{AriaLabel: "fan-in", MaxPerRank: 5}})
	assertGolden(t, l.SVG(), "graph_bounded")
	assertGoldenHTML(t, l.Table(), "graph_bounded_table")
}

// arcsFixture puts self-loops and within-column edges in a middle column and
// in the last column, where the right margin must make room for them, plus a
// long right-to-left edge.
func arcsFixture() ([]GraphNode, []GraphEdge) {
	nodes := []GraphNode{
		{ID: "src", Rank: 0},
		{ID: "m0", Rank: 1}, {ID: "m1", Rank: 1}, {ID: "m2", Rank: 1},
		{ID: "t0", Rank: 2}, {ID: "t1", Rank: 2}, {ID: "t2", Rank: 2}, {ID: "t3", Rank: 2},
	}
	edges := []GraphEdge{
		{From: "src", To: "m0"},
		{From: "src", To: "m1"},
		{From: "m0", To: "m2", Style: GraphEdgeDashed}, // same column, two steps
		{From: "m1", To: "m1"},                         // self-loop, middle column
		{From: "m0", To: "t0"},
		{From: "m2", To: "t1", Emphasis: true},
		{From: "m1", To: "t2"},
		{From: "t0", To: "t3"},                         // same column, three steps, last column
		{From: "t1", To: "t0", Style: GraphEdgeDashed}, // same column, one step
		{From: "t3", To: "t3"},                         // self-loop, last column
		{From: "t1", To: "src"},                        // right to left across two columns
	}
	return nodes, edges
}

func TestGraph_Arcs_Golden(t *testing.T) {
	nodes, edges := arcsFixture()
	l := mustLayout(t, nodes, edges, GraphOptions{AriaLabel: "loops and arcs", RankMode: GraphRankGiven})
	assertGolden(t, l.SVG(), "graph_arcs")
}

// graphFixtureLayouts lays out every fixture the geometry and markup checks
// below run over.
func graphFixtureLayouts(t *testing.T) map[string]GraphLayout {
	t.Helper()
	pn, pe := pipelineFixture()
	en, ee := egoFixture()
	wn, we := wideFixture(12)
	an, ae := arcsFixture()
	// mixed: collapsed columns on both sides of a linked focus, joined to it
	// forwards and backwards by solid, dashed and emphasized edges.
	mixed := []GraphNode{{ID: "focus", Rank: 1, Href: "/c/focus"}}
	var mixedEdges []GraphEdge
	style := func(dashed bool) GraphEdgeStyle {
		if dashed {
			return GraphEdgeDashed
		}
		return GraphEdgeSolid
	}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("n%02d", i)
		mixed = append(mixed, GraphNode{ID: id, Rank: 2 * (i % 2), Href: "/c/" + id})
		mixedEdges = append(mixedEdges,
			GraphEdge{From: id, To: "focus", Style: style(i%3 == 0), Emphasis: i%4 == 0},
			GraphEdge{From: "focus", To: id, Style: style(i%5 == 0)})
	}
	// collapsed arcs: lanes within one column and right to left.
	cn := []GraphNode{{ID: "hub", Rank: 0}, {ID: "peer", Rank: 1, Kind: GraphNodeFocus}}
	var ce []GraphEdge
	for i := 0; i < 5; i++ {
		id := fmt.Sprint("n", i)
		cn = append(cn, GraphNode{ID: id, Rank: 1})
		ce = append(ce, GraphEdge{From: id, To: "hub", Style: style(i == 4)}, GraphEdge{From: id, To: "peer", Style: style(i%2 == 0)})
	}
	return map[string]GraphLayout{
		"layered":        mustLayout(t, pn, pe, GraphOptions{}),
		"ego":            mustEgo(t, "store.Resolve", en, ee, egoOptions()),
		"bounded":        mustEgo(t, "focus", wn, we, EgoOptions{GraphOptions: GraphOptions{MaxPerRank: 5}}),
		"empty":          mustLayout(t, nil, nil, GraphOptions{}),
		"arcs":           mustLayout(t, an, ae, GraphOptions{RankMode: GraphRankGiven}),
		"mixed":          mustLayout(t, mixed, mixedEdges, GraphOptions{RankMode: GraphRankGiven, MaxPerRank: 4}),
		"collapsed arcs": mustLayout(t, cn, ce, GraphOptions{RankMode: GraphRankGiven, MaxPerRank: 2}),
		"whitespace": mustLayout(t, []GraphNode{{ID: "a", Label: "tab\there", Group: "line\nbreak"}, {ID: "b", Label: `&<>"'`}},
			[]GraphEdge{{From: "a", To: "b"}}, GraphOptions{Description: "carriage\rreturn"}),
	}
}

var (
	graphViewBoxRe = regexp.MustCompile(`viewBox="0 0 ([0-9]+) ([0-9]+)"`)
	graphPathRe    = regexp.MustCompile(`<path d="([^"]+)"`)
	graphRectRe    = regexp.MustCompile(`<rect x="([-0-9.]+)" y="([-0-9.]+)" width="([-0-9.]+)" height="([-0-9.]+)"`)
	graphNumRe     = regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?`)
)

func parseNums(s string) []float64 {
	var out []float64
	for _, m := range graphNumRe.FindAllString(s, -1) {
		var v float64
		fmt.Sscan(m, &v)
		out = append(out, v)
	}
	return out
}

// TestGraph_GeometryStaysInsideTheViewBox: every edge control point, arrowhead
// and box lies inside the SVG's own viewBox, so nothing is clipped. A Bézier
// curve stays inside its control points, so checking them bounds the curve —
// including self-loops and same-column arcs in the last column, which rely on
// the right margin reserved for them.
func TestGraph_GeometryStaysInsideTheViewBox(t *testing.T) {
	for name, l := range graphFixtureLayouts(t) {
		svg := l.SVG()
		vb := graphViewBoxRe.FindStringSubmatch(svg)
		if vb == nil {
			t.Fatalf("%s: no viewBox", name)
		}
		w, h := parseNums(vb[1])[0], parseNums(vb[2])[0]
		for _, m := range graphPathRe.FindAllStringSubmatch(svg, -1) {
			nums := parseNums(m[1])
			for i := 0; i+1 < len(nums); i += 2 {
				if x, y := nums[i], nums[i+1]; x < 0 || x > w || y < 0 || y > h {
					t.Errorf("%s: path point (%g, %g) outside the %gx%g viewBox: %s", name, x, y, w, h, m[1])
				}
			}
		}
		for _, m := range graphRectRe.FindAllStringSubmatch(svg, -1) {
			v := parseNums(strings.Join(m[1:], " "))
			if v[0] < 0 || v[1] < 0 || v[0]+v[2] > w || v[1]+v[3] > h {
				t.Errorf("%s: box %v outside the %gx%g viewBox", name, v, w, h)
			}
		}
	}
}

// TestGraph_ArcsStayInTheirGap: a same-column arc or self-loop bulges into the
// gap on its column's right, never into the next column's boxes.
func TestGraph_ArcsStayInTheirGap(t *testing.T) {
	for name, l := range graphFixtureLayouts(t) {
		for _, e := range l.edges {
			a := l.nodes[e.from]
			if a.rank != l.nodes[e.to].rank || a.rank == len(l.cols)-1 {
				continue // the last column is bounded by the viewBox test
			}
			var b strings.Builder
			l.writeEdge(&b, e)
			limit := l.cols[a.rank+1].x
			for _, m := range graphPathRe.FindAllStringSubmatch(b.String(), -1) {
				nums := parseNums(m[1])
				for i := 0; i < len(nums); i += 2 {
					if nums[i] >= limit {
						t.Errorf("%s: arc from %q reaches x=%g, into the next column at %g", name, a.label, nums[i], limit)
					}
				}
			}
		}
	}
}

// TestGraph_SVGIsWellFormedXML: the SVG is catalogued as portable, so it must
// parse as XML on its own (a standalone .svg file or an <img> source), not only
// inside an HTML parser's error recovery.
func TestGraph_SVGIsWellFormedXML(t *testing.T) {
	for name, l := range graphFixtureLayouts(t) {
		d := xml.NewDecoder(strings.NewReader(l.SVG()))
		for {
			_, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("%s: SVG is not well-formed XML: %v", name, err)
				break
			}
		}
	}
}

func TestGraph_Empty_Golden(t *testing.T) {
	l := mustLayout(t, nil, nil, GraphOptions{AriaLabel: "no dependencies"})
	assertGolden(t, l.SVG(), "graph_empty")
	if !strings.Contains(l.Table(), ">No nodes to show</td>") || !strings.Contains(l.Table(), ">No edges</td>") {
		t.Errorf("empty table must say so in words: %s", l.Table())
	}
}

// ---- determinism ----------------------------------------------------------

func TestGraph_Deterministic(t *testing.T) {
	nodes, edges := pipelineFixture()
	first := mustLayout(t, nodes, edges, GraphOptions{})
	for i := 0; i < 25; i++ {
		again := mustLayout(t, nodes, edges, GraphOptions{})
		if again.SVG() != first.SVG() || again.Table() != first.Table() {
			t.Fatal("same input produced different output")
		}
	}
	en, ee := egoFixture()
	a := mustEgo(t, "store.Resolve", en, ee, egoOptions())
	b := mustEgo(t, "store.Resolve", en, ee, egoOptions())
	if a.SVG() != b.SVG() || a.Table() != b.Table() {
		t.Fatal("ego layout is non-deterministic")
	}
}

func TestGraph_DoesNotMutateInput(t *testing.T) {
	nodes, edges := egoFixture()
	before := fmt.Sprint(nodes, edges)
	mustEgo(t, "store.Resolve", nodes, edges, egoOptions())
	mustLayout(t, nodes, edges, GraphOptions{})
	if fmt.Sprint(nodes, edges) != before {
		t.Fatal("layout mutated caller slices")
	}
}

// ---- ranking and ordering -------------------------------------------------

func TestGraph_LongestPathRanks(t *testing.T) {
	nodes, edges := pipelineFixture()
	l := mustLayout(t, nodes, edges, GraphOptions{})
	want := map[string]int{"fetch sources": 0, "lint config": 0, "parse": 1, "extract symbols": 2, "fuse edges": 3, "write index": 4, "retry": 4}
	for label, col := range want {
		if got, _ := column(t, l, label); got != col {
			t.Errorf("%s in column %d, want %d", label, got, col)
		}
	}
	notes := strings.Join(l.Notes(), " ")
	for _, want := range []string{"1 edge closing a cycle is drawn right to left.", "1 duplicate edge was merged."} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes %q missing %q", notes, want)
		}
	}
}

func TestGraph_BarycenterRemovesCrossing(t *testing.T) {
	// Input order crosses: a0→b1 and a1→b0. One sweep reorders column 1.
	nodes := []GraphNode{{ID: "a0"}, {ID: "a1"}, {ID: "b0"}, {ID: "b1"}}
	edges := []GraphEdge{{From: "a0", To: "b1"}, {From: "a1", To: "b0"}}
	l := mustLayout(t, nodes, edges, GraphOptions{})
	_, i0 := column(t, l, "b0")
	_, i1 := column(t, l, "b1")
	if i1 > i0 {
		t.Errorf("barycenter left the crossing in place: b0@%d b1@%d", i0, i1)
	}
}

func TestGraph_StableTieBreakKeepsInputOrder(t *testing.T) {
	// Every node in column 1 has the same single neighbour: equal barycenters.
	nodes := []GraphNode{{ID: "root"}, {ID: "c"}, {ID: "a"}, {ID: "b"}}
	edges := []GraphEdge{{From: "root", To: "c"}, {From: "root", To: "a"}, {From: "root", To: "b"}}
	l := mustLayout(t, nodes, edges, GraphOptions{})
	for want, label := range []string{"c", "a", "b"} {
		if _, got := column(t, l, label); got != want {
			t.Errorf("%s at %d, want input position %d", label, got, want)
		}
	}
}

func TestGraph_GivenRanksAndEmptyColumns(t *testing.T) {
	nodes := []GraphNode{{ID: "x", Rank: 0}, {ID: "y", Rank: 2}}
	l := mustLayout(t, nodes, []GraphEdge{{From: "x", To: "y"}}, GraphOptions{
		RankMode: GraphRankGiven, RankLabels: []string{"Inputs", "Transforms", "Outputs", "Archive"},
	})
	if len(l.cols) != 4 {
		t.Fatalf("labels declare 4 columns, got %d", len(l.cols))
	}
	svg := l.SVG()
	if strings.Count(svg, ">None</text>") != 2 {
		t.Errorf("both empty columns must say None: %s", svg)
	}
	for _, head := range []string{">Inputs</text>", ">Transforms</text>", ">Outputs</text>", ">Archive</text>"} {
		if !strings.Contains(svg, head) {
			t.Errorf("missing heading %s", head)
		}
	}
	if c, _ := column(t, l, "y"); c != 2 {
		t.Errorf("given rank ignored: y in column %d", c)
	}
}

func TestGraph_SelfLoopAndBackEdge(t *testing.T) {
	nodes := []GraphNode{{ID: "a", Rank: 0}, {ID: "b", Rank: 1}}
	edges := []GraphEdge{{From: "a", To: "a"}, {From: "b", To: "a"}, {From: "a", To: "b"}}
	l := mustLayout(t, nodes, edges, GraphOptions{RankMode: GraphRankGiven})
	svg := l.SVG()
	if strings.Count(svg, `fill="none" stroke=`) != 3 {
		t.Errorf("want three edge paths (loop, back, forward): %s", svg)
	}
	if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
		t.Errorf("degenerate geometry: %s", svg)
	}
}

// ---- bounds ---------------------------------------------------------------

func TestGraph_BoundedColumnAggregatesAndSaysSo(t *testing.T) {
	nodes, edges := wideFixture(12)
	l := mustEgo(t, "focus", nodes, edges, EgoOptions{GraphOptions: GraphOptions{MaxPerRank: 5}})
	if got := len(l.order[0]); got != 5 {
		t.Fatalf("caller column draws %d nodes, want 4 kept + 1 aggregate", got)
	}
	agg := l.nodes[l.order[0][4]]
	if !agg.generated || agg.label != "+8 more" || len(agg.members) != 8 {
		t.Fatalf("last node must be the generated aggregate: %+v", agg)
	}
	// The emphasized caller (caller11) outranks degree/input order and is kept.
	if _, idx := column(t, l, "caller11"); idx < 0 {
		t.Error("emphasized node was collapsed")
	}
	want := "Callers shows 4 of 12 nodes; 8 are collapsed into “+8 more” (limit 5 per column)."
	if notes := l.Notes(); len(notes) == 0 || notes[0] != want {
		t.Errorf("notes = %q, want %q", notes, want)
	}
	if !strings.Contains(l.SVG(), "<desc>") || !strings.Contains(l.SVG(), "8 are collapsed") {
		t.Error("the SVG description must carry the reduction note")
	}
	table := l.Table()
	// Priority: the emphasized caller11, then (equal degree) input order.
	for _, kept := range []string{"caller11", "caller00", "caller01", "caller02"} {
		if c, _ := column(t, l, kept); c != 0 {
			t.Errorf("%s should be kept in the caller column", kept)
		}
	}
	for _, want := range []string{"<td>aggregate</td>", "caller03, caller04", "Merged edges", "<td>8</td>", "+8 more (Callers)"} {
		if !strings.Contains(table, want) {
			t.Errorf("table missing %q", want)
		}
	}
}

// TestGraph_AggregateKeepsEachLineStyle: redirecting collapsed callers onto
// "+N more" must not restyle them. One heuristic caller among eight collapsed
// ones stays one dashed line; the seven resolved ones stay a solid line with
// their own count, in the picture and the table alike.
func TestGraph_AggregateKeepsEachLineStyle(t *testing.T) {
	nodes := []GraphNode{{ID: "focus"}}
	var edges []GraphEdge
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("caller%02d", i)
		nodes = append(nodes, GraphNode{ID: id})
		e := GraphEdge{From: id, To: "focus"}
		if i == 11 {
			e.Style = GraphEdgeDashed
		}
		edges = append(edges, e)
	}
	l := mustEgo(t, "focus", nodes, edges, EgoOptions{GraphOptions: GraphOptions{MaxPerRank: 5, SolidLabel: "Resolved call", DashedLabel: "Heuristic call"}})
	table := l.Table()
	for _, want := range []string{
		"<tr><td>+8 more (Callers)</td><td>focus</td><td>Resolved call</td><td>no</td><td>7</td></tr>",
		"<tr><td>+8 more (Callers)</td><td>focus</td><td>Heuristic call</td><td>no</td><td>1</td></tr>",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("table misstates the collapsed edges; missing %s in %s", want, table)
		}
	}
	svg := l.SVG()
	// Four kept callers plus one solid and one dashed line from the aggregate.
	if got := strings.Count(svg, `fill="none" stroke=`); got != 6 {
		t.Errorf("edge paths = %d, want 6 (the aggregate keeps one line per style)", got)
	}
	// One dashed edge plus the dashed legend sample.
	if got := len(graphDashRe.FindAllString(svg, -1)); got != 2 {
		t.Errorf("dashed encodings = %d, want 2", got)
	}
	// Distinct lines, not one drawn over the other: the two aggregate lines
	// leave from different points.
	agg := l.nodes[l.order[0][len(l.order[0])-1]]
	starts := map[string]bool{}
	for _, e := range l.edges {
		if l.nodes[e.from].generated {
			starts[fmtNum(e.sy)] = true
		}
	}
	if !agg.generated || len(starts) != 2 {
		t.Errorf("aggregate lines must leave from distinct ports: %v", starts)
	}

	// Emphasis is kept apart the same way: four emphasized callers collapse
	// beside six plain ones.
	nodes, edges = []GraphNode{{ID: "focus"}}, nil
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("caller%02d", i)
		nodes = append(nodes, GraphNode{ID: id})
		edges = append(edges, GraphEdge{From: id, To: "focus", Emphasis: i < 6})
	}
	l = mustEgo(t, "focus", nodes, edges, EgoOptions{GraphOptions: GraphOptions{MaxPerRank: 3, EmphasisLabel: "Reaches a test"}})
	table = l.Table()
	for _, want := range []string{
		"<tr><td>+10 more (Callers)</td><td>focus</td><td>Solid</td><td>no</td><td>6</td></tr>",
		"<tr><td>+10 more (Callers)</td><td>focus</td><td>Solid</td><td>yes</td><td>4</td></tr>",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("emphasis spread across collapsed edges; missing %s in %s", want, table)
		}
	}
}

// TestGraph_ParallelBackAndSameColumnLinesDoNotOverlap: a collapsed column's
// lines that run right to left or stay in their column get separate lanes too.
func TestGraph_ParallelBackAndSameColumnLinesDoNotOverlap(t *testing.T) {
	// Column 1 keeps only the focus; n0–n4 collapse. Their edges run right to
	// left to hub and within column 1 to the focus, each mixing solid and dashed.
	nodes := []GraphNode{{ID: "hub", Rank: 0}, {ID: "peer", Rank: 1, Kind: GraphNodeFocus}}
	var edges []GraphEdge
	for i := 0; i < 5; i++ {
		id := fmt.Sprint("n", i)
		nodes = append(nodes, GraphNode{ID: id, Rank: 1})
		style := GraphEdgeSolid
		if i == 4 {
			style = GraphEdgeDashed
		}
		edges = append(edges, GraphEdge{From: id, To: "hub", Style: style})
	}
	edges = append(edges, GraphEdge{From: "n3", To: "peer"}, GraphEdge{From: "n4", To: "peer", Style: GraphEdgeDashed})
	l := mustLayout(t, nodes, edges, GraphOptions{RankMode: GraphRankGiven, MaxPerRank: 2})
	paths := map[string]int{}
	matches := regexp.MustCompile(`<path d="([^"]+)" fill="none"`).FindAllStringSubmatch(l.SVG(), -1)
	for _, m := range matches {
		paths[m[1]]++
	}
	if len(matches) != 4 {
		t.Errorf("drawn edges = %d, want 4 (solid and dashed, to hub and to the focus)", len(matches))
	}
	for d, n := range paths {
		if n > 1 {
			t.Errorf("%d edges share one drawn path %q", n, d)
		}
	}
}

func TestGraph_EdgesInsideAnAggregateAreCounted(t *testing.T) {
	nodes := []GraphNode{{ID: "r"}}
	var edges []GraphEdge
	for i := 0; i < 4; i++ {
		id := fmt.Sprint("n", i)
		nodes = append(nodes, GraphNode{ID: id, Rank: 1})
		edges = append(edges, GraphEdge{From: "r", To: id, Emphasis: i == 0})
	}
	edges = append(edges, GraphEdge{From: "n2", To: "n3"}, GraphEdge{From: "n3", To: "n3"})
	l := mustLayout(t, nodes, edges, GraphOptions{RankMode: GraphRankGiven, MaxPerRank: 2})
	if notes := strings.Join(l.Notes(), " "); !strings.Contains(notes, "2 edges between collapsed nodes are not drawn.") {
		t.Errorf("hidden edges must be counted: %q", notes)
	}
}

func TestGraph_FocusSurvivesBounding(t *testing.T) {
	var nodes []GraphNode
	for i := 0; i < 10; i++ {
		nodes = append(nodes, GraphNode{ID: fmt.Sprint("n", i)})
	}
	nodes[9].Kind = GraphNodeFocus
	l := mustLayout(t, nodes, nil, GraphOptions{MaxPerRank: 3})
	if _, idx := column(t, l, "n9"); idx < 0 {
		t.Fatal("focus collapsed")
	}
}

// ---- ego ------------------------------------------------------------------

func TestGraph_EgoSidesAndDepth(t *testing.T) {
	nodes, edges := egoFixture()
	l := mustEgo(t, "store.Resolve", nodes, edges, egoOptions())
	want := map[string]int{"cli.run": 0, "api.Handle": 1, "api.Batch": 1, "TestResolve": 1, "store.Resolve": 2, "index.Lookup": 3, "cache.Get": 3, "disk.Read": 4}
	for label, col := range want {
		if got, _ := column(t, l, label); got != col {
			t.Errorf("%s in column %d, want %d", label, got, col)
		}
	}
	heads := []string{"Callers (depth 2)", "Callers", "Focus", "Callees", "Callees (depth 2)"}
	for c, h := range heads {
		if l.cols[c].label != h {
			t.Errorf("column %d heading %q, want %q", c, l.cols[c].label, h)
		}
	}
	f := l.nodes[l.order[2][0]]
	if f.kind != GraphNodeFocus {
		t.Errorf("focus not marked: %+v", f)
	}
	want2 := "2 of 10 supplied nodes are more than 2 steps from the focus or not connected to it, and are not shown."
	if notes := l.Notes(); len(notes) != 1 || notes[0] != want2 {
		t.Errorf("notes = %q, want %q", notes, want2)
	}
}

func TestGraph_EgoTieGoesToCallees(t *testing.T) {
	nodes := []GraphNode{{ID: "f"}, {ID: "a"}}
	edges := []GraphEdge{{From: "f", To: "a"}, {From: "a", To: "f"}}
	l := mustEgo(t, "f", nodes, edges, EgoOptions{})
	if c, _ := column(t, l, "a"); c != 2 {
		t.Errorf("a node at equal distance both ways belongs to callees, got column %d", c)
	}
	if !strings.Contains(l.Table(), `<tr><th scope="row">None</th><td>Callers</td><td>empty column</td></tr>`) {
		t.Errorf("the table must state the empty side too: %s", l.Table())
	}
	if !strings.Contains(l.SVG(), "Callers: None.") {
		t.Error("the SVG description must state the empty side")
	}
	if !strings.Contains(l.SVG(), ">None</text>") {
		t.Error("the empty caller side must be drawn as an explicit None column")
	}
}

func TestGraph_EgoRefusals(t *testing.T) {
	nodes, edges := egoFixture()
	cases := map[string]EgoOptions{
		"rank mode":   {GraphOptions: GraphOptions{RankMode: GraphRankGiven}},
		"rank labels": {GraphOptions: GraphOptions{RankLabels: []string{"x"}}},
		"depth":       {Depth: 9},
		"negative":    {Depth: -1},
	}
	for name, o := range cases {
		if _, err := LayoutEgo("store.Resolve", nodes, edges, o); !errors.Is(err, ErrInvalidGraph) {
			t.Errorf("%s: err = %v, want ErrInvalidGraph", name, err)
		}
	}
	if _, err := LayoutEgo("missing", nodes, edges, EgoOptions{}); !errors.Is(err, ErrInvalidGraph) {
		t.Errorf("missing focus: %v", err)
	}
	if _, err := LayoutEgo("store.Resolve", nodes, edges, EgoOptions{UpstreamLabel: "Callers\x0b"}); !errors.Is(err, ErrInvalidGraph) {
		t.Errorf("control character in a column word: %v", err)
	}
}

// ---- typed refusals -------------------------------------------------------

func TestGraph_InvalidInputFailsBeforeOutput(t *testing.T) {
	ok := []GraphNode{{ID: "a"}, {ID: "b"}}
	many := make([]GraphNode, GraphMaxNodes+1)
	for i := range many {
		many[i] = GraphNode{ID: fmt.Sprint(i)}
	}
	cases := []struct {
		name  string
		nodes []GraphNode
		edges []GraphEdge
		o     GraphOptions
	}{
		{"empty id", []GraphNode{{ID: ""}}, nil, GraphOptions{}},
		{"long id", []GraphNode{{ID: strings.Repeat("x", 257)}}, nil, GraphOptions{}},
		{"duplicate id", []GraphNode{{ID: "a"}, {ID: "a"}}, nil, GraphOptions{}},
		{"invalid utf8", []GraphNode{{ID: "a", Label: "\xff"}}, nil, GraphOptions{}},
		{"unknown kind", []GraphNode{{ID: "a", Kind: "hero"}}, nil, GraphOptions{}},
		{"script href", []GraphNode{{ID: "a", Href: "javascript:alert(1)"}}, nil, GraphOptions{}},
		{"protocol relative href", []GraphNode{{ID: "a", Href: "//evil.example/x"}}, nil, GraphOptions{}},
		{"unknown endpoint", ok, []GraphEdge{{From: "a", To: "z"}}, GraphOptions{}},
		{"unknown style", ok, []GraphEdge{{From: "a", To: "b", Style: "dotted"}}, GraphOptions{}},
		{"too many nodes", many, nil, GraphOptions{}},
		{"too many edges", ok, make([]GraphEdge, GraphMaxEdges+1), GraphOptions{}},
		{"max per rank 1", ok, nil, GraphOptions{MaxPerRank: 1}},
		{"max per rank 51", ok, nil, GraphOptions{MaxPerRank: 51}},
		{"unknown rank mode", ok, nil, GraphOptions{RankMode: "radial"}},
		{"negative rank", []GraphNode{{ID: "a", Rank: -1}}, nil, GraphOptions{RankMode: GraphRankGiven}},
		{"rank too far", []GraphNode{{ID: "a", Rank: GraphMaxColumns}}, nil, GraphOptions{RankMode: GraphRankGiven}},
		{"invalid option text", ok, nil, GraphOptions{AriaLabel: "\xfe"}},
		// XML 1.0 forbids these characters: a standalone SVG would not parse.
		{"control in label", []GraphNode{{ID: "a", Label: "tab\x0bvt"}}, nil, GraphOptions{}},
		{"control in id", []GraphNode{{ID: "a\x00"}}, nil, GraphOptions{}},
		{"control in group", []GraphNode{{ID: "a", Group: "pkg\x1b[31m"}}, nil, GraphOptions{}},
		{"C1 control in label", []GraphNode{{ID: "a", Label: "next\u0085line"}}, nil, GraphOptions{}},
		{"noncharacter in label", []GraphNode{{ID: "a", Label: "x￿"}}, nil, GraphOptions{}},
		{"control in option text", ok, nil, GraphOptions{DashedLabel: "form\x0cfeed"}},
		{"control in rank label", ok, nil, GraphOptions{RankLabels: []string{"bell\x07"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, err := LayoutGraph(tc.nodes, tc.edges, tc.o)
			if !errors.Is(err, ErrInvalidGraph) {
				t.Fatalf("err = %v, want ErrInvalidGraph", err)
			}
			if len(l.nodes) != 0 || l.desc != "" {
				t.Fatalf("refused input still produced a partial layout: %+v", l)
			}
		})
	}
	chain := make([]GraphNode, GraphMaxColumns+1)
	var edges []GraphEdge
	for i := range chain {
		chain[i] = GraphNode{ID: fmt.Sprint(i)}
		if i > 0 {
			edges = append(edges, GraphEdge{From: fmt.Sprint(i - 1), To: fmt.Sprint(i)})
		}
	}
	if _, err := LayoutGraph(chain, edges, GraphOptions{}); !errors.Is(err, ErrInvalidGraph) {
		t.Errorf("a path longer than %d columns must be refused: %v", GraphMaxColumns, err)
	}
}

func TestGraph_AllowedHrefs(t *testing.T) {
	for _, href := range []string{"/symbols/a", "symbols/a?x=1#y", "https://example.com/a"} {
		if _, err := LayoutGraph([]GraphNode{{ID: "a", Href: href}}, nil, GraphOptions{}); err != nil {
			t.Errorf("%q refused: %v", href, err)
		}
	}
}

// ---- safety and accessibility ---------------------------------------------

func TestGraph_EscapesEveryCallerString(t *testing.T) {
	hostile := `<script>alert(1)</script>`
	nodes := []GraphNode{
		{ID: `"><x`, Label: hostile, Group: hostile, Href: "/a?q=<b>"},
		{ID: "b", Label: "b"},
	}
	l := mustLayout(t, nodes, []GraphEdge{{From: `"><x`, To: "b", Style: GraphEdgeDashed}}, GraphOptions{
		AriaLabel: hostile, Description: hostile, DashedLabel: hostile, EmphasisLabel: hostile, RankLabels: []string{hostile},
	})
	for name, out := range map[string]string{"svg": l.SVG(), "table": l.Table()} {
		if strings.Contains(out, "<script>") || strings.Contains(out, `"><x`) || strings.Contains(out, "<b>") {
			t.Errorf("%s did not escape caller text: %s", name, out)
		}
	}
}

func TestGraph_AccessibleNameDescriptionAndKeyboardPath(t *testing.T) {
	nodes, edges := egoFixture()
	l := mustEgo(t, "store.Resolve", nodes, edges, egoOptions())
	svg, table := l.SVG(), l.Table()
	for _, want := range []string{`role="img"`, `aria-label="store.Resolve neighbourhood"`, `<title>store.Resolve neighbourhood</title>`, "<desc>"} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %s", want)
		}
	}
	// The drawing is hidden from assistive technology as one unit; its name
	// and description carry the summary, and the table carries the structure.
	if !strings.Contains(svg, `</desc><g aria-hidden="true">`) || !strings.HasSuffix(svg, "</g></svg>") {
		t.Errorf("drawing must sit in one aria-hidden group after title/desc: %.300s", svg)
	}
	// Pointer-only links inside the picture; real links in the table.
	if n := strings.Count(svg, `<a href="`); n != strings.Count(svg, `tabindex="-1"`) || n == 0 {
		t.Errorf("every SVG link must be removed from the tab order (links=%d)", n)
	}
	if strings.Contains(table, "tabindex") || !strings.Contains(table, `<a href="/symbols/api.Handle">api.Handle</a>`) {
		t.Errorf("table must carry ordinary focusable links: %s", table)
	}
	for _, want := range []string{"<caption>store.Resolve neighbourhood: nodes</caption>", "<caption>store.Resolve neighbourhood: edges</caption>", `<th scope="col">Node</th>`, `<th scope="row">`, "<td>Heuristic call</td>", `<th scope="col">Reaches a test</th>`} {
		if !strings.Contains(table, want) {
			t.Errorf("table missing %s", want)
		}
	}
	// The legend names the encodings in words, not only by line style/color.
	for _, want := range []string{">Resolved call</text>", ">Heuristic call</text>", ">Reaches a test</text>", ">Focus</text>"} {
		if !strings.Contains(svg, want) {
			t.Errorf("legend missing %s", want)
		}
	}
}

// TestGraph_LinkedNodesAreFullSizeTargets: once any node links somewhere,
// every node box is at least 44px high, with or without a group line, so each
// pointer target meets the kit's 44px target rule. Unlinked graphs stay compact.
func TestGraph_LinkedNodesAreFullSizeTargets(t *testing.T) {
	rectH := regexp.MustCompile(`<a href="[^"]*" tabindex="-1"><rect [^>]*height="([0-9.]+)"`)
	for name, nodes := range map[string][]GraphNode{
		"one line":   {{ID: "a", Href: "/a"}, {ID: "b"}},
		"group line": {{ID: "a", Href: "/a", Group: "pkg/a"}, {ID: "b"}},
	} {
		l := mustLayout(t, nodes, []GraphEdge{{From: "a", To: "b"}}, GraphOptions{})
		m := rectH.FindAllStringSubmatch(l.SVG(), -1)
		if len(m) != 1 {
			t.Fatalf("%s: want one linked node box, got %d", name, len(m))
		}
		var h float64
		fmt.Sscan(m[0][1], &h)
		if h < 44 || l.nodeH < 44 {
			t.Errorf("%s: linked node box is %gpx high, below the 44px target", name, h)
		}
	}
	plain := mustLayout(t, []GraphNode{{ID: "a"}, {ID: "b"}}, []GraphEdge{{From: "a", To: "b"}}, GraphOptions{})
	if plain.nodeH != gNodeH1 {
		t.Errorf("unlinked one-line nodes = %gpx, want the compact %gpx", plain.nodeH, gNodeH1)
	}
}

func TestGraph_LongLabelsTruncateButStayWhole(t *testing.T) {
	long := "internal/console.(*Projection).ResolveEverySingleCallEdgeInTheFleet"
	l := mustLayout(t, []GraphNode{{ID: "a", Label: long}}, nil, GraphOptions{})
	svg := l.SVG()
	if !strings.Contains(svg, "…</text>") {
		t.Errorf("long label should be truncated with an ellipsis in the box: %s", svg)
	}
	if !strings.Contains(svg, "<title>"+long+"</title>") || !strings.Contains(l.Table(), long) {
		t.Error("the full label must survive in the tooltip and the table")
	}
}

// TestGraph_TableNamesRepeatedLabelsUniquely: the picture tells two String
// methods apart by position and group line; the table, the only structured
// route for keyboard and screen-reader users, must name each one uniquely too.
func TestGraph_TableNamesRepeatedLabelsUniquely(t *testing.T) {
	l := mustLayout(t, []GraphNode{
		{ID: "a.String", Label: "String", Group: "pkg/a", Href: "/s/a.String"},
		{ID: "b.String", Label: "String", Group: "pkg/b"},
		{ID: "main"},
	}, []GraphEdge{{From: "main", To: "a.String"}, {From: "a.String", To: "b.String", Style: GraphEdgeDashed}}, GraphOptions{})
	table := l.Table()
	for _, want := range []string{
		"<tr><td>main</td><td>String (pkg/a)</td>",
		"<tr><td>String (pkg/a)</td><td>String (pkg/b)</td><td>Dashed</td>",
		`<th scope="row"><a href="/s/a.String">String (pkg/a)</a></th>`,
		`<th scope="row">String (pkg/b)</th>`,
		`<th scope="row">main</th>`, // a unique label stays unqualified
	} {
		if !strings.Contains(table, want) {
			t.Errorf("table missing %s: %s", want, table)
		}
	}

	// Same label and same group: only the ID tells them apart, in the edges
	// table and in a collapsed node's member list.
	nodes := []GraphNode{{ID: "hub"}}
	var edges []GraphEdge
	for _, id := range []string{"x.T.String", "x.U.String", "x.V.String"} {
		nodes = append(nodes, GraphNode{ID: id, Label: "String", Group: "pkg/x"})
		edges = append(edges, GraphEdge{From: "hub", To: id})
	}
	l = mustLayout(t, nodes, edges, GraphOptions{MaxPerRank: 2})
	table = l.Table()
	for _, want := range []string{
		"<tr><td>hub</td><td>String (x.T.String)</td>",
		"<td>String (x.U.String), String (x.V.String)</td>",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("table missing %s: %s", want, table)
		}
	}
	rowHeads := regexp.MustCompile(`<th scope="row">(?:<a [^>]+>)?([^<]+)`).FindAllStringSubmatch(table, -1)
	seen := map[string]bool{}
	for _, m := range rowHeads {
		if seen[m[1]] {
			t.Errorf("two node rows are both named %q", m[1])
		}
		seen[m[1]] = true
	}
}

func TestGraph_TableEmitsNoClassStyleOrColor(t *testing.T) {
	nodes, edges := egoFixture()
	outs := map[string]string{
		"ego":     mustEgo(t, "store.Resolve", nodes, edges, egoOptions()).Table(),
		"bounded": mustEgo(t, "focus", wideNodes(t), wideEdges(t), EgoOptions{}).Table(),
	}
	for name, out := range outs {
		if strings.Contains(out, "class=") || strings.Contains(out, "style=") {
			t.Errorf("%s table carries class/style — it must stay plain semantic HTML: %s", name, out)
		}
		if m := literalColor.FindString(out); m != "" {
			t.Errorf("%s table carries literal color %q", name, m)
		}
	}
}

func wideNodes(t *testing.T) []GraphNode { n, _ := wideFixture(12); return n }
func wideEdges(t *testing.T) []GraphEdge { _, e := wideFixture(12); return e }

// TestGraph_RolePairsMeetContrastFloors measures every text and mark role pair
// the graph draws against the kit's own light and dark tokens — semantic-theme
// evidence for the default palette, not for an app-owned override.
func TestGraph_RolePairsMeetContrastFloors(t *testing.T) {
	src, err := os.ReadFile("../../../cue/tokens/kit.cue")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	type pair struct {
		fg, bg string
		floor  float64
	}
	pairs := []pair{
		{"ink", "surface-raised", tokens.TextFloor},          // node label
		{"ink-muted", "surface-raised", tokens.TextFloor},    // node group line
		{"ink", "accent-soft", tokens.TextFloor},             // focus label
		{"ink-muted", "accent-soft", tokens.TextFloor},       // focus group line
		{"ink", "surface-sunken", tokens.TextFloor},          // aggregate label
		{"ink-muted", "surface", tokens.TextFloor},           // headings, legend, "None"
		{"ink-muted", "surface", tokens.MeaningFloor},        // edges; aggregate border outside
		{"ink-muted", "surface-sunken", tokens.MeaningFloor}, // aggregate border inside
		{"accent", "surface", tokens.MeaningFloor},           // emphasized edges, focus border outside
		{"accent", "accent-soft", tokens.MeaningFloor},       // focus border inside
		// The legend gives the aggregate's dashed box a meaning ("Collapsed or
		// grouped nodes"), so that border is measured above. Normal node borders
		// and the empty-column outline use line-strong decoratively (about
		// 1.8:1): those boxes are identified by their text, measured above.
	}
	for _, scheme := range []string{"light", "dark"} {
		for _, p := range pairs {
			got, err := tokens.ContrastRoles(tok, p.fg, p.bg, scheme)
			if err != nil {
				t.Fatal(err)
			}
			if got < p.floor {
				t.Errorf("%s on %s (%s) = %.2f:1, below %.1f:1", p.fg, p.bg, scheme, got, p.floor)
			}
		}
	}
}

// TestGraph_AggregateCueIsMeasuredRole: an app-supplied aggregate has no
// "+N more" wording, so its dashed border is its only visual cue. The border
// and the matching legend sample use the measured ink-muted role, not the
// decorative line-strong one.
func TestGraph_AggregateCueIsMeasuredRole(t *testing.T) {
	l := mustLayout(t, []GraphNode{{ID: "in"}, {ID: "checks", Label: "3 policy checks", Kind: GraphNodeAggregate}},
		[]GraphEdge{{From: "in", To: "checks"}}, GraphOptions{})
	svg := l.SVG()
	aggBox := regexp.MustCompile(`<rect [^>]*fill="var\(--ak-color-surface-sunken\)" stroke="([^"]+)"[^>]*stroke-dasharray="3 3"/>`)
	m := aggBox.FindAllStringSubmatch(svg, -1)
	if len(m) != 2 { // the node and its legend sample
		t.Fatalf("aggregate boxes = %d, want the node and the legend sample: %s", len(m), svg)
	}
	for _, s := range m {
		if s[1] != cssColor("ink-muted") {
			t.Errorf("aggregate border uses %s; its meaning needs the measured ink-muted role", s[1])
		}
	}
	if !strings.Contains(svg, ">Collapsed or grouped nodes</text>") {
		t.Error("legend must name the aggregate cue in words")
	}
}

var graphDashRe = regexp.MustCompile(`stroke-dasharray="5 4"`)

func TestGraph_DashedMeaningIsNotColorOnly(t *testing.T) {
	nodes, edges := egoFixture()
	svg := mustEgo(t, "store.Resolve", nodes, edges, egoOptions()).SVG()
	// Two dashed edges plus the dashed legend sample.
	if got := len(graphDashRe.FindAllString(svg, -1)); got != 3 {
		t.Errorf("dashed encodings = %d, want 3", got)
	}
	if !strings.Contains(svg, `stroke-width="2.5"`) {
		t.Error("emphasis must also change stroke width, not only color")
	}
}

// graph.go — a deterministic layered graph (Sugiyama-lite) and ego graph.
package viz

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/app-kit/appkit/ui/internal/webref"
	"github.com/rivo/uniseg"
)

// Input bounds. LayoutGraph and LayoutEgo refuse larger inputs before doing
// any layout work; aggregate or filter upstream instead. Drawn nodes are
// separately bounded per column by GraphOptions.MaxPerRank.
const (
	GraphMaxNodes   = 2000
	GraphMaxEdges   = 8000
	GraphMaxColumns = 64
)

// ErrInvalidGraph wraps every input refusal from LayoutGraph and LayoutEgo.
// Nothing is rendered for refused input: there is no partial layout.
var ErrInvalidGraph = errors.New("viz: invalid graph input")

// GraphNodeKind selects a node's treatment. The zero value is GraphNodeNormal.
type GraphNodeKind string

const (
	GraphNodeNormal GraphNodeKind = "normal"
	// GraphNodeFocus is drawn with a thicker accent border and bold label —
	// the subject of an ego graph or the node a reader should start from.
	GraphNodeFocus GraphNodeKind = "focus"
	// GraphNodeAggregate is a node that stands for several others. The layout
	// also generates aggregates ("+N more") when a column exceeds MaxPerRank.
	GraphNodeAggregate GraphNodeKind = "aggregate"
)

// GraphEdgeStyle is an edge's line style. The zero value is GraphEdgeSolid.
// What solid and dashed MEAN is app-owned; name it with GraphOptions.SolidLabel
// and DashedLabel so the legend and table say it in words.
type GraphEdgeStyle string

const (
	GraphEdgeSolid  GraphEdgeStyle = "solid"
	GraphEdgeDashed GraphEdgeStyle = "dashed"
)

// GraphRankMode chooses how nodes are assigned to columns.
type GraphRankMode string

const (
	// GraphRankLongestPath (the default) places sources in column 0 and every
	// other node one column right of its furthest predecessor. Cycles are broken
	// deterministically; the edges that close them are drawn right-to-left and
	// counted in Notes.
	GraphRankLongestPath GraphRankMode = "longest-path"
	// GraphRankGiven uses each node's Rank (0-based column) as supplied.
	GraphRankGiven GraphRankMode = "given"
)

// GraphNode is one vertex. ID is the identity edges refer to; Label is what a
// reader sees (defaults to ID). Where several nodes share a Label, the table
// alternative qualifies it with the Group, or with the ID where label and
// group still collide, so every row and edge endpoint names exactly one node.
//
// Text fields must be valid UTF-8 without control characters other than tab,
// line feed and carriage return, so the SVG stays well-formed XML.
type GraphNode struct {
	ID    string
	Label string
	// Group is an optional secondary line (a package, file or owner). It is
	// drawn under the label and listed in the table alternative.
	Group string
	// Rank is the 0-based column, used only with GraphRankGiven.
	Rank int
	Kind GraphNodeKind
	// Href optionally links the node: a relative reference or an absolute
	// http(s) URL. Anything else is refused. The table alternative carries the
	// keyboard-reachable link; the SVG link is pointer-only (see LayoutGraph).
	// When any node links, every node box is at least 44px high so each
	// pointer target meets the 44px target size; give the table's links the
	// same minimum hit area in app CSS.
	Href string
}

// GraphEdge is a directed edge From → To between node IDs. Duplicate From/To
// pairs are merged (dashed if any duplicate is dashed, emphasized if any is
// emphasized) and the merge is counted in the table and Notes.
//
// Edges redirected onto a generated "+N more" node are merged only with edges
// of the same Style and Emphasis: the aggregate keeps one line per style, each
// drawn separately and counted in its own table row, so one dashed edge never
// restyles its resolved neighbours (or the reverse).
type GraphEdge struct {
	From, To string
	Style    GraphEdgeStyle
	// Emphasis draws the edge thicker and in the accent role — a non-color cue
	// (width) plus color, with the meaning named by EmphasisLabel.
	Emphasis bool
}

// GraphOptions configures LayoutGraph. Zero values give useful defaults.
type GraphOptions struct {
	// AriaLabel is the accessible name, the SVG <title> and the table
	// captions' prefix. Default "graph".
	AriaLabel string
	// Description is prepended to the generated SVG <desc> summary.
	Description string
	// RankMode defaults to GraphRankLongestPath.
	RankMode GraphRankMode
	// RankLabels names columns from the left. Supplying more labels than
	// computed columns adds empty (labelled) columns on the right.
	RankLabels []string
	// MaxPerRank bounds the nodes drawn in one column (default 8, range 2–50).
	// A fuller column keeps MaxPerRank-1 nodes — focus first, then nodes on an
	// emphasized edge, then by degree, then input order — and collapses the
	// rest into one "+N more" aggregate, stated in Notes.
	MaxPerRank int
	// Words for the legend and table. Defaults "Solid", "Dashed", "Emphasized".
	SolidLabel, DashedLabel, EmphasisLabel string
	// EmptyText is shown when there are no nodes. Default "No nodes to show".
	EmptyText string
	// HideLegend suppresses the legend the SVG otherwise draws when it uses
	// more than one encoding (dashed or emphasized edges, focus or aggregate
	// nodes). The table alternative always states each edge's meaning.
	HideLegend bool
}

// EgoOptions configures LayoutEgo. LayoutEgo computes the columns itself, so
// GraphOptions.RankMode and RankLabels must be left empty.
type EgoOptions struct {
	GraphOptions
	// Depth is how many steps to follow in each direction (default 2, 1–8).
	Depth int
	// Column headings. Defaults "Callers", "Callees" and "Focus"; columns more
	// than one step away read "Callers (depth 2)".
	UpstreamLabel, DownstreamLabel, FocusLabel string
}

// GraphLayout is a validated, positioned graph. SVG and Table render the same
// layout, so the picture and its table alternative cannot disagree.
type GraphLayout struct {
	title, desc string
	o           GraphOptions
	cols        []gCol
	nodes       []gNode // visible nodes; aggregates included
	order       [][]int // per column, visible node indices top to bottom
	edges       []gEdge // visible edges in visual order
	notes       []string
	w, h        float64
	top, innerH float64
	nodeH       float64
	hasHead     bool
	hasGroup    bool
	hasLink     bool
	legend      []gLegend
}

type gCol struct {
	label, empty string
	x, w         float64
}

type gNode struct {
	id, label, group, href string
	name                   string // unique words for the table alternative
	kind                   GraphNodeKind
	rank                   int
	members                []string // generated aggregates: collapsed names, input order
	generated              bool
	x, y, w                float64
	idx                    int
}

type gEdge struct {
	from, to         int
	dashed, emphasis bool
	count            int
	sy, ty           float64 // attachment heights for forward edges
	lane, lanes      int     // position among edges joining the same two nodes
}

type gLegend struct {
	kind, label string
}

// Geometry. Text widths are estimated conservatively per display cell (the
// kit sets no font; the consumer's does). A label that would not fit its
// box is truncated with an ellipsis; the full text is in the node's tooltip
// and in the table alternative.
const (
	gMargin    = 12.0
	gRankGap   = 64.0
	gGapY      = 12.0
	gNodeH1    = 28.0
	gNodeH2    = 40.0
	gTarget    = 44.0 // minimum node height once any node links (pointer target)
	gPadX      = 10.0
	gMinNodeW  = 72.0
	gMaxNodeW  = 200.0
	gHeadH     = 24.0
	gLegendH   = 20.0
	gCell11    = 7.0
	gCell10    = 6.4
	gLoopSpace = 24.0
	gArrow     = 7.0
	gBackShift = 5.0
	gLaneGap   = 4.0  // vertical offset between parallel back or same-column lines
	gLoopReach = 18.0 // self-loop control points, right of the node
)

// graphText reports whether s may appear in the output: valid UTF-8 with no
// control characters other than tab, line feed and carriage return, and no
// U+FFFE/U+FFFF. XML 1.0 forbids the rest, and html.EscapeString passes them
// through, so a standalone SVG would otherwise fail to parse.
func graphText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' || r == 0xFFFE || r == 0xFFFF {
			return false
		}
	}
	return true
}

func graphErr(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidGraph}, args...)...)
}

func (o GraphOptions) normalized() (GraphOptions, error) {
	if o.AriaLabel == "" {
		o.AriaLabel = "graph"
	}
	switch o.RankMode {
	case "":
		o.RankMode = GraphRankLongestPath
	case GraphRankLongestPath, GraphRankGiven:
	default:
		return o, graphErr("unknown RankMode %q", o.RankMode)
	}
	switch {
	case o.MaxPerRank == 0:
		o.MaxPerRank = 8
	case o.MaxPerRank < 2 || o.MaxPerRank > 50:
		return o, graphErr("MaxPerRank %d outside 2–50", o.MaxPerRank)
	}
	if len(o.RankLabels) > GraphMaxColumns {
		return o, graphErr("%d RankLabels exceed %d columns", len(o.RankLabels), GraphMaxColumns)
	}
	if o.SolidLabel == "" {
		o.SolidLabel = "Solid"
	}
	if o.DashedLabel == "" {
		o.DashedLabel = "Dashed"
	}
	if o.EmphasisLabel == "" {
		o.EmphasisLabel = "Emphasized"
	}
	if o.EmptyText == "" {
		o.EmptyText = "No nodes to show"
	}
	o.RankLabels = append([]string(nil), o.RankLabels...)
	texts := append([]string{o.AriaLabel, o.Description, o.SolidLabel, o.DashedLabel, o.EmphasisLabel, o.EmptyText}, o.RankLabels...)
	for _, s := range texts {
		if !graphText(s) {
			return o, graphErr("option text is not valid UTF-8 or contains a control character")
		}
	}
	return o, nil
}

// normalizeGraph validates nodes and edges and merges duplicate edges. It
// returns nodes in input order, edges in first-occurrence order and the number
// of duplicate edges merged.
func normalizeGraph(nodes []GraphNode, edges []GraphEdge) ([]gNode, []gEdge, int, error) {
	if len(nodes) > GraphMaxNodes {
		return nil, nil, 0, graphErr("%d nodes exceed the %d-node input bound", len(nodes), GraphMaxNodes)
	}
	if len(edges) > GraphMaxEdges {
		return nil, nil, 0, graphErr("%d edges exceed the %d-edge input bound", len(edges), GraphMaxEdges)
	}
	index := make(map[string]int, len(nodes))
	ns := make([]gNode, len(nodes))
	for i, n := range nodes {
		switch {
		case n.ID == "":
			return nil, nil, 0, graphErr("node %d has an empty ID", i)
		case len(n.ID) > 256:
			return nil, nil, 0, graphErr("node %d ID exceeds 256 bytes", i)
		case !graphText(n.ID) || !graphText(n.Label) || !graphText(n.Group):
			return nil, nil, 0, graphErr("node %d text is not valid UTF-8 or contains a control character", i)
		}
		if _, dup := index[n.ID]; dup {
			return nil, nil, 0, graphErr("duplicate node ID %q", n.ID)
		}
		index[n.ID] = i
		kind := n.Kind
		switch kind {
		case "":
			kind = GraphNodeNormal
		case GraphNodeNormal, GraphNodeFocus, GraphNodeAggregate:
		default:
			return nil, nil, 0, graphErr("node %q has unknown Kind %q", n.ID, n.Kind)
		}
		if n.Href != "" && !webref.Allowed(n.Href) {
			return nil, nil, 0, graphErr("node %q Href is not a relative or http(s) reference", n.ID)
		}
		label := n.Label
		if label == "" {
			label = n.ID
		}
		ns[i] = gNode{id: n.ID, label: label, group: n.Group, href: n.Href, kind: kind, rank: n.Rank}
	}
	type pair struct{ a, b int }
	seen := make(map[pair]int, len(edges))
	es := make([]gEdge, 0, len(edges))
	for i, e := range edges {
		a, okA := index[e.From]
		b, okB := index[e.To]
		if !okA || !okB {
			return nil, nil, 0, graphErr("edge %d refers to an unknown node (%q → %q)", i, e.From, e.To)
		}
		var dashed bool
		switch e.Style {
		case "", GraphEdgeSolid:
		case GraphEdgeDashed:
			dashed = true
		default:
			return nil, nil, 0, graphErr("edge %d has unknown Style %q", i, e.Style)
		}
		if j, ok := seen[pair{a, b}]; ok {
			es[j].dashed = es[j].dashed || dashed
			es[j].emphasis = es[j].emphasis || e.Emphasis
			es[j].count++
			continue
		}
		seen[pair{a, b}] = len(es)
		es = append(es, gEdge{from: a, to: b, dashed: dashed, emphasis: e.Emphasis, count: 1})
	}
	return ns, es, len(edges) - len(es), nil
}

// LayoutGraph validates and lays out a directed graph left to right.
//
// Columns come from GraphOptions.RankMode. Within a column, nodes are ordered
// by four barycenter sweeps (the mean position of their neighbours in the
// columns already placed), ties broken by the previous position and then input
// order, so the same input always yields the same picture. A column holding
// more than MaxPerRank nodes collapses its remainder into one "+N more" node;
// every such reduction is stated in Notes, the SVG <desc> and the table.
//
// The SVG is role="img" with <title> and <desc>; the drawing itself sits in an
// aria-hidden group. Node links inside it are pointer conveniences with
// tabindex="-1": keyboard and screen-reader users reach the same destinations
// through GraphLayout.Table, which renders them as ordinary links. Colors are token roles only; the output carries no class
// attribute and needs no CSS build (L3a).
func LayoutGraph(nodes []GraphNode, edges []GraphEdge, opts GraphOptions) (GraphLayout, error) {
	o, err := opts.normalized()
	if err != nil {
		return GraphLayout{}, err
	}
	ns, es, dup, err := normalizeGraph(nodes, edges)
	if err != nil {
		return GraphLayout{}, err
	}
	var notes []string
	ncol := 0
	switch o.RankMode {
	case GraphRankGiven:
		for _, n := range ns {
			if n.rank < 0 || n.rank >= GraphMaxColumns {
				return GraphLayout{}, graphErr("node %q Rank %d outside 0–%d", n.id, n.rank, GraphMaxColumns-1)
			}
			ncol = max(ncol, n.rank+1)
		}
	default:
		reversed := rankLongestPath(ns, es)
		for _, n := range ns {
			if n.rank >= GraphMaxColumns {
				return GraphLayout{}, graphErr("longest path needs more than %d columns", GraphMaxColumns)
			}
			ncol = max(ncol, n.rank+1)
		}
		if reversed > 0 {
			notes = append(notes, fmt.Sprintf("%d %s closing a cycle %s drawn right to left.", reversed, plural(reversed, "edge", "edges"), plural(reversed, "is", "are")))
		}
	}
	ncol = max(ncol, len(o.RankLabels))
	cols := make([]gCol, ncol)
	for c := range cols {
		if c < len(o.RankLabels) {
			cols[c].label = o.RankLabels[c]
		}
		cols[c].empty = "None"
	}
	if dup > 0 {
		notes = append(notes, fmt.Sprintf("%d duplicate %s merged.", dup, plural(dup, "edge was", "edges were")))
	}
	return buildGraphLayout(ns, es, cols, o, notes), nil
}

// LayoutEgo lays out the neighbourhood of one focus node: the focus in its own
// column, nodes that reach it (callers — From → … → focus) in columns to the
// left by distance, and nodes it reaches (callees) to the right, up to Depth
// steps each way. A node found in both directions goes to the nearer side,
// callees on a tie. Nodes beyond Depth or unconnected to the focus are omitted
// and counted in Notes. An empty side is drawn as an explicit "None" column.
// Every edge between shown nodes is drawn, including edges within a column
// and edges pointing right to left.
func LayoutEgo(focus string, nodes []GraphNode, edges []GraphEdge, opts EgoOptions) (GraphLayout, error) {
	if opts.RankMode != "" || len(opts.RankLabels) > 0 {
		return GraphLayout{}, graphErr("LayoutEgo computes columns; leave RankMode and RankLabels empty")
	}
	switch {
	case opts.Depth == 0:
		opts.Depth = 2
	case opts.Depth < 1 || opts.Depth > 8:
		return GraphLayout{}, graphErr("Depth %d outside 1–8", opts.Depth)
	}
	if opts.UpstreamLabel == "" {
		opts.UpstreamLabel = "Callers"
	}
	if opts.DownstreamLabel == "" {
		opts.DownstreamLabel = "Callees"
	}
	if opts.FocusLabel == "" {
		opts.FocusLabel = "Focus"
	}
	for _, s := range []string{opts.UpstreamLabel, opts.DownstreamLabel, opts.FocusLabel} {
		if !graphText(s) {
			return GraphLayout{}, graphErr("option text is not valid UTF-8 or contains a control character")
		}
	}
	o, err := opts.GraphOptions.normalized()
	if err != nil {
		return GraphLayout{}, err
	}
	o.RankMode = GraphRankGiven
	ns, es, dup, err := normalizeGraph(nodes, edges)
	if err != nil {
		return GraphLayout{}, err
	}
	f := -1
	for i, n := range ns {
		if n.id == focus {
			f = i
			break
		}
	}
	if f < 0 {
		return GraphLayout{}, graphErr("focus %q is not a node", focus)
	}
	out := make([][]int, len(ns))
	in := make([][]int, len(ns))
	for _, e := range es {
		if e.from != e.to {
			out[e.from] = append(out[e.from], e.to)
			in[e.to] = append(in[e.to], e.from)
		}
	}
	down := bfsDepth(f, out, opts.Depth)
	up := bfsDepth(f, in, opts.Depth)
	shown := make([]int, 0, len(ns))
	remap := make([]int, len(ns))
	maxUp, maxDown := 1, 1
	for i := range ns {
		remap[i] = -1
		switch {
		case i == f:
			ns[i].rank = 0
			ns[i].kind = GraphNodeFocus
		case down[i] > 0 && (up[i] < 0 || down[i] <= up[i]):
			ns[i].rank = down[i]
			maxDown = max(maxDown, down[i])
		case up[i] > 0:
			ns[i].rank = -up[i]
			maxUp = max(maxUp, up[i])
		default:
			continue
		}
		remap[i] = len(shown)
		shown = append(shown, i)
	}
	sub := make([]gNode, len(shown))
	for j, i := range shown {
		sub[j] = ns[i]
		sub[j].rank += maxUp
	}
	var subEdges []gEdge
	for _, e := range es {
		if remap[e.from] >= 0 && remap[e.to] >= 0 {
			e.from, e.to = remap[e.from], remap[e.to]
			subEdges = append(subEdges, e)
		}
	}
	cols := make([]gCol, maxUp+1+maxDown)
	for c := range cols {
		d := c - maxUp
		switch {
		case d < 0:
			cols[c].label = depthLabel(opts.UpstreamLabel, -d)
		case d == 0:
			cols[c].label = opts.FocusLabel
		default:
			cols[c].label = depthLabel(opts.DownstreamLabel, d)
		}
		cols[c].empty = "None"
	}
	var notes []string
	if omitted := len(ns) - len(shown); omitted > 0 {
		notes = append(notes, fmt.Sprintf("%d of %d supplied nodes %s more than %d %s from the focus or not connected to it, and %s not shown.",
			omitted, len(ns), plural(omitted, "is", "are"), opts.Depth, plural(opts.Depth, "step", "steps"), plural(omitted, "is", "are")))
	}
	if dup > 0 {
		notes = append(notes, fmt.Sprintf("%d duplicate %s merged.", dup, plural(dup, "edge was", "edges were")))
	}
	return buildGraphLayout(sub, subEdges, cols, o, notes), nil
}

func depthLabel(label string, d int) string {
	if d == 1 {
		return label
	}
	return fmt.Sprintf("%s (depth %d)", label, d)
}

// bfsDepth returns each node's step distance from start along adj, -1 when
// unreached within limit; start itself is 0.
func bfsDepth(start int, adj [][]int, limit int) []int {
	dist := make([]int, len(adj))
	for i := range dist {
		dist[i] = -1
	}
	dist[start] = 0
	queue := []int{start}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		if dist[u] == limit {
			continue
		}
		for _, v := range adj[u] {
			if dist[v] < 0 {
				dist[v] = dist[u] + 1
				queue = append(queue, v)
			}
		}
	}
	return dist
}

// rankLongestPath assigns ns[i].rank in place and returns how many edges were
// reversed to break cycles. A depth-first search in input order marks edges to
// a node still on the stack as reversed; ranks are then the longest path from a
// source over the resulting acyclic graph, visited in stable Kahn order.
func rankLongestPath(ns []gNode, es []gEdge) int {
	out := make([][]int, len(ns))
	for i, e := range es {
		if e.from != e.to {
			out[e.from] = append(out[e.from], i)
		}
	}
	reversed := make([]bool, len(es))
	state := make([]byte, len(ns))
	var visit func(u int)
	visit = func(u int) {
		state[u] = 1
		for _, i := range out[u] {
			v := es[i].to
			switch state[v] {
			case 0:
				visit(v)
			case 1:
				reversed[i] = true
			}
		}
		state[u] = 2
	}
	for u := range ns {
		if state[u] == 0 {
			visit(u)
		}
	}
	succ := make([][]int, len(ns))
	indeg := make([]int, len(ns))
	count := 0
	for i, e := range es {
		if e.from == e.to {
			continue
		}
		a, b := e.from, e.to
		if reversed[i] {
			a, b = b, a
			count++
		}
		succ[a] = append(succ[a], b)
		indeg[b]++
	}
	for i := range ns {
		ns[i].rank = 0
	}
	queue := make([]int, 0, len(ns))
	for i := range ns {
		if indeg[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, v := range succ[u] {
			ns[v].rank = max(ns[v].rank, ns[u].rank+1)
			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	return count
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func colName(cols []gCol, c int) string {
	if cols[c].label != "" {
		return cols[c].label
	}
	return fmt.Sprintf("Column %d", c+1)
}

// buildGraphLayout bounds each column, orders nodes and computes geometry.
// ns carry valid ranks in [0, len(cols)).
func buildGraphLayout(ns []gNode, es []gEdge, cols []gCol, o GraphOptions, notes []string) GraphLayout {
	l := GraphLayout{title: o.AriaLabel, o: o, cols: cols}
	nameNodes(ns)

	// ---- bound each column --------------------------------------------------
	degree := make([]int, len(ns))
	emph := make([]bool, len(ns))
	for _, e := range es {
		degree[e.from]++
		degree[e.to]++
		if e.emphasis {
			emph[e.from], emph[e.to] = true, true
		}
	}
	byCol := make([][]int, len(cols))
	for i, n := range ns {
		byCol[n.rank] = append(byCol[n.rank], i)
	}
	vis := make([]int, len(ns))
	l.order = make([][]int, len(cols))
	for c, members := range byCol {
		keep := members
		var collapsed []int
		if len(members) > o.MaxPerRank {
			cand := append([]int(nil), members...)
			sort.SliceStable(cand, func(i, j int) bool {
				a, b := cand[i], cand[j]
				if fa, fb := ns[a].kind == GraphNodeFocus, ns[b].kind == GraphNodeFocus; fa != fb {
					return fa
				}
				if emph[a] != emph[b] {
					return emph[a]
				}
				if degree[a] != degree[b] {
					return degree[a] > degree[b]
				}
				return a < b
			})
			keep = append([]int(nil), cand[:o.MaxPerRank-1]...)
			collapsed = append([]int(nil), cand[o.MaxPerRank-1:]...)
			sort.Ints(keep)
			sort.Ints(collapsed)
		}
		for _, i := range keep {
			vis[i] = len(l.nodes)
			l.order[c] = append(l.order[c], len(l.nodes))
			l.nodes = append(l.nodes, ns[i])
		}
		if len(collapsed) > 0 {
			agg := gNode{
				label:     fmt.Sprintf("+%d more", len(collapsed)),
				kind:      GraphNodeAggregate,
				rank:      c,
				generated: true,
			}
			// Several columns can each hold "+N more"; the table names the column.
			agg.name = agg.label + " (" + colName(cols, c) + ")"
			for _, i := range collapsed {
				vis[i] = len(l.nodes)
				agg.members = append(agg.members, ns[i].name)
			}
			l.order[c] = append(l.order[c], len(l.nodes))
			l.nodes = append(l.nodes, agg)
			notes = append(notes, fmt.Sprintf("%s shows %d of %d nodes; %d are collapsed into “+%d more” (limit %d per column).",
				colName(cols, c), len(keep), len(members), len(collapsed), len(collapsed), o.MaxPerRank))
		}
	}

	// ---- redirect edges onto visible nodes ----------------------------------
	// Edges landing on the same two visible nodes (only possible through a
	// generated aggregate: duplicates were merged in normalizeGraph) merge only
	// when their style and emphasis agree. Each style keeps its own line and
	// count, so the aggregate never restates a resolved edge as heuristic.
	type pair struct{ a, b int }
	type laneKey struct {
		pair
		dashed, emphasis bool
	}
	seen := make(map[laneKey]int, len(es))
	lanes := make(map[pair]int, len(es))
	internal := 0
	for _, e := range es {
		a, b := vis[e.from], vis[e.to]
		if a == b && l.nodes[a].generated {
			internal += e.count
			continue
		}
		k := laneKey{pair{a, b}, e.dashed, e.emphasis}
		if j, ok := seen[k]; ok {
			l.edges[j].count += e.count
			continue
		}
		seen[k] = len(l.edges)
		l.edges = append(l.edges, gEdge{from: a, to: b, dashed: e.dashed, emphasis: e.emphasis, count: e.count, lane: lanes[k.pair]})
		lanes[k.pair]++
	}
	for i, e := range l.edges {
		l.edges[i].lanes = lanes[pair{e.from, e.to}]
	}
	if internal > 0 {
		notes = append(notes, fmt.Sprintf("%d %s between collapsed nodes %s not drawn.", internal, plural(internal, "edge", "edges"), plural(internal, "is", "are")))
	}

	// ---- order within columns: barycenter sweeps ----------------------------
	pos := make([]float64, len(l.nodes))
	setPos := func(c int) {
		n := len(l.order[c])
		for i, v := range l.order[c] {
			pos[v] = float64(i) - float64(n-1)/2
		}
	}
	for c := range l.order {
		setPos(c)
	}
	// One neighbour entry per line between two nodes, not per style lane, so
	// splitting an aggregate's edges by style does not move it.
	nbrs := make([][]int, len(l.nodes))
	for _, e := range l.edges {
		if e.from != e.to && e.lane == 0 {
			nbrs[e.from] = append(nbrs[e.from], e.to)
			nbrs[e.to] = append(nbrs[e.to], e.from)
		}
	}
	reorder := func(c int, left bool) {
		type item struct {
			v   int
			bc  float64
			cur int
		}
		var items []item
		var pinned []int
		for cur, v := range l.order[c] {
			if l.nodes[v].generated {
				pinned = append(pinned, v)
				continue
			}
			sum, cnt := 0.0, 0
			for _, u := range nbrs[v] {
				if r := l.nodes[u].rank; (left && r < c) || (!left && r > c) {
					sum += pos[u]
					cnt++
				}
			}
			bc := pos[v]
			if cnt > 0 {
				bc = sum / float64(cnt)
			}
			items = append(items, item{v, bc, cur})
		}
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].bc != items[j].bc {
				return items[i].bc < items[j].bc
			}
			return items[i].cur < items[j].cur
		})
		next := make([]int, 0, len(l.order[c]))
		for _, it := range items {
			next = append(next, it.v)
		}
		l.order[c] = append(next, pinned...)
		setPos(c)
	}
	for iter := 0; iter < 4; iter++ {
		for c := 1; c < len(l.order); c++ {
			reorder(c, true)
		}
		for c := len(l.order) - 2; c >= 0; c-- {
			reorder(c, false)
		}
	}
	for _, col := range l.order {
		for i, v := range col {
			l.nodes[v].idx = i
		}
	}

	// ---- geometry -----------------------------------------------------------
	for _, n := range l.nodes {
		if n.group != "" && !n.generated {
			l.hasGroup = true
		}
		if n.href != "" {
			l.hasLink = true
		}
	}
	for _, c := range cols {
		if c.label != "" {
			l.hasHead = true
		}
	}
	l.nodeH = gNodeH1
	if l.hasGroup {
		l.nodeH = gNodeH2
	}
	if l.hasLink {
		// A node link is a pointer target: at least 44px, like every other
		// kit target. Unlinked graphs keep the compact boxes.
		l.nodeH = max(l.nodeH, gTarget)
	}
	maxCount := 1
	for _, col := range l.order {
		maxCount = max(maxCount, len(col))
	}
	l.top = gMargin
	if l.hasHead {
		l.top += gHeadH
	}
	l.innerH = float64(maxCount)*l.nodeH + float64(maxCount-1)*gGapY
	x := gMargin
	for c := range cols {
		w := gMinNodeW
		if cols[c].label != "" {
			w = max(w, min(gMaxNodeW, textW(cols[c].label, gCell11)))
		}
		if len(l.order[c]) == 0 {
			w = max(w, min(gMaxNodeW, textW(cols[c].empty, gCell11)+2*gPadX))
		}
		for _, v := range l.order[c] {
			n := l.nodes[v]
			w = max(w, min(gMaxNodeW, textW(n.label, gCell11)+2*gPadX))
			if n.group != "" {
				w = max(w, min(gMaxNodeW, textW(n.group, gCell10)+2*gPadX))
			}
		}
		cols[c].x, cols[c].w = x, w
		x += w + gRankGap
	}
	for c, col := range l.order {
		off := float64(maxCount-len(col)) * (l.nodeH + gGapY) / 2
		for i, v := range col {
			l.nodes[v].x = cols[c].x
			l.nodes[v].w = cols[c].w
			l.nodes[v].y = l.top + off + float64(i)*(l.nodeH+gGapY)
		}
	}
	// ---- ports: spread forward edges along the node sides -------------------
	outs := make([][]int, len(l.nodes)) // forward edges leaving on the right
	ins := make([][]int, len(l.nodes))  // forward edges entering on the left
	for i, e := range l.edges {
		l.edges[i].sy = l.nodes[e.from].y + l.nodeH/2
		l.edges[i].ty = l.nodes[e.to].y + l.nodeH/2
		if l.nodes[e.from].rank < l.nodes[e.to].rank {
			outs[e.from] = append(outs[e.from], i)
			ins[e.to] = append(ins[e.to], i)
		}
	}
	span := l.nodeH / 2
	spread := func(n gNode, list []int, other func(gEdge) int, set func(*gEdge, float64)) {
		if len(list) < 2 {
			return
		}
		sort.SliceStable(list, func(a, b int) bool {
			oa, ob := l.nodes[other(l.edges[list[a]])], l.nodes[other(l.edges[list[b]])]
			if oa.y != ob.y {
				return oa.y < ob.y
			}
			return oa.rank < ob.rank
		})
		for k, i := range list {
			set(&l.edges[i], n.y+(l.nodeH-span)/2+span*float64(k)/float64(len(list)-1))
		}
	}
	for v, n := range l.nodes {
		spread(n, outs[v], func(e gEdge) int { return e.to }, func(e *gEdge, y float64) { e.sy = y })
		spread(n, ins[v], func(e gEdge) int { return e.from }, func(e *gEdge, y float64) { e.ty = y })
	}

	graphW := gMargin
	if len(cols) > 0 {
		last := cols[len(cols)-1]
		graphW = last.x + last.w + gMargin
		// Self-loops and same-column arcs in the last column bulge right of it:
		// reserve room for their furthest control point plus the stroke, so the
		// viewBox never clips them (at least gLoopSpace, the historical room).
		reach := 0.0
		for _, e := range l.edges {
			if a := l.nodes[e.from]; a.rank == len(cols)-1 && a.rank == l.nodes[e.to].rank {
				reach = max(reach, l.arcReach(e))
			}
		}
		if reach > 0 {
			graphW += max(gLoopSpace, reach+2-gMargin)
		}
	} else {
		graphW = 240
	}

	// ---- edges in visual order: plain first, emphasized drawn on top --------
	sort.SliceStable(l.edges, func(i, j int) bool {
		a, b := l.edges[i], l.edges[j]
		if a.emphasis != b.emphasis {
			return !a.emphasis
		}
		na, nb := l.nodes[a.from], l.nodes[b.from]
		if na.rank != nb.rank {
			return na.rank < nb.rank
		}
		if na.idx != nb.idx {
			return na.idx < nb.idx
		}
		ta, tb := l.nodes[a.to], l.nodes[b.to]
		if ta.rank != tb.rank {
			return ta.rank < tb.rank
		}
		return ta.idx < tb.idx
	})

	// ---- legend --------------------------------------------------------------
	var hasSolid, hasDashed, hasEmph, hasFocus, hasAgg bool
	for _, e := range l.edges {
		hasSolid = hasSolid || !e.dashed
		hasDashed = hasDashed || e.dashed
		hasEmph = hasEmph || e.emphasis
	}
	for _, n := range l.nodes {
		hasFocus = hasFocus || n.kind == GraphNodeFocus
		hasAgg = hasAgg || n.kind == GraphNodeAggregate
	}
	if !o.HideLegend && (hasDashed || hasEmph || hasFocus || hasAgg) {
		if hasSolid && (hasDashed || hasEmph) {
			l.legend = append(l.legend, gLegend{"solid", o.SolidLabel})
		}
		if hasDashed {
			l.legend = append(l.legend, gLegend{"dashed", o.DashedLabel})
		}
		if hasEmph {
			l.legend = append(l.legend, gLegend{"emphasis", o.EmphasisLabel})
		}
		if hasFocus {
			l.legend = append(l.legend, gLegend{"focus", "Focus"})
		}
		if hasAgg {
			l.legend = append(l.legend, gLegend{"aggregate", "Collapsed or grouped nodes"})
		}
	}
	legendW := 0.0
	for _, it := range l.legend {
		legendW += legendItemW(it)
	}
	l.w = max(graphW, legendW+2*gMargin)
	l.h = l.top + l.innerH + gMargin
	if len(cols) == 0 {
		l.h = l.top + gNodeH1 + gMargin
	}
	if len(l.legend) > 0 {
		l.h += gLegendH + 8
	}

	visibleEdges := len(l.edges)
	summary := fmt.Sprintf("%d %s and %d %s in %d %s.",
		len(l.nodes), plural(len(l.nodes), "node", "nodes"), visibleEdges, plural(visibleEdges, "edge", "edges"),
		len(cols), plural(len(cols), "column", "columns"))
	parts := []string{}
	if o.Description != "" {
		parts = append(parts, o.Description)
	}
	parts = append(parts, summary)
	if len(l.nodes) == 0 {
		parts = append(parts, o.EmptyText+".")
	}
	for c := range cols {
		if len(l.order[c]) == 0 && len(l.nodes) > 0 {
			parts = append(parts, colName(cols, c)+": "+cols[c].empty+".")
		}
	}
	parts = append(parts, notes...)
	l.desc = strings.Join(parts, " ")
	l.notes = notes
	return l
}

func legendItemW(it gLegend) float64 { return 24 + 6 + textW(it.label, gCell11) + 18 }

// textW estimates the rendered width of s at one display cell = cell px.
func textW(s string, cell float64) float64 { return float64(uniseg.StringWidth(s)) * cell }

// fitText truncates s at a grapheme boundary so it fits maxW, adding "…".
func fitText(s string, maxW, cell float64) string {
	if textW(s, cell) <= maxW {
		return s
	}
	budget := maxW - cell
	var b strings.Builder
	w := 0.0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		cw := float64(g.Width()) * cell
		if w+cw > budget {
			break
		}
		b.WriteString(g.Str())
		w += cw
	}
	return b.String() + "…"
}

// Notes returns the declared reductions and adjustments this layout made:
// collapsed columns, cycle-closing edges, merged duplicates and (for LayoutEgo)
// omitted nodes. Empty means the picture shows every supplied node and edge.
func (l GraphLayout) Notes() []string { return append([]string(nil), l.notes...) }

// SVG renders the layout. Same layout → byte-identical output.
func (l GraphLayout) SVG() string {
	title := l.title
	if title == "" {
		title = "graph"
	}
	w, h := int(l.w+0.999), int(l.h+0.999)
	if w <= 0 || h <= 0 {
		w, h = 240, 52
	}
	var b strings.Builder
	b.WriteString(svgOpen(w, h, title, ""))
	b.WriteString(svgTitle(title))
	desc := l.desc
	if desc == "" {
		desc = "0 nodes and 0 edges in 0 columns."
	}
	b.WriteString("<desc>" + esc(desc) + "</desc>")
	// The drawing is one picture: its accessible name and description are the
	// <title> and <desc> above. Chrome otherwise exposes an SVG image's
	// children (and node links) to assistive technology; the table alternative
	// is the structured, navigable representation.
	b.WriteString(`<g aria-hidden="true">`)

	muted, ink := cssColor("ink-muted"), cssColor("ink")
	if len(l.cols) == 0 {
		empty := l.o.EmptyText
		if empty == "" {
			empty = "No nodes to show"
		}
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" dominant-baseline="middle" font-size="12" fill="%s">%s</text>`,
			fmtNum(float64(w)/2), fmtNum(float64(h)/2), muted, esc(empty))
		b.WriteString("</g></svg>")
		return b.String()
	}

	// Column headings.
	for _, col := range l.cols {
		if l.hasHead && col.label != "" {
			room := min(col.w+gRankGap-8, l.w-col.x-4)
			fmt.Fprintf(&b, `<text x="%s" y="%s" font-size="11" font-weight="600" fill="%s">%s</text>`,
				fmtNum(col.x), fmtNum(gMargin+12), muted, esc(fitText(col.label, room, gCell11)))
		}
	}

	// Edges, then empty-column placeholders and nodes on top of them.
	for _, e := range l.edges {
		l.writeEdge(&b, e)
	}
	for c, col := range l.cols {
		if len(l.order[c]) == 0 {
			y := l.top + (l.innerH-l.nodeH)/2
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="6" fill="%s" stroke="%s" stroke-dasharray="3 3"/>`,
				fmtNum(col.x), fmtNum(y), fmtNum(col.w), fmtNum(l.nodeH), cssColor("surface"), cssColor("line-strong"))
			fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" dominant-baseline="middle" font-size="11" fill="%s">%s</text>`,
				fmtNum(col.x+col.w/2), fmtNum(y+l.nodeH/2), muted, esc(col.empty))
		}
	}
	for c := range l.order {
		for _, v := range l.order[c] {
			l.writeNode(&b, l.nodes[v], ink, muted)
		}
	}

	// Legend.
	if len(l.legend) > 0 {
		y := l.top + l.innerH + 8 + gLegendH/2
		x := gMargin
		for _, it := range l.legend {
			switch it.kind {
			case "solid", "dashed", "emphasis":
				stroke, width, dash := muted, "1.5", ""
				if it.kind == "emphasis" {
					stroke, width = cssColor("accent"), "2.5"
				}
				if it.kind == "dashed" {
					dash = ` stroke-dasharray="5 4"`
				}
				fmt.Fprintf(&b, `<path d="M%s %s H%s" stroke="%s" stroke-width="%s"%s/>`,
					fmtNum(x), fmtNum(y), fmtNum(x+24), stroke, width, dash)
			case "focus":
				fmt.Fprintf(&b, `<rect x="%s" y="%s" width="24" height="12" rx="3" fill="%s" stroke="%s" stroke-width="2"/>`,
					fmtNum(x), fmtNum(y-6), cssColor("accent-soft"), cssColor("accent"))
			case "aggregate":
				fmt.Fprintf(&b, `<rect x="%s" y="%s" width="24" height="12" rx="3" fill="%s" stroke="%s" stroke-dasharray="3 3"/>`,
					fmtNum(x), fmtNum(y-6), cssColor("surface-sunken"), muted)
			}
			fmt.Fprintf(&b, `<text x="%s" y="%s" dominant-baseline="middle" font-size="11" fill="%s">%s</text>`,
				fmtNum(x+30), fmtNum(y), muted, esc(it.label))
			x += legendItemW(it)
		}
	}
	b.WriteString("</g></svg>")
	return b.String()
}

func (l GraphLayout) writeNode(b *strings.Builder, n gNode, ink, muted string) {
	fill, stroke, width, dash, weight, text := cssColor("surface-raised"), cssColor("line-strong"), "1", "", "", ink
	switch n.kind {
	case GraphNodeFocus:
		fill, stroke, width, weight = cssColor("accent-soft"), cssColor("accent"), "2", ` font-weight="600"`
	case GraphNodeAggregate:
		// The legend gives this dashed box a meaning, and an app-supplied
		// aggregate has no "+N more" words to fall back on, so its border uses
		// the ink-muted role measured at 3:1 against both fills, not the
		// decorative line-strong one.
		fill, stroke, dash = cssColor("surface-sunken"), muted, ` stroke-dasharray="3 3"`
	}
	b.WriteString("<g")
	if !n.generated {
		fmt.Fprintf(b, ` data-node-id="%s"`, esc(n.id))
	}
	b.WriteString(">")
	tip := n.label
	if n.group != "" {
		tip += " — " + n.group
	}
	if n.generated {
		shown := n.members
		if len(shown) > 8 {
			shown = shown[:8]
		}
		tip += ": " + strings.Join(shown, ", ")
		if len(n.members) > 8 {
			tip += ", …"
		}
	}
	b.WriteString("<title>" + esc(tip) + "</title>")
	if n.href != "" {
		fmt.Fprintf(b, `<a href="%s" tabindex="-1">`, esc(n.href))
	}
	fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="6" fill="%s" stroke="%s" stroke-width="%s"%s/>`,
		fmtNum(n.x), fmtNum(n.y), fmtNum(n.w), fmtNum(l.nodeH), fill, stroke, width, dash)
	room := n.w - 2*gPadX
	// One line sits centred; a label and group line sit 6px above and 8px
	// below the centre (14 and 28 in a 40px box).
	labelY := n.y + l.nodeH/2
	if l.hasGroup && n.group != "" {
		labelY -= 6
	}
	fmt.Fprintf(b, `<text x="%s" y="%s" dominant-baseline="middle" font-size="11" fill="%s"%s>%s</text>`,
		fmtNum(n.x+gPadX), fmtNum(labelY), text, weight, esc(fitText(n.label, room, gCell11)))
	if l.hasGroup && n.group != "" {
		fmt.Fprintf(b, `<text x="%s" y="%s" dominant-baseline="middle" font-size="10" fill="%s">%s</text>`,
			fmtNum(n.x+gPadX), fmtNum(n.y+l.nodeH/2+8), muted, esc(fitText(n.group, room, gCell10)))
	}
	if n.href != "" {
		b.WriteString("</a>")
	}
	b.WriteString("</g>")
}

// arcReach is how far right of its column a self-loop or same-column arc
// reaches: the offset of its control points. Arcs grow with the rows they
// span but stay inside the gap before the next column.
func (l GraphLayout) arcReach(e gEdge) float64 {
	if e.from == e.to {
		return gLoopReach
	}
	steps := l.nodes[e.from].idx - l.nodes[e.to].idx
	if steps < 0 {
		steps = -steps
	}
	return min(gRankGap-16, 16+8*float64(steps))
}

// writeEdge draws one edge. Every path ends on a horizontal tangent so each
// arrowhead is an axis-aligned triangle: no trigonometry, so output is exact
// across platforms.
func (l GraphLayout) writeEdge(b *strings.Builder, e gEdge) {
	a, t := l.nodes[e.from], l.nodes[e.to]
	stroke, width := cssColor("ink-muted"), "1.5"
	if e.emphasis {
		stroke, width = cssColor("accent"), "2.5"
	}
	dash := ""
	if e.dashed {
		dash = ` stroke-dasharray="5 4"`
	}
	// Parallel lines between the same two nodes (an aggregate's style lanes)
	// that are not spread along forward ports sit a few pixels apart.
	lane := (float64(e.lane) - float64(e.lanes-1)/2) * gLaneGap
	ay, ty := a.y+l.nodeH/2+lane, t.y+l.nodeH/2+lane
	if a.rank < t.rank {
		ay, ty = e.sy, e.ty
	}
	var d string
	var tipX, tipY float64
	pointsRight := true
	switch {
	case e.from == e.to: // self-loop on the right side
		x, reach := a.x+a.w, l.arcReach(e)
		y1, y2 := a.y+7, a.y+l.nodeH-7
		tipX, tipY, pointsRight = x, y2, false
		d = fmt.Sprintf("M%s %s C%s %s %s %s %s %s", fmtNum(x), fmtNum(y1), fmtNum(x+reach), fmtNum(y1), fmtNum(x+reach), fmtNum(y2), fmtNum(x+gArrow), fmtNum(y2))
	case a.rank < t.rank: // forward: right side → left side
		x1, x2 := a.x+a.w, t.x
		dx := (x2 - x1) / 2
		tipX, tipY = x2, ty
		d = fmt.Sprintf("M%s %s C%s %s %s %s %s %s", fmtNum(x1), fmtNum(ay), fmtNum(x1+dx), fmtNum(ay), fmtNum(x2-dx), fmtNum(ty), fmtNum(x2-gArrow), fmtNum(ty))
	case a.rank > t.rank: // backward: left side → right side, shifted down
		x1, x2 := a.x, t.x+t.w
		dx := (x1 - x2) / 2
		y1, y2 := ay+gBackShift, ty+gBackShift
		tipX, tipY, pointsRight = x2, y2, false
		d = fmt.Sprintf("M%s %s C%s %s %s %s %s %s", fmtNum(x1), fmtNum(y1), fmtNum(x1-dx), fmtNum(y1), fmtNum(x2+dx), fmtNum(y2), fmtNum(x2+gArrow), fmtNum(y2))
	default: // same column: arc on the right side
		x, bulge := a.x+a.w, l.arcReach(e)
		tipX, tipY, pointsRight = x, ty, false
		d = fmt.Sprintf("M%s %s C%s %s %s %s %s %s", fmtNum(x), fmtNum(ay), fmtNum(x+bulge), fmtNum(ay), fmtNum(x+bulge), fmtNum(ty), fmtNum(x+gArrow), fmtNum(ty))
	}
	fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s"%s/>`, d, stroke, width, dash)
	base := tipX - gArrow
	if !pointsRight {
		base = tipX + gArrow
	}
	fmt.Fprintf(b, `<path d="M%s %s L%s %s L%s %s Z" fill="%s"/>`,
		fmtNum(tipX), fmtNum(tipY), fmtNum(base), fmtNum(tipY-3.5), fmtNum(base), fmtNum(tipY+3.5), stroke)
}

// Table renders the table alternative for the same layout: a nodes table and
// an edges table (each with a <caption>), followed by the layout notes. It is
// plain semantic HTML with no class or style attributes, so the consuming app
// styles it with its own CSS. Node links are ordinary keyboard-reachable links.
func (l GraphLayout) Table() string {
	title := l.title
	if title == "" {
		title = "graph"
	}
	o := l.o
	if o.EmptyText == "" {
		o.EmptyText = "No nodes to show"
	}
	var hasGroup, hasMembers, hasMerged bool
	for _, n := range l.nodes {
		hasGroup = hasGroup || n.group != ""
		hasMembers = hasMembers || len(n.members) > 0
	}
	for _, e := range l.edges {
		hasMerged = hasMerged || e.count > 1
	}
	var b strings.Builder
	b.WriteString("<div>")
	b.WriteString("<table><caption>" + esc(title+": nodes") + "</caption><thead><tr>")
	heads := []string{"Node", "Column", "Kind"}
	if hasGroup {
		heads = append(heads, "Group")
	}
	if hasMembers {
		heads = append(heads, "Represents")
	}
	for _, h := range heads {
		b.WriteString(`<th scope="col">` + h + "</th>")
	}
	b.WriteString("</tr></thead><tbody>")
	rows := 0
	for c := range l.order {
		if len(l.order[c]) == 0 {
			// The picture draws an explicit placeholder; the table says the same.
			rows++
			b.WriteString(`<tr><th scope="row">` + esc(l.cols[c].empty) + "</th><td>" + esc(colName(l.cols, c)) + "</td><td>empty column</td>")
			if hasGroup {
				b.WriteString("<td></td>")
			}
			if hasMembers {
				b.WriteString("<td></td>")
			}
			b.WriteString("</tr>")
		}
		for _, v := range l.order[c] {
			n := l.nodes[v]
			rows++
			b.WriteString(`<tr><th scope="row">`)
			if n.href != "" {
				b.WriteString(`<a href="` + esc(n.href) + `">` + esc(n.name) + "</a>")
			} else {
				b.WriteString(esc(n.name))
			}
			b.WriteString("</th><td>" + esc(colName(l.cols, c)) + "</td><td>" + kindWord(n.kind) + "</td>")
			if hasGroup {
				b.WriteString("<td>" + esc(n.group) + "</td>")
			}
			if hasMembers {
				b.WriteString("<td>" + esc(strings.Join(n.members, ", ")) + "</td>")
			}
			b.WriteString("</tr>")
		}
	}
	if rows == 0 {
		fmt.Fprintf(&b, `<tr><td colspan="%d">%s</td></tr>`, len(heads), esc(o.EmptyText))
	}
	b.WriteString("</tbody></table>")

	b.WriteString("<table><caption>" + esc(title+": edges") + "</caption><thead><tr>")
	eheads := []string{"From", "To", "Line", esc(o.EmphasisLabel)}
	if hasMerged {
		eheads = append(eheads, "Merged edges")
	}
	for _, h := range eheads {
		b.WriteString(`<th scope="col">` + h + "</th>")
	}
	b.WriteString("</tr></thead><tbody>")
	edges := append([]gEdge(nil), l.edges...)
	sort.SliceStable(edges, func(i, j int) bool {
		na, nb := l.nodes[edges[i].from], l.nodes[edges[j].from]
		if na.rank != nb.rank {
			return na.rank < nb.rank
		}
		if na.idx != nb.idx {
			return na.idx < nb.idx
		}
		ta, tb := l.nodes[edges[i].to], l.nodes[edges[j].to]
		if ta.rank != tb.rank {
			return ta.rank < tb.rank
		}
		if ta.idx != tb.idx {
			return ta.idx < tb.idx
		}
		// An aggregate's style lanes: solid before dashed, plain before emphasized.
		if edges[i].emphasis != edges[j].emphasis {
			return !edges[i].emphasis
		}
		return !edges[i].dashed && edges[j].dashed
	})
	for _, e := range edges {
		line, em := o.SolidLabel, "no"
		if e.dashed {
			line = o.DashedLabel
		}
		if e.emphasis {
			em = "yes"
		}
		b.WriteString("<tr><td>" + esc(l.nodes[e.from].name) + "</td><td>" + esc(l.nodes[e.to].name) + "</td><td>" + esc(line) + "</td><td>" + em + "</td>")
		if hasMerged {
			fmt.Fprintf(&b, "<td>%d</td>", e.count)
		}
		b.WriteString("</tr>")
	}
	if len(edges) == 0 {
		fmt.Fprintf(&b, `<tr><td colspan="%d">No edges</td></tr>`, len(eheads))
	}
	b.WriteString("</tbody></table>")
	if len(l.notes) > 0 {
		b.WriteString("<ul>")
		for _, n := range l.notes {
			b.WriteString("<li>" + esc(n) + "</li>")
		}
		b.WriteString("</ul>")
	}
	b.WriteString("</div>")
	return b.String()
}

// nameNodes sets the words the table alternative uses for each node, so that
// every row, edge endpoint and collapsed-member list names exactly one node:
// the label alone where it is unique; qualified by the group where several
// nodes share the label; by the ID where label and group still collide.
func nameNodes(ns []gNode) {
	type labelGroup struct{ label, group string }
	byLabel := make(map[string]int, len(ns))
	byGroup := make(map[labelGroup]int, len(ns))
	for _, n := range ns {
		byLabel[n.label]++
		byGroup[labelGroup{n.label, n.group}]++
	}
	for i, n := range ns {
		switch {
		case byLabel[n.label] == 1:
			ns[i].name = n.label
		case n.group != "" && byGroup[labelGroup{n.label, n.group}] == 1:
			ns[i].name = n.label + " (" + n.group + ")"
		default:
			ns[i].name = n.label + " (" + n.id + ")"
		}
	}
	// A qualified name can still equal another node's plain label or group
	// qualifier; the ID, unique by contract, settles those.
	taken := make(map[string]int, len(ns))
	for _, n := range ns {
		taken[n.name]++
	}
	for i, n := range ns {
		if taken[n.name] > 1 {
			ns[i].name = n.label + " (" + n.id + ")"
		}
	}
}

func kindWord(k GraphNodeKind) string {
	switch k {
	case GraphNodeFocus:
		return "focus"
	case GraphNodeAggregate:
		return "aggregate"
	}
	return "node"
}

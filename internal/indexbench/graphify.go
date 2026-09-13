package indexbench

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// graphifyGraph is the subset of graphify's networkx node-link output
// (docs/bench/competitors.md's graphify entry, `graphify-out/graph.json`)
// LoadGraphifyEdges needs.
type graphifyGraph struct {
	Nodes []graphifyNode `json:"nodes"`
	Edges []graphifyEdge `json:"edges"`
	Links []graphifyEdge `json:"links"` // networkx node-link form, what v0.9.61 writes
}

type graphifyNode struct {
	ID             string          `json:"id"`
	Label          string          `json:"label"`
	FileType       string          `json:"file_type"`
	SourceFile     string          `json:"source_file"`
	SourceLocation string          `json:"source_location"`
	Community      json.RawMessage `json:"community,omitempty"`
}

type graphifyEdge struct {
	Source         string  `json:"source"`
	Target         string  `json:"target"`
	Relation       string  `json:"relation"`
	Confidence     string  `json:"confidence"`
	SourceFile     string  `json:"source_file"`
	SourceLocation string  `json:"source_location"`
	Weight         float64 `json:"weight"`
}

// graphifyKeptRelations are the relation kinds LoadGraphifyEdges turns into
// comparable call edges: both "calls" and "references" are symbol->symbol
// structural edges, scored against the same go/types call-graph truth
// fathom's own find_callers is scored against (M6, docs/vision/
// 27-strengths-bench.md §6). Every other relation graphify emits
// (imports_from, contains, method, mixes_in, uses, depends_on, inherits)
// answers a different question — package structure, membership, type
// hierarchy — that a call-graph truth cannot adjudicate, so it is dropped
// with a named count rather than silently discarded (BP-30's rule,
// extended to a loader).
var graphifyKeptRelations = map[string]bool{
	// Only "calls" is a caller→callee claim the go/types truth can judge.
	// "references" in v0.9.61 is mostly function→TYPE (a parameter or a
	// return type), which is not a call and not a reference site either;
	// it is dropped with its count so the row says what was left out.
	"calls": true,
}

// LoadGraphifyEdges reads a graphify graph.json (docs/bench/competitors.md)
// and returns its calls/references edges as a comparable EdgeSet: each kept
// edge's source and target node ids are resolved to their label through the
// node table (a source or target id absent from it is dropped and counted
// under "unresolved_source"/"unresolved_target"), and every other relation
// is dropped with a named count in EdgeSet.DroppedKinds. Every produced
// Edge carries Kind EdgeKindCall — see graphifyKeptRelations — so the
// result is scored by CompareEdges exactly as fathom's own find_callers
// answers are.
func LoadGraphifyEdges(path string) (EdgeSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EdgeSet{}, fmt.Errorf("indexbench: read graphify graph %s: %w", path, err)
	}
	var g graphifyGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return EdgeSet{}, fmt.Errorf("indexbench: parse graphify graph %s: %w", path, err)
	}
	labelByID := make(map[string]string, len(g.Nodes))
	for _, n := range g.Nodes {
		labelByID[n.ID] = n.Label
	}

	set := EdgeSet{DroppedKinds: map[string]int{}}
	edges := append(append([]graphifyEdge{}, g.Edges...), g.Links...)
	for _, e := range edges {
		if !graphifyKeptRelations[e.Relation] {
			set.DroppedKinds[e.Relation]++
			continue
		}
		from, ok := labelByID[e.Source]
		if !ok {
			set.DroppedKinds["unresolved_source"]++
			continue
		}
		to, ok := labelByID[e.Target]
		if !ok {
			set.DroppedKinds["unresolved_target"]++
			continue
		}
		set.Edges = append(set.Edges, Edge{From: graphifyLabelName(from), To: graphifyLabelName(to), Kind: EdgeKindCall})
	}
	sort.Slice(set.Edges, func(i, j int) bool {
		if set.Edges[i].To != set.Edges[j].To {
			return set.Edges[i].To < set.Edges[j].To
		}
		return set.Edges[i].From < set.Edges[j].From
	})
	if len(set.DroppedKinds) == 0 {
		set.DroppedKinds = nil
	}
	return set, nil
}

// graphifyLabelName turns a graphify node label into the name CompareEdges
// resolves: v0.9.61 labels Go functions "Name()" and methods ".Method()"
// (the receiver is not in the label — a bare method name resolves only
// when it is unambiguous in the truth, else it is counted unadjudicated).
func graphifyLabelName(label string) string {
	label = strings.TrimSuffix(strings.TrimSpace(label), "()")
	return strings.TrimPrefix(label, ".")
}

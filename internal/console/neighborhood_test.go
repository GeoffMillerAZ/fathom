package console

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// graphFixture is a call graph as adjacency: callee → callers.
type graphFixture struct {
	callers map[string][]NeighborEdge
	callees map[string][]NeighborEdge
}

func (g *graphFixture) add(from, to string, resolved bool) {
	if g.callers == nil {
		g.callers, g.callees = map[string][]NeighborEdge{}, map[string][]NeighborEdge{}
	}
	kind := ""
	if resolved {
		kind = "direct"
	}
	g.callers[to] = append(g.callers[to], NeighborEdge{Key: from, Resolved: resolved, Kind: kind})
	g.callees[from] = append(g.callees[from], NeighborEdge{Key: to, Resolved: resolved, Kind: kind})
}

func (g *graphFixture) input(focus string, bound int) NeighborhoodInput {
	return NeighborhoodInput{
		Service: "svc", Focus: focus, Bound: bound,
		Callers: func(k string) []NeighborEdge { return g.callers[k] },
		Callees: func(k string) []NeighborEdge { return g.callees[k] },
		IsTest:  func(k string) bool { return strings.Contains(k, ".Test") },
	}
}

// TestNeighborhood_BoundedAndAggregated pins OC-37: a symbol's callers and
// callees to depth 2, resolved and heuristic edges kept distinct, every
// column bounded with the surplus aggregated into one node that counts what
// it stands for by resolution — and the result is deterministic.
func TestNeighborhood_BoundedAndAggregated(t *testing.T) {
	var g graphFixture
	const focus = "pkg/store.Resolve"
	// 40 callers: 25 resolved, 15 heuristic, two of them tests.
	for i := 0; i < 40; i++ {
		g.add(fmt.Sprintf("pkg/api.Handler%02d", i), focus, i < 25)
	}
	g.add("pkg/store.TestResolve", focus, true)
	g.add("pkg/store.TestResolveMissing", focus, false)
	// Depth 2: a caller of a drawn caller, and one of an aggregated caller.
	g.add("cmd/main.run", "pkg/api.Handler00", true)
	g.add("cmd/main.hidden", "pkg/api.Handler39", false)
	// Callees: two, one resolved one heuristic, one of them with its own callee.
	g.add(focus, "pkg/index.Lookup", true)
	g.add(focus, "pkg/index.Guess", false)
	g.add("pkg/index.Lookup", "pkg/index.scan", true)
	// A cycle back into the focus: scan is both a depth-2 callee and a
	// direct caller. It is drawn once (callee side), its edge into the
	// focus drawn, never a second node.
	g.add("pkg/index.scan", focus, true)

	m := Neighborhood(g.input(focus, 6))
	if m.Depth != 2 || m.Bound != 6 {
		t.Fatalf("defaults: depth %d bound %d", m.Depth, m.Bound)
	}
	if m.CallersResolved != 27 || m.CallersHeuristic != 16 || m.CalleesResolved != 1 || m.CalleesHeuristic != 1 {
		t.Errorf("direct counts = %d/%d callers, %d/%d callees", m.CallersResolved, m.CallersHeuristic, m.CalleesResolved, m.CalleesHeuristic)
	}

	nodes := map[string]NeighborNode{}
	perColumn := map[string]int{}
	for _, n := range m.Nodes {
		nodes[n.ID] = n
		perColumn[fmt.Sprintf("%s:%d", n.Side, n.Depth)]++
	}
	for col, n := range perColumn {
		if n > 6 {
			t.Errorf("column %s draws %d nodes, over the bound of 6", col, n)
		}
	}
	agg, ok := nodes["more:caller:1"]
	if !ok || !agg.Aggregate || agg.Count != 42-5 || agg.Label != "+37 more callers" {
		t.Fatalf("the depth-1 callers must aggregate beyond the bound: %+v", agg)
	}
	if agg.AggregateResolved+agg.AggregateHeur != agg.Count {
		t.Errorf("aggregate splits %d+%d, counts %d", agg.AggregateResolved, agg.AggregateHeur, agg.Count)
	}
	// Resolved callers are drawn first; the first test among them is drawn.
	if !nodes["pkg/store.TestResolve"].Test {
		t.Errorf("a resolved test caller ranks ahead of plain resolved callers: %+v", m.Nodes)
	}
	for _, n := range m.Nodes {
		if n.Side == "caller" && n.Depth == 1 && !n.Aggregate && strings.HasPrefix(n.ID, "pkg/api.Handler") {
			var idx int
			fmt.Sscanf(strings.TrimPrefix(n.ID, "pkg/api.Handler"), "%d", &idx)
			if idx >= 25 {
				t.Errorf("heuristic caller %s drawn while resolved ones were aggregated", n.ID)
			}
		}
	}
	// Depth 2 expands only drawn nodes: run (caller of a drawn caller) is
	// drawn; hidden (caller of an aggregated one) is not.
	if _, ok := nodes["cmd/main.run"]; !ok {
		t.Error("a caller of a drawn caller must be drawn at depth 2")
	}
	if _, ok := nodes["cmd/main.hidden"]; ok {
		t.Error("an aggregated node is never expanded")
	}
	if n := nodes["pkg/index.scan"]; n.Side != "callee" || n.Depth != 2 {
		t.Errorf("callee at depth 2 = %+v", n)
	}

	// Edges: resolved and heuristic stay distinct, into the aggregate too.
	var sawRes, sawHeur, aggRes, aggHeur bool
	for _, l := range m.Links {
		if _, ok := nodes[l.From]; !ok {
			t.Errorf("edge from an undrawn node: %+v", l)
		}
		if _, ok := nodes[l.To]; !ok {
			t.Errorf("edge to an undrawn node: %+v", l)
		}
		switch {
		case l.From == "more:caller:1" && l.Resolved:
			aggRes = true
			if l.Count < 2 {
				t.Errorf("a merged edge into the aggregate carries its count: %+v", l)
			}
		case l.From == "more:caller:1":
			aggHeur = true
		case l.Resolved:
			sawRes = true
		default:
			sawHeur = true
		}
		if l.From == "pkg/store.TestResolve" && !l.TestReach {
			t.Error("an edge out of a test function is marked test-reaching")
		}
	}
	if !sawRes || !sawHeur || !aggRes || !aggHeur {
		t.Errorf("resolved/heuristic edges must stay distinct (drawn %v/%v, aggregate %v/%v)", sawRes, sawHeur, aggRes, aggHeur)
	}
	// The cycle back into the focus is drawn as an edge, not a new node.
	cycle := false
	for _, l := range m.Links {
		cycle = cycle || (l.From == "pkg/index.scan" && l.To == focus)
	}
	if !cycle {
		t.Error("an edge back into a drawn node must be drawn")
	}
	if m.AggregatedNodes != 37 || len(m.Notes) < 2 || !strings.Contains(strings.Join(m.Notes, " "), "aggregated") {
		t.Errorf("the reduction must be stated in words: %d aggregated, notes %v", m.AggregatedNodes, m.Notes)
	}

	// Deterministic: the same graph, the same JSON.
	a, _ := json.Marshal(m)
	b, _ := json.Marshal(Neighborhood(g.input(focus, 6)))
	if string(a) != string(b) {
		t.Error("the neighbourhood must be deterministic")
	}
	// A tighter bound keeps fewer, never zero, and still counts the rest.
	small := Neighborhood(g.input(focus, 2))
	if small.Bound != 2 || small.AggregatedNodes <= m.AggregatedNodes {
		t.Errorf("bound 2 = %+v", small)
	}
	// Nothing chosen, or a name that did not resolve, is said in words.
	if none := Neighborhood(NeighborhoodInput{Service: "svc"}); none.Unresolved == "" || len(none.Nodes) != 0 {
		t.Errorf("no focus = %+v", none)
	}
	if miss := Neighborhood(NeighborhoodInput{Service: "svc", Unresolved: "no symbol named Nope"}); miss.Unresolved != "no symbol named Nope" {
		t.Errorf("unresolved = %+v", miss)
	}
}

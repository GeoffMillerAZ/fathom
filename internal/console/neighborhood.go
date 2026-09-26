package console

// neighborhood.go — OC-37 (vision 35 §4.6 "the code neighborhood"; design
// §3 Index, §5 FATHOM-KIT-02): a symbol's callers and callees to a bounded
// depth, with resolved (type-checked) edges kept distinct from heuristic
// (syntax-only) ones, and every column bounded — the rest aggregated into
// one node that says how many it stands for and of which kind. A hub with
// five thousand callers renders as a handful of nodes and "+4,993 more", in
// the picture, the table alternative and the JSON alike.
//
// The projection is pure: the adapter supplies the graph as two lookup
// functions, so a test builds any shape it needs without a store.

import (
	"fmt"
	"sort"
	"strings"
)

// NeighborEdge is one caller or callee of a symbol as the call graph
// resolved it.
type NeighborEdge struct {
	Key string
	// Resolved: core.EdgeConfidenceOf calls this exact (caller, callee)
	// pair resolved — a typed pass confirmed it. false is a heuristic edge:
	// syntax-only, unresolved, or file-resolved (a kind, and still never
	// type-checked).
	Resolved bool
	// Kind is the stored edge kind ("direct", "iface-dispatch",
	// "file-resolved", …); "" for a syntax-only edge.
	Kind string
}

// NeighborhoodInput is one request.
type NeighborhoodInput struct {
	Service string
	// Focus is the symbol's canonical key; "" when the query did not
	// resolve (Unresolved then says why).
	Focus      string
	Unresolved string
	// Candidates are the qualified keys an ambiguous name could mean; the
	// page offers each as a link instead of guessing one.
	Candidates []string
	// Callers and Callees return a symbol's direct neighbours.
	Callers func(key string) []NeighborEdge
	Callees func(key string) []NeighborEdge
	// IsTest reports whether a key is a test function (its edges into the
	// neighbourhood are the test-reaching ones the page emphasizes).
	IsTest func(key string) bool
	// Depth is how many steps to follow each way (default 2, at most 3).
	Depth int
	// Bound is the most nodes one column shows before the rest are
	// aggregated (default 6, at least 2).
	Bound int
}

// NeighborNode is one drawn node.
type NeighborNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Group is the node's package path (the key before its last dot).
	Group string `json:"group,omitempty"`
	// Side is "focus", "caller" or "callee"; Depth is its distance.
	Side  string `json:"side"`
	Depth int    `json:"depth"`
	Test  bool   `json:"test,omitempty"`
	// Aggregate nodes stand for Count nodes that were not drawn, split by
	// how their edges resolved.
	Aggregate         bool `json:"aggregate,omitempty"`
	Count             int  `json:"count,omitempty"`
	AggregateResolved int  `json:"aggregate_resolved,omitempty"`
	AggregateHeur     int  `json:"aggregate_heuristic,omitempty"`
}

// NeighborLink is one drawn edge, From calls To.
type NeighborLink struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Resolved bool   `json:"resolved"`
	Kind     string `json:"kind,omitempty"`
	// TestReach marks an edge out of a test function.
	TestReach bool `json:"test_reach,omitempty"`
	// Count > 1 on an edge into an aggregate: how many edges it merges.
	Count int `json:"count,omitempty"`
}

// NeighborhoodModel is the panel's projection.
type NeighborhoodModel struct {
	Service    string         `json:"service"`
	Focus      string         `json:"focus,omitempty"`
	Unresolved string         `json:"unresolved,omitempty"`
	Candidates []string       `json:"candidates,omitempty"`
	Depth      int            `json:"depth"`
	Bound      int            `json:"bound"`
	Nodes      []NeighborNode `json:"nodes"`
	Links      []NeighborLink `json:"links"`
	// Callers / Callees are the focus's direct neighbour counts, split by
	// resolution — the whole set, before any bound.
	CallersResolved   int `json:"callers_resolved"`
	CallersHeuristic  int `json:"callers_heuristic"`
	CalleesResolved   int `json:"callees_resolved"`
	CalleesHeuristic  int `json:"callees_heuristic"`
	AggregatedNodes   int `json:"aggregated_nodes"`
	AggregatedColumns int `json:"aggregated_columns"`
	// Notes say, in words, every reduction the model made.
	Notes []string `json:"notes"`
}

// Neighborhood walks the focus's callers and callees breadth-first to
// Depth, keeping at most Bound nodes per column: resolved edges first, then
// test functions, then by key — deterministic, so the same graph always
// draws the same neighbourhood. A column's surplus becomes one aggregate
// node counting what it stands for, and a node is expanded to the next
// depth only when it was drawn.
func Neighborhood(in NeighborhoodInput) NeighborhoodModel {
	depth, bound := in.Depth, in.Bound
	if depth <= 0 {
		depth = 2
	}
	if depth > 3 {
		depth = 3
	}
	if bound <= 0 {
		bound = 6
	}
	if bound < 2 {
		bound = 2
	}
	m := NeighborhoodModel{Service: in.Service, Focus: in.Focus, Unresolved: in.Unresolved, Candidates: in.Candidates, Depth: depth, Bound: bound, Nodes: []NeighborNode{}, Links: []NeighborLink{}, Notes: []string{}}
	if in.Focus == "" {
		if m.Unresolved == "" {
			m.Unresolved = "no symbol chosen"
		}
		return m
	}
	isTest := in.IsTest
	if isTest == nil {
		isTest = func(string) bool { return false }
	}
	callers, callees := in.Callers, in.Callees
	if callers == nil {
		callers = func(string) []NeighborEdge { return nil }
	}
	if callees == nil {
		callees = func(string) []NeighborEdge { return nil }
	}
	for _, e := range callers(in.Focus) {
		if e.Resolved {
			m.CallersResolved++
		} else {
			m.CallersHeuristic++
		}
	}
	for _, e := range callees(in.Focus) {
		if e.Resolved {
			m.CalleesResolved++
		} else {
			m.CalleesHeuristic++
		}
	}

	m.Nodes = append(m.Nodes, neighborNode(in.Focus, "focus", 0, isTest(in.Focus)))
	seen := map[string]bool{in.Focus: true}
	drawn := map[string]bool{in.Focus: true}
	// Callees first: a node reachable both ways is drawn once, on the
	// callee side (the kit's LayoutEgo breaks the same tie the same way).
	m.walk("callee", callees, []string{in.Focus}, seen, drawn, depth, bound, isTest)
	m.walk("caller", callers, []string{in.Focus}, seen, drawn, depth, bound, isTest)
	m.Links = dedupeLinks(m.Links)

	if m.AggregatedNodes > 0 {
		m.Notes = append(m.Notes, fmt.Sprintf("%s beyond the bound of %d per column %s aggregated into %d \"more\" %s, each counting the edges it stands for by resolution.",
			countNoun(m.AggregatedNodes, "node"), bound, plural(m.AggregatedNodes, "is", "are"), m.AggregatedColumns, plural(m.AggregatedColumns, "node", "nodes")))
	}
	return m
}

// walk expands one side breadth-first. frontier holds the drawn nodes of
// the previous depth; each depth is one column.
func (m *NeighborhoodModel) walk(side string, next func(string) []NeighborEdge, frontier []string, seen, drawn map[string]bool, depth, bound int, isTest func(string) bool) {
	for d := 1; d <= depth && len(frontier) > 0; d++ {
		type cand struct {
			key   string
			edges []NeighborLink // edges between this candidate and the frontier
			res   bool           // any edge resolved
			test  bool
		}
		byKey := map[string]*cand{}
		var order []string
		for _, from := range frontier {
			for _, e := range next(from) {
				if e.Key == "" || seen[e.Key] {
					// An edge back into a drawn node is still drawn: every
					// edge between shown nodes is part of the picture. (A
					// node already folded into an aggregate stays counted
					// there, never drawn twice.)
					if e.Key != "" && drawn[e.Key] && e.Key != from {
						m.Links = append(m.Links, m.link(side, from, e, isTest))
					}
					continue
				}
				c := byKey[e.Key]
				if c == nil {
					c = &cand{key: e.Key, test: isTest(e.Key)}
					byKey[e.Key] = c
					order = append(order, e.Key)
				}
				c.edges = append(c.edges, m.link(side, from, e, isTest))
				c.res = c.res || e.Resolved
			}
		}
		if len(order) == 0 {
			return
		}
		sort.Slice(order, func(i, j int) bool {
			a, b := byKey[order[i]], byKey[order[j]]
			if a.res != b.res {
				return a.res
			}
			if a.test != b.test {
				return a.test
			}
			return a.key < b.key
		})
		keep := order
		var rest []string
		if len(order) > bound {
			keep, rest = order[:bound-1], order[bound-1:]
		}
		var nextFrontier []string
		for _, k := range keep {
			c := byKey[k]
			seen[k] = true
			drawn[k] = true
			nextFrontier = append(nextFrontier, k)
			m.Nodes = append(m.Nodes, neighborNode(k, side, d, c.test))
			m.Links = append(m.Links, c.edges...)
		}
		if len(rest) > 0 {
			agg := NeighborNode{ID: fmt.Sprintf("more:%s:%d", side, d), Side: side, Depth: d, Aggregate: true, Count: len(rest)}
			// One merged edge per (frontier node, resolution) into the
			// aggregate, each with its count: a heuristic edge never
			// restyles its resolved neighbours.
			type mk struct {
				from string
				res  bool
			}
			merged := map[mk]*NeighborLink{}
			var mkOrder []mk
			for _, k := range rest {
				c := byKey[k]
				seen[k] = true
				if c.res {
					agg.AggregateResolved++
				} else {
					agg.AggregateHeur++
				}
				for _, l := range c.edges {
					frontierEnd := l.To
					if side == "callee" {
						frontierEnd = l.From
					}
					key := mk{frontierEnd, l.Resolved}
					if merged[key] == nil {
						nl := NeighborLink{Resolved: l.Resolved}
						if side == "caller" {
							nl.From, nl.To = agg.ID, frontierEnd
						} else {
							nl.From, nl.To = frontierEnd, agg.ID
						}
						merged[key] = &nl
						mkOrder = append(mkOrder, key)
					}
					merged[key].Count++
				}
			}
			noun := "callers"
			if side == "callee" {
				noun = "callees"
			}
			agg.Label = fmt.Sprintf("+%s more %s", thousands(len(rest)), noun)
			m.Nodes = append(m.Nodes, agg)
			for _, k := range mkOrder {
				m.Links = append(m.Links, *merged[k])
			}
			m.AggregatedNodes += len(rest)
			m.AggregatedColumns++
			m.Notes = append(m.Notes, fmt.Sprintf("%s at depth %d: %s drawn, %s aggregated (%d resolved, %d heuristic).",
				noun, d, thousands(len(keep)), thousands(len(rest)), agg.AggregateResolved, agg.AggregateHeur))
		}
		frontier = nextFrontier
	}
}

// dedupeLinks keeps the first of any repeated (from, to, resolution) edge.
func dedupeLinks(in []NeighborLink) []NeighborLink {
	type k struct {
		from, to string
		res      bool
	}
	seen := map[k]bool{}
	out := make([]NeighborLink, 0, len(in))
	for _, l := range in {
		key := k{l.From, l.To, l.Resolved}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, l)
	}
	return out
}

// link orients one edge: on the caller side the neighbour calls the
// frontier node; on the callee side the frontier node calls it.
func (m *NeighborhoodModel) link(side, frontier string, e NeighborEdge, isTest func(string) bool) NeighborLink {
	l := NeighborLink{Resolved: e.Resolved, Kind: e.Kind}
	if side == "caller" {
		l.From, l.To = e.Key, frontier
	} else {
		l.From, l.To = frontier, e.Key
	}
	l.TestReach = isTest(l.From)
	return l
}

// neighborNode labels a key by its last segment and groups it by the rest.
func neighborNode(key, side string, depth int, test bool) NeighborNode {
	label, group := key, ""
	if i := strings.LastIndex(key, "."); i > 0 && i < len(key)-1 {
		// A method key ("pkg.Type.Method") keeps its receiver in the label.
		head := key[:i]
		if j := strings.LastIndex(head, "."); j > 0 && strings.ContainsAny(head[j+1:j+2], "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			label, group = key[j+1:], head[:j]
		} else {
			label, group = key[i+1:], head
		}
	}
	// A store key carries its service ("fathom::internal/x/x.Name"); the
	// group line shows the package alone.
	if i := strings.Index(group, "::"); i >= 0 {
		group = group[i+2:]
	}
	if i := strings.Index(label, "::"); i >= 0 {
		label = label[i+2:]
	}
	return NeighborNode{ID: key, Label: label, Group: group, Side: side, Depth: depth, Test: test}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

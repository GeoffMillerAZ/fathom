package callgraph

// selectreach.go — the forward reach `fathom tests select` reads (TQ-30,
// wave hon1, O1001): each test's reach under rapid type analysis, the rule
// golang.org/x/tools/go/callgraph/rta applies, over the stored edges (the
// selection's graph is BuildBoth's, not a go/ssa program, so the rule is
// applied here rather than by that package).
//
// An interface-dispatch edge into T's method is followed for a test only
// once something the test binary runs holds a value of T
// (core.EdgeInstantiates: a composite literal, new, a conversion, a
// declared variable, a call returning T): the test's own reach, or the
// initializers (func init, "<pkg>.init" variable initializers) of every
// package that reach touches, since a linked package's initializers run
// before the test (the CLI's verbs register their runner types from
// init, and a test that calls run reaches them only through dispatch).
// What an initializer reaches constructs types and links packages; it is
// not the test's reach, as before. A service whose typed edges carry no
// instantiation facts, or a method in a package the typed pass did not
// load, keeps the old reading: every implementation the typed pass knows.
//
// Under-approximations it inherits from ProductReach's linking: a package
// linked only by a blank import or by a constant no edge names runs its
// initializers unseen.
//
// Selection only: the graph, its stored edges and every other reader
// (blast_radius, localize, review's test_impact, ProductReach) are
// untouched; this file adds a reader.

import (
	"path"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// SelectionHit is one target a test reaches, and whether the walk's first
// path to it crossed an interface-dispatch edge.
type SelectionHit struct {
	Target      string
	ViaDispatch bool
}

// selEdge is one adjacency entry of the selection walk: the callee index,
// whether the edge is an interface dispatch, and the receiver type the
// callee waits on (-1: followed unconditionally).
type selEdge struct {
	to       int32
	dispatch bool
	waitType int32
}

// SelectionReach returns, for each test in tests (canonical keys or bare
// names; a name g does not know reaches nothing and is absent), the
// targets its forward reach touches under the rule above, sorted by
// target. Deterministic: adjacency is sorted and the walk is a BFS.
func SelectionReach(g *Graph, tests, targets []string) map[string][]SelectionHit {
	out := map[string][]SelectionHit{}
	if g == nil || len(tests) == 0 || len(targets) == 0 {
		return out
	}
	keys := make([]string, 0, len(g.nodes))
	for k := range g.nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	idx := make(map[string]int32, len(keys))
	for i, k := range keys {
		idx[k] = int32(i)
	}
	isTarget := make([]bool, len(keys))
	anyTarget := false
	for _, t := range targets {
		if i, ok := idx[g.canonicalize(t)]; ok {
			isTarget[i] = true
			anyTarget = true
		}
	}
	if !anyTarget {
		return out
	}

	rta := len(g.instantiates) > 0
	typeIdx := map[string]int32{}
	typeOf := func(id string) int32 {
		if i, ok := typeIdx[id]; ok {
			return i
		}
		i := int32(len(typeIdx))
		typeIdx[id] = i
		return i
	}
	adj := make([][]selEdge, len(keys))
	inst := make([][]int32, len(keys))
	for i, k := range keys {
		for _, next := range g.callees[k] {
			j, ok := idx[next]
			if !ok {
				continue
			}
			e := selEdge{to: j, waitType: -1}
			if g.edgeKind[k+"\x00"+next] == core.EdgeIfaceDispatch {
				e.dispatch = true
				if n := g.nodes[next]; rta && n.Kind == core.KindGoMethod && g.typedDirs[path.Dir(n.FilePath)] {
					e.waitType = typeOf(methodReceiverID(n))
				}
			}
			adj[i] = append(adj[i], e)
		}
		if rta {
			for _, tk := range g.instantiates[k] {
				inst[i] = append(inst[i], typeOf(typeKeyID(tk)))
			}
		}
	}
	// The link units (package directories) and their initializers: a
	// package the walk touches is linked into the test binary, and its init
	// functions run before the test.
	dirIdx := map[string]int32{}
	nodeDir := make([]int32, len(keys))
	var initsByDir [][]int32
	for i, k := range keys {
		n := g.nodes[k]
		d := linkUnit(n)
		di, ok := dirIdx[d]
		if !ok {
			di = int32(len(dirIdx))
			dirIdx[d] = di
			initsByDir = append(initsByDir, nil)
		}
		nodeDir[i] = di
		if n.Name == "init" {
			initsByDir[di] = append(initsByDir[di], int32(i))
		}
	}

	s := &selWalk{
		adj: adj, inst: inst, isTarget: isTarget, nodeDir: nodeDir, initsByDir: initsByDir, keys: keys,
		visited: make([]uint32, len(keys)), initVisited: make([]uint32, len(keys)), via: make([]bool, len(keys)),
		instStamp: make([]uint32, len(typeIdx)), pending: make([][]selItem, len(typeIdx)), linked: make([]uint32, len(dirIdx)),
	}
	for _, t := range tests {
		src, ok := idx[g.canonicalize(t)]
		if !ok {
			continue
		}
		if hits := s.walk(src); len(hits) > 0 {
			out[t] = hits
		}
	}
	return out
}

// selWalk is the per-test walk's state, stamped per walk so no array is
// cleared between tests.
type selWalk struct {
	adj        [][]selEdge
	inst       [][]int32
	isTarget   []bool
	nodeDir    []int32
	initsByDir [][]int32
	keys       []string

	stamp       uint32
	visited     []uint32 // reached from the test
	initVisited []uint32 // reached from a linked package's initializer
	via         []bool
	instStamp   []uint32
	linked      []uint32
	pending     [][]selItem
	touched     []int32
	queue       []selItem
	hits        []SelectionHit
}

// selItem is one node on the walk's queue: fromTest is false for code a
// linked initializer reaches (it constructs types and links packages, but
// a target it touches is not the test's reach), via whether the path
// crossed an interface-dispatch edge.
type selItem struct {
	node     int32
	fromTest bool
	via      bool
}

func (s *selWalk) visit(it selItem) {
	v := it.node
	if s.visited[v] == s.stamp {
		return
	}
	if !it.fromTest {
		if s.initVisited[v] == s.stamp {
			return
		}
		s.initVisited[v] = s.stamp
		s.queue = append(s.queue, it)
		return
	}
	s.visited[v] = s.stamp
	s.via[v] = it.via
	if s.isTarget[v] {
		s.hits = append(s.hits, SelectionHit{Target: s.keys[v], ViaDispatch: it.via})
	}
	s.queue = append(s.queue, it)
}

// walk is one test's BFS. A dispatch edge whose receiver type nothing the
// walk reached has constructed yet waits in pending until a reached node,
// the test's or a linked initializer's, constructs that type.
func (s *selWalk) walk(src int32) []SelectionHit {
	s.stamp++
	for _, t := range s.touched {
		s.pending[t] = s.pending[t][:0]
	}
	s.touched, s.queue, s.hits = s.touched[:0], s.queue[:0], nil
	s.visit(selItem{node: src, fromTest: true})
	for len(s.queue) > 0 {
		cur := s.queue[0]
		s.queue = s.queue[1:]
		u := cur.node
		if d := s.nodeDir[u]; s.linked[d] != s.stamp {
			s.linked[d] = s.stamp
			for _, in := range s.initsByDir[d] {
				s.visit(selItem{node: in})
			}
		}
		for _, t := range s.inst[u] {
			if s.instStamp[t] == s.stamp {
				continue
			}
			s.instStamp[t] = s.stamp
			for _, p := range s.pending[t] {
				s.visit(p)
			}
			s.pending[t] = s.pending[t][:0]
		}
		for _, e := range s.adj[u] {
			if s.visited[e.to] == s.stamp {
				continue
			}
			it := selItem{node: e.to, fromTest: cur.fromTest, via: cur.fromTest && (s.via[u] || e.dispatch)}
			if e.waitType >= 0 && s.instStamp[e.waitType] != s.stamp {
				if len(s.pending[e.waitType]) == 0 {
					s.touched = append(s.touched, e.waitType)
				}
				s.pending[e.waitType] = append(s.pending[e.waitType], it)
				continue
			}
			s.visit(it)
		}
	}
	hits := s.hits
	sort.Slice(hits, func(i, j int) bool { return hits[i].Target < hits[j].Target })
	return hits
}

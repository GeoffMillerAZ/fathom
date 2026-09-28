package callgraph

// condense.go — transitive reach over the SCC condensation of a Graph
// (TQ-2, docs/plans/test-intelligence.md; owner decision recorded in
// docs/design/test-intelligence.md §1): the call graph is collapsed into
// its strongly connected components, which form a DAG, and reach sets are
// computed once per component by a bitset union in topological order —
// never a BFS per symbol. Every member of a cycle reaches (and is reached
// by) every other member; a self-loop alone is not a cycle, so a symbol
// never counts itself.
//
// Memory: one bitset of ceil(components/64) words per component that is
// still needed. A component's set is released as soon as every component
// that consumes it has been computed (a reference count on the DAG's
// out-degree), so live sets track the frontier of the walk, not the whole
// graph.

import (
	"crypto/sha256"
	"encoding/hex"
	"math/bits"
	"sort"
)

// condensation is g's SCC decomposition over a deterministic node order.
type condensation struct {
	keys    []string // node index -> canonical key, sorted
	comp    []int    // node index -> component id
	members [][]int  // component id -> node indices (sorted)
	// order lists component ids sinks first (Tarjan's emission order): a
	// component appears after every component it calls.
	order []int
	succ  [][]int // component -> distinct successor components (DAG, no self)
	pred  [][]int // component -> distinct predecessor components
}

// condense runs an iterative Tarjan over g's callee adjacency.
func condense(g *Graph) condensation {
	keys := make([]string, 0, len(g.nodes))
	for k := range g.nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	idx := make(map[string]int, len(keys))
	for i, k := range keys {
		idx[k] = i
	}
	adj := make([][]int, len(keys))
	for i, k := range keys {
		for _, c := range g.callees[k] {
			if j, ok := idx[c]; ok {
				adj[i] = append(adj[i], j)
			}
		}
	}

	n := len(keys)
	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	comp := make([]int, n)
	var members [][]int
	var stack []int
	next := 0
	type frame struct{ v, edge int }
	for root := 0; root < n; root++ {
		if index[root] >= 0 {
			continue
		}
		call := []frame{{v: root}}
		index[root], low[root] = next, next
		next++
		stack = append(stack, root)
		onStack[root] = true
		for len(call) > 0 {
			f := &call[len(call)-1]
			if f.edge < len(adj[f.v]) {
				w := adj[f.v][f.edge]
				f.edge++
				if index[w] < 0 {
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					onStack[w] = true
					call = append(call, frame{v: w})
				} else if onStack[w] && index[w] < low[f.v] {
					low[f.v] = index[w]
				}
				continue
			}
			v := f.v
			call = call[:len(call)-1]
			if len(call) > 0 {
				p := call[len(call)-1].v
				if low[v] < low[p] {
					low[p] = low[v]
				}
			}
			if low[v] == index[v] {
				id := len(members)
				var m []int
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					comp[w] = id
					m = append(m, w)
					if w == v {
						break
					}
				}
				sort.Ints(m)
				members = append(members, m)
			}
		}
	}

	nc := len(members)
	succ := make([][]int, nc)
	pred := make([][]int, nc)
	for c := 0; c < nc; c++ {
		seen := map[int]bool{}
		for _, v := range members[c] {
			for _, w := range adj[v] {
				d := comp[w]
				if d == c || seen[d] {
					continue
				}
				seen[d] = true
				succ[c] = append(succ[c], d)
				pred[d] = append(pred[d], c)
			}
		}
		sort.Ints(succ[c])
	}
	for c := range pred {
		sort.Ints(pred[c])
	}
	order := make([]int, nc)
	for i := range order {
		order[i] = i // Tarjan emits sinks first: component ids ARE that order
	}
	return condensation{keys: keys, comp: comp, members: members, order: order, succ: succ, pred: pred}
}

type bitset []uint64

func newBitset(n int) bitset { return make(bitset, (n+63)/64) }
func (b bitset) set(i int)   { b[i/64] |= 1 << (uint(i) % 64) }
func (b bitset) union(o bitset) {
	for i := range b {
		b[i] |= o[i]
	}
}
func (b bitset) has(i int) bool { return b[i/64]&(1<<(uint(i)%64)) != 0 }

// closure computes, for each component in walk order, the set of
// components reachable along next (succ for descendants, pred for
// ancestors) and hands it to visit, then releases it once every consumer
// has read it. keep names components whose sets visit may retain.
func (c condensation) closure(walk []int, next, consumers [][]int, keep map[int]bool, visit func(comp int, set bitset)) {
	nc := len(c.members)
	sets := make([]bitset, nc)
	remaining := make([]int, nc)
	for i := range consumers {
		remaining[i] = len(consumers[i])
	}
	for _, x := range walk {
		s := newBitset(nc)
		for _, y := range next[x] {
			s.set(y)
			s.union(sets[y])
			remaining[y]--
			if remaining[y] == 0 && !keep[y] {
				sets[y] = nil
			}
		}
		sets[x] = s
		visit(x, s)
		if remaining[x] == 0 && !keep[x] {
			sets[x] = nil
		}
	}
}

// TransitiveCallerCounts returns, for every node in g, how many DISTINCT
// other nodes reach it through the caller relation — the transitive caller
// count TQ-2's centrality normalizes. Computed over the SCC condensation
// (one bitset union per component, sources first), never a BFS per node.
func TransitiveCallerCounts(g *Graph) map[string]int {
	c := condense(g)
	out := make(map[string]int, len(c.keys))
	walk := make([]int, len(c.order))
	for i, x := range c.order {
		walk[len(walk)-1-i] = x // sources first
	}
	c.closure(walk, c.pred, c.succ, nil, func(x int, anc bitset) {
		n := 0
		for w, word := range anc {
			for word != 0 {
				b := bits.TrailingZeros64(word)
				n += len(c.members[w*64+b])
				word &= word - 1
			}
		}
		if len(c.members[x]) > 1 {
			n += len(c.members[x]) - 1
		}
		for _, v := range c.members[x] {
			out[c.keys[v]] = n
		}
	})
	return out
}

// Reach is ForwardReach's answer: the forward (callee) reach set of each
// requested source, over g's condensation.
type Reach struct {
	c    condensation
	sets map[string]bitset // source key -> descendant components (plus its own, on a cycle)
}

// ForwardReach computes the forward reach set of each source in one walk
// over g's SCC condensation (sinks first). A source absent from g reaches
// nothing. Sources may be bare names or keys; the result is keyed by the
// string passed in.
func ForwardReach(g *Graph, sources []string) *Reach {
	c := condense(g)
	idx := make(map[string]int, len(c.keys))
	for i, k := range c.keys {
		idx[k] = i
	}
	want := map[int][]string{}
	keep := map[int]bool{}
	for _, s := range sources {
		k := g.canonicalize(s)
		if i, ok := idx[k]; ok {
			comp := c.comp[i]
			want[comp] = append(want[comp], s)
			keep[comp] = true
		}
	}
	r := &Reach{c: c, sets: map[string]bitset{}}
	c.closure(c.order, c.succ, c.pred, keep, func(x int, desc bitset) {
		srcs, ok := want[x]
		if !ok {
			return
		}
		s := append(bitset(nil), desc...)
		if len(c.members[x]) > 1 {
			s.set(x)
		}
		for _, src := range srcs {
			r.sets[src] = s
		}
	})
	return r
}

// Keys returns source's reach set as sorted canonical keys (nil when it
// reaches nothing or is unknown).
func (r *Reach) Keys(source string) []string {
	s, ok := r.sets[source]
	if !ok {
		return nil
	}
	var out []string
	for comp := range r.c.members {
		if s.has(comp) {
			for _, v := range r.c.members[comp] {
				out = append(out, r.c.keys[v])
			}
		}
	}
	sort.Strings(out)
	return out
}

// Fingerprint is a digest of source's reach set: two sources with the same
// fingerprint reach exactly the same symbols (exact set equality — the
// "redundant" test class compares these, never a near-duplicate score).
func (r *Reach) Fingerprint(source string) string {
	h := sha256.New()
	for _, k := range r.Keys(source) {
		h.Write([]byte(k))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

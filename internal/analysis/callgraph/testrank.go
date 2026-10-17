package callgraph

import (
	"path"
	"sort"
	"strings"
)

// RankedTest is one test RankedTestImpact returns: its key, the hops of the
// shortest reverse path from the symbol to it (a test that calls the
// symbol is 1), and the path distance between its directory and the
// symbol's (0: the same package directory).
type RankedTest struct {
	Key     string
	Hops    int
	DirDist int
}

// RankedTestImpact is TestImpact's set (the same reverse walk, every edge
// kind it follows) ranked by relevance (O1128): fewest hops first, then
// the nearest package (DirDist), then key. A budget that keeps a prefix
// can therefore never cut a direct test while it shows a deeper one.
// Deterministic. symbol may be a full key or bare name.
func RankedTestImpact(g *Graph, symbol string) []RankedTest {
	key := g.canonicalize(symbol)
	if key == "" {
		return []RankedTest{}
	}
	from := path.Dir(g.nodes[key].FilePath)
	hops := map[string]int{key: 0}
	queue := []string{key}
	var out []RankedTest
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, caller := range g.callers[cur] {
			if _, seen := hops[caller]; seen {
				continue
			}
			hops[caller] = hops[cur] + 1
			if n := g.nodes[caller]; strings.HasPrefix(n.Name, "Test") {
				out = append(out, RankedTest{Key: caller, Hops: hops[caller], DirDist: dirDistance(from, path.Dir(n.FilePath))})
			}
			queue = append(queue, caller)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hops != out[j].Hops {
			return out[i].Hops < out[j].Hops
		}
		if out[i].DirDist != out[j].DirDist {
			return out[i].DirDist < out[j].DirDist
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// dirDistance is the number of directory steps between two repo-relative
// directories: up from a to their common prefix, then down to b.
func dirDistance(a, b string) int {
	split := func(d string) []string {
		if d == "." || d == "" {
			return nil
		}
		return strings.Split(d, "/")
	}
	as, bs := split(a), split(b)
	common := 0
	for common < len(as) && common < len(bs) && as[common] == bs[common] {
		common++
	}
	return len(as) - common + len(bs) - common
}

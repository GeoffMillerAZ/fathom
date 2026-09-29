package explmap

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrUnknownMap is a map id nothing knows.
	ErrUnknownMap = errors.New("unknown map")
	// ErrCycle is an include that leads back to a map on its own route.
	ErrCycle = errors.New("include cycle")
	// ErrTooDeep is an include chain longer than MaxDepth.
	ErrTooDeep = errors.New("include chain deeper than the cap")
)

// Node is one map reached by a resolution, at its nearest depth.
type Node struct {
	ID string
	// Depth is the length of the nearest include route from the root (0 for
	// the root itself).
	Depth int
	// Route is that nearest route, root first.
	Route []string
}

// Resolution is a map's include DAG flattened: every reachable map exactly
// once, nearest first.
type Resolution struct {
	Root string
	// Nodes is breadth-first from the root; a map reached along several
	// routes appears once, at its nearest depth, and at equal depth the
	// earlier include comes first. Nodes[0] is the root.
	Nodes []Node
	// Height is the longest include chain below the root.
	Height int
	// Missing are include ids that resolve to no map, sorted. They are
	// reported so a reader sees the gap; they are never silently skipped.
	Missing []string
}

// ResolveMap walks the include references below root and returns them as a
// DAG (XM-2): shared bases are allowed and resolved once, a cycle is refused
// with its route named, and a chain longer than MaxDepth is refused with the
// longest route named, so expansion stays bounded however maps are composed.
func ResolveMap(root string, maps Lookup) (Resolution, error) {
	if _, ok := maps.Map(root); !ok {
		return Resolution{}, fmt.Errorf("%w %q", ErrUnknownMap, root)
	}
	// Depth-first with an on-route set: the cycle check and the longest
	// chain. height memoizes the longest chain below each finished map, so
	// a shared base is walked once however many routes reach it.
	height := map[string]int{}
	chain := map[string][]string{} // longest chain below a map, the map first
	onRoute := map[string]bool{}
	missing := map[string]bool{}
	var route []string
	var walk func(id string) error
	walk = func(id string) error {
		if onRoute[id] {
			cyc := append(append([]string{}, route[indexOf(route, id):]...), id)
			return fmt.Errorf("%w: %s", ErrCycle, strings.Join(cyc, " -> "))
		}
		if _, done := height[id]; done {
			return nil
		}
		m, ok := maps.Map(id)
		if !ok {
			missing[id] = true
			height[id] = 0
			return nil
		}
		onRoute[id] = true
		route = append(route, id)
		best, bestChain := 0, []string{id}
		for _, inc := range m.Includes {
			if err := walk(inc); err != nil {
				return err
			}
			if _, known := maps.Map(inc); !known {
				continue
			}
			if h := height[inc] + 1; h > best {
				best = h
				bestChain = append([]string{id}, chain[inc]...)
			}
		}
		route = route[:len(route)-1]
		onRoute[id] = false
		height[id] = best
		chain[id] = bestChain
		return nil
	}
	if err := walk(root); err != nil {
		return Resolution{}, err
	}
	if height[root] > MaxDepth {
		return Resolution{}, fmt.Errorf("%w of %d: %s (%d levels)", ErrTooDeep, MaxDepth, strings.Join(chain[root], " -> "), height[root])
	}

	// Breadth-first for the nearest depth and the priority order.
	res := Resolution{Root: root, Height: height[root]}
	seen := map[string]bool{root: true}
	queue := []Node{{ID: root, Depth: 0, Route: []string{root}}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		res.Nodes = append(res.Nodes, n)
		m, _ := maps.Map(n.ID)
		for _, inc := range m.Includes {
			if seen[inc] || missing[inc] {
				continue
			}
			seen[inc] = true
			r := append(append([]string{}, n.Route...), inc)
			queue = append(queue, Node{ID: inc, Depth: n.Depth + 1, Route: r})
		}
	}
	for id := range missing {
		res.Missing = append(res.Missing, id)
	}
	sort.Strings(res.Missing)
	return res, nil
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return 0
}

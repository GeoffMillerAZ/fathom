package catalog

import "sort"

// Variant is one item pinned on facet values — design §2.2's "a variant
// is that item pinned on those axes." Values holds exactly one value per
// axis the item declares; it is never partial.
type Variant struct {
	NodeID string
	Values map[string]string
}

// Variants expands n's Facets into its full variant set: the Cartesian
// product of every axis's allowed values, enumerated in deterministic
// order (axes sorted by name; each axis's values sorted; the last axis
// varies fastest). Facets are never turned into child Nodes — this is
// the whole reason facets are metadata, not tree levels: an item with 3
// facet axes yields many variants without a single new Node.
//
// An item with no facets has exactly one variant: itself, with an empty
// Values map.
func Variants(n Node) []Variant {
	if len(n.Facets) == 0 {
		return []Variant{{NodeID: n.ID(), Values: map[string]string{}}}
	}

	axes := make([]string, 0, len(n.Facets))
	for axis := range n.Facets {
		axes = append(axes, axis)
	}
	sort.Strings(axes)

	values := make([][]string, len(axes))
	total := 1
	for i, axis := range axes {
		vs := append([]string(nil), n.Facets[axis]...)
		sort.Strings(vs)
		values[i] = vs
		total *= len(vs)
	}
	if total == 0 {
		return nil
	}

	out := make([]Variant, 0, total)
	idx := make([]int, len(axes))
	for {
		v := make(map[string]string, len(axes))
		for i, axis := range axes {
			v[axis] = values[i][idx[i]]
		}
		out = append(out, Variant{NodeID: n.ID(), Values: v})

		pos := len(axes) - 1
		for pos >= 0 {
			idx[pos]++
			if idx[pos] < len(values[pos]) {
				break
			}
			idx[pos] = 0
			pos--
		}
		if pos < 0 {
			break
		}
	}
	return out
}

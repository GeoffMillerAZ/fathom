package catalog

import "sort"

// RollupEntry is one node's descendant count.
type RollupEntry struct {
	ID    string
	Count int // descendants (children, grandchildren, ...); not including itself
}

// Rollup returns a descendant count per node, sorted by ID — "N packages
// in M groups" (design §2.3), computed on read, never stored.
func Rollup(cat Catalog) []RollupEntry {
	out := make([]RollupEntry, 0, len(cat.Nodes))
	for _, n := range cat.Nodes {
		out = append(out, RollupEntry{ID: n.ID(), Count: len(cat.Descendants(n.ID()))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// DepthBucket is the node count at one depth.
type DepthBucket struct {
	Depth int
	Count int
}

// DepthHistogram counts nodes per depth, sorted by Depth ascending.
func DepthHistogram(cat Catalog) []DepthBucket {
	counts := map[int]int{}
	for _, n := range cat.Nodes {
		counts[Depth(n.ID())]++
	}
	depths := make([]int, 0, len(counts))
	for d := range counts {
		depths = append(depths, d)
	}
	sort.Ints(depths)
	out := make([]DepthBucket, 0, len(depths))
	for _, d := range depths {
		out = append(out, DepthBucket{Depth: d, Count: counts[d]})
	}
	return out
}

// FacetCoverage is how many nodes declare a given value on one axis.
type FacetCoverage struct {
	Axis  string
	Value string
	Count int
}

// CoverageByFacet counts, for every (axis, value) pair declared anywhere
// in cat, how many nodes declare it — sorted by Axis then Value.
func CoverageByFacet(cat Catalog) []FacetCoverage {
	type key struct{ axis, value string }
	counts := map[key]int{}
	for _, n := range cat.Nodes {
		for axis, vals := range n.Facets {
			for _, v := range vals {
				counts[key{axis, v}]++
			}
		}
	}
	keys := make([]key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].axis != keys[j].axis {
			return keys[i].axis < keys[j].axis
		}
		return keys[i].value < keys[j].value
	})
	out := make([]FacetCoverage, 0, len(keys))
	for _, k := range keys {
		out = append(out, FacetCoverage{Axis: k.axis, Value: k.value, Count: counts[k]})
	}
	return out
}

// Orphan is a node whose ParentID does not resolve to any node in the catalog.
type Orphan struct {
	ID       string
	ParentID string
}

// Orphans finds every node whose declared parent does not exist, sorted by ID.
func Orphans(cat Catalog) []Orphan {
	ids := make(map[string]bool, len(cat.Nodes))
	for _, n := range cat.Nodes {
		ids[n.ID()] = true
	}
	var out []Orphan
	for _, n := range cat.Nodes {
		if n.ParentID != "" && !ids[n.ParentID] {
			out = append(out, Orphan{ID: n.ID(), ParentID: n.ParentID})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// DuplicateGroup is a set of sibling nodes claiming the same Slug under
// the same ParentID — a structural conflict, since they would collide on
// ID. Neither New nor Build reject this outright; DuplicateSlugs is how a
// caller checks for it explicitly.
type DuplicateGroup struct {
	ParentID string
	Slug     string
	Count    int
}

// DuplicateSlugs finds every (ParentID, Slug) pair claimed by more than
// one node, sorted by ParentID then Slug.
func DuplicateSlugs(cat Catalog) []DuplicateGroup {
	type key struct{ parentID, slug string }
	counts := map[key]int{}
	for _, n := range cat.Nodes {
		counts[key{n.ParentID, n.Slug}]++
	}
	var out []DuplicateGroup
	for k, c := range counts {
		if c > 1 {
			out = append(out, DuplicateGroup{ParentID: k.parentID, Slug: k.slug, Count: c})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ParentID != out[j].ParentID {
			return out[i].ParentID < out[j].ParentID
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// DuplicateStableKey is a StableKey claimed by more than one node — an
// authoring error (or an overlay colliding with a base node) that makes
// StableKey-based lookup ambiguous. Verify and Build both refuse rather
// than build a lossy key index when this exists (see verify.go's
// DuplicateStableKeyError and build.go's LayerStableKeyCollisionError):
// a drift gate that cannot trust its own StableKey index must say so, not
// silently pick one node and drop the rest.
type DuplicateStableKey struct {
	StableKey string
	IDs       []string // every node ID claiming StableKey, sorted
}

// DuplicateStableKeys finds every non-empty StableKey claimed by more
// than one node in cat, sorted by StableKey (IDs sorted within each
// group). Nodes with an empty StableKey are never compared — an empty
// StableKey means "no stable key," not a shared one.
func DuplicateStableKeys(cat Catalog) []DuplicateStableKey {
	byKey := map[string][]string{}
	for _, n := range cat.Nodes {
		if n.StableKey == "" {
			continue
		}
		byKey[n.StableKey] = append(byKey[n.StableKey], n.ID())
	}
	var out []DuplicateStableKey
	for key, ids := range byKey {
		if len(ids) < 2 {
			continue
		}
		sorted := append([]string(nil), ids...)
		sort.Strings(sorted)
		out = append(out, DuplicateStableKey{StableKey: key, IDs: sorted})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StableKey < out[j].StableKey })
	return out
}

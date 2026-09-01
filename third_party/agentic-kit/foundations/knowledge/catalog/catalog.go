package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// Catalog is an immutable, flat set of Nodes — this package's own "node
// set" vocabulary. There is no pointer tree: every relationship (parent,
// child, ancestor, descendant) is derived from ID and ParentID on read,
// which is what makes arbitrary depth free — nothing here knows or cares
// how deep a catalog goes.
type Catalog struct {
	// Nodes is sorted by ID. Treat it as read-only: New and Build are the
	// only constructors, and nothing in this package mutates a Catalog
	// once returned.
	Nodes []Node
}

// New builds a Catalog from an unordered slice of nodes: validates every
// slug, then sorts by ID, so two independently constructed node sets with
// the same content always produce byte-identical Catalogs (and therefore
// byte-identical renders).
func New(nodes []Node) (Catalog, error) {
	out := make([]Node, len(nodes))
	copy(out, nodes)
	for i, n := range out {
		if err := ValidateSlug(n.Slug); err != nil {
			return Catalog{}, fmt.Errorf("catalog: node %d: %w", i, err)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return Catalog{Nodes: out}, nil
}

// Get returns the node with the given ID and whether it was found.
func (c Catalog) Get(id string) (Node, bool) {
	i := sort.Search(len(c.Nodes), func(i int) bool { return c.Nodes[i].ID() >= id })
	if i < len(c.Nodes) && c.Nodes[i].ID() == id {
		return c.Nodes[i], true
	}
	return Node{}, false
}

// Children returns id's immediate children, sorted by Order then Slug.
// id == "" returns the catalog's top-level nodes.
func (c Catalog) Children(id string) []Node {
	var out []Node
	for _, n := range c.Nodes {
		if n.ParentID == id {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// Ancestors returns id's ancestors, ordered from the root down to (but
// excluding) id itself. Returns nil if id is not in the catalog or is
// already top-level.
func (c Catalog) Ancestors(id string) []Node {
	cur, ok := c.Get(id)
	if !ok {
		return nil
	}
	var chain []Node
	for cur.ParentID != "" {
		parent, ok := c.Get(cur.ParentID)
		if !ok {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// Descendants returns every node nested under id (children,
// grandchildren, ...), sorted by ID. Because a node's ID is the
// slash-joined path of its ancestors, "nested under id" is exactly "ID
// has prefix id+"/"" — traversal needs no recursion and no depth limit,
// which is exactly what makes arbitrary depth free.
func (c Catalog) Descendants(id string) []Node {
	prefix := id + "/"
	var out []Node
	for _, n := range c.Nodes {
		if strings.HasPrefix(n.ID(), prefix) {
			out = append(out, n)
		}
	}
	return out
}

// ByDepth returns every node whose Depth is within [min, max] (inclusive), sorted by ID.
func (c Catalog) ByDepth(min, max int) []Node {
	var out []Node
	for _, n := range c.Nodes {
		d := Depth(n.ID())
		if d >= min && d <= max {
			out = append(out, n)
		}
	}
	return out
}

// FacetPredicate decides whether a node's facet map matches a query.
type FacetPredicate func(facets map[string][]string) bool

// ByFacet returns every node whose Facets satisfy pred, sorted by ID.
func (c Catalog) ByFacet(pred FacetPredicate) []Node {
	var out []Node
	for _, n := range c.Nodes {
		if pred(n.Facets) {
			out = append(out, n)
		}
	}
	return out
}

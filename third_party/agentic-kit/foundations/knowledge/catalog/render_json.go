package catalog

import (
	"encoding/json"
	"sort"
)

// jsonNode is the wire shape for one node in RenderJSON's output. Facets
// is a slice, not a map, so json.Marshal's already-deterministic struct
// encoding never has to fall back on Go's randomized map key order.
type jsonNode struct {
	ID         string         `json:"id"`
	ParentID   string         `json:"parent_id,omitempty"`
	Title      string         `json:"title,omitempty"`
	Synopsis   string         `json:"synopsis,omitempty"`
	Order      int            `json:"order,omitempty"`
	StableKey  string         `json:"stable_key,omitempty"`
	Hidden     bool           `json:"hidden,omitempty"`
	Facets     []jsonFacet    `json:"facets,omitempty"`
	Provenance jsonProvenance `json:"provenance"`
}

type jsonFacet struct {
	Axis   string   `json:"axis"`
	Values []string `json:"values"`
}

type jsonProvenance struct {
	Source             string   `json:"source,omitempty"`
	Extractor          string   `json:"extractor,omitempty"`
	Watermark          string   `json:"watermark,omitempty"`
	ContributingLayers []string `json:"contributing_layers,omitempty"`
	Overridden         bool     `json:"overridden,omitempty"`
	OverriddenBy       string   `json:"overridden_by,omitempty"`
}

func toJSONNode(n Node) jsonNode {
	axes := make([]string, 0, len(n.Facets))
	for axis := range n.Facets {
		axes = append(axes, axis)
	}
	sort.Strings(axes)
	facets := make([]jsonFacet, 0, len(axes))
	for _, axis := range axes {
		vals := append([]string(nil), n.Facets[axis]...)
		sort.Strings(vals)
		facets = append(facets, jsonFacet{Axis: axis, Values: vals})
	}
	return jsonNode{
		ID:        n.ID(),
		ParentID:  n.ParentID,
		Title:     n.Title,
		Synopsis:  n.Synopsis,
		Order:     n.Order,
		StableKey: n.StableKey,
		Hidden:    n.Hidden,
		Facets:    facets,
		Provenance: jsonProvenance{
			Source:             n.Provenance.Source,
			Extractor:          n.Provenance.Extractor,
			Watermark:          n.Provenance.Watermark,
			ContributingLayers: n.Provenance.ContributingLayers,
			Overridden:         n.Provenance.Overridden,
			OverriddenBy:       n.Provenance.OverriddenBy,
		},
	}
}

// RenderJSON renders cat as indented, deterministic JSON: nodes in
// cat.Nodes order (already sorted by ID), facets as a sorted slice
// rather than a map, no timestamps. Same node set, same bytes, always.
func RenderJSON(cat Catalog) ([]byte, error) {
	nodes := make([]jsonNode, 0, len(cat.Nodes))
	for _, n := range cat.Nodes {
		nodes = append(nodes, toJSONNode(n))
	}
	return json.MarshalIndent(nodes, "", "  ")
}

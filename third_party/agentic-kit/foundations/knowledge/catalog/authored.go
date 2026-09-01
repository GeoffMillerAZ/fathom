package catalog

import (
	"encoding/json"
	"fmt"
)

// AuthoredNode is the natural, nested shape a hand-written taxonomy is
// authored in — mirrors #Node in schema.cue (CUE owns the shape; this
// type owns the behavior, per the kit's core convention). LoadAuthoredJSON
// parses this shape; Flatten converts it into the flat Catalog every
// other function in this package operates on.
type AuthoredNode struct {
	Slug      string              `json:"slug"`
	Title     string              `json:"title,omitempty"`
	Synopsis  string              `json:"synopsis,omitempty"`
	Order     int                 `json:"order,omitempty"`
	StableKey string              `json:"stable_key,omitempty"`
	Facets    map[string][]string `json:"facets,omitempty"`
	Hidden    bool                `json:"hidden,omitempty"`
	Source    string              `json:"source,omitempty"`
	Extractor string              `json:"extractor,omitempty"`
	Watermark string              `json:"watermark,omitempty"`
	Children  []AuthoredNode      `json:"children,omitempty"`
}

// LoadAuthoredJSON parses JSON in the AuthoredNode shape (the natural
// export of a schema.cue-validated authored catalog — e.g. `cue export
// --out json`, or any hand-written JSON matching the same fields) into a
// flat Catalog via Flatten. data must decode to a JSON array of root
// nodes.
func LoadAuthoredJSON(data []byte, layerName string) (Catalog, error) {
	var roots []AuthoredNode
	if err := json.Unmarshal(data, &roots); err != nil {
		return Catalog{}, fmt.Errorf("catalog: parse authored JSON: %w", err)
	}
	return Flatten(roots, layerName)
}

// Flatten converts a forest of AuthoredNode trees into a flat Catalog,
// computing each Node's ParentID from its position in the tree. If
// layerName is non-empty, the result is passed through Build as a single
// named Layer (so an authored catalog can itself be one input to a
// larger Build alongside a generated base or other overlays); if empty,
// Flatten calls New directly.
func Flatten(roots []AuthoredNode, layerName string) (Catalog, error) {
	var nodes []Node
	var walk func(a AuthoredNode, parentID string)
	walk = func(a AuthoredNode, parentID string) {
		n := Node{
			ParentID:  parentID,
			Slug:      a.Slug,
			Title:     a.Title,
			Synopsis:  a.Synopsis,
			Order:     a.Order,
			StableKey: a.StableKey,
			Facets:    a.Facets,
			Hidden:    a.Hidden,
			Provenance: Provenance{
				Source:    a.Source,
				Extractor: a.Extractor,
				Watermark: a.Watermark,
			},
		}
		nodes = append(nodes, n)
		id := n.ID()
		for _, c := range a.Children {
			walk(c, id)
		}
	}
	for _, r := range roots {
		walk(r, "")
	}
	if layerName == "" {
		return New(nodes)
	}
	return Build(Layer{Name: layerName, Nodes: nodes})
}

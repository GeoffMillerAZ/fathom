package catalogkit

import "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"

// Spec is a CUE-exported collection of local catalog layers (#Spec).
type Spec struct {
	Version int     `json:"version"`
	Layers  []Layer `json:"layers"`
}

// Layer is a named contribution. Requires declares all precedence edges.
// Pin, when present, is LayerDigest of this declaration with Pin omitted.
type Layer struct {
	Name      string                 `json:"name"`
	Requires  []string               `json:"requires,omitempty"`
	Pin       string                 `json:"pin,omitempty"`
	Nodes     []catalog.AuthoredNode `json:"nodes,omitempty"`
	Overrides []Override             `json:"overrides,omitempty"`
}

// Override is a compare-and-set patch. Identity and source cannot be rewritten.
type Override struct {
	ID             string `json:"id"`
	ExpectedDigest string `json:"expected_digest"`
	Reason         string `json:"reason"`
	Set            Patch  `json:"set"`
}

// Patch distinguishes omitted values from explicit false, zero and empty values.
// Facets, when supplied, replaces the whole facet map; ordinary nodes union it.
type Patch struct {
	Title    *string              `json:"title,omitempty"`
	Synopsis *string              `json:"synopsis,omitempty"`
	Order    *int                 `json:"order,omitempty"`
	Hidden   *bool                `json:"hidden,omitempty"`
	Facets   *map[string][]string `json:"facets,omitempty"`
}

// Grant permits named fields for exactly one layer and node. It is supplied by
// the receiver, separately from Spec. No grant is implied by a dependency edge.
type Grant struct {
	Layer  string   `json:"layer"`
	ID     string   `json:"id"`
	Fields []string `json:"fields"`
}

// Limits bounds one resolution; zero fields use the documented defaults.
type Limits struct {
	MaxLayers int `json:"max_layers,omitempty"`
	MaxNodes  int `json:"max_nodes,omitempty"`
	MaxDepth  int `json:"max_depth,omitempty"`
	MaxBytes  int `json:"max_bytes,omitempty"`
}

// Policy is receiver-owned permission and resource configuration (#Policy).
type Policy struct {
	Grants []Grant `json:"grants,omitempty"`
	Limits Limits  `json:"limits,omitempty"`
}

// Change records one applied patch and its exact before/after semantic digests.
type Change struct {
	Layer  string   `json:"layer"`
	ID     string   `json:"id"`
	Fields []string `json:"fields"`
	Reason string   `json:"reason"`
	Before string   `json:"before"`
	After  string   `json:"after"`
}

// SourcePin identifies a resolved layer declaration. It is not a signature.
type SourcePin struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// Bundle is one resolved catalog with its ordered sources, audit trail and digest.
// Treat Catalog as read-only after resolution. VerifyBundle detects later changes.
type Bundle struct {
	Version      int             `json:"version"`
	Digest       string          `json:"digest"`
	Order        []string        `json:"order"`
	Sources      []SourcePin     `json:"sources"`
	PolicyDigest string          `json:"policy_digest"`
	Catalog      catalog.Catalog `json:"catalog"`
	Changes      []Change        `json:"changes"`
}

// Row is the stable analytics projection used by Rows and ExportNDJSON.
type Row struct {
	ID        string              `json:"id"`
	ParentID  string              `json:"parent_id"`
	Title     string              `json:"title"`
	Synopsis  string              `json:"synopsis"`
	Order     int                 `json:"order"`
	StableKey string              `json:"stable_key"`
	Hidden    bool                `json:"hidden"`
	Facets    map[string][]string `json:"facets"`
	Source    string              `json:"source"`
	Extractor string              `json:"extractor"`
	Watermark string              `json:"watermark"`
	Layers    []string            `json:"layers"`
}

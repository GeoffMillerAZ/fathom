// Package catalog builds catalogs that cannot lie.
//
// It exists because the fleet already maintains several hierarchical
// catalogs — a licensed component corpus, this kit's own package
// catalog, a consumer's MCP tool catalog, a consumer's layer catalog —
// each hand-rolling the same machinery badly and independently, and each
// capable of claiming an entry that no longer exists or omitting one that
// does. A catalog's whole value is being trusted without verification, so
// this package's central contribution is not the tree: it is Verify, the
// drift gate (see docs/designs/catalog-kit.md §4 in the owning repo for
// the full design rationale).
//
// # Model
//
// A Catalog is a flat, immutable set of Node values — this package's own
// vocabulary calls it a "node set." There is no pointer tree: every
// relationship (parent, child, ancestor, descendant, depth) is derived on
// read from a Node's ID, which is the slash-joined path of its ancestors'
// slugs (JoinID). That is what makes arbitrary depth free: nothing in
// this package special-cases how deep a catalog goes, because depth is
// data, never a type or a constant.
//
// Facets are not tree levels. An item's Facets field is an axis->allowed-
// values map; Variants expands it into the item's full variant set
// (design's running example: one component × 3 facet axes = many
// variants of one item) without ever creating a child Node for an axis.
//
// Derived views — Rollup, DepthHistogram, CoverageByFacet, Orphans,
// DuplicateSlugs — are pure functions recomputed from the node set on
// every call. Nothing here caches a count.
//
// Every node carries Provenance: where it came from (Source, Extractor,
// Watermark) and, once it passes through Build, which layers contributed
// to it. A node without provenance cannot fully participate in the drift
// gate.
//
// # Determinism
//
// Every exported function in this package is a pure function of its
// inputs: no bare time.Now() (Provenance.Fresh takes an explicit now),
// no map iteration order leaking into output (facet keys/values are
// always sorted before use), and every renderer sorts children by Order
// then Slug. Two independently constructed node sets with the same
// content always produce byte-identical renders — that is what makes a
// generated catalog file diffable, and a catalog nobody diffs is a
// catalog nobody audits.
//
// # Extension: layers
//
// A catalog is usually built from more than one source: a base catalog
// (shipped, embedded, or generated) plus zero or more project-local
// overlays, mirroring this fleet's embedded-packs ∪ project-local-layers
// precedent. Build(layers ...Layer) merges them in order: the first layer
// to mention an ID adds the node; a later layer mentioning the same ID
// EXTENDS it (facet values are unioned, never counted as an override) or
// OVERRIDEs it (a later layer's non-zero scalar fields replace the
// earlier ones, and the node's Provenance records that it was overridden
// and by which layer — a silent override is exactly the class of lie the
// drift gate exists to prevent). There is no delete: the only way an
// overlay suppresses a node is to mark it Hidden, which removes it from
// renderers but leaves it fully visible to Verify and every compute
// function.
//
// # Non-goals
//
// This package ships no storage engine, no network client, no embeddings,
// no UI, and no server: MCP tool shapes (ToolSpecs, SearchHandler,
// GetHandler, TreeHandler) are handlers-as-functions for a consumer to
// register on its own server under its own verb names. There is no
// cross-catalog federation.
//
// # Concurrency
//
// Catalog, Node, and every value type in this package are immutable once
// constructed by New, Build, or Flatten: every exported function reads
// its inputs and returns a new value, never mutating a Catalog or Node in
// place. That makes every exported function in this package safe for
// concurrent use from multiple goroutines without any additional
// synchronization — there is no shared mutable state to protect.
package catalog

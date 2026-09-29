// Package catalog defines the CUE-first contract for AUTHORED catalog
// nodes: the natural, nested shape a hand-written taxonomy is written in.
// The Go type AuthoredNode in authored.go mirrors this definition; any
// drift between the two is a bug.
//
// This schema governs input only. The flat runtime Node/Catalog types in
// node.go/catalog.go are Flatten's output, computed and never
// hand-authored, so they are not CUE-validated — CUE owns the authored
// shape, Go owns the computed behavior, per the kit's own rule
// (AGENTS.md: "CUE owns the shape of data; Go owns behavior").
package catalog

// #Node is one authored catalog entry, recursively nested to whatever
// depth the taxonomy needs. There is deliberately no depth limit declared
// here: depth is data, not part of this schema (design §2.1).
#Node: {
	// slug is this node's own path segment: lowercase kebab-case,
	// matching the kit's own directory-naming convention.
	slug: string & =~"^[a-z0-9]+(-[a-z0-9]+)*$"

	title?:    string
	synopsis?: string
	order?:    int

	// stable_key, if set, survives renames — Verify uses it to tell a
	// moved node from a deleted-and-added pair. Never empty if present.
	stable_key?: string & !=""

	// hidden marks a node rendered out but still present for Verify —
	// the only suppression an overlay may apply (there is no delete).
	hidden?: bool

	source?:    string
	extractor?: string
	watermark?: string

	// facets is the axis->allowed-values map for an item (design §2.2).
	// Facets are metadata, never additional #Node levels.
	facets?: [string]: [...string]

	// children nests this schema recursively — arbitrary depth, no
	// max, no special-casing at any level.
	children?: [...#Node]
}

// #Catalog is a forest of top-level #Node values — the shape
// LoadAuthoredJSON parses.
#Catalog: [...#Node]

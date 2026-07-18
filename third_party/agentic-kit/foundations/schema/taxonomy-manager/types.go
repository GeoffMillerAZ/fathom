// Package taxonomymanager provides a classification hierarchy manager for
// org-configurable taxonomies.
//
// A Taxonomy is a directed acyclic graph of Category nodes connected by
// parent references. All entries are data — no labels or IDs are hard-coded
// in this package; callers supply them via Add.
//
// The Manager enforces three invariants:
//  1. Category IDs must be unique within the taxonomy.
//  2. A category's Parent must be "" (root) or reference an already-added
//     category, preventing dangling references.
//  3. Cycles are rejected at Add time, keeping the graph a valid DAG.
//
// Concurrency: Manager is not safe for concurrent use; guard with your own synchronisation.
package taxonomymanager

// Category is a single node in a classification hierarchy.
type Category struct {
	// ID is a unique, stable identifier for this category.
	// Must be non-empty.
	ID string

	// Parent is the ID of the parent category. An empty string means this
	// category is a root node.
	Parent string

	// Label is a human-readable display name.
	// Must be non-empty.
	Label string
}

// Taxonomy is a named, versioned collection of categories.
type Taxonomy struct {
	// Name identifies the taxonomy (e.g. "cost-center").
	Name string

	// Version is a semver-style string for audit tracking (e.g. "1.0.0").
	Version string
}

// Package taxonomymanager defines CUE contracts for classification taxonomy
// records. These constraints mirror the Go types and are evaluated with
// `cue vet ./...`.
package taxonomymanager

// #Category represents a single node in a classification hierarchy.
// Categories are org-configurable: no labels or IDs are hard-coded here.
#Category: {
	// ID is a unique, stable identifier for this category node.
	ID: string & !=""

	// Parent is the ID of the parent category, or "" for root categories.
	Parent: string

	// Label is a human-readable display name for the category.
	Label: string & !=""
}

// #Taxonomy is a named, versioned collection of categories that together form
// a classification hierarchy. The categories list may be empty for a freshly
// created taxonomy.
#Taxonomy: {
	// Name is a unique identifier for the taxonomy (e.g. "cost-center").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0") for audit tracking.
	Version: string & !=""

	// Categories is the list of category nodes in this taxonomy.
	// May be empty when first created.
	Categories: [...#Category]
}

// Package directinvestigator has no data contract of its own — it is a
// behavioral-only package (the pure-Go Tier-2 engine). Its output is
// CoverageManifest and Finding from evidence-signature, and Locator/SpanProof
// from evidence-span and evidence-anchor; those packages own the shape.
//
// This file exists to satisfy the standard package shape requirement and to
// document the Config constraints in CUE form.
package directinvestigator

// #Config describes the investigator configuration constraints.
// The Go Config struct mirrors these; any drift is a bug.
#Config: {
	// root_dir is the directory to walk. Must be non-empty.
	root_dir: string & !=""

	// max_file_bytes caps the per-file read size. Files larger than this are
	// recorded as Oversize skips. Must be positive.
	max_file_bytes: int & >0

	// tree_hash is the content/git hash of the tree at the time of search.
	// Ties the resulting CoverageManifest to a specific snapshot.
	tree_hash: string
}

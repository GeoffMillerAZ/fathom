// Package testlayer defines the CUE schema for a repo's OPTIONAL test-layer
// classification config (RC-39, docs/vision/28-receipts.md §5): an open
// vocabulary of named test layers ("unit", "integration", "e2e", or any
// project-specific name) matched by build tag, directory, or glob.
// internal/testlayer is the Go-side dual-mode loader (LoadLayers: a
// declared config file, or DefaultLayers() when none is present).
//
// Mirrors schema/guard/schema.cue's conventions (closedness-as-authoring-
// gate, non-empty-list fields guard against a silently-deadened entry).
package testlayer

// #Layers is the top-level shape of a testlayer config file.
#Layers: {
	layers: [...#Layer]
}

// #Layer names one test layer and the signals that classify a test file
// into it. At least one of build_tags/dirs/globs should be set for a layer
// to ever match anything — CUE cannot express "one of N optional fields"
// as a closed-schema constraint, so this is Go-enforced (mirrors
// guardspec's scopes/command_scopes either/or check).
#Layer: {
	// name is the layer identifier Classify returns (e.g. "integration",
	// "e2e", or a project-specific name like "contract").
	name: string & !=""

	// globs are path.Match glob patterns matched against a test file's
	// repo-relative, slash-form path.
	globs?: [...string] & [_, ...]

	// build_tags are Go build tags (`//go:build <tag>`, or the legacy
	// `// +build <tag>`) that classify a test file into this layer when
	// the caller-supplied tag set for that file contains one of them.
	build_tags?: [...string] & [_, ...]

	// dirs are path segment names (e.g. "e2e", "integration") — a test
	// file under a directory with this exact segment name classifies into
	// this layer.
	dirs?: [...string] & [_, ...]
}

// Package sourceregistry defines the CUE-first contracts for the source
// registry. Go types in types.go mirror these definitions; any drift is a bug.
package sourceregistry

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Kind enumerates the valid source kinds.
#Kind: "markdown" | "readme" | "doc" | "other"

// #Source describes a single evidence source.
#Source: {
	// ID uniquely identifies this source.
	ID: _nonEmpty

	// Kind classifies the source family.
	Kind: #Kind

	// URI is the stable locator for this source.
	// May be empty for in-memory or synthetic sources.
	URI: string
}

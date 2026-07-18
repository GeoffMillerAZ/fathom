// Package documentparsing defines the CUE-first contract for parsed document
// segments. The Go struct Segment in types.go mirrors this definition.
// Any drift between the two is a bug.
package documentparsing

// #Segment is one logical chunk of a parsed document.
// Order must be non-negative; Heading and Text may both be empty (e.g. for a
// preamble-only document or a heading with no body).
#Segment: {
	// Heading is the section title extracted from an ATX heading line.
	// Empty string is allowed (preamble before the first heading).
	Heading: string

	// Text is the stripped body content that follows the heading.
	// Empty string is allowed (a heading with no body text).
	Text: string

	// Order is the zero-based index of this segment in the document.
	// Must be >= 0.
	Order: int & >=0
}

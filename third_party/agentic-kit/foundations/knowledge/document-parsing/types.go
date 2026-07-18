// Package documentparsing parses markdown and plain-text documents into a
// flat sequence of Segment values. Each Segment captures a heading (if any),
// the body text that follows it, and its zero-based ordinal within the
// document. The output of Parse is deterministic: the same input always
// produces the same slice in the same order.
//
// Concurrency contract: Parse is a pure function with no shared mutable state;
// it is safe to call concurrently from multiple goroutines. Segment is a plain
// value type safe to read concurrently.
package documentparsing

// Segment is one logical chunk of a parsed document.
//
// CUE mirror: #Segment in schema.cue.
type Segment struct {
	// Heading is the section heading extracted from a Markdown ATX heading
	// line (e.g. "# Introduction" → "Introduction"). It is empty for the
	// preamble that appears before the first heading.
	Heading string

	// Text is the body content that follows the heading up to the next
	// heading or the end of the document. Leading and trailing whitespace is
	// stripped from the whole block, but internal line breaks are preserved.
	Text string

	// Order is the zero-based index of this segment in the document.
	// Segments are always returned in ascending Order.
	Order int
}

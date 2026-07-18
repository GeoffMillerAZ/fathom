// Package contextrenderer renders a bounded context block from a set of prose
// segments and a token budget. It is deterministic: for a given (segments,
// budget) pair the output string is always the same. Segments are sorted by
// Priority descending; lowest-priority segments are truncated first when the
// budget is exhausted.
//
// Concurrency: values are immutable and safe for concurrent use.
package contextrenderer

// Segment is a single piece of context content with a priority and token
// weight. It mirrors #Segment in schema.cue.
type Segment struct {
	// ID is a unique identifier for this segment.
	ID string

	// Content is the prose text of this segment.
	Content string

	// Priority is the importance rank of this segment. Higher values are
	// rendered first; lower values are dropped first when the budget is tight.
	Priority int

	// Tokens is the estimated token count for Content. Must be > 0.
	// If zero or negative it is treated as 1.
	Tokens int
}

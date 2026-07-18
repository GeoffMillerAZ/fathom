// Package contextrenderer defines CUE contracts for the context-renderer
// primitive. A Segment carries a piece of prose with a priority and token
// weight. The renderer assembles segments into a bounded context block that
// fits within a caller-supplied token budget.
package contextrenderer

// #Segment is a single piece of context content with priority and size metadata.
#Segment: {
	// ID is a unique identifier for this segment.
	ID: string & !=""

	// Content is the prose text of this segment.
	Content: string & !=""

	// Priority is the importance rank of this segment. Higher values are
	// included first; lower values are dropped first when the budget is tight.
	// Must be >= 0.
	Priority: int & >=0

	// Tokens is the estimated token count for Content. Must be > 0.
	Tokens: int & >0
}

// valid.cue — example Segment values that satisfy #Segment.
// Used by `cue vet ./...` to confirm the schema accepts well-formed data.
package contextrenderer

import "github.com/geoffmilleraz/agentic-kit/foundations/prose/context-renderer:contextrenderer"

// HighPrioritySegment is a valid, high-priority segment.
HighPrioritySegment: contextrenderer.#Segment & {
	ID:       "seg-001"
	Content:  "This is the most important context block."
	Priority: 10
	Tokens:   8
}

// LowPrioritySegment is a valid, low-priority segment that may be dropped
// under a tight budget.
LowPrioritySegment: contextrenderer.#Segment & {
	ID:       "seg-002"
	Content:  "This is supplementary background context."
	Priority: 1
	Tokens:   6
}

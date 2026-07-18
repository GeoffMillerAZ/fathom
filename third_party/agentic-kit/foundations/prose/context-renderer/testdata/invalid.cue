// invalid.cue — values that do NOT satisfy #Segment.
// Each bad value is commented out so that `cue vet ./...` passes on this
// directory. Uncomment any block to observe the vet error it produces.
package contextrenderer

// ZeroTokens demonstrates a Segment with Tokens=0, which violates Tokens: >0.
// Uncomment to observe: cue vet will reject the value.
//
// ZeroTokensSegment: contextrenderer.#Segment & {
// 	ID:       "seg-bad-001"
// 	Content:  "Some content."
// 	Priority: 5
// 	Tokens:   0   // ERROR: must be > 0
// }

// NegativePriority demonstrates a Segment with Priority < 0.
// Uncomment to observe: cue vet will reject the value.
//
// NegativePrioritySegment: contextrenderer.#Segment & {
// 	ID:       "seg-bad-002"
// 	Content:  "Some content."
// 	Priority: -1   // ERROR: must be >= 0
// 	Tokens:   5
// }

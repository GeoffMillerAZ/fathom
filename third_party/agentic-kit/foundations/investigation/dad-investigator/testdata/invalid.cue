// invalid.cue — values that do NOT satisfy #EvidenceItem or #Decision.
// Each bad value is commented out so that `cue vet ./...` passes on this
// directory. Uncomment any block to observe the vet error it produces.
package dadinvestigator

// ZeroWeightEvidence demonstrates an EvidenceItem with Weight=0.
// Uncomment to observe: cue vet will reject because Weight must be > 0.
//
// ZeroWeightEvidence: dadinvestigator.#EvidenceItem & {
// 	ID:      "ev-bad-001"
// 	Content: "Some evidence."
// 	Weight:  0   // ERROR: must be > 0
// }

// BadConfidenceDecision demonstrates a Decision with an invalid Confidence.
// Uncomment to observe: cue vet will reject "very-high" as not in #Confidence.
//
// BadConfidenceDecision: dadinvestigator.#Decision & {
// 	Finding:    "A finding."
// 	Confidence: "very-high"   // ERROR: not a valid #Confidence value
// 	Terminal:   true
// }

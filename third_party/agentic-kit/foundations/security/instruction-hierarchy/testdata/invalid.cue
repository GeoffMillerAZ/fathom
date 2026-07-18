// invalid.cue — examples that do NOT satisfy the instruction-hierarchy schema.
// All examples are commented out. Uncomment any block to observe the cue vet error.
package instructionhierarchy

// BadTier: demonstrates an unrecognised Tier.
// cue vet error: Tier: 2 errors in empty disjunction:
//   conflicting values "operator" and "system" ...
//
// BadTier: {
// 	Tier:    "operator"  // ERROR: not a valid #Tier value
// 	Content: "Do something."
// }

// EmptyContent: demonstrates an Instruction with empty Content.
// cue vet error: Content: invalid value "" (does not satisfy != "")
//
// EmptyContent: {
// 	Tier:    "user"
// 	Content: ""  // ERROR: content must be non-empty
// }

// RejectedWithoutReason: demonstrates a RejectedInstruction missing Reason.
// cue vet error: Reason: field not allowed OR incomplete value
//
// RejectedNoReason: {
// 	Instruction: {Tier: "untrusted", Content: "do something"}
// 	// ERROR: Reason is required and must be non-empty
// }

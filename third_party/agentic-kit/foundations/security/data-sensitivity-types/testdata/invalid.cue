// invalid.cue — examples that do NOT satisfy #Classification.
// All examples below are commented out. Uncomment any block to observe
// the cue vet error it produces.
package datasensitivitytypes

// BadLevel: demonstrates an unrecognised sensitivity level.
// cue vet error: Level: 2 errors in empty disjunction:
//   Level: conflicting values "top_secret" and "public" (mismatched types)
//   ... (not a valid #SensitivityLevel value)
//
// BadLevel: {
// 	Level:  "top_secret"
// 	Reason: "This level does not exist in the enum."
// }

// EmptyReason: demonstrates a Classification with an empty Reason.
// cue vet error: Reason: invalid value "" (does not satisfy != "")
//
// EmptyReason: {
// 	Level:  "internal"
// 	Reason: ""
// }

// MissingReason: demonstrates a Classification with no Reason field.
// cue vet error: Reason: field not allowed OR incomplete value
//
// MissingReason: {
// 	Level: "confidential"
// }

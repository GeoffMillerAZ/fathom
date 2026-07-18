// invalid.cue — values that do NOT satisfy #Checklist.
// Each bad value is commented out so that `cue vet ./...` passes on this
// directory. Uncomment any block to observe the vet error it produces.
package agenticpracticesevaluator

// EmptyPracticeName demonstrates a Practice with an empty Name field.
// Uncomment to observe: cue vet will reject the empty string constraint.
//
// BadPracticeName: agenticpracticesevaluator.#Checklist & {
// 	Name:    "bad-checklist"
// 	Version: "1.0.0"
// 	Practices: [
// 		{
// 			Name:        ""   // ERROR: Name must be non-empty
// 			Description: "A practice."
// 		},
// 	]
// }

// EmptyChecklist demonstrates a Checklist with no Practices.
// Uncomment to observe: cue vet will reject because Practices must have
// at least one element.
//
// EmptyPractices: agenticpracticesevaluator.#Checklist & {
// 	Name:       "empty"
// 	Version:    "1.0.0"
// 	Practices:  []   // ERROR: at least one practice is required
// }

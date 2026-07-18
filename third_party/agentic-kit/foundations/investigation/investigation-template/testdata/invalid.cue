// invalid.cue — demonstrates #Template values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the valid.cue
// sibling is the one vetted in CI). It is kept here as a documentation
// artifact so reviewers understand what the schema rejects.
package investigationtemplate

// _badEmptyName: empty Name violates the non-empty string constraint.
// _badEmptyName: #Template & {
// 	Name:  ""
// 	Steps: ["step-one"]
// }

// _badEmptyStep: an empty step label violates the non-empty constraint on
// each element of Steps.
// _badEmptyStep: #Template & {
// 	Name:  "some-template"
// 	Steps: ["step-one", ""]
// }

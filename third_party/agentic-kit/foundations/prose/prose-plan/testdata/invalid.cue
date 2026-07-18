// invalid.cue — demonstrates #SectionPlan and #ProsePlan values that VIOLATE
// the schema. This file is intentionally NOT included in the `cue vet` path
// (the valid.cue sibling is the one vetted in CI). It is kept here as a
// documentation artifact so reviewers understand what the schema rejects.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _badSecEmptyTitle:
//     Title: "" violates string & !="" — Title must not be empty
//
//   _badSecEmptyIntent:
//     Intent: "" violates string & !="" — Intent must not be empty
//
//   _badPlanEmpty:
//     Sections: [] violates [#SectionPlan, ...#SectionPlan] — at least one
//     section is required
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package proseplan

// // _badSecEmptyTitle — Title must not be empty
// _badSecEmptyTitle: #SectionPlan & {
// 	Title:  ""
// 	Intent: "Describe the problem."
// }

// // _badSecEmptyIntent — Intent must not be empty
// _badSecEmptyIntent: #SectionPlan & {
// 	Title:  "Introduction"
// 	Intent: ""
// }

// // _badPlanEmpty — Sections must contain at least one element
// _badPlanEmpty: #ProsePlan & {
// 	Sections: []
// }

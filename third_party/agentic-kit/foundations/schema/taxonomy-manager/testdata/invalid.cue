// Package taxonomymanager — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidCategory_EmptyID:
//     ID: "" violates string & !="" — category ID must not be empty
//
//   invalidCategory_EmptyLabel:
//     Label: "" violates string & !="" — label must not be empty
//
//   invalidTaxonomy_EmptyName:
//     Name: "" violates string & !="" — taxonomy name must not be empty
//
//   invalidTaxonomy_EmptyVersion:
//     Version: "" violates string & !="" — version must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package taxonomymanager

// // invalidCategory_EmptyID — ID must not be empty
// invalidCategory_EmptyID: #Category & {
// 	ID:     ""
// 	Parent: ""
// 	Label:  "Some Label"
// }

// // invalidCategory_EmptyLabel — Label must not be empty
// invalidCategory_EmptyLabel: #Category & {
// 	ID:     "eng"
// 	Parent: ""
// 	Label:  ""
// }

// // invalidTaxonomy_EmptyName — Name must not be empty
// invalidTaxonomy_EmptyName: #Taxonomy & {
// 	Name:       ""
// 	Version:    "1.0.0"
// 	Categories: []
// }

// // invalidTaxonomy_EmptyVersion — Version must not be empty
// invalidTaxonomy_EmptyVersion: #Taxonomy & {
// 	Name:       "cost-center"
// 	Version:    ""
// 	Categories: []
// }

// invalid.cue — demonstrates #KnowledgeCard values that VIOLATE the schema.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _emptyTitle:
//     Title: "" violates string & !="" — Title must be non-empty
//
//   _emptySourceID:
//     SourceID: "" violates string & !="" — SourceID must be non-empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package knowledgecardgeneration

// // _emptyTitle — Title must not be empty.
// _emptyTitle: #KnowledgeCard & {
// 	Title:    ""
// 	Fields:   {"key": "value"}
// 	SourceID: "docs/some-file.md"
// }

// // _emptySourceID — SourceID must not be empty.
// _emptySourceID: #KnowledgeCard & {
// 	Title:    "service-capability: Something"
// 	Fields:   {"key": "value"}
// 	SourceID: ""
// }

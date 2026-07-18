// invalid.cue — demonstrates #ProseSegment and #ProseBlock values that VIOLATE
// the schema. This file is intentionally NOT included in the `cue vet` path
// (the valid.cue sibling is the one vetted in CI). It is kept here as a
// documentation artifact so reviewers understand what the schema rejects.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _badSegEmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   _badSegEmptyText:
//     Text: "" violates string & !="" — Text must not be empty
//
//   _badBlockEmpty:
//     Segments: [] violates [#ProseSegment, ...#ProseSegment] — at least one
//     segment is required
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package proseascode

// // _badSegEmptyID — ID must not be empty
// _badSegEmptyID: #ProseSegment & {
// 	ID:           ""
// 	Text:         "Some text."
// 	EvidenceRefs: []
// }

// // _badSegEmptyText — Text must not be empty
// _badSegEmptyText: #ProseSegment & {
// 	ID:           "seg-bad"
// 	Text:         ""
// 	EvidenceRefs: []
// }

// // _badBlockEmpty — Segments must contain at least one element
// _badBlockEmpty: #ProseBlock & {
// 	Segments: []
// }

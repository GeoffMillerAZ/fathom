// invalid.cue — demonstrates #Event values that VIOLATE the schema.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _badKind:
//     Kind: "metric" violates #Kind enum — must be one of tool_call/llm_call/error/info
//
//   _emptySource:
//     Source: "" violates string & !="" — source must not be empty
//
//   _emptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   _emptyTimestamp:
//     Timestamp: "" violates string & !="" — timestamp must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)
package event

// // _badKind — "metric" is not in the #Kind enum
// _badKind: #Event & {
// 	ID:        "some-source/1704067200000000000"
// 	Kind:      "metric"
// 	Source:    "some-source"
// 	Timestamp: "2024-01-01T00:00:00Z"
// }

// // _emptySource — Source must not be empty
// _emptySource: #Event & {
// 	ID:        "some-source/1704067200000000000"
// 	Kind:      "info"
// 	Source:    ""
// 	Timestamp: "2024-01-01T00:00:00Z"
// }

// // _emptyID — ID must not be empty
// _emptyID: #Event & {
// 	ID:        ""
// 	Kind:      "info"
// 	Source:    "some-source"
// 	Timestamp: "2024-01-01T00:00:00Z"
// }

// // _emptyTimestamp — Timestamp must not be empty
// _emptyTimestamp: #Event & {
// 	ID:        "some-source/1704067200000000000"
// 	Kind:      "info"
// 	Source:    "some-source"
// 	Timestamp: ""
// }

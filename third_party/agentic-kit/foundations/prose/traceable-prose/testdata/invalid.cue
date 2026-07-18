// invalid.cue — demonstrates #TraceableNode and #RegenerationEvent values that
// VIOLATE the schema. This file is intentionally NOT included in the
// `cue vet` path. It is kept here as a documentation artifact.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _badNodeEmptyID:
//     ID: "" violates string & !=""
//
//   _badNodeRedactedNoReason:
//     Redacted: true but RedactionReason: "" violates the conditional
//     _nonEmpty constraint
//
//   _badRegenEmptyTrigger:
//     TriggeredBy: "" violates string & !=""
//
//   _badRegenEmptyNodeIDs:
//     AffectedNodeIDs: [] violates [_nonEmpty, ..._nonEmpty] — at least one
//     ID required
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package traceableprose

// // _badNodeEmptyID — ID must not be empty
// _badNodeEmptyID: #TraceableNode & {
// 	ID:                ""
// 	Text:              "Some text."
// 	SourceRefs:        []
// 	Redacted:          false
// 	RedactionReason:   ""
// 	NeedsRegeneration: false
// }

// // _badNodeRedactedNoReason — Redacted=true requires a non-empty RedactionReason
// _badNodeRedactedNoReason: #TraceableNode & {
// 	ID:                "node-bad"
// 	Text:              "[REDACTED]"
// 	SourceRefs:        []
// 	Redacted:          true
// 	RedactionReason:   ""
// 	NeedsRegeneration: false
// }

// // _badRegenEmptyTrigger — TriggeredBy must not be empty
// _badRegenEmptyTrigger: #RegenerationEvent & {
// 	TriggeredBy:     ""
// 	AffectedNodeIDs: ["node-001"]
// 	OccurredAt:      "2026-06-05T10:00:00Z"
// 	Note:            ""
// }

// // _badRegenEmptyNodeIDs — AffectedNodeIDs must have at least one element
// _badRegenEmptyNodeIDs: #RegenerationEvent & {
// 	TriggeredBy:     "correction-001"
// 	AffectedNodeIDs: []
// 	OccurredAt:      "2026-06-05T10:00:00Z"
// 	Note:            ""
// }

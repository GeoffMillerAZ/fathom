// invalid.cue — demonstrates #Entry values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the valid.cue
// sibling is the one vetted in CI). It is kept here as a documentation
// artifact so reviewers understand what the schema rejects.
package eventstore

// _badNegativeSeq: Seq < 0 violates the >=0 constraint.
// _badNegativeSeq: #Entry & {
// 	Seq:         -1
// 	Kind:        "some.event"
// 	PayloadHash: "abc123"
// 	PrevHash:    "0000000000000000000000000000000000000000000000000000000000000000"
// 	Hash:        "def456"
// 	RecordedAt:  "2025-01-15T10:00:00Z"
// }

// _badEmptyKind: empty Kind violates the _nonEmpty constraint.
// _badEmptyKind: #Entry & {
// 	Seq:         0
// 	Kind:        ""
// 	PayloadHash: "abc123"
// 	PrevHash:    "0000000000000000000000000000000000000000000000000000000000000000"
// 	Hash:        "def456"
// 	RecordedAt:  "2025-01-15T10:00:00Z"
// }

// _badEmptyPayloadHash: empty PayloadHash violates the _nonEmpty constraint.
// _badEmptyPayloadHash: #Entry & {
// 	Seq:         0
// 	Kind:        "some.event"
// 	PayloadHash: ""
// 	PrevHash:    "0000000000000000000000000000000000000000000000000000000000000000"
// 	Hash:        "def456"
// 	RecordedAt:  "2025-01-15T10:00:00Z"
// }

// _badEmptyHash: empty Hash violates the _nonEmpty constraint.
// _badEmptyHash: #Entry & {
// 	Seq:         0
// 	Kind:        "some.event"
// 	PayloadHash: "abc123"
// 	PrevHash:    "0000000000000000000000000000000000000000000000000000000000000000"
// 	Hash:        ""
// 	RecordedAt:  "2025-01-15T10:00:00Z"
// }

// _badEmptyRecordedAt: empty RecordedAt violates the _nonEmpty constraint.
// _badEmptyRecordedAt: #Entry & {
// 	Seq:         0
// 	Kind:        "some.event"
// 	PayloadHash: "abc123"
// 	PrevHash:    "0000000000000000000000000000000000000000000000000000000000000000"
// 	Hash:        "def456"
// 	RecordedAt:  ""
// }

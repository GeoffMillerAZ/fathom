// invalid.cue — demonstrates #ManifestEntry values that VIOLATE the schema.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _emptyCardID:
//     CardID: "" violates string & !="" — CardID must be non-empty
//
//   _emptyHash:
//     Hash: "" violates string & !="" — Hash must be non-empty
//
//   _emptyCreatedAt:
//     CreatedAt: "" violates string & !="" — CreatedAt must be non-empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package memorymanifest

// // _emptyCardID — CardID must not be empty.
// _emptyCardID: #ManifestEntry & {
// 	CardID:    ""
// 	Hash:      "abc123"
// 	CreatedAt: "2024-11-01T09:00:00Z"
// }

// // _emptyHash — Hash must not be empty.
// _emptyHash: #ManifestEntry & {
// 	CardID:    "card-001"
// 	Hash:      ""
// 	CreatedAt: "2024-11-01T09:00:00Z"
// }

// // _emptyCreatedAt — CreatedAt must not be empty.
// _emptyCreatedAt: #ManifestEntry & {
// 	CardID:    "card-001"
// 	Hash:      "abc123"
// 	CreatedAt: ""
// }

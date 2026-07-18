// invalid.cue — demonstrates #Anchor and #InclusionProof values that VIOLATE
// the schema. This file is intentionally NOT vetted with `cue vet` (it is
// commented-out documentation). Corresponding Go-level enforcement is in
// Anchor.Validate() (validate.go) and Prove()/Verify() (merkle.go).
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// Expected errors per binding are described in the leading comment of each
// block.
package anchor

// // _badAnchorEmptyRoot: Root must be non-empty (_hexString = string & !="").
// _badAnchorEmptyRoot: #Anchor & {
// 	Root:      ""
// 	LeafCount: 1
// 	CreatedAt: "2026-01-01T00:00:00Z"
// 	Authority: "opentimestamps"
// 	Status:    "pending"
// }

// // _badAnchorZeroLeafCount: LeafCount must be >= 1.
// _badAnchorZeroLeafCount: #Anchor & {
// 	Root:      "deadbeefdeadbeef"
// 	LeafCount: 0
// 	CreatedAt: "2026-01-01T00:00:00Z"
// 	Authority: "opentimestamps"
// 	Status:    "pending"
// }

// // _badAnchorNegativeLeafCount: LeafCount must be >= 1.
// _badAnchorNegativeLeafCount: #Anchor & {
// 	Root:      "deadbeefdeadbeef"
// 	LeafCount: -5
// 	CreatedAt: "2026-01-01T00:00:00Z"
// 	Authority: "opentimestamps"
// 	Status:    "confirmed"
// }

// // _badAnchorEmptyCreatedAt: CreatedAt must be non-empty (_nonEmpty).
// _badAnchorEmptyCreatedAt: #Anchor & {
// 	Root:      "deadbeefdeadbeef"
// 	LeafCount: 1
// 	CreatedAt: ""
// 	Authority: "opentimestamps"
// 	Status:    "pending"
// }

// // _badAnchorEmptyAuthority: Authority must be non-empty (_nonEmpty).
// _badAnchorEmptyAuthority: #Anchor & {
// 	Root:      "deadbeefdeadbeef"
// 	LeafCount: 1
// 	CreatedAt: "2026-01-01T00:00:00Z"
// 	Authority: ""
// 	Status:    "pending"
// }

// // _badAnchorUnknownStatus: Status must be "pending" | "confirmed".
// // Error: "processing" is not one of the enumerated #AnchorStatus values.
// _badAnchorUnknownStatus: #Anchor & {
// 	Root:      "deadbeefdeadbeef"
// 	LeafCount: 1
// 	CreatedAt: "2026-01-01T00:00:00Z"
// 	Authority: "opentimestamps"
// 	Status:    "processing"
// }

// // _badProofNegativeLeafIndex: LeafIndex must be >= 0.
// _badProofNegativeLeafIndex: #InclusionProof & {
// 	LeafIndex:  -1
// 	Siblings:   []
// 	Directions: []
// }

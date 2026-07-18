// invalid.cue — demonstrates #Segment values that VIOLATE the schema.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _negativeOrder:
//     Order: -1 violates int & >=0 — Order must be non-negative
//
//   _nonIntOrder:
//     Order: 1.5 violates int — Order must be an integer
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package documentparsing

// // _negativeOrder — Order -1 is out of the valid range (must be >= 0).
// _negativeOrder: #Segment & {
// 	Heading: "Bad Section"
// 	Text:    "some text"
// 	Order:   -1
// }

// // _nonIntOrder — Order 1.5 is not an integer.
// _nonIntOrder: #Segment & {
// 	Heading: "Another Bad Section"
// 	Text:    "some text"
// 	Order:   1.5
// }

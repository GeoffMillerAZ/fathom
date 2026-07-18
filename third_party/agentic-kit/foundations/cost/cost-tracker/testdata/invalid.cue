// invalid.cue — demonstrates #PriceTable values that VIOLATE the schema.
// This file contains only comments (it is intentionally not vetted as live
// data); it documents the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _negativeRate:
//     InputPerMTok: -1.0 violates number & >=0 — rates must be non-negative
//
//   _missingField:
//     CacheWritePerMTok is required by #ModelPrice — all four rates must be
//     stated explicitly (use 0 for "free / not billed"), there are no defaults
//
//   _emptyModelName:
//     "": the pattern constraint [!=""] rejects empty model identifiers
//
//   _nonNumberRate:
//     OutputPerMTok: "15.0" violates number — rates are numbers, not strings
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)
package costtracker

// // _negativeRate — rates must be >= 0
// _negativeRate: costtracker.#PriceTable & {
// 	"example-model": {
// 		InputPerMTok:      -1.0
// 		OutputPerMTok:     2.0
// 		CacheReadPerMTok:  0
// 		CacheWritePerMTok: 0
// 	}
// }

// // _missingField — all four rates are required, no defaults
// _missingField: costtracker.#PriceTable & {
// 	"example-model": {
// 		InputPerMTok:     1.0
// 		OutputPerMTok:    2.0
// 		CacheReadPerMTok: 0
// 	}
// }

// // _emptyModelName — model identifiers must be non-empty
// _emptyModelName: costtracker.#PriceTable & {
// 	"": {
// 		InputPerMTok:      1.0
// 		OutputPerMTok:     2.0
// 		CacheReadPerMTok:  0
// 		CacheWritePerMTok: 0
// 	}
// }

// // _nonNumberRate — rates are numbers, not strings
// _nonNumberRate: costtracker.#PriceTable & {
// 	"example-model": {
// 		InputPerMTok:      1.0
// 		OutputPerMTok:     "15.0"
// 		CacheReadPerMTok:  0
// 		CacheWritePerMTok: 0
// 	}
// }

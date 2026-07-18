// invalid.cue — demonstrates #Source values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path.
// It is kept as a documentation artifact so reviewers understand what the
// schema rejects.
package sourceregistry

// _badKind: "unknown" is not a valid Kind value.
// _badKind: #Source & {
// 	ID:   "src-bad"
// 	Kind: "unknown"
// 	URI:  "somewhere"
// }

// _emptyID: empty ID violates the non-empty string constraint.
// _emptyID: #Source & {
// 	ID:   ""
// 	Kind: "doc"
// 	URI:  "https://example.com"
// }

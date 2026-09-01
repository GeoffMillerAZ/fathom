// invalid.cue — demonstrates #Node values that VIOLATE the schema in
// ../schema.cue. This file is NOT vetted with `cue vet` (it is
// intentionally invalid). It is kept here as documentation of the
// constraints schema.cue enforces; the same constraints are asserted
// live by Go rejection tests (TestNewRejectsInvalidSlug, ValidateSlug's
// table in node_test.go).
//
// The examples below would fail `cue vet` with errors like:
//
//   _badSlug:
//     slug: "Not Kebab Case" does not match =~"^[a-z0-9]+(-[a-z0-9]+)*$"
//
//   _emptyStableKey:
//     stable_key: "" violates string & !="" — stable_key must be
//     non-empty when present
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package catalog

// // _badSlug — slug must be lowercase kebab-case, not free text.
// _badSlug: #Node & {
// 	slug: "Not Kebab Case"
// }

// // _emptyStableKey — stable_key, if present, must be non-empty.
// _emptyStableKey: #Node & {
// 	slug:       "thing"
// 	stable_key: ""
// }

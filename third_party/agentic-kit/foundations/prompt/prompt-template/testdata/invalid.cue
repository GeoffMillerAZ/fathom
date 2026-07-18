// invalid.cue — demonstrates #Template values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the
// valid.cue sibling is the one vetted in CI). It is kept here as a
// documentation artifact so reviewers understand what the schema rejects.
//
// Uncomment any block and run `cue vet .` from this directory to observe the
// rejection. The equivalent Go-level rejections are covered by Validate()
// tests in prompttemplate_test.go. Note that a syntactically broken
// text/template body (e.g. "{{.unclosed") is NOT a CUE-level violation —
// CUE only requires Text to be a non-empty string; parseability is enforced
// by Go's Template.Validate.
package prompttemplate

// _emptyName: Name must be non-empty (_nonEmpty constraint).
// _emptyName: #Template & {
// 	Name:    ""
// 	Version: "1.0.0"
// 	Text:    "Summarize: {{.incident}}"
// }

// _emptyVersion: Version must be non-empty (_nonEmpty constraint).
// _emptyVersion: #Template & {
// 	Name:    "summarize-incident"
// 	Version: ""
// 	Text:    "Summarize: {{.incident}}"
// }

// _emptyText: Text must be non-empty (_nonEmpty constraint).
// _emptyText: #Template & {
// 	Name:    "summarize-incident"
// 	Version: "1.0.0"
// 	Text:    ""
// }

// _extraField: #Template is closed — unknown fields are rejected.
// _extraField: #Template & {
// 	Name:     "summarize-incident"
// 	Version:  "1.0.0"
// 	Text:     "Summarize: {{.incident}}"
// 	Registry: "nope"
// }

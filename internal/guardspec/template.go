package guardspec

// template.go — the COMMENTED example `fathom init` scaffolds at
// .fathom/guard.cue (docs/design/guarded-edits.md deliverable 1: "scaffolds
// a commented example"). Mirrors internal/bindspec.ExampleTemplate: an
// entirely commented-out worked example, never an active guard — guarding a
// scope is an explicit, reviewed opt-in.

// ExampleTemplate renders a commented `.fathom/guard.cue` scaffold: valid,
// empty CUE with the worked examples from docs/design/guarded-edits.md
// §2.1/§2.5 shown as a comment block for a human to uncomment and edit.
//
// Critic finding 1 (phase-O merge, BLOCKING): every optional field is
// written WITHOUT a trailing '?' — see internal/bindspec.ExampleTemplate's
// doc comment for the full CUE-instance-vs-schema reasoning this mirrors.
func ExampleTemplate() string {
	return `// guard.cue — pre-edit guidance for critical scopes (docs/design/
// guarded-edits.md). Files/commands matched here get their governing
// rules injected BEFORE the edit/command runs, not just after.
//
// Uncomment and edit to guard a scope. Every field below except "scopes"
// (or "command_scopes" in the second example) is OPTIONAL — omit a line
// entirely to leave it unset; never leave a trailing '?' on a field name
// here (that syntax belongs in the SCHEMA, not an instance — see
// schema/guard/schema.cue — and would silently drop the field instead of
// setting it):
//
// guards: [{
// 	scopes: ["internal/store/**", "**/migrations/**"]  // consumer-tree globs
// 	note:            "Store schema is spec-governed; read docs/spec first."
// 	max_rules:       4    // OPTIONAL: pre-edit payload cap (default 4)
// 	include_modules: true // OPTIONAL: also serve bound platform-module rules
// }]
//
// Commands can be guarded the same way (§2.5):
//
// guards: [{
// 	command_scopes: ["terraform apply*", "kubectl *", "*/migrate *"]
// 	note: "Infra commands are guarded; check the runbook first."
// }]
`
}

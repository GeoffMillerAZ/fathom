package bindspec

// template.go — the COMMENTED example `fathom init` scaffolds at
// .fathom/modules.cue (docs/design/platform-modules.md deliverable 1: "no
// active config by default"). Unlike modspec.Template (which emits a live,
// ValidateStrict-passing module.cue), this template emits an entirely
// commented-out example — binding a module is an explicit, reviewed opt-in
// (design §3.1 doctrine), never something `fathom init` activates on a
// repo's behalf.

// ExampleTemplate renders a commented `.fathom/modules.cue` scaffold: valid
// CUE (an empty file, so `cue vet` and this repo's own loaders see nothing
// to load), with the worked example from docs/design/platform-modules.md
// §3.1 shown entirely as a comment block for a human to uncomment and edit.
func ExampleTemplate() string {
	return `// modules.cue — platform-module bindings (docs/design/platform-modules.md).
// No org rule ever reaches this repo until a binding below is uncommented,
// reviewed, and committed — mounting a module on the daemon only makes it
// AVAILABLE; this file is what makes it APPLY.
//
// Uncomment and edit to bind a module this daemon has mounted (--module):
//
// bindings: [{
// 	module:  "platform-standards"    // mounted module name (daemon --module)
// 	scopes?: ["infra/**", "**/*.go"] // optional: restrict further than the
// 	                                  // module's own scopes (never widens)
// 	max_rules?: 2                    // per-edit budget contribution cap
// 	min_tier?:  "doc-claimed"        // serve nothing below this tier
// }]
`
}

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
//
// Critic finding 1 (phase-O merge, BLOCKING): the example below writes
// every optional field WITHOUT a trailing '?' — `?:` is CUE's optional-
// field marker, meaningful in a SCHEMA (#Binding, schema/binding/
// schema.cue), but in a plain INSTANCE like this one it declares the
// field optional-and-therefore-ABSENT: LoadBindings' LookupPath would see
// no `scopes` at all, silently widening the trust boundary (design §3.2's
// "intersects, never widens" becomes its exact inverse) rather than
// failing loudly. A human who uncomments this block gets a working,
// concrete binding, not a silently-inert one; the schema's own `?:`
// markers are what make each of these fields optional TO OMIT, by simply
// deleting the line.
func ExampleTemplate() string {
	return `// modules.cue — platform-module bindings (docs/design/platform-modules.md).
// No org rule ever reaches this repo until a binding below is uncommented,
// reviewed, and committed — mounting a module on the daemon only makes it
// AVAILABLE; this file is what makes it APPLY.
//
// Uncomment and edit to bind a module this daemon has mounted (--module).
// Every field below except "module" is OPTIONAL — omit a line entirely to
// leave it unset; never leave a trailing '?' on a field name here (that
// syntax belongs in the SCHEMA, not an instance — see schema/binding/
// schema.cue — and would silently drop the field instead of setting it):
//
// bindings: [{
// 	module: "platform-standards"     // mounted module name (daemon --module)
// 	scopes: ["infra/**", "**/*.go"]  // OPTIONAL: restrict further than the
// 	                                  // module's own scopes (never widens);
// 	                                  // omit this line entirely to leave
// 	                                  // the module's own scopes as-is
// 	max_rules: 2                     // OPTIONAL: per-edit budget cap (default 2)
// 	min_tier:  "doc-claimed"         // OPTIONAL: serve nothing below this tier
// }]
`
}

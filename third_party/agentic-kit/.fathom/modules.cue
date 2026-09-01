// modules.cue — platform-module bindings (docs/design/platform-modules.md).
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

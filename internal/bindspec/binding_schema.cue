// Package binding defines the CUE schema for a consumer repo's platform-
// module binding file (docs/design/platform-modules.md §3.1): a repo's
// `.fathom/modules.cue` — the explicit, reviewable opt-in that lets a bound
// platform module's compiled rules reach this repo's obligations serving.
//
// Doctrine (design §3.1, normative): "no org rule ever reaches a repo that
// did not bind it." Central mounting (the daemon's --module flag) makes a
// module *available*; only THIS file, committed in the consumer repo and
// reviewed through its normal gates, makes it *apply*.
//
// This file mirrors schema/module/schema.cue's conventions deliberately —
// same regex-shaped name field, same `| *default` pattern, same closedness
// discipline (an authoring gate, not a reader-conformance instrument, per
// CLAUDE.md "New CUE schemas"). internal/bindspec is the Go-side dual-mode
// loader (LoadBindings the lenient reader, ValidateStrict the closed-schema
// authoring gate) — mirroring internal/modspec exactly.
//
// Status: v0. No path remapping, no auto-binding — see design §4 (non-goals).
package binding

// #Bindings is the top-level shape of a `.fathom/modules.cue` file: a
// list of module bindings. An empty file has nothing to scaffold
// as valid — `fathom init`'s commented example template is the honest way
// to ship "no bindings configured yet"; an explicit `bindings: []` is the
// SANCTIONED inert form (one vocabulary with the activate skill and the
// kit packs' install steps — plan IO-7): present, declaring nothing,
// valid. Only a malformed entry is an error.
#Bindings: {
	bindings: [...#Binding]
}

// #Binding is one repo's opt-in to one mounted platform module.
#Binding: {
	// module names the mounted module (daemon --module) this binding opts
	// into — same name shape as schema/module/schema.cue's #Module.name.
	module: string & =~"^[a-z][a-z0-9-]*$"

	// scopes optionally RESTRICTS (never widens, design §3.2) which files
	// this binding applies its module's rules to, layered on top of the
	// module's own scope globs written against consumer trees. Absent
	// means the module's own scopes apply exactly as the module authored
	// them.
	scopes?: [...string] & [_, ...]

	// max_rules caps this binding's contribution to a single obligations
	// call (design §3.1's worked example: `max_rules?: 2`); the repo's own
	// rules keep priority within the overall MaxObligations budget
	// regardless of this cap.
	max_rules: int & >0 | *2

	// min_tier floors which evidence tier this binding will let through;
	// absent means no floor — every tier the module compiled may serve.
	min_tier?: "verified" | "pre-scan" | "doc-claimed" | "sme-stated" | "assumed"
}

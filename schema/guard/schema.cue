// Package guard defines the CUE schema for a repo's guarded-edits
// configuration (docs/design/guarded-edits.md §2.1): `.fathom/guard.cue` —
// a per-project, committed, reviewable list of critical scopes where
// governing rules must be injected BEFORE the edit (PreToolUse), not just
// after.
//
// Mirrors schema/binding/schema.cue's and schema/module/schema.cue's
// conventions (closedness-as-authoring-gate, `| *default`, non-empty-list
// guards against a silently-deadened config). internal/guardspec is the
// Go-side dual-mode loader (LoadGuards / ValidateStrict), mirroring
// internal/modspec and internal/bindspec exactly.
//
// Status: v0. No auto-derived guards, no blocking, no per-user guards — see
// design §3 (non-goals).
package guard

// #Guards is the top-level shape of a `.fathom/guard.cue` file.
#Guards: {
	guards: [...#Guard] & [_, ...]
}

// #Guard names one critical scope: a set of file globs and/or command
// patterns where the first mistake is expensive enough to warrant
// pre-edit / pre-command guidance instead of the default post-edit channel.
#Guard: {
	// scopes are consumer-tree file globs this guard governs (design §2.1's
	// worked example: `["internal/store/**", "**/migrations/**"]`).
	scopes?: [...string] & [_, ...]

	// note is the author's own one-liner, served verbatim ABOVE the rules —
	// the cheapest possible "why this file is special" (design §2.1).
	note?: string

	// max_rules caps the pre-edit payload; defaults to 4.
	max_rules: int & >0 | *4

	// include_modules: also serve bound platform-module rules (phase O
	// binding, docs/design/platform-modules.md) for files this guard
	// matches — off by default (a guard is repo-local by construction; an
	// author opts in explicitly).
	include_modules: bool | *false

	// command_scopes are shell-command glob patterns (design §2.5) matched
	// against the command string of a PreToolUse(Bash) call — the same
	// guard extended to commands, not just file edits.
	command_scopes?: [...string] & [_, ...]
}

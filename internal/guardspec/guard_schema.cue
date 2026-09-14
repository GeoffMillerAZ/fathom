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
	guards: [...#Guard] // explicit `guards: []` is the sanctioned inert form (IO-7, one vocabulary with bindings)

	// layer_rules are compiled governance rules (VB-13,
	// docs/vision/29-verbs-not-lookups.md §3 wave C; ADR-035 decision 3):
	// served live at obligations time (internal/guardspec.EvaluateLayerRule)
	// AND compiled to a Go test (`fathom rules gen-test`) so the same rule
	// text fails `task validate` and any CI, not just an agent's edit.
	// Defaults to empty so a guard.cue predating this field stays valid.
	layer_rules: [...#LayerRule] | *[]
}

// #LayerRule is one compiled governance rule. Only `id`, `kind`, `sentence`
// and `severity` are schema-required; the fields a given `kind` actually
// reads (from/to; scope; allow/guarded; from_service/to_service) are
// enforced in Go (internal/guardspec.LoadLayerRules), the same split
// #Guard uses for "at least one of scopes/command_scopes" — CUE cannot
// cleanly express "these fields are required only for THIS kind" on an
// open struct.
#LayerRule: {
	// id is a short, stable, repo-unique identifier — cited by the
	// generated test's subtest name and the obligations line
	// ("layer rule <id>: <sentence>").
	id: string & =~"^[a-z][a-z0-9-]*$"

	// kind selects which structural check this rule performs.
	kind: "import_direction" | "no_cycles" | "boundary" | "cross_service_direction"

	// sentence is the human-readable governing rule, served verbatim.
	sentence: string

	// severity: "error" fails the generated test on a violation; "warn" is
	// served live but never fails the generated test (advisory-strength
	// compiled rule).
	severity: "error" | "warn" | *"error"

	// import_direction: packages matching any `from` glob may not import
	// packages matching any `to` glob.
	from?: [...string] & [_, ...]
	to?: [...string] & [_, ...]

	// no_cycles: package glob(s) to check for import cycles among
	// themselves — a package outside `scope` is never part of a reported
	// cycle even if it sits on a path between two in-scope packages.
	scope?: [...string] & [_, ...]

	// boundary: only a package matching an `allow` glob may import a
	// package matching a `guarded` glob.
	allow?: [...string] & [_, ...]
	guarded?: [...string] & [_, ...]

	// cross_service_direction: from_service must never depend on
	// to_service, over the fleet's typed cross-service edges.
	from_service?: string
	to_service?: string
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

// Package plan defines the CUE schema for fathom plan files and their
// generated verification status output.
//
// A plan file declares a set of tasks with acceptance contracts (#Plan).
// The gen-status-cue command evaluates each task against the live code index
// and writes a status document (#Status) that is derived from this schema.
//
// Source of truth: the Markdown plan file. #Plan documents are GENERATED
// (fathom gen-plan-cue) — the typed shadow of the Markdown grammar (ADR-018);
// hand-authoring plan CUE is deprecated. #Status is generated — do not
// hand-edit it.
package plan

// #Expect is the acceptance contract for a single plan task. The zero value
// is the STRICTEST contract: a test must statically reach the symbol, any
// contradiction fails the task, and the symbol must be at the "verified" tier.
// Loosen individual fields explicitly.
#Expect: {
	// symbol is the claimed symbol name that must exist in the code index.
	symbol?: string

	// package is an optional repo-relative directory the symbol's file must
	// live under (disambiguates bare names).
	package?: string

	// reachable_from is an optional entrypoint that must reach symbol via
	// the static call graph.
	reachable_from?: string

	// skip_reason justifies skip_test (the "skip:" annotation key).
	skip_reason?: string

	// skip_test, when true, waives the requirement that a Test* function
	// statically reaches symbol. Default: false (test IS required).
	skip_test?: bool | *false

	// allow_contradiction, when true, does not fail the task when
	// conflicting evidence is detected for symbol. Default: false.
	allow_contradiction?: bool | *false

	// min_tier is the minimum evidence tier required for symbol. Default:
	// "verified" (extracted directly from source code).
	min_tier?: "verified" | "pre-scan" | "doc-claimed" | "sme-stated" | "assumed" | *"verified"

	// oracle names a non-index contract (ADR-030). "lexical" is the only
	// class: path must exist in the working tree and be non-empty, and
	// pattern (a Go regexp, line-anchored) must match it at least once.
	// One contract per row — a task carrying both symbol and oracle is
	// refused by verify.LintPlan and fails verification.
	oracle?: "lexical"

	// path is the repo-relative file a lexical oracle reads. Required with
	// oracle; never absolute, never escaping the repo.
	path?: string

	// pattern is the regexp a lexical oracle must match in path. Required
	// when path is a markdown file (existence proves nothing about prose);
	// optional for other files, where existence + non-empty suffices.
	pattern?: string

	// max/min/eq bound a lexical oracle's pattern match count (ADR-044
	// §3.1, CV-29): omitted keeps the oracle's original "at least one
	// match" default. eq is refused combined with either max or min — one
	// bound shape per row.
	max?: int
	min?: int
	eq?:  int
}

// #Task is one unit of planned work with an acceptance contract.
#Task: {
	// id is the unique task identifier (e.g. "T-1", "PROJ-42").
	id: string

	// title is a human-readable description of the task.
	title?: string

	// spec_ids lists declared specification identifiers that this task
	// implements (e.g. ["[US-042]", "[FR-007]"]).
	spec_ids?: [...string]

	// state is the authored checkbox state (grammar v2): absent = open.
	state?: "claimed" | "in-progress" | "skipped"

	// disposition refines WHY a skipped task is skipped (requires reason).
	disposition?: "refused" | "deferred"

	// reason is the free-text justification for a disposition or skip.
	reason?: string

	// trigger is the revival condition for a deferred task — recorded and
	// surfaced only, never machine-evaluated in v0 (ADR-018).
	trigger?: string

	// expect is the acceptance contract for this task.
	expect?: #Expect

	// gone declares an absence contract this row proves once claimed
	// (ADR-044 §3.1): "import <path>", "module <path>", `pattern
	// "<regex>"` (optionally with paths), or "callers <Symbol>" (with
	// package). Absence proofs, never presence.
	gone?: string

	// tests refines a `gone: callers` row: "allowed" permits nonzero test
	// referents to the retired symbol; default requires zero test
	// referents too.
	tests?: string

	// retires lists capability-inventory keys (ADR-044 §2.1) this row
	// declares lost — exact match or a "<prefix>*" wildcard.
	retires?: [...string]

	// surface_change lists capability-inventory keys this row declares
	// intentionally changed (not removed) — same matching rule as retires.
	surface_change?: [...string]
}

// #Plan is the top-level schema for a hand-edited plan file.
#Plan: {
	// service is the fathom service ID to evaluate the plan against.
	service: string

	// baseline pins the base commit every row in this plan is proved
	// against (ADR-044 §4, the "Baseline: <sha>" header line) — written
	// by `fathom refactor plan`, read by `fathom refactor status` and
	// `fathom conserve`.
	baseline?: string

	// tasks is the list of tasks to verify.
	tasks: [...#Task]
}

// ─── Output (generated) ──────────────────────────────────────────────────────
//
// The types below describe the #Status document written by gen-status-cue.
// They are read-only from the plan author's perspective.

// #CriterionStatus is the outcome of a single acceptance criterion.
#CriterionStatus: "pass" | "fail" | "unknown" | "n/a"

// #Criterion is one checked clause of a task's contract.
#Criterion: {
	// name is the criterion identifier (e.g. "symbol_present", "tier",
	// "test_reaches", "edge_wired", "no_contradiction", "spec_covered",
	// "runtime_pass").
	name: string

	// status is the outcome of this criterion.
	status: #CriterionStatus

	// detail is a human-readable citation (file:line, tier label, etc.).
	detail?: string
}

// #TaskVerdict is the rolled-up verdict for one task.
#TaskVerdict: {
	// id is the task identifier from the plan.
	id: string

	// title is the task title (may be absent if not set in the plan).
	title?: string

	// status is the overall outcome: pass, fail, or unknown.
	status: "pass" | "fail" | "unknown"

	// label is a short verdict label (e.g. "verified", "claimed-but-absent",
	// "present-but-untested", "edge-not-wired", "contradicted",
	// "likely-unconfirmed", "below-required-tier", "spec-uncovered",
	// "no-contract").
	label: string

	// criteria is the ordered list of individual criterion outcomes.
	criteria: [...#Criterion]
}

// #Status is the top-level schema for the generated STATUS.cue document.
#Status: {
	// service is the fathom service ID that was evaluated.
	service: string

	// summary holds report-level pass/fail/unknown counts.
	summary: {
		total:   int
		pass:    int
		fail:    int
		unknown: int
	}

	// tasks is the list of per-task verdicts, sorted by task ID.
	tasks: [...#TaskVerdict]
}

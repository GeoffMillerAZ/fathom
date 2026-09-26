// Package researchbundle defines the CUE schema for fathom's sanitized,
// shareable research-flywheel bundle (`fathom research bundle`,
// docs/design/research-flywheel.md). It is the ALLOWLIST, enforced twice:
// internal/research/bundle.Build only ever writes typed Go fields sourced
// from an already-aggregated Input (Go-side allowlist — see build.go's own
// doc comment), and this closed struct rejects any field outside the named
// set (CUE-side allowlist) — a recipient can validate a shared bundle.json
// independently of the fathom binary, with no need to trust the sender.
//
// This file is authoritative AND live: internal/research/bundle embeds a
// byte-identical copy (internal/research/bundle/research_bundle_schema.cue,
// drift-tested against this file — see internal/config/schema_test.go for
// the repo's precedent) and validates every bundle it builds against it
// before writing.
package researchbundle

// #Label is fathom's own tool/surface/bench-arm name shape: lowercase ASCII
// letters, digits, underscore, 1-64 chars — or the literal "unrecognized"
// sentinel (itself already matching this pattern, so no alternation is
// needed for it). Critic Finding 1 (2026-07-28, proven end-to-end): a
// call-log Tool/Surface value is logged verbatim even for an unresolved
// tool name or an unrecognized surface — client-controlled strings, not
// fathom-controlled ones, despite the Go field being typed string. This is
// the CUE-side half of the fix (internal/research/bundle's sanitizeLabel is
// the Go-side half): a bundle carrying a value outside this shape fails
// validation here even if the binary that produced it had a bug.
#Label: =~"^[a-z0-9_]{1,64}$"

#SurfaceCount: close({
	// surface is fathom's own surface label (e.g. "agent", "hook",
	// "companion") or "" for bare/unscoped — never a repo or service name,
	// and never anything outside #Label's shape (or empty).
	surface: "" | #Label
	calls:   int & >=0
})

#ToolCount: close({
	// tool is one of fathom's own registered MCP/CLI tool names, or the
	// "unrecognized" sentinel for anything that didn't match #Label.
	tool:  #Label
	calls: int & >=0
})

#ServiceHashCount: close({
	// service_hash is a salted sha256 hex digest (64 lowercase hex chars) —
	// never the plaintext service/repo name. The salt that produced it is
	// generated once, stored locally, and never leaves the machine.
	service_hash: =~"^[0-9a-f]{64}$"
	calls:        int & >=0
})

#BenchArmCount: close({
	// arm is a context-bench arm label ("fathom", "grep", …) — not a repo
	// name, and constrained to #Label like tool/surface.
	arm:           #Label
	task_count:    int & >=0
	success_count: int & >=0
	success_rate:  number & >=0 & <=1
	mean_tokens:   number & >=0
	mean_wall_ms:  number & >=0
})

// #CallTraffic is the MCP call log's aggregate view. Present in #Bundle
// only when the call log was actually read over the report window (critic
// Finding 3): its absence means "not instrumented," never a fabricated
// all-zero group.
#CallTraffic: close({
	total_calls: int & >=0
	surface_traffic?: [...#SurfaceCount]
	tool_calls?: [...#ToolCount]
	latency_p50_ms?:        number & >=0
	latency_p95_ms?:        number & >=0
	staleness_report_calls: int & >=0
})

// #Injection is the hooks beat log's aggregate view. Present only when that
// (separate) log was actually read.
#Injection: close({
	seen:                int & >=0
	answered:            int & >=0
	coverage_rate?:      number & >=0 & <=1
	obligations_served:  int & >=0
})

// #VerifyGate is the call log's verify_plan/review aggregate. Present under
// the same condition as #CallTraffic (same underlying instrument).
#VerifyGate: close({
	claimed:                  int & >=0
	verified:                 int & >=0
	failing:                  int & >=0
	instrumented_calls:       int & >=0
	predates_instrumentation: int & >=0
	review_active_findings:   int & >=0
})

// #RetroOutcomeCount is one (channel, mechanical outcome) cell of the
// retro corpus (IR-5, docs/design/assess-lanes.md §5): channel is fathom's
// own firing channel ("guard", "obligations") and outcome its own
// mechanical outcome ("committed", "modified_uncommitted", "untouched",
// "unjoinable") — both closed vocabularies mapped through #Label, anything
// else collapsing to "unrecognized". Never a file, a session, or a path.
#RetroOutcomeCount: close({
	channel: #Label
	outcome: #Label
	firings: int & >=0
})

// #Retros is the opt-in retro corpus group (IR-5): present only when the
// human passed --include-retros AND the corpus directory was read — its
// absence means "not read", never an all-zero stand-in. Counts only; no
// assess verdict ever rides here (uncalibrated verdicts stay local).
#Retros: close({
	retros:                int & >=0
	unreadable:            int & >=0
	off_arm_retros:        int & >=0
	guard_fires:           int & >=0
	obligation_injections: int & >=0
	silent_on_clean:       int & >=0
	unjoinable:            int & >=0
	outcomes?: [...#RetroOutcomeCount]
	custodian_remedied:   int & >=0
	custodian_unremedied: int & >=0
})

// #Bundle is the closed top-level shape — the allowlist itself. Any field
// not named here fails validation by construction: a stray "file_path" or
// "repo" key introduced upstream by mistake is a hard CUE error, never a
// silent pass-through.
#Bundle: close({
	schema_version: string

	// window_since/window_until/generated_at are RFC3339 timestamps — the
	// report window and generation time, never a hostname or timezone name.
	window_since: string
	window_until: string
	generated_at: string

	// fathom's own version/commit identity — never the adopting repo's.
	fathom_version: string
	commit_sha?:    =~"^[0-9a-f]{7,40}$"
	commit_dirty:   bool
	// coarse platform only (Go's runtime.GOOS/GOARCH) — never a hostname.
	goos:   string
	goarch: string

	call_traffic?: #CallTraffic
	injection?:    #Injection
	verify_gate?:  #VerifyGate

	total_observations?: int & >=0
	open_observations?:  int & >=0

	service_activity?: [...#ServiceHashCount]

	bench_arms?: [...#BenchArmCount]

	retros?: #Retros
})

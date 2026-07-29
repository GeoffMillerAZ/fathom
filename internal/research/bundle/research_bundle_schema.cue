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

#SurfaceCount: close({
	// surface is fathom's own surface label (e.g. "agent", "hook",
	// "companion", or "" for bare/unscoped) — never a repo or service name.
	surface: string
	calls:   int & >=0
})

#ToolCount: close({
	// tool is one of fathom's own registered MCP/CLI tool names.
	tool:  string
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
	// name.
	arm:           string
	task_count:    int & >=0
	success_count: int & >=0
	success_rate:  number & >=0 & <=1
	mean_tokens:   number & >=0
	mean_wall_ms:  number & >=0
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

	total_calls: int & >=0
	surface_traffic?: [...#SurfaceCount]
	tool_calls?: [...#ToolCount]

	latency_p50_ms?: number & >=0
	latency_p95_ms?: number & >=0

	injection_seen:           int & >=0
	injection_answered:       int & >=0
	injection_coverage_rate?: number & >=0 & <=1
	obligations_served:       int & >=0

	verify_claimed:                  int & >=0
	verify_verified:                 int & >=0
	verify_failing:                  int & >=0
	verify_instrumented_calls:       int & >=0
	verify_predates_instrumentation: int & >=0
	review_active_findings:          int & >=0

	staleness_report_calls: int & >=0

	total_observations?: int & >=0
	open_observations?:  int & >=0

	service_activity?: [...#ServiceHashCount]

	bench_arms?: [...#BenchArmCount]
})

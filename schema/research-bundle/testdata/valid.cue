package researchbundle

// A concrete, fully-populated #Bundle instance — `cue vet ./...` checks this
// validates against the schema in the same directory.
example: #Bundle & {
	schema_version: "v1"

	window_since: "2026-07-21T00:00:00Z"
	window_until: "2026-07-28T00:00:00Z"
	generated_at: "2026-07-28T00:00:00Z"

	fathom_version: "fathom v0.1.0"
	commit_sha:     "abc1234def5"
	commit_dirty:   false
	goos:           "darwin"
	goarch:         "arm64"

	total_calls: 42
	surface_traffic: [
		{surface: "agent", calls: 30},
		{surface: "hook", calls: 12},
	]
	tool_calls: [
		{tool: "find_symbol", calls: 20},
		{tool: "verify_plan", calls: 5},
	]

	latency_p50_ms: 12.5
	latency_p95_ms: 88.0

	injection_seen:           10
	injection_answered:       9
	injection_coverage_rate:  0.9
	obligations_served:       8

	verify_claimed:                  5
	verify_verified:                 4
	verify_failing:                  1
	verify_instrumented_calls:       5
	verify_predates_instrumentation: 2
	review_active_findings:          3

	staleness_report_calls: 2

	total_observations: 6
	open_observations:  2

	service_activity: [
		{service_hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85", calls: 30},
	]

	bench_arms: [
		{arm: "fathom", task_count: 13, success_count: 13, success_rate: 1.0, mean_tokens: 5000, mean_wall_ms: 42000},
		{arm: "grep", task_count: 13, success_count: 11, success_rate: 0.846, mean_tokens: 8800, mean_wall_ms: 91000},
	]
}

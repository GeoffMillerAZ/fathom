package researchbundle

// A concrete, fully-populated #Bundle instance — `cue vet ./...` checks this
// validates against the schema in the same directory.
example: #Bundle & {
	schema_version: "v2"

	window_since: "2026-07-21T00:00:00Z"
	window_until: "2026-07-28T00:00:00Z"
	generated_at: "2026-07-28T00:00:00Z"

	fathom_version: "fathom v0.1.0"
	commit_sha:     "abc1234def5"
	commit_dirty:   false
	goos:           "darwin"
	goarch:         "arm64"

	call_traffic: {
		total_calls: 42
		surface_traffic: [
			{surface: "agent", calls: 30},
			{surface: "hook", calls: 12},
		]
		tool_calls: [
			{tool: "find_symbol", calls: 20},
			{tool: "verify_plan", calls: 5},
			{tool: "unrecognized", calls: 1},
		]
		latency_p50_ms:         12.5
		latency_p95_ms:         88.0
		staleness_report_calls: 2
	}

	injection: {
		seen:               10
		answered:           9
		coverage_rate:      0.9
		obligations_served: 8
	}

	verify_gate: {
		claimed:                  5
		verified:                 4
		failing:                  1
		instrumented_calls:       5
		predates_instrumentation: 2
		review_active_findings:   3
	}

	total_observations: 6
	open_observations:  2

	service_activity: [
		{service_hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", calls: 30},
	]

	bench_arms: [
		{arm: "fathom", task_count: 13, success_count: 13, success_rate: 1.0, mean_tokens: 5000, mean_wall_ms: 42000},
		{arm: "grep", task_count: 13, success_count: 11, success_rate: 0.846, mean_tokens: 8800, mean_wall_ms: 91000},
	]

	// IR-5: the opt-in retro corpus group — counts over closed labels only.
	retros: {
		retros:                4
		unreadable:            0
		off_arm_retros:        1
		guard_fires:           3
		obligation_injections: 5
		silent_on_clean:       12
		unjoinable:            1
		outcomes: [
			{channel: "guard", outcome: "committed", firings: 2},
			{channel: "obligations", outcome: "modified_uncommitted", firings: 1},
		]
		custodian_remedied:   1
		custodian_unremedied: 0
	}
}

// A minimal instance: no call_traffic/injection/verify_gate at all (the
// not-instrumented case, critic Finding 3) still validates — every group is
// optional.
minimal: #Bundle & {
	schema_version: "v2"
	window_since:   "2026-07-21T00:00:00Z"
	window_until:   "2026-07-28T00:00:00Z"
	generated_at:   "2026-07-28T00:00:00Z"
	fathom_version: "fathom v0.1.0"
	commit_dirty:   false
	goos:           "darwin"
	goarch:         "arm64"
}

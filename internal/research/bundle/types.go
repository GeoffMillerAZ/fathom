// Package bundle builds fathom's sanitized, shareable research-flywheel
// bundle (`fathom research bundle`, docs/design/research-flywheel.md). The
// package is small and single-purpose on purpose: Bundle is the ALLOWLIST —
// the only fields a shareable bundle may ever carry — and Build (build.go)
// is the ONLY constructor, so a raw map or free-text field from local
// evidence (a repo path, a symptom string, an error message) has no code
// path into the output type.
package bundle

// Bundle is the sanitized, shareable research-flywheel artifact. Every
// field here is safe to hand to a third party: aggregate counts, rates,
// percentiles, durations, fathom's own tool-name frequencies, verdict
// distributions, fathom's own version/commit, coarse platform, and the
// report window. Never construct one by hand outside Build — Build is the
// allowlist boundary (see build.go's doc comment).
//
// Field order and JSON tags mirror schema/research-bundle/schema.cue's
// #Bundle exactly; internal/research/bundle/research_bundle_schema.cue
// embeds that file byte-identical (schema_test.go's drift gate).
type Bundle struct {
	SchemaVersion string `json:"schema_version"`

	// WindowSince/WindowUntil/GeneratedAt are RFC3339 timestamps — the
	// report window and generation time. Never a hostname, never a
	// timezone name beyond UTC.
	WindowSince string `json:"window_since"`
	WindowUntil string `json:"window_until"`
	GeneratedAt string `json:"generated_at"`

	// FathomVersion/CommitSHA/CommitDirty identify the fathom BINARY that
	// produced this bundle — never the adopting repo's own version/commit.
	FathomVersion string `json:"fathom_version"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	CommitDirty   bool   `json:"commit_dirty"`
	// GOOS/GOARCH are Go's own coarse platform identifiers — never a
	// hostname or a more granular OS version string.
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`

	TotalCalls     int            `json:"total_calls"`
	SurfaceTraffic []SurfaceCount `json:"surface_traffic,omitempty"`
	ToolCalls      []ToolCount    `json:"tool_calls,omitempty"`

	LatencyP50MS *float64 `json:"latency_p50_ms,omitempty"`
	LatencyP95MS *float64 `json:"latency_p95_ms,omitempty"`

	InjectionSeen         int      `json:"injection_seen"`
	InjectionAnswered     int      `json:"injection_answered"`
	InjectionCoverageRate *float64 `json:"injection_coverage_rate,omitempty"`
	ObligationsServed     int      `json:"obligations_served"`

	VerifyClaimed                 int `json:"verify_claimed"`
	VerifyVerified                int `json:"verify_verified"`
	VerifyFailing                 int `json:"verify_failing"`
	VerifyInstrumentedCalls       int `json:"verify_instrumented_calls"`
	VerifyPredatesInstrumentation int `json:"verify_predates_instrumentation"`
	ReviewActiveFindings          int `json:"review_active_findings"`

	StalenessReportCalls int `json:"staleness_report_calls"`

	TotalObservations *int `json:"total_observations,omitempty"`
	OpenObservations  *int `json:"open_observations,omitempty"`

	// ServiceActivity distinguishes per-service traffic WITHOUT naming any
	// service: ServiceHash is a salted sha256 hex digest (hash.go), never
	// the plaintext service/repo ID.
	ServiceActivity []ServiceHashCount `json:"service_activity,omitempty"`

	BenchArms []BenchArmCount `json:"bench_arms,omitempty"`
}

// SurfaceCount is one fathom surface's call count in the window. Surface is
// fathom's own surface label ("agent", "hook", "companion", or "" for
// bare/unscoped) — never a repo or service name.
type SurfaceCount struct {
	Surface string `json:"surface"`
	Calls   int    `json:"calls"`
}

// ToolCount is one fathom tool's call count in the window. Tool is one of
// fathom's own registered MCP/CLI tool names.
type ToolCount struct {
	Tool  string `json:"tool"`
	Calls int    `json:"calls"`
}

// ServiceHashCount is one service's call count, identified only by a salted
// hash (hash.go) — the plaintext service/repo name is discarded the moment
// Build computes the hash and never reaches this struct.
type ServiceHashCount struct {
	ServiceHash string `json:"service_hash"`
	Calls       int    `json:"calls"`
}

// BenchArmCount is one context-bench arm's aggregate outcome. Arm is a bench
// arm label ("fathom", "grep", …), never a repo name.
type BenchArmCount struct {
	Arm          string  `json:"arm"`
	TaskCount    int     `json:"task_count"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	MeanTokens   float64 `json:"mean_tokens"`
	MeanWallMS   float64 `json:"mean_wall_ms"`
}

// Package bundle builds fathom's sanitized, shareable research-flywheel
// bundle (`fathom research bundle`, docs/design/research-flywheel.md). The
// package is small and single-purpose on purpose: Bundle is the ALLOWLIST —
// the only fields a shareable bundle may ever carry — and Build (build.go)
// is the ONLY constructor, so a raw map or free-text field from local
// evidence (a repo path, a symptom string, an error message) has no code
// path into the output type.
package bundle

// SchemaVersionCurrent bumped v1 -> v2 (critic Finding 3, 2026-07-28): the
// call-traffic/injection/verify-gate fields moved from bare top-level
// scalars to three optional pointer groups (CallTraffic/Injection/
// VerifyGate below) so a recipient can tell "this instrument was never
// read" (group absent) apart from "this instrument was read and genuinely
// measured zero" (group present with 0 values) — the same never-a-
// fabricated-zero doctrine the local (unsanitized) report already honors,
// now also on the artifact third parties actually read.

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

	// CallTraffic is nil when the MCP call log was absent/unreadable over
	// this window — "not instrumented," never a fabricated all-zero group
	// (critic Finding 3). Present (non-nil) whenever the log WAS read, even
	// if it held zero rows in the window: that is a real, measured zero.
	CallTraffic *CallTrafficStats `json:"call_traffic,omitempty"`
	// Injection is nil when the hooks beat log was absent/unreadable — same
	// not-instrumented-vs-real-zero discipline, a separate instrument from
	// CallTraffic (the beat log is a different file).
	Injection *InjectionStats `json:"injection,omitempty"`
	// VerifyGate is nil under the same condition as CallTraffic — verify
	// stats are call-log-derived, the same instrument.
	VerifyGate *VerifyGateStats `json:"verify_gate,omitempty"`

	TotalObservations *int `json:"total_observations,omitempty"`
	OpenObservations  *int `json:"open_observations,omitempty"`

	// ServiceActivity distinguishes per-service traffic WITHOUT naming any
	// service: ServiceHash is a salted sha256 hex digest (hash.go), never
	// the plaintext service/repo ID.
	ServiceActivity []ServiceHashCount `json:"service_activity,omitempty"`

	BenchArms []BenchArmCount `json:"bench_arms,omitempty"`

	// Retros is the opt-in retro corpus group (IR-5,
	// docs/design/assess-lanes.md §5): nil unless the human passed
	// --include-retros AND the corpus directory was read — "not read",
	// never an all-zero stand-in. Counts over closed labels only.
	Retros *RetroStats `json:"retros,omitempty"`
}

// RetroStats aggregates the local retro corpus (the per-session artifacts
// `fathom retro --write` and the Stop hook land under the state directory's
// artifacts/retros/). Every field is a count; Outcomes' labels are fathom's
// own closed vocabularies, mapped by the caller (cmd/fathom's
// aggregateRetroCorpus) — never a file, session id, or path.
type RetroStats struct {
	Retros               int                 `json:"retros"`
	Unreadable           int                 `json:"unreadable"`
	OffArmRetros         int                 `json:"off_arm_retros"`
	GuardFires           int                 `json:"guard_fires"`
	ObligationInjections int                 `json:"obligation_injections"`
	SilentOnClean        int                 `json:"silent_on_clean"`
	Unjoinable           int                 `json:"unjoinable"`
	Outcomes             []RetroOutcomeCount `json:"outcomes,omitempty"`
	CustodianRemedied    int                 `json:"custodian_remedied"`
	CustodianUnremedied  int                 `json:"custodian_unremedied"`
}

// RetroOutcomeCount is one (channel, mechanical outcome) cell: how many
// firings on that channel ended in that outcome across the corpus.
type RetroOutcomeCount struct {
	Channel string `json:"channel"`
	Outcome string `json:"outcome"`
	Firings int    `json:"firings"`
}

// CallTrafficStats is the MCP call log's aggregate view over the report
// window: total calls, per-surface/per-tool breakdowns, and latency
// percentiles. Present in Bundle only when the call log was actually read
// (Build/aggregateBundleInput's CallLogRead gate) — see Bundle.CallTraffic.
type CallTrafficStats struct {
	TotalCalls           int            `json:"total_calls"`
	SurfaceTraffic       []SurfaceCount `json:"surface_traffic,omitempty"`
	ToolCalls            []ToolCount    `json:"tool_calls,omitempty"`
	LatencyP50MS         *float64       `json:"latency_p50_ms,omitempty"`
	LatencyP95MS         *float64       `json:"latency_p95_ms,omitempty"`
	StalenessReportCalls int            `json:"staleness_report_calls"`
}

// InjectionStats is the hooks beat log's aggregate view: how many
// edit/write events the edit-hook saw, how many it answered, and how many
// of those answers actually emitted an obligations payload. Present in
// Bundle only when the hooks beat log was actually read — see
// Bundle.Injection.
type InjectionStats struct {
	Seen              int      `json:"seen"`
	Answered          int      `json:"answered"`
	CoverageRate      *float64 `json:"coverage_rate,omitempty"`
	ObligationsServed int      `json:"obligations_served"`
}

// VerifyGateStats is the call log's verify_plan/review aggregate: claimed/
// verified/failing task counts, how many calls carried a recoverable
// verdict_outcome versus predated that field, and review's active-finding
// total. Present in Bundle only when the call log was actually read — see
// Bundle.VerifyGate.
type VerifyGateStats struct {
	Claimed                 int `json:"claimed"`
	Verified                int `json:"verified"`
	Failing                 int `json:"failing"`
	InstrumentedCalls       int `json:"instrumented_calls"`
	PredatesInstrumentation int `json:"predates_instrumentation"`
	ReviewActiveFindings    int `json:"review_active_findings"`
}

// SurfaceCount is one fathom surface's call count in the window. Surface is
// fathom's own surface label ("agent", "hook", "companion", or "" for
// bare/unscoped) — never a repo or service name. Constrained to
// labelPattern (or empty) by sanitizeLabel before it ever reaches here
// (critic Finding 1): a surface value is caller-declared on every request
// (the X-Fathom-Origin/path selection) and the daemon that logs it is
// always-on and unauthenticated by design, so it must never be trusted
// verbatim just because the Go field is typed string.
type SurfaceCount struct {
	Surface string `json:"surface"`
	Calls   int    `json:"calls"`
}

// ToolCount is one fathom tool's call count in the window. Tool is one of
// fathom's own registered MCP/CLI tool names, or the sanitizeLabel sentinel
// "unrecognized" — internal/mcp/dispatch.go logs the raw REQUESTED tool
// name to the call log even when it fails to resolve to a real tool, so an
// unvalidated Tool string is client-controlled, not fathom-controlled
// (critic Finding 1, proven end-to-end against the pre-fix code).
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
// arm label ("fathom", "grep", …), sanitizeLabel-constrained like Tool
// (critic Finding 1/4) — never a repo name and never raw enough to break
// out of a Markdown table cell (Finding 6).
type BenchArmCount struct {
	Arm          string  `json:"arm"`
	TaskCount    int     `json:"task_count"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	MeanTokens   float64 `json:"mean_tokens"`
	MeanWallMS   float64 `json:"mean_wall_ms"`
}

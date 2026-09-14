// Package scenariobench is the change bench (vision 27, ADR-033): a corpus of
// scenarios — a change to a pinned subject, run by an agent across sessions
// under a turn budget, in arms that differ only in data — scored by oracles
// fathom does not own. This file is the contract every other file in the
// package and cmd/scenariobench builds against; it is deliberately free of
// behaviour.
package scenariobench

import "time"

// Corpus is one committed scenario corpus (testdata/scenariobench/*.json).
type Corpus struct {
	Version     int            `json:"version"`
	Description string         `json:"description"`
	Arms        map[string]Arm `json:"arms"`
	Tasks       []Task         `json:"tasks"`
	Criterion   Criterion      `json:"criterion"`
	Sources     []SourcePin    `json:"sources,omitempty"`
	Notes       map[string]any `json:"notes,omitempty"`
}

// SourcePin records a third-party input the corpus was derived from (ADR-031).
type SourcePin struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Revision string `json:"revision"`
	License  string `json:"license"`
	SHA256   string `json:"sha256,omitempty"`
}

// Arm is data, never code: what differs between treatments. The realistic
// treatment (fathom plus Read/Grep/Bash) is the default; the control is the
// same fork stripped of fathom with the same notes-file instruction.
type Arm struct {
	// Wired runs `fathom init` on the subject (hooks, MCP entry, stanza,
	// compiled obligations when CompileLayers is true) before session 1.
	Wired         bool `json:"wired"`
	CompileLayers bool `json:"compile_layers,omitempty"`
	// MCP is the mcpServers block handed to the harness (empty = none).
	MCP map[string]any `json:"mcp,omitempty"`
	// AllowedTools / DisallowedTools are passed to the harness verbatim.
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`
	// Instructions selects the instruction-file variant written into the
	// worktree: "stanza" (fathom's), "plain" (the subject's own file
	// untouched), "none" (the file removed for the run), "text" (writes
	// InstructionsText verbatim — a competitor arm's own routing stanza,
	// RC-8).
	Instructions string `json:"instructions"`
	// InstructionsText is the CLAUDE.md body RenderArm writes for
	// Instructions == "text": a competitor arm's own routing stanza, the
	// same shape as fathom's (which verb answers which question shape),
	// so a tool that is present and un-named is never measured as unused
	// for want of a name (vision 28 §2 F2).
	InstructionsText string `json:"instructions_text,omitempty"`
	// NotesFile adds the same "keep notes in NOTES.md" instruction to every
	// session so continuity is compared against a real alternative.
	NotesFile bool `json:"notes_file,omitempty"`
	// Setup runs in the worktree before session 1 (a competitor's build).
	Setup []string `json:"setup,omitempty"`
	// Surface pins what re-keys this arm's cells: "fathom" (the fathom
	// commit), a competitor version string, or "" for a pure control.
	Surface string            `json:"surface,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// Prediction is the pre-registered expectation for this arm, per corpus.
	Prediction string `json:"prediction,omitempty"`
}

// Mode is how sessions are shaped.
type Mode string

const (
	// ModeSingle: one agent, Sessions.Count sessions in the same worktree.
	ModeSingle Mode = "single"
	// ModePlannerWorkers: one planning session writes the plan, then
	// Sessions.Workers worker sessions each receive one row.
	ModePlannerWorkers Mode = "planner-workers"
)

// Sessions is the budget that forces hand-offs.
type Sessions struct {
	Count    int  `json:"count"`
	MaxTurns int  `json:"max_turns"`
	Workers  int  `json:"workers,omitempty"`
	Mode     Mode `json:"mode"`
	// MaxWorkersTotal caps how many worker sessions ModePlannerWorkers may
	// run in total across the whole cell (O55 item 3): after a worker
	// finishes its row, runPlannerWorkers assigns the next unchecked plan
	// row to a new worker session rather than stopping at Workers sessions
	// — a plan with more rows than Workers used to leave the extra rows
	// unassigned forever. 0 defaults to Workers*3 (runPlannerWorkers).
	MaxWorkersTotal int `json:"max_workers_total,omitempty"`
}

// Subject is the pinned repository a task runs against.
type Subject struct {
	RepoURL    string   `json:"repo_url"`
	BaseCommit string   `json:"base_commit"`
	Service    string   `json:"service"`
	Language   string   `json:"language"`
	Setup      []string `json:"setup,omitempty"`
	// PostWireSetup (O45 item 4) runs AFTER the wired arm's own scratch
	// daemon is up, has ingested once, and (when CompileLayers) has
	// compiled — the correct place for a staleness/freshness task's own
	// injected drift. Setup runs before any of that, so drift placed there
	// is already visible to the first ingest and a compiled claim's
	// citation, defeating the very staleness/freshness comparison the task
	// exists to make (H1-T12/H1-T13/FRESH-M7-01, docs/bench/scenarios.md).
	// Runs for EVERY arm (wired or not) via PrepareSubject's
	// applyPostWireSetup, in wt.Path, after the (unwired) arm's own Setup
	// has already had its chance to build a precomputed artifact from the
	// pre-drift tree.
	PostWireSetup []string `json:"post_wire_setup,omitempty"`
	// TestCommand runs the oracle's test selection; {{tests}} is replaced
	// by the regex of test names. Default: go test -run '{{tests}}' ./...
	TestCommand string `json:"test_command,omitempty"`
	// ExcludePaths (O53 defect 3) are glob patterns, relative to the
	// worktree root, that PrepareSubject removes from the worktree at
	// prepare time — after checkout, before any ingest, for EVERY arm —
	// so a file the corpus itself was derived from (an answer key) never
	// sits inside the worktree an agent can Read/Grep/Bash its way into.
	// The corpus loader (corpus.go) requires this to cover
	// "testdata/contextbench/**", "testdata/scenariobench/**" and
	// "docs/bench/**" for any task whose derivation names the home lookup
	// corpus (testdata/contextbench/tasks-home.json).
	ExcludePaths []string `json:"exclude_paths,omitempty"`
}

// Detector is a rule's own enforcement: a command whose exit code decides.
type Detector struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	ExpectExit int    `json:"expect_exit"`
}

// Oracle decides success and violations, after the last session, never by
// the agent.
type Oracle struct {
	FailToPass []string   `json:"fail_to_pass,omitempty"`
	PassToPass []string   `json:"pass_to_pass,omitempty"`
	Detectors  []Detector `json:"detectors,omitempty"`
	// Coverage requests a coverprofile from the test run (SDD readings).
	Coverage bool `json:"coverage,omitempty"`
	// AnswerAllOf is the answer-only oracle (SB-32 lookups-vs.json): every
	// token must appear (case-insensitive substring, subject to
	// oracleAnswerMatches' negation/quoting/conclusion-span rules — O53
	// defect 5) in the LAST session's SessionResult.FinalAnswer. A task
	// may carry this ALONE — no tree-level check at all — see AnswerOnly
	// (oracle.go).
	AnswerAllOf []string `json:"answer_all_of,omitempty"`
	// AnswerAnyOf declares groups of spelling-equivalent gold tokens (O53
	// defect 5a, mirroring internal/contextbench.Task.AnswerAnyOf/VB-36):
	// each inner slice is one required fact, satisfied by ANY member
	// appearing in the answer — the same false-fail contextbench's own
	// H1-T02 hit ("Edge struct" vs the literal "core.Edge") for the same
	// fact spelled differently. Groups are required exactly like
	// AnswerAllOf's own tokens: every group (and every plain AnswerAllOf
	// token) must be satisfied. AnswerAllOf keeps working unchanged when
	// this is empty — AnswerAnyOf only ADDS grouped alternatives.
	AnswerAnyOf [][]string `json:"answer_any_of,omitempty"`
	// Localization is VB-25's "files" oracle (vision 29 Wave G, the LCA
	// bug-localization track): the LAST session's FinalAnswer is parsed
	// for a file set (ParseAnswerFiles) and scored against Gold.Files
	// (ScoreLocalization) — precision, recall, hit@1, hit@5 — instead of
	// a test run. Like AnswerAllOf, a task may carry this ALONE with no
	// tree-level check at all; unlike AnswerAllOf, success is decided by
	// the top-ranked file matching gold (LocalizationOnly, oracle.go), not
	// substring containment. Never combined with AnswerAllOf on the same
	// task — the two answer-only oracles ask different questions.
	Localization bool `json:"localization,omitempty"`
}

// Gold is what the accepted change touched, from the pinned patch.
type Gold struct {
	Files     []string `json:"files,omitempty"`
	Functions []string `json:"functions,omitempty"`
	PatchSHA  string   `json:"patch_sha256,omitempty"`
	// TestPatch is the upstream TEST PATCH verbatim (SWE-bench semantics:
	// hidden from the agent's sessions, applied by the oracle at
	// evaluation time — docs/bench/scenarios.md "Test patch and
	// calibration"). Empty when the source carried none.
	TestPatch string `json:"test_patch,omitempty"`
	// TestPatchSHA256 pins TestPatch's bytes, so a calibration cache keyed
	// on it invalidates the moment the patch text changes.
	TestPatchSHA256 string `json:"test_patch_sha256,omitempty"`
}

// Rule is the documented rule a guards task is built around (M1).
type Rule struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Source string `json:"source"`
}

// ViolationProbe (RC-50, docs/plans/receipts.md) is a tiny scripted edit
// that violates a guards task's own Rule — the corpus's mechanical proof
// that its detectors actually detect, not merely that they run. A probe
// test (internal/scenariobench/guardsm1_probe_test.go) applies task's own
// Subject.Setup (if any — some tasks materialize their edit-site scaffold
// there rather than committing it to this repo) and then Commands to a
// throwaway copy of the subject tree, and asserts the task's rule
// detector goes from green to red. Commands are self-contained shell (a
// sed, a heredoc, a git commit) — never a model call.
type ViolationProbe struct {
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
}

// Category is the change's shape, assigned by ClassifyChange or by hand
// with a derivation note.
type Category string

const (
	CategoryBugFix      Category = "bug_fix"
	CategorySignature   Category = "signature_change"
	CategoryFeature     Category = "cross_package_feature"
	CategoryGovernance  Category = "governance_bound"
	CategoryUpgrade     Category = "dependency_upgrade"
	CategoryOrientation Category = "orientation"
	CategoryFreshness   Category = "freshness"
	// CategoryLocalization is VB-25 (vision 29 Wave G): a bug-localization
	// task (Oracle.Localization) — "which files does the fix need", never
	// an edit — distinct from every other category here, all of which
	// either ARE an edit or assess one.
	CategoryLocalization Category = "localization"
)

// Task is one scenario.
type Task struct {
	ID             string            `json:"id"`
	Category       Category          `json:"category"`
	Prompt         string            `json:"prompt"`
	Subject        Subject           `json:"subject"`
	Sessions       Sessions          `json:"sessions"`
	Oracle         Oracle            `json:"oracle"`
	Gold           Gold              `json:"gold,omitzero"`
	Rule           *Rule             `json:"rule,omitempty"`
	ViolationProbe *ViolationProbe   `json:"violation_probe,omitempty"`
	Predictions    map[string]string `json:"predictions"`
	Derivation     string            `json:"derivation"`
	Requires       []string          `json:"requires,omitempty"`
	MergedAt       string            `json:"merged_at,omitempty"`
	// Disposition marks a task the driver must not run: "dropped" — an
	// oracle that cannot be won from the task text (RC-15, vision 28 §2
	// F10: SB-cli__cli-10009's fail_to_pass names tests of a package
	// absent at base). Empty runs the task normally.
	Disposition string `json:"disposition,omitempty"`
	// Reason names why Disposition was set — required whenever Disposition
	// is non-empty, so a dropped task is never silent about why.
	Reason string `json:"reason,omitempty"`
	// RequiresLayer (O45 item 1) says this task's prompt assumes a wired
	// arm's compiled obligations layer already serves rules — nil defaults
	// to true for Category obligations/staleness/claim_provenance/verify
	// (requiresCompiledLayer, subject.go), false otherwise. Set explicitly
	// to override either direction.
	RequiresLayer *bool `json:"requires_layer,omitempty"`
	// ProbeFile (O45 item 1) is the file PrepareSubject asks a wired,
	// layer-requiring task's own scratch daemon to serve obligations for,
	// after compiling: zero rules back means the compiled layer is not
	// actually reachable and the cell is LOST rather than measuring an
	// empty layer. "" defaults to DefaultProbeFile.
	ProbeFile string `json:"probe_file,omitempty"`
}

// Criterion is the pre-registered bar for a corpus.
type Criterion struct {
	ID                         string  `json:"id"`
	Treatment                  string  `json:"treatment"`
	Control                    string  `json:"control"`
	MinTrials                  int     `json:"min_trials"`
	RequireSuccessAtLeastEqual bool    `json:"require_success_at_least_equal"`
	MaxTokensPerSuccessRatio   float64 `json:"max_tokens_per_success_ratio"`
	Registered                 string  `json:"registered"`
}

// BoxCheck is one `[x]` the agent wrote into a plan or checklist.
type BoxCheck struct {
	Session int    `json:"session"`
	Turn    int    `json:"turn"`
	RowID   string `json:"row_id,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
	Package string `json:"package,omitempty"`
	File    string `json:"file"`
	// Text is the raw checklist line as written (e.g. "- [x] implement
	// `Foo`"), captured whether or not the row carries a v2-grammar
	// {symbol: ...} contract. FalseDoneRate uses it, at scoring time, as a
	// fallback source of a symbol for a row Symbol left empty (a plain
	// checklist with no contract) — never at record time, since no tree
	// exists yet to confirm a backticked identifier names a real function.
	Text string `json:"text,omitempty"`
}

// SessionResult is the harness-independent record of one session.
type SessionResult struct {
	Index             int            `json:"index"`
	Role              string         `json:"role"` // single | planner | worker
	RowID             string         `json:"row_id,omitempty"`
	Turns             int            `json:"turns"`
	ToolSearchTurns   int            `json:"toolsearch_turns"`
	InputTokens       int64          `json:"input_tokens"`
	OutputTokens      int64          `json:"output_tokens"`
	CacheReadTokens   int64          `json:"cache_read_tokens"`
	CacheCreation     int64          `json:"cache_creation_tokens"`
	TotalTokens       int64          `json:"total_tokens"`
	CostUSD           float64        `json:"cost_usd"`
	FirstEditTurn     int            `json:"first_edit_turn"` // 0 = no edit
	TokensToFirstEdit int64          `json:"tokens_to_first_edit"`
	FilesRead         []string       `json:"files_read"`   // sorted, unique, repo-relative
	FilesEdited       []string       `json:"files_edited"` // sorted, unique
	BytesRead         int64          `json:"bytes_read"`
	BoxChecks         []BoxCheck     `json:"box_checks,omitempty"`
	ToolCalls         map[string]int `json:"tool_calls"`
	// MCPServers maps every MCP server name the transcript's own
	// system/init event reported to the connection status it gave that
	// server ("connected", or whatever the harness reported otherwise) —
	// set only from that event, never inferred from ToolCalls. Nil when
	// the transcript carried no init event with an mcp_servers list at all
	// (the defect this type closes: a session can show tool_calls with no
	// way to prove any MCP server was actually behind them).
	MCPServers map[string]string `json:"mcp_servers,omitempty"`
	// MCPTools is the init event's own tool list, counted for entries
	// namespaced to an MCP server ("mcp__" prefix) — a session whose
	// MCPServers all report connected but MCPTools is 0 still ran with no
	// MCP tool schema loaded (Claude Code defers MCP schemas behind
	// ToolSearch unless the config entry carries "alwaysLoad": true).
	MCPTools     int           `json:"mcp_tools,omitempty"`
	PayloadBytes int64         `json:"payload_bytes"`
	WallTime     time.Duration `json:"wall_time_ns"`
	RawPath      string        `json:"raw_path,omitempty"`
	// HooksFired maps every hook event name (e.g. "SessionStart",
	// "PostToolUse") to how many `system`/`hook_response` events the
	// transcript carried for it, counted regardless of the hook's own
	// outcome — a hook that fired and errored still proves the harness's
	// `--settings` file wired something, unlike one that never fired at
	// all (docs/bench/scenarios.md's clean-room smoke test: real shape is
	// `{"type":"system","subtype":"hook_response","hook_event":"SessionStart",...}`).
	// Nil when the transcript carried no hook_response event at all — a
	// wired session with nil/zero SessionStart is the instrument error
	// runOneSession gates on, exactly like an absent MCP server.
	//
	// "obligations_injected" (O45 item 8) is a SYNTHETIC key sharing this
	// same map rather than a real hook_event name: it counts, among the
	// "PostToolUse" hook_response events already counted above, how many
	// carried the obligations marker ("Obligations — governing rules" /
	// "⚖️ fathom obligations —") in their own rendered text
	// (hookResponseAdditionalContext) — a PostToolUse hook can fire for
	// reasons other than obligations injection, so this is always <= the
	// "PostToolUse" count, never a separate event class of its own.
	HooksFired map[string]int `json:"hooks_fired,omitempty"`
	// BudgetHit is true when the turn cap ended the session (Claude Code's
	// error_max_turns): the designed hand-off, never counted as an error.
	BudgetHit bool `json:"budget_hit,omitempty"`
	// FinalAnswer is the terminal `result` stream event's own text (SB-32's
	// answer-only oracle) — set whenever the harness's transcript carries
	// one, never inferred from FilesRead/FilesEdited. When the session
	// ended on its turn budget (BudgetHit) and the result event carried no
	// text of its own, this falls back to the last assistant text block in
	// the transcript (O45 item 3) — AnswerFrom names which.
	FinalAnswer string `json:"final_answer,omitempty"`
	// AnswerFrom names where FinalAnswer came from: "result" (the terminal
	// result event's own text) or "last_text" (the error_max_turns
	// fallback) — "" only when FinalAnswer is itself empty. Kept so a
	// stored cell is honest about having used the fallback, never silently
	// indistinguishable from a session that answered before its budget hit.
	AnswerFrom string `json:"answer_from,omitempty"`
	// SessionStartContext is the FIRST "system"/"hook_response" event's own
	// rendered text for hook_event "SessionStart" (hookResponseAdditionalContext,
	// stream.go) — fathom's session primer, captured verbatim so
	// primerInstrumentError (sessions.go, O45 item 7) can tell a wired
	// session whose primer honestly named the indexed service from one
	// that silently talked to the wrong (or no) daemon. "" when no
	// SessionStart hook_response was seen at all, or its own text could
	// not be extracted.
	SessionStartContext string `json:"session_start_context,omitempty"`
	// Error is the harness's or Claude's own reported session failure, or
	// (runOneSession) the instrument-truth gate's message — kept for the
	// existing "stop after an errored session" behavior. InstrumentError
	// carries the SAME message only for an instrument-truth failure, so
	// CellLost can flag a cell on any ONE such session, not only when
	// every session in it errored.
	Error           string `json:"error,omitempty"`
	InstrumentError string `json:"instrument_error,omitempty"`
	// AnswerKeyLeak (O53 defect 3) is set when a post-session scan
	// (AnswerKeyLeakScan) found a tool_result in this session's raw
	// transcript naming a path Subject.ExcludePaths was supposed to have
	// removed from the worktree before any session ran — proof the answer
	// key the task was derived from reached the agent anyway. CellLost
	// treats any ONE such session as voiding the whole cell, exactly like
	// InstrumentError.
	AnswerKeyLeak bool `json:"answer_key_leak,omitempty"`
	// BaselineRanFathom (O53 defect 4) is set when a post-session scan
	// (BaselineRanFathomScan) found a Bash tool_use in this session's raw
	// transcript invoking the fathom binary despite this arm's PATH
	// exclusion — proof the "no fathom on PATH" isolation a non-wired arm
	// depends on did not hold. CellLost treats any ONE such session as
	// voiding the whole cell, exactly like InstrumentError.
	BaselineRanFathom bool `json:"baseline_ran_fathom,omitempty"`
}

// DetectorResult is one detector's verdict.
type DetectorResult struct {
	Name     string `json:"name"`
	Exit     int    `json:"exit"`
	Violated bool   `json:"violated"`
}

// OracleResult is what the tests and detectors said after the last session.
type OracleResult struct {
	Passed        bool             `json:"passed"`
	FailToPassOK  bool             `json:"fail_to_pass_ok"`
	PassToPassOK  bool             `json:"pass_to_pass_ok"`
	Detectors     []DetectorResult `json:"detectors,omitempty"`
	CoverProfile  string           `json:"cover_profile,omitempty"`
	TestOutputRef string           `json:"test_output_ref,omitempty"`
	// Calibration is set only for a task carrying a Gold.TestPatch — what
	// CalibrateSubject found (oracle.go): how many fail_to_pass tests
	// actually failed at base with the test patch applied (the rest are
	// not this task's to prove), and how many pass_to_pass tests were
	// even eligible to be broken (the rest already failed at base —
	// environment, not the agent).
	Calibration *CalibrationSummary `json:"calibration,omitempty"`
	// Localization is set only for a task carrying Oracle.Localization —
	// ScoreLocalization's own verdict (oracle.go), computed from the LAST
	// session's FinalAnswer against Gold.Files.
	Localization *LocalizationReading `json:"localization,omitempty"`
}

// LocalizationReading is VB-25 (vision 29 Wave G): how a bug-localization
// answer's own parsed file set (ParseAnswerFiles) scored against
// Gold.Files — computed once, from the FinalAnswer text, never at record
// time (no tree exists to confirm anything, the same reason FalseDoneRate
// defers its own backtick fallback to scoring). HitAt1/HitAt5 are ranked
// on ParseAnswerFiles' own order (first-mentioned first); Recall/Precision
// are set-based and order-independent.
type LocalizationReading struct {
	HitAt1        bool    `json:"hit_at_1"`
	HitAt5        bool    `json:"hit_at_5"`
	Recall        float64 `json:"recall"`
	Precision     float64 `json:"precision"`
	GoldFiles     int     `json:"gold_files"`
	AnsweredFiles int     `json:"answered_files"`
	NotMeasurable string  `json:"not_measurable,omitempty"`
}

// CalibrationSummary is the counted (never per-test) shape of one task's
// test-patch calibration, small enough to live on every stored CellResult.
// The raw per-test sets live in the calibration cache (oracle.go's
// Calibration type), keyed by (subject repo, base_commit, test-patch sha).
type CalibrationSummary struct {
	FailToPassFailedAtBase int    `json:"fail_to_pass_failed_at_base"`
	FailToPassTotal        int    `json:"fail_to_pass_total"`
	PassToPassEffective    int    `json:"pass_to_pass_effective"`
	PassToPassTotal        int    `json:"pass_to_pass_total"`
	NotMeasurable          string `json:"not_measurable,omitempty"`
}

// PlanCoverageReading is M3.
type PlanCoverageReading struct {
	GoldFunctions int      `json:"gold_functions"`
	Named         int      `json:"named"`
	Fraction      float64  `json:"fraction"`
	Spurious      []string `json:"spurious,omitempty"`
	PlanPath      string   `json:"plan_path,omitempty"`
	NotMeasurable string   `json:"not_measurable,omitempty"`
}

// FalseDoneReading is M2: claims against the independent oracle. Claimed
// and Refuted count only ADJUDICABLE rows — those FalseDoneRate could
// attach a symbol to, directly or by backtick fallback (SB-21 follow-up,
// public-pilot-2/reuse-pilot-1, 2026-09-13): a row with no symbol contract
// and no resolvable backticked identifier is not a false-done candidate at
// all, and must not inflate Refuted the way it did before this reading
// distinguished the two.
type FalseDoneReading struct {
	Claimed        int      `json:"claimed"`
	Refuted        int      `json:"refuted"`
	RefutedRows    []string `json:"refuted_rows,omitempty"`
	NotAdjudicable int      `json:"not_adjudicable,omitempty"`
	// NotAdjudicableRows names each row FalseDoneRate could not attach any
	// symbol to (checkLabel, then why), for the same debugging purpose
	// RefutedRows serves for refutation.
	NotAdjudicableRows []string `json:"not_adjudicable_rows,omitempty"`
	// MatchedByBacktick names each row whose symbol came from a backticked
	// identifier in Text rather than a v2-grammar contract — never silent,
	// since it is a weaker match than an authored {symbol: ...}.
	MatchedByBacktick []string `json:"matched_by_backtick,omitempty"`
	NotMeasurable     string   `json:"not_measurable,omitempty"`
}

// ResumptionReading is M4.
type ResumptionReading struct {
	Sessions            int   `json:"sessions"`
	TokensToFirstEditS2 int64 `json:"tokens_to_first_edit_from_session_2"`
	// TokensToFirstEditAll is RC-13 (vision 28 §2.4): the same field summed
	// over EVERY session, session 1 included — unlike TokensToFirstEditS2
	// (deliberately session>=2 only, M4's cold-hand-off cost), a consumer
	// like FactsReused needs the whole population. Always computed, with no
	// NotMeasurable gate: even a single session has one number.
	TokensToFirstEditAll int64  `json:"tokens_to_first_edit_all_sessions"`
	DuplicatedEdits      int    `json:"duplicated_edits"`
	NotMeasurable        string `json:"not_measurable,omitempty"`
}

// OverlapReading is M5.
type OverlapReading struct {
	Workers        int     `json:"workers"`
	ReexploreBytes int64   `json:"reexplore_bytes"`
	WorkerBytes    int64   `json:"worker_bytes"`
	Ratio          float64 `json:"ratio"`
	NotMeasurable  string  `json:"not_measurable,omitempty"`
}

// DriftReading is contract drift after the change.
type DriftReading struct {
	Rows          int      `json:"rows"`
	Drifted       []string `json:"drifted,omitempty"`
	NotMeasurable string   `json:"not_measurable,omitempty"`
}

// Readings are computed offline from sessions + oracle; never gated unless
// the corpus's criterion names them.
type Readings struct {
	TotalTokens  int64   `json:"total_tokens"`
	TotalTurns   int     `json:"total_turns"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	PayloadBytes int64   `json:"payload_bytes"`
	// ToolCalls sums every session's own ToolCalls (SessionResult), name to
	// count.
	ToolCalls map[string]int `json:"tool_calls,omitempty"`
	// HooksFired sums every session's own HooksFired (SessionResult), hook
	// event name to count — the same aggregation ToolCalls gets
	// (AggregateReadings).
	HooksFired map[string]int `json:"hooks_fired,omitempty"`
	// SurfaceCalls counts, within ToolCalls, every call to a tool
	// namespaced to the arm's own Surface (SurfaceCallCount) — 0 for a
	// pure control (empty Surface) by definition, never "not measured".
	SurfaceCalls int                  `json:"surface_calls,omitempty"`
	PlanCoverage *PlanCoverageReading `json:"plan_coverage,omitempty"`
	FalseDone    *FalseDoneReading    `json:"false_done,omitempty"`
	Resumption   *ResumptionReading   `json:"resumption,omitempty"`
	Overlap      *OverlapReading      `json:"overlap,omitempty"`
	Drift        *DriftReading        `json:"drift,omitempty"`
	RuleServed   *bool                `json:"rule_served,omitempty"` // guards: obligation carried the rule before the first claim
	// CompileCost is a wired, CompileLayers cell's own compile spend
	// (readingsCompileCost, from Worktree.CompileCost) — nil for an
	// unwired or non-CompileLayers arm: "compiled nothing" is a fact,
	// never a zero (RC-10, vision 28 §2 F6).
	CompileCost *CompileCostReading `json:"compile_cost,omitempty"`
	// CompileCostUSD is CompileCost.CostUSD, carried as its own field (and
	// summed across a ledger row's cells) so a reader sees the compile
	// spend's own share — it is ALSO folded into TotalCostUSD (ApplyCompileCost),
	// since a wired cell's compile step is real spend the row's total must
	// include, not a number that only shows up in a side column.
	CompileCostUSD float64 `json:"compile_cost_usd,omitempty"`
	// RuleServedBeforeEdit is RC-12 (vision 28 §2.4, F12): RuleServedBeforeEdit's
	// own direct transcript reading — did a served-rule text (the obligations
	// hook's live render, or the SessionStart stanza) reach the transcript
	// before the first Edit/Write of a governed file? Complements RuleServed
	// (the beat-log reading) rather than replacing it.
	RuleServedBeforeEdit *bool `json:"rule_served_before_edit,omitempty"`
	// Violation is RC-12: whether any of OracleResult.Detectors reported
	// Violated — the rule detectors' own outcome, already computed by the
	// oracle, carried onto the row so M1 is a rate over stored readings and
	// not prose re-derived from the detector list each time.
	Violation *bool `json:"violation,omitempty"`
	// FactsReused is RC-13 (vision 28 §2.4, F11): among a hand-off brief's
	// pointers, how many did this session's own transcript reference (tool
	// input or a read result) before its first edit — see FactsReused.
	FactsReused *int `json:"facts_reused,omitempty"`
	// GoldSourceTool is VB-4 (vision 29 §1 R2, ADR-035 decision 5): the
	// tool whose result FIRST held this task's own gold signal — never
	// merely whether the cell passed. GoldSourceNone ("none") when no
	// tool result ever carried it (the agent knew or guessed); empty only
	// for a cell this reading was never computed for (a cell scored
	// before VB-4). See GoldSourceTool (goldsource.go).
	GoldSourceTool string `json:"gold_source_tool,omitempty"`
	// GoldSourceEvidence is the matched token, file, or rule path that
	// decided GoldSourceTool.
	GoldSourceEvidence string `json:"gold_source_evidence,omitempty"`
	// Localization is VB-25: OracleResult.Localization carried onto the
	// cell's own Readings verbatim, so a stored CellResult and a ledger
	// row's PerTrial both carry it without a separate lookup into Oracle.
	Localization *LocalizationReading `json:"localization,omitempty"`

	// --- row-only aggregates (AggregateRowReadings, cell.go) ---
	// A single cell/trial never sets these — they summarize what cannot
	// simply sum across a ledger row's own trials: a bool per trial, or a
	// ratio per trial. Nil/zero means no trial in the row measured it, the
	// same convention every other not-measured field here uses.

	// ViolationTrials counts, among a row's own trials, how many reported
	// Violation == true — nil when no trial ever measured Violation (no
	// Rule on the task).
	ViolationTrials *int `json:"violation_trials,omitempty"`
	// RuleServedBeforeEditTrials counts, among a row's own trials, how many
	// reported RuleServedBeforeEdit == true — nil when no trial ever
	// measured it.
	RuleServedBeforeEditTrials *int `json:"rule_served_before_edit_trials,omitempty"`
	// OverlapRatioMean/OverlapRatioN average Overlap.Ratio across the
	// row's own trials that measured it (ModePlannerWorkers tasks only);
	// N is the trial count the mean was taken over, so a mean of 1 reads
	// differently from a mean of 5. N == 0 means no trial measured it.
	OverlapRatioMean float64 `json:"overlap_ratio_mean,omitempty"`
	OverlapRatioN    int     `json:"overlap_ratio_n,omitempty"`
	// ResumptionTokensToFirstEditMean/N average Resumption's own
	// TokensToFirstEditAll across the row's own trials that measured it.
	ResumptionTokensToFirstEditMean float64 `json:"resumption_tokens_to_first_edit_mean,omitempty"`
	ResumptionTokensToFirstEditN    int     `json:"resumption_tokens_to_first_edit_n,omitempty"`
	// LocalizationHitAt1Rate/LocalizationHitAt5Rate/LocalizationFileRecallMean
	// average Localization's own HitAt1/HitAt5/Recall across the row's own
	// trials that measured it (Localization != nil, NotMeasurable empty);
	// LocalizationN is the trial count the means were taken over — 0 means
	// no trial in the row measured it, the same not-measured convention
	// every other row-only aggregate here uses. These are VB-25's own
	// headline numbers (docs/bench/scenarios.md "External corpus (LCA bug
	// localization)"): localization_hit_at_1, localization_hit_at_5, and
	// the mean file recall.
	LocalizationHitAt1Rate     float64 `json:"localization_hit_at_1,omitempty"`
	LocalizationHitAt5Rate     float64 `json:"localization_hit_at_5,omitempty"`
	LocalizationFileRecallMean float64 `json:"file_recall,omitempty"`
	LocalizationN              int     `json:"localization_n,omitempty"`
}

// CompileCostReading is the wave-0 receipt (RC-10) of what a wired,
// CompileLayers cell's obligations-layer compile actually cost — the same
// shape as subject.go's own CompileCost (PrepareSubject's runtime record),
// named distinctly because that type already exists for a different
// purpose (Worktree.CompileCost) and this one is the JSON-persisted
// reading on CellResult/ScenarioLedgerRow.
type CompileCostReading struct {
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
	CostUSD   float64 `json:"cost_usd"`
	Cached    bool    `json:"cached,omitempty"`
}

// CellResult is one (task, arm, trial), the unit of the cell store.
type CellResult struct {
	TaskID         string          `json:"task_id"`
	Arm            string          `json:"arm"`
	Trial          int             `json:"trial"`
	Category       Category        `json:"category"`
	Sessions       []SessionResult `json:"sessions"`
	Oracle         OracleResult    `json:"oracle"`
	Readings       Readings        `json:"readings"`
	Success        bool            `json:"success"`
	CellKey        string          `json:"cell_key"`
	CorpusSHA256   string          `json:"corpus_sha256"`
	ModelID        string          `json:"model_id"`
	HarnessVersion string          `json:"harness_version"`
	ArmSurface     string          `json:"arm_surface"`
	// Surface is the arm's own configured Surface (Arm.Surface) verbatim —
	// "fathom", a competitor's "name@version", or "" for a pure control.
	// Kept distinct from ArmSurface (the RESOLVED key material: a wired
	// arm's fathom commit, never the literal "fathom" sentinel), because
	// that resolution is exactly what SurfaceCallCount must NOT use — an
	// MCP server is registered under Surface's own literal name, so a
	// later `score` recompute needs this, not ArmSurface, to know which
	// tool prefix to count.
	Surface      string `json:"surface,omitempty"`
	Borrowed     bool   `json:"borrowed,omitempty"`
	SourceRunID  string `json:"source_run_id,omitempty"`
	RunID        string `json:"run_id"`
	StartedAt    string `json:"started_at"`
	WorktreePath string `json:"worktree_path,omitempty"`
	// Gold is the task's own gold record, persisted so `scenariobench
	// score` can recompute PlanCoverage/ContractDrift later without
	// reloading the corpus.
	Gold Gold `json:"gold,omitzero"`
	// PlanPath / AgentDiffPath are the SDD readings' own captured
	// artifacts under this cell's CellDir: the agent's own plan file (""
	// when none was found) and its whole-session diff, captured right
	// after the sessions and before the oracle. Both live under CellDir,
	// which — unlike the shared subject worktree a later cell on the same
	// (subject, base_commit, arm) resets and reuses — is this cell's own
	// and is never touched again.
	PlanPath      string `json:"plan_path,omitempty"`
	AgentDiffPath string `json:"agent_diff_path,omitempty"`
	// SubjectClone / BaseCommit are PrepareSubject's own shared clone
	// (Worktree.SubjectClone) and the task's pinned commit, persisted so
	// `scenariobench score` can rebuild the final tree in a detached temp
	// worktree (base_commit + AgentDiffPath applied) when WorktreePath's
	// SHARED, REUSED directory has already been reset by a later cell.
	SubjectClone string `json:"subject_clone,omitempty"`
	BaseCommit   string `json:"base_commit,omitempty"`
	// InstrumentDefect mirrors ScenarioLedgerRow's own field (ADR-034
	// decision 2, RC-36): O53 defect 7's `scenariobench void` command sets
	// it on every stored cell record filed under the voided
	// (run_id, task_id, arm), so the cell store itself — not only the
	// ledger row — carries the fact that this reading measured an
	// instrument defect, not the thing it claims. Empty for a sound cell.
	InstrumentDefect string `json:"instrument_defect,omitempty"`
	// RowsAssigned / RowsUnassigned are ModePlannerWorkers' own accounting
	// (O55 item 3): how many plan rows actually got a worker session
	// (RunSessions' reassignment loop, sessions.go) versus how many were
	// still unchecked when it stopped (task.Sessions.MaxWorkersTotal
	// reached, or a worker session errored). nil for ModeSingle — there is
	// no plan-row assignment to report.
	RowsAssigned   *int `json:"rows_assigned,omitempty"`
	RowsUnassigned *int `json:"rows_unassigned,omitempty"`
	// InstrumentFilesExcluded is how many whole-file sections
	// FilterInstrumentOwnedDiff dropped from agent.diff (O55 item 4) —
	// `fathom init`'s own writes, never the agent's — so the exclusion is
	// visible on the cell rather than a silent shrink of AgentDiffPath. 0
	// (the common unwired-arm case) is a real, reported count, not absence
	// — always present, never omitempty.
	InstrumentFilesExcluded int `json:"instrument_files_excluded"`
	// StoreRoot is set only by BorrowScenarioCell (O55 item 5), to the
	// cellsDir the cell was actually read from — ephemeral like Borrowed/
	// SourceRunID (StoreScenarioCell never writes a borrowed cell back), so
	// this is never present on a cell as originally stored.
	StoreRoot string `json:"store_root,omitempty"`
}

// Harness runs one session and returns what happened; the Claude Code
// harness shells out to `claude -p`, the fake harness replays committed
// stream transcripts so the driver is testable without spend.
type Harness interface {
	Name() string
	Version() string
	RunSession(req SessionRequest) (SessionResult, error)
}

// SessionRequest is everything a harness needs for one session.
type SessionRequest struct {
	Worktree     string
	Prompt       string
	SystemAppend string
	Arm          Arm
	MaxTurns     int
	Model        string
	RawDir       string
	Index        int
	Role         string
	RowID        string
	Env          map[string]string
	// MCPConfigPath is the MCP config RenderArm actually wrote for this
	// worktree (wt.Arm.MCPConfigPath) — what the harness must hand the
	// agent, since it (never Arm.MCP directly) carries RenderArm's own
	// normalization (alwaysLoad defaulted true). Empty only for a
	// worktree RenderArm never touched (a test constructing a
	// SessionRequest by hand); ClaudeHarness falls back to writing
	// Arm.MCP verbatim in that case.
	MCPConfigPath string
	// SettingsPath is the arm's own rendered `.claude/settings.json`
	// (wt.Arm.SettingsPath, RenderArm) — passed to the harness as
	// `--settings` under `--restricted` mode (RC-6), so a wired arm's real
	// hooks still fire while the operator's own user/project/local
	// settings never load. Empty only for a worktree RenderArm never
	// touched (a test constructing a SessionRequest by hand); buildClaudeArgs
	// simply omits `--settings` in that case.
	// MCPServerNames are the rendered arm's server names (wt.Arm's
	// ArmFiles.MCPServerNames): the harness allow-lists `mcp__<name>` for
	// each when the arm carries no allow-list of its own, so a wired arm's
	// tools never prompt under --permission-mode default.
	MCPServerNames []string
	SettingsPath   string
	// AllowedTools / DisallowedTools are the RENDERED lists (wt.Arm's
	// ArmFiles, not Arm's own) — ArmFiles.AllowedTools additionally
	// carries "mcp__<server>" for every MCP server this arm declares, so
	// a harness reading these two fields (never Arm.AllowedTools) always
	// sees an allow-list that covers the MCP surface it was also handed.
	AllowedTools    []string
	DisallowedTools []string
}

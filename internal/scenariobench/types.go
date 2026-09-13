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
	// untouched), "none" (the file removed for the run).
	Instructions string `json:"instructions"`
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
}

// Subject is the pinned repository a task runs against.
type Subject struct {
	RepoURL    string   `json:"repo_url"`
	BaseCommit string   `json:"base_commit"`
	Service    string   `json:"service"`
	Language   string   `json:"language"`
	Setup      []string `json:"setup,omitempty"`
	// TestCommand runs the oracle's test selection; {{tests}} is replaced
	// by the regex of test names. Default: go test -run '{{tests}}' ./...
	TestCommand string `json:"test_command,omitempty"`
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
}

// Gold is what the accepted change touched, from the pinned patch.
type Gold struct {
	Files     []string `json:"files,omitempty"`
	Functions []string `json:"functions,omitempty"`
	PatchSHA  string   `json:"patch_sha256,omitempty"`
}

// Rule is the documented rule a guards task is built around (M1).
type Rule struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Source string `json:"source"`
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
)

// Task is one scenario.
type Task struct {
	ID          string            `json:"id"`
	Category    Category          `json:"category"`
	Prompt      string            `json:"prompt"`
	Subject     Subject           `json:"subject"`
	Sessions    Sessions          `json:"sessions"`
	Oracle      Oracle            `json:"oracle"`
	Gold        Gold              `json:"gold,omitzero"`
	Rule        *Rule             `json:"rule,omitempty"`
	Predictions map[string]string `json:"predictions"`
	Derivation  string            `json:"derivation"`
	Requires    []string          `json:"requires,omitempty"`
	MergedAt    string            `json:"merged_at,omitempty"`
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
	PayloadBytes      int64          `json:"payload_bytes"`
	WallTime          time.Duration  `json:"wall_time_ns"`
	RawPath           string         `json:"raw_path,omitempty"`
	Error             string         `json:"error,omitempty"`
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

// FalseDoneReading is M2: claims against the independent oracle.
type FalseDoneReading struct {
	Claimed       int      `json:"claimed"`
	Refuted       int      `json:"refuted"`
	RefutedRows   []string `json:"refuted_rows,omitempty"`
	NotMeasurable string   `json:"not_measurable,omitempty"`
}

// ResumptionReading is M4.
type ResumptionReading struct {
	Sessions            int    `json:"sessions"`
	TokensToFirstEditS2 int64  `json:"tokens_to_first_edit_from_session_2"`
	DuplicatedEdits     int    `json:"duplicated_edits"`
	NotMeasurable       string `json:"not_measurable,omitempty"`
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
	TotalTokens  int64                `json:"total_tokens"`
	TotalTurns   int                  `json:"total_turns"`
	TotalCostUSD float64              `json:"total_cost_usd"`
	PayloadBytes int64                `json:"payload_bytes"`
	PlanCoverage *PlanCoverageReading `json:"plan_coverage,omitempty"`
	FalseDone    *FalseDoneReading    `json:"false_done,omitempty"`
	Resumption   *ResumptionReading   `json:"resumption,omitempty"`
	Overlap      *OverlapReading      `json:"overlap,omitempty"`
	Drift        *DriftReading        `json:"drift,omitempty"`
	RuleServed   *bool                `json:"rule_served,omitempty"` // guards: obligation carried the rule before the first claim
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
	Borrowed       bool            `json:"borrowed,omitempty"`
	SourceRunID    string          `json:"source_run_id,omitempty"`
	RunID          string          `json:"run_id"`
	StartedAt      string          `json:"started_at"`
	WorktreePath   string          `json:"worktree_path,omitempty"`
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
}

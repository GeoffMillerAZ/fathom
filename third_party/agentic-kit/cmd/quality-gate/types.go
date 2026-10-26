// Command quality-gate evaluates versioned repository boundary policies against
// immutable Git objects. Model judgments may add review holds; they never erase
// structural failures or authorize effects. It does not execute repository code.
//
// Concurrency: each invocation owns its state. Independent invocations are safe;
// advisory cache writes use atomic replacement and never authorize a required gate.
package main

import (
	"context"
	"time"
)

type selector struct {
	Prefixes []string `json:"prefixes"`
	Suffixes []string `json:"suffixes,omitempty"`
}

type rule struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Boundaries []string  `json:"boundaries"`
	Select     selector  `json:"select"`
	Companion  *selector `json:"companion,omitempty"`
	MaxBytes   int       `json:"max_bytes,omitempty"`
}

type limits struct {
	MaxFiles       int `json:"max_files"`
	MaxFileBytes   int `json:"max_file_bytes"`
	MaxTotalBytes  int `json:"max_total_bytes"`
	TimeoutSeconds int `json:"timeout_seconds"`
}

type labeledCase struct {
	ID         string  `json:"id"`
	Expected   string  `json:"expected"`
	Actual     string  `json:"actual"`
	Confidence float64 `json:"confidence"`
	Provenance string  `json:"provenance"`
}

type calibration struct {
	Site           string        `json:"site"`
	QuestionSHA256 string        `json:"question_sha256"`
	Provider       string        `json:"provider"`
	ResolvedModel  string        `json:"resolved_model"`
	ValidUntil     time.Time     `json:"valid_until"`
	MinPerClass    int           `json:"min_per_class"`
	MinN           int           `json:"min_n"`
	MinAccuracy    float64       `json:"min_accuracy"`
	MinWilsonLo    float64       `json:"min_wilson_lo"`
	Cases          []labeledCase `json:"cases"`
}

type modelSite struct {
	ID            string       `json:"id"`
	Version       string       `json:"version"`
	Question      string       `json:"question"`
	Boundaries    []string     `json:"boundaries"`
	Select        selector     `json:"select"`
	Mode          string       `json:"mode"` // shadow or hold
	MinConfidence float64      `json:"min_confidence"`
	Calibration   *calibration `json:"calibration,omitempty"`
}

type modelPolicy struct {
	Provider         string      `json:"provider"` // none, openai, typesafe, openrouter
	ModelEnv         string      `json:"model_env,omitempty"`
	ExpectedModelEnv string      `json:"expected_model_env,omitempty"`
	APIKeyEnv        string      `json:"api_key_env,omitempty"`
	MaxInputBytes    int         `json:"max_input_bytes"`
	TimeoutSeconds   int         `json:"timeout_seconds"`
	CacheTTLSeconds  int         `json:"cache_ttl_seconds"`
	MaxBatches       int         `json:"max_batches,omitempty"` // omitted means one per invocation
	Sites            []modelSite `json:"sites"`
}

type policy struct {
	Version int         `json:"version"`
	ID      string      `json:"id"`
	Limits  limits      `json:"limits"`
	Rules   []rule      `json:"rules"`
	Model   modelPolicy `json:"model"`
}

type change struct {
	Path       string `json:"path"`
	BeforeOID  string `json:"before_oid,omitempty"`
	AfterOID   string `json:"after_oid,omitempty"`
	BeforeMode string `json:"before_mode,omitempty"`
	AfterMode  string `json:"after_mode,omitempty"`
	Before     []byte `json:"-"`
	After      []byte `json:"-"`
}

type snapshot struct {
	BaseTree string   `json:"base_tree"`
	Tree     string   `json:"tree"`
	Revision string   `json:"revision,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Changes  []change `json:"changes"`
}

type finding struct {
	RuleID string `json:"rule_id"`
	Kind   string `json:"kind"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Reason string `json:"reason"`
}

type ruleResult struct {
	ID      string `json:"id"`
	State   string `json:"state"` // pass, fail, not-applicable, unknown
	Matched int    `json:"matched"`
	Reason  string `json:"reason,omitempty"`
}

type boundaryCoverage struct {
	RequiredRules   []string `json:"required_rules"`
	RequiredSites   []string `json:"required_sites"`
	ShadowSites     []string `json:"shadow_sites"`
	UnselectedPaths []string `json:"unselected_paths"`
}

type siteResult struct {
	ID          string   `json:"id"`
	Mode        string   `json:"mode"`
	Outcome     string   `json:"outcome"` // clear, concern, insufficient, unknown, not-applicable, pending (internal preflight)
	Confidence  *float64 `json:"confidence,omitempty"`
	Calibration string   `json:"calibration"`
	Reason      string   `json:"reason"`
	Held        bool     `json:"held"`
}

type modelResult struct {
	Provider        string       `json:"provider"`
	Model           string       `json:"model,omitempty"`
	RequestSHA256   string       `json:"request_sha256,omitempty"`
	Cache           string       `json:"cache"`
	InputTokens     int          `json:"input_tokens,omitempty"`
	OutputTokens    int          `json:"output_tokens,omitempty"`
	RemoteAttempted bool         `json:"remote_attempted"`
	BatchOrdinal    int          `json:"batch_ordinal,omitempty"`
	BatchLimit      int          `json:"batch_limit,omitempty"`
	Reason          string       `json:"reason,omitempty"`
	Sites           []siteResult `json:"sites"`
}

type report struct {
	Version      int              `json:"version"`
	PolicyID     string           `json:"policy_id"`
	PolicyCommit string           `json:"policy_commit,omitempty"`
	PolicySHA256 string           `json:"policy_sha256"`
	Boundary     string           `json:"boundary"`
	Observe      bool             `json:"observe"`
	Snapshot     snapshot         `json:"snapshot"`
	Rules        []ruleResult     `json:"rules"`
	Findings     []finding        `json:"findings"`
	Model        modelResult      `json:"model"`
	Coverage     boundaryCoverage `json:"coverage"`
	State        string           `json:"state"` // pass, hold, unknown, observed
	Reason       string           `json:"reason,omitempty"`
}

// gitSource reads only Git plumbing. IndexFile is an explicit alternate-index
// selection; other inherited GIT_* variables must not redirect the repository.
type gitSource struct{ Dir, IndexFile, GitBin string }

type pushUpdate struct{ LocalRef, LocalOID, RemoteRef, RemoteOID string }

type modelOptions struct {
	AllowRemote bool
	CacheDir    string           // advisory-only, never read for any hold-mode site
	Now         func() time.Time // nil defaults to time.Now in the evaluator
	Budget      *modelBudget     // invocation-owned; nil creates the policy default
	PrepareOnly bool             // input validation only; no credentials, cache or provider dispatch
}

// modelBudget is invocation-local and used serially by the CLI. A cache hit
// consumes no dispatch. It bounds requests, not provider billing or tokens.
type modelBudget struct{ limit, used int }

func newModelBudget(p modelPolicy) *modelBudget {
	n := p.MaxBatches
	if n == 0 {
		n = 1
	}
	return &modelBudget{limit: n}
}

func (b *modelBudget) reserve() (int, bool) {
	if b == nil || b.limit < 1 || b.used >= b.limit {
		return 0, false
	}
	b.used++
	return b.used, true
}

type modelEvaluator func(context.Context, policy, string, string, snapshot, modelOptions) modelResult

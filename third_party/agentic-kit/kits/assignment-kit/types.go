// Package assignmentkit evaluates bounded, caller-declared assignments. Functions
// are safe for concurrent use when callers hold their inputs immutable; outputs
// are detached. The package performs no effects and grants no action authority.
package assignmentkit

import (
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	"time"
)

// Assignment is one immutable versioned acceptance contract.
type Assignment struct {
	Version       int         `json:"version"`
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	IntentRefs    []Reference `json:"intent_refs"`
	PolicyDigest  string      `json:"policy_digest"`
	SubjectDigest string      `json:"subject_digest"`
	Goals         []Goal      `json:"goals"`
	Concerns      []Concern   `json:"concerns"`
	Budgets       []Budget    `json:"budgets"`
	Deadline      time.Time   `json:"deadline"`
	Rules         []Rule      `json:"rules"`
}

// Reference binds an intent or parent artifact label to immutable content.
type Reference struct {
	ID             string `json:"id"`
	RevisionDigest string `json:"revision_digest"`
}

// Goal is a required or stretch outcome with dependent goals.
type Goal struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"`
	DependsOn  []string        `json:"depends_on"`
	Profile    fitness.Profile `json:"profile"`
	Confidence []ConfidenceBar `json:"confidence"`
}

// ConfidenceBar constrains heuristic evidence for one criterion.
type ConfidenceBar struct {
	CriterionID string  `json:"criterion_id"`
	Minimum     float64 `json:"minimum"`
	BasisDigest string  `json:"basis_digest"`
	SiteDigest  string  `json:"site_digest"`
}

// Scope is independently authored expected required coverage and pins.
type Scope struct {
	AssignmentDigest string   `json:"assignment_digest"`
	PolicyDigest     string   `json:"policy_digest"`
	RequiredGoalIDs  []string `json:"required_goal_ids"`
}

// GoalInput supplies observed evidence and optional calibrated readings.
type GoalInput struct {
	GoalID       string                `json:"goal_id"`
	Observations []fitness.Observation `json:"observations"`
	Readings     []ConfidenceReading   `json:"readings"`
}

// ConfidenceReading binds one observation and a caller-qualified calibration basis.
type ConfidenceReading struct {
	CriterionID       string  `json:"criterion_id"`
	ObservationDigest string  `json:"observation_digest"`
	BasisDigest       string  `json:"basis_digest"`
	SiteDigest        string  `json:"site_digest"`
	Value             float64 `json:"value"`
}

// Concern is a blocking or advisory acceptance issue.
type Concern struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// ConcernInput is a caller-declared issue resolution.
type ConcernInput struct {
	ID              string    `json:"id"`
	State           string    `json:"state"`
	Reason          string    `json:"reason"`
	ArtifactDigests []string  `json:"artifact_digests"`
	ObservedAt      time.Time `json:"observed_at"`
	ValidUntil      time.Time `json:"valid_until"`
}

// Budget is an exact, application-defined unit limit.
type Budget struct {
	ID    string `json:"id"`
	Unit  string `json:"unit"`
	Limit int64  `json:"limit"`
}

// Usage reports all known spend for a budget.
type Usage struct {
	BudgetID string `json:"budget_id"`
	Spent    int64  `json:"spent"`
}

// Progress contains caller-counted signals; the kit does not infer them.
type Progress struct {
	Attempts    int64 `json:"attempts"`
	NoProgress  int64 `json:"no_progress"`
	Ambiguities int64 `json:"ambiguities"`
	Conflicts   int64 `json:"conflicts"`
}

// Rule is a deterministic threshold for one signal.
type Rule struct {
	ID        string `json:"id"`
	Counter   string `json:"counter"`
	Threshold int64  `json:"threshold"`
	Action    string `json:"action"`
	Route     string `json:"route"`
	Reason    string `json:"reason"`
}

// Input is one snapshot of evidence, resource use, and progress.
type Input struct {
	AssignmentDigest string         `json:"assignment_digest"`
	PolicyDigest     string         `json:"policy_digest"`
	Goals            []GoalInput    `json:"goals"`
	Concerns         []ConcernInput `json:"concerns"`
	Usage            []Usage        `json:"usage"`
	Progress         *Progress      `json:"progress"`
}

// ConfidenceResult is the assignment-specific acceptance overlay for a heuristic bar.
// The underlying profile assessment remains an unchanged evidence projection.
type ConfidenceResult struct {
	CriterionID       string `json:"criterion_id"`
	Status            string `json:"status"`
	Reason            string `json:"reason"`
	ObservationDigest string `json:"observation_digest"`
	BasisDigest       string `json:"basis_digest"`
	SiteDigest        string `json:"site_digest"`
}

// GoalResult retains the profile trace and dependency status.
type GoalResult struct {
	ID                  string             `json:"id"`
	Kind                string             `json:"kind"`
	Satisfied           bool               `json:"satisfied"`
	BlockedDependencies []string           `json:"blocked_dependencies"`
	Assessment          fitness.Assessment `json:"assessment"`
	Confidence          []ConfidenceResult `json:"confidence"`
	AcceptedSoftScore   *float64           `json:"accepted_soft_score"`
}

// ConcernResult retains a projected state and reason.
type ConcernResult struct {
	ID              string   `json:"id"`
	Severity        string   `json:"severity"`
	State           string   `json:"state"`
	Reason          string   `json:"reason"`
	ArtifactDigests []string `json:"artifact_digests"`
}

// BudgetResult exposes known, missing, and exhausted usage.
type BudgetResult struct {
	ID        string `json:"id"`
	Unit      string `json:"unit"`
	Limit     int64  `json:"limit"`
	Spent     *int64 `json:"spent"`
	Exhausted bool   `json:"exhausted"`
}

// Cause is one retained reason for the selected disposition.
type Cause struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Detail string `json:"detail"`
}

// Result is a detached caller-declared snapshot, never authorization.
type Result struct {
	Trust            string          `json:"trust"`
	ActionAuthority  bool            `json:"action_authority"`
	AssignmentDigest string          `json:"assignment_digest"`
	PolicyDigest     string          `json:"policy_digest"`
	ScopeDigest      string          `json:"scope_digest"`
	EvidenceDigest   string          `json:"evidence_digest"`
	EvaluatedAt      time.Time       `json:"evaluated_at"`
	Status           string          `json:"status"`
	Goals            []GoalResult    `json:"goals"`
	Concerns         []ConcernResult `json:"concerns"`
	Budgets          []BudgetResult  `json:"budgets"`
	MatchedRules     []string        `json:"matched_rules"`
	Causes           []Cause         `json:"causes"`
}

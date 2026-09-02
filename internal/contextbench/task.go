// Package contextbench implements the falsifiability gate for fathom's ×10
// tokenomics claim (docs/plans/falsifiability.md B1, coding-agent-excellence.md
// §0 sentence 5 / §7 gate 6): "context-bench shows equal-or-better task success
// at ≤1/10th the discovery tokens vs grep/Read."
//
// # Committed corpus vs run artifacts
//
// The task corpus (prompt + ground truth + scoring rule) is DATA, not a run
// artifact, so it is committed at testdata/contextbench/tasks.json — anyone
// can re-derive every ground-truth answer by grepping the fathom/agentic-kit
// repos at the commit recorded in each task's "source" field (never by
// querying the live fathom daemon, so the corpus stays reproducible without a
// running index). Everything the *runner* (scripts/context-bench.sh) produces
// — raw claude -p transcripts, per-arm ArmResult JSON, the aggregated
// results-<ts>.json, and the rendered summary table — is a run artifact and
// lands under workspace/artifacts/context-bench/ per the repo's workspace/
// convention (CLAUDE.md), never committed.
//
// # Scoring rules
//
// Every task's ground_truth is a list of expected strings (symbol names,
// file paths, or ADR IDs) and a scoring_rule that says how many of them must
// appear, as a case-sensitive substring, in the arm's final answer text:
//
//   - exact-match: ground_truth has exactly one string; it must appear.
//   - all-of: every string in ground_truth must appear.
//   - any-of: at least one string in ground_truth must appear.
//
// "exact-match" scores by substring containment of the single expected
// answer, not by full-string equality — an LLM's answer is prose ("The
// caller is cmd/fathom/servehttp.go, line 105."), so requiring the output to
// equal the ground truth verbatim would fail every real transcript. The rule
// name signals cardinality (there is exactly one correct answer), not a
// stricter comparison than all-of/any-of use.
package contextbench

import (
	"encoding/json"
	"fmt"
	"os"
)

// ScoringRule names how many ground-truth strings must appear in an arm's
// answer for a task to score as a pass. See the package doc comment.
type ScoringRule string

const (
	ExactMatch ScoringRule = "exact-match"
	AllOf      ScoringRule = "all-of"
	AnyOf      ScoringRule = "any-of"
)

// TaskClass names the index-native capability a home-turf task exercises
// (docs/plans/home-bench.md H1-1). It is optional and absent throughout the
// original grep-native B1 corpus (testdata/contextbench/tasks.json, every
// task there is by construction grep-answerable); the home-turf corpus
// (testdata/contextbench/tasks-home.json) populates it on every task so the
// mix across fathom's governance/enforcement surfaces is machine-checkable,
// not just asserted in prose.
type TaskClass string

const (
	ClassObligations     TaskClass = "obligations"
	ClassVerify          TaskClass = "verify"
	ClassBlastRadius     TaskClass = "blast_radius"
	ClassClaimProvenance TaskClass = "claim_provenance"
	ClassConcernSearch   TaskClass = "concern_search"
	ClassContradiction   TaskClass = "contradiction"
	ClassStaleness       TaskClass = "staleness"

	// The public localization corpus (docs/plans/bench-public.md PB-14/15,
	// derived by internal/contextbench/publicloc from gold patches at a
	// pinned base commit, never authored): which files, which functions,
	// and which callers a developer-authored fix actually touched.
	ClassFileLocalization     TaskClass = "file_localization"
	ClassFunctionLocalization TaskClass = "function_localization"
	ClassFindCallers          TaskClass = "find_callers"
)

// Task is one context-bench prompt: a question with a falsifiable answer.
type Task struct {
	ID          string      `json:"id"`
	Category    string      `json:"category,omitempty"`
	Class       TaskClass   `json:"class,omitempty"`
	Prompt      string      `json:"prompt"`
	GroundTruth []string    `json:"ground_truth"`
	ScoringRule ScoringRule `json:"scoring_rule"`
	Source      string      `json:"source,omitempty"`
	Notes       string      `json:"notes,omitempty"`
}

// Validate reports whether t is well-formed: non-empty id/prompt/ground
// truth, a recognized scoring rule, and — for exact-match — exactly one
// ground-truth string (see the package doc comment for why exact-match is
// still a substring check, just over a single expected answer).
func (t Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task has no id")
	}
	if t.Prompt == "" {
		return fmt.Errorf("task %s: empty prompt", t.ID)
	}
	if len(t.GroundTruth) == 0 {
		return fmt.Errorf("task %s: empty ground_truth", t.ID)
	}
	for _, g := range t.GroundTruth {
		if g == "" {
			return fmt.Errorf("task %s: ground_truth contains an empty string", t.ID)
		}
	}
	switch t.ScoringRule {
	case ExactMatch:
		if len(t.GroundTruth) != 1 {
			return fmt.Errorf("task %s: scoring_rule exact-match requires exactly one ground_truth string, got %d", t.ID, len(t.GroundTruth))
		}
	case AllOf, AnyOf:
		// any non-empty ground_truth list is valid
	default:
		return fmt.Errorf("task %s: unknown scoring_rule %q (want exact-match, all-of, or any-of)", t.ID, t.ScoringRule)
	}
	switch t.Class {
	case "", ClassObligations, ClassVerify, ClassBlastRadius, ClassClaimProvenance, ClassConcernSearch, ClassContradiction, ClassStaleness,
		ClassFileLocalization, ClassFunctionLocalization, ClassFindCallers:
		// unset is valid (back-compat with the B1 corpus); a recognized class is valid
	default:
		return fmt.Errorf("task %s: unknown class %q (want one of obligations, verify, blast_radius, claim_provenance, concern_search, contradiction, staleness, file_localization, function_localization, find_callers)", t.ID, t.Class)
	}
	return nil
}

// taskFile is the on-disk shape of testdata/contextbench/tasks.json.
type taskFile struct {
	Version     int    `json:"version"`
	Description string `json:"description,omitempty"`
	Tasks       []Task `json:"tasks"`
}

// LoadTasks reads and validates the task corpus at path (JSON, taskFile
// shape). Every task is validated; the first invalid task's error is
// returned so a malformed corpus fails loudly rather than running a broken
// bench silently.
func LoadTasks(path string) ([]Task, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading tasks file %s: %w", path, err)
	}
	var tf taskFile
	if err := json.Unmarshal(raw, &tf); err != nil {
		return nil, fmt.Errorf("parsing tasks file %s: %w", path, err)
	}
	if len(tf.Tasks) == 0 {
		return nil, fmt.Errorf("tasks file %s: no tasks", path)
	}
	for _, task := range tf.Tasks {
		if err := task.Validate(); err != nil {
			return nil, fmt.Errorf("tasks file %s: %w", path, err)
		}
	}
	return tf.Tasks, nil
}

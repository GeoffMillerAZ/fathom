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

	// TruthAtHead names, for every ground-truth token, the repo file its
	// derivation rests on (PB-1). TestHomeCorpusTruthAtHead re-checks each
	// anchor against the working tree, so a ground truth that goes false at
	// HEAD is a gate failure rather than a discovery made after the tokens
	// were spent. Optional: the grep-native B1 corpus predates it.
	TruthAtHead []TruthAnchor `json:"truth_at_head,omitempty"`

	// AnswerAnyOf declares groups of spelling-equivalent gold tokens
	// (VB-36, O47): each inner slice is one required fact, satisfied by
	// ANY member appearing in the answer. O47 lost two otherwise-correct
	// wired trials on H1-T02 to exactly this — the model answered
	// "Edge struct", the corpus demanded the literal "core.Edge" — so the
	// scorer needed a way to accept a spelling variant of a fact without
	// loosening the fact itself. Groups are required exactly like
	// GroundTruth's own tokens: under exact-match/all-of every group (and
	// every plain GroundTruth token) must be satisfied; under any-of,
	// satisfying one plain token OR one group is enough. GroundTruth keeps
	// working completely unchanged — AnswerAnyOf only ADDS grouped
	// alternatives; see Score.
	AnswerAnyOf [][]string `json:"answer_any_of,omitempty"`

	// Requires lists environment capabilities the task cannot be scored
	// without (PB-12) — e.g. "compiled-layer" for a task whose premise is
	// what the LLM-compiled obligations layer serves. RequiresSatisfied
	// decides whether a given environment may run it. Empty = deterministic.
	Requires []string `json:"requires,omitempty"`

	// Exercises names the agent-surface tools the task was written to
	// exercise (BP-6). It is what makes a targeted run possible: a diff
	// maps to tool names (ToolsForFiles) and tool names map to the tasks
	// that exercise them (SelectTasks). Declared honestly per task — it
	// says what the PROMPT was written to require, not everything an arm
	// might happen to call.
	Exercises []string `json:"exercises,omitempty"`

	// MaxPredicted caps how many answer lines a fenced answer block may
	// carry under precision-aware scoring (PB-8, criterion H2-2). Zero means
	// the default, max(5, 2·|gold|) — see MaxPredictedOrDefault.
	MaxPredicted int `json:"max_predicted,omitempty"`

	// MinTrials is this task's own replication floor (RC-18, vision 28
	// §2.4): a task flaky at the corpus's default n is registered with a
	// higher floor (e.g. 5) so the runner and the ledger honor it instead
	// of silently under-replicating just that task. Zero means "use the
	// corpus/criterion default" (MinTrialsOrDefault) — most tasks never set
	// this field.
	MinTrials int `json:"min_trials,omitempty"`
}

// MinTrialsOrDefault returns t's own replication floor when registered
// (MinTrials > 0), else corpusDefault — the criterion's own MinTrials. A
// per-task floor only ever raises the bar for that task; it never lowers it
// below the corpus default.
func (t Task) MinTrialsOrDefault(corpusDefault int) int {
	if t.MinTrials > corpusDefault {
		return t.MinTrials
	}
	return corpusDefault
}

// TruthAnchorKind says how a TruthAnchor is checked against the tree.
type TruthAnchorKind string

const (
	// AnchorFile: Token appears verbatim in File.
	AnchorFile TruthAnchorKind = "file"
	// AnchorPath: Token is a repo-relative path that exists.
	AnchorPath TruthAnchorKind = "path"
	// AnchorLine: Token cites the 1-based line of File where Needle first
	// appears (Token ends with that number, e.g. "CLAUDE.md#L15"), and no
	// other line number of File makes Token a substring of its citation —
	// the guard against the bare-number hole that let "29" match anything
	// containing 29.
	AnchorLine TruthAnchorKind = "line"
	// AnchorForm: Token is an answer form the prompt prescribes (e.g.
	// "verdict: stale") or a value the prompt itself supplies; nothing on
	// disk can confirm it, so Why must say what does.
	AnchorForm TruthAnchorKind = "form"
	// AnchorHeading: Token cites File by the heading id in Needle (e.g.
	// Token "ADR.md#ADR-025", Needle "ADR-025") — the record's own
	// "## ADR-025 …" heading, resolved to wherever it CURRENTLY sits
	// (resolveHeadingLine, citation.go) rather than a hardcoded line
	// number. Item 3, wave N8: an AnchorLine citation into an ADR record
	// breaks on every ADR.md insertion above it; AnchorHeading survives
	// one, because the heading is looked up fresh on every check instead
	// of comparing against a line baked into the corpus.
	AnchorHeading TruthAnchorKind = "heading"
)

// TruthAnchor ties one ground-truth token to the evidence it was derived
// from, in a shape a test can re-check without a daemon.
type TruthAnchor struct {
	Token  string          `json:"token"`
	Kind   TruthAnchorKind `json:"kind"`
	File   string          `json:"file,omitempty"`
	Needle string          `json:"needle,omitempty"`
	Why    string          `json:"why,omitempty"`

	// BaseCommit pins this anchor's resolution to a specific historical
	// git commit's own blob content (`git show <base_commit>:<file>`)
	// rather than the current working tree (wave n13, vs-pilot-5's
	// second reading, H1-T07): a ground truth whose derivation mirrors a
	// scenariobench task's own pinned Subject.BaseCommit must be checked
	// against THAT commit's file content, never live HEAD — HEAD drifts
	// as the file grows (ADR.md gains records above an existing one) and
	// checking it instead would silently paper over exactly the
	// stale-citation defect the reading found ("ADR.md#L597" was true
	// only two days after the pinned base_commit, never at it). Empty
	// (the default, every task before this field existed): resolve
	// against the working tree exactly as before.
	BaseCommit string `json:"base_commit,omitempty"`
}

// MaxPredictedOrDefault returns the task's answer-line cap under
// precision-aware scoring: MaxPredicted when set, else max(5, 2·|gold|) —
// wide enough that a correct multi-file answer with a line of context per
// hit fits, narrow enough that dumping the tree does not.
func (t Task) MaxPredictedOrDefault() int {
	if t.MaxPredicted > 0 {
		return t.MaxPredicted
	}
	if n := 2 * len(t.GroundTruth); n > 5 {
		return n
	}
	return 5
}

// RequiresSatisfied reports whether env — the set of capabilities the
// runner's environment provides — covers every capability the task
// requires (PB-12). A task with no requirements runs anywhere; a task that
// requires "compiled-layer" is skipped, not failed, on a scratch daemon
// that never compiled one, so a deterministic subset of the corpus can be
// scored honestly without the LLM-compiled obligations layer.
func RequiresSatisfied(task Task, env []string) bool {
	have := make(map[string]bool, len(env))
	for _, e := range env {
		have[e] = true
	}
	for _, r := range task.Requires {
		if !have[r] {
			return false
		}
	}
	return true
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
	if len(t.GroundTruth) == 0 && len(t.AnswerAnyOf) == 0 {
		return fmt.Errorf("task %s: empty ground_truth (and no answer_any_of groups)", t.ID)
	}
	for _, g := range t.GroundTruth {
		if g == "" {
			return fmt.Errorf("task %s: ground_truth contains an empty string", t.ID)
		}
	}
	for i, group := range t.AnswerAnyOf {
		if len(group) == 0 {
			return fmt.Errorf("task %s: answer_any_of[%d] is empty", t.ID, i)
		}
		for _, alt := range group {
			if alt == "" {
				return fmt.Errorf("task %s: answer_any_of[%d] contains an empty string", t.ID, i)
			}
		}
	}
	switch t.ScoringRule {
	case ExactMatch:
		if n := len(t.GroundTruth) + len(t.AnswerAnyOf); n != 1 {
			return fmt.Errorf("task %s: scoring_rule exact-match requires exactly one required item (ground_truth + answer_any_of combined), got %d", t.ID, n)
		}
	case AllOf, AnyOf:
		// any non-empty combination of ground_truth/answer_any_of is valid
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
	for i, a := range t.TruthAtHead {
		if a.Token == "" {
			return fmt.Errorf("task %s: truth_at_head[%d] has an empty token", t.ID, i)
		}
		switch a.Kind {
		case AnchorFile, AnchorPath:
			if a.File == "" && a.Kind == AnchorFile {
				return fmt.Errorf("task %s: truth_at_head[%d] (%s) needs a file", t.ID, i, a.Kind)
			}
		case AnchorLine:
			if a.File == "" || a.Needle == "" {
				return fmt.Errorf("task %s: truth_at_head[%d] (line) needs file and needle", t.ID, i)
			}
		case AnchorForm:
			if a.Why == "" {
				return fmt.Errorf("task %s: truth_at_head[%d] (form) must say why nothing on disk confirms it", t.ID, i)
			}
		case AnchorHeading:
			if a.File == "" || a.Needle == "" {
				return fmt.Errorf("task %s: truth_at_head[%d] (heading) needs file and needle (the heading id, e.g. ADR-025)", t.ID, i)
			}
		default:
			return fmt.Errorf("task %s: truth_at_head[%d] unknown kind %q (want file, path, line, heading, or form)", t.ID, i, a.Kind)
		}
	}
	if err := validateExercises(t); err != nil {
		return err
	}
	if t.MaxPredicted < 0 {
		return fmt.Errorf("task %s: max_predicted must be >= 0", t.ID)
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

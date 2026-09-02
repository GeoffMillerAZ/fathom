package publicloc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Arm is one condition of the run. Three are registered: the treatment,
// the control, and the memorisation floor.
type Arm struct {
	Name  string `json:"name"`
	Tools string `json:"tools"`
	Role  string `json:"role"`
}

// RegisteredArms is the three-arm design of ADR-031 §2.2.
var RegisteredArms = []Arm{
	{Name: "fathom", Tools: "fathom MCP companion surface only (read-only; ingest denied)", Role: "treatment"},
	{Name: "grep", Tools: "Bash grep/find/cat only", Role: "control"},
	{Name: "none", Tools: "no tools", Role: "memorisation floor — a task this arm passes in >= leak_rule.min_passes of leak_rule.of_trials trials is leaked and excluded from the verdict"},
}

// LeakRule is the committed none-arm exclusion rule.
type LeakRule struct {
	Arm       string `json:"arm"`
	MinPasses int    `json:"min_passes"`
	OfTrials  int    `json:"of_trials"`
}

// PrecisionRule is the H2-2 answer-format and set-scoring registration.
type PrecisionRule struct {
	AnswerBlock               string `json:"answer_block"`
	MaxPredictedDefault       string `json:"max_predicted_default"`
	TestFilesExcludedFromGold bool   `json:"test_files_excluded_from_gold"`
	Reported                  string `json:"reported"`
}

// ReliabilityRule is the pre-registered statistical criterion for run 1.
type ReliabilityRule struct {
	Primary               string  `json:"primary"`
	CISuccessDiffExcludes float64 `json:"ci_success_diff_excludes"`
	CILevel               float64 `json:"ci_level"`
	MinNonLeakedTasks     int     `json:"min_non_leaked_tasks"`
	BootstrapSeed         int64   `json:"bootstrap_seed"`
	Secondary             string  `json:"secondary"`
}

// MachineCriterion is the corpus's machine_criterion block. The first five
// fields are the shape internal/contextbench.Criterion reads today; the
// rest register H2-2's arms, leak rule, precision and statistics for the
// evaluator that grows into them (PB-8/9/10). Unknown fields are ignored
// by the current loader, so an old scorer still evaluates the H2-1 part.
type MachineCriterion struct {
	ID                           string  `json:"id"`
	Description                  string  `json:"description"`
	RequireStrictlyMoreSuccesses bool    `json:"require_strictly_more_successes"`
	MaxTokenRatio                float64 `json:"max_token_ratio"`
	MinTrials                    int     `json:"min_trials"`

	Arms        []string        `json:"arms"`
	LeakRule    LeakRule        `json:"leak_rule"`
	Precision   PrecisionRule   `json:"precision"`
	Reliability ReliabilityRule `json:"reliability"`
}

// RegisteredCriterion is H2-2 as committed for the public corpus's first
// run. A loss under it is published as-is.
var RegisteredCriterion = MachineCriterion{
	ID:                           "H2-2",
	Description:                  "fathom wins on the public corpus only if BOTH hold over the non-leaked tasks: (1) the seeded, task-clustered paired bootstrap 95% CI on the per-task success difference (fathom minus grep) excludes 0 in fathom's favour, on at least 30 non-leaked tasks — which entails strictly more successes; AND (2) fathom's aggregate tokens are <= 1.0x grep's. Success per cell is all-of over the fenced answer block within max_predicted; precision and recall are reported beside it. A task the none arm passes in >= 2 of 3 trials is leaked and excluded from the verdict, listed in the row. Any cell under 3 trials, or any task absent from an arm, makes the run indeterminate — never a win (ADR-027).",
	RequireStrictlyMoreSuccesses: true,
	MaxTokenRatio:                1.0,
	MinTrials:                    3,
	Arms:                         []string{"fathom", "grep", "none"},
	LeakRule:                     LeakRule{Arm: "none", MinPasses: 2, OfTrials: 3},
	Precision: PrecisionRule{
		AnswerBlock:               "fenced",
		MaxPredictedDefault:       "max(5, 2*|gold|)",
		TestFilesExcludedFromGold: true,
		Reported:                  "precision and recall per cell, beside pass/fail; never gating",
	},
	Reliability: ReliabilityRule{
		Primary:               "paired per-task success difference, fathom minus grep, task-clustered bootstrap CI",
		CISuccessDiffExcludes: 0,
		CILevel:               0.95,
		MinNonLeakedTasks:     30,
		BootstrapSeed:         20260902,
		Secondary:             "aggregate token ratio <= 1.0 (the median is reported for interpretation only)",
	},
}

// Corpus is testdata/contextbench/tasks-public-go.json.
type Corpus struct {
	Version     int                  `json:"version"`
	Description string               `json:"description"`
	Sources     []Pin                `json:"sources"`
	Selection   SelectionReport      `json:"selection"`
	Arms        []Arm                `json:"arms"`
	Criterion   MachineCriterion     `json:"machine_criterion"`
	Instances   []InstanceDerivation `json:"instances"`
	// CorpusSHA256 is the digest of this document's canonical JSON with
	// this field empty — the identity a ledger row carries so two runs can
	// be compared only when they measured the same questions.
	CorpusSHA256 string `json:"corpus_sha256"`
	Tasks        []Task `json:"tasks"`
}

// Description is fixed text: a timestamp or a host name here would make
// two identical derivations hash differently.
const Description = "Public Go localization corpus (docs/plans/bench-public.md PB-14..18, ADR-031): every ground truth is derived, never authored — the files and functions a developer's merged gold patch changed at a pinned base commit, and their static callers as computed by cmd/loctruth (go/types), from the pinned upstream snapshots listed in sources under the selection rule recorded in selection. Rebuild with `contextbenchscore import`; the result must reproduce corpus_sha256 bit-for-bit. The runner ingests each task's subject at base_commit into a scratch store under workspace/ (never the fleet store, never the post-fix tree). Criterion H2-2 is registered in machine_criterion before any run."

// BuildCorpus selects, derives, and hashes. It is deterministic in the
// order of instances, pins, and classes. A non-empty classes list keeps
// only those task classes — a pre-registration choice recorded in
// selection.classes, made before a run, never after one.
func BuildCorpus(instances []Instance, pins []Pin, rule SelectionRule, env Env, classes ...string) (Corpus, error) {
	selected, report := Select(instances, rule)
	tasks, derivations, err := DeriveLocalizationTasks(selected, env)
	if err != nil {
		return Corpus{}, err
	}
	if len(classes) > 0 {
		keep := map[string]bool{}
		for _, c := range classes {
			keep[c] = true
		}
		report.Classes = make([]string, 0, len(keep))
		for c := range keep {
			report.Classes = append(report.Classes, c)
		}
		sort.Strings(report.Classes)
		kept := tasks[:0]
		for _, t := range tasks {
			if keep[t.Class] {
				kept = append(kept, t)
			}
		}
		tasks = kept
		for i := range derivations {
			n := 0
			for _, t := range tasks {
				if t.Derivation.InstanceID == derivations[i].InstanceID {
					n++
				}
			}
			derivations[i].Tasks = n
		}
	}
	bySource := map[string]int{}
	for _, in := range selected {
		bySource[in.Source]++
	}
	sortedPins := append([]Pin(nil), pins...)
	for i := range sortedPins {
		sortedPins[i].Selected = bySource[sortedPins[i].Source]
		if info, ok := Sources[sortedPins[i].Source]; ok {
			sortedPins[i].URL = info.URL
			sortedPins[i].License = info.License
		}
	}
	sort.Slice(sortedPins, func(i, j int) bool { return sortedPins[i].Source < sortedPins[j].Source })
	if tasks == nil {
		tasks = []Task{}
	}
	if derivations == nil {
		derivations = []InstanceDerivation{}
	}
	c := Corpus{
		Version:     1,
		Description: Description,
		Sources:     sortedPins,
		Selection:   report,
		Arms:        RegisteredArms,
		Criterion:   RegisteredCriterion,
		Instances:   derivations,
		Tasks:       tasks,
	}
	c.CorpusSHA256 = CorpusSHA256(c)
	return c, nil
}

// MarshalCanonical is the one encoding of a corpus: two-space indent, no
// HTML escaping, trailing newline. The file on disk is exactly these
// bytes, so the committed file and the hash agree by construction.
func MarshalCanonical(c Corpus) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// CorpusSHA256 hashes the canonical JSON of c with corpus_sha256 blank.
func CorpusSHA256(c Corpus) string {
	c.CorpusSHA256 = ""
	b, err := MarshalCanonical(c)
	if err != nil {
		// Only an unmarshalable value (none exist in Corpus) can fail.
		panic(err)
	}
	return SHA256Hex(b)
}

// LoadCorpus reads a corpus file and checks its recorded hash against a
// recomputation, so a hand-edited corpus is caught at load rather than
// after a run has spent against it.
func LoadCorpus(path string) (Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, err
	}
	var c Corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return Corpus{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if got := CorpusSHA256(c); got != c.CorpusSHA256 {
		return Corpus{}, fmt.Errorf("%s: corpus_sha256 is %s but the content hashes to %s — the file was edited after derivation; re-run contextbenchscore import", path, c.CorpusSHA256, got)
	}
	return c, nil
}

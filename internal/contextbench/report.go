package contextbench

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ArmResult is one task's outcome for one arm ("fathom" or "grep" by
// convention; the runner is free to use other arm names for future variants).
type ArmResult struct {
	TaskID string `json:"task_id"`
	Arm    string `json:"arm"`
	// Trial is the 1-based repetition index within one run. It is omitted
	// for single-trial runs, so every artifact written before repetition
	// existed still parses (a missing Trial reads as 0 and groups as one
	// cell — see StatsByTaskArm).
	Trial   int  `json:"trial,omitempty"`
	Success bool `json:"success"`
	IsError bool `json:"is_error,omitempty"`

	Matched []string `json:"matched,omitempty"`
	Missing []string `json:"missing,omitempty"`

	// Token accounting mirrors claude -p --output-format json's usage
	// block. TotalTokens sums all four — the runner's own baseline includes
	// substantial cache_creation/cache_read overhead from CLAUDE.md and tool
	// schemas that is roughly symmetric across arms (both arms run in the
	// repo's default startup mode so the same system prompt is cached); see
	// scripts/context-bench.sh's header for the full methodology note.
	InputTokens         int64 `json:"input_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	TotalTokens         int64 `json:"total_tokens"`

	WallTimeMS int64  `json:"wall_time_ms"`
	NumTurns   int    `json:"num_turns,omitempty"`
	RawResult  string `json:"raw_result,omitempty"`

	// Additive since PB-3/PB-4/PB-8/PB-11 (all omitempty; older artifacts
	// parse and read as not-recorded).

	// ModelID is the PRIMARY model the envelope reported: the modelUsage
	// entry with the most output tokens (PrimaryModelID), as opposed to
	// the alias the runner asked for. Claude Code also drives a small
	// model for tool search and summaries, and joining every id the
	// envelope mentions made the cell key and the ledger row read as if
	// the run had been measured on two models at once.
	ModelID string `json:"model_id,omitempty"`
	// AuxModels lists every OTHER model the envelope reported, sorted —
	// the harness's own small-model traffic. Additive and reported: it
	// never enters the cell key, because it is not what the run measured.
	AuxModels []string `json:"aux_models,omitempty"`
	// CostUSD is the envelope's total_cost_usd.
	CostUSD float64 `json:"cost_usd,omitempty"`
	// Scoring names the rule the cell was scored under ("substring" when
	// unset — every artifact before H2-2). Precision/Recall/Predicted are
	// set only under precision scoring.
	Scoring   string  `json:"scoring,omitempty"`
	Precision float64 `json:"precision,omitempty"`
	Recall    float64 `json:"recall,omitempty"`
	Predicted int     `json:"predicted,omitempty"`
	// TokensToFirstGoldHit and ToolCalls come from the stream-json
	// transcript when the runner captured one (FirstGoldHit).
	TokensToFirstGoldHit int64 `json:"tokens_to_first_gold_hit,omitempty"`
	ToolCalls            int   `json:"tool_calls,omitempty"`

	// Additive since BP-3/BP-4 (ADR-032 §2): the cell's identity and
	// provenance. CellKey is the hash of the invariants the cell was
	// bought under (CellKey/CellInputs); ArmSurface is the subject the arm
	// was talking to (the fathom binary commit for the fathom arm, empty
	// otherwise); Borrowed says this reading was not paid for by the run
	// that reports it, and SourceRunID names the run that was.
	CellKey     string `json:"cell_key,omitempty"`
	ArmSurface  string `json:"arm_surface,omitempty"`
	Borrowed    bool   `json:"borrowed,omitempty"`
	SourceRunID string `json:"source_run_id,omitempty"`

	// Additive since BP-9 (ADR-032 §3, H2-3): the transcript metrics the
	// cost half of H2-3 reads. PayloadBytes is the discovery cost fathom
	// controls; ToolSearchTurns is the harness tax, reported and never
	// gated; GoldSourceTool names the tool whose result first carried a
	// gold token ("answer" when only the final answer did). See
	// TranscriptMetrics.
	PayloadBytes        int64  `json:"payload_bytes,omitempty"`
	ToolSearchTurns     int    `json:"toolsearch_turns,omitempty"`
	TurnsExclToolSearch int    `json:"turns_excl_toolsearch,omitempty"`
	GoldSourceTool      string `json:"gold_source_tool,omitempty"`
}

// claudeEnvelope is the subset of `claude -p --output-format json`'s result
// envelope this package depends on (verified against a live probe; see
// docs/plans/falsifiability.md B1-2). Unknown fields are ignored.
type claudeEnvelope struct {
	IsError    bool   `json:"is_error"`
	NumTurns   int    `json:"num_turns"`
	Result     string `json:"result"`
	DurationMS int64  `json:"duration_ms"`
	Usage      struct {
		InputTokens              int64 `json:"input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
	} `json:"usage"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	// ModelUsage is keyed by the concrete model id the call resolved to.
	// Claude Code reports MORE than one: the model the run asked for plus
	// the small model it drives for tool search and summaries.
	ModelUsage map[string]modelUsage `json:"modelUsage"`
}

// modelUsage is the per-model slice of the envelope's usage block. Only the
// output tokens are read — they are what says which model did the work.
type modelUsage struct {
	OutputTokens int64 `json:"outputTokens"`
}

// resolvedModelID returns the envelope's PRIMARY model and every other model
// it reported (BP-34).
func (e claudeEnvelope) resolvedModelID() (primary string, aux []string) {
	out := make(map[string]int64, len(e.ModelUsage))
	for id, u := range e.ModelUsage {
		out[id] = u.OutputTokens
	}
	return PrimaryModelID(out)
}

// PrimaryModelID picks the model that DID the run's work from a map of model
// id to output tokens, and returns every other model seen as an additive,
// sorted aux list (BP-34).
//
// A context-bench cell is measured on one model; Claude Code's envelope names
// several, because the harness drives a small model of its own for tool
// search and summaries. Joining them ("a+b") made a two-model string the cell
// key and the ledger row's model_id, which reads as if the bench had measured
// a blend — and changes the key the moment the harness changes its helper
// model, refusing every borrow for a reason that has nothing to do with the
// subject. Output tokens is the discriminator: the model answering the task
// generates the answer.
//
// Deterministic: ties (including an all-zero map, which is what a probe
// envelope with empty modelUsage objects looks like) break lexicographically,
// so the same envelope always yields the same primary.
func PrimaryModelID(outputTokens map[string]int64) (primary string, aux []string) {
	ids := make([]string, 0, len(outputTokens))
	for id := range outputTokens {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if primary == "" || outputTokens[id] > outputTokens[primary] {
			primary = id
		}
	}
	for _, id := range ids {
		if id != primary {
			aux = append(aux, id)
		}
	}
	return primary, aux
}

// ArmResultFromClaudeJSON parses one `claude -p --output-format json` result
// envelope (raw) and scores it against task, producing the ArmResult the
// runner appends to its results file. A claude-reported error (is_error:
// true — e.g. "Not logged in") always scores as a failure regardless of
// whether its (usually empty or diagnostic) result text happens to contain a
// ground-truth substring, so an auth/transport failure can never masquerade
// as a task success.
func ArmResultFromClaudeJSON(task Task, arm string, raw []byte) (ArmResult, error) {
	return armResultFromClaudeJSON(task, arm, raw, ScoringSubstring)
}

// ArmResultFromClaudeJSONWithPrecision is ArmResultFromClaudeJSON under
// H2-2's precision-aware scoring (ScoreWithPrecision): the answer must be
// a fenced block within the task's MaxPredicted, and the result carries
// precision and recall beside pass/fail.
func ArmResultFromClaudeJSONWithPrecision(task Task, arm string, raw []byte) (ArmResult, error) {
	return armResultFromClaudeJSON(task, arm, raw, ScoringPrecision)
}

func armResultFromClaudeJSON(task Task, arm string, raw []byte, scoring string) (ArmResult, error) {
	var env claudeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ArmResult{}, fmt.Errorf("parsing claude -p JSON for task %s (arm %s): %w", task.ID, arm, err)
	}

	total := env.Usage.InputTokens + env.Usage.CacheCreationInputTokens + env.Usage.CacheReadInputTokens + env.Usage.OutputTokens
	ar := ArmResult{
		TaskID:              task.ID,
		Arm:                 arm,
		IsError:             env.IsError,
		InputTokens:         env.Usage.InputTokens,
		CacheCreationTokens: env.Usage.CacheCreationInputTokens,
		CacheReadTokens:     env.Usage.CacheReadInputTokens,
		OutputTokens:        env.Usage.OutputTokens,
		TotalTokens:         total,
		WallTimeMS:          env.DurationMS,
		NumTurns:            env.NumTurns,
		RawResult:           env.Result,
		CostUSD:             env.TotalCostUSD,
	}
	ar.ModelID, ar.AuxModels = env.resolvedModelID()
	switch scoring {
	case ScoringPrecision:
		ps := ScoreWithPrecision(task, env.Result)
		ar.Success = ps.Pass && !env.IsError
		ar.Matched, ar.Missing = ps.Matched, ps.Missing
		ar.Scoring = ScoringPrecision
		ar.Precision, ar.Recall, ar.Predicted = ps.Precision, ps.Recall, ps.Predicted
	default:
		sr := Score(task, env.Result)
		ar.Success = sr.Pass && !env.IsError
		ar.Matched, ar.Missing = sr.Matched, sr.Missing
	}
	return ar, nil
}

// Aggregate summarizes one arm's results across all tasks.
type Aggregate struct {
	Arm          string  `json:"arm"`
	TaskCount    int     `json:"task_count"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	TotalTokens  int64   `json:"total_tokens"`
	MeanTokens   float64 `json:"mean_tokens"`
	TotalWallMS  int64   `json:"total_wall_ms"`
	MeanWallMS   float64 `json:"mean_wall_ms"`
}

// Summarize aggregates results per arm.
func Summarize(results []ArmResult) map[string]Aggregate {
	agg := map[string]Aggregate{}
	for _, r := range results {
		a := agg[r.Arm]
		a.Arm = r.Arm
		a.TaskCount++
		if r.Success {
			a.SuccessCount++
		}
		a.TotalTokens += r.TotalTokens
		a.TotalWallMS += r.WallTimeMS
		agg[r.Arm] = a
	}
	for arm, a := range agg {
		if a.TaskCount > 0 {
			a.SuccessRate = float64(a.SuccessCount) / float64(a.TaskCount)
			a.MeanTokens = float64(a.TotalTokens) / float64(a.TaskCount)
			a.MeanWallMS = float64(a.TotalWallMS) / float64(a.TaskCount)
		}
		agg[arm] = a
	}
	return agg
}

// RenderMarkdown renders a deterministic per-task, per-arm results table
// plus an aggregate summary row per arm. Rows are sorted by (task ID, arm)
// so the output is stable across runs with the same input, regardless of
// the order results were appended in.
func RenderMarkdown(results []ArmResult) string {
	sorted := append([]ArmResult(nil), results...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TaskID != sorted[j].TaskID {
			return sorted[i].TaskID < sorted[j].TaskID
		}
		return sorted[i].Arm < sorted[j].Arm
	})

	var b strings.Builder
	b.WriteString("| task | arm | success | total_tokens | wall_time_ms | num_turns |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, r := range sorted {
		status := "FAIL"
		if r.Success {
			status = "PASS"
		}
		if r.IsError {
			status = "ERROR"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d |\n", r.TaskID, r.Arm, status, r.TotalTokens, r.WallTimeMS, r.NumTurns)
	}

	b.WriteString("\n| arm | tasks | successes | success_rate | total_tokens | mean_tokens/task |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	agg := Summarize(results)
	arms := make([]string, 0, len(agg))
	for arm := range agg {
		arms = append(arms, arm)
	}
	sort.Strings(arms)
	for _, arm := range arms {
		a := agg[arm]
		fmt.Fprintf(&b, "| %s | %d | %d | %.0f%% | %d | %.0f |\n", a.Arm, a.TaskCount, a.SuccessCount, a.SuccessRate*100, a.TotalTokens, a.MeanTokens)
	}

	return b.String()
}

// RunResults is the machine-readable results-<ts>.json artifact the runner
// writes to workspace/artifacts/context-bench/.
type RunResults struct {
	GeneratedAt string               `json:"generated_at"`
	TasksFile   string               `json:"tasks_file"`
	Results     []ArmResult          `json:"results"`
	Summary     map[string]Aggregate `json:"summary"`
}

// NewRunResults builds the results artifact payload, computing Summary from
// results so callers never have to keep the two in sync by hand.
func NewRunResults(generatedAt, tasksFile string, results []ArmResult) RunResults {
	return RunResults{
		GeneratedAt: generatedAt,
		TasksFile:   tasksFile,
		Results:     results,
		Summary:     Summarize(results),
	}
}

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
	TaskID  string `json:"task_id"`
	Arm     string `json:"arm"`
	Success bool   `json:"success"`
	IsError bool   `json:"is_error,omitempty"`

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
}

// ArmResultFromClaudeJSON parses one `claude -p --output-format json` result
// envelope (raw) and scores it against task, producing the ArmResult the
// runner appends to its results file. A claude-reported error (is_error:
// true — e.g. "Not logged in") always scores as a failure regardless of
// whether its (usually empty or diagnostic) result text happens to contain a
// ground-truth substring, so an auth/transport failure can never masquerade
// as a task success.
func ArmResultFromClaudeJSON(task Task, arm string, raw []byte) (ArmResult, error) {
	var env claudeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ArmResult{}, fmt.Errorf("parsing claude -p JSON for task %s (arm %s): %w", task.ID, arm, err)
	}

	sr := Score(task, env.Result)
	success := sr.Pass && !env.IsError

	total := env.Usage.InputTokens + env.Usage.CacheCreationInputTokens + env.Usage.CacheReadInputTokens + env.Usage.OutputTokens

	return ArmResult{
		TaskID:              task.ID,
		Arm:                 arm,
		Success:             success,
		IsError:             env.IsError,
		Matched:             sr.Matched,
		Missing:             sr.Missing,
		InputTokens:         env.Usage.InputTokens,
		CacheCreationTokens: env.Usage.CacheCreationInputTokens,
		CacheReadTokens:     env.Usage.CacheReadInputTokens,
		OutputTokens:        env.Usage.OutputTokens,
		TotalTokens:         total,
		WallTimeMS:          env.DurationMS,
		NumTurns:            env.NumTurns,
		RawResult:           env.Result,
	}, nil
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

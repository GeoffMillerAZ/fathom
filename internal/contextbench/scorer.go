package contextbench

import (
	"sort"
	"strings"
)

// ScoreResult is the verdict for one task's arm output.
type ScoreResult struct {
	TaskID  string   `json:"task_id"`
	Pass    bool     `json:"pass"`
	Matched []string `json:"matched,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

// Score checks output against task's ground truth per its scoring rule.
// Matching is a case-sensitive substring check: file paths, symbol names,
// and ADR IDs are specific tokens, and lowercasing risks false positives
// across differently-cased identifiers in the same codebase (see
// TestScore_CaseSensitiveSubstring). Matched/Missing are always returned
// sorted, so results are deterministic regardless of ground_truth's
// declaration order.
func Score(task Task, output string) ScoreResult {
	var matched, missing []string
	for _, want := range task.GroundTruth {
		if strings.Contains(output, want) {
			matched = append(matched, want)
		} else {
			missing = append(missing, want)
		}
	}
	sort.Strings(matched)
	sort.Strings(missing)

	var pass bool
	switch task.ScoringRule {
	case ExactMatch, AllOf:
		pass = len(missing) == 0 && len(matched) > 0
	case AnyOf:
		pass = len(matched) > 0
	}

	return ScoreResult{TaskID: task.ID, Pass: pass, Matched: matched, Missing: missing}
}

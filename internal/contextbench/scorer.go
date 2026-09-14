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
//
// task.AnswerAnyOf (VB-36, O47) adds groups of spelling-equivalent gold
// tokens beside task.GroundTruth's plain, all-required tokens: a group is
// satisfied by ANY one of its members, and — like a GroundTruth token — a
// satisfied group contributes the member that actually matched to Matched,
// while an unsatisfied group contributes its full alternatives (joined by
// " | ") to Missing, so a reader can see what was demanded. Every group
// participates in the scoring rule exactly as a GroundTruth token would:
// exact-match/all-of require every group (and every plain token) to be
// satisfied; any-of is satisfied by one plain token or one group.
// GroundTruth's own behavior is unchanged when AnswerAnyOf is empty.
func Score(task Task, output string) ScoreResult {
	var matched, missing []string
	for _, want := range task.GroundTruth {
		if strings.Contains(output, want) {
			matched = append(matched, want)
		} else {
			missing = append(missing, want)
		}
	}
	for _, group := range task.AnswerAnyOf {
		hit := ""
		for _, alt := range group {
			if strings.Contains(output, alt) {
				hit = alt
				break
			}
		}
		if hit != "" {
			matched = append(matched, hit)
		} else {
			missing = append(missing, strings.Join(group, " | "))
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

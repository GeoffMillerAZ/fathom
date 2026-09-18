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
		if tokenPresent(output, want) {
			matched = append(matched, want)
		} else {
			missing = append(missing, want)
		}
	}
	for _, group := range task.AnswerAnyOf {
		hit := ""
		for _, alt := range group {
			if tokenPresent(output, alt) {
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

// tokenPresent reports whether want occurs in output as a whole token. A
// token that ends in a digit — every "file#L<n>" citation — counts only
// where the next byte is NOT a digit, so "CLAUDE.md#L10" never scores an
// answer that cited "CLAUDE.md#L100" (the substring hole the corpus's
// truth-at-head check used to refuse by forbidding such tokens outright;
// this rule closes it at the scorer, where it belongs, for every token).
// "CLAUDE.md#L10-L12" still satisfies "CLAUDE.md#L10": a range that starts
// on the cited line cites it.
func tokenPresent(output, want string) bool {
	if want == "" {
		return false
	}
	if !endsInDigit(want) {
		return strings.Contains(output, want)
	}
	for start := 0; start < len(output); {
		idx := strings.Index(output[start:], want)
		if idx < 0 {
			return false
		}
		end := start + idx + len(want)
		if end >= len(output) || output[end] < '0' || output[end] > '9' {
			return true
		}
		start = start + idx + 1
	}
	return false
}

func endsInDigit(s string) bool {
	return s != "" && s[len(s)-1] >= '0' && s[len(s)-1] <= '9'
}

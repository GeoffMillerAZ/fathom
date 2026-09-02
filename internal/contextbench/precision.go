package contextbench

import (
	"sort"
	"strings"
)

// Precision-aware scoring (PB-8, criterion H2-2; docs/vision/26-proof-
// credibility.md §2.2).
//
// Substring all-of scoring is passable by dumping the tree: an arm that
// lists every file in the repo matches every path token. The localization
// benchmarks the field converged on grade a SET-valued answer with a
// precision term, so this file adds one without touching how H2-1 scores:
//
//   - the answer goes in a fenced block (``` … ```); the LAST fenced block
//     in the output is the answer, so an arm may think aloud first;
//   - each non-empty line of the block is one predicted item, capped by the
//     task's MaxPredicted (default max(5, 2·|gold|));
//   - precision = predicted lines that carry a gold token ÷ predicted lines;
//     recall = gold tokens found ÷ gold tokens;
//   - pass = the task's scoring rule over the block's text AND the block is
//     within the cap. A missing block is a fail, not a fallback to prose.
//
// Test files are excluded from gold as the localization papers do: a path
// token ending in _test.go is dropped before scoring, so a fix's test is
// neither credited nor demanded. A task whose gold is only test files has
// no gold left and cannot pass — which is the right reading of it.
//
// Because this changes the success definition after H2-1 has recorded
// data, it is registered as a NEW criterion, H2-2, beside H2-1 in the
// corpus; H2-1's verdicts stand as history (T5).

// PrecisionScore is ScoreWithPrecision's verdict for one cell.
type PrecisionScore struct {
	ScoreResult
	// Fenced is false when no fenced answer block was found; Pass is then
	// false regardless of what the prose contained.
	Fenced bool `json:"fenced"`
	// Predicted counts non-empty lines in the answer block.
	Predicted    int `json:"predicted"`
	MaxPredicted int `json:"max_predicted"`
	// OverBudget is true when Predicted exceeds MaxPredicted; Pass is then
	// false even if every gold token was present — that is the point.
	OverBudget bool    `json:"over_budget"`
	Precision  float64 `json:"precision"`
	Recall     float64 `json:"recall"`
}

// ScoreWithPrecision scores output against task under H2-2's rules. The
// base pass/fail is the task's own ScoringRule applied to the block text,
// so an any-of task still needs one hit and an all-of task all of them.
func ScoreWithPrecision(task Task, output string) PrecisionScore {
	gold := GoldWithoutTestFiles(task.GroundTruth)
	ps := PrecisionScore{MaxPredicted: task.MaxPredictedOrDefault()}
	ps.TaskID = task.ID

	block, ok := LastFencedBlock(output)
	ps.Fenced = ok
	if !ok {
		ps.Missing = append([]string(nil), gold...)
		sort.Strings(ps.Missing)
		return ps
	}

	var lines []string
	for _, l := range strings.Split(block, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	ps.Predicted = len(lines)
	ps.OverBudget = ps.Predicted > ps.MaxPredicted

	scored := Score(Task{ID: task.ID, GroundTruth: gold, ScoringRule: task.ScoringRule}, block)
	ps.Matched, ps.Missing = scored.Matched, scored.Missing

	hits := 0
	for _, l := range lines {
		for _, g := range gold {
			if strings.Contains(l, g) {
				hits++
				break
			}
		}
	}
	if ps.Predicted > 0 {
		ps.Precision = float64(hits) / float64(ps.Predicted)
	}
	if len(gold) > 0 {
		ps.Recall = float64(len(ps.Matched)) / float64(len(gold))
	}
	ps.Pass = scored.Pass && !ps.OverBudget && len(gold) > 0
	return ps
}

// GoldWithoutTestFiles drops path tokens that name Go test files. Order is
// preserved; the input is not modified.
func GoldWithoutTestFiles(gold []string) []string {
	out := make([]string, 0, len(gold))
	for _, g := range gold {
		if strings.HasSuffix(g, "_test.go") {
			continue
		}
		out = append(out, g)
	}
	return out
}

// LastFencedBlock returns the contents of the last ``` fenced block in
// text (any info string after the opening fence is ignored), and false if
// there is none. An unterminated final fence is treated as running to the
// end of the text, since a truncated envelope should still score what it
// managed to say.
func LastFencedBlock(text string) (string, bool) {
	const fence = "```"
	idx := strings.Index(text, fence)
	if idx < 0 {
		return "", false
	}
	var last string
	found := false
	rest := text
	for {
		open := strings.Index(rest, fence)
		if open < 0 {
			break
		}
		after := rest[open+len(fence):]
		// Skip the info string (up to end of line).
		nl := strings.Index(after, "\n")
		if nl < 0 {
			break
		}
		body := after[nl+1:]
		closeIdx := strings.Index(body, fence)
		if closeIdx < 0 {
			last, found = body, true
			break
		}
		last, found = body[:closeIdx], true
		rest = body[closeIdx+len(fence):]
	}
	return last, found
}

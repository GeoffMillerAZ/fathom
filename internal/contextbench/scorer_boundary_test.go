package contextbench

import "testing"

// A line citation token never scores an answer that cited a longer line
// number, and a range that starts on the cited line does count.
func TestScore_LineCitationTokenNeverMatchesLongerLine(t *testing.T) {
	task := Task{ID: "t", GroundTruth: []string{"CLAUDE.md#L10"}, ScoringRule: AllOf}
	cases := map[string]bool{
		"the rule is at CLAUDE.md#L100.":             false,
		"the rule is at CLAUDE.md#L10.":              true,
		"the rule is at CLAUDE.md#L10":               true,
		"the rule is at CLAUDE.md#L10-L12 now":       true,
		"nothing cited":                              false,
		"CLAUDE.md#L101 and later CLAUDE.md#L10 too": true,
	}
	for output, want := range cases {
		if got := Score(task, output).Pass; got != want {
			t.Errorf("Score(%q) pass = %v, want %v", output, got, want)
		}
	}
}

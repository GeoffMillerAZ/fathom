package readout

import (
	"strings"
	"testing"
)

func TestTestDeltaUsesLastDecidedCompleteSameShape(t *testing.T) {
	const id = "example.com/m/cart.TestA"
	base := TestRunRow{Invocation: "go test", Result: "fail", Head: "h1", Tree: "t1", Scope: "./...", Failing: []string{id}}
	now := TestRunNow{Invocation: "go test", Head: "h2", Tree: "t2", Scope: "./...", Failing: []string{id}}
	if line, outcome := RunDelta(nil, now); line != "" || outcome != DeltaNotCompared {
		t.Errorf("first run: line=%q outcome=%q", line, outcome)
	}
	for _, tc := range []struct {
		name string
		row  TestRunRow
	}{
		{"unknown", TestRunRow{Invocation: "go test", Result: "unknown", Head: "h2", Tree: "t2", Scope: "./..."}},
		{"partial failure", TestRunRow{Invocation: "go test", Result: "fail", Partial: true, Head: "h2", Tree: "t2", Scope: "./..."}},
		{"partial pass", TestRunRow{Invocation: "go test", Result: "pass", Partial: true, Head: "h2", Tree: "t2", Scope: "./..."}},
		{"other scope", TestRunRow{Invocation: "go test", Result: "pass", Head: "h2", Tree: "t2", Scope: "./other"}},
		{"other command", TestRunRow{Invocation: "task test", Result: "pass", Head: "h2", Tree: "t2", Scope: "./..."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line, outcome := RunDelta([]TestRunRow{base, tc.row}, now)
			if line != "delta vs the last run: still failing cart.TestA (since h1)" || outcome != DeltaCompared {
				t.Fatalf("line=%q outcome=%q", line, outcome)
			}
			line, outcome = RunDelta([]TestRunRow{tc.row}, now)
			if line != "" || outcome != DeltaNotCompared {
				t.Fatalf("no decided same-shape baseline: line=%q outcome=%q", line, outcome)
			}
		})
	}
	pass := base
	pass.Result, pass.Failing = "pass", nil
	line, outcome := RunDelta([]TestRunRow{base, pass}, now)
	if outcome != DeltaCompared || !strings.Contains(line, "new cart.TestA") || strings.Contains(line, "still failing") {
		t.Fatalf("latest decided run must win: line=%q outcome=%q", line, outcome)
	}
}

package readout

import "testing"

func TestTestDeltaUnknownBaseNeverLabelsNew(t *testing.T) {
	base := TestRunRow{Invocation: "go test", Result: "unknown", Head: "h1", Tree: "t1", Scope: "./..."}
	now := TestRunNow{Invocation: "go test", Head: "h2", Tree: "t2", Scope: "./...", Failing: []string{"example.com/m/cart.TestA"}}
	line, outcome := RunDelta([]TestRunRow{base}, now)
	if line != "" || outcome != DeltaNotCompared {
		t.Fatalf("unknown baseline: %q (%s)", line, outcome)
	}
}

package readout

import (
	"strings"
	"testing"
)

func TestTestDeltaUnknownBaseNeverLabelsNew(t *testing.T) {
	base := TestRunRow{Invocation: "go test", Result: "unknown", Head: "h1", Tree: "t1", Scope: "./..."}
	now := TestRunNow{Invocation: "go test", Head: "h2", Tree: "t2", Scope: "./...", Failing: []string{"example.com/m/cart.TestA"}}
	line, _ := RunDelta([]TestRunRow{base}, now)
	if strings.Contains(line, "new ") || !strings.Contains(line, "not compared") {
		t.Fatalf("unknown baseline: %q", line)
	}
}

package decision_test

import (
	"context"
	"errors"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

func TestNone(t *testing.T) {
	var n decision.None

	if got := n.Name(); got != "none" {
		t.Errorf("Name() = %q, want %q", got, "none")
	}

	resp, err := n.Decide(context.Background(), decision.Request{
		State:     "s",
		Questions: []decision.Question{validNoulQuestion("q1")},
	})
	if !errors.Is(err, decision.ErrNoProvider) {
		t.Fatalf("Decide() error = %v, want ErrNoProvider", err)
	}
	if resp.Provider != "none" {
		t.Errorf("Decide() Response.Provider = %q, want %q", resp.Provider, "none")
	}

	// None satisfies Decider.
	var _ decision.Decider = decision.None{}
}

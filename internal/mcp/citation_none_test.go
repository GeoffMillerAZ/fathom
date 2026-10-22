package mcp_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func TestCitationProviderNoneMatchesUnconfigured(t *testing.T) {
	unconfigured := jg2Envelope(t, jg2Client(t))
	loaded, err := config.Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Config.Decide.Provider = "none"
	none := jg2Client(t)
	none.WithDecision(decision.None{}, loaded.Config.Decide, nil)
	if got := jg2Envelope(t, none); got != unconfigured {
		t.Fatalf("provider none differs from unconfigured obligations\nnone: %s\nunconfigured: %s", got, unconfigured)
	}
}

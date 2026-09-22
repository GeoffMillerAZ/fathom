package decide_test

import (
	"errors"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// TestOpen_NoneAndLLM pins DP-2 (docs/plans/decision-port.md): "none" builds
// the kit's zero-configuration decision.None without ever calling lane;
// "llm" calls lane with cfg.Profile and wraps the returned client in
// decision.NewLLM; any other provider is refused by name; a lane failure
// propagates.
func TestOpen_NoneAndLLM(t *testing.T) {
	t.Run("none never calls lane", func(t *testing.T) {
		called := false
		lane := func(profile string) (llmclient.LLMClient, error) {
			called = true
			return nil, nil
		}
		ports, err := decide.Open(config.Decide{Provider: "none"}, lane, "test-model", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if called {
			t.Error("lane must not be called for provider \"none\"")
		}
		if _, ok := ports.Decider.(decision.None); !ok {
			t.Errorf("Decider = %T, want decision.None", ports.Decider)
		}
	})

	t.Run("empty provider defaults like none", func(t *testing.T) {
		ports, err := decide.Open(config.Decide{}, nil, "", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if _, ok := ports.Decider.(decision.None); !ok {
			t.Errorf("Decider = %T, want decision.None", ports.Decider)
		}
	})

	t.Run("llm calls lane with the configured profile", func(t *testing.T) {
		var gotProfile string
		client := llmclient.NewMockClient()
		lane := func(profile string) (llmclient.LLMClient, error) {
			gotProfile = profile
			return client, nil
		}
		ports, err := decide.Open(config.Decide{Provider: "llm", Profile: "work"}, lane, "test-model", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if gotProfile != "work" {
			t.Errorf("lane called with profile %q, want %q", gotProfile, "work")
		}
		l, ok := ports.Decider.(*decision.LLM)
		if !ok {
			t.Fatalf("Decider = %T, want *decision.LLM", ports.Decider)
		}
		if l.Name() != "llm" {
			t.Errorf("Name() = %q, want %q", l.Name(), "llm")
		}
		if ports.Cfg.Provider != "llm" {
			t.Errorf("Cfg = %+v, want it echoed back", ports.Cfg)
		}
	})

	t.Run("unknown provider refused by name", func(t *testing.T) {
		_, err := decide.Open(config.Decide{Provider: "jev"}, nil, "", nil)
		if err == nil {
			t.Fatal("expected an error for an unknown provider")
		}
		if !strings.Contains(err.Error(), "jev") {
			t.Errorf("error = %v, want it to name %q", err, "jev")
		}
	})

	t.Run("lane error propagates", func(t *testing.T) {
		wantErr := errors.New("boom: no credential")
		lane := func(profile string) (llmclient.LLMClient, error) { return nil, wantErr }
		_, err := decide.Open(config.Decide{Provider: "llm"}, lane, "test-model", nil)
		if err == nil || !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want a wrap of %v", err, wantErr)
		}
	})
}

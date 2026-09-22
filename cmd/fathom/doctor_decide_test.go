package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
)

// fakePingerClient is a minimal llmclient.LLMClient that also implements
// llmclient.Pinger, so checkDecisionProvider's ping branch runs (and
// succeeds) without a real credential or network call.
type fakePingerClient struct {
	pingErr error
}

func (fakePingerClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	return llmclient.Response{Text: "ok"}, nil
}

func (fakePingerClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch := make(chan llmclient.StreamChunk)
	close(ch)
	return ch, nil
}

func (f fakePingerClient) Ping(ctx context.Context) error { return f.pingErr }

var (
	_ llmclient.LLMClient = fakePingerClient{}
	_ llmclient.Pinger    = fakePingerClient{}
)

// TestDoctorDecisionProvider pins DP-4 (docs/plans/decision-port.md):
// provider "none" (the default, absent config included) is skipped naming
// why; provider "llm" with a lane that pings successfully is a pass; an
// unknown provider value is a fail naming it. The "unknown provider" case
// is driven against decisionProviderVerdict directly — reaching it through
// a real config.toml is impossible by construction, since #Decide's
// provider enum already refuses that value at config.Load.
func TestDoctorDecisionProvider(t *testing.T) {
	t.Run("none is skipped", func(t *testing.T) {
		root := t.TempDir()
		build := func(config.Profile) (llmclient.LLMClient, error) {
			t.Fatal("build must not be called for provider none")
			return nil, nil
		}
		got := checkDecisionProvider(root, build)
		if got.Status != doctorSkipped {
			t.Fatalf("Status = %v, want skipped: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "none") {
			t.Errorf("Reason = %q, want it to name provider none", got.Reason)
		}
	})

	t.Run("llm with a fake lane pings and passes", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
			t.Fatal(err)
		}
		toml := `[decide]
provider = "llm"
profile  = "work"

[profiles.work]
provider = "claude-cli"
`
		if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}

		var gotProfile config.Profile
		build := func(p config.Profile) (llmclient.LLMClient, error) {
			gotProfile = p
			return fakePingerClient{}, nil
		}
		got := checkDecisionProvider(root, build)
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		if gotProfile.Provider != "claude-cli" {
			t.Errorf("build called with profile %+v, want the [profiles.work] profile", gotProfile)
		}
		if !strings.Contains(got.Reason, "work") {
			t.Errorf("Reason = %q, want it to name the profile", got.Reason)
		}
	})

	t.Run("unknown provider fails naming it", func(t *testing.T) {
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "jev"}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail {
			t.Fatalf("Status = %v, want fail: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "jev") {
			t.Errorf("Reason = %q, want it to name %q", got.Reason, "jev")
		}
	})
}

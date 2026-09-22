package main

import (
	"context"
	"net/http"
	"net/http/httptest"
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

// TestDoctorDecisionProvider_TypeSafe pins DP-17: provider "typesafe" with
// no resolvable key is a FAIL naming the missing key; a 401 from a real
// ping is a FAIL naming the endpoint; a 200 System One envelope is a PASS
// naming the model and endpoint. decide.Open is not exercised here — this
// drives decisionProviderVerdict directly, the same way the "unknown
// provider" case above does, since the ping path needs a real (fake)
// server, not a lane seam.
func TestDoctorDecisionProvider_TypeSafe(t *testing.T) {
	t.Run("no key fails", func(t *testing.T) {
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "typesafe"}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail {
			t.Fatalf("Status = %v, want fail: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "no API key") {
			t.Errorf("Reason = %q, want it to name the missing key", got.Reason)
		}
	})

	t.Run("401 ping failure fails naming the endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
		}))
		defer srv.Close()

		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{
			Provider: "typesafe", Endpoint: srv.URL, APIKeyEnv: "FATHOM_TEST_TYPESAFE_KEY",
		}}}
		t.Setenv("FATHOM_TEST_TYPESAFE_KEY", "bad-key")
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail {
			t.Fatalf("Status = %v, want fail: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, srv.URL) {
			t.Errorf("Reason = %q, want it to name the endpoint %q", got.Reason, srv.URL)
		}
		if strings.Contains(got.Reason, "bad-key") {
			t.Errorf("Reason = %q, must never carry the key", got.Reason)
		}
	})

	t.Run("200 envelope passes naming model and endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":2}}`))
		}))
		defer srv.Close()

		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{
			Provider: "typesafe", Endpoint: srv.URL, Model: "jev-latest", APIKeyEnv: "FATHOM_TEST_TYPESAFE_KEY",
		}}}
		t.Setenv("FATHOM_TEST_TYPESAFE_KEY", "good-key")
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "jev-latest") || !strings.Contains(got.Reason, srv.URL) {
			t.Errorf("Reason = %q, want it to name the model and endpoint", got.Reason)
		}
	})
}

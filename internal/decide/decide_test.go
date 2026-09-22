package decide_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

// TestOpen_TypeSafe pins DP-16 (docs/plans/decision-port.md): provider
// "typesafe" builds the kit's TypeSafe Decider directly (no lane) over a
// key resolved by config.Decide.ResolveAPIKey; an empty key is an error
// (never a silent None — decide.Open never falls back on its own, only the
// cmd/fathom caller does, per I14); cfg.Endpoint is honored as the base URL
// a real Decide call hits.
func TestOpen_TypeSafe(t *testing.T) {
	t.Run("empty key is an error", func(t *testing.T) {
		_, err := decide.Open(config.Decide{Provider: "typesafe"}, nil, "", nil)
		if err == nil {
			t.Fatal("expected an error for a typesafe provider with no API key")
		}
		if !strings.Contains(err.Error(), "typesafe") {
			t.Errorf("error = %v, want it to name the typesafe provider", err)
		}
	})

	t.Run("key set via env builds a typesafe Decider", func(t *testing.T) {
		t.Setenv("TYPESAFE_API_KEY", "test-key")
		ports, err := decide.Open(config.Decide{Provider: "typesafe"}, nil, "", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if ports.Decider == nil || ports.Decider.Name() != "typesafe" {
			t.Errorf("Decider = %v, want Name() == %q", ports.Decider, "typesafe")
		}
		if ports.Cfg.Provider != "typesafe" {
			t.Errorf("Cfg = %+v, want it echoed back", ports.Cfg)
		}
	})

	t.Run("endpoint honored by a fake server on Decide", func(t *testing.T) {
		var gotPath, gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":2}}`))
		}))
		defer srv.Close()

		t.Setenv("TYPESAFE_TEST_KEY", "sekret")
		ports, err := decide.Open(config.Decide{Provider: "typesafe", Endpoint: srv.URL, APIKeyEnv: "TYPESAFE_TEST_KEY"}, nil, "", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		resp, err := ports.Decider.Decide(context.Background(), decision.Request{
			State:     "ping",
			Questions: []decision.Question{{ID: "ping", Kind: decision.KindNoul, Instructions: "The statement is a ping."}},
		})
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if _, ok := resp.Answers["ping"]; !ok {
			t.Errorf("Answers = %+v, want a \"ping\" answer", resp.Answers)
		}
		if gotPath != "/v1/systemone" {
			t.Errorf("request path = %q, want /v1/systemone (cfg.Endpoint %q honored)", gotPath, srv.URL)
		}
		if gotAuth != "Bearer sekret" {
			t.Errorf("Authorization = %q, want Bearer sekret", gotAuth)
		}
	})
}

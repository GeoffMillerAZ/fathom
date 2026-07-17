package main

// llmprofile_equivalence_test.go — S2 drift guard (coordinator review of
// docs/plans/config-experience.md CF-3): compile.go's six env-only
// build*Client functions and llmprofile.go's six build*ClientProfile
// functions are deliberately parallel implementations (the originals are
// byte-frozen under compile_test.go's 15 tests; the profile path is
// additive). Two parallel switches plus a duplicated env→field mapping is
// exactly where silent divergence breeds — and S1 proved a day-one
// divergence (the bedrock fail-fast) already happened. This table drives
// every lane through BOTH paths from the same environment and asserts they
// agree on construct-vs-error — and, for the HTTP-routed lanes, that both
// route to the same path+query.
//
// The env→profile bridge is the real one: config.ResolveProfile over an
// empty Config, which is precisely how production resolves "no config file,
// env only" (the byte-for-byte-preserved overlay lane).

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
)

func TestEnvAndProfileLanes_Equivalent(t *testing.T) {
	type lane struct {
		name string
		// env is applied on top of a fully cleared provider environment.
		// Values may be computed per-run (fake binaries, httptest URLs), so
		// it's a func.
		env func(t *testing.T, srvURL string) map[string]string
		// wantConstruct: both paths must construct (nil error); false: both
		// must fail.
		wantConstruct bool
		// routeCheck drives one Complete through each constructed client
		// against the fake OpenAI server and asserts both hit the same
		// path+query.
		routeCheck bool
		// model for routeCheck Complete calls.
		model string
	}

	lanes := []lane{
		{
			name: "anthropic constructs with key",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "anthropic", "ANTHROPIC_API_KEY": "test-key"}
			},
			wantConstruct: true,
		},
		{
			name: "anthropic fails without key",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "anthropic"}
			},
			wantConstruct: false,
		},
		{
			name: "default (empty provider) behaves as anthropic",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"ANTHROPIC_API_KEY": "test-key"}
			},
			wantConstruct: true,
		},
		{
			name: "claude-cli constructs with resolvable bin",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "claude-cli", "FATHOM_CLAUDE_BIN": newFakeClaudeCLI(t)}
			},
			wantConstruct: true,
		},
		{
			name: "claude-cli fails on missing bin",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "claude-cli", "FATHOM_CLAUDE_BIN": t.TempDir() + "/does-not-exist"}
			},
			wantConstruct: false,
		},
		{
			name: "openai constructs and routes via base-url override",
			env: func(t *testing.T, srvURL string) map[string]string {
				return map[string]string{
					"FATHOM_LLM_PROVIDER":    "openai",
					"OPENAI_API_KEY":         "test-key",
					"FATHOM_OPENAI_BASE_URL": srvURL + "/v1",
				}
			},
			wantConstruct: true,
			routeCheck:    true,
			model:         "model-x",
		},
		{
			name: "azure-openai constructs and routes to deployment",
			env: func(t *testing.T, srvURL string) map[string]string {
				return map[string]string{
					"FATHOM_LLM_PROVIDER":     "azure-openai",
					"AZURE_OPENAI_API_KEY":    "test-key",
					"AZURE_OPENAI_ENDPOINT":   srvURL,
					"AZURE_OPENAI_DEPLOYMENT": "equiv-deployment",
				}
			},
			wantConstruct: true,
			routeCheck:    true,
			model:         "gpt-4o",
		},
		{
			name: "azure-openai fails without endpoint",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "azure-openai", "AZURE_OPENAI_DEPLOYMENT": "d"}
			},
			wantConstruct: false,
		},
		{
			name: "azure-openai fails without deployment",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "azure-openai", "AZURE_OPENAI_ENDPOINT": "https://x.openai.azure.com"}
			},
			wantConstruct: false,
		},
		{
			name: "bedrock constructs with env credentials and model",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{
					"FATHOM_LLM_PROVIDER":   "bedrock",
					"FATHOM_COMPILE_MODEL":  "anthropic.claude-3-5-sonnet-20241022-v2:0",
					"AWS_ACCESS_KEY_ID":     "test-access-key",
					"AWS_SECRET_ACCESS_KEY": "test-secret-key",
					"AWS_REGION":            "us-east-1",
				}
			},
			wantConstruct: true,
		},
		{
			name: "bedrock fails without a model",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{
					"FATHOM_LLM_PROVIDER":   "bedrock",
					"AWS_ACCESS_KEY_ID":     "test-access-key",
					"AWS_SECRET_ACCESS_KEY": "test-secret-key",
					"AWS_REGION":            "us-east-1",
				}
			},
			wantConstruct: false,
		},
		{
			name: "codex-cli constructs on healthy ping",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "codex-cli", "FATHOM_CODEX_BIN": newFakeCodexCLI(t, fakeCodexCLIHealthyScript)}
			},
			wantConstruct: true,
		},
		{
			name: "codex-cli fails on degraded ping",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "codex-cli", "FATHOM_CODEX_BIN": newFakeCodexCLI(t, fakeCodexCLIDegradedScript)}
			},
			wantConstruct: false,
		},
		{
			name: "unknown provider fails in both",
			env: func(t *testing.T, _ string) map[string]string {
				return map[string]string{"FATHOM_LLM_PROVIDER": "bogus-provider"}
			},
			wantConstruct: false,
		},
	}

	for _, lc := range lanes {
		t.Run(lc.name, func(t *testing.T) {
			clearProfileEnv(t)
			t.Setenv("HOME", t.TempDir()) // ambient ~/.aws etc. must never decide an outcome

			var gotPath, gotQuery string
			srv := newFakeOpenAIServer(t, &gotPath, &gotQuery)

			for k, v := range lc.env(t, srv.URL) {
				t.Setenv(k, v)
			}

			// Path 1: the frozen env-only lane.
			envClient, envErr := buildLLMClient()

			// Path 2: the profile lane, fed by the SAME environment through
			// the production env-overlay bridge.
			p, _, resolveErr := config.ResolveProfile(config.Config{Profiles: map[string]config.Profile{}}, nil, "")
			if resolveErr != nil {
				t.Fatalf("ResolveProfile: %v", resolveErr)
			}
			profClient, profErr := buildLLMClientForProfile(p)

			if (envErr == nil) != (profErr == nil) {
				t.Fatalf("lanes DIVERGED: env-only err = %v, profile err = %v", envErr, profErr)
			}
			if lc.wantConstruct && envErr != nil {
				t.Fatalf("both lanes failed but construction was expected: %v", envErr)
			}
			if !lc.wantConstruct && envErr == nil {
				t.Fatal("both lanes constructed but failure was expected")
			}

			if !lc.routeCheck || envErr != nil {
				return
			}

			complete := func(cl llmclient.LLMClient) (path, query string) {
				resp, err := cl.Complete(context.Background(), llmclient.Request{Model: lc.model, Prompt: "ping"})
				if err != nil {
					t.Fatalf("Complete: %v", err)
				}
				if resp.Text != "pong" {
					t.Fatalf("Text = %q, want pong", resp.Text)
				}
				return gotPath, gotQuery
			}
			envPath, envQuery := complete(envClient)
			profPath, profQuery := complete(profClient)
			if envPath != profPath {
				t.Errorf("route DIVERGED: env-only path %q, profile path %q", envPath, profPath)
			}
			if envQuery != profQuery {
				t.Errorf("route DIVERGED: env-only query %q, profile query %q", envQuery, profQuery)
			}
			if strings.Contains(lc.name, "azure") && !strings.Contains(profPath, "equiv-deployment") {
				t.Errorf("azure path %q does not carry the deployment", profPath)
			}
		})
	}
}

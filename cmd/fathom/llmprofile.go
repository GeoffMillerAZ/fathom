package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/bedrock"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/claudecli"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/codexcli"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/openai"
	"github.com/geoffmilleraz/fathom/internal/config"
)

// llmprofile.go — CF-3 (docs/plans/config-experience.md): a --profile-aware
// construction path that sits ALONGSIDE compile.go's original buildLLMClient
// and its six build*Client functions, which this file deliberately never
// touches. Those functions and compileModel() are exercised directly by 15
// pre-existing compile_test.go tests asserting fathom's env-only behavior;
// leaving them byte-for-byte unmodified is what keeps that coverage honest
// (rather than "trust the refactor preserved it").
//
// A config.Profile is always fully resolved BEFORE reaching this file
// (internal/config.ResolveProfile layers the env-overlay on top of any
// named/default profile, per the file's stated precedence: flags > env >
// project > user > defaults) — so every function here reads only p's
// fields, never os.Getenv directly, except through config.Profile's own
// ResolveAPIKey (which is intentionally lazy — see its doc comment).

// resolveActiveProfile loads the user and project config layers, merges
// them, and resolves the active Profile: flagProfile (a --profile value)
// wins, then the merged default_profile, then the bare env-overlay (which
// reproduces pre-profile fathom's behavior when no config file exists at
// all — see internal/config.ResolveProfile's doc comment).
func resolveActiveProfile(flagProfile string) (config.Profile, config.Provenance, error) {
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		return config.Profile{}, nil, err
	}
	return config.ResolveProfile(loaded.Config, loaded.Provenance, flagProfile)
}

// buildLLMClientForProfile constructs the LLM client for a resolved
// Profile — the six-lane switch buildLLMClient implements against raw env,
// mirrored here against Profile fields instead. See the file doc for why
// this is a separate implementation rather than a refactor of
// buildLLMClient.
func buildLLMClientForProfile(p config.Profile) (llmclient.LLMClient, error) {
	switch p.Provider {
	case "", config.ProviderAnthropic:
		return buildAnthropicClientProfile(p)
	case config.ProviderClaudeCLI:
		return buildClaudeCLIClientProfile(p)
	case config.ProviderOpenAI:
		return buildOpenAIClientProfile(p)
	case config.ProviderAzureOpenAI:
		return buildAzureOpenAIClientProfile(p)
	case config.ProviderBedrock:
		return buildBedrockClientProfile(p)
	case config.ProviderCodexCLI:
		return buildCodexCLIClientProfile(p)
	default:
		return nil, fmt.Errorf("unknown provider %q in profile (want one of \"anthropic\", \"claude-cli\", \"openai\", \"azure-openai\", \"bedrock\", \"codex-cli\")", p.Provider)
	}
}

// compileModelForProfile resolves the compile model the same way
// compileModel() does (FATHOM_COMPILE_MODEL first, the cheap-tier default
// last) but consults p.Model between them — an exact-model pin from a
// config-file profile, which env still beats per the stated precedence.
func compileModelForProfile(p config.Profile) string {
	if m := compileModel(); m != defaultCompileModel {
		return m // FATHOM_COMPILE_MODEL was set — env wins over the profile
	}
	if p.Model != "" {
		return p.Model
	}
	return defaultCompileModel
}

func buildAnthropicClientProfile(p config.Profile) (llmclient.LLMClient, error) {
	apiKey, err := p.ResolveAPIKey(context.Background())
	if err != nil {
		return nil, fmt.Errorf("resolve profile API key: %w", err)
	}
	cl, err := anthropic.New(anthropic.Config{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("construct Anthropic client: %w", err)
	}
	return cl, nil
}

func buildClaudeCLIClientProfile(p config.Profile) (llmclient.LLMClient, error) {
	cl, err := claudecli.New(claudecli.Config{Bin: p.Bin})
	if err != nil {
		return nil, fmt.Errorf("construct Claude CLI client: %w", err)
	}
	return cl, nil
}

func buildOpenAIClientProfile(p config.Profile) (llmclient.LLMClient, error) {
	apiKey, err := p.ResolveAPIKey(context.Background())
	if err != nil {
		return nil, fmt.Errorf("resolve profile API key: %w", err)
	}
	cl, err := openai.New(openai.Config{APIKey: apiKey, BaseURL: p.Endpoint})
	if err != nil {
		return nil, fmt.Errorf("construct OpenAI client: %w", err)
	}
	return cl, nil
}

func buildAzureOpenAIClientProfile(p config.Profile) (llmclient.LLMClient, error) {
	if p.Endpoint == "" {
		return nil, fmt.Errorf("profile provider=azure-openai requires endpoint (profile field, or AZURE_OPENAI_ENDPOINT via the env overlay) — the Azure resource base URL, e.g. https://<resource>.openai.azure.com or, for Azure Government, https://<resource>.openai.azure.us")
	}
	if p.Deployment == "" {
		return nil, fmt.Errorf("profile provider=azure-openai requires deployment (profile field, or AZURE_OPENAI_DEPLOYMENT via the env overlay)")
	}
	apiKey, err := p.ResolveAPIKey(context.Background())
	if err != nil {
		return nil, fmt.Errorf("resolve profile API key: %w", err)
	}
	cl, err := openai.New(openai.Config{
		APIKey:          apiKey,
		BaseURL:         p.Endpoint,
		AzureDeployment: p.Deployment,
		AzureAPIVersion: p.APIVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("construct Azure OpenAI client: %w", err)
	}
	return cl, nil
}

func buildBedrockClientProfile(p config.Profile) (llmclient.LLMClient, error) {
	// Fail-fast mirrors buildBedrockClient's own doc: Bedrock has no
	// meaningful default model, so a missing one must fail HERE, naming the
	// requirement, before any credential resolution. The check is on the
	// raw sources (profile field, env var) — NOT on compileModelForProfile's
	// result, which never returns "" (it falls back to the Anthropic-API
	// cheap-tier default, a model id that does not exist on Bedrock and
	// would otherwise construct fine, pass a Ping-based `fathom providers`
	// check — Ping never touches the model — and only fail opaquely on the
	// first real compile).
	if p.Model == "" && os.Getenv("FATHOM_COMPILE_MODEL") == "" {
		return nil, fmt.Errorf("profile provider=bedrock requires a model (profile field, or FATHOM_COMPILE_MODEL via the env overlay) set to a Bedrock model ID (e.g. \"anthropic.claude-3-5-sonnet-20241022-v2:0\") or inference-profile ID")
	}
	model := compileModelForProfile(p)
	cl, err := bedrock.New(bedrock.Config{
		Region:       p.Region,
		BaseURL:      p.Endpoint,
		DefaultModel: model,
	})
	if err != nil {
		return nil, fmt.Errorf("construct Bedrock client (model %q): %w", model, err)
	}
	return cl, nil
}

func buildCodexCLIClientProfile(p config.Profile) (llmclient.LLMClient, error) {
	cl, err := codexcli.New(codexcli.Config{Bin: p.Bin})
	if err != nil {
		return nil, fmt.Errorf("construct Codex CLI client: %w", err)
	}

	// codexcli.Client.Ping already bounds itself to its own internal
	// pingTimeout (15s, agentic-kit foundations/llmclient/codexcli/ping.go) —
	// this wrapper uses a longer bound than that (not a shorter one, unlike
	// buildCodexCLIClient's untouched 5s) so it is never the tighter
	// constraint under CPU contention (e.g. `go test -race ./...` running
	// many packages' subprocess-spawning tests concurrently, including
	// providers.go's own second Ping of this same lane via ProbeAll).
	pingCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := cl.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("codex-cli liveness check failed: the installed codex CLI may be silently degraded — undocumented config keys this adapter depends on (system-prompt mapping, ambient-context suppression) may have been dropped by a CLI upgrade, so fathom refuses to compile through it rather than risk a silent quality or cost regression: %w", err)
	}
	return cl, nil
}

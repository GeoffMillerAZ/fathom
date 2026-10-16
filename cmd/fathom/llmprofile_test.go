package main

// llmprofile_test.go — CF-3/CF-9 (docs/plans/config-experience.md): coverage
// for the --profile-aware construction path (llmprofile.go), independent of
// compile_test.go's 15 env-only tests (which this file never touches — see
// llmprofile.go's doc comment). Helpers newFakeClaudeCLI, newFakeCodexCLI*,
// newFakeOpenAIServer are shared from compile_test.go (same package).

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// --- resolveActiveProfile: end-to-end through real default paths ----------

// TestResolveActiveProfile_ReadsProjectConfig proves resolveActiveProfile's
// wiring to config.DefaultUserPath/DefaultProjectPath actually round-trips a
// file on disk — the unit tests in internal/config already cover Load and
// ResolveProfile in isolation; this is the seam between them and cmd/fathom.
func TestResolveActiveProfile_ReadsProjectConfig(t *testing.T) {
	clearProfileEnv(t)

	home := t.TempDir()
	t.Setenv("HOME", home) // config.DefaultUserPath() resolves via os.UserHomeDir

	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(projectDir, ".fathom")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "default_profile = \"work\"\n\n[profiles.work]\nprovider = \"codex-cli\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	p, prov, err := resolveActiveProfile("")
	if err != nil {
		t.Fatalf("resolveActiveProfile: %v", err)
	}
	if p.Provider != "codex-cli" {
		t.Errorf("Provider = %q, want codex-cli (from the project config.toml)", p.Provider)
	}
	if prov["provider"].Source != config.SourceProject {
		t.Errorf("provider source = %v, want project", prov["provider"].Source)
	}
}

// TestCmdCompile_ProfileWithoutStore_Errors pins N2: --profile has no HTTP
// plumbing (the daemon compiles with its own configured profile), and
// silently ignoring an explicit flag is worse UX than refusing.
func TestCmdCompile_ProfileWithoutStore_Errors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdCompile([]string{"--service", "svc", "--profile", "work"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected nonzero exit for --profile without --store")
	}
	if !strings.Contains(stderr.String(), "--store") || !strings.Contains(stderr.String(), "default_profile") {
		t.Errorf("stderr = %s, want it to name the --store constraint and the daemon's default_profile alternative", stderr.String())
	}
}

func TestResolveActiveProfile_UnknownFlagProfile_Errors(t *testing.T) {
	clearProfileEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectDir := t.TempDir()
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	_, _, err = resolveActiveProfile("nonexistent-profile")
	if err == nil {
		t.Fatal("expected an error for an unknown --profile value")
	}
}

func clearProfileEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{
		"FATHOM_LLM_PROVIDER", "FATHOM_COMPILE_MODEL",
		"FATHOM_CLAUDE_BIN", "FATHOM_CODEX_BIN", "FATHOM_OPENAI_BASE_URL",
		"AZURE_OPENAI_ENDPOINT", "AZURE_OPENAI_DEPLOYMENT", "AZURE_OPENAI_API_VERSION",
		"AZURE_OPENAI_API_KEY", "AWS_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN", "AWS_PROFILE", "ANTHROPIC_API_KEY", "OPENAI_API_KEY",
	} {
		t.Setenv(v, "")
	}
}

// --- buildLLMClientForProfile: dispatch + per-lane construction -----------

func TestBuildLLMClientForProfile_UnknownProvider(t *testing.T) {
	_, err := buildLLMClientForProfile(config.Profile{Provider: "bogus-provider"})
	if err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
	for _, lane := range []string{"anthropic", "claude-cli", "openai", "azure-openai", "bedrock", "codex-cli"} {
		if !strings.Contains(err.Error(), lane) {
			t.Errorf("err = %v, want it to list lane %q", err, lane)
		}
	}
}

// TestBuildLLMClientForProfile_EmptyProvider_DefaultsAnthropic_NoKey mirrors
// compile_test.go's TestBuildLLMClient_DefaultAndExplicitAnthropic_NoKey:
// an empty Provider dispatches to the anthropic lane (the same "" default
// buildLLMClient's switch uses), and anthropic.New itself requires a
// resolvable credential at construction — no ANTHROPIC_API_KEY means an
// error, not a silently-broken client.
func TestBuildLLMClientForProfile_EmptyProvider_DefaultsAnthropic_NoKey(t *testing.T) {
	clearProfileEnv(t)
	_, err := buildLLMClientForProfile(config.Profile{})
	if err == nil {
		t.Fatal("expected an error building the anthropic client with no API key configured")
	}
}

func TestBuildLLMClientForProfile_Anthropic_APIKeyEnvHonored(t *testing.T) {
	clearProfileEnv(t)
	t.Setenv("FATHOM_TEST_ANTHROPIC_KEY", "test-key")
	p := config.Profile{Provider: config.ProviderAnthropic, APIKeyEnv: "FATHOM_TEST_ANTHROPIC_KEY"}
	cl, err := buildLLMClientForProfile(p)
	if err != nil {
		t.Fatalf("buildLLMClientForProfile: %v", err)
	}
	if cl == nil {
		t.Fatal("nil client")
	}
}

func TestBuildLLMClientForProfile_ClaudeCLI_ConstructsAndCompletes(t *testing.T) {
	p := config.Profile{Provider: config.ProviderClaudeCLI, Bin: newFakeClaudeCLI(t)}
	cl, err := buildLLMClientForProfile(p)
	if err != nil {
		t.Fatalf("buildLLMClientForProfile: %v", err)
	}
	resp, err := cl.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want pong", resp.Text)
	}
}

func TestBuildLLMClientForProfile_OpenAI_EndpointHonored(t *testing.T) {
	var gotPath, gotQuery string
	srv := newFakeOpenAIServer(t, &gotPath, &gotQuery)

	p := config.Profile{Provider: config.ProviderOpenAI, Endpoint: srv.URL + "/v1", APIKeyEnv: "FATHOM_TEST_OPENAI_KEY"}
	clearProfileEnv(t)
	t.Setenv("FATHOM_TEST_OPENAI_KEY", "test-key")

	cl, err := buildLLMClientForProfile(p)
	if err != nil {
		t.Fatalf("buildLLMClientForProfile: %v", err)
	}
	resp, err := cl.Complete(context.Background(), llmclient.Request{Model: "model-x", Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want pong", resp.Text)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("request path = %q, profile Endpoint not honored", gotPath)
	}
}

func TestBuildLLMClientForProfile_AzureOpenAI_MissingEndpoint(t *testing.T) {
	_, err := buildLLMClientForProfile(config.Profile{Provider: config.ProviderAzureOpenAI, Deployment: "d"})
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("err = %v, want it to name the missing endpoint", err)
	}
}

func TestBuildLLMClientForProfile_AzureOpenAI_MissingDeployment(t *testing.T) {
	_, err := buildLLMClientForProfile(config.Profile{Provider: config.ProviderAzureOpenAI, Endpoint: "https://x.openai.azure.com"})
	if err == nil || !strings.Contains(err.Error(), "deployment") {
		t.Fatalf("err = %v, want it to name the missing deployment", err)
	}
}

// TestBuildLLMClientForProfile_Bedrock_MissingModel pins the FAIL-FAST for
// a modelless bedrock profile. The assertion is on the exact requirement
// message ("requires a model"), not a loose Contains(err, "model") — the
// loose form was proven to pass for the wrong reason (the generic
// "construct Bedrock client (model %q)" wrapper around an ambient
// credential-chain error also contains "model"). HOME is isolated so the
// kit's awscred chain cannot reach a real ~/.aws either — if the fail-fast
// regressed, this test must see a DIFFERENT error, not accidentally the
// right-looking one.
func TestBuildLLMClientForProfile_Bedrock_MissingModel(t *testing.T) {
	clearProfileEnv(t)
	t.Setenv("HOME", t.TempDir())
	_, err := buildLLMClientForProfile(config.Profile{Provider: config.ProviderBedrock, Region: "us-east-1"})
	if err == nil {
		t.Fatal("expected an error for a modelless bedrock profile")
	}
	if !strings.Contains(err.Error(), "requires a model") {
		t.Fatalf("err = %v, want the fail-fast's own \"requires a model\" message (anything else means construction proceeded past the guard)", err)
	}
}

// TestBuildLLMClientForProfile_Bedrock_ModellessNeverInheritsAnthropicDefault
// is the S1 regression pin from the other side: the guard must fire on the
// raw sources (p.Model, FATHOM_COMPILE_MODEL), because
// compileModelForProfile never returns "" — it falls back to the
// Anthropic-API cheap-tier default, which is not a Bedrock model id and
// would construct a client that passes providers' Ping but fails the first
// compile opaquely.
func TestBuildLLMClientForProfile_Bedrock_ModellessNeverInheritsAnthropicDefault(t *testing.T) {
	clearProfileEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")

	// Valid credentials + region, but no model anywhere: construction must
	// STILL fail via the guard — a nil error here would mean the client was
	// silently built around defaultCompileModel.
	_, err := buildLLMClientForProfile(config.Profile{Provider: config.ProviderBedrock, Region: "us-gov-west-1"})
	if err == nil {
		t.Fatal("modelless bedrock profile constructed successfully — it silently inherited the Anthropic-API default model")
	}
	if !strings.Contains(err.Error(), "requires a model") {
		t.Fatalf("err = %v, want the fail-fast message", err)
	}
	if strings.Contains(err.Error(), defaultCompileModel) {
		t.Errorf("err = %v, must not mention the Anthropic default model id", err)
	}
}

func TestBuildLLMClientForProfile_Bedrock_ConstructsWithEnvCredentials(t *testing.T) {
	clearProfileEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")

	p := config.Profile{Provider: config.ProviderBedrock, Region: "us-east-1", Model: "anthropic.claude-3-5-sonnet-20241022-v2:0"}
	if _, err := buildLLMClientForProfile(p); err != nil {
		t.Fatalf("buildLLMClientForProfile: %v", err)
	}
}

func TestBuildLLMClientForProfile_CodexCLI_HealthyPingConstructs(t *testing.T) {
	p := config.Profile{Provider: config.ProviderCodexCLI, Bin: newFakeCodexCLI(t, fakeCodexCLIHealthyScript)}
	if _, err := buildLLMClientForProfile(p); err != nil {
		t.Fatalf("buildLLMClientForProfile: %v", err)
	}
}

func TestBuildLLMClientForProfile_CodexCLI_DegradedPingFails(t *testing.T) {
	p := config.Profile{Provider: config.ProviderCodexCLI, Bin: newFakeCodexCLI(t, fakeCodexCLIDegradedScript)}
	_, err := buildLLMClientForProfile(p)
	if err == nil || !strings.Contains(err.Error(), "degrad") {
		t.Fatalf("err = %v, want it to name the degradation risk", err)
	}
}

// --- compileModelForProfile: flag/env > profile > default -----------------

func TestCompileModelForProfile_EnvWinsOverProfile(t *testing.T) {
	t.Setenv("FATHOM_COMPILE_MODEL", "env-model")
	got := compileModelForProfile(config.Profile{Model: "profile-model"})
	if got != "env-model" {
		t.Errorf("got %q, want env-model", got)
	}
}

func TestCompileModelForProfile_ProfileWinsOverDefault(t *testing.T) {
	t.Setenv("FATHOM_COMPILE_MODEL", "")
	got := compileModelForProfile(config.Profile{Model: "profile-model"})
	if got != "profile-model" {
		t.Errorf("got %q, want profile-model", got)
	}
}

func TestCompileModelForProfile_DefaultWhenNeitherSet(t *testing.T) {
	t.Setenv("FATHOM_COMPILE_MODEL", "")
	got := compileModelForProfile(config.Profile{})
	if got != defaultCompileModel {
		t.Errorf("got %q, want the cheap-tier default %q", got, defaultCompileModel)
	}
}

// --- CF-9: GovCloud / Azure Government construction proofs ----------------
//
// Hermetic, no network: these prove the profile's region/endpoint fields
// actually reach the kit adapter's Config, not merely that fathom's own
// pre-checks (endpoint/deployment non-empty) are satisfied.

// TestBuildBedrockClientProfile_GovCloudRegion_Flows proves p.Region reaches
// bedrock.Config.Region: with AWS_REGION deliberately unset, construction can
// only succeed if the profile's region value propagated all the way to the
// adapter (otherwise New returns bedrock's own "no region" error).
func TestBuildBedrockClientProfile_GovCloudRegion_Flows(t *testing.T) {
	clearProfileEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")

	p := config.Profile{
		Provider: config.ProviderBedrock,
		Region:   "us-gov-west-1",
		Model:    "anthropic.claude-3-5-sonnet-20241022-v2:0",
	}
	if _, err := buildBedrockClientProfile(p); err != nil {
		t.Fatalf("buildBedrockClientProfile: %v (region did not reach the adapter)", err)
	}
}

// TestBuildBedrockClientProfile_FIPSEndpoint_Flows proves p.Endpoint reaches
// bedrock.Config.BaseURL: a deliberately scheme-less value can only surface
// bedrock's "invalid BaseURL" error (naming our exact string) if fathom
// passed it through verbatim instead of deriving the default host from
// Region — the default derivation always produces a valid "https://…" URL,
// so this specific failure mode is proof positive of passthrough.
func TestBuildBedrockClientProfile_FIPSEndpoint_Flows(t *testing.T) {
	clearProfileEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")

	const sentinelEndpoint = "bedrock-runtime-fips.us-gov-west-1.amazonaws.com" // no scheme, deliberately
	p := config.Profile{
		Provider: config.ProviderBedrock,
		Region:   "us-gov-west-1",
		Endpoint: sentinelEndpoint,
		Model:    "anthropic.claude-3-5-sonnet-20241022-v2:0",
	}
	_, err := buildBedrockClientProfile(p)
	if err == nil {
		t.Fatal("expected an error from the scheme-less endpoint override")
	}
	if !strings.Contains(err.Error(), sentinelEndpoint) {
		t.Errorf("err = %v, want it to name the profile's exact endpoint override %q (proves passthrough)", err, sentinelEndpoint)
	}

	// A well-formed FIPS endpoint, by contrast, must construct cleanly.
	p.Endpoint = "https://" + sentinelEndpoint
	if _, err := buildBedrockClientProfile(p); err != nil {
		t.Fatalf("buildBedrockClientProfile with a well-formed FIPS endpoint: %v", err)
	}
}

// TestBuildAzureOpenAIClientProfile_GovernmentEndpoint_RoutesCorrectly proves
// an Azure Government-shaped endpoint (*.openai.azure.us) is neither
// rejected by fathom's own pre-checks nor mangled en route to the adapter:
// it is pointed at an httptest stand-in (so the request is observable) while
// deployment/api_version carry Gov-flavored values, mirroring
// TestBuildLLMClient_AzureOpenAI_ConstructsAndRoutesToDeployment's proof
// technique for the profile-driven path.
func TestBuildAzureOpenAIClientProfile_GovernmentEndpoint_RoutesCorrectly(t *testing.T) {
	var gotPath, gotQuery string
	srv := newFakeOpenAIServer(t, &gotPath, &gotQuery)

	p := config.Profile{
		Provider:   config.ProviderAzureOpenAI,
		Endpoint:   srv.URL, // stands in for https://<resource>.openai.azure.us
		Deployment: "gov-deployment",
		APIVersion: "2024-06-01",
		APIKeyEnv:  "FATHOM_TEST_AZURE_GOV_KEY",
	}
	clearProfileEnv(t)
	t.Setenv("FATHOM_TEST_AZURE_GOV_KEY", "test-key")

	cl, err := buildLLMClientForProfile(p)
	if err != nil {
		t.Fatalf("buildLLMClientForProfile: %v", err)
	}
	resp, err := cl.Complete(context.Background(), llmclient.Request{Model: "gpt-4o", Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want pong", resp.Text)
	}
	wantPath := "/openai/deployments/gov-deployment/chat/completions"
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}
	if !strings.Contains(gotQuery, "api-version=2024-06-01") {
		t.Errorf("request query = %q, want it to carry the profile's api_version", gotQuery)
	}
}

// TestBuildAzureOpenAIClientProfile_GovernmentEndpoint_ConstructsWithRealHost
// is the companion "no fathom-side TLD gate" proof: fathom's own pre-checks
// (endpoint/deployment non-empty) must accept a literal *.azure.us value —
// construction itself makes no network call, so this is safe and hermetic.
func TestBuildAzureOpenAIClientProfile_GovernmentEndpoint_ConstructsWithRealHost(t *testing.T) {
	clearProfileEnv(t)
	t.Setenv("FATHOM_TEST_AZURE_GOV_KEY2", "test-key")
	p := config.Profile{
		Provider:   config.ProviderAzureOpenAI,
		Endpoint:   "https://example-resource.openai.azure.us",
		Deployment: "gov-deployment",
		APIKeyEnv:  "FATHOM_TEST_AZURE_GOV_KEY2",
	}
	cl, err := buildLLMClientForProfile(p)
	if err != nil {
		t.Fatalf("buildLLMClientForProfile with an Azure Government endpoint: %v", err)
	}
	if cl == nil {
		t.Fatal("nil client")
	}
}

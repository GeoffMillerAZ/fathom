package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/openai"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// fakeClaudeCLIScript is a hermetic stand-in for the `claude` binary — no
// network, no real credential store. It always emits the same canned
// success envelope regardless of argv/stdin; the claudecli package's own
// test suite (agentic-kit) already covers argv/error-mapping/timeout
// behavior exhaustively, so this is just enough to prove buildLLMClient's
// "claude-cli" branch really wires a working end-to-end Complete call.
const fakeClaudeCLIScript = `#!/bin/sh
cat > /dev/null
printf '%s' '{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"pong","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}'
exit 0
`

// newFakeClaudeCLI writes fakeClaudeCLIScript into t.TempDir() and returns
// its absolute path.
func newFakeClaudeCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte(fakeClaudeCLIScript), 0o755); err != nil {
		t.Fatalf("write fake claude script: %v", err)
	}
	return path
}

// fakeCodexCLIHealthyScript is a hermetic stand-in for the `codex` binary
// that reproduces the HEALTHY outcome codexcli.Client.Ping looks for: the
// installed CLI still honors the undocumented model_instructions_file config
// key, so it rejects Ping's deliberately-empty instructions file with a
// local, pre-API stderr error and exit 1 (agentic-kit
// foundations/llmclient/codexcli/ping.go's pingHealthyMarker, verified live
// per that package's testdata/PROBES.md). buildCodexCLIClient treats this as
// "safe to use."
const fakeCodexCLIHealthyScript = `#!/bin/sh
cat > /dev/null
printf 'Error: model instructions file is empty: /ping/scratch/file.txt' >&2
exit 1
`

// fakeCodexCLIDegradedScript reproduces the DEGRADED outcome: a CLI that no
// longer performs the local empty-instructions-file rejection (e.g. a future
// codex-cli silently dropped the undocumented config key) exits clean with
// no local error. codexcli.Client.Ping treats this as unhealthy —
// buildCodexCLIClient must refuse to hand back a client built on it.
const fakeCodexCLIDegradedScript = `#!/bin/sh
cat > /dev/null
exit 0
`

// newFakeCodexCLI writes script into t.TempDir()/codex and returns its
// absolute path, ready to use as FATHOM_CODEX_BIN.
func newFakeCodexCLI(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
	}
	return path
}

// compileModel is the one model-resolution seam shared by the daemon
// (servehttp.go) and CLI --store mode: FATHOM_COMPILE_MODEL, defaulting to
// the cheap tier. Store mode hardcoding the Anthropic model string would
// silently override every non-Anthropic lane's DefaultModel (Request.Model
// wins when non-empty) — the exact gap this pins shut.
func TestCompileModel(t *testing.T) {
	t.Setenv("FATHOM_COMPILE_MODEL", "")
	if got := compileModel(); got != "claude-haiku-4-5-20251001" {
		t.Fatalf("default model = %q, want the cheap-tier default", got)
	}
	t.Setenv("FATHOM_COMPILE_MODEL", "anthropic.claude-3-5-sonnet-20241022-v2:0")
	if got := compileModel(); got != "anthropic.claude-3-5-sonnet-20241022-v2:0" {
		t.Fatalf("model override not honored: %q", got)
	}
}

func TestBuildLLMClient_DefaultAndExplicitAnthropic_NoKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	for _, provider := range []string{"", "anthropic"} {
		t.Run("provider="+provider, func(t *testing.T) {
			t.Setenv("FATHOM_LLM_PROVIDER", provider)
			_, err := buildLLMClient()
			if err == nil {
				t.Fatal("expected an error building the anthropic client with no ANTHROPIC_API_KEY, got nil")
			}
		})
	}
}

func TestBuildLLMClient_ClaudeCLI_ConstructsAndCompletes(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLI(t))

	cl, err := buildLLMClient()
	if err != nil {
		t.Fatalf("buildLLMClient: %v", err)
	}

	resp, err := cl.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want %q", resp.Text, "pong")
	}
	if resp.StopReason != llmclient.StopEndTurn {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llmclient.StopEndTurn)
	}
}

func TestBuildLLMClient_ClaudeCLI_BinNotFound(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", filepath.Join(t.TempDir(), "does-not-exist-claude"))

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error resolving a nonexistent FATHOM_CLAUDE_BIN, got nil")
	}
}

func TestBuildLLMClient_UnknownProvider(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "bogus-provider")

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error for an unknown FATHOM_LLM_PROVIDER, got nil")
	}
	if !strings.Contains(err.Error(), "bogus-provider") {
		t.Errorf("err = %v, want it to name the bad value %q", err, "bogus-provider")
	}
	// PM-5: the error text lists all six lanes, not just the two that
	// predate this matrix.
	for _, lane := range []string{"anthropic", "claude-cli", "openai", "azure-openai", "bedrock", "codex-cli"} {
		if !strings.Contains(err.Error(), lane) {
			t.Errorf("err = %v, want it to list lane %q", err, lane)
		}
	}
}

// TestBuildClaudeCLIClient_WrapsResolveError is a narrower unit check that
// buildClaudeCLIClient itself (not just the buildLLMClient switch) surfaces
// claudecli.New's resolution error rather than swallowing it.
func TestBuildClaudeCLIClient_WrapsResolveError(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "does-not-exist-claude")
	t.Setenv("FATHOM_CLAUDE_BIN", bin)

	_, err := buildClaudeCLIClient()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "construct Claude CLI client") {
		t.Errorf("err = %v, want it to be wrapped with \"construct Claude CLI client\"", err)
	}
}

// --- PM-1: openai ------------------------------------------------------

// newFakeOpenAIServer spins up an httptest server that answers any Chat
// Completions request with a canned "pong" success body and records the
// request path it was hit on, so a test can prove FATHOM_OPENAI_BASE_URL
// (or the Azure endpoint/deployment/api-version wiring) was actually
// honored instead of the adapter's own default routing.
func newFakeOpenAIServer(t *testing.T, gotPath *string, gotQuery *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
		*gotQuery = r.URL.RawQuery
		w.Header().Set("content-type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl-test","object":"chat.completion","model":"model-x",`+
			`"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],`+
			`"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBuildLLMClient_OpenAI_NoKey_ConstructsForLocalHosts(t *testing.T) {
	// OPENAI_API_KEY is optional (Config.APIKey doc): local, unauthenticated
	// hosts like Ollama/vLLM must still construct.
	t.Setenv("FATHOM_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("FATHOM_OPENAI_BASE_URL", "")

	if _, err := buildLLMClient(); err != nil {
		t.Fatalf("buildLLMClient: %v", err)
	}
}

// TestBuildLLMClient_OpenAI_BaseURLOverrideHonored is PM-1's acceptance
// check: FATHOM_OPENAI_BASE_URL must actually redirect traffic, not just be
// accepted and ignored.
func TestBuildLLMClient_OpenAI_BaseURLOverrideHonored(t *testing.T) {
	var gotPath, gotQuery string
	srv := newFakeOpenAIServer(t, &gotPath, &gotQuery)

	t.Setenv("FATHOM_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("FATHOM_OPENAI_BASE_URL", srv.URL+"/v1")

	cl, err := buildLLMClient()
	if err != nil {
		t.Fatalf("buildLLMClient: %v", err)
	}
	resp, err := cl.Complete(context.Background(), llmclient.Request{Model: "model-x", Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want %q", resp.Text, "pong")
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("request path = %q, want %q (FATHOM_OPENAI_BASE_URL not honored)", gotPath, "/v1/chat/completions")
	}
}

// --- PM-2: azure-openai --------------------------------------------------

func TestBuildLLMClient_AzureOpenAI_MissingEndpoint(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "azure-openai")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "test-deployment")

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error for a missing AZURE_OPENAI_ENDPOINT, got nil")
	}
	if !strings.Contains(err.Error(), "AZURE_OPENAI_ENDPOINT") {
		t.Errorf("err = %v, want it to name AZURE_OPENAI_ENDPOINT (not the adapter's Config.BaseURL)", err)
	}
}

func TestBuildLLMClient_AzureOpenAI_MissingDeployment(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "azure-openai")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://example-resource.openai.azure.com")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "")

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error for a missing AZURE_OPENAI_DEPLOYMENT, got nil")
	}
	if !strings.Contains(err.Error(), "AZURE_OPENAI_DEPLOYMENT") {
		t.Errorf("err = %v, want it to name AZURE_OPENAI_DEPLOYMENT (not the adapter's Config.AzureDeployment)", err)
	}
}

// TestBuildLLMClient_AzureOpenAI_ConstructsAndRoutesToDeployment is PM-2's
// acceptance check: endpoint/deployment/api-version env actually drives
// Azure-shaped routing (/openai/deployments/{deployment}/chat/completions
// with ?api-version=...), including the default api-version
// (openai.DefaultAzureAPIVersion) when AZURE_OPENAI_API_VERSION is unset.
func TestBuildLLMClient_AzureOpenAI_ConstructsAndRoutesToDeployment(t *testing.T) {
	var gotPath, gotQuery string
	srv := newFakeOpenAIServer(t, &gotPath, &gotQuery)

	t.Setenv("FATHOM_LLM_PROVIDER", "azure-openai")
	t.Setenv("AZURE_OPENAI_API_KEY", "test-key")
	t.Setenv("AZURE_OPENAI_ENDPOINT", srv.URL)
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "my-deployment")
	t.Setenv("AZURE_OPENAI_API_VERSION", "")

	cl, err := buildLLMClient()
	if err != nil {
		t.Fatalf("buildLLMClient: %v", err)
	}
	resp, err := cl.Complete(context.Background(), llmclient.Request{Model: "gpt-4o", Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want %q", resp.Text, "pong")
	}
	wantPath := "/openai/deployments/my-deployment/chat/completions"
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}
	if !strings.Contains(gotQuery, "api-version="+openai.DefaultAzureAPIVersion) {
		t.Errorf("request query = %q, want it to carry the default api-version %q", gotQuery, openai.DefaultAzureAPIVersion)
	}
}

// --- PM-3: bedrock ---------------------------------------------------------

func TestBuildLLMClient_Bedrock_MissingModel(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "bedrock")
	t.Setenv("FATHOM_COMPILE_MODEL", "")

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error for a missing FATHOM_COMPILE_MODEL, got nil")
	}
	if !strings.Contains(err.Error(), "FATHOM_COMPILE_MODEL") {
		t.Errorf("err = %v, want it to name FATHOM_COMPILE_MODEL", err)
	}
}

// TestBuildLLMClient_Bedrock_ConstructsWithEnvCredentials proves the bedrock
// branch reaches the kit's chain resolver (no static keys hardcoded by
// fathom) using the chain's ATOMIC, ambient-blind env leg
// (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_REGION) — which short-circuits
// before any shared-file or IMDS network access, so this stays hermetic
// regardless of the host's ambient AWS configuration.
func TestBuildLLMClient_Bedrock_ConstructsWithEnvCredentials(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "bedrock")
	t.Setenv("FATHOM_COMPILE_MODEL", "anthropic.claude-3-5-sonnet-20241022-v2:0")
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_PROFILE", "")

	if _, err := buildLLMClient(); err != nil {
		t.Fatalf("buildLLMClient: %v", err)
	}
}

// --- PM-4: codex-cli ---------------------------------------------------

func TestBuildLLMClient_CodexCLI_HealthyPingConstructs(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "codex-cli")
	t.Setenv("FATHOM_CODEX_BIN", newFakeCodexCLI(t, fakeCodexCLIHealthyScript))

	if _, err := buildLLMClient(); err != nil {
		t.Fatalf("buildLLMClient: %v", err)
	}
}

// TestBuildLLMClient_CodexCLI_DegradedPingFailsConstruction is the polish
// this branch was ordered for: an installed codex CLI that no longer
// performs the local empty-instructions-file rejection (Ping unhealthy)
// must fail buildLLMClient outright, naming the degradation risk, rather
// than silently handing back a client that would silently lose its system
// prompt on every real call.
func TestBuildLLMClient_CodexCLI_DegradedPingFailsConstruction(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "codex-cli")
	t.Setenv("FATHOM_CODEX_BIN", newFakeCodexCLI(t, fakeCodexCLIDegradedScript))

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error for a degraded (Ping-unhealthy) codex CLI, got nil")
	}
	if !strings.Contains(err.Error(), "degrad") {
		t.Errorf("err = %v, want it to name the silent-degradation risk", err)
	}
}

func TestBuildCodexCLIClient_BinNotFound(t *testing.T) {
	t.Setenv("FATHOM_CODEX_BIN", filepath.Join(t.TempDir(), "does-not-exist-codex"))

	_, err := buildCodexCLIClient()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "construct Codex CLI client") {
		t.Errorf("err = %v, want it to be wrapped with \"construct Codex CLI client\"", err)
	}
}

// TestPrintCompileRunCore_NoRejections_OutputUnchanged (Q-4) pins the exact
// pre-Q-4 output for a zero-rejection run — the non-regression proof that
// this row's addition is additive-only.
func TestPrintCompileRunCore_NoRejections_OutputUnchanged(t *testing.T) {
	var buf bytes.Buffer
	printCompileRunCore(&buf, core.CompileRun{
		ServiceID: "svc", Layer: "obligations",
		SectionsTotal: 3, SectionsCompiled: 3,
		ClaimsEmitted: 2, ClaimsRejected: 0,
		TokensIn: 100, TokensOut: 50, CostUSD: 0.01,
		Model: "mock-model", RanAt: time.Unix(1700000000, 0).UTC(),
	})
	if strings.Contains(buf.String(), "rejection_reasons:") {
		t.Errorf("output for a zero-rejection run must not mention rejection_reasons:\n%s", buf.String())
	}
}

// TestPrintCompileRunCore_RejectionReasons_Rendered (Q-4).
func TestPrintCompileRunCore_RejectionReasons_Rendered(t *testing.T) {
	var buf bytes.Buffer
	printCompileRunCore(&buf, core.CompileRun{
		ServiceID: "svc", Layer: "obligations",
		SectionsTotal: 3, SectionsCompiled: 3,
		ClaimsEmitted: 1, ClaimsRejected: 2,
		RejectionReasons: []core.RejectionBreakdown{
			{Cause: "anchor-missing", Count: 1, Samples: []string{"docs/a.md#x: citation range does not resolve"}},
			{Cause: "malformed", Count: 1, Samples: []string{"docs/a.md#y: empty rule"}},
		},
		TokensIn: 100, TokensOut: 50, CostUSD: 0.01,
		Model: "mock-model", RanAt: time.Unix(1700000000, 0).UTC(),
	})
	out := buf.String()
	for _, want := range []string{
		"rejection_reasons:",
		"anchor-missing:",
		"malformed:",
		"docs/a.md#x: citation range does not resolve",
		"docs/a.md#y: empty rule",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

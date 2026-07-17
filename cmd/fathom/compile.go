package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/bedrock"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/claudecli"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/codexcli"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/openai"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdCompile implements the "compile" subcommand: compile a semantic layer for
// a service. Runs against the daemon (HTTP mode, default) or a local store
// (--store direct mode).
//
// dry_run defaults true — spending requires --dry-run=false.
func cmdCompile(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("compile", stderr)
	service := fs.String("service", "", "service ID (required)")
	layer := fs.String("layer", "obligations", "layer to compile (must be defined in the semantic-layer manifest, schema/semantic/schema.cue; see ADR-016)")
	dryRun := fs.Bool("dry-run", true, "estimate cost without calling the LLM (default true; use --dry-run=false to spend)")
	storePath := fs.String("store", "", "DuckDB store path (direct mode; bypasses daemon)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	if _, err := semantic.LoadLayer(*layer); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *storePath != "" {
		return compileViaStore(*storePath, *service, *layer, *dryRun, stdout, stderr)
	}
	return compileViaHTTP(*url, *service, *layer, *dryRun, stdout, stderr)
}

// compileViaStore runs CompileLayer in-process against a local DuckDB store.
// F7 (docs/plans/findings-tracer.md FT-1): this path used to always call
// CompileObligations regardless of --layer, silently discarding a
// non-default --layer flag even after LoadLayer had validated it above.
func compileViaStore(storePath, service, layer string, dryRun bool, stdout, stderr io.Writer) int {
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", storePath, err)
		return 1
	}
	defer st.Close()

	conn := connector.NewLocalConnector()
	c := client.New(conn, st)
	defer c.Close()

	if !dryRun {
		llmCl, llmErr := buildLLMClient()
		if llmErr != nil {
			fmt.Fprintf(stderr, "error: %v\nSet ANTHROPIC_API_KEY, or set FATHOM_LLM_PROVIDER=claude-cli with a logged-in Claude Code CLI, or use --dry-run=true to estimate cost.\n", llmErr)
			return 1
		}
		c.WithLLM(llmCl, "claude-haiku-4-5-20251001", 0.001, 0.005)
	}

	ctx := context.Background()
	run, compErr := c.CompileLayer(ctx, service, layer, dryRun)
	if compErr != nil {
		fmt.Fprintf(stderr, "error: compile layer %q: %v\n", layer, compErr)
		return 1
	}

	printCompileRunCore(stdout, run)
	return 0
}

// compileViaHTTP calls the compile_layer MCP tool over HTTP on the daemon.
func compileViaHTTP(baseURL, service, layer string, dryRun bool, stdout, stderr io.Writer) int {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "compile_layer",
			"arguments": map[string]any{
				"service": service,
				"layer":   layer,
				"dry_run": dryRun,
			},
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: marshal request: %v\n", err)
		return 1
	}

	hc := &http.Client{Timeout: 120 * time.Second}
	resp, err := hc.Post(baseURL+"/rpc", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(stderr, "error: POST %s/rpc: %v\n", baseURL, err)
		return 1
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		fmt.Fprintf(stderr, "error: reading response: %v\n", err)
		return 1
	}

	inner, err := unwrapRPCResult(raw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	var payload map[string]any
	if err := json.Unmarshal(inner, &payload); err != nil {
		fmt.Fprintf(stderr, "error: parse compile_layer result: %v\n", err)
		return 1
	}
	printCompileRunMap(stdout, payload)
	return 0
}

// buildLLMClient constructs the llmclient.LLMClient the compile path uses,
// selecting the provider via FATHOM_LLM_PROVIDER — the single construction
// site both compileViaStore and serve-http call, so the provider choice is
// made once and consistently everywhere fathom compiles a semantic layer.
//
//   - unset or "anthropic" (the default): buildAnthropicClient — the
//     Anthropic Messages API, needs ANTHROPIC_API_KEY.
//   - "claude-cli": buildClaudeCLIClient — shells out to a locally
//     logged-in Claude Code CLI instead, so compiles ride a Claude Pro/Max
//     subscription's OAuth login rather than a metered key. FATHOM_CLAUDE_BIN
//     optionally overrides the resolved binary path (e.g. for a launchd
//     daemon whose PATH does not include wherever the CLI was installed).
//   - "openai": buildOpenAIClient — the OpenAI Chat Completions API (or any
//     OpenAI-compatible endpoint via FATHOM_OPENAI_BASE_URL), needs
//     OPENAI_API_KEY (optional for unauthenticated local hosts).
//   - "azure-openai": buildAzureOpenAIClient — the same OpenAI adapter in
//     Azure mode, needs AZURE_OPENAI_ENDPOINT + AZURE_OPENAI_DEPLOYMENT
//     (+ AZURE_OPENAI_API_KEY, AZURE_OPENAI_API_VERSION).
//   - "bedrock": buildBedrockClient — AWS Bedrock via the kit's full
//     credential-chain resolver (explicit → env → shared files, including
//     `aws sso login` sessions and credential_process → IMDSv2); no static
//     keys required. Needs FATHOM_COMPILE_MODEL set to a Bedrock model or
//     inference-profile ID.
//   - "codex-cli": buildCodexCLIClient — shells out to a locally logged-in
//     Codex CLI (ChatGPT plan OAuth). FATHOM_CODEX_BIN optionally overrides
//     the resolved binary path.
//
// Any other value is a typed configuration error, not a silent fallback.
//
// Cost-rate wiring (WithLLM's inRatePerKTok/outRatePerKTok) is unchanged
// across every subscription-backed lane (claude-cli, codex-cli): the
// reported cost_usd on a CompileRun is the same API-equivalent accounting
// every provider produces — it does NOT reflect actual marginal spend, which
// is $0 against a subscription's plan limits under those two lanes. See
// docs/use-cases/coding-agents.md §5 for the full per-lane cost story.
func buildLLMClient() (llmclient.LLMClient, error) {
	switch provider := os.Getenv("FATHOM_LLM_PROVIDER"); provider {
	case "", "anthropic":
		return buildAnthropicClient()
	case "claude-cli":
		return buildClaudeCLIClient()
	case "openai":
		return buildOpenAIClient()
	case "azure-openai":
		return buildAzureOpenAIClient()
	case "bedrock":
		return buildBedrockClient()
	case "codex-cli":
		return buildCodexCLIClient()
	default:
		return nil, fmt.Errorf("unknown FATHOM_LLM_PROVIDER %q (want one of \"anthropic\", \"claude-cli\", \"openai\", \"azure-openai\", \"bedrock\", \"codex-cli\")", provider)
	}
}

// buildAnthropicClient constructs an Anthropic LLM client from the environment.
// Returns an error when ANTHROPIC_API_KEY is unset.
func buildAnthropicClient() (llmclient.LLMClient, error) {
	cl, err := anthropic.New(anthropic.Config{})
	if err != nil {
		return nil, fmt.Errorf("construct Anthropic client: %w", err)
	}
	return cl, nil
}

// buildClaudeCLIClient constructs a Claude Code CLI LLM client from the
// environment. FATHOM_CLAUDE_BIN, when set, overrides the resolved `claude`
// binary path; otherwise it is resolved from PATH (see
// foundations/llmclient/claudecli's package doc). Returns an error when the
// binary cannot be resolved (e.g. not installed, or not on PATH and
// FATHOM_CLAUDE_BIN is unset).
func buildClaudeCLIClient() (llmclient.LLMClient, error) {
	cl, err := claudecli.New(claudecli.Config{Bin: os.Getenv("FATHOM_CLAUDE_BIN")})
	if err != nil {
		return nil, fmt.Errorf("construct Claude CLI client: %w", err)
	}
	return cl, nil
}

// buildOpenAIClient constructs a vanilla OpenAI (or OpenAI-compatible)
// client from the environment. OPENAI_API_KEY is optional — unset works
// against local, unauthenticated hosts (Ollama, vLLM), matching the
// adapter's own Config.APIKey doc. FATHOM_OPENAI_BASE_URL, when set,
// overrides the API base URL (e.g. "http://localhost:11434/v1" for Ollama);
// unset uses the adapter's default "https://api.openai.com/v1".
func buildOpenAIClient() (llmclient.LLMClient, error) {
	cl, err := openai.New(openai.Config{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: os.Getenv("FATHOM_OPENAI_BASE_URL"),
	})
	if err != nil {
		return nil, fmt.Errorf("construct OpenAI client: %w", err)
	}
	return cl, nil
}

// buildAzureOpenAIClient constructs the same OpenAI adapter in Azure mode
// from AZURE_OPENAI_API_KEY, AZURE_OPENAI_ENDPOINT (the Azure resource base
// URL), AZURE_OPENAI_DEPLOYMENT, and optional AZURE_OPENAI_API_VERSION
// (empty defers to the adapter's pinned GA default, openai.DefaultAzureAPIVersion).
//
// AZURE_OPENAI_ENDPOINT and AZURE_OPENAI_DEPLOYMENT are checked here, before
// calling openai.New, so a missing one fails with fathom's own env-var name
// in the error — openai.New's own validation names its Go Config fields
// (Config.BaseURL, Config.AzureDeployment), not the environment variables an
// operator actually sets, so pre-checking gives the operator something they
// can act on directly.
func buildAzureOpenAIClient() (llmclient.LLMClient, error) {
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("FATHOM_LLM_PROVIDER=azure-openai requires AZURE_OPENAI_ENDPOINT (the Azure resource base URL, e.g. https://<resource>.openai.azure.com) to be set")
	}
	deployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	if deployment == "" {
		return nil, fmt.Errorf("FATHOM_LLM_PROVIDER=azure-openai requires AZURE_OPENAI_DEPLOYMENT to be set")
	}

	cl, err := openai.New(openai.Config{
		APIKey:          os.Getenv("AZURE_OPENAI_API_KEY"),
		BaseURL:         endpoint,
		AzureDeployment: deployment,
		AzureAPIVersion: os.Getenv("AZURE_OPENAI_API_VERSION"),
	})
	if err != nil {
		return nil, fmt.Errorf("construct Azure OpenAI client: %w", err)
	}
	return cl, nil
}

// buildBedrockClient constructs an AWS Bedrock client. No fathom-specific
// AWS env is read: the kit's awscred chain resolves credentials itself
// (explicit → AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_SESSION_TOKEN →
// AWS_PROFILE/shared config+credentials files, including `aws sso login`
// sessions and credential_process → IMDSv2) and region (AWS_REGION, or the
// shared profile's region key) — no static keys are required from fathom.
//
// FATHOM_COMPILE_MODEL is fathom's own requirement, not the kit's: Bedrock
// has no meaningful default model (unlike the Anthropic/OpenAI APIs), so a
// missing FATHOM_COMPILE_MODEL fails here, before any credential resolution,
// naming the requirement directly rather than surfacing an opaque
// unknown-model error from the first Bedrock InvokeModel call.
func buildBedrockClient() (llmclient.LLMClient, error) {
	model := os.Getenv("FATHOM_COMPILE_MODEL")
	if model == "" {
		return nil, fmt.Errorf("FATHOM_LLM_PROVIDER=bedrock requires FATHOM_COMPILE_MODEL to be set to a Bedrock model ID (e.g. \"anthropic.claude-3-5-sonnet-20241022-v2:0\") or inference-profile ID")
	}

	cl, err := bedrock.New(bedrock.Config{DefaultModel: model})
	if err != nil {
		return nil, fmt.Errorf("construct Bedrock client (model %q): %w", model, err)
	}
	return cl, nil
}

// buildCodexCLIClient constructs a Codex CLI LLM client from the
// environment, mirroring buildClaudeCLIClient: FATHOM_CODEX_BIN, when set,
// overrides the resolved `codex` binary path; otherwise it is resolved from
// PATH (see foundations/llmclient/codexcli's package doc).
//
// Unlike claude-cli, this branch also Pings the constructed client before
// returning it. codexcli's Request.System mapping and ambient-context
// suppression ride undocumented codex-cli config keys (see the kit's
// codexcli.Config doc) that a future CLI upgrade could silently drop —
// silently losing the system prompt and reverting to the CLI's ~13k-token
// built-in persona on every call, with no error anywhere. Ping is the
// adapter's built-in liveness tripwire for exactly that risk: zero-token,
// local-only, ~300ms. Failing it here — at construction, before any compile
// runs — turns a silent quality/cost regression into a loud, actionable
// construction error naming the risk, instead of letting a fathom customer
// run a compile through a degraded adapter.
func buildCodexCLIClient() (llmclient.LLMClient, error) {
	cl, err := codexcli.New(codexcli.Config{Bin: os.Getenv("FATHOM_CODEX_BIN")})
	if err != nil {
		return nil, fmt.Errorf("construct Codex CLI client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cl.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("codex-cli liveness check failed: the installed codex CLI may be silently degraded — undocumented config keys this adapter depends on (system-prompt mapping, ambient-context suppression) may have been dropped by a CLI upgrade, so fathom refuses to compile through it rather than risk a silent quality or cost regression: %w", err)
	}
	return cl, nil
}

// printCompileRunCore formats a core.CompileRun for human consumption.
func printCompileRunCore(w io.Writer, run core.CompileRun) {
	dryMarker := ""
	if run.DryRun {
		dryMarker = " [DRY RUN — estimate only, no LLM calls, no claims written]"
	}
	fmt.Fprintf(w, "service:           %s\n", run.ServiceID)
	fmt.Fprintf(w, "layer:             %s\n", run.Layer)
	fmt.Fprintf(w, "dry_run:           %v%s\n", run.DryRun, dryMarker)
	fmt.Fprintf(w, "sections_total:    %d\n", run.SectionsTotal)
	fmt.Fprintf(w, "sections_compiled: %d\n", run.SectionsCompiled)
	if run.SectionsFailed > 0 {
		fmt.Fprintf(w, "sections_failed:   %d (malformed/truncated model responses — re-run compiles just these)\n", run.SectionsFailed)
	}
	if run.BudgetStopped {
		fmt.Fprintf(w, "budget_stopped:    true (%s — re-run to resume; the digest gate skips completed sections)\n", run.RefusalReason)
	}
	if run.Refused {
		fmt.Fprintf(w, "refused:           true (%s)\n", run.RefusalReason)
	}
	fmt.Fprintf(w, "claims_emitted:    %d\n", run.ClaimsEmitted)
	fmt.Fprintf(w, "claims_rejected:   %d\n", run.ClaimsRejected)
	fmt.Fprintf(w, "tokens_in:         %d\n", run.TokensIn)
	fmt.Fprintf(w, "tokens_out:        %d\n", run.TokensOut)
	fmt.Fprintf(w, "cost_usd:          %.6f\n", run.CostUSD)
	fmt.Fprintf(w, "duration_ms:       %d\n", run.DurationMS)
	if run.Model != "" {
		fmt.Fprintf(w, "model:             %s\n", run.Model)
	}
	if !run.RanAt.IsZero() {
		fmt.Fprintf(w, "ran_at:            %s\n", run.RanAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
}

func printCompileRunMap(w io.Writer, m map[string]any) {
	dryRun, _ := m["dry_run"].(bool)
	drMarker := ""
	if dryRun {
		drMarker = " [DRY RUN — estimate only, no LLM calls, no claims written]"
	}
	fmt.Fprintf(w, "service:           %v\n", m["service"])
	fmt.Fprintf(w, "layer:             %v\n", m["layer"])
	fmt.Fprintf(w, "dry_run:           %v%s\n", dryRun, drMarker)
	fmt.Fprintf(w, "sections_total:    %v\n", m["sections_total"])
	fmt.Fprintf(w, "sections_compiled: %v\n", m["sections_compiled"])
	if v, ok := m["sections_failed"]; ok && fmt.Sprint(v) != "0" && v != nil {
		fmt.Fprintf(w, "sections_failed:   %v\n", v)
	}
	if bs, _ := m["budget_stopped"].(bool); bs {
		fmt.Fprintf(w, "budget_stopped:    true (%v — re-run to resume; the digest gate skips completed sections)\n", m["refusal_reason"])
	}
	if rf, _ := m["refused"].(bool); rf {
		fmt.Fprintf(w, "refused:           true (%v)\n", m["refusal_reason"])
	}
	fmt.Fprintf(w, "claims_emitted:    %v\n", m["claims_emitted"])
	fmt.Fprintf(w, "claims_rejected:   %v\n", m["claims_rejected"])
	fmt.Fprintf(w, "tokens_in:         %v\n", m["tokens_in"])
	fmt.Fprintf(w, "tokens_out:        %v\n", m["tokens_out"])
	fmt.Fprintf(w, "cost_usd:          %v\n", m["cost_usd"])
	fmt.Fprintf(w, "duration_ms:       %v\n", m["duration_ms"])
	if model, _ := m["model"].(string); model != "" {
		fmt.Fprintf(w, "model:             %s\n", model)
	}
	if ranAt, _ := m["ran_at"].(string); ranAt != "" {
		fmt.Fprintf(w, "ran_at:            %s\n", ranAt)
	}
}

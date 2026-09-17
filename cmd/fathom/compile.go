package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	profile := fs.String("profile", "", "named LLM profile from ~/.fathom/config.toml or project .fathom/config.toml (direct --store mode only; HTTP mode uses the daemon's own configured profile — CF-3, docs/plans/config-experience.md); default: the config's default_profile, or the env-only lanes when no config file defines one")
	force := fs.Bool("force", false, "A2-1 (docs/plans/activation.md): override the idempotency guard and recompile even when the store's latest compile_run for (service, layer) already ran at the same corpus watermark and prompt identity (direct --store mode only). Logged into the new compile_runs row.")
	unsafeSkipEvalGate := fs.Bool("unsafe-skip-eval-gate", false, "D3-1 (docs/plans/expert-depth.md): override the eval-gate refusal and compile a layer with no passing eval/RESULTS.md record (direct --store mode only). Logged into the new compile_runs row.")
	resume := fs.Bool("continue", false, "O54 deliverable 3: resume a budget-stopped compile of this (service, layer) from the first not-yet-compiled section instead of restarting — skips the pre-flight refusal that otherwise fires whenever the REMAINING corpus alone still estimates over this SAME run's max_tokens_per_run budget (direct --store mode only). The recorded compile_runs row's resumed_from names the section.")
	maxTokensPerRun := fs.String("max-tokens-per-run", "", "wave O4 item 2: override the layer's own budgets.max_tokens_per_run (schema/semantic/schema.cue #Layer.budgets) for this compile only — a positive integer, or \"all\" for unlimited (direct --store mode only). Use this when a layer's default budget stops a compile part-way (budget_stopped, sections_compiled < sections_total) and the caller wants the WHOLE corpus compiled in one run — e.g. a scenariobench subject's own compile step, which otherwise serves root boilerplate for the sections the default budget never reached. Empty (default) leaves the layer's own configured budget untouched.")
	maxUSDPerRun := fs.String("max-usd-per-run", "", "wave O4 item 2: override the layer's own budgets.max_usd_per_run for this compile only — a positive number, or \"all\" for unlimited (direct --store mode only). Same override shape as --max-tokens-per-run, for the layer's dollar cap instead of its token cap.")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	// Advisory pre-check only — NEVER a rejection (LC-3, docs/plans/
	// layer-catalog.md, critic finding 3): this handler can only see CWD's
	// own .fathom/layers/, but the authoritative root is the SERVICE's
	// ingested path (ServiceMeta), which only the backend — compileViaStore's
	// client.CompileLayer, or the daemon behind compileViaHTTP — can resolve.
	// A hard rejection here false-negatives the cross-repo case (`fathom
	// compile --service B --layer <B's project pack>` run from repo A), the
	// exact bug class this slice closed at the MCP handler by deleting its
	// pre-check entirely. Unknown at CWD therefore warns and passes through;
	// the backend's own resolution error is the only rejection.
	validateRoot, rerr := os.Getwd()
	if rerr != nil {
		validateRoot = ""
	}
	if _, err := semantic.LoadLayerIn(validateRoot, *layer); err != nil {
		fmt.Fprintf(stderr, "warning: %v\nwarning: (checked against the current directory only — proceeding; the compile resolves layers against the service's own ingested root)\n", err)
	}
	// N2 (config-experience review): HTTP mode has no --profile plumbing —
	// the daemon compiles with ITS OWN configured profile
	// ([daemon].default_profile, read at daemon startup). Silently ignoring
	// an explicit flag would be worse than refusing, so refuse, naming the
	// constraint and both ways out.
	if *profile != "" && *storePath == "" {
		fmt.Fprintln(stderr, "error: --profile applies to direct --store mode only; in HTTP mode the daemon compiles with its own configured profile ([daemon].default_profile in ~/.fathom/config.toml, restart-to-apply — see docs/configuration.md). Pass --store for a local compile under this profile, or configure the daemon.")
		return 1
	}
	// A2-1/D3-1: both compile guards run against the local store the direct
	// --store path already has open; HTTP mode has no guard plumbing into the
	// daemon yet (docs/plans/activation.md scoping note). Silently ignoring
	// an explicit override flag would be worse than refusing outright.
	if (*force || *unsafeSkipEvalGate) && *storePath == "" {
		fmt.Fprintln(stderr, "error: --force and --unsafe-skip-eval-gate apply to direct --store mode only (the A2-1/D3-1 compile guards reason about the local store). Pass --store, or omit these flags for an unguarded HTTP-mode compile.")
		return 1
	}
	// --continue shares the same direct-store-only constraint (HTTP mode has
	// no guard/resume plumbing into the daemon).
	if *resume && *storePath == "" {
		fmt.Fprintln(stderr, "error: --continue applies to direct --store mode only. Pass --store.")
		return 1
	}
	// --max-tokens-per-run/--max-usd-per-run share the same direct-store-only
	// constraint (HTTP mode has no budget-override plumbing into the daemon
	// — the daemon compiles under its own configured layer budgets).
	if (*maxTokensPerRun != "" || *maxUSDPerRun != "") && *storePath == "" {
		fmt.Fprintln(stderr, "error: --max-tokens-per-run and --max-usd-per-run apply to direct --store mode only. Pass --store, or omit these flags for an HTTP-mode compile under the daemon's own configured layer budgets.")
		return 1
	}
	budget, berr := parseBudgetOverride(*maxTokensPerRun, *maxUSDPerRun)
	if berr != nil {
		fmt.Fprintf(stderr, "error: %v\n", berr)
		return 1
	}

	if *storePath != "" {
		return compileViaStore(*storePath, *service, *layer, *profile, *dryRun, *force, *unsafeSkipEvalGate, *resume, budget, stdout, stderr)
	}
	return compileViaHTTP(*url, *service, *layer, *dryRun, stdout, stderr)
}

// parseBudgetOverride turns --max-tokens-per-run/--max-usd-per-run's raw
// flag values into a *semantic.BudgetOverride: "" leaves that field nil (no
// override — the layer's own layer.cue budget stands), "all"
// (case-insensitive) maps to a pointer to 0 (enforceBudgets' and
// BudgetOverride's own "<=0 means unlimited" convention), and anything else
// parses as a plain number (strconv.Atoi for tokens, strconv.ParseFloat for
// USD) — a malformed value is a usage error naming the flag and the bad
// value, never a silently ignored override. Returns nil (not a zero-value
// BudgetOverride) when NEITHER flag was given, so compileViaStore's callers
// can tell "no override requested" (pass nil straight through) from "an
// override was requested, possibly to 0/unlimited".
func parseBudgetOverride(rawTokens, rawUSD string) (*semantic.BudgetOverride, error) {
	if rawTokens == "" && rawUSD == "" {
		return nil, nil
	}
	var out semantic.BudgetOverride
	if rawTokens != "" {
		if strings.EqualFold(rawTokens, "all") {
			zero := 0
			out.MaxTokensPerRun = &zero
		} else {
			n, err := strconv.Atoi(rawTokens)
			if err != nil {
				return nil, fmt.Errorf("--max-tokens-per-run %q: want a positive integer or \"all\": %w", rawTokens, err)
			}
			out.MaxTokensPerRun = &n
		}
	}
	if rawUSD != "" {
		if strings.EqualFold(rawUSD, "all") {
			zero := 0.0
			out.MaxUSDPerRun = &zero
		} else {
			f, err := strconv.ParseFloat(rawUSD, 64)
			if err != nil {
				return nil, fmt.Errorf("--max-usd-per-run %q: want a positive number or \"all\": %w", rawUSD, err)
			}
			out.MaxUSDPerRun = &f
		}
	}
	return &out, nil
}

// compileViaStore runs CompileLayer in-process against a local store.
// F7 (docs/plans/findings-tracer.md FT-1): this path used to always call
// CompileObligations regardless of --layer, silently discarding a
// non-default --layer flag even after LoadLayer had validated it above.
//
// A2-1/D3-1 (docs/plans/activation.md, docs/plans/expert-depth.md): before
// spending anything, two pre-flight guards run in order — the eval-gate
// (D3-1) refuses a layer with no passing eval/RESULTS.md record, then the
// idempotency guard (A2-1) refuses an unchanged-since-last-time recompile.
// Either can be overridden by its own flag; an override is recorded as a
// free-text note on the run's CompileRun.Escape field — the durable,
// queryable "logged into compile_runs" audit trail both flags' kill
// criteria (docs/vision/11-dial-to-eleven.md Phase D) count against. Both
// guards are pure pre-flight: a refusal never opens an LLM client and never
// writes a compile_runs row.
func compileViaStore(storePath, service, layer, profileFlag string, dryRun, force, unsafeSkipEvalGate, resume bool, budget *semantic.BudgetOverride, stdout, stderr io.Writer) int {
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", storePath, err)
		return 1
	}
	defer st.Close()

	conn := connector.NewLocalConnector()
	c := client.New(conn, st)
	defer c.Close()

	ctx := context.Background()
	escape := ""

	if !dryRun {
		// The service's actual ingested root (ServiceMeta) — the same
		// authoritative source CompileLayer itself resolves against, unlike
		// the CWD-only advisory check above.
		root := ""
		if meta, found, merr := st.GetServiceMeta(ctx, service); merr == nil && found {
			root = meta.Path
		}
		layerDef, lerr := semantic.LoadLayerIn(root, layer)
		if lerr != nil {
			fmt.Fprintf(stderr, "error: resolve layer %q: %v\n", layer, lerr)
			return 1
		}

		// D3-1: refuse a spending compile of an un-evaled layer.
		if !layerDef.Evaled {
			if !unsafeSkipEvalGate {
				fmt.Fprintf(stderr, "error: layer %q has no passing eval/RESULTS.md record (LayerDef.Evaled is false) — refusing to compile (docs/layers.md \"The gate\"). Pass --unsafe-skip-eval-gate to override; the override is logged into compile_runs.\n", layer)
				return 1
			}
			escape = fmt.Sprintf("unsafe-skip-eval-gate: layer %q has no passing eval/RESULTS.md record", layer)
			fmt.Fprintf(stderr, "warning: --unsafe-skip-eval-gate: compiling layer %q despite no passing eval/RESULTS.md record\n", layer)
		}

		// A2-1: refuse an idempotent recompile — same (service, layer) at the
		// same corpus watermark AND the same prompt identity as the latest
		// recorded compile_run. An empty watermark means freshness is
		// unknown (ServiceWatermark's contract) — never refuse on that, or
		// two unrelated first-ever compiles of an unknown-freshness corpus
		// would falsely look "unchanged".
		watermark, _ := c.ServiceWatermark(ctx, service)
		if promptIdentity, ierr := semantic.PromptIdentity(layerDef); ierr == nil && watermark != "" {
			runs, rerr := st.CompileRuns(ctx, service, layer)
			if rerr != nil {
				// Fail-open by design (a guard must not brick compiles on a
				// store read hiccup) — but LOUDLY (bounce finding 1: on the
				// pre-migration fleet store this error was the only signal
				// the guard had been skipped, and it was swallowed).
				fmt.Fprintf(stderr, "warning: idempotency guard skipped — could not read compile_runs for (%s, %s): %v\n", service, layer, rerr)
			}
			if rerr == nil && len(runs) > 0 {
				latest := runs[0] // newest-first
				// Only a CLEANLY COMPLETED run makes a recompile idempotent.
				// A refused, budget-stopped, or partially-failed run's own
				// documented remedy is "re-run to resume" (the digest gate
				// skips completed sections) — refusing those would force
				// --force on the honest path and pollute the escape counts
				// the kill criteria read (bounce finding 4).
				completedClean := !latest.Refused && !latest.BudgetStopped && latest.SectionsFailed == 0
				if completedClean && latest.CorpusWatermark == watermark && latest.PromptVersion == promptIdentity {
					switch {
					case resume:
						// O54 NIT: --continue is itself the "yes, proceed"
						// signal for a layer that turns out to already be
						// complete — no --force, no escape note. The compile
						// below finds nothing left to do (every section's
						// digest still matches) and says so plainly instead
						// of this guard refusing outright.
					case force:
						note := fmt.Sprintf("force: idempotent recompile of run %s (watermark=%q, prompt=%q)", latest.RunID, watermark, promptIdentity)
						if escape != "" {
							escape += "; " + note
						} else {
							escape = note
						}
						fmt.Fprintf(stderr, "warning: --force: recompiling despite an unchanged watermark/prompt identity (prior run %s)\n", latest.RunID)
					default:
						fmt.Fprintf(stderr, "error: refusing idempotent recompile: compile_run %s for (%s, %s) already ran at corpus watermark %q and prompt identity %q (ran_at=%s) — nothing has changed since. Pass --force to override (logged into compile_runs), or --continue to confirm there is nothing left to resume.\n",
							latest.RunID, service, layer, watermark, promptIdentity, latest.RanAt.UTC().Format(time.RFC3339))
						return 1
					}
				}
			}
		}

		p, _, profErr := resolveActiveProfile(profileFlag)
		if profErr != nil {
			fmt.Fprintf(stderr, "error: resolving --profile %q: %v\n", profileFlag, profErr)
			return 1
		}
		llmCl, llmErr := buildLLMClientForProfile(p)
		if llmErr != nil {
			fmt.Fprintf(stderr, "error: %v\nSet ANTHROPIC_API_KEY, or set FATHOM_LLM_PROVIDER=claude-cli with a logged-in Claude Code CLI, or configure a profile in ~/.fathom/config.toml, or use --dry-run=true to estimate cost.\n", llmErr)
			return 1
		}
		c.WithLLM(llmCl, compileModelForProfile(p), 0.001, 0.005)
	}

	var run core.CompileRun
	var compErr error
	// wave O4, item 2: a budget override (even to "all"/unlimited) always
	// goes through CompileLayerWithBudget, which is Resume-aware itself
	// (the same resume bool CompileLayerResume passes) — one call site
	// instead of a 2x2 branch on (resume, budget != nil).
	if budget != nil {
		run, compErr = c.CompileLayerWithBudget(ctx, service, layer, dryRun, escape, resume, budget)
	} else if resume {
		run, compErr = c.CompileLayerResume(ctx, service, layer, dryRun, escape)
	} else {
		run, compErr = c.CompileLayerWithEscape(ctx, service, layer, dryRun, escape)
	}
	if compErr != nil {
		fmt.Fprintf(stderr, "error: compile layer %q: %v\n", layer, compErr)
		return 1
	}

	// O54 NIT: --continue against a layer that turns out to already be
	// fully compiled (every section's digest still matches — ResumedFrom
	// stays "" exactly in that case, CompileInput.Resume's own doc
	// comment) prints one plain line instead of the full run dump; a
	// DryRun estimate never counts as "nothing to resume".
	if resume && !dryRun && !run.Refused && run.ResumedFrom == "" {
		fmt.Fprintf(stdout, "layer %s is complete: %d/%d sections compiled; nothing to resume\n", layer, run.SectionsTotal, run.SectionsTotal)
		return 0
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

// defaultCompileModel is the cheap-tier default compileModel() falls back
// to. Named (rather than a bare literal) so llmprofile.go's
// compileModelForProfile (CF-3, docs/plans/config-experience.md) can detect
// "FATHOM_COMPILE_MODEL was actually set" without re-reading the env var
// under a second name — compileModel() itself is otherwise untouched.
const defaultCompileModel = "claude-haiku-4-5-20251001"

// compileModel resolves the compile model from FATHOM_COMPILE_MODEL,
// defaulting to the cheap tier (obligations compiles are cents-scale by
// design, ADR-016 §6). Shared by CLI --store mode and the serve-http daemon
// — Request.Model wins over any adapter's DefaultModel when non-empty, so a
// hardcoded string here would silently override every non-Anthropic lane.
func compileModel() string {
	if m := os.Getenv("FATHOM_COMPILE_MODEL"); m != "" {
		return m
	}
	return defaultCompileModel
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
		fmt.Fprintf(w, "budget_stopped:    true (%s — re-run to resume; the digest gate skips completed sections. If the REMAINING corpus alone still exceeds this same budget, a plain re-run refuses at the pre-flight gate — pass --continue to keep making progress without restarting: `fathom compile --service %s --layer %s --dry-run=false --continue`)\n", run.RefusalReason, run.ServiceID, run.Layer)
	}
	if run.Refused {
		fmt.Fprintf(w, "refused:           true (%s)\n", run.RefusalReason)
	}
	if run.Escape != "" {
		fmt.Fprintf(w, "escape:            %s\n", run.Escape)
	}
	if run.ResumedFrom != "" {
		fmt.Fprintf(w, "resumed_from:      %s\n", run.ResumedFrom)
	}
	if len(run.UncompiledScopes) > 0 {
		fmt.Fprintf(w, "uncompiled_scopes: %s\n", strings.Join(run.UncompiledScopes, ", "))
	}
	fmt.Fprintf(w, "claims_emitted:    %d\n", run.ClaimsEmitted)
	fmt.Fprintf(w, "claims_rejected:   %d\n", run.ClaimsRejected)
	// Q-4 (docs/plans/expert-suite-q.md, O19's fix): a run with zero
	// rejections prints nothing new here — byte-for-byte identical to
	// before this row.
	if len(run.RejectionReasons) > 0 {
		fmt.Fprintf(w, "rejection_reasons:\n")
		for _, rb := range run.RejectionReasons {
			fmt.Fprintf(w, "  %-20s %d\n", rb.Cause+":", rb.Count)
			for _, sample := range rb.Samples {
				fmt.Fprintf(w, "    - %s\n", sample)
			}
		}
	}
	if run.DryRun && run.EstTokensOutHigh > 0 {
		// O8: never a confident point before a run — input is a bytes/4
		// heuristic (observed running ~3x low), output is a bracketed
		// range, and cost/duration print as ranges or not at all.
		fmt.Fprintf(w, "tokens_in:         ~%d (estimate: bytes/4 heuristic — real usage has run ~3x higher)\n", run.TokensIn)
		fmt.Fprintf(w, "tokens_out:        unknown before a run — est. %d-%d (input x2-x8, layer-dependent)\n", run.EstTokensOutLow, run.EstTokensOutHigh)
		fmt.Fprintf(w, "cost_usd:          est. %.2f-%.2f (a range, never a point — O8)\n", run.EstCostLowUSD, run.EstCostHighUSD)
		fmt.Fprintf(w, "duration:          est. %.0f-%.0f min (at ~100 output tok/s observed)\n", float64(run.EstTokensOutLow)/6000.0, float64(run.EstTokensOutHigh)/6000.0)
	} else {
		fmt.Fprintf(w, "tokens_in:         %d\n", run.TokensIn)
		fmt.Fprintf(w, "tokens_out:        %d\n", run.TokensOut)
		fmt.Fprintf(w, "cost_usd:          %.6f\n", run.CostUSD)
		fmt.Fprintf(w, "duration_ms:       %d\n", run.DurationMS)
	}
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

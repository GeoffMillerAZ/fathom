package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
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
		llmCl, llmErr := buildAnthropicClient()
		if llmErr != nil {
			fmt.Fprintf(stderr, "error: %v\nSet ANTHROPIC_API_KEY or use --dry-run=true to estimate cost.\n", llmErr)
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

// buildAnthropicClient constructs an Anthropic LLM client from the environment.
// Returns an error when ANTHROPIC_API_KEY is unset.
func buildAnthropicClient() (llmclient.LLMClient, error) {
	cl, err := anthropic.New(anthropic.Config{})
	if err != nil {
		return nil, fmt.Errorf("construct Anthropic client: %w", err)
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

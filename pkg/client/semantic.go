package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// WithLLM attaches an LLM client for semantic-layer compilation. model is
// recorded in every CompileRun provenance row; inRatePerKTok /
// outRatePerKTok are the USD-per-1k-token rates for the cost ledger (zero
// rates record honest zeros). Returns c for chaining.
func (c *Client) WithLLM(llm llmclient.LLMClient, model string, inRatePerKTok, outRatePerKTok float64) *Client {
	c.llm = llm
	c.llmModel = model
	c.llmInRate = inRatePerKTok
	c.llmOutRate = outRatePerKTok
	return c
}

// Obligations returns the obligations governing file for service, capped at
// maxRules (≤ semantic.MaxObligations; ≤0 means the default cap).
func (c *Client) Obligations(ctx context.Context, service, file string, maxRules int) (semantic.Obligations, error) {
	return semantic.ObligationsFor(ctx, c.st, service, file, maxRules)
}

// ObligationsIndexStale reports whether the index's fingerprint for file
// differs from the file's current content on disk. It is best-effort: true
// only when both the disk file and the stored fingerprint exist AND they
// disagree. Any error (disk, store, missing meta) returns false — the caller
// should treat the index as authoritative when freshness cannot be determined.
//
// Fingerprint formula matches the fusion pipeline (Pass 3) exactly via
// fusion.FingerprintHash — the epoch-salted content hash.
func (c *Client) ObligationsIndexStale(ctx context.Context, service, file string) bool {
	// Resolve disk path: absolute passthrough; otherwise join with meta root.
	diskPath := file
	if !filepath.IsAbs(diskPath) {
		meta, found, err := c.st.GetServiceMeta(ctx, service)
		if err != nil || !found || meta.Path == "" {
			return false
		}
		diskPath = filepath.Join(meta.Path, file)
	}

	data, err := os.ReadFile(diskPath)
	if err != nil {
		return false
	}
	diskHash := fusion.FingerprintHash(data)

	stored, found, err := c.st.Fingerprint(ctx, service, file)
	if err != nil || !found {
		return false
	}
	return diskHash != stored
}

// CompileObligations runs the obligations-layer compile for service. When
// dryRun is true it estimates sections/tokens without calling the LLM or
// writing claims. When dryRun is false and no LLM client is configured it
// returns an error directing the caller to configure ANTHROPIC_API_KEY or run
// with dry_run:true.
func (c *Client) CompileObligations(ctx context.Context, service string, dryRun bool) (core.CompileRun, error) {
	if !dryRun && c.llm == nil {
		return core.CompileRun{}, fmt.Errorf("client: no LLM configured for CompileObligations (set ANTHROPIC_API_KEY and call WithLLM, or use dry_run:true to estimate cost)")
	}

	// Resolve corpus: all source files filtered to the obligations include list.
	files, err := c.Sources(ctx, service)
	if err != nil {
		return core.CompileRun{}, fmt.Errorf("client.CompileObligations: fetch sources: %w", err)
	}
	corpus := semantic.SelectCorpus(files, semantic.ObligationsInclude)

	// Collect distinct sorted SourceRefs from the live index for scope verification.
	obs, err := c.st.QueryObservations(ctx, service, "")
	if err != nil {
		return core.CompileRun{}, fmt.Errorf("client.CompileObligations: list indexed files: %w", err)
	}
	seen := make(map[string]struct{}, len(obs))
	for _, o := range obs {
		if o.SourceRef != "" {
			seen[o.SourceRef] = struct{}{}
		}
	}
	indexedFiles := make([]string, 0, len(seen))
	for f := range seen {
		indexedFiles = append(indexedFiles, f)
	}
	sort.Strings(indexedFiles)

	// Current corpus watermark for provenance.
	watermark, _ := c.ServiceWatermark(ctx, service)

	comp := semantic.Compiler{
		St:             c.st,
		LLM:            c.llm,
		Redactor:       llmsec.NewRedactor(llmsec.DefaultPosture()),
		Model:          c.llmModel,
		InRatePerKTok:  c.llmInRate,
		OutRatePerKTok: c.llmOutRate,
	}
	return comp.Compile(ctx, semantic.CompileInput{
		ServiceID:    service,
		Docs:         corpus,
		IndexedFiles: indexedFiles,
		Watermark:    watermark,
		DryRun:       dryRun,
	})
}

// SweepClaims runs the token-free post-ingest citation sweep for service
// (ADR-016 §4.3). It re-hashes every served claim's citations against the
// current corpus on disk and flips status on drift.
func (c *Client) SweepClaims(ctx context.Context, service string) (semantic.SweepReport, error) {
	// Determine the root path from service_meta.
	root := ""
	if meta, found, err := c.st.GetServiceMeta(ctx, service); err == nil && found {
		root = meta.Path
	}

	fetch := func(path string) ([]byte, bool) {
		diskPath := path
		if !filepath.IsAbs(path) && root != "" {
			diskPath = filepath.Join(root, path)
		}
		data, err := os.ReadFile(diskPath)
		if err != nil {
			return nil, false
		}
		return data, true
	}

	return semantic.Sweep(ctx, c.st, service, fetch)
}

// ClaimsSummary returns per-layer claim counts by status for service, plus
// the most recent CompileRun's ran_at, cost_usd, and model when available.
// Returns nil when no claims or compile runs exist.
func (c *Client) ClaimsSummary(ctx context.Context, service string) (map[string]any, error) {
	claims, err := c.st.ClaimsForService(ctx, service, "")
	if err != nil {
		return nil, fmt.Errorf("client.ClaimsSummary: claims: %w", err)
	}
	if len(claims) == 0 {
		return nil, nil
	}

	// Count by layer + status.
	type layerKey struct{ layer, status string }
	counts := map[layerKey]int{}
	layers := map[string]struct{}{}
	for _, cl := range claims {
		k := layerKey{layer: cl.Layer, status: string(cl.Status)}
		counts[k]++
		layers[cl.Layer] = struct{}{}
	}

	layerMap := map[string]any{}
	for layer := range layers {
		statusMap := map[string]any{}
		for _, s := range []string{"fresh", "stale", "contradicted", "superseded", "refuted"} {
			if n := counts[layerKey{layer: layer, status: s}]; n > 0 {
				statusMap[s] = n
			}
		}
		layerMap[layer] = statusMap
	}

	// Attach last CompileRun if any.
	runs, err := c.st.CompileRuns(ctx, service, "")
	if err == nil && len(runs) > 0 {
		last := runs[0]
		layerMap["_last_compile"] = map[string]any{
			"ran_at":   last.RanAt.UTC().Format("2006-01-02T15:04:05Z"),
			"cost_usd": last.CostUSD,
			"model":    last.Model,
			"layer":    last.Layer,
			"dry_run":  last.DryRun,
		}
	}

	return layerMap, nil
}

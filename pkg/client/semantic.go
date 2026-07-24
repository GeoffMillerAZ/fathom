package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

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

// CompileLayer runs the compile for the named manifest layer against
// service, generalizing what was CompileObligations to any layer defined in
// schema/semantic/schema.cue (FT-1: findings-tracer.md — F7 fixed here, the
// M1-3 compile_layer/compile plumbing validated the requested layer name via
// LoadLayer but then discarded it and always ran the obligations compile).
// When dryRun is true it estimates sections/tokens without calling the LLM or
// writing claims. When dryRun is false and no LLM client is configured it
// returns an error directing the caller to configure ANTHROPIC_API_KEY or run
// with dry_run:true.
func (c *Client) CompileLayer(ctx context.Context, service, layerName string, dryRun bool) (core.CompileRun, error) {
	return c.CompileLayerWithEscape(ctx, service, layerName, dryRun, "")
}

// CompileLayerWithEscape is CompileLayer with an audit note attached: when a
// caller's own pre-flight guard (the idempotency guard's --force, A2-1, or
// the eval-gate's --unsafe-skip-eval-gate, D3-1 — both cmd/fathom compile.go)
// let a would-be-refused compile proceed anyway, escape names why. It flows
// into the recorded CompileRun.Escape field — the durable "logged into
// compile_runs" audit trail both flags' kill criteria count against. Pass ""
// for a normal run (CompileLayer's contract, unchanged).
func (c *Client) CompileLayerWithEscape(ctx context.Context, service, layerName string, dryRun bool, escape string) (core.CompileRun, error) {
	if !dryRun && c.llm == nil {
		return core.CompileRun{}, fmt.Errorf("client: no LLM configured for CompileLayer (set ANTHROPIC_API_KEY and call WithLLM, or use dry_run:true to estimate cost)")
	}

	// Resolve the layer definition from the union of embedded (built-in)
	// packs and the SERVICE's own project-local .fathom/layers/ (LC-3,
	// docs/plans/layer-catalog.md): its corpus include list, kind, tier, and
	// budgets are the single authoritative source — no hand-synced Go mirror
	// exists anymore. root resolution mirrors SweepClaims below: service_meta
	// is the only place a service's on-disk root is known, and an
	// unresolvable root (a service ingested without meta, e.g. some
	// synthetic test corpora) falls back to embedded-only — a project pack
	// simply isn't reachable for that service, not a hard error.
	root := ""
	if meta, found, merr := c.st.GetServiceMeta(ctx, service); merr == nil && found {
		root = meta.Path
	}
	layer, err := semantic.LoadLayerIn(root, layerName)
	if err != nil {
		return core.CompileRun{}, fmt.Errorf("client.CompileLayer: load layer manifest: %w", err)
	}

	// Resolve corpus: all source files filtered to the layer's include list.
	files, err := c.Sources(ctx, service)
	if err != nil {
		return core.CompileRun{}, fmt.Errorf("client.CompileLayer: fetch sources: %w", err)
	}
	corpus := semantic.SelectCorpus(files, layer.CorpusInclude, layer.CorpusExclude)

	// Collect distinct sorted SourceRefs from the live index for scope verification.
	obs, err := c.st.QueryObservations(ctx, service, "")
	if err != nil {
		return core.CompileRun{}, fmt.Errorf("client.CompileLayer: list indexed files: %w", err)
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
		Layer:          layer,
		InRatePerKTok:  c.llmInRate,
		OutRatePerKTok: c.llmOutRate,
	}
	return comp.Compile(ctx, semantic.CompileInput{
		ServiceID:    service,
		Docs:         corpus,
		IndexedFiles: indexedFiles,
		Watermark:    watermark,
		DryRun:       dryRun,
		Escape:       escape,
	})
}

// CompileObligations is a thin wrapper over CompileLayer("obligations"),
// kept for existing callers (FT-1).
func (c *Client) CompileObligations(ctx context.Context, service string, dryRun bool) (core.CompileRun, error) {
	return c.CompileLayer(ctx, service, semantic.LayerObligations, dryRun)
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

	// now: nil defers to Sweep's real-time.Now fallback — pkg/client wires no
	// clock here (mirrors CompileLayer above, which also leaves Compiler.Now
	// unset); the injected-clock knob exists for internal/semantic's own
	// deterministic tests.
	return semantic.Sweep(ctx, c.st, service, fetch, nil)
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

// ClaimFeedback appends a human/machine verdict to claim_events for claimID
// of service — the write path behind the obligations tool's feedback action
// (D2, docs/plans/expert-depth.md): "the already-hot channel, NOT a new
// tool." action is "refute" or "corroborate", mirroring `fathom claim
// refute|corroborate` (cmd/fathom/claim.go) exactly: refute appends
// ClaimEvent{Verdict:"refuted"} AND flips the claim's status to
// core.ClaimRefuted — suppression is refutation with a note; corroborate
// appends ClaimEvent{Verdict:"corroborated"} and NEVER changes status or
// tier — corroboration is provenance, not evidence-laundering (the doctrine
// core.ClaimRefuted's doc comment states). Returns an error for an unknown
// action or a claim_id that does not exist for service — no dangling events
// for a typo'd ID nothing ever serves.
func (c *Client) ClaimFeedback(ctx context.Context, service, claimID, action, note, author string) error {
	var verdict string
	var newStatus core.ClaimStatus
	switch action {
	case "refute":
		verdict, newStatus = "refuted", core.ClaimRefuted
	case "corroborate":
		verdict = "corroborated"
	default:
		return fmt.Errorf("client.ClaimFeedback: unknown action %q (want refute or corroborate)", action)
	}

	claims, err := c.st.ClaimsForService(ctx, service, "")
	if err != nil {
		return fmt.Errorf("client.ClaimFeedback: load claims: %w", err)
	}
	found := false
	for _, cl := range claims {
		if cl.ClaimID == claimID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("client.ClaimFeedback: claim %q not found for service %q", claimID, service)
	}

	ev := core.ClaimEvent{ClaimID: claimID, Verdict: verdict, Author: author, Note: note, At: time.Now().UTC()}
	if err := c.st.AppendClaimEvent(ctx, ev); err != nil {
		return fmt.Errorf("client.ClaimFeedback: append event: %w", err)
	}
	if newStatus != "" {
		if err := c.st.SetClaimStatus(ctx, service, claimID, newStatus); err != nil {
			return fmt.Errorf("client.ClaimFeedback: set status: %w", err)
		}
	}
	return nil
}

// PutClaims upserts semantic claims and their links directly, bypassing
// layer compilation — the review-verb findings bridge (C2-1, docs/plans/
// review-verb.md; internal/analysis/review.AttachClaims builds the
// content-addressed claims this persists) uses this to write review
// findings onto the "findings" layer. Durability rides the store's own
// upsert (PutClaims' "status = CASE WHEN ... = 'refuted' THEN ... ELSE
// excluded.status END"): a claim already refuted stays refuted no matter
// how many times an unchanged finding re-emits the same ClaimID.
func (c *Client) PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error {
	return c.st.PutClaims(ctx, claims, links)
}

// ClaimsForService is a thin passthrough to the store's read path — for
// callers (like review_diff's findings bridge) that need to read back
// current claim status without going through a specific layer's serving
// helper (e.g. semantic.ObligationsFor). layer "" returns every layer.
func (c *Client) ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error) {
	return c.st.ClaimsForService(ctx, serviceID, layer)
}

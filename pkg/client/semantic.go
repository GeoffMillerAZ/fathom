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

// RootObligations returns "what governs this repository" (RC-22): the
// served claims whose source is a root-level governing doc or whose scope
// glob is unanchored/wildcard-rooted, capped at maxRules — see
// semantic.RootObligationsFor.
func (c *Client) RootObligations(ctx context.Context, service string, maxRules int) (semantic.Obligations, error) {
	return semantic.RootObligationsFor(ctx, c.st, service, maxRules)
}

// ObligationsLayer is Obligations' VB-9 sibling (ADR-035 decision 2):
// layer selects rules/claims/all — see semantic.ObligationsForLayer and
// semantic.ObligationLayerFilter.
func (c *Client) ObligationsLayer(ctx context.Context, service, file string, maxRules int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	return semantic.ObligationsForLayer(ctx, c.st, service, file, maxRules, layer)
}

// RootObligationsLayer is RootObligations' VB-9 sibling — see
// semantic.RootObligationsForLayer.
func (c *Client) RootObligationsLayer(ctx context.Context, service string, maxRules int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	return semantic.RootObligationsForLayer(ctx, c.st, service, maxRules, layer)
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

// CompileLayerWithBudget is CompileLayerWithEscape/CompileLayerResume with
// an explicit per-run budget override (wave O4, item 2; ADR-037 addendum):
// budget's non-nil fields REPLACE the resolved layer's own
// Budgets.MaxTokensPerRun/MaxUSDPerRun for this call only — see
// semantic.BudgetOverride's own doc comment for the "0/negative means
// unlimited" convention. Pass budget == nil for the unmodified behavior
// (identical to calling CompileLayerWithEscape/CompileLayerResume
// directly); cmd/fathom compile.go's --max-tokens-per-run/--max-usd-per-run
// flags are this method's only caller today, letting an operator (or a
// scenariobench subject's own compile step, internal/scenariobench/
// subject.go's compileLayers) request "all" of a corpus in one run instead
// of accepting whatever budget_stopped point the layer's default budget
// produces — instrument truth over the compiled layer's own coverage, not
// a silent partial compile masquerading as complete.
func (c *Client) CompileLayerWithBudget(ctx context.Context, service, layerName string, dryRun bool, escape string, resume bool, budget *semantic.BudgetOverride) (core.CompileRun, error) {
	return c.compileLayer(ctx, service, layerName, dryRun, escape, resume, budget)
}

// CompileLayerWithEscape is CompileLayer with an audit note attached: when a
// caller's own pre-flight guard (the idempotency guard's --force, A2-1, or
// the eval-gate's --unsafe-skip-eval-gate, D3-1 — both cmd/fathom compile.go)
// let a would-be-refused compile proceed anyway, escape names why. It flows
// into the recorded CompileRun.Escape field — the durable "logged into
// compile_runs" audit trail both flags' kill criteria count against. Pass ""
// for a normal run (CompileLayer's contract, unchanged).
func (c *Client) CompileLayerWithEscape(ctx context.Context, service, layerName string, dryRun bool, escape string) (core.CompileRun, error) {
	return c.compileLayer(ctx, service, layerName, dryRun, escape, false, nil)
}

// CompileLayerResume is O54 deliverable 3 (`fathom compile --layer
// obligations --continue`): CompileLayerWithEscape with
// semantic.CompileInput.Resume wired true — see that field's doc comment.
// A budget-stopped layer at a fixed max_tokens_per_run otherwise refuses at
// the PRE-FLIGHT gate forever once the remaining corpus alone estimates
// over budget; this skips that refusal so the SAME per-run budget keeps
// making incremental progress call over call, the mid-run gate capping
// each call exactly as it always has. The returned CompileRun's
// ResumedFrom names the first not-yet-compiled section this call picked up
// from.
func (c *Client) CompileLayerResume(ctx context.Context, service, layerName string, dryRun bool, escape string) (core.CompileRun, error) {
	return c.compileLayer(ctx, service, layerName, dryRun, escape, true, nil)
}

// ObligationsLayerCoverage reports the obligations layer's LIVE compile
// completeness for service (O54 deliverable 2, second review round): the
// CURRENT corpus, not the possibly-stale numbers the latest compile_runs
// row recorded — see semantic.LiveLayerCoverage. Resolves the layer
// definition and its corpus the same way a real compile does
// (compileLayer); a resolution failure at either step degrades to the
// store-only semantic.ObligationsLayerCoverage rather than failing
// obligations serving outright — this is a serve-time enrichment, never a
// hard dependency.
func (c *Client) ObligationsLayerCoverage(ctx context.Context, service string) (semantic.LayerCoverage, bool, error) {
	root := ""
	if meta, found, merr := c.st.GetServiceMeta(ctx, service); merr == nil && found {
		root = meta.Path
	}
	layerDef, err := semantic.LoadLayerIn(root, semantic.LayerObligations)
	if err != nil {
		return semantic.ObligationsLayerCoverage(ctx, c.st, service, semantic.LayerObligations)
	}
	files, err := c.Sources(ctx, service)
	if err != nil {
		return semantic.ObligationsLayerCoverage(ctx, c.st, service, semantic.LayerObligations)
	}
	corpus := semantic.SelectCorpus(files, layerDef.CorpusInclude, layerDef.CorpusExclude)
	return semantic.LiveLayerCoverage(ctx, c.st, service, layerDef, corpus)
}

// RawFallbackObligations is wave O1's fix (ADR-037) for the product defect
// the benches found: `obligations` served nothing useful when the compiled
// layer's coverage for the edited path's own scope was incomplete
// (budget_stopped, no compile_runs row at all, or a section the compile
// simply never reached). It ranks the obligations layer's own UNCOMPILED
// sections that govern file by the same governing-scope proximity ranker
// compiled claims use (semantic.RawFallbackRules → rankByGoverningScope,
// VB-8) and returns up to max of the nearest as raw rows, alongside why —
// the shared coverage-defect note every returned row also carries
// individually.
//
// ok=false is an honest "nothing to add here", covering four cases: file is
// empty (root-scope calls are out of this mechanism's scope — kept small),
// the layer/corpus cannot be resolved (an enrichment-only degrade, never a
// hard failure), the file's own nearest governing guide is ALREADY fully
// compiled (semantic.NearestUncompiledGuide finds no gap), or the layer has
// no recorded compile_runs row AND already has compiled claims of its own
// (a store that seeds claims directly rather than through a real compile —
// the L1 payload-replay fixture is exactly this shape; guessing at a gap
// from a bookkeeping absence alone would be a false positive, not a real
// one). The genuine "never compiled at all" case (no compile_runs row AND
// zero claims for this layer) DOES fire — the guards-pilot-6 defect this
// exists to fix (a brand-new file's PostToolUse hook naming only "index is
// behind" and nothing else).
func (c *Client) RawFallbackObligations(ctx context.Context, service, file string, max int) ([]semantic.Obligation, string, bool, error) {
	if file == "" || max <= 0 {
		return nil, "", false, nil
	}
	root := ""
	if meta, found, merr := c.st.GetServiceMeta(ctx, service); merr == nil && found {
		root = meta.Path
	}
	layerDef, err := semantic.LoadLayerIn(root, semantic.LayerObligations)
	if err != nil {
		return nil, "", false, nil
	}
	files, err := c.Sources(ctx, service)
	if err != nil {
		return nil, "", false, nil
	}
	corpus := semantic.SelectCorpus(files, layerDef.CorpusInclude, layerDef.CorpusExclude)
	claims, err := c.st.ClaimsForService(ctx, service, layerDef.Name)
	if err != nil {
		return nil, "", false, nil
	}
	cov, found, cerr := semantic.LiveLayerCoverage(ctx, c.st, service, layerDef, corpus)
	if cerr != nil {
		return nil, "", false, nil
	}

	var status string
	switch {
	case !found:
		if len(claims) > 0 {
			// Claims exist without a recorded compile run — a store seeded
			// directly (indexbench's L1 fixture) rather than through a real
			// compile. Nothing here says the LAYER is actually incomplete;
			// guessing at a gap from bookkeeping absence alone would be a
			// false positive on exactly the fixture the L1 payload-replay
			// golden pins byte-for-byte.
			return nil, "", false, nil
		}
		status = "no compile_runs row"
	default:
		if _, gok := semantic.NearestUncompiledGuide(cov, file); !gok {
			return nil, "", false, nil // this file's own governing guide is already fully compiled
		}
		if cov.BudgetStopped {
			status = "budget_stopped"
		} else {
			status = "uncompiled"
		}
	}

	why := fmt.Sprintf("compiled layer incomplete: %s; raw nearest guide served", status)
	// wave O2, item 1: the file's own declared symbol names, when the index
	// has them, are as strong a relevance signal as its stem — resolved
	// here (the only place in this call chain that already holds a store
	// handle) and handed to RawFallbackRules as plain data, which stays a
	// pure function of its inputs.
	extraTokens := semantic.SymbolNamesForFile(ctx, c.st, service, file)
	// wave O3, item 2: a root-wide convention (workspace/, the gate) names
	// no file/package/symbol of its own, so it can never earn a "strong"
	// lexical tie via the above — what DOES tie it to file is file's own
	// BEHAVIOUR. files was already fetched (Sources) to build corpus above;
	// find the edited file's own content in it (a Faux/synthetic file, or
	// one connector.Fetch skipped, simply isn't there — degrade to no
	// signal, never an error) and fold semantic.BehaviorTokens' verdict
	// into the same extraTokens path a resolved symbol name already uses.
	for _, sf := range files {
		if sf.Path == file {
			extraTokens = append(extraTokens, semantic.BehaviorTokens(string(sf.Content))...)
			break
		}
	}
	rows, rerr := semantic.RawFallbackRules(claims, layerDef, corpus, file, max, why, extraTokens)
	if rerr != nil || len(rows) == 0 {
		return nil, "", false, nil
	}
	return rows, why, true, nil
}

func (c *Client) compileLayer(ctx context.Context, service, layerName string, dryRun bool, escape string, resume bool, budget *semantic.BudgetOverride) (core.CompileRun, error) {
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
	// wave O4, item 2: apply the caller's per-run budget override, if any,
	// AFTER resolving the layer's own manifest budgets — Apply is a pure
	// function (never mutates layer.Budgets in place), so every downstream
	// use of layer (corpus selection, prompt identity, etc.) is unaffected;
	// only the Budgets Compiler actually enforces are replaced.
	layer = budget.Apply(layer)

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
		Resume:       resume,
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

// SetClaimStatus is a thin passthrough to the store's status flip — the
// review reconcile (review.StaleReviewClaims) uses it to supersede
// review-minted claims whose findings a later run no longer emits.
func (c *Client) SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error {
	return c.st.SetClaimStatus(ctx, serviceID, claimID, status)
}

// CompileRunsFor (Q-25, docs/plans/expert-suite-q.md) is a thin passthrough
// to the store's compile_runs read, newest-first — used by `/statusz`
// (cmd/fathom/statusz.go) to report the last compile per layer per
// service, and available to any other caller needing a service's full
// compile history without ClaimsSummary's single-most-recent-run
// shortcut.
func (c *Client) CompileRunsFor(ctx context.Context, service string) ([]core.CompileRun, error) {
	return c.st.CompileRuns(ctx, service, "")
}

// AppendClaimEvent (Q-5, docs/plans/expert-suite-q.md) is a thin passthrough
// to the store's append-only claim-event log — ClaimFeedback already uses
// c.st.AppendClaimEvent internally for the refute/corroborate actions; this
// exported wrapper lets a CLI command (fathom review's fixed-finding
// reconcile, recordReviewSupersedeEvents) record a verdict too, without
// going through ClaimFeedback's action-string dispatch.
func (c *Client) AppendClaimEvent(ctx context.Context, ev core.ClaimEvent) error {
	return c.st.AppendClaimEvent(ctx, ev)
}

// ClaimEventsSince (docs/design/findings-as-evidence.md §3.3/§3.7, §6 items
// 3/7) is a thin passthrough to the store's bulk, time-windowed
// claim_events read (Q-6's ClaimEventsSince) — findings pattern derivation
// and the `fathom findings status`/`near` verbs need the whole
// claim_events population (or a wide window of it) to resolve each
// finding-claim's own resolution outcome, the same store-wide-not-service-
// scoped caveat Store.ClaimEventsSince's own doc comment states (claim_events
// carries no service_id column).
func (c *Client) ClaimEventsSince(ctx context.Context, since, until time.Time) ([]core.ClaimEvent, error) {
	return c.st.ClaimEventsSince(ctx, since, until)
}

// SourceFingerprint returns the store's current content fingerprint for
// (service, repo-relative path) — the fusion hash-gate table
// semantic.DriftedFindings reads. The review findings bridge records it as
// each persisted claim's scope-fp drift anchor (FT-6), mirroring the
// compiler's own verifyDraft.
func (c *Client) SourceFingerprint(ctx context.Context, serviceID, path string) (hash string, found bool, err error) {
	return c.st.Fingerprint(ctx, serviceID, path)
}

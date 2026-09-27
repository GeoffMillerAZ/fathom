package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
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

// WithDecision attaches the decision port (ADR-042, docs/vision/
// 32-decision-port.md) this Client threads through CompileLayer's
// pre-filter (DP-9) and the Obligations*/ObligationsLayer*/ObligationsAsOf*
// re-rank (DP-7/DP-8): d answers each site's batched fast-decision
// questions, cfg carries the thresholds both sites read (CompileThreshold,
// RankWithinTier), and rec is the shared decisions.jsonl sink both sites
// append their decision.Record rows to — a nil rec is decide.Recorder's own
// safe no-op, so a caller that wants the decider wired but no log written
// can pass nil. Never calling WithDecision (decider stays the zero value,
// nil) leaves every decision-port-aware call site byte-identical to before
// this method existed: see decisionActive. Returns c for chaining, like
// WithLLM.
func (c *Client) WithDecision(d decision.Decider, cfg config.Decide, rec *decide.Recorder) *Client {
	c.decider = d
	c.decideCfg = cfg
	c.decideRec = rec
	return c
}

// WithLogger routes the underlying fusion.Pipeline's own diagnostics
// (the typed-edge pass's process-wide-slot/load-failure/discovery-gap
// lines, DG-4 diagnostics sweep) through log instead of a bare
// fmt.Fprintf(os.Stderr, ...). Never calling WithLogger (pipe.logger stays
// nil) leaves those lines byte-identical to before this method existed —
// like WithLLM/WithDecision, this is additive. Returns c for chaining.
func (c *Client) WithLogger(log *dlog.Logger) *Client {
	c.pipe.SetLogger(log)
	return c
}

// decisionActive reports whether c has a decision port worth calling: a
// non-nil Decider that is not the kit's own zero-configuration
// decision.None stub. PrefilterSections and rerankWithinTiers already treat
// nil/None identically to "no call, unchanged output" on their own, but
// call sites in this package use decisionActive to decide whether to wire
// the Decider/options through AT ALL, so a Client that never calls
// WithDecision (or is explicitly given decision.None) builds a
// semantic.Compiler/ObligationsOptions indistinguishable from one built
// before the decision port existed.
func (c *Client) decisionActive() bool {
	if c.decider == nil {
		return false
	}
	_, isNone := c.decider.(decision.None)
	return !isNone
}

// decisionOpts builds the semantic.ObligationsOptions the file-scoped
// obligations entry points (Obligations, ObligationsLayer, ObligationsAsOf)
// pass to their *Opts sibling. Hunk is always empty here — none of these
// three exposed methods takes a hunk argument (only the MCP obligations
// tool's own edit-hook path, which another wave may wire directly against
// semantic.ObligationsForOpts, ever has one). When c.decisionActive() is
// false this is the zero-value ObligationsOptions{}, which
// ObligationsForOpts/ObligationsForLayerOpts/ObligationsAsOfOpts document as
// byte-identical to their pre-Opts, non-decision-port siblings.
//
// RerankTimeout (DP-25, O109) is threaded from c.decideCfg.RerankTimeoutMS
// ([decide] rerank_timeout_ms, config.DefaultDecide's own default 1500ms) —
// the ONE place a real decision port supplies this bound; a Client that
// never calls WithDecision, or is given decision.None, never reaches this
// line at all (the early return above), so semantic.rerankWithinTiers'
// own zero-value fallback (defaultRerankTimeout) is exercised only by a
// caller that builds ObligationsOptions directly, outside this path.
//
// ADR-044 decision 8 (I14): decide.SiteAllowed(c.decideCfg,
// decide.SiteObligationsRerank) is a second, independent gate a real
// decider must also clear — "obligations_rerank" absent from [decide]
// sites returns the SAME zero-value ObligationsOptions{} as a Client that
// never called WithDecision at all, so a blocked site makes no call,
// byte-identical to the port simply being off.
func (c *Client) decisionOpts() semantic.ObligationsOptions {
	if !c.decisionActive() || !decide.SiteAllowed(c.decideCfg, decide.SiteObligationsRerank) {
		return semantic.ObligationsOptions{}
	}
	return semantic.ObligationsOptions{
		Decider:        c.decider,
		RankWithinTier: c.decideCfg.RankWithinTier,
		RerankTimeout:  time.Duration(c.decideCfg.RerankTimeoutMS) * time.Millisecond,
	}
}

// coreDecisionRecordsToKit converts internal/core's dependency-free
// core.DecisionRecord (what a CompileRun/Obligations payload carries across
// the internal/semantic boundary) back into the kit's own decision.Record
// shape, which decide.Recorder.Append consumes — the inverse of
// internal/semantic's own (unexported) toCoreDecisionRecords. nil in, nil
// out.
func coreDecisionRecordsToKit(recs []core.DecisionRecord) []decision.Record {
	if len(recs) == 0 {
		return nil
	}
	out := make([]decision.Record, len(recs))
	for i, r := range recs {
		out[i] = decision.Record{
			Site:         r.Site,
			Provider:     r.Provider,
			Model:        r.Model,
			QuestionID:   r.QuestionID,
			Kind:         decision.Kind(r.Kind),
			Choice:       r.Choice,
			Confidence:   r.Confidence,
			Score:        r.Score,
			Noul:         r.Noul,
			Latency:      time.Duration(r.LatencyMS) * time.Millisecond,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
		}
	}
	return out
}

// recordDecisionErrors (DP-22, O109) logs each of errs as one
// decisions.jsonl error row — the caller-side half of a rerank chunk that
// hard-failed (internal/semantic.DecisionError carries the data;
// internal/semantic itself stays free of the decide.Recorder type). model
// is always "" here: the decision.Decider interface has no Model()
// accessor, and a hard Decide error returns no Response to read one off.
// A nil/empty errs, or a Client with no decideRec wired, is a safe no-op —
// decide.Recorder.RecordError is itself nil-receiver-safe.
func (c *Client) recordDecisionErrors(errs []semantic.DecisionError) {
	for _, de := range errs {
		c.decideRec.RecordError(de.Site, de.Provider, "", de.Questions, de.Latency, de.Err)
	}
}

// Obligations returns the obligations governing file for service, capped at
// maxRules (≤ semantic.MaxObligations; ≤0 means the default cap).
//
// DP-7/DP-9: routed through semantic.ObligationsForOpts so a Client with an
// active decision port (WithDecision, decisionActive) re-ranks eligible
// rules within their governing-scope tier — see decisionOpts. A Client that
// never calls WithDecision passes a zero-value semantic.ObligationsOptions,
// which ObligationsForOpts documents as byte-identical to the pre-Opts
// ObligationsFor path. DP-22: obl.DecisionErrors carries one entry per
// rerank chunk whose Decide call itself hard-failed — recordDecisionErrors
// logs each as an error row (O109: before this, a failed chunk left no
// trace in decisions.jsonl at all).
func (c *Client) Obligations(ctx context.Context, service, file string, maxRules int) (semantic.Obligations, error) {
	obl, err := semantic.ObligationsForOpts(ctx, c.st, service, file, maxRules, c.decisionOpts())
	c.decideRec.Append(obl.Decisions)
	c.recordDecisionErrors(obl.DecisionErrors)
	return obl, err
}

// ObligationsAsOf answers "what governed file AT asOf" (SU-19, ADR-041
// decision 7) — see semantic.ObligationsAsOf's own doc comment for why this
// is a separate, slower read path from Obligations/ObligationsLayer. DP-9:
// routed through semantic.ObligationsAsOfOpts the same way Obligations is
// routed through ObligationsForOpts — see decisionOpts. DP-22: see
// Obligations' own doc comment on recordDecisionErrors.
func (c *Client) ObligationsAsOf(ctx context.Context, service, file string, maxRules int, asOf time.Time) (semantic.Obligations, error) {
	obl, err := semantic.ObligationsAsOfOpts(ctx, c.st, service, file, maxRules, asOf, c.decisionOpts())
	c.decideRec.Append(obl.Decisions)
	c.recordDecisionErrors(obl.DecisionErrors)
	return obl, err
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
// semantic.ObligationLayerFilter. DP-9: routed through
// semantic.ObligationsForLayerOpts, Obligations' own decision-port routing
// — see decisionOpts.
func (c *Client) ObligationsLayer(ctx context.Context, service, file string, maxRules int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	obl, err := semantic.ObligationsForLayerOpts(ctx, c.st, service, file, maxRules, layer, c.decisionOpts())
	c.decideRec.Append(obl.Decisions)
	c.recordDecisionErrors(obl.DecisionErrors)
	return obl, err
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
// A file over HotPathReadCap is never read and returns false; callers that
// can carry the skip use ObligationsIndexCheck, which names it.
//
// Fingerprint formula matches the fusion pipeline (Pass 3) exactly via
// fusion.FingerprintHash — the epoch-salted content hash.
func (c *Client) ObligationsIndexStale(ctx context.Context, service, file string) bool {
	stale, _ := c.ObligationsIndexCheck(ctx, service, file)
	return stale
}

// ObligationsIndexCheck is ObligationsIndexStale with the skip named (RT-26,
// docs/design/responsible-tenant.md §4.3): it stats the file before reading
// it, and a file over HotPathReadCap (or not a regular file) is never read.
// Such a file is reported not stale — nothing was compared, so nothing may
// claim the index is behind or trigger a refresh — and unchecked names why,
// so a caller reports the freshness as unknown rather than as current
// (invariant I38). The stored fingerprint is looked up before the read, so
// a file the index never fingerprinted is not read either. unchecked is ""
// whenever the comparison ran or could not run for an ordinary reason
// (missing file, no metadata, no fingerprint).
func (c *Client) ObligationsIndexCheck(ctx context.Context, service, file string) (stale bool, unchecked string) {
	// Resolve disk path: absolute passthrough; otherwise join with meta root.
	diskPath := file
	if !filepath.IsAbs(diskPath) {
		meta, found, err := c.st.GetServiceMeta(ctx, service)
		if err != nil || !found || meta.Path == "" {
			return false, ""
		}
		diskPath = filepath.Join(meta.Path, file)
	}

	info, err := os.Stat(diskPath)
	if err != nil {
		return false, ""
	}
	if !info.Mode().IsRegular() || info.Size() > HotPathReadCap {
		why := "not a regular file"
		if info.Mode().IsRegular() {
			why = oversizeNote(info.Size(), HotPathReadCap)
		}
		return false, "index freshness not checked: " + why + "; not refreshed"
	}

	stored, found, err := c.st.Fingerprint(ctx, service, file)
	if err != nil || !found {
		return false, ""
	}
	// Design §4.3: hash only when the stat moved since the last hash. The
	// memo keeps the hash, never the verdict, so a stored fingerprint that
	// moved (a re-ingest) is compared afresh without a read.
	if e, ok := c.editedLookup(diskPath, info); ok && e.hashOK {
		return e.hash != stored, ""
	}
	data, rinfo, skipped, err := readFileCapped(diskPath, HotPathReadCap)
	if err != nil {
		return false, ""
	}
	if skipped != "" {
		return false, "index freshness not checked: " + skipped + "; not refreshed"
	}
	hash := fusion.FingerprintHash(data)
	c.editedStore(diskPath, rinfo, func(e *editedEntry) { e.hash, e.hashOK = hash, true })
	return hash != stored, ""
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
// row recorded — see semantic.LiveLayerCoverage. The layer definition
// resolves through the memoized pack union; the corpus is the layer's own
// guides only (layerCorpus, RT-143: paths from one names-only walk per
// store watermark, each guide stat'ed and read under the hot-path cap), never
// the tree's contents. A guide it could not read is named in UnreadCorpus,
// making the counts a lower bound (I38). A resolution failure degrades to
// the store-only semantic.ObligationsLayerCoverage rather than failing
// obligations serving outright — this is a serve-time enrichment, never a
// hard dependency.
func (c *Client) ObligationsLayerCoverage(ctx context.Context, service string) (semantic.LayerCoverage, bool, error) {
	layerDef, err := semantic.LoadLayerIn(c.serviceRoot(ctx, service), semantic.LayerObligations)
	if err != nil {
		return semantic.ObligationsLayerCoverage(ctx, c.st, service, semantic.LayerObligations)
	}
	corpus, unread, err := c.layerCorpus(ctx, service, layerDef)
	if errors.Is(err, tenancy.ErrOverBudget) {
		// RT-153 / I38: halted by the governor — named, not a fallback.
		return semantic.LayerCoverage{}, false, err
	}
	if err != nil {
		return semantic.ObligationsLayerCoverage(ctx, c.st, service, semantic.LayerObligations)
	}
	cov, found, err := semantic.LiveLayerCoverage(ctx, c.st, service, layerDef, corpus)
	if err == nil && len(unread) > 0 {
		// Carried whether or not a compile run was recorded: with found
		// false the caller renders a "coverage unknown" placeholder, and
		// an unread guide must be named there too, never dropped (I38).
		cov.UnreadCorpus = skipStrings(unread)
	}
	return cov, found, err
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
// RT-81 (docs/design/responsible-tenant.md §4.3): the corpus is the
// layer's own guides (layerCorpus — never the tree's contents), and the
// only other file read is the edited one, stat'ed first and capped at
// HotPathReadCap. An edited file over the cap is not read: its behaviour
// tokens are simply absent (ranking falls back to path and symbol
// signals) and why says so.
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
	root := c.serviceRoot(ctx, service)
	layerDef, err := semantic.LoadLayerIn(root, semantic.LayerObligations)
	if err != nil {
		return nil, "", false, nil
	}
	corpus, unread, err := c.layerCorpus(ctx, service, layerDef)
	if errors.Is(err, tenancy.ErrOverBudget) {
		// RT-153 / I38: a governor halt is named, never "nothing to add".
		return nil, "", false, err
	}
	if err != nil {
		return nil, "", false, nil
	}
	corpus = c.probeAncestorGuides(ctx, service, layerDef, file, corpus)
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
			// Coverage was computed without the unread guides, so "fully
			// compiled" is only known for the guides that were read. An
			// unread guide that governs file is not known compiled (I38):
			// fire, and name it below.
			if _, uok := semantic.NearestUncompiledGuide(semantic.LayerCoverage{UncompiledScopes: skipPaths(unread)}, file); !uok {
				return nil, "", false, nil // this file's own governing guide is already fully compiled
			}
			status = "governing guide not read"
			break
		}
		if cov.BudgetStopped {
			status = "budget_stopped"
		} else {
			status = "uncompiled"
		}
	}

	why := fmt.Sprintf("compiled layer incomplete: %s; raw nearest guide served", status)
	if len(unread) > 0 {
		// I38: a guide the corpus read skipped (over the cap, EPERM) is
		// named, and the served rows are a lower bound — never a silent
		// absence.
		why += fmt.Sprintf("; %d guide(s) not read, rows are a lower bound (%s)", len(unread), strings.Join(skipStrings(unread), "; "))
	}
	// wave O2, item 1: the file's own declared symbol names, when the index
	// has them, are as strong a relevance signal as its stem — resolved
	// here (the only place in this call chain that already holds a store
	// handle) and handed to RawFallbackRules as plain data, which stays a
	// pure function of its inputs.
	extraTokens := semantic.SymbolNamesForFile(ctx, c.st, service, file)
	// wave O3, item 2: a root-wide convention (workspace/, the gate) names
	// no file/package/symbol of its own, so it can never earn a "strong"
	// lexical tie via the above — what DOES tie it to file is file's own
	// BEHAVIOUR. Read the edited file alone, under the cap (RT-81); a file
	// that is missing or unreadable simply isn't a signal, and one over the
	// cap is named in why rather than read.
	if tokens, skipped := c.editedFileTokens(ctx, root, service, file); skipped != "" {
		why += "; edited file not read for relevance (" + skipped + ")"
	} else {
		extraTokens = append(extraTokens, tokens...)
	}
	rows, rerr := semantic.RawFallbackRules(claims, layerDef, corpus, file, max, why, extraTokens)
	if rerr != nil || len(rows) == 0 {
		return nil, "", false, nil
	}
	return rows, why, true, nil
}

// editedFileTokens derives the raw fallback's behaviour signal from the one
// edited file: from the service root on disk, stat'ed and capped at
// HotPathReadCap, and kept against the file's stat so an unchanged file is
// not read again (design §4.3); for a connector with no on-disk root,
// through the connector, capped after the fetch. tokens is nil (and
// skipped "") when the file is simply not there.
func (c *Client) editedFileTokens(ctx context.Context, root, service, file string) (tokens []string, skipped string) {
	if root != "" {
		if st, err := os.Stat(root); err == nil && st.IsDir() {
			path := file
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, filepath.FromSlash(file))
			}
			info, serr := os.Stat(path)
			if serr != nil {
				return nil, ""
			}
			if e, ok := c.editedLookup(path, info); ok && e.tokensOK {
				return e.tokens, ""
			}
			data, rinfo, why, rerr := readFileCapped(path, HotPathReadCap)
			if rerr != nil || why != "" {
				return nil, why
			}
			tokens = semantic.BehaviorTokens(string(data))
			c.editedStore(path, rinfo, func(e *editedEntry) { e.tokens, e.tokensOK = tokens, true })
			return tokens, ""
		}
	}
	data, err := c.conn.Fetch(ctx, core.SourceRef{Service: service, Path: file})
	if errors.Is(err, tenancy.ErrOverBudget) {
		// RT-153 / I38: not read because the governor halted it — named
		// in why, never the silent "file simply isn't there".
		return nil, err.Error()
	}
	if err != nil {
		return nil, ""
	}
	if int64(len(data)) > HotPathReadCap {
		return nil, oversizeNote(int64(len(data)), HotPathReadCap)
	}
	return semantic.BehaviorTokens(string(data)), ""
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

	// CV-41 (gap-closing wave 3, item 3): NoPriorRun is true only when this
	// service has NO compile_runs row at all for layerName — a genuinely
	// first-ever compile. A read error here degrades to false (the ordinary,
	// SAFER pre-flight-refusal path stays in force) rather than silently
	// bypassing a spend guard on a store hiccup — the one place this method
	// fails CLOSED rather than open, since the thing being skipped is a cost
	// protection, not an availability nicety. DryRun compiles never reach
	// compile_runs (semantic.Compile's own "never persisted" DryRun branch),
	// so this is unaffected by how many dry-run estimates preceded it.
	noPriorRun := false
	if priorRuns, rerr := c.st.CompileRuns(ctx, service, layerName); rerr == nil {
		noPriorRun = len(priorRuns) == 0
	}

	comp := semantic.Compiler{
		St:             c.st,
		LLM:            c.llm,
		Redactor:       llmsec.NewRedactor(llmsec.DefaultPosture()),
		Model:          c.llmModel,
		Layer:          layer,
		InRatePerKTok:  c.llmInRate,
		OutRatePerKTok: c.llmOutRate,
	}
	// ADR-042 decision 2 / DP-9: wire the compile pre-filter's decision port
	// only when a real decider is configured (non-nil AND not the kit's own
	// decision.None stub) — PrefilterSections already treats a nil or None
	// Decider as the identity (no call, sections_prefiltered stays 0), but
	// leaving Compiler.Decider unset in that case keeps a Compiler built
	// through this path byte-identical to one built before WithDecision
	// existed, rather than merely behaviorally equivalent.
	//
	// ADR-044 decision 8 (I14): decide.SiteAllowed(c.decideCfg,
	// decide.SiteCompilePrefilter) is a second, independent gate a real
	// decider must also clear — "compile_prefilter" absent from [decide]
	// sites leaves comp.Decider unset, the SAME "no call at all" shape as
	// decisionActive() being false.
	if c.decisionActive() && decide.SiteAllowed(c.decideCfg, decide.SiteCompilePrefilter) {
		comp.Decider = c.decider
		comp.PrefilterThreshold = c.decideCfg.CompileThreshold
	}
	run, err := comp.Compile(ctx, semantic.CompileInput{
		ServiceID:    service,
		Docs:         corpus,
		IndexedFiles: indexedFiles,
		Watermark:    watermark,
		DryRun:       dryRun,
		Escape:       escape,
		Resume:       resume,
		NoPriorRun:   noPriorRun,
	})
	// DP-3/DP-9: log whatever the pre-filter decided, regardless of err —
	// a partial run that failed mid-compile may still have asked the
	// decision port about earlier sections, and those decisions were real.
	c.decideRec.Append(coreDecisionRecordsToKit(run.Decisions))
	// DP-22 (O109): run.PrefilterError is set exactly when the pre-filter's
	// own Decide call hard-failed (aborting the whole run) — log it as an
	// error row too, so the failure that killed this compile is visible in
	// decisions.jsonl, not only in the returned err.
	if run.PrefilterError != nil {
		c.decideRec.RecordError(
			run.PrefilterError.Site,
			run.PrefilterError.Provider,
			"",
			run.PrefilterError.Questions,
			time.Duration(run.PrefilterError.LatencyMS)*time.Millisecond,
			errors.New(run.PrefilterError.Err),
		)
	}
	return run, err
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
		// SU-19 (ADR-041 decision 7): refute (the only action that sets
		// newStatus) is terminal — close the validity window at the same
		// instant. corroborate never reaches here.
		if err := c.st.SetClaimValidTo(ctx, service, claimID, ev.At); err != nil {
			return fmt.Errorf("client.ClaimFeedback: close validity window: %w", err)
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

// SetClaimValidTo is a thin passthrough to the store's bi-temporal window
// close (SU-19, ADR-041 decision 7) — every native supersede/refute path
// calls it beside SetClaimStatus/AppendClaimEvent.
func (c *Client) SetClaimValidTo(ctx context.Context, serviceID, claimID string, t time.Time) error {
	return c.st.SetClaimValidTo(ctx, serviceID, claimID, t)
}

// ClaimsAnchoredTo is a thin passthrough to semantic.ClaimsAnchoredTo
// (SU-21, ADR-041 decision 7) — the claims anchored to symbolKey via an
// "anchors" claim link, for rename_impact's claims_anchored receipt.
func (c *Client) ClaimsAnchoredTo(ctx context.Context, serviceID, symbolKey string) ([]string, error) {
	return semantic.ClaimsAnchoredTo(ctx, c.st, serviceID, symbolKey)
}

// WriteSupersedes is a thin passthrough to semantic.WriteSupersedes (SU-20,
// ADR-041 decision 7) — records that successorID replaces predecessorID via
// a "supersedes" claim link. See semantic.WriteSupersedes' own doc comment
// for why this is called only where a genuine successor claim exists.
func (c *Client) WriteSupersedes(ctx context.Context, serviceID, successorID, predecessorID, why string) error {
	return semantic.WriteSupersedes(ctx, c.st, serviceID, successorID, predecessorID, why)
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

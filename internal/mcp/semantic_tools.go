package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modserve"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// ─── optional backend interfaces ─────────────────────────────────────────────

// obligationsBackend is the optional backend capability for serving compiled
// obligations to a coding agent. pkg/client implements it; test fakes may
// omit it.
type obligationsBackend interface {
	Obligations(ctx context.Context, service, file string, maxRules int) (semantic.Obligations, error)
	ObligationsIndexStale(ctx context.Context, service, file string) bool
}

// obligationsIndexCheckBackend is obligationsBackend.ObligationsIndexStale
// with the skip named (RT-26, docs/design/responsible-tenant.md §4.3): a
// file over the hot-path read cap is never read, so its freshness is
// unknown, and unchecked says so. The obligations tool carries it as
// index_unchecked (I38: a skipped read is reported, never passed off as
// fresh). pkg/client implements it; a backend that omits it serves the
// bool alone, as before.
type obligationsIndexCheckBackend interface {
	ObligationsIndexCheck(ctx context.Context, service, file string) (stale bool, unchecked string)
}

// obligationsIndexFreshness asks the backend for file's index freshness,
// preferring the check that names a skipped read.
func obligationsIndexFreshness(ctx context.Context, backend any, ob obligationsBackend, service, file string) (bool, string) {
	if cb, ok := backend.(obligationsIndexCheckBackend); ok {
		return cb.ObligationsIndexCheck(ctx, service, file)
	}
	return ob.ObligationsIndexStale(ctx, service, file), ""
}

// obligationsAsOfBackend is the optional backend capability for SU-19's
// bi-temporal read (ADR-041 decision 7): "what governed file AT asOf"
// instead of "what governs it now". pkg/client implements it via
// semantic.ObligationsAsOf; a backend that omits it (an older module, a
// minimal test fake) makes the obligations tool refuse an as_of argument
// outright rather than silently answering the live question instead.
type obligationsAsOfBackend interface {
	ObligationsAsOf(ctx context.Context, service, file string, maxRules int, asOf time.Time) (semantic.Obligations, error)
}

// rootObligationsBackend is the optional backend capability for the
// CORRECT root-scope answer (semantic.RootObligationsFor: source- and
// glob-breadth selection, not a literal-path probe against
// obligationsBackend.Obligations — see rootObligations' doc comment for why
// the probe cannot be made correct). pkg/client implements it; a backend
// that omits it falls back to the probe, which still finds a NARROW
// "CLAUDE.md#section"-style scope but silently drops a repo-wide
// "**/*.go"-style one (the MAJOR finding this type fixes).
type rootObligationsBackend interface {
	RootObligations(ctx context.Context, service string, maxRules int) (semantic.Obligations, error)
}

// obligationLayerBackend is the optional backend capability for VB-9's
// layer split (ADR-035 decision 2): a file-scoped obligations call that
// takes an explicit semantic.ObligationLayerFilter. pkg/client implements
// it via semantic.ObligationsForLayer; a backend that omits it (an older
// module, a minimal test fake) degrades to the pre-VB-9 obligationsBackend
// call, which always serves the "all" union regardless of what the caller
// asked for — see obligationLayers' doc comment.
type obligationLayerBackend interface {
	ObligationsLayer(ctx context.Context, service, file string, maxRules int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error)
}

// reviewObligationsBackend is the optional backend capability a review
// verb asks for its obligations through (GL-5, docs/design/governance-
// learning.md §3): the same file-scoped serve as obligationsBackend, plus
// the review-audience claims (BUGBOT.md rules) the edit hook never sees.
// pkg/client implements it with semantic.ObligationsOptions.Review; a
// backend that omits it degrades to plain Obligations — review rules
// simply absent, never an error.
type reviewObligationsBackend interface {
	ReviewObligations(ctx context.Context, service, file string, maxRules int) (semantic.Obligations, error)
}

// rootObligationsLayerBackend is obligationLayerBackend's root-scope
// sibling (semantic.RootObligationsForLayer).
type rootObligationsLayerBackend interface {
	RootObligationsLayer(ctx context.Context, service string, maxRules int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error)
}

// compileBackend is the optional backend capability for on-demand
// semantic-layer compilation. CompileLayer takes the layer name the caller
// requested (FT-1/F7: compile_layer previously validated the name via
// LoadLayer and then discarded it, always compiling obligations regardless
// of what was asked for).
type compileBackend interface {
	CompileLayer(ctx context.Context, service, layer string, dryRun bool) (core.CompileRun, error)
}

// claimsSummaryBackend is the optional backend capability for attaching a
// per-layer claims summary to staleness_report.
type claimsSummaryBackend interface {
	ClaimsSummary(ctx context.Context, service string) (map[string]any, error)
}

// claimFeedbackBackend is the optional backend capability for the
// obligations tool's feedback action (D2, docs/plans/expert-depth.md): the
// refute/corroborate write path riding the already-hot obligations channel
// instead of shipping a new tool. pkg/client.ClaimFeedback implements it,
// mirroring `fathom claim refute|corroborate` (cmd/fathom/claim.go).
type claimFeedbackBackend interface {
	ClaimFeedback(ctx context.Context, service, claimID, action, note, author string) error
}

// layerCoverageBackend is the optional backend capability for O54
// deliverable 2's coverage disclosure: the obligations layer's LATEST
// compile run completeness (sections_total/sections_compiled/
// budget_stopped/uncompiled_scopes), attached to every obligations
// response as layer_coverage — the same execute-and-flag doctrine ADR-014
// applies elsewhere, applied to a compiled layer's own corpus
// completeness. pkg/client.Client implements it via semantic.
// ObligationsLayerCoverage; a backend that omits it (an older module, a
// minimal test fake) simply serves no layer_coverage field — an honest
// "coverage unknown", never a fabricated full-coverage claim.
type layerCoverageBackend interface {
	ObligationsLayerCoverage(ctx context.Context, service string) (semantic.LayerCoverage, bool, error)
}

// rawFallbackBackend is the optional backend capability for wave O1's
// raw-section fallback (ADR-037): the product defect the benches found —
// `obligations` served NOTHING USEFUL when the compiled layer's coverage
// for the edited path's own scope was incomplete (a budget-stopped
// compile, no compile_runs row at all, or a section the compile simply
// never reached). Silence, or a generic root-tier rule that happened to
// compile, is never the answer; this ranks the layer's own UNCOMPILED
// sections by the exact same governing-scope proximity ranker compiled
// claims use (rankByGoverningScope, VB-8) and serves the nearest ones,
// flagged Tier:"raw" with a Why note, ahead of any farther-tier compiled
// row. pkg/client.Client implements it via semantic.RawFallbackRules; a
// backend that omits it (an older module, a minimal test fake) degrades to
// the existing layer_coverage note-only disclosure — an enrichment, never
// a hard dependency, exactly like layerCoverageBackend above.
type rawFallbackBackend interface {
	RawFallbackObligations(ctx context.Context, service, file string, max int) (rows []semantic.Obligation, why string, ok bool, err error)
}

// rootObligationsProbe is the FALLBACK-only bait path for rootObligations,
// used solely when the backend does not implement rootObligationsBackend:
// never read for content, only fed to obligationsBackend.Obligations' glob
// matching. It is NOT a correct root-scope filter — a claim's scope glob is
// matched against this one literal string, so a narrow "CLAUDE.md"-shaped
// glob matches, but a broad, genuinely repo-wide glob like "**/*.go" does
// NOT (the string "CLAUDE.md" is not a "*.go" file), even when it was
// proposed by the root CLAUDE.md itself — the MAJOR finding
// semantic.RootObligationsFor exists to fix. Kept only as a degrade for a
// backend with no better answer; every real backend should implement
// rootObligationsBackend instead.
const rootObligationsProbe = "CLAUDE.md"

// rootObligations answers "what governs this repository" for an obligations
// call with no file argument (RC-22, docs/vision/28-receipts.md §3.5): the
// compiled layer's rules whose SOURCE is a root-level governing doc or
// whose scope glob is unanchored/wildcard-rooted (semantic.
// RootObligationsFor — selection by scope breadth, not by matching one
// literal probe path), budgeted exactly like a file-scoped call. Prefers
// backend.(rootObligationsBackend) when the concrete backend offers it
// (every real one does); a backend that does not (a minimal test fake, an
// older module) degrades to the rootObligationsProbe bait path, which is
// narrower — see its doc comment for exactly what it misses. Either way,
// zero rules means no compiled claim's source or scope reaches the repo
// root — an honest "nothing governs this repo at that level" answer, the
// same zero-output-on-clean contract a file-scoped call makes.
func rootObligations(ctx context.Context, backend QueryInterface, ob obligationsBackend, service string, maxRules int) (semantic.Obligations, error) {
	var obl semantic.Obligations
	var err error
	if rb, ok := backend.(rootObligationsBackend); ok {
		obl, err = rb.RootObligations(ctx, service, maxRules)
	} else {
		obl, err = ob.Obligations(ctx, service, rootObligationsProbe, maxRules)
	}
	// A root-scope answer never carries a single file — true by
	// construction in semantic.RootObligationsFor, enforced again here so
	// it holds regardless of which path served it (a test double, or a
	// future backend, that forgets to clear it).
	obl.File = ""
	return obl, err
}

// validObligationLayer normalizes the obligations tool's "layer" request
// value: "" defaults to "rules" (VB-9, ADR-035 decision 2 — the default
// answers "what governs this file" with human-authored rules only), a
// recognized value passes through, anything else is refused rather than
// silently degraded — a typo in "layer" must not be read as "give me
// everything".
func validObligationLayer(raw string) (semantic.ObligationLayerFilter, *rpcError) {
	switch semantic.ObligationLayerFilter(raw) {
	case "":
		return semantic.ObligationLayerRules, nil
	case semantic.ObligationLayerRules, semantic.ObligationLayerClaims, semantic.ObligationLayerAll:
		return semantic.ObligationLayerFilter(raw), nil
	default:
		return "", &rpcError{Code: ErrCodeParams, Message: `obligations: layer must be "rules", "claims" or "all"`}
	}
}

// obligationLayers resolves the obligations tool's request to the compiled
// rules it actually serves (VB-9, ADR-035 decision 2, the anchor for this
// function): "layer" defaults to "rules" — human-authored CLAUDE.md/
// AGENTS.md/ADR claims only (RoleGoverns) — "claims" serves ONLY plan-row
// bindings resolved by their own {symbol, package} contract (a row with no
// contract binds to nothing — never by prose keyword, the O23 defect),
// and "all" is the pre-VB-9 union, prose fallback for a non-plan-row
// corpus (findings/**) included. rootScope selects semantic.
// RootObligationsFor's "what governs this repository" question instead of
// a file lookup.
//
// A backend that predates the layer split (obligationLayerBackend /
// rootObligationsLayerBackend) degrades to its pre-VB-9 Obligations/
// RootObligations call and reports servedLayer "all" — the caller sees
// the legacy union, never an error, because a lookup that still works
// beats one that refuses.
func obligationLayers(ctx context.Context, backend QueryInterface, ob obligationsBackend, service, file string, maxRules int, layer semantic.ObligationLayerFilter, rootScope bool) (obl semantic.Obligations, servedLayer string, rerr *rpcError) {
	if rootScope {
		if rb, ok := backend.(rootObligationsLayerBackend); ok {
			o, err := rb.RootObligationsLayer(ctx, service, maxRules, layer)
			if err != nil {
				return semantic.Obligations{}, "", &rpcError{Code: ErrCodeInternal, Message: err.Error()}
			}
			o.File = ""
			return o, string(layer), nil
		}
		o, err := rootObligations(ctx, backend, ob, service, maxRules)
		if err != nil {
			return semantic.Obligations{}, "", &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		return o, string(semantic.ObligationLayerAll), nil
	}

	if lb, ok := backend.(obligationLayerBackend); ok {
		o, err := lb.ObligationsLayer(ctx, service, file, maxRules, layer)
		if err != nil {
			return semantic.Obligations{}, "", &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		return o, string(layer), nil
	}
	o, err := ob.Obligations(ctx, service, file, maxRules)
	if err != nil {
		return semantic.Obligations{}, "", &rpcError{Code: ErrCodeInternal, Message: err.Error()}
	}
	return o, string(semantic.ObligationLayerAll), nil
}

// renderObligationReceipt assembles the obligations tool's response
// envelope (VB-10, ADR-035 decision 2, the anchor for this function).
// Three guarantees, stated once so the shape means the same thing on every
// call:
//
//  1. Rules ride the shared 16 KB tool budget (fitToolBudget — the same
//     discipline find_callers/outline/symbol_context use), so a
//     snippet-laden payload can never blow the caller's context even
//     though MaxObligations already caps rule COUNT to 6. Rules arrive
//     already ordered fresh-before-stale (semantic.ObligationsFor's own
//     contract), so a byte-budget cut here can only ever drop from the
//     stale tail — a stale rule is demoted below the budget line, never
//     served first, by construction rather than by a second sort.
//  2. Each rule carries its cited lines inlined (withSnippets/BP-20,
//     reusing symbol_context's read_source-backed snippet mechanism) so a
//     trusted rule costs no extra round trip.
//  3. displaced is broken out per governing-scope tier (VB-8's
//     rankByGoverningScope, semantic.Obligations.DisplacedByTier) — a
//     caller can see WHAT KIND of rule the cap dropped: a root-level
//     catch-all crowding out a package rule reads very differently than
//     the reverse.
func renderObligationReceipt(ctx context.Context, backend QueryInterface, service string, obl semantic.Obligations, rootScope bool, servedLayer string, includeSnippets, indexStale bool) (map[string]any, string) {
	result := map[string]any{
		"service": obl.Service,
		"file":    obl.File,
	}
	// coverageSkipped (RT-146): set when a hook-origin whole-tree read was
	// refused for the raw fallback or the layer coverage below.
	var coverageSkipped string
	if obl.AsOf != "" {
		result["as_of"] = obl.AsOf
	}
	if rootScope {
		result["root"] = true
		result["note"] = "Root-scope obligations only (repo-root/CLAUDE.md-level rules) — pass file for file-scoped rules."
	}
	if servedLayer != "" {
		result["layer"] = servedLayer
	}
	// DP-8 (ADR-042 decision 2, O109): ranked_by is ALWAYS present — mirrors
	// semantic.Obligations.RankedBy's own doc comment ("Always present,
	// never omitempty") — so a caller can tell "reranking did not run"
	// ("none") from "this field does not exist yet" without a second
	// signal. rank_note rides beside it only when a configured Decider's
	// own Decide call failed (semantic.Obligations.RankNote's doc
	// comment); both were computed correctly on obl by
	// semantic.ObligationsForOpts/ForLayerOpts/AsOfOpts all along — this
	// function simply never read them onto the wire before O109 caught a
	// live client seeing neither field at all.
	result["ranked_by"] = obl.RankedBy
	if obl.RankNote != "" {
		result["rank_note"] = obl.RankNote
	}
	// DP-24 (O109): ranked_count/candidates_total disclose how many
	// scope-eligible claims the DP-7 rerank actually asked the Decider
	// about (ranked_count) out of how many existed before the served-
	// window tier filter narrowed that down (candidates_total) — omitted
	// together whenever no rerank ran at all (RankWithinTier off, no
	// Decider, or nothing scope-eligible), the same condition that leaves
	// both semantic.Obligations fields at zero.
	if obl.RankedCount > 0 {
		result["ranked_count"] = obl.RankedCount
	}
	if obl.CandidatesTotal > 0 {
		result["candidates_total"] = obl.CandidatesTotal
	}

	// wave O1 / ADR-037: fill a real coverage gap with raw guide content
	// BEFORE the rules below are snippet-rendered and byte-budgeted, so a
	// raw row gets the exact same inlined citation and cap treatment a
	// compiled row gets — never a second-class payload shape. Prepended,
	// never appended: a raw row exists specifically because it is NEARER
	// than what compiled — a nested guide's own section, say, when only a
	// root-tier rule managed to compile — so it must not be displaced
	// below a farther compiled row that survived only because the nearer
	// one never got the chance to compete for a claim in the first place.
	// Skipped for a root-scope call (no single file to rank sections
	// against) and for the "claims" layer filter (raw sections are
	// human-authored governing rules — RoleGoverns material — never a
	// plan-row binding). A backend that omits rawFallbackBackend (an older
	// module, a minimal test fake) is untouched — existing goldens for a
	// complete layer render byte-identically, since a real gap is required
	// before this ever adds a row.
	if !rootScope && obl.File != "" && obl.AsOf == "" && servedLayer != string(semantic.ObligationLayerClaims) {
		if rfb, ok := backend.(rawFallbackBackend); ok {
			rows, _, rok, rerr := rfb.RawFallbackObligations(ctx, service, obl.File, semantic.RawFallbackReserve)
			switch {
			case errors.Is(rerr, core.ErrSourcesBusy):
				// RT-146 / I38: the hook's whole-tree read was skipped, so
				// the raw fallback could not be computed — named below as
				// coverage_skipped, never a silent "no gap".
				coverageSkipped = core.ErrSourcesBusy.Error()
			case errors.Is(rerr, tenancy.ErrOverBudget):
				// RT-153 / I38: the governor halted the guide reads.
				coverageSkipped = rerr.Error()
			case rerr == nil && rok && len(rows) > 0:
				obl.Rules = append(rows, obl.Rules...)
			}
		}
	}

	// summary (wave L1): computed from obl.Rules right here — AFTER the raw
	// fallback prepend above, so the summary never lags the priority order
	// the "rules" field itself will render — never from result["rules"]
	// (only built below, and shaped differently per includeSnippets).
	// BA-4's "never refuses" guarantee (elisionFor's own doc comment) means
	// fitToolBudget below always keeps at least rendered[0], so obl.Rules[0]
	// here is always rules[0] on the wire.
	summary := obligationsSummary(obl, rootScope)

	// stale_count/remedy (J2, app-kit O6/O16/O24/O26/O29/O30): a stale or
	// contradicted rule is still served (flagged, never silently dropped —
	// Obligation.Status's own doc comment), but a caller reading only
	// "rules" must not have to scan every row's status to notice one is
	// non-binding. Computed from obl.Rules itself — the raw-fallback rows
	// prepended above never carry a Status, so they never count. Absent
	// (both fields) when nothing is stale: byte-identical to the payload
	// before this field existed.
	staleCount, staleLayer := 0, ""
	for _, r := range obl.Rules {
		if r.Status == "" {
			continue
		}
		staleCount++
		if staleLayer == "" && r.Layer != "" {
			staleLayer = r.Layer
		}
	}
	if staleCount > 0 {
		result["stale_count"] = staleCount
		result["remedy"] = staleRemedy(staleLayer, service)
	}

	displaced := obl.Displaced
	if len(obl.Rules) > 0 {
		var rendered []any
		if includeSnippets {
			// BP-20/D3: inline each rule's cited lines so the agent stops
			// paying a read_source turn per rule it trusts. Default ON for
			// the TOOL (an agent calling obligations directly is about to
			// read the citation anyway) and OFF for the EDIT HOOK, which
			// passes include_snippets:false — the decision-moment payload's
			// budget is ~320 tokens and its rendered bytes must not move.
			rendered = withSnippets(ctx, backend, service, obl.Rules)
		} else {
			rendered = make([]any, len(obl.Rules))
			for i, r := range obl.Rules {
				rendered[i] = r
			}
		}
		kept, byteElided := fitToolBudget(rendered, defaultCharBudget)
		result["rules"] = kept
		// A byte-budget cut is the SAME kind of eviction the MaxObligations
		// cap already reports — fold it into the same honest integer rather
		// than introducing a second, differently-named count.
		displaced += byteElided
	}
	if obl.Watermark != "" {
		result["compiled_at_watermark"] = obl.Watermark
	}
	// T5-2/O14: surface how many served-eligible claims a cap dropped this
	// call — a recompile that blends a new layer in can silently evict a
	// rule an agent relied on yesterday; displaced turns that into a
	// visible integer instead of silence.
	if displaced > 0 {
		result["displaced"] = displaced
	}
	if len(obl.DisplacedByTier) > 0 {
		result["displaced_by_tier"] = obl.DisplacedByTier
	}
	if obl.DisplacedRepo > 0 {
		result["displaced_repo"] = obl.DisplacedRepo
	}
	if obl.DisplacedModule > 0 {
		result["displaced_module"] = obl.DisplacedModule
	}
	if len(obl.ModuleNotes) > 0 {
		result["module_notes"] = obl.ModuleNotes
	}
	if indexStale {
		result["index_stale"] = true
	}
	// O54 deliverable 2 / O60: coverage disclosure on every response, when
	// the backend can report it — the same execute-and-flag doctrine
	// ADR-014 already applies elsewhere, applied to the compiled layer's
	// OWN corpus completeness. A backend that never implements this
	// capability at all (an older module, a minimal test fake) carries no
	// layer_coverage — an honest "coverage unknown" by omission, never a
	// fabricated full-coverage claim. But a REAL backend that implements it
	// and answers found:false (no compile_runs row for this service — a
	// layer compiled before that tracking existed is the live case) must
	// NOT collapse into that same silent omission: without a key at all, a
	// stranger reading the envelope cannot tell a complete layer from an
	// unrecorded one. Observed live on the operator daemon: an obligations
	// response carrying compiled_at_watermark, displaced, file, layer,
	// note, root, rules, service — no layer_coverage anywhere, even though
	// rules were serving fine. So that case gets an explicit
	// status:"unknown" placeholder instead of silence.
	if cb, ok := backend.(layerCoverageBackend); ok && obl.AsOf == "" {
		cov, found, cerr := cb.ObligationsLayerCoverage(ctx, service)
		switch {
		case errors.Is(cerr, core.ErrSourcesBusy), errors.Is(cerr, tenancy.ErrOverBudget):
			// RT-146 / RT-153 / I38: coverage could not be measured this
			// call (a busy tree read, or the governor halted it) — named,
			// never omitted as if there were nothing to report.
			coverageSkipped = core.ErrSourcesBusy.Error()
			if errors.Is(cerr, tenancy.ErrOverBudget) {
				coverageSkipped = cerr.Error()
			}
			result["layer_coverage"] = map[string]any{"status": "skipped", "note": coverageSkipped}
		case cerr != nil:
			// An actual RPC/store error answering the coverage question is
			// left exactly as before: no layer_coverage key. Distinct from
			// found:false, which is an honest answer, not a failure.
		case !found:
			placeholder := map[string]any{
				"status": "unknown",
				"note":   "layer compiled before coverage was recorded; recompile to record coverage",
			}
			// I38: a guide the live coverage could not read is named even
			// when the numbers are unknown — never dropped with them.
			if len(cov.UnreadCorpus) > 0 {
				placeholder["unread_corpus"] = cov.UnreadCorpus
			}
			result["layer_coverage"] = placeholder
		default:
			result["layer_coverage"] = cov
			// The queried file's nearest guide was never compiled: name it,
			// with the resume command, rather than let a caller read a
			// partial layer as if it were the whole answer. Never fires for
			// a root-scope call (obl.File is always "" there — no path to
			// be nearest to) and never overwrites the root-scope note set
			// above, since the two are mutually exclusive by construction.
			if guide, gok := semantic.NearestUncompiledGuide(cov, obl.File); gok {
				coverage := fmt.Sprintf("%d/%d sections compiled", cov.SectionsCompiled, cov.SectionsTotal)
				if cov.BudgetStopped {
					coverage = fmt.Sprintf("budget stopped at %d/%d sections", cov.SectionsCompiled, cov.SectionsTotal)
				}
				result["note"] = fmt.Sprintf("%s governs this path and was not compiled: %s; run `fathom compile --layer obligations --continue`", guide, coverage)
			}
		}
	}
	if coverageSkipped != "" {
		result["coverage_skipped"] = coverageSkipped
	}
	// SR-27 (surface reckoning F-35): the retired spec_refs tool's one
	// real consumer (the edit hook, 306 calls) reads this additive
	// section instead of a second RPC. Same gate as the raw-fallback
	// prepend above: a real file, not a root-scope or as-of (historical)
	// call — obligationsSpecRefs itself never errors, only degrades to
	// nil, so this never affects the rest of the answer.
	if !rootScope && obl.File != "" && obl.AsOf == "" {
		if refs := obligationsSpecRefs(ctx, backend, service, obl.File); len(refs) > 0 {
			result["spec_refs"] = refs
		}
	}
	return result, summary
}

// staleRemedy (J2) is the ONE precise remedy line every stale-rule surface
// (this tool's stale_count/remedy pair, and every hook's trailing stale
// block — cmd/fathom/stalerules.go's own copy) renders identically: layer
// is the first stale rule's own compiled layer, defaulting to the
// obligations layer when unset (a stale rule with no recorded layer is not
// an expected shape, but the remedy must never be empty).
func staleRemedy(layer, service string) string {
	if layer == "" {
		layer = semantic.LayerObligations
	}
	return fmt.Sprintf("fathom compile --layer %s --service %s", layer, service)
}

// obligationsSummary renders obligations' answer-first sentence (wave L1):
// "governing rule for <file>: <rule head> (tier, source:line)" — or the
// root-scope/no-rule equivalent — built from obl.Rules[0], the exact row
// that (BA-4's never-refuses guarantee) always survives into "rules"[0] on
// the wire.
func obligationsSummary(obl semantic.Obligations, rootScope bool) string {
	subject := "governing rule for " + obl.File
	if rootScope {
		subject = "root-scope governing rule"
	}
	if len(obl.Rules) == 0 {
		if rootScope {
			return "no root-scope governing rules found"
		}
		return "no governing rule found for " + obl.File
	}
	r := obl.Rules[0]
	head := r.Rule
	const headMaxChars = 70
	if utf8.RuneCountInString(head) > headMaxChars {
		runes := []rune(head)
		head = strings.TrimRight(string(runes[:headMaxChars]), " ") + "…"
	}
	return capSummary(fmt.Sprintf("%s: %s (%s, %s)", subject, head, r.Tier, r.Src))
}

// ─── registerSemanticTools ───────────────────────────────────────────────────

// registerSemanticTools wires the obligations and compile_layer MCP tools.
// modules may be nil (no --module store mounted on this daemon); when set,
// the obligations handler additionally consults the service's
// .fathom/modules.cue (docs/design/platform-modules.md §3.3) via
// internal/modserve.Bind.
func registerSemanticTools(s *Server, backend QueryInterface, modules moduleBackend) {
	// obligations: return the compiled rules governing a file at decision time.
	// Returns empty rules when no rules apply (zero-output-on-clean contract).
	s.registerTool(toolDescriptor{
		Name:        "obligations",
		Description: "Governing rules for a file, or (no file) root-scope rules. This result IS the answer for what governs it; a follow-up call only helps after a change. Reach for it when about to edit. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"file": map[string]any{
					"type":        "string",
					"description": "File being edited; omit for root scope",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Alias for file",
				},
				"layer": map[string]any{
					"type":        "string",
					"enum":        []string{"rules", "claims", "all"},
					"description": "Which claims to serve: \"rules\" (default), \"claims\" (plan-row bindings), or \"all\".",
				},
				"max_rules": map[string]any{
					"type":        "integer",
					"description": "Max rules (default 6)",
				},
				"include_snippets": map[string]any{
					"type":        "boolean",
					"description": "Inline cited lines (default true)",
				},
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"refute", "corroborate"},
					"description": "Records a verdict instead of listing rules",
				},
				"claim_id": map[string]any{
					"type": "string",
				},
				"note": map[string]any{
					"type": "string",
				},
				"author": map[string]any{
					"type": "string",
				},
				"as_of": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Service         string `json:"service"`
			File            string `json:"file"`
			Path            string `json:"path"` // alias for File — see the "path" schema entry above (O82)
			Layer           string `json:"layer"`
			MaxRules        int    `json:"max_rules"`
			IncludeSnippets *bool  `json:"include_snippets"`
			Action          string `json:"action"`
			ClaimID         string `json:"claim_id"`
			Note            string `json:"note"`
			Author          string `json:"author"`
			AsOf            string `json:"as_of"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: service required"}
		}
		// O82: `path` aliases `file` — a caller that reached for the more
		// common name used to silently fall back to root-scope rules
		// (rootScope := p.File == "" read true because the operator's
		// argument landed in a field this handler never read). `file`
		// stays authoritative when both are given.
		if p.File == "" && p.Path != "" {
			p.File = p.Path
		}

		// D2 feedback action (docs/plans/expert-depth.md): a non-empty action
		// switches this call from the file-lookup path to the refute/
		// corroborate write path — the already-hot obligations channel
		// doubling as claim_feedback rather than shipping a new tool.
		if p.Action != "" {
			if p.Action != "refute" && p.Action != "corroborate" {
				return nil, &rpcError{Code: ErrCodeParams, Message: `obligations: action must be "refute" or "corroborate"`}
			}
			if p.ClaimID == "" {
				return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: claim_id required for feedback"}
			}
			if p.Note == "" {
				return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: note required for feedback"}
			}
			fb, ok := backend.(claimFeedbackBackend)
			if !ok {
				return nil, &rpcError{Code: ErrCodeInvalid, Message: "backend does not support claim feedback"}
			}
			if err := fb.ClaimFeedback(ctx, p.Service, p.ClaimID, p.Action, p.Note, p.Author); err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
			}
			return map[string]any{"service": p.Service, "claim_id": p.ClaimID, "action": p.Action}, nil
		}

		// SU-19 (ADR-041 decision 7): as_of switches this call from "what
		// governs file now" to "what governed it AT this instant" — a
		// separate, slower read (semantic.ObligationsAsOf's own doc
		// comment), so it is handled here rather than folded into the live
		// obligationLayers path below. File-scoped only: a root-scope
		// as_of answer is not yet supported (deliberately out of this
		// row's scope — see the plan).
		if p.AsOf != "" {
			if p.File == "" {
				return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: as_of requires file (root-scope as_of is not supported)"}
			}
			aob, ok := backend.(obligationsAsOfBackend)
			if !ok {
				return nil, &rpcError{Code: ErrCodeInvalid, Message: "backend does not support as_of"}
			}
			// GL-17: one resolver for every verb that takes an as_of.
			asOf, aerr := newAsOfFilter(ctx, backend, p.Service, p.AsOf, "obligations")
			if aerr != nil {
				return nil, aerr
			}
			obl, oerr := aob.ObligationsAsOf(ctx, p.Service, p.File, p.MaxRules, asOf.instant())
			if oerr != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: oerr.Error()}
			}
			var indexStale bool
			var indexUnchecked string
			if ob, ok := backend.(obligationsBackend); ok {
				indexStale, indexUnchecked = obligationsIndexFreshness(ctx, backend, ob, p.Service, p.File)
			}
			includeSnippets := p.IncludeSnippets == nil || *p.IncludeSnippets
			body, summary := renderObligationReceipt(ctx, backend, p.Service, obl, false, string(semantic.ObligationLayerAll), includeSnippets, indexStale)
			if indexUnchecked != "" {
				body["index_unchecked"] = indexUnchecked
			}
			return withSummary(summary, body), nil
		}

		ob, ok := backend.(obligationsBackend)
		if !ok {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "backend does not serve semantic layers"}
		}

		layer, lerr := validObligationLayer(p.Layer)
		if lerr != nil {
			return nil, lerr
		}

		// RC-22 (docs/vision/28-receipts.md §3.5): no file means "what
		// governs this repository" — root-scope rules only, never an
		// error. A real file argument keeps the exact pre-RC-22 behavior
		// below, module union included.
		rootScope := p.File == ""

		obl, servedLayer, oerr := obligationLayers(ctx, backend, ob, p.Service, p.File, p.MaxRules, layer, rootScope)
		if oerr != nil {
			return nil, oerr
		}

		// Platform-modules.md §3.3: union in rules from every module this
		// service binds via its own .fathom/modules.cue, when this daemon
		// has any --module store mounted at all. modules is nil on a
		// daemon started with no --module flags -- modserve.Bind itself
		// also no-ops on a nil host, so this call is a pure
		// backward-compatible no-op for every pre-phase-O deployment.
		// Skipped for the root-scope call above: module bindings are a
		// file-scoped federation (a real path is what modserve.Bind keys
		// its union off of), out of RC-22's "root conventions" shape.
		if modules != nil && !rootScope {
			max := p.MaxRules
			if max <= 0 || max > semantic.MaxObligations {
				max = semantic.MaxObligations
			}
			if path, _, found, serr := backend.ServiceMeta(ctx, p.Service); serr == nil && found && path != "" {
				obl = modserve.Bind(ctx, modules, path, p.File, max, obl)
			}
		}

		staleFile := p.File
		if rootScope {
			staleFile = rootObligationsProbe
		}
		indexStale, indexUnchecked := obligationsIndexFreshness(ctx, backend, ob, p.Service, staleFile)
		includeSnippets := p.IncludeSnippets == nil || *p.IncludeSnippets
		body, summary := renderObligationReceipt(ctx, backend, p.Service, obl, rootScope, servedLayer, includeSnippets, indexStale)
		if indexUnchecked != "" {
			body["index_unchecked"] = indexUnchecked
		}
		return withSummary(summary, body), nil
	})

	// compile_layer: trigger obligations compilation for a service. Defaults to
	// dry_run:true so cost is estimated before any spending commit.
	s.registerTool(toolDescriptor{
		Name:        "compile_layer",
		Description: "Compile a semantic layer for a service. dry_run defaults true (estimate cost before spending). Layer must be defined in the semantic-layer manifest (schema/semantic/schema.cue); see ADR-016.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"layer": map[string]any{
					"type":        "string",
					"description": "Layer name (default: obligations)",
				},
				"dry_run": map[string]any{
					"type":        "boolean",
					"description": "When true (default), estimate cost without calling the LLM or writing claims",
				},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		cb, ok := backend.(compileBackend)
		if !ok {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "backend does not support layer compilation"}
		}
		var p struct {
			Service string  `json:"service"`
			Layer   *string `json:"layer"`
			DryRun  *bool   `json:"dry_run"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "compile_layer: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "compile_layer: service required"}
		}
		layer := "obligations"
		if p.Layer != nil {
			layer = *p.Layer
		}
		// LC-3 (docs/plans/layer-catalog.md): layer names are no longer
		// embedded-only — a project's .fathom/layers/ is part of the valid
		// set too, rooted at the SERVICE's own path, which this handler has
		// no way to resolve (only cb.CompileLayer does, via ServiceMeta). A
		// pre-check against semantic.LoadLayer here would reject a valid
		// project-local layer before ever reaching the backend's real
		// resolution — removed; cb.CompileLayer below is the single source
		// of truth for "is this layer valid" and surfaces its own error.
		// dry_run defaults TRUE — spending requires explicit dry_run:false.
		dryRun := true
		if p.DryRun != nil {
			dryRun = *p.DryRun
		}

		run, err := cb.CompileLayer(ctx, p.Service, layer, dryRun)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		result := map[string]any{
			"run_id":            run.RunID,
			"service":           run.ServiceID,
			"layer":             run.Layer,
			"dry_run":           run.DryRun,
			"sections_total":    run.SectionsTotal,
			"sections_compiled": run.SectionsCompiled,
			"sections_failed":   run.SectionsFailed,
			"claims_emitted":    run.ClaimsEmitted,
			"claims_rejected":   run.ClaimsRejected,
			"tokens_in":         run.TokensIn,
			"tokens_out":        run.TokensOut,
			"cost_usd":          run.CostUSD,
			"duration_ms":       run.DurationMS,
			"model":             run.Model,
			"prompt_version":    run.PromptVersion,
			"ran_at":            run.RanAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		// A budget-stopped run returns nil error and is otherwise
		// indistinguishable from success — surface it (F5, M1-7). Refused
		// runs surface via the error path above, but keep the fields
		// symmetric in case a backend ever ledgers-and-returns.
		if run.BudgetStopped {
			result["budget_stopped"] = true
		}
		if run.Refused {
			result["refused"] = true
		}
		if run.RefusalReason != "" {
			result["refusal_reason"] = run.RefusalReason
		}
		// DP-6 (ADR-042 decision 2, O109): the decision-port pre-filter's
		// own disclosure — how many sections it skipped before compilation
		// (never compiled, never failed) and, when the core type carries
		// it, which ones by stable section id. core.CompileRun.
		// SectionsPrefiltered/PrefilteredSections were already correctly
		// populated by the compiler; this handler hand-copies named fields
		// off CompileRun and had never copied these two, so a live
		// --dry-run whose pre-filter genuinely ran and skipped sections
		// reported nothing about it. Omitted when zero — a run with no
		// Decider configured (or nothing skipped) stays byte-identical to
		// before these fields existed.
		if run.SectionsPrefiltered > 0 {
			result["sections_prefiltered"] = run.SectionsPrefiltered
		}
		if len(run.PrefilteredSections) > 0 {
			result["prefiltered_sections"] = run.PrefilteredSections
		}
		return result, nil
	})
}

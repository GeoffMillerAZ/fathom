package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modserve"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ─── optional backend interfaces ─────────────────────────────────────────────

// obligationsBackend is the optional backend capability for serving compiled
// obligations to a coding agent. pkg/client implements it; test fakes may
// omit it.
type obligationsBackend interface {
	Obligations(ctx context.Context, service, file string, maxRules int) (semantic.Obligations, error)
	ObligationsIndexStale(ctx context.Context, service, file string) bool
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
		Name: "obligations",
		Description: "Return compiled obligations (governing rules) for a file at decision time, or — with no file and no action — the root-scope rules that answer \"what governs this repository\" (RC-22): the compiled layer's rules whose scope reaches the repo root / CLAUDE.md level, budgeted the same way, with root:true and a note to pass file for file-scoped rules. Empty rules with index_stale:false means a clean file. Each rule carries a layer label (which manifest layer compiled it) and, when federated in from a bound platform module (docs/design/platform-modules.md), a module label naming it. displaced:N (when present) is the total count of served-eligible rules the shared cap dropped this call; displaced_repo/displaced_module split it once a binding is configured. module_notes (when present) names any bound module this daemon does not have mounted — never silent. This result IS the answer for which rules govern this file (or the repository) right now — a follow-up call only helps once the file or the rule set changes, or to record a verdict via action. " +
			"Also serves the claim feedback write path (D2): pass action:\"refute\"|\"corroborate\" with claim_id and note instead of file to record a verdict — refute flips the claim's status and stops it serving; corroborate never changes status or tier. Reach for it when you are about to edit a file and the hook did not run (a harness without PostToolUse hooks), to ask what governs the whole repository, or to record a verdict on a served claim. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"file": map[string]any{
					"type":        "string",
					"description": "Repo-relative file path being edited. Omit (with no action) for root-scope obligations — \"what governs this repository\" — instead of an error.",
				},
				"max_rules": map[string]any{
					"type":        "integer",
					"description": "Maximum rules to return (default 6)",
				},
				"include_snippets": map[string]any{
					"type":        "boolean",
					"description": "Inline each rule's cited lines as snippet, capped at 240 bytes (default true). Set false for the smallest possible payload.",
				},
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"refute", "corroborate"},
					"description": "Feedback verb (D2): when set, this call records a claim_events verdict instead of looking up rules. Requires claim_id and note.",
				},
				"claim_id": map[string]any{
					"type":        "string",
					"description": "Claim ID being refuted/corroborated (required when action is set)",
				},
				"note": map[string]any{
					"type":        "string",
					"description": "Verdict rationale (required when action is set)",
				},
				"author": map[string]any{
					"type":        "string",
					"description": "Verdict author (optional; empty for an automated caller)",
				},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Service         string `json:"service"`
			File            string `json:"file"`
			MaxRules        int    `json:"max_rules"`
			IncludeSnippets *bool  `json:"include_snippets"`
			Action          string `json:"action"`
			ClaimID         string `json:"claim_id"`
			Note            string `json:"note"`
			Author          string `json:"author"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: service required"}
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

		ob, ok := backend.(obligationsBackend)
		if !ok {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "backend does not serve semantic layers"}
		}

		// RC-22 (docs/vision/28-receipts.md §3.5): no file means "what
		// governs this repository" — root-scope rules only, never an
		// error. A real file argument keeps the exact pre-RC-22 behavior
		// below, module union included.
		rootScope := p.File == ""

		var obl semantic.Obligations
		var err error
		if rootScope {
			obl, err = rootObligations(ctx, backend, ob, p.Service, p.MaxRules)
		} else {
			obl, err = ob.Obligations(ctx, p.Service, p.File, p.MaxRules)
		}
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
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

		// Build response: always include obligations payload; append index_stale
		// when true. Serialise as map so index_stale is a first-class field.
		result := map[string]any{
			"service": obl.Service,
			"file":    obl.File,
		}
		if rootScope {
			result["root"] = true
			result["note"] = "Root-scope obligations only (repo-root/CLAUDE.md-level rules) — pass file for file-scoped rules."
		}
		if len(obl.Rules) > 0 {
			// BP-20/D3: inline each rule's cited lines so the agent stops
			// paying a read_source turn per rule it trusts. Default ON for
			// the TOOL (an agent calling obligations directly is about to
			// read the citation anyway) and OFF for the EDIT HOOK, which
			// passes include_snippets:false — the decision-moment payload's
			// budget is ~150 tokens and its rendered bytes must not move.
			if p.IncludeSnippets == nil || *p.IncludeSnippets {
				result["rules"] = withSnippets(ctx, backend, p.Service, obl.Rules)
			} else {
				result["rules"] = obl.Rules
			}
		}
		if obl.Watermark != "" {
			result["compiled_at_watermark"] = obl.Watermark
		}
		// T5-2/O14: surface how many served-eligible claims the
		// MaxObligations cap dropped this call — a recompile that blends a
		// new layer in can silently evict a rule an agent relied on
		// yesterday; displaced turns that into a visible integer instead of
		// silence. O14 split (platform-modules.md §3.3 deliverable 2):
		// displaced stays the TOTAL (back-compat superset); displaced_repo/
		// displaced_module appear only once a binding actually contributed.
		if obl.Displaced > 0 {
			result["displaced"] = obl.Displaced
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
		staleFile := p.File
		if rootScope {
			staleFile = rootObligationsProbe
		}
		if ob.ObligationsIndexStale(ctx, p.Service, staleFile) {
			result["index_stale"] = true
		}
		return result, nil
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
		return result, nil
	})
}

package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/core"
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

// ─── registerSemanticTools ───────────────────────────────────────────────────

// registerSemanticTools wires the obligations and compile_layer MCP tools.
func registerSemanticTools(s *Server, backend QueryInterface) {
	// obligations: return the compiled rules governing a file at decision time.
	// Returns empty rules when no rules apply (zero-output-on-clean contract).
	s.registerTool(toolDescriptor{
		Name: "obligations",
		Description: "Return compiled obligations (governing rules) for a file at decision time. Empty rules with index_stale:false means a clean file. " +
			"Also serves the claim feedback write path (D2): pass action:\"refute\"|\"corroborate\" with claim_id and note instead of file to record a verdict — refute flips the claim's status and stops it serving; corroborate never changes status or tier.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"file": map[string]any{
					"type":        "string",
					"description": "Repo-relative file path being edited (omit when action is set)",
				},
				"max_rules": map[string]any{
					"type":        "integer",
					"description": "Maximum rules to return (default 6)",
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
			Service  string `json:"service"`
			File     string `json:"file"`
			MaxRules int    `json:"max_rules"`
			Action   string `json:"action"`
			ClaimID  string `json:"claim_id"`
			Note     string `json:"note"`
			Author   string `json:"author"`
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

		if p.File == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "obligations: file required"}
		}
		ob, ok := backend.(obligationsBackend)
		if !ok {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "backend does not serve semantic layers"}
		}

		obl, err := ob.Obligations(ctx, p.Service, p.File, p.MaxRules)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}

		// Build response: always include obligations payload; append index_stale
		// when true. Serialise as map so index_stale is a first-class field.
		result := map[string]any{
			"service": obl.Service,
			"file":    obl.File,
		}
		if len(obl.Rules) > 0 {
			result["rules"] = obl.Rules
		}
		if obl.Watermark != "" {
			result["compiled_at_watermark"] = obl.Watermark
		}
		if ob.ObligationsIndexStale(ctx, p.Service, p.File) {
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

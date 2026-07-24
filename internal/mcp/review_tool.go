// Package mcp — review_diff, the program's ONE new MCP tool (C1-1, docs/
// plans/review-verb.md): composes diff impact + obligations/findings +
// architecture conformance + spec coverage into ONE cited verdict over an
// explicit changed-files list, replacing the 8-step manual sequence
// docs/use-cases/change-review.md previously documented tool-by-tool.
package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
)

// reviewClaimsBackend is the optional backend capability review_diff's
// findings bridge (C2-1, FT-7's pre-armed trigger) uses to persist review
// findings as content-addressed claims and read back a prior verdict's
// status. pkg/client.Client implements it via PutClaims/ClaimsForService
// (thin store passthroughs). Test fakes may omit it: review_diff still
// returns a full verdict, with findings_persisted:false and no claim IDs
// attached — the composed analysis never depends on this capability.
type reviewClaimsBackend interface {
	PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)
}

func registerReviewDiff(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "review_diff",
		Description: "Compose diff impact (blast radius, typed-edge aware) + obligations/findings scoped to the changed files + architecture conformance (project rule pack) + spec coverage into ONE cited verdict for a set of changed files. Each dimension reports checked:false with a reason instead of a clean-shaped pass when it cannot run (e.g. no architecture rule pack configured). When the backend supports claim persistence, findings are written as content-addressed claims on the \"findings\" layer: a finding refuted via `fathom claim refute` or the obligations tool's feedback action stays refuted (status:\"refuted\", excluded from count, but still listed) across every later re-review that re-emits the identical finding.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":       map[string]any{"type": "string", "description": "Service / repo identifier"},
				"changed_files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Repo-relative changed file paths (e.g. from git diff --name-only)"},
				"concern":       map[string]any{"type": "string", "description": "Optional concern token (currently descriptive only; carried for forward compatibility)"},
			},
			"required": []string{"service", "changed_files"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service      string   `json:"service"`
			ChangedFiles []string `json:"changed_files"`
			Concern      string   `json:"concern"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "review_diff: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "review_diff: service required"}
		}
		if len(p.ChangedFiles) == 0 {
			return nil, &rpcError{Code: ErrCodeParams, Message: "review_diff: changed_files required"}
		}

		in, rpcErr := buildReviewInput(ctx, backend, p.Service, p.ChangedFiles, p.Concern)
		if rpcErr != nil {
			return nil, rpcErr
		}
		v := review.Run(in)
		persisted := applyAndPersistFindings(ctx, backend, &v)

		return v.Payload(persisted), nil
	})
}

// buildReviewInput does every I/O review.Run itself never performs:
// ListAllSymbols/TypedEdges for impact, per-changed-file Obligations for the
// obligations dimension, ServiceMeta + internal/projectrules.ResolveRules
// for arch_conformance (review_diff never accepts inline arch rules — it
// always resolves through the service's own project rule pack, or reports
// checked:false), and the changed files' own source content for
// spec_coverage.
func buildReviewInput(ctx context.Context, backend QueryInterface, service string, changedFiles []string, concern string) (review.Input, *rpcError) {
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil {
		return review.Input{}, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
	}
	typed, _ := backend.TypedEdges(ctx, service)

	in := review.Input{
		Service:      service,
		ChangedFiles: changedFiles,
		Concern:      concern,
		Nodes:        nodes,
		TypedEdges:   typed,
	}

	if ob, ok := backend.(obligationsBackend); ok {
		in.ObligationsSupported = true
		in.ObligationsByFile = make(map[string][]review.ObligationRule, len(changedFiles))
		for _, f := range changedFiles {
			obl, oerr := ob.Obligations(ctx, service, f, 0)
			if oerr != nil {
				continue // best-effort per file: a lookup error degrades that file's rules, not the whole dimension
			}
			for _, r := range obl.Rules {
				in.ObligationsByFile[f] = append(in.ObligationsByFile[f], review.ObligationRule{
					File: f, Rule: r.Rule, Src: r.Src, Tier: r.Tier, Kind: r.Kind, Status: r.Status,
				})
			}
		}
	}

	corpusRoot := ""
	if path, _, found, metaErr := backend.ServiceMeta(ctx, service); metaErr == nil && found {
		corpusRoot = path
	}
	rules, source, deny, checked, reason, resolveErr := projectrules.ResolveRules(corpusRoot, nil, false)
	if resolveErr != nil {
		return review.Input{}, &rpcError{Code: ErrCodeInternal, Message: "review_diff: project rules: " + resolveErr.Error()}
	}
	in.ArchRules = rules
	in.ArchRuleSource = source
	in.ArchDenyByDefault = deny
	in.ArchChecked = checked
	in.ArchReason = reason

	if files, serr := backend.Sources(ctx, service); serr == nil {
		changedSet := make(map[string]bool, len(changedFiles))
		for _, f := range changedFiles {
			changedSet[f] = true
		}
		for _, f := range files {
			if changedSet[f.Path] {
				in.ChangedSources = append(in.ChangedSources, f)
			}
		}
	}

	return in, nil
}

// applyAndPersistFindings runs the C2-1 findings bridge when the backend
// supports it: read the service's existing findings-layer claims, attach
// content-addressed claim IDs to v's findings, label any that are already
// refuted (excluding them from their dimension's active Count without
// dropping them from Findings), then persist this run's claims — relying
// on the store's own refutation-durability upsert, never re-deriving that
// logic here. Returns whether persistence happened; false (with v's
// findings left ClaimID-less) when the backend does not implement
// reviewClaimsBackend at all.
func applyAndPersistFindings(ctx context.Context, backend QueryInterface, v *review.Verdict) bool {
	rcb, ok := backend.(reviewClaimsBackend)
	if !ok {
		return false
	}
	existing, _ := rcb.ClaimsForService(ctx, v.Service, review.ClaimsLayer)

	watermark := ""
	if _, indexedCommit, found, err := backend.ServiceMeta(ctx, v.Service); err == nil && found {
		watermark = indexedCommit
	}
	claims, links := review.AttachClaims(v, time.Now, watermark)
	review.ApplyClaimStatus(v, existing)

	if len(claims) == 0 {
		return true // nothing to persist is not a failure
	}
	return rcb.PutClaims(ctx, claims, links) == nil
}

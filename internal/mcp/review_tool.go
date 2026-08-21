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
// findings as content-addressed claims, read back prior verdict status,
// reconcile fixed findings (SetClaimStatus → superseded), and anchor each
// claim's scope-fp drift link (SourceFingerprint). pkg/client.Client
// implements all four as thin store passthroughs. Test fakes may omit it:
// review_diff still returns a full verdict, with findings_persisted:false
// and no claim IDs attached — the composed analysis never depends on this
// capability.
type reviewClaimsBackend interface {
	PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)
	SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error
	SourceFingerprint(ctx context.Context, serviceID, path string) (hash string, found bool, err error)
}

func registerReviewDiff(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "review_diff",
		Description: "Compose diff impact (blast radius, typed-edge aware) + obligations/findings scoped to the changed files + architecture conformance (project rule pack) + spec coverage into ONE cited verdict for a set of changed files. Each dimension reports checked:false with a reason instead of a clean-shaped pass when it cannot run (e.g. no architecture rule pack configured). WRITES TO THE CLAIMS STORE when the backend supports persistence (findings_persisted reports it): arch_conformance and spec_coverage findings — the deterministic-oracle dimensions only, never obligations (which re-serves existing claims) or impact context — are persisted as content-addressed claims on the \"findings\" layer with hashed citation + scope-fingerprint anchors, and review-minted claims whose finding this run no longer emits are superseded (absent = fixed). A finding refuted via `fathom claim refute` or the obligations tool's feedback action stays refuted (status:\"refuted\", excluded from count, still listed) across every later re-review that re-emits the identical finding. For a report-only run without the store write, use the CLI mirror: fathom review --since <ref> --no-persist.",
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

		in, sources, rpcErr := buildReviewInput(ctx, backend, p.Service, p.ChangedFiles, p.Concern)
		if rpcErr != nil {
			return nil, rpcErr
		}
		v := review.Run(in)
		persisted, resolutions := applyAndPersistFindings(ctx, backend, &v, sources)

		payload := v.Payload(persisted)
		if vo := reviewVerdictOutcome(v, resolutions); vo != nil {
			payload["verdict_outcome"] = vo
		}
		return payload, nil
	})
}

// reviewVerdictOutcome computes review_diff's gate-shaped result summary
// (IT-7, docs/plans/instrument-truth.md), mirroring verify_plan's own
// verdict_outcome pattern (verify_plan_tool.go's verdictOutcome) so the
// call logger's existing extraction (calllog.go's verdictOutcomeFrom) picks
// review_diff calls up with zero logger changes:
//
//   - active: the sum of Count (active, non-refuted findings) across every
//     CHECKED dimension — exactly Verdict.Summary's own "active finding(s)"
//     figure.
//   - fixed / superseded: this run's reconciled claims (resolutions, from
//     applyAndPersistFindings — a supersededClaimID→ResolutionKind map)
//     split by review.ClassifyResolution's two outcomes: fixed is a
//     code-change resolution (the scoped file still exists — the issue was
//     actually fixed in place); superseded is a scope-disappearance
//     resolution (the scoped file is gone — reconciled, but not provably
//     fixed).
//
// Returns nil when no dimension ran at all: nothing gate-relevant happened
// in this call, so there is nothing to log (mirrors verdictOutcome's own
// >0-claimed opt-in gate for verify_plan).
func reviewVerdictOutcome(v review.Verdict, resolutions map[string]string) map[string]any {
	checked := false
	active := 0
	for _, d := range v.Dimensions {
		if d.Checked {
			checked = true
			active += d.Count
		}
	}
	if !checked {
		return nil
	}
	fixed, superseded := 0, 0
	for _, kind := range resolutions {
		switch kind {
		case core.ResolutionCodeChange:
			fixed++
		case core.ResolutionScopeDisappearance:
			superseded++
		}
	}
	return map[string]any{"active": active, "fixed": fixed, "superseded": superseded}
}

// buildReviewInput does every I/O review.Run itself never performs:
// ListAllSymbols/TypedEdges for impact, per-changed-file Obligations for the
// obligations dimension, ServiceMeta + internal/projectrules.ResolveRules
// for arch_conformance (review_diff never accepts inline arch rules — it
// always resolves through the service's own project rule pack, or reports
// checked:false), and the changed files' own source content for
// spec_coverage. The full source list is also returned: the persist step
// needs it to hash cited lines (semantic.Sweep's contract), including arch
// citations on files OUTSIDE the changed set.
func buildReviewInput(ctx context.Context, backend QueryInterface, service string, changedFiles []string, concern string) (review.Input, []core.SourceFile, *rpcError) {
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil {
		return review.Input{}, nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
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
		return review.Input{}, nil, &rpcError{Code: ErrCodeInternal, Message: "review_diff: project rules: " + resolveErr.Error()}
	}
	in.ArchRules = rules
	in.ArchRuleSource = source
	in.ArchDenyByDefault = deny
	in.ArchChecked = checked
	in.ArchReason = reason

	sources, serr := backend.Sources(ctx, service)
	if serr == nil {
		changedSet := make(map[string]bool, len(changedFiles))
		for _, f := range changedFiles {
			changedSet[f] = true
		}
		for _, f := range sources {
			if changedSet[f.Path] {
				in.ChangedSources = append(in.ChangedSources, f)
			}
		}
	}

	return in, sources, nil
}

// applyAndPersistFindings runs the C2-1 findings bridge when the backend
// supports it, via review.BuildPersist (the one composition shared with the
// CLI): read the service's existing findings-layer claims, attach
// content-addressed claim IDs (hashed cites + scope-fp anchors, so the
// daemon's post-refresh sweep and drift passes verify rather than falsify
// them), label already-refuted findings, persist this run's claims (the
// store's refutation-durability upsert, never re-derived here), and
// supersede review-minted claims this run no longer emits (absent = fixed).
// Returns whether persistence fully happened (false when the backend does
// not implement reviewClaimsBackend, or any write failed), plus a
// supersededClaimID→review.ClassifyResolution-kind map for every claim this
// run reconciled as fixed (IT-7's verdict_outcome fixed/superseded split,
// reviewVerdictOutcome) — classified against the PRE-run existing claims'
// own Scope and whether each scoped file is still present in sources (the
// full corpus source list, not just the changed files), mirroring
// cmd/fathom/review.go's runReview resolve closure exactly.
func applyAndPersistFindings(ctx context.Context, backend QueryInterface, v *review.Verdict, sources []core.SourceFile) (bool, map[string]string) {
	rcb, ok := backend.(reviewClaimsBackend)
	if !ok {
		return false, nil
	}
	existing, _ := rcb.ClaimsForService(ctx, v.Service, review.ClaimsLayer)
	existingByID := make(map[string]core.SemanticClaim, len(existing))
	for _, cl := range existing {
		existingByID[cl.ClaimID] = cl
	}

	watermark := ""
	if _, indexedCommit, found, err := backend.ServiceMeta(ctx, v.Service); err == nil && found {
		watermark = indexedCommit
	}
	content := make(map[string][]byte, len(sources))
	for _, f := range sources {
		content[f.Path] = f.Content
	}
	claims, links, supersede := review.BuildPersist(v, existing, review.PersistInput{
		Now:       time.Now,
		Watermark: watermark,
		FileContent: func(path string) ([]byte, bool) {
			data, ok := content[path]
			return data, ok
		},
		ScopeFingerprint: func(path string) (string, bool) {
			hash, found, ferr := rcb.SourceFingerprint(ctx, v.Service, path)
			return hash, ferr == nil && found
		},
	})

	okAll := true
	if len(claims) > 0 {
		if err := rcb.PutClaims(ctx, claims, links); err != nil {
			okAll = false
		}
	}
	fileExists := func(path string) bool { _, ok := content[path]; return ok }
	resolutions := make(map[string]string, len(supersede))
	for _, id := range supersede {
		if cl, ok := existingByID[id]; ok {
			resolutions[id] = review.ClassifyResolution(cl.Scope, fileExists)
		}
		if err := rcb.SetClaimStatus(ctx, v.Service, id, core.ClaimSuperseded); err != nil {
			okAll = false
		}
	}
	return okAll, resolutions
}

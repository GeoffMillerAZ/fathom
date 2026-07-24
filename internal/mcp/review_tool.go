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
		persisted := applyAndPersistFindings(ctx, backend, &v, sources)

		return v.Payload(persisted), nil
	})
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
// Returns whether persistence fully happened; false when the backend does
// not implement reviewClaimsBackend, or any write failed.
func applyAndPersistFindings(ctx context.Context, backend QueryInterface, v *review.Verdict, sources []core.SourceFile) bool {
	rcb, ok := backend.(reviewClaimsBackend)
	if !ok {
		return false
	}
	existing, _ := rcb.ClaimsForService(ctx, v.Service, review.ClaimsLayer)

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
	for _, id := range supersede {
		if err := rcb.SetClaimStatus(ctx, v.Service, id, core.ClaimSuperseded); err != nil {
			okAll = false
		}
	}
	return okAll
}

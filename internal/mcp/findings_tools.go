// Package mcp — findings_near, the §3.6 differentiating verb (docs/design/
// findings-as-evidence.md, §6 item 6): findings whose cited scope lies
// within N call-graph hops of a subject symbol/file, ranked by proximity
// then recency. Registered on the INVESTIGATION surface only (not the
// agent surface — no net tool growth there, per the mission's own
// instruction). Reuses internal/analysis/callgraph's existing traversal via
// internal/analysis/findings; adds no new storage.
package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/analysis/findings"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// findingsBackend is the optional backend capability findings_near needs:
// the service's finding-kind claims and the store-wide claim_events
// population (for resolution-outcome labeling via RankNear's Status/
// Superseded fields — no separate lookup needed since core.SemanticClaim
// already carries Status). pkg/client.Client implements both; a test fake
// may omit either (see the handler's own degrade path).
type findingsBackend interface {
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)
}

func registerFindingsNear(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "findings_near",
		Description: "Findings whose cited scope lies within N call-graph hops of a subject symbol or file (bidirectional: callers and callees), ranked by proximity then recency. The answer to \"has anyone seen this before?\" — prior review/audit precedent near the code you're investigating, each finding carrying its typed class, status, and (for a superseded finding) how it was resolved. Superseded findings are excluded unless include_superseded is true, and are ALWAYS labeled when included — precedent, never mistaken for something still live.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":            map[string]any{"type": "string", "description": "Service / repo identifier"},
				"subject":            map[string]any{"type": "string", "description": "A canonical symbol key (service::pkg.Name) or a repo-relative file path"},
				"hops":               map[string]any{"type": "integer", "description": "Call-graph hop radius (default 2)"},
				"include_superseded": map[string]any{"type": "boolean", "description": "Include superseded/refuted findings, always labeled (default false)"},
			},
			"required": []string{"service", "subject"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service           string `json:"service"`
			Subject           string `json:"subject"`
			Hops              int    `json:"hops"`
			IncludeSuperseded bool   `json:"include_superseded"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "findings_near: invalid args"}
		}
		if p.Service == "" || p.Subject == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "findings_near: service and subject are required"}
		}
		if p.Hops <= 0 {
			p.Hops = 2
		}

		// The shared snapshot and syntax+typed graph (RT-74): one build
		// per (service, watermark), never one per call.
		snap, err := serviceSnapshot(ctx, backend, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "findings_near: " + err.Error()}
		}
		graph, _, _, err := memo.GraphForTyped(ctx, snap, memo.GraphBoth, typedLoader(backend, p.Service))
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "findings_near: " + err.Error()}
		}

		var claims []core.SemanticClaim
		fb, ok := backend.(findingsBackend)
		if !ok {
			// Honest degrade: no findingsBackend capability (a minimal
			// test fake, or a future backend that never wired claims) —
			// the neighborhood itself is still real, just empty of
			// findings, and the note says why.
			return map[string]any{
				"subject": p.Subject, "hops": p.Hops, "include_superseded": p.IncludeSuperseded,
				"found": true, "note": "not measurable: backend does not serve semantic claims",
				"findings": []any{},
			}, nil
		}
		claims, err = fb.ClaimsForService(ctx, p.Service, "findings")
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "findings_near: " + err.Error()}
		}

		fileHops := findings.FileNeighborhoodOn(graph, snap.Nodes, p.Subject, p.Hops)
		if fileHops == nil {
			return map[string]any{
				"subject": p.Subject, "hops": p.Hops, "include_superseded": p.IncludeSuperseded,
				"found": false, "note": "subject not found in the index (no matching symbol or file)",
				"findings": []any{},
			}, nil
		}

		entries := findings.RankNear(claims, fileHops, p.IncludeSuperseded)
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			compiledAt := ""
			if !e.Claim.CompiledAt.IsZero() {
				compiledAt = e.Claim.CompiledAt.UTC().Format("2006-01-02T15:04:05Z")
			}
			out = append(out, map[string]any{
				"claim_id": e.Claim.ClaimID, "subject": e.Claim.Subject, "class": string(e.Claim.Class),
				"rule": e.Claim.Rule, "scope": e.Claim.Scope, "status": string(e.Claim.Status),
				"hops": e.Hops, "superseded": e.Superseded, "compiled_at": compiledAt,
			})
		}
		note := ""
		if len(out) == 0 {
			note = "no findings within the neighborhood"
		}
		return map[string]any{
			"subject": p.Subject, "hops": p.Hops, "include_superseded": p.IncludeSuperseded,
			"found": true, "note": note, "findings": out,
		}, nil
	})
}

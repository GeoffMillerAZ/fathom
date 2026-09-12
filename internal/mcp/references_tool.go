package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/references"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// registerReferences wires the "references" tool into the investigation group.
// It performs a fleet-wide occurrence search across all source files for a
// query string, returning structured occurrences (text matches, tier pre-scan)
// plus high-confidence symbol definitions (verified tier).
func registerReferences(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "references",
		Description: "Fleet-wide occurrence search: find every mention of a query string across code, docs, and config for one or more services. Returns structured occurrences (text matches, tier=pre-scan) plus symbol definitions found in the index (tier from the symbol's own evidence tier). Definitions are exact-name matches by default; include_heuristic: true also lists symbols whose name merely contains the query. Occurrences are unaffected. Every row carries depth (0 = a definition, 1 = a mention) and edge_confidence (resolved = the mention's enclosing symbol has a typed call edge to the definition, or the definition is at the verified tier; heuristic = text match only), summarised in by_depth. Reach for it when you need every place a name appears — docs and config included — not just the call graph. Required: query.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "String to search for. Alias: \"symbol\" (additive — same field, either name works; both may be given only if they agree).",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Alias for \"query\" (P1-1: the hot-tool quartet's primary argument is named differently per tool; this lets a caller reuse the same field name across all four).",
				},
				"services": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Service IDs to search (default: all known services)",
				},
				"word": map[string]any{
					"type":        "boolean",
					"description": "Whole-word match only",
				},
				"regex": map[string]any{
					"type":        "boolean",
					"description": "Treat query as a regular expression",
				},
				"case_insensitive": map[string]any{
					"type":        "boolean",
					"description": "Case-insensitive matching",
				},
				"max": map[string]any{
					"type":        "integer",
					"description": "Maximum total occurrences to return (default 200)",
				},
				"include_heuristic": map[string]any{
					"type":        "boolean",
					"description": "Also list definitions whose name merely CONTAINS the query (\"FooBar\" for \"Foo\"). Default false; occurrences are unaffected.",
				},
			},
			"required": []string{"query"},
		},
		group:   GroupInvestigation,
		aliases: map[string]string{"symbol": "query"},
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}

		var p struct {
			Query           string   `json:"query"`
			Services        []string `json:"services"`
			Word            bool     `json:"word"`
			Regex           bool     `json:"regex"`
			CaseInsensitive bool     `json:"case_insensitive"`
			Max             int      `json:"max"`
			// IncludeHeuristic (BP-26) restores the substring-containment
			// definition matching below. It is opt-in because a hit on
			// "FooBar" for query "Foo" is not a reference to Foo: the L0
			// oracle counted those separately and they are the definition
			// half's whole precision loss (docs/bench/README.md).
			IncludeHeuristic bool `json:"include_heuristic"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "references: invalid args"}
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "references: invalid args"}
		}
		query, aliasErr := resolveAliasedString(raw, "references", "query", "symbol")
		if aliasErr != nil {
			return nil, aliasErr
		}
		p.Query = query
		if p.Query == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "references: query required"}
		}

		maxOccs := p.Max
		if maxOccs <= 0 {
			maxOccs = 200
		}

		// Resolve service list.
		svcs := p.Services
		if len(svcs) == 0 {
			all, err := backend.Services(ctx)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "references: list services: " + err.Error()}
			}
			svcs = all
		}
		sort.Strings(svcs)

		opts := references.Options{
			Word:            p.Word,
			Regex:           p.Regex,
			CaseInsensitive: p.CaseInsensitive,
		}

		type defEntry struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Tier    string `json:"tier"`
			// Depth/EdgeConfidence (BD-2): a definition is depth 0; its
			// confidence is the index's own evidence tier (verified =
			// resolved, anything weaker = heuristic).
			Depth          int    `json:"depth"`
			EdgeConfidence string `json:"edge_confidence"`
		}
		type occEntry struct {
			Service string `json:"service"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Text    string `json:"text"`
			Tier    string `json:"tier"`
			// Depth/EdgeConfidence (BD-2): a mention is depth 1 — one hop
			// from the definition; resolved only when the symbol enclosing
			// the mention has a TYPED call edge to a definition of the
			// query in the same service, heuristic for a bare text match.
			Depth          int    `json:"depth"`
			EdgeConfidence string `json:"edge_confidence"`
		}

		var defs []defEntry
		var occs []occEntry
		truncated := false

		for _, svc := range svcs {
			// ── definitions: symbol index lookup ─────────────────────────────
			nodes, err := backend.ListAllSymbols(ctx, svc)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "references: list symbols " + svc + ": " + err.Error()}
			}
			var defKeys []string
			for _, n := range nodes {
				isDef := false
				if n.Name == p.Query {
					isDef = true
				} else if p.IncludeHeuristic && !p.Word && !p.Regex {
					// Substring match on name — opt-in since BP-26.
					if strings.Contains(n.Name, p.Query) {
						isDef = true
					}
				}
				if isDef {
					tier := string(n.Tier)
					if tier == "" {
						tier = string(core.TierVerified)
					}
					conf := core.EdgeConfidenceHeuristic
					if tier == string(core.TierVerified) {
						conf = core.EdgeConfidenceResolved
					}
					defs = append(defs, defEntry{
						Service:        svc,
						Symbol:         n.Name,
						File:           n.FilePath,
						Line:           n.Line,
						Tier:           tier,
						Depth:          0,
						EdgeConfidence: string(conf),
					})
					defKeys = append(defKeys, n.Key())
				}
			}

			// ── occurrences: source file scan ─────────────────────────────────
			if truncated {
				// Already at cap; skip further source scans.
				continue
			}
			sources, err := backend.Sources(ctx, svc)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "references: sources " + svc + ": " + err.Error()}
			}
			found, err := references.Find(p.Query, sources, opts)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: "references: " + err.Error()}
			}
			// Typed-edge confirmation for mentions (BD-2): built lazily,
			// only when this service has both definitions of the query and
			// typed edges to confirm a mention against.
			var mentionConf func(file string, line int) core.EdgeConfidence
			if len(defKeys) > 0 && len(found) > 0 {
				if typed, tErr := backend.TypedEdges(ctx, svc); tErr == nil && len(typed) > 0 {
					mentionConf = mentionConfidence(nodes, typed, defKeys)
				}
			}
			for _, occ := range found {
				if len(occs) >= maxOccs {
					truncated = true
					break
				}
				conf := core.EdgeConfidenceHeuristic
				if mentionConf != nil {
					conf = mentionConf(occ.File, occ.Line)
				}
				occs = append(occs, occEntry{
					Service:        svc,
					File:           occ.File,
					Line:           occ.Line,
					Text:           occ.Text,
					Tier:           string(core.TierPreScan),
					Depth:          1,
					EdgeConfidence: string(conf),
				})
			}
		}

		// applyBudget's own `of` counts occurrences seen before THIS cut
		// (i.e. after maxOccs may already have stopped the scan early,
		// reflected separately in `truncated`) — the two caps are
		// independent and both stay visible: `truncated` alone can be true
		// with no `elided` block when maxOccs capped but the budget did not.
		occs, el := applyBudget(occs, budgetDefaults("references", s.budgets),
			"service order, then source-scan order",
			"re-call with a larger budget, narrow \"services\", or tighten the query")
		// by_depth counts what the caller sees: every definition plus the
		// occurrences that survived BOTH caps (maxOccs above, the budget
		// here), so the summary never claims rows the response elided.
		infos := make([]depthInfo, 0, len(defs)+len(occs))
		for _, d := range defs {
			infos = append(infos, depthInfo{Depth: d.Depth, Confidence: core.EdgeConfidence(d.EdgeConfidence)})
		}
		for _, o := range occs {
			infos = append(infos, depthInfo{Depth: o.Depth, Confidence: core.EdgeConfidence(o.EdgeConfidence)})
		}
		resp := map[string]any{
			"query":       p.Query,
			"definitions": defs,
			"occurrences": occs,
			"count":       len(occs),
			// Which definition matching answered (BP-26): "resolved" =
			// exact-name definitions only, "all" = plus substring hits.
			"edge_filter": map[bool]string{true: "all", false: "resolved"}[p.IncludeHeuristic],
			"truncated":   truncated || el != nil,
			"by_depth":    withDepthBuckets(infos),
		}
		if el != nil {
			resp["elided"] = el
		}
		return resp, nil
	})
}

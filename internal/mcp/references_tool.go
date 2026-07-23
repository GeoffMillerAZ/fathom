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
		Description: "Fleet-wide occurrence search: find every mention of a query string across code, docs, and config for one or more services. Returns structured occurrences (heuristic text matches, tier=pre-scan) plus symbol definitions found in the index (tier=verified/pre-scan from the symbol's own evidence tier).",
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
		}
		type occEntry struct {
			Service string `json:"service"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Text    string `json:"text"`
			Tier    string `json:"tier"`
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
			for _, n := range nodes {
				isDef := false
				if n.Name == p.Query {
					isDef = true
				} else if !p.Word && !p.Regex {
					// substring match on name
					if strings.Contains(n.Name, p.Query) {
						isDef = true
					}
				}
				if isDef {
					tier := string(n.Tier)
					if tier == "" {
						tier = string(core.TierVerified)
					}
					defs = append(defs, defEntry{
						Service: svc,
						Symbol:  n.Name,
						File:    n.FilePath,
						Line:    n.Line,
						Tier:    tier,
					})
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
			for _, occ := range found {
				if len(occs) >= maxOccs {
					truncated = true
					break
				}
				occs = append(occs, occEntry{
					Service: svc,
					File:    occ.File,
					Line:    occ.Line,
					Text:    occ.Text,
					Tier:    string(core.TierPreScan),
				})
			}
		}

		occs, budgetTrunc := fitToBudget(occs, 0)
		return map[string]any{
			"query":       p.Query,
			"definitions": defs,
			"occurrences": occs,
			"count":       len(occs),
			"truncated":   truncated || budgetTrunc,
		}, nil
	})
}

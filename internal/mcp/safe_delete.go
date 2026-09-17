// Package mcp — safe_delete_check (ADR-035 decision 1, docs/vision/
// 29-verbs-not-lookups.md §3 wave A, plan VB-2): "can this symbol go" —
// read-only, never a mutator, a sibling of rename_impact answering the
// question one step further down the same road (rename touches every
// call site; delete needs to know there are none left).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// safeDeleteReferent is one remaining use of a symbol safe_delete_check
// found — deleting the symbol would break this. Tier names the strongest
// evidence behind the row: "typed" (the Go typed pass confirmed a real
// call-graph edge) or "heuristic" (the syntax pass's bare-name match).
type safeDeleteReferent struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Symbol string `json:"symbol"`
	Tier   string `json:"tier"`
}

func registerSafeDeleteCheck(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "safe_delete_check",
		Description: "The answer grep cannot give before a delete: every remaining referent of a symbol, each with file:line and a resolution tier (typed = a confirmed call-graph edge, heuristic = a bare-name match), test references listed separately from code references, and — when referent_count is 0 — a proven-zero receipt naming the index commit and which edge kinds were actually checked. Read-only: a sibling of rename_impact one step further down the same road, never a mutator. An interface (or a method only an interface declares) lists its concrete implementations as referents, since deleting it breaks them even though nothing \"calls\" it. Reach for it when you are about to delete a symbol and need to know whether anything still depends on it, rather than trusting a clean grep. symbol_id is an alias for symbol (a fully-qualified SymbolKey). Required: service, symbol.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":   map[string]any{"type": "string", "description": "Service / repo identifier"},
				"symbol":    map[string]any{"type": "string", "description": "Symbol name (bare or fully-qualified SymbolKey)"},
				"symbol_id": map[string]any{"type": "string", "description": "Alias for symbol — a fully-qualified SymbolKey"},
			},
			"required": []string{"service", "symbol"},
		},
		aliases: map[string]string{"symbol_id": "symbol"},
		group:   GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "safe_delete_check: invalid args"}
		}
		var p struct {
			Service string `json:"service"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "safe_delete_check: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "safe_delete_check: service required"}
		}
		symbol, aliasErr := resolveAliasedString(raw, "safe_delete_check", "symbol", "symbol_id")
		if aliasErr != nil {
			return nil, aliasErr
		}
		if symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "safe_delete_check: symbol or symbol_id required"}
		}

		navs := s.loadNavGraphs(ctx, backend, p.Service)
		if navs == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "safe_delete_check: no symbols for service " + p.Service}
		}

		heurNodes, err := backend.FindCallers(ctx, p.Service, symbol)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		typedKeys, typedOK := resolvedCallersOnly(navs, symbol)

		byKey := make(map[string]core.SymbolNode, len(navs.Nodes))
		for _, n := range navs.Nodes {
			byKey[n.Key()] = n
		}

		// tierOf classifies every caller key found by EITHER pass. A key
		// the typed pass confirmed is reported "typed" even when the
		// syntax pass also found it — typed is strictly stronger evidence
		// for the same call site, never a second, weaker row.
		tierOf := make(map[string]string, len(heurNodes))
		for _, n := range heurNodes {
			tierOf[n.Key()] = string(core.EdgeConfidenceHeuristic)
		}
		if typedOK {
			for k := range typedKeys {
				tierOf[k] = string(core.EdgeConfidenceResolved)
			}
		}

		referents := []safeDeleteReferent{}
		var testReferents []safeDeleteReferent
		var mix resolutionMix
		for key, conf := range tierOf {
			n, ok := byKey[key]
			if !ok {
				for _, hn := range heurNodes {
					if hn.Key() == key {
						n, ok = hn, true
						break
					}
				}
			}
			if !ok {
				continue
			}
			r := safeDeleteReferent{File: n.FilePath, Line: n.Line, Symbol: n.Name, Tier: tierLabel(conf)}
			mix.addConfidence(core.EdgeConfidence(conf))
			if strings.HasPrefix(n.Name, "Test") {
				testReferents = append(testReferents, r)
			} else {
				referents = append(referents, r)
			}
		}

		// An interface (or a method only an interface declares) has no
		// "caller" at all, yet deleting it breaks every concrete
		// implementation — list those as referents too (VB-2).
		for _, im := range safeDeleteImplementations(ctx, backend, p.Service, symbol, navs.Nodes) {
			tier := "heuristic"
			if im.Via == "typed" {
				tier = "typed"
			}
			referents = append(referents, safeDeleteReferent{File: im.FilePath, Line: im.Line, Symbol: im.Name, Tier: tier})
			if tier == "typed" {
				mix.Typed++
			} else {
				mix.Heuristic++
			}
		}

		sortSafeDeleteReferents(referents)
		sortSafeDeleteReferents(testReferents)

		// checkedEdges names the strongest evidence tier the zero-referent
		// (or non-zero) verdict actually rests on: "typed" when the
		// service's typed pass ran (both typed and heuristic edges were
		// consulted), "heuristic" when only the syntax pass is available
		// (a non-Go corpus, or a Go corpus with no resolved edges yet) —
		// the same vocabulary RC-19/BP-26 already use for edge_confidence.
		checkedEdges := "heuristic"
		if navs.TypedAvailable {
			checkedEdges = "typed"
		}

		_, indexCommit, metaFound, _ := backend.ServiceMeta(ctx, p.Service)

		// referent_count: 0 alongside an empty referents array IS the
		// proven-zero receipt — checked_edges names what was actually
		// checked to reach it, and index_commit (when the backend can
		// name one) anchors it to a commit a stranger can re-derive
		// against. Always the same three fields either way, never a
		// field that changes shape between zero and non-zero.
		// resolution (VB-19): the typed/heuristic mix behind referents +
		// test_referents (interface implementers included) — always
		// present, even {0,0} on a proven-zero answer.
		resp := map[string]any{
			"referents":      referents,
			"referent_count": len(referents),
			"checked_edges":  checkedEdges,
			"resolution":     mix,
		}
		if len(testReferents) > 0 {
			resp["test_referents"] = testReferents
		}
		if metaFound && indexCommit != "" {
			resp["index_commit"] = indexCommit
		}
		return withSummary(safeDeleteSummary(symbol, referents, checkedEdges), resp), nil
	})
}

// safeDeleteSummary renders safe_delete_check's answer-first sentence (wave
// L1): the count and the first three sites, or — when referents is empty —
// the proven-zero receipt line, built from referents itself (the exact,
// already-sorted list the "referents" field returns).
func safeDeleteSummary(symbol string, referents []safeDeleteReferent, checkedEdges string) string {
	if len(referents) == 0 {
		return fmt.Sprintf("0 referents for %s — proven zero (%s edges checked)", symbol, checkedEdges)
	}
	n := len(referents)
	if n > 3 {
		n = 3
	}
	sites := make([]string, n)
	for i, r := range referents[:n] {
		sites[i] = fmt.Sprintf("%s:%d", r.File, r.Line)
	}
	summary := fmt.Sprintf("%d referent%s for %s: %s", len(referents), pluralS(len(referents)), symbol, strings.Join(sites, ", "))
	if len(referents) > n {
		summary += fmt.Sprintf(" (+%d more)", len(referents)-n)
	}
	return capSummary(summary)
}

// tierLabel renders a core.EdgeConfidence as safe_delete_check's tier
// vocabulary: "resolved" is reported as "typed" (the plan's own naming —
// a confirmed call-graph edge), anything else as "heuristic".
func tierLabel(conf string) string {
	if conf == string(core.EdgeConfidenceResolved) {
		return "typed"
	}
	return "heuristic"
}

// safeDeleteImplementations answers the interface half of "can this go":
// symbol may itself be a Go interface (every implementer is a referent —
// deleting the interface breaks them even though none of them "call" it),
// or a method name only one indexed interface declares (mirrors
// references_tool.go's interfaceMethodQuery) — in which case the
// implementers' OWN method sites are the referents. Neither shape returns
// anything for an ordinary function/method symbol.
func safeDeleteImplementations(ctx context.Context, backend QueryInterface, service, symbol string, nodes []core.SymbolNode) []implementationEntry {
	for _, n := range nodes {
		if n.Kind == core.KindGoInterface && (n.Name == symbol || n.Key() == symbol) {
			return implementationsOf(ctx, backend, service, n, nodes)
		}
	}
	sources, err := backend.Sources(ctx, service)
	if err != nil || len(sources) == 0 {
		return nil
	}
	if iface, method, ok := interfaceMethodQuery(sources, nodes, symbol); ok {
		return implementationsForMethod(ctx, backend, service, iface, method, nodes)
	}
	return nil
}

// sortSafeDeleteReferents orders referents deterministically and puts
// typed edges first (VB-2's own ordering): tier ("typed" before
// "heuristic"), then file, then line, then symbol name — so a proven-zero
// (or non-zero) answer never depends on map iteration order, and the
// strongest evidence always leads.
func sortSafeDeleteReferents(rs []safeDeleteReferent) {
	rank := func(tier string) int {
		if tier == "typed" {
			return 0
		}
		return 1
	}
	sort.Slice(rs, func(i, j int) bool {
		if ri, rj := rank(rs[i].Tier), rank(rs[j].Tier); ri != rj {
			return ri < rj
		}
		if rs[i].File != rs[j].File {
			return rs[i].File < rs[j].File
		}
		if rs[i].Line != rs[j].Line {
			return rs[i].Line < rs[j].Line
		}
		return rs[i].Symbol < rs[j].Symbol
	})
}

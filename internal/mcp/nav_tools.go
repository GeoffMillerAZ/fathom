// Package mcp — navigation and analysis tool registrations backed by the
// symbolnav, complexity, and godobject analysis packages.
//
// registerNavTools wires the coding-group tools:
//   - find_callees, symbol_context, outline, entrypoints
//   - complexity, god_object
//
// and the investigation-group tools:
//   - community
package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/complexity"
	"github.com/geoffmilleraz/fathom/internal/analysis/godobject"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// registerNavTools is called from registerTools (tools.go). fleet may be
// nil (non-fleet stdio serve-mcp); when set, registerSymbolContext
// additionally consults it for cross-service call edges (P1-2).
func registerNavTools(s *Server, backend QueryInterface, fleet FleetBackend) {
	registerFindCallees(s, backend)
	registerSymbolContext(s, backend, fleet)
	registerOutline(s, backend)
	registerEntrypoints(s, backend)
	registerComplexity(s, backend)
	registerGodObject(s, backend)
	registerCommunity(s, backend)
}

// ─── find_callees ─────────────────────────────────────────────────────────────

func registerFindCallees(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "find_callees",
		Description: "Return the direct callees (symbols called by) the named symbol in the given service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"symbol":  map[string]any{"type": "string", "description": "Symbol name (bare or fully-qualified SymbolKey)"},
			},
			"required": []string{"service", "symbol"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_callees: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_callees: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "find_callees: symbol required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		g := callgraph.Build(nodes)
		callees := symbolnav.Callees(g, nodes, p.Symbol)
		return map[string]any{"callees": stringsToAny(callees)}, nil
	})
}

// ─── symbol_context ───────────────────────────────────────────────────────────

// symbolContextSnippetMaxLines caps symbol_context's inline definition
// snippet (T5-6, docs/plans/trust-and-tokenomics.md): small enough to stay
// inside one response, generous enough to show a typical function body — so
// a "what is X and show it to me" question completes in ONE turn instead of
// a symbol_context call followed by a separate read. Read via the exact same
// safe, bounded readSourceRange path read_source uses (readsource.go, T5-4),
// so a snippet and a read_source citation of the same lines are identical.
const symbolContextSnippetMaxLines = 40

func registerSymbolContext(s *Server, backend QueryInterface, fleet FleetBackend) {
	s.registerTool(toolDescriptor{
		Name:        "symbol_context",
		Description: "Return the neighbourhood context for a symbol: its definition, direct callers, direct callees, related tests, and (when readable) an inline source snippet capped at 40 lines. This result IS the answer for a symbol's immediate neighbourhood; a follow-up call only helps once you need lines beyond the 40-line cap (read_source) or a different symbol's neighbourhood. Reach for it when you have located a symbol and need its neighbourhood — definition, callers, callees, tests, source — in one call before changing it. Required: service, symbol.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"symbol":  map[string]any{"type": "string", "description": "Symbol name (bare or fully-qualified SymbolKey)"},
				"snippet": map[string]any{"type": "boolean", "description": "Include the inline definition source snippet (default true). Set false to omit it."},
			},
			"required": []string{"service", "symbol"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
			Snippet *bool  `json:"snippet"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "symbol_context: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "symbol_context: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "symbol_context: symbol required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		g := callgraph.Build(nodes)
		sctx := symbolnav.SymbolContext(nodes, g, p.Symbol)
		var defDTO *symbolNodeDTO
		if sctx.Def.Name != "" {
			d := symbolNodeDTO{
				Service:   sctx.Def.Service,
				Name:      sctx.Def.Name,
				Kind:      string(sctx.Def.Kind),
				FilePath:  sctx.Def.FilePath,
				Line:      sctx.Def.Line,
				EndLine:   sctx.Def.EndLine,
				Signature: sctx.Def.Signature,
				Tier:      string(sctx.Def.Tier),
			}
			defDTO = &d
		}
		// RC-19: no budget line existed for symbol_context before this row
		// — callers/callees are the fields a hot, widely-called symbol can
		// blow up. Each gets its own half of the budget; the two counts
		// combine into one elision line (both name the same refinement).
		callers, callersElided := fitToolBudget(sctx.Callers, defaultCharBudget/2)
		callees, calleesElided := fitToolBudget(sctx.Callees, defaultCharBudget/2)
		result := map[string]any{
			"def":           defDTO,
			"callers":       stringsToAny(callers),
			"callees":       stringsToAny(callees),
			"related_tests": stringsToAny(sctx.RelatedTests),
		}
		if line := elisionLine(callersElided+calleesElided, "depth"); line != "" {
			result["elision"] = line
		}

		// resolution (VB-19, docs/vision/29-verbs-not-lookups.md §3 wave E):
		// the typed/heuristic mix behind exactly the "callers" and
		// "callees" this response returned (post-budget), tallied from a
		// graph augmented with typed edges (best-effort — a non-Go service
		// or one with no typed edges tallies every row heuristic). typed
		// is fetched ONCE and reused below for typed_edges too (review F5
		// — this handler previously called backend.TypedEdges twice).
		typedEdges, typedErr := backend.TypedEdges(ctx, p.Service)
		var mix resolutionMix
		var tg *callgraph.Graph
		if typedErr == nil && len(typedEdges) > 0 {
			tg = callgraph.Build(nodes)
			callgraph.AddTypedEdges(tg, typedEdges)
		} else {
			tg = g
		}
		kindOf := func(details []callgraph.EdgeInfo) map[string]core.EdgeKind {
			m := make(map[string]core.EdgeKind, len(details))
			for _, d := range details {
				m[d.Key] = d.Kind
			}
			return m
		}
		callerKinds := kindOf(callgraph.CallersDetail(tg, p.Symbol))
		calleeKinds := kindOf(callgraph.CalleesDetail(tg, p.Symbol))
		for _, k := range callers {
			mix.addKind(callerKinds[k])
		}
		for _, k := range callees {
			mix.addKind(calleeKinds[k])
		}
		if ic := indexCommitFor(ctx, backend, p.Service); ic != "" {
			result["index_commit"] = ic
		}

		// Inline definition snippet (T5-6): additive, honest-absence field —
		// omitted (never faked) when the definition has no file/line, its
		// source is not retrievable (no ServiceMeta corpus root, file
		// missing, binary, etc.) via the same bounded read_source path, or
		// the caller opted out with snippet:false. When the read stopped at
		// the cap with more file below it, snippet_truncated:true says so —
		// a 100-line function must never LOOK 40 lines long (most Go defs
		// carry end_line 0, so the agent cannot infer clipping on its own).
		wantSnippet := p.Snippet == nil || *p.Snippet
		if wantSnippet && sctx.Def.Name != "" && sctx.Def.FilePath != "" && sctx.Def.Line > 0 {
			if rng, snErr := readSourceRange(ctx, backend, p.Service, sctx.Def.FilePath, sctx.Def.Line, 0, symbolContextSnippetMaxLines, sourceReadMaxBytes); snErr == nil && len(rng.Lines) > 0 {
				snippet := make([]any, len(rng.Lines))
				for i, ln := range rng.Lines {
					snippet[i] = map[string]any{"line": ln.Line, "text": ln.Text}
				}
				result["snippet"] = snippet
				if rng.Note != "" {
					result["snippet_truncated"] = true
				}
			}
		}

		// Typed Go call edges (ADR-021 §3): kind-labeled callers/callees,
		// read from the SAME typed-augmented graph (tg) the resolution
		// tally above already built — never mutating g/sctx, so the
		// existing syntax-only "callers"/"callees" fields keep their exact
		// contract. Best-effort — a non-Go service, or one with no typed
		// edges yet, simply omits this field.
		if len(typedEdges) > 0 {
			if edges := typedEdgeContext(tg, p.Symbol); edges != nil {
				result["typed_edges"] = edges
			}
		}

		// Cross-service callers/callees (P1-2, closing O12(a)): g/sctx above
		// are built from p.Service's own nodes only, so a call crossing a
		// service boundary — e.g. fathom's cmdInstrument calling agentic-
		// kit's heartbeat.Load/CheckLiveness/Rollup — never appears in
		// "callees", and a caller living in another service never appears
		// in "callers". When a fleet backend is wired (fleet stores),
		// crossServiceCallees/crossServiceCallers fill that gap: callees
		// resolve sctx.Def's own raw Calls entries against every OTHER
		// service's exported symbols (mirroring platformlink.
		// BuildFromSymbols' qualified-match rule, internal/platformlink/
		// platformlink.go); callers reuse cross_repo_callers' own
		// fleet-wide matching (platform_tools.go's fleetCallers) filtered
		// to services other than p.Service. Both land in their OWN
		// "cross_service_*" fields — "callers"/"callees" keep their exact
		// existing (same-service, string-key) shape — so the two scopes
		// are never silently merged; each cross-service hit also carries
		// an explicit "scope":"cross-service" tag. nil fleet or no
		// cross-service hits omit the fields entirely.
		// resolution (review F4): cross-service hits count too, every one
		// heuristic — no typed pass crosses a process boundary today
		// (ADR-021 is Go-only within one service; vision 29 §1 R7, VB-30's
		// service_ref edges are still a future wave), so a cross-service
		// callee/caller can never be anything but heuristic evidence. This
		// runs AFTER cross_service_callees/callers below are computed so
		// the mix always matches every row this response actually returns.
		if sctx.Def.Name != "" {
			if callees := crossServiceCallees(fleet, p.Service, sctx.Def); len(callees) > 0 {
				result["cross_service_callees"] = callees
				mix.Heuristic += len(callees)
			}
		}
		if callers := crossServiceCallers(fleet, p.Service, p.Symbol); len(callers) > 0 {
			result["cross_service_callers"] = callers
			mix.Heuristic += len(callers)
		}
		result["resolution"] = mix
		return result, nil
	})
}

// typedEdgeContext renders symbol's typed callers/callees (kind-labeled)
// from a graph already augmented via callgraph.AddTypedEdges. Syntax-only
// edges (Kind == "") are excluded — already covered by the plain
// "callers"/"callees" fields. Returns nil when neither side has any typed
// edges, so the caller can omit the field entirely.
func typedEdgeContext(g *callgraph.Graph, symbol string) map[string]any {
	callers := kindLabeledEdges(callgraph.CallersDetail(g, symbol))
	callees := kindLabeledEdges(callgraph.CalleesDetail(g, symbol))
	if len(callers) == 0 && len(callees) == 0 {
		return nil
	}
	return map[string]any{"callers": callers, "callees": callees}
}

// kindLabeledEdges converts callgraph.EdgeInfo values with a non-empty
// (typed) Kind into JSON-ready maps, dropping syntax-only ("") entries.
func kindLabeledEdges(details []callgraph.EdgeInfo) []any {
	var out []any
	for _, d := range details {
		if d.Kind == "" {
			continue
		}
		out = append(out, map[string]any{"key": d.Key, "kind": string(d.Kind)})
	}
	return out
}

// ─── outline ─────────────────────────────────────────────────────────────────

func registerOutline(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "outline",
		Description: "List all symbols defined in a specific file, ordered by line number. Reach for it when you need a file's structure — every definition with its line — without reading the whole file. Budgeted; the elision line names the refinement (depth or kinds). Required: service, file.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"file":    map[string]any{"type": "string", "description": "Repo-relative file path"},
			},
			"required": []string{"service", "file"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			File    string `json:"file"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "outline: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "outline: service required"}
		}
		if p.File == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "outline: file required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		outlined := symbolnav.Outline(nodes, p.File)
		// RC-19: no budget line existed for outline before this row —
		// measured 16,778 B for internal/core/model.go on this repo's own
		// index. Keep whole rows up to budget, then a one-line elision.
		symbols, elidedCount := fitToolBudget(toNodeDTOs(outlined), defaultCharBudget)
		resp := map[string]any{"symbols": symbols}
		if line := elisionLine(elidedCount, "depth or kinds"); line != "" {
			resp["elision"] = line
		}
		return resp, nil
	})
}

// ─── entrypoints ─────────────────────────────────────────────────────────────

func registerEntrypoints(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "entrypoints",
		Description: "Return the likely program entrypoints in the service: main functions, HTTP handlers, and CLI root commands. Reach for it when you are orienting in an unfamiliar service and need where execution starts before tracing inward. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "entrypoints: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "entrypoints: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		eps := symbolnav.Entrypoints(nodes)
		return map[string]any{"entrypoints": toNodeDTOs(eps)}, nil
	})
}

// ─── complexity ───────────────────────────────────────────────────────────────

func registerComplexity(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "complexity",
		Description: "Return cyclomatic-complexity hotspots and Maintainability Index for the service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"top_n":   map[string]any{"type": "integer", "description": "Number of hotspots to return (default 10)"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			TopN    int    `json:"top_n"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "complexity: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "complexity: service required"}
		}
		topN := p.TopN
		if topN <= 0 {
			topN = 10
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		hotspots := complexity.Hotspots(nodes, topN)
		mi := complexity.MaintainabilityIndex(nodes)
		hs := make([]any, len(hotspots))
		for i, h := range hotspots {
			hs[i] = map[string]any{
				"name":       h.Name,
				"file":       h.FilePath,
				"line":       h.Line,
				"complexity": h.Complexity,
			}
		}
		return map[string]any{
			"hotspots":              hs,
			"maintainability_index": mi,
		}, nil
	})
}

// ─── god_object ───────────────────────────────────────────────────────────────

func registerGodObject(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "god_object",
		Description: "Identify god-object types in the service (types with too many methods or fields).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":          map[string]any{"type": "string", "description": "Service / repo identifier"},
				"method_threshold": map[string]any{"type": "integer", "description": "Minimum method count to flag (default 10)"},
				"field_threshold":  map[string]any{"type": "integer", "description": "Minimum field count to flag (default 10)"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service         string `json:"service"`
			MethodThreshold int    `json:"method_threshold"`
			FieldThreshold  int    `json:"field_threshold"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "god_object: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "god_object: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		cfg := godobject.Config{
			MethodThreshold: p.MethodThreshold,
			FieldThreshold:  p.FieldThreshold,
		}
		gods := godobject.Analyze(nodes, cfg)
		result := make([]any, len(gods))
		for i, g := range gods {
			result[i] = map[string]any{
				"type":    g.Type,
				"methods": g.Methods,
				"fields":  g.Fields,
			}
		}
		return map[string]any{"god_objects": result}, nil
	})
}

// ─── community ────────────────────────────────────────────────────────────────

func registerCommunity(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "community",
		Description: "Return the community of symbols closely related to the given symbol: direct callers, callees, and symbols sharing the same package or file.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"symbol":  map[string]any{"type": "string", "description": "Symbol name (bare or fully-qualified SymbolKey)"},
			},
			"required": []string{"service", "symbol"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "community: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "community: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "community: symbol required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		g := callgraph.Build(nodes)
		members := symbolnav.Community(g, nodes, p.Symbol)
		return map[string]any{"members": stringsToAny(members)}, nil
	})
}

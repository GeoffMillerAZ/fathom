// Package mcp — navigation and analysis tool registrations backed by the
// symbolnav, complexity, and godobject analysis packages.
//
// registerNavTools wires the coding-group tools:
//   - symbol_context (carries callees too, SR-21), outline
//   - complexity, god_object
//
// entrypoints was unregistered by SR-22 (F-10, design §4): repo_map's
// header now carries the same symbolnav.Entrypoints detector, capped at 10
// with a "more" count (repoMapEntrypoints, extra_tools.go).
//
// and the investigation-group tools:
//   - community
package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// registerNavTools is called from registerTools (tools.go). fleet may be
// nil (non-fleet stdio serve-mcp); when set, registerSymbolContext
// additionally consults it for cross-service call edges (P1-2).
func registerNavTools(s *Server, backend QueryInterface, fleet FleetBackend) {
	registerSymbolContext(s, backend, fleet)
	registerOutline(s, backend)
	// registerComplexity, registerGodObject, registerCommunity (F-77) were
	// unregistered by SR-19 (surface reckoning): 0 lifetime calls each.
	// The underlying analyses (internal/analysis/complexity, godobject,
	// symbolnav.Community) stay — only their own tests call them now; the
	// complexity metric U13 reads is a separate stored column
	// (internal/store/conformance.go), untouched.
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

// symbolContextCandidateCap bounds the candidate list an ambiguous/hub
// symbol_context answer renders (J3(a), app-kit O12) — enough to show the
// shape of the ambiguity without becoming the unbounded payload this exists
// to stop.
const symbolContextCandidateCap = 10

// symbolContextHubCallerCap (J3(a), app-kit O12): a symbol that resolves to
// exactly ONE definition but has more callers than this is a hub — the
// bare `symbol_context Run` query the dogfood journal found returning a
// ~63k-token cross-service caller set. Past this cap, symbol_context
// refuses the neighbourhood the same way it refuses true name ambiguity:
// scope_required:true, no callers/callees/snippet.
const symbolContextHubCallerCap = 50

// ambiguousSymbolContextResult is the ONE shape symbol_context returns
// instead of a neighbourhood when resolution is unsafe to serve (J3(a)):
// candidates is capped at symbolContextCandidateCap ({symbol_key, file,
// line} per row); count is the FULL size of whatever candidates is a
// sample of — for ambiguous:true, the number of distinct definitions the
// bare name matched (candidates may be a truncated sample of it); for
// scope_required:true, the resolved definition's own caller count (the hub
// fan-in that tripped the cap) — candidates there is the single resolved
// definition itself, so a caller can still see its file/line without a
// second lookup. hint always points at the fix: qualify the query.
func ambiguousSymbolContextResult(candidates []core.SymbolNode, count int, hub bool) map[string]any {
	shown := candidates
	if len(shown) > symbolContextCandidateCap {
		shown = shown[:symbolContextCandidateCap]
	}
	rows := make([]any, len(shown))
	for i, n := range shown {
		rows[i] = map[string]any{"symbol_key": n.Key(), "file": n.FilePath, "line": n.Line}
	}
	result := map[string]any{
		"candidates": rows,
		"count":      count,
		"hint":       "pass the fully-qualified key or service",
	}
	if hub {
		result["scope_required"] = true
	} else {
		result["ambiguous"] = true
	}
	return result
}

func registerSymbolContext(s *Server, backend QueryInterface, fleet FleetBackend) {
	s.registerTool(toolDescriptor{
		Name:        "symbol_context",
		Description: "Neighbourhood for a symbol: definition, callers/callees, tests, a snippet. This result IS the answer for its neighbourhood; a follow-up call only helps past the 40-line cap. Reach for it when about to change a symbol. Required: service, symbol.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"symbol":  map[string]any{"type": "string", "description": "Symbol name (bare or fully-qualified)"},
				"snippet": map[string]any{"type": "boolean", "description": "Include the source snippet (default true)"},
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
		// The service's symbols and graphs come from the navigation memo —
		// one load and build per (service, watermark), shared by concurrent
		// callers. Building them per call cost 2.45–9.78 GB of allocation and
		// ~6 GiB of footprint under a 20-call burst (wave-A incident replay).
		ng, lerr := loadNavGraphsErr(ctx, backend, p.Service)
		if lerr != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: lerr.Error()}
		}
		nodes, g := ng.Nodes, ng.Syntax

		// J3(a) (app-kit O12): resolve BEFORE building any neighbourhood.
		// symbolnav.ResolveTiered mirrors the internal resolution
		// SymbolContext would do anyway, but exposes the candidate set
		// instead of collapsing ambiguity to a silent empty answer — and
		// lets a uniquely-resolved but huge-fan-in symbol (a "hub") be
		// caught before its caller list is ever computed for output.
		if key, candidates, ok := symbolnav.ResolveTiered(nodes, p.Symbol); len(candidates) > 1 {
			return ambiguousSymbolContextResult(candidates, len(candidates), false), nil
		} else if ok {
			if callers := callgraph.RenameImpact(g, key); len(callers) > symbolContextHubCallerCap {
				return ambiguousSymbolContextResult(candidates, len(callers), true), nil
			}
		}

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
		if line := elisionLine(callersElided+calleesElided, "find_callers (depth) for every caller"); line != "" {
			result["elision"] = line
		}

		// resolution (VB-19, docs/vision/29-verbs-not-lookups.md §3 wave E):
		// the typed/heuristic mix behind exactly the "callers" and
		// "callees" this response returned (post-budget), tallied from a
		// graph augmented with typed edges (best-effort — a non-Go service
		// or one with no typed edges tallies every row heuristic). typed
		// is fetched ONCE and reused below for typed_edges too (review F5
		// — this handler previously called backend.TypedEdges twice).
		typedEdges := ng.Typed
		var mix resolutionMix
		tg := g
		if len(typedEdges) > 0 {
			tg = ng.Both // shared and read-only: never AddTypedEdges on it
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
			if callees := crossServiceCallees(ctx, fleet, p.Service, sctx.Def); len(callees) > 0 {
				result["cross_service_callees"] = callees
				mix.Heuristic += len(callees)
			}
		}
		if callers := crossServiceCallers(ctx, fleet, p.Service, p.Symbol); len(callers) > 0 {
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
		Description: "Lists every symbol defined in a file, ordered by line. Reach for it when editing a file you have not read yet. Required: service, file.",
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
		nodes, err := serviceSymbols(ctx, backend, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		outlined := symbolnav.Outline(nodes, p.File)
		// RC-19: no budget line existed for outline before this row —
		// measured 16,778 B for internal/core/model.go on this repo's own
		// index. Keep whole rows up to budget, then a one-line elision.
		symbols, elidedCount := fitToolBudget(toNodeDTOs(outlined), defaultCharBudget)
		resp := map[string]any{"symbols": symbols}
		if line := elisionLine(elidedCount, "read_source with start_line/end_line"); line != "" {
			resp["elision"] = line
		}
		return resp, nil
	})
}

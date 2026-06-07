// Package mcp — navigation and analysis tool registrations backed by the
// symbolnav, complexity, godobject, and apisurface analysis packages.
//
// registerNavTools wires the coding-group tools:
//   - find_callees, call_hierarchy, symbol_context, outline, entrypoints
//   - complexity, god_object, api_surface
//
// and the investigation-group tools:
//   - community, type_hierarchy
package mcp

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/geoffmilleraz/sextant/internal/analysis/apisurface"
	"github.com/geoffmilleraz/sextant/internal/analysis/callgraph"
	"github.com/geoffmilleraz/sextant/internal/analysis/complexity"
	"github.com/geoffmilleraz/sextant/internal/analysis/godobject"
	"github.com/geoffmilleraz/sextant/internal/analysis/symbolnav"
)

// registerNavTools is called from registerTools (tools.go).
func registerNavTools(s *Server, backend QueryInterface) {
	registerFindCallees(s, backend)
	registerCallHierarchy(s, backend)
	registerSymbolContext(s, backend)
	registerOutline(s, backend)
	registerEntrypoints(s, backend)
	registerComplexity(s, backend)
	registerGodObject(s, backend)
	registerAPISurface(s, backend)
	registerCommunity(s, backend)
	registerTypeHierarchy(s, backend)
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

// ─── call_hierarchy ───────────────────────────────────────────────────────────

func registerCallHierarchy(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "call_hierarchy",
		Description: "Return a call-hierarchy tree rooted at symbol, expanding both callers and callees up to depth hops.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"symbol":  map[string]any{"type": "string", "description": "Symbol name (bare or fully-qualified SymbolKey)"},
				"depth":   map[string]any{"type": "integer", "description": "Expansion depth (default 2, max 5)"},
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
			Depth   int    `json:"depth"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "call_hierarchy: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "call_hierarchy: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "call_hierarchy: symbol required"}
		}
		depth := p.Depth
		if depth <= 0 {
			depth = 2
		}
		if depth > 5 {
			depth = 5
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		g := callgraph.Build(nodes)
		tree := symbolnav.CallHierarchy(g, nodes, p.Symbol, depth)
		return treeToMap(tree), nil
	})
}

// treeToMap converts a symbolnav.Tree into a JSON-serialisable map recursively.
func treeToMap(t symbolnav.Tree) map[string]any {
	children := make([]any, len(t.Children))
	for i, c := range t.Children {
		children[i] = treeToMap(c)
	}
	return map[string]any{
		"key":      t.Key,
		"children": children,
	}
}

// ─── symbol_context ───────────────────────────────────────────────────────────

func registerSymbolContext(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "symbol_context",
		Description: "Return the neighbourhood context for a symbol: its definition, direct callers, direct callees, and related tests.",
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
				Signature: sctx.Def.Signature,
				Tier:      string(sctx.Def.Tier),
			}
			defDTO = &d
		}
		return map[string]any{
			"def":           defDTO,
			"callers":       stringsToAny(sctx.Callers),
			"callees":       stringsToAny(sctx.Callees),
			"related_tests": stringsToAny(sctx.RelatedTests),
		}, nil
	})
}

// ─── outline ─────────────────────────────────────────────────────────────────

func registerOutline(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "outline",
		Description: "List all symbols defined in a specific file, ordered by line number.",
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
		return map[string]any{"symbols": toNodeDTOs(outlined)}, nil
	})
}

// ─── entrypoints ─────────────────────────────────────────────────────────────

func registerEntrypoints(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "entrypoints",
		Description: "Return the likely program entrypoints in the service: main functions, HTTP handlers, and CLI root commands.",
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

// ─── api_surface ──────────────────────────────────────────────────────────────

func registerAPISurface(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "api_surface",
		Description: "Return the exported public API surface of the service (exported funcs, methods, types, interfaces, consts, vars).",
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
			return nil, &rpcError{Code: ErrCodeParams, Message: "api_surface: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "api_surface: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		surface := apisurface.Surface(nodes)
		return map[string]any{"symbols": toNodeDTOs(surface)}, nil
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

// ─── type_hierarchy ───────────────────────────────────────────────────────────

func registerTypeHierarchy(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "type_hierarchy",
		Description: "Return a map from interface name to the list of types that implement it, across the entire service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "type_hierarchy: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "type_hierarchy: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		hier := symbolnav.TypeHierarchy(nodes)
		// Sort implementors within each interface for determinism, then marshal.
		result := make(map[string]any, len(hier))
		for iface, impls := range hier {
			sorted := make([]string, len(impls))
			copy(sorted, impls)
			sort.Strings(sorted)
			result[iface] = stringsToAny(sorted)
		}
		return map[string]any{"hierarchy": result}, nil
	})
}

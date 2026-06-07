// Package mcp — analysis tool registrations backed by callgraph.Graph.
//
// registerAnalysisTools wires rename_impact, reachable, call_path, test_impact,
// dead_code, and import_cycles into the server's tool registry.
//
// The call-graph tools build a *callgraph.Graph on each invocation by calling
// backend.ListAllSymbols and passing the result to callgraph.Build.  This is
// intentionally simple: caching can be added later without changing the public
// tool API.
package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/sextant/internal/analysis/callgraph"
	"github.com/geoffmilleraz/sextant/internal/depgraph"
)

// registerAnalysisTools is called from registerTools (tools.go) and adds the
// six analysis tools.  backend may be nil; tools that require it return
// ErrCodeInternal when backend == nil.
func registerAnalysisTools(s *Server, backend QueryInterface) {
	registerRenameImpact(s, backend)
	registerReachable(s, backend)
	registerCallPath(s, backend)
	registerTestImpact(s, backend)
	registerDeadCode(s, backend)
	registerImportCycles(s)
}

// ─── buildGraph is a shared helper ──────────────────────────────────────────

// buildGraph fetches all symbols for service from backend and constructs a
// callgraph.Graph.  Returns (nil, rpcError) on any failure.
func buildGraph(ctx context.Context, backend QueryInterface, service string) (*callgraph.Graph, *rpcError) {
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "list symbols: " + err.Error()}
	}
	return callgraph.Build(nodes), nil
}

// ─── rename_impact ────────────────────────────────────────────────────────────

func registerRenameImpact(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "rename_impact",
		Description: "List every call site that would need updating if symbol were renamed (direct callers only).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Symbol name (bare or fully-qualified SymbolKey)",
				},
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
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_impact: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_impact: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_impact: symbol required"}
		}
		g, rpcErr := buildGraph(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		callers := callgraph.RenameImpact(g, p.Symbol)
		// Return as []any so JSON serialises to an array of strings.
		return map[string]any{"callers": stringsToAny(callers)}, nil
	})
}

// ─── reachable ────────────────────────────────────────────────────────────────

func registerReachable(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "reachable",
		Description: "Report whether there is any directed call path from one symbol to another.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"from": map[string]any{
					"type":        "string",
					"description": "Starting symbol (bare name or fully-qualified SymbolKey)",
				},
				"to": map[string]any{
					"type":        "string",
					"description": "Target symbol (bare name or fully-qualified SymbolKey)",
				},
			},
			"required": []string{"service", "from", "to"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			From    string `json:"from"`
			To      string `json:"to"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: service required"}
		}
		if p.From == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: from required"}
		}
		if p.To == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "reachable: to required"}
		}
		g, rpcErr := buildGraph(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"reachable": callgraph.Reachable(g, p.From, p.To)}, nil
	})
}

// ─── call_path ────────────────────────────────────────────────────────────────

func registerCallPath(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "call_path",
		Description: "Return the shortest directed call path between two symbols (BFS, lexicographically smallest among ties).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"from": map[string]any{
					"type":        "string",
					"description": "Starting symbol (bare name or fully-qualified SymbolKey)",
				},
				"to": map[string]any{
					"type":        "string",
					"description": "Target symbol (bare name or fully-qualified SymbolKey)",
				},
			},
			"required": []string{"service", "from", "to"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			From    string `json:"from"`
			To      string `json:"to"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "call_path: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "call_path: service required"}
		}
		if p.From == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "call_path: from required"}
		}
		if p.To == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "call_path: to required"}
		}
		g, rpcErr := buildGraph(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		path := callgraph.CallPath(g, p.From, p.To)
		found := path != nil
		return map[string]any{"found": found, "path": stringsToAny(path)}, nil
	})
}

// ─── test_impact ──────────────────────────────────────────────────────────────

func registerTestImpact(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "test_impact",
		Description: "Return all test functions (names prefixed with 'Test') that transitively reach the given symbol via the call graph.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Symbol name (bare or fully-qualified SymbolKey)",
				},
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
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: service required"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "test_impact: symbol required"}
		}
		g, rpcErr := buildGraph(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		tests := callgraph.TestImpact(g, p.Symbol)
		return map[string]any{"tests": stringsToAny(tests)}, nil
	})
}

// ─── dead_code ────────────────────────────────────────────────────────────────

func registerDeadCode(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "dead_code",
		Description: "Find symbols that are neither reachable from any entrypoint nor called by anything else in the graph.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier",
				},
				"entrypoints": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Entrypoint symbol names (bare or fully-qualified SymbolKeys); typically main functions or exported handlers",
				},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service     string   `json:"service"`
			Entrypoints []string `json:"entrypoints"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "dead_code: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "dead_code: service required"}
		}
		g, rpcErr := buildGraph(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		dead := callgraph.DeadCode(g, p.Entrypoints)
		return map[string]any{"dead": stringsToAny(dead)}, nil
	})
}

// ─── import_cycles ────────────────────────────────────────────────────────────

func registerImportCycles(s *Server) {
	s.registerTool(toolDescriptor{
		Name:        "import_cycles",
		Description: "Detect import cycles in a package import graph.  Returns the strongly-connected components that form cycles.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pkg_imports": map[string]any{
					"type":        "object",
					"description": "Map of package path → list of imported package paths",
					"additionalProperties": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
			},
			"required": []string{"pkg_imports"},
		},
		group: GroupShared,
	}, func(_ context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			PkgImports map[string]json.RawMessage `json:"pkg_imports"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "import_cycles: invalid args"}
		}
		if p.PkgImports == nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "import_cycles: pkg_imports required"}
		}

		// Decode each entry: the JSON value may be []string or []interface{}.
		pkgImports := make(map[string][]string, len(p.PkgImports))
		for pkg, raw := range p.PkgImports {
			var imports []string
			if err := json.Unmarshal(raw, &imports); err != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: "import_cycles: pkg_imports values must be arrays of strings"}
			}
			pkgImports[pkg] = imports
		}

		cycles := depgraph.ImportCycles(pkgImports)
		// Convert [][]string to []any for consistent JSON serialisation.
		result := make([]any, len(cycles))
		for i, cyc := range cycles {
			result[i] = stringsToAny(cyc)
		}
		return map[string]any{"cycles": result}, nil
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// stringsToAny converts []string to []any so json.Marshal always produces a
// JSON array (nil []string would marshal as null).
func stringsToAny(ss []string) []any {
	if ss == nil {
		return []any{}
	}
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

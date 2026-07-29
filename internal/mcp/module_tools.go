// Package mcp — the module-host scoped surface's tools (docs/plans/
// gke-module-host.md GKE-MH-5; docs/spec/expert-module-v0.md §4's serving
// floor: enumerate, scope-lookup, staleness). Registered only when a
// moduleBackend is wired in (NewServerWithModules) — a plain NewServer /
// NewServerWithFleet process never sees these tools at all, so every
// existing surface's pinned tool-list assertions are unaffected.
package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/modulehost"
)

// moduleBackend is the minimal capability the /rpc/modules surface's tools
// need — satisfied by *modulehost.Host (cmd/fathom's serve-http wiring
// passes one when --module flags mount at least one store) or a fake for
// testing. Mirrors the QueryInterface/FleetBackend pattern already used
// elsewhere in this package: a narrow interface, not a hard dependency on
// modulehost's concrete Host type.
type moduleBackend interface {
	ListModules(ctx context.Context) ([]modulehost.ModuleInfo, error)
	FederatedObligations(ctx context.Context, module, file string, max int) ([]modulehost.FederatedObligation, error)
	ModuleStaleness(ctx context.Context, module string) ([]modulehost.ModuleStaleness, bool, error)
}

// registerModuleTools wires the /rpc/modules-scoped floor tool set into s.
// Every tool is GroupModules — visible on bare /rpc (the full, unscoped
// registry, same as every other group) and the named "modules" surface,
// never added to companionTools (/rpc/agent's allowlist stays untouched —
// the plan's design section's doctrine-4 argument for why this is a new
// surface rather than an /rpc/agent extension).
func registerModuleTools(s *Server, mb moduleBackend) {
	s.registerTool(toolDescriptor{
		Name:        "list_modules",
		Description: "List every module store mounted read-only on this host (docs/plans/gke-module-host.md): one row per (module, corpus/service), each carrying the module's identity and the ingest-time watermark (indexed_commit) it was compiled at (expert-module-v0.md §4.1's Enumerate requirement).",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		group: GroupModules,
	}, func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		infos, err := mb.ListModules(ctx)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		return map[string]any{"modules": infos}, nil
	})

	s.registerTool(toolDescriptor{
		Name:        "module_obligations",
		Description: "Scope-lookup query (expert-module-v0.md §4.2, the obligations-shaped pattern) over one or every mounted module: returns the claims governing the given file, each carrying its citation, evidence tier, and non-fresh status (store-format-v0.md §3 Server conformance), tagged with the module (and service/corpus) that served it — never merged into an ambiguous unattributed set.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"module": map[string]any{"type": "string", "description": "Optional: scope to one mounted module by name. Omit to fan out across every mounted module."},
				"file":   map[string]any{"type": "string", "description": "Repo-relative file path to look up governing rules for"},
				"max":    map[string]any{"type": "integer", "description": "Maximum rules per (module, service) pair (default: semantic.MaxObligations)"},
			},
			"required": []string{"file"},
		},
		group: GroupModules,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Module string `json:"module"`
			File   string `json:"file"`
			Max    int    `json:"max"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "module_obligations: invalid args"}
		}
		if p.File == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "module_obligations: file required"}
		}
		results, err := mb.FederatedObligations(ctx, p.Module, p.File, p.Max)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		return map[string]any{"results": results}, nil
	})

	s.registerTool(toolDescriptor{
		Name:        "module_staleness",
		Description: "Report per-(module, service) staleness (expert-module-v0.md §4.3): the module's compile-time watermark (indexed_commit) and whether it could be re-checked. A mounted module is a detached corpus by default — status reads \"unknown-detached\" unless the module's corpus path is present and re-checkable on this machine, never silently reported as fresh.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"module": map[string]any{"type": "string", "description": "Mounted module name"},
			},
			"required": []string{"module"},
		},
		group: GroupModules,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Module string `json:"module"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "module_staleness: invalid args"}
		}
		if p.Module == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "module_staleness: module required"}
		}
		report, found, err := mb.ModuleStaleness(ctx, p.Module)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		if !found {
			return nil, &rpcError{Code: ErrCodeNotFound, Message: "module_staleness: module " + p.Module + " is not mounted"}
		}
		return map[string]any{"module": p.Module, "corpora": report}, nil
	})
}

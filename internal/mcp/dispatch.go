// Package mcp — dispatch layer.
//
// registerBuiltins wires the core MCP protocol methods (initialize, ping,
// tools/list, tools/call, notifications/initialized). Tool handlers are
// registered separately in tools.go via registerTools.
package mcp

import (
	"context"
	"encoding/json"
	"time"
)

// ToolGroup classifies a tool so the active profile filter can include/exclude
// it from tools/list responses.
type ToolGroup string

const (
	GroupCoding        ToolGroup = "coding"
	GroupInvestigation ToolGroup = "investigation"
	GroupShared        ToolGroup = "shared"
	GroupPlatform      ToolGroup = "platform"
	// GroupModules is the module-host floor tool set (docs/plans/
	// gke-module-host.md GKE-MH-5): list_modules, module_obligations,
	// module_staleness. Visible on bare /rpc (the full registry, same as
	// every other group) and on the named "modules" surface
	// (POST /rpc/modules) — never added to companionTools (/rpc/agent's
	// allowlist stays untouched, doctrine 4).
	GroupModules ToolGroup = "modules"
)

// toolDescriptor is the MCP tool schema entry returned in tools/list.
type toolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	group       ToolGroup      // unexported; not serialised

	// aliases maps an additive alias parameter name to the primary/native
	// parameter name it stands in for (P1-1: find_symbol's "symbol" alias
	// for "name", references' for "query", query_symbols' for "concern").
	// Unexported, not serialised — schemas document the alias in prose
	// (InputSchema's property description), and this map exists purely so
	// dispatch-level required-field validation (P1-8, see
	// handleToolsCallWithCtx) knows an alias satisfies its primary's
	// "required" entry. nil for tools with no alias.
	aliases map[string]string
}

// toolEntry pairs a descriptor with its handler.
type toolEntry struct {
	desc    toolDescriptor
	handler func(ctx context.Context, args json.RawMessage) (any, *rpcError)
}

// registerTool adds a tool to the server's tool registry.
func (s *Server) registerTool(desc toolDescriptor, h func(ctx context.Context, args json.RawMessage) (any, *rpcError)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append(s.tools, toolEntry{desc: desc, handler: h})
}

// findTool returns the toolEntry for a given name, or (zero, false).
func (s *Server) findTool(name string) (toolEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, te := range s.tools {
		if te.desc.Name == name {
			return te, true
		}
	}
	return toolEntry{}, false
}

// registerBuiltins wires the MCP protocol handlers: initialize, ping,
// tools/list, tools/call, notifications/initialized.
func registerBuiltins(s *Server) {
	s.Register("initialize", s.handleInitialize)
	s.Register("ping", s.handlePing)
	s.Register("tools/list", s.handleToolsList)
	s.Register("tools/call", s.handleToolsCall)
	// notifications/initialized is a notification — the Serve loop swallows it
	// before dispatch; this registration is defensive so it doesn't return
	// -32601 if somehow a client sends it with a non-null id.
	s.Register("notifications/initialized", func(_ context.Context, _ json.RawMessage) (any, *rpcError) {
		return map[string]any{}, nil
	})
}

// ─── built-in handlers ───────────────────────────────────────────────────────

func (s *Server) handleInitialize(_ context.Context, _ json.RawMessage) (any, *rpcError) {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools":     map[string]any{"listChanged": false},
			"resources": map[string]any{"listChanged": false},
			"prompts":   map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    "fathom",
			"version": "1.0.0",
		},
	}, nil
}

func (s *Server) handlePing(_ context.Context, _ json.RawMessage) (any, *rpcError) {
	return map[string]any{"pong": true}, nil
}

func (s *Server) handleToolsList(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
	descs := s.visibleToolsCtx(ctx)
	// Convert to []any for JSON serialisation so the output matches the MCP
	// spec (array of objects with name/description/inputSchema).
	tools := make([]any, len(descs))
	for i, d := range descs {
		tools[i] = map[string]any{
			"name":        d.Name,
			"description": d.Description,
			"inputSchema": d.InputSchema,
		}
	}
	return map[string]any{"tools": tools}, nil
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	if s.callLogger == nil {
		return s.handleToolsCallWithCtx(ctx, params)
	}
	// Extract tool name for logging before dispatching.
	var nameOnly struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(params, &nameOnly) // best-effort; errors handled below
	return s.loggedToolsCall(ctx, nameOnly.Name, params)
}

// loggedToolsCall wraps handleToolsCallWithCtx with call logging.
func (s *Server) loggedToolsCall(ctx context.Context, toolName string, params json.RawMessage) (any, *rpcError) {
	// Extract service from arguments for the log entry (best-effort).
	var args struct {
		Arguments struct {
			Service string `json:"service"`
		} `json:"arguments"`
	}
	_ = json.Unmarshal(params, &args)

	start := time.Now()
	result, rpcErr := s.handleToolsCallWithCtx(ctx, params)
	latency := time.Since(start)

	// Determine group + off-surface flag from the tool descriptor. The flag is
	// recomputed here (pure: scope + descriptor) rather than threaded through
	// the dispatch return path.
	group := ""
	offSurface := false
	if te, ok := s.findTool(toolName); ok {
		group = string(te.desc.group)
		if sc := scopeFrom(ctx); sc.enforced() && !s.toolOnSurface(sc.surface, te.desc) && !s.strictFor(sc.surface) {
			offSurface = true
		}
	}

	var logErr error
	if rpcErr != nil {
		// Construct a Go error-like message for the log entry without importing errors.
		logErr = &rpcErrWrapper{rpcErr}
	}
	claimsAttached := claimsAttachedFrom(result)
	verdictOutcome := verdictOutcomeFrom(result)
	resultCount, matchedVia := outcomeFrom(result)
	s.callLogger.log(toolName, args.Arguments.Service, group, s.effectiveSurface(ctx), "", originFrom(ctx), offSurface, claimsAttached, verdictOutcome, callOutcome{ResultCount: resultCount, MatchedVia: matchedVia}, latency, logErr)
	return result, rpcErr
}

// validateRequired checks args against desc's declared InputSchema
// "required" list (P1-8). A field is satisfied when it is present in args,
// OR when an alias registered for it in desc.aliases is present (P1-1's
// interaction — see the comment at the call site in
// handleToolsCallWithCtx). Malformed args (not a JSON object) are reported
// as ErrCodeParams here rather than deferred to the handler, so every tool
// gets the same "invalid args" shape regardless of whether it declares any
// required fields.
func validateRequired(desc toolDescriptor, args json.RawMessage) *rpcError {
	required, _ := desc.InputSchema["required"].([]string)
	if len(required) == 0 {
		return nil
	}
	present := map[string]json.RawMessage{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &present); err != nil {
			return &rpcError{Code: ErrCodeParams, Message: desc.Name + ": invalid args"}
		}
	}
	for _, field := range required {
		if _, ok := present[field]; ok {
			continue
		}
		satisfied := false
		for alias, primary := range desc.aliases {
			if primary != field {
				continue
			}
			if _, ok := present[alias]; ok {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return &rpcError{Code: ErrCodeParams, Message: desc.Name + ": missing required parameter \"" + field + "\""}
		}
	}
	return nil
}

// rpcErrWrapper adapts *rpcError to the error interface for logging.
type rpcErrWrapper struct{ e *rpcError }

func (w *rpcErrWrapper) Error() string { return w.e.Message }

func (s *Server) handleToolsCallWithCtx(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: ErrCodeParams, Message: "invalid tools/call params"}
	}
	if p.Name == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "tools/call: name required"}
	}
	te, ok := s.findTool(p.Name)
	if !ok {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "tool not found: " + p.Name}
	}
	sc := scopeFrom(ctx)
	// Hosted-modules security gate (GKE-MH-7, critic finding 2): checked
	// BEFORE — and independent of — sc.enforced(), because bare /rpc
	// deliberately has enforced()==false (it is the full, unauthenticated
	// registry by design) and would otherwise dispatch a GroupModules tool
	// with no gate at all. hostedModuleBlocked only ever fires when hosted
	// mode is active (a --modules-token-env was configured); the local,
	// loopback-only daemon never reaches this branch.
	if s.hostedModuleBlocked(sc, te.desc) {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "tool " + p.Name + " requires the " + SurfaceModules + " surface (hosted mode is active)"}
	}
	// Execute-and-flag (ADR-014 amendment): an off-surface call runs and is
	// LOGGED as off_surface — an error teaches agents to abandon the surface,
	// which looks identical to no-demand in the log. Hard gating stays
	// available behind strictSurfaces for any future multi-tenant shape (and
	// is ALWAYS on for the modules surface while hosted — strictFor).
	if sc.enforced() && !s.toolOnSurface(sc.surface, te.desc) && s.strictFor(sc.surface) {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "tool " + p.Name + " is not on the " + sc.surface + " surface; POST /rpc serves the full surface"}
	}
	// P1-8: enforce the declared InputSchema "required" fields before
	// invoking the handler, so the served contract matches what tools/list
	// advertises (previously only individually-written handler checks
	// enforced anything — C4-5's audit found at least one tool,
	// license_policy, that declared a field required and never checked it).
	//
	// Interaction with P1-1's parameter aliases: a handler may accept an
	// ADDITIVE alias for its primary argument (e.g. find_symbol's "symbol"
	// for "name"). validateRequired treats a present alias as satisfying
	// its primary's requirement (te.desc.aliases, populated at
	// registration) — so a call that only supplies the alias is NOT
	// rejected here for "missing" the native name; the handler resolves
	// which value wins (and errors loudly if both are given with different
	// values) after this gate passes. Alias resolution therefore never
	// needs to run before this check — it is encoded declaratively in
	// aliases and consulted here, rather than mutating p.Arguments.
	if rpcErr := validateRequired(te.desc, p.Arguments); rpcErr != nil {
		return nil, rpcErr
	}
	result, rpcErr := te.handler(ctx, p.Arguments)
	if rpcErr != nil {
		return nil, rpcErr
	}
	// Wrap result in MCP content envelope.
	b, err := json.Marshal(result)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "marshal result: " + err.Error()}
	}
	return map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": string(b)},
		},
	}, nil
}

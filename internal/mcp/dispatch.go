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
)

// toolDescriptor is the MCP tool schema entry returned in tools/list.
type toolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	group       ToolGroup      // unexported; not serialised
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

	// Determine group from the tool descriptor.
	group := ""
	if te, ok := s.findTool(toolName); ok {
		group = string(te.desc.group)
	}

	var logErr error
	if rpcErr != nil {
		// Construct a Go error-like message for the log entry without importing errors.
		logErr = &rpcErrWrapper{rpcErr}
	}
	s.callLogger.log(toolName, args.Arguments.Service, group, s.effectiveSurface(ctx), "", latency, logErr)
	return result, rpcErr
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
	if sc := scopeFrom(ctx); sc.enforced() && !toolOnSurface(sc.surface, te.desc) {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "tool " + p.Name + " is not on the " + sc.surface + " surface; POST /rpc serves the full surface"}
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

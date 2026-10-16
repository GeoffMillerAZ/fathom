package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// callMeta carries per-request facts to the call log: which tool ran (set for
// tools/call only), whether its result was an in-band error, and — only when
// Config.LogPayloads is set — the raw arguments and result text.
type callMeta struct {
	tool    string
	isError bool
	args    json.RawMessage
	result  string
}

// dispatch routes one non-notification request to its method handler. Unknown
// methods — including the unadvertised prompts/* and resources/* capabilities
// — return -32601; see doc.go, "Protocol scope and roadmap".
func (s *Server) dispatch(ctx context.Context, req rpcRequest) (any, *rpcError, callMeta) {
	switch req.Method {
	case "initialize":
		result, rpcErr := s.handleInitialize(req.Params)
		return result, rpcErr, callMeta{}
	case "ping":
		// MCP ping: respond promptly with an empty result.
		return emptyObject{}, nil, callMeta{}
	case "notifications/initialized":
		// Normally a notification, swallowed before dispatch. Defensive: a
		// client that (incorrectly) sends it WITH an id still gets a benign
		// empty result instead of -32601.
		return emptyObject{}, nil, callMeta{}
	case "tools/list":
		return s.handleToolsList(), nil, callMeta{}
	case "tools/call":
		return s.handleToolsCall(ctx, req.Params)
	default:
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "method not found: " + req.Method}, callMeta{}
	}
}

// handleInitialize negotiates the protocol version (echo the client's
// revision when known, else offer our latest) and reports tools-only
// capabilities plus the configured serverInfo.
func (s *Server) handleInitialize(params json.RawMessage) (any, *rpcError) {
	requested := ""
	if len(params) > 0 {
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "invalid initialize params: " + err.Error()}
		}
		requested = p.ProtocolVersion
	}
	version := latestProtocolVersion
	if knownProtocolVersions[requested] {
		version = requested
	}
	return initializeResult{
		ProtocolVersion: version,
		ServerInfo:      serverInfo{Name: s.cfg.Name, Version: s.cfg.Version},
	}, nil
}

// handleToolsList converts registry.Defs() — already sorted by name — into
// MCP tool descriptors. Pagination params are ignored: the full list is
// always returned in one page, so any cursor a client sends is moot.
func (s *Server) handleToolsList() any {
	defs := s.reg.Defs()
	tools := make([]toolDescriptor, len(defs))
	for i, d := range defs {
		schema := d.InputSchema
		if len(schema) == 0 {
			// MCP requires inputSchema to be a JSON Schema object; a tool
			// registered without one takes no arguments.
			schema = json.RawMessage(`{"type":"object"}`)
		}
		eff, _ := s.reg.EffectOf(d.Name)
		tools[i] = toolDescriptor{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: schema,
			Annotations: annotationsFromEffect(eff),
		}
	}
	return toolsListResult{Tools: tools}
}

// annotationsFromEffect maps a tool's declared Effect onto the MCP
// annotations object emitted in tools/list. nil (undeclared) emits no
// annotations at all — the kit never invents a declaration on a tool's
// behalf, and a client applying the MCP defaults to the absence reads it as
// destructive, which is the strict truth. A declared Effect emits all four
// hints explicitly (see toolAnnotations).
func annotationsFromEffect(eff *toolregistry.Effect) *toolAnnotations {
	if eff == nil {
		return nil
	}
	return &toolAnnotations{
		ReadOnlyHint:    eff.ReadOnlyHint,
		DestructiveHint: eff.DestructiveHint,
		IdempotentHint:  eff.IdempotentHint,
		OpenWorldHint:   eff.OpenWorldHint,
	}
}

// handleToolsCall synthesizes a tool_use ContentBlock from the MCP params and
// hands it to registry.Dispatch, which owns policy, timeout, and panic
// handling. Every execution failure mode comes back as an in-band isError
// result; -32602 is reserved for structurally invalid params.
func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, *rpcError, callMeta) {
	if len(params) == 0 {
		return nil, &rpcError{Code: ErrCodeParams, Message: "tools/call: params required"}, callMeta{}
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: ErrCodeParams, Message: "invalid tools/call params: " + err.Error()}, callMeta{}
	}
	if p.Name == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "tools/call: name required"}, callMeta{}
	}

	args := p.Arguments
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage(`{}`)
	}

	// The synthesized id is internal plumbing (Dispatch echoes it on the
	// tool_result; MCP responses do not carry it); a counter keeps it unique
	// per server without consulting the clock.
	id := fmt.Sprintf("mcp-call-%d", s.callSeq.Add(1))
	res := s.reg.Dispatch(ctx, llmclient.ToolUseBlock(id, p.Name, args))

	meta := callMeta{tool: p.Name, isError: res.IsError}
	if s.cfg.LogPayloads {
		meta.args = args
		meta.result = res.Content
	}
	return toolCallResult{
		Content: []contentItem{{Type: "text", Text: res.Content}},
		IsError: res.IsError,
	}, nil, meta
}

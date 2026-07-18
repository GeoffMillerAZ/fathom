package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// BridgeOptions configures RegisterAll.
type BridgeOptions struct {
	// Timeout, when non-zero, is the per-tool timeout passed to each registered
	// Tool.Timeout. Zero means inherits the registry's Options.DefaultTimeout.
	Timeout interface{ String() string } // kept as a doc signal; actual type below

	// CapabilityMap, when non-nil, provides per-tool capability declarations for
	// MCP tools bridged via RegisterAll. Keys are tool names; values are the
	// CapabilitySet to attach to each Tool on registration. Tools absent from
	// the map are registered with nil Capabilities (unknown), which is treated
	// as potentially all three trifecta capabilities under TrifectaStrict mode.
	// Use toolregistry.DeclareNone() to explicitly mark a tool as having no
	// relevant capabilities.
	//
	// MCP tool descriptions are untrusted content (see tool-description
	// hash-pinning in the package doc). Operators who set CapabilityMap are
	// asserting that they trust the described capability of each named tool at
	// the time they configure the bridge; hash-pinning detects subsequent
	// description changes that might invalidate that assertion.
	CapabilityMap map[string]toolregistry.CapabilitySet
}

// maxResultBytes is the maximum byte length of a tools/call result accepted
// into the registry. Results exceeding this are replaced with a descriptive
// error — the model should see a bounded response, never an unbounded one.
const maxResultBytes = 1 << 20 // 1 MiB

// RegisterAll registers every tool the client knows about (from its initial
// tools/list) into reg. Each MCP tool becomes a toolregistry.Tool whose
// handler calls the MCP server's tools/call and maps the result back to
// (string, error). All existing Registry governance — policy checks, timeouts,
// and panic recovery — applies to MCP tools through the standard Dispatch path
// with no special-casing.
//
// Capability declarations: MCP tools are foreign and their capability
// declarations are unknown by default (nil CapabilitySet), which is treated as
// potentially all three trifecta capabilities under TrifectaStrict mode. Pass
// BridgeOptions.CapabilityMap to supply per-tool declarations. Tools absent
// from the map remain unknown. See toolregistry.CapabilitySet for the
// declared-vs-unknown distinction and trifecta enforcement details.
//
// Error mapping:
//   - ToolCallError (MCP in-band isError) → the content string as a Go error,
//     which Dispatch surfaces as an is_error tool_result.
//   - Result content exceeding maxResultBytes → error naming the size, so the
//     model sees a bounded message rather than a truncated or silently huge result.
//   - RPCError → wrapped and returned; Dispatch surfaces it as is_error.
//   - Malformed response → error.
//
// Concurrency contract: RegisterAll is intended to be called once at startup.
// After it returns, the registered handlers are called concurrently by the
// Registry's Dispatch. Each handler call is independently serialized through
// Client.mu (send+receive pairs do not interleave), so concurrent tool calls
// proceed sequentially through the MCP session — matching the MCP spec's
// intent for sequential tool execution and preventing response mis-routing.
func RegisterAll(reg *toolregistry.Registry, c *Client, opts BridgeOptions) error {
	tools, err := c.ListTools(context.Background())
	if err != nil {
		// ToolChangedError at this point would be surprising (we just listed in
		// New), but if it happens treat it as a fatal bridge error.
		return fmt.Errorf("mcpclient.RegisterAll: list tools: %w", err)
	}
	for _, tool := range tools {
		if rerr := registerOne(reg, c, tool, opts); rerr != nil {
			return rerr
		}
	}
	return nil
}

// registerOne registers a single MCP tool into reg, threading capability
// declarations from opts.CapabilityMap when present.
func registerOne(reg *toolregistry.Registry, c *Client, tool ToolInfo, opts BridgeOptions) error {
	schema := tool.InputSchema
	if len(schema) == 0 {
		schema = json.RawMessage(`{"type":"object"}`)
	}

	// Capability: look up in CapabilityMap; absent = nil (unknown).
	var caps toolregistry.CapabilitySet
	if opts.CapabilityMap != nil {
		if declared, ok := opts.CapabilityMap[tool.Name]; ok {
			caps = declared
		}
	}

	name := tool.Name // capture for closure
	t := toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		},
		Capabilities: caps,
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			result, err := c.CallTool(ctx, name, input)
			if err != nil {
				var tcErr *ToolCallError
				if errors.As(err, &tcErr) {
					// In-band MCP error: surface to the model as a handler error
					// so Dispatch marks it is_error. The content is already the
					// server's error text.
					return "", fmt.Errorf("%s", tcErr.Content)
				}
				return "", err
			}
			if len(result) > maxResultBytes {
				return "", fmt.Errorf("mcpclient: tool %q result exceeds %d bytes (%d bytes received)", name, maxResultBytes, len(result))
			}
			return result, nil
		},
	}
	if err := reg.Register(t); err != nil {
		return fmt.Errorf("mcpclient.RegisterAll: register %q: %w", tool.Name, err)
	}
	return nil
}

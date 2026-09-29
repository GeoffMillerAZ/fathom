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

	// EffectMap, when non-nil, is the OPERATOR's per-tool effect
	// declaration and wins over anything the server annotated. It is the
	// way to declare effects for an untrusted server (whose own annotations
	// RegisterAll discards) or to override a trusted one. Tools absent from
	// the map take the server's annotation when Options.TrustedServer is
	// set, and stay undeclared otherwise. Like CapabilityMap, an entry is the
	// operator's word at configuration time; hash-pinning detects a later
	// description or annotation change that might invalidate it.
	EffectMap map[string]toolregistry.Effect

	// VerifyPinBeforeCall makes each registered handler refresh and verify the
	// selected tool's metadata before sending tools/call. The zero value leaves
	// the legacy CallTool behavior unchanged. Verification is scoped to the
	// dispatched tool and cannot make a newly listed tool trusted by itself.
	VerifyPinBeforeCall bool
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
// Effect declarations follow the same rule with one extra source: the
// server's own MCP annotations, which are attached to the registered Tool
// ONLY when Options.TrustedServer is set (MCP: annotations are untrusted
// unless the server is). BridgeOptions.EffectMap is the operator's override
// and wins over the server either way. A tool with neither is registered
// undeclared — which kits/agent-harness EffectStrict reads as destructive.
//
// Error mapping:
//   - ToolCallError (MCP in-band isError) → the content string as a Go error,
//     which Dispatch surfaces as an is_error tool_result.
//   - Result content exceeding maxResultBytes → error naming the size, so the
//     model sees a bounded message rather than a truncated or silently huge result.
//   - RPCError → wrapped and returned; Dispatch surfaces it as is_error.
//   - Malformed response → error.
//
// When BridgeOptions.VerifyPinBeforeCall is set, each handler performs a fresh
// tools/list before tools/call and refuses missing, unpinned, or drifted tool
// metadata. This is not a remote transaction: server behavior can still change
// after verification and before tools/call.
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

	// Effect: the operator's EffectMap wins; else the server's annotation,
	// but only from a trusted server; else undeclared (nil).
	var eff *toolregistry.Effect
	if opts.EffectMap != nil {
		if declared, ok := opts.EffectMap[tool.Name]; ok {
			e := declared
			eff = &e
		}
	}
	if eff == nil && tool.EffectTrusted && tool.Effect != nil {
		e := *tool.Effect
		eff = &e
	}

	name := tool.Name // capture for closure
	t := toolregistry.Tool{
		Def: llmclient.ToolDef{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		},
		Capabilities: caps,
		Effect:       eff,
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var result string
			var err error
			if opts.VerifyPinBeforeCall {
				result, err = c.CallToolVerified(ctx, name, input)
			} else {
				result, err = c.CallTool(ctx, name, input)
			}
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

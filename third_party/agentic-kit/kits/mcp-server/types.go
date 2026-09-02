package mcpserver

import (
	"encoding/json"
	"io"
)

// JSON-RPC 2.0 error codes used by this server.
const (
	// ErrCodeParse: malformed JSON, or a message exceeding MaxMessageBytes.
	ErrCodeParse = -32700
	// ErrCodeInvalid: well-formed JSON that is not a JSON-RPC request object,
	// including JSON-RPC batch arrays (deliberately unsupported; see doc.go).
	ErrCodeInvalid = -32600
	// ErrCodeNotFound: unknown method (including unadvertised capabilities
	// such as prompts/* and resources/*).
	ErrCodeNotFound = -32601
	// ErrCodeParams: structurally invalid params for a known method.
	ErrCodeParams = -32602
	// ErrCodeInternal: the server failed to marshal a response.
	ErrCodeInternal = -32603
)

// DefaultMaxMessageBytes is the message-size bound applied when
// Config.MaxMessageBytes is zero: 4 MiB, matching common MCP client limits.
const DefaultMaxMessageBytes = 4 << 20

// knownProtocolVersions are the MCP protocol revisions this server can speak.
// The tools-only surface served here (initialize, ping, tools/list,
// tools/call over newline-delimited stdio) is identical across all of them,
// so "knowing" a revision means echoing it back in initialize. Extend this
// set when new revisions are verified.
//
// 2025-11-25 adds no wire changes for a stdio+tools-only server: the
// initialize request/response shape, tools/list, tools/call, ping, and
// notifications/initialized are identical. Behavioral additions in that
// revision (elicitation, completion, structured tool output, OAuth 2.1
// resource server) are capability-gated — a client that requests them sees
// the capability absent in the initialize response and does not attempt them.
// Adding 2025-11-25 here therefore costs nothing and lets up-to-date clients
// negotiate it rather than falling back to the older revision.
var knownProtocolVersions = map[string]bool{
	"2024-11-05": true,
	"2025-03-26": true,
	"2025-06-18": true,
	"2025-11-25": true,
}

// latestProtocolVersion is offered when the client requests a revision this
// server does not know, per the MCP version-negotiation rules.
const latestProtocolVersion = ProtocolRevision

// Config configures a Server. The CUE contract for the data-shaped fields
// lives in schema.cue; CallLog is a behavioral seam with no CUE shape.
type Config struct {
	// Name is the server name reported in the initialize serverInfo.
	Name string

	// Version is the server version reported in the initialize serverInfo.
	Version string

	// MaxMessageBytes bounds one newline-delimited JSON-RPC message. Zero or
	// negative means DefaultMaxMessageBytes (4 MiB). Oversized messages are
	// rejected with -32700 before parsing; see doc.go, "Input bounds and
	// resync".
	MaxMessageBytes int

	// CallLog, when non-nil, receives one JSONL entry per dispatched request
	// via foundations/persistence/jsonl-log: ts, method, tool, duration_ms,
	// is_error. Tool arguments and results are NEVER included unless
	// LogPayloads is set. nil disables logging.
	CallLog io.Writer

	// LogPayloads opts the call log into recording tools/call arguments and
	// results. SENSITIVE: payloads carry user data — leave this off unless
	// the log's storage and retention are scoped for it.
	LogPayloads bool
}

// rpcRequest is a minimal JSON-RPC 2.0 request. ID stays a json.RawMessage so
// string, number, and null ids pass through verbatim.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// rpcResponse is the envelope written back to the client. Exactly one of
// Result and Error is set.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC 2.0 error object: {code, message, data?}.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// emptyObject marshals to {} — the MCP-spec result for ping and the shape of
// an advertised-but-optionless capability.
type emptyObject struct{}

// initializeResult is the initialize response per the MCP spec.
type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
}

// capabilities advertises what this server can do. v1 is tools only; see
// doc.go, "Protocol scope and roadmap".
type capabilities struct {
	Tools emptyObject `json:"tools"`
}

// serverInfo identifies the server to the client, from Config.Name/Version.
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// toolsListResult is the tools/list response.
type toolsListResult struct {
	Tools []toolDescriptor `json:"tools"`
}

// toolDescriptor is one MCP tool entry, built from an llmclient.ToolDef plus
// the tool's declared Effect, when it has one (annotations is omitted for an
// undeclared tool, so the descriptor is byte-identical to the pre-Effect
// shape).
type toolDescriptor struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema json.RawMessage  `json:"inputSchema"`
	Annotations *toolAnnotations `json:"annotations,omitempty"`
}

// toolAnnotations is the MCP ToolAnnotations object (spec 2025-03-26
// onward, unchanged through 2026-07-28), field names verbatim. Every hint is
// emitted explicitly — never omitted — because the spec DEFAULTS an absent
// destructiveHint to true and an absent openWorldHint to true, which would
// silently invert an explicit all-false declaration. title is not carried:
// the kit has no display-name concept separate from the tool name.
type toolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

// toolCallResult is the tools/call response: the registry's tool_result
// content as a single text item, with the in-band error flag.
type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError"`
}

// contentItem is one MCP content element; this server emits type "text" only.
type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

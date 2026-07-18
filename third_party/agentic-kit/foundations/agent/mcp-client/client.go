package mcpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// offeredVersions are the MCP protocol revisions this client offers during
// initialize, in preference order (highest first). The server picks from this
// list; we record its choice in Client.ProtocolVersion.
var offeredVersions = []string{"2025-11-25", "2025-06-18"}

// Options configures a Client.
type Options struct {
	// ClientName is the name reported in the initialize clientInfo.
	// Defaults to "agentic-kit/mcp-client" when empty.
	ClientName string

	// ClientVersion is the version reported in the initialize clientInfo.
	// Defaults to "0" when empty.
	ClientVersion string
}

// ToolInfo holds the description fields of one MCP tool and the SHA-256
// hash used for pin verification.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	// Hash is SHA-256(name ‖ "\x00" ‖ description ‖ "\x00" ‖ inputSchema).
	Hash string
}

// ToolChangedError is returned when a listed tool's description hash does not
// match the pinned value recorded at first list or last Repin.
type ToolChangedError struct {
	Name    string
	OldHash string
	NewHash string
}

// Error implements the error interface, naming the tool and the pinned vs.
// current hash prefixes.
func (e *ToolChangedError) Error() string {
	return fmt.Sprintf("mcpclient: tool %q description changed (pin %s → %s); call Repin to accept", e.Name, e.OldHash[:8], e.NewHash[:8])
}

// ToolCallError is returned when the MCP server reports an in-band tool
// error (isError == true in the response).
type ToolCallError struct {
	Name    string
	Content string
}

// Error implements the error interface, naming the tool and its in-band
// error content.
func (e *ToolCallError) Error() string {
	return fmt.Sprintf("mcpclient: tool %q returned error: %s", e.Name, e.Content)
}

// Client connects to an MCP server over a Transport and exposes its tools.
// Construct with New; the zero value is not usable.
//
// Concurrency contract: Client is safe for concurrent use. Calls and list
// operations are serialized through an internal send/receive mutex. The pin
// table is guarded by a separate sync.RWMutex so Repin and VerifyPin may be
// called from any goroutine.
type Client struct {
	tr              Transport
	opts            Options
	ProtocolVersion string // set by New after successful handshake

	mu    sync.Mutex // serialises all send+receive sequences
	idSeq atomic.Int64

	pinMu sync.RWMutex
	pins  map[string]string // tool name → hash
}

// New opens a Client by performing the MCP initialize handshake over tr. It
// offers protocol versions 2025-11-25 and 2025-06-18 and records the server's
// chosen version. On success New immediately calls tools/list and pins the
// returned tool descriptions.
//
// New returns a non-nil error if the handshake fails, the server returns an
// RPC error, or the initial pin fails due to a parse error.
func New(ctx context.Context, tr Transport, opts Options) (*Client, error) {
	if opts.ClientName == "" {
		opts.ClientName = "agentic-kit/mcp-client"
	}
	if opts.ClientVersion == "" {
		opts.ClientVersion = "0"
	}
	c := &Client{
		tr:   tr,
		opts: opts,
		pins: make(map[string]string),
	}
	if err := c.handshake(ctx); err != nil {
		return nil, err
	}
	// Pin tool descriptions at startup so any subsequent drift is detected.
	if _, err := c.ListTools(ctx); err != nil {
		return nil, fmt.Errorf("mcpclient: initial tool list: %w", err)
	}
	return c, nil
}

// Close closes the underlying transport. Subsequent calls on the Client will
// return transport errors.
func (c *Client) Close() error {
	return c.tr.Close()
}

// ListTools requests tools/list from the server and returns the tool
// descriptions, verifying them against the pinned hashes. On first call (made
// internally by New) the hashes are recorded; on subsequent calls any tool
// whose hash differs from the pin causes a ToolChangedError for that tool —
// the call still returns the full list so the caller can inspect all changes,
// but the error is non-nil.
//
// Descriptions of foreign tools are untrusted content: they may contain
// prompt-injection payloads. Pin / ToolChangedError is the detection mechanism.
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	resp, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: tools/list: %w", err)
	}

	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: tools/list parse: %w", err)
	}

	tools := make([]ToolInfo, len(result.Tools))
	var firstErr error

	c.pinMu.Lock()
	defer c.pinMu.Unlock()

	for i, t := range result.Tools {
		h := toolHash(t.Name, t.Description, t.InputSchema)
		tools[i] = ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			Hash:        h,
		}
		if existing, pinned := c.pins[t.Name]; pinned {
			if existing != h && firstErr == nil {
				firstErr = &ToolChangedError{Name: t.Name, OldHash: existing, NewHash: h}
			}
		} else {
			// First list: record the pin.
			c.pins[t.Name] = h
		}
	}
	return tools, firstErr
}

// Repin explicitly re-records the current description hash for name, silencing
// future ToolChangedError for that tool. Call this only after reviewing the new
// description and confirming it is safe — rebinding requires a deliberate
// human or policy decision.
func (c *Client) Repin(name, hash string) {
	c.pinMu.Lock()
	defer c.pinMu.Unlock()
	c.pins[name] = hash
}

// VerifyPin returns the pinned hash for name and whether one exists.
func (c *Client) VerifyPin(name string) (hash string, pinned bool) {
	c.pinMu.RLock()
	defer c.pinMu.RUnlock()
	h, ok := c.pins[name]
	return h, ok
}

// CallTool invokes the named tool with args and returns the text content of
// the response. It performs NO pin verification of its own: it neither
// re-fetches the tool description nor consults the pin table recorded at
// list time. Description drift is caught only at the next ListTools call
// (which re-verifies every pin) — a pre-call pin check is a proposed
// hardening item in TRACKING.md's pull queue, not a current behavior.
//
// It returns ToolCallError when the server reports isError == true in the
// response. Other errors are transport-level or parse failures.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	params, err := json.Marshal(struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}{Name: name, Arguments: args})
	if err != nil {
		return "", fmt.Errorf("mcpclient: tools/call marshal: %w", err)
	}

	resp, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return "", fmt.Errorf("mcpclient: tools/call: %w", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("mcpclient: tools/call parse: %w", err)
	}

	// Collect text content.
	var text string
	for _, item := range result.Content {
		if item.Type == "text" {
			text = item.Text
			break
		}
	}

	if result.IsError {
		return "", &ToolCallError{Name: name, Content: text}
	}
	return text, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// handshake sends initialize, reads the response, and sends
// notifications/initialized. It must be called once before any other method.
func (c *Client) handshake(ctx context.Context) error {
	params, err := json.Marshal(struct {
		ProtocolVersion string   `json:"protocolVersion"`
		Capabilities    struct{} `json:"capabilities"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}{
		ProtocolVersion: offeredVersions[0],
		ClientInfo: struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}{Name: c.opts.ClientName, Version: c.opts.ClientVersion},
	})
	if err != nil {
		return fmt.Errorf("mcpclient: initialize params: %w", err)
	}

	result, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("mcpclient: initialize: %w", err)
	}

	var initResult struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("mcpclient: initialize parse: %w", err)
	}
	c.ProtocolVersion = initResult.ProtocolVersion

	// Send notifications/initialized (notification: no id, no response expected).
	notif, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	return c.tr.Send(ctx, notif)
}

// call sends a JSON-RPC request and returns the raw result bytes. It is the
// only entry point into the send+receive loop and is guarded by c.mu so
// pipelining cannot interleave responses.
func (c *Client) call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	id := c.idSeq.Add(1)

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if len(params) > 0 {
		req["params"] = json.RawMessage(params)
	}
	msg, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.tr.Send(ctx, msg); err != nil {
		return nil, err
	}

	raw, err := c.tr.Receive(ctx)
	if err != nil {
		return nil, err
	}

	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if resp.Error != nil {
		return nil, &RPCError{Code: resp.Error.Code, Message: resp.Error.Message}
	}
	return resp.Result, nil
}

// toolHash returns hex(SHA-256(name ‖ NUL ‖ description ‖ NUL ‖ inputSchema)).
// The NUL separators prevent concatenation collisions between fields.
func toolHash(name, description string, inputSchema json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte{0})
	h.Write([]byte(description))
	h.Write([]byte{0})
	h.Write(inputSchema)
	return hex.EncodeToString(h.Sum(nil))
}

// RPCError is a JSON-RPC protocol-level error returned by the server.
type RPCError struct {
	Code    int
	Message string
}

// Error implements the error interface, reporting the JSON-RPC error code
// and message.
func (e *RPCError) Error() string {
	return fmt.Sprintf("mcpclient: rpc error %d: %s", e.Code, e.Message)
}

// IsRPCError reports whether err is (or wraps) an RPCError with the given code.
func IsRPCError(err error, code int) bool {
	var rpcErr *RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == code
}

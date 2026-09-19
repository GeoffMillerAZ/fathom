package mcpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"
)

// offeredVersions are the MCP protocol revisions this client offers during
// initialize, in preference order (highest first). The server picks from this
// list; we record its choice in Client.ProtocolVersion.
var offeredVersions = []string{ProtocolRevision, "2025-06-18"}

// Options configures a Client.
type Options struct {
	// ClientName is the name reported in the initialize clientInfo.
	// Defaults to "agentic-kit/mcp-client" when empty.
	ClientName string

	// ClientVersion is the version reported in the initialize clientInfo.
	// Defaults to "0" when empty.
	ClientVersion string

	// TrustedServer marks the server's tool annotations as trusted. MCP
	// requires clients to treat annotations (readOnlyHint, destructiveHint,
	// …) as UNTRUSTED unless they come from a trusted server; this flag is
	// that judgement, made by the operator at configuration time. When
	// false (the default) ListTools still parses annotations into
	// ToolInfo.Effect but reports EffectTrusted=false, and RegisterAll
	// registers the tool with NO effect — undeclared, which a strict harness
	// reads as destructive. When true, RegisterAll attaches the parsed
	// Effect to the registered Tool. Description hash-pinning covers
	// annotations either way, so a trusted server that later flips a hint
	// trips ToolChangedError.
	TrustedServer bool
}

// ToolInfo holds the description fields of one MCP tool and the SHA-256
// hash used for pin verification.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema json.RawMessage

	// Annotations is the raw MCP annotations object as the server sent it
	// (nil when absent). It is part of the pinned Hash.
	Annotations json.RawMessage

	// Effect is Annotations mapped onto the registry's Effect type with the
	// MCP defaults applied to absent hints (see effectFromAnnotations); nil
	// when the server sent none or sent something malformed.
	Effect *toolregistry.Effect

	// EffectTrusted reports whether Effect may be believed: it is
	// Options.TrustedServer, stamped per listing so a consumer holding a
	// ToolInfo cannot lose the provenance of the hint.
	EffectTrusted bool

	// Hash is SHA-256(name ‖ NUL ‖ description ‖ NUL ‖ canonical inputSchema),
	// extended with canonical annotations when the tool carries any. Insignificant
	// JSON whitespace and object-key order do not cause drift.
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
	return fmt.Sprintf("mcpclient: tool %q description changed (pin %s → %s); call Repin to accept", e.Name, shortHash(e.OldHash), shortHash(e.NewHash))
}

// ToolNotPinnedError is returned by CallToolVerified when name did not have a
// pin before verification started. A tools/list response received during that
// verification cannot authorize a newly discovered tool.
type ToolNotPinnedError struct {
	Name string
}

// Error implements the error interface.
func (e *ToolNotPinnedError) Error() string {
	return fmt.Sprintf("mcpclient: tool %q has no pre-existing metadata pin", e.Name)
}

// ToolNotFoundError is returned by CallToolVerified when a pinned tool is
// absent from the fresh tools/list response.
type ToolNotFoundError struct {
	Name string
}

// Error implements the error interface.
func (e *ToolNotFoundError) Error() string {
	return fmt.Sprintf("mcpclient: tool %q is absent from tools/list", e.Name)
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
	return c.listTools(ctx, false)
}

func (c *Client) listTools(ctx context.Context, strict bool) ([]ToolInfo, error) {
	resp, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: tools/list: %w", err)
	}

	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
			Annotations json.RawMessage `json:"annotations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: tools/list parse: %w", err)
	}
	if strict {
		if err := validateMetadataJSON(resp); err != nil {
			return nil, err
		}
		seen := make(map[string]bool)
		for _, tool := range result.Tools {
			if strings.TrimSpace(tool.Name) == "" || strings.ContainsRune(tool.Name, 0) || seen[tool.Name] || strings.ContainsRune(tool.Description, 0) {
				return nil, fmt.Errorf("mcpclient: ambiguous tool metadata identity")
			}
			seen[tool.Name] = true
			if !metadataObject(tool.InputSchema) || (len(tool.Annotations) > 0 && !metadataObject(tool.Annotations)) {
				return nil, fmt.Errorf("mcpclient: tool metadata schema/annotations must be objects")
			}
		}
	}

	tools := make([]ToolInfo, len(result.Tools))
	var firstErr error

	c.pinMu.Lock()
	defer c.pinMu.Unlock()

	for i, t := range result.Tools {
		h := toolHash(t.Name, t.Description, t.InputSchema, t.Annotations)
		tools[i] = ToolInfo{
			Name:          t.Name,
			Description:   t.Description,
			InputSchema:   t.InputSchema,
			Annotations:   t.Annotations,
			Effect:        effectFromAnnotations(t.Annotations),
			EffectTrusted: c.opts.TrustedServer,
			Hash:          h,
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
// (which re-verifies every pin). Use CallToolVerified to opt into a fresh
// pre-call check; existing CallTool behavior remains unchanged.
//
// It returns ToolCallError when the server reports isError == true in the
// response. Other errors are transport-level or parse failures.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	return c.callTool(ctx, name, args)
}

// CallToolVerified refreshes the named tool's metadata and refuses to call it
// unless it had a pin before verification began and its fresh metadata matches
// the currently pinned hash. It returns ToolNotPinnedError for a tool observed
// only in the fresh list, ToolNotFoundError when a pinned tool disappeared, and
// ToolChangedError when the metadata drifted. No tools/call request is sent on
// those failures or on a tools/list error.
//
// Repin is concurrent-safe: a Repin completed while tools/list is in flight is
// honored if its new pin matches the fresh metadata. This verification and the
// following remote tools/call request are necessarily not atomic; a server can
// change its behavior after verification.
func (c *Client) CallToolVerified(ctx context.Context, name string, args json.RawMessage) (string, error) {
	// Check before ListTools because that method records pins for newly observed
	// tools. Otherwise a malicious list response could authorize its own call.
	if _, pinned := c.VerifyPin(name); !pinned {
		return "", &ToolNotPinnedError{Name: name}
	}

	tools, err := c.listTools(ctx, true)
	if err != nil {
		var changed *ToolChangedError
		if !errors.As(err, &changed) {
			return "", fmt.Errorf("mcpclient: verified tools/list: %w", err)
		}
		// ListTools reports the first changed tool in the complete listing. The
		// verified path is deliberately scoped to the tool about to be called;
		// its hash below decides whether this call is safe to dispatch.
	}

	var fresh *ToolInfo
	for i := range tools {
		if tools[i].Name == name {
			fresh = &tools[i]
			break
		}
	}
	if fresh == nil {
		return "", &ToolNotFoundError{Name: name}
	}

	// Keep local authorization stable through dispatch. This cannot freeze the
	// remote server, but Repin must not invalidate our local check before send.
	c.pinMu.RLock()
	defer c.pinMu.RUnlock()
	pinnedHash, pinned := c.pins[name]
	if !pinned {
		return "", &ToolNotPinnedError{Name: name}
	}
	if pinnedHash != fresh.Hash {
		return "", &ToolChangedError{Name: name, OldHash: pinnedHash, NewHash: fresh.Hash}
	}

	return c.callTool(ctx, name, args)
}

func (c *Client) callTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
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

// toolHash returns hex(SHA-256(name ‖ NUL ‖ description ‖ NUL ‖ inputSchema
// [‖ NUL ‖ annotations])). JSON fields are normalized through encoding/json so
// irrelevant whitespace and object-key order do not cause a pin change. The
// NUL separators prevent concatenation collisions between fields.
func toolHash(name, description string, inputSchema, annotations json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte{0})
	h.Write([]byte(description))
	h.Write([]byte{0})
	h.Write(canonicalJSON(inputSchema))
	if len(annotations) > 0 {
		h.Write([]byte{0})
		h.Write(canonicalJSON(annotations))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalJSON returns encoding/json's stable representation of raw. Raw
// metadata comes from a successfully parsed JSON-RPC response; retaining raw
// bytes on the impossible invalid path preserves a deterministic hash instead
// of silently dropping the field.
func canonicalJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber() // distinct integers beyond 2^53 must retain distinct pins
	if err := decoder.Decode(&value); err != nil {
		return raw
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return canonical
}

func shortHash(hash string) string {
	const prefixLen = 8
	if len(hash) <= prefixLen {
		return hash
	}
	return hash[:prefixLen]
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

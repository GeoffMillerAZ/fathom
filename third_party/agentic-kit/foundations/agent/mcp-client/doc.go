// Package mcpclient is the consuming side of the Model Context Protocol: it
// connects to an MCP server, lists its tools, calls them, and bridges them
// into a foundations/agent/tool-registry Registry so that ALL existing
// governance — policy checks, timeouts, panic recovery, and injection
// screening at the harness layer — applies to foreign tools without any
// special-casing.
//
// # Transport
//
// Communication with the server goes through a Transport seam:
//
//	type Transport interface {
//		Send(ctx context.Context, msg []byte) error
//		Receive(ctx context.Context) ([]byte, error)
//		Close() error
//	}
//
// This package ships two Transport implementations:
//
//   - IOTransport speaks the same newline-delimited JSON-RPC 2.0 framing
//     that kits/mcp-server uses over stdio — one message per line,
//     CRLF-tolerant, with a configurable size bound.
//   - NewHTTPTransport speaks MCP Streamable HTTP (2025-11-25, stateless
//     mode): each message is POSTed to a single endpoint; request responses
//     arrive as the HTTP response body, notifications are acknowledged with
//     202/204 and no body. Send injects a W3C traceparent header from the
//     ambient event.TraceContext (zero/absent trace → no header). SSE
//     response streams, mcp-session-id resumability, retries, TLS, and auth
//     are NOT provided — the injected Doer owns connection-level concerns.
//
// Subprocess management is NOT in this package. Spawning the server process
// is the caller's responsibility; foundations/agent/bounded-exec exists for
// that. The composition looks like:
//
//	runner, _ := boundedexec.New(boundedexec.Config{...})
//	result, _ := runner.Start(ctx, "path/to/mcp-server", args...)
//	tr := mcpclient.NewIOTransport(result.Stdin, result.Stdout, mcpclient.DefaultMaxMessageBytes)
//	client, _ := mcpclient.New(ctx, tr, mcpclient.Options{})
//
// For a remote server, substitute the transport line:
//
//	tr := mcpclient.NewHTTPTransport("https://host/mcp", httpClient, mcpclient.DefaultMaxMessageBytes)
//
// Tests drive everything through in-process pipes connected to a scripted fake
// server (stdio) and httptest servers (HTTP) — no real subprocesses, no
// network.
//
// # Initialization and version negotiation
//
// New performs the MCP initialize handshake synchronously: it sends an
// initialize request offering protocol versions 2025-06-18 and 2025-11-25,
// records the server's chosen version, and sends notifications/initialized.
// The server's choice is accessible via Client.ProtocolVersion.
//
// # Tool-description hash-pinning (tool-poisoning defence)
//
// Foreign tool descriptions are untrusted content. An attacker who can modify
// an MCP server's tool descriptions between calls can inject instructions into
// the agent's context ("ignore previous instructions…" hidden in a description
// the agent re-reads on each tools/list). This is the tool-poisoning attack
// documented in the 2026 landscape findings [landscape/tool-poisoning].
//
// Pin records a SHA-256 of (name ‖ description ‖ inputSchema) for each tool at
// first list. Every subsequent tools/list — and every tools/call — verifies the
// descriptions match the pinned hash. If any tool's description has changed,
// Receive returns a ToolChangedError rather than silently forwarding modified
// instructions to the model. Rebinding a changed tool requires an explicit
// Repin call, making drift visible and auditable.
//
// # Registry bridge
//
// RegisterAll adapts every tool from a Client into a tool-registry Tool whose
// handler calls the MCP server's tools/call and maps the result back to the
// registry's (string, error) contract. Policy, timeouts, and panic recovery are
// inherited from the Registry — they apply to MCP tools just as they do to
// local tools, with no special-casing. Oversized or malformed results are
// mapped to typed errors; the registry handler never panics.
//
// # Concurrency contract
//
// Client is safe for concurrent use: Send and Receive are serialized by
// internal mutexes; the pin table is guarded by a sync.RWMutex. Neither
// IOTransport nor the HTTP transport is safe for concurrent use standalone —
// the Send→Receive pairing must be driven by a single goroutine (or the
// Client's internal serialization). RegisterAll is called once at startup;
// after that the registered tools are dispatched concurrently by the Registry.
// Context cancellation is honoured on every blocking Receive call: Receive
// runs the blocking read in a goroutine and selects between it and ctx.Done().
// An abandoned reader goroutine exits when the pipe closes or data arrives.
package mcpclient

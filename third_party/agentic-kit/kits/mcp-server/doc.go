// Package mcpserver is an MCP (Model Context Protocol) server kit: it
// exposes the tools of a foundations/agent/tool-registry Registry to any MCP
// client (Claude Code, Claude Desktop, ...) over two transports sharing one
// dispatch core — newline-delimited JSON-RPC 2.0 on a byte stream (stdio,
// Serve) and MCP Streamable HTTP (NewHTTPHandler, a single stateless POST
// endpoint). A project that already has a Registry becomes an MCP server in
// ~20 lines:
//
//	reg := toolregistry.New(toolregistry.Options{DefaultTimeout: 30 * time.Second})
//	reg.MustRegister(toolregistry.Tool{
//		Def: llmclient.ToolDef{
//			Name:        "echo",
//			Description: "echo text back",
//			InputSchema: toolregistry.ObjectSchema(
//				toolregistry.Prop{Name: "text", Type: "string", Required: true},
//			),
//		},
//		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
//			var in struct {
//				Text string `json:"text"`
//			}
//			if err := json.Unmarshal(input, &in); err != nil {
//				return "", err
//			}
//			return in.Text, nil
//		},
//	})
//	srv := mcpserver.New(mcpserver.Config{Name: "my-tools", Version: "1.0.0"}, reg, time.Now)
//	if err := srv.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
//		log.Fatal(err)
//	}
//
// Composition: this kit composes foundations/agent/tool-registry (the single
// tool table — Defs feeds tools/list, Dispatch executes tools/call and owns
// policy, timeout, and panic handling), foundations/llmclient (the ToolDef and
// ContentBlock types the registry speaks), and
// foundations/persistence/jsonl-log (the optional call log). The kit itself
// performs no I/O beyond the streams handed to Serve and the optional
// Config.CallLog writer.
//
// Concurrency contract: Serve processes ONE request at a time, in arrival
// order — MCP clients rarely pipeline, and sequential processing keeps tool
// side effects ordered, the same rationale as toolregistry.DispatchAll.
// Response writes are serialized by a per-Serve mutex. The Streamable HTTP
// handler is DIFFERENT: it serves requests concurrently (one per HTTP
// request goroutine) — the dispatch core is stateless, the registry is
// concurrency-safe, the call-log writer is mutex-guarded, and the tool_use
// id counter is atomic, so no request ordering is guaranteed over HTTP. A
// Server is safe for concurrent use: multiple Serve calls on distinct stream
// pairs and any number of HTTP handlers may run in parallel; a single stream
// pair must be given to only one Serve call. The clock injected into New is
// called from the Serve goroutine and from HTTP request goroutines; a
// non-nil injected clock must therefore be safe for concurrent use
// (time.Now, the default, is).
//
// # Protocol scope and roadmap
//
// Transports are stdio (Serve: newline-delimited JSON-RPC 2.0, one message
// per line, as the MCP stdio transport specifies) and Streamable HTTP
// (NewHTTPHandler; see the dedicated section below). Both call the same
// serveOne dispatch core, so JSON-RPC semantics — and the response bytes —
// are identical across transports. The old HTTP+SSE two-connection transport
// is deprecated in the MCP spec and is deliberately not built.
//
// v1 capabilities are tools only: initialize / notifications/initialized,
// ping, tools/list, and tools/call. The prompts and resources capabilities
// are NOT advertised in the initialize response; if a client calls
// prompts/list, resources/list, etc. anyway, the server returns method-not-
// found (-32601), which spec-conforming clients treat as "capability absent".
// Roadmap: prompts and resources would each add a capability entry, a
// list/get method pair, and a foundations-level source to serve them from —
// the dispatch core here does not change.
//
// initialize negotiates the protocol version: when the client requests a
// revision this server knows, that revision is echoed back; otherwise the
// server answers with its latest known revision, per the MCP version-
// negotiation rules.
//
// # Streamable HTTP transport
//
// NewHTTPHandler serves the same tool surface over MCP Streamable HTTP
// (2025-11-25 revision): a single POST endpoint where every JSON-RPC message
// is one HTTP request. The implementation is STATELESS by design — no
// session store, no Mcp-Session-Id header, no initialize-before-use
// enforcement (the stdio side keeps no handshake state either) — which is
// correct for a tools-only server behind a load balancer and matches the
// direction of the 2026 stateless RC. Per request:
//
//   - Origin validation runs first, before any body read (DNS-rebinding
//     protection, a spec MUST): a request bearing an Origin header not in
//     HTTPOptions.AllowedOrigins gets 403 with an empty body, and an EMPTY
//     allowlist rejects all Origin-bearing requests (fail closed). Requests
//     without an Origin header — non-browser clients — always pass.
//   - Non-POST methods get 405. In particular there is no GET SSE stream;
//     405 is the spec's prescribed answer for a server that offers none.
//   - MCP-Protocol-Version handling per the 2025-11-25 spec: a known header
//     value proceeds; an ABSENT header assumes revision 2025-03-26 (the
//     spec's backwards-compatibility SHOULD — behaviorally a no-op here
//     because this tools-only surface is identical across all known
//     revisions); an unknown or malformed value gets 400 Bad Request (a
//     spec MUST).
//   - Body size is bounded by HTTPOptions.MaxBodyBytes (default: the
//     server's MaxMessageBytes) via http.MaxBytesReader; an oversized body
//     is answered HTTP 200 with JSON-RPC -32700 carrying a best-effort id —
//     the stdio oversized-message policy on the HTTP mechanism.
//   - A request yields HTTP 200, Content-Type: application/json, one
//     JSON-RPC response object. A notification yields 202 Accepted with an
//     empty body. The request Accept header is ignored: a JSON-only
//     stateless server always answers application/json and never opens an
//     SSE stream — this implementation's stateless-mode reading of the
//     2025-11-25 Accept requirement.
//
// Cross-process tracing: the handler parses an inbound W3C traceparent
// header (foundations/telemetry/event.ParseTraceparent) and attaches a child
// TraceContext to the request context before dispatch, so telemetry emitted
// by tool handlers carries the caller's RunID across the process boundary.
// An absent or malformed header degrades to "no trace", never an error. The
// stdio transport carries NO trace context — byte streams have no headers.
//
// Non-guarantees, stated plainly: no TLS and no authentication (put a
// reverse proxy or the endpoint-guard sidecar in front for anything
// non-local); no SSE streaming, keepalives, or progress notifications; no
// server-to-client notifications or requests (impossible in stateless mode —
// there is no channel to send them on); no session resumability. A
// client-to-server JSON-RPC *response* POSTed to the endpoint (meaningful
// only when a server issues requests, which this one never does) is
// dispatched like any request-shaped message and answered -32601, identical
// to the stdio behavior, rather than the spec's 202-for-responses — a
// deliberate consequence of keeping dispatch byte-identical across
// transports.
//
// # JSON-RPC 2.0 correctness
//
// Notifications — messages without an id (or with a literal null id) — are
// never answered, including notifications/initialized. Responses carry the
// request id verbatim (string and number ids pass through as
// json.RawMessage). Error objects are {code, message, data?}. JSON-RPC batch
// arrays are rejected with -32600: MCP clients do not batch, and the MCP spec
// removed JSON-RPC batching in the 2025-06-18 revision. Unknown methods
// return -32601, malformed JSON -32700, well-formed JSON that is not an
// object -32600, and invalid params -32602. The server is otherwise lenient:
// it does not validate the "jsonrpc" version field, and it does not enforce
// initialize-before-use ordering (it keeps no handshake state).
//
// # Input bounds and resync (this is an attack surface)
//
// Every line feeds tool execution, so message size is bounded BEFORE parsing:
// Config.MaxMessageBytes (default 4 MiB) caps one newline-delimited message,
// enforced via a bufio.Reader so memory stays bounded regardless of input.
// An oversized message produces a -32700 error carrying a best-effort id —
// recovered from the retained prefix when the id field appears there, null
// otherwise — and the remainder of the line is then discarded. Resync is
// deterministic because the framing is newline-delimited: skipping to the
// next '\n' always lands on a message boundary. The only way resync can fail
// is a transport read error during the skip; in that case Serve fails closed,
// terminating with a wrapped error that names the resync failure. A client
// that never sends another newline keeps the server discarding (bounded
// memory, no responses) until EOF — indistinguishable from a silent client.
//
// tools/call routes EVERY failure mode through the registry's in-band
// contract: unknown tool, policy denial, handler error, handler panic, and
// timeout all surface as an MCP result with isError true (the calling model
// should see and react to them), never as a protocol-level error. Protocol
// errors (-32602) are reserved for structurally invalid params.
//
// # Call log privacy
//
// When Config.CallLog is set, every dispatched request appends one JSONL
// entry: ts, method, tool (tools/call only), duration_ms (measured with the
// injected clock), is_error. Tool ARGUMENTS and RESULTS are NEVER logged by
// default — they carry user data. Config.LogPayloads opts in to recording
// them; treat such logs as SENSITIVE and scope their retention accordingly.
// Pre-dispatch rejections (parse errors, batches, oversized messages) and
// notifications are not logged. Logging failures are deliberately swallowed:
// the call log never disrupts serving.
package mcpserver

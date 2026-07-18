# Design note: MCP Streamable HTTP transport

**Status:** BUILT (2026-07-15, H6 Stage B) — `kits/mcp-server/http.go`
(`NewHTTPHandler` + `HTTPOptions`), over the extracted `serveOne` dispatch
core; cross-transport conformance in `kits/mcp-server/http_test.go`
(`TestStreamableHTTPConformance`). SSE, sessions, and prompts/resources
remain deferred per the table below.  
**Subscribe when (fired):** an MCP server must serve non-local clients
(roadmap #15) — proving consumer: fathom `serve-http`
(`../sextant/cmd/fathom/servehttp.go`), whose hand-rolled pattern this
absorbs  
**Closes (protocol half):** roadmap #21 — W3C trace context propagation

---

## What this is not

The deprecated HTTP+SSE transport (one GET SSE stream + separate POST endpoint,
two connections per session) is explicitly not being built. It was removed in the
2025-11-25 spec revision and is already absent from Claude Code's client. The
old HTTP+SSE doc comment in `kits/mcp-server/doc.go` ("deliberately not built")
stays accurate.

---

## Streamable HTTP: the design direction

The 2025-11-25 / 2026 stateless RC collapses the two-connection SSE model into
a single POST endpoint: every MCP request is an HTTP POST with
`Content-Type: application/json`; the server replies with either
`Content-Type: application/json` (single-result) or
`Content-Type: text/event-stream` (streamed result, used for long tool calls or
server-sent notifications). No persistent session channel; no GET endpoint;
no `mcp-session-id` unless the server opts into it for resumability.

### Handshake

None beyond standard HTTP. The `initialize` RPC is the first POST; the server
responds with its chosen `protocolVersion` in the body, same JSON shape as
stdio. Clients that want session affinity send `mcp-session-id` on subsequent
requests; the kit's first implementation will be **stateless** (no session
store), which is correct for tools-only servers behind a load balancer and
matches the RC's stateless default.

### Transport-agnostic dispatch core

The current `kits/mcp-server` dispatch core is already transport-agnostic.
The functions that need zero changes:

| Symbol | Role |
|--------|------|
| `(*Server).dispatch` | method router — reads `rpcRequest`, returns `(any, *rpcError, callMeta)` |
| `(*Server).handleInitialize` | version negotiation + capabilities |
| `(*Server).handleToolsList` | registry → MCP tool descriptors |
| `(*Server).handleToolsCall` | registry.Dispatch wrapper |
| `(*Server).serveReq` / `logCall` | clock-measured dispatch + call log |
| all `rpc*` / `*Result` types | JSON envelope types |

What needs factoring to add Streamable HTTP:

1. **`Serve(ctx, in io.Reader, out io.Writer) error`** is the stdio-specific
   entry point (bufio framing, newline-delimited). An HTTP handler needs to call
   `s.serveReq` directly — or a thin `serveOne(ctx, rpcRequest)` that packages
   the call-log dance without the I/O loop. Extracting that one method (≈15
   lines) is the only required refactor.

2. **Response encoding**: stdio writes `json.Marshal(resp) + '\n'`;
   HTTP writes the same JSON body with `Content-Type: application/json` (single
   result) or an SSE envelope for streamed results. The JSON types are reused.

3. **Size bounding**: `readLine` / `readBufSize` / `bestEffortID` are
   stdio-only. HTTP gets `http.MaxBytesReader` instead — the same policy, a
   different mechanism.

### As-built findings (2026-07-15)

**MCP-Protocol-Version request header** (verified against the published
2025-11-25 spec, Streamable HTTP §Protocol Version Header, at build time):

- The client **MUST** include `MCP-Protocol-Version: <revision>` on all
  requests subsequent to initialize.
- If the server does not receive the header (and has no other way to know
  the version), it **SHOULD** assume `2025-03-26`. The kit does exactly
  this; behaviorally it is a no-op because the tools-only surface is
  identical across all `knownProtocolVersions`.
- An invalid or unsupported value **MUST** get `400 Bad Request`. The kit
  returns 400 for any value not in `knownProtocolVersions`.

All three behaviors are conformance-tested (header present / absent /
unknown) in `TestStreamableHTTPConformance`, not asserted in prose alone.

**Origin validation** (spec security MUST, DNS-rebinding protection):
implemented — `HTTPOptions.AllowedOrigins`, exact-match; a request bearing
an unlisted `Origin` gets 403 with an empty body before any body read; an
empty allowlist rejects ALL Origin-bearing requests (fail closed); requests
without `Origin` (non-browser clients) pass.

**Accept header**: the request `Accept` header is ignored. A JSON-only
stateless server always answers `application/json` and never opens an SSE
stream; the 2025-11-25 Accept requirement is a client-side MUST (advertise
both content types), and a server that picks the `application/json`
response path satisfies it. This is the stateless-mode reading recorded as
the kit's position.

**Client-to-server JSON-RPC responses**: a *response*-shaped POST body
(meaningful only when the server issues requests, which a stateless server
never does) is dispatched like any request-shaped message and answered
-32601 — identical to stdio, a deliberate consequence of byte-identical
dispatch, in place of the spec's 202-for-responses.

**Spec-revision check (2026-07-15, re-verified):** the MCP 2026-07-28 RC
makes the protocol core stateless and removes the initialize handshake and
`Mcp-Session-Id`; the final ships 2026-07-28. The stateless direction here
is confirmed; `latestProtocolVersion` stays at `2025-11-25` until the final
revision publishes (do not bump on RC rumor).

### Streaming response path

For the tools-only surface, streaming is optional: a tools/call result is
always a single JSON object, so the `application/json` response path suffices.
SSE becomes relevant when (a) a tool call takes long enough that the client
needs a heartbeat keepalive, or (b) server-sent notifications (progress,
`notifications/tools/list_changed`) are added in a later wave. Defer to that
subscribe-when.

---

## W3C trace context (roadmap #21 — protocol half)

The kit carries trace context in-process via `foundations/telemetry/event.TraceContext`
(`RunID`, `SpanID`, `ParentSpanID`) and `ev.WithTrace` / `ev.TraceFrom`.
Cross-process propagation means serialising that into the W3C `traceparent`
header format:

```
traceparent: 00-<trace-id>-<parent-id>-<flags>
```

For the kit's fields: `RunID` maps to the 128-bit trace-id (zero-padded or
hex-encoded); `SpanID` maps to the 64-bit parent-id; flags = `01` (sampled).

**Where this lands on the Streamable HTTP change:**
- Inbound: the HTTP handler reads `traceparent` from the request header and
  attaches it to the context via `ev.WithTrace` before calling `serveReq`.
- Outbound MCP client (from `foundations/agent/mcp-client`): adds `traceparent`
  to every POST from the ambient `ev.TraceContext` in the context.

The stdio transport cannot carry headers by definition; trace propagation is
therefore Streamable-HTTP-only, which is why roadmap #21 lists "subscribe when:
the first multi-process agent topology" — that topology requires HTTP anyway.

**What is deferred:** the full W3C `tracestate` vendor bag, parent-span
auto-increment, and OTLP export (roadmap roadmap #26 / OTel GenAI, Wave 3).
The header read/write is ≈20 lines per side and does not require OTel.

---

## What stays deferred

| Item | Subscribe when |
|------|----------------|
| `mcp-session-id` resumability | first stateful server (long-running tool with reconnect) |
| SSE keepalive / progress notifications | first tool call > 30 s with a human waiting |
| prompts / resources capabilities | first server with content to serve (roadmap #15) |
| Full OTel GenAI semconv mapping | Wave 3 (roadmap #26) |
| HTTPS / OAuth 2.1 resource server | first public-facing endpoint |

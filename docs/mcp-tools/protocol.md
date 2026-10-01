## Protocol contract

What a client that never reads `CLAUDE.md` can learn from the wire alone
(`docs/plans/reach-parity.md` RP-9..RP-13, `docs/vision/25-harness-port.md` §1).
Served by `serve-http` and `serve-mcp` alike; the contract lives in
`internal/mcp/protocol.go` and `outputschema.go`.

- **Protocol versions.** `initialize` echoes the client's `protocolVersion`
  when it is in the supported table — `2025-11-25`, `2025-06-18`, `2025-03-26`,
  `2024-11-05`, newest first — and otherwise answers with the newest supported
  (`negotiateProtocolVersion`; the 2025-11-25 lifecycle rule). All four are
  legacy-era revisions (the `initialize` handshake, one JSON response per
  POST): fathom does not claim `2026-07-28` (per-request `_meta`,
  `server/discover`, SSE streaming), so a modern-only client is told
  `2025-11-25` and decides for itself. Over HTTP the `MCP-Protocol-Version`
  request header is tolerated — never a 400 — and every response carries the
  revision the server negotiates for it. Proven end to end by
  `TestServeHTTPNegotiatesWithKitClient` (`cmd/fathom`), which drives
  `POST /rpc/agent` through agentic-kit's `mcp-client` Streamable-HTTP
  transport — a client fathom did not write — initialize → tools/list →
  tools/call.
- **Caller identity (RT-164, ADR-047 §10.4).** Every request the `fathom`
  binary sends to a daemon host (loopback, `localhost`, or `FATHOM_HTTP`'s
  host — never any other) carries `X-Fathom-Caller:
  pid=<n>;ppid=<n>;origin=<channel>;session=<id>;test=0|1`. `test=1` when the
  sender is a test binary or runs with `FATHOM_TEST=1` (inherited by a
  process a test execs). `origin` and `session` keep `[A-Za-z0-9._:/-]`, map
  anything else to `_` and are capped at 128 bytes, so a value cannot add or
  override a field; a parser ignores unknown fields
  (`client.ParseCallerHeader`). `X-Fathom-Origin` is unchanged.
- **Abandoned by the governor (RT-172, ADR-047 §2.1).** When the tenancy
  ladder's refuse rung cancels an in-flight tree or fleet call, the call
  answers a JSON-RPC error (`-32603`) whose message is the cause and whose
  `data` is `{"outcome": "abandoned-by-governor"}` — never whatever the
  handler computed under the cancelled context. The call-log row carries
  `outcome: "abandoned-by-governor"`; `/metrics` counts
  `fathom_governor_abandoned_total`.
- **Refused before dispatch by the lifecycle modes (RT-39..RT-44, ADR-047
  §2.3, §3.4).** HTTP 503, `Retry-After`, JSON-RPC `-32050` `{message:
  "fathom busy", data: {reason, class, level, retry_after_ms}}`: `tree` and
  `fleet` in safe mode (`fathom safe mode since 21:07: … — run fathom
  resources`); `tree`, `fleet` and `ingest` while draining after SIGTERM;
  `ingest` in store distress (SQLITE_FULL/IOERR from a read or a write, a
  low store volume); a second concurrent heavy call during the 120 s
  warm-up (under `admission = "observe"` as under `"enforce"`; only `"off"`
  skips it); and every call while the daemon waits for its store (`store
  unavailable: operation not permitted (EPERM, tcc-wedge) — …`,
  `retry_after_ms: 30000`). `point` and `index` are served in safe mode. A
  `batch` is gated by each of its sub-calls' classes (a batch carrying
  `list_services` is refused in safe mode) and its class is its heaviest
  sub-call's.
- **Admission (RT-53..RT-65, ADR-047 §3; "Cost classes" below).** Under
  `[tenancy].admission = "enforce"` a refused call answers HTTP 503 with
  `Retry-After` and JSON-RPC error `-32050`, message `fathom busy: <reason>`,
  `data: {reason, class, level, retry_after_ms, in_flight, outcome:
  "refused"}` (a `batch` sub-call's refusal is that entry's `error`, the batch
  itself answers 200); the 503 carries `X-Fathom-Busy-Scope: class` (slots,
  deadline, caller gone) or `daemon` (refuse rung, safe mode), and hooks back
  off daemon-wide only on `daemon`. A caller already gone is refused
  `caller_gone` before its handler runs. A call whose context ended — its client left, or its
  class deadline or `X-Fathom-Deadline` passed — answers `-32603` with
  `data: {outcome: "abandoned", cause}`, never what the handler computed. A
  call whose producer the meter stopped at the class's per-call byte ceiling
  (enforce only) carries `budget: {partial: true, elided, bytes_read,
  reason}` in its envelope or its `batch` entry, `elided` a lower bound. The managed
  daemon answers a test caller (`X-Fathom-Caller … test=1`) 409 `{reason:
  "test binary reached the operator daemon"}`. Call-log rows are
  `schema_version` 3: `outcome` on every row (`ok`, `partial`, `refused`,
  `abandoned`, `abandoned-by-governor`, `error`), `class`, `queued_ms`,
  `bytes_read` and `caller {pid, ppid, session, test}`.
- **Descriptions (RP-9, diet O101).** Every agent-surface description
  carries a "Reach for it when …" sentence and a `Required: a, b.` sentence
  naming exactly the arguments its own `inputSchema` marks required —
  per-tool truth, not a blanket rule (ADR-006 as amended: the concern is
  the engine's parameter, not every tool's).
  `TestAgentSurfaceDescriptionsNameRequiredArgs` iterates the agent-surface
  allowlist. O101 (the wave-diet brief) caps every description at 260
  bytes and every `inputSchema` property description at 120 bytes, flat —
  no per-tool exception (`surface_description_caps_test.go`); alias/history
  prose was dropped from argument descriptions (the aliases themselves are
  unchanged) and moved to this document where it was not already here.
- **Annotations (RP-10).** Every agent-surface tool serves MCP
  `ToolAnnotations`: `readOnlyHint`, `destructiveHint`, `idempotentHint`,
  `openWorldHint`. Sixteen are read-only; `ingest` (re-indexes: not read-only,
  idempotent, never destructive) and `obligations` (its `action:"refute"|
  "corroborate"` path writes a claim's status: not read-only, idempotent, not
  destructive) are the two writers. Nothing on the surface is open-world. The
  long tail on bare `/rpc` serves no annotations rather than guessed ones.
- **Output schemas (RP-11, off by default since O101).** Every
  agent-surface tool still HAS a generated `outputSchema` (`outputSchemaFor`,
  `internal/mcp/outputschema.go`'s `agentResultTypes` registry; root
  `type: object`, additional properties permitted because fields are
  additive by contract) and `tools/call` still returns the result as
  `structuredContent` beside the text block, unconditionally. What changed
  (O101, the wave-diet brief, bench evidence: the wired arm carried a
  median 15,219 cache-read tokens/turn vs 5,165 for grep, and `outputSchema`
  was 18,827 of the 48,341-byte `tools/list` payload — 39%) is whether
  `outputSchema` rides on `tools/list` itself: **off by default**, so the
  schema tax every connected session pays before its first call drops by
  that much. A client that validates `structuredContent` against the
  advertised schema asks for it back with `?contracts=true` on the HTTP
  surface (`POST /rpc/agent?contracts=true`, `POST /rpc?contracts=true`, …;
  `internal/mcp/surface.go`'s `WithContracts`, read from the query string in
  `cmd/fathom/servehttp.go`) — a stdio session gets the diet unconditionally
  (no query string to opt in from). `TestOutputSchemasMatchLiveResults`
  requests `WithContracts(ctx, true)` to get schemas back onto the wire, then
  validates every live result against the schema it advertised — the
  generated schemas themselves, and the fact that a request can still ask
  for them, are unchanged; only the served default moved.
- **Order and cost (RP-12).** `tools/list` is sorted by name, byte-stable
  across calls, so a client that pins per-tool hashes (the kit client does)
  or diffs the list sees no spurious change. `capabilities` reports
  `schema_bytes` — the serialised size of the request surface's `tools/list`,
  the context every session pays before its first call (TD-2's diet is a
  measured number, not a hunch).
- **Answer-first `summary` (wave L1).** Two home-bench readings
  (`docs/bench/readings/vs-pilot-6-second-reading.md`, `vs-pilot-5`) found the
  tools handing the agent the right evidence and the model reasoning past
  it: `find_callers(RenderMarkdown, depth=2)` listed the direct caller
  first, and the final answer named a depth-2 caller three hops out; a
  `search` hit landed on the right line, and the answer cited a different
  document. The fix is presentation, not more data. `find_callers`,
  `references`, `blast_radius` (every `kind`, including the retired
  `rename_impact`/`safe_delete_check` tools' own `kind:"rename"`/`"delete"`
  answers — SR-24), `test_impact`, `localize`, `search`, and `obligations`
  each lead their
  JSON with a `summary` string (`internal/mcp/summary.go`'s
  `summaryEnvelope` splices it in ahead of every other key — plain
  `encoding/json` on a `map[string]any` would otherwise sort it
  alphabetically wherever `"s"` falls) — one sentence, ≤ 200 chars, naming
  the direct answer in the question's own terms (e.g. `find_callers`: "3
  direct callers of X: a (file:12), b (file:9), c (file:3); 5 more at
  depth 2"). `summary` is built from the SAME sorted, post-budget data the
  rest of the body carries — never from a wider pre-elision set — so it can
  never claim a row the body does not actually return, and it rides outside
  every truncating tool's item-slicing entirely (it is never itself cut).
  For the callers/references family specifically: `depth` on each row and
  the `by_depth` summary mean a direct (depth-1) result and a transitive
  (depth 2+) one are never conflated — `summary` only ever names depth-1
  rows as "direct".

---

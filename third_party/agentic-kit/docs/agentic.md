# The agentic stack — how the pieces compose into a working agent

Audience: you, starting a new agent project on this kit, and the AI agents
helping you. Every package named here exists and every symbol is real; when a
doc comment says more, the package doc is the authority
(`foundations/llmclient/doc.go` is the contract ADR).

---

## 1. The 60-second picture

```
              user input
                  │
                  ▼
┌────────────────────────────────────────────────────────────────┐
│ kits/agent-harness — Run(ctx, input) (RunResult, error)        │
│                                                                │
│  budget.Charge ─► model call ─► dispatch tools ─► screen       │
│  (fail-closed,        │            results ─► loop until stop  │
│   optional)           │           (injection-detector, opt.)   │
│                                                                │
│  foundations/agent/conversation: transcript invariants —       │
│  role alternation, exact tool_use/tool_result pairing.         │
│  Malformed transcripts are unrepresentable, not "validated".   │
└─────────┬─────────────────────────────┬────────────────────────┘
          │ llmclient.Request           │ tool_use blocks
          ▼                             ▼
┌──────────────────────────┐  ┌─────────────────────────────────┐
│ LLM middleware stack     │  │ foundations/agent/tool-registry │
│   WithCost               │  │   Defs()  → Request.Tools       │
│    └ WithTelemetry       │  │   DispatchAll → tool_result     │
│       └ WithRetry        │  │   blocks (policy, timeout,      │
│          └ provider:     │  │   panic recovery — all in-band, │
│   anthropic / MockClient │  │   never a Go error)             │
│   / replay cassette      │  └──────────────┬──────────────────┘
└──────────────────────────┘                 ▼
                                   tool handlers: plain Go funcs;
                                   subprocess tools go through
                                   agent/bounded-exec (NOT a sandbox)

cross-cutting, injected, never hard-wired into the loop:
  telemetry/event + sink   one RunID correlates harness + llm_call events
  cost/cost-tracker        usage → USD (price table is config, no defaults)
  governance/budget-guard  hard pre-call cap, fail-closed
```

The harness owns the loop; everything else is a seam. Required deps: an
`llmclient.LLMClient` and a `toolregistry.Registry`. Every other dep
(`Budget`, `Detector`, `Sink`, `Journal`) is optional — nil turns the feature
off. See `kits/agent-harness/doc.go` for the full seam inventory.

## 2. The LLM contract in 10 lines

1. One interface: `llmclient.LLMClient` — `Complete(ctx, Request)` and
   `Stream(ctx, Request)`. Everything that calls a model goes through it.
2. `Request.Messages []Message` is the conversation; `Request.Prompt` is the
   single-turn shorthand (used only when `Messages` is empty).
3. A `Message` is a `Role` plus ordered `[]ContentBlock` — you need blocks the
   moment tools enter: text, tool_use, tool_result are block types, not fields.
4. `ToolDef{Name, Description, InputSchema}` declares a tool on
   `Request.Tools`; the model answers with `BlockToolUse` blocks
   (`Response.ToolCalls()`), you reply with `BlockToolResult` blocks.
5. `StopReason` is an OPEN string set — never switch exhaustively. Know
   `StopEndTurn`, `StopToolUse`, `StopMaxTokens`, `StopRefusal`; treat
   anything else as "stopped for a reason this build does not recognise".
6. `StopToolUse` is a SUCCESS, not an error: never retried, never escalated.
7. `BlockUnknown` carries provider block JSON verbatim in `ContentBlock.Raw`
   and `Response.AsMessage()` echoes it back untouched. This is what keeps
   thinking models working: thinking blocks survive multi-turn round-trips
   even though this package does not model them.
8. `Usage.CacheReadInputTokens` is the ground truth for prompt caching;
   `CacheHint` is a best-effort request, silently a no-op below the model's
   minimum cacheable prefix.
9. `WithRetry` retries rate-limit/server errors only, honors `Retry-After`.
10. `Ladder` escalates models only on an explicit caller `Signal`;
    `Rung.Apply` fills `Request.Model` when empty.

## 3. Building an agent

Copy `examples/agent-cli/` — it is the hello-world agent and the template,
deliberately small. Its `README.md` documents the copy-as-new-project steps;
`docs/adopting.md` covers the module wiring. The structure to keep:

- **`newRegistry`** — one `toolregistry.Tool{Def, Handler}` per tool:
  `ObjectSchema(Prop{...})` for the input schema, a handler whose errors
  become `is_error` tool_results. Its filesystem tools are path-confined via
  `os.Root` — the pattern for any tool that touches disk.
- **the middleware stack** — `WithCost(WithTelemetry(WithRetry(base)))`,
  retry INNERMOST so cost/telemetry see one logical call (the rationale is in
  `foundations/llmclient/instrument/doc.go`).
- **`agentharness.New(Config, Deps)`** then `Run(ctx, prompt)` — `Config` sets
  `MaxTurns`, `System`, `Model`, `EstimatedUSDPerCall`; `Deps` injects the
  wrapped client and the registry, plus optional governance seams.
- **a testable `run()`** — flags in, exit code out, so the whole binary is
  exercised hermetically in `main_test.go`.
- **three modes behind one flag** — `mock` (scripted `MockClient`), `replay`
  (committed cassette, hash-matched), `live` (`anthropic.New`, needs
  `ANTHROPIC_API_KEY`). Same composition in every mode.

Stop handling: `RunResult.Stop` is a value, not an error —
`completed | max_turns | budget | refusal | max_tokens | canceled`. `Run`
returns a Go error only for infrastructure failures (and `Stop` is then
`StopError` with the transcript preserved in `RunResult.Messages`).

## 4. Exposing tools over MCP

`kits/mcp-server` turns the same registry into a stdio MCP server — tools are
defined once and serve both surfaces (the agent loop's `Request.Tools` and an
MCP client's `tools/list`):

```go
srv := mcpserver.New(mcpserver.Config{Name: "my-tools", Version: "1.0.0"}, reg, time.Now)
err := srv.Serve(ctx, os.Stdin, os.Stdout)
```

Newline-delimited JSON-RPC 2.0, version-negotiated `initialize`,
`tools/list` + `tools/call`; tools-only v1 (prompts/resources are a documented
roadmap, answered with `-32601` until then). Message size is bounded before
parsing (`Config.MaxMessageBytes`, default 4 MiB) with deterministic resync
that fails closed on ambiguous transport state. Every tool failure mode is
in-band (`isError: true`) via the registry's dispatch contract. The optional
JSONL call log records metadata only — arguments and results are excluded
unless you opt in with `Config.LogPayloads` (then treat the log as sensitive).

## 5. Testing strategy — the ladder

Cheapest rung that catches your bug wins; the example walks all four.

1. **`MockClient`** — script the model's side (`MockEntryOK`,
   `MockEntryToolUse`, `MockEntryRateLimit`, ...); tools genuinely execute.
   The default for every loop/governance test. No key, no network.
2. **Replay cassettes** (`foundations/llmclient/replay`) — record once
   (`NewRecordingClient`), replay forever (`NewReplayClient` +
   `MatchByRequestHash()` so prompt/schema drift fails loudly instead of
   lying). Scrubbing is ON by default but is best-effort secret-shape
   redaction, not a guarantee — review every cassette before committing
   (gaps are listed in `foundations/llmclient/replay/doc.go`).
3. **`llm-test` conformance** — only when writing a NEW client:
   `llmtest.RunConformance(t, Harness{...}, Capabilities{...})` is the v2
   contract floor (streaming lifecycle, cancellation, round-trips). The
   anthropic client passes it against scripted `httptest` servers.
4. **`task smoke-live`** — the one real-API check; prints `SKIP` without
   `ANTHROPIC_API_KEY`. Never part of the gate.

## 6. Cost & observability

- `llminstrument.WithTelemetry` emits exactly one `llm_call` event
  (`event.KindLLMCall`) per LOGICAL call — retries surface as the `attempts`
  attribute, not extra events, so dashboards keep a stable shape during retry
  storms. Sink failures are swallowed and counted; telemetry never breaks the
  call path.
- `llminstrument.WithCost` records cache-aware usage into a
  `costtracker.Tracker` against a caller-supplied `PriceTable`. **No prices
  are shipped** — a stale embedded price table is worse than none.
  `Tracker.Report()` gives the per-label/per-model breakdown.
- Trace correlation: the harness mints an `event.TraceContext` per run (or
  adopts one from ctx via `event.TraceFrom`), and `event.Apply` stamps
  `trace.run_id` on every event — so harness step events
  (`Source: "kits/agent-harness"`) and the instrument's `llm_call` events
  join on one RunID.
- For scrape-style metrics, the pattern lives in `cmd/endpoint-guard`:
  `metrics.NewRegistry()`, `registry.Counter(name, labelPairs...).Inc()` at
  the call sites, and a `GET /metrics` handler writing `registry.Expose()`
  (Prometheus text format, stdlib only).

## 7. Governance hooks in the loop

- **Budget, fail-closed**: with `Deps.Budget` set, the harness charges
  `Config.EstimatedUSDPerCall` BEFORE every model call; a denial ends the run
  with `StopBudget`. The charge is an honest estimate (the call hasn't
  happened yet) — pair it with `WithCost` for usage-based actuals. One shared
  `budgetguard.Guard` across runs shares the budget; that is the point.
- **Injection scanning of tool results**: with `Deps.Detector` set, every
  tool result is scanned before re-entering the conversation.
  `InjectionBlock` (default) replaces it with an `is_error` tool_result — the
  model sees the refusal and the raw payload provably never enters the
  transcript; `InjectionAnnotate` keeps the payload and appends a warning
  trailer. Detection is pattern-based and best-effort, which is why blocking
  is the default.
- **Subprocess tools**: route them through `foundations/agent/bounded-exec`,
  which bounds wall-clock time, output bytes, and environment — and is
  **NOT A SANDBOX**. The child runs with the host's full ambient authority:
  filesystem, network, memory/CPU, and privileges are all unrestricted;
  process-group cleanup is best-effort and unix-only. Real isolation needs OS
  primitives (containers, seccomp, jails) layered around it.

## 8. Deliberately not here yet

- **Cross-session memory** — every `Run` builds a fresh conversation by
  design; cross-run state lives in the injected deps. A persistent agent
  memory store waits for a real consumer to shape it.
- **Eval-runner** — replay cassettes + the conformance suite cover regression
  today; a scored task-eval harness is deferred until there are tasks worth
  scoring.
- **OpenAI-compatible chat client** — anthropic is the only real provider so
  far; `llm-test` is the contract any second client must pass, so adding one
  is mechanical when needed (embeddings already speak OpenAI-compatible via
  `foundations/embedding/http-embedder`).

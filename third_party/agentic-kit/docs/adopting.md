# Adopting agentic-kit

How to build a new project on `github.com/geoffmilleraz/agentic-kit` (Go 1.26, stdlib-only).
The copyable starting point is [`examples/agent-cli`](../examples/agent-cli/README.md); the
first real consumer is [sextant](https://github.com/geoffmilleraz/sextant), whose wiring is
shown verbatim below.

## 1. Wiring the dependency

### Local sibling development (sextant's pattern)

Check out your project next to the kit and wire **both** a `go.work` and a
`require` + `replace` pair. These are sextant's actual lines:

```
# go.work
go 1.26

use (
	.
	../agentic-kit
)
```

```
# go.mod
require github.com/geoffmilleraz/agentic-kit v0.0.0

replace github.com/geoffmilleraz/agentic-kit => ../agentic-kit
```

Why both: `go.work` serves your editor and local builds, but workspaces are
ignored in CI and under `GOWORK=off` — the `require v0.0.0` + `replace` pair is
what keeps those builds and `go mod tidy` working. Skip either half and one of
the two worlds breaks.

### Replace + full vendoring, no `go.work` (throughline's pattern)

The second production consumer keeps the `require` + `replace` pair, runs
`go mod vendor`, and commits `vendor/` — with **no** `go.work`:

```
# go.mod
require github.com/geoffmilleraz/agentic-kit v0.2.0

replace github.com/geoffmilleraz/agentic-kit => ../agentic-kit
```

What it buys: hermetic, reproducible builds — CI and fresh clones build from
the committed `vendor/` alone, with no sibling checkout and no `GOPRIVATE`
setup. What it costs, honestly:

- **Vendor drift is silent.** The default build uses the vendored kit
  snapshot, not the sibling — kit edits change nothing until you re-run
  `go mod vendor`. Make re-vendoring a habit after every kit pull, or add a
  freshness check: building with `-mod=readonly` bypasses `vendor/` and
  resolves the `replace` to the sibling at HEAD (exactly what the kit's
  `task throughline-check` does).
- **Keep the `require` version honest.** Bump it to the real tag that matches
  what you vendored, so the declaration is correct if the `replace` is ever
  dropped.

### Tagged consumption

Drop the `replace`, require a real tag (`v0.1.0` and `v0.2.0` exist):

```
require github.com/geoffmilleraz/agentic-kit v0.2.0
```

The repo is **private**, so `go get` needs:

```sh
export GOPRIVATE=github.com/geoffmilleraz/*
git config --global url."git@github.com:".insteadOf "https://github.com/"   # if you auth via SSH
```

## 2. Starter menu by project type

Every sketch below compiles against the real packages — symbols verified, not aspirational.

### Agent CLI / tool

Import `kits/agent-harness` + `foundations/agent/tool-registry` +
`foundations/llmclient` (+ `/anthropic` for live, `/replay` for tests,
`/instrument` for cost + telemetry). **Copy `examples/agent-cli` — it is this
exact composition with three working tools, mock/replay/live modes, and tests.**

```go
client, err := anthropic.New(anthropic.Config{APIKey: key, DefaultModel: "claude-opus-4-8"})

reg := toolregistry.New(toolregistry.Options{DefaultTimeout: 10 * time.Second})
reg.MustRegister(toolregistry.Tool{
	Def: llmclient.ToolDef{
		Name:        "word_count",
		Description: "Count whitespace-separated words in text.",
		InputSchema: toolregistry.ObjectSchema(
			toolregistry.Prop{Name: "text", Type: "string", Required: true}),
	},
	Handler: func(ctx context.Context, input json.RawMessage) (string, error) { /* ... */ },
})

h, err := agentharness.New(
	agentharness.Config{MaxTurns: 8, System: systemPrompt, Model: "claude-opus-4-8", MaxTokensPerCall: 1024},
	agentharness.Deps{Client: client, Registry: reg, Now: time.Now})
result, err := h.Run(ctx, "How many words are in testdata/sample.txt?")
```

The harness drives `foundations/agent/conversation` internally, so transcript
invariants hold by construction. Optional `Deps` seams: `Sink` (telemetry),
`Budget` (spend caps), `Detector` (injection screening), `Journal` (run trace).
For production, wrap the client `WithCost(WithTelemetry(WithRetry(base)))` —
see `foundations/llmclient/instrument`'s doc for why that order.

### MCP server

`foundations/agent/tool-registry` + `kits/mcp-server`. Zero LLM code: a
registry becomes a stdio MCP server for Claude Code / Claude Desktop.

```go
reg := toolregistry.New(toolregistry.Options{DefaultTimeout: 30 * time.Second})
reg.MustRegister(toolregistry.Tool{ /* Def + Handler, exactly as above */ })

srv := mcpserver.New(mcpserver.Config{Name: "my-tools", Version: "1.0.0"}, reg, time.Now)
if err := srv.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
	log.Fatal(err)
}
```

Tools written once serve both an agent loop and MCP — the registry is the
single tool table. `Config.CallLog` adds a JSONL call log (method, tool,
duration — payloads only if you opt in via `LogPayloads`, which is sensitive).

### Governed public endpoint

`kits/endpoint-guard` is the layered gauntlet (breaker → identity → strikes →
rate limits → optional proof-of-work → topic gate → answer cache → paid LLM
call). **`cmd/endpoint-guard` is the reference deployment** — config file,
fail-closed startup checks, `/ask` `/healthz` `/metrics`, state snapshots.
Start from it rather than hand-wiring:

```go
guard, err := endpointguard.New(cfg, endpointguard.Deps{
	Answerer: client,           // the paid llmclient.LLMClient call
	Judge:    client,           // topic-gate judge (required if Embedder is nil)
	Embedder: emb,              // optional: similarity gate + near-dup cache
	Detector: detector,         // security/injection-detector — required
	Now:      time.Now,
	RandHex:  randHex,          // crypto/rand hex source — required
})
mux.Handle("POST /ask", guard.HTTPHandler())
```

`HTTPHandler` self-caps request bodies (`cfg.MaxBodyBytes`, default 64 KiB →
templated 413), so hand-wired deployments get the body cap for free.
`cfg` covers rate limits, budget, strikes, topic exemplars, cache, PoW — copy
`cmd/endpoint-guard/config.example.json`. Governance rationale: [governance.md](governance.md).

### RAG / investigation

`foundations/embedding/http-embedder` (any OpenAI-compatible backend — OpenAI,
Ollama, vLLM) + `foundations/embedding/vector-store` + `foundations/retrieval/*`.

```go
emb, err := httpembedder.New(httpembedder.Config{
	BaseURL: "http://localhost:11434/v1", Model: "nomic-embed-text"})

idx := vectorstore.NewIndexer(emb, vectorstore.NewMemoryVectorStore())
err = idx.IndexTexts(ctx, vectorstore.Document{
	ID: "doc-1", Text: chunk, Meta: map[string]string{"source": path}})

hits, err := idx.SearchText(ctx, "what does the contract say about retention?", 5)
```

In tests, swap in `embeddingclient.NewMockEmbedder(dim)` — deterministic, no
network. For claims-grade work (graded evidence, source linking, conflict
detection), build on the evidence spine: read
[`foundations/evidence/README.md`](../foundations/evidence/README.md) first —
it documents which `Span` type is canonical.

## 3. Testing without a key or network

- **`llmclient.MockClient`** scripts the model side; your tools genuinely
  execute. Helpers: `MockEntryOK`, `MockEntryToolUse`, `MockEntryWithResponse`,
  `MockEntryErr`, `MockEntryRateLimit`. This is the shape of every hermetic
  agent test in the kit.
- **`foundations/llmclient/llm-test`** (`llmtest.RunConformance`) is the
  exported conformance suite — if you write your own `LLMClient` adapter, run
  it against the same contract checks the in-tree clients pass.
- **Replay cassettes** (`foundations/llmclient/replay`): record a live session
  once with `NewRecordingClient`, commit the cassette, replay in CI with
  `NewReplayClient(entries, llmreplay.MatchByRequestHash())` — any drift in
  prompt, schemas, or tool results fails loudly instead of lying. Scrubbing is
  on by default (`DefaultScrubber`), **but cassettes are still sensitive
  artifacts — review before committing or sharing**. `examples/agent-cli`
  shows the full pattern, including a test that fails when the committed
  cassette goes stale.

## 4. Conventions you must know

- **Hyphenated dirs, collapsed package names.** `agent/tool-registry` is
  `package toolregistry`. Write the alias explicitly so readers never guess:
  `toolregistry "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry"`.
- **stdlib-only, forever.** The kit's `go.mod` has no `require` block. What
  that buys you: the kit can never cause a dependency conflict in your project.
- **CUE owns shape, Go owns behavior.** Each package's `schema.cue` is the
  data contract; Go mirrors it. New to CUE? Read [why-cue.md](why-cue.md).
- **Injected clocks everywhere.** Library code takes `now func() time.Time`;
  wire real time by passing `time.Now` (as the sketches above do) and a fixed
  clock in tests.
- **Go 1.26 minimum** — deliberate: a single-owner ecosystem tracks the latest
  toolchain instead of carrying compatibility shims.
- **All five tiers are importable.** `foundations/`, `kits/`, `runtimes/`,
  `surfaces/`, and `toolchains/` are all ordinary library packages of the one
  module — the tier expresses a package's *role* in the layering, not its
  importability, so import (and vendor) from any of them rather than forking
  types locally.

## 5. Stability policy

- **v0.x: breaking changes are allowed** — but always called out in the
  CHANGELOG and the commit message, never silent.
- Tags are cut only at green gates. **`task validate` (fmt-check + vet +
  cue-vet + `go test -race ./...`) is the contract** — if it's green at a tag,
  that tag is safe to build on.
- `task downstream-check` builds and tests both downstream consumers (sextant
  and throughline) against kit HEAD, so the patterns in this doc are exercised
  continuously, not just documented.

## 6. Data handling, honestly

- **Append-only stores have no deletion or retention story.** That includes
  provenance journals and event stores, strike ledgers, replay cassettes, MCP
  call logs, and JSONL telemetry logs. Do **not** persist regulated PII
  through them unless you add an external retention/erasure layer — the kit
  deliberately does not pretend to provide one.
- **DLP scrubbing is best-effort shape redaction**, not a guarantee: the
  replay scrubber redacts secret-*shaped* text, can miss secrets split across
  stream chunks, and does not scan tool inputs or tool definitions at all.
  Treat anything recorded as sensitive until reviewed.
- Related honest framings that carry over to your project: bounded-exec is
  **not** a sandbox, and the untrusted-content wrapper is defense-in-depth,
  not a security boundary.

# Throughline → agentic-kit v0.2.0+ adoption brief

This file is a **paste-able prompt** for a Claude Code session running in
`/Volumes/m2ext/code/geoffmilleraz/throughline`. It was produced by a kit-side
audit of throughline on 2026-06-10 (every claim below was verified against
both codebases on that date — re-verify file paths before editing; they may
have moved). The kit-side blockers it depends on are **already shipped**.

---

## The prompt (copy everything below this line)

You are working in the throughline repo. The sibling library
`../agentic-kit` shipped v0.2.0+ ("the agentic uplift"): a real Anthropic
client, an agentic LLM contract (multi-turn/tool-use/streaming), record/replay
testing, cost/telemetry/egress middleware, and hardened security packages.
A kit-side audit of this repo found that throughline's evidence/provenance
spine usage (15 foundations, 155 imports) is healthy and needs **no changes**,
but its hand-rolled LLM substrate (~1,900 lines) is now superseded, and several
kit primitives were built specifically from throughline's own proven patterns.
Read `../agentic-kit/CHANGELOG.md`, `../agentic-kit/docs/agentic.md`, and
`../agentic-kit/docs/practices.md` first.

Work through the phases IN ORDER; each phase ends with the full throughline
gate green (`task validate` or its equivalent) plus `go build -mod=readonly
./... && go test -mod=readonly ./...` (which resolves the replace directive to
kit HEAD, bypassing vendor/). Re-run `go mod vendor` after each phase since
this repo mandates vendored builds. Commit per phase.

**Phase 0 — hygiene (15 min).**
Bump `go.mod`: `require github.com/geoffmilleraz/agentic-kit v0.2.0` (the
require currently says v0.1.0, which is dishonest — the replace directive
masks it, but any build without the sibling checkout would silently resolve
pre-uplift code). Re-vendor. Nothing should change behaviorally.

**Phase 1 — swap the LLM substrate to kit llmclient v2.**
Replace `adapters/llmclient/anthropic.go` (~191 lines of hand-rolled Messages
API HTTP) with a thin adapter over `foundations/llmclient/anthropic`:
- OAuth/bearer is now supported kit-side: `anthropic.Config{AuthScheme:
  anthropic.AuthSchemeBearer, BetaHeaders: []string{"oauth-2025-04-20"}}` for
  Claude Code OAuth credentials, `AuthSchemeAPIKey` (default) for API keys.
  Keep `adapters/claudeauth` (credential FILE resolution is deliberately
  yours, not the kit's) — it now just feeds the kit Config.
- Replace your zero-token health probe (`adapters/llmclient/health_probe.go`)
  with the kit's `Client.Ping(ctx)` (GET /v1/models, typed errors).
- Replace the `RewireInner`/`Available()` hot-swap logic in `app/llm/client.go`
  with `llmclient.NewSwappable` (atomic `Swap`, `Available()`, typed
  non-retryable `UnavailableError` when unconfigured).
- Keep `app/llm`'s Completer port (hexagonal boundary is yours); implement it
  over `llmclient.LLMClient`. Map your Message strings to
  `llmclient.TextBlock` content. archtest forbids net/http in app/ — the kit
  client stays behind adapters/, only llmclient types cross into app/.
- Wrap per the kit's documented composition:
  `WithCost(WithTelemetry(WithEgress(WithRetry(client))))`.
- Replace `adapters/llmclient/fake.go` with `llmclient.NewMockClient` (and its
  `MockEntry*` helpers); adopt `foundations/llmclient/replay` cassettes for
  chat-service tests (scrubbing is ON by default; cassettes are sensitive
  artifacts — keep them out of git unless reviewed).
You gain for free: SSE **streaming** (`Stream` → `StreamChunk`, the upgrade
path for streamed Expert Chat), typed errors with Retry-After-aware retries,
tool use for later milestones, prompt-cache hints, key-never-in-errors
hardening. Delete what the kit now owns; keep the port.

**Phase 2 — egress gate becomes kit middleware.**
Your `app/llm/client.go` GatedClient (tokenize-before-egress) was generalized
INTO the kit as `foundations/llmclient/egress` (`llmegress.WithEgress` +
`GateFromVault(vault, namespace, author, rules...)`). Rebase your gate on it:
your per-case namespace and PII rules stay yours (caller data); the fail-closed
plumbing, deep-copy, and once-per-logical-call-under-retry guarantees are now
kit-tested. Keep attaching ai-provenance in your wrapper (the kit deliberately
leaves provenance attachment to the consumer).

**Phase 3 — real prompt provenance.**
`app/llm/client.go` hashes prompts by hand and hard-codes
PromptTemplateVersion `"1"`; `app/elicitation/generators.go` fakes hashes.
Adopt `foundations/prompt/prompt-template`: define versioned `Template`s for
the chat system prompt and elicitation generators; `Render(...)` returns real
`PromptHash`/`ParamsHash`; `Rendered.ApplyTo(*aiprovenance.AIAuthor)` is a
one-liner. This makes your Q11 "reproducible AI findings" invariant actually
true — today a prompt edit silently breaks reproduction.

**Phase 4 — cost ceilings (your M19.5, zero new design).**
Your design doc 32 M19.5 names the kit packages already: wire
`llminstrument.WithCost` + `costtracker.PriceTable` (label = case ID — your
per-case client scoping gives this for free) and `governance/budget-guard`
(`Charge` fail-closed = "halts at the configured budget, not a bill").
Tokens are already captured; spend is currently invisible.

**Phase 5 — injection defense on hostile evidence (SECURITY, do not skip).**
`app/chat/service.go` buildSystemPrompt embeds opposing-party evidence
(emails, SMS) directly into the system prompt with ZERO injection screening —
the canonical hostile-injection vector for a legal-adversarial product. Screen
retrieved context with `security/injection-detector` (v0.2.0: 22/22 recall on
the attack corpus) and fence it with `security/untrusted-content-wrapper`
(per-wrap nonce fences; defense-in-depth, not a boundary — treat flags as
review signals). Plan the same screen for the M18.1 relevance pass.

**Phase 6 — citation round-trip.**
Replace `parseCitations` in `app/chat/service.go` with the kit's
`evidencecitation.NewParser("span", "authority")` — `Parse` (byte offsets),
`Validate(citations, known)` as the anti-fabrication gate (reject model-invented
span IDs — you currently have no such check), `Strip`/`Replace` for display.

**Later, when their milestones open (no action now):**
- M18.1 gated-LLM analysis pass → `foundations/agent/conversation` +
  `kits/agent-harness` (budget + injection screening built in).
- `surfaces/mcp` → `kits/mcp-server` + `foundations/agent/tool-registry`
  (write only the tool handlers; transport/dispatch/logging are kit-tested).
- Semantic search → `embedding/http-embedder` (OpenAI-compatible; works with
  local Ollama since Anthropic has no embeddings API) + `embedding/vector-store`
  port; your sqlite-vec adapter implements the port (a kit-side conformance
  suite for durable backends is roadmap — you are its likely subscriber, see
  `../agentic-kit/docs/roadmap.md` §22).

**Do NOT migrate:** the evidence/provenance spine usage (already current),
OCR/whisper/parsing adapters, SQLite persistence, claudeauth file reading,
the legal pack, editions, the web surface — all deliberately yours. One
improvement available: your `adapters/ocr` and `adapters/whispercpp` shell out
via raw `exec.CommandContext`; `foundations/agent/bounded-exec` adds output
caps, hard deadlines, and process-group cleanup (it is NOT a sandbox — read
its non-guarantees).

Definition of done per phase: throughline gate green, `-mod=readonly` build+test
green, vendor re-synced, deleted code actually deleted (no commented-out
corpses), and a one-line CHANGELOG/docs note in throughline recording the
adoption. If a kit API doesn't fit cleanly, do NOT work around it silently —
record the friction; it is kit-feedback gold (the kit absorbs proven patterns;
that's how `llmegress`, bearer auth, `Swappable`, and the citation parser got
built from this repo's own code).

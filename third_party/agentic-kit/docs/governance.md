# Governing a public LLM endpoint (the `endpoint-guard` kit)

When you expose a prompt to the open internet — say a "ask my résumé" bot — three
things can go wrong: people ask it things it shouldn't answer, bots hammer it, and
your model bill explodes. The **`governance`** foundations + the **`endpoint-guard`**
kit exist to make all three economically impossible, **using only what's inside the
pod it runs in** (no Redis, no captcha service, no external rate-limiter).

> TL;DR: every request runs a cheapest-first gauntlet that spends **$0 on the model
> for anything it rejects**, a **global budget breaker** hard-caps the bill for
> everyone, an **answer cache** makes repeat questions free, and a **built-in
> proof-of-work** challenge (HMAC-signed server-side — clients cannot mint or tamper
> with one) makes bots pay CPU before they can reach the paid path.

---

## The gauntlet (cheapest-first, fail-closed)

`endpointguard.Guard.Handle(ctx, Request)` runs each request through layers ordered
from free → expensive. The first layer that says "no" short-circuits, so rejects
never reach the model:

```
0. body cap (HTTPHandler)       > MaxBodyBytes (64 KiB default) → templated 413
1. circuit-breaker.Allow()      open?  → 503 Unavailable      (global kill / budget tripped)
2. identity.Extract()           hash the caller (no raw IP/PII stored)
3. strike-ledger.Blocked(id)    blocked? → 403 Blocked        (repeat offender)
4. rate-limiter.Allow(id|global) over?  → 429 RateLimited
5. proof-of-work (if enabled)   — one HMAC+SHA to verify: cheaper than any
                                  embedding/judge call, so it runs FIRST
      no solution      → 402 + HMAC-signed Challenge
      invalid / replay → strike → 400                         (Issuer.Verify is authoritative)
6. topic-gate.Evaluate(question)
      injection/abuse → HardDeny → strike → 400/403           ($0 model)
      off-topic       → SoftDeny → 422 OffTopic               ($0 model)
      ambiguous       → LLM judge — budget-charged BEFORE it fires
                        (TopicJudgeCostUSD; charge refused → templated 422
                         + breaker.ForceOpen(), judge never called)
      on-topic        → continue
7. answer-cache.Get(question)   hit → 200 (Cached)            ($0 model — the big saver)
8. PAID model call (llmclient)  → cache.Put → budget.Charge → 200 Answered
      model error → fail-closed → 503
      budget tripped by this call → breaker.ForceOpen()  (next caller gets 503)
```

**Refusals are templated, and the only LLM spend a rejected request can cause —
a topic-gate judge call — is charged to the budget guard before it fires**
(`TopicJudgeCostUSD`, an honest estimate, default $0.001/call). An attacker can
spam off-topic questions, but the budget — not the attacker — bounds how many
"no"s you pay for: once it trips, the breaker force-opens and judge calls stop.
When PoW is enabled it runs *before* the topic gate, so a bot must burn CPU
before it can trigger even an embedding call. Honest caveat: embedding
similarity calls are still uncharged — they are an order of magnitude cheaper
than a judge call, but not $0. (Cache hits and judge calls each count one
request against the quota dimensions — a judged-then-answered request consumes
two charges, conservative by design — so "free" traffic can't starve the
breaker's accounting.)

---

## Why these choices (the wise defaults)

- **Global budget breaker is the primary bill cap, not per-identity blacklisting.**
  IPs rotate; VPNs and incognito defeat blacklists. So the thing that actually makes
  the bill *un-blowuppable* is `budget-guard` → `circuit-breaker`: it trips on the
  first of `{req/min, req/day, est-$/day}` and then **everyone** gets a polite 503
  until cooldown. For a low-traffic site, being aggressive here costs you nothing.
  The strike-ledger is still there (good for honest repeat offenders + an auditable,
  append-only record), but it's secondary.

- **Zero external infrastructure.** Everything is in-memory inside the pod, with an
  optional local-file **snapshot** (`Guard.Snapshot()/Restore()`, written by the
  sidecar) so a restart doesn't forget the blocklist and budget. No database, no
  Redis, no captcha vendor — one self-contained process.

- **Built-in proof-of-work instead of a captcha service.** A server-side
  `proofofwork.Issuer` mints hashcash-style challenges **HMAC-SHA256-signed over
  (nonce, bits, expiry)**; the client burns CPU to solve, and `Issuer.Verify`
  authenticates the MAC (constant-time), enforces a difficulty floor (`MinBits`) and
  expiry, *then* checks the work, *then* checks replay — so a forged challenge
  (`Bits=0`, far-future expiry) is rejected outright and failed attempts never
  consume a nonce. The replay seen-set is **bounded and fail-closed**: entries evict
  only once their challenge expires; at the cap with nothing evictable,
  verification is refused rather than weakened. The HMAC secret is config
  (`PoW.Secret`, base64; `openssl rand -base64 32`) — production runs refuse to
  start without one; `--demo` disables PoW entirely.

- **Cheap topic gate before the expensive model.** `topic-gate` decides on-topic-ness
  with deterministic prefilters → an embedding-similarity check against your topic
  exemplars (≈$0.00001) → and only escalates to a tiny LLM judge when genuinely
  ambiguous. Off-topic asks are refused for free. **Every judge call is
  budget-charged before it fires** (`TopicJudgeCostUSD`): the kit wraps the judge
  in a charging middleware, so a refused charge means no judge call — a templated
  fail-closed refusal — and the breaker force-opens exactly like the paid-call
  budget trip. **No embedder?** The gate runs **judge-only**
  (`topicgate.Config.JudgeOnly`): prefilters still apply, but every surviving
  question costs one (budget-charged) LLM judge call — workable, but the
  embedding layer exists precisely to make most decisions free. Wire one if you
  possibly can.

- **Fail-closed everywhere.** If the judge or model errors or is unavailable, the
  answer is *deny*, never *allow*. Errors must never become a bypass. The same rule
  governs startup: see "production mode" below.

- **Answer cache.** Résumé-style answers are near-static, so most real traffic should
  be cache hits costing $0. The cache matches exact normalized questions and (if an
  embedder is wired) near-duplicates by cosine similarity; without an embedder it
  degrades to exact-match-only.

---

## Production mode (`cmd/endpoint-guard` without `--demo`)

The sidecar now wires **real providers**: the Anthropic Messages API
(`foundations/llmclient/anthropic`) as both the paid answerer and the topic-gate
judge, and an OpenAI-compatible embeddings backend
(`foundations/embedding/http-embedder` — OpenAI, Ollama, vLLM, ...) when the
`Embedding` config section is enabled.

**Startup fails closed.** Each of these refuses to start with an error naming
exactly what to set: a missing or placeholder `Identity.Salt`, PoW enabled without
an adequate `PoW.Secret` (≥ `proofofwork.MinSecretLen` bytes), no `LLM.Model`, or an
unset API-key environment variable (`LLM.APIKeyEnv`, default `ANTHROPIC_API_KEY` —
the key itself never lives in the config file). `--demo` skips these checks and runs
on mocks.

**Edge hardening.** The `http.Server` carries timeouts against slow-loris and
slow-reader connections (read-header 5s, read 30s, write 60s, idle 120s), and the
`/ask` body is capped via `http.MaxBytesReader` (`MaxBodyBytes`, default 64 KiB)
*before* any JSON decoding — oversized bodies get a templated 413, never an LLM
call. The kit's `HTTPHandler` enforces the same cap itself
(`Config.MaxBodyBytes`), so even hand-wired deployments that skip the sidecar
are body-capped by default.

**Config is seconds-canonical.** Duration knobs are integer `*Seconds` fields
(`Cache.TTLSeconds`, `PoW.TTLSeconds`, `Strikes.BaseBlockSeconds`, ...), matching the
CUE contract in `cmd/endpoint-guard/schema.cue`; the legacy int64-nanosecond shape is
still accepted during the transition, and `*Seconds` wins when both are present.

---

## Portability: one policy, in-pod *and* at the edge (WASM)

Every `foundations/governance/*` package is **pure-Go and compiles to
`GOOS=wasip1`/`GOOS=js GOARCH=wasm`**. That means the same audited policy can run as a
Go service in your pod *or* be pushed to a Cloudflare/Vercel edge worker or the
browser. `examples/governance-wasm/` ships the **proof-of-work solver as browser
WASM** (the client solves the challenge locally) plus an edge-side length/empty
prefilter — so you can reject the cheap junk at the edge before it ever reaches your
origin. Build it with `task wasm-build`.

One honest boundary: the browser-side `govVerifyPoW` is a **local pre-check only**
(solution meets difficulty, not expired) — it proves nothing to anyone else, because
only the server holds the HMAC secret. **Server-side `Issuer.Verify` is the
authoritative verification**; the client's only job is to burn CPU on a server-issued
challenge and return it verbatim.

---

## Using it (when you build the résumé site)

```sh
# demo mode — no API keys, mock model + embedder, PoW off, every topic allowed:
# see the gauntlet work end-to-end
go run ./cmd/endpoint-guard --demo --addr :8787
curl -s -XPOST localhost:8787/ask -d '{"question":"what languages does he know?"}'

# real: copy config.example.json, then set Identity.Salt, PoW.Secret, LLM.Model,
# your topic exemplars and budget ceilings; export ANTHROPIC_API_KEY; enable the
# Embedding section if you have an embeddings endpoint (even a local Ollama).
go run ./cmd/endpoint-guard --config my-guard.json --state-file workspace/state/guard.json
```

`cmd/endpoint-guard/config.example.json` documents every knob — budget ceilings
(`MaxReqPerMin`/`MaxReqPerDay`/`MaxUSDPerDay`), per-call cost estimates
(`EstCostPerCallUSD`, `TopicJudgeCostUSD`), PoW difficulty + secret, strike
threshold + backoff, topic exemplars + similarity thresholds, cache TTL, the
`LLM`/`Embedding` provider sections, and `MaxBodyBytes` — and is itself validated by
`schema.cue`. Defaults are deliberately aggressive-low (≈20 req/min, 500 req/day,
$2/day) — tune up as needed.

Endpoints: `POST /ask`, `GET /healthz`, `GET /metrics`. `/metrics` is Prometheus
text format: `uptime_seconds`, `request_count`, one
`endpoint_guard_requests_total{outcome="..."}` counter per gauntlet outcome
(`answered`, `cache_hit`, `refused_topic`, `refused_injection`, `blocked_strike`,
`rate_limited`, `breaker_open`, `challenge_required`, `pow_failed`,
`answerer_error`), plus `endpoint_guard_budget_trips_total` and
`endpoint_guard_{llm,embedding}_errors_total` (LLM errors labeled
`role=answerer|judge`). State is snapshotted periodically and on shutdown to
`--state-file` (under `workspace/`, gitignored).

### Data handling, honestly

Identity is a salted hash — no raw IP or PII is stored. But the **strike-ledger is
append-only by design** (that's what makes it auditable), and snapshots persist it
across restarts: there is currently **no retention or deletion story** — no TTL on
strike records, no per-identity erasure. The answer cache holds question/answer
text (TTL-bounded, in-memory + snapshots). If you operate this under a data-removal
obligation, deleting the state file is today's only eraser — plan accordingly.

## Building blocks (`foundations/governance/`)

| Package | Role |
|---|---|
| `rate-limiter` | token-bucket, per-key + global |
| `circuit-breaker` | closed/open/half-open + manual kill-switch |
| `budget-guard` | req/min · req/day · $/day quotas → trips the breaker |
| `identity` | salted-hash caller id (no raw PII) |
| `strike-ledger` | append-only violations + escalating-backoff blocklist (no deletion/retention) |
| `proof-of-work` | HMAC-signed `Issuer` (`Issue`/`Verify`, constant-time MAC check, bounded fail-closed seen-set) + client-side `Solve` |
| `topic-gate` | prefilter → injection-detector → embedding similarity → LLM judge (or judge-only); the kit budget-charges every judge call before it fires |
| `answer-cache` | normalized + near-dup answer cache (TTL; exact-only without an embedder) |
| `kits/endpoint-guard` | composes all of the above into `Handle` + `HTTPHandler` + outcome metrics |
| `cmd/endpoint-guard` | sidecar: real providers, fail-closed startup, edge timeouts/body caps, config, snapshots |

All deterministic, injected-clock, fail-closed, and tested with abuse-path
simulations (forged PoW challenges, budget trips, every reject path asserted to
make zero answerer calls).

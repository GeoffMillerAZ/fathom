# expertchat-service — a thin governed-chat example

The smallest **runnable** consumer of `kits/expertchat`: it wires the
governed answer loop to a retriever and a model, then prints either a
`CitedAnswer` (text plus every citation's module identity, tier, staleness
status, and quoted anchor) or a typed `Refusal`. All the actual logic —
retrieval, tier/staleness policy, citation validation, refusal — lives in
`kits/expertchat`; this program is composition only, the same "thin example,
not a new server" shape as `examples/agent-cli`.

It is a **composition example** for `docs/plans/gke-expertchat.md`'s GKE-EC-6
row: a consumer of a fathom module host's `/rpc/modules` surface
(`docs/plans/gke-module-host.md`, Phase G), which has not shipped yet — so
this example ships with a built-in `-retriever demo` mode that needs no
server at all, plus a real `-retriever http` mode ready for when Phase G
lands.

## Running it

```sh
cd examples/expertchat-service

# 1. Demo retriever + mock LLM: no server, no key, no network.
go run . -query "what does the loop refuse to do?"

# 2. Ask the same question as JSON.
go run . -query "what does the loop refuse to do?" -json

# 3. Point at a real module host once one exists (Phase G).
go run . -query "..." -retriever http -url http://localhost:7575/rpc/modules

# 4. Real Anthropic model, demo corpus (never prints/logs the key).
export ANTHROPIC_API_KEY=sk-ant-...
go run . -query "..." -llm live
```

### Flags

| Flag              | Default                | Meaning                                                              |
| ----------------- | ----------------------- | ---------------------------------------------------------------------- |
| `-query`          | (a sample question)     | the question to ask                                                    |
| `-retriever`      | `demo`                  | `demo` (built-in corpus) or `http` (a real module host)               |
| `-url`            | `$EXPERTCHAT_MODULE_HOST_URL` | module-host JSON-RPC URL (`-retriever http` only)                |
| `-module`         | (empty = every module)  | scope the http retriever to one mounted module                        |
| `-llm`            | `mock`                  | `mock` (no key, no network) or `live` (`$ANTHROPIC_API_KEY`)          |
| `-model`          | `claude-opus-4-8`       | model name (live mode only)                                            |
| `-tier-threshold` | (empty = no floor)      | minimum `scoring.Tier` required to cite                                |
| `-require-fresh`  | `false`                 | refuse rather than cite stale evidence                                 |
| `-json`           | `false`                 | print the result as JSON instead of `CitedAnswer.Render()`             |

Exit codes: `0` an answer was composed and printed, `3` expertchat refused
(a typed, expected outcome — not a process failure), `1` an operational
failure (retriever/model call), `2` a usage error.

## What it demonstrates

- **The refusal path is a real, distinct outcome, not an error path in
  disguise.** `-tier-threshold verified` against the demo corpus (all
  `doc-claimed`) exits `3` with `REFUSED (below-tier-threshold): ...` —
  see `TestRun_TierThresholdAboveDemoCorpus_Refuses`.
- **Citations always carry full provenance.** Every printed citation shows
  module, tier, staleness status, source ref, and the quoted anchor text —
  never just the answer text on its own.
- **The retriever is swappable without touching the loop.** `-retriever demo`
  and `-retriever http` both implement `expertchat.Retriever`; `main.go`'s
  `newRetriever` is the only place that knows the difference.

## Wiring point for a real UI

`app-kit`'s `appkit/ui/chat` (a separate repo/track) is the documented
first-party UI consumer of `expertchat.Run`'s output — this example prints
the same `CitedAnswer`/`Refusal` values a UI would render, just as text/JSON
instead of markup. See `kits/expertchat`'s package doc.

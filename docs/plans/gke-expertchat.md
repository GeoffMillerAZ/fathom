# GKE expertchat — the governed answer loop (repo: agentic-kit)

**This plan's rows land in `agentic-kit`, not `fathom`.** Every `{symbol,
package}` anchor below names a future agentic-kit symbol that does not exist
in this repo's index and never will — `fathom verify` against this plan
verifies nothing until it runs with `--service agentic-kit` against the
kit's own index, once the kit's own gate (`task validate` in
`../agentic-kit`, per that repo's `CLAUDE.md`) has landed the code these
rows claim. This plan lives under fathom's `docs/plans/` because it is part
of the governed-knowledge-engine wave (`docs/vision/
12-governed-knowledge-engine.md` §4 Phase H) and fathom's plan discipline is
the one in force for this program — but its verify command, when these rows
are ready to be checked, is run from the kit's own worktree against the
kit's own store, per-repo, exactly as CLAUDE.md's "Refresh through the
server, never the CLI" / per-repo verify convention already requires for any
cross-repo claim.

Design (normative — see `docs/vision/12-governed-knowledge-engine.md` §3):

- **`expertchat` is a kit, not a foundation.** It composes `foundations/
  llmclient` (the model turn — real Anthropic client + the record/replay
  cassette backend for deterministic tests), `foundations/review/scoring`
  (the same evidence-tier vocabulary `docs/spec/store-format-v0.md` §7
  closes over — reused, not reinvented, so a module-host claim's tier
  string means the same thing on both sides of the MCP boundary),
  `foundations/retrieval` (the module-host query step), and
  `foundations/telemetry` (the existence-check instrumentation
  `docs/vision/11-dial-to-eleven.md` §2 law 7 already names) — matching
  the kit's own layering rule (`../agentic-kit/CLAUDE.md`: "kits compose
  foundations... imports point downward only").
- **The governed answer loop's contract, stated normatively:** retrieve via
  the module host (`docs/plans/gke-module-host.md`'s `/rpc/modules`
  surface, an ordinary MCP client call from the kit's own `foundations/
  llmclient`-composed tool-call path — `expertchat` is a *consumer* of that
  surface, exactly the "any MCP client is a frontend" framing in
  `docs/vision/12-governed-knowledge-engine.md` §3) → an answer **MUST**
  cite every claim it asserts with the claim's tier and staleness status →
  an answer with zero retrieved evidence **MUST** refuse rather than answer
  from the model's own prior. This is D-EM4 (`docs/design/
  semantic-layers.md` §5, restated as a conformance requirement by
  `docs/spec/expert-module-v0.md` §2) given to a multi-turn chat loop
  instead of a single lookup tool.
- **No new fathom MCP tool.** `expertchat` is entirely kit-side; it talks to
  fathom's module host the same way any MCP client would. Doctrine 4
  ("consolidate, don't grow") is honored by construction here — there is no
  fathom-side surface change this plan could cause even if it wanted to.
- **`app-kit`'s `appkit/ui/chat`** is the optional first-party UI. Optional
  because the loop's own contract (above) is UI-agnostic and MCP-native — a
  bare terminal MCP client is an equally conforming consumer of
  `expertchat`'s output. This plan does not build the UI; it names the
  wiring point so app-kit's own track knows where to attach.

## Tasks

- [ ] `expertchat` package: governed answer composition — retrieve via the module-host MCP surface, compose a cited answer, using `foundations/llmclient` for the model turn {id: GKE-EC-1, symbol: Run, package: kits/expertchat}
- [ ] Citation-mandatory response type: a response value with an unsourced claim is unconstructable (a type-level guarantee, not a runtime linter) — mirrors `foundations/review/scoring`'s existing tier vocabulary rather than inventing a parallel one {id: GKE-EC-2, symbol: CitedAnswer, package: kits/expertchat}
- [ ] Refusal-without-evidence behavior: zero retrieved evidence from the module host MUST produce a refusal response, never a model-prior answer with no citation {id: GKE-EC-3, symbol: Run, package: kits/expertchat}
- [ ] Tier/staleness rendering contract: every citation in a composed answer carries its evidence tier and non-fresh status when applicable, mirroring `docs/spec/store-format-v0.md` §3 Server conformance at the chat layer {id: GKE-EC-4, symbol: CitedAnswer, package: kits/expertchat}
- [ ] Deterministic test coverage via the kit's replay backend: a fixed module-host response cassette produces a byte-identical composed answer, no live model call required {id: GKE-EC-5, symbol: Run, package: kits/expertchat, skip_test: true, skip: "this row's own claim IS the test; the kit's replay-backend convention (foundations/llmclient/replay) is the mechanism, not a Test reaching Run itself"}
- [ ] Thin hosted chat service example: a runnable consumer of `docs/plans/gke-module-host.md`'s hardened, read-only, authenticated hosted surface (Phase I of the governed-knowledge-engine program) — not a new server, a composition example {id: GKE-EC-6, symbol: main, package: examples/expertchat-service}
- [ ] `app-kit` `appkit/ui/chat` wiring pointer documented: the UI-side attachment point for `expertchat`'s output, named but not built here (app-kit's own track owns the UI) {id: GKE-EC-7, symbol: Run, package: kits/expertchat, skip_test: true, skip: "documentation row — the wiring pointer is a doc comment on Run naming appkit/ui/chat as the first-party UI consumer, not a code dependency in either direction"}

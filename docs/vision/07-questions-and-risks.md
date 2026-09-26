# 07 — Red team: risks, and the questions you're not asking

**Status:** accepted 2026-07-16 as a record — its decisions were taken where
they belong: D2 as ADR-014's amendment (vision 05), D3 as the findings tracer
(vision 03), R1 and R2 as `docs/intent/scope.md` non-goals; Q4 (who pays) was
never answered and stays open as `docs/intent/questions.md` Q4.

**Verdict.** You are reaching too far — in sequence, not in architecture —
and the evidence is your own telemetry. The kernel is genuinely good and
the wedge is real; the correct move this quarter is ruthless subtraction.
The steelman first, because it deserves to be said plainly: in four weeks
you shipped five major waves — exactly the velocity that produces second
systems. The proposed superstructure (knowledge experts, CUE catalog,
hosted backend, plugin system, per-use-case chat) is generalizing from one
working instance before a second exists, and the repo already shows the
habit at small scale: 6 dead analysis packages, `adr`/`ownership` complete
and unwired, enrichment schema'd and unreachable, `schema.cue` never read
by the runtime, `claim_events` modeled and dead, `batch` never called,
concern scoping (a founding pillar, ADR-006) dead in practice. **You build
ahead of demand as a habit. The ambition list is that habit at 10×.**

## 1. Risks, rated — with the cheapest evidence gate for each

| # | Risk | Rating | Cheapest evidence gate |
|---|---|---|---|
| R1 | **Hosted multi-tenant expert backend** | **FATAL at this stage** — store rewrite + auth/tenancy + SLA + data governance for other people's code intelligence, on a bus factor of one. Born-portable stores are the hosted strategy: the file travels, you never hold the data. | An external party runs local fathom 4 weeks and asks *in writing* for a hosted view. Until then: zero lines. |
| R2 | **First-party chat / incident command center** | FATAL as a build target, NON-ISSUE to skip — chat competes with your best distribution channel (Claude Code *is* a chat interface); incident-precedent is already entry-conditioned (≥15–20 findings; you have ~5). | The entry condition is already written. Honor it. |
| R3 | **Plugin system designed a priori** | MANAGEABLE via rule of three — freezing abstractions from n=1 would have baked in every lesson the first fleet compile taught the hard way. | Build layer 2 by hand; diff the two layers; what varies is the manifest surface, what's identical is the engine ([02 §3](02-shape.md)). |
| R4 | **Separate CUE catalog system** | NON-ISSUE as a system; small MANAGEABLE item inside fathom (M1 manifest loading). | Same gate as R3. |
| R5 | **30-language excellence** | MANAGEABLE, already resolved on paper, at risk of relitigation — the adopted vision re-scoped breadth to demand-driven depth, but the old program still reads as "active priority." Sharper: **O6 is in Go**, the one language you verify with. Breadth in languages you don't dogfood while the wedge language's edges are broken is backwards. | Fix O6; re-verify the H1 plan without skip_test. Each further language earns depth when a real repo in it needs the verify gate. |
| R6 | **The 87-tool MCP surface** | MANAGEABLE and URGENT — every registered tool is a forever-liability (schema, docs, tests, naming-collision surface that measurably causes wrong-tool calls). | Gate already passed — the call log is the evidence. Act ([05](05-surfaces.md)). |
| R7 | **Findings/hosted conflation** | The good idea (findings backend) is MANAGEABLE and cheap; the wrapper (hosted) was the fatal part. | Persist one analyzer's output as `Kind:"finding"` claims; run one real investigation against accumulated findings; let the hosted question resurface with data ([03](03-findings-and-experts.md)). |
| R8 | **"Preprocessing is game-changing"** | PLAUSIBLE but UNPROVEN — the null hypothesis (good CLAUDE.md + grep) is undefeated. | Fleet compile → ≥90% injection 4 weeks → violations trending down + one context-bench run. Testable in a month. |

## 2. The great questions you're not asking

1. **Who is user #2, and what is the unit of value they'd install for?**
   Everything so far is fleet-internal. The unit with the most evidence is
   the **verify gate** (top genuinely-chosen tool, catches real
   over-claims, blocking in CI) — a lefthook-class dev-infra primitive, not
   a knowledge platform. Gate: cold-clone `fathom init` on a repo you don't
   own; watch someone else run `fathom verify`.
2. **What is the trust model?** Why should anyone believe a fathom claim?
   The pieces exist (tiers, citations, hash-resolution, honest exit-2), but
   the flagship gate mostly runs on skip_test and tier is a ranking
   tie-break, not an arbitration rule. Trust is the entire moat versus
   "just ask the LLM" — O6 and serve-time precision are strategy items.
3. **What breaks at 100×?** In-memory O(all-observations) queries,
   JSON-string call edges suffix-matched (collision risk), single-writer
   store, memory-only vectors. Non-issue today; fatal only if you chase
   hosted/enterprise — one more reason not to.
4. **Who pays, and does anyone need to?** If fathom stays your
   force-multiplier, $0 is fine and most of the ambition list is
   unnecessary. If it's a product, the verify gate is the only artifact
   with a visible buyer shape. Deciding costs one honest sentence and
   changes everything downstream. *(Decision required — see [08](08-roadmap.md).)*
5. **What is deliberately out of scope forever?** The non-goals list is
   good (no connectors, no web-scale search, no IDE services, no
   embeddings-first chat) but hosted multi-tenant and first-party chat
   aren't on it. Put them there with written entry conditions.
6. **Single-maintainer sustainability and platform risk.** Every shipped
   surface is a forever-maintenance liability (87 registered tools, 37
   analysis packages, 19 CLI verbs, docs already drifting — README still
   describes the superseded WASM path; architecture.md omits the entire
   semantic stratum). And the harness vendors are actively building native
   indexing/memory/context tooling: anything a harness might absorb
   (generic code search, chat) is on borrowed time, while governed claims
   with hash-verified citations and blocking gates is the part they're
   least likely to replicate. Manage by deletion discipline and
   concentration on the moat.
7. **What is the wedge vs the platform?** You keep asking platform
   questions ("should these be different tools?") before the wedge
   question ("has one channel changed agent behavior anywhere?") is
   answered. The wedge is *governed claims injected and gated in the coding
   loop*; everything else is a downstream consumer of that substrate.

## 3. The three decisions that matter this quarter

**D1 — Declare the wedge and freeze the perimeter.** The product through
the quarter is *governed claims for coding agents* — verify gate +
obligations injection on the deterministic index. Write hosted backend,
chat interfaces, plugin API, CUE catalog, and language breadth onto the
entry-conditioned list with explicit gates. Then finish H1's own exit
criteria — the bottleneck on every bigger idea.

**D2 — Prune the surface now.** The evidence arrived ([05](05-surfaces.md)).

**D3 — Land findings-as-claims locally instead of designing the expert
platform.** `Kind:"finding"` + `claim_events` verdicts + one real
investigation over accumulated findings ([03](03-findings-and-experts.md)).
The cheapest possible test of the expert-system idea: it either becomes the
seed with evidence behind it, or it dies for $0.

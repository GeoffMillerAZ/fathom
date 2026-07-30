# 13 — The Expert Suite: builder and chat, dial at 11

**Status:** founding doc, owner-commissioned 2026-07-28 ("you are the owner,
the expert, the designer, the architect… set your vision for what the
potential for world-class product in this category is").
**Builds on:** `12-governed-knowledge-engine.md` (module host, expertchat,
law 9), `docs/spec/` (store-format-v0, expert-module-v0), the shipped
compile/eval/serve machinery (ADR-016/017/024), and the falsifiability
record (docs/plans/trust-and-tokenomics.md T5-8).
**Owner asks folded in verbatim:** a CLI + CUE-backed config experience with
Charm (crush-class) polish including mouse support; a web UI builder that
gathers sources, identifies missing sources, quality-tests, and agentically
advises on readiness; a hosted chat experience with both a Charm TUI and a
web UI; and — the meta-ask — better framing and bigger questions.

## 1. The product thesis

The category is **governed expertise**. Everyone in 2026 can compile a
corpus into an AI that answers questions. Nobody ships the three properties
that make an answer *worth acting on*: you can see **why to trust it**
(tier + citation + anchor), **whether it is still true** (per-claim
staleness against a watermark), and **what it refuses to claim** (typed
refusals instead of confident fabrication). Fathom's engine already has
those properties. The suite's job is to make them **reachable by people who
will never read a spec** — and to make the authoring of a trustworthy
module as guided, measurable, and finishable as compiling code.

The unit of product is the **expert module**: a portable, self-describing,
provably-fresh knowledge artifact (expert-module-v0.md). The suite is the
module's full lifecycle:

```
BUILD (studio)  →  PROVE (evals + readiness gate)  →  SERVE (module host)
      ↑                                                      ↓
      └──────────── the gap flywheel (§6) ←──────── CONSUME (chat)
```

## 2. The better questions (the reframing the owner asked for)

The obvious questions — "what should the builder UI look like," "how do we
add chat" — produce commodity products. These are the questions that
produce a category leader, and each one disciplines a section below:

1. **What makes a module TRUSTWORTHY enough to ship, and how does a
   non-expert author reach that state?** Not "what features does the
   builder have." The builder is a *path to a provable state*, and the
   state is defined by instruments (evals, coverage, anchors, freshness),
   not by the author's feeling of doneness. (§4)
2. **What does a governed answer owe its reader?** Not "how good is the
   chat." An answer owes its provenance, its freshness, and an honest
   account of what it could not say. A refusal is a *product surface*, not
   an error state. (§5)
3. **Where does the next module improvement come from?** Not "how do we
   make authors work harder." Consumption failures — refusals, stale
   citations, low-tier answers — are the highest-signal authoring backlog
   in existence, and today every product in the market throws them away.
   (§6)
4. **What is a UI, if every capability is a verb?** Not "CLI vs TUI vs
   web." One set of engine verbs; three renderings of the same state for
   three kinds of hands. A surface that can do something the verbs cannot
   is a bug. (§3, law 10)
5. **What would make this falsifiable?** Every experience ships with a bar
   it can miss publicly: time-to-first-servable-module, refusal-to-fix
   latency, answer-trust audit. The suite is measured the way the benches
   were — pre-registered, recorded either way. (§9)

## 3. Personas and surfaces: verbs → TUI → web

**Two personas, three surfaces, one state.**

- **The author** (builds modules): ranges from a CLI-native engineer to a
  domain expert who has never opened a terminal.
- **The consumer** (asks questions): ranges from an agent over MCP to a
  person in a browser who must be able to trust what they read.

**Surface doctrine (new law 10 — verb-first surfaces):** every capability
is a plain, scriptable CLI verb *first* (`fathom module …`); the TUI is an
interactive rendering over the same verbs; the web UI is a server-rendered
rendering over the same Go packages. No surface-only capability, ever. This
is what keeps three surfaces honest, testable, and cheap: the surfaces
render state and dispatch verbs; they never own logic.

- **CLI verbs:** deterministic, JSON-emitting, CUE-validated config
  (`module.cue` is the single source of truth for a module's definition —
  the config experience is *editing a validated document*, with `--explain`
  provenance like `fathom config`).
- **TUI (Charm reference implementation):** Bubble Tea + Lipgloss + Bubbles
  + Huh — the crush-class bar: full mouse support (Bubble Tea has it),
  resizable panes, fuzzy pickers, inline forms for CUE fields with
  validation-as-you-type, live progress for compiles/evals, and glow-style
  markdown rendering for claim/citation previews. Pure Go — the fleet's
  no-Node rule (D2) holds even at the pretty layer.
- **Web UI (app-kit path A):** Go server-rendered on `appkit/ui/*` (chrome,
  table, stat, callout, form, viz, chat), MatDash shell patterns re-themed
  through kit tokens. No Node, no SPA build: the web experience is the same
  state machine the TUI renders, served over HTTP with app-kit components.
  This is the surface for the author and consumer who need "a really
  advanced user interface" — and it is *still* one Go binary.

## 4. The Builder (studio): a guided path to a provable state

**Name:** `fathom studio` (TUI + web entry point); underneath it, the
`fathom module` verb family. The studio is a **readiness compiler for
knowledge**: it takes a person with sources and a scope, and walks them —
measurably — to a module that passes the same gates fathom's own layers
pass.

**The loop, as the author experiences it:**

1. **Declare** — `fathom module init`: an interactive (Huh-form / web-form)
   authoring of `module.cue`: identity, scope statement, corpus selectors,
   tier profile, license. CUE validation is the UX, not a build error:
   fields explain themselves, invalid states are unrepresentable.
2. **Gather** — attach sources: local globs, doc folders, findings exports
   (connector SPI; provider-stratum connectors stay parked per ADR-024).
   The inventory panel shows what's actually in corpus — file classes,
   sizes, languages, tool kinds — before any LLM spend.
3. **Map** — the coverage map, the studio's centerpiece instrument. Two
   layers, honestly separated:
   - *Deterministic gaps:* scope globs matching zero files; spec IDs cited
     but never defined; dangling cross-refs; declared concerns with zero
     claim density; sections whose anchors failed verification; watermark
     older than the corpus mtime spread.
   - *Agentic gaps (eval-gated, advisory):* an LLM pass that reads the
     scope statement and the corpus map and proposes "you claim to cover
     X; nothing in corpus speaks to X" — every proposal cited to what WAS
     searched, served with its tier, killable by the same eval discipline
     as any layer. **The advisor recommends; the gate decides (law 11).**
4. **Compile** — the existing machinery (incremental, hash-gated, budgeted,
   dry-run first with cost estimate; identity = prompt+model+inputs).
   Studio renders the budget before and the ledger after.
5. **Prove** — evals (fixtures + deliberate traps, the ≥0.90/≥0.75
   discipline), citation/anchor verification, contradiction scan.
6. **Verdict** — the **module doctor**: a deterministic scorecard that
   rolls conformance (spec markers), eval results, coverage, anchor
   health, freshness, and provenance completeness into a named readiness
   level: `draft → eval-passed → servable → published`. Each promotion has
   criteria a script can check; the studio's "is it ready?" answer is a
   verdict with reasons, never a vibe. The agentic advisor may say "ready
   in my judgment"; only the gate promotes.

**Missing pieces this section commissions:** the `#Module` manifest loader
(spec §7's promised consumer), `fathom module` verbs (init/sources/map/
gaps/compile/eval/doctor/pack), the coverage-map instrument, the agentic
gap advisor as an eval-gated layer, the readiness ladder, and `pack` as the
thin serializer the product-vision already scoped.

## 5. The Chat: a governed answer, rendered like it matters

The expertchat kit (citation-mandatory answers, typed refusals, tier +
staleness on every citation) is the contract. The chat *experiences* render
that contract with pride instead of hiding it:

- **Answers:** every citation is a first-class affordance — tier badge,
  module identity, staleness flag, anchor text on hover/expand, one action
  to view the cited source (module-scoped read, the module host's known
  gap). Trust is *visible*, not a footnote.
- **Refusals are product:** a refusal renders as "here is what I searched
  (modules, scope), here is why I can't answer (no evidence / stale-only /
  below tier), and here is the one-click/one-verb act that would fix it" —
  which feeds §6. No apology theater; an honest dead end with a next move.
- **TUI chat** (Charm): conversation pane + evidence rail; citations
  expandable inline; module picker; mouse everywhere it helps.
- **Web chat** (app-kit `ui/chat` — it exists for exactly this): the
  first-party hosted experience, server-rendered, with the evidence rail
  as the differentiating pane no competitor shows.
- **Hosted:** law 9 holds — read-only surface, bearer auth, module-scoped;
  the hardened `/rpc/modules` from phase G is the only backend either chat
  needs. Feedback (refute/corroborate a claim from the chat surface) is
  the one *write*, and it goes through the existing claim-feedback
  channel, attributed and rate-limited — never a store write from the
  chat path.

## 6. The gap flywheel: consumption failures are the authoring backlog

The suite's compounding advantage, and the thing to build like a product,
not a log file: **every refusal, stale-citation serve, and low-tier answer
is captured (locally, consent-first, per the research-flywheel privacy
model) as a structured gap item** — what was asked, what was searched, why
it failed. The studio's Map panel shows these beside the static gaps:
"12 questions this month died in your pricing-policy scope." The author
fixes sources → recompile → the same question now answers with a fresh
citation. Builder and chat stop being two products and become one loop —
authoring driven by real demand, demand answered by provable authoring.
No surveyed competitor closes this loop; most never see it exists.

## 7. Architecture and repos (who owns what)

| Piece | Lives | Why |
|---|---|---|
| `fathom module` verbs, coverage map, module doctor, manifest loader, pack | fathom | engine-adjacent, store-native |
| Agentic gap advisor | fathom (a layer: eval-gated, killable) | it IS a semantic layer |
| `expertchat` loop | agentic-kit (shipped) | model-turn composition is kit business |
| Studio TUI + chat TUI | new fleet repo `fathom-studio` (Go, Charm) | product surface, releases on its own cadence; imports fathom's public client + kit — never internals |
| Web studio + web chat | same `fathom-studio` repo, app-kit path A | one product repo, two renderings, one state machine |
| Hosted chat backend | fathom `serve-http` (phase G hardened) + kit expertchat service example grown up | already law-9 shaped |

The TUI and web surfaces consume `pkg/client` and the HTTP surfaces only —
the suite is the first true external consumer of the published store
format and `/rpc/modules`, which makes it the H2 portability story's
proof: if the studio needs a fathom internal, the spec failed.

## 8. Doctrine (carried, plus two new laws)

Laws 1–9 carry. New:

10. **Verb-first surfaces.** Every UI capability is a scriptable CLI verb
    first; TUI and web render state and dispatch verbs. A surface-only
    capability is a defect. (Corollary: every studio screen is testable
    headless.)
13. **User scope may add defaults; only project scope may govern**
    (`docs/design/scoped-config.md`). A user-scope file contributes additive
    machine-level defaults and the operator's own notes; it never narrows,
    weakens, or widens committed governance, and every contribution is
    disclosed at the point of use.

12. **No saving is claimed without a counterfactual.** A token number is a
    COST until something establishes what would have happened otherwise
    (`docs/design/production-tokenomics.md` §2). Payload cost is directly
    measurable; savings need substitution evidence or a pre-registered A/B.
    Reporting cost as saving is the dishonesty this program retracted a ×10
    claim over.

11. **The advisor recommends; the gate decides.** Agentic judgment
    (gap proposals, readiness advice) is served with tier + citation and
    is eval-gated like any layer — and it can never promote a module's
    readiness level. Promotion is deterministic criteria only. This is
    the trust boundary that keeps "agentic advising" from becoming vibes
    with a progress bar.

## 9. Execution program (phases; each becomes a gated plan on start)

Sequenced so every phase ships a usable thing and proves a layer beneath
the next. **(op)** = owner action. Kill/terminal criteria per law 5.

| Phase | Ships | Exit / kill criteria |
|---|---|---|
| **J — Module verbs + doctor** | `fathom module` family: init (Huh-less plain first), sources, map (deterministic gaps only), compile/eval wrappers, doctor + readiness ladder, manifest loader, pack | terminal: a stranger-shaped test (scripted cold run) goes corpus → `servable` verdict in one sitting, every promotion criterion machine-checked; kill for `pack`: none (spec-mandated) |
| **K — Studio TUI** | `fathom-studio` repo: Bubble Tea studio over J's verbs (inventory, map, compile, doctor panels; mouse + keyboard parity), chat TUI over expertchat | terminal: J's cold run completed entirely inside the TUI by someone who never types a verb; kill: if TUI state ever diverges from verb output (law 10 violation) → stop, fix the verb layer |
| **L — Agentic gap advisor + flywheel intake** | the advisor layer (eval fixtures + traps first), refusal/gap capture from chat → Map panel (consent-first, local) | gate: advisor precision ≥0.90 on fixture traps before it renders in any surface; kill: two consecutive eval regressions → advisor off, deterministic map stays |
| **M — Web studio + web chat** | app-kit path A app in `fathom-studio`: server-rendered studio + chat with the evidence rail | terminal: feature parity with TUI proven by the shared headless state tests; entry: K shipped (the state machine must exist before its second renderer) |
| **N — Hosted, published, measured** | hardened hosted chat (phase I grown up), module `published` level live, the suite's own falsifiability bench: pre-registered time-to-servable + refusal-to-fix latency + answer-trust audit | entry: ≥1 real module authored by someone who isn't me (op); recorded either way, per the house record |

| **O — Platform modules in the coding loop** | binding schema (`.fathom/modules.cue`), binding-aware obligations serving (repo claims ∪ bound module rules, split displacement accounting), module-tagged edit-hook rendering, doctor binding checks — the platform stratum's proper delivery vehicle (design: `docs/design/platform-modules.md`) plus guarded edits — pre-edit guidance for configured critical files (`docs/design/guarded-edits.md`) — both adopted 2026-07-29 | entry: phase J merged (readiness ladder gates what may be bound: `servable`+); terminal: a bound module's rule injects module-tagged into an edit in a consumer repo, staleness-flagged when not fresh; kill: law 8 applies per module — zero served rules across a qualifying window in every bound repo → unbind candidate |

| **P — Production tokenomics** | payload cost accounting ("what fathom SPENT"), the counting-only discovery-share observer (the denominator the program has asserted blind since product-vision §2), session token ledger, `FATHOM_CHANNELS=off`, service-identity caching (3,624 `list_services` calls/week are hook plumbing), and the pre-registered A/B protocol — design: `docs/design/production-tokenomics.md` | entry: phase O merged (the PreToolUse observer channel exists); terminal for A–D: the report states cost-per-edit and discovery share from real traffic, both labeled correlational; **kill (law 12): if the A/B shows ON no better after 40 sessions, every production savings claim is retracted where it appears** |

**Standing entry-condition honesty:** external-adoption gates from
product-vision H3 remain unmet and unclaimed; this program, like the chat
pull-forward, is an owner decision to build the capability that *generates*
that evidence. The suite's own benches (N) are the falsifiability answer.

## 10. What world-class means here (the bars, falsifiable)

- **Author bar:** corpus → provably `servable` module in under a day for a
  motivated non-engineer, with zero spec reading — measured by the phase-J
  cold run, then re-measured with a real human (N).
- **Consumer bar:** every rendered answer carries tier + citation +
  freshness with zero configuration; a refusal converts to an authoring
  gap item in one action; no uncited sentence is renderable by
  construction (the kit already proves this at the type level — the
  surfaces must not launder it away).
- **Craft bar:** the TUI is the reference Charm application people cite
  (crush-class polish: mouse, resize, forms, live progress); the web UI is
  indistinguishable in capability from the TUI (law 10 makes this cheap
  instead of heroic); everything remains pure Go, one product binary plus
  the daemon.
- **Category bar:** when a third party builds a module with the studio and
  serves it to their own consumers without us in the loop, the
  product-vision's platform *event* has occurred — the suite exists to
  make that event boring.

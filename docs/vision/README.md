# Fathom excellence review — recommendations (2026-07-14)

> **Engine-assumption note (2026-07-23, ADR-025).** SS-8 deleted the DuckDB
> backend outright once the fleet finished migrating to pure-Go SQLite
> (ADR-020) — there is no DuckDB compiled into fathom anywhere now. This
> directory's DuckDB-specific mechanisms below (`.duckdb` store format,
> `ATTACH`-based federation) were written when DuckDB was still the engine;
> none has been accepted into a binding spec yet (see Status below), so
> whichever one is accepted needs a technical re-check against the
> SQLite-only reality first, not a port of the DuckDB-specific mechanism as
> written.

**Status: FOR REVIEW.** Nothing in this directory is binding. These are
recommendations produced by the 2026-07 excellence review (a 13-agent
survey + strategist panel over the repo, the call log, the design docs, and
the July-2026 competitive landscape, followed by synthesis and an
adversarial critique pass). Items you accept graduate into ADRs,
`docs/design/`, and `docs/plans/` as durable specs; items you reject die
here. This directory is deliberately **excluded from the obligations
corpus** — it is exactly the kind of aspirational prose the first fleet
compile taught us never to compile into rules.

## The thesis in one paragraph

Fathom's genuine, verified differentiation is not code navigation (being
commoditized by free LSP/tree-sitter MCP servers), not analysis findings
(owned by Semgrep/CodeQL), and not AI review (saturated). It is the thing
no surveyed tool does: **governing knowledge about code the way code
governs itself** — claims that are content-addressed, pinned to citation
hashes, automatically swept stale/contradicted when their sources drift,
carried in portable single-file stores with provenance, and **delivered by
configuration, not agent choice** (hooks that inject, gates that block).
Everything you want to build — expert modules, findings backends, connected
experts, review products — is a *consumer of that substrate*, and the
substrate is largely shipped. The risk is not the architecture; it is
sequence. Finish proving the wedge (obligations injection + verify gate,
measured) before laying stones no one is standing on.

## Direct answers to your questions

| Your question | Verdict | Doc |
|---|---|---|
| Knowledge expert built on top? | Yes — as expert-module packs (data) over the existing claims stratum, not a new system | [03](03-findings-and-experts.md) |
| Backend for reviews / investigations / compliance — separate tools or part of fathom? | Part of fathom, as **module packs** (CUE manifest + prompt + corpus + eval + kill gate). No new repos. | [02](02-shape.md) |
| CUE cataloging companion system? | Don't build a companion. Make the CUE manifest you already have **load-bearing** inside fathom; a pack format follows; a registry only if H3 conditions fire. | [02 §4](02-shape.md) |
| Hosted expert backend linking modules, findings layered on? | Findings-as-claims **now, locally** (the slot is already reserved in `internal/core/claims.go`); federation = DuckDB ATTACH over portable stores, **contract now, host never-until-pulled**. | [03](03-findings-and-experts.md) |
| How to handle findings vs code versions? | Mostly solved by shipped ADR-016/017 machinery (watermarks + citation hashes + the $0 sweep); one new mechanic needed — the remediation-drift flag on scoped code. | [03 §2](03-findings-and-experts.md) |
| Projects extending semantic layers — plugin system? | Yes, but data-defined (CUE manifests), not code plugins. Rule of three: build layer 2 by hand first, then generalize. | [02 §3](02-shape.md) |
| General tool for many domains without bloat — plugins? what? | One substrate, N data-defined packs; an admission rule + a KILL.md per pack; no plugin API needed. | [06 §4](06-use-cases.md), [02 §5](02-shape.md) |
| Pluggable chat interfaces per use case? | MCP **is** the pluggable chat interface. Command center = failure-mode module + prompt over MCP (entry-conditioned at 15–20 findings); an optional read-only store viewer later. Learning platform: trap. | [03 §4](03-findings-and-experts.md), [06](06-use-cases.md) |
| Am I reaching too far? | Architecturally no — your shipped design anticipates all of it. In **sequence**, yes. Adopt the rung ladder. | [07](07-questions-and-risks.md) |
| What am I not thinking about? | Use-case whitespace + the questions you're not asking | [06](06-use-cases.md), [07](07-questions-and-risks.md) |
| How do you frame this better? | *Fathom is the truth-maintenance system for agent instructions* — and the enforcement layer for spec-driven development. | [01 §4](01-thesis.md) |
| Making coding agents incredible? | Freeze new capability; finish + instrument the loop (8 ranked gaps, ordered). SDD enforcement is the killer feature. | [04](04-coding-agents.md) |
| What does world-class mean, measurably? | The metric table — injection coverage, serve-time precision, share of discovery, adoption friction, context-bench — is the operational definition for the wedge. | [04 §1](04-coding-agents.md) |
| Is up-front AI preprocessing world-class? | Half right — and the half matters. Distilling *code* loses; distilling *binding documents into governed claims* is differentiated **in design, unproven in measured value** until the context-bench and injection gates run. | [01](01-thesis.md), [07 R8](07-questions-and-risks.md) |
| Clean up the tools/MCP surface? | Yes, now — the evidence arrived. Two surfaces, a curated agent surface (15 tools at founding; 16 since T5-4 added `read_source`), kill aliases, execute-and-flag gate. | [05](05-surfaces.md) |
| CUE-driven deterministic SDD state machine? *(2026-07-15 follow-up)* | Goal right — evidence-gated "done" is confirmed whitespace (the whole field self-reports). Mechanism wrong — build a **reconciler**, not a machine: markdown authored, CUE-schema'd underneath, status derived by the pure verify function, transitions enforced at blocking channels. CUE never enters an authoring loop (the Dagger lesson). | [09](09-deterministic-sdd.md) |
| Deterministic SDD — in fathom or a family alongside it? | Inside fathom, in place (verify-kernel + `schema/plan` extensions); agentic-kit supplies oracle classes behind the Evidence SPI; "bearing" reserved as the intent-spine name only if H3 ever fires. Pilot: the agentic-kit handoff program, ~4 dev-days, 5 metrics, 4 kill criteria. | [09 §4–5](09-deterministic-sdd.md) |

## The documents

1. **[01-thesis.md](01-thesis.md)** — what fathom is at its best; the honest preprocessing verdict; differentiation.
2. **[02-shape.md](02-shape.md)** — kernel / module packs / satellites; extensibility mechanism; the CUE decision; bloat discipline.
3. **[03-findings-and-experts.md](03-findings-and-experts.md)** — the findings model; connected experts; hosted; chat.
4. **[04-coding-agents.md](04-coding-agents.md)** — the path to incredible for Claude Code (and Codex); ranked gaps; SDD enforcement.
5. **[05-surfaces.md](05-surfaces.md)** — MCP surface cleanup: keep/kill/merge, conventions, gate semantics.
6. **[06-use-cases.md](06-use-cases.md)** — the portfolio with FOCUS/OPTION/TRAP verdicts; layer-2 recommendation; landscape.
7. **[07-questions-and-risks.md](07-questions-and-risks.md)** — red team: risks rated, the questions you're not asking, three decisions.
8. **[08-roadmap.md](08-roadmap.md)** — the consolidated sequence and your decision checklist. **Start your review here or at 01.**
9. **[09-deterministic-sdd.md](09-deterministic-sdd.md)** *(added 2026-07-15)* — deterministic SDD: the reconciler reframe, where CUE goes and never goes, the handoff-program pilot, and the dial-to-11 trajectory.
10. **[10-rearchitecture.md](10-rearchitecture.md)** — the rearchitecture pass (complete; superseded in emphasis by 11).
11. **[11-dial-to-eleven.md](11-dial-to-eleven.md)** — the founding activation program; doctrine laws 1–8.
12. **[12-governed-knowledge-engine.md](12-governed-knowledge-engine.md)** — the governed knowledge engine; law 9 (hosted read-only before writable).
13. **[13-expert-suite.md](13-expert-suite.md)** — the expert-module suite; laws 10–12 (verb-first, advisor-vs-gate, no saving without a counterfactual).
14. **[14-impact-observatory.md](14-impact-observatory.md)** *(added 2026-08-07)* — the impact observatory: the counterfactual tokenomics program, version-stamped telemetry, the fleet fold, sharing-is-a-human-verb, narrate mode, auto-wire/reconcile, and the observatory web surface (candidate laws 13–15).
15. **[15-reach.md](15-reach.md)** *(added 2026-08-07)* — reach: obligation packs as shippable products (the ui-appkit pack and the contribute-back loop), the language-pack contract and tier list (C++/Conan named), and the dependency-plane decision (shared depgraph+hash library born in ci-kit; Bazel as reference semantics and REAPI reuse, never a dependency).
16. **[16-skills-pack.md](16-skills-pack.md)** *(added 2026-08-07)* — the skills pack: adoption as one install — the `/fathom:xyz` Claude Code plugin (skills+hooks+MCP, marketplace updates as the reconcile channel), the open-standard `.agents/skills` rendering (Codex+Gemini shared path), the pinned rendering, and kit-shipped packs (app-kit + agentic-kit fathom-packs).
17. **[17-observatory-ui.md](17-observatory-ui.md)** — the value-realization UI over the impact verbs.
18. **[18-impact-questions.md](18-impact-questions.md)** — the four questions the observatory must answer.
19. **[19-exploration-ledger.md](19-exploration-ledger.md)** — per-question cost: raw exploration against indexed lookup.
20. **[20-ambient-fathom.md](20-ambient-fathom.md)** *(added 2026-08-08)* — **ambient fathom**: the identity shift from retrieval to *memory and judgement*, the answer to "does fathom stop mattering as context windows grow", a candid weakness table, and the **gate** on the owner's 2026-08-08 asks (accepted / reframed / rejected, each with its reason). Introduces **law 15** (fathom writes only what it owns and never resolves a conflict) and sequences phases T–Y. Designs: [capability-registry](../design/capability-registry.md), [self-healing-fleet](../design/self-healing-fleet.md), [scope-and-purge](../design/scope-and-purge.md), [anchored-memory](../design/anchored-memory.md), [budgeted-answers](../design/budgeted-answers.md), [realization-grade](../design/realization-grade.md); plan: [ambient-fathom](../plans/ambient-fathom.md).
21. **[21-continuity-threads.md](21-continuity-threads.md)** *(added 2026-08-12)* — **continuity threads**: the third dimension (space/truth/**time**); the thread as the unit of work with sessions as events on it; mechanical deposits via SessionEnd/PreCompact/Stop, source-aware ≤600-token injection; the tokenomics gate (cost bounded now, saving pre-registered against the measured 22× cache-rent line, with a registered kill); the SDD spine (intent → plan → proof → record → knowledge, threads as the conveyor); the transcript-mining refusal upheld. Plan rows: TH-1..5 in [ambient-fathom](../plans/ambient-fathom.md).
22. **[22-config-custodian.md](22-config-custodian.md)** *(added 2026-08-13)* — **the config custodian**: "instructed means true" — agent configuration joins the evidence plane. Concerns as falsifiable wants in CUE (user scope contracts, repo scope governs — the capability registry's law-13 shape reused); mechanical, index-backed audits (zero standing LLM); AUDIT/PROPOSE/REFRESH verbs with auto-editing of user prose refused outright (law 15 unsoftened); attention surfaces in doctor, the flywheel, and the session brief. Rows land with the build slice.

23. **[23-doctrine-plane.md](23-doctrine-plane.md)** *(added 2026-08-13)* — **the doctrine plane**: the elevation rule (first-class = identity + claims + checker + surface; what can't carry a checkable claim stays honored prose). ADRs accepted as the strongest candidate ("decided means honored"); VISION reframed as promotion-pipeline tracking; GOALS gated to pre-registered bets (instrument + threshold or refused); ENGINEERING_PRACTICES accepted as content, refused as a fourth registry — one CUE doctrine catalog compiling to existing enforcement surfaces. The SDD observatory panel with the claimed-vs-proven gap as headline, verify ledger before any trend chart.

24. **[24-work-frontier.md](24-work-frontier.md)** *(added 2026-08-21)* — **the work frontier**: the owner's fleet-coordination asks (discovery routers, per-step discovery reuse, work-item leases with heartbeats, DAG-gated dispatch) gated through a 12-agent evaluation. Registers the **dispatch boundary** (fathom serves facts about the work — frontier, eligibility, declared intent; the loop belongs to the harness, forever per 09). Adopts: `ctx:` context-pack pointers + fresh materializer, `needs:` + derived eligibility, TTL-only advisory leases (heartbeats/supervisors refused), `fathom prime` + the opening-moves stanza table for Codex parity's pull tier. Declines the saved-query router. §7 is the evidence verdict of 2026-08-21: verify gate actionable now, thread bet collecting at 4/20 per its own instrument, A/B structurally one-armed (ON: 0 admissible units) — re-arm gated on the instrument-truth wave. Plans: [work-frontier](../plans/work-frontier.md), [instrument-truth](../plans/instrument-truth.md).

25. **[25-harness-port.md](25-harness-port.md)** *(added 2026-09-02)* — **the harness port**: adapters are data behind one seam (`internal/harness`: descriptor registry + one codec per harness, Claude Code byte-identical by golden), on the four open standards the field settled on (`AGENTS.md`, Agent Skills, Agent Plugins 1.0, MCP over HTTP) with Claude Code's hook JSON as the interchange envelope; a first-class degradation ladder (A hooks+inject / B block-only / C JS-only / D none); the fired assumption (Codex has Claude-shaped hooks) and the narrow amendment of 04 §5's sequencing rule (ADR-029: a second harness's inject tier ships `unverified` behind an explicit flag, measurement stays Claude-only); the kit gets the contract on FA-22. Plans: [harness-port](../plans/harness-port.md), [reach-parity](../plans/reach-parity.md).

26. **[26-proof-credibility.md](26-proof-credibility.md)** *(added 2026-09-02)* — **proof credibility**: instruments that cannot lie about themselves (store provenance on gate verdicts — O20/O5 structural; the honesty sweep; ADR-030's lexical oracle for non-code rows; a trust lock over hook commands, MCP entries and skill trees), a bench a stranger can re-run (ADR-031: corpus truth at HEAD, a `none` arm with a committed leak rule, a precision term, statistics owned by the kit, a derived Go localization corpus from public datasets, a free regression gate), depth-bucketed confidence-tagged blast radius and test-gap risk in review, and done-means-proven at the claim moment (`TaskCompleted`/`InstructionsLoaded`/`ConfigChange`). Plans: [proof-credibility](../plans/proof-credibility.md), [bench-public](../plans/bench-public.md), [blast-depth](../plans/blast-depth.md). *Amended 2026-09-12 by ADR-032, the bench pyramid (`../design/bench-pyramid.md`, plan [bench-pyramid](../plans/bench-pyramid.md)): free index-oracle, payload-replay and corpus-gate layers in the gate, remembered cells, targeted runs, H2-3.*

## How this review was grounded
- **[27 — The strengths bench](27-strengths-bench.md)** — ACCEPTED 2026-09-13 (ADR-033). Every claim fathom makes, as a matrix row: versus, an oracle fathom does not own, predicted win and predicted no-effect, written before spend; one scenario driver with arms as data; SDD, guards, exploration-reuse, competitor and freshness rows; repeatable by a stranger.
- **[28 — Receipts](28-receipts.md)** — ACCEPTED 2026-09-13 (ADR-034). The first round's second reading: every wired cell had run without fathom's tools, so a reading, a hand-off and an answer are all claims that carry their evidence or are not made — instrument truth on every cell, evidence anchors the gate verifies, a budget and a receipt on every answer, discoveries as fresh-resolved pointers for workers and the next session, coverage as a code fact.
- **[29 — Verbs, not lookups](29-verbs-not-lookups.md)** — ACCEPTED 2026-09-14 (ADR-035). With the tools present, agents grep first and lookups tie; fathom's lead is the answer grep cannot give. The impact verbs join the agent surface, obligations ranks by governing scope, layer rules compile to tests, every answer carries a resolution tier and can be re-derived without fathom, and fathom enters a corpus it does not own.

Every **repo-facing** number is measured, not argued: the call log
(`workspace/logs/mcp-calls.jsonl`, 1,186 calls over 8 days at the review
snapshot), the adoption baseline (0 fathom calls vs 193 greps, recorded in
`CHANGELOG.md`), the live tool registry (87 tools via `capabilities`), the
dogfood observations, and the shipped code as of `3bb6c29` plus the
uncommitted obligations-v2 hardening. **Landscape figures** (competitor
stats, market claims) come from July-2026 web surveys and are sourced
inline in [06 §3](06-use-cases.md) and [09](09-deterministic-sdd.md) —
treat them as survey claims, not measurements. Where a claim is speculation, the docs label it. One known
caveat the critique pass surfaced: the call log has no origin field, so
gate/hook plumbing and genuine agent tool-picking are not fully separable
([04 Gap 1](04-coding-agents.md), [05](05-surfaces.md)).

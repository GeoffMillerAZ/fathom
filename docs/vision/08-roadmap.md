# 08 — The consolidated roadmap and your decision checklist

**Status:** accepted 2026-07-16 — §1's slices ran as plans and §5's lines became
records (ADR-014's amendment, ADR-016's M1 amendment, ADR-018), though its
checklist boxes were never ticked; the who-pays line (07 Q4) is still open as
`docs/intent/questions.md` Q4. Plans: `docs/plans/archive/gap1-instrumentation.md`,
`docs/plans/surfaces-week1.md`, `docs/plans/m1-manifest.md`.

> **Delta, 2026-07-16 (round 2 — agentic-kit v0.7.0 landed the fathom asks):**
> the kit now ships the three seams this roadmap's next slices compose
> (its [`2026-07-fathom-asks.md`](../../third_party/agentic-kit/docs/plans/2026-07-fathom-asks.md), mirrored in-tree by ADR-019, gate-verified there):
> `prompttemplate.Template.TextHash()` → §1 item 6's content-addressed
> PromptVersion becomes a method call; `llmclient/structured`
> (`ErrTruncated`/`ErrMalformed`) → the obligations compile's
> errBadResponse hand-classification retires inside the M1/precision slice;
> `telemetry/heartbeat` (beats + `CheckLiveness`) → §1 item 1's
> denominator/liveness watchdog composes a kit primitive instead of
> hand-rolling. v0.7.0 is the pin for H1.6's go.work decoupling. Hard specs
> for the next three slices: `docs/plans/archive/gap1-instrumentation.md`,
> `docs/plans/surfaces-week1.md`, `docs/plans/m1-manifest.md`. Also
> executed ahead of schedule: O6 (H1 branch), O9 epoch salt, SDD reconciler
> v0 (ADR-018) — the verify-gate credibility preconditions are met.

**Verdict.** Nothing in this review preempts H1 — it sharpens it. The
sequence below folds every accepted recommendation into the existing
H1/H2/H3 frame, amends two things (layer-2 candidate; ADR-014 gate
semantics), and adds the connective tissue (M1 manifest loading,
findings-as-claims) that turns the expert-system ambition into cheap,
evidence-generating steps.

## 1. Now → H1 exit (~8 weeks — the gate clocks, not the build, set the floor)

Ordered; earlier items unblock later gates. Detail in [04 §6](04-coding-agents.md)
and [05 §5](05-surfaces.md). Honest arithmetic from the critique pass: the
build fits in ~6 weeks only if items 6–7 are treated as start-when-green
(they may slip toward H2 without endangering H1 exit), and the two 4-week
gate clocks (injection ≥90% from the week-3 recompile; verify gate in ≥2
repos from week-3/4 wiring) complete around week 7–8 — judge each gate when
its clock has actually run, not at a calendar line.

1. **Instrument the flagship channel** (Gap 1): injection-coverage
   denominator, channel-liveness watchdog, **call-origin tagging** (a
   prerequisite for every demand number and kill clock downstream),
   root-cause the 07-11–07-14 silence. Commit the in-flight obligations-v2
   hardening; fleet compile; start the ≥90% coverage clock (coverage
   measures channel firing, so the later precision recompile doesn't reset
   it).
2. **O6** — receiver-method call edges in goextract; re-verify the H1 plan
   with `skip_test` removed. (Also the R5 gate.)
3. **Serve-time precision pass** on obligations v2 (Gap 3) — the markdown
   counter-examples observed during this review are the work order.
4. **Surface cleanup week 1** ([05](05-surfaces.md)): alias +
   `set_profile` kills; `verify_plan`/`capabilities` promotion;
   registration flip to the agent surface; execute-and-flag gate (ADR-014
   amendment). Week 2: arg vocabulary, server-side service resolution,
   response envelope.
5. **`fathom init`** (+ AGENTS.md emission): wire repos #2 and #3; the
   verify-gate ≥2-repo clock starts.
6. **M1 — the manifest goes live** ([02 §3](02-shape.md)) — *start when
   items 1–5 are green; may slip toward H2 without endangering H1 exit*:
   load `schema.cue`, delete the Go mirrors, content-address the full
   effective prompt (a **flag-day recompile** — fold into item 3's
   precision recompile so the fleet pays once), enforce run-level budgets,
   widen `#Layer.tier` for `verified`, unlock manifest-listed layers.
7. **Findings-as-claims tracer** (D3, [03 §5](03-findings-and-experts.md))
   — *sequenced after M1, on which it depends; same start-when-green rule*:
   unlock `Kind:"finding"`, wire `claim_events` with the refutation-
   durability rule (`fathom claim refute|corroborate`), remediation-drift
   flag (incl. compile-time scope fingerprints), `fathom report` for one
   analyzer.
8. **Grep-miss taxonomy** (Gap 6) and **context-bench** (Gap 8). Then judge
   each H1 exit gate as its clock completes (~week 7–8).

Also in this window, cheap hygiene the review surfaced: fix stale
README/architecture.md (WASM-era extraction text; missing semantic
stratum), index `docs/use-cases/` + `docs/design/` from `docs/README.md`,
correct the "~25 languages" call-edge overstatement, delete the 6 dead
analysis packages (wire `churn` into `hotspots` first), tombstone
enrichment, record the 3 verify-gate catches.

## 2. H2 (publish and deepen) — amended

As adopted in product-vision.md, plus these changes if you accept them:
- **Layer 2 = ADR/architecture-rules pack** (recommended over failure-mode;
  decision below). Ships via M2: serving-template vocabulary,
  `fathom layer install`, project-local `.fathom/layers/`.
- **Store-format spec v0** — start the doc early; it disciplines every
  schema change and is the composition contract's first artifact.
- **`fathom render`** — read-only HTML report over a store; the
  compliance-probe sales artifact. Entry-conditioned on the compliance
  design partner existing (its named consumer, per [03 §4](03-findings-and-experts.md))
  — not an unconditional H2 line item.
- Cross-file callers for TS/JS/Python/Java: unchanged, demand-driven.
- Positioning for the publish moment: *the enforcement layer for
  spec-driven development* + *the truth-maintenance system for agent
  instructions*.

## 3. H3 (ecosystem) — unchanged, with the never-until list made explicit

Federation via DuckDB ATTACH; compliance edition; multi-expert
orchestration above the engine — all entry-conditioned exactly as written.
Added to the standing non-goals with written entry conditions:
- **Hosted multi-tenant expert service** — until an external party asks in
  writing after 4 weeks of local use.
- **First-party chat interfaces** — until an external consumer builds UI
  on the store format (that event, not a roadmap phase, is platform
  status).
- **Plugin API (Go SPI promotion)** — until the first named external Go
  consumer; `extractor.Extractor` first, never the monolithic `store.Store`.
- **Pack registry/catalog service** — until H3 entry conditions fire; then
  start with a git repo of packs.

## 4. Program law (the two rules, restated once)

> **Admission:** a capability enters the kernel only with a wired delivery
> channel AND survives a 30-day call-log kill gate; everything else is a
> module pack with its own KILL.md.

> **Sequencing:** nothing graduates from contract to code without a named
> consumer or a passed gate: spec → library → CLI/MCP verb → surface →
> service, one rung at a time.

## 5. Your decision checklist

Accepting a line means it graduates to an ADR / design doc / plan; the
linked docs carry the full argument for each.

- [ ] **Wedge declaration (D1):** the quarter's product is governed claims
      for coding agents; the perimeter freezes; the never-until list
      ([§3](#3-h3-ecosystem--unchanged-with-the-never-until-list-made-explicit))
      is adopted into product-vision.md non-goals.
- [ ] **Shape:** one repo; kernel / module-pack / satellite boundary;
      module packs as data ([02](02-shape.md)). → new ADR.
- [ ] **Extensibility = loaded CUE manifests (M1 now, M2 with layer 2);**
      no plugin API, no sidecar, no buildmode=plugin ([02 §3](02-shape.md)).
      → new ADR; supersedes the "CUE catalog companion" idea.
- [ ] **Findings-as-claims (D3):** `Kind:"finding"` unlock + `claim_events`
      wiring + `fathom report` + remediation-drift flag
      ([03](03-findings-and-experts.md)). → plan items.
- [ ] **Hosted / chat:** contract-now-build-never-until-pulled
      ([03 §4](03-findings-and-experts.md)). → non-goals amendment.
- [ ] **Surfaces:** two-surface topology, curated agent surface (15 at founding; 16 since T5-4's `read_source`),
      registration flip, execute-and-flag ([05](05-surfaces.md)).
      → ADR-014 amendment + H1 item 5 execution.
- [ ] **Layer 2 = ADR/architecture-rules** (vs failure-mode playbook)
      ([06 §2](06-use-cases.md)). → H2 plan amendment.
- [ ] **Coding-agent order of work** (Gaps 1–8, [04](04-coding-agents.md)).
      → docs/plans/ update.
- [ ] **The who-pays sentence** ([07 Q4](07-questions-and-risks.md)):
      write it. Force-multiplier ($0, ambition list shrinks) or product
      (verify gate leads, everything else follows it).
- [ ] **Deterministic SDD = reconciler + pilot** ([09](09-deterministic-sdd.md),
      added 2026-07-15): plan-grammar v2 (stable IDs, typed dispositions,
      reason/trigger keys), `verify --require-all`, `gen-plan-cue`
      round-trip, `stop-hook` (advisory→blocking), minimal `cmd` oracle —
      ~4–5 dev-days piloted on the agentic-kit handoff program. Honest
      sequencing: only the *spec authoring* (H1–H8 born typed) is truly
      parallel — those specs must be written now anyway; the dev-days
      sequence **after** §1 item 1 (Gap 1 instrumentation stays first) and
      displace nothing, since items 6–7 are already start-when-green. O6
      remains the hard precondition for the determinism pitch; week-4
      keep/kill decision on the five metrics. Also adds 09 §6's
      out-of-scope-forever list (workflow GUI, durable workflow engine,
      codegen-from-specs, CUE authoring UX, general task tracker) to
      product-vision **non-goals** — unconditional, unlike the
      entry-conditioned never-until entries in §3.
      → schema/plan v2 + plan doc + ADR on acceptance.
- [ ] **Hygiene batch** (§1 tail):
      stale docs, dead packages, enrichment tombstone, gate-catch log.

## 6. What this review changes in existing docs, once accepted

- `docs/design/product-vision.md` — non-goals additions; layer-2 swap;
  already-shipped items marked (ADR-017 is code, not prose).
- `docs/design/semantic-layers.md` §6 + `docs/design/expert-modules.md`
  §3.1 — the obligations corpus was narrowed from "~15 slow-churn governing
  docs" to 3 binding files (design docs excluded by decision after the v1
  fleet compile); mark both superseded on corpus composition and cost.
- `ADR.md` — ADR-014 amendment (gate semantics: execute-and-flag; surfaces
  are ergonomics/measurement, never security; loopback default;
  set_profile's stdio clause retired); new ADRs: product shape / module
  packs; manifest-loaded layers (incl. the `#Layer.tier` widening amending
  ADR-016's tier gating).
- `docs/plans/` — the [§1](#1-now--h1-exit-8-weeks--the-gate-clocks-not-the-build-set-the-floor)
  sequence as contract-bearing checklists (with index-provable symbols, per
  the plans-README rules).
- `CLAUDE.md` — the extraction-path text is stale in the *understating*
  direction: it still says heuristics with "tree-sitter/WASM is the
  documented future path (ADR-004)" while 12+ languages already ship on CGO
  tree-sitter (ADR-013). Fix alongside the README/architecture batch.

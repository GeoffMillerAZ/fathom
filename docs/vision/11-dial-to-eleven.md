# 11 — Dial to eleven: the activation program

**Status:** accepted 2026-07-22 (drafted as a founding doc awaiting owner
greenlight) — its six phases became gated plans that day (commit 6d7855cf),
which §5 names as what greenlight means; vision 12 found A–E complete or in
flight. Plans: `docs/plans/activation.md`, `docs/plans/archive/falsifiability.md`,
`docs/plans/review-verb.md`, `docs/plans/expert-depth.md`,
`docs/plans/new-use-cases.md`, `docs/plans/kit-uplift.md`.
**Successor to** `10-rearchitecture.md` (complete 2026-07-22, all five slices shipped).
**Inputs:** an 8-agent grounded design workflow (4 evidence readers over the
call log / semantic store / deferred inventory / kit boundary + 4 pillar
designers), a use-case divergence pass, an adversarial verification pass on
this doc itself (every §1 number re-derived from raw sources; corrections
applied), and the owner's direct questions (2026-07-22): where next; dial to
11 on coding assistant, expert builder, review, per-project extensibility;
uplift agentic-kit; find the use cases the owner's own vision is missing;
SDD progress; artifact cleanup.

## 1. The reframing finding: the ceiling moved

The rearchitecture removed the engine ceilings (store concurrency, language
truth, edge ambiguity, tool identity). The evidence sweep shows the next
ceiling is not the engine at all — it is **activation and composition**.
Fathom keeps building excellent machinery and then not turning it on:

- **Forced channels work; pulled channels don't.** 764 genuine edit-hook
  obligations injections (p50 13ms for the tagged edit-hook cohort) vs the
  entire M2 orientation quadrant (`entrypoints`/`outline`/`pack_context`/
  `repo_map`) at **zero calls ever**, across two measurement rounds. 4,625
  of 4,707 logged calls are hook/gate plumbing (Gap-1 origin tags prove it);
  genuine interactive tool-picking since instrumentation is ~17 calls.
- **Eval-perfect ≠ production.** `adr-rules` and `spec-bindings` are
  eval-gated at 1.00/1.00 and listed as shipped in the catalog — with **zero
  compiled claims in any production store, ever** (verified directly against
  the live fleet store: all 292 claims are `layer=obligations,
  service=fathom`). Only one pack has ever produced production claims, for
  only one fleet repo, and those claims are 73 commits / 277 corpus-lines
  stale because the flag-day recompile (OP-6) is fully coded and was never
  run.
- **Named levers left unwired.** `brief` is the founding design's "biggest
  tokenomics lever" (semantic-layers.md:226) and has no code path into
  `semantic_claims`. `claim_feedback` — the design's answer to the
  truth-lens attack — was never built as a tool; `claim_events` has zero
  rows in the fleet's life (queried directly). Refutation durability, the
  moat's sharpest edge, is currently fictional in served behavior.
- **Composition debt.** The change-review use case's own doc is an 8-step
  manual MCP sequence; the five analyses it composes exist and there is no
  single verb. context-bench — the falsifiability gate for the entire ×10
  tokenomics claim — has never been built (the longest-unstarted H1 exit
  item, flagged by the roadmap itself).
- **Pulled adoption is measured at zero, with a named confound.** The
  user-scope registration does point at `/rpc/agent` (verified in
  `~/.claude.json`) and path-derived surface tagging has been live since
  2026-07-16 — and in the six days since, zero interactive calls have
  arrived via `/rpc/agent` across ~3,300 logged calls. The confound: the
  flip date was never recorded, and sessions predating it never see the
  surface (restart required), so no-demand and no-connected-sessions are
  currently indistinguishable. B2 removes the confound (recorded t0 +
  connected-client existence check per doctrine 7) before the kill clock
  judges the quadrant. CLAUDE.md still documents the old bare-`/rpc` URL.

**Evidence-population caveat (owner, 2026-07-23 — load-bearing):** the call
log covers one machine over ~16 days during which the owner deliberately
paused all non-fathom work to strengthen fathom/agentic-kit first; most of
his development happens on a work machine this log never sees. The only
interactive sessions in the window were agent sessions building fathom
itself. Therefore the pulled-channel zero-call findings prove **zero
exposure, not zero demand** — the intended consumers (throughline, app-kit,
work-machine sessions) have never seen these tools. The supply-side findings
above (packs never compiled, levers never wired, verbs never composed) are
untouched by this caveat: they are activation debt, not demand signals. The
program's justification rests on the second class, and its adoption-based
kill decisions are gated accordingly (law 8).

**The program named by this finding: stop building new machinery until the
built machinery is switched on, measured, and composed into verbs someone is
forced to encounter — so that when the paused consumers return, the
machinery is already on and the adoption evidence that arrives is clean.**

## 2. Doctrine (carried + two new laws)

1. **Governed evidence is the moat.** Claims with provenance, tiers,
   staleness, refutation durability. Competitors do vibes.
2. **Determinism first.** CUE-validated rules and pure analyzers wherever
   possible; LLM only at the semantic margin, always eval-gated.
3. **Never silent.** Cited refusals beat confident fabrication — including
   "empty config reads as clean" (see C4's honesty audit).
4. **Consolidate, don't grow.** No net tool growth; every addition rides a
   forced channel (see §4 for the explicit arithmetic).
5. **Minimal theater.** Every move carries a kill criterion, or an explicit
   "done-when-stamped" terminal state for pure instrument/activation work.
6. **NEW — Activation is stamped, never prose-documented.** OP-6 sat inert
   for 14 days; the SW-7 flip date was never recorded and cost us the
   ability to distinguish no-demand from no-connection. Any operator-run
   activation step gets a stamp file (`workspace/logs/*stamp*`) the moment
   it runs, and plans reference stamps, not memory.
7. **NEW — Instruments get existence checks.** An adoption metric whose
   traffic source can silently vanish (or never start) is theater. Every
   measurement channel ships with a "is anything flowing?" assertion
   (candidate for promotion into agentic-kit telemetry, K3).
8. **NEW — Kill clocks require a consumer population.** Zero-call evidence
   gathered while the intended consumers are deliberately paused proves
   exposure-zero, not demand-zero. Every adoption-based kill carries a
   recorded workload precondition — the clock starts only when qualifying
   consumer traffic exists (sessions from non-fathom/kit repos appearing in
   the call log, or the owner declaring consumer work resumed — stamped per
   law 6). Until then the tool is *held as unmeasured*, never killed.

## 3. The program

Phases are sequenced by leverage-per-effort; A is days, not weeks. Each
phase becomes a gated plan (`docs/plans/`) on greenlight. **(op)** marks
owner-action items — everything else is agent-executable.

### Phase A — Activation (turn on what's built)

| id | move | kill / terminal state |
|---|---|---|
| A1 | Surface-tag all traffic sources (hooks/CLI send their surface); record the A1 stamp as the adoption-clock t0; fix CLAUDE.md's stale `/rpc` line | done-when-stamped (instrument repair) |
| A2 | **(op)** Flag-day recompile of **every eval-passed pack with an applicable corpus, per fleet repo** — obligations fleet-wide (OP-6) plus the first-ever production compiles of adr-rules and spec-bindings on fathom — after adding a compile idempotency guard (same watermark + same prompt identity → refuse); stamped | post-recompile recall < 0.75 twice for any pack → halt serving that pack, back to eval |
| A3 | Compile obligations for agentic-kit + app-kit — expert modules 2 and 3, claimed zero-new-code | ANY code change needed beyond flags → expert-modules v1 portability claim is false; fix before proceeding |
| A4 | De-theater the catalog: `eval` and `production` (claim count + last compile) as separate columns in docs/layers.md + the layers CLI | a pack still at 0 production claims 30 days **after its A2 compile** → out of the catalog |
| A5 | **(op)** SS-8 (DuckDB+CGO removal) and XT-6b (CGO tree-sitter removal) when their bake clocks fire — **both clocks are already running** (migration stamped 2026-07-23T01:56Z; fleet.sqlite live). **Outcome: SS-8 fired and executed 2026-07-23** — DuckDB driver/bindings deleted outright, `migrate-store` removed, `internal/store` now pure Go and passes `go test -race` under `CGO_ENABLED=0` (`docs/plans/store-sqlite.md`, ADR-025). XT-6b (tree-sitter) not yet fired — separate trigger, still the sole reason `CGO_ENABLED=1` remains required for the full binary | already defined in their plans |

### Phase B — Falsifiability (measure or retract)

| id | move | kill criterion |
|---|---|---|
| B1 | **context-bench**: 10–15 real fleet tasks, run twice by Sonnet subagents (fathom surface on vs off), measuring task success + tokens-to-completion; results in `workspace/artifacts/context-bench/` | fathom arm < 5× token reduction or worse success → retract the ×10 claim everywhere it appears. **Outcome (first run 2026-07-23): criterion FIRED — equal success 15/15, 1.38× aggregate token reduction; retraction applied (falsifiability.md B1-3/B1-4)**. **Second swing on fathom's own home turf (2026-07-24, docs/plans/home-bench.md H3-1): equal success 12/13 vs 12/13, 1.071× token premium — the home-turf pulled-tool story is refuted too; surviving pitch is equal answers / cheaper on 8-of-13 / 2–3× faster / governance unmeasured**. **SUPERSEDED 2026-07-24 (second swing, docs/plans/trust-and-tokenomics.md T5-8, win-audited): after the causal fixes (read_source, line anchors, O12 trio) the same unchanged criteria PASSED — 13/13 vs 11/13 success at 0.570× tokens on home turf; the lookup bench improved to 1.96× (retraction of ×10 still stands, <5×). Honest pitch: strictly better answers at ~2× fewer tokens, 2–3× faster.** |
| B2 | 30-day adoption clock (honoring 05-surfaces' original 30-day discipline; the earlier flip's clock is void for lack of a recorded t0) — **the clock starts at law 8's workload precondition, not at A1's stamp** — with a connected-client existence check so the measurement can't silently read zero; then fold-or-kill the orientation quadrant: fold a ~100-token repo-map-lite into the session hook (M1), kill the four tools if still zero-call | fold breaches M1's token budget → trim or drop; clock never starts while consumers stay paused |
| B3 | verify_plan latency SLO (p50 2.2s today): explicit p95 budget + fail-open timeout matching the 2s hook convention | origin-tagged Stop-hook verifies never breach 2s → non-issue, record and close |

### Phase C — The review verb (compose what exists)

C1/C2 are **blocked on C3's acceptance** — the channel decision comes first,
so nothing is built spec-only.

| id | move | kill criterion |
|---|---|---|
| C3 | **(op)** Accept the forced channel: non-blocking `fathom review --since origin/main` summary in the pre-push gate (warn-not-block, same fail-open pattern as the verify gate) + rewrite the review_change MCP prompt to call C1 | declined → C1/C2 are not built |
| C1 | `review_diff`: one pure-Go composed verb (diff impact w/ typed edges + blast radius + obligations + arch conformance + spec coverage), one cited verdict; CLI mirror; **the program's one new MCP tool** | 30-day zero-call after C3's channel ships, clock gated on law 8's workload precondition → kill per the surfaces admission rule |
| C2 | Findings-as-claims bridge (FT-7's pre-armed trigger): review findings become content-addressed ClaimIDs — **a refuted finding stays refuted across re-reviews**, the property no LLM reviewer has | findings layer's own KILL.md; zero refute/corroborate calls in 30 days → keep claims, kill the ceremony |
| C4 | `.fathom/rules/archconformance.cue` (CUE rule packs for deterministic analyzers — the per-project extensibility wedge) + the "empty-config reads as clean" honesty audit across all analyzer tools | zero fleet repos populate rules in 3 weeks → stop generalizing |

### Phase D — Expert depth (make the conscience real)

| id | move | kill criterion |
|---|---|---|
| D1 | `brief` consumes fresh claims (the named tokenomics lever) — verbatim, cited, tier-tagged | claims-block hit rate < 5% of brief calls after 30 days → revert the wiring (keep the flag), record why claims aren't where briefs look |
| D2 | `claim_feedback` as an action on the already-hot obligations tool + ONE automated caller (the review skill refutes stale findings) | claim_events still 0 rows 30 days post-wiring → kill the automation, keep the CLI escape hatch |
| D3 | Eval-gate enforcement in `fathom compile` (refuse un-evaled layers without a logged `--unsafe-skip-eval-gate`) | flag used ≥3× in normal operation in 60 days → gate ergonomics wrong, redesign |

### Phase E — New use cases (the ones the owner asked us to find)

Ranked by (fleet leverage × differentiation). Each is a tracer bullet with
its own kill; none adds an MCP tool — CLI/gate/ingest surfaces only, MCP
exposure gated on demonstrated demand (doctrine 4).

| id | use case | tracer bullet | kill criterion |
|---|---|---|---|
| E2 | **AI-authorship provenance ledger** — "which code is AI-written, is it riskier: cited and provable"; unclaimed category, near-zero cost, every fleet commit already carries the trailer | ingest full commit bodies (subject-only today, verified: 0 of 763 stored messages contain a body), `kind:"authorship"` claims from Co-Authored-By trailers at tier `verified` (deterministic oracle), one query joining authorship × staleness × complexity | fleet-history backfill shows no usable signal in the join → keep the ledger (cheap, honest data), retract the risk-correlation pitch |
| E3 | **SDD computed progress** — state computed from proof, never asserted (the anti-Jira) | `plans status` (proven/claimed/deferred rollup) + `plans next` (the evidence-derived work frontier for agents) as **CLI verbs**; verify-history regression timelines **depend on D2's claim_events wiring** | agents/gate don't consume `plans next` within 30 days of it appearing in the session brief → keep `status` (reporting), kill `next` |
| E4 | **Governed artifact lifecycle** — the proof survives the cleanup; bounds the gate's O(all-plans-forever) cost (verify_plan p50 2.2s and growing) | `plans archive`: archivable = all rows proven/dispositioned + IDs absorbed into CHANGELOG/ADR + anchors resolve; claims persist in the store past archival (**depends on A2 producing production claims for the relevant packs**); pre-push verifies active plans only; tombstone-restore must exist before the first real archive | first restored-from-archive plan reveals a criterion false-positive → halt archiving until the criterion gains whatever check missed it |
| E5 | **Policy-gate distribution** — ship gates, not dashboards; answers the deferred "org dashboard" without the org buyer (product-vision.md:137's stated blocker) | rides C4: arch/dep/license rules as `.fathom/` config + lefthook, the verify-gate distribution shape generalized | C4's own kill (no fleet adoption in 3 weeks) kills this too |
| E6 | Reverse SDD drift: public surface no spec/binding governs (`api_surface` minus bindings coverage) | one report mode | zero consumers in 30 days → fold into `plans status` output or drop |

**Parked with entry conditions:**
- **Concurrent-agent coordination substrate** (`declare_intent`/
  `check_conflicts` over the shared daemon): the strategic story is strong —
  the shared governed index is an arbiter no single-session tool has — but
  the honest audit found no documented incident of a concurrent-agent write
  conflict yet (O3–O5 are shared-daemon *operational* incidents, all fixed,
  not edit collisions), and the build is a real week+ (new store table,
  agent identity, TTL semantics, two tools). **Entry condition: the first
  documented multi-agent write conflict in the fleet.** Until then it stays
  parked per the build-ahead-of-demand rule (07's R6).
- OSS-maintainer PR trust gate (needs an external audience + a diff-scoped
  connector fetch mode) — revisit at public release.
- Regulated-industry RTM (spec tools ~80% there; runtime test-result
  ingestion is the real cost; no fleet repo in-domain).
- Leadership tech-debt briefs (wire the dead-but-built churn loader when an
  audience exists).

### Phase K — agentic-kit uplift (parallel track, D8-disciplined)

The boundary evidence: fathom imports **13 distinct kit packages, touching 7
of the kit's 27 foundations categories (~8% of its 153 foundation leaves)**
— the relationship is llmclient + a handful of leaves, not the evidence
spine. Uplift means truth and discipline, not bulk adoption:

| id | move | kill / terminal state |
|---|---|---|
| K1 | Execute H4 Stage 4: delete fathom's `internal/vector` (185 LOC) in favor of the kit's vector-store — the kit already generalized this exact code FROM fathom; the duplication is a live doctrine violation | H4 Stage 4's own written acceptance checks (grep-zero, import proof) |
| K2 | FILE (don't build) two parked kit asks per the kit's own D8 discipline: the evidence-agnostic proof-gate kernel of `internal/verify` (469 LOC) and the eval-harness kernel — each with a named-consumer trigger | kill is baked in at filing: never builds until a second named consumer appears |
| K3 | Promote doctrine law 7 (instrument existence checks) into kit telemetry as a reusable assertion; A1/B1 consume it immediately | done-when-consumed by A1/B1; if it fires spuriously ≥3× in 60 days, redesign |
| K4 | Record the deliberate non-adoptions (fathom's own MCP server vs kits/mcp-server, own eval harness vs eval-runner) as one ADR paragraph each — defensible choices with no written rationale | done-in-a-day; terminal |

## 4. Anti-moves (the consolidated NOT list)

- **Net tool arithmetic, stated honestly:** the program adds exactly one MCP
  tool (`review_diff`, forced-channel-gated by C3). Phase E adds none (CLI/
  gate/ingest only). The four orientation tools are killed only if B2's
  clean 30-day measurement says so — the offset is conditional, not assumed.
- No natural-language prose reviewer (Greptile/CodeRabbit's already-rated trap).
- No public Extractor/Connector/Store plugin SDK before a second real consumer.
- No federation/ATTACH, no store-format spec v0, no `fathom pack` serializer
  (H2/H3 entry conditions unmet).
- No failure-mode playbook layer until the findings corpus reaches its stated
  ≥15 entry condition (~5 today).
- No hosted multi-tenant anything (FATAL, risk register R1). Born-portable
  stores remain the distribution story.
- No human-assertable status anywhere in SDD moves — computed from proof or
  it doesn't exist.
- No PreToolUse grep-blocking before the grep-miss taxonomy shows ≥30%
  index-answerable.

## 5. What only the owner can do

1. ~~Run the store migration~~ — **done, stamped 2026-07-23T01:56Z**;
   fleet.sqlite is live and the SS-8/XT-6b bake clocks are running.
2. A2's flag-day recompile — **11 service·packs total: obligations × 9
   fleet services + adr-rules and spec-bindings × fathom. At ~5¢ per
   service·pack that is ≤ $0.60 total on a metered API lane, or $0 marginal
   on the claude-cli subscription lane** (the lane the eval gates already
   run on). Agent-preparable, operator-fired, stamped.
3. C3's channel acceptance (a pre-push hook change is an operator decision).
4. Greenlight this program → each phase becomes a gated plan with the
   standard verify discipline.

## 6. Falsifiability of the program itself

B1 (context-bench) is the keystone — and under the evidence-population
caveat it is the *only* honest measurement available until consumer work
resumes: with organic demand evidence structurally absent (consumers
paused), a benchmarked on/off comparison over real fleet tasks is
workload-independent and decides the tokenomics claim now. If the measured
advantage is not decisive, the correct response is to retract the claim and
re-found the assistant pillar on the forced channels alone (which are
proven). Every phase-E row carries its own kill in the table above, each
adoption-based one gated on law 8. The program's justification deliberately
rests on the caveat-immune evidence class (unactivated machinery,
composition debt, unbuilt falsifiability) — if that class were somehow
shown wrong too, the thesis is refuted and the next doc should say so.

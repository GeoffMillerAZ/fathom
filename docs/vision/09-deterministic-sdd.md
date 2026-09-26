# 09 — Deterministic SDD: the reconciler, not the machine

**Status:** accepted 2026-07-15 as ADR-018 (pilot KEEP 2026-08-26); the v0
reconciler shipped the same day. Plan: `docs/plans/sdd-reconciler-v0.md`.

> **Delta, 2026-07-15 (post-agentic-kit upgrade + implementation):** the
> handoff program executed faster than this doc assumed. H1 fixed the gate's
> recall — **O6 (method-call edges) is fixed and merged** along with callgraph
> dedup, the `package:` grammar, and index hygiene — so the hard precondition
> below is met. `kits/eval-runner` now exists (H5 Stage 1). The H1–H8 specs
> were authored before typing landed, so the "born typed" pilot premise is
> stale: the pilot shifts to shadow-typing the live handoff plans + the next
> program born typed. **The v0 build shipped** (ADR-018,
> `docs/design/sdd-reconciler.md`, `docs/plans/sdd-reconciler-v0.md`):
> grammar v2, `verify --require-all`, `gen-plan-cue` round-trip, `stop-hook`
> (advisory-first). The 4-week metrics window starts when the hooks are wired.

**Verdict.** Your goal is right and the whitespace is real: across the entire
July-2026 SDD field — [Spec Kit](https://github.com/github/spec-kit) (~122k★),
AWS Kiro, [OpenSpec](https://github.com/Fission-AI/OpenSpec) (~61k★),
[beads](https://github.com/steveyegge/beads) (~25k★), Task Master (star counts
via GitHub API, 2026-07-15) — **"done" is agent self-report in every shipped
tool**;
the only deterministic gates sit at the PR boundary and are spec-blind. Nobody
anchors task state to a code index. Fathom's verify gate is the only credible
occupant of that gap. But the mechanism you named — *a state machine driven by
CUE, with plans/specs/state/ADRs all authored as CUE* — is the one version of
this idea with a documented graveyard (Dagger, MDE round-trip, ASL/Argo
YAML-workflow rot). The excellent version dissolves the state machine into a
**reconciler**: intent declared in markdown (CUE-schema'd underneath), evidence
observed by the index and sibling oracles, status **derived** by a pure
function — never stored, never authored — and transitions enforced at channels
that physically block (Stop hook, pre-push, CI). You already built most of it:
`verify.VerifyPlan` is a pure `(plan, evidence) → verdicts` function with
citations, and your own memory note pre-registered the conclusion —
*"dev-state-machine = evidence-backed verify layer, not a tracker."*

## 1. The three lessons that shape everything

**Dagger (the load-bearing datapoint).** Dagger 0.1/0.2 was exactly
"CI/CD as CUE" — a funded team, a killer use case — and they killed it
([official sunset post](https://dagger.io/blog/ending-cue-support/), Dec
2023). Hykes, verbatim from his HN comments on the sunset: *"the number one
complaint from our early users was having to learn CUE. The number two
complaint was bugs in the language … including crippling memory leaks …
remaining a CUE-only platform was suicide."*
Every CUE success (Timoni, your own `schema/plan` + `schema/semantic`,
app-kit's "CUE is source of truth") concentrates authoring on one expert or
a generator and uses CUE only for *what is valid*, never *what happens
next*. And your fleet already ran the experiment in miniature: the
CUE-as-state path (`gen-status-cue` + the `#Status` schema) shipped and
**has never been generated in anger — zero readers, zero artifacts on
disk**. The market of one has spoken.

**Statecharts-as-data (XState, Step Functions ASL, Argo).** Machines-as-data
survive only with managed runtimes and visual tooling, and still grow a bad
embedded expression language at the `when:` clause. Temporal shows the
opposite pole: real durable processes want code + a log. Both poles are
products, not features a solo dev builds.

**Kubernetes (the winning dissolution).** Nobody authors a transition table.
Desired state is declared (data), actual state is observed, controllers
derive status as typed Conditions. The state machine dissolves. Fathom
already is this pattern: observations in, level-triggered re-derivation on
HEAD drift, verdicts out.

**The rule that reconciles your ambition with the evidence:**

> **CUE defines what is *valid* and what status *derives*. Humans and agents
> author markdown. Go evaluates. CUE never enters an authoring loop and
> never becomes the engine.**

ADRs, engineering practices, and contribution guides do **not** get
rewritten in CUE — they are the obligations layer's *corpus* (ADR-016,
shipped); the typed part is the compiled claim and its citation. Forcing
rationale into CUE is the BDD-glue mistake: a hand-maintained second
codebase between intent and code.

## 2. The category error worth naming (your md/json/jsonl itch)

The heterogeneity that bothers you is a symptom, not the disease. Classify
the agentic-kit audit's seven headline failures: **zero were caused by
format heterogeneity.** The failures that involve state at all trace to
(a) *unverified claims* — which the verify gate already fixes — and (b)
*unmechanized honor rules* (the remaining two — LLM-surface lag, unproven
on-ramp — are capability/adoption gaps no plan format would have changed,
which makes the same point). The honor-rule class: prose
triggers ("verify the trigger first"), ad-hoc states (`[~]`, "kit-side done,
donor NOT deleted"), and three hand-synced ledgers (plan checkboxes,
TRACKING.md, CHANGELOG — the audit's #1 was TRACKING claiming GREEN while
reality disagreed). TRACKING and CHANGELOG drift not because they're
different file types but because they're projections of events no ledger
records. Fix the ledger and the oracle; the formats become views.

## 3. The four real gaps (the whole build, and nothing more)

The live programs — agentic-kit's uplift + handoff, fathom's H1 plan — have
already written the requirements in prose. Everything below is additive to
`internal/verify` + `schema/plan`:

1. **Stable task identity.** Markdown task IDs are positional (`md-001…`) —
   inserting a line renumbers everything, making history impossible. Add an
   author-owned `id:` key to the existing `{…}` annotation block.
2. **Typed dispositions.** Today's grammar has `[ ]`/`[x]`/`[-]`; the live
   programs hand-invented `[~]`, "honestly refused", "deferred pending
   trigger", "kit-side done, donor NOT deleted" — machine-invisible prose.
   Type the minimum: `claimed | in-progress | refused | deferred | skipped |
   open`, where refused/deferred require a `reason:` cite and deferred
   carries a `trigger:` (recorded and surfaced — not machine-evaluated in
   v0). Note the corpus's own legend: agentic-kit's TRACKING defines `[~]`
   as *in progress*, so `[~]` maps to in-progress, not deferred; deferral is
   an explicit disposition key.
   Disposition is the *one* legitimately authored state: intent is not
   derivable from evidence. Everything else stays computed.
3. **Persisted prior status + diff.** Each verify run is stateless today; a
   Pass→Fail regression is invisible. Adopt STATUS.cue as the persisted
   previous state (it's already generated, already schema'd, and nothing
   reads it — zero new schema): `status --diff` emits typed transition
   events, with timestamps from git commits, preserving the no-clock purity
   of the evaluator. History without a runtime. TRACKING/CHANGELOG rows
   become *generated projections* of this one ledger, ending three-way
   hand-sync.
4. **Claim classes beyond `symbol`.** The gate's founding catch (a bash-
   heredoc "symbol" claimed `[x]`) was *rewritten out of the gated file*
   because the grammar couldn't type it. Gates become conjunctions of checks
   against named **oracles**:

   | Oracle class | Proves | Status |
   |---|---|---|
   | `index` | symbol_present, tier, test_reaches, edge_wired, no_contradiction, spec_covered | shipped (`verify.Evidence`) |
   | `cmd` | a pinned command ran and passed (`go test -run X`, `task validate`) — finally a real `runtime_pass` | small, new (pilot item F) — adds the claim class that lets contracts stop needing test-reachability waivers |
   | `lexical` | ABSENT_PROVEN with coverage manifest + fail-closed skip accounting | shipped in agentic-kit (`evidence-signature`/`direct-investigator`); fathom is its designed downstream adapter |
   | `eval` | behavioral claims via eval-runner `#EvalRecord`s, judge-independence enforced | future (agentic-kit H5); verdict vocabulary already compatible |
   | `human` | ratified decisions, doc-shipped, donor-deleted — an sme-stated signed journal line | new, tiny — makes the currently-invisible claim classes gateable |

   **Conjunction only — no expression language.** The moment a gate needs
   OR/negation/arithmetic, that logic becomes a named oracle written in Go
   (tested, deterministic). This is the single most important restraint;
   it is where ASL/Argo/XState all rotted.

**Hard precondition: O6.** 12 of 16 claimed tasks in the only live gated
plan waive `test_reaches` via `skip_test: true` (the method-edge recall
gap). A gate that's routinely waived is theater with extra steps — the
Gherkin rot dynamic (when the check fights honest work, people route around
it, and then it checks nothing). The two fixes are distinct: **O6 removes
the false-negative cause of the waivers** (method-reached symbols verify
again); **the `cmd` oracle adds `runtime_pass`** — a stronger claim class
("the test ran and passed") that makes reachability waivers unnecessary
rather than merely rarer. Do O6 before any determinism pitch; track the
skip ratio weekly and if it doesn't fall below ~30% in 4 weeks, the
criterion is theater — remove it rather than train gaming.

## 4. Where it lives (your "family alongside fathom?" question)

**Inside fathom, in place — extend `internal/verify` + `internal/sdd` +
`schema/plan`.** No `cmd/sdd` binary, no sibling repo, no new agentic-kit
process package. Each alternative fails the program law today: the verb
rung is `fathom verify`/`fathom plan` subcommands; a second binary is the
*service* rung, needing a consumer that wants the driver without the index
(none exists — the index IS the oracle). agentic-kit's role is settled and
correct: it supplies oracle classes behind the `verify.Evidence` SPI;
fathom imports the kit, never the reverse (the kit's eval-ladder design
names fathom a downstream Tier-1 adapter it deliberately never imports).

**One CUE story, not two.** This *instantiates* the module-pack mechanism
([02 §3](02-shape.md)) on a second artifact class — same `schema/` family,
same `cue vet` gate today, and the same M1 loader once M1 lands (the pilot
needs only `schema/plan` + `cue vet` and does **not** wait on M1) — and
*extends* the killer-feature positioning ([04 §4](04-coding-agents.md))
with the item that section didn't yet name: typed non-code states and
status diffing. A parallel CUE subsystem would repeat the "CUE cataloging
companion" mistake 02 §4 already rejected.

**Framing and naming.** "Development control plane" is the correct
*internal north star* — declared intent, observed state, reconcile loop,
actuators — and the wrong external banner (it claims orchestration
authority the system doesn't have; Claude Code drives, fathom gates). Keep
the shipped banner: **"the enforcement layer for spec-driven development."**
The dial-11 phrase for docs: *"GitOps for software intent — declared in
specs, observed by the index, reconciled by the gate."* Reserve the
nautical name **bearing** (the declared course, vs fathom's measured depth)
for the intent spine *if* the H3 gate ever fires — until then it's a schema
family and a verb, not a brand.

## 5. The pilot: run it on the agentic-kit handoff program, this week

The perfect subject exists right now: `handoff-h0-index.md` is drafted
(uncommitted) and the H1–H8 specs it indexes **don't exist yet** — they can
be *born* in the extended grammar with zero mid-program rewrite. The
uplift plan is frozen history; fathom's H1 plan is the untouched control.

**Code (~3.5–4 dev-days total, all in fathom, TDD per house rules):**
- **A.** Grammar extension: author-owned `id:`, `[~]`→deferred, `reason:`/
  `trigger:`/`skip:` annotation keys (`internal/verify/markdown.go` +
  `schema/plan/schema.cue` + testdata). ~0.5d.
- **B.** `fathom verify --require-all` — the wave-advance oracle: gate also
  fails on unclaimed/unverified tasks; exit protocol unchanged. ~0.5d.
- **C.** `gen-plan-cue` + lossless round-trip test in `task validate` — the
  CUE shadow that proves "everything could be CUE" without changing what
  anyone authors. CUE becomes output, never input — which means also
  retiring or re-documenting the *existing* hand-authored CUE/JSON plan
  input (`gen-status-cue`'s `loadPlan` and `schema/plan/schema.cue`'s
  "hand-edited #Plan" comment), so the invariant is true rather than
  aspirational. ~1d.
- **D.** `fathom stop-hook` — on Claude Code Stop, verify claims flipped to
  `[x]` this session (working-tree vs HEAD diff); block with the failing
  criterion's citation as `reason`; daemon-down = silent exit 0 (edit-hook
  doctrine). Advisory first week, then blocking. ~1–1.5d.
- **E** (stretch): persist previous STATUS.cue; weekly sweep reports
  Pass→Fail regressions — STATUS.cue's first machine consumer (and its
  first generation in anger). ~0.5d.
- **F** (week 3+, sequenced after O6): minimal `cmd` oracle — run a pinned
  command, map exit code → verdict, record the output digest as evidence.
  This is the `runtime_pass` unlock the oracle table names; without a
  schedule slot it would be a precondition nobody builds. ~1d.

**Five falsifiable metrics (4-week window):** (1) zero checkbox-drift
incidents in the H program vs the audit baseline (weekly `--strict` sweep);
(2) 100% of wave advances made by recorded `--require-all` exit codes, not
coordinator judgment (~5 spine edges — countable); (3) Stop hook catches
≥1 real over-claim, with spurious blocks counted; (4) authoring tax ≤10
min/spec, measured while writing H1–H8; (5) 100% of skips carry `reason:`,
skip share ≤ the 75% control baseline (12 of 16 claimed tasks on the
control plan) and falling once O6 lands.

**Kill criteria (any one suffices, decide week 4):** authoring tax >15
min/spec or ≥3 honest states the grammar can't express; round-trip loses
tasks; >25% of new-spec claims forced onto escape hatches by false fails
(evidence layer not ready — retreat to verify-only); the hook catches zero
real over-claims and causes ≥2 spurious blocks, or `--require-all` runs get
pasted after the fact as decoration. Machinery nobody consults is theater
regardless of how deterministic it is.

**Must-nots:** no new repo or package family; no rewrite of existing plans;
no CUE authoring by humans or agents; no transition engine, lifecycle
enums, or event log beyond the STATUS.cue diff; no blocking of either live
program (fail-open stays fail-open; hook advisory-first).

## 6. The trajectory at 11 (every stage entry-conditioned, never dated)

1. **The development reconciler proven** *(entry: pilot metrics pass)* —
   every fleet change flows spec → gate → evidence → claim. "Compliance-
   grade" becomes literal: every transition verdict is a content-addressed,
   citation-hashed claim in a born-portable store — **the auditor gets a
   `.duckdb`, not a screenshot**; "every claimed task in Q3 whose evidence
   was below tier-verified" is one SQL query, reproducible against the
   snapshot hash. `runtime_pass` closes by ingesting CI results as an
   evidence tier. *Earns next:* 100% of fleet commits gated; planted false
   claims (D10 canary discipline) caught ≥95%; median gate cost <1s; zero
   months with the gate disabled.
2. **Autonomous maintenance loops** *(entry: you trust a verdict you didn't
   watch)* — the reconciler runs on cron/CI computing drift (spec
   unsatisfied, obligation violated, CVE, doc-drift); each drift item opens
   as a gated work item with an acceptance contract pre-attached. **The
   human moves up a level: approving transitions, not reviewing diffs.**
   *Earns next:* majority of drift items closed with zero human diff-reads;
   zero gate-should-have-caught regressions.
3. **Safe multi-agent fan-out** *(entry: SubagentStop/TaskCompleted wired
   to verify)* — waves as parallelizable DAG nodes with per-task contracts:
   an orchestrator spawns N workers whose done-ness is **machine-checked**,
   killing the over-claiming failure mode that makes agent teams
   untrustworthy. The handoff program's coordinator/implementer/ledger
   roster is this stage's prose prototype. *Earns next:* over-claims caught
   at SubagentStop >0 then trending down; wave throughput ≥2× solo at equal
   defect-escape; a full wave with the coordinator never reading a diff.
4. **Enforcement backend for everyone's specs** *(entry: an external repo
   wants the gate)* — Spec Kit/Kiro/OpenSpec all emit markdown checklists
   and self-report completion; two of them (`/speckit.converge`, OpenSpec
   deltas) show the incumbents know drift is their weakness. Converters
   (their artifacts → fathom's claim schema) ship as data-defined module
   packs: **they own the 122k-star authoring funnel; fathom owns the only
   deterministic "done."** MCP makes the state surface portable to
   Codex/Cursor/Gemini; git hooks + CI backstop everywhere. *Earns next:*
   one external repo with a documented blocked push; converter coverage
   ≥90% on a sampled corpus; time-to-adopt under an hour.
5. **Expert modules whose gates ARE the process** *(entry: the store holds
   enough history that precedent queries return non-empty)* — the
   compliance module stops generating reports and becomes the process; the
   incident module walks claim → task → spec → the gated transition that
   approved it and files a finding *as precedent*, so the next plan touching
   that spec class inherits a stricter contract automatically.
   Organizational learning as data — no vendor has it because no vendor has
   the claim ledger.

**Out of scope forever, even at 11:** workflow GUI / visual state editors;
hosted orchestration / multi-tenant SaaS (the artifact travels — ADR-017);
a durable workflow engine (no Temporal clone, no `tools/flow` runtime);
generating code from specs (the Tessl/MDE round-trip trap, failing again in
public — code stays the only master; fathom verifies *against* it); CUE as
agent/human authoring UX; a general task tracker (intent+status without
evidence is commoditized and already churning).

## 7. The honest answer to "am I being naive?"

**About the substrate, yes — about the goal, no.** "Everything could be CUE
with deterministic driving through it" is the Dagger bet at solo scale, the
MDE bet about source-of-truth, and your documented build-ahead-of-demand
habit given a lattice; the md/json/jsonl heterogeneity caused none of your
recorded failures, and your one existing CUE-state artifact has no reader.
But the underlying conviction — that self-reported "done" is the disease and
evidence-gated state is the cure — is confirmed whitespace with no shipped
competitor, your verify gate already caught a real lie on its first run, and
the gaps it can't see are documented in your own programs' prose
workarounds. You were one framing away: **deterministically *derived* state,
and no machine.**

Three conditions under which this is a great idea rather than a trap:
1. **CUE never enters an authoring loop** — markdown in, CUE underneath,
   Go evaluating, forever.
2. **The handoff program is the sole backlog and sole consumer** — every
   typed state/claim class/trigger must be mechanically read by a hook or
   the coordinator within 30 days of shipping, or it's pruned. Nothing
   admitted from imagination of future consumers.
3. **It ships as verify-kernel extensions inside fathom, one rung, no
   family.** The moment it asks for a new repo, binary, always-on service,
   or the word "family," that request is the trap announcing itself — and
   H1's exit gates keep running first.

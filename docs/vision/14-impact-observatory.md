# 14 — The impact observatory: proving help, not asserting it

> Status: FOR REVIEW · Date: 2026-08-07 · Non-binding (this directory is
> excluded from the obligations corpus). Builds on: 11 (dial to eleven,
> laws 1–8), 12 (law 9), 13 (laws 10–12), 07 (the red-team verdict),
> docs/design/observability-surfaces.md, research-flywheel.md,
> production-tokenomics.md (all binding). Motivated by: the 2026-08-07
> fleet audit (18 services, five wiring shapes, 19.3k call log, 46.9k
> beats) and four owner directives — a deployment dashboard, reliable
> token-savings estimation, flywheel centralization with opt-in-only
> sharing and version-stamped records, an enhanced-understanding mode,
> and an auto-wire mode.

## 0. The claim this document defends

Fathom's differentiation is governed knowledge (01). The next differentiator
is **governed self-knowledge**: fathom should be the one tool in the agentic
stack that can *prove* what it costs and *prove* what it changes, with the
same honesty machinery it applies to code claims. Everything below is that
one idea applied to five surfaces. Nothing here weakens the red-team's D1–D3
(wedge frozen, surface pruned, findings-as-claims local) — this is
instrumentation and rendering over the wedge, not new perimeter.

## 1. What the audit established (the ground truth this round answers)

- The machinery genuinely fires: 3,849 edit-hook beats in one day,
  obligations answering at scale, stop-hook verify gating real plans.
- But 90% of traffic is fathom's own plumbing; agent discovery is real and
  bottom-rung (read_source 8× everything else); the upper ladder
  (symbol_context, pack_context, cross_repo_*) is nearly untouched.
- Wiring is **five different partial shapes across the fleet** — full stack
  in exactly two repos, every hook pointing at a stale repo binary rather
  than the deployed one, the CLI on nobody's PATH, and two registry
  anomalies (a malformed duplicate service still fed by a consumer's
  post-commit curl; a nested corpus root) that required manual archaeology.
- The strongest un-designed win: **planning-as-a-remote-corpus** — 792
  read_source calls in four days against a repo with zero local wiring.
  Cross-repo reading is a killer app nobody wired for.
- Every telemetry record type except the research bundle carries **no
  fathom version**; every artifact is local; **nothing anywhere phones
  home** (verified structurally: zero HTTP-client code in the flywheel
  paths).

## 2. The counterfactual program — how "token savings" becomes a number worth saying

Law 12 stands: a token figure is a cost until an arm establishes the
counterfactual. The program that turns it into a *reliable estimate*:

- **The arms already exist.** `FATHOM_CHANNELS=off` is the OFF arm's kill
  switch; instrument D (the session ledger) records **harness-reported
  real tokens** per session from the transcript, not estimates. What's
  missing is only the discipline wrapper: `fathom arm off --sessions N`
  stamps the ledger rows with an `arm` field and doctor announces the arm
  loudly (the O15 lesson: a silent kill switch is a measurement bug).
- **Paired, pre-registered, ranged.** Comparisons are declared before the
  data (the T5-8 bench discipline, promoted from bench to production):
  matched task classes, N sessions per arm, criteria filed in
  `workspace/artifacts/research/` before the first OFF session. Results
  print as ranges with the arm counts — never a point, never a headline
  without its denominator.
- **Three honest layers, never blended**: (a) *measured cost* — payload
  bytes/4 estimates and ledger-real tokens, always labeled which; (b)
  *correlational signals* — discovery-share, turn counts, compaction
  frequency (on the benchmark list, still uninstrumented); (c) *armed
  savings* — the only layer allowed to use the word "saved". The
  tokenomics explainer's OBSERVED/SPECULATIVE tagging becomes the schema:
  every rendered impact number carries its layer.
- **Session-level attribution**: the ledger gains
  `injected_rules_used` (did a later edit touch a file an obligation
  named?), `verify_catches`, and `discovery_substitutions` (an index call
  within 60s of a would-be grep) — mechanism-labeled events, so the
  narrative layer (§6) has facts to cite instead of vibes.

## 3. Version-stamped telemetry (law 13 candidate)

Every persistent record fathom writes gains two additive fields:
`schema_version` (per record type, monotonic) and `fathom_version` (the
producing binary's commit, exactly what the research bundle already
carries). Call log, hook beats, session ledger, daily rollups, statusz
JSON — all of them. Rationale: the fleet fold (§4) and any trend claim is
garbage the moment a record's semantics changed under it (the
251,834-catch headline was exactly this class of error *within* one
version); version-stamping makes relevance filtering mechanical and makes
"this number predates the fix" a query, not an investigation. Old rows
without the fields read as `version: unknown` — never backfilled, never
guessed (law 3).

## 4. The fleet fold — centralized consumption, still local

The flywheel's per-repo artifacts stay per-repo (research-flywheel §8 was
right to refuse silent aggregation). What's added is an explicit,
human-run fold: **`fathom fleet report`** — reads every indexed service's
`report.json` + the user-central beats/ledger, writes ONE fleet artifact
(`~/.fathom/fleet/report-<date>.{json,md}`) with per-service rows:
freshness, wiring shape vs the current template, injection coverage,
traffic, spend, open observations, plans status. It is the audit that took
an agent an afternoon, as a standing one-command product. Local file, no
daemon writes, no network — the dashboard's data source and the
auto-wire's worklist. Consumption is centralized; **collection stays
sovereign per repo**.

## 5. Sharing is a human verb (law 13)

Elevated from design to doctrine: **nothing fathom writes ever crosses the
network un-asked; the only shareable artifact is the sanitized bundle, and
a human runs the verb that creates it and the hand that moves it.** The
bundle's three-layer allowlist (closed Go struct → closed CUE schema →
byte-level label sanitizer, adversarially tested) is the reference pattern
for any future shared artifact, including the fleet fold: if a fleet
bundle is ever wanted, it is the same construction — salted hashes, no
free text, no repo identity, version-stamped, `--dry-run` prints the exact
bytes. Opt-in is not a setting; it is the absence of any code path that
could do otherwise. This is the clause that keeps a pioneering adopter
out of trouble with their own security team — and it is fathom's
sharpest differentiator against every cloud code-AI tool.

## 6. Narrate mode — the enhanced-understanding dial

The owner's ask, designed under law 12's constraint:

- **`fathom narrate on --for 2w`** (or `--sessions N`): an explicitly
  expensive, **auto-expiring** mode — activation is stamped with its
  expiry, and doctor shows it like the arm flag. Off is the default;
  re-arming is deliberate. This is the pioneering dial: turn it on to
  build the case for your org, turn it off and keep the reports.
- What it records while on: **story rows** — for each intervention, the
  mechanism-labeled event plus its surrounding context (the rule injected
  and the next three tool calls; the verify catch and the diff of the
  claim it blocked; the discovery substitution and the grep it replaced),
  appended to a per-repo `workspace/artifacts/narrate/` JSONL,
  version-stamped, never in the bundle.
- What it writes weekly: **the impact narrative** — an LLM-written,
  citation-bearing story ("this week fathom injected 214 rules; 9 were
  followed by an edit conforming to the rule; verify blocked 3 claimed
  tasks, here are their diffs") where every sentence cites story rows by
  id, and the narrative's own spend prints in its footer. A narrative
  that cannot cite a row for a claim does not make the claim (law 3
  applied to prose).
- **Law 14 candidate**: narrative telemetry is a *lens* over
  mechanism-labeled events, never a source of truth — nothing downstream
  (gates, metrics, bundles) may consume a narrative as evidence.
- Flywheel dividend: story rows where the agent *ignored* an injection are
  the highest-value tuning signal fathom has never captured — the
  "what to do better" the owner asked for, as data.

## 7. Auto-wire — reconcile, don't scaffold (law 15 candidate)

`fathom fleet reconcile` (dry-run default, `--apply` to act) does
continuously what the audit did by hand:

1. **Hook reconciliation in place** — diff each repo's installed stack
   against the current template per scope; add missing events/matchers
   (the Grep|Glob|Read read-path), dedupe, and *upgrade* — the current
   "init would duplicate, so do nothing" dead-end becomes a diff.
2. **Binary indirection** — hooks point at a stable shim
   (`~/.local/bin/fathom` → the deployed libexec binary), installed by
   reconcile; hook behavior stops depending on when a repo checkout was
   last rebuilt. Closes the fleet-wide PATH observation.
3. **Registration hygiene in the daemon** — ValidServiceID enforced at
   ingest (not just client-side), corpus roots canonicalized via
   EvalSymlinks at registration, nested-root ingests refused with the
   containing service named, and `remove_service`/`rename_service`
   shipped so registry surgery stops being sqlite archaeology.
4. **Third-party wiring lint** — crawl lefthook.yml/taskfiles/CLAUDE.md
   stanzas that mention fathom; flag stale names (the `../geoffmiller-cloud`
   curl, the "sextant"-branded stanza) and non-resolvable invocations
   (the non-interactive-shell PATH failure that made a dead gate look
   green).
5. **Scoped instruments** — the adoption gauge gains `--service` and a
   daemon-side call-log path so "is fathom helping in repo X" is
   answerable from repo X; hook beats gain a `service` field so doctor
   stops citing one repo's beats as evidence about another.
6. **Frontier detection** — sessions observed in unindexed repos surface
   as a reconcile row ("pumpkin: 3 sessions this week, not indexed"),
   because the audit's strongest finding was value appearing where nobody
   wired anything.

Law 15 candidate: **wiring is reconciled, not scaffolded — a drifted hook
stack is a diff to apply, never a refusal to touch.**

**Self-identification, then self-healing (the no-stopping property).** The
owner's operational ask is that wiring problems surface and fix themselves
instead of being discovered by audits. Mechanism, in three tiers of consent:

- **Tier 0 — every hook self-describes.** Hooks stamp their beats with the
  template version they were installed from and the binary version they
  invoked; skew between installed-template, current-template, and deployed
  binary becomes a computable fact per repo, not an investigation.
- **Tier 1 — the session-hook is the tripwire.** It already runs at every
  session start and prints one context line; it gains a wiring self-check
  (template skew, shim-vs-stale-path, stanza staleness, unindexed cwd) and
  prints the *exact* remedy — `fathom fleet reconcile --apply` or the
  rewire prompt — so every drifted session opens by telling its agent how
  to fix it. Detection is free and universal; nothing mutates.
- **Tier 2 — consented auto-apply.** A config flag (`reconcile.auto = true`,
  per-user or per-repo) lets the session-hook apply the *safe* subset
  (hook-command path repointing to the shim, additive matcher upgrades,
  stanza regeneration) at session start, logging a beat for each applied
  diff. Structural changes (scope moves, removals) always stay behind the
  explicit verb. Hooks never rewrite hooks mid-session — application
  happens at session boundaries, where the harness re-reads them.

The upgrade treadmill inverts: advancing fathom no longer means re-wiring
consumers by hand; it means the next session in each repo notices, says
so, and — where consented — fixes itself.

## 8. The observatory — the dashboard, sequenced honestly

Law 10 (verb-first) orders the build: every screen below is a rendering
of a verb that already works headless.

- **Rung 0 (exists)**: `/statusz`, doctor, instrument, metrics series,
  studio TUI (shipped, 3 commits, phase K scope).
- **Rung 1**: the fleet fold verb (§4) + `fathom narrate report` — the
  data plane of every screen, consumable as markdown/JSON with no UI at
  all.
- **Rung 2 — `fathom-observatory`** (in the fathom-studio repo, a second
  binary): the web app observability-surfaces.md already commissions —
  Go server-rendered, app-kit path A, ui/viz SVG charts, loopback-first
  with the same optional bearer gate as /rpc/modules. Four boards:
  **Fleet** (per-service freshness/wiring/coverage/traffic/observations —
  the §4 fold, live), **Spend** (the cost ledger, never captioned as
  savings), **Proof** (the counterfactual scoreboard — armed results
  only, with their pre-registrations; empty until arms have run, and it
  *says why*), and **Story** (narrate mode's weekly narratives with
  their citations expandable, the org-pioneer's demo surface).
- The dashboard never writes, never computes a number the CLI can't, and
  renders "not measurable: <reason>" strings verbatim — the honesty rules
  are inherited, not re-implemented.

## 9. The wider vision round (safety · correctness · planning · SDD · governance · tokenomics)

- **Correctness first, still**: the open O1 code fix (deterministic
  service resolution — the 36% silent-zero class), the session-hook
  degraded branch (O2), the CUE evidence tier (O3 — `vet_clean` /
  `export_golden` so CUE-first repos stop needing `skip_test` on
  everything), and doctor's three wrong-repo checks (O5). These are the
  trust floor under every metric above.
- **Planning/SDD**: the brace-tokenizer fix + loud LintPlan (O4);
  `fathom plans status` becomes a fold row so SDD progress is a fleet
  fact; the plan grammar's next increment is *spec traceability
  density* (what share of claimed work cites a spec ID) as an honest,
  mechanism-labeled metric — never a "quality score".
- **Governance**: obligations compiles are due fleet-wide once the daemon
  has an LLM profile — with O8's dry-run cost fix landed *first*, so the
  owner's "what will it cost" gets a range, not a confident zero. Expert
  modules proceed on the readiness ladder (draft → eval-passed →
  servable → published), with O17/O18/O19's authoring loop fixes (draft
  lane, rejection reasons, root-resolution) as the gate to calling it a
  product.
- **Safety**: narrate mode makes near-misses visible (guard previews that
  fired, verify catches, refusals) — the safety story is told from the
  same story rows, not a separate system. Nothing in this round adds an
  actuator; fathom's writes remain: local files, its own store, hook
  stdout.
- **Tokenomics**: §2 is the program. The one new number worth chasing
  this quarter: **compaction frequency** under fathom-on vs fathom-off —
  it is the metric closest to what a working engineer *feels*, it is on
  the benchmark list, and nobody in the market can currently prove it.

## 10. Activation order (each stamped, per law 6)

1. Ops now (no code): the PATH shim; fix the `../geoffmiller-cloud`
   consumer curl; `fathom remap` platform-engineering to its canonical
   root; registry surgery for the malformed twin + nested root per the O1
   playbook.
2. Version stamps (§3) + arm/ledger fields (§2) — additive, small, they
   gate everything else's trustworthiness.
3. `fathom fleet report` (§4) + reconcile dry-run (§7) — the audit as
   product.
4. Narrate mode v0 (§6): story rows only, no LLM weekly yet; validate the
   capture cost is tolerable.
5. The counterfactual pilot: one pre-registered OFF-arm week in one busy
   repo; publish the range, whatever it says (a null result retires the
   savings claim honestly — law 12 cuts both ways).
6. Observatory rung 2, Fleet + Spend boards first; Proof and Story light
   up only when their verbs have data.

The order is falsifiable end to end: every step ships a verb whose output
a later step renders, and no screen exists before its number does.

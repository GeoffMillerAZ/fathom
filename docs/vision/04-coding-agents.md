# 04 — Making fathom incredible for coding agents

**Status:** accepted 2026-07-16 — §6's order of work ran as plans (Gap 1, Gap 3
with §4 item 3, Gap 4), all proven; ADR-029 (2026-09-02) amended §5's
sequencing rule; the H1 exit it schedules is still unjudged (futures F2).
Plans: `docs/plans/gap1-instrumentation.md`,
`docs/plans/o8-precision-and-speclines.md`, `docs/plans/init-and-adoption.md`.

**Verdict.** The coding-agent thesis is right and half-proven; the
differentiator is the governed-claims pipeline delivered through forced
channels, not code preprocessing (see [01 §2](01-thesis.md)). The single
recommendation: **freeze new capability; finish and instrument the
existing loop** — the eight gaps below, in order. The coding-agent
companion is not one use case among many; it is the proving ground that
makes every other ambition credible.

## 1. World-class, measurably

| Metric | Target | Status (verified during this review) |
|---|---|---|
| **Injection coverage** — % of Edit/Write in indexed repos receiving obligations | ≥90%, payload p95 ≤320 tokens (ADR-016 amended 2026-09-24; was 150) | **Unmeasured — no denominator exists** (Gap 1) |
| **Injected-obligation precision** — % of injected rules binding-and-relevant to the edit | **≥0.95 at serve time** (tighter than the 0.90 compile eval; serve noise is trust-fatal) | Never sampled. Live counter-example: markdown edits receive "use the race detector" (over-broad scope) and "bucket pending results by language" (a compiled description of how the system works, not a rule) |
| **Retrieval latency** | p95 ≤200ms per companion call | **Met** — obligations p50 36ms / p95 90ms (max 745ms), staleness_report ≤17ms in the log |
| **Fathom share of discovery** (vs Grep/Read) | ≥40% sustained 4 weeks | Baseline **0%** (0 fathom calls vs 193 greps) |
| **New-repo adoption friction** | ≤10 min, one binary, one command | Manual multi-step recipe; `fathom init` not started |
| **Context-bench** | equal-or-better task success at ≤1/10 discovery tokens | **Built and run 2026-07-23** (docs/plans/falsifiability.md B1): equal success 15/15, 1.38× aggregate tokens, 2.81× wall — the ×10 form is retracted; it was marketing. **2026-07-24 home-bench follow-up** (docs/plans/home-bench.md H3-1) re-ran the same criterion on fathom's own home turf (index-native tasks) and it also did NOT clear "strictly better success": 12/13 vs 12/13, equal, 1.071× token premium. Surviving pitch: equal answers, cheaper on 8/13, 2–3× faster, governance unmeasured by that bench. **SUPERSEDED 2026-07-24 (second swing, docs/plans/trust-and-tokenomics.md T5-8, win-audited): after the causal fixes (read_source, line anchors, O12 trio) the same unchanged criteria PASSED — 13/13 vs 11/13 success at 0.570× tokens on home turf; the lookup bench improved to 1.96× (retraction of ×10 still stands, <5×). Honest pitch: strictly better answers at ~2× fewer tokens, 2–3× faster.** |
| **Channel liveness** *(new)* | every forced channel proves it fired recently, or alarms | Does not exist — and it just failed silently (Gap 1) |

## 2. What works, what doesn't

**Works.** The enforcement inversion — *delivery is configuration, not
agent habit* — is the most important product decision in the program, and
it is evidence-driven. The call log proves it: ~95% of all MCP traffic is
hook-driven; non-hook calls are ~50 in 8 days, led by `verify_plan` (30) —
though note the critique-pass caveat: the verify CLI itself calls the
`verify_plan` tool over `/rpc` and the pre-push gate runs that CLI, so with
no origin field in the log, some unknown share of those 30 is gate plumbing
rather than agent tool-picking. Mechanics are excellent: silent-fail 2s edit-hook,
zero-output-on-clean, compile-time citation verification, $0 sweep,
fail-open pre-push vs fail-closed CI. Latency is solved. The verify gate
catching 3 over-claims by its own author on first run is the best demo the
product has.

**Doesn't.** The pull path is dead on arrival (companion surface: 11 calls
on launch day; the only call since was a gated `capabilities` bounce a week
later — see [05](05-surfaces.md)). 71 of 87 tools have
zero calls; `batch` has never been called. Measured arg-shape failures
(`references` requiring `query` when sibling tools take `name`/`symbol`).
And the flagship injected channel has no instrumentation of its own health.

## 3. The eight gaps, ranked

**Gap 1 — Injection coverage has no denominator, the channel went dark
unnoticed, and calls carry no origin.** The log shows obligations at
144/470/456 calls on 07-08/09/10 (log dates are UTC), then **zero calls of
any kind 07-11 through 07-14**. Maybe no sessions ran; maybe the hook died —
*the log cannot distinguish these*: it records injections, not edit events.
Fix: the edit-hook writes a local event per edit seen (injection emitted /
empty / error) so coverage = injections/edits is computable; the
session-hook emits a liveness line; a weekly rollup alarms on
zero-edit-days-with-sessions. **Include call-origin tagging in the same
change** (e.g. an `X-Fathom-Origin: edit-hook|session-hook|verify-cli|agent`
header recorded in the log) — without it, gate/hook plumbing pollutes every
"agent demand" number, including the 30-day kill clocks in
[05](05-surfaces.md). Until this exists the ≥90% gate is unfalsifiable and
H1 cannot exit honestly. **Days of work; do it first.**

**Gap 2 — O6: receiver-method call edges.** `test_reaches` misses symbols
reached via method calls, forcing `skip_test: true` on 12 of 16 gated plan
claims. One extractor fix (receiver-type resolution in goextract)
simultaneously upgrades verify_plan, test_impact, find_callers, and
blast_radius — the densest single improvement available. It is also a
*strategy* item: the trust model rests on it ([01 §5](01-thesis.md)).

**Gap 3 — Serve-time precision pass on obligations v2.** Sample what
actually injects across the fleet (the markdown counter-examples above are
real); tighten scope discipline (code-scoped rules must not carry `**` or
doc-matching globs) and re-run the precision eval against served payloads,
not just compile output. Trust-fatal if skipped.

**Gap 4 — `fathom init`.** One command, one binary, idempotent, <10 min
including first ingest: ignore file, daemon registration, hook entries,
lefthook snippets, plan template, AGENTS.md/CLAUDE.md stanzas. Every other
gate needs repos #2..N wired; the verify-gate exit criterion (≥2 repos ≥4
weeks) is stalled partly on this. Include the service-identity fix: unknown
service must return `did_you_mean`, never silent-empty ([05](05-surfaces.md)).

**Gap 5 — Surface prune + arg normalization.** See [05](05-surfaces.md).
Wrong-tool and wrong-arg failures are measured trust killers.

**Gap 6 — Grep-miss taxonomy.** 193 greps, zero classified. Sample them:
which were index-answerable vs legitimately textual (string literals,
comments, log messages)? Yields (a) the realistic ceiling for the 40%
share gate, (b) which one or two retrieval improvements matter (likely
content search over indexed sources — the one class where fathom concedes
to grep by design), (c) whether the rejected PreToolUse grep-hint deserves
revival at its ≥30% evidence bar.

**Gap 7 — Working-tree honesty beyond the edited file.** Query results
silently reflect the last ingest during heavy uncommitted work. Cheap fix:
stamp every companion response with the watermark + a "N files drifted"
count so the agent can self-doubt. A dirty-overlay ingest is
over-engineering for now.

**Gap 8 — Context-bench.** ~20 discovery tasks over fauxtown + one real
repo; grep/Read agent vs fathom-first agent; task success + tokens. This is
what makes the ×10 claim a fact — and what defeats (or confirms) the null
hypothesis that a good CLAUDE.md plus grep is enough.

Deferred, correctly: cross-file callers for TS/JS/Python/Java (H2,
demand-driven — for a Go-heavy fleet O6 dominates). Meanwhile fix the
overstatement in `docs/use-cases/cross-repo-blast-radius.md` ("~25
languages" call edges) — exactly the kind of over-claim the verify gate
exists to punish.

## 4. Spec-driven enforcement: the killer feature

The SDD wave (Spec Kit: 30+ agent integrations) has a hole: the verify
phase is human prose. Nobody gates a spec's claimed completion against the
actual code index. `fathom verify` is that gate, already live. To make it
killer:

1. **Land O6** so contracts stop needing `skip_test` — a gate that waives
   its test criterion 75% of the time is a demo, not a product.
2. **Make plan mode emit contracts by default** — a plan-template skill so
   `{symbol, reachable_from, min_tier}` is what plans look like, grounded
   via `pack_context`/`test_impact` at authoring time.
3. **Close the loop at edit time**: when an edited file cites a spec ID,
   the edit-hook injects the spec's binding line alongside obligations —
   literally "tying knowledge concepts to code," through the channel that
   already works.
4. **Stop-hook strict mode** in contract-bearing repos: a session cannot
   end on an unproven `[x]`.
5. **Record every gate catch** (3 already happened — write them down).
   Catches are the sales asset.

Positioning: *fathom verify is the enforcement layer for spec-driven
development* — riding 30+ agent integrations someone else built.

## 5. Beyond Claude Code (Codex et al.)

Stratify channels by portability:
- **L1 gates (lefthook pre-push, CI `--strict`): 100% agent-agnostic** —
  they bind Codex, Cursor, humans, and cron equally. The strongest argument
  for gate-first strategy.
- **MCP over HTTP: portable** — Codex and serious harnesses speak MCP.
  `fathom init` emits config stanzas for both.
- **Passive context:** add AGENTS.md emission (fold into gen-claude-md /
  init). Cheap.
- **The genuinely Claude-Code-specific piece is L2 injection** (SessionStart
  / PostToolUse). Mitigations in preference order: rely on L1 + MCP
  documented in AGENTS.md; a pre-commit advisory printing obligations for
  staged files (commit-time, harness-agnostic); a wrapper shim only if a
  second harness becomes a real consumer. **Defer all Codex work except
  AGENTS.md-in-init until the Claude Code numbers are green.**

## 6. The order of work

1. **Week 1:** Gap 1 (denominator + liveness watchdog + origin tagging);
   root-cause the 07-11–07-14 silence; commit the in-flight v2 work. Also
   the *cheap* surface actions from [05 §5](05-surfaces.md) — alias +
   `set_profile` kills, verify_plan/capabilities promotion, registration
   flip, execute-and-flag gate — they are hours, not weeks, and the flip
   must precede any kill clock.
2. **Weeks 1–3:** Gap 2 (O6); re-verify the H1 plan without skip_test.
3. **Weeks 2–4:** Gap 3 (serve-time precision recompile — fold the M1
   prompt-identity change in here so the fleet pays for one recompile, per
   [02 §3](02-shape.md); the injection-*coverage* clock is unaffected — it
   measures channel firing — but precision is sampled against *this*
   recompile's payloads) + Gap 4 (`fathom init` + service-identity); wire
   repos #2 and #3, starting the verify-gate clock.
4. **Weeks 3–5:** Gap 5 (arg normalization + envelope; remaining prune
   rides the 30-day clock) + Gap 6 (grep-miss taxonomy) + Gap 7 (watermark
   stamp).
5. **Weeks 5–6:** Gap 8 (context-bench).
6. **Week ~8:** judge H1 exit — not week 6: the ≥90% injection clock (4
   weeks from the week-3 compile) and the ≥2-repo verify-gate clock (4
   weeks from week-3/4 wiring) both complete around week 7–8. Judge each
   gate when its clock has actually run.

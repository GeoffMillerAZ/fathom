# Intervention outcomes — do the firings actually help?

> Owner ask, 2026-08-14: *"do we have instrumentation on how well guards,
> concerns and stuff are working? … journal entry every time these fire,
> or a retro at the end of every session … it's ok if it costs tokens …
> the extra tokens should try not to harm our benchmarks … so we can do
> analytics and create evidence-backed claims."* Binding companion to
> `docs/design/continuity-threads.md` §4.1 (outcomes derive from disk);
> rows IR-1..5 in `docs/plans/ambient-fathom.md`.

## 1. The gate on the ask

- **Per-fire LLM journaling: REFUSED.** An LLM narrating inside a session
  contaminates the session it measures (the owner's own caveat, made
  structural), costs tokens continuously, and records what the model
  *thinks* happened while it is happening. Retro beats journal.
- **Session-end retro: ACCEPTED, two lanes** — the §4.1 shape, reused
  rather than re-invented:
  - **Mechanical retro (standing, free): `fathom retro`.** Joins the
    firing record against what is on disk afterward. No LLM, no daemon
    dependency, runnable for any session window, forever.
  - **LLM retro (benchmark lane, priced): `fathom assess --interventions`.**
    Judges the rows mechanics cannot decide (did the edit follow the
    rule's *spirit*?), from on-disk artifacts — diffs, plan deltas, gate
    results — **never transcripts** (the refusal is not softened for
    evaluation; what shipped is also more honest than what was said).
    Explicitly invoked or inside a declared bench window; per-run token
    cost reported in its own output.
- **Benchmark isolation (the owner's caveat, pre-registered):** retro and
  assess run AFTER a session and inject nothing into any session; their
  spend is metered under its own label and excluded from arm/bet unit
  token totals. A bench unit's numbers never include the cost of judging
  it.

## 2. IR-1 — firings become joinable (the wiring gap)

Today's beats say a guard fired; they do not say *in which session* or
*about which file* — so "did it help" is unjoinable. Every edit-hook beat
(pre and post, seen/answered/error) now carries `session` and `file` meta
(the values are already in scope in the hook; the beat just never kept
them), and the custodian's attention line emits a `custodian/attention`
beat when injected. Absent values are omitted, never written as `""` —
an unrecorded fact must not look recorded. The beat log stays local
(law 13) and bounded (existing rotation).

## 3. IR-2 — the mechanical retro

`fathom retro [--session S | --last] [--project DIR] [--json] [--write]`

Window = the span of the session's own beats (self-contained; no ledger
dependency). For the project's firings in the window:

| Firing | Mechanical outcome |
|---|---|
| guard fired (pre, emitted) on file F | F later committed in/after the window / modified-uncommitted / untouched — the note preceded a real edit, or it stopped one (untouched after a fire is a *signal*, not proof; assess judges intent) |
| obligations injected (post, emitted) on F | same join + rules-served count from the payload-cost beats |
| custodian attention injected | re-run the audit NOW: violations cleared since (remedied) or still standing (age named) |
| discovery beats | raw vs indexed counts for the window (the exploration ledger's view, session-scoped) |

Honesty rules: firings without a `file`/`session` stamp (pre-IR-1 rows)
are counted and named as unjoinable, never dropped; a window inside an
OFF arm says "channels were silent by design — absence of firings is the
experiment, not adoption failure"; `--write` lands the retro under
`workspace/artifacts/retros/` (a scenario corpus for analytics and,
sanitized and opt-in, the research flywheel — IR-5).

## 4. What this buys

The claims ladder: today fathom can say "7,065 injections served"
(activity). The retro adds "N injections preceded a committed edit to the
governed file; M guard fires preceded an abandoned edit" (outcome-shaped
evidence, mechanical). Assess adds "of those, K edits complied with the
injected rule, judged from the diff" (quality, priced, occasional). Only
the arm carries causality (law 12) — the retro never claims the edit
happened *because* of the firing, and every surface that renders retro
data says so.

## 5. Out of scope for v1

`assess` itself (IR-3 — designed and built in `docs/design/assess-lanes.md`,
ADR-046: typed decision-port questions, spend isolated under IR-4);
turn-position within the session (ts ordering suffices until someone
needs more); any auto-filed observation from a retro (a human or an
explicitly-invoked agent reads the retro and decides — instrument, not
hand, applied to feedback).

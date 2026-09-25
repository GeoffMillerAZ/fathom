# CV-M8: the fleet recompile (2026-09-25)

**Verdict (plain language). CV-M8 passes.** The fleet recompile ran for
every service in the fleet with no compiled layers, or with fully stale
ones, against CV-41's new default layer set (`obligations` always, plus
`adr-rules`/`spec-bindings` wherever their own corpus matches). Dry-run
priced first, on the production model, then spend recorded: **26 layer
compiles across 12 services, 2,798 claims emitted, $7.05 total** over
three passes. Two real tooling defects surfaced and were fixed mid-run
(items 2 and 3 of this same wave), not worked around: the dry-run priced
every one of those 26 layers at cost_usd "est. 0.00-0.00" — a confident
zero, not a genuine estimate — until item 2 gave the dry run a real
per-model price; and the default set's own bigger layers
(`adr-rules`/`spec-bindings` on a repository whose corpus exceeds one
run's `max_tokens_per_run`) hit the per-run budget's pre-flight refusal on
every sizeable repository in the fleet, exactly the gap item 3 closes —
without it, CV-41's new default set could activate on paper but could
never actually run a first compile on any real repository bigger than one
budget.

Bar (CV-M8, `docs/plans/refactor-conservation.md`): "the fleet recompile
ran for services with no or fully stale layers, dry-run priced first, with
the spend recorded." Met: every pass below was preceded by a dry-run price
(once item 2 made that price real), and every dollar spent is accounted
for in the per-layer table.

## What ran

Three passes were needed, not because the fleet is unusually large, but
because the two defects above were found BY this run and fixed IN it,
mid-program — the same "a program is a better test of the tooling than any
unit test in isolation" pattern CV-M6 already established for the refactor
verbs.

1. **First pass ($1.70).** `fathom compile --dry-run` against every
   service's CV-41 default set. The dry run's own cost_usd read "est.
   0.00-0.00" for all 26 layers regardless of size — item 2's defect,
   found here first. Proceeding to `--dry-run=false` anyway (the estimate
   being wrong is not the same as the layer being free), every layer whose
   whole corpus fit inside one run's default `max_tokens_per_run` compiled
   cleanly; the rest split into two failure shapes: some stopped
   mid-corpus as `budget_stopped` (a corpus a little over budget, making
   real but partial progress), others refused outright at the pre-flight
   gate (a corpus so far over budget that even the first section's
   estimate alone exceeded it) — item 3's defect, found here second, on
   the exact CV-41-activated layers (`adr-rules`/`spec-bindings`) this
   wave's own default-set change was supposed to let compile.
2. **Items 2 and 3 fixed** (this same wave, this same session): the
   dry-run price table (`cmd/fathom/modelprice.go`) and the NoPriorRun
   pre-flight bypass (`internal/semantic.CompileInput.NoPriorRun`,
   `pkg/client`'s compile_runs-history check) — see their own plan rows and
   `CHANGELOG.md` for what each fixed and how each was tested. `bin/fathom`
   rebuilt from this branch before continuing.
3. **Second pass ($4.60).** Resumed the 3 layers pass 1 left
   `budget_stopped` (`fathom compile --continue`, ordinary digest-gated
   resume — no override needed, exactly the "resumable with --continue, as
   today" item 3 promises) and forced the 5 layers pass 1's pre-flight
   gate had refused outright, now via `--max-tokens-per-run all` (the
   wave O4 item 2 override, unrelated to this wave's own NoPriorRun fix —
   an explicit, logged escape rather than a silent default change) so each
   of those 5 whole, oversized corpora compiled completely in one call.
   This pass is the most expensive of the three: the 5 forced whole-corpus
   layers are exactly the fleet's biggest (`fathom spec-bindings` at 622
   claims, `app-kit adr-rules` at 323, `throughline adr-rules` at 242,
   `fathom adr-rules` at 227, `platform-engineering spec-bindings` at 429 —
   the five largest rows in the table below).
4. **Third pass ($0.75).** The 2 layers still incomplete after pass 2 (a
   partial resume can itself budget-stop again on a large enough remaining
   tail) finished with a further `--continue`.
5. **Total: 26 layer compiles, 2,798 claims, $7.05**, the production
   compile model (`claude-haiku-4-5-20251001`) throughout, no dry-run
   spend counted (dry runs are never persisted to `compile_runs`,
   `internal/semantic.Compile`'s own "never persisted" DryRun branch — the
   $7.05 is real spend only).

## Per-layer results

| Service | Layer | Claims | Cost |
|---|---|---:|---:|
| agentic-kit | obligations | 11 | $0.015 |
| agentic-kit | spec-bindings | 177 | $0.839 |
| app-kit | adr-rules | 323 | $0.958 |
| app-kit | obligations | 8 | $0.012 |
| app-kit | spec-bindings | 39 | $0.060 |
| car-assistant | obligations | 3 | $0.008 |
| conjure | obligations | 19 | $0.020 |
| conjure | spec-bindings | 95 | $0.113 |
| cue-wasm | adr-rules | 6 | $0.021 |
| fathom | adr-rules | 227 | $0.690 |
| fathom | obligations | 188 | $0.161 |
| fathom | spec-bindings | 622 | $0.866 |
| geoffmiller-cloud | adr-rules | 42 | $0.956 |
| geoffmiller-cloud | obligations | 12 | $0.015 |
| geoffmiller-cloud | spec-bindings | 24 | $0.508 |
| growing-gardens-bi | adr-rules | 7 | $0.035 |
| growing-gardens-bi | obligations | 3 | $0.003 |
| imbue | adr-rules | 6 | $0.116 |
| imbue | obligations | 5 | $0.074 |
| platform-engineering | obligations | 107 | $0.088 |
| platform-engineering | spec-bindings | 429 | $0.528 |
| resume_as_code | obligations | 1 | $0.008 |
| resume_as_code | spec-bindings | 177 | $0.118 |
| study | obligations | 22 | $0.018 |
| throughline | adr-rules | 242 | $0.805 |
| throughline | obligations | 3 | $0.015 |
| **total** | | **2798** | **$7.05** |

26 rows, 12 services, every row cross-checked against the sum: claims
total exactly 2,798, cost totals exactly $7.05 (rounding each row to
thousandths, as billed).

## What this shows

The fleet recompile is what this wave's two code fixes were FOR, not a
formality after them: CV-41's default-set change (`adr-rules`/
`spec-bindings` activate wherever their corpus matches) was correct in
isolation but inert in practice until (a) an operator could trust the
dry-run price enough to decide whether to spend at all, and (b) a first
compile of one of those newly-activated, often-larger layers could
actually start on a real repository without already knowing to reach for
an override. Both gaps were found by running the real, fleet-wide program
this row asks for, not by inspection — the same discipline CV-M6 (the
dogfood refactor program) already established for this ADR's own refactor
verbs.

## Reproduce

```sh
for svc in agentic-kit app-kit car-assistant conjure cue-wasm fathom \
           geoffmiller-cloud growing-gardens-bi imbue platform-engineering \
           resume_as_code study throughline; do
  bin/fathom compile --service "$svc" --dry-run          # price first
  bin/fathom compile --service "$svc" --dry-run=false     # spend
done
# Resume any budget_stopped layer:
bin/fathom compile --service <svc> --layer <layer> --dry-run=false --continue
# Force a pre-flight-refused whole-corpus layer through in one call:
bin/fathom compile --service <svc> --layer <layer> --dry-run=false --max-tokens-per-run all
```

Measured 2026-09-25, `bin/fathom` built at this session's `work` branch
HEAD (post items 2 and 3), against the real fleet services named above —
never the shared fleet store (each service's own store, per
`docs/plans/README.md`'s ordinary compile path).

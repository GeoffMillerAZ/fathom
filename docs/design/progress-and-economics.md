# Progress and economics: views over computed state, and cost per proven outcome

Status: accepted 2026-09-27 (ADR-049 decisions 4 and 5). Vision:
`docs/vision/38-proof-layer-roadmap.md`. Plan: `docs/plans/progress-and-economics.md` (PE-).
Goal G22. U1's extensions and U12 live in `docs/design/work-intelligence.md`. Constraints
carried in: the "no dispatch loop / no tracker" non-goal (refined 2026-09-27), the "no generic
token-usage dashboard" non-goal, ADR-045 (every console panel renders a verb; one projection
per metric with truth states), law 12 and G2 (admissibility), I38 (blindness is never a
healthy zero), I58 (no ranking of people), P22 (measured, not argued).

## 1. Problem

Plans carry hundreds of gated rows and the verify ledger holds about 17,000 readings, but the
console shows the newest state per plan only. Nobody can see weighted progress, a burn-up
that is computed rather than claimed, which proven rows stopped being proven, or what a
proven row cost. The owner asked for progress views and a usage dashboard; the market has
usage dashboards and trackers, and neither can say whether the work is really done.

## 2. Decisions

1. Status is computed by `fathom verify`; every view is read-only. No drag, click or
   assignment writes a plan or a status.
2. Proof history is a projection of the existing verify ledger. Its rows gain per-row
   verdicts additively (schema 3); no new ledger file.
3. Row `weight:` (spec interop §4) weights every rollup; default 1. Authored weights are never
   overwritten; a deterministic estimate is proposed beside them (section 10).
4. Economics joins the existing session ledger (3,359 sessions) and the Stop hook's
   incremental transcript fold to proof. Only cost per proven outcome is shown; raw usage is
   the clients' job.
5. Every number on these pages traces to an instrument with a truth state (ADR-045).

## 3. The proof history (verify ledger, schema 3)

Today a row of `~/.fathom/verify-ledger.jsonl` is one plan's counts at one run (schema 2:
total, claimed, verified, failing, commit, provenance, origin). Schema 3 adds an optional
`rows` array: `{id, state, criterion, weight}` per row, where `state` is `open`,
`claimed-proven`, `claimed-unproven`, `unknown` or `dispositioned`. v1 and v2 rows still
parse and contribute plan-level counts only; nothing is backfilled.

Size bound: a `rows` array is written only when the plan's row digest changed since the
previous reading for that plan (a digest field on every row), so a pre-push gate re-run on an
unchanged tree appends counts without the array. `fathom gc` already rotates the ledger.

## 4. The views

All on the console under Progress, each backed by a CLI verb with `--json` (law 10):
`fathom plans status` grows `--weighted`, `--history` and `--since`.

| View | Computed from | Notes |
|---|---|---|
| Weighted table | the newest reading per plan, rows weighted | fleet, then project, plan, section; proven weight over total weight |
| Derived kanban | row states | columns Open, Claimed-unproven, Proven, Refused or deferred; read-only; cards link to the row and its failing criterion |
| Burn-up with projected finish | proof history per day | projection is a linear fit over the last 14 days of proven weight, shown with its interval; hidden when fewer than 7 days of data |
| Frontier | `plans next --eligible-only` | rows whose `needs:` are satisfied |
| Proof regressions | proof history | rows once `claimed-proven` that now read otherwise, with the first reading that changed and its commit (U12 generalizes this) |
| Time-to-proof | proof history | first appearance of a row id to its first `claimed-proven` reading |
| Claim honesty | proof history plus the session ledger | claimed-unproven rate per agent identity (model, harness), never per person (I58) |
| Heatmap | proof history | proven weight by plan and week |
| Goal gauges | scope bets and their plans' measurement rows | a gauge per G-goal: its measurement rows proven over total, and the review date |
| Fun, true | proof history | streaks of days with a proven row, "proved today", each a count, never a score |

Filters on every view: service, plan, wave (a plan section label), goal, agent identity, date
range.

## 5. Outcome economics (N3)

**Inputs.** The session ledger (sessions, harness, model, timings), the Stop hook's
transcript fold (tokens in and out, cache reads and writes, tool calls), Codex session files
read by the same fold on the harness port, the call log (fathom calls and their receipts'
bytes), and proof history. Transcripts are read incrementally from the stored offset
(`cmd/fathom/transcript_offset.go`), within the hook's deadline; nothing new is captured.

**The join.** A session's proven outcome is the set of rows that moved to `claimed-proven` in
a verify reading whose commit the session produced (commit authored during the session's
window in its working tree), plus verify passes it ran. Cost is tokens priced by a
user-editable price table (`~/.fathom/prices.toml`, no network), wall time, and fathom calls.

**Views.** Live session feed (read-only, the SSE channel ADR-045 built); tokens and cost per
proven row, per plan and per language; tool-call mix; cache hit rate; fathom-on versus
fathom-off shown only when G2's admissibility test passes for the compared units, otherwise
the panel says why it is withheld.

**Cost of every answer.** Every receipt already carries bytes; it gains an estimated token
count (`internal/cost.Estimate`, four characters per token, named as an estimate) and the
console shows it per answer.

## 6. What is refused

- Kanban write-back, status set by a drag or click, assignment, due dates.
- A generic usage dashboard: tokens per day, per model, spend alerts, quotas.
- Any per-person figure (I58).
- A projected finish with fewer than 7 days of data, or without its interval.
- A second ledger file for proof history.

## 7. Tenancy and cost

Views read the ledgers through bounded tail reads (the newest N readings per plan, N set by
the view's range) and cache projections in `memo` groups keyed by ledger size (I51). The
transcript fold is incremental and already inside the Stop hook budget. No new table in the
store.

## 8. Measurement

| Reading | Threshold |
|---|---|
| PE-M1 | every number on the Progress and Economics pages traces to an instrument with a truth state; an audit lists each panel, its verb and its source |
| PE-M2 | the ledger reconstructs burn-up and proof regressions for 30 consecutive days on this repository, matching `plans status` at each day's last reading |
| PE-M3 | tokens per proven row joined for Claude Code and Codex sessions, with the unjoined share named |
| PE-M4 | the pages measured like G13's UX-M1 (task completion by a first-time reader), PASS |
| PE-M5 | the ledger-pollution reading (section 11): rows under unregistered roots counted, and burn-up, cost per proven row and claim honesty recomputed with and without them |

## 9. Jev, weighed

- **Where it could help:** none on these views; they are counts.
- **Where it cannot:** projections, attribution of cost to rows, honesty rates.
- **Cost:** not applicable.
- **The measurement:** none planned. Task preflight (U6) is the economics-adjacent Jev site,
  in work intelligence.

## 10. Weights and cost per weight point (accepted 2026-09-29, ADR-056 decision 10)

The rows live in `docs/plans/work-signals.md` (WS-8 to WS-11, WS-M3) with stall detection and the preflight
baseline; this design owns how they land on these pages (PE-22). Everything here is deterministic and keyed by
agent identity, never a person (I58).

**What exists on main.** The grammar's `weight:` (a positive integer, default 1, never gating, SI-7), the
`plans status --weighted` rollups by fleet, project, plan and section (PE-3), cost per proven row per plan
and language (PE-16) and the session-to-proof join (PE-15).

**The gap.** Almost every row is weight 1, so the weighted percentage equals the unweighted one and cost per
row hides that a large row and a trivial row cost differently.

**Proposed weights.** A deterministic estimator from row features: anchor type, planned files and the blast
radius of the anchor's package, the rows it depends on, and the historical effort of similar proven rows
from the U6 feature index (WI-15). It is shown as "estimated weight" beside any authored weight and never
overwrites it; the author accepts it into the plan with one command (WS-9). The U6 difficulty Score (WI-18)
may refine an estimate only after it passes calibration against a fitted baseline (WS-M4); with provider
`none` the deterministic estimate stands, and a refinement is a proposal for the author, so progress computed
by `fathom verify` stays model-free (ADR-049 decision 7).

**Calibrated against actual effort.** Estimates are compared with the tokens, turns and wall time of each
proven row (the join), and the error is published (WS-M3); an estimator that does not beat "every row
weighs 1" on held-out rows is not shown as a progress figure.

**Economics by weight.** Cost per proven WEIGHT POINT, not only per row, on the Progress and Economics
pages, per model and harness (feeding U5's model scorecard) and per zone (U2's interest), estimated weights
marked as estimates and authored ones as authored (PE-22). The same view is where a cheaper model that cost
more per proven weight point than a stronger one becomes visible (WS-5).

## 11. Ledger hygiene: registered roots only (2026-09-29, observation O121)

**Found.** Scratch-clone runs of `fathom verify` and `fathom intent audit` append rows to the user-level
ledgers keyed by the scratch path. On 2026-09-28 the verify ledger held 19,639 rows, 13,514 of them under
`/private/tmp` style roots (7,729 from one scratch clone), and the intent ledger 91 of 180. Every view that
reads a ledger without a root filter (proof history, burn-up, the session join's unjoined share, cost per
proven row, the verify-gate figure, the research report) counts them, which distorts proof history and
economics.

**Rules.** (1) Readers ask one function for the registered service roots (PE-23) and read only rows under
them, naming how many rows they ignored (PE-24, PE-25); the file itself is never rewritten by a reader.
(2) A documented `FATHOM_HOME` relocates the whole `~/.fathom`, and `--ledger <path>` names one ledger file;
the per-file variables already in the code (`FATHOM_VERIFY_LEDGER`, `FATHOM_INTENT_LEDGER`,
`FATHOM_SESSION_LEDGER`, `FATHOM_HOOKS_LOG`) keep winning over it; test and scratch runs use them (PE-26 to
PE-28). (3) `fathom gc` gains the class `ledger-unregistered`: the dry run counts rows and bytes per
unregistered root, and `--apply`, run by the owner, archives them under `~/.fathom/archive/` in the owner's
2026-09-29 layout: `<ledger>-unregistered-<date>.jsonl`, plain JSONL with rows byte-identical to the originals
(appended when the day's file exists), `SHA256SUMS` updated and one row added to the `README.md` table; restore
is `cat <archive> >> <ledger>`. Rows are moved, never deleted, the live file is swapped atomically carrying any
row appended meanwhile, and neither the daemon's daily pass nor doctor plans or applies the class (PE-29). This
differs from the existing `ledgers` class, which keeps a ledger whole because its readers read only the live
file: unregistered-root rows are not history of registered work.

**Why not delete or rewrite.** The ledgers are evidence (ADR-047 decision 13: evidence is archived, never
deleted), and a root that is unregistered today may be registered tomorrow; the filter is on read, the
archive is reversible.

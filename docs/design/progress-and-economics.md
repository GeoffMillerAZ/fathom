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
3. Row `weight:` (spec interop §4) weights every rollup; default 1.
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

## 9. Jev, weighed

- **Where it could help:** none on these views; they are counts.
- **Where it cannot:** projections, attribution of cost to rows, honesty rates.
- **Cost:** not applicable.
- **The measurement:** none planned. Task preflight (U6) is the economics-adjacent Jev site,
  in work intelligence.

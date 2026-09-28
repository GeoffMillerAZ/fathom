# Work intelligence: agent effort, friction and cost joined to code and proof

Status: accepted 2026-09-27 (ADR-051 decisions 3 and 6). Vision:
`docs/vision/38-proof-layer-roadmap.md` §4. Plan: `docs/plans/work-intelligence.md` (WI-).
Goal G24. Builds on `docs/design/progress-and-economics.md` (proof history as a projection of
the verify ledger, the session ledger join, the price table). Constraints carried in: I58 (no
ranking of people), law 12 and G2 (causal claims only where the A/B is admissible), ADR-042
and I42 (the decision port, default none), ADR-045 (every number from an instrument with a
truth state), the dispatch non-goal (fathom recommends; the harness decides), ADR-041
decision 8 (no prose inference over transcripts).

## 1. Problem

Teams now pay for most code in agent tokens and minutes. Nobody can say where that money goes
by part of the codebase, where agents struggle and why, which model does which work well on
this code, or how hard a task is before it starts. Fathom already records the agent activity
(call log, hook log, session ledger, transcripts through the Stop fold, discoveries, threads)
and the proof; the products below are joins of the two.

## 2. Zones

A **zone** is the unit every view aggregates to: by default a Go package or, for other
languages, the deepest directory holding indexed symbols; `.fathom/config.toml` `[zones]` may
name coarser zones by path globs. A change belongs to the zones of the files it touched,
split evenly across them.

## 3. The products

### U1. Cost per proven outcome, extended

Progress and economics computes tokens, cost and time per proven row per plan and language.
Work intelligence adds: per zone; CI minutes, when a CI summary file the user names (JUnit
`time` totals, or a workflow-run JSON export) is ingested and bound to a commit; and the split
between first-attempt proofs and proofs after claimed-unproven retries.

### U2. The technical-debt interest rate

Measured, not estimated. For each proven change (a commit that moved at least one row to
proven, or a verify-passing change on a branch), effort is: tokens, turns, wall time, rework
(lines changed again within 7 days), reverted edits (hook-observed edits undone in the same
session), and claimed-unproven retries. Per zone, the **interest rate** is the zone's median
effort per proven change divided by the fleet median. A zone at 3.0 costs three times the
typical change; the figure is shown with n, the interquartile range, and the model mix of
the sessions behind it (a zone worked mostly by a small model is named as such, not blamed).

### U3. The agent friction map, with remedies

Per file, symbol and zone, counted from the activity record:

| Signal | Source |
|---|---|
| rediscovery | a symbol found by search in a session after a discovery pointer to it existed |
| blocked edits | PreToolUse denies and edit-hook violations (the governance signal source, GL-10) |
| claimed-unproven | proof history |
| reverts | hook-observed edits undone in the same session |
| search loops | at least 5 consecutive search or lookup calls without an edit or answer |

Each hotspot gets a computed remedy, shown as a proposal with its evidence, never applied:

| Pattern | Proposed remedy |
|---|---|
| repeated blocks on one rule | the rule is unclear: show its text and the blocked edits |
| rediscovery of one symbol | add an anchor or a discovery pointer |
| violations of a served rule | the rule is ignored: move it to a hook (U4's class) |
| a doc span flagged possibly stale near the hotspot | refresh the doc (G20's drift) |
| search loops ending in a symbol with no references | an unexported or dynamically dispatched symbol: add an anchor or a governing note |

### U5. The model and harness scorecard

Per (model, harness, effort setting): proven rows, claimed-unproven rate (honesty), tokens and
cost per proven row, rework, and n. Descriptive only. Every cell shows n and the confounders
that differ between cells (zone mix, row weight mix, time period); cells below n = 10 are
greyed. A causal sentence ("model A is cheaper for this work") appears only when G2's
admissibility test passes for the compared units (law 12); otherwise the page says the
comparison is descriptive. Rows are agent identities, never people (I58).

### U6. Task preflight and right-sizing (the Jev site)

Before work starts on a plan row or a task, a **difficulty profile** from deterministic
features: blast radius of the row's anchor, governing rules touched and their levels,
certificates to re-prove, the zone's interest rate (U2), and past effort on rows with similar
features (nearest neighbours over the feature vector, k = 5). Output: the profile, a
deterministic tier recommendation from a published rule table, and, when `[decide]` is
configured, the decision port's Score over the same features with its calibration. The
recommendation is advisory: printed by `fathom plans context --row <id>` and shown on the
console; the harness decides what to run. The outcome (model used, effort, proven or not) is
recorded to calibrate.

**Jev, weighed.** Where it helps: a Score over mixed features is the port's shape, and this is
the best new site on the roadmap. Where it does not: it never dispatches, never picks for the
harness, never gates. Cost: one call per preflight, only when asked. The measurement (WI-M4):
an A/B at n at least 3 per arm over at least 30 rows, Score versus the deterministic rule
table, judged by cost per proven row and first-attempt proof rate. Drop the site if null or
negative.

**The profile and the rule table (WI-16, WI-17).** `internal/console.DifficultyProfile` joins
what fathom already stores: the anchor's transitive callers (bounded walk, a capped count is a
lower bound), the rules governing its files by level (`error`, `warn`, `info`, the change risk
receipt's vocabulary), the certificates a change would re-prove, each of the row's zones' U2
interest rate (a zone needs at least 3 proven changes to have one), the row's own
claimed-unproven readings and plan weight, and the effort of the k = 5 nearest proven rows.
"Nearest" is over the facts a past change also has: its zone count and overlap with the row's
zones, its claimed-unproven retries and its weight. Blast radius, rules and certificates are not
stored per past change, so they are not part of the distance, and the profile says so. Every
part carries a truth state; an unknown fact is absent with its reason, never zero.

The tier recommendation is the highest floor among the rules of this table that fire (revision
1, `TierRuleTableVersion`). The tiers are relative to what the harness offers: `light` is its
smallest, fastest model and lowest effort setting, `heavy` its largest and highest. Rules on one
fact are banded, so at most one per fact fires. An unknown fact never lowers a tier and never
certifies a light one: with rules unevaluated, a fired tier is "at least" and no fired rule
means no recommendation.

| Rule | When | Floor |
|---|---|---|
| T1 | more than 50 transitive callers of the anchor | heavy |
| T2 | 11 to 50 transitive callers of the anchor | standard |
| T3 | a governing rule at level error | heavy |
| T4 | a governing rule at level warn and none at level error | standard |
| T5 | 5 or more certificates to re-prove | heavy |
| T6 | 1 to 4 certificates to re-prove | standard |
| T7 | a zone's interest rate of 2.0 or more (a zone needs at least 3 proven changes to have one) | heavy |
| T8 | a zone's interest rate from 1.2 up to 2.0 | standard |
| T9 | 2 or more claimed-unproven readings of the row so far | heavy |
| T10 | 1 claimed-unproven reading of the row so far | standard |
| T11 | a plan row weight of 5 or more | standard |
| T12 | the 5 nearest proven rows' median tokens at least 2.0x the fleet median (needs 3 such rows) | standard |
| T13 | fewer than 50% of the 5 nearest proven rows proven at the first attempt (needs 3 such rows) | standard |

### U12. Proof regressions, generalized

Anything once proven that no longer holds, with the reading and commit where it changed:
plan rows (proof history), certificates (live to flaky, broken or unproven), conservation
(a plane that read `conserved` and now does not), and invariant enforcement (an invariant's
enforcing test that stopped reaching or passing). One list, newest first; each entry links the
breaking commit's diff.

### U13. Hotspots with context

A view over T1's stored factors plus bus factor: per symbol, size (the L1 range as a named
proxy for complexity; the cognitive analyzer stays retired), churn, centrality, uncovered
share, and the number of distinct commit authors in 365 days shown as a count, never with
names ranked (a single-author file is flagged as a bus-factor risk, not a person).

## 4. Tenancy and cost

Every product is a projection over ledgers and stored facts, computed in the background after
ingest or on a bounded tail read, cached in `memo` groups (I51). No new capture: the hook log,
session ledger and transcript fold already exist, and the rework and revert signals are
derived from hook events already recorded. Nearest-neighbour search for U6 runs over at most
the last 5,000 proven rows.

## 5. Measurement

| Reading | Threshold |
|---|---|
| WI-M1 | U1 to U6 on the console, every number traced to an instrument with a truth state |
| WI-M2 | U2's interest rate on at least 3 repositories with at least 30 proven rows each; the top zone at least 1.2x the median, or the non-separation recorded |
| WI-M3 | the friction map's top 10 hotspots on this repository with remedies, checked by a human (at least 7 judged useful) |
| WI-M4 | U6's A/B read: keep or drop the Jev site |
| WI-M5 | U12 over 30 days: every regression found by the list matches a real state change, none missed on a seeded set of 10 |

## 6. Refused

- A productivity score, leaderboard or any view keyed by a person (I58).
- Predictive estimates without a calibrated reading.
- Auto-applied remedies.
- Dispatch: preflight never starts, assigns or routes work.
- Reading transcript prose to infer intent (ADR-041 decision 8); only counts and tool events.
- A causal claim outside G2's admissibility.

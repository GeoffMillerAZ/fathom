# Realization grade — from "is it wired" to "is it working"

> Phase Y of `docs/vision/20-ambient-fathom.md`. Answers the owner's asks
> for *"deeper analytics and dashboards and observability from both the
> fleet and individual project perspectives"* and *"the fleet dashboard can
> keep track of which projects are using it optimally."*
>
> **Status:** design. Extends `wiringGrade`
> (`cmd/fathom/observatory.go:591`) rather than replacing it.

---

## 1. What the current grade measures, and what it misses

`wiringGrade` folds four signals — hooks referenced, CLAUDE.md stanza
present, index fresh, any traffic recorded — into A–F plus the one command
that raises the letter. It is good, it is honest about invisible signals
(it drops them from the denominator rather than scoring them zero), and it
has been the most-used thing on the fleet page.

It measures **configuration**. Three of its four signals are "a file says
the right thing"; the fourth is "at least one call happened, ever."

That is the wrong ceiling. A repository can score A and deliver nothing:
wired perfectly, one call in July, no obligations firing, no gate running.
Grading configuration rewards the act of configuring, which is exactly the
behaviour a tool should *not* optimize for.

---

## 2. Five dimensions

The grade becomes a fold over five, in dependency order. Each is a
precondition for the next, which is what makes a single letter meaningful
rather than an average of unrelated things.

| # | Dimension | Question | Signal (all already collected) |
|---|---|---|---|
| 1 | **Wired** | can fathom operate here? | hooks + stanza + registration (today's grade) |
| 2 | **Fresh** | are answers current? | indexed commit vs repo HEAD |
| 3 | **Queried** | is anyone asking? | calls in the window, not calls ever |
| 4 | **Preventing** | is it acting before edits? | obligation injections in the window |
| 5 | **Gating** | is it catching claims? | verify runs in the window |

**Dimension 3 changes from "ever" to "in the window."** A repository that
was busy in July and silent since is not currently realizing value, and the
grade should say so. The current `CallsTotal > 0` test can never go down,
which makes it useless as a health signal.

**Dimensions 4 and 5 are new and are the point.** They are the only two
that measure fathom *doing something* rather than *being available*. Both
are already counted per-service in the call log — this is a fold over data
that exists, not new instrumentation.

---

## 3. Grading

Each dimension resolves to `yes` / `no` / `not-applicable`, and
not-applicable leaves the denominator, exactly as `wiringGrade` already
does for invisible hooks. A repository with no plans has nothing for the
gate to check, so gating is not-applicable and it is not punished for it.

The letter is the same fraction fold, over five dimensions instead of four.
Two additions make it honest:

**The ladder is ordered, so the letter names the first broken rung.** A
repository that is wired, fresh and queried but not preventing gets
`B — not preventing`, not a bare `B`. The letter alone was never enough to
act on; the label is what makes it a to-do.

**A repository that has never had a session in its window is graded
`— idle`, not `F`.** An indexed reference corpus that nobody codes in is
working exactly as intended and must not appear as a failure. This is the
same distinction the index-only honesty work made for the fleet tiles, and
getting it wrong would make the fleet page cry wolf on nine repositories
that are fine.

---

## 4. Realized vs. available

The fleet page's ranking question is not "who has the best letter" but
**"where is the most unrealized value"** — which is a different order, and
the one that tells you where to spend the next ten minutes.

For each repository: dimensions passed, against dimensions *available*
(total minus not-applicable). A large, busy repository failing two
dimensions outranks a tiny one failing three, because the gap is worth
more. Ranking is by `(dimensions_unrealized × session_count_in_window)`,
descending — a deterministic key, no weighting knobs, no tuning.

The fleet page's default sort becomes this. The current sort (by call
count) answers "who is busy", which nobody needs to be told.

---

## 5. Per-project depth

`/observatory/service` today shows the adoption gauge, hook heartbeat and
freshness. It becomes the page that answers *"what is fathom doing here,
and what is it missing here"* — the project-level analogue of the value
brief, and it reuses the brief's structure deliberately so the two read
alike:

**Realization ladder** — the five dimensions as a vertical ladder, each
green/amber/grey with its evidence inline (*"preventing: 412 injections
across 23 sessions"*), and the first broken rung carrying its remedy —
which, after phase U, is a button rather than a command string.

**Activity over time** — a 30-day sparkline per dimension, not just total
calls. The interesting shape is a repository where *queried* holds steady
while *preventing* falls off: that is hooks silently breaking, and it is
invisible on a single-series chart.

**Tool profile** — which tools this repository's sessions actually use,
against the fleet median. A repository using only `find_symbol` when the
fleet median uses six tools is under-adopting, and that is a coachable,
specific finding rather than a vague "use it more."

**Cost here** — phase X's accounting, scoped to this repository: tokens
fathom put into contexts, against discovery actions served. Observational,
never captioned as saving (law 12).

**What is unmeasured here** — the brief's work order, scoped to one
repository. *"No verify runs — nothing to catch claims. No exploration
ledger rows — per-question cost unknown here."* Each with the one action
that changes it, or an explicit "nothing to run."

**Scope** — the suppression and purge panel from
`docs/design/scope-and-purge.md` §4.

---

## 6. Fleet depth

The fleet page gains three things and loses none.

**Grade distribution over time** — a stacked area of A/B/C/D/F across 30
days. This is the *only* chart that shows whether the self-healing work
(phase U) is doing anything, and it is the honest scoreboard for the whole
program: if this does not improve, the buttons did not work.

**Time-to-remedy** — median days from a gap first appearing to it closing.
Pre-registered in `self-healing-fleet.md` §7 as that phase's own success
criterion; this is where it is displayed.

**Cohort comparison** — dimensions passed, by repository, as a small
multiple. Reading across it answers "is this repository unusual, or is
this normal here" in one glance, which no single-repository page can.

---

## 7. Charting discipline

The observatory is an app-kit app (CLAUDE.md, binding). Charts come from
`appkit/ui/viz` — Sparkline, Gauge, Bars — never hand-rolled SVG; this repo
already deleted one hand-rolled primitive set the same day it was written.
Colour is role utilities only, so light and dark follow the kit tokens.

Three rules specific to this data, from the house dataviz discipline:

- **One axis, always.** Sessions and tokens are different scales and never
  share a chart. Two charts, or index both to a common base.
- **Absence is drawn as absence.** A dimension with no data renders as
  `stat.Absent` with its reason, never as a zero-height bar — a zero bar
  reads as "measured, and it is zero", which is a different and false
  claim.
- **Colour follows the entity, not its rank.** Filtering the fleet must not
  repaint the survivors.

---

## 8. Test obligations

- Each dimension resolves independently, with not-applicable leaving the
  denominator; proven by a table test over the 2^5 combinations that
  matter.
- A repository with no sessions in the window grades `idle`, never `F`.
- The letter carries the first broken rung in ladder order.
- Ranking by unrealized value is deterministic and stable under equal keys.
- The 30-day series is built from real call-log rows and renders `Absent`
  with a reason for days with no data — never an interpolated line.
- Grade computation is pure over a service row plus a window: same inputs,
  same letter, no clock reads outside the injected `now`.
- The service page renders with **every** instrument absent and produces no
  fabricated zeros (the law-3 regression, and the one most likely to
  regress here).

---

## 9. Explicitly out of scope

- Projections: "raising this to A would save N tokens" (vision 20 X2).
- Cross-machine or cross-user comparison (law 13).
- Any grade that improves by adding configuration without changing
  behaviour — that is the defect this phase exists to fix.

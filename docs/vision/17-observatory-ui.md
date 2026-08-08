# Vision 17 — The Observatory UI: value realization at a glance

> Owner directive 2026-08-08: "push for the next level of excellence and
> observability… help the UI in fathom call out the wins… as honest as we
> can but fathom can help sell itself to orgs by showing its value…
> better flywheels, better feedback… massively upgraded visualizations."
> IO-28's trigger fired (fleet report + narrate landed with real
> artifacts); this vision is the screen layer over those verbs, and
> docs/plans/observatory.md is its binding promotion.

## 1. The stance: honest is the pitch

Fathom sells itself the way it verifies plans: no number without its
instrument, no claim without its denominator. The observatory NEVER
invents a metric — a missing instrument renders as a named absence with
the exact command that would light it up (law 3). A token-savings figure
appears ONLY as a counterfactual range with arm counts (law 12). Story
rows appear under an explicit "narrative is a lens, not evidence" banner
(law 14). Nothing here crosses a network: pages are loopback, the brief
is a local file a HUMAN chooses to send (law 13). That discipline is the
demo: an org that sees honest absences trusts the numbers that are
present.

## 2. Architecture: statusz's pattern, grown up

Daemon-served on serve-http (loopback, allowlist-gated in hosted mode),
pure Go, zero dependencies, no template engine, no JavaScript required
for correctness. One model — BuildObservatoryModel — renders to JSON
(?format=json) and HTML from the same value, exactly like /statusz. The
model reuses the fleet-report builders (fleetServiceRow), the metrics
rollups, the session ledger, arm/narrate state, and the observations
backlog; a short in-process cache keeps page loads snappy without a
refresh daemon. Visuals are app-kit components (owner
redirect 2026-08-08): dashboard widgets, stat bands with stat.Absent for
uninstrumented metrics, viz.Sparkline/Gauge — kit tokens carry light/dark
with no JavaScript; CSS builds via the checksum-pinned Tailwind binary
(task ui:css). Cool at a glance, still printable, air-gap safe.

## 3. The three screens and the verb

- **/observatory** — the fleet: a tile per service (wiring, freshness,
  traffic, open observations), fleet totals, arm + narrate banners.
  Every warning names its remedy command — the page is a to-do list,
  not a mood board.
- **/observatory/service?id=S** — one repo: adoption gauge (surface
  split, top tools), hook heartbeat by template version, plan/verify
  state, freshness with the exact re-ingest command.
- **/observatory/impact** — the wins page: verify catches, discovery
  substitutions, obligations served (each with date range and
  denominator); the counterfactual pilot as a STATE MACHINE
  (pre-registered → collecting N/10 per arm → range published, or
  honestly lapsed); story-row highlights under the lens banner while
  narrate is active.
- **`fathom brief`** — renders the same impact model to a dated local
  markdown brief for humans to share deliberately: real numbers,
  version stamps, an explicit "not measured" section. The org pitch IS
  the honesty.

## 4. The flywheel, made visible

The feedback loop that improves fathom is the observations backlog; the
observatory surfaces it (open/closed per repo, latest filed) so friction
begets fixes visibly. Wiring drift (reconcile's diff) and index
staleness render with their one-command remedies — the UI keeps the
fleet wired BY making unwiredness embarrassing at a glance.

## 5. Deliberately not now

Live-updating JS and websockets (a meta-refresh is enough until it
isn't); cross-org/hosted sharing of any page (law 13 — sharing stays a
human verb); chart libraries (SVG primitives carry rung-1 needs);
per-screen auth (hosted-mode allowlist already gates loopback surfaces).

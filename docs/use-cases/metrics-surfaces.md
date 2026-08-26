# Observability Surfaces: Spend, Traffic, Channels, Interventions

Use fathom's `metrics` verb family to turn its own local instrumentation
(the call log, the hooks beat log, `compile_runs`, `claim_events`) into
queryable, time-ranged reports — cost, traffic, channel liveness, and
mechanism-labeled interventions — plus a daemon status page at
`/statusz`.

---

## When to use this

- You want to know what a compile layer actually **costs** — runs, tokens,
  USD, cost per claim — and how close it's running to its configured
  `max_usd_per_run` budget.
- You want traffic shape over time: call volume, p50/p95 latency, error
  rate, and how much of it is fathom's own index tools versus plumbing
  versus everything else.
- You want to know whether the forced channels (obligations injection,
  guarded pre-edit warnings) are actually firing in this window, not just
  wired up.
- You are about to write a claim like "fathom refused N unproven-done
  checkoffs this week" and want the number backed by a mechanism, not a
  guess — see `docs/design/observability-surfaces.md` §1 for why "mistakes
  prevented" was rejected in favor of mechanism-labeled Interventions.
- You want a graph over 30/90 days without rescanning a growing JSONL every
  time — daily rollups make that possible.
- You want a quick daemon health check from a browser: `/statusz` on the
  existing `serve-http` listener.

This is a **CLI-only** feature (plus the one HTTP status page) — no new MCP
tool exists for any of it (law 10, "verb-first surfaces": fathom-studio,
not fathom, gets the interactive screens — see
`docs/vision/13-expert-suite.md` §9 row Q).

---

## Tools

| Command | Kind | What it gives you |
|---|---|---|
| `fathom metrics spend` | CLI | `compile_runs`-derived: runs, tokens in/out, USD, cost per claim, budget headroom vs. each layer's `max_usd_per_run`, escape-hatch (`--force`/`--unsafe-skip-eval-gate`) runs flagged. Grouped `--by layer\|day\|model`. Requires `--store` and `--service`. |
| `fathom metrics traffic` | CLI | Call-log-derived: volume, p50/p95 latency, error rate, and a discovery/plumbing/other split. Grouped `--by surface\|tool\|hour`. |
| `fathom metrics channels` | CLI | Hooks-beat-log-derived: injection coverage, guarded pre/post seen/served, dedup suppression, module-tagged rule serving — phase P's "not measurable" wording carried through verbatim where it applies. |
| `fathom metrics interventions` | CLI | The design's own five-row table, each row naming its mechanism: unproven-done refusals, review findings superseded, non-conforming artifacts refused (always "not measurable" today — no code path persists this event yet), obligations served, guarded pre-warns. `--store` is optional — only the review-superseded row needs it. |
| `fathom metrics series <metric>` | CLI | A time series for one named metric (see the list below), `--bucket day` (rollups plus today's live tail) or `--bucket hour` (computed entirely from live raw logs — no persisted hourly rollup exists). |
| `fathom metrics rollup` | CLI | Builds one date's daily rollup file at `workspace/artifacts/metrics/daily/YYYY-MM-DD.json` — deterministic, overwrites idempotently for the same date, never deletes another date's file. |
| `fathom metrics rotate` | CLI | The call log's retention verb: dry-run by default (reports what would move); `--apply` archives lines older than `--keep-days` to a dated sibling file — **never deletes**, per the design's "never delete raw data implicitly" rule. |
| `GET /statusz` | HTTP | Daemon health, per-service index freshness, channel liveness, last compile per layer. JSON via `Accept: application/json` or `?format=json`; HTML otherwise. Loopback-only; gated inside the `/rpc/modules` bearer-token allowlist when `--modules-token-env` is configured. |
| `GET /observatory…` | HTTP | The daemon's observatory pages: `/observatory` (fleet), `/observatory/service?id=` (one service), `/observatory/wiring/` (preview/apply/undo of repo wiring), `/observatory/impact`, `/observatory/story`, `/observatory/brief` — loopback-only; see "Observatory series" below (**experimental**) |

Every verb accepts `--since DURATION` (default 7 days) or `--from`/`--to`
(RFC3339, given together) for the report window, and `--json` for
machine-readable output.

**`metrics series`'s valid metric names:** `spend_total_usd`,
`spend_cost_per_claim_usd`, `traffic_total_calls`, `traffic_error_rate`,
`channels_injection_coverage_rate`, `channels_obligations_served`,
`interventions_unproven_done_refusals`, `interventions_review_superseded`.
An unknown name lists these rather than silently returning empty. The
metric name may come before or after the flags — `metrics series
spend_total_usd --bucket day` and `metrics series --bucket day
spend_total_usd` both work.

---

## Observatory series (RG-6/7/8, experimental)

The service page draws four 30-day series — queried, discovering,
preventing, gating — from real call-log rows. A day with **no call-log
rows at all** renders `Absent` with the reason `no call-log rows for
<date>`; a day the log was alive but this service saw nothing is a
measured 0. Sparklines never draw across an absent day (one segment per
measured run), and the JSON never carries a zero total for an absent day.
The fleet page shows the grade distribution (A/B/C/D/F/idle) over 30
days; a day without a snapshot is Absent — today's fold only, until daily
snapshots persist, and the page says so. Symbols/traffic/serves tiles
render `Absent`, never 0, when the call log or store could not be read.
The wiring page's mutating endpoints (`preview` returns a `plan_digest`;
`apply` requires it and refuses 409 if the repo changed since; `undo`
requires the `receipt_digest` apply returned) accept only loopback,
same-origin requests for registered service roots.

---

## `FATHOM_METRICS` tiering

Three tiers, `off`/`basic`/`full`, default `basic`. **No tier spends LLM
tokens** — this is arithmetic on data already in hand at emit time, never
an LLM-analysed report. `fathom doctor`'s `metrics_tier` check reports the
active tier (always a `pass` — an operator's deliberate choice, never a
wiring failure), and names the fallback explicitly if the env var holds an
unrecognized value.

`full` is reserved for payload sampling and per-section compile-diagnostic
retention beyond what's captured unconditionally today (see the adoption
guide below for the "on for a tuning window, off after" pattern) — as of
this phase, compile rejection-reason capture (the highest-value item here)
already runs unconditionally regardless of tier.

---

## Honesty rules every surface in this family follows

1. **No spend line is ever captioned as a saving** (law 12). `metrics
   spend` is a cost ledger, full stop.
2. **Every ratio has a zero-denominator guard.** A group with calls but no
   errors reports `0%`; a group with NO calls reports an honest
   "not measurable" note — never a fabricated 0% or 100%. This closes the
   exact bug class phase P's own critic round found and fixed once
   (`buildDiscoverySection`'s zero-beats-fabricates-100% bug) — here it's
   closed structurally, via one shared `Ratio` helper every aggregator
   uses.
3. **The Interventions table never infers agent behavior.** Every row
   names a mechanism (a beat, a verdict, a claim_event) — row 3
   ("non-conforming artifacts refused") has no such mechanism today and
   says so plainly rather than guessing zero.
4. **Estimates say so.** Token figures elsewhere in this repo (phase P's
   payload cost) already carry `estimate`/`≈` in both the field name and
   the rendered line; this family's own figures (`compile_runs`' recorded
   `cost_usd`/token counts) are real recorded values, not estimates, and
   are not relabeled as such.

---

## Worked example

```sh
# What did the obligations layer cost this week, and how close to budget?
fathom metrics spend --store workspace/db/fathom.sqlite --service fathom \
  --since 168h --by layer

# Is the pulled-tools share of traffic moving? (still correlational —
# see docs/design/production-tokenomics.md §4)
fathom metrics traffic --by tool --since 24h --json

# Are the forced channels actually firing today?
fathom metrics channels --since 24h

# The design's own Interventions table, this week:
fathom metrics interventions --store workspace/db/fathom.sqlite --since 168h

# Build today's rollup (a cron/launchd job would call this nightly):
fathom metrics rollup --store workspace/db/fathom.sqlite --service fathom

# 30-day spend trend:
fathom metrics series spend_total_usd --bucket day --since 720h --json

# What would `metrics rotate` do to a 6-month-old call log? (dry run)
fathom metrics rotate --keep-days 90
```

---

## Maintained

This doc covers: `fathom metrics spend`, `fathom metrics traffic`, `fathom
metrics channels`, `fathom metrics interventions`, `fathom metrics series`,
`fathom metrics rollup`, `fathom metrics rotate`, `GET /statusz`, the
`/observatory` pages (fleet, service, wiring, impact, story, brief). Update
it when any of these verbs' flags, output shape, the observatory series
semantics, or the `FATHOM_METRICS` tiering behavior change — see also `docs/design/observability-surfaces.md`
for the full design rationale and `docs/plans/expert-suite-q.md` for the
implementation record.

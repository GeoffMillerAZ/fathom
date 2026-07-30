# Observability surfaces — spend, health, and honest intervention counts

**Status:** adopted design, owner-commissioned 2026-07-30 ("CLI based
informational diagnostics/observability/monitoring screens… how much it
spent… timeframes and graphs… per project… the same thing with a web GUI…
metrics on token usage and savings… how many times it saved you from making
a mistake… as long as recording that data wouldn't take more tokens than it
saves, or can be turned off… dashboards and status boards on localhost… the
web one could embed the docs and guides"). Owner asked for an ownership
verdict: **accepted with one metric rejected and the location reframed.**
Registered as phase Q in `docs/vision/13-expert-suite.md` §9. Depends on
phase P's instruments (`docs/design/production-tokenomics.md`).

## 1. Verdict, stated plainly

**Accepted:** spend/health/traffic screens, time-framed and per-project, in
a Charm TUI and a richer web app with the docs embedded, plus a localhost
status board. The data already largely exists and is currently unread:
`compile_runs` records `model`, `tokens_in/out`, `cost_usd` per run; the call
log carries surface/tool/latency/origin; the beat log carries channel
liveness; phase P adds payload cost and discovery share. Rendering existing
telemetry costs **zero runtime tokens** — this is reading, not instrumenting.

**Rejected as specified: "how many times it saved you from a mistake."**
That number cannot be produced honestly — it requires knowing what the agent
*would* have done, which is the counterfactual law 12 forbids inventing. I
will not ship a fabricated "mistakes prevented" counter, however good it
would look.

**Accepted in its place — "Interventions", counting only verifiable events**
(each labeled by mechanism, no inference):

| Intervention | Mechanism (why it is real, not inferred) | Live number today |
|---|---|---|
| unproven "done" claims refused | `verify_plan` returned a failing verdict on a row an author/agent had checked off | **57 in the last 7d** |
| review findings raised then superseded | a finding persisted, then reconciled as fixed on a later push — the fix is recorded, not guessed | from `claim_events` |
| non-conforming artifacts refused | `store.MountReadOnly` conformance refusal, module-doctor promotion denial | from the run log |
| governed edits with rules delivered | obligations served on an edit (delivery is the fact; obedience is not claimed) | 1,966 in 7d |
| guarded critical edits pre-warned | pre-hook served before the tool ran on a guarded scope | 96 in 7d |

The distinction is the product: "rules delivered" is a fact about fathom;
"mistake prevented" is a claim about the agent. We report the first.

**Reframed: the location.** Building a second TUI plus a second web stack
inside fathom would duplicate phases K and M and violate law 10. The screens
belong in **fathom-studio**, which already is the Charm + app-kit renderer
over a headless state machine. Fathom gains **verbs**; studio gains
**screens**; the web version then costs almost nothing because it renders the
same state. One UI codebase, two frontends, no third stack.

## 2. Fathom's side: the metrics verbs (law 10 — verbs first)

One family, all `--json`, all deterministic, all read-only over local files
and stores:

- `fathom metrics spend [--since|--from/--to] [--service|--module] [--by layer|day|model]`
  — from `compile_runs`: runs, tokens in/out, USD, **cost per claim**
  (17 claims / $0.256 = $0.015 on the first real platform module — the kind
  of number that makes tuning decisions), budget headroom against each
  layer's `max_usd_per_run`, and escape-hatch usage (`--unsafe-skip-eval-gate`
  runs flagged, since those are the cheap-but-untrusted ones).
- `fathom metrics traffic [--since] [--service] [--by surface|tool|hour]`
  — call log rollups: volume, p50/p95, error rate, plumbing-vs-discovery
  split (phase P's index-tool set).
- `fathom metrics channels [--since]` — injection coverage, guarded pre/post,
  dedup, module serves, each with phase P's honest "not measurable" lines
  carried through verbatim.
- `fathom metrics interventions [--since]` — §1's table, mechanism-labeled.
- `fathom metrics series <metric> --bucket hour|day --since` — the time
  series graphs need, computed from rollups (§4).

`research report` stays the narrative snapshot; `metrics` is the queryable
substrate underneath it. Neither duplicates the other's logic — the report
consumes the same aggregators.

## 3. The cost-of-measurement contract (the owner's explicit condition)

Three tiers via `FATHOM_METRICS` (default **basic**), reported by `doctor`:

- **off** — no beats, no payload accounting; `FATHOM_CHANNELS=off` remains the
  separate, blunter kill switch for the hooks themselves.
- **basic (default)** — everything phase P specifies: byte counts, token
  *estimates*, counts, latencies. All of it is arithmetic on data already in
  hand at emit time. **Marginal LLM token cost: zero.** Marginal wall cost:
  one append per hook invocation.
- **full** — adds per-call payload sampling (the rendered payload text
  retained for the tuning window, sampled at a configurable rate) and
  per-section compile diagnostics (the rejection reasons O19 showed are
  discarded today). This is the "on while tuning, off after" tier: it costs
  disk and privacy surface, still **not** LLM tokens.

**The honest headline:** no tier of this design spends model tokens to
measure model tokens. The only thing that ever could is an LLM-analysed
report, which this design does not have and does not want.

## 4. Retention and rollups (what makes graphs possible)

Graphs over 30/90 days must not rescan a growing JSONL. Nightly-or-on-demand
**daily rollup files** (`workspace/artifacts/metrics/daily/YYYY-MM-DD.json`,
deterministic, tiny, additive-only) hold the day's aggregates; `metrics
series` reads rollups plus today's live tail. Raw logs get a documented
retention window with a rotation verb; rollups are the long memory. Never
delete raw data implicitly.

## 5. Studio's side: the screens

**Charm TUI** (`fathom-studio`, new screens over the metrics verbs):
sparklines and braille-cell bar charts (pure Go — the fleet's no-Node rule
holds), a time-range selector (24h / 7d / 30d / custom), a project switcher
driven by `list_services`, and four boards: **Spend**, **Traffic**,
**Channels**, **Interventions**. Mouse plus full keyboard parity, per law 10
and phase K's precedent.

**Web app** (app-kit path A, Go server-rendered, no Node): the richer
surface the owner asked for. **The licensed corpora are the source here, not
hand-written CSS** — query the `appkit` MCP before writing markup (fleet
convention), pull the pattern, strip hardcoded colors, map to kit role
utilities via `get_kit_theming`, render through app-kit's pure-Go
`appkit/ui/*`. Grounding search already done, real corpus entries to start
from: `Application UI > Layout > List containers > Separate cards` and
`Application UI > Lists > Grid Lists > Logos cards with description list`
for the per-service/per-layer boards, `Marketing > Page Sections > Stats >
With description` for the headline figures, MatDash shells for the
sidebar/topbar/widget-grid frame. Charts stay app-kit `ui/viz` SVG —
neither corpus ships charting.

The surfaces it adds beyond the TUI —
- the same four boards with real charts (`appkit/ui/viz` SVG, MatDash shell
  patterns re-themed through kit tokens; TailwindPlus/MatDash consulted via
  the appkit MCP for the shell and card patterns);
- drill-downs the TUI can't fit: per-tool latency distributions, per-layer
  compile history, per-claim provenance, the module readiness ladder rendered
  as a ladder;
- **embedded docs**: `docs/**` rendered in-app with search, so the guides sit
  beside the metrics they explain (the docs are already in-repo; this is a
  markdown renderer plus an index, not a docs site build);
- a **status board** at `/statusz` on the existing `serve-http` listener
  (**deliberately NOT corpus-styled**: `/statusz` is served by the fathom
  daemon binary, whose doctrine is one pure-Go binary with no asset
  pipeline — adopting TailwindPlus there would mean an app-kit dependency
  plus a Tailwind build step inside the daemon to style a read-only
  diagnostic whose real contract is its JSON. It carries ~40 inline CSS
  lines and a comment saying exactly this; the corpus earns its keep in
  this web app, which supersedes it as the dashboard) —
  daemon health, index freshness per service, channel liveness, last compile
  per layer — loopback-only, read-only, and subject to law 9 (read-only
  before writable) and the hosted-mode allowlist if a token is configured.

## 6. Honesty rules binding every surface

1. Law 12 holds: a **Spend** board may never be captioned as savings.
2. Every "not measurable" from phase P renders as that text, not as 0 or a
   blank cell.
3. Estimates are labeled `≈` and their basis is one hover/keystroke away.
4. Correlational panels (discovery share) carry the label until the A/B runs.
5. `interventions` counts mechanisms, never inferred agent behavior (§1).
6. Nothing leaves the machine; the research-bundle allowlist remains the only
   share path.

## 7. Non-goals

No Prometheus/OTel exporter (parked, trigger: a second consumer asks); no
alerting; no retention of user code or prompts in any tier (payload sampling
stores the *rendered rule payload*, which is compiled claims — never source);
no auth beyond loopback plus the existing bearer for the hosted case; no
third UI stack in fathom itself.

## 8. Deliverables (phase-Q plan rows derive from here)

1. `internal/metrics` aggregators (pure, deterministic, tested) over call
   log, beat log, `compile_runs`, and phase-P ledgers.
2. The five `fathom metrics …` verbs with `--json` and time-range flags.
3. `FATHOM_METRICS` tiering + `doctor` reporting the active tier.
4. Daily rollups + `metrics series` + a documented retention/rotation verb.
5. Compile rejection-reason capture (O19's fix; feeds Spend and the phase-L
   advisor).
6. `/statusz` on `serve-http` (loopback, read-only, allowlisted under
   hosted mode).
7. **fathom-studio:** four TUI boards; then the web app with charts,
   drill-downs, embedded docs, and the status board.
8. Docs: a use-case guide for the metrics surfaces; adoption-guide section
   on turning `full` on for a tuning window and off after.

## 9. Sequencing

Phase Q starts **after phase P merges** — P owns `research_report.go` and the
beat schema this reads, and two workers in one file is how merge pain is
manufactured. Fathom's verbs land first (they are the contract), studio's
screens second, the web app third with phase M.

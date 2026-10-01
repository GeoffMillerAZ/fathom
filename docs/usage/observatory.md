## The observatory console and the brief

The daemon serves the Observatory console (vision 35, ADR-045; wave F0 of
`docs/plans/observatory-console.md`). Every page renders on one app-kit shell:
a grouped sidebar (Now, Fleet, Proof, Knowledge, Evidence, System, Learn — a
group shows once a page registers in it), a breadcrumb trail, a page heading
with the question it answers and the verbs it renders as copyable chips, a
light/dark/system theme control (browser storage key `fathom-theme`), a skip
link, and a search field. Error pages (404, 400, an unknown project, an
unreadable store) render on the same shell with the reason and the remedy.

| Route | Page |
|---|---|
| `/` | Now — the attention queue (gate failures, unproven claims, stale or unknown indexes, certificate gaps, uncompiled governing guides, open observations: each with its evidence link and remedy), what changed since your last look, a stat band and the fleet pulse; the JSON carries `fathom brief --json` and `fathom fleet report --json` |
| `/fleet` | Every project in one table — `?sort=<column>&dir=asc\|desc`, `?health=failed\|degraded\|unknown\|healthy`, `?q=` (name or path), `?lang=`, `?page=` — rolled up by health; the default order is unrealized value (RG-4); the JSON carries `fathom fleet report --json` computed by the verb's own function |
| `/projects/{svc}` | One project's workspace (was `/observatory/service?id=S`): nine tabs, each a URL — Overview here; Index, Intent, Activity and Wiring at `/projects/{svc}/index`, `/intent`, `/activity`, `/wiring`; Governance, Plans, Proof and Refactor open `/layers`, `/plans`, `/certifications` and `/refactors` with `?svc=` |
| `/projects/{svc}/index` | Languages with engine and tier, typed share and edge kinds, inventory keys (`fathom report evidence --service S --store P --json`); `?symbol=` draws the code neighborhood (callers and callees to depth 2, resolved solid and heuristic dashed, six per column and the rest aggregated, with a table); the read-only Scope panel (`fathom scope list`; `?scope=<glob>` previews what a suppression would hide and what a purge would delete, per location, and names the commands — the page runs neither) |
| `/projects/{svc}/intent` | `fathom intent audit --read-only`: the drift radar (decisions due for review — computable triggers that fired — and the archconformance rules whose ingest-time violation count rose, with the commits that added them), invariants with rung and audit state, decisions/futures/questions with their conditions, unproven claims, debt over time; a run is kept five minutes per commit |
| `/projects/{svc}/activity` | The project's calls from the resolved call log (a day strip, `fathom metrics traffic --by tool` over the 14-day window, the newest calls) and its sessions from the ledger |
| `/projects/{svc}/wiring` | `fathom doctor --repo-only` rows, drift (failing wiring rows plus the preview plan's apply and BLOCKED items), receipts (`fathom fleet receipts`), beside the unchanged Preview → Apply → Undo |
| `/layers` | The strata, the projects × layers coverage matrix (a cell nothing compiled is hatched "not compiled", never 0) and the claim lifecycle — `fathom layers strata --json`, `fathom layers coverage --json` |
| `/layers/{layer}` | One layer's compile runs, rejection reasons, projects and uncompiled scopes — `fathom layers show <layer> --json` |
| `/claims?layer=&svc=&status=` | The claims a count counts, 50 a page (`&page=N`), each linked to its page; every claim count on `/layers`, `/layers/{layer}` and `/packs` links here — `fathom claim list --json` |
| `/rules?svc=&class=` | Rule effectiveness (GL-18, U4): each governing rule with the edit hook's served, top changed, still served, blocked and refuted counts over 30 days (served over 90), each with its n, the class it falls in — ignored, contested, never triggered, effective — and the change the evidence proposes (a proposal: fathom never edits a rules file or retires a rule from here). No rule-outcome log is an absent signal, not "never triggered", and never triggered waits until the log reaches back 90 days — `fathom claim effectiveness --json` |
| `/claims/{id}` | One claim: statement, citation excerpt, scope, tier, status, events (`?svc=` when two projects hold the id) — `fathom claim show <id> --json` |
| `/packs` | Dialect packs as a language × capability matrix and layer packs with where each is active — `fathom report packs --json` |
| `/explain?svc=&file=` | The serving explainer: candidates by scope tier, the cap, the served list and each displaced rule's reason — `fathom obligations --explain --json` |
| `/impact` | The four questions, Q1–Q4 in order: each counterfactual arm a range (p25–p75 of real tokens per turn, median marked) with its session count; every tool's outcome "not instrumented" where no instrument exists, never a colored zero; and the catches timeline — verify refusals, conserve losses, integrity catches, guard refusals — filterable by kind, window (`?days=`) and project (`?svc=`; a project no registry entry and no instrument names is the shell's 404), its rows the table. Renders `fathom metrics impact --json` and `fathom metrics catches --json` |
| `/sessions`, `/sessions/{id}` | A table of sessions (one row per session: arm, turns, first edit, tokens, fathom calls, catches), filterable (`?arm=`, `?repo=`, `?q=`), sortable (`?sort=`, `?dir=`), 50 to a page; and one session's replay, windowed — 200 events a page (`?per=`, never more than 2,000 per response), filterable by channel (`?channel=`). Renders `fathom metrics sessions [--id S] --json` |
| `/operate` | The daemon (version, commit, uptime, memory, goroutines); calls per day per surface as a heatmap whose missing day is hatched, never zero, with p50/p95/errors per day in its table; latency and errors by surface and by caller class; the tools by caller class and the zero-call list; the flywheel; `config --explain`; time-to-remedy; a project's capabilities (`?svc=`); the harness matrix. Each panel names the verb that prints it: `metrics load`, `metrics traffic`, `instrument --adoption`, `fleet remedy`, `config --explain` and `harness matrix` |
| `/decide` | The decision port: provider and sites, the ruling log beside the call log (`?site=`), and the latest calibration reading's table. Renders `fathom decide log --json` |
| `/brief` | The shareable brief; `?format=md` downloads it |
| `/wiring` | Per-repo Preview → Apply → Undo (the POSTs stay at `/observatory/wiring/*` behind the local gate) |
| `/statusz` | Daemon status (JSON schema 3 unchanged) |
| `/plans` (`?svc=`, `?page=`) | Every plan across the fleet (or one repository) with a proof bar counted from the gate's verdicts, the claimed-but-unproven gap as the headline (a named lower bound while rows sit measured-stale), and the claimed-vs-proven trend of the scope's current plans once the verify ledger holds 14 days of their rows; carries `fathom plans status --service S --json` per repository and `fathom plans history [--service S] --json` for the trend |
| `/progress` (`?svc=`, `?plan=`, `?wave=`, `?goal=`, `?agent=`, `?from=`, `?to=`) | Read-only views over computed state (PE-4..PE-12): goal gauges per scope bet, weighted progress by fleet/repository/plan/wave (`plans status --weighted`), the derived kanban (Open, Claimed-unproven, Proven, Refused or deferred; cards link to the row and its failing criterion), the frontier (`plans next --eligible-only`), the burn-up with a projected finish (a 14-day linear fit with its interval, withheld under 7 days), proof regressions with the commit that changed them, time-to-proof, claim honesty per agent identity (never per person), and the proof heatmap with streak and proved-today counts — all projected from the verify ledger's schema-3 proof history |
| `/work` (`?svc=`) | Work intelligence for one repository (WI-8..WI-13): the model and harness scorecard (descriptive unless G2 admits a comparison), the interest rate and cost per zone with the first-attempt split, the friction map with proposed remedies, and hotspots. Zones come from `[zones]` in the repository's `.fathom/config.toml`; CI minutes come only from the CI summary files `[[work.ci_summaries]]` names — each a JUnit XML file (its `time` totals) or a workflow-run JSON export, by a repo-relative or absolute `path`, with the `commit` it built (a workflow run's own `head_sha` wins; a JUnit file must name one) — and a file the page could not read or parse is named in the cost-per-zone panel |
| `/plans/{svc}/{plan}` | The plan's proof bar, its rows by state on a read-only board (each card names its contract kind; a card opens the row's evidence in a drawer, and is a link without JavaScript), every row as a table, and the burn-up; carries `plans status`, `plans next` and `plans history` for the plan |
| `/plans/{svc}/{plan}/rows/{id}` | One row's evidence: contract, verdict criterion by criterion, the call path from its test to its symbol (or the line its lexical oracle matched), and its history from the plan's commits; `?format=fragment` is the drawer's body; carries `fathom verify --row ID --json` |
| `/certifications` (`?svc=`) | Targets by their best certificate's state (heatmap and table), each repository's line, and the gap list with remedies; carries `certify status` (and, for one repository, `certify gaps`), each chip naming the daemon's `--store` |
| `/refactors?svc=&since=` | `fathom conserve --since` for one repository: the planes as chips (a plane that did not run reads "not run", never "conserved"), the findings, and the reading set with an excerpt of each partial function's unexecuted lines (partial and unexecuted read "not measured" without execution evidence); the chip names the daemon's `--store`, and the page never writes the repository's index; without `svc` a chooser |
| `/learn` (`?svc=`) | How fathom works: the pipeline for one project (default: the most-queried in the call log) — each stage's live counts with their truth states, the `fathom capabilities` switches that deliver it, and a link to the page that shows it in full; the JSON carries `fathom fleet report --json` and that project's `fathom capabilities --repo R --service S --json` |
| `/learn/ask` (`?svc=&try=&q=`) | Ask the right way: nine question shapes, the agent-surface tool that answers each and how often it was asked on the project; `?try=<shape>` makes one read-only call of that tool on `/rpc/agent` (logged with the console's origin, never counted as usage) with `q` or the project's own example input; the JSON carries `fathom harness matrix --json` |
| `/learn/guides`, `/learn/guides/{slug}` | The use-case guides, the glossary (`glossary`) and the laws (`laws`, anchors `#law-1`…`#law-15`), rendered from the documents built into the binary with a table of contents |
| `/search?q=` | Projects, plans and plan rows, symbols, rules and records, verbs and pages, grouped; groups are listed by their best match (a tie goes to the smaller group, then that order) and every query word must match; a row or record ID matches exactly (`OC-1`, `ADR-045`); each group shows 8 of an exact total, and `&group=<slug>` (`projects`, `plans-and-rows`, `symbols`, `rules-and-records`, `verbs`, `pages`) pages through all of it, 50 at a time (`&page=N`). A query of more than 8 words or 200 characters is a 400. `?format=fragment` is the HTML fragment the command palette will load, `?format=json` the results |

- Every page answers `?format=json` (or `Accept: application/json`) with one
  envelope: `schema`, `page`, `route`, `title`, `group`, `question`, `verbs`,
  `metrics` (the shared metrics it shows, each a value with its truth state —
  `present`, `zero`, `absent`, `stale`, `partial`, `error` — reason, source and
  denominator), `sections` (each declared verb's `--json`) and `model`.
- Shared metrics are computed once (`internal/console`): `fleet.services`,
  `fleet.symbols`, `calls.total` (every row in the resolved call log —
  unattributed rows are named, never dropped), `sessions.total` (distinct
  sessions, never ledger rows) and `observations.open` (a partial count stays a
  "≥" lower bound everywhere, the brief included); `fleet.wired_fresh`;
  `plans.proven` and `plans.claimed_unproven` (the claimed rows the gate proves
  and does not, `plans status` over every plan — the Plans page's headline);
  `claims.current` (governed claims serving can still use: fresh, stale,
  contradicted); `certify.live` and `certify.gaps` (`certify gaps` summed over
  every service). One metric per question (`console.MetricCatalog`): the Now,
  Fleet, Plans, Layers, Packs and Certifications pages all read these, and
  the ids that once answered the same questions a second way
  (`plans.verified`, `claims.fresh`, `certificates.live`) are gone.
- A call-log, verify-ledger or hooks-log row naming a service the daemon's
  registry does not list (a test fixture's `svc`, a deregistered repo) is not
  the fleet's (OC-H2): the catches timeline (`/impact`, `metrics catches`) and
  every per-service call figure leave it out and name it instead —
  `unregistered` (rows and service ids) on the timeline and in each lane's
  denominator, `unregistered_calls` in `fleet report --json`, and a split in
  `calls.total`'s denominator, which still counts every row. A project filter
  naming only such rows is a 404 saying why. A row naming no service is
  unattributed, never excluded; with the registry unreadable nothing is
  excluded and `registry_note` says so.
- The Proof pages verify plans in the background through the path `plans
  status` takes, keyed by each plan's content, the index watermark and HEAD: a
  page waits a bounded time, shows a plan still verifying as "not yet
  verified" (its claims are never counted proven), and its panels say
  "Updated" over `/events` when the verdicts land. `/refactors` does the same
  for `fathom conserve`.
- The fleet page's grade distribution over 30 days (RG-7) is
  **experimental**: days without a snapshot are Absent — today's fold
  only, until daily snapshots persist (the page says so). Symbols/traffic/
  serves tiles render Absent, never 0, when the call log or store could
  not be read. A project's 30-day per-dimension series (RG-6) renders a day
  with no call-log rows as Absent and never draws a sparkline across it.
- `fathom brief [--print|--out DIR]` — the same impact model as a dated
  local markdown brief, with an explicit "Not measured" section.
  Sharing is a human verb (law 13): the verb writes a file and stops.

**The call log is resolved once** (OC-2). `serve-http` records itself in
`~/.fathom/daemon/<id>/record.json` (its absolute `--call-log`, store, pid and
commit; the legacy `~/.fathom/daemons/<host>-<port>.json` is still read this release,
RT-166) once its listener is bound, and renders its own pages from that log.
Every metrics, instrument and flywheel verb, `fleet report` and `research
report`/`bundle` then resolve the log the same way: `--call-log`, else
`FATHOM_CALL_LOG`, else the record of the daemon they talk to (`FATHOM_HTTP`,
default `localhost:7575`), else a named `--service`'s own root (J5), else
`[daemon].call_log`, else the legacy default path. A verb run from any directory
now reads the log the daemon writes; `metrics rotate` inspects that same log and
`review` appends its T5-7 gate row to it. A daemon started with `--call-log ""`
records logging off, and verbs talking to it then read no log (rule `none`)
rather than a stale legacy one. Every verb names the log and the rule that chose
it: a `call_log` object (`path`, `rule`, `note`) in its JSON (`fleet report`
keeps `call_log_path` beside it) and a `call log: <path> (rule …)` line in its
text; `research bundle` names it on stderr, never inside the shareable bundle.

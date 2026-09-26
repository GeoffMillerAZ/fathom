# The Observatory console — mechanics

Vision: `docs/vision/35-observatory-console.md` (ADR-045). Plan: `docs/plans/observatory-console.md`.
Mockups drawn from the fleet's real data: the "Fathom Observatory Console" design canvas
(owner artifact, 2026-09-25). Doctrine that binds every line below: `cmd/fathom/CLAUDE.md`
(app-kit only, role utilities, `task ui:css`, a 200 is not a rendering, absent is never 0,
mutations stay local) and `PRODUCT.md`.

## 1. Architecture

**Projections are the product.** `internal/console` holds the page projections. They are
pure functions over the store, the ledgers and the repository, and they return typed models
with an explicit truth state per value. `cmd/fathom` renders them with app-kit and serves
their JSON unchanged. A CLI verb and a page that show the same thing call the same
projection, so they cannot disagree (P10).

- `internal/console` imports no UI package.
- Every projection function takes a `Sources` value naming the store, the resolved call log,
  the ledgers, and an injected clock. That keeps it deterministic and testable on the
  fauxtown fixture fleet.
- A `Value[T]` carries `{v T, state Present|Zero|Absent|Stale|Partial|Error, reason, source,
  denominator, as_of}`. Renderers map `Absent` to `stat.Absent`. JSON omits `v` for
  `Absent`/`Error` and always carries `state` and `reason`.
- Each metric shown on more than one page has exactly one projection function. The
  consistency test (`TestConsoleMetricsAgreeAcrossPages`) renders every page's JSON on the
  fixture fleet and asserts equal values for equal metric IDs.

**The call log is resolved once.** CLI verbs today read `./workspace/logs/mcp-calls.jsonl`
relative to the working directory, so the same verb reports different numbers from
different directories. `console.ResolveCallLog` returns the daemon's configured `--call-log`
(from the daemon's own record, `~/.fathom/daemons/<host>-<port>.json`, written once
serve-http's listener is bound), then `[daemon].call_log` in config (the one config key
for the log), then the legacy relative path; an explicit `--call-log` or
`FATHOM_CALL_LOG` still wins for its own invocation. The result carries which rule chose it, and every metrics verb and page
uses it.

**Pages register themselves.** `cmd/fathom/console_registry.go` holds a table of `Page{Route,
Group, Title, Nav, Verbs []string, Build func(ctx, req) (Model, error), Render func(w, Model)}`.
Each page lives in its own `cmd/fathom/console_<page>.go` and registers in `init()`. So:

- the router, the sidebar, the command palette index and the JSON parity test all read one
  table;
- parallel page work touches separate files;
- `cmd/fathom/ui/input.css` scans `cmd/fathom/console_*.go` by glob, so a new page file
  cannot silently lose its classes.

**Routes.**

| Route | Page | Verbs it renders |
|---|---|---|
| `/` | Now | `brief --json`, `fleet report --json`; per project, `plans status --json` and `certify status --json` (from the recorded run at HEAD and the store; see JSON) |
| `/fleet` | Fleet | `fleet report --json` |
| `/projects/{svc}` (`?tab=`) | Project workspace | per tab (§3) |
| `/plans`, `/plans/{svc}/{plan}`, `/plans/{svc}/{plan}/rows/{id}` | Proof ledger | `plans status/next/history --json`, `verify --row --json` |
| `/certifications` | Certifications | `certify status/gaps --json` |
| `/refactors` (`?svc=&since=`) | Refactor | `conserve --since --json` |
| `/layers`, `/layers/{layer}`, `/claims`, `/claims/{id}` | Semantic layers | `layers strata/coverage/show --json`, `claim list --json` (the rows behind every claim count), `claim show --json` |
| `/packs` | Packs | `capabilities --json`, `report evidence --json`, `layers list --json` |
| `/explain` (`?svc=&file=`) | Serving explainer | `obligations --file --explain --json` |
| `/impact`, `/sessions`, `/sessions/{id}` | Evidence | `instrument`, `flywheel`, `metrics …`, session ledger |
| `/intent` (`?svc=`) | Intent | `intent audit --json --read-only` |
| `/operate`, `/decide` | System | `metrics traffic/series`, `flywheel`, `capabilities`, `config explain`, decision log |
| `/wiring` | Wiring | `doctor --json`, `fleet reconcile`, `fleet receipts --json` (preview/apply/undo unchanged) |
| `/learn`, `/learn/ask`, `/learn/guides/{slug}` | Learn | `capabilities`, `harness capabilities`, use-case guides |
| `/search` | Palette results | the registry, services, plans, rows, symbols, intent records, verbs |
| `/events` | Live freshness (SSE) | the event bus (§4) |

The old `/observatory*` routes keep answering. Each HTML route 308-redirects to its new
home. `?format=json` on an old route keeps returning `observatoryModel` v1 unchanged,
because it is part of the compatibility surface (`docs/intent/scope.md`). `/statusz` moves
onto the shell; its JSON v3 is unchanged.

**JSON.** Every new page answers `?format=json` (or `Accept: application/json`) with its
projection. `TestConsolePagesRenderTheirVerbs` asserts that, for each page and each verb it
names, the page JSON's section equals the verb's `--json` output on the fixture fleet.

A per-project verb (`plans status --service {svc}`, `certify status --service {svc}`) is
declared as a page's `ProjectVerbs`: the page renders its answer for every project from a
recorded instrument instead of running it, and carries a section per template keyed by
service, in the verb's own shape. The parity test runs each concrete verb and holds the page
to it: `certify status` exactly; `plans status` for every plan whose recorded run is at the
project's HEAD (proven, claimed-unproven, total), after a real `fathom verify` on the fixture.

**Plan proof is the run at HEAD.** The console never runs a verify to render. A plan's
proof is the newest verify-ledger run recorded at the project's HEAD commit for the same
service, wherever it ran: the registered root or any clone (the pre-push gate verifies in a
detached clone). Only when no such run exists does the page fall back to the registered
root's newest run, and then the value is stale: it names the commit the run proved, its date
and its age, and its findings read in the past tense. A failure recorded at an older commit
is never a gate failure on Now nor "failed" on Fleet; a plan no run ever checked is listed,
never skipped.

**Caching.** A per-projection cache is keyed by its input watermarks: store commit and
watermark, ledger size and mtime, plan file hashes. The cache is bounded, and nothing
re-renders when the watermarks are unchanged. The project set Now and Fleet share re-folds on
each 30 s model rebuild from reads cached by their own watermarks: the verify ledger's parse
by its size and mtime, each project's store facts (claims, certify status, the compile
record) by the store file's write stamp, the project's HEAD and its intent corpus's stamps,
and the typed-edge failures by the store stamp. Budgets:

- warm render ≤ 150 ms on the fixture fleet and on the real fleet;
- HTML ≤ 250 KB per page;
- session replay windowed to ≤ 2,000 events per response, with the rest paged.

## 2. Shell and interaction

- **Shell.** `dashboard.RenderShell` with grouped `NavGroup`s (Now, Fleet, Proof,
  Knowledge, Evidence, System, Learn), `chrome.Breadcrumbs`, a page heading with meta and
  actions, `appearance` theme control (light / dark / system; storage key `fathom-theme`),
  and a skip link. Error pages (404, 400, unknown service, store unreadable) render on the
  shell with the reason and the remedy.
- **Panels.** Each panel is a `dashboard.Widget` carrying title, question, a verb chip (the
  exact command, copyable), and a footer with source, denominator and freshness. Truth
  states render distinctly: zero is a number, absent is `stat.Absent` with its reason, stale
  carries its age, partial names its lower bound, error names what failed.
- **Tables.** `table.Table` with sortable headers and `Row.Href` drill-down. Filters are URL
  query parameters. `chrome.Pagination` pages anything over 50 rows.
- **Command palette.** ⌘K / Ctrl-K opens `overlay.RenderDialog` with a search field.
  Results come from `/search?q=` as an HTML fragment, grouped (Projects, Plans and rows,
  Symbols, Rules and records, Verbs, Pages). Groups are listed by their best match (a tie
  goes to the smaller group, then that order), so an exact hit is never below a group
  of weaker ones. Every query word
  must match; a row or record ID matches exactly; the in-order fuzzy match counts only
  inside a compact span. Each group shows its top 8 of an exact total and links to its
  own page (`&group=`), paged 50 at a time. A query is bounded (8 words, 200
  characters; past that, a 400 on the shell). Arrow keys move, Enter opens, Esc closes
  and focus returns. Without JavaScript, `/search?q=` is a full page with the same
  results.
- **Decision port in the palette.** For a free-text query the palette may show one hedged
  "Suggested view" line from the existing route site. That happens only when `[decide]
  provider` is on and `sites` includes `route`, and it never replaces results.
- **Live freshness.** An L2 script subscribes to `/events`. A panel carries
  `data-console-source="plans:fathom"`. When an event names that source, the panel shows
  "updated — refresh this panel" (a link to the same URL with its anchor). Content never
  moves under the reader.
- **Drawers.** Row evidence opens `overlay.RenderDrawer` over the plan page. The same
  content is a real page at `/plans/{svc}/{plan}/rows/{id}`.
- **Accessibility.**
  - One `h1` per page and landmarks.
  - Tables use `th scope`.
  - Every chart has a table alternative behind `chrome.Disclosure`.
  - 44 px targets.
  - Visible focus, and color is never the only channel.
  - Both themes, and no horizontal overflow at 390 px.

## 3. The pages

**Now.** The attention queue is an ordered list of items. An item is a problem or pending
decision with its area, evidence link and remedy. It is ordered by what blocks proof first:

1. gate failures (at HEAD);
2. unproven claims (at HEAD, then plans last verified at an older commit, then plans no
   run ever checked);
3. stale or failed indexes;
4. certificate gaps;
5. uncompiled governing guides;
6. open observations.

An item carries a truth state: present, stale (recorded of an older commit), partial (a
lower bound) or error (unreadable; its count is then omitted, never 0).

Beside the queue sit a stat band, "since you last looked" (pushes, deploys, measurements,
restarts, taken from the ledgers and the daemon state; paged 50 at a time, every change in
the window on some page, "new" counted over all of them), and a fleet pulse. The fleet
pulse shows the realization ladder per project with one shared sparkline scale.

**Fleet.** A sortable, filterable table of every project. Columns: symbols, languages,
freshness, realization ladder, plans proven/open, claims fresh, certificates live, calls
over 14 days, and open observations. Above it, `topology.Render` rolls projects up by
health. Unknown is never green.

**Project workspace.** Tabs are `chrome.Tabs`, each tab a URL:

- *Overview.* Realization ladder, wired → fresh → queried → preventing → gating. Each rung
  shows its evidence, and the first unmet rung shows its remedy (RG-5).
- *Index.* Languages, engines and tiers; typed share; edge kinds; inventory keys; the code
  neighborhood for a chosen symbol (§5).
- *Governance.* The layers and claims for this repo, plus the serving explainer.
- *Plans.* This repo's plans.
- *Proof.* Certificates and verify history.
- *Refactor.* The reading set against a chosen base.
- *Intent.* A read-only audit.
- *Activity.* Sessions and tool calls.
- *Wiring.* Doctor checks, drift, and receipts.

**Proof ledger.**

- *Portfolio.* Every plan across the fleet with a proof bar: proven, claimed-unproven, open,
  op measured, op waiting, deferred, refused.
- *Plan page.* The proof bar, then a board (`kanban.Board`) of rows by state. Cards carry
  the row ID, title and contract kind.
- *Burn-up.* Scope against proven over time, from the plan's git history, cross-checked with
  the verify ledger where its rows name the plan.
- *Row evidence.* Contract fields; the verify result; the reach path from test to symbol
  (`callgraph.TestImpactDetail`) or the lexical-oracle match line; and history (first proven,
  refutations).
- *Certifications.* An invariants × certificates matrix with states, plus the gap list.

**Semantic layers.**

- *Strata.* Four bands:
  - Dialect: the extractors, from `capabilities` `extraction_engines`.
  - Provider: parked.
  - Platform: `.fathom/layers` and kit packs.
  - Concern: the embedded catalog.

  Each layer block shows current claims, a lifecycle strip (fresh / stale / superseded),
  compile runs and cost.
- *Coverage matrix.* Projects × layers. A missing cell is hatched and says "not compiled".
- *Layer page.* Compile runs with cost, accepted and rejected counts, rejection reasons,
  uncompiled scopes and a dry-run command.
- *Claim page.* Statement, citation excerpt, scope globs, tier, status, and the event
  history (re-anchored, contradicted, corroborated, refuted).

**Packs.** Dialect packs as a language × capability matrix:

- engine and tier;
- typed edges and their share;
- integrity support and inventory keys;
- measured precision and recall where a reading exists (L0 oracle, CV-M7).

Layer packs are listed with their source (embedded, project, or kit `fathom-pack`), the
repositories where each is active, and what each contributes.

**Serving explainer.** Given a service and a file, `obligations --explain` returns:

- every candidate claim with its scope tier;
- the ranking and the cap;
- the served list;
- each displaced claim with the reason it lost (tier, rank, cap);
- raw-fallback rows and the coverage note.

The page draws the four scope columns and the served list. `--explain` is additive, and the
default `obligations` output is unchanged. Serving must also stop serving two claims whose
normalized statements are identical: today it served "Inject clocks…" twice for
`internal/conserve/conserve.go`.

**Refactor.** The conservation planes as verdict chips. A plane not run for the range shows
"not run", never "conserved". Below it: the reading set's counts and items, and a code
excerpt per partial item with its unexecuted changed lines highlighted (§5).

**Impact.** Questions come in a fixed order (Q1–Q4). The counterfactual arms are range bars
with their counts. Catches go on a filterable timeline. Every tool's outcome column says
"not instrumented" where there is no instrument, never a colored zero. One projection per
metric removes today's contradictions:

- the realization grade alone, never the wiring grade under the same name;
- "≥ N" kept as a lower bound everywhere;
- sessions deduplicated;
- calls counted from the resolved log, with unattributed calls shown as their own row.

**Sessions.** A table of sessions (arm, turns, first edit, tokens, fathom calls, catches)
and a windowed, filterable replay.

**Intent.** Invariants with enforcement kind and audit state, decisions with revisit
triggers (fired or not), futures, questions, and the intent ledger's debt over time.
`intent audit` gains `--read-only`, used by the page, so rendering never appends ledger
rows.

**Operate.**

- Daemon: version, commit, uptime, memory, goroutines.
- Traffic per day and per surface with p50/p95 and error rate.
- Tools, labelled by caller class (agent, edit hook, gate, plumbing, refresh), plus the
  zero-call list.
- The flywheel loops, capabilities, `config explain`, and the harness matrix.

**Decision port.** The provider and its sites, the ruling log (site, kind, confidence,
latency, applied), and the calibration table from the latest reading.

**Learn.**

- *How fathom works.* The pipeline diagram with live counts for the selected project. Each
  stage links to its page.
- *Ask the right way.* Question shapes mapped to tools, each with a "try it" link that
  renders the real, read-only answer on the selected project through the page's own
  projection. Nothing new is exposed.
- *Guides.* The use-case guides rendered with `docs.RenderToCRail`, plus a glossary and the
  laws.

## 4. Live events

An in-process bus in `serve-http` publishes:

- `ingest` (service, commit);
- `compile` (service, layer, run);
- `verify` (service, plan, result), observed by polling the verify ledger's size every 2 s;
- `session` (new ledger row);
- `daemon` (restart, adopt).

`/events` streams them as SSE with an ID, sends a keepalive every 25 s, and replays the last
64 events on reconnect through `Last-Event-ID`. The stream is bounded to 16 subscribers.
Events carry names and IDs only, never content. Event IDs count per process, so each SSE
id carries the process's epoch (`<epoch>-<n>`): a reconnect with an id from another epoch
(the common case: a daemon restart) replays the whole ring, the restart event first. An
`ingest` event means the service's indexed content moved (its commit or the store's
content digest), never a hash-gated no-op re-ingest; a `session` event names the service
whose root the row's repo label names. Each event also names a fleet-wide key (`index`,
`layers`, `plans`, `sessions`, `daemon`) for panels that summarize every service, and a
reconnect whose cursor the replay cannot cover (another epoch, or older than the ring)
is first sent a `gap` event naming every source (`*`).

## 5. App-kit: adoption and contributions

**Adopt from the current mirror:**

- `table`, `chrome`, `topology`, `kanban`, `overlay`, `appearance`, `docs`;
- `analytics` (comparison recipe, `WriteTableCSV` for exports);
- `viz` Heatmap, Funnel, Steps, StackBar, RangeBars, MeterRow and Timeline filters;
- `hitl.DiffView`.

**Build just in time and return** (ADR-0066; each gets one CUE inventory item, converted-core-v1
gates, a catalog entry, a Gallery specimen, golden tests and a contribution record in
`docs/app-kit.md`, and stays experimental until a second consumer):

| ID | Piece | Layer | Notes |
|---|---|---|---|
| FATHOM-KIT-01 | Command palette | L2 + L3b | Dialog, server-rendered fragment, keyboard contract, no-JS page fallback |
| FATHOM-KIT-02 | Layered graph renderer | L3a | Deterministic layered DAG and ego graph; bounded nodes with an aggregate node; table alternative; no class attributes, role colors only |
| FATHOM-KIT-03 | Heatmap truth states | L3a | Missing and unknown cells drawn distinctly from zero, with a legend; enhances `viz.Heatmap` |
| FATHOM-KIT-04 | Code excerpt | L3b | Line numbers, per-line state, anchors, keyboard-scrollable viewport |
| FATHOM-KIT-05 | Unified diff | L3b | Line-level diff for `hitl`, beside the field-level `DiffView` |
| FATHOM-KIT-06 | Live freshness | L2 | SSE subscriber and "updated" affordance with bounded reconnect; never re-renders content |
| FATHOM-KIT-07 | Page heading | L3b | Title, meta, actions and breadcrumbs slot |
| FATHOM-KIT-08 | Catalog entries | catalog | Records for shipped but uncatalogued packages (topology, viz family, vizframe, hitl pieces, motion, explainer, export, chrome) |

Domain views stay in fathom: the proof ledger, the strata, the serving explainer, the
dialect matrix and the attention queue.

## 6. Verification

- Page tests on the fixture fleet: structure (one `h1`, landmarks, `th scope`), verb chips
  present, no literal colors or `dark:` in rendered classes, truth states rendered, and
  links resolve to registered routes.
- `TestConsolePagesRenderTheirVerbs` (JSON parity).
- `TestConsoleMetricsAgreeAcrossPages` (consistency).
- `TestConsoleBudgets` (render time and HTML size on the fixture fleet).
- `task ui:css`, and both themes screenshotted in a browser at 1440 and 390 px (doctrine
  rule 4).
- UX-M1: the ten task journeys scripted with Playwright against a scratch daemon. Clicks
  from Now are counted, and each answer is compared with its verb.

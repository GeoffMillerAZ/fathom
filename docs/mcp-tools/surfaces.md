## Tool groups, profiles, and surfaces

Tools are divided into four groups (`GroupShared`, `GroupCoding`, `GroupInvestigation`,
`GroupPlatform`; `GroupModules` went with the module tools, SR-17) that describe fathom's internal taxonomy. `set_profile` — the old,
process-global way to filter `tools/list` on stdio — was **removed entirely**
(docs/plans/surfaces-week1.md SW-2; amends ADR-014's stdio clause): no production caller
reaches it any more, so a **stdio** session always sees the full registry. Scoping now
lives entirely over HTTP.

**Which harness reaches these tools, and how, is registry data** (ADR-029): every
harness descriptor carries its MCP config path, HTTP key name, `mcp add` command and
tool-name pattern. `fathom harness list` names the registered harnesses and
`fathom harness capabilities [--harness ID]` prints the per-capability vehicle table
(the `ladder` subcommand, `fathom harness ladder`, prints the tier B/C/D rows — stanza +
MCP + skills, no injection) —
including the `mcp-tools` row, which is the only channel by which a tier-C/D harness
(no hooks, no injection) reaches anything on this page. An unverified descriptor
renders every row as `unverified on this machine` — Claude Code and Codex are verified
against real sessions on this machine, the rest are not; the catalog below is
unaffected by the harness in use. Each descriptor also carries the harness's full
EVENT catalogue with a reason per unwired event, which `fathom doctor`'s advisory
`hooks_coverage` row prints. `fathom intent render --playbook` is the one verb that
writes outside `.fathom/` and the instruction files: a root `INTENT.md` inside fathom
markers, rendered as pointers into `docs/intent/`. `fathom harness matrix` renders the per-harness adoption matrix
(tier, instruction file, skills path, MCP config + `mcp add` line, verified-against)
that `docs/adoption/other-harnesses.md` pins, `fathom harness hooks --harness ID`
prints that harness's hook entries as paste-ready JSON for the cases where fathom
refuses to write the file itself (Codex loads hooks only from its user-global
`$CODEX_HOME/hooks.json`, and user-global configuration is ask-first), and
`fathom plugin render` packages
the registration for two plugin loaders at once — Claude Code's `.mcp.json`
(`type: http`) and the Agent Plugins 1.0 `mcp.json` (`streamable-http`, loopback) —
over one `skills/` tree.

### Path-scoped surfaces (HTTP)

Over **HTTP** (`serve-http`, the shared daemon — default `127.0.0.1:7575`, loopback
only) each request selects an immutable **surface** via the path:

| Surface | Endpoint | Contents |
|---|---|---|
| *(full)* | `POST /rpc` | Every registered tool, unauthenticated — the default for schema-deferring clients such as Claude Code; includes `ingest` (writes) and `compile_layer` (LLM spend). |
| `agent` | `POST /rpc/agent` | The default eight-tool floor (see below) — the user-scope MCP registration points here |
| `extended` | `POST /rpc/extended` | The opt-in superset (see below): the default eight plus the navigation/context/freshness tools and the remaining promoted verbs |
| `companion` | `POST /rpc/companion` | Compatible alias for `extended` (not `agent`): a client already pinned to `companion` keeps its old tool set minus the surface reckoning's own deletions/merges |
| ~~`coding` / `investigation` / `shared` / `platform`~~ | *(deleted)* | The four group surfaces were removed 2026-08-26 (ADR-014 amendment 2, `docs/plans/honesty-fixes.md` HF-5): no client registration, stanza, skill, or doc ever pointed at one, so their zero traffic proved exposure-zero, not demand-zero (law 8) — they go on law 4's ground, a surface with no reachable consumer. The group **taxonomy** (`GroupCoding`, `GroupInvestigation`, …) still classifies every tool. Use `POST /rpc` for everything, `POST /rpc/agent` for the curated hot loop. |

**Agent default (8 tools, surface reckoning F-86, `docs/design/surface-reckoning.md`
§1.1):** `blast_radius`, `find_symbol`, `obligations`, `read_source`, `review_diff`,
`search`, `symbol_context`, `verify_plan` — the answers an agent gets before opting
into anything else (`TestDefaultSurfaceIsEight`).

**Extended/companion allowlist (21 tools):** the default eight above plus
`find_callers`, `findings_near`, `ingest`, `list_services`, `localize`, `outline`,
`pack_context`, `query_symbols`, `references`, `repo_map`, `staleness_report`,
`status`, `test_impact`. `capabilities`, `discoveries`, `entrypoints`, `memory_search`,
`rename_impact` and `safe_delete_check` left the curated surfaces this way too — each
merged into a surviving tool or removed outright by its own SR-* row in wave 2, not
this one; `status` (SR-23, F-11) is the one merge that lands ON this list, since it
answers what `capabilities` used to.
`memory_search` (AM-8) was the third net-new tool on this allowlist; anchored memory
was removed outright by SR-13 (F-45, ADR-053). `read_source` (T5-4, docs/plans/trust-and-tokenomics.md)
is the program's second net-new tool added to this allowlist since its founding 15
(surfaces-week1, ADR-014 amendment) — justified by measured evidence, not added
speculatively: the home-bench H3-1 row found the surface had no raw-text read, costing
16 turns / 601k tokens on a single task that a plain `sed -n` would have answered in one
(docs/plans/home-bench.md).

**Exploration maps (ADR-056, `fathom map`; `docs/usage.md`, "`map`") are on the CLI
and on no MCP surface yet, by design.** The default eight stays eight:
`TestDefaultSurfaceUnchangedByMap` pins the list and fails any map-shaped tool on
it. A map tool, when one is built (a read of `map show`), belongs on this extended
surface (and its `companion` alias): it is added to `extendedTools`, the tool
tables below and the cost classes in the same change, and never to the default.
Until then a sub-agent reaches a map through its shell:
`fathom map show <id> --budget 1200`, which re-resolves every entry at read time.

**Gate semantics: execute-and-flag, not hard-gate** (ADR-014 amendment, 2026-07-16,
surfaces week-1). An off-surface `tools/call` — or `batch` sub-call — still **executes**;
a rejection only teaches an agent to abandon the surface, which looks identical to
no-demand in the call log. The JSONL call log (`serve-http --call-log`, default
`logs/mcp-calls.jsonl` under the repo's state directory — outside the repo tree by
default, O63 item 5, `internal/statedir`) records the pass-through with `off_surface:true`
(`batch` sub-calls are logged individually with `via:"batch"`, each with its own
`off_surface` flag) so demand is measurable instead of argued. Surfaces are **ergonomics
and measurement, never a security boundary** — bare `/rpc` already serves the full,
unauthenticated registry. The pre-amendment hard gate (an off-surface call returns an
error naming the surface) stays available for any future multi-tenant shape, behind the
server's `strictSurfaces` option.

**Budgeted answers (BA-5/BA-9, `docs/design/budgeted-answers.md`; the project-settable
block is experimental).** Class-budgeted tools size their answer in tokens and disclose
elision and per-call response cost: `list` = `query_symbols`, `references`; `composite`
= `pack_context`; `map` = `repo_map` (`find_symbol`, `find_callers`, `symbol_context`,
`outline` are not class-budgeted). Defaults resolve project → user → built-in from the
`budgets: {list, composite, map}` block of `.fathom/capabilities.cue` /
`~/.fathom/capabilities.cue` (closed schema; an unknown key such as `lists` is refused
with a did-you-mean) — honoured by `serve-http`; stdio `serve-mcp` still uses the
built-ins (documented gap). The observatory's Cost panel renders the same figures.

Surface boundaries apply to tools and prompts, not resources: canned prompts are hidden
on surfaces that reference tools not on that surface (prompts are pre-declared
workflows, so they stay hidden rather than execute-and-flag), while `resources/read`
(manifest, per-service summaries) stays available everywhere — it is data, not
capability.

**Suppression disclosure (phase V, `fathom scope`).** When a service has
active scope suppressions, any tool response whose results were narrowed by
them carries a `suppressed` block in its envelope — count, reason, and the
`fathom scope` command to inspect or reverse. Hidden is disclosed, never
silent (law 3): a reader can always tell "no results" from "results
withheld by a suppression someone registered". `read_source` refuses
suppressed and service-ignored paths by name, with the unsuppress command
in the refusal.

**Deferred refresh disclosure (RT-150, responsible tenant).** When
auto-refresh did not run for a service a tool read — the daemon is at `shed`
or above, paused (safe mode), in its warm-up after a start, or the service is
quarantined — the envelope carries `"refresh": {"<service>": "deferred
(<reason>) — served from the index as it stands; freshness not checked …"}`.
The answer is the index as it stands, never presented as fresh.

**Queued refresh disclosure (RT-99, responsible tenant wave D).** A read of a
service whose HEAD drifted never waits for an ingest: it asks the daemon's
refresh queue for one and answers at once from the index as it stands, and the
envelope says so — `"refresh": {"<service>": "queued (index_stale) — served from
the index at <indexed>; HEAD is <head>; …"}` (`refreshing` once the queue runs
it; `queued, paused (shed) …` while the ladder holds background work).
`staleness_report` adds `refresh: "queued" | "refreshing"` beside
`reingest_recommended` while the queue holds the service, and
`refresh_failure: "<reasons> refresh failed at <RFC 3339>: <error>"` when the
service's last queued ingest failed (cleared by the next success) — a
post-commit request has no caller to hear its failure.

### Plain HTTP endpoints (non-MCP)

The separate CLI command `fathom theme-guide` provides an opt-in synthetic
Observatory appearance review at loopback `/dev/theme`; it adds no MCP tool or
daemon route. [App-kit adoption](../app-kit.md#local-theme-guide-integration-2026-09-22)
owns its selected kit identity and experimental preview/reset/export boundary.

`serve-http` also answers a handful of plain (non-JSON-RPC) POST/GET routes
outside the tool registry — hooks call these directly rather than through
`tools/call`:

- `POST /refresh-request` — `{service}` queues a whole-service refresh on the
  daemon's refresh queue (RT-111, the post-commit hook's route) and answers at
  once: `202 {service, state, coalesced?, follow_up?, paused?}` — a request for a
  queued service joins it, one while its ingest runs becomes the one follow-up;
  `503 {refused}` (with `Retry-After`) at refuse, in store distress, and for
  this background request in safe mode or for a quarantined service; `404` for a
  service the daemon does not index (a hook never onboards one).
- `POST /refresh-file` — `{service, file}` re-extracts one file's
  observations/syntax edges and fingerprint (SU-5/SU-6, ADR-041 decisions 3
  and 4); the pre-edit hook's per-file freshness check. It stats before it
  reads: a file over 2 MiB is never opened, is fingerprinted by size, and
  answers `{"refreshed": false, "skipped": "oversize (…; not read)"}` —
  never an error (RT-27). The memoized graph is patched from the file's own
  rows and its packages' rows, never a whole-service symbol load. A file
  outside the corpus an ingest walks (under an always-skipped or
  default-ignored cache directory, or matched by an ignore file) answers
  `{"refreshed": false, "skipped": "outside the corpus: …"}` and any rows an
  earlier refresh left for it are purged.
- `GET /indexed?service=NAME` — `{"service", "indexed"}`: whether the service
  has a `service_meta` row with a non-zero `indexed_at` and at least one
  symbol (RT-80, `pkg/client.IsIndexed`; the count is the watermark-cached
  SQL aggregate (RT-144), never a symbol load; exact id first, then
  case-insensitive). A service ingested from an empty tree, or purged, is
  not indexed — the meaning the `list_services` probe had. The
  meta-only "is X indexed" probe `fathom verify`, the stop and edit hooks,
  `fathom plans`, `fathom init` and the console ask; they fall back to
  `list_services` only against a daemon that answers this route 4xx.
- `GET /identity?path=DIR[&path=TOPLEVEL]` (RT-133) — which indexed service a
  directory is, from the daemon's in-process registry: `{"identity_schema": 1,
  "service", "root", "matched_by": "root"|"basename", "symbols", "count_error",
  "languages", "path"}` (`service` empty when nothing matches; 400 for a relative
  path, 503 + `Retry-After` when the registry cannot be read). The hooks' cached
  identity resolver asks it instead of listing every service; a daemon without
  the route (any 4xx) falls back to `list_services`.
- `GET /findings/summary?service=NAME` — repo-wide open finding-kind claim
  count plus the oldest compiled-at (FL-21, `docs/vision/30-findings-loop.md`
  §3.4).
- `POST /route` — `{"prompt": "..."}` suggests which MCP tool's
  question-shape best fits a prompt (DP-9, ADR-042 decision 2,
  `docs/vision/32-decision-port.md` §2 item 3). One decision-port `choice`
  question over the fixed verb set (`localize`, `find_symbol`, `references`,
  `find_callers`, `obligations`, `staleness_report`, `search`, `none`)
  answers `{"shape", "verb", "confidence", "band", "provider"}` — `provider`
  names the configured `[decide]` provider that answered (`none` | `llm` |
  `typesafe`). Provider `none`, a decider error, or an invalid answer all degrade to
  `{"shape":"","verb":"","confidence":0,"band":"unsure","provider":"none"}`
  (plus an `"error"` field on a genuine failure) rather than a non-200
  status — **this route is a SUGGESTION about the prompt's shape, never a
  code fact**: it never asserts that a symbol exists, a caller calls, or a
  rule is binding, only which lookup verb a question shaped like this one
  usually wants. Every answered decision is appended to `decisions.jsonl`
  (`internal/decide.DefaultRecorderPath`, beside the configured call log)
  under site `"route"`. `fathom prompt-hook` is this route's only caller
  today (DP-10, amended 2026-09-24 — ADR-042 decision 8/CV-M11: route's own
  calibration reading found its `proceed`-band choice right only 45.7% of the
  time, well under the 0.9-right-nine-times-in-ten bar, with no confidence
  cutoff that fixes it): at or above the `proceed` band OR in the `confirm`
  band it injects the SAME hedged line — never a firm instruction, at any
  confidence — and nothing below the confirm floor (0.5) or on any
  failure. **A decision-port call that hard-fails (no answer at all —
  `/route`'s own decider error, a `compile_layer` pre-filter chunk, or an
  `obligations` re-rank chunk) is ALSO logged** (DP-22/DP-23, O109): a row
  naming `site`/`provider`/`questions`/`latency_ms`/`error`,
  `applied:false` — so silence in `decisions.jsonl` means "never asked",
  never "asked and failed".
- `GET /metrics` — text exposition (`fathom_uptime_seconds`,
  `fathom_requests_total`, plus O108's `go_memstats_heap_inuse_bytes`/
  `_heap_sys_bytes`/`_sys_bytes`, `go_goroutines`, `process_uptime_seconds`,
  `fathom_graph_memo_entries`/`_hits_total`/`_misses_total`,
  `fathom_open_stores`, `fathom_autorefresh_total`,
  `fathom_typed_pass_total`, and — vision 35 — `fathom_console_page_views_total{route}`
  and `fathom_console_event_subscribers`; and the tenancy families of RT-124,
  design §13.3: `fathom_tenancy_level`, `fathom_budget_bytes`,
  `fathom_footprint_bytes`, `fathom_children_footprint_bytes`,
  `fathom_host_pressure_level`, `fathom_swap_used_bytes`,
  `fathom_volume_free_bytes{vol}`, `fathom_inflight{class,origin}`,
  `fathom_oldest_inflight_seconds{class}`, `fathom_queue{class}`,
  `fathom_rejected_total{class,reason,origin}`, `fathom_abandoned_total{tool,cause}`,
  `fathom_bytes_read_total{tool}`, `fathom_bytes_written_total{subsystem}`,
  `fathom_cache_bytes{group}`, `fathom_lifecycle_starts_total{reason}`,
  `fathom_memory_released_bytes`, `fathom_gc_cpu_fraction`,
  `fathom_sqlite_memory_used_bytes`, `fathom_quarantined_services` — a reading
  the process cannot take is a `#` comment naming why, never a zero;
  `go_memstats_heap_sys_bytes` carries a HELP line naming it virtual size) — `fathom doctor`'s `daemon_footprint`
  row reads only its `go_memory_limit_bytes` (the GC hint); the footprint it judges
  comes from `/statusz`'s `tenancy` block or the resource ring (`fathom resources`).
- `GET /events` (OC-13) — the console's live-freshness stream: server-sent
  events with an `id:`, a keepalive every 25 s, replay of the last 64 events
  after `Last-Event-ID`, at most 16 subscribers (503 + `Retry-After` past it).
  `ingest`, `compile`, `verify`, `session` and `daemon` events carry names and
  IDs only — never content — and name the panel sources they invalidate.
- The Observatory console's pages (`/`, `/fleet`, `/projects/{svc}`,
  `/impact`, `/sessions`, `/sessions/{id}`, `/brief`, `/wiring`, `/statusz`,
  `/search`) and the legacy `/observatory*` routes (308 for HTML, the old JSON
  for `?format=json`) are HTML surfaces over the same projections, not tools;
  [usage.md](../usage.md#the-observatory-console-and-the-brief) lists them. None
  of them adds an MCP tool or changes a tool's contract.
- `GET /debug/pprof/*` (O108, RT-123) — the stdlib `net/http/pprof` handlers
  (`heap`, `goroutine`, `allocs`, `profile`, `cmdline`, `symbol`, `trace`),
  mounted on the daemon's own mux behind the loopback `Host` check (checked by
  the handlers themselves too, so they serve this machine only); `seconds` is capped at 30 (the response names a cap in
  `X-Fathom-Pprof-Seconds-Capped`) and one profile runs at a time — a second
  concurrent request is 429 with `Retry-After`, never queued.
- `GET /debug/inflight` (RT-123) — the in-flight table as JSON (`class`,
  `tool`, `service`, `origin`, `age_seconds`, oldest first), loopback only;
  it takes no profile slot. `curl -s localhost:7575/debug/pprof/heap -o
  heap.pb.gz && go tool pprof -top heap.pb.gz` reads a live snapshot;
  `docs/guides/fleet-index.md`'s daemon-ops section has the operational
  playbook.

---

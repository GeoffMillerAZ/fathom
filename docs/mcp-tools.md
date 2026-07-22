# MCP Tools Reference

Fathom exposes a JSON-RPC 2.0 MCP server (stdio or HTTP). This page documents every
registered tool, prompt, and resource.

See [usage.md](usage.md) for how to connect a client and switch profiles.
See [architecture.md](architecture.md) for how the tool groups map to the three-layer design.

---

## Tool groups, profiles, and surfaces

Tools are divided into four groups (`GroupShared`, `GroupCoding`, `GroupInvestigation`,
`GroupPlatform`) that describe fathom's internal taxonomy. `set_profile` — the old,
process-global way to filter `tools/list` on stdio — was **removed entirely**
(docs/plans/surfaces-week1.md SW-2; amends ADR-014's stdio clause): no production caller
reaches it any more, so a **stdio** session always sees the full registry. Scoping now
lives entirely over HTTP.

### Path-scoped surfaces (HTTP)

Over **HTTP** (`serve-http`, the shared daemon — default `127.0.0.1:7575`, loopback
only) each request selects an immutable **surface** via the path:

| Surface | Endpoint | Contents |
|---|---|---|
| *(full)* | `POST /rpc` | Every registered tool, unauthenticated — the default for schema-deferring clients such as Claude Code; includes `ingest` (writes) and `compile_layer` (LLM spend) |
| `agent` | `POST /rpc/agent` | The primary 15-tool agent surface (see below) — the user-scope MCP registration points here |
| `companion` | `POST /rpc/companion` | Compatible alias for `agent`: identical 15-tool allowlist |
| `coding` / `investigation` / `shared` / `platform` | `POST /rpc/<name>` | The group's tools plus all shared tools (internal taxonomy; zero observed traffic as of the surfaces-week1 review, kept for completeness) |

**Agent/companion 15-tool allowlist:** `capabilities`, `entrypoints`, `find_callers`,
`find_symbol`, `ingest`, `list_services`, `obligations`, `outline`, `pack_context`,
`query_symbols`, `references`, `repo_map`, `staleness_report`, `symbol_context`,
`verify_plan`.

**Gate semantics: execute-and-flag, not hard-gate** (ADR-014 amendment, 2026-07-16,
surfaces week-1). An off-surface `tools/call` — or `batch` sub-call — still **executes**;
a rejection only teaches an agent to abandon the surface, which looks identical to
no-demand in the call log. The JSONL call log (`serve-http --call-log`, default
`workspace/logs/mcp-calls.jsonl`) records the pass-through with `off_surface:true`
(`batch` sub-calls are logged individually with `via:"batch"`, each with its own
`off_surface` flag) so demand is measurable instead of argued. Surfaces are **ergonomics
and measurement, never a security boundary** — bare `/rpc` already serves the full,
unauthenticated registry. The pre-amendment hard gate (an off-surface call returns an
error naming the surface) stays available for any future multi-tenant shape, behind the
server's `strictSurfaces` option.

Surface boundaries apply to tools and prompts, not resources: canned prompts are hidden
on surfaces that reference tools not on that surface (prompts are pre-declared
workflows, so they stay hidden rather than execute-and-flag), while `resources/read`
(manifest, per-service summaries) stays available everywhere — it is data, not
capability.

---

## Shared tools (GroupShared)

These tools are visible under every group profile and group surface. One exception: the
`agent`/`companion` surface shows only its explicit 15-tool allowlist (see above) — other
shared tools are still callable there, execute-and-flagged rather than hidden.

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `find_symbol` | `service` (req), `name` (req) | `{symbols: [SymbolNode], matched_via, truncated?, did_you_mean?}` | Locate symbol definitions with tiered matching: exact/key-suffix → case-insensitive → substring (capped at 20, tightest first); a full miss returns up to 5 concern-ranked `did_you_mean` suggestions. `matched_via` = `exact` \| `case_insensitive` \| `substring` \| `none`. Also on the agent/companion surface. |
| `capabilities` | — | `{tools, active_surface, active_profile, tool_count, extraction_engines}` | List the tools visible on this request's surface and the active surface/profile. Also on the agent/companion surface — the discovery escape hatch that makes a small surface safe. `extraction_engines` (ADR-021 §4 / XT-5) maps file extension → which engine actually produced its symbols for the languages this migration touched — `"tree-sitter-wasm"` (wazero-executed, `internal/extractor/tsengine`) for `.py`/`.js`/`.jsx`/`.ts`/`.tsx`/`.rb` (exactly the extensions the connector discovers; `.pyi`/`.mjs`/`.cjs` are engine-parseable but not wired, so not claimed). Not a full 24-language registry — see `internal/extractor.EngineProvenance`. |
| `server_stats` | `service` (opt) | `{uptime_seconds, request_count, started_at, unclassified_files?, oversize_files?, classified_zero_files?, typed_edges?}` | Return server uptime and total request count. When the backend supports it, also reports ingest **event counters** (ADR-021 §2): `unclassified_files` (yaml/cue-family files whose content matched no known tool), `oversize_files` (files skipped for exceeding the 2MB extraction cap), and `classified_zero_files` (per-tool map: files that DID classify but whose extractor emitted zero symbols). These count ingest events **since this server process started** — they reset on daemon restart and are not properties of the index (the same file re-ingested twice counts twice). Note: `unclassified` here (classifier verdict on yaml/cue content) is a different measure from cloc's `unclassified` language bucket (path has no registered extractor) — see the `cloc` row. With `service`, adds `typed_edges: {service, total, direct, iface_dispatch, unresolved}` (ADR-021 §3) — all-zero means the typed pass has not run for that store. |
| `staleness_report` | `service` (req) | `{service, source_count, sources, indexed_commit, current_commit, reingest_recommended, layers?}` | Return a staleness summary: distinct source files, last-ingested state, and git-commit drift (`reingest_recommended` is `true` when the repo HEAD has advanced since the last ingest). When compiled claims exist, appends a `layers` map with a per-layer claims summary. |
| `list_sources` | `service` (req) | `{sources: [string]}` | List distinct source file paths ingested for the service |
| `secret_scan` | `service` (req) | `{findings: [Finding]}` | Scan source files for hardcoded tokens, high-entropy strings, AWS keys, JWTs, PEM headers, credential-named variables |
| `shell_out` | `service` (req) | `{findings: [Finding]}` | Find shell-out call sites: `exec.Command` (Go), `subprocess` (Python), `child_process` (JS/TS) |
| `http_client_calls` | `service` (req) | `{findings: [Finding]}` | Find HTTP client call sites: `http.Get/Post/.Do()` (Go), `fetch`/`axios` (JS/TS), `requests` (Python) |
| `import_cycles` | `pkg_imports` (req): `{pkg: [imports]}` | `{cycles: [[string]]}` | Detect import cycles in a package import graph; returns SCCs that form cycles |
| `brief` | `service` (req), `symbols` (opt: `[string]`), `concern` (opt), `budget` (opt, default 16000 chars) | `{brief: string}` | Pre-pack worker context: resolve named symbols, run a concern-scoped query, and return a paste-ready markdown brief. Truncation is always explicit — the output says so when it hits the budget. Demoted off the agent/companion surface (surfaces week-1): zero calls, overlaps `repo_map`+`pack_context`; still execute-and-flags there under the default gate. |
| `obligations` | `service` (req), `file` (req), `max_rules` (opt, default 6) | `{service, file, rules?, compiled_at_watermark?, index_stale?}` | Return compiled claims governing a file at decision time, across every compiled layer. Empty result with no `index_stale` field means a clean file (zero-output-on-clean contract). Each rule carries its source citation and evidence tier; drifted or contradicted rules are flagged. Rules from non-obligation layers carry `kind` (e.g. `"finding"`); finding-kind rules whose scoped code changed since compile carry `code_drifted:true` (remediation-drift annotation, never a status flip). Also on the agent/companion surface. |

`Finding` shape: `{file, line, rule, message}`.

---

## Coding tools (GroupCoding)

Symbol navigation, structural analysis, and impact analysis for active coding tasks.

### Core queries

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `query_symbols` | `service` (req), `concern` (req), `keywords` (opt), `top` (opt, default 20) | `{nodes: [SymbolNode], staleness, truncated}` | Concern-scoped symbol search. Returns ranked SymbolNodes. `concern` is mandatory (ADR-006). The `staleness` block is constant-size: `{sources: [≤3 oldest stale], stale_sources, total_sources, fresh_sources, oldest_ingested_at, reingest_recommended}` — the full source list and commit drift live in `staleness_report`. |
| `find_callees` | `service` (req), `symbol` (req) | `{callees: [string]}` | Return direct callees (symbols called by) the named symbol |
| `call_hierarchy` | `service` (req), `symbol` (req), `depth` (opt, default 2, max 5) | `{key, children: [Tree]}` | Bidirectional call-hierarchy tree rooted at symbol, expanding callers and callees |
| `symbol_context` | `service` (req), `symbol` (req) | `{def, callers, callees, related_tests, typed_edges?}` | Neighbourhood context: definition, direct callers, direct callees, related test functions. `typed_edges` (ADR-021 §3, Go services only) is `{callers: [{key, kind}], callees: [{key, kind}]}` — kind-labeled (`direct` / `iface-dispatch`) typed edges NOT already covered by the plain `callers`/`callees` lists, e.g. an interface-dispatch fan-out invisible to the syntax-only graph. Omitted when the service has no typed edges. |
| `outline` | `service` (req), `file` (req) | `{symbols: [SymbolNode]}` | List all symbols defined in a specific file, ordered by line number |
| `entrypoints` | `service` (req) | `{entrypoints: [SymbolNode]}` | Return likely program entrypoints: `main` functions, HTTP handlers, CLI root commands |

### Quality analysis

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `complexity` | `service` (req), `top_n` (opt, default 10) | `{hotspots: [{name, file, line, complexity}], maintainability_index}` | Cyclomatic-complexity hotspots and Maintainability Index for the service |
| `cognitive` | `service` (req) | `{functions: [{func, file, line, score}]}` | Gocognit-style cognitive complexity for every Go function (non-Go files skipped) |
| `god_object` | `service` (req), `method_threshold` (opt, default 10), `field_threshold` (opt, default 10) | `{god_objects: [{type, methods, fields}]}` | Identify god-object types with too many methods or fields |
| `api_surface` | `service` (req) | `{symbols: [SymbolNode]}` | Exported public API surface: exported funcs, methods, types, interfaces, consts, vars |
| `antipatterns` | `service` (req) | `{findings: [Finding]}` | Detect coding anti-patterns: magic numbers, panic sites, goroutines, regex-in-loop, wall-clock usage, mutex imbalances |
| `cloc` | `service` (req) | `{by_lang: {lang: {blank, comment, code, total}}, total, binary: {files, bytes}, generated: {files, bytes}}` | Count blank, comment, and code lines per language, plus a grand total. Files with no registered extractor appear under the explicit `unclassified` label (a **language** bucket keyed on path — distinct from `server_stats`' `unclassified_files`, which is the yaml/cue content-classifier's event counter); binary files (NUL-byte sniff) are excluded from all line counts and reported only as a file-count/bytes `binary` bucket. Generated files (ADR-021 §2: a "Code generated ... DO NOT EDIT" header — for Go, the official convention line before the package clause; for other languages, a comment-token-prefixed marker in the first lines) get the same treatment via a `generated` bucket — excluded from `by_lang`/`total` so protoc/mockgen/stringer output never inflates the code-you-write line count |
| `deprecation` | `service` (req) | `{findings: [{file, line, symbol}]}` | Scan for usage of deprecated stdlib symbols (e.g. `ioutil`, `os.SEEK_*`) |
| `env_keys` | `service` (req) | `{keys: [{name, file, line}]}` | Extract environment-variable read sites from source files (Go, JS/TS, Python) |

### Impact / call-graph analysis

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `rename_impact` | `service` (req), `symbol` (req) | `{callers: [string]}` | List every call site that needs updating if the symbol is renamed (direct callers) |
| `reachable` | `service` (req), `from` (req), `to` (req) | `{reachable: bool}` | Report whether any directed call path exists from one symbol to another |
| `call_path` | `service` (req), `from` (req), `to` (req) | `{found: bool, path: [string]}` | Return the shortest directed call path between two symbols (BFS, lexicographically smallest among ties) |
| `dead_code` | `service` (req), `entrypoints` (opt: `[string]`) | `{dead: [string]}` | Find symbols neither reachable from any entrypoint nor called by anything in the graph |

---

## Investigation tools (GroupInvestigation)

For cross-service investigation, incident response, and understanding code structure.

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `find_callers` | `service` (req), `symbol` (req) | `{callers: [SymbolNode], typed_callers?: [{key, kind}]}` | Find all symbols that call the named symbol within the service. `typed_callers` (ADR-021 §3, Go services only) lists callers reachable ONLY via a typed edge — e.g. an interface-dispatch caller across an ambiguous method name the syntax-only `callers` list can never include — each labeled `kind: "direct"` or `"iface-dispatch"`. Omitted when the service has no typed edges. Also on the agent/companion surface. |
| `blast_radius` | `service` (req), `changed_files` (req: `[string]`) | `{symbols: [string], tests: [string], files: [string]}` | Blast radius of a set of changed files: symbols defined in those files plus every transitive caller (Go services additionally follow typed edges, ADR-021 §3 — an interface-dispatch caller is included even when the syntax-only graph would refuse it), affected test functions, and touched file paths. |
| `diff_impact` | `service` (req), `changed_files` (req: `[string]`) | `{symbols: [string], tests: [string], files: [string]}` | Alias for `blast_radius` suited to diff/PR workflows; identical semantics, including typed-edge traversal. |
| `community` | `service` (req), `symbol` (req) | `{members: [string]}` | Community of closely related symbols: direct callers, callees, symbols in the same package or file |
| `type_hierarchy` | `service` (req) | `{hierarchy: {interface: [implementors]}}` | Map from interface name to types that implement it, across the entire service |
| `test_impact` | `service` (req), `symbol` (req) | `{tests: [string]}` | Test functions (prefixed `Test`) that transitively reach the symbol via the call graph |
| `verify_plan` | `service` (req); `tasks` (opt: `[{id, title?, spec_ids?, symbol?, reachable_from?, skip_test?, allow_contradiction?, min_tier?}]`) **or** `plan_markdown` (opt: a Markdown checklist — each `- [ ]/[x] title [SPEC-ID] {symbol: Foo}` line becomes a task); `concern` (opt) | `{service, summary:{total,pass,fail,unknown}, tasks:[{id,title,status,label,criteria:[{name,status,detail}]}]}` | Evidence-backed definition-of-done: per-task verdict (`verified` / `claimed-but-absent` / `present-but-untested` / `edge-not-wired` / `contradicted` / `likely-unconfirmed` / `spec-uncovered` / `no-contract`) composed from `find_symbol` + call graph + contradictions + spec coverage, with cited evidence. **STATIC**: `test_reaches` means a test reaches the symbol, not that it passes; `runtime_pass` is always `unknown`. For Go services, `test_reaches` additionally consumes typed call edges (ADR-021 §3): a path crossing an interface-dispatch edge — previously an unconditional refusal for an ambiguous method name shared by multiple receivers — now verifies, with the `test_reaches` criterion's `detail` naming it explicitly: `"... (reached via interface dispatch, N implementer(s))"`. (The tool's own schema is deliberately terse; this row is the full reference.) Promoted onto the agent/companion surface (surfaces week-1): the top non-hook tool in the daemon's call log. |
| `references` | `query` (req); `services` (opt, default all); `word` (opt), `regex` (opt), `case_insensitive` (opt); `max` (opt, default 200) | `{query, definitions:[{service,symbol,file,line,tier}], occurrences:[{service,file,line,text,tier}], count, truncated}` | Fleet-wide occurrence search: every mention of an identifier/string across all services' source — text matches at the `pre-scan` tier plus symbol definitions from the index at their real tier. The structured, evidence-tiered alternative to grepping repo-by-repo. |
| `markers` | `service` (req) | `{markers: [{file, line, kind, text, ticket}]}` | Scan source files for annotation comments: TODO, FIXME, HACK, XXX, BUG, DEPRECATED |
| `routes` | `service` (req) | `{routes: [{method, path, handler, file, line}]}` | Extract HTTP route registrations from Go, JS/TS, and Python source files |
| `endpoint_inventory` | `service` (req) | `{endpoints: [{method, path, handlers, files, lines}]}` | Aggregate HTTP routes across all files into a deduplicated endpoint inventory |
| `dependencies` | `service` (req) | `{dependencies: [{name, version, direct, ecosystem}]}` | Parse dependency lockfiles (go.sum, npm, pip, gem, cargo) and return a SBOM-style dependency list |
| `spec_refs` | `service` (req), `pattern` (opt regex), `spec` (opt) | `{specs: {spec_id: [{file, line}]}, total_refs, files_scanned}` | Scan service source files for spec-ID references. Default pattern covers `[US/EP/FR/NFR-N]`, `MN.N`, and `doc N`. Results are deterministically sorted (spec_id asc, file asc, line asc). Use `spec` to filter to a single normalised ID (e.g. `"M18.2"`). |
| `traverse` | `service` (req), `start` (req), `kinds` (opt: `["calls"\|"imports"\|"spec-ref"\|"route"\|"env-key"]`), `depth` (opt, default 3), `direction` (opt: `"out"\|"in"\|"both"`, default `"out"`) | `{paths: [[{from,to,kind,file,line}…]…], truncated: bool, edge_counts: {kind: n}}` | BFS traversal of the typed edge graph starting at a symbol, file, spec ID, or route. Assembles call edges (symbol→callee), import edges (pkg→pkg), spec-ref edges (file→spec_id), route edges (route→handler), and env-key edges (file→key) on demand. Returns at most 200 paths; `truncated=true` when capped. No silent caps. |
| `history` | `service` (req), `file` (opt, suffix match), `spec` (opt, e.g. `"M18.2"`), `symbol` (opt, resolved to file), `limit` (opt, default 20) | `{commits: [{hash, date, author, message, files_touched_count, spec_ids}], count}` | Return git commit history for a service, ingested during last ingest (up to 500 commits). Filter by file path suffix, spec ID, or symbol name (resolved to its file). Results newest-first. |
| `compile_layer` | `service` (req), `layer` (opt, default `"obligations"`), `dry_run` (opt, **default true**) | `{run_id, service, layer, dry_run, sections_total, sections_compiled, claims_emitted, claims_rejected, tokens_in, tokens_out, cost_usd, duration_ms, model, prompt_version, ran_at}` | Compile a semantic layer for a service. `dry_run` defaults `true` — estimates cost without calling the LLM or writing claims; set `dry_run:false` to execute. Layers are module packs (`docs/layers.md` catalog reference; schema shape in `schema/semantic/schema.cue`, ADR-016): the embedded catalog ships `obligations`, `findings`, `adr-rules`, `spec-bindings`, and the requested layer's own service also unions any project-local `.fathom/layers/` pack (see `fathom layers`) — the requested name routes all the way to the compiler regardless of source. Not on the agent/companion allowlist; execute-and-flags there under the default gate. |

`markers`/`routes`/`dependencies` previously had `test_debt`/`api_routes`/`deps` aliases;
they were deleted as zero-call structural redundancy (surfaces week-1, SW-1) — use the
canonical names.

---

## Platform tools (GroupPlatform)

Fleet-wide operations across all ingested services.

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `list_services` | — | `{services: [{service, symbol_count, languages}], count}` | List all services in the fleet with detected languages and symbol counts |
| `query_cross_service` | `keyword` (req) | `{matches: [{service, symbol, file_path, kind}], count}` | Case-insensitive keyword search across every service in the fleet |
| `fleet_drift` | — | `{drift: [{key, service, value, majority}], count}` | Report version/kind drift: symbols that appear in multiple services with conflicting kinds |
| `cross_repo_callers` | `symbol` (req) | `{callers: [{service, symbol, file_path}], count}` | Find all callers of a symbol across every service via stored call edges (formerly aliased as `platform_callers`, deleted as zero-call structural redundancy — surfaces week-1, SW-1) |
| `cross_repo_blast_radius` | `symbol` (req) | `{hits: [{service, symbol, file}], count}` | Blast radius of a symbol across all services: the symbol in each service plus its cross-service callers |

---

## SymbolNode shape

Queries and navigation tools return `SymbolNode` objects:

```json
{
  "service":   "payment-service",
  "name":      "ValidateToken",
  "kind":      "GoFunc",
  "file_path": "internal/auth/token.go",
  "line":      42,
  "signature": "func ValidateToken(ctx context.Context, tok string) error",
  "tier":      "verified"
}
```

`tier` is the evidence tier: `verified`, `pre-scan`, `doc-claimed`, `sme-stated`, or
`assumed`. See [architecture.md](architecture.md#evidence-tiers-adr-007-adr-009).

---

## Prompts

Fathom registers four canned prompts (via `prompts/list` and `prompts/get`). Each prompt
returns a structured sequence of instructions that guides an AI agent to use the
appropriate fathom tools for the task.

| Prompt | Arguments | Purpose |
|---|---|---|
| `investigate_incident` | `service` (req), `symptom` (req) | Guide an investigation into a production incident: entrypoints → complexity hotspots → call hierarchy → community → staleness check |
| `review_change` | `service` (req), `change` (req) | Review a code change for safety: rename impact → test impact → API surface → complexity → call path |
| `onboard_service` | `service` (req) | Generate an onboarding tour for a new engineer: entrypoints → API surface → sources → type hierarchy → god objects |
| `plan_requirement` | `service` (req), `requirement` (req) | Plan implementation of a new requirement: API surface → type hierarchy → entrypoints → complexity → dead code |

### Using a prompt

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/get",
  "params": {
    "name": "investigate_incident",
    "arguments": {
      "service": "payment-service",
      "symptom": "POST /checkout returning 500 after deploy"
    }
  }
}
```

---

## Resources

Fathom registers two MCP resource types (via `resources/list` and `resources/read`).

| URI | MIME type | Contents |
|---|---|---|
| `fathom:///manifest` | `application/json` | Server manifest: registered tools, active profile, known service IDs, server version |
| `fathom:///service/<id>` | `application/json` | Per-service symbol summary: symbol count, file count, file list, kind breakdown |

### Reading the manifest

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "resources/read",
  "params": { "uri": "fathom:///manifest" }
}
```

Response `contents[0].text` is a JSON object:
```json
{
  "server": "fathom",
  "version": "1.0.0",
  "tools": ["query_symbols", "find_callers", ...],
  "active_profile": "coding",
  "services": ["auth-service", "payment-service"]
}
```

### Reading a service resource

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "resources/read",
  "params": { "uri": "fathom:///service/payment-service" }
}
```

Response `contents[0].text` is a JSON object:
```json
{
  "service": "payment-service",
  "symbol_count": 891,
  "file_count": 38,
  "files": ["internal/auth/token.go", ...],
  "kinds": {"GoFunc": 47, "GoType": 12, ...}
}
```

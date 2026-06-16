# MCP Tools Reference

Sextant exposes a JSON-RPC 2.0 MCP server (stdio or HTTP). This page documents every
registered tool, prompt, and resource.

See [usage.md](usage.md) for how to connect a client and switch profiles.
See [architecture.md](architecture.md) for how the tool groups map to the three-layer design.

---

## Tool groups and profiles

Tools are divided into four groups. Clients call `set_profile` to filter the visible
surface to the group appropriate for the current task.

| Profile | Group constant | Contents |
|---|---|---|
| `shared` | `GroupShared` | Always available — `set_profile`, `find_symbol`, meta tools, security scans, `import_cycles` |
| `coding` | `GroupCoding` | Symbol navigation, complexity, call-graph, impact analysis |
| `investigation` | `GroupInvestigation` | Cross-service, incident investigation, callers, test impact, routes, deps |
| `platform` | `GroupPlatform` | Fleet-wide: list services, cross-service search, drift, cross-repo callers |

---

## Shared tools (GroupShared)

These tools are visible regardless of the active profile.

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `set_profile` | `profile`: `"coding"\|"investigation"\|"shared"\|"platform"` | `{profile, ok}` | Switch active profile to filter the tool list |
| `find_symbol` | `service` (req), `name` (req) | `{symbols: [SymbolNode]}` | Locate symbol definitions matching a bare name or SymbolKey in the service |
| `capabilities` | — | `{tools, active_profile, tool_count}` | List all currently visible tools and the active profile |
| `server_stats` | — | `{uptime_seconds, request_count, started_at}` | Return server uptime and total request count |
| `staleness_report` | `service` (req) | `{service, source_count, sources, indexed_commit, current_commit, reingest_recommended}` | Return a staleness summary: distinct source files, last-ingested state, and git-commit drift (`reingest_recommended` is `true` when the repo HEAD has advanced since the last ingest) |
| `list_sources` | `service` (req) | `{sources: [string]}` | List distinct source file paths ingested for the service |
| `secret_scan` | `service` (req) | `{findings: [Finding]}` | Scan source files for hardcoded tokens, high-entropy strings, AWS keys, JWTs, PEM headers, credential-named variables |
| `shell_out` | `service` (req) | `{findings: [Finding]}` | Find shell-out call sites: `exec.Command` (Go), `subprocess` (Python), `child_process` (JS/TS) |
| `http_client_calls` | `service` (req) | `{findings: [Finding]}` | Find HTTP client call sites: `http.Get/Post/.Do()` (Go), `fetch`/`axios` (JS/TS), `requests` (Python) |
| `import_cycles` | `pkg_imports` (req): `{pkg: [imports]}` | `{cycles: [[string]]}` | Detect import cycles in a package import graph; returns SCCs that form cycles |
| `brief` | `service` (req), `symbols` (opt: `[string]`), `concern` (opt), `budget` (opt, default 16000 chars) | `{brief: string}` | Pre-pack worker context: resolve named symbols, run a concern-scoped query, and return a paste-ready markdown brief. Truncation is always explicit — the output says so when it hits the budget. |

`Finding` shape: `{file, line, rule, message}`.

---

## Coding tools (GroupCoding)

Symbol navigation, structural analysis, and impact analysis for active coding tasks.

### Core queries

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `query_symbols` | `service` (req), `concern` (req), `keywords` (opt), `top` (opt, default 20) | `{nodes: [SymbolNode], staleness, truncated}` | Concern-scoped symbol search. Returns ranked SymbolNodes. `concern` is mandatory (ADR-006). |
| `find_callees` | `service` (req), `symbol` (req) | `{callees: [string]}` | Return direct callees (symbols called by) the named symbol |
| `call_hierarchy` | `service` (req), `symbol` (req), `depth` (opt, default 2, max 5) | `{key, children: [Tree]}` | Bidirectional call-hierarchy tree rooted at symbol, expanding callers and callees |
| `symbol_context` | `service` (req), `symbol` (req) | `{def, callers, callees, related_tests}` | Neighbourhood context: definition, direct callers, direct callees, related test functions |
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
| `cloc` | `service` (req) | `{by_lang: {lang: {blank, comment, code, total}}, total}` | Count blank, comment, and code lines per language, plus a grand total |
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
| `find_callers` | `service` (req), `symbol` (req) | `{callers: [SymbolNode]}` | Find all symbols that call the named symbol within the service |
| `community` | `service` (req), `symbol` (req) | `{members: [string]}` | Community of closely related symbols: direct callers, callees, symbols in the same package or file |
| `type_hierarchy` | `service` (req) | `{hierarchy: {interface: [implementors]}}` | Map from interface name to types that implement it, across the entire service |
| `test_impact` | `service` (req), `symbol` (req) | `{tests: [string]}` | Test functions (prefixed `Test`) that transitively reach the symbol via the call graph |
| `verify_plan` | `service` (req); `tasks` (opt: `[{id, title?, spec_ids?, symbol?, reachable_from?, skip_test?, allow_contradiction?, min_tier?}]`) **or** `plan_markdown` (opt: a Markdown checklist — each `- [ ]/[x] title [SPEC-ID] {symbol: Foo}` line becomes a task); `concern` (opt) | `{service, summary:{total,pass,fail,unknown}, tasks:[{id,title,status,label,criteria:[{name,status,detail}]}]}` | Evidence-backed definition-of-done: per-task verdict (`verified` / `claimed-but-absent` / `present-but-untested` / `edge-not-wired` / `contradicted` / `likely-unconfirmed` / `spec-uncovered` / `no-contract`) composed from `find_symbol` + call graph + contradictions + spec coverage, with cited evidence. **STATIC**: `test_reaches` means a test reaches the symbol, not that it passes; `runtime_pass` is always `unknown`. |
| `references` | `query` (req); `services` (opt, default all); `word` (opt), `regex` (opt), `case_insensitive` (opt); `max` (opt, default 200) | `{query, definitions:[{service,symbol,file,line,tier}], occurrences:[{service,file,line,text,tier}], count, truncated}` | Fleet-wide occurrence search: every mention of an identifier/string across all services' source — text matches at the `pre-scan` tier plus symbol definitions from the index at their real tier. The structured, evidence-tiered alternative to grepping repo-by-repo. |
| `markers` | `service` (req) | `{markers: [{file, line, kind, text, ticket}]}` | Scan source files for annotation comments: TODO, FIXME, HACK, XXX, BUG, DEPRECATED |
| `test_debt` | `service` (req) | same as `markers` | Alias for `markers` |
| `routes` | `service` (req) | `{routes: [{method, path, handler, file, line}]}` | Extract HTTP route registrations from Go, JS/TS, and Python source files |
| `api_routes` | `service` (req) | same as `routes` | Alias for `routes` |
| `endpoint_inventory` | `service` (req) | `{endpoints: [{method, path, handlers, files, lines}]}` | Aggregate HTTP routes across all files into a deduplicated endpoint inventory |
| `dependencies` | `service` (req) | `{dependencies: [{name, version, direct, ecosystem}]}` | Parse dependency lockfiles (go.sum, npm, pip, gem, cargo) and return a SBOM-style dependency list |
| `deps` | `service` (req) | same as `dependencies` | Alias for `dependencies` |
| `spec_refs` | `service` (req), `pattern` (opt regex), `spec` (opt) | `{specs: {spec_id: [{file, line}]}, total_refs, files_scanned}` | Scan service source files for spec-ID references. Default pattern covers `[US/EP/FR/NFR-N]`, `MN.N`, and `doc N`. Results are deterministically sorted (spec_id asc, file asc, line asc). Use `spec` to filter to a single normalised ID (e.g. `"M18.2"`). |
| `traverse` | `service` (req), `start` (req), `kinds` (opt: `["calls"\|"imports"\|"spec-ref"\|"route"\|"env-key"]`), `depth` (opt, default 3), `direction` (opt: `"out"\|"in"\|"both"`, default `"out"`) | `{paths: [[{from,to,kind,file,line}…]…], truncated: bool, edge_counts: {kind: n}}` | BFS traversal of the typed edge graph starting at a symbol, file, spec ID, or route. Assembles call edges (symbol→callee), import edges (pkg→pkg), spec-ref edges (file→spec_id), route edges (route→handler), and env-key edges (file→key) on demand. Returns at most 200 paths; `truncated=true` when capped. No silent caps. |
| `history` | `service` (req), `file` (opt, suffix match), `spec` (opt, e.g. `"M18.2"`), `symbol` (opt, resolved to file), `limit` (opt, default 20) | `{commits: [{hash, date, author, message, files_touched_count, spec_ids}], count}` | Return git commit history for a service, ingested during last ingest (up to 500 commits). Filter by file path suffix, spec ID, or symbol name (resolved to its file). Results newest-first. |

---

## Platform tools (GroupPlatform)

Fleet-wide operations across all ingested services.

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `list_services` | — | `{services: [{service, symbol_count, languages}], count}` | List all services in the fleet with detected languages and symbol counts |
| `query_cross_service` | `keyword` (req) | `{matches: [{service, symbol, file_path, kind}], count}` | Case-insensitive keyword search across every service in the fleet |
| `fleet_drift` | — | `{drift: [{key, service, value, majority}], count}` | Report version/kind drift: symbols that appear in multiple services with conflicting kinds |
| `cross_repo_callers` | `symbol` (req) | `{callers: [{service, symbol, file_path}], count}` | Find all callers of a symbol across every service via stored call edges |
| `platform_callers` | `symbol` (req) | same as `cross_repo_callers` | Alias for `cross_repo_callers` |
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

Sextant registers four canned prompts (via `prompts/list` and `prompts/get`). Each prompt
returns a structured sequence of instructions that guides an AI agent to use the
appropriate sextant tools for the task.

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

Sextant registers two MCP resource types (via `resources/list` and `resources/read`).

| URI | MIME type | Contents |
|---|---|---|
| `sextant:///manifest` | `application/json` | Server manifest: registered tools, active profile, known service IDs, server version |
| `sextant:///service/<id>` | `application/json` | Per-service symbol summary: symbol count, file count, file list, kind breakdown |

### Reading the manifest

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "resources/read",
  "params": { "uri": "sextant:///manifest" }
}
```

Response `contents[0].text` is a JSON object:
```json
{
  "server": "sextant",
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
  "params": { "uri": "sextant:///service/payment-service" }
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

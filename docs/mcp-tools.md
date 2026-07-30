# MCP Tools Reference

Fathom exposes a JSON-RPC 2.0 MCP server (stdio or HTTP). This page documents every
registered tool, prompt, and resource.

See [usage.md](usage.md) for how to connect a client and switch profiles.
See [architecture.md](architecture.md) for how the tool groups map to the three-layer design.

---

## Tool groups, profiles, and surfaces

Tools are divided into five groups (`GroupShared`, `GroupCoding`, `GroupInvestigation`,
`GroupPlatform`, `GroupModules`) that describe fathom's internal taxonomy. `set_profile` — the old,
process-global way to filter `tools/list` on stdio — was **removed entirely**
(docs/plans/surfaces-week1.md SW-2; amends ADR-014's stdio clause): no production caller
reaches it any more, so a **stdio** session always sees the full registry. Scoping now
lives entirely over HTTP.

### Path-scoped surfaces (HTTP)

Over **HTTP** (`serve-http`, the shared daemon — default `127.0.0.1:7575`, loopback
only) each request selects an immutable **surface** via the path:

| Surface | Endpoint | Contents |
|---|---|---|
| *(full)* | `POST /rpc` | Every registered tool, unauthenticated — the default for schema-deferring clients such as Claude Code; includes `ingest` (writes) and `compile_layer` (LLM spend). Hides the four module tools instead when the daemon is in hosted-modules mode (see below) |
| `agent` | `POST /rpc/agent` | The primary agent surface (see below) — the user-scope MCP registration points here |
| `companion` | `POST /rpc/companion` | Compatible alias for `agent`: identical allowlist |
| `coding` / `investigation` / `shared` / `platform` | `POST /rpc/<name>` | The group's tools plus all shared tools (internal taxonomy; zero observed traffic as of the surfaces-week1 review, kept for completeness) |
| `modules` | `POST /rpc/modules` | The module host's floor tool set (docs/plans/gke-module-host.md) — see "Module host tools" below for the tool list and the hosted-mode restriction |

**Agent/companion allowlist (16 tools):** `capabilities`, `entrypoints`, `find_callers`,
`find_symbol`, `ingest`, `list_services`, `obligations`, `outline`, `pack_context`,
`query_symbols`, `read_source`, `references`, `repo_map`, `staleness_report`,
`symbol_context`, `verify_plan`. `read_source` (T5-4, docs/plans/trust-and-tokenomics.md)
is the program's second net-new tool added to this allowlist since its founding 15
(surfaces-week1, ADR-014 amendment) — justified by measured evidence, not added
speculatively: the home-bench H3-1 row found the surface had no raw-text read, costing
16 turns / 601k tokens on a single task that a plain `sed -n` would have answered in one
(docs/plans/home-bench.md).

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
`agent`/`companion` surface shows only its explicit allowlist (see above) — other
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
| `obligations` | `service` (req), `file` (req), `max_rules` (opt, default 6) | `{service, file, rules?, compiled_at_watermark?, index_stale?, displaced?, displaced_repo?, displaced_module?, module_notes?}` | Return compiled claims governing a file at decision time, across every compiled layer. Empty result with no `index_stale` field means a clean file (zero-output-on-clean contract). Each rule carries its source citation, evidence tier, and a `layer` label naming which manifest layer compiled it (T5-2/O14 — `"obligations"`, `"adr-rules"`, `"spec-bindings"`, `"findings"`, …); drifted or contradicted rules are flagged. `displaced` (present only when > 0) counts served-eligible rules the `max_rules` cap dropped this call — a recompile blending a new layer in can evict a previously-served rule, and this makes the eviction visible instead of silent. Rules from non-obligation layers carry `kind` (e.g. `"finding"`); finding-kind rules whose scoped code changed since compile carry `code_drifted:true` (remediation-drift annotation, never a status flip). **Phase O (docs/design/platform-modules.md):** when the calling service's `.fathom/modules.cue` binds a mounted platform module, matching module rules are unioned in, each carrying `module:"<name>"` (module-tagged, absent for every repo-local rule); `displaced` is then the TOTAL repo+module count (a compatible superset of its pre-phase-O meaning), split by `displaced_repo`/`displaced_module`; a binding naming a module this daemon does NOT have mounted produces a `module_notes` entry (`{module, note}`) — never silence. Also on the agent/companion surface, and (module-tagged, budget-capped) on the PreToolUse guard hook (docs/design/guarded-edits.md) for scopes named in `.fathom/guard.cue`. |
| `ingest` | `service` (req), `path` (opt, absolute corpus root — required only when onboarding a NEW service) | `{service, ok, unclassified_files?, oversize_files?}` | Re-ingest the named service to refresh the symbol store, or onboard a brand-new service into the index when `path` is given. The optional counters are a before/after delta of this call's ingest-event stats (ADR-021 §2), present only when the backend supports them. Also on the agent/companion surface. |
| `repo_map` | `service` (req), `budget` (opt, default 4000) | `{map: string, budget}` | Build a budget-bounded symbol map ranked by outbound call-edge count (most-connected symbols first), suitable for LLM context assembly. Also on the agent/companion surface. |
| `pack_context` | `service` (req), `concern` (req), `budget` (opt, default 4000) | `{included: [{id,text,score}], tokens, dropped, budget}` | Pack ranked evidence items from a concern-scoped query into a token budget, returning the highest-scoring items that fit. Also on the agent/companion surface. |
| `license` | `service` (req) | `{spdx}` | Detect the SPDX license identifier from the service's `LICENSE`/`LICENSE.txt`/`LICENSE.md`/`LICENCE` source file; `"UNKNOWN"` when none is found. |
| `license_policy` | `service` (req), `allowed` (req: `[string]`, SPDX identifiers) | `{spdx, ok, reason, allowed}` | Evaluate the detected license against an allowed list. `allowed` is `required` in the schema, but the analyzer itself is what actually protects the answer: an empty `allowed` **fails closed** (`ok:false`, reason `"no licenses are allowed"`) rather than a false permit — see the C4-5 honesty audit (docs/plans/review-verb.md). |
| `security_rollup` | `service` (req), `allowed_licenses` (opt: `[string]`) | `{secrets, shell_outs, http_calls, license, license_ok, license_reason, secret_count, shell_out_count, http_call_count}` | Aggregate security posture in one report: `secret_scan` + `shell_out` + `http_client_calls` findings plus the detected license and its policy check. `license_ok` shares `license_policy`'s fail-closed contract — an empty `allowed_licenses` reports `license_ok:false` with a reason, never a false "ok" (C4-5 audit). |
| `batch` | `calls` (req: `[{name, arguments}]`) | `{results: [{name, result?, error?}], count}` | Execute N tool calls in a single request; a per-call error is reported in that call's result entry rather than aborting the batch. Sub-calls are logged individually in the call log (`via:"batch"`) with their own `off_surface` flag, so surface telemetry reflects the tools a batch actually exercised. |
| `estimate_cost` | `text` (req), `rate_per_ktok` (opt, default 0.002) | `{tokens, usd, rate}` | Estimate the token count and USD cost of a text string at a given per-thousand-token rate; records the estimate into the running cost ledger `cost_report` reads. |
| `cost_report` | — | `{tools: [{tool,tokens,usd}], total_tokens, total_usd}` | Return accumulated token counts and USD costs recorded by `estimate_cost` and other cost-tracking tools, since the server process started. |

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
| `symbol_context` | `service` (req), `symbol` (req), `snippet` (opt bool, default `true`) | `{def, callers, callees, related_tests, typed_edges?, snippet?, snippet_truncated?}` | Neighbourhood context: definition, direct callers, direct callees, related test functions. `typed_edges` (ADR-021 §3, Go services only) is `{callers: [{key, kind}], callees: [{key, kind}]}` — kind-labeled (`direct` / `iface-dispatch`) typed edges NOT already covered by the plain `callers`/`callees` lists, e.g. an interface-dispatch fan-out invisible to the syntax-only graph. Omitted when the service has no typed edges. `snippet` (T5-6, docs/plans/trust-and-tokenomics.md) is the definition's source, inline, line-numbered, capped at 40 lines — read via the same safe, bounded path as `read_source`, so a define-and-show answer completes in ONE turn; `snippet_truncated:true` marks a snippet that stopped at the cap with more file below it (a 100-line function must never LOOK 40 lines long — most defs carry `end_line` 0, so the flag is the only clipping signal). Pass `snippet:false` to omit the snippet entirely (smaller payload). Additive and honest: omitted (never faked) when the definition has no file/line or its source isn't retrievable (no `ServiceMeta` corpus root, file missing, binary, etc.) — `def`/`callers`/`callees` are unaffected either way. |
| `outline` | `service` (req), `file` (req) | `{symbols: [SymbolNode]}` | List all symbols defined in a specific file, ordered by line number |
| `entrypoints` | `service` (req) | `{entrypoints: [SymbolNode]}` | Return likely program entrypoints: `main` functions, HTTP handlers, CLI root commands |
| `read_source` | `service` (req), `file` (req, repo-relative), `start_line` (opt), `end_line` (opt) | `{file, lines: [{line, text}], start_line, end_line, total_lines, clamped_eof, note?}` | Bounded, read-only raw-text read from the service's INDEXED corpus (T5-4, docs/plans/trust-and-tokenomics.md) — the agent surface's `sed -n` equivalent, line-numbered so results are pinnable as citations. No `start_line`/`end_line`: a capped whole-file view from line 1. Capped at 200 lines / 16000 bytes per call, whichever is hit first; `end_line == total_lines` in the response means EOF was reached; a single line longer than the byte cap comes back TRUNCATED to it with the note set — never whole (minified JS cannot void the cap). **Readable = what ingest walks, enforced:** refuses (RPC error) a `.fathomignore`'d path (the same compiled matcher `Discover` applies, file and ancestor dirs), the always-skipped directories (`vendor`, `.git`, `testdata`, `generated`, `node_modules`, `.idea`, `.vscode`, `.claude`), `.env`-pattern files (unconditional defense in depth — secrets-shaped files are never served even when a repo forgot to ignore them; other dot-paths like `.github/` stay readable since they ARE indexed), a file over the 2MB read ceiling (`os.Stat`-checked before any read; equals the indexer's own extraction cap, so such files aren't in the index either), a binary file (`connector.IsBinary`), an absolute `file`, path traversal, a symlink resolving outside the corpus root (`filepath.EvalSymlinks` containment on both the root and the candidate — a same-corpus symlink target reads fine), and an EXPLICIT range wider than the line cap (narrow it, or make repeated calls — never silently narrowed). An explicit `start_line`/`end_line` landing beyond EOF is clamped, not refused, and noted (`clamped_eof:true`); running past the line/byte cap is separately noted. An empty file reports `total_lines: 0`. The corpus root is resolved from `ServiceMeta` with the same absolute-path discipline as `arch_conformance`'s project-rule loader (O10/C4 class): a service with no recorded root, or a pre-O10 relative one, is refused with a re-ingest pointer rather than resolved against the daemon's ambient CWD. Also on the agent/companion surface. |

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
| `arch_conformance` | `service` (req), `rules` (opt: `[{from_pkg,to_pkg,allow}]`, `"*"` wildcard), `deny_by_default` (opt bool) | `{violations: [{from,to,reason}], count, rule_source, checked, reason?}` | Check the service's call-graph edges against architectural layering rules. When `rules` is omitted, falls back to the service's own `.fathom/rules/archconformance.cue` project rule pack (C4-2/C4-3); `rule_source` names which source was used (`"inline"` \| `"project"` \| `"none"`), and `checked` is `false` (with a `reason`) when neither source had anything to check — never a clean-shaped `{violations:[],count:0}` for a check that didn't run (closes the C4-5 confirmed-bug instance). **Disclosed gap:** edges are built from function-origin/function-target call keys only — a method's `Package` is its receiver type, not its declaring Go package, so method-origin and method-target edges aren't yet normalized into the checked set (tracked fix: both-sides symbol-key→declaring-package resolution). |
| `naming` | `service` (req) | `{issues: [{symbol, rule, message}], count}` | Check Go symbol naming conventions: exported PascalCase, no underscores, correct acronym capitalization, interface `-er` suffix |
| `struct_tags` | `service` (req) | `{issues: [{file, line, field, message}], count}` | Audit Go struct tags: missing `json` tags on exported fields, malformed tag syntax |
| `err_audit` | `service` (req) | `{findings: [{file, line, message}], count}` | Audit Go error-handling patterns: ignored errors, log-only handlers, naked error returns |
| `doc_coverage` | `service` (req) | `{covered: [string], uncovered: [string], pct}` | Report documentation coverage of exported symbols: which have a doc comment (non-empty `Signature`), which don't, and the overall percentage |
| `stale_docs` | `service` (req) | `{drifts: [{file, line, ref}], count}` | Scan Markdown source files for stale references: inline code spans or link targets that no longer correspond to a known symbol or file |
| `clones` | `service` (req) | `{groups: [{hash, files: [string]}], count}` | Detect structurally duplicate code (clone groups) across source files using content-normalised hashing |
| `hotspots` | `service` (req), `top_n` (opt, default 20) | `{hotspots: [{name, file, line, complexity}]}` | Return top-N complexity-based hotspots (highest cyclomatic complexity). Git-churn hotspots require direct repo git access and are not available here. |
| `migration_targets` | `service` (req), `deprecated_pkgs` (opt: `[string]`) | `{targets: [{symbol, reason}], count}` | Identify symbols that depend on deprecated packages (`io/ioutil`, `golang.org/x/net/context`, `golang.org/x/oauth2/internal`, `github.com/golang/protobuf`). `deprecated_pkgs` is **additive**: it is merged with the built-in default list (deduplicated), never a replacement — the default list is always checked, matching the parameter's own description. |
| `sql_map` | `service` (req) | `{tables: [{name, file, line}], count}` | Extract SQL table references from source files (`FROM`/`JOIN`/`INTO`/`UPDATE` patterns) |
| `field_usage` | `service` (req), `struct_name` (req) | `{fields: [{field, used}], struct}` | Report which fields of a named Go struct are used vs unused within its source file |
| `get_go_symbols` | `service` (req), `kinds` (opt: `[string]`) | `{symbols: [SymbolNode], count}` | Return all Go-kind symbols (`GoFunc`, `GoMethod`, `GoType`, `GoInterface`, `GoConst`, `GoVar`), optionally filtered to specific kinds. An empty/omitted `kinds` returns every Go kind — "no filter requested", not a policy no-op. |
| `api_diff` | `service_a` (req), `service_b` (req) | `{added: [string], removed: [string], changed: [string]}` | Compute the API surface diff between two services: exported symbols added, removed, and changed between the baseline (`service_a`) and the comparison service (`service_b`) |
| `dep_drift` | `service_a` (req), `service_b` (req) | `{drift: [{name, current, baseline}], count}` | Compare dependency versions between two services' lockfiles (`service_b` against `service_a` as baseline) and report packages with version mismatches |
| `package_deps` | `service` (req) | `{package_deps: {pkg: [imported_pkg, ...]}}` | Build a package-level dependency graph from symbol packages and their call edges |
| `proto_contract` | `service` (req) | `{contracts: [{file, package, services, messages}], count}` | Parse `.proto` source files and return the extracted services, RPCs, messages, and fields |
| `proto_conformance` | `service` (req) | `{issues: [{file, location, rule, message}], count}` | Check `.proto` files for naming convention violations (PascalCase services/messages/RPCs, snake_case fields) |
| `schema_export` | `service` (req), `format` (opt: `"json"` (default) \| `"dot"`) | `{format, output}` | Export the service's symbol graph as JSON or Graphviz DOT |

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
| `review_diff` | `service` (req), `changed_files` (req: `[string]`), `concern` (opt) | `{service, changed_files, dimensions:[{name,checked,reason?,count,findings:[{kind,symbol?,file?,line?,message,severity,citation?,claim_id?,status?}],tests?,files?}], summary, findings_persisted}` | The review-verb composed tool (C1-1, `docs/plans/review-verb.md`): diff impact + obligations/findings scoped to the changed files + architecture conformance (project rule pack) + spec coverage, in ONE cited verdict. Each `dimensions[]` entry reports `checked:false` plus `reason` instead of a clean-shaped pass when it cannot run (no rule pack, backend doesn't serve semantic layers, service never ingested). **Writes to the claims store** when the backend supports persistence (`findings_persisted` reports it): the `arch_conformance` and `spec_coverage` findings — the deterministic-oracle dimensions only, never `obligations` (which re-serves existing claims) or `impact` context — persist as content-addressed claims on the `findings` layer (tier `verified`), each with a hashed line-citation and a scope-fingerprint drift anchor so the daemon's post-refresh sweep/drift passes verify rather than falsify them; review-minted claims whose finding a later run over the same files no longer emits are `superseded` (absent = fixed). A finding refuted via `fathom claim refute` / the `obligations` tool's feedback action carries `status:"refuted"` (excluded from `count`, still listed) across every later re-review that re-emits the identical finding. CLI mirror: `fathom review --store DB --service S --since <ref>` (`--no-persist` for a pure report — the pre-push summary hook uses it). |
| `findings_near` | `service` (req), `subject` (req: symbol key or repo-relative file path), `hops` (opt, default 2), `include_superseded` (opt, default false) | `{subject, hops, include_superseded, found, note?, findings:[{claim_id,subject,class,rule,scope,status,hops,superseded,compiled_at}]}` | Findings-as-evidence's differentiating verb (docs/design/findings-as-evidence.md §3.6): review/audit findings on the `findings` layer whose cited scope lies within `hops` call-graph hops of `subject` (bidirectional — callers and callees), ranked by proximity then recency. `found:false` with an honest `note` when `subject` resolves to no known symbol or file. Superseded/refuted findings are excluded unless `include_superseded:true`, and are ALWAYS labeled `superseded:true` when included — precedent, never mistaken for something still live. Reuses the existing call-graph traversal; adds no new storage. CLI mirror: `fathom findings near <symbol\|file> --store DB --service S [--hops N] [--include-superseded]`. |
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
| `spec_coverage` | `service` (req), `spec_ids` (req: `[string]`), `prefixes` (opt: `[string]`, default US/EP/FR/NFR) | `{coverage: {spec_id: [symbol]}, uncovered: [string], orphans: [string]}` | Build a spec coverage matrix: for each declared spec ID, which symbols implement it; which declared specs are uncovered; which referenced IDs in the code are orphans (not in the declared `spec_ids` set) |
| `spec_status` | `service` (req), `spec_ids` (req: `[string]`) | `{statuses: {spec_id: "covered"\|"uncovered"}}` | Return the coverage status for each spec ID in the list |
| `traceability` | `service` (req), `prefixes` (opt: `[string]`, default US/EP/FR/NFR) | `{refs: {file_path: [spec_id, ...]}}` | Extract spec-ID references from source files and build a file → spec-ID traceability map |
| `propagate_concern` | `service` (req), `concern` (req), `seed` (req) | `{symbols: [string]}` | Walk the call graph backward from a seed symbol (`package.Name`), collecting every symbol whose name/package contains the concern token (case-insensitive substring), plus their transitive callers |
| `evidence_summary` | `service` (req) | `{summary: {tier: count}}` | Return per-evidence-tier observation counts for a service (`verified`, `pre-scan`, `doc-claimed`, `sme-stated`, `assumed`) |
| `contradiction_scan` | `service` (req), `concern` (req) | `{contradictions: [...], count}` | Scan concern-scoped query results for contradictions: claims with conflicting evidence for the same symbol, plus symbols that appear with conflicting `kind`s across nodes (`type:"kind_conflict"` entries) |
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

## Module host tools (GroupModules)

The GKE **module host** (docs/plans/gke-module-host.md; docs/spec/store-format-v0.md;
docs/spec/expert-module-v0.md) mounts N conforming expert-module stores **READ-ONLY**
beside the main fleet store, via `serve-http --module <path>` (repeatable). Every
result carries per-module provenance — never merged into an ambiguous unattributed
set — and every claim keeps its citation, evidence tier, and non-fresh status
(store-format-v0.md §3 Server conformance, generalized to N stores).

| Tool | Arguments | Returns | Purpose |
|---|---|---|---|
| `list_modules` | — | `{modules: [{module, path, service?, indexed_commit?, indexed_at?, error?}]}` | Enumerate every mounted module's (module, corpus/service) pairs and the ingest-time watermark each was compiled at (expert-module-v0.md §4.1). A module whose own discovery failed contributes one `error`-carrying row rather than aborting the whole call. |
| `module_obligations` | `module` (opt — omit to fan out across every mounted module), `file` (req), `max` (opt) | `{results: [{module, service, obligations, error?}]}` | Scope-lookup query (expert-module-v0.md §4.2, the `obligations`-shaped pattern): claims governing `file`, each still carrying citation/tier/status, tagged with the module and service that served it. |
| `module_query` | `module` (opt), `text` (req; alias `concern`), `max` (opt) | `{results: [{module, service, watermark, nodes?, staleness, claims?, error?}]}` | Free-text query — `module_obligations` alone requires a scope-glob `file` and cannot answer a free-text question. Fans out the concern-scoped symbol engine (`query_symbols`' own engine) AND a claim-text substring match per (module, service) pair; a pair answers via ranked symbols, matched claims, both, or neither (omitted when neither). `watermark` (that pair's `service_meta.indexed_commit`) rides on every entry regardless of whether it answered. Each `claims[]` entry is `{claim_id, text, src, anchor, tier, status?, kind?}` — a **wire shape distinct from `module_obligations`'** (`{rule, src, tier, status?, kind?}`, unchanged): `claim_id` is the durable `core.SemanticClaim.ClaimID` (the same identity `fathom claim refute`/`claim_events` key on — a consumer's cite tokens and feedback path need this, not just the rule text), and `anchor` is the literal cited text from `claim_links.anchor_text` (`""` for a legacy hash-only citation that predates it, never a guess). |
| `module_staleness` | `module` (req) | `{module, corpora: [{module, service, indexed_commit?, current_commit?, status}]}` | Per-(module, service) staleness (expert-module-v0.md §4.3). A mounted module is a **detached corpus** by default: `status` reads `"unknown-detached"` unless the module's `service_meta.path` is present and re-checkable on THIS machine — `"fresh"`/`"drifted"` only then. A `manifest:`-watermarked module is re-checked via the same content-addressed algorithm it was compiled under, **never** a live git HEAD (a corpus nested inside some unrelated enclosing git checkout would otherwise false-drift permanently). |

**Mount conformance (`--module <path>`, repeatable):** checked at mount time against
store-format-v0.md §2's markers (`application_id == 0x46415448` AND `user_version >= 1`)
and the required-schema tables for the level the host requires (`L0` by default). A
non-conforming or unreadable store **aborts serve-http startup loudly** — never a
partial mount. Every mount opens the store file genuinely read-only at the SQLite/VFS
layer (`file:<path>?mode=ro`) — there is no write path to a mounted module, structurally
(docs/vision/12-governed-knowledge-engine.md law 9).

**Hosted mode (`--modules-token-env <NAME>`):** names an environment variable holding a
bearer token required on `POST /rpc/modules` (constant-time compare; the token itself is
never a flag/config value, never logged). **Default off** — the local, loopback-only,
unauthenticated daemon is unaffected. When a token IS configured, hosted mode is a
strictly narrower surface, not merely an auth check bolted onto the existing one:

- `/rpc/modules` serves **only** `list_modules`, `module_obligations`, `module_query`,
  `module_staleness`, `capabilities`, `server_stats` — never the rest of `GroupShared`
  (no `ingest`, no `compile_layer`, no `batch`, no fleet-backed reads like `repo_map` /
  `find_symbol` / `staleness_report` that a modules-token holder could otherwise pivot
  through to read the fleet's own indexed services).
- Off-allowlist calls on `/rpc/modules` are **hard-gated** (a named error), never
  execute-and-flag — the one surface in this server where that ADR-014 ergonomics
  choice is deliberately overridden.
- The four module tools become unreachable from **every other path** — bare `/rpc`,
  any other named surface, or a `batch` sub-call — so there is never an unauthenticated
  route to module data once a token is configured.

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
  "end_line":  0,
  "signature": "func ValidateToken(ctx context.Context, tok string) error",
  "tier":      "verified"
}
```

`tier` is the evidence tier: `verified`, `pre-scan`, `doc-claimed`, `sme-stated`, or
`assumed`. See [architecture.md](architecture.md#evidence-tiers-adr-007-adr-009).

`end_line` (T5-5, docs/plans/trust-and-tokenomics.md) is the last line covered by this
symbol, when its extractor computed a real extent — currently `DocChunk` headings only,
set to the line before the next heading (or EOF for the last one). Omitted (zero) for
every other kind: a single-line definition has no separate extent worth recording. Pair
it with `read_source` (`start_line: line, end_line: end_line`) to pull a doc section's
real, line-numbered text instead of guessing its extent by arithmetic off the heading
line — the fix for the T08 class in `docs/plans/home-bench.md`'s H3-1 row.

---

## Prompts

Fathom registers four canned prompts (via `prompts/list` and `prompts/get`). Each prompt
returns a structured sequence of instructions that guides an AI agent to use the
appropriate fathom tools for the task.

| Prompt | Arguments | Purpose |
|---|---|---|
| `investigate_incident` | `service` (req), `symptom` (req) | Guide an investigation into a production incident: entrypoints → complexity hotspots → call hierarchy → community → staleness check |
| `review_change` | `service` (req), `changed_files` (req), `change` (opt), `concern` (opt) | Review a code change for safety by calling the composed `review_diff` verb (C3-1, `docs/plans/review-verb.md`) — one cited verdict instead of a manual tool sequence |
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

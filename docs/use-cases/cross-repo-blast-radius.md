# Cross-repo blast radius & impact

Answer "if I change X, what breaks — here and in every other service?" across a language-agnostic fleet.

## When to use this

- You are about to rename, refactor, or delete a symbol in a shared library and need to know every consumer, in every language, before you touch anything.
- You are reviewing a PR that touches a widely-called function and want to surface all call sites and affected tests automatically.
- You want to confirm that a planned rename has zero callers left after a migration.
- You need to know whether two symbols are connected at all — or find the shortest path between them — to reason about transitive breakage.

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `find_callers` | MCP tool (investigation) | All symbols in the **current service** that call a named symbol |
| `test_impact` | MCP tool (investigation) | Test functions that transitively reach the symbol via the call graph |
| `blast_radius` | MCP tool (investigation) | Blast radius of **changed files**: symbols defined there + their transitive callers + affected tests + touched files |
| `diff_impact` | MCP tool (investigation) | Alias of `blast_radius`, framed for diff/PR review |
| `rename_impact` | MCP tool (coding) | Every call site that must be updated if the symbol is renamed |
| `reachable` | MCP tool (coding) | Boolean: is there any directed call path from symbol A to symbol B? |
| `cross_repo_callers` | MCP tool (platform) | All callers of a symbol **across every ingested service** via stored call edges (its `platform_callers` alias was deleted, SW-1) |
| `references` | MCP tool (investigation) | Fleet-wide occurrence search for an identifier or string across every service's source, evidence-tiered — catches string/config mentions no call edge records |
| `cross_repo_blast_radius` | MCP tool (platform) | Blast radius of a **symbol across all services**: the symbol plus every cross-service caller |
| `list_services` | MCP tool (platform) | All services in the fleet with detected languages and symbol counts |
| `staleness_report` | MCP tool (shared) | Whether the index is out of date vs. current HEAD |
| `ingest` | MCP tool | Re-index a service in-session to pull in new call edges |
| `fathom platform-callers` | CLI | Cross-repo caller lookup from the terminal (language-agnostic) |

## How it enhances Claude Code

Without fathom, Claude Code must grep across repos and parse raw source to answer impact questions — a slow, incomplete, and token-heavy process that misses transitive callers.

With fathom:

1. **Call edges are pre-indexed** across ~25 languages. `cross_repo_callers` returns structured results in one round-trip instead of many grep passes.
2. **Test impact is automatic.** `test_impact` walks the call graph transitively; Claude Code gets an exact test list to run rather than guessing.
3. **Rename safety is explicit.** `rename_impact` returns every call site as a list; Claude Code can iterate it, edit each site, and confirm the list shrinks to zero.
4. **Path existence is a boolean gate.** `reachable` lets Claude Code answer "does this change propagate to the payment path?" without speculative reasoning — reducing plan drift.
5. **Interface-dispatch callers are no longer invisible.** For Go services (ADR-021 §3), `find_callers`/`blast_radius`/`diff_impact`/`symbol_context` additionally consume typed call edges: a caller reachable only through an interface method shared by several concrete implementations — previously an unconditional refusal, since the syntax-only graph can't disambiguate an overloaded method name — now shows up, kind-labeled (`direct` / `iface-dispatch`) in `find_callers`'s `typed_callers` and `symbol_context`'s `typed_edges`, and folded directly into `blast_radius`/`diff_impact`'s symbol/test/file sets.

The result: impact review that would take dozens of file reads and grep calls collapses to 3-5 structured tool calls, and Claude Code can cite specific files and line numbers from the evidence rather than hedging.

## Worked example

**Scenario:** The `ValidateToken` function in `auth-lib` is being refactored. You need to know what breaks across the fleet before touching the signature.

**1. Confirm the index is fresh.**

```json
{ "name": "staleness_report", "arguments": { "service": "auth-lib" } }
```

Returns `{ "reingest_recommended": false, "indexed_commit": "a3f9c1d", "current_commit": "a3f9c1d" }`. Good — index is current.

**2. Find cross-repo callers (fleet-wide).**

```json
{ "name": "cross_repo_callers", "arguments": { "symbol": "ValidateToken" } }
```

Returns:
```json
{
  "callers": [
    { "service": "payment-service",  "symbol": "ProcessCheckout",    "file_path": "internal/checkout/handler.go" },
    { "service": "order-service",    "symbol": "SubmitOrder",         "file_path": "cmd/api/routes.go" },
    { "service": "notification-svc", "symbol": "sendAuthenticatedMsg","file_path": "pkg/notify/send.py" }
  ],
  "count": 3
}
```

Three services — in Go and Python — call `ValidateToken`. The Python caller is only visible because call edges are language-agnostic; a Go-only grep would have missed it.

**3. Find callers inside `auth-lib` itself.**

```json
{ "name": "find_callers", "arguments": { "service": "auth-lib", "symbol": "ValidateToken" } }
```

Returns internal callers: `RefreshSession`, `AdminOverride`. Both need to be updated.

**4. Find all tests that transitively cover the symbol.**

```json
{ "name": "test_impact", "arguments": { "service": "auth-lib", "symbol": "ValidateToken" } }
```

Returns `{ "tests": ["TestValidateToken_Expired", "TestRefreshSession_Invalid", "TestAdminOverride_Bypass"] }`. Run exactly these before and after the change.

**5. Confirm rename scope.**

```json
{ "name": "rename_impact", "arguments": { "service": "auth-lib", "symbol": "ValidateToken" } }
```

Returns the same caller list as `find_callers` but formatted as call sites — use this as the checklist to update.

**6. Gate on reachability before touching the payment path.**

```json
{ "name": "reachable", "arguments": { "service": "payment-service", "from": "ProcessCheckout", "to": "ChargeCard" } }
```

Returns `{ "reachable": true }`. The checkout → charge path exists, so this change carries payment-path risk. Escalate review accordingly.

**CLI equivalent (from a terminal, outside a Claude Code session):**

```sh
fathom platform-callers --store workspace/db/fleet.sqlite --symbol ValidateToken
```

Produces the same cross-repo caller list. The `--repos` flag is a legacy Go-only filter; omit it to get language-agnostic results.

### Diff / PR variant: blast radius of a change

When you have a set of changed files (a PR, a commit), use `blast_radius` (or its PR-framed alias `diff_impact`) to get the symbols those files define plus everything transitively affected — the exact review surface:

```json
{ "name": "blast_radius", "arguments": { "service": "auth-lib", "changed_files": ["internal/auth/token.go"] } }
```

Returns the defined symbols, their transitive callers, the affected tests, and the touched files.

For a single symbol's reach across the **whole fleet** in one call, use `cross_repo_blast_radius`:

```json
{ "name": "cross_repo_blast_radius", "arguments": { "symbol": "ValidateToken" } }
```

Returns the symbol in each service plus every cross-service caller — the one-call answer to "who is affected, fleet-wide?"

## Setup & notes

**Surface:** there is no profile switch (`set_profile` was removed, SW-2). stdio `serve-mcp` serves the full registry; the shared HTTP daemon scopes by path — `POST /rpc/agent` (the 19-tool allowlist: `find_callers`, `references`, `list_services`, `staleness_report`, `ingest` are on it), and bare `POST /rpc` for everything (the group endpoints were deleted — HF-5). An off-surface call (`test_impact`, `rename_impact`, `reachable`, `cross_repo_*` from `/rpc/agent`) still executes — it is logged `off_surface:true`, never refused.

**Cross-repo edges require call-edge data in the index.** If `cross_repo_callers` returns zero results for a symbol you know is called externally, the index was built without call edges populated. Re-ingest with a current build:

```json
{ "name": "ingest", "arguments": { "service": "auth-lib" } }
```

After ingest, verify the `Package` column and `Calls` edges are present by checking `find_callees` on a known symbol. CLI `fathom ingest` can contend with a concurrent Claude Code session's own write (SQLite serializes writers, ADR-020) and fail past `busy_timeout` — use the MCP `ingest` tool instead.

**Staleness matters for impact analysis.** A long-running `serve-mcp` process can outlive commits. Always call `staleness_report` before a blast-radius review. If `reingest_recommended` is `true`, call `ingest` before proceeding — stale call edges produce false negatives (missing callers).

**`platform_callers` is gone.** The MCP alias was deleted as zero-call redundancy (surfaces week-1, SW-1); call `cross_repo_callers`. The CLI verb is still `fathom platform-callers` (hyphenated).

**Mentions that are not call edges.** A symbol referenced from config, a route table, a string or a language the extractor treats heuristically leaves no call edge. `references { "query": "ValidateToken" }` returns every occurrence across all services (text hits at the `pre-scan` tier, definitions at their real tier) — run it beside `cross_repo_callers` before declaring zero consumers.

**`--repos` flag is Go-only (legacy).** When using the CLI, omit `--repos` to get language-agnostic results. The flag filters to a subset of repos using Go module paths and does not apply to Python, TypeScript, Java, or other languages.

**`concern` is not required for impact tools.** `find_callers`, `rename_impact`, `reachable`, `cross_repo_callers`, and `test_impact` take `symbol` as their scope — no `concern` argument needed. (`query_symbols` and `pack_context` do require `concern`; they are for a different pattern.)

## Maintained

This doc covers: `find_callers` (incl. `depth`, `edge_confidence`, `by_depth`, `transitive_callers`), `test_impact`, `blast_radius` (incl. `by_depth`), `diff_impact`, `rename_impact`, `reachable`, `cross_repo_callers`, `cross_repo_blast_radius`, `references`, `list_services`, `staleness_report`, `ingest`, `fathom platform-callers`. Update it when these change.

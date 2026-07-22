# Fleet & Codebase Health

Use fathom to get an objective quality and security posture across an entire fleet of services, or drill into a single repo's complexity and dependency surface.

---

## When to use this

- You want a fleet-wide sweep: which services exist, what languages they use, and where security or quality risks concentrate.
- You are preparing for a platform review, incident post-mortem, or architectural audit and need facts, not guesses.
- You need to identify refactor targets — high-complexity functions, god objects, or services with significant technical debt.
- You want a dependency inventory (SBOM-style) or a line-count breakdown before making a build or upgrade decision.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `list_services` | MCP tool | All ingested services with symbol counts and detected languages |
| `fleet_drift` | MCP tool | Symbols that appear in multiple services with conflicting kinds — a signal of diverged shared code |
| `query_cross_service` | MCP tool | Keyword search across every service in the fleet simultaneously |
| `complexity` | MCP tool | Cyclomatic-complexity hotspots and Maintainability Index for one service |
| `god_object` | MCP tool | Types with too many methods or fields — structural debt signals |
| `secret_scan` | MCP tool | Hardcoded tokens, high-entropy strings, credential-named variables |
| `shell_out` | MCP tool | Shell-out call sites (exec, subprocess, child_process) |
| `http_client_calls` | MCP tool | HTTP client call sites across Go, JS/TS, Python |
| `cloc` | MCP tool | Line counts by language (blank / comment / code) |
| `dependencies` | MCP tool | SBOM-style dependency list from lockfiles |
| `hotspots` | MCP tool | Top-N complexity hotspots per service (also returned nested inside `complexity`) |
| `security_rollup` | MCP tool | Aggregated security posture: `secret_scan` + `shell_out` + `http_client_calls` + license, in one call |
| `staleness_report` | MCP tool | Whether the index is in sync with HEAD; flags `reingest_recommended` |
| `set_profile` | MCP tool | Switch the visible tool set |
| `ingest` | MCP tool | Re-index a service in-session after commits |
| `fathom ingest` | CLI | Index a repo into the fleet store (use when no session holds the DuckDB lock) |

> **Security rollup:** `security_rollup` is a single MCP tool (GroupShared) that internally runs `secret_scan`, `shell_out`, and `http_client_calls` and returns an aggregated posture report including license findings. Use it for a one-call sweep; call the individual tools when you need granular control.

---

## How it enhances Claude Code

Without fathom, an agent doing a health sweep reads files, greps, and re-derives structure on every session — expensive, error-prone, and blind to the fleet.

With fathom:

- **Fleet orientation in one call.** `list_services` returns symbol counts and languages for every ingested service. The agent knows the fleet's shape before reading a single file.
- **Targeted security sweeps.** `secret_scan`, `shell_out`, and `http_client_calls` run against the pre-indexed store. Findings carry file and line — the agent can surface them directly or use them to open exactly the right files.
- **Complexity-driven refactor targeting.** `complexity` returns the top-N hotspot functions with cyclomatic scores and an overall Maintainability Index. The agent proposes concrete refactor targets with evidence, not intuition.
- **Drift detection across the fleet.** `fleet_drift` flags symbols that exist in multiple services with conflicting kinds — a structural signal that shared code has forked or that service boundaries are blurring.
- **Honest dependency picture.** `dependencies` parses lockfiles and returns direct/indirect flags and ecosystems. An agent can identify outdated or risky transitive deps without parsing lock files manually.

The result: the agent spends tokens on analysis and recommendations, not on discovery and file parsing.

---

## Worked example

**Scenario:** A platform engineer asks Claude Code to produce a health posture summary for the entire fleet, then identify the highest-risk service for an upcoming reliability sprint.

**Step 1 — Orient: what is in the fleet?**

```json
{ "name": "list_services" }
```

Returns:
```json
{
  "services": [
    { "service": "auth-service",    "symbol_count": 412,  "languages": ["Go"] },
    { "service": "payment-service", "symbol_count": 891,  "languages": ["Go", "TypeScript"] },
    { "service": "notify-service",  "symbol_count": 203,  "languages": ["Python"] }
  ],
  "count": 3
}
```

The agent now knows three services are indexed. `payment-service` is the largest.

**Step 2 — Security posture sweep across services**

For each service, call the three security tools. Here shown for `payment-service`:

```json
{ "name": "secret_scan",       "arguments": { "service": "payment-service" } }
{ "name": "shell_out",         "arguments": { "service": "payment-service" } }
{ "name": "http_client_calls", "arguments": { "service": "payment-service" } }
```

`secret_scan` returns a finding:
```json
{
  "findings": [
    { "file": "internal/gateway/client.go", "line": 34, "rule": "high-entropy-string", "message": "possible hardcoded secret" }
  ]
}
```

`shell_out` returns empty; `http_client_calls` returns three outbound call sites.

The agent logs: `payment-service` has 1 secret-scan hit and 3 outbound HTTP sites. Repeat for the remaining services to build the fleet-wide posture.

**Step 3 — Find complexity hotspots in the highest-risk service**

```json
{ "name": "complexity", "arguments": { "service": "payment-service", "top_n": 5 } }
```

Returns:
```json
{
  "hotspots": [
    { "name": "ProcessCheckout",   "file": "internal/checkout/handler.go", "line": 88,  "complexity": 24 },
    { "name": "ValidateLineItems", "file": "internal/checkout/validate.go", "line": 201, "complexity": 18 }
  ],
  "maintainability_index": 52
}
```

A Maintainability Index of 52 (scale 0–100; below 65 is a concern) and a complexity-24 function give the agent concrete, citable targets for the sprint backlog.

**Step 4 — Check for god objects**

```json
{ "name": "god_object", "arguments": { "service": "payment-service", "method_threshold": 10, "field_threshold": 10 } }
```

Returns:
```json
{
  "god_objects": [
    { "type": "CheckoutService", "methods": 17, "fields": 6 }
  ]
}
```

**Step 5 — Fleet drift check**

```json
{ "name": "fleet_drift" }
```

Highlights any symbol that exists in multiple services with conflicting kinds — a sign that shared logic may need to be extracted or reconciled.

---

## Setup & notes

**Profiles:**

- Use `set_profile` with `"platform"` for fleet-wide operations (`list_services`, `fleet_drift`, `query_cross_service`). Platform profile includes all shared tools. Over the shared HTTP daemon, register against `POST /rpc/platform` instead.
- Use `set_profile` with `"coding"` for single-service quality metrics (`complexity`, `god_object`, `cloc`, `dependencies`, `antipatterns`). Coding profile also includes shared tools including `secret_scan`, `shell_out`, `http_client_calls`. Over HTTP, use `POST /rpc/coding`.
- If you need both fleet and per-service tools in the same session, call `set_profile "platform"` first (it exposes the broadest surface including shared tools), then call per-service tools directly. (`set_profile` is available on stdio `serve-mcp` only; over HTTP, the path-scoped endpoint is the equivalent.)

**Concern argument:** The tools in this use case do **not** require a `concern` argument — they are structural/metric queries, not concern-scoped symbol searches. (`query_symbols` and `pack_context` require a concern; those are covered in other use-case docs.)

**Staleness:** A long-running `serve-mcp` session can outlive commits. Before a posture sweep, call `staleness_report` per service to check whether `reingest_recommended` is `true`. If it is, re-index with the `ingest` MCP tool (not the CLI — the CLI cannot acquire the DuckDB write lock while the server holds it):

```json
{ "name": "ingest", "arguments": { "service": "payment-service" } }
```

**Registering the MCP server** (one-time, user-scoped):

```sh
claude mcp add --scope user fathom -- \
  /abs/path/bin/fathom serve-mcp \
  --store /abs/path/workspace/db/fleet.sqlite
```

**Ingesting a repo** (CLI, outside a session or before first use):

```sh
fathom ingest --service payment-service --path /path/to/payment-service
```

**`dependencies` parses lockfiles** — `go.sum`, `package-lock.json`/`yarn.lock`, `requirements.txt`/`Pipfile.lock`, `Gemfile.lock`, `Cargo.lock`. Services without a recognised lockfile return an empty list; that is expected for generated or vendored repos.

---

## Maintained

This doc covers: `list_services`, `fleet_drift`, `query_cross_service`, `complexity`, `hotspots`, `god_object`, `security_rollup`, `secret_scan`, `shell_out`, `http_client_calls`, `cloc`, `dependencies`, `deps`, `staleness_report`, `set_profile`, `ingest`, `fathom ingest`. Update it when these change.

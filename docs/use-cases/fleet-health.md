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
| `complexity` | MCP tool | Cyclomatic-complexity hotspots and Maintainability Index for one service |
| `god_object` | MCP tool | Types with too many methods or fields — structural debt signals |
| `secret_scan` | MCP tool | Hardcoded tokens, high-entropy strings, credential-named variables |
| `shell_out` | MCP tool | Shell-out call sites (exec, subprocess, child_process) |
| `http_client_calls` | MCP tool | HTTP client call sites across Go, JS/TS, Python |
| `cloc` | MCP tool | Line counts by language (blank / comment / code) |
| `staleness_report` | MCP tool | Whether the index is in sync with HEAD; flags `reingest_recommended` |
| `ingest` | MCP tool | Re-index a service in-session after commits; a run that could read nothing is a JSON-RPC error naming the cause, never `ok:true` |
| `fathom ingest` | CLI | Index a repo into the fleet store (avoid while a session is writing the same store — SQLite serializes writers, ADR-020); prints a `read-failed: N file(s)` column and exits 1 (`ingest degraded`/`ingest refused`) when files could not be read |
| `fathom fleet report\|reconcile\|receipts\|undo` | CLI | Wiring health across every daemon-known repo; `reconcile` dry-runs (`--apply` to write) hook/stanza/shim drift; every write is receipted (`.fathom/receipts.jsonl`) and `undo` reverses it. Not yet listed by `fathom help` — `fathom fleet -h` and `fathom fleet <sub> -h` |
| `fathom doctor` | CLI | Per-repo wiring diagnosis: registration, daemon, hooks, stanza, skills, config, obligations, traffic, concerns |
| `fathom config audit` | CLI | The config custodian: evidence for each `.fathom/concerns.cue` contract (forbidden patterns, documented commands, line budgets, resolving pointers, stanza tools served, cited handoffs) |
| `fathom capabilities list\|explain\|set` | CLI | What is switched on in a repo and why anything is off (`capabilities.cue`, project governs user — law 13) |
| `fathom scope` / `fathom purge` | CLI | Reversible suppression vs. irreversible, verified removal (see Setup & notes) |

---

## How it enhances Claude Code

Without fathom, an agent doing a health sweep reads files, greps, and re-derives structure on every session — expensive, error-prone, and blind to the fleet.

With fathom:

- **Fleet orientation in one call.** `list_services` returns symbol counts and languages for every ingested service. The agent knows the fleet's shape before reading a single file.
- **Targeted security sweeps.** `secret_scan`, `shell_out`, and `http_client_calls` run against the pre-indexed store. Findings carry file and line — the agent can surface them directly or use them to open exactly the right files.
- **Complexity-driven refactor targeting.** `complexity` returns the top-N hotspot functions with cyclomatic scores and an overall Maintainability Index. The agent proposes concrete refactor targets with evidence, not intuition.
- **Drift detection across the fleet.** `fleet_drift` flags symbols that exist in multiple services with conflicting kinds — a structural signal that shared code has forked or that service boundaries are blurring.

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

**Surfaces:** there is no profile switch (`set_profile` was removed, SW-2). stdio `serve-mcp` serves the full registry. Over the shared HTTP daemon the path is the scope: the group endpoints were deleted 2026-08-26 (HF-5), so bare `POST /rpc` serves the fleet and quality tools (`list_services`, `fleet_drift`, `cross_repo_*`, `complexity`, `god_object`, `cloc`, `secret_scan`, `staleness_report`, `ingest`); `POST /rpc/agent` is the 17-tool allowlist (`list_services`, `staleness_report`, `ingest` are on it; the security and quality tools are not). An off-surface call still executes and is logged `off_surface:true` — never refused — so one registration serves every step above.

**Concern argument:** The tools in this use case do **not** require a `concern` argument — they are structural/metric queries, not concern-scoped symbol searches. (`query_symbols` and `pack_context` require a concern; those are covered in other use-case docs.)

**Staleness:** A long-running `serve-mcp` session can outlive commits. Before a posture sweep, call `staleness_report` per service to check whether `reingest_recommended` is `true`. If it is, re-index with the `ingest` MCP tool (not the CLI — a concurrent CLI write can contend with the server's own write and fail past `busy_timeout`, SQLite serializes writers per ADR-020):

```json
{ "name": "ingest", "arguments": { "service": "payment-service" } }
```

**Registering the MCP server** (one-time, user-scoped — the shared daemon, agent surface):

```sh
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent
```

(stdio alternative for a single session: `claude mcp add --scope user fathom -- /abs/path/bin/fathom serve-mcp --store /abs/path/workspace/db/fleet.sqlite`.)

**Ingesting a repo** (CLI, outside a session or before first use):

```sh
fathom ingest --service payment-service --path /path/to/payment-service
```

**Ingest tells you what it could not read** (IT-14/IT-15). A file that fails with EPERM/EACCES/EIO/ENOENT is named with its errno under `read-failed: N file(s)` (first five inline); readable files are still indexed and the run exits 1 with `error: ingest degraded: …`. A run that could read *nothing* while the store already holds that service refuses to write — no watermark advance, no purge — `error: ingest refused: …`, and on `operation not permitted` adds the hint `external-volume TCC wedge? tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`. The daemon's `ingest` tool returns the same cause and hint as a JSON-RPC error.

**Hiding vs. destroying.** `fathom scope suppress <glob> --reason R [--service S] [--until 7d]` hides matching paths from every answer, reversibly — the answer then carries a `suppressed` block naming the reason and the `unsuppress <id>` command (`scope list`, `scope preview <glob>` to see what a glob would touch). `fathom purge <glob> --service S --reason R --dry-run|--yes` is the irreversible twin: it deletes the rows and then hunts the raw store bytes (`--probe` adds a literal, e.g. the leaked credential) — exit 0 clean, exit 1 still present, exit 2 = SQL zero but bytes unchecked; `purge history` lists what was destroyed.

**Fleet wiring health** (law 15, SH-7/SH-8). `fathom fleet report` writes a per-repo wiring report (`--json` to stdout); `fathom fleet reconcile` dry-runs every daemon-known repo (`--repo` to pick) and distinguishes a *stale* stanza from one that is *up to date (will be re-marked with a digest on apply; no prose changes)*; `--apply` writes only through `internal/wiring` — one plan, one apply, one receipt per repo in `.fathom/receipts.jsonl` (gitignored), listed by `fathom fleet receipts` and reversed by `fathom fleet undo --last`. A tracked stanza region (`<!-- fathom:begin v3 sha256:… -->`) that a human edited is **blocked and named** — exit 1, file byte-identical, other files still applied; a `v1` region (`<!-- fathom:begin -->`, no digest) is adopted and upgraded on first write, which is not drift. `fathom doctor` (`--harness claude|codex|auto`) is the single-repo view, including a `concerns` row from `fathom config audit`; `fathom capabilities list` shows what is on and `capabilities set <id> on|off --scope project|user` changes it, splicing only the capabilities block and preserving the rest of the file byte-for-byte. The `stanza_tools_served` and `handoff_cites_instruments` concern kinds are **experimental** (new this cycle); `stanza_tools_served` is the one custodian check that asks the daemon, and degrades to *unable to check* when it is down.

---

## Maintained

This doc covers: `list_services`, `fleet_drift`, `complexity`, `god_object`, `secret_scan`, `shell_out`, `http_client_calls`, `cloc`, `staleness_report`, `ingest` (incl. `root` and the worktree re-root refusal), `fathom ingest` (incl. `read-failed`/`degraded`/`refused`, the worktree re-root refusal), `fathom fleet` (`report`/`reconcile`/`receipts`/`undo`; `reconcile` plans the shipped skills through the wiring engine — a hand-authored copy is BLOCKED by name; `fathom fleet -h`), `fathom doctor`, `fathom config audit`, `fathom capabilities` (`list`/`explain`/`set`), `fathom scope` (reversible suppression: list/suppress/unsuppress/preview), `fathom purge` (irreversible, verified removal + history). Update it when these change.

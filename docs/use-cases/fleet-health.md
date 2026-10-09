# Fleet & Codebase Health

Use fathom to get an objective fleet-wiring posture across an entire fleet of services. (The per-service MCP quality/security scanners this doc used to cover — `complexity`, `god_object`, `secret_scan`, `shell_out`, `cloc` — were unregistered by SR-19, surface reckoning F-77: 0 lifetime calls each.)

---

## When to use this

- You want a fleet-wide sweep: which services exist and what languages they use.
- You are preparing for a platform review, incident post-mortem, or architectural audit and need facts, not guesses.
- You want to check a repo's fathom wiring health, or its `.fathom/concerns.cue` contract, across the fleet.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `list_services` | MCP tool | All ingested services with symbol counts and detected languages |
| `staleness_report` | MCP tool | Whether the index is in sync with HEAD; flags `reingest_recommended` |
| `ingest` | MCP tool | Re-index a service in-session after commits; a run that could read nothing is a JSON-RPC error naming the cause, never `ok:true` |
| `fathom ingest` | CLI | Index a repo into the fleet store (avoid while a session is writing the same store — SQLite serializes writers, ADR-020); prints a `read-failed: N file(s)` column and exits 1 (`ingest degraded`/`ingest refused`) when files could not be read |
| `fathom fleet report\|reconcile\|receipts\|undo` | CLI | Wiring health across every daemon-known repo; `reconcile` dry-runs (`--apply` to write) hook/stanza/shim drift; every write is receipted (`.fathom/receipts.jsonl`) and `undo` reverses it. Not yet listed by `fathom help` — `fathom fleet -h` and `fathom fleet <sub> -h` |
| `fathom doctor` | CLI | Per-repo wiring diagnosis: registration, daemon, hooks, stanza, skills, config, obligations, traffic, concerns |
| `fathom config audit` | CLI | The config custodian: evidence for each `.fathom/concerns.cue` contract (forbidden patterns, documented commands, line budgets, resolving pointers, stanza tools served, cited handoffs) |
| `fathom config propose` | CLI | The custodian's diff-offering twin: a unified diff for each violation it can mend mechanically (a moved pointer or script target), the remedy for the rest; writes nothing — the session applies it |
| `fathom capabilities list\|explain\|set` | CLI | What is switched on in a repo and why anything is off (`capabilities.cue`, project governs user — law 13) |
| `fathom scope` / `fathom purge` | CLI | Reversible suppression vs. irreversible, verified removal (see Setup & notes) |

---

## How it enhances Claude Code

Without fathom, an agent doing a health sweep reads files, greps, and re-derives structure on every session — expensive, error-prone, and blind to the fleet.

With fathom:

- **Fleet orientation in one call.** `list_services` returns symbol counts and languages for every ingested service. The agent knows the fleet's shape before reading a single file.
- **Wiring health in one call.** `fathom fleet report`/`reconcile` names every repo's hook/stanza/shim drift without the agent re-deriving it per repo.

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

**Step 2 — Check fleet wiring health**

```sh
fathom fleet report --json
```

Names each repo's hook/stanza/shim state — a *stale* stanza, a missing hook, or *up to
date* — without the agent re-deriving wiring drift per repo. `fathom fleet reconcile
--apply` writes the fix, one receipt per repo.

---

## Setup & notes

**Surfaces:** there is no profile switch (`set_profile` was removed, SW-2). stdio `serve-mcp` serves the full registry. Over the shared HTTP daemon the path is the scope: the group endpoints were deleted 2026-08-26 (HF-5), so bare `POST /rpc` serves the fleet tools (`list_services`, `cross_repo_*`, `staleness_report`, `ingest`); `POST /rpc/agent` is the eight-tool default surface, and `POST /rpc/extended` (alias `/rpc/companion`) the 21-tool superset (`list_services`, `staleness_report`, `ingest` are on it). An off-surface call still executes and is logged `off_surface:true` — never refused. (`fleet_drift` was unregistered, SR-12, surface reckoning F-22; `complexity`, `god_object`, `cloc`, `secret_scan`, `shell_out` by SR-19, F-77 — 0 lifetime calls each.)

**Concern argument:** `list_services` and `staleness_report` do **not** require a `concern` argument — they are structural queries, not concern-scoped symbol searches. (`query_symbols` and `pack_context` require a concern; those are covered in other use-case docs.)

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

This doc covers: `list_services`, `staleness_report`, `ingest` (incl. `root` and the worktree re-root refusal), `fathom ingest` (incl. `read-failed`/`degraded`/`refused`, the worktree re-root refusal), `fathom fleet` (`report`/`reconcile`/`receipts`/`undo`; `reconcile` plans the shipped skills through the wiring engine — a hand-authored copy is BLOCKED by name; `fathom fleet -h`), `fathom doctor`, `fathom config audit`, `fathom config propose`, `fathom capabilities` (`list`/`explain`/`set`), `fathom scope` (reversible suppression: list/suppress/unsuppress/preview), `fathom purge` (irreversible, verified removal + history). Update it when these change.

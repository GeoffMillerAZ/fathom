# Getting started: wire fathom into Claude Code

How to build, index, register, and start using fathom as a live code-intelligence layer for Claude Code (and other AI coding agents) — from zero to first query.

> Wiring shortcut: `fathom init` performs the per-repo steps below in one
> idempotent command — see [coding-agents.md](coding-agents.md) for the
> canonical hook/gate/stanza guidance. This doc remains the from-source walkthrough.

---

## Quickstart (TL;DR)

```sh
# 1. Build (macOS — the tree-sitter extractor bindings require CGO, ADR-013)
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom

# 2. Index your repo
./bin/fathom ingest --service my-service --path . \
  --store workspace/db/fleet.sqlite

# 3. Register with Claude Code (--scope user = all repos + subagents)
claude mcp add --scope user fathom -- \
  /abs/path/bin/fathom serve-mcp \
  --store /abs/path/workspace/db/fleet.sqlite

# 4. In Claude Code: confirm the tool surface, then query
# (MCP tool calls — see Worked example below)
```

---

## When to use this

- You are adopting fathom in a repo for the first time and need a complete setup walkthrough.
- You want Claude Code sessions (including spawned subagents) to have access to live symbol intelligence for your codebase.
- You are indexing multiple repos into a shared fleet store for cross-repo queries.
- You want to generate a `CLAUDE.md` symbol guide that AI agents can reference without live queries.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `init` | CLI | One-command repo wiring: `.fathomignore`, ingest, hooks, CLAUDE.md/AGENTS.md stanza, config scaffold; `--harness <id>` picks the descriptor; `--skills[=claude\|codex\|agents\|all\|codex-prompts]` installs the shipped agent skills (bare = `.agents/skills` + `.claude/skills`); `--json` reports every write, the receipt id and the manual steps as data |
| `doctor` | CLI | Deterministic wiring diagnosis (`--harness claude\|codex\|auto` or a registry id, `--json`): registration, daemon, hooks, gate posture, stanza, skills (`.agents/skills` for every non-Claude harness), config, obligations, traffic, concerns |
| `prime` | CLI | Pull-tier session primer for harnesses without a SessionStart hook (Codex); `fathom prime -h` (`-dir`, `-url`) |
| `ingest` | CLI | Index one service into the SQLite store (sha256 hash-gated; re-ingest is near-free); names every file it could not read (`read-failed: N file(s)`) and exits 1 rather than claiming a clean run |
| `ingest-diff` | CLI | Ingest only files changed since a git ref |
| `watch` | CLI | Poll and re-ingest on file changes automatically |
| `serve-mcp` | CLI | Stdio MCP server — one client (editor/agent session) |
| `serve-http` | CLI | HTTP MCP server — multi-client, CI, platform use; endpoints `POST /rpc`, `POST /rpc/{surface}`, `GET /healthz`, `GET /events`, `GET /metrics`; `--call-log` flag for JSONL tool-call logging; `--module <path>` (repeatable) mounts a conforming expert-module store read-only onto `/rpc/modules`; `--modules-token-env <NAME>` gates that surface behind a bearer token for hosted deployments |
| `gen-claude-md` | CLI | Generate a static `CLAUDE.md` symbol guide from the store |
| `version` | CLI | Print binary version (use to detect staleness vs git HEAD) |
| `capabilities` | MCP tool | List all currently visible tools and the active surface/profile (see Step 5) |
| `ingest` | MCP tool | Re-index a service from inside a live session (bypasses the single-writer CLI lock) |
| `staleness_report` | MCP tool | Compare last-ingested state to current git HEAD; `reingest_recommended` flag |

---

## How it enhances Claude Code

Claude Code has no innate knowledge of your codebase structure. Without fathom, agents resort to repeated grep/read sweeps — slow, token-heavy, and prone to missing cross-file relationships. With fathom:

- **Symbol queries replace file sweeps.** `query_symbols` returns ranked, concern-scoped results in one call, typically cutting context token usage ~77% vs naive file reads.
- **Call-graph edges are precomputed.** Callers, callees, and cross-repo paths are stored as indexed edges — no grep required.
- **Staleness is surfaced, not hidden.** Every query result carries a staleness indicator; `staleness_report` tells the agent exactly when the index has drifted from HEAD.
- **Subagents share the same server.** `--scope user` in the MCP registration means any Claude Code subagent spawned in any repo can call fathom tools without per-project setup.
- **Surfaces keep the tool payload focused.** Register the HTTP daemon at `/rpc/agent` for the curated 19-tool coding hot loop, or bare `/rpc` for the full registry. (The four group endpoints were deleted 2026-08-26 — ADR-014 amendment 2, HF-5 — because nothing wired ever pointed at them.) Nothing is locked out either way: an off-surface call still executes, flagged `off_surface:true` in the call log, so unmet demand is measurable instead of argued.

---

## Worked example

### Step 1 — Build

If you are building from source (the tree-sitter extractor bindings require CGO, ADR-013):

```sh
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom
```

On Linux, omit `MACOSX_DEPLOYMENT_TARGET`. If you have a prebuilt binary, skip this step.

Fathom builds on `agentic-kit` via `go.work` + `replace`. Adopters who only want the binary do not need to understand this — just build once and use.

---

### Step 2 — Create a `.fathomignore`

Place a `.fathomignore` at your repo root (the directory you will pass to `--path`). Uses gitignore syntax. The following dirs are already skipped by default: `vendor/`, `.git/`, `testdata/`, `generated/`, `node_modules/`, `.idea/`, `.vscode/`.

Typical additions:

```gitignore
# Build output
bin/
dist/

# Scratch and local stores
tmp/
workspace/

# Reference clones (not your code)
ref/

# Generated files — index the templates, not the output
*.pb.go
*_gen.go
*_templ.go
```

Matched directories are pruned from the walk; matched files are skipped. Exclude anything sensitive — the index is queryable by any connected MCP client and is not behind an egress gate.

---

### Step 3 — Index your repo

```sh
./bin/fathom ingest \
  --service my-service \
  --path . \
  --store workspace/db/fleet.sqlite
```

Output:
```
service:    my-service
discovered: 142
extracted:  38
skipped:    104  (unchanged files, hash-gated)
symbols:    891
```

**Re-ingest is near-free** — the 3-pass pipeline (filter → parallel fetch → post-fetch hash-gate → extract → store) skips any file whose SHA-256 matches the stored fingerprint. Only changed files are re-extracted.

**Ingest is honest about what it could not read** (IT-14/IT-15): a file that fails with EPERM/EACCES/EIO/ENOENT appears under `read-failed: N file(s)` with its errno (first five inline); readable files are still indexed and the run exits 1 with `error: ingest degraded: …`. A run that could read nothing while the store already holds the service refuses to write at all (`error: ingest refused: …`) and, on `operation not permitted`, prints the external-volume TCC hint (`tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`).

**Multi-repo fleet:** loop `ingest` over sibling repos, each with its own `--service` ID, all writing into one shared store:

```sh
for svc in auth-service payment-service order-service; do
  ./bin/fathom ingest \
    --service "$svc" \
    --path "../$svc" \
    --store workspace/db/fleet.sqlite
done
```

This populates the fleet for cross-repo queries (`cross_repo_callers`) via the `platform` surface.

Alternatively, use `platform-ingest` if your services are subdirectories under one root:

```sh
./bin/fathom platform-ingest \
  --root /src/services \
  --store-dir workspace/db/platform
```

---

### Step 4 — Register the MCP server with Claude Code

Use absolute paths — Claude Code launches the process from your home directory, not the repo root.

```sh
claude mcp add --scope user fathom -- \
  /abs/path/to/bin/fathom serve-mcp \
  --store /abs/path/to/workspace/db/fleet.sqlite
```

`--scope user` makes this MCP server available in every Claude Code session and to all subagents spawned within those sessions, regardless of which repo is open.

Equivalent JSON (in `~/.claude/claude_desktop_config.json` or equivalent):

```json
{
  "mcpServers": {
    "fathom": {
      "command": "/abs/path/to/bin/fathom",
      "args": [
        "serve-mcp",
        "--store",
        "/abs/path/to/workspace/db/fleet.sqlite"
      ]
    }
  }
}
```

---

### Step 5 — Select a tool surface and verify

There is no `set_profile` tool anymore — it was retired (ADR-014 amendment,
2026-07-16) once path-scoped HTTP surfaces replaced the old process-global
profile switch. Scoping now depends entirely on transport:

- **stdio (`serve-mcp`)** always serves the full tool registry — there is
  no way to narrow it per-session.
- **HTTP (`serve-http`)** scopes by the request **path**: each surface is
  an immutable slice of the same daemon/store, selected once at MCP
  registration time (see Step 4), not per-call.

| Surface | Endpoint | Contents |
|---|---|---|
| *(full)* | `POST /rpc` | Every registered tool, unauthenticated |
| `agent` | `POST /rpc/agent` | The eight-tool default floor (surface reckoning SR-33; `docs/mcp-tools.md` lists it) — the recommended registration |
| `companion` | `POST /rpc/companion` | Compatible alias for `agent`: identical 19-tool allowlist |
| ~~`coding` / `investigation` / `shared` / `platform`~~ | *(deleted)* | Removed 2026-08-26 (ADR-014 amendment 2, HF-5) — unreachable by any wired client. The group taxonomy still classifies tools; use `POST /rpc`. |
| ~~`modules`~~ | *(removed)* | SR-17 (surface reckoning F-68) removed `/rpc/modules`, its four tools and hosted-modules mode; `serve-http --module <path>` still mounts module stores for `obligations`' bindings |

Register the daemon against the curated surface:

```sh
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent
```

**Off-surface calls still execute.** A tool that isn't on the registered
surface is never hard-rejected — it runs normally, and the daemon's call
log (`--call-log`) records it with `off_surface:true` so real demand for a
wider surface is measurable instead of argued. Surfaces are ergonomics and
measurement, never a security boundary — bare `/rpc` already serves the
full, unauthenticated registry, loopback-only (see [mcp-tools.md](../mcp-tools.md)).

Then call `capabilities` to confirm what's visible on this request:

```json
{ "name": "capabilities", "arguments": {} }
```

Returns:
```json
{
  "active_surface": "agent",
  "active_profile": "",
  "tool_count": 17,
  "tools": ["blast_radius", "find_symbol", "obligations", "read_source", "symbol_context", "verify_plan", ...]
}
```

---

### Step 6 — First query

```json
{
  "name": "query_symbols",
  "arguments": {
    "service": "my-service",
    "concern": "authentication",
    "keywords": ["token", "validate"],
    "top": 10
  }
}
```

`concern` is **required** for `query_symbols` (ADR-006). It scopes the ranked result set and is the primary mechanism for the ~77% token reduction vs unbounded symbol dumps.

Returns ranked `SymbolNode` objects:
```json
{
  "nodes": [
    {
      "service": "my-service",
      "name": "ValidateToken",
      "kind": "GoFunc",
      "file_path": "internal/auth/token.go",
      "line": 42,
      "signature": "func ValidateToken(ctx context.Context, tok string) error",
      "tier": "verified"
    }
  ],
  "staleness": { "reingest_recommended": false },
  "truncated": false
}
```

---

## The one-command path: `fathom init`, then `fathom doctor`

Everything below Step 2 is what `fathom init` (run from the repo root) does in one idempotent pass: `.fathomignore`, ingest into the daemon, the hook stack in `.claude/settings.json`, the CLAUDE.md/AGENTS.md stanza, a `.fathom/` config scaffold. Add `--skills` to install the shipped skills (activate, custodian, doctor, feedback, impact, onboard, plan, rewire, verify): bare `--skills` (= `all`) writes `.agents/skills/` (the Agent Skills standard path — Codex, Gemini CLI, Cursor, Copilot read it; spec-only frontmatter) **and** `.claude/skills/` (Claude Code); `--skills=codex` or `=agents` writes `.agents/skills/` only, `=claude` writes `.claude/skills/` only, and the deprecated user-global `~/.codex/prompts/` twin is `--skills=codex-prompts` only (`docs/upgrades/0.8.0-skills-on-the-standard.md`). Every file it writes goes through one plan → apply → receipt (`.fathom/receipts.jsonl`, reversible with `fathom fleet undo`); a stanza region a human hand-edited is blocked and named, the other steps still run. Then `fathom doctor` proves the wiring (registration, daemon, hooks installed *and* firing, stanza, skills, config, obligations, traffic), and on a harness without a SessionStart hook `fathom prime` prints the session brief by hand.

## Wiring Claude Code hooks

Two hooks turn fathom from a tool the agent must remember to call into one it is nudged toward automatically (`fathom init` writes them; this is the manual form).

### SessionStart — `fathom session-hook`

The `session-hook` subcommand runs at session start and injects a service-context message: "this repo is service X — use fathom tools before Grep/Read." When the repo is not in the fleet index it still exits 0 but prints an explicit not-indexed line with the ingest hint (the O2 fix) — never silence. Like `prime`, it is listed by `fathom help`; `fathom session-hook -h` shows its flags (`-dir`, `-url`, `--harness`).

Register it in `~/.claude/settings.json` under `hooks.SessionStart`:

```json
"hooks": {
  "SessionStart": [
    {
      "hooks": [
        {
          "type": "command",
          "command": "/abs/path/to/bin/fathom session-hook"
        }
      ]
    }
  ]
}
```

Replace `/abs/path/to/bin/fathom` with the absolute path to your built binary.

### PostToolUse — `fathom edit-hook` (obligations injection)

The `edit-hook` subcommand fires after every `Edit`, `Write`, or `MultiEdit` call. It calls the `obligations` MCP tool for the file just written and injects any governing rules into the session context. It is silent when no rules apply (zero-output-on-clean) and silent on any error — it never blocks a write.

Register it in `~/.claude/settings.json` under `hooks.PostToolUse`:

```json
"hooks": {
  "PostToolUse": [
    {
      "matcher": "Edit|Write|MultiEdit",
      "hooks": [
        {
          "type": "command",
          "command": "/abs/path/to/bin/fathom edit-hook",
          "timeout": 5
        }
      ]
    }
  ]
}
```

The hook requires the obligations layer to be compiled for the service first (`fathom compile --service <id>` or via the `compile_layer` MCP tool with `dry_run:false`). Until compiled, the hook exits clean.

### Lefthook pre-push — `fathom verify`

If your repo has plan files in `docs/plans/*.md`, the `lefthook.yml` pre-push hook runs `fathom verify` to gate claimed-but-unproven tasks before they leave the workstation. See [spec-verification.md](spec-verification.md) for the full gate reference.

---

## serve-mcp vs serve-http

| | `serve-mcp` | `serve-http` |
|---|---|---|
| Transport | stdio (no port) | HTTP on `--addr` (default `127.0.0.1:7575`, loopback) |
| Clients | One — the process that launched it | Many concurrent |
| Use case | One editor / Claude Code session | Multiple concurrent Claude Code sessions, CI, multi-agent platform |
| Endpoints | — | `POST /rpc` (full surface), `POST /rpc/{surface}` (scoped `tools/list`; off-surface `tools/call` still executes, flagged `off_surface:true` — see Step 5; surface ∈ `agent`/`companion`/`coding`/`investigation`/`shared`/`platform`/`modules`), `GET /healthz`, `GET /events` (SSE), `GET /metrics`, `GET /statusz`, `/observatory` (fleet/service/wiring/impact/story/brief pages, loopback), `POST /reload` |
| Call log | — | `--call-log <path>` writes a JSONL tool-call log (default `workspace/logs/mcp-calls.jsonl` relative to daemon working dir; empty string disables) |
| Store writes | Serializes with any other writer (SQLite, ADR-020) | Same |

For a **single** Claude Code session, `serve-mcp` registered with Claude Code is simplest. **Running more than one session at a time? Use `serve-http`.** The stdio model spawns one server per session, each opening its own connection to the same store; a shared `serve-http` daemon avoids N separate re-ingests and the write contention two sessions' own writes could otherwise hit. Also use `serve-http` to call fathom from CI without per-job MCP registration. Full multi-session setup (launchd + HTTP registration): see [claude-code-integration.md](../claude-code-integration.md).

---

## Refresh discipline and the write lock

SQLite serializes writers (ADR-020). While `serve-mcp` (or `serve-http`) is mid-write, a concurrent CLI `ingest` can hit `busy_timeout` and fail with a lock error. This is expected.

**To refresh the index during a live session**, call the `ingest` MCP tool instead of the CLI:

```json
{
  "name": "ingest",
  "arguments": { "service": "my-service" }
}
```

This runs the ingest pipeline inside the server process, which already holds the lock.

**Staleness detection:** a long-running server can outlive commits. Check:

```json
{ "name": "staleness_report", "arguments": { "service": "my-service" } }
```

Returns `reingest_recommended: true` when the repo HEAD has advanced since the last ingest. You can also compare `fathom version` output to `git rev-parse HEAD` manually.

**Automatic staleness on every query:** `query_symbols` and other query tools embed a staleness indicator in their response. Agents should surface a re-ingest prompt to the user when `reingest_recommended` is `true`.

For always-current indexes in long-lived sessions, consider `fathom watch` in a separate terminal — it polls and re-ingests on file changes, but it requires the MCP server to not be holding the lock, which means you cannot use `watch` and `serve-mcp` against the same store file simultaneously. Solutions:

- Use two store files: a live file for `watch` + CLI work, a separate file for `serve-mcp` (refresh it explicitly).
- Or rely on the in-session `ingest` MCP tool after each significant change.

---

## Generate a static CLAUDE.md symbol guide

For AI agents that work better with a pre-loaded symbol reference rather than live MCP queries, generate a `CLAUDE.md`:

```sh
./bin/fathom gen-claude-md \
  --service my-service \
  --store workspace/db/fleet.sqlite \
  --output workspace/artifacts/CLAUDE.md
```

Output is deterministic (symbols sorted by kind then name). The file includes a kind/count breakdown and a full symbol listing. Drop it into your repo or feed it to an agent's context manually.

---

## Setup & notes

- **Absolute paths everywhere.** The `claude mcp add` command and the JSON config both require absolute paths — Claude Code launches the binary from the home directory, not the repo root.
- **`--scope user` is recommended** over `--scope project`. User scope means subagents in any repo see the fathom tools; project scope limits it to one repo's sessions.
- **One store, many services.** A single `fleet.sqlite` can hold all your services. There is no need for per-service store files unless you are using `platform-ingest` + `serve-http` in platform mode (which uses per-service `.sqlite` files).
- **`workspace/` is gitignored** in fathom's own repo convention. Apply the same pattern in adopter repos: add `workspace/` to `.gitignore` so store files and generated artifacts are never committed.
- **macOS build note.** `MACOSX_DEPLOYMENT_TARGET=14.0` is required because the tree-sitter extractor bindings compile CGO objects that need it to link cleanly (ADR-013) — not because of the store, which is pure Go and needs neither CGO nor a deployment target (SS-8/ADR-025 deleted the DuckDB backend that used to be the reason). On Linux this env var is ignored.
- **No `concern`, no `query_symbols`.** The tools `query_symbols` and `pack_context` both require a `concern` argument. `verify_plan` accepts it as optional. This is intentional (ADR-006) — it prevents unbounded symbol dumps and is the source of the token savings. (`contradiction_scan` was unregistered, SR-19, surface reckoning F-77: 0 lifetime calls.)
- **Prime every session (recommended).** Registering the MCP server makes the tools *available*; it doesn't make an agent *reach for them* — left to habit, agents grep. Add the `session-hook` SessionStart hook (see [usage.md → `session-hook`](../usage.md#session-hook)) so each session opens with "this repo is service X; use fathom before Grep/Read," or an honest signal when the index doesn't cover the repo.

---

## Next steps

Once fathom is wired in and returning results, see the per-use-case guides for how to apply it to specific workflows:

- **[coding-context.md](coding-context.md)** — Stay in the relevant slice of the codebase during active feature work.
- **[cross-repo-blast-radius.md](cross-repo-blast-radius.md)** — Before a refactor, find all callers across every service.
- **[spec-verification.md](spec-verification.md)** — Verify that implementation matches a spec or requirement document.
- **[incident-investigation.md](incident-investigation.md)** — Trace a production symptom to call sites and code paths.
- **[change-review.md](change-review.md)** — Gate a PR on rename impact, test coverage, and complexity.
- **[fleet-health.md](fleet-health.md)** — Surface drift, dead code, anti-patterns, and SBOM across all services.

---

## Maintained

This doc covers: `init` (`--skills`, `--harness`, `--json`), `doctor`, `prime`, `ingest` (incl. `read-failed`, `--json`), `ingest-diff`, `watch`, `serve-mcp`, `serve-http` (incl. `--module`/`--modules-token-env`, `/statusz`, `/observatory`), `gen-claude-md`, `platform-ingest`, `version`, the `/rpc`/`/rpc/agent` (`/rpc/companion`)/`/rpc/modules` path-scoped surfaces, `capabilities`, `ingest` (MCP tool), `staleness_report`, `query_symbols`, `session-hook`, `edit-hook`, `fathom verify` (incl. the one-line provenance notice when the daemon's build is not this binary's), `fathom doctor`'s daemon_fresh and wiring_drift rows, `fathom fleet reconcile` (`--rebaseline`), `/statusz`'s build stamp (binary_commit, binary_dirty, built_at). Every verb here is listed by `fathom help` (the table renders from the dispatch table since 0.6.0); `fathom <verb> -h` shows flags. Update it when these change.

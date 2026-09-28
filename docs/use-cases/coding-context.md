# Concern-scoped coding context

Give Claude Code (or any AI coding agent) a precise, token-efficient context slice instead of dumping a whole repo into the prompt.

Scope: this doc covers the **query patterns**. The wiring — `fathom init`,
hooks, gates, CLAUDE.md/AGENTS.md — lives in [coding-agents.md](coding-agents.md).

---

## When to use this

- An agent is about to edit code in a specific area and needs to understand the relevant symbols, callers, and call graph without reading every file.
- You want to assemble a tight, ranked context bundle that fits inside a token budget.
- You want a structural overview of a repo (entry points, most-connected symbols) as a lightweight orientation step.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `query_symbols` | MCP tool | Concern-scoped ranked symbol list; `concern` and `service` required |
| `find_symbol` | MCP tool | Definition(s) by name — tiered: exact → case-insensitive → substring, `matched_via` says which; a full miss returns `did_you_mean` suggestions |
| `outline` | MCP tool | All symbols in a specific file, ordered by line number |
| `repo_map` | MCP tool | Budget-bounded symbol map ranked by outbound call-edge count |
| `pack_context` | MCP tool | Packs ranked evidence items into a token budget; `concern` and `service` required |
| `symbol_context` | MCP tool | Definition + direct callers + direct callees + related tests for one symbol |
| `read_source` | MCP tool | Bounded, line-numbered raw-text read from the indexed corpus (200 lines / 8000 bytes per call by default, `max_bytes` up to 16000; the bytes are the `lines` array as sent, envelope and escaping included) — the agent surface's `sed -n`; refuses ignored or suppressed paths by name |
| `references` | MCP tool | Occurrence search for an identifier or string (your service by default, `services: ["*"]` for the fleet), evidence-tiered — the structured alternative to grepping repo by repo |
| `memory_search` | MCP tool | Anchored memories for a service, filtered by `query` and/or `file`; every entry carries `tier` and `state` (**experimental** — new, unmeasured) |
| `find_callees` | MCP tool | Direct callees of a named symbol |
| `entrypoints` | MCP tool | Likely program entry points: `main` functions, HTTP handlers, CLI root commands |
| `batch` | MCP tool | Execute N tool calls in a single request |
| `fathom prime` | CLI | Pull-tier session primer for harnesses without a SessionStart hook (Codex): prints the index/wiring status and the fathom-first discipline. Not yet listed by `fathom help` — `fathom prime -h` (`-dir`, `-url`) |
| `fathom memory add\|list\|review\|show` | CLI | Anchored memory: a memory with no anchor (symbol/file/commit/claim/path) is refused; anchor drift marks it `suspect`/`orphaned` |
| `gen-claude-md` | CLI | Generate a `CLAUDE.md` from stored symbols for static context injection |

---

## How it enhances Claude Code

Without fathom, an agent either reads files speculatively (burning tokens on irrelevant code) or misses context entirely and drifts.

Concern scoping (ADR-006) measures approximately 77% fewer tokens than a naive full-symbol dump, because `query_symbols` and `pack_context` both rank symbols by relevance to the stated concern and drop everything else. The agent receives only the symbols, callers, and call edges that matter for the change it is about to make.

The concrete payoffs:

- **Less drift**: the agent starts with an accurate picture of the code it will touch, not a stale mental model reconstructed from file reads.
- **Fewer tokens**: concern + budget gating means context stays within model limits even for large services.
- **Honest gating**: results carry a `staleness` field; the agent can detect when the index is behind HEAD and trigger a re-ingest before trusting the data.
- **Cross-file visibility**: `symbol_context` surfaces callers and callees across the repo in one call, eliminating the need for grep sweeps to understand impact.

---

## Worked example

Scenario: an agent is about to modify token validation in `auth-service` and needs tight context.

**1. Pick the surface at registration time, not per call.** There is no
profile switch: stdio `serve-mcp` always serves the full registry, and the
shared HTTP daemon scopes by path — register against `POST /rpc/agent`
(the curated coding hot loop; `/rpc/companion` is an identical
alias) or `POST /rpc` for everything. Every tool below except
`find_callees` and `batch` is on the agent surface; the rest still execute
there, flagged `off_surface` in the call log rather than refused.

**2. Query concern-scoped symbols** to find everything relevant to token validation:

```json
query_symbols {
  "service": "auth-service",
  "concern": "auth token validation",
  "keywords": ["token", "validate", "expire"],
  "top": 15
}
```

Returns `{ "nodes": [...], "staleness": { "reingest_recommended": false }, "truncated": false }`. The `nodes` list is ranked: `ValidateToken`, `CheckExpiry`, `ParseClaims`, etc. — only the ~15 symbols relevant to this concern, not the full 400-symbol index.

**3. Get neighbourhood context** for the primary symbol:

```json
symbol_context {
  "service": "auth-service",
  "symbol": "ValidateToken"
}
```

Returns `{ "def": {...}, "callers": ["HandleLogin", "AuthMiddleware"], "callees": ["ParseClaims", "CheckExpiry"], "related_tests": ["TestValidateToken", "TestValidateToken_Expired"] }`.

**4. Pack into a token budget** for the actual edit context:

```json
pack_context {
  "service": "auth-service",
  "concern": "auth token validation",
  "budget": 3000
}
```

Returns `{ "included": [...], "tokens": 2847, "dropped": 4, "budget": 3000 }`. The four lowest-scoring symbols were dropped to stay within budget.

**5. (Optional) Batch orientation calls** to save round-trips at session start:

```json
batch {
  "calls": [
    { "name": "entrypoints",    "arguments": { "service": "auth-service" } },
    { "name": "repo_map",       "arguments": { "service": "auth-service", "budget": 2000 } },
    { "name": "outline",        "arguments": { "service": "auth-service", "file": "internal/auth/token.go" } }
  ]
}
```

Returns all three results in one response. Use `batch` when the calls are independent and you want to minimise latency.

---

## Setup & notes

**Surface**: `set_profile` was removed (surfaces week-1, SW-2). Over HTTP the path is the scope — `POST /rpc/agent` lists the 19-tool allowlist (`docs/mcp-tools.md`) and bare `POST /rpc` lists everything (the group endpoints were deleted — HF-5). Off-surface calls execute and are logged `off_surface:true` — surfaces are ergonomics and measurement, never a security boundary. stdio `serve-mcp` always serves the full registry.

**Raw text without leaving the surface**: `read_source { "service", "file", "start_line", "end_line" }` returns numbered lines from the *indexed* corpus — cite them as `file:line`. It reads what ingest walks: a `.fathomignore`'d or `fathom scope`-suppressed path is refused by name, with the unsuppress command in the refusal.

**Memories beside obligations** (**experimental**, AM-6/AM-8): once a repo has `.fathom/memories.jsonl`, the edit-hook payload gains a `🧠 fathom memory — <file>:` block after the obligation lines, listing memories anchored to the edited file with `[tier, state]`, inside the same 150-token budget (displaced count named). `memory_search { "service", "query"?, "file"?, "limit"? }` is the pull path; retired memories never surface. File one with `fathom memory add --claim … --anchor symbol:Foo --evidence machine-check:…` — no anchor, no memory (put preferences in CLAUDE.md instead).

**Concern is required**: `query_symbols` and `pack_context` both require a `concern` argument. This is enforced at the API level — calls without it are rejected. The concern should describe the functional area being changed (e.g. `"auth token validation"`, `"payment retry logic"`), not a file name or symbol name.

**Re-ingest when stale**: `serve-mcp` is a long-running process; the index can fall behind after commits. Check `staleness_report` or inspect the `staleness` field on `query_symbols` results. To refresh in-session, call the `ingest` MCP tool — a concurrent CLI `ingest` can contend with the session's own writes (SQLite serializes writers, ADR-020) and fail past `busy_timeout`.

**Budget units**: `pack_context` and `repo_map` `budget` is in tokens (default 4000). `read_source` `max_bytes` is in bytes of the `lines` array as sent (default 8000, ceiling 16000). Set these according to your model's context headroom. Elision is always disclosed in the response, never silent.

**Project-settable defaults** (**experimental**, BA-9): a `budgets` block in `.fathom/capabilities.cue` (project, governs) or `~/.fathom/capabilities.cue` (user default) sets the class defaults in tokens — `budgets: { list: 1500, composite: 4000, map: 2500 }`, where `list` = `query_symbols`/`references`, `composite` = `pack_context`, `map` = `repo_map`. The block is closed: an unknown key is refused with a did-you-mean. `find_symbol`, `find_callers`, `symbol_context` and `outline` are not class-budgeted. Honoured by the `serve-http` daemon; stdio `serve-mcp` still uses the built-ins.

**Priming a session without hooks**: `fathom prime` (Codex, or any harness with no SessionStart hook) prints the same "this repo is service X — resolve via fathom before grep" brief the `session-hook` injects.

**Static context injection**: for sessions where the MCP server is not available, run `fathom gen-claude-md --service auth-service` to write a `CLAUDE.md` to `workspace/artifacts/CLAUDE.md`. This gives the agent a static symbol index to reason from, though it does not support concern scoping or staleness detection.

---

## Decision-moment obligations

When an agent edits a file it should know the governing rules for that file — architectural constraints, naming conventions, test requirements — before writing, not after review catches a violation. Fathom surfaces these via the compiled obligations layer (ADR-016).

**Primary channel — PostToolUse edit-hook:** The `fathom edit-hook` CLI subcommand is wired as a Claude Code `PostToolUse` hook (matcher: `Edit|Write|MultiEdit`). After every file write it calls `obligations` on the file just edited and injects any governing rules into the session context. Silent on clean files (zero-output-on-clean contract); silent on any error so it never blocks a write.

**Pull path — `obligations` MCP tool:** Call directly when you need to inspect governing rules before editing or when the edit-hook is not wired:

```json
obligations {
  "service": "my-service",
  "file": "internal/auth/token.go"
}
```

Returns `{service, file, rules?, compiled_at_watermark?, index_stale?}`. An empty response with no `index_stale` field means no obligations apply to the file. Each rule carries its source citation and evidence tier; stale or contradicted rules are flagged explicitly. `max_rules` caps the list (default 6).

**Compile the layer first:** obligations requires a prior `compile_layer` run to populate claims (`dry_run` defaults true — estimate cost before executing). This is a one-time spending step per service; see `docs/design/coding-agent-excellence.md §5.2` for the design rationale.

---

## Maintained

This doc covers: `query_symbols`, `find_symbol`, `outline`, `repo_map`, `pack_context`, `symbol_context`, `read_source`, `references`, `memory_search`, `find_callees`, `entrypoints`, `batch`, `staleness_report`, `gen-claude-md`, `obligations`, `edit-hook` (incl. the memory block), `fathom memory` (`add`/`list`/`review`/`show`), `fathom prime` (listed by `fathom help`; `fathom prime -h`), the `budgets` block in `capabilities.cue`, and the `/rpc/agent` surface facts. Update it when these change.

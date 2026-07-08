# Concern-scoped coding context

Give Claude Code (or any AI coding agent) a precise, token-efficient context slice instead of dumping a whole repo into the prompt.

---

## When to use this

- An agent is about to edit code in a specific area and needs to understand the relevant symbols, callers, and call graph without reading every file.
- You want to assemble a tight, ranked context bundle that fits inside a token budget.
- You need a paste-ready brief for a worker sub-agent that should not receive the full codebase.
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
| `brief` | MCP tool | Paste-ready markdown brief combining symbol resolution + concern query; respects a char budget |
| `symbol_context` | MCP tool | Definition + direct callers + direct callees + related tests for one symbol |
| `find_callees` | MCP tool | Direct callees of a named symbol |
| `call_hierarchy` | MCP tool | Bidirectional call tree rooted at a symbol (callers and callees, configurable depth) |
| `entrypoints` | MCP tool | Likely program entry points: `main` functions, HTTP handlers, CLI root commands |
| `batch` | MCP tool | Execute N tool calls in a single request |
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
- **Worker-ready briefs**: `brief` produces a single markdown string a parent agent can paste into a sub-agent's prompt, with explicit truncation notices when the budget is reached.

---

## Worked example

Scenario: an agent is about to modify token validation in `auth-service` and needs tight context.

**1. Switch to the coding profile** to filter the tool surface to coding-relevant tools.

On stdio `serve-mcp`:

```json
set_profile { "profile": "coding" }
```

Returns: `{ "profile": "coding", "ok": true }`

Over the shared HTTP daemon, `set_profile` is rejected — register against `POST /rpc/companion` (curated 15-tool coding hot-loop) or `POST /rpc/coding` instead.

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

**5. (Optional) Prepare a worker brief** if handing off to a sub-agent:

```json
brief {
  "service": "auth-service",
  "concern": "auth token validation",
  "symbols": ["ValidateToken", "CheckExpiry"],
  "budget": 16000
}
```

Returns `{ "brief": "# auth-service — auth token validation\n\n..." }` — a self-contained markdown string ready to paste.

**6. (Optional) Batch orientation calls** to save round-trips at session start:

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

**Profile**: set `coding` at session start (stdio `serve-mcp` only — call `set_profile { "profile": "coding" }`; over the shared HTTP daemon, register against `POST /rpc/companion` or `POST /rpc/coding` instead). This exposes `query_symbols`, `symbol_context`, `find_callees`, `call_hierarchy`, `outline`, `entrypoints`, and the shared tools (`find_symbol`, `repo_map`, `pack_context`, `brief`, `batch`). Without a profile set, all tool groups are visible.

**Concern is required**: `query_symbols` and `pack_context` both require a `concern` argument. This is enforced at the API level — calls without it are rejected. The concern should describe the functional area being changed (e.g. `"auth token validation"`, `"payment retry logic"`), not a file name or symbol name.

**Re-ingest when stale**: `serve-mcp` is a long-running process; the index can fall behind after commits. Check `staleness_report` or inspect the `staleness` field on `query_symbols` results. To refresh in-session, call the `ingest` MCP tool — do not use the CLI `ingest` subcommand while the session holds the single-writer DuckDB lock.

**Budget units**: `pack_context` and `repo_map` `budget` is in tokens (default 4000). `brief` `budget` is in characters (default 16000). Set these according to your model's context headroom.

**Static context injection**: for sessions where the MCP server is not available, run `fathom gen-claude-md --service auth-service` to write a `CLAUDE.md` to `workspace/artifacts/CLAUDE.md`. This gives the agent a static symbol index to reason from, though it does not support concern scoping or staleness detection.

**`call_hierarchy` depth**: defaults to 2, maximum 5. Keep it low (2–3) for tight context; higher depths expand rapidly and may exceed budget.

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

This doc covers: `query_symbols`, `find_symbol`, `outline`, `repo_map`, `pack_context`, `brief`, `symbol_context`, `find_callees`, `call_hierarchy`, `entrypoints`, `batch`, `set_profile`, `staleness_report`, `gen-claude-md`, `obligations`, `edit-hook`. Update it when these change.

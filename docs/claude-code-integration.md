# Claude Code integration guide

How to wire sextant into Claude Code so coding sessions (and every subagent
they spawn) query a pre-built index instead of re-reading the repo. This is
the playbook proven on a four-repo fleet on 2026-06-12; measured results in
[assessment-claude-code-integration.md](assessment-claude-code-integration.md)
— headline: discovery agents eliminated entirely, wave wall-clock roughly
halved, comparable workers dropped from ~148 tool calls to ~32.

## 1. Build and index

```sh
cd sextant
CGO_ENABLED=1 go build -o bin/sextant ./cmd/sextant   # macOS: MACOSX_DEPLOYMENT_TARGET=14.0 silences linker warnings

# One store for the whole fleet — one service per repo:
for repo in my-app my-lib; do
  bin/sextant ingest --service $repo --path ../$repo --store workspace/db/fleet.duckdb
done
```

Full ingest is seconds per repo; re-ingest is hash-gated (~0.2 s when clean).

## 2. `.sextantignore` — do this BEFORE first ingest

Gitignore-style file at each repo root. Two purposes:

1. **Noise**: exclude mirrored third-party source, scratch dirs, build
   output, generated code (index the sources, not the codegen — e.g.
   `*_templ.go` when `.templ` files are indexed).
2. **Privacy — non-negotiable**: exclude any directory holding user data,
   case files, secrets, or PII. The index is readable by every connected
   MCP client and sits OUTSIDE any application egress/DLP gate.

```gitignore
ref/
tmp/
workspace/
bin/
data/          # user/case data — never index
*_templ.go
```

Skipping this cost us 15k junk symbols on day one (own repo: 2,427 files
indexed → 222 after).

## 3. Register the MCP server

User scope makes it available in every repo and to every subagent:

```sh
claude mcp add --scope user sextant -- \
  /abs/path/sextant/bin/sextant serve-mcp --store /abs/path/sextant/workspace/db/fleet.duckdb
claude mcp list   # expect: sextant … ✔ Connected
```

Beware shell line-wrapping: a wrapped command registers `--store` without
its value and the server silently fails to start. Verify with `claude mcp
list` every time. Sessions pick the server up at start — a rebuild of the
binary or store needs a session restart to take effect.

## 4. The operating contract (CLAUDE.md) — where the value actually comes from

The index pays off only if agents are *told* to use it. Add a section like
this to each repo's CLAUDE.md (adapt the service name):

```markdown
## Sextant fleet index (code intelligence — use it first)

1. **Index before exploration.** Resolve "where/what/who-calls" questions
   via sextant tools (`find_symbol`, `query_symbols`, `find_callers`,
   `outline`, `repo_map`, `pack_context`) before any Grep/Read sweep.
   Read files to edit them, not to discover them.
   **Never re-discover what the architect already discovered:** worker
   prompts carry a pre-packed context section (exact signatures, file
   paths, verbatim contract types). Workers trust it and verify only what
   they change; if it conflicts with the code, STOP and report — never
   silently drift.
2. **Refresh:** call the MCP `ingest` tool ({"service":"<repo>"}) at
   session start and after each commit — hash-gated, ~0.2 s when clean.
3. **Honesty:** after heavy uncommitted edits, run `staleness_report`
   before trusting results.
4. **Privacy:** keep sensitive dirs in `.sextantignore`.
```

Name the ~6 tools that matter. Don't list all ~80 — modern Claude Code
defers MCP tool schemas (near-zero context cost when unused); the curation
that matters is in the prompt, not the server.

## 5. Refresh discipline (single-writer constraint)

DuckDB allows ONE writing process. While any Claude Code session is open,
its MCP server holds the store lock and **CLI ingest fails** — measured,
not theoretical. Therefore:

- **Primary**: refresh through the live server's `ingest` MCP tool
  (session start + after each commit). Encode it in CLAUDE.md so the agent
  does it unprompted.
- **Belt-and-braces**: a best-effort post-commit hook for commits made with
  no session running (lefthook example):

```yaml
post-commit:
  commands:
    sextant-refresh:
      run: |
        [ -x ../sextant/bin/sextant ] || exit 0
        ../sextant/bin/sextant ingest --service my-app --path . \
          --store ../sextant/workspace/db/fleet.duckdb >/dev/null 2>&1 || true
```

No daemon/watcher needed: hash-gating makes on-demand refresh effectively
free, and `staleness_report` makes drift visible instead of silent.

## 6. The pre-packed brief pattern (biggest tokenomics lever)

The architect does recon ONCE via the index, then ships it verbatim inside
each worker prompt:

- exact function/type signatures workers will call or implement,
- file paths and line-anchored snippets for everything load-bearing,
- the inter-team contract as literal Go (or your language) declarations,
- the instruction: *trust this section; verify only what you change; if
  the code disagrees, stop and report the conflict — never adapt silently.*

Re-derivation is the most expensive failure mode (it's the whole pre-index
baseline). The stop-and-report clause matters: a worker that quietly
deviates from a pinned contract produces code that compiles and then
no-ops at runtime — we hit exactly this once before adding the rule.

## 7. Pitfalls checklist

- [ ] `.sextantignore` written before first ingest (privacy + noise).
- [ ] `claude mcp list` shows ✔ Connected after registration.
- [ ] CLAUDE.md operating contract added to EVERY repo in the fleet.
- [ ] Session restart after rebuilding the sextant binary or recreating
      the store (the running server keeps its old file handle).
- [ ] Schema/extractor changes need a from-scratch re-ingest — the hash
      gate skips unchanged files, so new extractors never see them
      otherwise (`rm` the store and re-ingest; it's seconds).
- [ ] Don't trust mid-wave query results for files you just edited:
      refresh first or read the file.

## 8. What to expect (measured on a 4-repo fleet, ~12k symbols)

- Concern query answering "where is X and what governs it": one ~600-token
  response replacing a 30–80k-token Explore-agent sweep — including design
  docs and schemas ranked alongside the code they govern.
- Discovery-tier agents: eliminated (2 per wave → 0).
- Comparable implementation workers: ~148 tool calls → ~32 at similar
  token size (spend moves from searching to building).
- Wave wall-clock: roughly halved.
- Worker token totals per wave: only modestly lower — read-to-edit and
  code-writing are irreducible; set expectations accordingly.

Full numbers, method, and caveats:
[assessment-claude-code-integration.md](assessment-claude-code-integration.md).

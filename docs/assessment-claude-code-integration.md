# Assessment: Fathom × Claude Code integration (2026-06-12)

> **Historical record (2026-06-12; banner added 2026-08-25).** The measured
> numbers below are a point-in-time record and stay accurate for that day;
> the stack they describe has moved on. The store is pure-Go SQLite, not
> DuckDB (ADR-020/025 — [why-duckdb.md](why-duckdb.md)); the registry is 54
> tools plus `memory_search`, and a session reaches the 17-tool `/rpc/agent`
> surface ([mcp-tools.md](mcp-tools.md)); the current wiring guide is
> [use-cases/coding-agents.md](use-cases/coding-agents.md). The §5 backlog is
> not the roadmap — `fathom plans status --service fathom` and
> [intent/scope.md](intent/scope.md) are. The savings figures here predate
> the A/B counterfactual (experimental: ON arm still collecting, ≥20 units per
> arm before any savings claim is admissible, law 12).

Owner-requested report on whether wiring the fathom fleet index into Claude
Code sessions improved **effectiveness, correctness, and tokenomics** — with
measured numbers where they exist, estimates labeled as estimates, and an
honest verdict on whether this reinvented existing wheels.

## 1. What was integrated (one day, 2026-06-12)

- Fleet index: throughline + agentic-kit + app-kit + fathom in one store
  (DuckDB at the time; SQLite since ADR-025) — ~12k symbols, 531 doc chunks,
  CUE definitions, Templ components; ~4 MB; full rebuild seconds; hash-gated
  no-op re-ingest ~0.2 s.
- MCP server registered user-scope → every Claude Code session (and every
  subagent it spawns) got the whole registry (~80 tools then; 54 +
  `memory_search` today, 17 on the agent surface); the six that matter are
  named in each repo's CLAUDE.md: `find_symbol`, `query_symbols`, `find_callers`,
  `outline`, `repo_map`, `pack_context`.
- Operating contract in all four repos: index-before-exploration; refresh
  via the MCP `ingest` tool at session start + post-commit (the DuckDB store
  was single-writer — measured then: CLI ingest failed on the lock while a
  session lived; SQLite serializes writers instead, ADR-020; best-effort
  post-commit lefthook covers no-session commits);
  `staleness_report` before trusting results after heavy edits;
  `.fathomignore` keeps case data out (the index sits outside the DLP gate).
- Pre-packed worker briefs codified: the architect's fathom recon (exact
  signatures, paths, verbatim contract types) ships inside worker prompts;
  workers never re-discover it, and must stop-and-report on contract
  conflicts instead of silently drifting.

Integration-day fixes the dogfooding itself forced (all committed same day):
`.fathomignore` + dir-only filter bug (own index 2,427→222 files), engine
keyword fallback (query_symbols/pack_context were dead over MCP), staleness
report trimmed from ~6 KB of fresh-file rows per query to stale-only +
counts (~100×), bare-name `find_symbol`, `cross_repo_callers` (returned 0
always — matcher never handled the `pkg.Name` edge format the Go extractor
emits), result dedup, and a Templ extractor (Throughline's entire UI layer —
128 components incl. Go→Templ call edges — was invisible).

## At a glance

(Full SVG version: [assessment-visual.html](assessment-visual.html) — open in a browser.)

```text
One discovery question ("where is X, who calls it, which doc governs it")
  before  ██████████████████████████████████████████████  ~55,000 tok (Explore sweep, 30k–80k)
  after   ▏                                                   ~600 tok (one index query)   ~90× less

Discovery agents per feature wave
  before  ██  2
  after       0

Tool calls per comparable backend worker
  M17.4 (before)  █████████████████████████████████████  148
  M17.3 (before)  ██████████████████████████████          120
  M18.1 (after)   ████████                                 32
  M18.2 (after)   ███████████▌                             47

Wave wall-clock, dispatch → verified commit
  M17.7/8 (before)  ███████████████████████████  ~105 min
  M18.1   (after)   ███████████▌                  ~45 min
  M18.2   (after)   ████████████▎                 ~48 min

Worker tokens per wave (honest: implementation cost is irreducible)
  M17.7/8 + Explore (before)  █████████████████████  ~421k (321k recorded + ~100k est. Explore)
  M18.1 (after)               █████████████           258k, zero Explore
  M18.2 (after)               ██████████▋             213k, zero Explore

Whole-period subagent spend (context only — scopes differ)
  pre  (4.5 days, M16+M17+reviews)  ███████████████████████████████  11.77M tok · 8,333 calls
  post (2.5 h, M18.1+M18.2+fathom) █▋                                0.60M tok ·   337 calls

Integration-day dogfooding fixes (fathom's own hygiene)
  own-repo files indexed   2,427 → 222     (.fathomignore + dir-filter fix)
  staleness block / query  ~6 KB → ~60 B   (stale-only + counts)
  cross_repo_callers       always 0 → works (edge-format match bug)
```

## 2. Measured numbers

Method: every subagent completion in the session transcript records
`subagent_tokens` + `tool_uses`. Records deduped by task-id and bucketed
against the cutover (2026-06-12 21:10 UTC, first session with the registered
server). Two honesty caveats: (a) **Explore-agent runs record no token
usage**, so pre-integration totals *understate* the real baseline (those
waves used raw-walk Explore agents; post-integration waves used none);
(b) architect main-loop tokens are not separable in either era.

**Whole-period frame** (scope differs — context, not a like-for-like):

| | runs | subagent tokens | tool uses |
|---|---|---|---|
| Pre (06-08 → 06-12 21:10, M16+M17 era) | 38 | 11,771,296 | 8,333 |
| Post (06-12 21:10 → 23:50, M18 era) | 7 | 599,422 | 337 |

**Wave-level frame** (fairest: same architect, same team pattern, similar
scope per wave — one product milestone built, reviewed, live-verified,
committed):

| Wave | Workers | Worker tokens | Explore agents | Wall clock |
|---|---|---|---|---|
| M17.7+17.8+17.6 (pre) | 3 | 321,199 | 2 (cost unrecorded) | ~1 h 45 m |
| M17.1–17.4 (pre, 4 milestones) | 4 | 463,772 | several (unrecorded) | spread over day |
| M18.1 (post) | 2 | 258,465 | **0** | ~45 m |
| M18.2 (post) | 2 + 1 dead retry | 213,215 | **0** | ~48 m |

**Architect recon** (from the session itself): M18.1 contracts were pinned
from 6 index calls + 3 targeted file reads; M18.2 from 2 index calls + 4
reads + 2 greps. Pre-integration the equivalent step was 1–2 background
Explore agents per wave plus wider architect reads.

**Per-worker behavior shift**: comparable backend builds went from ~148 tool
uses (M17.4) to ~32 (M18.1 backend) at similar token size — fewer, larger,
better-aimed actions; the spend moved from searching to building. Worker
token totals per wave did *not* collapse, and shouldn't: read-to-edit and
writing code are irreducible. The eliminated category is **discovery and
re-derivation** — the Explore tier is simply gone, and wall-clock per wave
roughly halved.

## 3. Effectiveness and correctness (qualitative, with receipts)

- **Docs+code fusion is real**: querying concern "element-coverage" returned
  the governing design-doc section (doc 31 §M18.1) *and* the exact code it
  governs (`buildEvidenceMap`, kit `Assess` signature) as one ranked list.
  Both M18 designs were grounded this way before any file was opened.
- **Correctness substitute for a broken tool**: gopls is chronically stale in
  this multi-module workspace (documented failure mode across sessions);
  `find_callers` answered who-calls questions correctly every time it was
  used, including across the Go→Templ language boundary.
- **Evidence-tier honesty carries over**: query results carry tiers and a
  staleness summary; the operating contract makes "the index might be stale
  mid-wave" an explicit, managed state instead of a silent wrong answer.
- **What it did not fix**: worker contract-drift (M18.1's count-vs-links
  signature drift compiled and would have no-opped at runtime) is a
  coordination failure, not a discovery failure — caught by architect
  integration review both eras at a similar rate. The fix was procedural
  (stop-and-report rule), not index-shaped.

## 4. Did we reinvent the wheel?

Honest component-by-component comparison:

- **LSP / gopls (incl. LSP-over-MCP servers like Serena)**: covers maybe
  60% of the *code-navigation* value (definitions, references) — when it
  works. It is stale-prone in this exact workspace, single-language,
  single-repo-rooted, knows nothing about Markdown design docs, CUE schemas,
  or Templ, and carries no notion of evidence tiers or staleness honesty.
- **Aider's repo-map / similar token-budget maps**: equivalent to our
  `repo_map`/`pack_context` for one repo; no query model, no concern
  ranking, no fleet, no docs fusion.
- **Sourcegraph / GitHub code search / Glean**: real cross-repo symbol+ref
  indexes — but cloud-hosted (a non-starter: case evidence adjacency +
  local-first doctrine), heavyweight, and still code-only.
- **ctags/tree-sitter indexers, embeddings-RAG over the repo**: either
  symbols without semantics or semantics without structure; none ship an
  MCP surface, deterministic hash-gated freshness, or call edges.

Verdict: **no single existing tool does this combination**, and the
combination is where the measured value came from: (1) code + design docs +
schemas + templates in *one ranked query surface*, (2) fleet-wide across
four sibling repos that genuinely co-evolve, (3) local-first single-file
artifact with deterministic incremental refresh, (4) MCP-native so every
subagent inherits it for free, (5) the project's own honesty discipline
(tiers, staleness, no fabricated relevance) applied to the dev tooling.
Roughly 60% of the raw capability overlaps things that exist; the 40% that
doesn't (doc-chunk fusion, fleet queries, honesty semantics, and full
control to extend it — e.g., today's Templ extractor in one afternoon) is
exactly the part this codebase's workflow leaned on hardest. Building it was
not wheel-reinvention; *not* having control of it would have meant living
with gopls-grade staleness and code-only answers.

## 5. Open improvement backlog (owner-approved direction: keep tuning)

1. Line numbers in symbol results (in flight — observations don't persist
   them; needs store column + re-ingest).
2. Mid-wave staleness: `ingest-diff` on query-miss or an editing-session
   watch mode, so heavy uncommitted edits stop requiring discipline.
3. Spec-drift phase 2: teach `internal/sdd` the doc/milestone ID conventions
   (`doc 31`, `M18.2`) + tracker parsing → "which files implement M18.2" as
   a query, drift as a report.
4. Pre-packed brief generator: a `brief` tool that assembles the architect's
   pinned-contract pack (symbols + signatures + doc excerpts) in one call.
5. LLM knowledge cards + persisted embeddings: deferred until cost-gated;
   the deterministic layer is carrying the load fine so far.

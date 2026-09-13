# Competitors — install, run, query, cost, exactly as documented

M6/M7 (`docs/vision/27-strengths-bench.md` §6, plan `docs/plans/strengths-bench.md`
wave 4) score fathom's index against a static graph (graphify) and two LSP-backed
MCP servers (Serena, gopls) — never fathom's opinion of them. Every command below
is copied from the tool's own docs, at the pinned version, on the date verified.
`internal/indexbench.LoadGraphifyEdges` and `ProbeReferencesViaMCP` consume exactly
these outputs; `testdata/scenariobench/lookups-vs.json`'s arms run exactly these
commands as `setup` / `mcp`.

## graphify

**Verified against**: github.com/Graphify-Labs/graphify (formerly safishamsi/graphify),
Apache-2.0, release v0.9.61 (2026-09-12), verified 2026-09-13.

- **Install**: `pip install graphifyy && graphify install`
- **Build**: `/graphify .` inside Claude Code (code files go through a
  tree-sitter AST — no LLM; docs go through the host LLM). A plain-CLI build
  form also exists — check the installed README for the exact invocation
  before scripting it (`graphify --help` names it).
- **Query surface**: `graphify --mcp` starts a stdio MCP server exposing
  `query_graph`, `get_node`, `get_neighbors`, `get_community`, `god_nodes`,
  `graph_stats`, `shortest_path`, `list_prs`, `get_pr_impact`, `triage_prs`.
- **Output format**: `graphify-out/graph.json`, a networkx node-link graph —
  `nodes`: `{id, label, file_type, source_file, source_location, community}`;
  `edges`: `{source, target, relation, confidence, source_file,
  source_location, weight}`, `relation ∈ {imports_from, uses, contains,
  calls, inherits, method, mixes_in, references, depends_on}`, `confidence ∈
  {EXTRACTED, INFERRED, AMBIGUOUS}`, `source_location` like `"L6"`.
  `LoadGraphifyEdges` (`internal/indexbench/graphify.go`) keeps only the
  `calls` and `references` relations — the two that are symbol→symbol
  structural edges a call-graph truth can adjudicate — and names every other
  relation it drops (`EdgeSet.DroppedKinds`).
- **Staleness model**: a precomputed artifact. `graphify --update` refreshes
  it incrementally by content hash; `graphify --watch` keeps it live while
  editing; `graphify hook install` wires a post-commit refresh. Between
  refreshes the artifact answers from before the edit — the freshness task
  (M7) in `lookups-vs.json` is built around exactly this gap.
- **License**: Apache-2.0.
- **Cost shape**: precompute. One `/graphify .` (or CLI-form) build pays the
  whole indexing cost; every query after that reads the committed
  `graph.json` and costs nothing further until the next `--update`.
- **Bench arm command** (`lookups-vs.json`, arm `graphify`): `setup: ["pip
  install graphifyy", "graphify install", "/graphify ."]`, `mcp:
  {"graphify": {"command": "graphify", "args": ["--mcp"]}}`.

## Serena

**Verified against**: github.com/oraios/serena, MIT, release v1.7.0
(2026-08-09), verified 2026-09-13.

- **Install**: `uv tool install -p 3.13 serena-agent`
- **Run / register**: `claude mcp add serena -- serena start-mcp-server
  --context claude-code --project "$(pwd)"`. Go support requires `gopls` on
  `PATH` (Serena drives it as its Go language-server backend).
- **Query surface** (the tools this bench calls): `find_symbol
  {name_path, kind, relative_path, body_location}`; `find_referencing_symbols
  {name_path, relative_path}` → one entry per reference:
  `{name_path, relative_path, kind, line}`; `find_implementations`;
  `find_declaration`; `get_symbols_overview`.
- **Output format**: JSON tool results over MCP — no separate on-disk
  artifact; every answer is computed on demand from the live `gopls`
  session Serena drives.
- **Staleness model**: query on demand. Serena holds no precomputed graph;
  each call re-asks the LSP backend against the file on disk, so a query
  right after an edit sees the edit.
- **License**: MIT.
- **Cost shape**: per-query. No build step; the first query pays for
  spinning up `gopls`, every query after that is a live LSP round trip.
- **Bench arm command** (`lookups-vs.json`, arm `serena`): `mcp:
  {"serena": {"command": "serena", "args": ["start-mcp-server",
  "--context", "claude-code", "--project", "."]}}`.
- **`ProbeReferencesViaMCP` argument mapping** (`internal/indexbench/mcpprobe.go`):
  tool `find_referencing_symbols`, args `{"name_path": <symbol>,
  "relative_path": <declaring file>}`; response parsed as the JSON array
  above into `file:line` edges.

## gopls (MCP mode)

**Verified against**: gopls ≥ v0.20.0, BSD-3-Clause (the `golang.org/x/tools`
license), verified 2026-09-13.

- **Install**: `go install golang.org/x/tools/gopls@latest`
- **Run / register**: `gopls mcp` (stdio) or `gopls serve
  -mcp.listen=localhost:PORT` (TCP); `claude mcp add gopls -- gopls mcp`.
- **Query surface** (the tools this bench calls): `go_context`,
  `go_diagnostics`, `go_file_context`, `go_file_diagnostics`,
  `go_file_metadata`, `go_package_api`, `go_references`,
  `go_rename_symbol`, `go_search`, `go_symbol_references`, `go_vulncheck`,
  `go_workspace`. `go_references` / `go_symbol_references` return **prose**,
  not JSON: `"Located in the file: <path>"` … `"located on line N"`
  (1-indexed).
- **Output format**: prose tool results over MCP; no on-disk artifact.
- **Staleness model**: query on demand, same as Serena — `gopls` re-analyzes
  from the file on disk (with its own incremental cache) on every call.
- **License**: BSD-3-Clause.
- **Cost shape**: per-query. No build step; `gopls`'s own package-loading
  cache makes the first query in a workspace the expensive one.
- **Bench arm command** (`lookups-vs.json`, arm `gopls`): `mcp: {"gopls":
  {"command": "gopls", "args": ["mcp"]}}`.
- **`ProbeReferencesViaMCP` argument mapping**: tool `go_symbol_references`,
  args `{"symbol": <symbol>, "file": <declaring file>}`; response parsed by
  matching each `"Located in the file: …"` mention to the next `"located on
  line N"` mention, positionally, into `file:line` edges.

## What is NOT run on this machine

Per the plan's rules, none of the three tools above is installed, built or
queried against a real process on the machine that authored this bench —
`ProbeReferencesViaMCP` is proven against a scripted fake stdio MCP server
(`internal/indexbench/mcpprobe_test.go`) and `LoadGraphifyEdges` against a
committed fixture `graph.json` (`internal/indexbench/testdata/`). Running
the real tools (`SB-40`/`SB-41`, plan wave 6) is a separate, operator-gated
step that spends real installs and, for the paid lookup pilot, real tokens.

# The Assistant — `appkit-mcp` design

The kit's knowledge made ambient. One Go binary, installed into every project via
a committed `.mcp.json`, that lets any agent in any repo ask: *what component, what
pattern, what tokens, what decision — and why.*

Governing decisions: [ADR-0011](../adr/0011-mcp-design-assistant.md) (SDK, binary
shape, search-then-get), [ADR-0010](../adr/0010-component-data-pipeline.md) (the
index it queries), [ADR-0015](../adr/0015-ai-first-surface.md) (agents are the
user). Acceptance: VISION.md's ambient-knowledge test — in any project, an agent
with the Assistant connected answers "how do I build this screen the kit way?"
correctly, without a human explaining the kit.

---

## 1. Mission

Maximize two quantities in **every** project, kit-based or not:

1. **Kit adoption.** In a kit app, the Assistant keeps the project on-convention
   and on-version (`audit_project`, `upgrade_guide`). In a *foreign* project — a
   legacy React app, a one-off script, Throughline before its rebuild — the
   Assistant is the adoption wedge: `audit_project` reports "kit not present" plus
   concrete incremental steps, and the component corpus carries `html`/`react`/`vue`
   flavors (D10), so the search tools are genuinely useful before a single kit
   import exists. Value first, migration second.
2. **Design quality.** The TailwindPlus corpus, the kit's own components, the
   UI-architecture patterns, the design tokens, and the ADR corpus are all one
   query away. The agent building a screen should reach for `suggest_pattern` and
   `search_components` as reflexively as it reaches for grep.

The Assistant is *read-mostly and deterministic in v1*: it returns knowledge and
generated file contents; it does not write to the consumer repo, call models, or
hold sessions. The calling agent does the writing — it sees the diff, the human
sees the agent.

## 2. Shape of the server

One binary, `cmd/appkit-mcp`, on the official go-sdk ≥ v1.6.1 (pinned; see
ADR-0011). stdio default; `-http` flag for a long-lived shared instance.

```go
// cmd/appkit-mcp/main.go
func main() {
    httpAddr := flag.String("http", "", "serve Streamable HTTP on addr (e.g. 127.0.0.1:4910) instead of stdio")
    showVer := flag.Bool("version", false, "print version and index manifest digest")
    flag.Parse()

    cfg := assistant.ConfigFromEnv() // APPKIT_MCP_* vars only; no config files
    srv := assistant.New(cfg)        // *mcp.Server with all tools/resources/prompts registered

    switch {
    case *showVer:
        fmt.Println(assistant.VersionLine())
    case *httpAddr != "":
        h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
        log.Fatal(http.ListenAndServe(*httpAddr, h)) // loopback only per D14; SDK origin checks stay ON
    default:
        if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
            log.Fatal(err)
        }
    }
}
```

Configuration is env/flags only — consumer repos carry nothing but `.mcp.json`:

| Env var | Default | Meaning |
|---|---|---|
| `APPKIT_MCP_OLLAMA_URL` | `http://127.0.0.1:11434` | embedder endpoint for query vectors |
| `APPKIT_MCP_EMBED_MODEL` | pinned qwen3-embedding-0.6B digest (from index manifest) | must match the index's pinned model (D10) |
| `APPKIT_MCP_INDEX` | *(empty → embedded index)* | dev override: path to an external SQLite index |
| `APPKIT_MCP_EMBED_TIMEOUT` | `300ms` | budget before falling back to BM25-only |

Package layout in the monorepo (D13):

```
cmd/appkit-mcp/            main.go (flags, transport selection only)
internal/assistant/        server.go — registration of tools/resources/prompts
internal/assistant/comp/   component search/get (queries the D10 index)
internal/assistant/pat/    patterns: list/get/suggest
internal/assistant/theme/  tokens + theme CSS (reads the CUE token source via cuelang.org/go)
internal/assistant/adopt/  audit_project, scaffold_app, upgrade_guide
internal/assistant/dec/    ADR/decision corpus tools
internal/twplus/index/     shared with the pipeline: index open, hybrid RRF query, schema
```

## 3. Tool surface

Fifteen tools, five families — the **target** surface, shipped by ROADMAP phase:
v1 (Phase 2) is the components + decisions families only; patterns + tokens land
in Phase 3; adoption in Phase 5 (each family is tagged below). Every list/search
tool obeys the return-shape rules
in §4. Input structs use the SDK's generic `AddTool[In,Out]` with `json` +
`jsonschema` tags; descriptions are ≤ 2 sentences each so the *schema* cost of the
whole server stays under ~4k tokens (the 143k-token schema horror story is the
anti-pattern we design against — see ADR-0011 Context).

### 3.1 Components

*Ships in v1 (ROADMAP Phase 2).*

Backed by the D10 SQLite index (~657 components × ~12 variants ≈ 7.9k variant
records, `Section.Group.Category.Name` taxonomy).

**`search_components`** — the workhorse. Hybrid retrieval: structural filters as
SQL `WHERE`, FTS5 BM25 over names/descriptions/code, sqlite-vec KNN over the query
embedding, fused with Reciprocal Rank Fusion (k≈60) per the sqlite-vec author's
documented pattern (https://alexgarcia.xyz/blog/2024/sqlite-vec-hybrid-search/index.html).

```go
type SearchComponentsIn struct {
    Query           string `json:"query" jsonschema:"natural language or keywords, e.g. 'pricing table with toggle'"`
    Framework       string `json:"framework,omitempty" jsonschema:"html|react|vue; default html"`
    TailwindVersion int    `json:"tailwind_version,omitempty" jsonschema:"3|4; default 4"`
    Mode            string `json:"mode,omitempty" jsonschema:"light|dark|system|none"`
    Section         string `json:"section,omitempty" jsonschema:"e.g. marketing|application-ui|ecommerce"`
    Category        string `json:"category,omitempty"`
    Limit           int    `json:"limit,omitempty" jsonschema:"default 10, max 20"`
    Offset          int    `json:"offset,omitempty"`
}
```

Returns: `{items: [{name, summary, section, category, frameworks}], total,
has_more, next_offset, search_mode}` where `search_mode` is `"hybrid"` or
`"bm25"` (so the agent knows when the embedder was unreachable). Names + one-liners
only — never source.

**`get_component`** — full source, **one variant, by exact name**.
Params: `name` (exact, from search results), `framework`, `tailwind_version`,
`mode`. Returns `{name, source, language, notes, tokens_used}` for exactly one
variant. Requesting multiple components means multiple calls — by design.

**`get_component_preview`** — the cheap look-before-you-fetch step: a compact
structural skeleton (tag outline, slot names, signal/prop surface) and the preview
image if the corpus record carries one. An agent uses this to confirm fit before
spending the `get_component` budget.

**`list_component_categories`** — the `Section.Group.Category` tree with counts.
Small, cacheable, the map an agent orients with on first contact.

### 3.2 Patterns

*Ships in ROADMAP Phase 3.*

The scenarios in [ui-architecture-patterns.md](ui-architecture-patterns.md) served
programmatically — pattern docs are authored once as markdown with CUE-validated
front-matter metadata (D7) and indexed at build time.

**`list_patterns`** — `{items: [{slug, name, one_liner, scenario_keywords}],
has_more}`.

**`get_pattern`** — full pattern by slug: the scenario, the kit wiring (which
`appkit` packages, which subjects/KV buckets, which components), and the canonical
code sketch. One pattern per call.

**`suggest_pattern`** — the design-guidance tool: free-text scenario description
in ("user kicks off a 10-minute export and needs progress + cancel"), ranked
patterns out with rationale:

```go
type SuggestPatternIn struct {
    Scenario string `json:"scenario" jsonschema:"describe the screen/interaction in 1-3 sentences"`
    Limit    int    `json:"limit,omitempty" jsonschema:"default 3"`
}
// Out: {suggestions: [{slug, name, score, rationale, see_also_components []string}]}
```

Ranking is the same hybrid machinery as `search_components` over the pattern
corpus; the `rationale` is template-assembled from the pattern's own metadata
(deterministic — no model call in v1, see §6).

### 3.3 Tokens and theme

*Ships in ROADMAP Phase 3.*

**`get_design_tokens`** — the CUE-defined design tokens (D7) as DTCG JSON,
filterable by `group` (`color|space|type|radius|...`). **`get_theme_css`** — the
emitted Tailwind `@theme` CSS for a named kit theme. Both read the same CUE source
the build uses, so the Assistant can never disagree with the compiler.

### 3.4 Kit adoption

*Ships in ROADMAP Phase 5.*

**`audit_project`** — scan a repo, return structured findings:

```go
type AuditProjectIn struct {
    Path string `json:"path" jsonschema:"absolute path to the repo root"`
}
type Finding struct {
    ID       string `json:"id"`       // stable, e.g. "kit-version-behind", "app-cue-invalid", "ci-missing-templ-check"
    Severity string `json:"severity"` // info | warn | drift
    Area     string `json:"area"`     // version | manifest | conventions | ci | adoption
    Message  string `json:"message"`
    Fix      string `json:"fix"`      // one actionable sentence, may reference a tool or pattern
}
```

Checks, in order: `go.mod` kit version vs. the Assistant's own version; `app.cue`
present and valid (validated in-process via embedded `cuecontext`, D7); convention
drift (committed `*_templ.go` with CI regeneration check, loopback listener shape
per the Loopback Invariant, no Node artifacts per D2); missing CI checks. In a
foreign project it emits `area:"adoption"` findings instead: what the kit would
replace, the smallest first step (usually: serve one screen from a Go loopback
server), and which patterns apply.

**`scaffold_app`** — emit a walking skeleton per the platform contract. Returns a
**file map**; the calling agent writes the files (no hidden side effects):

```go
type ScaffoldAppIn struct {
    Name      string   `json:"name"`
    Platforms []string `json:"platforms,omitempty" jsonschema:"any of web,desktop; default both"`
    Theme     string   `json:"theme,omitempty" jsonschema:"kit theme name; default 'kit'"`
}
// Out: {files: [{path, content}], post_steps: []string}
```

Emitted files: `main.go` at the app root (loopback listener + per-launch token
per D5/D14), `app.cue` (identity, capabilities, nav, theme — the app manifest),
`ui/` (including `ui/shell.templ`) with a routes skeleton and one SSE screen in
the Tao-of-Datastar shape (D3), `store/migrations/`, `assets/`, `build/`,
Taskfile, `go.mod`, `.gitattributes`, the `CLAUDE.md` pack (D15), and the
project's own `.mcp.json` — the same tree `appkit new` emits (full layout in
`repo-layout.md`). No `islands/` is
scaffolded by default — that directory is created when the first L2 island is
admitted. `post_steps` carries the commands (`go mod tidy`, `task dev`).

`scaffold_app` is a thin MCP wrapper over the same internal scaffold package
that powers `appkit new` — one generator, two front ends.

**`upgrade_guide`** — `(from_version, to_version)` → ordered steps assembled from
the per-minor upgrade notes (D13), each `{step, description, commands, files_hint}`.
The Assistant ships the notes for every release up to its own version, so
`go install`-ing the new Assistant is step zero of every upgrade.

### 3.5 Decisions

*Ships in v1 (ROADMAP Phase 2).*

**`list_adrs`** — `{items: [{number, title, status, one_liner}]}`. Cheap.

**`get_adr`** — full ADR markdown by number. One per call.

**`ask_decisions`** — free-text question → FTS5 (BM25, hybrid when the embedder is
up) over the full decision corpus: every ADR, `../DECISIONS.md`, `../../VISION.md`,
and the pattern docs. Returns top passages with `{source_uri, excerpt, one_liner}` —
excerpts, not whole documents; the agent follows up with `get_adr` or a resource
read. This is the tool that answers *why*.

## 4. Return-shape discipline

The rules, binding for every current and future tool:

1. **Lists return names + one-liners, never source.** Default page 10, max 20,
   always `has_more` and `next_offset`, always `total` when cheap to compute.
2. **Full source by exact name, one item per call.** No tool accepts a list of
   names for content retrieval. An agent that wants five components makes five
   calls and *feels* the cost — that friction is the budget signal working.
3. **Structured over prose.** Findings, suggestions, and steps are typed JSON
   objects with stable `id`/`slug` fields, so agents can act on them without
   re-parsing prose.
4. **Degradation is declared.** Any response produced under fallback (BM25-only,
   missing preview, partial audit) says so in a `search_mode`/`warnings` field.
5. **Tool descriptions ≤ 2 sentences; input schemas flat.** Total schema cost for
   the whole server stays under ~4k tokens, measured in CI (a test renders the
   `tools/list` response and counts tokens against the budget).

## 5. Resources and prompts

Resources mirror the document corpus for clients that use them well; every
resource has a tool equivalent because tools are what today's clients reliably
call (ADR-0011, resources-only alternative). Stable URIs:

| URI | Content |
|---|---|
| `appkit://vision` | VISION.md |
| `appkit://decisions` | docs/DECISIONS.md (the register) |
| `appkit://adr/{number}` | resource template → each ADR |
| `appkit://pattern/{slug}` | resource template → each pattern doc |
| `appkit://upgrade-notes/{version}` | per-minor upgrade notes |

All embedded at build time (§6) — URIs are stable across releases; content is
whatever the Assistant's release shipped.

Two prompts, registered via `AddPrompt`:

- **`design_review_against_kit`** — walks the agent through reviewing the current
  diff/screen against kit patterns and tokens: calls `suggest_pattern` with the
  screen's scenario, diffs the implementation against the pattern's wiring, checks
  token usage via `get_design_tokens`, cites ADRs for any deviation.
- **`adopt_kit_in_existing_app`** — the foreign-project on-ramp: run
  `audit_project`, pick the smallest adoption step from its findings, scaffold the
  loopback skeleton alongside the existing app, route one screen through it.

## 6. Data backing

The Assistant owns no pipeline — it is a query head on D10's artifacts:

- **The component index**: the single SQLite file from
  [ADR-0010](../adr/0010-component-data-pipeline.md) (structural columns +
  contentless FTS5 + sqlite-vec `vec0` float32[768] + zstd-compressed code blobs),
  shipped inside the binary via `go:embed` and opened with ncruces/go-sqlite3
  v0.34.x + the sqlite-vec ncruces bindings — cgo-free, cross-compilable
  (https://github.com/ncruces/go-sqlite3/releases, v0.34.4 2026-06-04;
  https://github.com/asg017/sqlite-vec-go-bindings). At ~7.9k vectors, brute-force
  KNN is sub-millisecond; no ANN anywhere.
- **The docs corpus**: ADRs, DECISIONS.md, VISION.md, pattern docs, and upgrade
  notes are a **second embedded index**, built by the Assistant's release step
  and distinct from the D10 artifact — the component pipeline's `#Manifest`
  stays TailwindPlus-only (FTS5 +, when the release pipeline has the embedder,
  vectors). `ask_decisions` and `suggest_pattern` query it; resources serve the
  raw markdown from the same embedded store. Its digest is printed by `-version`
  alongside the component index's.
- **Query embeddings**: local Ollama `/api/embed` with the **same pinned
  qwen3-embedding-0.6B digest** the index was built with (D10; Ollama v0.30.x as
  of 2026-06). If the daemon is unreachable or slow (`APPKIT_MCP_EMBED_TIMEOUT`),
  search degrades gracefully to BM25 + structural filters and says so via
  `search_mode:"bm25"`. The Assistant never requires a network or an API key to
  be useful.

```sql
-- the hybrid query shape (one round trip, RRF k=60)
WITH fts AS (SELECT rowid, rank FROM comp_fts WHERE comp_fts MATCH :q LIMIT 40),
     vec AS (SELECT rowid, distance FROM comp_vec
             WHERE embedding MATCH :qvec AND k = 40 AND framework = :fw)
SELECT c.name, c.summary, ...
FROM components c
LEFT JOIN ... -- RRF: sum(1.0/(60+rank)) over both lists, structural WHERE on tw_version/mode
ORDER BY rrf_score DESC LIMIT :limit OFFSET :offset;
```

## 7. Token budgets

Per-response targets, enforced by tests that run each tool against the shipped
index and count tokens. Exceeding `max` is a release blocker, not a warning.

| Tool | target (p50) | max | Notes |
|---|---|---|---|
| `search_components` | 500 | 1,200 | 10 items, names + one-liners |
| `get_component` | 2,500 | 8,000 | one variant's source; large variants noted in preview first |
| `get_component_preview` | 300 | 800 | skeleton + prop surface |
| `list_component_categories` | 400 | 800 | full tree with counts |
| `list_patterns` | 300 | 600 | |
| `get_pattern` | 1,500 | 4,000 | scenario + wiring + sketch |
| `suggest_pattern` | 600 | 1,200 | 3 ranked suggestions |
| `get_design_tokens` | 800 | 2,000 | per group; full set near max |
| `get_theme_css` | 1,000 | 3,000 | one theme |
| `audit_project` | 1,200 | 3,000 | findings only, no file contents |
| `scaffold_app` | 3,000 | 8,000 | the one intentionally heavy tool |
| `upgrade_guide` | 1,500 | 4,000 | per minor-version hop |
| `list_adrs` | 400 | 800 | |
| `get_adr` | 1,500 | 3,000 | ADRs are 120–250 lines by current practice |
| `ask_decisions` | 800 | 2,000 | excerpts + URIs, never full docs |
| *tool schemas (all shipped tools, `tools/list`)* | — | 4,000 | CI-measured |

Worst realistic session (orient → search → preview → get → pattern → tokens) costs
≈ 6–8k tokens — under 4% of a 200k window. That is the number the 143k horror
story says we must defend.

## 8. Distribution and versioning

- **Install**: `go install <module>/cmd/appkit-mcp@latest` from the private module
  (GOPRIVATE, D13). One binary on `$PATH`, shared by every project on the machine.
- **Per-repo wiring**: each project commits
  `.mcp.json` at the root — `{"mcpServers":{"appkit":{"type":"stdio","command":"appkit-mcp"}}}`
  — which Claude Code reads at project scope (and Codex/Cursor read the same file;
  https://code.claude.com/docs/en/mcp). `scaffold_app` emits it; `audit_project`
  flags its absence.
- **Versioning**: the Assistant is versioned with the kit — same module, same tag
  (D13). `appkit-mcp -version` prints the module version and the embedded index
  manifest digest. `audit_project` compares its own version against the target
  repo's `go.mod`, so version skew is self-reporting.
- **`-http` mode**: one long-lived instance (e.g. launchd on the dev machine,
  `appkit-mcp -http 127.0.0.1:4910`) for shells and remote agents that prefer
  Streamable HTTP. Loopback bind only; the SDK's origin/DNS-rebinding protections
  stay on (D14; go-sdk v1.4.1 hardening, 2026-03-13). Anything beyond loopback is
  a future ADR (OAuth — go-sdk v1.5.0+, parts still experimental as of 2026-06).

## 9. Sampling and elicitation — deliberately later

The go-sdk supports server-initiated sampling (`ServerSession.CreateMessage`, incl.
sampling-with-tools since v1.4.0) and elicitation. We use **neither in v1**. Every
v1 tool is a deterministic function of (query, index, repo state) — testable in CI
without a model, reproducible across runs, and immune to spec churn in the
experimental surface (the 2026-07-28 revision may reshape exactly these features).

The earmarked exception: a future `design_review` tool that samples the *client's*
model to critique a screen against kit patterns — the server supplies the corpus,
the client supplies the judgment, no API key enters the Assistant. That lands only
after v1 has proven the deterministic core, and after the 2026-07-28 spec settles.

## 10. Relationship to the existing tailwindplus server

The current `tailwindplus` MCP server (running off the raw 336MB JSON) stays in
service until the Assistant reaches **component-tool parity**: search, get, and
preview returning equivalent-or-better results from the D10 index. Then the
Assistant supersedes it and the old server is retired from `.mcp.json` files (D10
already plans the downloader's reimplementation in-kit).

Continuity, where it carries: the parameter vocabulary is identical —
`framework` (html/react/vue), `tailwind_version` (3/4), `mode`
(light/dark/system/none) — and exact-name get is preserved. Tool *names* shorten
(`search_component_names` → `search_components`,
`get_component_by_full_name` → `get_component`,
`get_component_preview_by_full_name` → `get_component_preview`): the old names
were namespaced by necessity; inside `appkit-mcp` the family prefix does that job.
`list_component_names` is succeeded by `list_component_categories` +
`search_components` — orient with the category tree, then search; there is no
full-name dump. `list_tailwindplus_information` has no successor — its job is
split between `list_component_categories` and the server's own instructions
string.

## 11. Acceptance

### v1 acceptance (Phase 2)

The Assistant ships v1 when, in a clean repo with only `.mcp.json` committed, an
agent can — without human kit-explanation:

1. Find the right component and fetch exactly one variant's source
   (`search_components` → `get_component_preview` → `get_component`).
2. Answer "why no Node?" with ADR-0002's actual reasoning (`ask_decisions` →
   `get_adr`).
3. Do all of the above offline, with Ollama stopped, in BM25 mode — degraded, not
   broken.

### Full acceptance (post-Phase 5)

With every family shipped, the same agent can additionally:

1. Answer "how do I build a long-running-job screen the kit way?" with the correct
   pattern, components, and wiring (`suggest_pattern` → `get_pattern` →
   `search_components`).
2. Scaffold a new app that builds and runs (`scaffold_app`, then the agent writes
   and runs).
3. Audit a foreign repo and produce a credible first adoption step
   (`audit_project`).

Together the two lists are the ambient-knowledge test made executable; they run
as an evaluation script in CI against every release — the v1 subset gates Phase 2,
the full list gates post-Phase 5.

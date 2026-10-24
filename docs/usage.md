# Fathom Usage

How to ingest repos, query symbols, run the MCP server, and embed fathom in Go code.

See [architecture.md](architecture.md) for design context and [mcp-tools.md](mcp-tools.md)
for the full MCP tool reference.
Building from source, the gate and the repo's dev-loop conventions are under [Contributing](#contributing).

---

## Quickstart

```sh
# 1. Ingest the current directory as a service named "my-service"
fathom ingest --service my-service --path .

# 2. List stored symbols
fathom inspect --service my-service

# 3. Query symbols relevant to the "auth" concern
fathom query --service my-service --concern auth --keywords token,validate

# 4. Start a stdio MCP server for your editor/agent
fathom serve-mcp --store workspace/db/fathom.sqlite
```

---

### In a cloud session (Claude Code on the web, Codex cloud)

A cloud session runs in a fresh Linux VM, so the quickstart is done for you: one committed
`scripts/cloud-setup.sh` provisions the VM (go.mod's toolchain and the pinned tools in
`scripts/cloud-tools.env`, `bin/fathom`, an index pre-built inside a 270-second budget), and the
SessionStart wrapper starts a per-session daemon on `127.0.0.1:7575`. fathom's MCP tools reach the
agent through `fathom serve-mcp --url <daemon rpc url>`, a stdio front that opens no store (see
[serve-mcp](usage/serve-mcp.md)). Owner steps, the hosts to allow and the limits:
[adoption/cloud-sessions.md](adoption/cloud-sessions.md). A local session is unchanged.

### Which languages and formats extract, and how well

`fathom ingest` reads what [docs/languages.md](languages.md) lists. Its generated excellence board
(tiers T1..T6, the level each item meets and its next gap) is the one place that states levels,
so quote levels from there; it is regenerated, never hand-edited.

---

## Ignoring files: `.fathomignore`

`LocalConnector` always skips `vendor`, `.git`, `testdata`, `generated`,
`node_modules`, `.idea`, `.vscode` and `.claude`. It also skips the default
cache directories `.cache`, `.tools`, `target`, `dist`, `build`, `__pycache__`,
`.venv`, `.gradle`, `.next`, `.turbo` and `coverage` (RT-145), and names every one
it pruned (`excluded: N default-ignored cache dir(s) (…)` in the ingest summary,
`default_ignored_dirs` in `--json`). A corpus that commits source under one of those
names re-includes it in its own `.fathom/config.toml` — a bare name re-includes every
directory with that name, a path with a slash just that one:

```toml
[connector]
include = ["coverage", "dist"]
```

`include` never re-includes the always-skipped names. The connector never reads a
file over 2 MiB (RT-69): it is sized by a stat, counted under `oversize`, and never
opened. For repo-specific exclusions, place a
`.fathomignore` file at the service root (the `--path` directory). It uses
gitignore-style patterns — one per line, `#` comments, `!` negation, `**`
globs, trailing-slash dir-only rules, and leading-slash root anchoring
(`/fathom` matches only the root-level entry, never `cmd/fathom/`):

```gitignore
# Mirrored third-party source — not our code.
ref/
# Scratch and local stores.
tmp/
workspace/
# Generated code: index the sources, not the codegen.
*_templ.go
```

Matched directories are pruned from the walk; matched files are dropped.
Sensitive data directories (user content, case data, secrets) should always be
excluded here — the index is queryable by any connected MCP client and is not
behind an egress/DLP gate.

Build artifacts (compiled binaries, generated bundles) belong in `bin/` or
`workspace/` — both already ignored by convention above — never at the repo
root, where they get walked like any other file. As defense in depth,
connectors also detect binary content directly (a NUL byte in the first 8000
bytes, ADR-021 §1): such files are excluded from extraction, from cloc's
line counts (reported instead in a files/bytes-only `binary` bucket), and
from the regex-based scan tools. `.fathomignore` should still name known
build-artifact paths explicitly with a root-anchored entry (e.g. `/fathom`
for a stray `go build -o fathom` at repo root): an ignored path is never
discovered or fetched into memory at all, whereas the content sniff runs
only after the bytes have already been read.

---

## CLI subcommands

All subcommands share the convention:

- `--store` defaults to `<repo>/workspace/db/fathom.sqlite` — the enclosing git
  repository's `workspace/`, from any subdirectory — or `~/.fathom/db/fathom.sqlite`
  when the cwd is not inside a repository; never the bare cwd (invariant I31, O11).
  `[daemon].store` in `config.toml` and an explicit `--store` still win. The same
  rule resolves `--call-log` (`<repo>/workspace/logs/mcp-calls.jsonl`, else
  `~/.fathom/logs/mcp-calls.jsonl`; `FATHOM_CALL_LOG` and `[daemon].call_log` win)
- `--mode` selects the connector: `local` (default), `faux` (fixture data), or `github`
- Exit 0 on success, 1 on any error. `fathom --help`, `-h`, `--version`, `fathom help <verb>` and a sole `-h`, `--help`, `-help` or `help` after any verb print to stdout and exit 0 without running the verb; `verify` and `doctor` keep 0/1/2 for every other argument
- A verb's help lists its usage, summary and subcommands, then the verb's flags with their defaults; `fathom <verb> <subcommand> --help` (or `fathom help <verb> <subcommand>`) lists that subcommand's flags the same way

### `ingest`

Moved to [usage/ingest.md](usage/ingest.md).


### `ingest-diff`

Moved to [usage/ingest-diff.md](usage/ingest-diff.md).


### `watch`

Moved to [usage/watch.md](usage/watch.md).


### `inspect`

Moved to [usage/inspect.md](usage/inspect.md).


### `query`

Moved to [usage/query.md](usage/query.md).


### `serve-mcp`

Moved to [usage/serve-mcp.md](usage/serve-mcp.md).


### `serve-http`

Moved to [usage/serve-http.md](usage/serve-http.md).


### `platform-ingest`

Moved to [usage/platform-ingest.md](usage/platform-ingest.md).


### `platform-callers`

Moved to [usage/platform-callers.md](usage/platform-callers.md).


### `gen-agents-md`

Moved to [usage/gen-agents-md.md](usage/gen-agents-md.md).


### `gen-status-cue`

Moved to [usage/gen-status-cue.md](usage/gen-status-cue.md).


### `gen-platform-manifest`

Moved to [usage/gen-platform-manifest.md](usage/gen-platform-manifest.md).


### `migrate-store` (removed, SS-8)

Moved to [usage/migrate-store.md](usage/migrate-store.md).


### `version`

Moved to [usage/version.md](usage/version.md).


### `session-hook`

Moved to [usage/session-hook.md](usage/session-hook.md).


### `edit-hook`

Moved to [usage/edit-hook.md](usage/edit-hook.md) Also the commit pin line on a `git commit` (`[hooks] pin_check = "off"|"record"|"feedback"`, default `record`).


### `stop-hook`

Moved to [usage/stop-hook.md](usage/stop-hook.md).


### Hook fan-out: what a hook does per tool call

Moved to [usage/hook-fan-out.md](usage/hook-fan-out.md).


### Hook deadlines, the shared breaker and the kill switch

Moved to [usage/hook-deadlines.md](usage/hook-deadlines.md).


### `statusline`

`fathom statusline [--subagent] [--json]` prints one line saying whether the index matches the checkout you work in.
It reads Claude Code's status-line JSON on stdin (the project directory when present, else the working directory;
every other field is ignored, and malformed input still prints a state), asks the daemon once
(`GET /statusz?format=json&root=<dir>`) under the hook guard's deadline and breaker, and prints exactly one of:

| Line | Means |
|---|---|
| `fathom ● @<sha>` | indexed at HEAD, no tracked file changed |
| `fathom ● @<sha> +uncommitted` | indexed at HEAD, with tracked changes the index does not hold |
| `fathom ● @<sha> dirty unknown` | indexed at HEAD; whether the tree is dirty could not be read |
| `fathom ◐ <n> behind @<sha>` | HEAD is `n` commits past the indexed commit (`behind unknown` when that could not be counted) |
| `fathom ○ daemon down` | the request was sent and failed |
| `fathom · not checked (<verdict>)` | the hook guard sent no request (`breaker_open`, `busy`, `disabled`, `paused`): not a down daemon |
| `fathom ? not indexed` | no indexed service's root holds the directory |

An unknown never reads as `0 behind` or as clean. `--json` prints the same state as one object (`state`, `line`,
`behind` and `dirty`, null when unknown, and the reason). `--subagent` marks the input as the subagent status-line
payload, read the same way.

A plugin cannot ship `statusLine`, so add fathom as a segment of your own. Point `statusLine` in
`~/.claude/settings.json` at a script, and hand the same stdin to your line and to fathom's:

```json
{ "statusLine": { "type": "command", "command": "~/.claude/statusline.sh" } }
```

```sh
#!/bin/sh
# ~/.claude/statusline.sh: your status line, then fathom's segment.
input=$(cat)
mine=$(printf '%s' "$input" | ~/.claude/my-statusline.sh)   # your existing line, if any
seg=$(command -v fathom >/dev/null 2>&1 && printf '%s' "$input" | fathom statusline 2>/dev/null)
printf '%s  %s\n' "$mine" "${seg:-fathom: not on PATH}"
```

`/statusz?format=json&root=<dir>` is the daemon's answer the verb reads: the service whose recorded root holds
`dir` (the deepest when roots nest), with `head_commit`, `behind` and `dirty` read through the read-only git
wrapper under a 200 ms deadline, each null with a reason when it could not be read. Without `root` the JSON is
unchanged.


### `verify`

Moved to [usage/verify.md](usage/verify.md).


### `claim`

Moved to [usage/claim.md](usage/claim.md).


### `remap`

Moved to [usage/remap.md](usage/remap.md).


### `layers`

Moved to [usage/layers.md](usage/layers.md).


### `obligations`

Moved to [usage/obligations.md](usage/obligations.md).


### `compile`

Moved to [usage/compile.md](usage/compile.md).


### `config`

Moved to [usage/config.md](usage/config.md).


### `providers`

Moved to [usage/providers.md](usage/providers.md).


### `smoke`

Moved to [usage/smoke.md](usage/smoke.md).


### `instrument`

Moved to [usage/instrument.md](usage/instrument.md).


### `report`

Moved to [usage/report.md](usage/report.md).


### `init`

Moved to [usage/init.md](usage/init.md).


### `doctor`

Moved to [usage/doctor.md](usage/doctor.md).


### `plans` — computed SDD progress and archival

Moved to [usage/plans.md](usage/plans.md).


### `plans lines`, `gc` strays and binaries, `report used` — stewardship reports

See [usage/stewardship.md](usage/stewardship.md).


### `thread` — continuity across sessions (vision 21)

Moved to [usage/thread.md](usage/thread.md).


### `map` — exploration maps (ADR-056, experimental)

Moved to [usage/map.md](usage/map.md).


### `config audit` — the config custodian (vision 22, "instructed means true")

Moved to [usage/config-audit.md](usage/config-audit.md) Includes `agents_md_reaches_claude` with `import_only: true` (an import-only `CLAUDE.md`, exactly `@AGENTS.md`).


### `config propose` — the diff-offering verb (CC-9)

Moved to [usage/config-propose.md](usage/config-propose.md).


### `daemon` — install, inspect, stop and remove the serve-http agent

Moved to [usage/daemon.md](usage/daemon.md).


### `resources` — what the daemon takes from this machine

Moved to [usage/resources.md](usage/resources.md).


### `gc` — disk retention by class

Moved to [usage/gc.md](usage/gc.md).


### `intent` — the intent corpus, audited (ADR-026, **experimental**)

Moved to [usage/intent.md](usage/intent.md).


### `scope` / `purge` — reversible suppression and verified removal

Moved to [usage/scope-purge.md](usage/scope-purge.md).


### `retro` — did the firings actually help? (mechanical, free)

Moved to [usage/retro.md](usage/retro.md).


### `assess` — a priced second opinion on what shipped (explicit, off by default)

Moved to [usage/assess.md](usage/assess.md).


### `rate` — the session outcome, in one number

Moved to [usage/rate.md](usage/rate.md).


### `brief` / `flywheel` — the value story and the pipes it rides on

Moved to [usage/brief-flywheel.md](usage/brief-flywheel.md).


### `fleet` — report, reconcile, receipts, undo (law 15)

Moved to [usage/fleet.md](usage/fleet.md).


### `decide` — the decision port's log

Moved to [usage/decide.md](usage/decide.md).


### `prime` — the Codex pull tier

Moved to [usage/prime.md](usage/prime.md).


### `observe` — file a friction observation

Moved to [usage/observe.md](usage/observe.md).


### `review` — the change-review verb

Moved to [usage/review.md](usage/review.md).


### `review-key` — one reuse-eligibility token (FL-17/FL-18/FL-19)

Moved to [usage/review-key.md](usage/review-key.md).


### `conserve` — the refactor-conservation report (ADR-044, CV-19)

Moved to [usage/conserve.md](usage/conserve.md).


### `refactor` — refactors as programs (CV-33/CV-34, ADR-044 §4)

Moved to [usage/refactor.md](usage/refactor.md).


### `trace` — the evidence pack (CX-14..CX-18)

Moved to [usage/trace.md](usage/trace.md).


### `provenance note`

`fathom provenance note [<commit>]` (default `HEAD`) replaces the local note under `refs/notes/fathom/provenance` with declared harness, model, host label, session and lane as JSON. Use `--harness`, `--model`, `--session`, `--lane` or their defaults `FATHOM_HARNESS`, `FATHOM_MODEL`, `FATHOM_SESSION_ID`, `FATHOM_LANE`; session also falls back to the harness's session environment, whose harness label supplies the harness default. `--host-label` overrides the user-only `gate.host_label` config key; absent fields are `unknown`, and the machine name is never inferred. The confirmation prints the resolved commit SHA. Notes are declarations, not authenticated identities. The target commit and its message stay unchanged; notes are never pushed by this command.

### `receipt` — a stranger's re-derivation (VB-20)

Moved to [usage/receipt.md](usage/receipt.md).


### `rules` — layer rules as tests, and as history (VB-14, VB-16, SR-47w)

Moved to [usage/rules.md](usage/rules.md).


### `readout` — readout rules, offline (ADR-071)

Moved to [usage/readout.md](usage/readout.md).


### `tests coverage` / `tests select` — coverage, mutation, and test selection

Moved to [usage/tests-coverage-select.md](usage/tests-coverage-select.md).

### `culprit` — candidates for a red commit range

`fathom culprit <good>..<bad> [--failing TestA,TestB] [--service S] [--store DB]`
ranks commits by failing-test static reach, including edits to the failing test's
own file. Commits with missing file evidence rank above indexed commits without
reach, followed by descending blast radius and recency. Incomplete radii print as
`>=N`. Merges are labeled and use combined diffs to count their own resolution
changes rather than inherited changes.

Analysis uses changed files and the current read-only index, not historical symbol
bodies. The store defaults to `[daemon].store`, then the workspace database. Output
names the store and indexed commit, and reports whether that commit matches
`<bad>`, is behind it by a commit count, or is not its ancestor. Unavailable commit
metadata is unknown. A service with zero symbols gets a no-index-evidence notice.
Unresolved `--failing` names appear once and provide no negative evidence. Without
`--failing`, reach is unknown. Missing reach evidence never clears a commit.

The output prints bisect commands to try the top candidate, says when it is `<bad>`
itself, and includes testing its parent (`C^`) to confirm the boundary. Run and
classify the tests yourself: fathom runs no build, test, checkout or bisect. This is
a candidate list, not a causal verdict.


### `tests flaky` — same-tree failures followed by passes

`fathom tests flaky [--json] [--since 24h]` reads local `workspace/logs/gate.*`
files, saved hook output named `<16 hex>.log` in `workspace/tmp/test-run`, and
imported `<64 hex>.json` receipts with their matching `.log` files in
`[gate] receipts_dir` (default: the fathom home's `receipts` directory).
Unrelated log and JSON files are ignored.

A named Go verbose or JSON test failure followed by a pass counts only when both
runs share the same identity: the same clean git tree, gate name, Go version,
GOOS/GOARCH, GOWORK, CGO_ENABLED and GOFLAGS. The tree pins `Taskfile.yml` and
`lefthook.yml`, so no Task command echo is compared. A package `ok <pkg>` also
counts as a pass for tests in that package that an earlier matching run recorded as
failing, but only from a gate that passed. A `(cached)` result is no evidence. Saved
hook output counts only if it starts with the gate START line. An aggregate passing
gate without test or package results proves nothing.

Results rank by recorded failing gate duration, then package and test. Each failing
gate's full duration is attributed to each affected test, so costs can overlap.
JSON reports `lost_ns` in nanoseconds. Unknown or unreadable tree, command or
platform identity is counted separately; legacy hook diff digests never establish
a git tree. `--since` filters by recorded run end and excludes undated files.
Missing directories mean no evidence. This command reads files without a daemon,
network access or running tests.


### `tests ingest` / `tests status` — test-kit evidence (vision 33 / ADR-043 stage 0)

Moved to [usage/tests-ingest-status.md](usage/tests-ingest-status.md).


### `certify` — the compiled certifications layer (vision 33 / ADR-043 stages 1-3)

Moved to [usage/certify.md](usage/certify.md).


### `export` — interchange with other code-intel tooling (VB-32)

Moved to [usage/export.md](usage/export.md).


### `gen-plan-cue`

Moved to [usage/gen-plan-cue.md](usage/gen-plan-cue.md).


### `research` — the research flywheel

Moved to [usage/research.md](usage/research.md).


### `module` — expert-module lifecycle (**experimental**, law 9: read-only before writable)

Moved to [usage/module.md](usage/module.md).


### `metrics` — observability surfaces

Moved to [usage/metrics.md](usage/metrics.md).


### `findings` — findings as evidence (**experimental**; entry ≥15–20 resolved findings)

Moved to [usage/findings.md](usage/findings.md).


### `memory` — removed

Moved to [usage/memory.md](usage/memory.md).


### `plugin` — one tree, two manifests

Moved to [usage/plugin.md](usage/plugin.md).


## The observatory console and the brief

Moved to [usage/observatory.md](usage/observatory.md).

## Capabilities — what is on here, and why anything is off

`fathom capabilities` answers the most common adoption question in one
place. A capability is reported **on only when it is observably
operating** — a config file saying `guards: on` with no rules compiled
reports off, with the reason (law 3 applied to configuration).

```sh
fathom capabilities [--repo .] [--service S] [--url U]   # table: state, evidence, and the reason for every gap
fathom capabilities --cost              # what each one costs when on
fathom capabilities --json              # the model doctor and the observatory render
fathom capabilities explain guards      # what it does, what it costs, what evidence it produces
fathom capabilities set narrate on      # writes .fathom/capabilities.cue
fathom capabilities set narrate on --scope user
```

`set` splices only the `capabilities` block and preserves everything else in
the file byte-for-byte (a `budgets` block, comments); it refuses when the
file cannot be round-tripped. Non-identifier ids such as `verify-gate` are
written as quoted labels. The same file's optional `budgets` block (BA-9,
**experimental**) is documented in [configuration.md](configuration.md).

Two scopes, resolved built-in → user → project, with the winning scope
shown beside every value (law 13):

| File | May do |
|---|---|
| `~/.fathom/capabilities.cue` | set a **default** for repositories that have not spoken |
| `<repo>/.fathom/capabilities.cue` | **govern** — the final word for that repository |

A user-scope file that tries to set a project-only capability (a gate, a
guard) is **refused with the key named**, never silently ignored. An
unknown id is refused with the nearest known id suggested. `on?: false`
— which in CUE declares the field may exist rather than setting it, so
the line does nothing — is refused with the fix in the message.

`fathom doctor` carries one `capabilities` row. It fails **only** when a
capability is switched on and something is failing to deliver it (rules
compiled but no hook fires; an index behind HEAD). A capability whose
inputs are simply absent here — no module bound, no layer pack installed
— is not a failure and does not red-mark the row.

## Go client (`pkg/client`)

`pkg/client` exposes a thin `Client` that wires together the connector, fusion pipeline,
query engine, and store. Import it without touching any `internal/` package.

```go
import "github.com/geoffmilleraz/fathom/pkg/client"

// Open (or create) a SQLite store; wire a local-mode connector.
c, err := client.NewConstantClient(client.ModeLocal, "workspace/db/fathom.sqlite")
if err != nil { /* handle */ }
defer c.Close()

// Ingest a service.
report, err := c.Ingest(ctx, client.ServiceRef{ID: "my-service", Path: "."})
fmt.Println(report.Symbols) // number of symbols extracted

// Concern-scoped symbol query.
result, err := c.QuerySymbols(ctx, client.Query{
    ServiceID: "my-service",
    ConcernID: "authentication",
    Keywords:  []string{"token", "validate"},
    TopN:      20,
})
for _, n := range result.Nodes {
    fmt.Printf("%s %s %s\n", n.Kind, n.Name, n.FilePath)
}
if result.Staleness.ReingestRecommended {
    // re-ingest before trusting this data
}

// Find callers of a symbol.
callers, err := c.FindCallers(ctx, "my-service", "ValidateToken")

// Find a symbol definition.
nodes, err := c.FindSymbol(ctx, "my-service", "ValidateToken")

// List all stored symbols (no filtering).
all, err := c.ListAllSymbols(ctx, "my-service")

// List all ingested service IDs.
services, err := c.Services(ctx)

// Batch ingest.
reports, err := c.IngestBatch(ctx, []client.ServiceRef{
    {ID: "auth", Path: "/src/auth"},
    {ID: "billing", Path: "/src/billing"},
})
```

Available modes:

| Constant | Connector | Use |
|---|---|---|
| `client.ModeLocal` | `LocalConnector` | Reads from local directory path |
| `client.ModeFaux` | `FauxConnector` | Reads from `testdata/fauxtown/` fixture |
| `client.ModeGitHub` | `GitHubConnector` | Reads via `gh` CLI (requires `gh`) |

Pass `""` for `storePath` to use an in-memory SQLite database (useful in tests).

---

## Connecting an MCP client

### Claude Code (stdio)

Add fathom to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "fathom": {
      "command": "fathom",
      "args": ["serve-mcp", "--store", "/path/to/workspace/db/fathom.sqlite"]
    }
  }
}
```

Claude Code will launch the `fathom serve-mcp` process and communicate over stdio.

### Claude Code against a running daemon (stdio front)

To share the `serve-http` daemon's one writer instead of opening the store, point the stdio entry
at the daemon: `"args": ["serve-mcp", "--url", "http://127.0.0.1:7575/rpc/agent"]`. It forwards
each JSON-RPC line and answers every request with an id even when the daemon is down
([serve-mcp](usage/serve-mcp.md)). A cloud session uses this front.

### Any MCP-compatible client (stdio)

Pipe JSON-RPC 2.0 messages to the `serve-mcp` process. `initialize` echoes
your `protocolVersion` when it is one of `2025-11-25`, `2025-06-18`,
`2025-03-26`, `2024-11-05`, and answers `2025-11-25` otherwise; every
agent-surface tool in `tools/list` carries `annotations` and an
`outputSchema`, and `tools/call` returns `structuredContent` beside the text
block (`docs/mcp-tools.md` § Protocol contract):

```sh
# Initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"my-client","version":"1.0"}}}' \
  | fathom serve-mcp --store workspace/db/fathom.sqlite

# tools/list
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | fathom serve-mcp --store workspace/db/fathom.sqlite
```

### HTTP client

```sh
# Single JSON-RPC call via POST /rpc
curl -s -X POST http://localhost:7575/rpc \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "query_symbols",
      "arguments": {
        "service": "my-service",
        "concern": "authentication",
        "keywords": ["token"],
        "top": 10
      }
    }
  }'
```

### Selecting a tool surface

There is no profile-switching tool (the old `set_profile` was removed; ADR-014
amendment, 2026-07-16): a stdio session always sees the full registry, and over
HTTP a surface is selected by path — `POST /rpc/agent` is the eight-tool
default surface (`internal/mcp/surface.go`), `POST /rpc/extended` (also served at
`/rpc/companion` for compatibility) the 21-tool opt-in superset, and bare
`POST /rpc` the full registry. Off-surface calls execute and are logged `off_surface:true`
rather than rejected; the hard gate exists only behind the server's
`strictSurfaces` option.

See [mcp-tools.md](mcp-tools.md) for all tools, prompts, and resources, and
[use-cases/coding-agents.md](use-cases/coding-agents.md) for the canonical
coding-agent wiring (hooks, gates, CLAUDE.md/AGENTS.md).

---

## Contributing

Building fathom from source, the gate, and the repo's dev-loop conventions. Using fathom needs none of this.

### Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.27.3 (go.mod's `toolchain`; `go 1.26.2` is the source-build minimum, but a 1.26 build with `GOTOOLCHAIN=local` cannot type-load Go 1.27 sources) | Pure Go (ADR-066): Linux builds with `CGO_ENABLED=0` and no C compiler; macOS keeps cgo (Xcode Command Line Tools) only for the daemon's two memory readers; `go test -race` on Linux needs cgo and gcc |
| [CUE](https://cuelang.org) 0.16+ | Schema validation |
| [go-task](https://taskfile.dev) | `Taskfile.yml` is the only command runner |
| [lefthook](https://github.com/evilmartians/lefthook) | Git hooks (pre-commit gofmt, pre-push validate) |
| `third_party/agentic-kit`, `third_party/app-kit` | Committed mirrors of two private modules; `go.mod`'s `replace` lines point at them, so no sibling checkout is needed ([distribution.md](distribution.md)) |

Build and gate commands:

```sh
task            # list available tasks
task doctor     # toolchain + workspace health check
task validate   # full gate: fmt-check + vet + cue-vet + vendor-check + store-purego + scripts-check + go test -race (cgo on)
lefthook install
```

`store-purego` (invariant I2, ADR-025) runs `CGO_ENABLED=0 go test -race ./internal/store/`
on every validate/push — the store stays pure Go; `cmd/fathom/cgofree_test.go` (CS-9) keeps every Linux dependency cgo-free.

`task validate` runs under a machine-wide gate lease (RT-129, design §12.3): concurrent
gates on one machine — other repositories, other agents, a pre-push — queue on an flock
at `~/.fathom/locks/gate.lock` (`FATHOM_GATE_LOCK` overrides) and say who holds it
(`gate-lease: waiting for gate lease held by pid N (repo X)`); the gate runs at background
priority (`nice -n 10`; `FATHOM_GATE_NICE=off` opts out, `FATHOM_GATE_NICE=background` uses macOS's
`taskpolicy -b`, which measured ~3x slower: `cmd/fathom` 441 s against 168 s niced)
and `go test -p 4`. `task validate:unleased` is the same gate without the lease.

`task test` runs `cmd/fathom` (about 1,900 serial tests, ~357 s alone: the gate's critical path)
as four shards of one race build (`scripts/test-shards.sh`; `FATHOM_TEST_SHARDS` sets the count).
The partition is greedy longest-first over the measured seconds in
`scripts/test-shards.weights.tsv` (`task test:weights` re-measures them; a test missing from the
file weighs the median), so it is the same for the same test list. Before any test runs, each
shard's `-test.run` pattern must select exactly its planned tests and the shards together exactly
the binary's `-test.list`: a test dropped or run twice fails the gate. Each shard prints a
go-test `ok`/`FAIL` line with its wall time, and a failing shard's output is shown in full. The
shards run concurrently with `go test -race -p 4` over every other package, so at most 8 test
processes run at once (4 package binaries and 4 shards; fewer once the short packages finish),
still one gate per machine under the lease and niced. Measured at load ~11-13 under the lease, the
concurrent `task test` took 356 s against 463 s and 476 s with the shards after the packages;
`internal/fusion` (~270-290 s) was the other long pole. With the slowest single tests moved to
sibling packages, `task test` took 338 s at load 10-15: the shards 231 s with their build,
`internal/fusion/perfgate` 218 s (the longest package) and `internal/fusion` 137 s.
Unlike `go test`, the shards keep
no test-result cache, so `cmd/fathom` runs again even when nothing changed.

`internal/scenariobench` runs once per gate, inside `task test`; it is not also a separate
`validate:unleased` dependency (`task scenariobench` still runs it alone).

#### Pushing (`task push`)

```sh
task push      # third_party/agentic-kit/fathom-pack/scripts/push-check.sh, then: git push origin HEAD:main
```

INC-20 (2026-09-19): two pushes reached `origin/main` ungated after a push clone's
`.git/hooks/pre-push` was wiped by an unrelated process — `task validate` never ran
before either push, and both push logs were two lines long, the tell of a gate that
never fired. `task push` refuses to push at all unless `third_party/agentic-kit/fathom-pack/scripts/push-check.sh` finds
lefthook's own pre-push hook in place first (invariant I40, ADR-041 decision 10):

- exit 0, `push-check: ok (lefthook pre-push present)` — the hook is present, push
  proceeds.
- exit 3, one line naming why — the hook is missing, empty, or present but not
  lefthook's (a plain `git init`'s own `*.sample` hooks, for instance). Run
  `lefthook install` and re-run `task push`.
- exit 2 — an operational error (not a git repository, `git` not on `PATH`), not a
  verdict about the hook.

The check resolves the hook path through `git rev-parse --git-path hooks/pre-push`,
so a repo with `core.hooksPath` pointed elsewhere, or a linked worktree, is checked
where its own hook actually lives — never a hardcoded `.git/hooks/pre-push`. `git
push` directly (skipping `task push`) still works and is never itself gated — the
gate is `task push`'s own contract, not a server-side or repo-level enforcement; use
`task push` so the gate actually runs. `scripts/push-check_test.sh` (wired into
`task scripts-check`) proves the check against disposable temp repos: a missing hook,
a non-lefthook hook, a real lefthook hook, and a `core.hooksPath` variant.

#### Build-cache placement and disk guard

Fathom's supported build path (`task ...` and repository scripts) preserves the
machine's normal Go and temporary-directory settings by default. To give Fathom a
dedicated cache—on an external volume, for example—create the root first and export:

```sh
mkdir -p /path/on/the/intended/volume/fathom
export FATHOM_CACHE_ROOT=/path/on/the/intended/volume/fathom
export FATHOM_CACHE_MAX_MIB=8192 # optional; 0 disables automatic pruning
task validate
```

The root must be absolute, pre-existing, and writable. Requiring it to exist is
intentional: if a removable volume is absent, Fathom refuses instead of recreating the
mount path on the system disk. The build flow derives `GOCACHE`, `GOMODCACHE`,
`GOTMPDIR`, `TMPDIR`, and `UV_CACHE_DIR` beneath that root. Before a supported Go
invocation, a build cache larger than `FATHOM_CACHE_MAX_MIB` is trimmed by entry age
(entries unused for 7 days go first, then 3, then 1; an entry used within the last day is
never removed, because a running build may hold it, so the cache can stay above the limit
and the guard says so); module downloads are retained. To empty the build cache
outright, run `go clean -cache` yourself with no build running.

Raw `go` commands bypass repository policy. Use `task test`, `task build`, or the
corresponding repository script when cache placement matters.

#### Scratch and test runs: `FATHOM_HOME`, `--ledger` and the per-file variables

Fathom keeps per-user state in one directory, `~/.fathom`: the verify, intent, session
and assess ledgers, the hooks beat log, rule-outcome and preflight logs, session stamps
and ratings, the daemon records, lifecycle ledgers and logs, the fleet snapshots and the
wazero compilation cache. A scratch clone, a worker or a test run that runs `fathom
verify` or `fathom intent audit` there appends rows keyed by its scratch path to the
operator's real ledgers (observation O121: 13,514 of 19,639 verify rows on 2026-09-28).
**A scratch clone or a test run sets one of these overrides:**

| Override | Names | Wins over |
|---|---|---|
| `FATHOM_HOME=<dir>` | the whole per-user directory: every path above resolves under `<dir>` instead of `~/.fathom` (a relative value is made absolute against the working directory) | `~/.fathom` |
| `--ledger <path>` on `fathom verify` and `fathom intent audit` | the one ledger file that run appends to and reads: the verify ledger for `verify`, the intent ledger for `intent audit` | the per-file variable and `FATHOM_HOME` |
| `FATHOM_VERIFY_LEDGER`, `FATHOM_INTENT_LEDGER`, `FATHOM_SESSION_LEDGER`, `FATHOM_HOOKS_LOG` (and the other per-file variables: `FATHOM_ASSESS_LEDGER`, `FATHOM_PREFLIGHT_LEDGER`, `FATHOM_PURGES`, `FATHOM_RULE_OUTCOME_LOG`, `FATHOM_SESSION_RATINGS`, `FATHOM_SESSION_STAMPS`, `FATHOM_THREADS`, `FATHOM_CONSOLE_VIEWS`, `FATHOM_ARM_STATE`, `FATHOM_PRICE_TABLE`, `FATHOM_IGNORE_D`) | that one file or directory | `FATHOM_HOME` |

Precedence for one file: `--ledger`, then its per-file variable, then `FATHOM_HOME`, then
`~/.fathom`. Nothing is created under `FATHOM_HOME` until a run writes there, and a run
under it creates nothing under the real `~/.fathom`
(`TestScratchRunUnderFathomHomeLeavesRealLedgersUntouched`).

```sh
# a scratch verify that leaves the operator's ledgers alone
FATHOM_HOME="$(mktemp -d)" fathom verify --strict --store scratch.sqlite --service fathom --plan docs/plans/x.md
# or name just the ledger
fathom verify --ledger /tmp/scratch-verify.jsonl --store scratch.sqlite --service fathom --plan docs/plans/x.md
```

Two things deliberately do not move. The gate lease's lock (`scripts/gate-lease.sh`,
`~/.fathom/locks/gate.lock`, `FATHOM_GATE_LOCK`) stays one machine-wide file, or
concurrent gates stop queueing. And the operator-state guard on `serve-http --store` /
`--call-log` reads the operator's own daemon records under the real home, never
`FATHOM_HOME`'s, so a scratch environment cannot blind it. Under `FATHOM_HOME` a verb finds
no daemon record for the managed daemon and falls back to `--url` / `FATHOM_HTTP` / the
default address; a scratch run names its store with `--store`. Go tests are isolated by
`internal/testisolate`, which unsets `FATHOM_HOME` beside its private `HOME`; `task test`
also runs under a `FATHOM_HOME` inside `workspace/tmp/`.

---

### workspace/ convention

All build artifacts, logs, temp files, and local databases go under `workspace/` at the
repo root. This directory is gitignored. Never write generated files elsewhere in the tree.

| Path | Contents |
|---|---|
| `workspace/db/` | store files (default: `workspace/db/fathom.sqlite`) |
| `workspace/logs/` | Run and MCP call logs (`mcp-calls.jsonl`) |
| `workspace/artifacts/` | Generated artifacts (`AGENTS.md`, manifests, exports) |
| `workspace/tmp/` | Ephemeral scratch files |

`store.Open` auto-creates the parent directory when it does not exist.

This is fathom's own dev-loop convention — its `.fathom/config.toml` opts into it
(`[daemon] state_dir = "workspace"`). It is NOT the default for a repo fathom adopts
(O63 item 5, `internal/statedir`): a hook's own runtime state (session/observed-discovery
logs, retro/narrate artifacts, the receipts log, MCP call logs) defaults to OUTSIDE the
subject repository entirely, under `$XDG_STATE_HOME/fathom/<repo-id>/` (default
`~/.local/state/fathom/<repo-id>/`), so a fresh adopter's `git status` after `fathom init`
and normal use never shows fathom's own hook output. A repo opts back into the old
in-tree layout the same way this one does — `[daemon] state_dir = "workspace"` in its own
`.fathom/config.toml` — or `FATHOM_STATE_DIR` overrides the directory directly for one
invocation (hooks' fixed command lines can't pass a flag). The code index (`-store` /
`workspace/db/`) is unaffected either way — it is the artifact fathom exists to produce,
not runtime state, and stays under the repo's `workspace/` regardless.

# Adopting fathom in Claude Code — zero to working

The definitive, self-contained path to wiring fathom into Claude Code: build,
run a daemon, register the MCP surface, wire the hook stack, pick a provider
lane, and verify it actually works. You should not need to read any other
doc to follow this one; where a topic has a deeper reference elsewhere, it's
linked, not required.

See also: [docs/use-cases/coding-agents.md](../use-cases/coding-agents.md)
(the shorter canonical-wiring summary this guide expands on),
[docs/configuration.md](../configuration.md) (the full provider-config
reference), [docs/layers.md](../layers.md) (the semantic-layer catalog),
[docs/mcp-tools.md](../mcp-tools.md) (every MCP tool, not just the 16-tool
surface covered here).

---

## 1. Build and install

This section is about building the `fathom` **binary itself**, once, from
fathom's own source repo — a separate thing from the repo you're about to
wire fathom *into* (§2 onward). If you already have a `fathom` binary
(released build), skip to §2.

| Requirement | Notes |
|---|---|
| Go 1.26+ | CGO must be enabled — required by the tree-sitter extractor bindings (ADR-013). The store itself is pure Go (`modernc.org/sqlite`, ADR-020): SS-8 deleted the DuckDB backend outright once the fleet migrated (ADR-025), so the store no longer needs CGO at all |
| `../agentic-kit` | Sibling module via `go.work` + `replace`; absent, `go.mod`'s replace falls back to the in-tree `third_party/agentic-kit` mirror automatically (see `docs/distribution.md`) — adopters who only want the binary don't need to understand this, just build once |

Building fathom's own tests/schema-vet additionally wants
[CUE](https://cuelang.org) 0.16+, [go-task](https://taskfile.dev), and
[lefthook](https://github.com/evilmartians/lefthook) — but none of those are
runtime dependencies of the built binary: CUE schema validation (config
files, semantic-layer packs) is linked in via the Go CUE library, not
shelled out to a `cue` CLI, so a plain `go build` is all any adopter needs.

Build the binary to `bin/fathom` — this exact path is what every wiring
mechanism below expects (the post-commit binary-rebuild hook, `fathom init`'s
baked-in hook commands, and the daemon-install pattern in §5):

```sh
# macOS
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom

# Linux (MACOSX_DEPLOYMENT_TARGET is ignored/unneeded)
CGO_ENABLED=1 go build -o bin/fathom ./cmd/fathom
```

> `task build` (from fathom's own `Taskfile.yml`) runs the identical command
> and is equivalent to the one above. The fast inner-dev-loop build used by
> the calibration harness (`workspace/tmp/fathom`, not registrable with
> Claude Code or a daemon) is `task build-dev` — a differently-named task,
> so the two are never confused.

Sanity-check the build:

```sh
bin/fathom version   # prints the commit it was built from
```

---

## 2. Index a repo

The fast path is `fathom init` (§3) — it ingests for you. To index by hand:

```sh
bin/fathom ingest --service my-service --path . --store workspace/db/fleet.sqlite
```

Write a `.fathomignore` (gitignore syntax) at the repo root **before** first
ingest — it excludes noise (`workspace/`, `bin/`, build output) and anything
sensitive (the index is queryable by any connected MCP client and sits
outside any application egress/DLP boundary). `fathom init` writes a starter
one for you if none exists.

Re-ingest is hash-gated (~0.2s when nothing changed) — safe to run often.

---

## 3. The fast path: `fathom init`

From the repo root:

```sh
bin/fathom init
```

Idempotent and additive — re-running it changes nothing that's already
correct. It:

1. writes `.fathomignore` (create-if-absent),
2. ingests the repo into the daemon (if reachable) and confirms it indexed,
3. wires the three hooks (§7) into `.claude/settings.json`,
4. stamps a marker-delimited fathom operating-contract stanza into
   **both** `CLAUDE.md` and `AGENTS.md` (one canonical source, so Codex —
   which reads `AGENTS.md` automatically — gets the same doctrine with zero
   drift; §3.1 below),
5. write-if-absent scaffolds `~/.fathom/config.toml` (§9),
6. installs the shipped agent skills (§11) into `.claude/skills/` and/or
   `~/.codex/prompts/` — **only** when `--skills[=claude|codex|all]` is
   passed; omitted entirely by default, so a caller who never asked for
   skills sees no new files, and
7. prints the steps it deliberately does **not** do for you: MCP
   registration (§6, requires an interactive `claude mcp add`) and the
   lefthook pre-push snippet (§7.4, `lefthook.yml` is hand-curated —
   `init` never edits it).

Exit codes: `0` fully wired and the daemon confirms the service is indexed;
`2` wiring is complete but the daemon is unreachable, or reachable but the
service still shows zero symbols (operational — mirrors `verify`'s
fail-open doctrine, §7.4 below; never a failure of the files `init` wrote —
re-run once the daemon is up); `1` is never returned for an operational
reason **except** a `--skills` marker conflict (§11.1).

The rest of this guide is the manual/reference version of what `init`
automates, plus everything it doesn't (the daemon install, provider setup,
the tool surface, troubleshooting).

### 3.1 The `CLAUDE.md` / `AGENTS.md` stanza — markers, re-run semantics, corruption refusal

One canonical stanza (`cmd/fathom/init.go`'s `fathomStanza`) is written
verbatim into both files, wrapped in a marker pair:

```
<!-- fathom:begin -->
## fathom code intelligence
…
<!-- fathom:end -->
```

`applyStanza` (the function behind step 4 above) has exactly three
outcomes, chosen by what it finds at the target path:

- **File absent** → created with the stanza as its entire content.
- **File exists, never wired** (neither marker present) → the stanza is
  **appended** to the end of the file — every byte already there is
  preserved; your own `CLAUDE.md`/`AGENTS.md` content is never touched,
  let alone overwritten.
- **File exists, well-formed marker pair present** → only the content
  *between* the markers is **replaced** with the current stanza; everything
  outside the markers (your own sections, above or below) is untouched.
  This is what makes a re-run idempotent: if the stanza text hasn't
  changed, the file's bytes don't change either, and `init` prints `ok
  CLAUDE.md (already present)` instead of `updated`.
- **Corrupted marker pair** (only one of `fathom:begin`/`fathom:end` found,
  or `end` appears at or before `begin` — e.g. a hand-edit that deleted or
  reordered a marker line) → `init` **refuses loudly**, naming the file,
  rather than guess: silently appending a fresh block on top of a broken
  pair would compound the corruption (two `begin`s, an orphaned `end`, or
  worse) instead of surfacing it for a human to fix. Fix or remove the
  stray marker by hand, then re-run `init`.

Re-running `init` after editing your own content elsewhere in either file
is always safe — only the bytes between the markers are ever replaced.

---

## 4. `serve-mcp` (one-shot) vs `serve-http` (daemon)

| | `serve-mcp` | `serve-http` |
|---|---|---|
| Transport | stdio, spawned per session | HTTP, one process, `--addr` (default `:7575`) |
| Clients | Exactly one | Any number, concurrently |
| Store lock | Holds the store's single-writer lock for its lifetime | Same, but there's only ever one holder |
| Best for | A single Claude Code session on your machine | More than one concurrent session, CI, multiple agents/editors sharing one index |
| Refresh | `ingest` MCP tool (CLI `ingest` fails — lock held) | Same, plus `--auto-refresh` (default on): re-ingests on git-HEAD drift automatically |

**Run more than one Claude Code session at a time? You want `serve-http`.**
Two stdio `serve-mcp` processes both try to hold the same store's
single-writer lock; only the first wins and every other session (and every
`claude mcp list` health check) reports `✘ Failed to connect`. One shared
`serve-http` daemon makes every session an HTTP client of a single
lock-holder instead — this is also what the fleet-index dogfood pattern in
this repo's own `CLAUDE.md` runs.

One-shot stdio registration (single-session use only):

```sh
claude mcp add --scope user fathom -- \
  /abs/path/bin/fathom serve-mcp --store /abs/path/workspace/db/fleet.sqlite
```

For everything else in this guide, assume the daemon (§5–§6).

---

## 5. Running the daemon durably

A quick foreground test:

```sh
bin/fathom serve-http --store workspace/db/fleet.sqlite --addr 127.0.0.1:7575
# serve-http: listening on 127.0.0.1:7575
curl -s http://127.0.0.1:7575/healthz   # ok
```

For anything beyond a one-off test, install the binary and run it under a
supervisor so it survives logout/reboot **and** so a `bin/fathom` rebuild in
your repo doesn't silently swap out the binary a running daemon is
executing. The pattern this repo uses on macOS — copy to a stable path
outside the repo, point launchd at that copy:

```sh
mkdir -p ~/.local/libexec/fathom
cp bin/fathom ~/.local/libexec/fathom/fathom
```

```xml
<!-- ~/Library/LaunchAgents/com.example.fathom-http.plist — adapt the label and paths -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.example.fathom-http</string>
  <key>ProgramArguments</key><array>
    <string>/Users/you/.local/libexec/fathom/fathom</string>
    <string>serve-http</string>
    <string>--store</string><string>/abs/path/to/repo/workspace/db/fleet.sqlite</string>
    <string>--addr</string><string>127.0.0.1:7575</string>
    <string>--call-log</string><string>/abs/path/to/repo/workspace/logs/mcp-calls.jsonl</string>
  </array>
  <key>EnvironmentVariables</key><dict>
    <!-- go on PATH so daemon-side auto-refresh can run typed-edge enrichment for Go
         services (it shells out to `go list`); omit it and refreshes silently fall
         back to syntax-only edges. -->
    <key>PATH</key><string>/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>WorkingDirectory</key><string>/abs/path/to/repo</string>
  <key>StandardOutPath</key><string>/tmp/fathom-http.log</string>
  <key>StandardErrorPath</key><string>/tmp/fathom-http.err</string>
</dict></plist>
```

```sh
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.example.fathom-http.plist
curl -s http://127.0.0.1:7575/healthz   # ok
```

No launchd (Linux/CI)? `nohup fathom serve-http --store … --addr :7575 &` works
for a session but dies with the shell — use a systemd unit or process
supervisor for anything persistent.

**Deploying a rebuilt binary** — because the daemon runs the installed copy,
not `bin/fathom` directly, a committed Go fix needs a deliberate deploy step,
run from a real terminal (not a sandboxed agent tool — the restart can wedge
one):

```sh
cp bin/fathom ~/.local/libexec/fathom/fathom
launchctl kickstart -k gui/$(id -u)/com.example.fathom-http
```

On macOS, a newly copied binary has a new code hash — if the restart hangs,
look for a "fathom would like to access files on a removable volume" dialog
and click **Allow**. This is also why §13's binary-vs-data staleness check
matters: a committed fix does not reach the daemon until this step runs.

---

## 6. Register the MCP server with Claude Code

Register against the curated agent surface — the recommended, dogfooded
path:

```sh
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent
claude mcp get fathom            # expect: Type: http · Status: ✔ Connected
```

`--scope user` makes the server (and its tools) available to every Claude
Code session and every subagent spawned in any repo — not just the one you
registered it from.

**Why `/rpc/agent` and not bare `/rpc`.** The daemon serves several
path-scoped surfaces of the same store (`docs/mcp-tools.md`): `/rpc` is the
full ~80-tool registry (unauthenticated, includes `ingest` and
`compile_layer`); `/rpc/agent` (aliased `/rpc/companion`) is the curated
16-tool coding-agent hot loop (§10). Claude Code defers full-registry tool
schemas until a tool is actually used, so `/rpc` costs little either way —
`/rpc/agent` is still the better default because it's what's dogfooded, and
because it makes off-surface tool use *measurable* (calls outside the
16-tool set still execute — they're logged `off_surface:true`, never
silently rejected — see ADR-014's amendment in `docs/mcp-tools.md`). If you
want the full registry too, register it under a second name:

```sh
claude mcp add --transport http --scope user fathom-full http://127.0.0.1:7575/rpc
```

**The restart caveat.** MCP tool registration and schema discovery happen at
session start. A session that was already running when you ran `claude mcp
add` — or across a daemon restart, or after `fathom init` just wired
things — will not see the tools. **Restart the Claude Code session once**
after registering. This is the single most common "the tools aren't there"
report; see §13 before assuming anything is actually broken.

---

## 7. The hook stack

`fathom init` (§3) wires all three of these for you, idempotently. This
section is both the manual recipe and the reference for what gets written,
pulled verbatim from `cmd/fathom/init.go`'s hook specs and this repo's own
`lefthook.yml`.

### 7.1 SessionStart — `session-hook`

Fires at session start (`startup|resume|clear|compact`). Pings the daemon,
maps the project directory to a service, and injects one line: "this repo is
service X, use fathom before Grep/Read" — or an honest "daemon down" /
"repo not indexed" signal. Always exits 0; a 2-second HTTP timeout means a
dead daemon can't stall session start.

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          { "type": "command", "command": "/abs/path/bin/fathom session-hook", "timeout": 10 }
        ]
      }
    ]
  }
}
```

### 7.2 PostToolUse — `edit-hook` (obligations injection)

Fires after `Edit`, `Write`, or `MultiEdit`. Resolves the touched file,
calls the `obligations` MCP tool for it, and injects governing rules
(≤150 tokens total, ≤6 rules, each with a `src:` citation, evidence tier,
and a `code drifted since compile` flag when applicable) — plus, budget
permitting, spec-reference lines for any bracketed spec IDs the file cites.
**Silent on a clean file and silent on any error** (daemon down, file not
indexed) — it never blocks a write.

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          { "type": "command", "command": "/abs/path/bin/fathom edit-hook", "timeout": 5 }
        ]
      }
    ]
  }
}
```

This hook has nothing to inject until the `obligations` semantic layer has
been compiled for your service at least once:

```sh
bin/fathom compile --service my-service                  # dry-run: sections + cost estimate
bin/fathom compile --service my-service --dry-run=false   # cents-scale spend; digest-gated thereafter
```

Until then the hook exits clean (silent), by design — see `docs/layers.md`
for what `obligations` (and the other catalog layers — `adr-rules`,
`spec-bindings`, `findings`) actually compile from.

### 7.3 Stop — `stop-hook` (done-means-proven, session granularity)

Fires at session end. Diffs which plan tasks in `docs/plans/*.md` were newly
claimed `[x]` **during this session** (by comparing against the file's state
at git HEAD) and re-verifies just those against the index. If any fail,
it emits advisory `additionalContext` by default, or blocks session end with
`--block`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          { "type": "command", "command": "/abs/path/bin/fathom stop-hook", "timeout": 10 }
        ]
      }
    ]
  }
}
```

Add `--block` to the command to make this an actual gate rather than an
advisory note (opt in per repo — off by default because it's the strongest
in-session enforcement in the stack).

All three JSON snippets above merge additively into an existing
`.claude/settings.json` — `fathom init` detects an already-wired entry by
command substring (`fathom session-hook` etc.) and never duplicates it,
regardless of matcher grouping or the exact binary path baked in.

### 7.4 Pre-push — the `fathom verify` gate (lefthook)

This is a git-native gate, not a Claude Code hook — it runs on `git push`
regardless of which agent (or human) made the commits. Copied from this
repo's own `lefthook.yml` (generalize `<service>` and the glob to your repo):

```yaml
pre-push:
  commands:
    validate:
      run: task validate    # your repo's own build/test gate, if any — independent of fathom
    fathom-verify:
      run: |
        [ -x bin/fathom ] || exit 0
        shopt -s nullglob
        plans=()
        for f in docs/plans/*.md; do
          [ "$(basename "$f")" = "README.md" ] && continue   # READMEs describe, plans claim
          plans+=("$f")
        done
        [ ${#plans[@]} -eq 0 ] && exit 0
        rc=0
        for f in "${plans[@]}"; do
          bin/fathom verify --service <service> --plan "$f"
          code=$?
          if [ $code -eq 1 ]; then rc=1; fi
          if [ $code -eq 2 ]; then
            echo "fathom verify: operational error — allowing push (fail-open)"
          fi
        done
        exit $rc
```

Exit-code contract (`fathom verify`, per
`docs/design/coding-agent-excellence.md` §5.3): **0** every claimed task
verifies; **1** a claimed `[x]` task failed verification — **block** the
push; **2** operational (daemon unreachable, service not indexed, empty
index) — **warn and allow** the push. This asymmetry is deliberate: a gate
that blocks pushes because a daemon happened to be down gets disabled within
a week. In CI, add `--strict` (claimed-but-Unknown also blocks) and treat
exit 2 as a **failure** too — CI builds its own store
(`fathom ingest` into a temp store, then `verify --store`), so "daemon down"
is never a valid CI state.

Plan files themselves are Markdown checklists — grammar in
`docs/plans/README.md`:

```markdown
- [ ] Parser handles v2 keys {id: G2-1, symbol: ParseMarkdownPlan, package: internal/verify}
- [-] cmd oracle {id: D-1, disposition: deferred, reason: "no consumer yet", trigger: "skip share >30% after 4 weeks"}
```

`[x]` is a claim the gate checks (symbol exists, a test statically reaches
it, no unresolved contradictions) — never proof the test *passes*
(`test_reaches` is static reachability; `runtime_pass` stays Unknown until
CI evidence is ingested as its own tier).

---

## 8. Per-project `.fathom/`

Two config layers, both optional, both TOML, both validated against a CUE
schema on load (a bad key fails loudly, naming the file — never a silent
no-op):

- **User**: `~/.fathom/config.toml` — your personal provider profiles,
  shared across every repo. `fathom init` scaffolds this write-if-absent.
- **Project**: `<repo>/.fathom/config.toml` — profiles specific to one repo
  (a CI service account, a team-shared endpoint). Commit it only if every
  profile uses `api_key_env`/`api_key_cmd` indirection — never a literal
  secret (see §9).

Precedence (per-field, not per-file):

```
flags > env > project .fathom/config.toml > user ~/.fathom/config.toml > built-in defaults
```

`fathom config --explain` shows exactly which layer supplied every
resolved value.

**`.fathom/layers/<name>/`** is where a repo adds its own semantic-layer
module packs (`layer.cue` + `prompt.md` + `eval/`), layered on top of the
embedded catalog (`obligations`, `findings`, `adr-rules`, `spec-bindings` —
full reference in `docs/layers.md`). Presence in this directory **is** the
enable state — there's no separate on/off flag:

```sh
fathom layers list                 # embedded + project packs, source-labeled, override marked *
fathom layers show adr-rules       # full resolved definition + prompt provenance
fathom layers install ./my-pack    # validates against #Layer, copies into .fathom/layers/<name>/
```

---

## 9. Provider lanes, including claude-cli (no API key)

`fathom compile --dry-run=false` and `serve-http`'s `compile_layer` need an
LLM. Six lanes, selected by `provider` in a config profile or
`FATHOM_LLM_PROVIDER`:

| Lane | Auth mode | Env vars | Marginal-cost story |
|---|---|---|---|
| `anthropic` (default) | API key | `ANTHROPIC_API_KEY` | Metered, per-token Anthropic pricing |
| **`claude-cli`** | Subscription CLI (OAuth) | `FATHOM_CLAUDE_BIN` (optional binary override) | **$0 marginal** — draws from a Claude Pro/Max plan's own limits, not a metered key. `--safe-mode` suppresses ambient context to near-zero |
| `openai` | API key | `OPENAI_API_KEY` (optional for local/compatible hosts), `FATHOM_OPENAI_BASE_URL` | Metered OpenAI (or compatible-host) pricing; $0 against a self-hosted local model |
| `azure-openai` | API key (or Entra bearer) | `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_DEPLOYMENT`, `AZURE_OPENAI_API_VERSION` (optional) | Metered against the Azure resource's negotiated pricing |
| `bedrock` | AWS credential chain | none fathom-specific; `FATHOM_COMPILE_MODEL` required | Metered against AWS Bedrock per-token pricing, billed through AWS |
| `codex-cli` | Subscription CLI (ChatGPT plan OAuth) | `FATHOM_CODEX_BIN` (optional) | See `docs/adoption/codex.md` §4 — real cost, not $0 |

For a repo with no API key budget, the subscription lane is the fast path —
if you're logged into the Claude Code CLI, no further setup is needed:

```toml
# ~/.fathom/config.toml
default_profile = "personal"

[profiles.personal]
provider = "claude-cli"
```

```sh
fathom config --explain       # confirm what resolved and from where
fathom providers              # confirm auth health, $0 spend (a Ping, not a completion)
fathom smoke --profile personal   # one real tiny completion — confirms it actually works, small $
fathom compile --service my-service --dry-run=false
```

`fathom init` scaffolds `~/.fathom/config.toml` with every lane shown
commented out — uncomment one and set `default_profile`. Full precedence
rules, the secrets doctrine (there is no `api_key` field — a literal key in
the file is a hard validation error), GovCloud/Azure Government profiles,
and the daemon's own `[daemon]` config are in
[docs/configuration.md](../configuration.md).

---

## 10. The 16-tool agent surface — when to use what

The `/rpc/agent` (aka `/rpc/companion`) allowlist (`docs/mcp-tools.md`):

| Tool | Reach for it when… |
|---|---|
| `capabilities` | You need to confirm which tools/surface are actually live this session — the discovery escape hatch when a small surface is registered |
| `find_symbol` | You know (or can guess) a symbol's name and want its definition(s), fast — tiered matching with `did_you_mean` on a miss |
| `query_symbols` | You have a *concern*, not a name — "auth", "checkout flow" — and want ranked, relevant symbols instead of a name-exact lookup |
| `outline` | You've picked a file and want everything defined in it, in line order, before reading it |
| `repo_map` | You're orienting in a repo you don't know yet — the birds-eye structural view |
| `symbol_context` | You're about to touch a symbol and want definition + callers + callees + related tests — plus an inline source snippet of the definition (capped 40 lines) — in one call, instead of four |
| `find_callers` | You need every call site of a symbol — the structured alternative to grepping the repo |
| `references` | Fleet-wide occurrence search for an identifier or string across every indexed service — replaces cross-repo grep |
| `read_source` | You have a file and line range (from a citation, `find_symbol`, or a DocChunk's `line`/`end_line`) and want the exact current text, line-numbered — the surface's bounded `sed -n` |
| `pack_context` | You're an architect/lead assembling a pre-packed, paste-ready brief for a worker or subagent — signatures, file paths, contract types in one shot |
| `entrypoints` | You need to know where execution starts — `main`, HTTP handlers, CLI roots — e.g. before a blast-radius or dead-code question |
| `obligations` | Before editing a file, to see governing rules explicitly (this is also what the PostToolUse hook calls automatically after the edit — calling it before is a preview) |
| `verify_plan` | You want an evidence-backed verdict on a plan's claimed tasks without waiting for the pre-push gate — same engine `fathom verify` uses |
| `staleness_report` | Before trusting query results after heavy uncommitted edits, or when a query result looks stale/wrong |
| `list_services` | You need to know what's actually indexed (service IDs, symbol counts, languages) before assuming a repo is in the fleet |
| `ingest` | Force a re-index right now — after a commit, or when `staleness_report` says you should, or to onboard a repo that isn't indexed yet |

The full ~80-tool registry (`/rpc`) adds quality/impact analysis
(`complexity`, `dead_code`, `blast_radius`, …), security scans, and
platform/cross-repo tools — see `docs/mcp-tools.md` for the complete
reference; they still work when called off the agent surface (execute, just
flagged `off_surface` in the call log), so nothing is actually locked out.

---

## 11. Agent skills: `fathom-plan` / `fathom-verify` / `fathom-onboard` / `fathom-feedback`

`fathom init --skills[=claude|codex|all]` installs four shipped, versioned
skill sources (`dist/agents/skills/`, embedded in the binary — no fathom
source checkout needed at install time):

```sh
bin/fathom init --skills=claude
```

- **`fathom-plan`** — scaffold a gate-ready `docs/plans/<slug>.md` in plan
  grammar v2 (stable IDs, `{symbol, package}` contracts grounded via the
  fathom index, typed dispositions for skipped work) — the plan-authoring
  path; see
  [docs/adoption/spec-driven-development.md](spec-driven-development.md).
- **`fathom-verify`** — run and interpret the `fathom verify` gate: exit
  codes, `--strict`/`--require-all`, the fail-open convention, and computed
  progress (`fathom plans status`/`next`).
- **`fathom-onboard`** — wire a repo end-to-end into fathom (build, daemon,
  ingest, MCP registration, the hook stack, `.fathom/`) — the condensed,
  actionable version of this guide and `docs/adoption/codex.md`.
- **`fathom-feedback`** — file friction with a fathom tool the moment it's
  hit, via `fathom observe` — the flywheel's intake (§12 below).

Each ships two rendered shapes from one authored source: `SKILL.md`
(Claude Code's frontmatter + Markdown format, installed to
`.claude/skills/<name>/SKILL.md`) and `codex-prompt.md` (a single Codex
custom-prompt file — see `docs/adoption/codex.md` §2). `--skills=claude`
installs only the former, `--skills=codex` only the latter (to
`~/.codex/prompts/<name>.md`), `--skills=all` (or a bare `--skills`)
installs both.

### 11.1 Re-running / conflicts

Every installed file carries a `<!-- fathom:owned -->` marker line
(`cmd/fathom/skillsinstall.go`'s `skillMarker`) — the same ownership
convention as the `CLAUDE.md`/`AGENTS.md` stanza's markers (§3.1), one
level down: a whole skill file is fathom's or it isn't, so there's no
partial section to replace. A re-run overwrites **only** a file that
already carries the marker (or is byte-identical to what would be
written — a true no-op, printed as `already current`). If a file already
exists at a skill's target path **without** that marker — you already have
your own skill or Codex prompt with that exact name — `fathom init
--skills` refuses loudly (exit `1`, naming the path) instead of clobbering
it; rename your file or pick a different skill name to resolve the
conflict.

---

## 12. The flywheel: `observe` → `instrument --adoption` → `fathom-feedback`

Fathom's own development runs on a deliberate loop — **observe →
root-cause → fix → close** (`docs/dogfood-observations.md`) — and `fathom
init --skills` wires the same loop into every adopting repo. It is
**entirely local**: no hosted service, nothing phones home; both verbs are
plain CLI/file operations against files inside the repo.

**`fathom observe`** — structured friction capture, appended to
`.fathom/observations.md`:

```sh
bin/fathom observe \
  --symptom "find_symbol missed a method on an embedded interface" \
  --tool find_symbol \
  --expected "a hit for (*Thing).Method" \
  --actual "no match; had to grep the file by hand" \
  --repro 'find_symbol({"name":"Method"}) against service <svc>, commit <sha>'
```

Only `--symptom` is required. The entry lands in the dogfood-observations
shape — symptom → evidence → root cause → fix → status → impact —
append-only: first-time file creation is race-safe under concurrent agents
(`O_CREATE|O_EXCL`), and a pre-existing file at that path without fathom's
own marker is refused rather than silently reinterpreted.

**`fathom instrument --adoption`** — the flywheel's gauge, a
deterministic-given-inputs report over the MCP call log
(`workspace/logs/mcp-calls.jsonl` by default) plus `.fathom/observations.md`:
per-surface traffic, top/zero-call tools, the `brief` claims-attach hit
rate, and the open-observation count:

```sh
bin/fathom instrument --adoption
```

**The `fathom-feedback` skill** (§11 above) teaches an agent to call
`fathom observe` the moment it hits friction — mid-task, not "if I
remember" — because agents are the primary users of fathom's tools, so
agent-filed observations are the flywheel's intake. An observation
graduates into a fix (and a `CHANGELOG.md` entry) or is closed with a
recorded reason — the same discipline this repo's own
`docs/dogfood-observations.md` runs, now productized for every adopting
repo. Both verbs are local-only by design: no telemetry leaves the
repo, and nothing here depends on a daemon being reachable.

---

## 13. Troubleshooting

**Tools don't show up in a session.** Registration/schema discovery happens
at session start (§6) — restart the session once. Confirm with
`claude mcp get fathom` (expect `Status: ✔ Connected`) before assuming
anything else is wrong.

**`✘ Failed to connect` in `claude mcp list`.** You're on the per-session
stdio path (§4) with more than one session open — they're racing for the
same single-writer store lock. Switch to the shared `serve-http` daemon
(§5–§6).

**Staleness — know which kind you're looking at.** There are two, and they
have different fixes:

- *Data staleness* — the index is behind the working tree. Handled mostly
  automatically: `serve-http --auto-refresh` (default on) re-ingests a
  service when git HEAD has moved before answering a query. Force it with
  the `ingest` MCP tool, or check it explicitly with `staleness_report`
  (`{"service": "<svc>"}` → `reingest_recommended: true` means HEAD moved
  since last ingest). After *uncommitted* edits, `staleness_report` won't
  catch you — read the file you're about to change, or re-ingest.
- *Binary staleness* — the daemon is running old fathom **logic**, not old
  data. Because the daemon runs an installed copy
  (`~/.local/libexec/fathom/fathom`, §5), a committed Go fix does not reach
  it until you deploy: `cp bin/fathom ~/.local/libexec/fathom/fathom &&
  launchctl kickstart -k gui/$(id -u)/<your-label>`. Diagnose by comparing
  `bin/fathom version` (or the daemon's, via a tool call) to
  `git rev-parse HEAD` — a mismatch means stale, not buggy.

**A hook fires but injects nothing.** For `edit-hook`: check the
`obligations` layer has actually been compiled for the service
(`fathom layers list --store <store> --service <svc>` shows a claims count
per layer) — until then the hook is correctly silent, not broken (§7.2).
For any hook: confirm the binary path baked into `.claude/settings.json`
still resolves (a moved/renamed `bin/fathom` breaks the command silently
from the hook's point of view — hooks never surface their own exec errors,
by design, so check the path by hand).

**A doc or tool you remember isn't there anymore.** `set_profile` (the old
stdio profile-switch tool) was removed entirely — HTTP path-scoped surfaces
(§6) replaced it. If you're reading an older doc that still mentions it,
prefer this guide and `docs/mcp-tools.md`.

**An empty index reads as "cannot verify," never "all claims false."** If
`fathom verify` or `verify_plan` looks like it's failing every task, check
whether the service has any symbols at all first (`list_services`) — a
genuinely empty index is an operational error (exit 2), not a wall of
failed claims.

---

## 14. You know it works when…

```sh
# 1. Build is current
bin/fathom version                              # commit should match:
git rev-parse HEAD

# 2. Daemon is up
curl -s http://127.0.0.1:7575/healthz            # → ok

# 3. Claude Code sees it (after a session restart post-registration)
claude mcp get fathom                            # → Type: http · Status: ✔ Connected
```

Then, inside a Claude Code session:

- [ ] Calling `capabilities` returns `tool_count: 15` (or the count of
      whichever surface you registered) and lists `find_symbol`,
      `query_symbols`, `symbol_context`, `verify_plan`, …
- [ ] `find_symbol` (or `query_symbols` with a `concern`) against a symbol
      you know exists returns a hit with `"tier": "verified"`.
- [ ] After `fathom compile --service <svc> --dry-run=false` has run once,
      editing a file governed by `CLAUDE.md`/`ADR.md` triggers a
      `⚖️ fathom obligations — <file>:` block in the session's context on
      the next edit.
- [ ] `bin/fathom verify --service <svc> --plan docs/plans/<file>.md` exits
      `0` on a clean plan (or fails honestly, naming the unproven claim, if
      you check one off the index can't back yet).
- [ ] `git push` in the repo runs lefthook's pre-push hooks, including the
      `fathom-verify` command from §7.4.
- [ ] `bin/fathom init --skills=claude` (or `=all`) populates
      `.claude/skills/{fathom-plan,fathom-verify,fathom-onboard,fathom-feedback}/SKILL.md`
      (§11), and `bin/fathom instrument --adoption` (§12) runs cleanly,
      reporting `no .fathom/observations.md yet` until the first `fathom
      observe` call, then a real open-observation count after.

If every box above is checked, the full loop — index, serve, inject,
gate — is live.

---

## Maintained

This doc covers: `fathom init` (incl. `--skills[=claude|codex|all]` and the
`CLAUDE.md`/`AGENTS.md` marker-delimited stanza), `ingest`, `serve-mcp`,
`serve-http`, `compile`, `layers` (`list`/`show`/`install`), `verify`
(`--strict`/`--require-all`), `observe`, `instrument --adoption`, `config`,
`providers`, `smoke`, `session-hook`, `edit-hook`, `stop-hook`; the `/rpc`,
`/rpc/agent` (`/rpc/companion`) HTTP surfaces; the 16-tool agent allowlist;
the `SessionStart`/`PostToolUse`/`Stop` Claude Code hook wiring; the
lefthook pre-push verify gate; the shipped `fathom-plan`/`fathom-verify`/
`fathom-onboard`/`fathom-feedback` agent skills (`dist/agents/skills/`).
Update this doc's examples and this line when any of these change.

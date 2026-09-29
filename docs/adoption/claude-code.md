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
[docs/mcp-tools.md](../mcp-tools.md) (every MCP tool, not just the 19-tool
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

**Ingest is honest about what it could not read** (IT-14/IT-15). A file the
connector cannot read (EPERM/EACCES/EIO/ENOENT on the file read — transport
errors stay fatal) is printed in a `read-failed: N file(s)` column with its
path and errno (first 5 inline), the readable files are still indexed, and
the run exits `1` with `error: ingest degraded: …`. A run that could read
**nothing** while the store already holds observations for that service
**refuses to write** — no watermark advance, no purge — `error: ingest
refused: …`, exit `1`; on `operation not permitted` it adds the hint
`external-volume TCC wedge? tccutil reset SystemPolicyRemovableVolumes
<terminal bundle id>`. The daemon's `ingest` MCP tool returns the same cause
and hint as a JSON-RPC error (never `ok:true`). Go callers get
`client.ReadFailuresError` (`Refused`, `Existing`, `Hint()`).

**Narrowing what the index holds.** `fathom scope suppress <glob> --reason R
[--service S] [--until 7d]` hides matching files from every answer,
reversibly (`scope list`, `scope unsuppress <id>`, `scope preview <glob>`
shows what would match first). `fathom purge <glob> --service S --reason R
[--dry-run|--yes]` is the **irreversible** twin for leaked secrets — exit
`2` means the SQL rows are gone but the raw store bytes were not checked;
`fathom purge history` keeps the ledger.

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

Steps 3 and 4 go through one wiring engine (`internal/wiring`, law 15):
one plan → apply → one receipt per repo in `.fathom/receipts.jsonl`
(gitignored); `fathom fleet receipts` lists them and `fathom fleet undo
<receipt> --repo <root>` (or `--last`) reverses one.

Exit codes: `0` fully wired and the daemon confirms the service is indexed;
`2` wiring is complete but the daemon is unreachable, or reachable but the
service still shows zero symbols (operational — mirrors `verify`'s
fail-open doctrine, §7.4 below; never a failure of the files `init` wrote —
re-run once the daemon is up); `1` is never returned for an operational
reason **except** a `--skills` marker conflict (§11.1) or a wiring file
that law 15 **blocked** (§3.1) — a blocked file never aborts the other
steps; it is listed once at the end, one line per file.

The rest of this guide is the manual/reference version of what `init`
automates, plus everything it doesn't (the daemon install, provider setup,
the tool surface, troubleshooting).

### 3.1 The `CLAUDE.md` / `AGENTS.md` stanza — markers, re-run semantics, corruption refusal

One canonical stanza (`cmd/fathom/init.go`'s `fathomStanza`) is written
verbatim into both files, wrapped in a marker pair:

```
<!-- fathom:begin v3 sha256:… -->
## fathom code intelligence
…
<!-- fathom:end -->
```

After the first write the begin line carries the marker version and a
digest of the body (`v3 sha256:…`); a region written before v3 reads
`<!-- fathom:begin -->` with no digest. The wiring engine (`internal/wiring`,
shared with `fathom fleet reconcile --apply`) has these outcomes, chosen by
what it finds at the target path:

- **File absent** → created with the stanza as its entire content.
- **File exists, never wired** (neither marker present) → the stanza is
  **appended** to the end of the file — every byte already there is
  preserved; your own `CLAUDE.md`/`AGENTS.md` content is never touched,
  let alone overwritten.
- **File exists, v1 region** (`<!-- fathom:begin -->`, no digest) → adopted
  as fathom's and upgraded to a v3 marker on the first write. That is a
  migration, not drift.
- **File exists, tracked v3 region, body matches its digest** → only the
  content *between* the markers is **replaced** with the current stanza;
  everything outside the markers (your own sections, above or below) is
  untouched. A re-run with an unchanged stanza changes no bytes and prints
  `ok CLAUDE.md (already present)` instead of `updated`.
- **Tracked region a human edited** (body no longer matches its digest) →
  **BLOCKED** and named: exit `1`, the file stays byte-identical, the other
  files in the same run are still applied, and the final list says
  `drifted (hand-edited) region` per file. Move your edit outside the
  markers (or accept fathom's text), then re-run.
- **Corrupted marker pair** (only one of `fathom:begin`/`fathom:end` found,
  or `end` at or before `begin`) → also BLOCKED, reported as `could not be
  read/parsed` rather than guessed at: silently appending a fresh block on
  top of a broken pair would compound the corruption. Fix or remove the
  stray marker by hand, then re-run `init`.

Re-running `init` after editing your own content elsewhere in either file
is always safe — only the bytes between the markers are ever replaced.
`fathom doctor`'s `stanza_present` check accepts both marker spellings.

---

## 4. `serve-mcp` (one-shot) vs `serve-http` (daemon)

| | `serve-mcp` | `serve-http` |
|---|---|---|
| Transport | stdio, spawned per session | HTTP, one process, `--addr` (default `127.0.0.1:7575`, loopback) |
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
executing. The pattern this repo uses on macOS — install to a stable path
outside the repo, point launchd at that copy, and put every install and
redeploy through one task:

```sh
task deploy      # build → ~/.local/libexec/fathom/fathom → restart launchd → assert the daemon serves HEAD
```

**Never a raw `cp` onto the installed path.** `cp` truncates a running
executable in place, and a freshly copied, unsigned binary has a new code
hash, which voids the daemon's macOS TCC (removable-volume) grant — that
combination took the daemon down fleet-wide once (O25,
`docs/dogfood-observations.md`). `task deploy` (this repo's `Taskfile.yml`)
writes to a temp path and renames, signs with the `fathom-dev` identity
when one exists, `launchctl kickstart`s the agent, then asserts the running
process is the installed path **and** reports `git rev-parse HEAD`. Copy
that task into your own repo and adapt the label/paths; keep all four
properties.

**Install it with one command** (macOS):

```sh
fathom daemon install     # writes ~/Library/LaunchAgents/dev.fathom.serve-http.plist
                          # with YOUR binary path, store, call log, PATH and the Go
                          # toolchain variables, then bootstraps it
fathom daemon status      # what is installed, and whether it is actually serving
```

**Why the plist carries `GOPATH`/`GOCACHE`/`GOMODCACHE` and not just `PATH`:**
launchd does not give an agent a login shell's environment. With `GOPATH`
unset, go resolves its default to the literal, unexpanded `"$HOME/go"` and
refuses every command — so the daemon's typed-edge pass fails on every ingest
of a Go service and the index quietly degrades to syntax-only call edges,
while the same ingest run from your terminal type-checks fine. If you wrote
your plist by hand, or it was written by a fathom older than this note, run
`fathom daemon install --force` and check `fathom doctor`'s `typed_edges` row.

`install` resolves the binary by symlink (so launchd runs the stable installed
copy, never a `bin/` build output a rebuild would truncate), puts the store at
`~/.fathom/db/fathom.sqlite`, the call log at `~/.fathom/logs/mcp-calls.jsonl`,
and writes a PATH that includes your Go toolchain — **without `go` on PATH the
daemon's auto-refresh silently degrades Go services to syntax-only edges**, so
`install` says so out loud when it cannot find one. It refuses to stack a
second agent on the same port and tells you how to keep or replace the one
that is there; `fathom daemon uninstall` removes it and leaves your store and
logs alone.

Everything that mentions restarting the daemon — `fathom doctor`'s remedy, the
SessionStart line when the daemon is down, `fathom init`'s closing note —
names **the label actually installed on your machine**, discovered by reading
`~/Library/LaunchAgents`. Until 2026-09-01 they all named one developer's
label, so every other adopter was told to kickstart a service that did not
exist for them.

<details><summary>The plist it writes, if you would rather hand-roll one</summary>

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>dev.fathom.serve-http</string>
  <key>ProgramArguments</key><array>
    <string>/Users/you/.local/libexec/fathom/fathom</string>
    <string>serve-http</string>
    <string>--store</string><string>/Users/you/.fathom/db/fathom.sqlite</string>
    <string>--addr</string><string>:7575</string>
    <string>--call-log</string><string>/Users/you/.fathom/logs/mcp-calls.jsonl</string>
  </array>
  <key>EnvironmentVariables</key><dict>
    <key>PATH</key><string>/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>/Users/you/.fathom/logs/fathom-http.log</string>
  <key>StandardErrorPath</key><string>/Users/you/.fathom/logs/fathom-http.err</string>
</dict></plist>
```

```sh
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/dev.fathom.serve-http.plist
curl -s http://127.0.0.1:7575/healthz   # ok
```
</details>

No launchd (Linux/CI)? `nohup fathom serve-http --store … --addr :7575 &` works
for a session but dies with the shell — use a systemd unit or process
supervisor for anything persistent.

**Deploying a rebuilt binary** — because the daemon runs the installed copy,
not `bin/fathom` directly, a committed Go fix needs a deliberate deploy step,
run from a real terminal (not a sandboxed agent tool — the restart can wedge
one):

```sh
task deploy
```

If the restart hangs on macOS, look for a "fathom would like to access
files on a removable volume" dialog and click **Allow** (an unsigned,
per-hash grant re-prompts on every rebuild; the `fathom-dev` signing
identity the task uses makes the grant identity-based, granted once). This
is also why §13's binary-vs-data staleness check matters: a committed fix
does not reach the daemon until this step runs — and `task deploy`'s final
assert is what turns "I ran the deploy" into "the daemon serves HEAD".

---

## 6. Register the MCP server with Claude Code

Register against the curated agent surface — the recommended, dogfooded
path:

```sh
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent
claude mcp get fathom            # expect: Type: http · Status: ✔ Connected
```

Then add `"alwaysLoad": true` beside the entry's `url` in `~/.claude.json`
(or `.mcp.json`): with `ENABLE_TOOL_SEARCH` unset, Claude Code defers every
MCP tool schema behind `ToolSearch`, and a headless `claude -p` session pays
one ToolSearch turn — about a tenth of its tokens on a discovery task —
before its first fathom call (`.fathom/observations.md` O32; measured 1 turn
plain, 0 with the flag). `fathom plugin render` writes it already.

`--scope user` makes the server (and its tools) available to every Claude
Code session and every subagent spawned in any repo — not just the one you
registered it from.

**Why `/rpc/agent` and not bare `/rpc`.** The daemon serves several
path-scoped surfaces of the same store (`docs/mcp-tools.md`): `/rpc` is the
full registry (54 tools after the 2026-08-23 prune of 34 zero-call tools; the
surface reckoning, ADR-053, removed more since), unauthenticated, includes `ingest`
and `compile_layer`; `/rpc/agent` (aliased `/rpc/companion`) is the curated
19-tool coding-agent hot loop (§10). Claude Code defers full-registry tool
schemas until a tool is actually used, so `/rpc` costs little either way —
`/rpc/agent` is still the better default because it's what's dogfooded, and
because it makes off-surface tool use *measurable* (calls outside the
19-tool set still execute — they're logged `off_surface:true`, never
silently rejected — surfaces are execute-and-flag, never a security
boundary; see ADR-014's amendment in `docs/mcp-tools.md`). If you
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

`fathom init` (§3) wires all of these for you, idempotently. This section is
both the manual recipe and the reference for what gets written, pulled
verbatim from `cmd/fathom/init.go`'s hook specs and this repo's own
`lefthook.yml`.

**User-scope dedup.** Before writing any hook at PROJECT scope
(`.claude/settings.json`), `init` first reads Claude Code's USER scope
(`~/.claude/settings.json` — the same file, and only that file, `fathom
doctor`'s `hooks_installed` check reads) and skips any fathom hook already
wired there, printing an explicit `skip … — already wired at user scope`
line per skip. This matters if you (or an earlier `fathom init` run in
another repo) already wired these hooks globally: Claude Code applies BOTH
scopes, so writing an identical project-scope entry on top of an existing
user-scope one would make that hook fire twice per event. A hook wired at
user scope under a *different* command (not fathom's) doesn't count as
wired — fathom's own still installs alongside it. Re-running `init` after a
skip is a no-op for that hook (it stays skipped, not later re-added) unless
the user-scope wiring is removed, in which case the next `init` run installs
it at project scope as normal.

**Clean rooms and CI.** User-scope dedup is right on an operator's own
machine, and wrong for a harness that never reads user scope at all — a
Claude Code `--restricted` session started with `--settings <path>` (as
`cmd/scenariobench`'s clean room does, `docs/bench/scenarios.md`), or any
CI runner that hands the agent only a project-local settings file. On such
a machine, a fathom hook already wired globally would make `init` skip it
at project scope as usual, but the harness itself never sees the user-scope
copy — so the session ends up with NO fathom hooks at all, not a
double-fire. Pass `--hooks-scope project` (or set `FATHOM_HOOKS_SCOPE=project`
once, for a script that always wires clean rooms) to force every fathom
hook to project scope regardless of what user scope carries.

### 7.1 SessionStart — `session-hook`

Fires at session start (`startup|resume|clear|compact`). Pings the daemon,
maps the project directory to a service, and injects one line: "this repo is
service X, use fathom before Grep/Read" — or an honest "daemon down" /
"repo not indexed" signal. Always exits 0; a 2-second HTTP timeout means a
dead daemon can't stall session start.

On `startup` and `clear` **only** — the two sources where context was
actually lost — it additionally injects the **continuity thread brief**
(§7.7: local plans summary first, then filed handoff notes; ≤2,400 bytes,
budget-enforced, overflow named as `and N more`) and, when this repo has a
violated config concern or a recorded unproven claim, ONE **attention line**
(`⚠️ attention: config 2 · unproven 8 (+4) · fired 1 — fathom doctor`, ≤160 B):
the custodian's `concerns.cue` count (§8) and the intent corpus's last
recorded `fathom intent audit` share a line rather than taking two. The
intent half reads the ledger only — never an audit, never an RPC — and goes
silent when the recorded audit is older than 14 days, because a stale count
is not evidence about the working tree. The whole primer is capped at
3,200 bytes with any cut named. `resume` and `compact` keep their context, so those
sources inject neither — adding bytes there is pure tax. `fathom prime`
prints the same primer on demand (identity, freshness, a live example, the
thread brief, the attention line) for a harness with no SessionStart hook
— Codex — or for a human who wants to see what the session was told.

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

**Anchored memories no longer ride the payload**: SR-13 (surface reckoning
F-45, ADR-053) removed anchored memory — the hook's memory block, the
`memory_search` tool and the `fathom memory` verbs — for 0 memories in any
repository.

**The claim moment rides the same hook** (PC-22, vision 26 §4): when the
edited file is a depth-1 `docs/plans/*.md` and the edit added at least one
`[x]` relative to git HEAD, the hook runs the verify engine for that one
plan (the same daemon path the Stop hook uses) and injects the refusal —
`⛔ done-means-proven (claim moment): … [PC-9] … — symbol_present: …` — as
`additionalContext`, ahead of any obligations. Silent on a clean claim, on
an edit that claims nothing new (zero daemon traffic: the HEAD diff decides
first), and on any operational failure (I9). Every verify it runs leaves a
`~/.fathom/verify-ledger.jsonl` row with `origin: claim-moment` (the Stop
hook's rows say `origin: stop`, `fathom verify`'s say `origin: cli`); the
`claim-moment` beat channel records seen / clean / refused / error.

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
          bin/fathom verify --service <service> --plan "$f" --quiet
          code=$?
          if [ $code -eq 1 ]; then rc=1; fi
          if [ $code -eq 2 ]; then
            echo "fathom verify: operational error — allowing push (fail-open)"
          fi
        done
        exit $rc
```

`--quiet` prints only the failing rows (and, with `--strict`, the unknown
ones) plus the summary — a green plan costs one line per push instead of
one per row; it composes with `--quiet-pass`.

Exit-code contract (`fathom verify`, per
`docs/design/coding-agent-excellence.md` §5.3): **0** every claimed task
verifies; **1** a claimed `[x]` task failed verification — **block** the
push; **2** operational (daemon unreachable, service not indexed, empty
index) — **warn and allow** the push. This asymmetry is deliberate: a gate
that blocks pushes because a daemon happened to be down gets disabled within
a week. In CI, add `--strict` (claimed-but-Unknown also blocks) and treat
exit 2 as a **failure** too — CI builds its own store
(`fathom ingest` into a temp store, then `verify --store`), so "daemon down"
is never a valid CI state. (*Experimental in this repo:* no CI runner
exists yet; `scripts/ci.sh` is the manual, strict edition of the gate.)

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

### 7.5 PreToolUse — `edit-hook --pre` (guarded edits, phase O)

Fires BEFORE `Edit`, `Write`, `MultiEdit`, or `Bash` runs — the same
`edit-hook` binary/code path as §7.2, one flag apart. Silent-by-default for
every file/command: it checks `.fathom/guard.cue` locally (a glob/pattern
test, zero HTTP calls) before doing anything else, so an ungoverned edit
costs one comparison and nothing more (docs/design/guarded-edits.md).

`fathom init` scaffolds `.fathom/guard.cue` **commented out** — nothing is
guarded until you uncomment and commit a scope:

```cue
guards: [{
    scopes:          ["internal/store/**", "**/migrations/**"]
    note:            "Store schema is spec-governed; read docs/spec first."
    max_rules:       4    // OPTIONAL: pre-edit payload cap (default 4)
    include_modules: true // OPTIONAL: also serve bound platform-module rules (docs/use-cases/platform-modules.md)
}]
```

Every field except `scopes` here is optional — omit a line entirely to
leave it unset. Never write a trailing `?` on a field name in a file like
this one: `?:` is CUE's optional-field marker, meaningful in the SCHEMA
(`schema/guard/schema.cue`'s `#Guard`), but in this INSTANCE it silently
declares the field absent instead of setting it — `scopes?: [...]` would
leave the guard's scope unset entirely rather than restricting it.

A file matching a guard's `scopes` gets its `note` plus obligations
injected BEFORE the edit; a command matching `command_scopes` (e.g.
`["terraform apply*", "kubectl *"]`) gets the guard's `note` injected
before it runs — inject-only, never blocking (blocking is §7.4's job).

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          { "type": "command", "command": "/abs/path/bin/fathom edit-hook --pre", "timeout": 5 }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "/abs/path/bin/fathom edit-hook --pre", "timeout": 5 }
        ]
      },
      {
        "matcher": "Grep|Glob|Read",
        "hooks": [
          { "type": "command", "command": "/abs/path/bin/fathom edit-hook --pre", "timeout": 5 }
        ]
      }
    ]
  }
}
```

If a guard matched, the §7.2 PostToolUse render for the SAME edit collapses
to a one-line "already served pre-edit" marker rather than repeating the
rules — a short-lived local stamp (never a repo file, never committed)
tracks what the pre-hook already showed, scoped to the session when the
hook JSON carries `session_id`.

### 7.6 The discovery observer (phase P, production tokenomics) and the kill switch

The THIRD `PreToolUse` matcher group above (`Grep|Glob|Read`) is the SAME
`edit-hook --pre` binary again, routed to a **counting-only** path
(`discovery.go`) when Claude Code calls `Grep`, `Glob`, or `Read` —
docs/design/production-tokenomics.md §4's discovery-share denominator. It
injects nothing, never blocks, and makes **zero daemon calls** — it emits
one local beat per call with a coarse kind (`grep`/`glob`/`read`) and
returns instantly. `Bash` invocations get the same treatment alongside the
existing command-guard check: a command whose first token is
`grep`/`rg`/`find`/`ag`/`ack` emits a `shell-search` beat. `read` is
always its own bucket, never folded into `grep` (reading a file you
already located is dual-use, not necessarily discovery) —
`fathom research report`'s "discovery share" section reports the result,
labeled correlational.

**`FATHOM_CHANNELS=off`** is the kill switch for all of this at once:
session brief, both edit-hook phases (including the discovery observer
above), and the Stop hook all exit in total silence the instant this env
var is set to exactly `off`. It exists for the pre-registered A/B protocol
(`docs/plans/expert-suite-p.md` §5's OFF arm) and for debugging a hook
without noise. It is never silent about itself: `fathom doctor`'s
`channels_env` check reports a loud `warn` whenever it finds hooks
disabled by environment — a silent kill switch would repeat the exact O15
failure class this program has already been burned by once.

---

### 7.7 SessionEnd + PreCompact — continuity threads

Two disk-only hooks (they inject **nothing**; their context cost is zero
by construction) close the loop the brief in §7.1 reads from:

- **`session-end-hook`** (SessionEnd): on `/clear`/logout, deposits ONE
  mechanical fact per repo — dirty files, stashes, claimed-unproven plan
  rows. Latest deposit supersedes (no pile-up), and it **closes itself**
  once the tree comes clean. An empty session deposits nothing.
- **`pre-compact-hook`** (PreCompact, `manual|auto`): files a pointer
  (session id + repo) before compaction so the pre-summary span stays
  findable. It never reads transcript *content* — deposits are mechanical
  facts only, structurally.

The write side that matters most is yours, not the hooks': before a
`/clear`, file what the next session must know —

```sh
fathom thread note "purge suppression moved to the query layer — SP-1 next" \
  [--kind handoff-note] [--thread T] [--completes-when plan_row_checked:docs/plans/x.md#SP-1]
fathom thread point [--session ID]   # a pointer to where the detail lives
fathom thread list          # this repo's open items (deposits + notes)
fathom thread close <id> --why "…"   # or let a completes_when condition close it
fathom thread brief         # what the next fresh session will be shown (≤2,400 bytes)
fathom thread promote <thread> [--out docs/plans/<thread>.md]   # scaffold a plan from a thread
fathom thread handoff       # harness-neutral Markdown handoff of the open work
```

**Threads are scoped to the repo** (TH-6). `note`/`point` file on the
current repo's daemon-resolved service id by default (`general` only when
nothing resolves; `--thread` always wins). `list` and `brief` show this
repo's items and end with one line — `N open item(s) in other repos —
fathom thread list --all` — widen with `--all-repos` (list and brief) or
`--all` (list: every repo, plus closed and dormant items). Untagged legacy
items are never hidden. Notes with a `completes_when` condition
(`file_exists:<path>`, `plan_row_checked:<plan>#<ROW-ID>`, `repo_clean`)
close themselves when reality catches up, attributed `closed_by:
condition`. The derived spec plane (plans + `fathom verify`) always
outranks deposits in the brief — a note earns its bytes by saying what the
index cannot. The continuity bet itself is *experimental*: it is being
collected against a pre-registered criterion (4 of 20 units so far), so
treat the mechanism as shipped and the benefit as unmeasured.

### 7.8 InstructionsLoaded + ConfigChange — the harness as a witness (vision 26 §4)

Two more silent rows (exit 0 always, nothing on stdout — neither event
honors a decision, and a `ConfigChange` block would surface **no message
anywhere**, so fathom never emits one):

| Event | Matcher | Command | Timeout | What it leaves |
|---|---|---|---|---|
| `InstructionsLoaded` | none (every `load_reason`) | `fathom session-hook --event instructions-loaded` | 5s | one `instructions-loaded/loaded` beat `{file_path, memory_type, load_reason, session}` — the first *harness-side* evidence the stanza reached a session; `fathom doctor`'s `stanza_loaded` row reads it (PASS when a beat names this repo's `CLAUDE.md`/`AGENTS.md`, SKIP naming the wiring date otherwise — never FAIL) |
| `ConfigChange` | none (every `source`) | `fathom config audit --hook` | 10s | one `custodian/config-change` beat `{source, file_path, result, violated}` after running the custodian audit on the repo; the next session's attention line carries the violated count |

```json
{
  "hooks": {
    "InstructionsLoaded": [
      { "hooks": [ { "type": "command", "command": "/abs/path/bin/fathom session-hook --event instructions-loaded", "timeout": 5 } ] }
    ],
    "ConfigChange": [
      { "hooks": [ { "type": "command", "command": "/abs/path/bin/fathom config audit --hook", "timeout": 10 } ] }
    ]
  }
}
```

Both rows come from the one hook table (`internal/harness`, ADR-029), so
`fathom init`, `fathom plugin render`, `fathom fleet reconcile` and
`fathom doctor` all carry them; a repo wired before they existed gains
exactly these two entries on its next `init`/`reconcile` and loses nothing.

### 7.9 The advisory events — seven more rows that never block (wave 3)

Claude Code 2.1.270 offers **33** hook events. Fathom wires fifteen of them
and leaves eighteen alone *on purpose*; `fathom doctor`'s `hooks_coverage`
row names every unwired event with its reason, so "why isn't fathom on
`UserPromptSubmit`?" has an answer you can argue with instead of a silence.
It is advisory and never FAILs — an unwired event is a decision.

The seven rows below were added in wave 3 of `docs/plans/harness-port.md`.
Only two of them can inject: the hooks reference documents
`hookSpecificOutput.additionalContext` on `SubagentStart` and
`PostToolUseFailure` and **not** on the other five, so fathom records those
five as beats and says so rather than promising a channel the harness does
not have.

| Event | Matcher | Command | Injects? | What it does |
|---|---|---|---|---|
| `SubagentStart` | none | `fathom session-hook --event subagent-start` | yes | a ≤600 B primer — the service identity line plus the opening-moves table — so a subagent that never saw `SessionStart` still starts index-first. Since RC-28 (docs/vision/28-receipts.md §4), when the parent session already established discoveries (call-log pointers since its own recorded start), a "last session established: …" brief is appended, resolved fresh, within 1,200 bytes TOTAL — the ≤600 B primer stays the cap when there is nothing to add. Silent when the root is not an indexed service |
| `PostToolUseFailure` | `mcp__fathom__.*\|mcp__plugin_[^_]*_fathom__.*` | `fathom session-hook --event tool-failure` | yes | one remedy line after a **fathom** tool call fails: daemon down, repo not indexed, or "the wiring is fine, check the arguments". Silent for anyone else's failure, which is almost all of them. The matcher covers the plugin-scoped name (`mcp__plugin_<plugin>_<server>__<tool>`) as well as the plain one |
| `FileChanged` | `CLAUDE.md\|AGENTS.md\|concerns.cue\|config.toml` | `fathom config audit --hook --event file-changed` | no | the same custodian audit and beat `ConfigChange` writes, on the other trigger: a governed file changed *on disk* (a sibling agent, a rebase, a merge). The matcher is literal filenames — the reference documents no glob here |
| `WorktreeCreate` | — | not wired | — | a creation hook: a registered hook must create the worktree and print its path, so fathom does not register one (O34); the worktree line rides on `SubagentStart` when the subagent starts inside a worktree |
| `CwdChanged` | none | `fathom session-hook --event cwd-changed` | no | the identity line for the root the session moved to |
| `DirectoryAdded` | none | `fathom session-hook --event cwd-changed` | no | the same, for `/add-dir` and the SDK (the new root arrives in `directory_path`) |
| `Setup` | none | `fathom doctor --brief` | no | FAIL rows only, at most 5 lines, **exit 0 whatever it finds** — a setup diagnosis that fails setup is worse than no diagnosis |

A repo wired before wave 3 gains exactly these seven entries on its next
`fathom init` / `fathom fleet reconcile` and loses nothing: they are
appended after the existing rows, and the pre-existing table is pinned
byte-for-byte by `cmd/fathom/hookspecs_test.go`.

The test-run hook (ADR-056 decision 6, `docs/design/test-run-hook.md`) adds
two more rows, appended the same way. Both are inert until `[hooks] test_run
= true` (see `docs/configuration.md`):

| Event | Matcher | Command | Injects? | What it does |
|---|---|---|---|---|
| `PostToolUse` | `Bash` | `fathom session-hook --event test-run` | yes | after a Bash command that runs a declared test command, one line naming the failing tests and the changed symbols they reach; nothing on a pass. A piped test run (`\| tail`) arrives here, with no exit code |
| `PostToolUseFailure` | `Bash` | `fathom session-hook --event failed-test-run` | yes | the same for an unpiped run that exited non-zero: the output arrives in `error`, led by `Exit code N` |

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

Two more CUE files live here, both law-13-scoped (a user-scope default in
`~/.fathom/`, the repo file **governs**):

- **`.fathom/capabilities.cue`** — which fathom features are on in this
  repo. `fathom capabilities` (`list`, `--cost`) shows every capability
  with what it costs, whether it is operating, and WHY anything is off;
  `fathom capabilities explain <id>` goes one deeper; `fathom capabilities
  set <id> on|off [--scope project|user]` writes the file — it splices
  only the `capabilities` block and preserves every other byte (comments,
  the `budgets` block below), refusing when the file cannot be
  round-tripped; ids that are not identifiers (`verify-gate`) are written
  as quoted labels. The same file carries an optional **`budgets`** block
  (BA-9, *experimental — new*) sizing the answer budget per tool class in
  **tokens**, closed schema (an unknown key such as `lists` is refused
  with a did-you-mean), resolved project → user → built-in:

  ```cue
  budgets: { list: 1500, composite: 4000, map: 2500 }
  // list = query_symbols, references · composite = pack_context · map = repo_map
  // find_symbol / find_callers / symbol_context / outline are NOT class-budgeted
  ```

  Honoured by the `serve-http` daemon; stdio `serve-mcp` still uses the
  built-ins (a documented gap). Elision is always disclosed in the answer,
  and each response reports its own cost (the observatory's Cost panel
  aggregates them).
- **`.fathom/concerns.cue`** — the config custodian's contracts (vision
  22, "instructed means true"): falsifiable wants about this repo's agent
  config, each a `check: {kind: …}` — `forbidden_pattern` (a phrase that
  must not appear), `command_documented_and_exists`, `max_lines` (a line
  budget for a file), `pointers_resolve` (relative Markdown links), and
  two newer kinds (*experimental*): `stanza_tools_served` — every tool the
  fathom stanza names (backticked, or bare with an underscore such as
  `find_callers`) must be served by the daemon's `/rpc/agent` `tools/list`;
  daemon down or an empty registry is `unable to check`, never violated —
  and `handoff_cites_instruments` with `days: N` — a handoff note filed for
  this repo in the window that claims shipped/landed/done/fixed work must
  cite a commit sha, an active plan row id, `fathom verify`, or a retro
  artifact path. `fathom config audit` reports per-concern
  holds/violated/unable **with `file:line` evidence**, the doctor gains a
  `concerns` row, and §7.1's attention line fires while a violation
  exists. The custodian is otherwise offline: `stanza_tools_served` is the
  one kind that asks the daemon, and it degrades honestly. Fathom never
  edits your prose — it reports; the session decides (the
  `fathom-custodian` skill carries the loop). `fathom config audit
  --code-facts` (VB-28, *experimental*, off by default) extends the same
  audit to code-fact-shaped sentences in CLAUDE.md and beyond — including
  Claude Code's own per-project auto-memory under
  `~/.claude/projects/<encoded-root>/memory/*.md`, not only the stanza.
- **`.fathom/memories.jsonl`** — no longer read: anchored memory was removed
  by SR-13 (ADR-053). A leftover file makes `fathom purge` fail naming it.

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

## 10. The 19-tool agent surface — when to use what

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
| `references` | Occurrence search for an identifier or string in your own service, or across every indexed service with `services: ["*"]` — replaces cross-repo grep |
| `read_source` | You have a file and line range (from a citation, `find_symbol`, or a DocChunk's `line`/`end_line`) and want the exact current text, line-numbered — the surface's bounded `sed -n` |
| `pack_context` | You're an architect/lead assembling a pre-packed, paste-ready brief for a worker or subagent — signatures, file paths, contract types in one shot |
| `entrypoints` | You need to know where execution starts — `main`, HTTP handlers, CLI roots — e.g. before a blast-radius or dead-code question |
| `obligations` | Before editing a file, to see governing rules explicitly (this is also what the PostToolUse hook calls automatically after the edit — calling it before is a preview) |
| `verify_plan` | You want an evidence-backed verdict on a plan's claimed tasks without waiting for the pre-push gate — same engine `fathom verify` uses |
| `staleness_report` | Before trusting query results after heavy uncommitted edits, or when a query result looks stale/wrong |
| `list_services` | You need to know what's actually indexed (service IDs, symbol counts, languages) before assuming a repo is in the fleet |
| `ingest` | Force a re-index right now — after a commit, or when `staleness_report` says you should, or to onboard a repo that isn't indexed yet. A read failure comes back as a JSON-RPC error naming the cause (and the TCC hint), never `ok:true` (§2) |

The full registry (`/rpc`, 55 tools on this build) adds quality/impact
analysis (`complexity`, `dead_code`, `blast_radius`, …), `test_impact`,
spec traceability, and platform/cross-repo tools — see `docs/mcp-tools.md`
for the complete reference (a test keeps it in step with the registry);
they still work when called off the agent surface (execute, just flagged
`off_surface` in the call log), so nothing is actually locked out.

---

## 11. Agent skills — the ten shipped with the binary

`fathom init --skills[=claude|codex|agents|all|codex-prompts]` installs
ten shipped, versioned skill sources (`dist/agents/skills/`, embedded in
the binary — no fathom source checkout needed at install time):

```sh
bin/fathom init --skills          # bare = all: .agents/skills + .claude/skills
bin/fathom init --skills=claude   # .claude/skills only
```

Or package all ten with the hook template and the MCP registration as a
plugin — `fathom plugin render` writes one tree that carries both Claude
Code's `.claude-plugin/plugin.json` and the Agent Plugins 1.0
`plugin.json`/`mcp.json` over one `skills/` directory
(`docs/usage.md` §plugin; `claude --plugin-dir <out>` for a session, a
marketplace for a fleet):

```sh
bin/fathom plugin render --out fathom-plugin
claude plugin validate --strict fathom-plugin
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
- **`fathom-doctor`** — run and interpret `fathom doctor` (§13 below): the
  deterministic wiring checklist, remedies applied only with operator
  approval, plus the tie-in check (a live tool call, hook latency against
  the budget ledger).
- **`fathom-impact`** — narrate `fathom instrument --adoption` +
  `fathom plans status` + the open-observation backlog honestly: what was
  measured, the predates-instrumentation split, and the open frictions as
  the flywheel's ask (§12 below).
- **`fathom-rewire`** — reconcile a repo's wiring in place (`fathom fleet
  reconcile --apply`: hooks vs the current template, stale binary paths →
  the PATH shim, stanza content regenerated), with doctor's known false
  signals annotated so drift-chasing doesn't fix the wrong repo.
- **`fathom-activate`** — turn on the delivered-configuration features in
  a wired repo (guards, layer packs, bindings, concerns, loud gates),
  each proven by demonstration, never by assertion.
- **`fathom-custodian`** — the "instructed means true" loop: register
  falsifiable concerns about the agent config, read `fathom config
  audit`'s evidence, and apply fixes as the session — fathom reports, the
  session edits (§8).
- **`fathom-findings`** — the findings loop (`docs/vision/30-findings-loop.md`,
  ADR-040): import an external review kit's outbox, inspect
  (`findings status`/`near`/`candidates`), refute/corroborate with
  `fathom claim`, read a receipt honestly, and what fathom never does with
  an imported finding's authority.

Each is rendered from one authored source into three shapes, selected by
the `--skills` value:

| Value | Writes | Shape |
|---|---|---|
| `claude` | `.claude/skills/<name>/SKILL.md` | the embedded `SKILL.md` verbatim (Claude Code's frontmatter superset) |
| `codex`, `agents` | `.agents/skills/<name>/SKILL.md` | the Agent Skills standard path every other harness reads (Codex, Gemini CLI, Cursor, Copilot, …); frontmatter reduced to the six spec fields, `name` = the directory (`schema/skills/schema.cue` vets it under `task cue-vet`) |
| `all` (bare `--skills`) | both of the above | the pair covers 14 of the 15 surveyed harnesses; both repo-scoped |
| `codex-prompts` | `~/.codex/prompts/<name>.md` | the deprecated Codex custom-prompts twin (`codex-prompt.md`) — the only user-global write, explicit only (`docs/upgrades/0.8.0-skills-on-the-standard.md`) |

`fathom doctor`'s `skills_installed` row reads `.claude/skills` for Claude
Code and `.agents/skills` for every other `--harness`, naming each missing
skill; `fathom fleet reconcile` plans both repo-scoped shapes through the
wiring engine (a stale marker-owned copy is rewritten and receipted, a
hand-authored file by a skill's name is BLOCKED by name — §11.1).

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

Three more gauges sit beside them: `fathom flywheel` (each feedback loop
CLOSED/OPEN/IDLE with its evidence and remedy), `fathom rate [--note …]`
(a 1–5 outcome rating for the session — the human half of the A/B
criterion, *experimental*, optional), and the week-long
`fathom research report|bundle` runbook in
[docs/adoption/research-flywheel.md](research-flywheel.md). The A/B
counterfactual itself is still collecting (ON arm live, ≥20 units per arm
needed) — until then no savings claim is admissible (law 12).

---

## 13. Troubleshooting

**Start here: `fathom doctor`.** Before working through the entries below by
hand, run the deterministic wiring checklist — it covers MCP registration,
daemon reachability, binary freshness, the hook stack (installed AND
firing), the `CLAUDE.md`/`AGENTS.md` stanza, shipped skills, `.fathom/`
config validity, obligations serving, and surface traffic in one command:

```sh
bin/fathom doctor [--harness=claude|codex|auto] [--json]
```

Each row is `{name, status, reason, remedy}` — `pass`/`fail`/`skipped`
(never a guessed pass for an uncheckable state), with a one-command remedy
on every failure. Exit `0` all-pass, `1` a real problem, `2` doctor itself
couldn't run. See the `fathom-doctor` skill (§11) for interpretation and
the remedy-approval discipline. The entries below are the manual/reference
version of what `doctor` automates — useful when you want the "why," not
just the "what to run."

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
  it until you deploy: `task deploy` (never a raw `cp` — §5). Diagnose by
  comparing `bin/fathom version` (or the daemon's, via a tool call) to
  `git rev-parse HEAD` — a mismatch means stale, not buggy; `task deploy`
  ends by asserting exactly this.

**`ingest` says `degraded` or `refused`.** Read the `read-failed` column
(§2): the named paths could not be read. `stat` succeeds but `cat` fails on
an external volume is a macOS TCC wedge — run the `tccutil reset` hint the
error prints, then re-ingest. A refused run wrote nothing, so the previous
index is intact.

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

- [ ] Calling `capabilities` returns `tool_count: 17` (or the count of
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
      `.claude/skills/fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian}/SKILL.md`
      (§11), and `bin/fathom instrument --adoption` (§12) runs cleanly,
      reporting `no .fathom/observations.md yet` until the first `fathom
      observe` call, then a real open-observation count after.
- [ ] `bin/fathom doctor` (§13) reports every row `pass` — or, faster than
      working through this checklist by hand, just run it first.
- [ ] The daemon's observatory renders: `http://127.0.0.1:7575/observatory`
      (fleet page — a 30-day grade distribution, *experimental*: days
      without a persisted snapshot render Absent, today only until daily
      snapshots persist), `/observatory/service?id=<svc>` (per-dimension
      30-day series — queried, discovering, preventing, gating — from real
      call-log rows; a day with no rows renders Absent with its reason, a
      day the log was alive but this service saw nothing is a measured 0,
      and sparklines never draw across an absent day), `/observatory/impact`,
      `/observatory/brief`, and `/observatory/wiring/` (the §3.1 engine over
      HTTP: `preview` returns a plan plus `plan_digest`, `apply` requires it
      and answers 409 if the repo changed since, `undo` requires the
      `receipt_digest` apply returned; mutating endpoints accept loopback
      Host, same-origin, registered service ids and roots only). Tiles that
      cannot read the call log or store render Absent, never 0.

If every box above is checked, the full loop — index, serve, inject,
gate — is live.

---

## Maintained

This doc covers: `fathom init` (incl. `--skills[=claude|codex|all]` and the
`CLAUDE.md`/`AGENTS.md` marker-delimited stanza, v3 digest markers, law-15
blocking, receipts + `fleet undo`/`receipts`), `ingest` (read-failed /
degraded / refused), `scope`, `purge`, `serve-mcp`, `serve-http`,
`compile`, `layers` (`list`/`show`/`install`), `verify`
(`--strict`/`--require-all`/`--quiet`), `observe`, `instrument
--adoption`, `doctor` (`--harness=claude|codex|auto`, `--json`), `config`
(incl. `config audit` and the six concern kinds), `capabilities`
(`list`/`explain`/`set`, the `budgets` block), `memory`
(`add`/`list`/`review`/`show`), `providers`, `smoke`, `session-hook`
(incl. `--event instructions-loaded`), `session-end-hook`,
`pre-compact-hook`, `edit-hook` (obligations + the memory block + the
claim-moment verify), `stop-hook`, `config audit --hook`, `thread`
(`brief`/`note`/`point`/`list`/`close`/`promote`/`handoff`, repo scoping),
`task deploy`; the `/rpc`, `/rpc/agent` (`/rpc/companion`) HTTP surfaces;
the agent allowlist (incl. `read_source`); the
observatory pages (fleet, service, impact, brief, wiring); the
`SessionStart`/`SessionEnd`/`PreCompact`/`PostToolUse`/`PreToolUse`/`Stop`
Claude Code hook wiring; the lefthook pre-push verify gate; the ten
shipped agent skills (`dist/agents/skills/` — plan, verify, onboard,
feedback, doctor, impact, rewire, activate, custodian, findings). Update
this doc's examples and this line when any of these change.

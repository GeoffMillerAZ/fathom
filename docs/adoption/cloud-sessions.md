# Cloud sessions: Claude Code on the web and Codex cloud

A cloud session runs the agent in a fresh Linux VM instead of on your machine:
[Claude Code on the web](https://code.claude.com/docs/en/claude-code-on-the-web)
(claude.ai/code, the Claude apps, `claude --cloud`) and Codex cloud. This page
gets fathom working inside that VM with no change to your local setup.

What the VM gets:

- **Tools.** go-task, cue and lefthook at the versions pinned in
  `scripts/cloud-tools.env`, and `bin/fathom` built pure Go (`CGO_ENABLED=0`, ADR-066: no C compiler).
- **An index.** The repository is ingested into `~/.fathom-cloud/store.sqlite`
  during setup, so it is already on disk when the session starts. Today the
  setup indexes the fathom repository itself, as service `fathom`, one
  repository per environment.
- **A daemon per session.** It listens on `127.0.0.1:7575` inside the VM, the
  address the hooks already use.
- **The hook stack.** It comes from the repository's `.claude/settings.json`:
  the primer at session start, obligations on every edit, and the stop and
  compaction hooks.
- **fathom's MCP tools.** They reach Claude through a stdio front for that
  daemon (`fathom serve-mcp --url`).

What stays on your Mac:

- **Deploy.** `task deploy` needs codesign and launchd.
- **Your fleet daemon and its index of every repo.** The VM indexes only this
  repository.
- **Your `~/.claude` settings and MCP servers.** Cloud sessions do not read
  them.

## 1. Claude Code on the web: setup

1. **Give the cloud access to the repo.** Use either:
   - the Claude GitHub App on the repository (the browser onboarding asks), or
   - `/web-setup` in your terminal, which sends your `gh` token.

   A session needs one of the two to push its branch back to GitHub.
2. **Create the environment.** Open the environment selector at claude.ai/code
   and choose **Add environment**:
   - **Name:** `fathom`
   - **Network access:** `Trusted` (the default). It covers every host in §4.
   - **Environment variables:** none are required. For long gate runs, add
     `BASH_DEFAULT_TIMEOUT_MS=600000` and `BASH_MAX_TIMEOUT_MS=1800000`. The
     Bash tool otherwise moves a command to the background after 2 minutes.
   - **Setup script:** paste the script in §1.1.
3. **Make it the CLI default for this repo.** Run `/remote-env` in a local
   Claude Code session and pick `fathom`. `/remote-env` writes
   `remote.defaultEnvironmentId` to your user settings, so `claude --cloud`
   then uses `fathom` in every project. Keep it to this repo: the snapshot and
   the VM's MCP registration belong to the environment, not to a repository.
   A session on another repo in `fathom` starts from a snapshot built for
   fathom. If that other repo's session runs first, the cached snapshot has no
   fathom in it. To scope it, move the key from your user settings into this
   checkout's `.claude/settings.local.json`, which takes precedence over user
   settings.
4. **Start a session.** `claude --cloud` clones your current branch from
   GitHub, not your working tree, so push first. Then run:

   ```sh
   claude --cloud "Fix the flaky test in internal/store and run task validate"
   ```

   Or start one at claude.ai/code with the `fathom` environment selected.
5. **Bring it back.** Run `claude --teleport` for a picker, or
   `claude --teleport <session-id>`, from a clean checkout of this repo.
   Inside a CLI session, `/teleport` does the same. The cloud session's branch
   must be pushed; teleport fetches and checks it out.

### 1.1 The setup script to paste

```bash
#!/bin/bash
# fathom: provision this cloud VM (docs/adoption/cloud-sessions.md). Always exits 0.
# rev 2 (bump after a provisioning commit: the platform rebuilds its cached snapshot only when this text changes)
repo=""
for d in "$PWD" /home/user/fathom "$HOME/fathom" /workspace/fathom; do
  if [ -f "$d/scripts/cloud-setup.sh" ]; then repo="$d"; break; fi
done
if [ -z "$repo" ]; then
  f="$(find / -maxdepth 5 \( -path /proc -o -path /sys -o -path /dev \) -prune -o -path '*/scripts/cloud-setup.sh' -print 2>/dev/null | head -1)"
  [ -n "$f" ] && repo="$(dirname "$(dirname "$f")")"
fi
if [ -n "$repo" ]; then
  bash "$repo/scripts/cloud-setup.sh" || true
else
  echo "fathom: scripts/cloud-setup.sh not found yet; the session hook provisions in the background"
fi
exit 0
```

The pasted script is only a locator. The work is in the committed
`scripts/cloud-setup.sh`, so a change to provisioning is a reviewed commit
rather than an edit to the environment dialog.

The platform keys its cache on the pasted text, not on the committed script.
A commit that changes `scripts/cloud-setup.sh`, `scripts/cloud-tools.env` or
go.mod reaches a cached environment only when the snapshot expires, after
about seven days. Until then, sessions keep the snapshot's tools and the
`bin/fathom` built at setup; only `cloud-session.sh` is refreshed from the
clone. To roll a provisioning commit out at once, bump the `# rev` line in
the pasted text.

Each step of `scripts/cloud-setup.sh` checks before it acts, so it is also safe
as a maintenance script and on a cached snapshot. It always exits 0: a
non-zero exit would stop the session from starting.

### 1.2 What to expect

- **The first session in the environment runs the setup.** The CLI's checklist
  shows "running your setup script". In the measurements below it took about
  2.5 minutes. It ends with one line (this one is from 2026-10-03; the symbol
  count grows with the repository):

  ```
  cloud-setup: done in 154s — go1.26.2 (4s) · bin/fathom (78s) · index ingested (45486 symbols) (72s) · task v3.52.0, cue v0.16.1, lefthook v1.13.6 (150s, beside the build) · mcp registered (VM user scope) · failed: none · log /root/.fathom-cloud/setup.log
  ```

- **Later sessions skip it.** When setup finishes inside the roughly five-minute
  window, the platform snapshots the filesystem and new sessions start from
  that snapshot for about seven days. A change to the pasted setup script or
  to the allowed hosts rebuilds the snapshot. A commit to the files it runs
  does not (§1.1).
- **Every session starts the daemon.** The SessionStart hook does it, then
  prints fathom's primer ("this repo is the indexed service "fathom" …"). The
  daemon refreshes the index against the clone, so a snapshot that is days old
  catches up by itself. This took under a second in the measurements.
- **The tools show up in the session.** `/mcp` lists `fathom`, and edits get
  `⚖️ fathom obligations` lines like they do locally.
- **The gate works.** `lefthook install` has run in the clone, so a push from
  the session runs the same pre-push gate as your Mac.

Logs live in `~/.fathom-cloud/`:

- `setup.log`, with one log per step under `logs/`
- `session.log`
- `daemon.log`
- `mcp-calls.jsonl`

### 1.3 Checking it inside a session

Ask Claude to run these:

```sh
tail -1 ~/.fathom-cloud/setup.log          # the setup summary line
curl -s 127.0.0.1:7575/healthz             # ok
fathom doctor --brief                      # the wiring checklist, from the VM
```

To confirm the MCP tools, type `/mcp` yourself.

## 2. Codex cloud

Codex cloud uses the same committed script:

1. Create an environment for this repository.
2. Paste the §1.1 script as its **setup script**. Codex runs setup with
   internet access, so the Go proxy is reachable.
3. Optionally, use `bash scripts/cloud-setup.sh` as the **maintenance script**.
   It refreshes a cached container incrementally.

Codex has no hooks, so nothing starts the daemon for you. The agent runs this
once per task:

```sh
bash scripts/cloud-session.sh ensure       # starts the VM-local daemon; idempotent
```

AGENTS.md's stanza already asks Codex to run `fathom prime` at the start.

The setup also writes `[mcp_servers.fathom]` into the VM's
`~/.codex/config.toml`. Whether Codex cloud loads MCP servers is unverified.
Without them, the store-direct CLI works with no daemon:

- `fathom verify --service fathom --plan docs/plans/<plan>.md --store ~/.fathom-cloud/store.sqlite`
- `fathom review --since origin/main --store ~/.fathom-cloud/store.sqlite`

Codex cloud is **unmeasured**. The script's steps are the same as for
Claude Code, but no Codex cloud run has been made.

## 3. What runs where

| Piece | Lives in | Starts |
|---|---|---|
| Go toolchain, go-task, cue, lefthook, `bin/fathom`, the index | the VM snapshot (`scripts/cloud-setup.sh`) | once per snapshot (about 7 days) |
| The daemon, `127.0.0.1:7575`, unmanaged, auto-refresh on | the VM | each session: the SessionStart hook or the MCP front, whichever runs first (`scripts/cloud-session.sh`) |
| Hooks | the repo's `.claude/settings.json` (the cloud reads no user settings) | per event; the SessionStart entry is `cloud-session.sh hook fathom session-hook`, which exits at once when `CLAUDE_CODE_REMOTE` is not `true` |
| MCP | the VM's own `~/.claude.json` (user scope, written by setup): `bash ~/.fathom-cloud/cloud-session.sh mcp`, then `fathom serve-mcp --url http://127.0.0.1:7575/rpc/agent` | when Claude Code starts |
| Instructions | `AGENTS.md` alone, root and nested (ADR-059; the 2026-10-02 CLAUDE.md-import amendment was revoked 2026-10-17: Claude Code 2.1.281 and later read AGENTS.md in every session kind) | every session on Claude Code 2.1.281 or later |
| The gate | `task validate` and the lefthook pre-push, unchanged. Every step passes on Linux as root (CS-8); `-race` needs cgo off macOS, so `task test` runs with `CGO_ENABLED=1` and a C compiler (gcc) on PATH, and `store-purego` drops `-race` | on a push from the session |

A local Claude Code session in this checkout sees none of this:

- The SessionStart wrapper costs one `bash` start (about 10 ms) and prints
  nothing when `CLAUDE_CODE_REMOTE` is not `true`.
- The repo has no `.mcp.json`, so a local session uses whatever `fathom`
  server you registered at user scope.
- `.claude/settings.json` also carries five of fathom's template hook entries:
  the PostToolUse edit hook, the Grep/Glob/Read pre-hook, Stop, SessionEnd and
  PreCompact. If your user settings carry the same entries, Claude Code runs a
  handler defined in two settings files once.

## 4. Network hosts

All of these are on the **Trusted** list:

| Host | Used for |
|---|---|
| `proxy.golang.org`, `sum.golang.org` | Go itself (go.mod's toolchain through `GOTOOLCHAIN=auto`, or the toolchain zip when no `go` exists), every module, and the three tools (`go install module@version`) |
| `archive.ubuntu.com`, `security.ubuntu.com` | `rsync` for `task stranger-check`, installed only when absent |
| `github.com` (the session's GitHub proxy) | the clone itself |

Setup makes no GitHub release-asset download and no `dl.google.com` download.

## 5. Limits

- **The index covers this repository only.** Cross-repo answers
  (`platform_callers`, fleet views) see one service.
- **A slow setup leaves the index to the session.** When setup runs out of its
  270-second budget, it stops the ingest instead of overrunning the cache
  window. The first session then indexes through the daemon in the background,
  and answers are partial for a few minutes. Every later session does the
  same until a setup finishes inside the window.
- **No deploy in the cloud.** `task deploy` is macOS-only and stays local.
- **Nothing local carries over.** Your `~/.fathom` ledgers, thread notes,
  bench data and siblings do not reach the VM. `vendor-check` and
  `testkit-schema-check` skip without a sibling checkout, as they do in any
  fresh clone.
- **Multi-repo sessions get no hooks.** The cloud does not load repository
  hooks in a session with several repositories. The MCP front still starts the
  daemon.
- **The gate is slow on 4 vCPUs.** A full `task validate` took 627 s in the
  container below. Set the timeout variables in §1, step 2.
- **`task test` needs the VM's memory.** It is green on Linux as root (plan
  row CS-8) in a root container of the VM's shape (4 vCPU, 16 GB). Two
  things come from the session: the hook's TMPDIR off `/tmp` and go.mod's
  toolchain first on PATH (`cloud-session.sh` sets both). The default
  concurrency, four package binaries plus four `cmd/fathom` shards under
  `-race`, peaked at 14.9 GB of container memory, page cache included. On
  an 8 GB machine with no swap the kernel kills `internal/extractor`'s test
  binary; with swap the suite passed.

## 6. If `/mcp` does not list fathom

The platform's documentation says the user-scope MCP servers on *your machine*
do not carry over. Whether a server written into the *VM's* `~/.claude.json`
before Claude Code starts is honored is checked only by a real session (plan
row CS-M1). If it is not:

1. **The hooks still work.** They talk to the daemon, not to MCP, so the
   primer and obligations are unaffected.
2. **Claude can call the same tools over HTTP.** For example:

   ```sh
   curl -s 127.0.0.1:7575/rpc/agent -H 'Content-Type: application/json' \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"find_symbol","arguments":{"service":"fathom","name":"runServeMCPFront"}}}'
   ```

3. **A maintainer can commit a project `.mcp.json` entry.** Name it
   `fathom-cloud`, with the command `bash scripts/cloud-session.sh mcp`. The
   cloud loads a repository's `.mcp.json`. Locally the entry would show as a
   project server to approve or disable, which is why it is not committed by
   default.

## 7. The files

- **`scripts/cloud-setup.sh`** provisions the VM. Tests:
  `scripts/cloud-setup_test.sh`.
- **`scripts/cloud-session.sh`** runs per session: `hook`, `ensure` and `mcp`.
- **`scripts/cloud-tools.env`** holds the tool pins as plain `KEY=value`
  lines with no shell expansion, so a generator can write it and any reader
  can parse it without running it.
- **`.claude/settings.json`** carries the SessionStart wrapper and the five
  template hooks.
- **`cmd/fathom/servemcp_front.go`** is `fathom serve-mcp --url`
  ([usage](../usage/serve-mcp.md)).

## Measured, and not

Measured on 2026-10-02 in an `ubuntu:24.04` container (Colima, aarch64, 4 vCPU,
7.7 GB). The container started with Ubuntu's Go 1.22, so the toolchain switch
to go1.26.2 through the proxy was exercised.

| Step | Time |
|---|---|
| Cold setup | 154 s: toolchain 4 s, `bin/fathom` 78 s, ingest 72 s, tools beside the build |
| Earlier cold run, host under heavy load, tools competing with the build | 270 s; the ingest stopped at the budget and was deferred |
| Re-run on the same disk | 2 s |
| Cloud SessionStart, including the daemon start and a live primer | 0.85 s |
| SessionStart on a clone at another path (remapped first) | 1.1 s |
| MCP front: initialize, tools/list (the 8-tool agent surface) and a `find_symbol` answer | 34 ms |
| Obligations | returned by the VM daemon on a simulated edit |
| Full `task validate`, as root | 627 s: every step but `task test` green; 35 test failures before the session env fix, 8 left in `cmd/fathom` after it, plus the root and container classes in 7 packages (CS-8) |
| `task test` as root, 4 vCPU and 16 GB, 2026-10-03 (CS-8) | green in 9 min 21 s; the same container failed 18 tests in 8 packages before the CS-8 fix |
| `task validate` as root, 4 vCPU and 8 GB plus 12 GB swap, 2026-10-03 (CS-8) | green in 7 min 6 s, every step |

Not measured:

- a real Anthropic VM (x86_64, its proxy, its snapshot)
- whether the VM-written MCP registration is honored (CS-M1)
- Codex cloud
- a green `task validate` inside a real cloud session (CS-M2)

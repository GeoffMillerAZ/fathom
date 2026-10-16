# Adopting fathom in Claude Code

From a fresh machine to a Claude Code session that answers "who calls this",
"what breaks if I change it" and "what rules govern this file" from an index
instead of grep: build, index, run a daemon, register the tools, wire the
hooks, and check that it works. The detail behind each step (every hook's
behavior, the daemon supervisor, the `.fathom/` files, long troubleshooting) is
in [claude-code-internals.md](claude-code-internals.md); you do not need it to
follow this page.

Related: [coding-agents.md](../use-cases/coding-agents.md) (the short
summary), [configuration.md](../configuration.md) (provider config),
[mcp-tools.md](../mcp-tools.md) (every MCP tool), [codex.md](codex.md) (the
same story for Codex).

## What you get

- **Eight tools** your agent can call (`find_symbol`, `symbol_context`,
  `blast_radius`, `review_diff`, `obligations`, `verify_plan`, `read_source`,
  `search`), served from a local index of your repository.
- **Hooks** that put the index in front of the agent without being asked: a
  one-line identity and freshness note at session start, the rules that govern
  a file after each edit, and a check at session end that checkboxes the agent
  ticked in `docs/plans/*.md` are backed by code.
- **A pre-push gate** (`fathom verify`) that blocks a push claiming work the
  index cannot find.
- **Ten skills** that teach the agent the loops: planning, verifying, wiring,
  filing friction.

Everything runs on your machine. Nothing is sent to a hosted service.

## Install

You need Go 1.26 or later. fathom is pure Go; on Linux build with
`CGO_ENABLED=0` and no C compiler. On macOS, cgo (Xcode Command Line Tools)
serves only the daemon's two memory readers. The kits fathom depends on are
mirrored in the repository, so a fresh clone builds with nothing else checked
out.

```sh
git clone https://github.com/GeoffMillerAZ/fathom.git && cd fathom

# macOS
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom
# Linux
CGO_ENABLED=0 go build -o bin/fathom ./cmd/fathom

bin/fathom version     # prints the commit it was built from
```

Put `bin/fathom` somewhere stable (for example `~/.local/bin`); the hooks and
the daemon bake its path into their commands. CUE, go-task and lefthook are only
needed to work on fathom itself, and lefthook only if you want the pre-push gate.

## Index a repository

The fastest route is `fathom init` (next section), which ingests for you. By
hand:

```sh
fathom ingest --service my-service --path . --store ~/.fathom/db/fathom.sqlite
```

Write a `.fathomignore` (gitignore syntax) before the first ingest. It keeps
build output and anything sensitive out of the index, which any connected MCP
client can query. `fathom init` writes a starter one if none exists. Re-ingest
is hash-gated, about 0.2 seconds when nothing changed.

Ingest is honest about files it could not read: they are listed with their
errno, the readable files are still indexed, and the run exits `1`. A run that
could read nothing refuses to write rather than wipe a good index. How far each
language extracts is on the generated board in
[languages.md](../languages.md).

To hide files from every answer, `fathom scope suppress <glob> --reason R`
(reversible; `scope list` and `scope unsuppress <id>`). For a leaked secret
`fathom purge <glob> --service S --reason R` is the irreversible twin.

## Wire the repository with `fathom init`

From the repository root:

```sh
fathom init
```

It is idempotent and additive; re-running changes nothing that is already
correct. It:

1. writes `.fathomignore` if absent;
2. ingests the repo into the daemon, when one is reachable;
3. adds the hooks (below) to `.claude/settings.json`, skipping any already wired
   at user scope so nothing fires twice;
4. adds a marker-delimited fathom stanza to `AGENTS.md` (created if absent;
   your own text is never touched, only the bytes between the markers). If the
   repo already has a `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md`, it
   gets one marked region holding `@AGENTS.md`, because Claude Code reads
   `AGENTS.md` natively (2.1.277 and later) only when no CLAUDE file exists.
   `init` never creates or deletes a CLAUDE file;
5. scaffolds `~/.fathom/config.toml` if absent (provider profiles, below);
6. with `--skills` (or `--skills=claude|codex|agents|all`), installs the ten
   skills into `.claude/skills/`, plus a `.agents/skills` link for other
   harnesses. Omitted by default.

It does not register the MCP server or edit `lefthook.yml`; it prints those
two steps for you to do. Every change is written as one receipt;
`fathom fleet receipts` lists them and `fathom fleet undo --last` reverses one.

Exit codes: `0` fully wired and the daemon confirms the service is indexed;
`2` wiring is complete but the daemon is unreachable or the service still shows
no symbols (re-run once the daemon is up); `1` a file `init` refused to touch,
named once at the end. A region of `AGENTS.md` you edited between the markers is
refused rather than overwritten: move your text outside the markers and re-run.
The full marker and re-run rules are in
[the internals page](claude-code-internals.md#31-the-agentsmd-stanza-and-the-agentsmd-import--markers-re-run-semantics-corruption-refusal).

Claude Code on the web (`claude --cloud`) runs in a fresh VM; the one setup
script and the owner steps are in [cloud-sessions.md](cloud-sessions.md).

## Run the daemon

Two ways to serve the tools:

| | `serve-mcp` | `serve-http` |
|---|---|---|
| Transport | stdio, one process per session | HTTP, one process, `127.0.0.1:7575` by default |
| Clients | exactly one | any number at once |
| Best for | a single session | several sessions, CI, several agents sharing one index |

If you ever run more than one Claude Code session at a time, use `serve-http`.
Two stdio servers fight for the store's single-writer lock, and every session
past the first reports `Failed to connect`.

Try it in the foreground:

```sh
fathom serve-http --store ~/.fathom/db/fathom.sqlite --addr 127.0.0.1:7575
curl -s http://127.0.0.1:7575/healthz     # ok
```

On macOS, install it as a login agent with one command:

```sh
fathom daemon install    # writes the launchd plist with your binary, store and PATH
fathom daemon status     # what is installed and whether it is serving
fathom daemon uninstall  # removes the agent, keeps your store and logs
```

The plist carries your Go toolchain on its `PATH` (and `GOPATH`, `GOCACHE`,
`GOMODCACHE`); without them the daemon indexes Go services with syntax-only call
edges. `install` warns if it cannot find `go`. On Linux, run `serve-http` under
systemd or any supervisor.

After you rebuild fathom, the daemon keeps running the old installed copy until
you redeploy it. Do not `cp` over the installed binary: that truncates a running
executable and, on macOS, voids the daemon's removable-volume permission. Use the
install path above or, in a fathom checkout, `task deploy`, which swaps the
binary, restarts the agent and asserts it serves your commit.

## Register the tools with Claude Code

```sh
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent
claude mcp get fathom      # expect: Type: http, Status: Connected
```

`--scope user` makes the tools available in every repository and to every
subagent. Add `"alwaysLoad": true` beside the entry's `url` in `~/.claude.json`
(or `.mcp.json`) so Claude Code does not defer the tool schemas behind a
search turn; `fathom plugin render` writes it already.

**Restart the Claude Code session once after registering.** Tool discovery
happens at session start, and a session that was already open will not see the
tools. This is the most common "the tools aren't there" report.

Which path to register:

| Path | Tools | Use it for |
|---|---|---|
| `/rpc/agent` | 8 | the default; what your agent should use day to day |
| `/rpc/extended` (alias `/rpc/companion`) | 21 | sessions that also need the full caller graph, staleness, ingest, repo maps |
| `/rpc` | 25 | everything, including layer compile and service rename; unauthenticated |

A call to a tool outside the registered surface still runs; it is logged as
off-surface, never silently refused. If your harness takes only a stdio command,
`fathom serve-mcp --url http://127.0.0.1:7575/rpc/agent` forwards to the running
daemon without opening a store.

## The hooks

`fathom init` writes these into `.claude/settings.json`. Each is silent when it
has nothing to say and never blocks, except where noted. A dead daemon cannot
stall a session: the session-start hook gives up after two seconds.

| Event | Command | What the agent gets |
|---|---|---|
| SessionStart | `fathom session-hook` | one line: this repo is service X, use fathom before grep, or an honest "daemon down" or "not indexed". On a fresh start or `/clear` also a short handoff brief and, if something needs attention, one attention line |
| PostToolUse (Edit, Write, MultiEdit) | `fathom edit-hook` | up to six governing rules for the file just edited, each with its source. After a `git commit`, a note if a test a governing document names was deleted or weakened |
| PreToolUse | `fathom edit-hook --pre` | the note and rules for files you marked as guarded in `.fathom/guard.cue`, before the edit. Nothing is guarded until you uncomment a scope there |
| Stop | `fathom stop-hook` | re-verifies plan rows ticked during the session; advisory, or blocking if you add `--block` |
| SessionEnd, PreCompact | `session-end-hook`, `pre-compact-hook` | nothing injected; leave a continuity note for the next session |
| InstructionsLoaded, ConfigChange and others | `session-hook --event ...`, `config audit --hook` | nothing or one line; they record that the instructions loaded and audit config changes |

The rules in the PostToolUse hook come from the `obligations` layer, which is
empty until compiled once for your service:

```sh
fathom compile --service my-service                    # dry run: sections and cost estimate
fathom compile --service my-service --dry-run=false    # needs an LLM lane, see below
```

Until then the hook stays silent, by design. If a clean hook never seems to fire,
check that the binary path inside `.claude/settings.json` still exists; hooks do
not report their own exec errors.

To silence every fathom hook at once (for debugging or an A/B), set
`FATHOM_CHANNELS=off`. `fathom doctor` warns loudly while it is set.

Per-hook behavior, the JSON for each, the clean-room option
(`--hooks-scope project`) and the seven advisory events are in
[claude-code-internals.md](claude-code-internals.md#7-the-hook-stack).

### The pre-push gate

The pre-push gate is a git hook, not a Claude Code hook, so it protects against
any author, human or agent. Add it to your `lefthook.yml` (`lefthook install`
once):

```yaml
pre-push:
  commands:
    fathom-verify:
      run: |
        [ -x bin/fathom ] || exit 0
        set --
        for f in docs/plans/*.md; do
          [ -f "$f" ] || continue
          [ "$(basename "$f")" = "README.md" ] && continue
          set -- "$@" "$f"
        done
        [ $# -eq 0 ] && exit 0
        rc=0
        for f in "$@"; do
          bin/fathom verify --service <service> --plan "$f" --quiet
          code=$?
          if [ $code -eq 1 ]; then rc=1; fi
          if [ $code -eq 2 ]; then echo "fathom verify: operational error, allowing push"; fi
        done
        exit $rc
```

`fathom verify` exits `0` when every claimed task checks out, `1` when a task
ticked `[x]` has no code behind it (the push is blocked), and `2` when the check
could not run, such as a daemon that is down (the push is allowed). The
asymmetry is deliberate: a gate that blocks when a daemon is down gets disabled
within a week. In CI, add `--strict` and treat `2` as a failure too. A plan is a
Markdown checklist; the grammar is in [plans/README.md](../plans/README.md) and
the workflow in [spec-driven-development.md](spec-driven-development.md).

## The eight tools

| Tool | Reach for it when |
|---|---|
| `find_symbol` | you know or can guess a name and want its definition |
| `symbol_context` | you are about to touch a symbol: definition, callers, callees, tests and a snippet in one call |
| `blast_radius` | you are about to change, rename or delete something and need who breaks |
| `review_diff` | you have a diff and want one cited verdict: impact, governing rules, test coverage |
| `obligations` | before editing a file, to see the rules that govern it; with no file, the repo's root conventions |
| `verify_plan` | you want the verdict on a plan's claimed rows before the push gate |
| `read_source` | you have a file and line range and want the exact current text |
| `search` | you have a literal string or log line: grep with the enclosing symbol, or ranked search over indexed docs |

The index refreshes itself: a read of a drifted service answers from what it has
and queues a re-ingest in the background.

## Skills

`fathom init --skills` installs ten skills, embedded in the binary:
`fathom-plan` (write a gate-ready plan), `fathom-verify` (run and read the
gate), `fathom-onboard` (wire a repo end to end), `fathom-doctor` (read the
wiring checklist), `fathom-rewire` (repair drifted wiring),
`fathom-activate` (turn on guards, layers and gates, each proven by
demonstration), `fathom-custodian` (keep CLAUDE.md and AGENTS.md true),
`fathom-findings` (work with an imported review kit's findings),
`fathom-feedback` (file friction the moment you hit it) and `fathom-impact`
(report measured impact honestly).

A re-run overwrites only files carrying fathom's ownership marker; a file of
your own with a skill's name stops the install with exit `1` and is never
clobbered. To ship the skills, hooks and MCP registration as one Claude Code
plugin, run `fathom plugin render --out fathom-plugin` and
`claude --plugin-dir fathom-plugin`. See
[the internals page](claude-code-internals.md#111-re-running--conflicts) for the
install shapes.

## Per-project config and provider lanes

Two optional TOML layers, both validated on load, with a bad key failing loudly:
`~/.fathom/config.toml` (your profiles, shared across repos) and
`<repo>/.fathom/config.toml` (one repo). Precedence is flags, then environment,
then project, then user, then defaults; `fathom config --explain` shows which
layer supplied each value. `.fathom/` also holds `guard.cue` (guarded edit
scopes), `capabilities.cue` (which features are on; `fathom capabilities`),
`concerns.cue` (checks that your agent config stays true; `fathom config audit`)
and `layers/` (your own semantic-layer packs). Details:
[internals](claude-code-internals.md#8-per-project-fathom).

Compiling the `obligations` layer needs an LLM. Pick a lane with `provider` in a
profile or `FATHOM_LLM_PROVIDER`:

| Lane | Auth | Cost |
|---|---|---|
| `anthropic` (default) | `ANTHROPIC_API_KEY` | metered |
| `claude-cli` | the Claude Code CLI you are already logged into | draws on your plan's limits, no API key |
| `openai` | `OPENAI_API_KEY` (optional for a local host) | metered, or free against a local model |
| `azure-openai`, `bedrock` | cloud credentials | metered through your cloud account |
| `codex-cli` | ChatGPT plan login | see [codex.md](codex.md) |

With no API key, the subscription lane is the quick path:

```toml
# ~/.fathom/config.toml
default_profile = "personal"

[profiles.personal]
provider = "claude-cli"
```

```sh
fathom providers                  # auth health, no spend
fathom smoke --profile personal   # one real, tiny completion
fathom compile --service my-service --dry-run=false
```

There is no `api_key` field; a literal key in a config file is a validation
error. Use `api_key_env` or `api_key_cmd`. The full reference is
[configuration.md](../configuration.md).

## Troubleshooting

Start with `fathom doctor`. It checks MCP registration, daemon reachability,
binary freshness, that the hooks are installed and firing, the `AGENTS.md`
stanza and whether Claude Code loads it, the skills, config validity, and that
obligations are being served. Each row is pass, fail or skipped, with a
one-command remedy on every failure. Exit `0` is all clear, `1` a real problem,
`2` doctor itself could not run. Options: `--harness=claude|codex|auto`, `--json`.

| Symptom | Cause and fix |
|---|---|
| Tools don't show up | The session started before you registered. Restart it once; confirm with `claude mcp get fathom` |
| `Failed to connect` in `claude mcp list` | More than one session on stdio `serve-mcp` is fighting for the store lock. Use the `serve-http` daemon |
| Answers look old | Two kinds. The index behind the working tree refreshes itself on the next read when git HEAD moved (`staleness_report` shows it; uncommitted edits are not covered, so read the file). The daemon running old fathom code needs a redeploy; compare `fathom version` to `git rev-parse HEAD` |
| `ingest` says `degraded` or `refused` | The named paths could not be read. If `stat` works but `cat` fails on an external volume, it is a macOS permission wedge: run the `tccutil reset` command the error prints, then re-ingest. A refused run wrote nothing, so the old index is intact |
| A hook fires but injects nothing | For `edit-hook`, the `obligations` layer is not compiled yet (see above). For any hook, check the binary path in `.claude/settings.json` |
| `verify` fails every task | Check the service has symbols at all (`list_services`). An empty index is an operational error (exit `2`), not a wall of false claims |
| The stanza is present but Claude ignores it | Doctor's `stanza_present` row says why: a CLAUDE file without the `@AGENTS.md` import, an instruction-file setting that excludes `AGENTS.md`, or a Claude Code older than 2.1.277 |

More entries are in [the internals page](claude-code-internals.md#13-troubleshooting).

## You know it works when

```sh
fathom version                               # matches: git rev-parse HEAD
curl -s http://127.0.0.1:7575/healthz        # ok
claude mcp get fathom                        # Type: http, Status: Connected
```

Then, in a restarted Claude Code session:

- [ ] The session lists the eight tools.
- [ ] `find_symbol` on a symbol you know exists returns a hit.
- [ ] After compiling `obligations` once, editing a governed file shows a
      `fathom obligations` block in the session's context.
- [ ] `fathom verify --service <svc> --plan docs/plans/<file>.md` exits `0` on a
      clean plan, or names the unproven claim.
- [ ] `git push` runs the `fathom-verify` hook.
- [ ] `fathom doctor` reports every row `pass`. Faster than the list above:
      just run it first.

The daemon also serves a read-only observatory at
`http://127.0.0.1:7575/observatory` (fleet, per-service, impact and brief pages).
If every box is checked, the loop (index, serve, inject, gate) is live.

## Going further

- Friction capture and measurement (`fathom observe`, `fathom instrument
  --adoption`, `fathom flywheel`): [internals](claude-code-internals.md#12-the-flywheel-observe--instrument---adoption--fathom-feedback).
- Continuity notes between sessions (`fathom thread note`): [internals](claude-code-internals.md#77-sessionend--precompact--continuity-threads).
- Contributing to fathom itself: [usage.md](../usage.md#contributing).

## Maintained

This page covers `fathom init` (with `--skills`), `ingest`, `scope`, `purge`,
`serve-mcp`, `serve-http`, `daemon`, `compile`, `verify`, `doctor`, `providers`,
`smoke`, `config`, the `/rpc`, `/rpc/agent` and `/rpc/extended` surfaces, the
Claude Code hooks, the lefthook pre-push gate and the ten shipped skills. Update
it, and `claude-code-internals.md`, when any of these change.

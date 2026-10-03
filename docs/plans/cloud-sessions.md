# Cloud sessions: the plan (owner-architect decisions C1–C4, E1–E3, W1 of 2026-10-02)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/cloud-sessions.md` proves it. Owner steps and what runs where:
`docs/adoption/cloud-sessions.md`. Decision: ADR-059's amendment of 2026-10-02 (import-only CLAUDE.md files);
the rest is configuration and one CLI flag, below the ADR threshold. Platform facts: `docs/intent/domain.md`
(cloud sessions, read 2026-10-02).

Shape: one committed setup script shared by Claude Code on the web and Codex cloud provisions the VM (tools at
pins read from one file, `bin/fathom`, a pre-built index inside a 270-second budget); every session starts its own
daemon on 127.0.0.1:7575 because a cache keeps files, never processes; the repository's hooks talk to that daemon
and the harness reaches it through a stdio front, so tools and hooks read one store with one writer. Nothing a
local session reads changes behavior: the SessionStart wrapper exits at once off the VM, the five template hooks
it now carries are byte-identical to the user-scope ones (Claude Code runs a handler defined twice once), and no
`.mcp.json` is added.

## Build

- [x] `scripts/cloud-setup.sh` provisions a cloud VM: go.mod's toolchain through `GOTOOLCHAIN=auto` (or the toolchain zip from proxy.golang.org when no go exists, never dl.google.com), go-task, cue and lefthook by `go install` at their pins beside the CGO build of `bin/fathom`, the pre-ingest stopped at the budget, the MCP server registered in the VM's own user scope, one summary line with timings, exit 0 on any failure {id: CS-1, oracle: lexical, path: scripts/cloud-setup.sh, pattern: "^export GOTOOLCHAIN=auto"}
- [x] The tool pins live in one plain KEY=value file a generator can write (env-kit's agent-cloud substrate, E2) {id: CS-1b, oracle: lexical, path: scripts/cloud-tools.env, pattern: "^CUE_VERSION=v0\.16\.1$"}
- [x] The setup and session scripts are tested under fakes for go and fathom (pins installed as named, a failed tool named and exit 0, the re-run cheap, the budget deferral, a local session silent, the daemon flags, the remap, the unprovisioned VM), in `task scripts-check` {id: CS-1c, oracle: lexical, path: Taskfile.yml, pattern: "bash scripts/cloud-setup_test\.sh"}
- [x] `fathom serve-mcp --url` is a stdio front for a running serve-http daemon: no store opened, each line forwarded verbatim, the negotiated protocol revision carried, a notification answered with nothing, every request with an id answered even when the daemon is down {id: CS-2, symbol: runServeMCPFront, package: cmd/fathom, reachable_from: TestServeMCPURLForwardsToTheDaemon}
- [x] The SessionStart entry wraps `fathom session-hook` in `scripts/cloud-session.sh hook`: in a cloud VM it starts the daemon (remapping a moved clone first), exports PATH through CLAUDE_ENV_FILE and installs lefthook before the primer runs; off the VM it exits silently {id: CS-3, oracle: lexical, path: .claude/settings.json, pattern: "cloud-session\.sh.{0,4} hook fathom session-hook"}
- [x] `cloud-session.sh` exits before any work when `CLAUDE_CODE_REMOTE` is not `true` {id: CS-3b, oracle: lexical, path: scripts/cloud-session.sh, pattern: "exit 0 # a local session: nothing to do, nothing printed"}
- [x] Every `task validate` step except `task test` runs on Linux. `store-purego`'s `-race` under `CGO_ENABLED=0`, which Go supports only on macOS, runs without `-race` there. The gate-lease test caps nice at Linux's 19. macOS is unchanged, and every other macOS-only call is in `task deploy` {id: CS-4, oracle: lexical, path: Taskfile.yml, pattern: "uname -s.*= Darwin"}
- [x] `agents_md_reaches_claude` gains `import_only: true`: a CLAUDE.md holds only as exactly `@AGENTS.md` beside an AGENTS.md, every nested AGENTS.md needs one once the root exists, `.claude/CLAUDE.md` and `CLAUDE.local.md` are refused {id: CS-5, symbol: checkAgentsMDImportOnly, package: internal/custodian, reachable_from: TestAgentsMDImportOnly}
- [x] This repository's root CLAUDE.md is the one line `@AGENTS.md` (ADR-059 amendment 2026-10-02) {id: CS-5b, oracle: lexical, path: CLAUDE.md, pattern: "^@AGENTS\.md$"}
- [x] Each nested guide has its import, so a root CLAUDE.md strands none {id: CS-5c, oracle: lexical, path: internal/CLAUDE.md, pattern: "^@AGENTS\.md$"}
- [x] W1: the five template hook entries that lived only in the owner's user settings (the PostToolUse edit hook, the Grep/Glob/Read pre-hook, Stop, SessionEnd, PreCompact) are in the repository's settings, where a cloud session reads them; with an empty HOME `fathom fleet reconcile` reads the repo's wiring current {id: CS-6, oracle: lexical, path: .claude/settings.json, pattern: "fathom pre-compact-hook"}
- [x] The owner guide carries the environment steps, the setup script to paste, Codex cloud, the limits and the measured timings {id: CS-7, oracle: lexical, path: docs/adoption/cloud-sessions.md, pattern: "^### 1\.1 The setup script to paste$"}
- [x] README and the adoption start page link it {id: CS-7b, oracle: lexical, path: docs/adoption/README.md, pattern: "cloud-sessions\.md"}

- [x] `task test` passes on Linux as root, the cloud VM's user, in an ubuntu:24.04 root container with the VM's 4 vCPU and on the macOS gate. A test that stages an unreadable file now injects the refusal through `internal/fsfault`, because root reads a mode-000 file. Where the read is not fathom's own (git, the kit's read manifest, a SQLite open in a child process), the test stages a failure root also gets and keeps its chmod variant for other users. The two macOS assertions (signing identity, launchd restart hint) carry Linux equivalents. Four faults the container exposed were fixed in the product: the pre-push verify body is POSIX sh, because lefthook's `sh` is dash there; a held port's holder is read from /proc when there is no lsof; the toplevel cache checks `<top>/.git` itself, because Linux reuses inodes; and an abandoned grammar module's late reap no longer counts against a later leak check. The grandchild-kill test counts a zombie as gone. Load flakes on the 4-vCPU container were fixed in the tests: an ETXTBSY write, two console fixture races, and a SIGPIPE in a script test {id: CS-8, oracle: lexical, path: docs/bench/readings/cloud-sessions/first-session.md, pattern: "^CS-8 reading: task test green on Linux as root \(ubuntu:24\.04, 4 vCPU"}

## Readings (owner actions)

- [ ] (op) CS-M1: a real `claude --cloud` session in the `fathom` environment: the setup summary line and its seconds, `/mcp` lists fathom (the VM-written user-scope registration is honored, or the doc's fallback is taken), the primer is live, one edit gets obligations, and a second session starts from the snapshot {id: CS-M1, oracle: lexical, path: docs/bench/readings/cloud-sessions/first-session.md, pattern: "^CS-M1 reading:"}
- [ ] (op) CS-M2: `task validate` green inside a cloud session on the branch it pushes (the Linux gate end to end) {id: CS-M2, oracle: lexical, path: docs/bench/readings/cloud-sessions/first-session.md, pattern: "^CS-M2 reading:"}
- [ ] (op) CS-M3: a Codex cloud task with the same setup script: the summary line, `cloud-session.sh ensure`, and whether its MCP servers load {id: CS-M3, oracle: lexical, path: docs/bench/readings/cloud-sessions/codex.md, pattern: "^CS-M3 reading:"}

## Deferred

- [-] env-kit's agent-cloud substrate generates `scripts/cloud-tools.env`, the required hosts and the budget check from env.cue {id: CS-D1, disposition: deferred, reason: "owner decision E2: the hand-written script works first and the substrate is built in platform-engineering after it", trigger: "env-kit ships an agent-cloud substrate"}

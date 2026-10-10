# Domain: platform facts

Part of the domain router, `../domain.md` (the vocabulary and the section index).

## Platform facts

- macOS TCC removable-volume and Full Disk Access grants bind to code identity; an ad-hoc
  signed Go binary changes cdhash every build, so a redeployed launchd daemon silently
  loses the grant (crash-loop or an unpromptable hang). Consequence: `task deploy` signs
  with the persistent `fathom-dev` identity. Source: `docs/dogfood-observations.md` O10, `Taskfile.yml`.
- `cp` over a running executable truncates it in place; an unsigned copy is refused
  (`OS_REASON_CODESIGNING`). Consequence: temp path + rename, then a serving assertion.
  Source: O25, `Taskfile.yml` deploy.
- launchd `KeepAlive` means the daemon exiting *is* a restart. Consequence: the daemon
  self-adopts a verified newer binary (AU-1..3). Source: `CHANGELOG.md` 2026-08-12.
  `KeepAlive true` relaunches every exit, including exit 0 and a crash loop (5 relaunches in
  17 min, INC-23). `{SuccessfulExit: false}` relaunches only a non-zero exit or a signal
  death, so exit 0 means stay down. Consequence: plist schema 2, `ThrottleInterval 60`,
  self-restart and over-budget exit non-zero, and breaker and kill-switch stops exit 0.
  Source: ADR-047, design §8.1.
- Memory instruments measure different things. Go's `HeapSys` never falls for the life
  of the process, so it cannot show a bound holding. RSS (`ps`, `ri_resident_size`) leaves
  out compressed pages, and it counts `MADV_FREE`'d pages until the kernel reclaims them.
  `phys_footprint` (`ri_phys_footprint`) is the number Activity Monitor and jetsam use.
  Consequence: the budget, the ladder, `fathom resources` and doctor read `phys_footprint`,
  self plus children (I47). Source: INC-24, `.fathom/observations.md` O108.
- jetsam kills the largest processes under memory pressure, and they may be neighbours
  rather than the culprit. `runtime/debug.SetMemoryLimit` is a GC hint that never stops
  the process: the daemon reached about 29 GB under a 3 GiB limit. Consequence: the daemon
  enforces its own budget with an exit mark. Source: INC-23, `JetsamEvent-2026-09-26-*.ips`.
- macOS grows swap files on demand, so the used/total swap fraction is routinely high at
  normal pressure (4.9 of 6.1 GB at pressure level 1), which makes that fraction meaningless.
  A full swap volume turns the store's writes into `SQLITE_FULL`/`SQLITE_IOERR`.
  Consequence: the host triggers are pressure level, swap growth and swap-volume free
  space, never the used fraction, and SQLITE_FULL/IOERR is store distress that refuses
  writes by name (I38). Source: ADR-047 amendment, design §2.1.
- `HardResourceLimits.ResidentSetSize` is advisory on macOS, not enforced. `ProcessType
  Background` puts the daemon in background QoS, and its `git`/`go`/`rg` children inherit
  that class. Consequence: no plist RSS limit is relied on, and Background is on
  probation against hook latency (RT-M6). Source: design §8.1.
- `.gitignore` is the repository's own statement of what is cache: a Go build cache or a
  cargo registry inside a work tree can be gigabytes of files nobody asked about.
  Consequence: discovery can take its file list from `git ls-files -co --exclude-standard`
  (`respect_gitignore`, off until RT-M11 measures it), and the default cache directories
  are always pruned. Source: vision 36 §2, RT-145.
- Reads under an external volume can fail `EPERM` for a whole process tree while `stat`
  succeeds (a wedged TCC verdict). Tools then emit false signals: an empty index reporting
  "0 failed", `gofmt -l` naming an innocent file, "88 of 88 tools zero-call".
  Consequence: read errors are classified and named, never rendered as zero (IT-1);
  the reset is `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`.
  Source: `.fathom/observations.md` O16, O17.
- Claude Code's `Stop` event fires per turn, not per session; 200 ledger rows were 8
  sessions. Consequence: denominators count distinct session ids. Source: `CHANGELOG.md` 2026-08-09 (MI-1..3).
- Claude Code `SessionStart` carries a source (`startup|resume|clear|compact`); the brief
  prints only for startup and clear. Codex has no SessionStart hook, so `fathom thread
  brief` is instructed instead. Source: `CHANGELOG.md` 2026-08-13 (TH), `AGENTS.md`.
- Go map iteration is randomized. Consequence: candidates are sorted before matching;
  determinism is a tested contract. Source: `.fathom/observations.md` O1, `internal/AGENTS.md`.
- SQLite WAL gives many cross-process readers beside one writer; DuckDB's single writer
  forced daemon-stop rituals and made daemon wedges total outages. Source: ADR-020.
- The Go linker writes to `TMPDIR`, not `GOTMPDIR`; scratch inside the module makes
  `go vet` walk leftover build dirs and puts every `t.TempDir()` inside a git repo.
  Consequence: both point at a scratch dir on the roomy volume, outside the module.
  Source: `Taskfile.yml` env comment.
- lefthook buffers each step's output until it finishes, so a 22s advisory review reads as
  a hang and scripted pushes were killed at 60–90s. Consequence: `review --budget-seconds 20`. Source: O9.
- `git archive` embeds the commit timestamp in every tar entry, so identical trees hash
  differently as tars. Consequence: the vendor drift guard hashes per-file content. Source: ADR-019.
- Tree-sitter grammar `.wasm` blobs target the Emscripten ABI, not bare wazero; the working
  path compiles tree-sitter's own core plus one grammar to wasm32-wasi. Source: ADR-013, ADR-021 §4.
- A typed-judgment model (Jev 1.13 through the `typesafe` provider) answers choice, score and noul questions over
  one state. It cannot generate text, cannot count or compare dates, does not follow indirection and is weak on
  negation. Limits: 64k tokens per request (32k for the state plus the longest question), 20 requests per second
  binding before tokens do, no multi-state batch endpoint; measured 421 to 587 ms at about 22k tokens of state
  (130 to 375 ms for small states); $0.042 per million input tokens, output free. An llm lane costs 17 to 41
  times more per call and took 1.5 to 2.8 s (3 of 6 calls timed out at 3 s). Consequence: judged sites are
  text-shaped and small-unit, fan out one row per call, and the llm lane serves the CLI only. Source: ADR-042
  amendments, ADR-056, `docs/design/judgment-foundations.md` section 4.
- In Claude Code a hook cannot trigger compaction, PreCompact can block but cannot inject context, only
  SessionStart with source `compact` can add context afterwards, and a system message from UserPromptSubmit or
  Stop reaches the user only. Consequence: carry-across is the only fathom-side compaction piece; acting on
  "compact now" needs a harness that owns compaction. To verify against the current version (Q32). Source:
  `docs/design/carry-across.md` section 1.
- Claude Code cloud sessions (claude.ai/code, `claude --cloud`; read 2026-10-02) run a fresh Ubuntu 24.04
  x86_64 VM (4 vCPU, 16 GB) on a fresh clone: the repository's `.claude/settings.json` hooks, skills, `.mcp.json`
  and CLAUDE.md files load (single-repo sessions only); user `~/.claude/settings.json` and user-scope MCP servers
  from the owner's machine do not. A per-environment setup script runs as root before Claude Code launches and
  its filesystem (never a running process) is snapshotted for about seven days when it finishes in about five
  minutes; the snapshot is rebuilt only when the environment's script text or allowed hosts change (a committed
  file the script runs does not), and it serves every repository a session in that environment opens.
  `CLAUDE_CODE_REMOTE=true` marks the VM. Identical hook handlers defined in two settings files run once.
  In Claude Code's default instruction mode a CLAUDE.md in the working directory or above switches AGENTS.md off,
  nested ones included: a nested AGENTS.md then loads only through a CLAUDE.md beside it. Consequence: cloud
  wiring lives in the repository (ADR-059 amendment 2026-10-02, `docs/adoption/cloud-sessions.md`). To verify in
  a real cloud session: whether a VM-written `~/.claude.json` server is honored (CS-M1). Source:
  code.claude.com/docs/en/cloud-environments, /settings, /hooks, /memory.

### `doctor`

Deterministic wiring-diagnosis checklist for a fathom-adopting repo — the one-command
answer to "is fathom actually wired here, end to end?" Every row is `{name, status,
reason, remedy}`; `status` is `pass`, `fail`, `warn`, `skipped`, `unknown`, or `timeout`.
`unknown` names wiring that cannot be decided from readable configuration, and
`timeout` names a probe whose deadline expired. Neither is a pass or a confirmed
failure. Other inapplicable or uncheckable rows remain `skipped` with a reason. `remedy` is a single command, present on every `fail`. `doctor` is
strictly read-only: it never writes a file, calls a mutating MCP tool, or restarts
anything.

```sh
fathom doctor [--harness claude|codex|gemini-cli|cursor|copilot|agents|auto] [--path <dir>] [--service <id>] [--url <base>] [--json] [--repo-only]
```

| Flag | Default | Description |
|---|---|---|
| `--harness` | `auto` | `claude`, `codex`, any other registry id (`fathom harness list`), or `auto` (detects from each descriptor's presence markers in registry order — `.claude/settings.json`/`~/.claude.json`, `.codex/config.toml`, `.gemini/settings.json`, … — defaulting to `claude` with no evidence). A non-Claude harness adds the `harness_hooks` row, which reads the descriptor's hook files and reports `skipped: unverified` — never PASS |
| `--path` | cwd | Repo root to diagnose |
| `--service` | resolved from `--path` | Service ID — the same identity `fathom init` resolves |
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--json` | off | Machine-readable report instead of the human table |
| `--repo-only` | off | Only the eleven rows that read the repository's own files and the user scope's config — `hooks_installed`, `lefthook_gates`, `harness_hooks`, `stanza_present`, `skills_installed`, `wiring_drift`, `config_valid`, `binding_file`, `guard_file`, `guard_include_modules_unbound`, `user_scope_ignore` — no daemon, harness process or ledger; the Observatory's project wiring tab renders it. Exit codes unchanged |

Wiring and execution are separate evidence states:

- `configured`: a hook command or MCP entry was found in configuration.
- `configured (indirect)`: a project wrapper contains the fathom invocation.
  Doctor reads scripts and the selected Task or lefthook entry, at most two
  wrapper levels and 64 KiB total per command inspection. It never executes them.
  Dynamic commands, unreadable wrappers and deeper chains report `unknown`.
- `firing`: the separate `hooks_firing` row has execution evidence from the
  existing call or hook log. A configured command alone never establishes this.
  `surface_traffic` independently reports observed MCP usage.

Claude project MCP entries in `.mcp.json` and stdio entries using project wrappers
are recognized, alongside the existing harness configuration files. Recognition
establishes configuration only, without launching the stdio server.

Each step inherits the Setup hook's remaining deadline when doctor runs as a
hook. Standalone doctor has no overall run deadline. Daemon requests and doctor's
git probes retain their five-second caps. CLI pings retain their own budgets
(Codex construction: 20 s; other CLI pings: 15 s). Existing shorter request caps
remain in effect. Local work without cancellation, including the retention walk
and Observatory projection, runs outside the per-probe cap and keeps its verdict.
A daemon that does not answer within 5 s remains a confirmed FAIL. The brief names
FAIL, TIMEOUT and UNKNOWN rows with separate counts, so a hang cannot look clean.
An expired run stops before later rows and names the checks not run. JSON retains
`unknown`/`timeout` on the individual rows, and counts both under `summary.skipped`
for compatibility. Human output labels them `UNKNOWN` and `TIMEOUT`.

The founding eleven checks: `mcp_registration`, `daemon_reachable`, `binary_fresh` (the running
binary's own build vs `git rev-parse HEAD` of its own source tree — the LOCAL binary only,
and the row says so; a dirty build surfaces `vcs.modified`, and both linked-worktree
caveats — mismatch inside one, stamp-poisoning by ones nested under the checkout — are
printed only when verified to apply), `daemon_fresh` (PC-4: the daemon's `/statusz`
`binary_commit` against this binary's own stamp — PASS on the same revision, FAIL naming
both, SKIP with the reason for a daemon that predates `/statusz` schema 2 or an
unstamped binary; never compared to the repo's HEAD), `hooks_installed` (any settings scope: project `settings.json`,
`settings.local.json`, or user `~/.claude/settings.json`; when nothing readable wires
them but the call log shows them firing, the verdict names that contradiction instead
of a duplicate-installing remedy) / `hooks_firing` (both Claude Code only — Codex
reports `skipped`, honestly, since it has no hook mechanism), `lefthook_gates`
(harness-agnostic), `stanza_present` (ADR-059: FAIL when `AGENTS.md` lacks a
well-formed fathom region; WARN, with the remedy, when Claude Code will not load
it — a root CLAUDE file without an `@AGENTS.md` import or still carrying the old
stanza, a CLAUDE file in a parent directory, the agents-md plugin's
`instructionFiles` setting at `claude-md` or `managed-only` in managed or user
settings (`cc-plugin-agents-md@builtin` since Claude Code 2.1.285,
`agents-md@builtin` from 2.1.277; both read, a disagreement named), or `claude
--version` before 2.1.277; those machine-side remedies name only the native
path, never a CLAUDE import; PASS names the load path),
`stanza_loaded` (the harness-side twin of `stanza_present`, PC-25: PASS when an
`InstructionsLoaded` beat names this repo's `AGENTS.md` or a root CLAUDE file,
SKIP naming the wiring date when none does, and SKIP by name in a repository with
no root CLAUDE file, where Claude Code loads `AGENTS.md` without the event, or with
`stanza_present`'s warning and remedy when that warns Claude Code will not load it — never
FAIL), `skills_installed` (resolving the `.agents/skills` link), `wiring_drift` (PC-29: hooks,
stanza regions, each harness's MCP server entry — argv and env/header NAMES, never values —
and `fathom:owned` skill files against the digests in `.fathom/owned.json`; PASS at zero
drift naming any entry without a baseline yet, FAIL naming each drift by kind and path,
SKIP when a file could not be read), `config_valid`,
`obligations_served` (service resolved the way the hooks resolve it — every candidate
identity intersected with the daemon's service list, with a distinct not-indexed
verdict and drift-flagged rules counted into the reason), `surface_traffic`
(process-lifetime counters — "since this daemon process started"). Later rows:
`capabilities` (red only when something switched on is failing to deliver), `concerns`
(the custodian — a load error or a violation is the only red), `user_scope_ignore`,
`user_scope_guard`, `guard_file` / `guard_dead_scopes` / `guard_include_modules_unbound`,
`project` (every `.fathom/*.cue` key with its file, status and entry count; a key fathom does not
read warns; a file that does not parse, a key its reader refuses or two files setting one key fail),
`service_roots` (EW-20, O127: `list_services`' recorded root for every registered service is stat'ed; a
root that no longer exists is named with the `ingest` call that re-points it, `{"service":…,"path":…,"remap":true}`
— FAIL when it is this repository's own service, WARN for another's; a root that exists but cannot be read, such as
a wedged external volume, is named unreadable with no re-point offered; SKIP when the daemon names no roots),
`binding_file` / `binding_modules_mounted` / `binding_module_readiness` (SKIP names the real
reason: `list_modules` was removed by SR-17, so a current daemon never advertises it), `binary_on_path`,
`channels_env`, `ident_cache_env`, `metrics_tier`, `session_ledger_tokens`,
`cpp_toolchain` (CMake repos: `compile_commands.json` present), and
`decision_provider` (ADR-042, docs/vision/32-decision-port.md: `[decide] provider =
none` is `skipped` naming it off; `llm` pings the configured profile lane and, when
`decisions.jsonl` exists, names its last recorded `latency_ms`; any other value is a
`fail`), and `providers` (surface reckoning SR-31/SR-32: `fathom providers`'s own
zero/near-zero-token `ProbeAll` pass folded in — `skipped` naming it when no
`[profiles]` are configured and the bare env-overlay lane has nothing to probe,
`fail` naming the first broken lane when a configured profile cannot authenticate,
`pass` counting ok/not-probeable lanes otherwise; `providers` and `smoke` are hidden
`fathom help` aliases now — `smoke`'s real-token spend is never run here). Three rows
name the compile path's silent failures (GL-27): `profile_bins` (`fail` when a
configured profile's `bin` does not exist, naming the path, the profile, a same-named
binary now on PATH and the fix, editing `[profiles.<name>] bin` in
`~/.fathom/config.toml`; only the path is stat'ed, no binary runs), `compile_layer`
(`fail` when the daemon started with a configured provider it could not build, read
from `status(section: "daemon").compile_layer`; `skipped` when nothing is configured),
and `layers_stale` (`warn` per service when the obligations layer has fewer sections
compiled than total, naming the `--dry-run` compile first: a spending compile needs
owner approval). The stanza
check accepts both the v1 and the v3 (`sha256:`-digested) begin marker.

The tenancy rows (RT-49, RT-50; docs/design/responsible-tenant.md §13.3) read one
`fathom resources` projection — the daemon's live `/statusz` tenancy block, else its
resource ring and lifecycle ledger — and WARN, never FAIL: `daemon_footprint`
(phys_footprint plus children against the budget the daemon runs under; a daemon that
does not answer is a WARN naming the ring's last sample and its age, never a SKIP, and
the remedy names the level, the top in-flight tool and `fathom resources`, never a
restart), `daemon_lifecycle` (starts, unclean and over-budget exits, crash-looping at 3
counted exits in 30 min, the switch files, quarantined services; a ledger with no start
rows — every build before RT-37 — says starts and unclean exits are not recorded and that
crash-loop detection sees over-budget exits only, never "0"), `host_pressure` (pressure,
swap, swap-volume free against the resolved `[tenancy.host]` triggers; unknown pressure is
disclosed, and a ring sample older than 5 min is SKIPped with its age, never judged as
now), `scratch_daemons` (leaked unmanaged daemons — exactly the set `fathom daemon reap
--apply` kills, so its remedy clears the row — and stale records) and `tenancy_scope`
(WARNs on a repository `[tenancy]`, which is ignored and named, on admission `off`, and on
a `memory_limit` / `gc_hint` conflict; env overrides disclosed). Two more rows judge what
the hooks and the disk cost (RT-135, §11.3, §12.1), also WARN, never FAIL: `hook_budget`
(each channel's p99 wall time against its deadline over 24 h, from
`~/.fathom/hook-latency.jsonl`, which every hook invocation appends to, with its CPU time
as `cpu_ms` where the platform reports it (EW-29) — a channel whose rows carry it also
names `p50 wall … cpu …` beside n, `of N timed` when only some do, so a slow hook reads
as work or as waiting; minutes the
shared breaker held hooks off; edits served while it was open, whose guards were skipped;
harness kills — an edit-hook `seen` beat never answered — read from the beats of the whole
24 h across `hooks.jsonl` and `hooks.jsonl.1`; when the retained beats end sooner, the row
names the span they cover) and `disk_retention` (`fathom
gc`'s dry run: reclaimable bytes by class and the largest classes, what it only reports,
and the home volume's free space; WARN above 512 MiB reclaimable). `fathom resources`'
hooks line reads the same latency log, the CPU medians included.

Exit `0` no confirmed failure (including runs with `unknown`, `timeout`, `warn` or
`skipped` rows); `1` at least one check failed; `2` doctor itself could not
run (bad flags, unresolvable `--path`) — an operational error, not a verdict on the
repo's wiring. See the `fathom-doctor` skill for interpretation, the remedy-approval
discipline, and the tie-in/latency-budget checks doctor itself doesn't run.

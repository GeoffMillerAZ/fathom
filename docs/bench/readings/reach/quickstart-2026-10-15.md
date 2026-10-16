# README quick start reading, 2026-10-15

**Verdict.** The README quick start works as written on a clean clone of v0.11.0
(99aa205a). The first answer from the index arrived about 38 seconds of machine time
after the clone began, and 28.6 seconds of that was the build on a cold Go cache.

## Setup

- Machine: Apple M4, 10 cores, 32 GiB, macOS 27.0.1, Go 1.26.3.
- A fresh `git clone` of the public repository at 99aa205a, built with empty `GOCACHE`
  and `GOMODCACHE`, `GOWORK=off`, `CGO_ENABLED=1`, `MACOSX_DEPLOYMENT_TARGET=14.0`.
- A temporary `HOME` and a non-default port. Everything else matched the README
  commands, including `FATHOM_TENANCY_BUDGET=2GiB` and
  `--idle-exit=0 --ttl=0 --auto-refresh=true` on `serve-http`.
- Target repository: a shallow clone of psf/requests at 611c6162 (130 tracked files).
- The script drove each step with a subprocess and timed it by wall clock. The tool
  calls went to `/rpc/agent` over HTTP, as `curl` would send them.

## Timings

| Step | Seconds |
|---|---|
| clone fathom | 6.82 |
| build (cold cache) | 28.63 |
| `fathom version` (first run of a new binary) | 0.68 |
| daemon ready (`/healthz` ok) | 0.13 |
| clone psf/requests | 1.25 |
| `fathom init --skills` | 0.55 |
| `find_symbol merge_setting` | under 0.01 |
| `symbol_context merge_setting` | 0.02 |
| `blast_radius src/requests/utils.py` | under 0.01 |
| step 2 demo: `fathom ingest` | 0.05 |
| step 2 demo: `fathom verify` | 0.03 |

An earlier run the same day, without the budget variable and the lifetime flags, read
6.16 s for the clone, 28.47 s for the build and 0.55 s for init.

## What came back

- `init` printed `ok ingest (service "requests" indexed: 1497 symbols, python)` and a
  note that `scip-python` is not on `PATH`. Its output ran to 66 lines, most of them
  optional `MANUAL:` steps.
- `find_symbol` matched exactly: `src/requests/sessions.py:76`, tier `verified`.
- `symbol_context` listed three callers: `Session.merge_environment_settings`,
  `Session.prepare_request` and `merge_hooks`. It also returned
  `"resolution":{"typed":0,"heuristic":5}` and the indexed commit.
- The step 2 demo printed `3 tasks total; 3 claimed: 1 verified, 2 failing` and exited
  1, naming `test_reaches` for D-2 and `symbol_present` for D-3.
- `fathom doctor` read 58 checks: 36 pass, 1 warn, 2 fail, 19 skipped. The two FAIL rows
  were `lefthook_gates` (the target has no `lefthook.yml`) and `obligations_served`
  (nothing compiled yet). The WARN was `daemon_footprint`, unknown seconds after start.

## What it wrote

- In the target repository: `.fathomignore`, `.fathom/` (five commented examples and
  `owned.json`), `AGENTS.md`, `.claude/settings.json`, ten skills under
  `.claude/skills/`, and `.agents/skills` as a link to them.
- In the temporary home: `~/.fathom/` (the store, logs, config, the verify ledger), a
  shim at `~/.local/bin/fathom`, and `~/.local/state/fathom/`.

## Stranger check

The same day, `bash scripts/stranger-check.sh` passed (exit 0) on 99aa205a plus this
change's documentation edits. It copies the tracked tree, builds it with `GOWORK=off`,
starts a throwaway daemon with a temporary `HOME`, and runs
`fathom init --harness=none --skills` over the whole copy. It then ingests and verifies a
fixture, makes one `tools/call` on `/rpc/agent`, and fails on any unexpected doctor FAIL
row.

| Step | Seconds |
|---|---|
| build | 27.59 |
| daemon ready | 0.95 |
| init over fathom's own tree | 175.21 |
| fixture ingest | 0.09 |
| first proven answer (fixture verify) | 0.05 |
| **first proven answer, total** | **3 min 23 s** |

The script's whole run took 212 s. The previous committed reading
([stranger-2026-09-28.md](stranger-2026-09-28.md)) was 2 min 12 s, with init at 110 s,
measured at an earlier commit (2bb0c6ef).

## Not covered

`claude mcp add` and a live Claude Code session were not run here, because they write
the user's Claude Code configuration. The Linux build was not timed in this reading.

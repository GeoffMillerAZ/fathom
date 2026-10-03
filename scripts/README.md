# scripts/

Helper scripts for the fathom project.

## cloud-setup.sh, cloud-session.sh, cloud-tools.env

A cloud session VM (Claude Code on the web, Codex cloud):

- `cloud-setup.sh` provisions it once per environment snapshot. It installs
  the toolchain and the tools pinned in `cloud-tools.env`, builds
  `bin/fathom`, pre-builds the index and registers the MCP front.
- `cloud-session.sh` runs per session: `hook` is the SessionStart wrapper,
  `ensure` starts the VM-local daemon, and `mcp` is the registered stdio
  command.
- `cloud-setup_test.sh` tests both under fakes.

Owner steps: `docs/adoption/cloud-sessions.md`.

## fathom-adoption.sh

Measures Claude Code dogfood adoption: for each project's sessions it counts
fathom MCP tool calls vs. grep-style searches (Grep tool + `grep`/`rg` in Bash),
then reports pre/post hook-install totals and a per-ISO-week baseline.

```bash
bash scripts/fathom-adoption.sh fathom
bash scripts/fathom-adoption.sh app-kit throughline fathom
HOOK_DATE=2026-06-17 bash scripts/fathom-adoption.sh
```

Read-only — only parses `~/.claude/projects/*/*.jsonl` and
`workspace/logs/mcp-calls.jsonl`. Nothing is written.

### What it does

1. For each requested project name, glob-matches transcript dirs under
   `~/.claude/projects/` using substring matching plus an alias map — `fathom`
   also matches dirs containing `sextant` (and vice versa) to handle the
   sextant→fathom repo rename.
2. For each session file, counts fathom MCP calls (`mcp__fathom__*` /
   `mcp__sextant__*`) and grep calls (Grep tool or `grep`/`egrep`/`rg` in Bash).
3. Splits results at `HOOK_DATE` and shows Δ share before/after.
4. Appends a per-ISO-week rollup table so a weekly adoption baseline is readable
   directly.
5. Prints a daemon call-log summary (total calls, by surface, by via, top 10
   tools) from `workspace/logs/mcp-calls.jsonl`. If the file is absent, one line
   says so; the section never fails the script.

### Environment variables

| Variable          | Default        | Description                                   |
|-------------------|---------------|-----------------------------------------------|
| `HOOK_DATE`       | `2026-06-17`  | Hook-install date; sessions on/after = post.  |
| `FATHOM_CALL_LOG` | `workspace/logs/mcp-calls.jsonl` | Override daemon call-log path. |

---

## ci.sh

Runs the full validation gate and prints a compact pass/fail summary.

```bash
./scripts/ci.sh
```

### What it does

1. Verifies that the `task` CLI is available (see [Taskfile.dev](https://taskfile.dev/installation/)).
2. Changes to the repository root (parent of `scripts/`).
3. Runs `task validate`, which executes in sequence:
   - `fmt-check` — fails if any Go file is not `gofmt`-clean.
   - `vet` — runs `go vet ./...`.
   - `cue-vet` — validates CUE schema files (skipped when `cue` is not installed).
   - `test` — runs `go test -race ./...` (requires `CGO_ENABLED=1` for the tree-sitter extractor bindings, ADR-013 — not for the store, which is pure Go since SS-8 deleted the DuckDB backend, ADR-025).
4. Prints elapsed time and a PASS / FAIL banner.
5. Exits 0 on success, 1 on failure.

### Environment variables

| Variable     | Default | Description                                      |
|-------------|---------|--------------------------------------------------|
| `CGO_ENABLED` | `1`   | Must be `1` to compile the tree-sitter CGO bindings (ADR-013). |
| `TASK_FLAGS`  | (empty) | Extra flags forwarded to the `task` invocation. |

### Examples

```bash
# Standard CI run
./scripts/ci.sh

# With verbose task output
TASK_FLAGS=--verbose ./scripts/ci.sh
```

## gate-lease.sh

`task validate` runs under it (RT-129, design §12.3): an flock on
`~/.fathom/locks/gate.lock` (`FATHOM_GATE_LOCK` overrides) queues concurrent gates on one
machine and names the holder; the command runs niced (`nice -n 10`; `FATHOM_GATE_NICE=off`
opts out, `=background` uses macOS's `taskpolicy -b`). `gate-lease_test.sh` (in
`task scripts-check`) proves two holders serialize.

## tenancy-stress.sh

The responsible-tenant stress harness (RT-136, design §15.2) — committed, not in the
gate, and every run needs `--i-have-the-operators-go-ahead` on a quiet machine. It copies
a store with `sqlite3 .backup`, generates a fixture corpus under an external volume,
starts an unmanaged daemon (its own child, `--lifecycle-id tenancy-stress-<run>`, a scratch
`HOME`, `FATHOM_TENANCY_BUDGET` forced), drives the load mix, SIGKILLs and relaunches it,
samples `/metrics` every second and writes a reading with the verdict first. It refuses
the managed daemon's port, store and call log and a fixture on the internal disk;
`--self-check` exercises those refusals and is run by `tenancy-stress_test.sh` in
`task scripts-check`. `pipefail-grep_test.sh` (also in `scripts-check`) fails a
`scripts/*_test.sh` that pipes a `printf`/`echo` into `grep -q` under `pipefail`: grep
exiting on its first match SIGPIPEs a multi-chunk writer, which flaked the gate at nice 10
under load; use `grep -q ... <<<"$value"`.

```bash
bash scripts/tenancy-stress.sh --self-check
bash scripts/tenancy-stress.sh --i-have-the-operators-go-ahead --store-source workspace/db/fleet.sqlite \
  --run-root /Volumes/<external>/tmp            # RT-M3 / RT-M5 / RT-M10 (op rows)
```

# scripts/

Helper scripts for the fathom project.

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
   - `test` — runs `go test -race ./...` (requires `CGO_ENABLED=1` for the DuckDB driver).
4. Prints elapsed time and a PASS / FAIL banner.
5. Exits 0 on success, 1 on failure.

### Environment variables

| Variable     | Default | Description                                      |
|-------------|---------|--------------------------------------------------|
| `CGO_ENABLED` | `1`   | Must be `1` to compile the DuckDB CGO driver.   |
| `TASK_FLAGS`  | (empty) | Extra flags forwarded to the `task` invocation. |

### Examples

```bash
# Standard CI run
./scripts/ci.sh

# With verbose task output
TASK_FLAGS=--verbose ./scripts/ci.sh
```

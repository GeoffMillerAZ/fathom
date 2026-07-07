# scripts/

Helper scripts for the fathom project.

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

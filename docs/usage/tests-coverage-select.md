### `tests coverage` / `tests select` — coverage, mutation, and test selection

Full walkthrough: [docs/use-cases/test-coverage.md](../use-cases/test-coverage.md).

```sh
fathom tests coverage --profile [layer=]<file> [--profile <more>...] \
  [--foreign <format>=<file>...] \
  [--service <id>] [--store <path>] [--diff <ref>] [--mutation <report>] [--json]

fathom tests select --since <ref> [--safe] [--profile [layer=]<file>...] \
  [--service <id>] [--store <path>] [--json]
```

`tests coverage` joins one or more `go test -coverprofile` files (each
labeled by test layer) to indexed symbols: a per-symbol table (one column
per layer, plus `ANY`), or `--diff <ref>` to narrow to changed symbols with
no covering test in any layer. `--mutation <report>` (VB-34) adds a
`MUTATION` score column (0.0-1.0) from a joined mutation-testing report — see
"Mutation" in the use-case doc for the two accepted report shapes.

`--foreign <format>=<file>` (repeatable; TQ-14w) joins a non-Go coverage report to the same
indexed symbols, in the formats `fathom coverage ingest --format` accepts (`lcov`, `cobertura`,
`jacoco`, `coverage-py`, `istanbul`, `llvm-cov`), read under the same 256 MiB cap. A file whose
function symbols all carry line ranges is joined per symbol; a file whose symbols do not (or has
none) is joined per file and printed under `FILE-LEVEL`, never guessed at symbol granularity; a
report file no indexed file matches is named under `UNMATCHED`. `--profile` is optional when
`--foreign` is given. `--json` carries both views under `foreign` (`by_symbol` and `by_file`, each
row's `level` naming its granularity, plus `inputs` and `unmatched_files`) beside the Go view.
`--diff` and `--mutation` narrow and score the Go `--profile` view only, so `--diff` is refused
with `--foreign`.

`tests select --since <ref>` leads with the certifying-first selection (T4: TQ-15w,
TQ-16w, SR-50): the minimal set of tests that re-proves every LIVE certificate the diff
touched (a greedy set cover, deterministic tie-break), what it skipped, the touched
certificates no live test certifies, and its `go test` commands. `--safe` adds every test
that statically reaches a changed symbol. It reports the CI time saved, from each selected
and skipped test's last attested run duration, and appends it to
`workspace/artifacts/tests-select/time-saved.jsonl` (one line a run; an estimate with no
attested duration behind it prints `time saved: unknown` and records nothing). Selection is
advice to CI and never makes a certificate live (I43). Below it, VB-33 ranks `Test*` functions for a diff by
typed-edge reachability, corroborating statement coverage (when `--profile`
is given), and test layer (unit before integration before e2e), then emits
a `go test ./<pkg> -run '^(TestA|TestB)$'` invocation per package plus the
changed symbols no test reaches. A test that already certifies a target
whose scope intersects this diff (TC-23, vision 33 / ADR-043 stage 3) ranks
first, ahead of that reach ranking — advisory only, it never changes which
tests are selected. A replay ledger at
`workspace/artifacts/tests-select/replay.jsonl`, when present, adds a
measured predicted catch-rate — never a fabricated one.

| Flag (`tests coverage`) | Default | Description |
|---|---|---|
| `--profile` | (required unless `--foreign`; repeatable) | Go cover profile, as `[layer=]path`; a bare path defaults to layer `unit` |
| `--foreign` | (none, repeatable) | non-Go coverage report as `<format>=<file>`, joined to symbols by line range where the language has them, else per file |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--diff` | `""` | git ref: narrow to changed symbols with no covering test in any layer |
| `--mutation` | `""` | mutation-testing report; adds the `MUTATION` score column |
| `--json` | `false` | Emit JSON instead of a table |

| Flag (`tests select`) | Default | Description |
|---|---|---|
| `--since` | (required) | git ref: rank tests for the diff since this ref |
| `--safe` | `false` | also select every test that statically reaches a changed symbol, beside the certifying-first set |
| `--profile` | (none, repeatable) | Go cover profile(s); enables the coverage ranking tie-break |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--json` | `false` | Emit JSON instead of a table |

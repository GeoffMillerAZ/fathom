TQ-M6 reading: 3,314 -> 1,850 tests on war2 fixes, 3,661 -> 2,336 on fic; 0 of 5 mutation failures missed

# `tests select --safe` precision and soundness (wave hon1; O1001)

Verdict: `--safe` is narrower and no less safe. On war2's fixes diff it now
names 1,850 tests in 13 packages instead of 3,314 in 67; on fic's diff 2,336
in 24 instead of 3,661 in 70. Every test the four fixed mutations failed (5)
was in the narrowed selection, and so was the one a supplementary mutation
failed. Method: `safe-selection-precision-method.md` (committed before any
data, 7ecaa854). Run 2026-10-16 on scratch ingests only.

## Precision

Same checkout, same store, same command
(`tests select --safe --since <ref> --service fathom --json`); before is the
binary built at `b97e6e8c`, after the hon1-select binary at `d91e174a`.

| Diff | Tests before | Tests after | Packages before | Packages after | Via dispatch before | after |
|---|---|---|---|---|---|---|
| war2 fixes (`fb6327d9`, since `99aa205a`) | 3,314 | 1,850 | 67 | 13 | 3,252 | 1,792 |
| fic (`3bd33f80`, since `354c9b57`) | 3,661 | 2,336 | 70 | 24 | 3,138 | 1,671 |

TQ-29 kept 97 symbols of war2's changed files out (their text is the same at
`99aa205a`; 15 files read whole: non-Go files and Go files with a swapped
import) and 1,399 of fic's (24 read whole). O1001's two moved symbols,
`ConformanceError.Error` and `SQLiteStore.Close`, are among them.

What remains is static reach, not fan-out: on war2, 1,219 of the 1,850 are
cmd/fathom tests and 515 internal/mcp tests, and 1,164 tests reach
`gitro.BlobAt` itself (the CLI's verbs and the MCP tools are registered
from package initializers and reached through dispatch, and `read_source`
calls `BlobAt`). The new error types' `Error` methods now count only for
the 1,792 tests whose binary constructs them, not the 3,091 O1001 counted.
On fic the remainder is fic's own breadth: 1,673 tests reach
`fusion.EnrichTypedEdges`, and 1,958 reach the changed Taskfile `build`
task (a non-Go file, read at file level).

## Soundness (mutation check)

Scratch worktree of `d91e174a`, indexed (`db/mut.sqlite`); each break is a
same-line edit; `--since HEAD`; every package whose test binary links the
mutated package run with `go test -count=1 -json`.

| Mutation | Packages run | Failing tests | Selected before | Selected after | Missed after |
|---|---|---|---|---|---|
| `external.AnchorLinks`: `if ev.Anchor == nil \|\| true` | 12 | 2 | 602 | 598 | 0 |
| `mcp.readSourceAtRef`: first error check `\|\| true` | 8 | 2 | 1,178 | 1,178 | 0 |
| `store.ConformanceError.Error`: Detail dropped from the message | 37 | 0 | 3,134 | 682 | 0 |
| `store.sqliteReadOnlyDSN`: `mode=ro` to `mode=rw` | 37 | 1 | 3,134 | 682 | 0 |
| supplementary, not in the fixed four: `MountReadOnly`'s DuckDB refusal gives `ReasonOpenFailed` | 37 | 1 | 3,134 | 682 | 0 |

Failing tests: `cmd/fathom TestFindingsImportConformance`,
`internal/findings/external TestImportAnchorsHashAtSourceRevision`,
`internal/mcp TestReadSourceRefFiltersNameTheirFilter`,
`internal/mcp TestReadSourceRefServesBlobBytes`,
`internal/store TestMountReadOnly_SucceedsAndIsGenuinelyReadOnly`, and the
supplementary `internal/store TestMountReadOnly_RefusesDuckDBExtension`.
All are in the after selection. No package failed outside a named test.

No test reads `ConformanceError.Error`'s text (the conformance tests assert
`Reason` through `errors.As`), so that mutation counts 0, as the method
says; the supplementary reason mutation is a ConformanceError path a test
does read. n is small: these are weak oracles for the change's own risk,
and the reading claims no more than that no observed failure was missed.

## Limits

- Selection only: `review_diff`'s test impact, `blast_radius`, `localize`,
  the stored edges and `callgraph.Build` read as before.
- Rapid type analysis links a package's initializers when the test's reach
  touches the package. A package linked only by a blank import or a constant
  no edge names runs initializers this cannot see (ProductReach's documented
  under-approximation).
- What an initializer reaches constructs types but is not the test's reach,
  as before this change: a broken function called only from an `init` is
  still not selected.
- Comment-only edits inside a body count as changes (the text rule reads
  bytes, not tokens); a doc comment counts only for its `//go:` directives.
- A struct's field change counts for the type, not its methods; tests reach
  it through the type references at its construction sites.

Raw files (not committed): `/Volumes/m2ext/tmp/fathom-build/w-hon1-select/meas/`
and `/Volumes/m2ext/tmp/fathom-build/w-hon1-select/mutout/`.

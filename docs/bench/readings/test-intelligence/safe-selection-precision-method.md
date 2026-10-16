# TQ-M6 method: `tests select --safe` precision and soundness (wave hon1; O1001)

Fixed before any data is read (K19). The reading is
`safe-selection-precision.md` beside this file; its first line is
`TQ-M6 reading: <before> -> <after> tests on war2 fixes, <before> -> <after> on fic; 0 of <n> mutation failures missed`.

## What is measured

TQ-29 (a symbol counts as changed only when its own text changed) and TQ-30
(an interface-dispatch edge counts for a test only when that test's reach
constructs the implementing type) narrow `fathom tests select --safe`. The
reading answers two questions: how much smaller the selection is on two real
diffs (precision), and whether it still holds every test a real break fails
(soundness, the point of `--safe`).

## Subjects

| Diff | Checkout | `--since` | Why this range |
|---|---|---|---|
| war2 fixes | `fb6327d9` (the O1001 commit on war2-fixes) | `99aa205a` | the exact run O1001 records: 3,312 tests |
| fic | `3bd33f80` (fic-doctor, fic-verify, fic-hooks, fic-docs merges and the CLI-text fix) | `354c9b57` | fic's own changes, before main's lf8 merge `05a490ef` |

Each checkout is a detached worktree with its own scratch ingest
(`fathom ingest --service fathom --path . --store <w-hon1-select>/db/<name>.sqlite`),
never the shared daemon's store.

## Precision

Before: the `fathom` binary built at `b97e6e8c` (origin/main when the wave
started). After: the binary built from the hon1-select branch. Both run in the
same checkout, against the same store:

    fathom tests select --safe --since <ref> --service fathom --store <store> --json

The count is `len(.selection.tests)`; the package count is
`len(.selection.invocations)`. Both are recorded before and after.

## Soundness (mutation check)

Four functions war2 or fic changed: `findings/external.AnchorLinks`,
`store.sqliteReadOnlyDSN`, `mcp.readSourceAtRef`, and
`store.ConformanceError.Error` (the error path O1001 names). For each, on a
scratch worktree of the hon1-select HEAD the store indexes:

1. Break the body with a same-line edit that still compiles (return a wrong
   value or a nil result), so the index's spans and edges stay those of HEAD.
2. Run the after selection with `--since HEAD` (the working tree carries the
   break) and record the selected tests.
3. Run every package whose test binary links the mutated package
   (`go list -test -deps` names it) with `go test -count=1 -json`, and collect
   every top-level test whose Action is `fail`.
4. A miss is a failing test not in the selection. The reading reports the
   failing count n summed over the four mutations and the misses.

Pass: 0 misses over all four. One miss stops the wave item: it is reported,
and `--safe` is never shipped less safe. A mutation no test fails is reported
as such (n contributes 0) and does not count as evidence either way.

## Not measured

Wall time of the selection, `--profile` ranking, the certifying-first cover
(no live certificates are ingested in these scratch stores), and any language
other than Go: a non-Go changed file keeps every symbol it defines as changed.

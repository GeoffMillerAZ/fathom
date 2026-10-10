TQ-25 verdict: PASS

# `tests select --safe` and tests that read source by literal path (TQ-24, O783)

Plain-language verdict: before TQ-24, `--safe` missed any test that reads another package's source
through a literal path and never imports it (`internal/scope`'s
`TestSuppressionCoversOptionalStoreCapabilities` globs `../store/*.go` and parses it). After it, an
edit to a file that literal read names selects the reading test. The cost is two to three extra tests
per diff on fathom's own suite; the miss class is closed for literal paths and stays open for dynamic
ones, which are counted on every run.

Measured 2026-10-10 on a scratch ingest of fathom (fathom at origin/main fb7d9f3f plus TQ-24), binary
built from the same tree, `fathom tests select --safe --json --since HEAD` over a one-line edit.

## Census of source reads in fathom's own `_test.go` files

| Reading | Count |
|---|---|
| Calls to os.ReadFile / os.Open / os.ReadDir / filepath.Glob / fs.Glob / parser.ParseFile / parser.ParseDir | 905 |
| Path folded to a constant (literal, `+`, or filepath.Join/path.Join of literals) | 78, in 72 functions (tests or helpers) |
| Of those, resolving outside the test's own package directory (`../...`) | 41 |
| Dynamic or unresolved (a variable, a TempDir path, a parameter): ignored, disclosed | 827 |

The 827 are overwhelmingly tests reading files they themselves wrote to `t.TempDir()`; they depend on
no repository file. `tests select` prints the count on stderr every run.

## Selected tests, before and after (`--safe`)

| Synthetic diff | Before | After | TestSuppressionCoversOptionalStoreCapabilities |
|---|---|---|---|
| internal/store/backend.go | 828 | 830 | not selected, then selected |
| scripts/bench-ledger-render.sh | 558 | 560 | not selected, not selected (it reads no store file) |
| internal/extractor/dialects/typescript/dialect.json | 2128 | 2129 | not selected, not selected (unrelated) |

The extra tests in the last two rows are readers of those files by literal path (not itemized here).

## Ceiling

A path built from a variable is not matched (827 reads today). A Test function that makes the read
itself is added to the selection directly; a helper that makes it seeds the impact walk, which finds
the tests that call the helper.

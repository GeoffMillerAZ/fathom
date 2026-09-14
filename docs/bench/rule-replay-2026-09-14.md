# Rule replay — 2026-09-14

VB-16 (`docs/plans/verbs.md` wave C; `docs/vision/29-verbs-not-lookups.md`
§5 bullet 3): "a compiled layer rule catches nothing in the last 200
commits and nothing in thirty days of dogfood — governance stays
advisory." This is the replay half of that question: evaluate today's
three authored layer rules (`.fathom/guard.cue`'s `layer_rules`) against
every one of this repository's last 200 commits, not just the current
tree, using `fathom rules replay` (VB-16; `internal/analysis/rulereplay`).

Built as a third, independent producer of a `guardspec.LayerRuleIndex` —
alongside the call-derived live-obligations graph and `fathom rules
gen-test`'s go/packages loader — because replaying 200 historical commits
needs neither a running index nor a fully resolvable module at 200
different dependency states: it checks out each commit with `git archive`
into a throwaway temp directory (never this checkout's working tree) and
builds the import graph with `go/parser` alone, reading only each commit's
own source text. No daemon, no store, no `workspace/db`.

## Command

```sh
fathom rules replay --root . --since 200 --json > docs/bench/rule-replay-2026-09-14.json
fathom rules replay --root . --since 200
```

`--since 200` means the literal last 200 commits (`git log -n 200
--reverse HEAD`), not the two-dot range `HEAD~200..HEAD` — the latter
errors outright on a repository with 200 or fewer commits and, on a
repository with merge commits (this one has several in the replayed span),
can silently name a different, usually larger, set than "the last 200
commits" actually means. Every commit is evaluated against its own
archived tree; a violation is classified NEW when absent from the
commit's first parent, INHERITED when already present there.

## Headline numbers

| | |
|---|---|
| Commits replayed | 200 |
| Range | `24f9edac2735`  (2026-09-12T14:32:38-07:00) … `4bdc043df0ce` (2026-09-13T23:46:14-07:00) — about 33 hours of dogfood |
| Rules evaluated | `store-no-cgo`, `core-no-mcp-or-cmd`, `internal-no-cycles` (all three authored rules; `cross_service_direction` has none configured, so nothing was skipped) |
| Violations found | **0** |
| New violations | **0** |
| Commits with any violation | 0 / 200 |
| Commits with a parse warning | 0 / 200 |
| Elapsed | 190,659 ms (≈ 3m 11s) total, ≈ 953 ms/commit |

## The honest reading

Every one of the 200 replayed commits was clean against all three rules.
**Nothing was caught.** This matches the possibility §5 names directly:
"if nothing is caught anywhere, governance stays advisory and the row says
so." The result is not softened here — the compiled layer rules have not,
in this measured window, prevented or flagged a single regression, because
none of the guarded shapes (`internal/store` importing the tree-sitter
extractor, `internal/core` importing `internal/mcp`/`cmd/**`, an import
cycle under `internal/**`) occurred in these 200 commits. That is
consistent with the rules encoding invariants this codebase has in fact
held throughout the replayed window, not with the rules being untested —
`internal/guardspec`'s own unit tests (`layerrule_eval_test.go`) and this
package's synthetic-repo tests already prove each rule kind fires
correctly on a seeded violation; VB-16 asked a different question ("has it
ever fired for real here"), and for this repository, in this window, the
answer is no.

## Per-commit cost

≈ 953 ms/commit (200 commits in 190.7s), dominated by `git archive` +
`tar` extraction of the commit's ~2,100 `.go` files into a fresh temp
directory and the `go/parser` walk over them — not by any daemon, index,
or network access (none is used). Cost scales with repository size and
commit count; a 200-commit replay comfortably fits in a few minutes on
this repository and would need to grow roughly linearly for a much larger
one.

## Full report

The commit-by-commit detail (all 200 rows; every `violations` array is
empty) is in [`rule-replay-2026-09-14.json`](./rule-replay-2026-09-14.json).

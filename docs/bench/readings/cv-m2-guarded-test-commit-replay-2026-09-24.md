# CV-M2: 50-commit guarded-test replay (2026-09-24)

**Verdict (plain language).** The integrity check stays quiet on ordinary
development. Of the last 50 commits on this repo's own history that
touched a guarded test file, 9 (18%) produced an `edited` finding — the
kind design §1.3 marks "informational; never a loss on its own" — and
every one of those 9 is spot-checkable as an ordinary edit (assertion
counts rose, never fell, in the sample checked by hand). **Zero of 50**
produced a true loss-kind flag (`deleted`, `weakened`, `skipped`,
`golden_changed`, `marker_moved`) on a change that was not an actual
weakening. **CV-M2 passes, by a wide margin (0% against a 10% bar).**

Bar (design §1.2, ADR-044 revisit clause): at most 10% of the last 50
commits touching a guarded test may flag a loss-kind finding that is not a
true weakening. Measured 2026-09-24 on `fathom` at df6a85c5.

## Method

**Guarded-test file set** (89 files): the union of (a) every `_test.go`
file containing a `// certifies:` marker (32 files, `grep -rl` over the
tracked tree, non-`third_party`) and (b) every `_test.go` file defining a
`Test*` function named in `docs/intent/invariants.md`'s enforcement column
or `docs/intent/scope.md`'s Evidence column (96 named tests, 78 files
after resolving to their defining file). This is a file-level
approximation of "guarded test," not a per-function one — a commit
touching an unguarded test in a file that also holds a guarded one counts
as "touched a guarded test file," matching the plan row's own file-scoped
`GuardedTests` reachability, but coarser than the design's function-level
definition. Reach-based guardedness (the fifth reason, "reaches a changed
symbol") is not part of this file set — it would only ADD to the guarded
set, so this measurement's flag-rate denominator is if anything an
under-count, not an over-count.

**Commits**: `git log --format=%H -- <89 files>` on the branch's own
history, most-recent-first, capped at 50
(`docs/bench/readings/../../../cv-m2/last50_commits.txt`, spans
942768ba09 .. df6a85c5, 2026-09-22 through 2026-09-24 — three days of this
program's own wave-1 development, a demanding sample since every commit in
range is deliberately touching the very tests the check exists to guard).

**Check**: for each commit `C`, a shared scratch clone is checked out
detached at `C` and `bin/fathom conserve --service cvm2probe --store
<shared.sqlite> --since C^1 --json`'s `integrity` plane is read — no
per-commit ingest (the marker/manifest/invariant-enforcement/scope-evidence
guardedness reasons and the rename-normalized body-hash comparison are
both pure file-content reads; only the fifth, reach-based, reason needs an
index, and its absence only under-counts guardedness, never over-counts a
loss).

## Results

```
total commits: 50
commits with >=1 integrity finding: 9 / 50 (18.0%)
finding kind counts: {'edited': 9}
commits with a TRUE loss-kind finding: 0 / 50 (0.0%)
```

All 9 flagged commits are `TestConfig_DecideBlock` (6x) or
`TestCompileCertifications_TierAndStateLadders` (3x) — the two tests this
program's own wave-1/wave-1b/wave-1c work edited most, exactly the
"legitimately edited every day this program touches its own guarded tests"
population the bar is meant to tolerate. Spot check on
`942768ba09` (`TestConfig_DecideBlock`): assertion-call count (`t.Error*`
/`t.Fatal*`/`t.Fail*`) went from 22 at base to 27 at head — the test grew,
it did not shrink; `conserve`'s own verdict for this plane stayed
`conserved` on all 9 (the tool already treats `edited` as non-blocking, as
designed — the "0% loss-kind" figure above is the plane's own gate
behavior, not a post-hoc reclassification).

No `deleted`, `renamed`, `weakened`, `skipped`, `golden_changed`, or
`marker_moved` finding appeared on any of the 50 commits.

## Reproduce

```sh
git clone <fathom> /tmp/cv-m2 && cd /tmp/cv-m2
git log --format=%H -- <89 guarded-test files> | head -50 > commits.txt
while read c; do
  git checkout -q --detach "$c"
  bin/fathom conserve --service cvm2 --store cv.sqlite --since "$c"^1 --json
done < commits.txt
```

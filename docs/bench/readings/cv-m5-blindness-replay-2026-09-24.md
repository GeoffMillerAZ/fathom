# CV-M5: Blindness replay against fathom's own migrations (2026-09-24)

**Verdict (plain language). CV-M5 fails on one of its two migrations, and
the failure is a real, useful finding, not a fluke.** On ADR-041's vector
withdrawal, the detector is exactly right: the seeded copy flags, the
genuine range is silent. On ADR-025's DuckDB removal, the seeded copy
correctly ALSO flags — but so does the untouched, unseeded, genuine
deletion: 24 `body_resurrected` findings that are not resurrections at
all. The root cause is specific and fixable: `conserve.Resurrected`
matches a deleted base function against ANY function in a changed file at
head with the same body hash, without checking that the matching head
function is itself new-or-changed in this diff — so a repository that
carried two independently-written, structurally-convergent
implementations of the same interface (here: `DuckStore` and
`SQLiteStore`, both hand-written against the same `Store` interface with
naturally similar CRUD bodies) produces a false "resurrection" on every
shared method the instant one twin's file is deleted, even though nothing
was copied in the change that deleted it.

Bar (design §3.3): every seeded copy must flag as a resurrection loss;
every genuine range must produce none. **Result: 2/2 seeded copies flag
(pass); 1/2 genuine ranges are clean (fail — the DuckDB genuine range
produces 24 findings).**

## Method

Both migrations found by commit message (`git log --oneline`):
`356630db` "SS-8 — delete DuckDB backend + CGO requirement outright
(ADR-025)" and `bc0ee164` "SU-8 — delete the vector/hybrid-search path
(ADR-041 decision 2)". For each, base = the deletion commit's own parent,
amended with one commit adding a plan row that declares the retirement at
base (`retires: "api:....DuckStore.*"` / `"api:....internal/vector.*"`)
so the surface plane's ordinary "capability removed" findings are excused,
isolating the resurrection plane as the thing under test:

- **genuine**: (declared base) + the real deletion commit's own diff,
  applied verbatim (`git diff parent deletion-commit | git apply`),
  unmodified.
- **seeded**: (declared base) + the same real deletion diff + one new
  file that copies a retired function's exact body (statement-for-
  statement identical; only identifiers renamed, which the hash ignores)
  under a new name, attached to the surviving implementation. Every
  seeded function meets design §3.3's ≥5-statement floor (checked against
  the real `clone.FuncBodyHash`, not assumed — the first attempt, using a
  4-statement function, correctly did NOT flag, which is the floor
  working as designed, not a bug; the seeded fixture was corrected to a
  5-statement function).

`bin/fathom conserve --since <declared-base> --json`'s `resurrection`
plane, read on both.

## DuckDB (ADR-025) — fails

Seeded copy (`DuckStore.Fingerprint` → `SQLiteStore.LookupContentFingerprint`,
5 statements, byte-identical body): flags correctly, one extra
`DuckStore.Fingerprint` match beyond what the genuine range already shows.

Genuine range (the real `356630db`, nothing added): **24**
`body_resurrected` findings, e.g.:
```
DuckStore.ClaimEvents (deleted from internal/store/duckdb.go) reappears
as SQLiteStore.ClaimEvents at internal/store/sqlite.go — identical body,
6 statement(s)
```
Checked by hand: `SQLiteStore.ClaimEvents` is byte-identical at
`356630db^:internal/store/sqlite.go:967` and `356630db:internal/store/sqlite.go:968`
— it did not change in this diff at all (only its line number shifted, from
unrelated edits earlier in the same file). It was never copied, moved, or
resurrected; `DuckStore` and `SQLiteStore` simply implemented the same
`Store` interface with historically convergent CRUD bodies for years
before this commit deleted the DuckDB twin. `conserve.Resurrected`'s
matching pool ("every function in a changed file at head," `resurrection.go`
line ~84) includes `SQLiteStore.ClaimEvents` because `sqlite.go` is a
changed file for unrelated reasons — the check never asks whether the
MATCHING head function is itself part of the diff, only whether the file
that holds it is.

## Vector/hybrid-search (ADR-041) — passes

Seeded copy (`SymbolIndexer.SearchText` → `legacyEmbedIndexer.SearchSimilarSymbols`,
5 statements): flags correctly as the sole finding.

Genuine range (the real `bc0ee164`, nothing added): **0** findings on the
resurrection plane. `internal/vector` had no long-lived structural twin
elsewhere in the codebase, so there is nothing for the detector to
coincide with.

## What this shows

The detector's core mechanism works exactly as designed against a
deliberate copy (both seeded fixtures flag, at the right statement floor).
The failure mode this reading finds is orthogonal to Blindness itself: a
codebase that legitimately maintained two parallel, interface-conformant
implementations will produce a resurrection finding on EVERY shared
method the moment either implementation is deleted — a real false-positive
class, not a false negative, and the more dangerous direction for a check
whose whole purpose is to be trusted when it fires. A fix in the design's
own terms: `Resurrected`'s `head` matching pool should be restricted to
functions that are themselves ADDED, or whose body differs from what the
SAME name held at base in the SAME file — i.e., resurrection should ask
"does this specific head function look new," not merely "is this head
function's FILE among the changed files."

## Reproduce

```sh
git clone <fathom> /tmp/cv-m5-duckdb && cd /tmp/cv-m5-duckdb
git checkout 5a7bd817713f662b7b5f35e0bb7691c61b2afd46 -b base
# add docs/plans/cv-m5-ss8-retirement.md declaring `retires: "api:....DuckStore.*"`, commit
git diff 5a7bd817713f662b7b5f35e0bb7691c61b2afd46 356630db33f40d5411c4c659c71b0cf7fe315f18 | git apply && git add -A && git commit -m genuine
bin/fathom conserve --service cvm5 --store cv.sqlite --since <declared-base-sha> --json
```
(seeded: same, plus the new file with a copied ≥5-statement function under
a new name, before the final commit; vector the same shape against
`4edbc217`/`bc0ee164`.)

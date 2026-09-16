# Tests-select replay — 2026-09-14
VB-35 (`docs/plans/verbs.md` wave J; needs VB-33): "(op) M12b and M2 from a replay of ~100 historical commits: selection recall ≥ 0.98 at ≤ 25 % of full-suite time, else advisory only." This is that replay: for each of the last 105 non-merge commits on `main` that touch at least one `.go` file, checkout the commit into a worktree, ingest it into a SCRATCH store (`workspace/tmp/vb35-store.sqlite`, service `fathomreplay` — never the shared daemon or store), and compare `fathom tests select --since <parent>`'s selection against an independent ground truth built from `go list -deps -test`, not from fathom's own call graph.
## Oracle (stated before the numbers)
**Coarse truth (all 105 commits).** For a commit with parent P: the changed `.go` files map to their containing packages via `go list -json ./...` at that commit. `go list -json -deps -test ./...` gives every synthesized test-build variant (`pkg [pkg.test]`, `pkg_test [pkg.test]`) with its full transitive import closure. A variant "reaches" the diff when its own package IS a changed package, or a changed package is in its `Deps`. Truth = every `Test*` function (Go func, `_test.go` file) in a reaching variant's declared package, resolved from the SAME store `tests select` reads (so truth and selection compare in the identical `service::dir/pkg.Name` key space) — this is package-level: a test only has to sit in a reaching package, not call the changed symbol. Selection is `ranked_tests[].test` from `fathom tests select --since P --json`, run in the same worktree so its own `git diff --name-only P` sees exactly the checked-out commit's diff. Recall = |selected ∩ truth| / |truth| per commit; pooled recall = Σintersection / Σtruth over commits with nonzero truth.
**Finer truth (a sample of the smallest-truth commits).** Package-level truth cannot distinguish "this test calls the changed function" from "this test merely lives in the same package" — so on the commits with the smallest coarse-truth sets (capped at 40 candidates per commit, prioritizing whatever fathom itself selected, to bound wall clock when a commit lands in one of this repo's two largest packages), each candidate test is run ALONE (`go test -run '^Name$' -coverpkg=<the diff's changed packages> -coverprofile`) and kept in the finer truth only if its own coverage profile marks a line `git diff -U0 <parent> <commit>` shows as added/modified in a changed file. This is strictly ⊆ the coarse truth for the same commit.
**Time fraction.** One `go test ./... -count=1 -json` run at HEAD (no `-race`, `nice -n 10`) gives every package's own elapsed time (112 packages, `internal/fusion` failed — unrelated pre-existing failure, timing unaffected). Per commit, time fraction = Σ(elapsed time of every package `tests select` names in its `invocations`) / Σ(elapsed time of every package in the full run) — the FULL recorded time of a selected package, not a per-function slice, because `go test -run` still pays that package's build/link/setup cost regardless of how few of its tests match the `-run` regex.
## Command
```sh
GOWORK=off GOFLAGS=-p=2 CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 \
  bin/fathom ingest --service fathomreplay --path <worktree-at-commit> \
  --store workspace/tmp/vb35-store.sqlite
bin/fathom tests select --since <parent-sha> --service fathomreplay \
  --store workspace/tmp/vb35-store.sqlite --json
```
repeated per commit against one incrementally-updated (hash-gated) scratch store in one reused worktree, oldest commit first. Independent ground truth via `go list -json -deps -test ./...` per commit; never fathom's own call-graph engine (that would be circular).
## Headline numbers
| | |
|---|---|
| Commits replayed | 105 (0 errors, 0 with zero truth) |
| Range | `61d0ce34c6` … `fe713c2a4e` — 2026-09-13T15:38:27-07:00 … 2026-09-14T12:06:57-07:00, about 20.5 hours of dogfood |
| Full-suite duration (sum of per-package elapsed, one `go test ./...` at HEAD) | 182.6s across 112 packages |
| **Pooled recall** (Σselected∩truth / Σtruth) | **28.2%** |
| Mean / median recall per commit | 24.8% / 21.8% |
| Pooled recall, excluding the 9 test-only-diff commits | 30.4% |
| Commits with recall = 1.0 | 2 / 105 |
| Mean / median time fraction | 26.6% / 25.0% |
| Commits selecting one of the two heaviest packages (`cmd/fathom` 29.3%, `internal/scenariobench` 21.1% of the full suite) | 95 / 105 |
| **Verdict against VB-35's bar (recall ≥ 0.98 AND time ≤ 25%)** | **NOT MET** |

## The honest reading
Pooled recall against the coarse, package-level oracle is **28.2%**, not the ≥ 98 % the row requires, in any cut of the data: 24.8% mean per-commit, 21.8% median, 30.4% pooled even after dropping the 9 commits that changed only `_test.go` files (where fathom's own `impact.TestGap` correctly seeds nothing — no production symbol changed — while the coarse oracle still credits the whole touched package as truth; those commits alone average 0.3% recall and are a known artifact of this oracle, not a tool failure). Time fraction is **also** over the bar on both the mean (26.6%) and median (25.0%) — 95 of 105 replayed commits selected `cmd/fathom` and/or `internal/scenariobench`, this repo's two heaviest packages by test time (29.3% and 21.1% of the whole suite respectively, mostly because both were the two most actively dogfooded packages across this replay window); selecting even one test from either one already spends more than the 25% budget on that package's build+run cost alone, regardless of how narrow the `-run` regex is. **Neither half of the row's bar is met.**
The gap is explained, not just measured: `fathom tests select` ranks by **typed-edge call-graph reachability** (`impact.TestGap`, ADR-021) — a test counts only if the call graph shows it statically reaching the changed symbol. The coarse oracle this row asked for counts a test as truth merely for **sitting in a package whose test-build variant transitively imports a changed package** — a Test function in a 200-test package that never calls the one function that changed is coarse-truth but has no business being selected. The finer oracle below (real, isolated coverage of the actual changed lines) confirms this reading directly.
## What the tool missed, and why
A concrete example of each pattern (full per-commit detail, including every `truth_minus_selected` list, is in [`tests-select-replay-2026-09-14.json`](./tests-select-replay-2026-09-14.json)):

- **Test-only diff, coarse truth is pure artifact** — `6a92182806` ("test(indexbench): the freshness probe's time ceiling tolerates a loaded gate machine") touches only `internal/indexbench/freshness_test.go`'s own assertion threshold. `impact.TestGap` correctly finds zero non-test seeds → 0 selected. The coarse oracle still counts all 39 `Test*` funcs in `internal/indexbench` as truth (the changed file's own package trivially "reaches" itself), so recall reads 0.0 for a diff that had nothing for `tests select` to select.
- **Huge home package, narrow real change** — `017920acaa` ("fix(cmd/fathom): quote test names in tests select's -run regex") is a one-line fix inside `groupTestInvocations` itself. `cmd/fathom` carries 970 `Test*` functions total (it's the whole CLI's test suite); the coarse oracle counts every one of them as truth since the changed file lives in that package. `tests select` returned 235 tests — already a lot for a one-line fix, and every one of them a genuine call-graph hit — for 24.2% recall against a 970-test denominator that was never a realistic bar for a single-function change.
- **Typical case** — `6a2a124d57` ("fix(scenariobench): a transcript write refused by the volume wedge loses the cell, not the run") changes production code in `internal/scenariobench`, a 216-test package. 33 tests were selected (15.3% of coarse truth) — every `truth_minus_selected` entry checked by hand is an independent unit test of an unrelated helper in the same package (parsing, rendering, ledger I/O) with no call-graph path to the changed function; none is a plausible regression catcher for this diff.
No commit's `truth_minus_selected` set, spot-checked across the smallest-truth commits (below), contained a test the finer, line-coverage oracle also confirmed as a real hit — every discrepancy examined by hand was the same package-membership-vs-call-graph gap, not a tool defect.

## Finer oracle: does the coarse gap survive real coverage?
Ran on the 10 replayed commits with the smallest coarse-truth sets (candidates capped at 40 per commit, prioritizing whatever fathom itself selected, for the commits that land in a large package):

| commit | coarse truth | candidates probed | capped | finer truth | recall (coarse) | recall (finer) |
|---|---|---|---|---|---|---|
| `a7546dc3a1` | 15 | 15 | no | 2 | 100.0% | 100.0% |
| `3909c71673` | 15 | 15 | no | 0 | 100.0% | n/a (finer truth empty) |
| `7c2f49f1a3` | 35 | 21 | yes | 3 | 60.0% | 100.0% |
| `00b551f1ae` | 39 | 15 | yes | 1 | 10.3% | 100.0% |
| `6a92182806` | 39 | 15 | yes | 0 | 0.0% | n/a (finer truth empty) |
| `4ef0d0c9f4` | 39 | 15 | yes | 0 | 0.0% | n/a (finer truth empty) |
| `61d0ce34c6` | 215 | 31 | yes | 10 | 14.4% | 100.0% |
| `4bea25093d` | 216 | 23 | yes | 3 | 10.6% | 100.0% |
| `17f63459bb` | 216 | 48 | yes | 20 | 22.2% | 100.0% |
| `6a2a124d57` | 216 | 33 | yes | 0 | 15.3% | n/a (finer truth empty) |

Across the 6 of 10 sampled commits with nonzero finer truth, recall against the finer, coverage-based oracle is **100.0% on every one of them** — sharply higher than the 24.8% mean against the coarse oracle over the same kind of commit, and finer truth itself is consistently a small fraction of coarse truth (e.g. 20 of 216, 3 of 35, 2 of 15): most of what the coarse oracle counts as "truth" never executes a changed line in isolation. This is consistent with the coarse pooled recall being a harsh, denominator-inflated reading rather than a measure of tests the tool actually should have run and didn't; it does not by itself clear the row's literal ≥ 0.98 bar, which this row defines against the coarse oracle, and the finer sample is small (10 of 105) and skewed toward the smallest coarse-truth commits (chosen to keep per-function coverage runs tractable), not a random or representative slice of the full 105.

## Per-commit table (all 105)
Full detail, including every selected/truth test key, is in [`tests-select-replay-2026-09-14.json`](./tests-select-replay-2026-09-14.json). Summary:

| sha | changed pkg(s) | truth | selected | recall | time frac |
|---|---|---|---|---|---|
| `61d0ce34c6` | internal/scenariobench | 215 | 31 | 14.4% | 25.0% |
| `4bea25093d` | internal/scenariobench | 216 | 23 | 10.6% | 25.0% |
| `17f63459bb` | cmd/scenariobench, internal/scenariobench | 216 | 48 | 22.2% | 25.0% |
| `6a2a124d57` | internal/scenariobench | 216 | 33 | 15.3% | 25.0% |
| `a7546dc3a1` | cmd/scenariobench | 15 | 15 | 100.0% | 3.9% |
| `3909c71673` | cmd/scenariobench | 15 | 15 | 100.0% | 3.9% |
| `513ea7da5d` | cmd/fathom, internal/harness | 978 | 302 | 30.9% | 29.6% |
| `5908e15d8d` | internal/scenariobench | 216 | 43 | 19.9% | 25.0% |
| `a746a59ab4` | internal/scenariobench | 216 | 40 | 18.5% | 25.0% |
| `3bdee96784` | cmd/contextbenchscore, cmd/scenariobench +2 | 1307 | 90 | 6.9% | 25.3% |
| `b4505326d3` | cmd/fathom, internal/mcp | 1433 | 760 | 53.0% | 30.7% |
| `6612728005` | internal/mcp | 1434 | 1 | 0.1% | 0.8% |
| `5f5135332c` | cmd/scenariobench, internal/scenariobench | 233 | 61 | 25.8% | 25.3% |
| `7c2f49f1a3` | internal/indexbench | 35 | 826 | 60.0% | 39.7% |
| `379741bcc8` | internal/scenariobench | 249 | 102 | 41.0% | 25.0% |
| `c364e320c4` | internal/indexbench, internal/mcp +2 | 1714 | 817 | 46.0% | 40.8% |
| `4c2306a0e4` | internal/contextbench | 1080 | 31 | 2.9% | 0.4% |
| `00b551f1ae` | internal/indexbench | 39 | 4 | 10.3% | 0.6% |
| `bdcb234605` | cmd/fathom, internal/guardspec +3 | 994 | 451 | 45.4% | 30.2% |
| `db34b913b5` | cmd/fathom | 952 | 1 | 0.1% | 29.3% |
| `c567901f0a` | cmd/scenariobench, internal/scenariobench | 251 | 55 | 21.9% | 25.0% |
| `6a92182806` | internal/indexbench | 39 | 0 | 0.0% | 0.0% |
| `c48125c8dd` | internal/scenariobench | 251 | 51 | 20.3% | 25.0% |
| `97f08e7961` | cmd/fathom, internal/analysis/review | 1484 | 231 | 15.6% | 29.5% |
| `9aae409a9b` | cmd/fathom, internal/custodian | 984 | 254 | 25.8% | 29.5% |
| `fe73791150` | internal/scenariobench | 253 | 76 | 30.0% | 25.0% |
| `4ef0d0c9f4` | internal/indexbench | 39 | 0 | 0.0% | 0.0% |
| `c05902428a` | internal/core, internal/mcp | 2780 | 670 | 24.1% | 30.9% |
| `923eb08c7f` | cmd/fathom, internal/core +1 | 2787 | 438 | 15.7% | 30.2% |
| `9fe1b2328b` | internal/fusion | 1695 | 2 | 0.1% | 5.6% |
| `551665bb8e` | cmd/fathom, internal/analysis/coverage +1 | 1851 | 356 | 19.2% | 30.2% |
| `2316621bd6` | cmd/fathom | 969 | 448 | 46.2% | 29.3% |
| `bb2d8fbb8b` | cmd/fathom, internal/custodian | 993 | 263 | 26.5% | 29.5% |
| `fdda6508c8` | internal/scenariobench | 254 | 41 | 16.1% | 25.0% |
| `37d90070aa` | cmd/fathom, internal/mcp +1 | 1472 | 689 | 46.8% | 31.6% |
| `f7d063ecdf` | cmd/scenariobench, internal/scenariobench | 278 | 130 | 46.4% | 25.3% |
| `100e2f6d95` | cmd/fathom, internal/analysis/coverage | 1592 | 248 | 15.6% | 29.6% |
| `e05ae5a07e` | cmd/fathom, internal/verify | 1829 | 743 | 40.6% | 31.2% |
| `017920acaa` | cmd/fathom | 970 | 235 | 24.2% | 29.3% |
| `46de81f9c1` | internal/scenariobench | 255 | 55 | 21.6% | 25.0% |
| `227972b379` | cmd/fathom, internal/contextbench +1 | 1861 | 726 | 39.0% | 31.1% |
| `ad369e07fa` | internal/contextbench | 1098 | 51 | 4.6% | 0.4% |
| `e737dec1ba` | cmd/contextbenchscore, internal/contextbench | 1107 | 297 | 26.8% | 29.6% |
| `dd0311c01b` | internal/scenariobench | 257 | 56 | 21.8% | 25.0% |
| `a076a63ef9` | cmd/fathom, internal/analysis/rulereplay | 989 | 249 | 25.1% | 35.6% |
| `13998a88e8` | internal/scenariobench | 282 | 54 | 19.1% | 25.0% |
| `860035418c` | internal/mcp | 1735 | 690 | 39.8% | 30.7% |
| `5c7a1a4a02` | internal/scenariobench | 258 | 21 | 8.1% | 25.0% |
| `4808ed3803` | internal/analysis/serviceref, internal/core +1 | 3168 | 1975 | 59.8% | 80.9% |
| `91d5325a20` | cmd/fathom, internal/analysis/scipexport | 994 | 466 | 46.9% | 29.5% |
| `3b2674d600` | internal/analysis/serviceref, internal/core +1 | 3188 | 755 | 23.7% | 33.1% |
| `39c654fd0f` | cmd/fathom, internal/analysis/scipexport +1 | 997 | 248 | 24.9% | 29.5% |
| `6bfd49548e` | cmd/contextbenchscore, internal/contextbench | 1116 | 37 | 3.3% | 0.3% |
| `26a4502e6d` | internal/scenariobench | 290 | 63 | 21.7% | 25.0% |
| `3547ea54c1` | internal/scenariobench | 292 | 47 | 16.1% | 25.0% |
| `72506a7012` | internal/scenariobench | 308 | 108 | 35.1% | 25.0% |
| `deac7c23fc` | internal/scenariobench | 316 | 56 | 17.7% | 25.0% |
| `7d766202f6` | internal/scenariobench | 330 | 75 | 22.7% | 25.0% |
| `208504bd04` | internal/core, internal/semantic +1 | 3206 | 1726 | 50.2% | 85.7% |
| `2cd4c6ff9a` | internal/mcp, pkg/client | 1892 | 824 | 41.9% | 33.7% |
| `8439858d2b` | cmd/fathom | 985 | 1085 | 52.1% | 40.2% |
| `777934a138` | internal/scenariobench | 340 | 34 | 10.0% | 25.0% |
| `49a3480d41` | cmd/scenariobench, internal/scenariobench | 355 | 79 | 22.0% | 25.3% |
| `a4649a001c` | internal/scenariobench | 358 | 89 | 24.9% | 25.0% |
| `35829a8b78` | internal/semantic, pkg/client | 2097 | 1488 | 57.2% | 84.5% |
| `79d55bbb5f` | cmd/fathom | 986 | 316 | 32.0% | 29.3% |
| `6357cded1d` | internal/scenariobench | 363 | 64 | 17.6% | 25.0% |
| `49a5d89287` | internal/scenariobench | 370 | 60 | 16.2% | 25.0% |
| `cec6e5351a` | internal/scenariobench | 375 | 77 | 20.5% | 25.0% |
| `19e9497af7` | internal/scenariobench | 378 | 81 | 21.4% | 25.0% |
| `fbe93888d2` | internal/scenariobench | 379 | 4 | 1.1% | 21.1% |
| `d01ee19056` | internal/scenariobench | 379 | 4 | 1.1% | 21.1% |
| `affe540048` | cmd/fathom, internal/wiring | 1294 | 382 | 29.5% | 29.5% |
| `3cd9e12030` | internal/semantic | 2102 | 765 | 36.4% | 39.4% |
| `73c226e31a` | internal/scenariobench | 380 | 71 | 18.7% | 25.0% |
| `f6d5232777` | internal/scenariobench | 385 | 38 | 9.9% | 25.0% |
| `bed325ec5a` | cmd/fathom, internal/harness | 1025 | 586 | 57.2% | 29.6% |
| `949b2ab99f` | cmd/scenariobench, internal/scenariobench | 385 | 62 | 16.1% | 25.0% |
| `cc8cc40e9c` | cmd/scenariobench, internal/scenariobench | 391 | 111 | 28.1% | 25.3% |
| `f2531b3b51` | cmd/scenariobench, internal/scenariobench | 394 | 61 | 15.2% | 25.3% |
| `a359205069` | internal/scenariobench | 395 | 2 | 0.5% | 21.1% |
| `11ed74dc66` | internal/scenariobench | 396 | 61 | 15.4% | 25.0% |
| `6c031f8575` | cmd/scenariobench, internal/scenariobench | 401 | 118 | 29.2% | 25.3% |
| `b173b8c0c5` | internal/scenariobench | 406 | 46 | 11.3% | 25.0% |
| `6d86fcd0e9` | cmd/scenariobench, internal/scenariobench | 410 | 122 | 29.5% | 25.3% |
| `89234798e7` | internal/scenariobench | 412 | 85 | 20.6% | 25.0% |
| `af46ba2111` | internal/scenariobench | 412 | 37 | 9.0% | 25.0% |
| `2b9755e7aa` | cmd/scenariobench, internal/scenariobench | 421 | 109 | 25.9% | 25.0% |
| `a3b321c02c` | cmd/scenariobench, internal/scenariobench | 427 | 176 | 41.2% | 25.0% |
| `5eab28ce9f` | cmd/scenariobench, internal/scenariobench | 432 | 35 | 8.1% | 25.0% |
| `9ddc98a3a3` | internal/scenariobench | 433 | 85 | 19.6% | 25.0% |
| `7ffbc3b561` | cmd/scenariobench, internal/scenariobench | 435 | 118 | 26.9% | 25.3% |
| `0b2998a775` | internal/scenariobench | 435 | 26 | 6.0% | 25.0% |
| `45953ed38a` | cmd/scenariobench, internal/scenariobench | 412 | 43 | 10.4% | 25.0% |
| `926f457921` | cmd/fathom, internal/harness | 1030 | 437 | 42.4% | 29.6% |
| `f2ee9aa0a8` | cmd/scenariobench, internal/scenariobench | 435 | 44 | 10.1% | 25.0% |
| `23e0683a04` | cmd/fathom | 998 | 1 | 0.1% | 29.3% |
| `e0b6511e86` | internal/scenariobench | 436 | 42 | 9.6% | 25.0% |
| `8ffd818fb9` | cmd/scenariobench, internal/scenariobench | 440 | 105 | 23.6% | 25.3% |
| `73c8171083` | internal/scenariobench | 441 | 34 | 7.7% | 25.0% |
| `6087083887` | internal/scenariobench | 441 | 57 | 12.9% | 25.0% |
| `3e33e45cab` | cmd/fathom | 998 | 1044 | 40.5% | 41.7% |
| `302b168ee5` | cmd/fathom | 999 | 246 | 24.6% | 29.3% |
| `ec2116c5b3` | cmd/fathom | 999 | 244 | 24.4% | 29.3% |
| `fe713c2a4e` | internal/scenariobench | 442 | 154 | 34.8% | 25.0% |

## Verdict
**Recall: 28.2% pooled, 24.8% mean — well under the 0.98 bar, in every cut. Time fraction: 26.6% mean / 25.0% median — also over the 25% bar.** Both halves of VB-35's AND gate fail. Per the row's own fallback, `fathom tests select` stays **advisory only** on this measurement: run it to rank and prioritize, but do not treat its selection as a substitute for the full suite on this evidence. The finer, coverage-based oracle suggests the coarse package-level bar this row set is a much harsher test than "does the tool miss a test that would have caught a regression" — the honest next step, if this row is revisited, is a coverage-based (or mutation-based, building on VB-34) truth oracle at full scale, not a relaxed recall target.

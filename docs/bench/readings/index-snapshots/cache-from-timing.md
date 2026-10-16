# IXS-M0 cache-from timing: method (fixed before any timed run, K19)

This section is committed before data. It is not edited after a timed run. The reading and
the table are appended below the `---` line only.

## The bar (verbatim from docs/plans/index-snapshots.md IXS-M0)

KEEP at a cache-from median of at most 0.6x cold; above it, F27 stays deferred and phases 1
and 2 do not start. The bar is never moved after data.

## Commit list rule

The 21 most recent first-parent commits of `main` ending at 99aa205a (v0.11.0):
`git log --first-parent --format=%H -21 99aa205a`, oldest first. The oldest is an untimed
seed (it supplies the previous store for the first measured commit). The other 20 are
measured. No commit is substituted after seeing data.

## Definitions (no new code)

- binary: one bin built once from 99aa205a, `go build -o <w>/bin/fathom ./cmd/fathom`;
  `fathom version --json` recorded.
- cold: a fresh-store ingest of a clean checkout of commit N (a new directory per commit,
  cloned from a local mirror, so mtimes and inodes are new).
- warm: ingest of commit N in one persistent checkout advanced from N-1 by `git checkout N`,
  into the store that already holds N-1 (the normal incremental path).
- cache-from: a copy of commit N-1's cold store, `fathom remap --store <copy> --service
  fathom --path <clean checkout of N>` (cmd/fathom/remap.go), then ingest of that checkout
  into the copy.

Every ingest is `fathom ingest --store <s> --service fathom --path <checkout>`, with a
private FATHOM_HOME, the shared warm Go build cache (state recorded) and GOPROXY=off once
GOMODCACHE is warm, so no run fetches.

## Run order

One ingest at a time, each under the wave ingest lock. For each commit the three runs go in
a rotating order (cold-warm-cachefrom, warm-cachefrom-cold, cachefrom-cold-warm, repeating)
to cancel drift. Only N-1's stores and checkouts are kept. Per run: wall seconds
(`/usr/bin/time -l`), peak RSS, exit code, any `error: ingest degraded:` line, store bytes,
load average, `fathom resources` host pressure, swap-volume free.

## Failure rule

A failed or degraded run is recorded, and the medians are over the successful runs with n
stated. No commit is substituted after seeing data. A cache-from ratio is taken per commit
only when both the cache-from and the cold run of that commit succeeded; the medians of the
ratios are over those commits.

## Disclosure

This ran locally on the owner's Mac from clean checkouts, not in CI, against the row's "in CI": fathom had no CI
runner when the program was fixed (design 9.3), and the Linux gate workflow that landed on 2026-10-15 runs the test
suite, not a 62-ingest timing program.

---

## Verdict

Cache-from does not clear the bar. Seeding a new commit from the previous commit's cold store (copy, remap, ingest) took a median of 164.2 s against 203.7 s for a cold ingest, 0.81x, well above the 0.6x bar, and about 2.0x the normal incremental (warm) ingest at 81.5 s. Where the diff is tiny (7 of 20 commits, cache-from 16 to 24 s) cache-from is as cheap as warm; where the diff touches code that fans out, cache-from pays about the same as a cold ingest minus roughly 20 percent, because a clean checkout's new mtimes and inodes miss the stat fast path. F27 stays deferred; IXS phases 1 and 2 do not start. The row is answered either way: the reading is the line below.

IXS-M0 reading: cache-from median 164.2 s = 0.81x cold (bar 0.6x: CUT), 2.0x warm; n=20 of 20

## Per-commit table (seconds, wall)

| # | commit | cold s | warm s | cache-from s | cf/cold | cf/warm |
|--:|---|--:|--:|--:|--:|--:|
| 1 | 56746ba9 | 1133.89 | 178.79 | 190.34 | 0.168 | 1.065 |
| 2 | 3d4c0bda | 206.96 | 94.22 | 174.26 | 0.842 | 1.850 |
| 3 | fe6101dd | 188.10 | 138.85 | 167.64 | 0.891 | 1.207 |
| 4 | c39875f3 | 223.98 | 64.09 | 162.43 | 0.725 | 2.534 |
| 5 | 0b01f9c2 | 187.57 | 98.92 | 165.26 | 0.881 | 1.671 |
| 6 | e1fe5b8f | 199.64 | 118.62 | 160.84 | 0.806 | 1.356 |
| 7 | 4a5eba01 | 195.66 | 126.29 | 166.24 | 0.850 | 1.316 |
| 8 | ebc51e51 | 206.51 | 121.55 | 168.10 | 0.814 | 1.383 |
| 9 | 6a12e001 | 190.58 | 109.78 | 171.17 | 0.898 | 1.559 |
| 10 | d3f748e8 | 208.29 | 35.46 | 175.82 | 0.844 | 4.958 |
| 11 | e32430b5 | 187.84 | 21.72 | 19.07 | 0.102 | 0.878 |
| 12 | e01ca120 | 188.12 | 111.31 | 171.67 | 0.913 | 1.542 |
| 13 | 65e5acfa | 191.82 | 151.06 | 167.35 | 0.872 | 1.108 |
| 14 | 76040f10 | 195.98 | 20.09 | 16.20 | 0.083 | 0.806 |
| 15 | 62308ce0 | 216.71 | 19.41 | 17.54 | 0.081 | 0.904 |
| 16 | 302535e3 | 212.80 | 20.64 | 23.80 | 0.112 | 1.153 |
| 17 | a627ecca | 215.35 | 21.02 | 18.74 | 0.087 | 0.892 |
| 18 | ed0e0040 | 211.64 | 21.21 | 19.75 | 0.093 | 0.931 |
| 19 | 0e8bf109 | 214.72 | 17.89 | 17.65 | 0.082 | 0.987 |
| 20 | 99aa205a | 200.79 | 68.76 | 163.09 | 0.812 | 2.372 |

median cold 203.65 s (n=20), warm 81.49 s (n=20), cache-from 164.175 s (n=20)
median of per-commit cf/cold 0.808946 (n=20), cf/warm 1.26184 (n=20)
ratio of medians: cache-from/cold 0.806, cache-from/warm 2.015

Ratio of medians is the headline (164.175 / 203.65 = 0.806); the median of per-commit ratios is 0.809. Either is above 0.6. Commit 1 cold (1133.9 s) ran under host load from sibling waves; dropping it does not change the medians materially (cold median would be 206.5 s).

## Runs

62 ingests (2 seed + 60 measured: 20 commits x 3 modes), all exit 0, none degraded, n=20 of 20 per mode. No commit substituted. Order rotated per commit (cold-warm-cachefrom, warm-cachefrom-cold, cachefrom-cold-warm). Raw per-run rows (exit, degraded, RSS, store bytes, load, pressure, swap free) are in results.tsv under the wave scratch dir; peak RSS about 2.1 GB cold, 1.7 to 1.8 GB warm and cache-from.

## Environment

```
binary: {"dirty":false,"revision":"99aa205a8f4e2d36a279f26e958729ba55c53d8b","revision_time":"2026-10-11T23:19:00Z","tenancy_contract":1,"version":"fathom v0.11.0"}
go: go version go1.26.3 darwin/arm64  GOCACHE=$BUILD_ROOT/gocache-ew (14.5421 GiB)  GOMODCACHE=$BUILD_ROOT/gomodcache
host: 27.0.1 arm64 cores=10
```

Commits (oldest first, index 0 is the untimed seed):

```
0 4c413f98804518d522a750a2e32316a26abde380
1 56746ba91e59e26484895e3701492aadc04ab504
2 3d4c0bda1779b735f196979833a7ed0a3f89bede
3 fe6101dd7c9a0a630ff2acce5ccc6e3edb8e5896
4 c39875f301d06fe7d555673a1613cb9a601c567c
5 0b01f9c23eb8bf88659d891b8c80bbef0e8de28a
6 e1fe5b8f6ca70f006bf079cab9c687929912dc0c
7 4a5eba013b8df62f97ee54535faa44b0fc3c989d
8 ebc51e51ea3fd3e14a76de602c4d804f3a648b8d
9 6a12e001cb2ef05115dd367a8f44f06be9ef6c64
10 d3f748e8dd3766768e3e8449743d9d86d56ecf53
11 e32430b57dad9cfc120e1e5b7d4e52d0fdac62cf
12 e01ca1201d9e26ac7ea5c6f9d1cbb8cd5649b1ec
13 65e5acfa8724322b295d55b0628037bd0d8e9587
14 76040f10469b0d64019b933f9d6175d0a775d81b
15 62308ce0951743737442bf69ede96c76055527ec
16 302535e38c0e16d4f37b110fabce9ff7f2b6d40f
17 a627eccad1942d328ff306e856b2cf32e0245e4d
18 ed0e0040bf2bb300edae47bfcb131594dfc18019
19 0e8bf1094c13edf5c23143979bf3e5a7b2ba6310
20 99aa205a8f4e2d36a279f26e958729ba55c53d8b
```

Commands: `cache-from-timing.sh run` (this directory), which per ingest runs `lockf -k ingest.lock /usr/bin/time -l env GOPROXY=off fathom ingest --store <s> --service fathom --path <checkout>`; cache-from adds `fathom remap --store <copy> --service fathom --path <checkout>` after copying the previous cold store. The harness was interrupted once by a machine reboot after the seed ingests and resumed from results.tsv; no timed row was lost or repeated.

Disclosure: this ran locally on the owner's Mac from clean checkouts, not in CI, against the row's "in CI" (no CI runner existed when the program was fixed, design 9.3; the Linux gate workflow of 2026-10-15 runs the test suite only).

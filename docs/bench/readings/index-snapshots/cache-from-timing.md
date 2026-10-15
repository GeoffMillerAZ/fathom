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

This ran locally on the owner's Mac from clean checkouts, not in CI (fathom has no CI
runner, design 9.3), against the row's "in CI".

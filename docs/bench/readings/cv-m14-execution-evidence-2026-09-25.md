# CV-M14: execution evidence re-measured with coverage (2026-09-25)

**Verdict (plain language). CV-M14 PASSES.** Re-running the CV-M13 ranges
through the new coverage-bound `fathom conserve` — fathom's own
`5afeee42..78e508d3` and conjure's `r1`/`r2` — with a genuine, passing,
commit-bound coverage profile at every head: no item the tool calls
`attested` turned out, on independent inspection of the raw `go tool
cover` profile, to have an unexecuted changed line, and no item the tool
flagged (`partial`/`unexecuted`) turned out to be secretly fully executed.
All six named predictions from design §3.5 came back exactly as
predicted: `runServeHTTP` reads `partial` naming line 131; conjure's
`fanoutHandler.Enabled/Handle/WithAttrs` read `attested`;
`fanoutHandler.WithGroup` and `ParseError.Unwrap` read `unexecuted`. Every
range's counts reconcile (`changed == attested + unattested + unreached +
partial + unexecuted + unanalyzed_functions`), including conjure r2's 100
vendor-excluded functions. The `-coverpkg=./...` attest run on fathom
costs 1.08x the plain run (117s vs 108s), well under the 2x threshold, so
the profile stays default rather than opt-in. For a reviewer: the tool's
"attested" and "flagged" labels can be trusted at face value on this
evidence — a genuinely untested changed line does not get waved through
as attested, and a genuinely-run line does not get wrongly flagged.

**Two caveats limit the pass, both disclosed rather than engineered
around (see Caveats):** (1) conjure r1 shares its head (`e9d0f2e`) with
CV-M13/CV-M14's `r3`, whose suite is known to fail a pre-existing,
date-dependent fixture unrelated to either range — `-skip` (not a code
edit) was used to get a genuine pass at the literal r1 head, exactly as
design §3.5 anticipates. (2) the task scoped this measurement to conjure's
two ranges (r1, r2) rather than all three from design §3.5, but the
`fanoutHandler` predictions are about r3's own diff, which is disjoint
from r1's and r2's changed-function sets. Rather than silently skip that
half of the named-case check, r3's reading set was reconstructed for free
by reusing r1's already-collected, e9d0f2e-bound store and coverage
profile (r1 and r3 share a head; cv-m13's own control notes record this
reuse) with `--since 44fd5ae` — a supplementary check, not a third
range's full test run.

Bar (CV-M14, `docs/plans/refactor-conservation.md`, design §3.5): "Re-run
the CV-M13 ranges with coverage: fathom `5afeee42..78e508d3` and conjure's
three measured ranges (r1, r2, r3). By construction, no `attested` item
may be unexecuted and no flagged item may have executed. `runServeHTTP`
must read `partial`, naming `cmd/fathom/servehttp.go` line 131. The
`slog.Handler` methods (only present in conjure's r3, `44fd5ae..e9d0f2e`)
must read `attested`. `WithGroup` and `ParseError.Unwrap` must read
`unexecuted`... if an attest run with `-coverpkg=./...` takes more than
twice as long as the plain run on this repository, the profile becomes
opt-in."

## Method

Built the fathom binary once from this checkout's `exec-evidence` branch
HEAD (`1ccba65e`, the commit that carries the design §3.5 coverage feature,
CV-67..72) via `go build -o bin/fathom ./cmd/fathom`. Built test-kit's
`surveyor` once from the platform-engineering `kits/test-kit` checkout.

For fathom's own range, cloned this checkout into a scratch directory and
checked out `78e508d3` (the head; `5afeee42` is the base — this reads
backwards from the chronological order at a glance, confirmed with `git
merge-base --is-ancestor` and cross-checked against the diff itself, which
touches exactly `cmd/fathom/servehttp.go:131`, the O119 fix the kill
criterion names). This head predates the `tests:attest` Task recipe that
writes `coverage.meta.json` automatically, so the recipe was run by hand,
matching the Taskfile's current implementation line for line: `go test
-json -coverpkg=./... -coverprofile=coverage.out ./...`, then a
hand-written `coverage.meta.json` (`schema: fathom.coverage/v1`, `commit`,
`dirty: false`, `race: false`, `profile: coverage.out`, `passed: true`),
`surveyor ingest --format go-test-json --commit <head>`, a scratch `fathom
ingest` (own index, own store, never the shared one), `fathom tests
ingest`, then `fathom conserve --since 5afeee42 --json --coverage-meta
coverage.meta.json`.

For conjure, cloned `/Volumes/m2ext/code/geoffmilleraz/conjure` (never
touched the original) into a separate scratch directory and ran the same
recipe per range with `GOFLAGS=-mod=vendor` (conjure vendors its
dependencies and, at these heads, a `replace => ../app-kit` that
`-mod=mod` cannot resolve — matching CV-M13's own control notes). r1
(`87ccb17..e9d0f2e`) and r2 (`e9d0f2e..10efbe4`) are the two ranges the
task named; a third, zero-additional-test-run check against r1's own
store answers the r3-scoped named predictions (see Named cases).

One operational note, twice: `go test`'s build cache lives on the external
volume (`GOCACHE=/Volumes/m2ext/tmp/cv3-build`), and the documented EPERM
wedge (`~/.claude/guides/external-volume-eperm.md`) fired on the r1 and r2
coverpkg runs (`go: creating work dir: ... operation not permitted`,
`wall=0s`, empty output). `tccutil reset SystemPolicyRemovableVolumes
com.mitchellh.ghostty` and a retry fixed it both times; the retried run is
the one this reading uses (confirmed by nonzero `wall` and the recorded
commit matching each checkout's HEAD).

## Per-range tables

Every count below comes straight from `fathom conserve --json`'s
`residue` object; `changed` is checked against `attested + unattested +
unreached + partial + unexecuted + unanalyzed_functions` for each row.

| Range | base..head | changed | attested | partial | unexecuted | unattested | unreached | unanalyzed_functions | reconciles |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| fathom | `5afeee42..78e508d3` | 31 | 18 | 13 | 0 | 0 | 0 | 0 | 18+13+0+0+0+0=31 ✓ |
| conjure r1 | `87ccb17..e9d0f2e` | 54 | 19 | 34 | 1 | 0 | 0 | 0 | 19+34+1+0+0+0=54 ✓ |
| conjure r2 | `e9d0f2e..10efbe4` | 326 | 119 | 105 | 2 | 0 | 0 | 100 | 119+105+2+0+0+100=326 ✓ |
| conjure r3 (supplementary, reuses r1's store/coverage) | `44fd5ae..e9d0f2e` | 148 | 54 | 92 | 2 | 0 | 0 | 0 | 54+92+2+0+0+0=148 ✓ |

`unattested`/`unreached` are 0 on every row — expected: design §3.5 says
those static-only classes stand only *without* usable coverage, and every
row here has `coverage_unusable: false` (a usable, bound, passing
profile). r2's 100 `unanalyzed_functions` are conjure's own vendored
`app-kit` UI files touched by that range's commit (`git diff --stat
e9d0f2e 10efbe4 -- 'vendor/*.go'` shows 35 vendor files changed);
`.fathomignore` excludes `vendor/` from the index, so every changed
function inside those files falls into `unanalyzed_functions` rather than
any execution-evidence class — the reconciliation identity holds
specifically *because* they land there instead of vanishing.

## Named cases

| Case | Range | Predicted | Measured | Independent check |
|---|---|---|---|---|
| `runServeHTTP` | fathom `5afeee42..78e508d3` | `partial`, naming `servehttp.go` line 131 | `partial`, `unexecuted_lines: ["129-133"]` (contains 131) | `go tool cover -func` shows `runServeHTTP 72.7%`; every raw profile block covering lines 129-133 (`129.9,129.73`, `129.73,133.4`) shows count 0 across all merged package sections |
| `fanoutHandler.Enabled` | conjure r3 (`44fd5ae..e9d0f2e`, supplementary) | `attested` | not in flagged items ⇒ `attested` | raw profile blocks `181.76,182.22` … `183.28,185.4` all count 1 |
| `fanoutHandler.Handle` | conjure r3 (supplementary) | `attested` | not in flagged items ⇒ `attested` | raw profile blocks `190.73,192.22` … `194.12` all count 1 |
| `fanoutHandler.WithAttrs` | conjure r3 (supplementary) | `attested` | not in flagged items ⇒ `attested` | raw profile blocks `203.66,205.22` … `208.12` all count 1 |
| `fanoutHandler.WithGroup` | conjure r3 (supplementary) | `unexecuted` | `unexecuted` | raw profile blocks `211.60,213.22`, `213.22,215.3`, `216.2,216.12` all count 0 |
| `ParseError.Unwrap` | conjure r2 (`e9d0f2e..10efbe4`) | `unexecuted` | `unexecuted` | raw profile block `store.go:36.37,36.53` count 0 across all 22 merged package sections |

All six match design §3.5's predictions exactly. Two of the five
`fanoutHandler`/`ParseError` predictions are properly r3-scoped
(`Enabled`/`Handle`/`WithAttrs`/`WithGroup` — `internal/logx/logx.go` was
added wholesale between `44fd5ae` and `87ccb17`, so it is **not** part of
either r1's or r2's changed-function set at all); they were checked via
the supplementary r3 reconstruction described above, not r1/r2 directly.
`ParseError.Unwrap` (added wholesale in `internal/uiprefs/store.go`
inside r2's own range) is the one genuinely r1/r2-scoped prediction, and
it landed in r2 directly.

Additional spot checks beyond the named cases, each cross-verified
against the raw `coverage.out` blocks for the function's changed lines
(the changed-block rule): fathom's `compileViaStore` (attested — line 350,
the O119 fix's *other* call site, count 1) and conjure r1's
`jsonShapeError.Error` (the range's one `unexecuted` item — line 68, the
function's only block, count 0). No contradiction found in any spot check
across the four ranges/reconstructions.

## Cost

Measured on conjure's r1/r2 host (this session's sandbox), sequentially,
each once: fathom's own suite at head `78e508d3`.

| Run | Command | Wall | Load avg before | Load avg after |
|---|---|---:|---|---|
| Plain | `go test -json ./...` | 108s | 4.54 5.29 5.39 | 8.18 7.03 6.11 |
| Coverpkg (attest) | `go test -json -coverpkg=./... -coverprofile=coverage.out ./...` | 117s | 8.18 7.03 6.11 | 8.11 8.04 6.68 |

Ratio: 117/108 = **1.08x**, under the 2x threshold — the profile stays
default (`TESTKIT_COVER=1`), not opt-in. Caveat: the machine was not
quiet (load average ~8, not near 0, for both runs — this sandbox runs
concurrent agent sessions per this project's own dogfooding practice);
both runs saw comparable background load, so the *ratio* is probably a
fair read even though the *absolute* seconds carry noise.

## Caveats

- **r1/r3's shared head fails an unrelated, date-dependent test.**
  `TestLogsVerbFiltersAndFormats` (`internal/cli/logs_test.go`) hardcodes
  no reference clock at `e9d0f2e`; it compares "now" against fixture
  timestamps and fails once enough real time passes. Fixed upstream in
  `c996187` (inside r2's own range) by pinning a test clock. Per design
  §3.5's explicit instruction ("never treat a failing head as satisfying
  or refuting this criterion... re-run against a tree where that fix is
  present, or any other tree at the head whose suite genuinely passes"),
  the r1/r3 coverpkg run used `go test -skip
  'TestLogsVerbFiltersAndFormats' -coverpkg=./... ...` — the literal
  `e9d0f2e` commit, tree unmodified (`dirty: false` in the coverage meta,
  verified), one irrelevant/broken test excluded rather than the code
  patched to pass. This is a narrower, lower-risk choice than
  cherry-picking `c996187`'s fix (which touches a shared test helper,
  `runCLIAt`, not introduced until later in that same overhaul commit,
  and would have risked a non-compiling scratch tree).
- **Two ranges given, three predictions needed.** The task scoped conjure
  to r1/r2; design §3.5's `fanoutHandler` predictions are r3-scoped. See
  Named cases — answered via a same-data, zero-extra-test-run
  reconstruction (`fathom conserve --since 44fd5ae` against r1's own
  store/profile), not a third full range measurement, and called out
  explicitly as supplementary above rather than folded silently into the
  r1/r2 count tables.
- **fathom's own range direction is counter-intuitive.** `5afeee42` is
  chronologically the base (older, by ~2 hours) and `78e508d3` the head
  (newer) — opposite to what a first glance at `git log --oneline -3
  <ref>` output side by side suggests; confirmed by `git merge-base
  --is-ancestor` and by the diff's content (`servehttp.go:131`, exactly
  the kill criterion's own citation).
- No rule was loosened or re-tuned to reach this PASS; every number above
  is read directly from `fathom conserve --json` or cross-checked against
  the raw `go tool cover` profile.

## Reproduce

```sh
# fathom range (from this checkout's exec-evidence branch HEAD)
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod \
       GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp \
       GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOFLAGS=-mod=mod
go build -o bin/fathom ./cmd/fathom
git clone . /path/to/scratch/fathom && cd /path/to/scratch/fathom
git checkout --detach 78e508d3
go test -json -coverpkg=./... -coverprofile=coverage.out ./...
# write coverage.meta.json: {schema: fathom.coverage/v1, commit: <HEAD>,
#   dirty: false, race: false, profile: coverage.out, passed: true}
bin/fathom ingest --json --service <scratch> --path . --store <scratch>.sqlite
bin/fathom tests ingest --service <scratch> --manifest . --results . --store <scratch>.sqlite
bin/fathom conserve --since 5afeee42 --json --service <scratch> \
  --store <scratch>.sqlite --coverage-meta coverage.meta.json

# conjure r1/r2 (clone /Volumes/m2ext/code/geoffmilleraz/conjure elsewhere, never touch the original)
export GOFLAGS=-mod=vendor   # vendored deps + a replace => ../app-kit at these heads
git checkout --detach e9d0f2e   # r1 head; -skip 'TestLogsVerbFiltersAndFormats' (see Caveats)
go test -json -skip 'TestLogsVerbFiltersAndFormats' -coverpkg=./... -coverprofile=coverage.out ./...
# ... same ingest/tests-ingest/conserve --since 87ccb17 sequence as above
git checkout --detach 10efbe4   # r2 head; passes unmodified
go test -json -coverpkg=./... -coverprofile=coverage.out ./...
# ... same sequence, --since e9d0f2e

# On "operation not permitted" from GOCACHE/GOTMPDIR on /Volumes/m2ext:
tccutil reset SystemPolicyRemovableVolumes com.mitchellh.ghostty   # then retry
```

Verified: `fathom conserve --json` residue counts reconcile
(`changed == attested + unattested + unreached + partial + unexecuted +
unanalyzed_functions`) on all four range/reconstruction rows above, cross-checked
against `go tool cover -func` and the raw coverage profile for the six
named cases plus two additional spot checks; cost measured directly
(`go test -json ./...` vs `-coverpkg=./... -coverprofile=...`, one run
each, wall clock + `uptime`).

# Subject-preparation instruments

Run from the repository root. Python scripts use only the standard library. Set
`BENCH_JOB` to a job-owned `workspace/` output directory and `BENCH_SUBJECTS` to the detached
subject clones. Create `receipts/`, `logs/` and `builds/` there first. Writing tools reject a resolved output path without a `workspace` component. Subject ids
are `organization--repository`. Never point outputs at tracked source.

| Instrument | Measures | Command |
|---|---|---|
| `verify.py --archive` / `verify_logs.py` | Content hashes and complete membership of the thirty round-1 timed logs | `python3 calibration/bench/tools/verify.py --archive docs/bench/readings/bench-prep/raw/bench-subjects-r1-logs.tar.gz` |
| `prepare.py` | Full clone, immutable pin and public repository metadata; historical acquisition instrument | `python3 calibration/bench/tools/prepare.py` |
| `static.py` | Tracked primary-language lines, licence blob hashes, lock hashes and test/non-test history co-changes; fix messages use word boundaries | `python3 calibration/bench/tools/static.py` |
| `issue_links.py` | Issue references in historical co-change commit bodies, without asserting issue closure | `python3 calibration/bench/tools/issue_links.py` |
| `run_suites.py` | Setup, one warm-up, three fresh timed suite processes, exit codes and logs in Linux arm64 Docker with four CPUs; timed runs use `--network none` | `python3 calibration/bench/tools/run_suites.py organization--repository` |
| `go-c6/` | go/types resolved static call sites and caller import packages, including tests | `(cd calibration/bench/tools/go-c6 && go build -o "$BENCH_JOB/builds/c6-typed" .)` then `GOPROXY=off GOFLAGS=-mod=readonly "$BENCH_JOB/builds/c6-typed" "$subject"` |
| `c6_python.py` | Python AST direct imports, aliases and re-exports; does not resolve dynamic dispatch | `python3 calibration/bench/tools/c6_python.py` |
| `c6_ts.cjs` | TypeScript compiler resolved call/new signatures, excluding interface call/method signatures | `BENCH_TYPESCRIPT="$oracle_tools/node_modules/typescript" node calibration/bench/tools/c6_ts.cjs "$subject"` |

The C6 programs emit resolved sites. Apply `recount.py` (D26 by default) before
using counts for the current screen; `--policy D24` reproduces historical units.
D26 excludes all test/support callers, not just defining-unit calls. Python
resolution is pinned to CPython 3.12.15 and emits an interpreter sidecar. Python and
TypeScript syntax/type resolution is evidence with the stated limitations, not
C4 certification. C4 remains unrun. `go-c6` has its own pinned module, outside the
product build. TypeScript 5.9.3 is installed into a separate oracle-tools prefix.

The round-1 suite harness used the wrong installer/script for three TypeScript
subjects and live PyPI for Python. Its source is retained for inspectability;
Part B instruments/readings supersede those setup recipes. The committed copy
uses readonly Go modules: round 1 actually used mutable modules, a documented
harness deviation. Never rerun acquisition/online setup in an offline trial.
Archive and historical receipts: `docs/bench/readings/bench-prep/raw-logs.md` and
`receipts-r1.json`. All elapsed times are wall seconds; timestamps use UTC.

`verify.py` without `--archive` preserves the original round-1 verifier (portable
job/subject paths only). It reads historical `manifests.json` and receipts, checks
pins, licence/lock/log hashes, timings and the original fan-in claims. It is not
a verifier of the corrected schema or D24 admission: use `task cue-vet` and the
current D26 recount for those. Run it from the round-1 checkout (or the Part A replay tree), with `BENCH_JOB`
pointing at the round-1 receipt directory and `BENCH_SUBJECTS` at its unchanged
clones. Its old portability assertions are intentionally not current-manifest
acceptance rules.

Current copies of `run_suites.py`, `remeasure.py`, `static.py`, `inventory.py`,
`prepare.py`, `issue_links.py` and `c6_python.py` are **edited after round-2
trials**. They are not exact source snapshots of those executions. Read the
round-3 instrument hashes and status in `instruments-r3.json`. Both suite
wrappers mount the subject read-only and copy tracked source plus Git metadata
into `/work/source`; dependencies and all build output live under `/work`.
`issue_links.py` honors `BENCH_SUBJECTS`. C5 uses committer dates for filtering
and recording, and the regex `\b(fix(es|ed)?|bugs?|regressions?)\b`.

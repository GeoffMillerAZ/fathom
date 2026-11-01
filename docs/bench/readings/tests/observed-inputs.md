TQ-M8 verdict: FAIL

# What tests read: the six failed-gate triggers, before and after the observation (TQ-M8, ADR-090)

Plain-language verdict: before the observation, `fathom tests select --safe` named the failing test
for none of the six reproducible triggers behind this repository's failed pre-push gates of
2026-10-26 to 10-31. With Go's testlog observed per package and per test, it names it for four of
the six: seven of the eight failing tests are selected. The two misses are one test,
`TestCUDADriverOffline`, which reads the CUDA reading and its notes script through a Python
subprocess the testlog cannot see. The bar set before the run was five of six, so the reading is a
FAIL; the subprocess blind spot is the next row (TQ-40). Worth keeping: it catches the class that
failed seven of the nine test-failure gates, at about 3 s on a 45 s selection, and the full gate
still runs everything.

Measured 2026-10-31 on this repository at 4c6a0c01a, on the development Mac under load 14 to 16.
The observation: `scripts/test-inputs.sh` (`task test:inputs`) in a scratch worktree at HEAD, 171
packages listed, 156 with tests, 13 run test by test (packages that ran 20 s or more: 2,674 tests in
`cmd/fathom`), 1,238 s, 5,910 logs, 291 MB. The selection: a binary built from the change, `tests
select --since HEAD --safe --json` over a one-file edit in a scratch worktree, reading the daemon's
store (index at HEAD) and the observation.

## The triggers

| Trigger (the one-file edit) | Failing test (package) | Before | After |
|---|---|---|---|
| `docs/plans/language-breadth.md` | `TestRepoPlans_EveryIDBlockParsesAsARow` (internal/verify) | missed | selected (package whole) |
| `CHANGELOG.md` | `TestReleaseMatchesNewestChangelogHeading` (internal/buildinfo) | missed | selected (package whole) |
| `CHANGELOG.md` | `TestVersionMatchesNewestReleasedChangelogBlock` (cmd/fathom) | missed | selected (by name) |
| `docs/bench/readings/lang/cuda.json` | `TestCUDADriverOffline` (cmd/indexbench) | missed | missed: a Python subprocess reads it |
| `calibration/oracles/scip-clang/cuda-reading-notes.py` | `TestCUDADriverOffline` (cmd/indexbench) | missed | missed: a Python subprocess imports it |
| `ADR.md` | `TestGuardsCorpusDetectorsExist` (internal/scenariobench) | missed | selected (by name) |
| `ADR.md` | `TestHomeCorpusTruthAtHead` (internal/contextbench) | missed | selected (package whole) |
| a function body in `internal/mcp/server.go` | `TestTenancyRules` (internal/lint/tenancy) | missed | selected (package whole) |

Four of six triggers, seven of eight tests. The 2026-10-26 to 10-31 gates also failed on
`TestDaemonInstallDefaultsToLoopback` twice (code it calls, outside this reading),
`TestSynthesizeRoundTrips` (selected before through TQ-24's literal read) and
`TestEveryPackageWithDaemonDefaultsIsIsolated` (a walk of the packages; not reproduced here).

## What the selection runs

| Trigger | Named tests | Whole packages |
|---|---|---|
| plans | 17 | 5 (intent, langbench, recordids, specdialect, verify) |
| changelog | 9 | 2 (buildinfo, intent) |
| reading | 16 | 1 (langbench) |
| notes script | 2 | 0 |
| ADR.md | 19 | 5 (adr, contextbench, intent, recordids, semantic) |
| `internal/mcp/server.go` body | 1,430 (the call graph's reach) | 10 (packages whose tests read the whole source tree) |

At package level only, the same Go edit selected 14 packages whole, `cmd/fathom` among them
(10,533 repository files read outside the package). Per test, `cmd/fathom` runs by name. Six
`cmd/fathom` tests failed alone in the observation's environment; they stand on the package's log
and are selected for any file it read.

## Cost

`tests select` took 43 to 48 s with the observation and 42 to 46 s without it, two runs each on the
same diff: loading 291 MB of logs adds about 3 s; the call graph is the rest.

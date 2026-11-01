# Test intelligence: the tests that matter, ranked, selected and tied to contracts

Status: accepted 2026-09-27 (ADR-051 decision 2). Vision: `docs/vision/38-proof-layer-roadmap.md`
§4. Plan: `docs/plans/test-intelligence.md` (TQ-). Goal G23. Extends vision 33, ADR-043 and
`docs/design/test-certifications.md` without duplicating their rows: TC-1 to TC-24 are
shipped; TC-M2, TC-M3 and TC-M4 stay open in `docs/plans/test-certifications.md`; TC-25 (contract
certificates on cross-repo edges) is taken here as T5 with its trigger. Constraints carried
in: I43 (a certificate is never `live` without an attested passing run bound to a commit),
the honesty boundary of `test_reaches` (static reach, never a pass), I48 (no request
proportional to repository size), ADR-048 (end lines per language, which coverage mapping
needs), the retired churn and cognitive analyzers (not revived).

## 1. Problem

Coverage tools count lines. They cannot say which uncovered lines are dangerous, which tests
certify nothing, which tests a diff must re-run to re-prove what it touched, or which tests
exercise the library API an upgrade changed. Fathom holds the other halves of those joins:
centrality from the typed graph, governance severity from claims, incident history, liveness
from certificates, and (soon) links and dependency APIs from the system graph.

Owner decisions (2026-09-28), binding on T1:

- **Centrality (TQ-2)** is the transitive caller count over the SCC condensation of the call
  graph (syntax plus typed edges): one bitset union per component in topological order, never
  a BFS per symbol. It is computed at ingest and memoized per watermark: an ingest at the
  stored watermark that changed no input file keeps the stored rows.
- **Governance severity (TQ-3)** per file is 1.0 if an enforced guard (`enforce: true` in
  `.fathom/guard.cue`) scopes the file, 0.5 if any fresh obligation claim's scope matches it,
  and 0.1 otherwise. This replaces the rule-level mapping (`error`/`warn`) in the T1 table:
  obligation claims carry no level, and an enforced guard is the only rule that denies.

## 2. What is shipped and stays where it is

Certificates (test T of kind K at strength S certifies target X), the liveness states (live,
flaky, broken, unproven, unreachable), the `certify` verbs, the review dimension, Go coverage
profiles parsed into blocks and joined to symbols by layer (RC-37 onward), commit-bound
receipts. T7 (flakiness as staleness, receipts bound to commits) is already TC rows; this plan
adds only its measurement (TQ-M4). T8 (the requirement-linked tier) is `certified_by:`
(`docs/design/spec-interop.md` §4); TQ- rows reference it.

## 3. The items

### T1. Risk-weighted coverage

Per symbol: `risk = uncovered_share x centrality x (1 + churn) x severity x (1 + incidents)`,
each factor normalized to [0, 1] within the service, each shown separately so the rank is
explainable:

| Factor | Source |
|---|---|
| uncovered_share | coverage blocks joined to the symbol (RC-37 onward; T3 for other languages) |
| centrality | transitive caller count from the typed graph, normalized (the `blast_radius` walk, memoized at ingest) |
| churn | commits touching the symbol's file in 180 days, computed at ingest through `gitro` (a stored fact, never at request time; the retired churn analyzer is not revived) |
| severity | enforced guard scoping the file 1.0, fresh obligation claim 0.5, none 0.1 (§1 owner decision) |
| incidents | incident records (`docs/intent/incidents.md`, findings) whose evidence names the file |

Output: a ranked list, "untested and dangerous", each row with its factors and citations.
Surfaced through `test_impact` with `rank: risk` (no new tool) and a console view.

### T2. The test portfolio

Every test classified by what it certifies and reaches:

| Class | Rule |
|---|---|
| certifying | carries at least one live certificate |
| reaching governed code | reaches a symbol in a file with a governing rule, no certificate |
| orphan | certifies nothing and reaches nothing governed and nothing central (centrality below the service's 20th percentile) |
| redundant | same reach set and same certificate targets as another test (exact set equality; near-duplicates are not claimed) |
| critical target without a certificate | a symbol in the top decile of T1 risk with no live certificate (a gap, listed beside the tests) |

Per test, its purpose line: what it certifies, at which strength, or "reaches X, certifies
nothing".

### T3. Coverage for every language

One block model (file, start line, end line, statements, covered, covering tests when
present), parsers per format: lcov (`.info`), Cobertura XML, JaCoCo XML, coverage.py JSON,
Istanbul `coverage-final.json`, llvm-cov JSON export. Blocks join to symbols by line range,
which needs `EndLine` per language (ADR-048's L1 ranges, LB-F3); a language without ranges
joins at file level and says so. Ingested from a path the user or CI names (`fathom coverage
ingest <file>`), bound to the commit in the report or given on the command line; a report
without a commit is `unbound` and never counts toward a gate.

### T4. Certifying-first test selection

For a diff: the touched certificates are those whose target or reach intersects the changed
symbols. The selection is the minimal set of tests that re-proves every touched certificate
(a set cover over certificate targets; greedy by tests-per-certificate, deterministic
tie-break by name), plus every test that statically reaches a changed symbol when
`--safe` is set. A test among them whose only reach is a dispatch into error's `Error`,
`fmt.Stringer`'s `String` or `fmt.GoStringer`'s `GoString` is selected and counted, and
`--skip-stdlib-dispatch` leaves it out (O887). Output: the list, the certificates each re-proves, the tests skipped, and the
estimated CI time saved from the last attested run durations. Selection is advice to CI; it
never marks a certificate live (only an attested run does, I43).

### T5. Contract certificates on system-graph links

A cross-service link (`docs/design/system-graph.md`) is **certified** when a provider-side
test certifies the provider node and a consumer-side contract test (a consumer-driven contract
or a test against the provider's spec) certifies the consumer's call, both live. Uncertified
links at `spec` or `typed` tier are listed in the change risk receipt (U7). Takes TC-25 with its
trigger (TC-M2 passes and two fleet services declare port certificates) as the row's
`needs:`.

### T6. Dependency-upgrade test reach

For `contract_drift {pkg, from, to}`: the tests that statically reach our consumes of the
changed or removed `pkg:` symbols. "Run these before merging the bump", with the symbols each
test exercises.

### T9. What tests read, and the local loop (ADR-090)

T4 selects by what a test calls. A test that walks the source tree, globs `docs/plans` or reads
`CHANGELOG.md` reaches the changed file through no call edge and, for 1,209 of 1,359 reads in
this repository's tests, through no literal TQ-24 can match. Such tests failed seven of the nine
pre-push gates that failed on a test from 2026-10-26 to 10-31, and `tests select --safe` named the
failing test for none of the six triggers that reproduce.

- **The observation.** `task test:inputs` (`scripts/test-inputs.sh`) runs the suite once
  without `-race` and asks every test binary for Go's testlog (`-test.testlogfile`, the record
  `go test`'s own cache keys on: each open, stat, getenv and chdir through package os), through
  `go test -exec`. A package that ran 20 s or more is run again test by test from one
  `go test -c` binary, four processes at a time, one log per top-level test. Output:
  `workspace/artifacts/test-inputs/` (`meta.json` with the commit, `pkg/<dir>/_package.log`,
  `pkg/<dir>/_test.<Name>.log`), replaced whole. `internal/analysis/testinputs` reads it. It
  is a named file source, never a store table, and it records one run at one commit: about 20
  minutes and 290 MB on this repository. A read a test makes through a subprocess (a Python script
  it runs, a test binary re-run as a helper) is not in it: TQ-M8 read four of six triggers for that
  reason (TQ-40).
- **The join.** A changed file selects every test seen opening or stat'ing it. An added,
  deleted or renamed file also selects every test that opened the deepest directory that already
  existed at `--since`. A package runs whole (`go test ./<pkg>`, `"whole": true`) when it was
  observed only whole, when its log saw a read no test's own log explains, or when its log is not
  whole (missing, or cut short by a panic or an exit; `partial_packages`). The changed files include
  untracked files and both sides of a rename. A Test function in a changed test file that the
  index does not hold is selected by name (`new_tests`). With no `--store`, the selection reads
  the store the daemon records, and the JSON names it (`store`).
- **The runner.** `task test:affected` (`scripts/test-affected.sh`) selects for the diff since
  the merge-base with origin/main and runs it with `task t`'s isolation. It widens on doubt: no
  fathom, a failed or unreadable select, or no observation (a Go edit too: a test may walk the
  source tree) runs `task test`, and a changed shell script also runs `task scripts-check`. A
  linked worktree reads the main checkout's observation. Fathom selects; the
  repository's task runs (ADR-043 §2).
- **The oracle.** The push gate still runs everything. When `task test` fails,
  `scripts/test-boundary.sh` scores the selection for the change range against the failing
  tests, prints `tests select boundary: K of M failing test(s) selected` and appends a row to
  `workspace/artifacts/tests-select/boundary.jsonl`. A miss names a dependency kind fathom does
  not see (TQ-M7).
- **Not built.** A content-hash result cache (Go's test cache is one at package level, the CI
  kit's `InputKey` at the package closure), a package resolver (ADR-063 O1), a runner verb, and
  a second test registry. The manifest's declared `inputs` checked against observed reads
  (TQ-38) and the gate's own results as attested runs (TQ-39) are deferred rows with triggers.

## 4. Tenancy and cost

Churn and centrality are computed at ingest and stored per symbol (additive columns or a side
table), so T1 at request time is a sort over stored factors for one service with a `LIMIT`.
Coverage ingest is a CLI verb reading one file the user names, streamed, capped at 256 MiB.
T4 reads certificates for the changed symbols only.

## 5. Measurement

| Reading | Threshold |
|---|---|
| TQ-M1 | risk-weighted coverage ranked on the home fleet; a human check of the top 20 per service (at least 14 judged dangerous and under-tested) |
| TQ-M2 | a replayed month of this repository's diffs: selection re-proves every touched certificate while running at most 40% of the suite, and misses no failure a full run caught |
| TQ-M3 | coverage ingested for at least 5 languages (Go plus four T3 formats), blocks joined to symbols |
| TQ-M4 | T7 measured: flaky certificates counted over 30 days; every live certificate's run bound to a commit |
| TQ-M5 | the portfolio on this repository: counts per class, and a human check of 10 orphans and 10 redundant pairs |
| TQ-M7 | T9 at the boundary: every failing pre-push `task test` over 14 days scored; recall 1.0 over at least 10 failing tests, each miss filed with the dependency kind it names |
| TQ-M8 | T9 on the six reproducible triggers of the failed gates of 2026-10-26 to 10-31: the failing test selected for at least five, against none before |

## 6. Jev, weighed

- **Where it could help:** proposing which critical target a new test should certify first.
- **Where it cannot:** classification, selection or liveness; all deterministic.
- **Cost:** a call per gap listing.
- **The measurement:** none planned; T1's deterministic rank is the baseline any proposal would
  have to beat in an A/B at n at least 3.

## 7. Refused

- Counting lines as the product.
- Selection that marks anything proven.
- Reviving the churn or cognitive analyzers.
- Near-duplicate detection by similarity (redundancy is exact set equality).
- Coverage reports fetched from a CI service by fathom (the user names the file).

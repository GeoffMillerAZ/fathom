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
`--safe` is set. Output: the list, the certificates each re-proves, the tests skipped, and the
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

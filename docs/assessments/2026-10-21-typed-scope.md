# Typed loading and downstream verification

Verdict: repair the scope boundary before interpreting CT-5's performance ratio.
The 10x guard, typed test reachability and failure disclosure remain required.

## Findings and dispositions

- **F1, fixed in code:** the syntax baseline honors connector exclusions, including
  this repository's `third_party/`, but CT-5 passed nil to typed enrichment and
  selected the entire module. Production passed the discovered files but discarded
  unrelated packages only after loading them. Select sorted package-directory
  patterns within each owned module before loading, retain the existing per-file
  filter, and pass the same discovered scope to both CT-5 runs. Nil remains an
  explicit whole-module request. Tests cover selected/ignored broken packages,
  nested modules, external test variants, resolution, caching and empty loads.
- **F2, fixed in code:** gate preflight starts its shared deadline before Git root
  discovery. Expiration during that discovery returned before the diagnostic and
  observation. A deterministic discovery seam reproduces the missing hint/row.
  On deadline only, the existing filesystem work-tree check permits an honest
  not-checked report. Non-repositories, cancellation and the tested `GIT_DIR`,
  `GIT_WORK_TREE` and `GIT_COMMON_DIR` redirects remain silent. The deadline is unchanged.
- **F3, rejected explanation:** cache trimming alone does not explain the failing
  downstream ratio. At source 8d11ad68 and candidate kit afb436d9, fresh external
  caches still failed at 19.56x after CLI build and 16.08x after CLI plus all-package
  build. The original unequal-cohort trials are not comparable to the repaired
  measurement. No threshold relaxation, fake metadata, replacement golden or
  cache-priming target is adopted.

## Proof boundary

Behavioral regressions failed before each repair and focused race checks pass.
The matched-cohort direct run with a fresh external compiler cache failed at
13.80x: syntax CPU 13.942883 s, typed CPU 192.473197 s, 464 packages,
zero load failures, 188,998 in-scope edges. The same-store cached typed run took
7.989458 CPU s and loaded no packages. This diagnostic did not run the declared
CLI/all-package builds first. Full `task validate` subsequently passed with the
ordinary configured compiler cache: the non-race CT-5 target passed (121.877 s),
fusion's complete race suite passed (304.403 s), all 2,234 command tests passed
in four shards, and the standalone build/stranger adoption/configuration checks
passed. Successful CT-5 output suppresses its exact passing CPU ratio. This
record does not establish production latency or an absolute subsecond SLA. The
archived July CT-5 result used a smaller corpus and a warm compiler cache.

These are bug repairs under P19 (work follows scope), I23 (typed test reachability),
I25 (failed/empty loads stay visible) and ADR-070 (mirror ownership), with no new
dependency, exported protocol, ownership rule or decision-model site.

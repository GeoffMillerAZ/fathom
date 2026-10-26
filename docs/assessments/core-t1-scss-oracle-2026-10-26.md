# SCSS oracle spans and complete-source calibration

The old L1 range ceiling was an adapter bug. Pinned sass-parser 0.4.57 exposes Dart Sass
`source.end` through its lazy source wrapper. The adapter wrote zero instead. Regression tests
now check inclusive ends, multiline declarations and selectors, loop-variable token spans,
comments, Unicode/escaped identifiers and CRLF. A generic, explicit preamble/load-path fallback
lets Sass evaluate test files using their own runner setup. Only successful reference compiles
supply import truth. External support files never become subject files.

The complete discovered calibration corpus, with no `-only` filter, reads L1-L3 excellent:

| Instrument | Before | After |
|---|---|---|
| Historical range agreement | unmeasured | 1, 4,340/4,340 |
| Full-source file-edge precision | .9323 | 1, 310/310 |
| Historical measured files | 220 | 234 discovered of 235 tracked SCSS files |
| Full-source symbol precision | .9649 | .9649 |
| Full-source call precision / recall | .9741 / 1 | .9741 / 1 |
| Faults | 0 | 0 |

The source pins are unchanged: bootstrap v5.3.8 and primer/css v22.3.2. The eight files the
reference parser rejects remain extracted and counted against symbol precision. Six Bootstrap
test files compile with the exact preamble from its committed test runner and sass-true 9.0.0.
The generated source excluded by ordinary discovery is disclosed in the reading. Three timing
runs on Apple silicon, under nice and an initial load below 9, read 2.20, 2.34 and 2.37 MB per
CPU-second, median 2.34. Exploratory sandbox timing was discarded when nice reported a priority
error; the committed timing was rerun with the required priority.

A fresh independent holdout was pinned with unchanged success criteria in ffa03ef61 before its
first oracle run: foundation-sites v6.9.0, commit f5c7d29db04e459d94a998df9d8fd4a4447e767b.
No rule was changed after that reading. Its full corpus reads L2, L1/L2 excellent and L3 unmet:
range agreement 1 (2,972/2,972), symbol recall .9927 and precision .9987, call recall .9985 and
precision .9559, import resolution 1, file-edge recall 1 and precision .8239 (117/142). Twenty-five
counted file pairs lack successful compile attestation. Median throughput is 2.74 MB per
CPU-second (2.76, 2.73, 2.74), zero faults. This is the remaining written oracle ceiling, not
proof that those file-resolver edges are wrong. A future pass needs subject-native compile
configuration frozen before another independent holdout. The older Angular holdout remains
historical under the old oracle and restricted source scope.

Reproduce with the pinned packages and corpus SHAs in the oracle README and corpus shard.
Use the subject runner's exact setup for Bootstrap, run `indexbench lang-oracle` without
`-only`, then `indexbench lang-retime -runs 3 -write`. Committed oracle JSON readings retain
subject SHAs, counts, exclusions, timing basis and setup disclosure. No extractor, epoch,
generated board or plan checkbox changed. The controller must regenerate the language board
and apply its integration gate.

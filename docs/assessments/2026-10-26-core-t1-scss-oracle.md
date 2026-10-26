# SCSS oracle spans and complete-source calibration

The old L1 range ceiling was an adapter bug. Pinned sass-parser 0.4.57 exposes Dart Sass
`source.end` through its lazy source wrapper. The adapter wrote zero instead. Regression tests
now check inclusive ends, multiline declarations and selectors, loop-variable token spans,
comments, Unicode/escaped identifiers and CRLF. A generic, explicit preamble/load-path fallback
lets Sass evaluate test files using their own runner setup. Only successful reference compiles
supply import truth. Stderr diagnostics list each path attested by the fallback so later
readings can audit its runner scope. External support files never become subject files.

Measured: the [canonical reading](../bench/readings/lang/scss.json) holds computed grades
and counts for the complete discovered corpus without a `-only` filter.

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
Ordinary production discovery excludes the vendored Bootstrap `scss/vendor/_rfs.scss`
and Foundation `scss/vendor/normalize.scss` through its `vendor/` directory rule.
The files are not generated and were not removed by a lane-only filter. Three timing
runs on Apple silicon, under nice and an initial load below 9, read 2.20, 2.34 and 2.37 MB per
CPU-second, median 2.34. Exploratory sandbox timing was discarded when nice reported a priority
error; the committed timing was rerun with the required priority.

A fresh independent holdout was pinned with unchanged success criteria in ffa03ef61 before its
first oracle run: foundation-sites v6.9.0, commit f5c7d29db04e459d94a998df9d8fd4a4447e767b.
No rule was changed after that reading. Its computed grades are in the [independent reading](../bench/readings/lang/holdout/scss-foundation.json). Measured evidence:
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

## Candidate inventory (X-R1)

| Candidate or route | Version | Evidence and remaining limit |
|---|---|---|
| Dart Sass first-party syntax tree, `sass-parser` | 0.4.57 | Its lazy `source.end` exists. The repaired adapter now attests ranges. On Foundation it rejects one source file, which remains counted. |
| Dart Sass individual-file compile and `loadedUrls` | 1.105.1 | Foundation: 59 successful compiles, 77 failures, 36 import statements without compile truth. This leaves 25 extracted file pairs unattested, not proven false. |
| Subject-runner preamble and external load paths | adapter at this branch; sass-true 9.0.0 | This route fixed Bootstrap using its exact committed runner setup before the holdout. Foundation was frozen without setup and received no tuning after its first reading. A new setup is a new experiment, not retrospective truth for this result. |
| Universal Ctags SCSS parser | 6.2.1 | Existing O851 records number-as-class tags, missed sibling classes and loop variables, and absent ends. It cannot replace first-party span truth. |
| Third-party SCSS grammar | P233/tree-sitter-scss v1.0.1, ee500b9c | Extraction grammar, not an independent oracle. Reusing its query to attest itself would prove nothing. |

X-R1 moves the frozen Foundation pin and criteria to
`docs/bench/scss-spans-holdout.yaml`. It is never an input to the canonical corpus loader.
The original first-reading identity and counts remain unchanged.

Runnable checks from committed files (with the README's pinned packages installed):

```sh
node --test calibration/oracles/dart-sass/sass_oracle.test.cjs
go test -race ./internal/langbench -run 'GrammarPrefillDataMatchesQueue|Reading|Corpus'
ruby -r yaml -e 'ARGV.each { |p| YAML.safe_load(File.read(p)) }' calibration/corpus.d/scss.yaml docs/bench/scss-spans-holdout.yaml
```

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / compiler | [Dart Sass](https://sass-lang.com/documentation/js-api/) is measured at 1.105.1 for successful `loadedUrls`; [registry 1.106.1](https://registry.npmjs.org/sass/latest) is newer but does not automatically attest failed Foundation compiles. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-scss` search](https://github.com/search?q=scip-scss&type=repositories) yielded no measured SCSS indexer. |
| Language server | [VS Code CSS/SCSS language service](https://github.com/microsoft/vscode-css-languageservice) has navigation services. **LIVE UNMEASURED** against Foundation’s 25 unattested file pairs. |
| Registry AST | [sass-parser registry 0.5.1](https://registry.npmjs.org/sass-parser/latest) postdates measured 0.4.57. [`sass_api` 18.1.0](https://pub.dev/api/packages/sass_api) exposes AST spans and [`findDependencies`](https://pub.dev/documentation/sass_api/latest/sass_api/) returns declared URLs, **not resolved pairs**. Spans are a possible independent check; none of these version hits closes the 25-pair gap. |
| Runtime/load tracing | [Dart Sass compile API](https://sass-lang.com/documentation/js-api/) reports loads for successful compiles only; failed individual files remain counted and unattested. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.

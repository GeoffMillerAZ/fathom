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
CPU-second (2.76, 2.73, 2.74), zero faults. This was the original measured oracle limit, not
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

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. The source-ledger lookup itself was read-only; the dated native resolver measurement below records subsequent execution.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / compiler | [Dart Sass](https://sass-lang.com/documentation/js-api/) is measured at 1.105.1 for successful `loadedUrls`; [registry 1.106.1](https://registry.npmjs.org/sass/latest) is newer but does not automatically attest failed Foundation compiles. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-scss` search](https://github.com/search?q=scip-scss&type=repositories) yielded no measured SCSS indexer. |
| Language server | [VS Code CSS/SCSS language service](https://github.com/microsoft/vscode-css-languageservice) has navigation services. [`some-sass-language-server` 2.3.8](https://www.npmjs.com/package/some-sass-language-server) supports SCSS and indented Sass with `@use`/`@forward`; it is third-party tier C and **LIVE UNMEASURED** against Foundation's 25 unattested file pairs. |
| Registry AST / resolver | [sass-parser 0.5.1](https://registry.npmjs.org/sass-parser/latest) postdates measured 0.4.57. [`sass_api` 18.1.0](https://pub.dev/packages/sass_api/versions/18.1.0) exposes AST spans, [`findDependencies`](https://pub.dev/documentation/sass_api/18.1.0/sass_api/findDependencies.html) returns declared URLs, and [`ImportCache.canonicalize`](https://pub.dev/documentation/sass_api/18.1.0/sass_api/ImportCache/canonicalize.html) resolves a URL using Dart Sass's own resolver. Combined, `findDependencies` plus `ImportCache.canonicalize` can attest resolved (importer, imported) pairs without evaluating the stylesheet, including files that fail evaluation. This owner-resolution route is now **MEASURED**, tier A under P-T1; it attests all 25 previously missing pairs in the post-hoc Foundation comparison. It needs the same load paths and importers as a compile, so missing-load-path failures remain unresolved. |
| Runtime/load tracing | [Dart Sass compile API](https://sass-lang.com/documentation/js-api/) reports loads for successful compiles only. A custom JS importer hook can observe explicit `load` callbacks, but does not establish coverage of built-in loads. The `sass_api` resolver measurement below retains every parse and resolution gap. |

The original frozen Foundation result remains historical. The controller authorizes the format-owner resolver as tier A under P-T1; its measured result below supersedes the missing-resolver premise without treating a spent subject as fresh validation.

## Native resolver measurement after binding X-R4 F1

Measured: pinned Dart 3.13.7, sass_api 18.1.0 and Dart Sass 1.106.1 resolve
the complete declared dependency inventory while preserving the primary
sass-parser 0.4.57/Sass 1.105.1 non-import truth. The composite tool identity
names both producers. The [canonical reading](../bench/readings/lang/scss.json)
retains every non-import count, metric and family and remains L1-L3 excellent;
median-of-three throughput is 2.36 MB/CPU-s, zero faults.

Post hoc only: the [separate Foundation comparison](../bench/readings/scss-native-foundation-posthoc-2026-10-26.json)
attests all 25 formerly missing counted file pairs, moving file-edge precision
from .8239 (117/142) to 1 (142/142), with every non-import count/metric unchanged.
Its median throughput is 2.87 MB/CPU-s. Foundation is spent: this does
not replace its original untuned reading or provide independent validation.
The [counts/hashes receipt](data/2026-10-26-core-t1-scss-native-resolver.json)
records original and derived identities, source inventories and all gaps.

Native resolution reports one, sixteen and eleven unresolved unique URLs
across the three complete subjects; nine Foundation URLs are builtins. The
primary retains one successful Bootstrap content-block compile attestation
that findDependencies does not enumerate. Unresolved native evidence never
erases successful primary evidence; conflicting resolved targets fail.
meta.load-css URLs remain explicitly unpositioned and never get a fabricated
line. Eight primary parser-rejected files remain counted against symbol
precision. No extractor, discovery filter, production gate or source changed.

The fresh Spectre pin was committed before any source access; the complete
canonical/post-hoc evidence freeze is 678269006119029000aa81d54d42dd3890727a00.
Its attempted first validation is recorded below.

## Fresh Spectre validation: refused, now spent

Measured: all 76 tracked SCSS files were supplied unchanged, without filters or
compile setup. The frozen primary rejected five files, compiled 19 and failed
57 standalone evaluations. The native composite then refused a dependency
with no positioned primary statement; that file has no import entries in the
primary. No composite oracle JSON or Fathom grade was produced. The
[counts/hashes refusal receipt](data/2026-10-26-core-t1-scss-spectre-refusal.json)
retains the failure, source inventory and frozen tool/build identities.

Spectre is spent and remains outside the canonical corpus. No source, oracle,
filter or rule was changed after its read. The canonical measurements and
Foundation post-hoc repair remain valid, but do not establish fresh independent
validation of the composite. A named next candidate is first-party sass-parser
0.5.1, already identified by the registry audit: it is live and unmeasured,
not evidence of a repair. A follow-up must measure it on the canonical corpus,
freeze the new instrument, and preregister a different untouched holdout.
No exhausted parser/oracle ceiling is claimed.

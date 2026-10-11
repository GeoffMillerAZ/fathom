# RF-M1: does the convention `test` role agree with each test runner's own collection?

**Verdict: CUT. The convention `test` role does not meet the row's bars, so the role facet's
serving (RF-4) does not land.** On the 12 subjects whose runner ran, the convention found every
file the runner collects (recall 1.000 on all 12), but it also calls tests files the runner does not
collect from the repository root: helpers inside `__tests__/` (react-router, precision 0.720), the
example apps' own suites (flask, 0.815), abstract bases, `Test*` helpers and `*IT` classes (apache/maven,
0.785). Two small subjects cannot reach the Wilson bar even at perfect precision (cobra 17 of 17, lower
bound 0.816), and zod's binary assets put its unknown share at 6.1%. Six subjects pass every bar:
cli/cli, prometheus, fastapi, pip, laravel/framework and monolog. Unifying the classifiers (RF-2) moved
1.05% of served `search` flags, 0.18% without a labelled gain: that bar passes, and RF-2 stands.
fp-ts, guava and retrofit are unmeasured (their runner or build could not run as pre-registered).

The header below, through the Harness section, was committed alone before any read (48f06c58) and
is unchanged but for its pending verdict line, which the verdict line under Results replaces.

**Status: pre-registered 2026-10-10, before any read.** This header names the subjects at pinned
commits, the oracle command per ecosystem (Java's runner included, committed alone before any Java
read, K19), the files graded and the bars. Nothing below it has been measured yet. The verdict line
is written once, after the reading, in the form `RF-M1 verdict: KEEP` or `RF-M1 verdict: CUT`.

## What is graded

Only convention-basis roles, and of those only `test`, the one an independent oracle attests
(`docs/plans/repository-facts.md` RF-M1, vision 42 D2, RF-D8). The attribute, header, dialect and
linguist bases are their source's own answer and claim no reading.

- **The convention `test` role**: `repofacts.IsTest(path)` (the convention table plus the
  dialect-pack data in `internal/repofacts/packs.json`, path only), at the commit that records the
  reading.
- **The files graded**: per subject, the files fathom indexes (what `connector.NewLocalConnector()`
  discovers, the set `fathom ingest --dry-run --stats` counts) in the ecosystem's language: `.go`;
  `.py`; `.ts`, `.tsx`, `.mts`, `.cts`, `.js`, `.jsx`, `.mjs`, `.cjs`; `.php`; `.java`. An oracle file
  fathom does not index is counted and named apart, never silently dropped.
- **The oracle's set**: the files the ecosystem's runner collects (below), as repo-relative paths.

## Bars (exactly as the row states them)

Per subject:

- precision of the convention `test` role against the oracle at least **0.95**, with the Wilson 95%
  lower bound (z = 1.96) at least **0.90**;
- recall at least **0.90**;
- `unknown` at most **5%** of indexed files (the full `repofacts.Role` as `search` and
  `symbol_context` serve it, every basis), printed for every subject.

Once, over every subject plus fathom itself: unifying the classifiers (RF-2) changes at most **2%** of
`search` flags without a labelled gain, the same queries' flags read before (the pre-RF-2 rules,
frozen in the harness) and after (repofacts). Queries, each run through `search`'s own engine
(`searchRun`, ripgrep) on every subject, case-sensitive, the served first 50 hits and all hits both
counted: `func `, `class `, `import `, `TODO`, `test`, `error`, `return`, `config`, `assert`, `New`.
Every changed hit is labelled by the rule that changed it, as a gain (the new flag is the file's
truth by its ecosystem's convention) or not.

KEEP requires every measured subject to pass every bar and the flag bar to pass. A subject whose
oracle cannot run within the limits below is reported unmeasured, never substituted; an ecosystem
with no measured subject is reported unmeasured.

## Subjects (calibration corpus, pinned)

Three per ecosystem, from `calibration/corpus.yaml` and `calibration/corpus.d/`, fetched at depth 1 at
the corpus SHA (`calibration/clone.sh`'s method) into the worker's scratch. Where the corpus SHA is a
tag object, the commit it names is given too.

| Ecosystem | Subject | Ref | Corpus SHA | Commit read |
|---|---|---|---|---|
| Go | spf13/cobra | v1.9.1 | a655097faf7d54f78933a815984b9919d51a05d2 | 40b5bc1437a564fc795d388b23835e84f54cd1d1 |
| Go | cli/cli | v2.69.0 | 45ffa3c668f7105e92089220d42aa64c9acbd11c | same |
| Go | prometheus/prometheus | v3.9.1 | 64088ce2ad73e4caf589207d3a246e64e329e663 | 9ec59baffb547e24f1468a53eb82901e58feabd8 |
| TypeScript | colinhacks/zod | v4.4.3 | 1fb56a5c18c27102dbc92260a4007c7732a0ccca | same |
| TypeScript | gcanti/fp-ts | 2.16.9 | 669cd3ed7cb5726024331a7a1cf35125669feb30 | same |
| TypeScript | remix-run/react-router (corpus.d/react.yaml) | | dead4962d6c33e962f266af910b7a09e3ece30cf | same |
| Python | fastapi/fastapi | 0.119.1 | 864b569cf8453654fc3bc2c64108c0f644e2918c | same |
| Python | pallets/flask | 3.1.3 | 8d05782cf7e01c815ceed85eac9d744533af4c44 | 22d924701a6ae2e4cd01e9a15bbaf3946094af65 |
| Python | pypa/pip (corpus.d/python-packaging.yaml) | | 634a6ec1a5d9dcc2433571cdb2f4c58a4bb29caf | same |
| PHP | laravel/framework | v11.54.0 | 4e7ae67eedd803eaea8ceb5f7e113f495d2c0c58 | same |
| PHP | guzzle/guzzle | 7.9.3 | 7b2f29fe81dc4da0ca0ea7d42107a0845946ea77 | same |
| PHP | Seldaek/monolog | 3.9.0 | 10d85740180ecba7896c87e06a166e0c95a0e3b6 | same |
| Java | google/guava | v33.6.0 | de1d87eabff93cb4a7fa8e52df5fc46802c95b17 | same |
| Java | square/retrofit | parent-2.8.1 | da7d27425b9fa79d8ccc301fde20d83173463d26 | 7fa4b3e63735616cb4a2ce23f3ef08952178f8b0 |
| Java | apache/maven (corpus.d/maven.yaml) | maven-3.9.16 | 2bdd9fddda4b155ebf8000e807eb73fd829a51d5 | same |

Chosen before any read, by one rule: the corpus's subjects for the ecosystem whose runner is the
row's. Left out, with why: django (its suite runs under its own `tests/runtests.py`, not pytest),
nestjs/nest, typeorm/typeorm and ReactiveX/rxjs (mocha, which the row does not name); react-router is
the corpus's next TypeScript subject whose runner (jest) the row names.

## Oracles (the runner's own collection)

Toolchains and caches live under the worker's scratch and `/Volumes/m2ext/tmp/fathom-build/toolcache`;
one toolchain at a time; a subject's `node_modules`, `vendor`, `target` and virtualenv are deleted
after its read. Dependency installs use the subject's own lockfile or requirements and skip install
scripts where the tool allows.

- **Go**: in every directory holding a tracked `go.mod`,
  `go list -e -find -f '{{.Dir}}<TAB>{{join .TestGoFiles " "}} {{join .XTestGoFiles " "}}' ./...`
  with `GOWORK=off GOFLAGS=-mod=mod GOPROXY=off`, the union over `GOOS` linux, darwin and windows. If
  the module graph cannot load offline, the same command with the default `GOPROXY` (downloads into
  the shared module cache only).
- **Python**: a virtualenv from `uv` with the subject installed editable plus its own test
  requirements, then `python -m pytest --collect-only -q -p no:cacheprovider` at the repo root under
  the subject's own pytest configuration. The set is the file of every collected node id plus every
  file a collection error names.
- **TypeScript**: dependencies from the subject's lockfile; vitest subjects `npx vitest list
  --filesOnly` (vitest's `list`, files only), jest subjects `npx jest --listTests`, each under the
  subject's own configuration.
- **PHP**: `composer install --no-interaction --no-scripts --ignore-platform-reqs`, then
  `vendor/bin/phpunit --list-tests-xml` under the subject's own `phpunit.xml(.dist)`; each listed test
  class mapped to its file through `composer dump-autoload --optimize`'s class map.
- **Java (pre-registered here, K19; the row names no runner)**: the JUnit Platform Console Launcher
  1.13.4 (`junit-platform-console-standalone`, its Jupiter, Vintage and Suite engines)
  `discover --scan-classpath target/test-classes` over each module's compiled test classes, with the
  module's test class path from `mvn dependency:build-classpath`, after
  `mvn -q -DskipTests test-compile` (dependencies fetched once into a toolcache Maven repository,
  `MAVEN_OPTS=-Xmx1g`, one module build at a time). The set is the source file of every discovered
  test class (an inner class maps to its outer class's file). If a subject's build cannot complete
  within 1 GiB of heap and 30 minutes, the subject is unmeasured; with no measured subject, Java is
  unmeasured.

## Harness

`TestRFM1RoleReading` in `internal/mcp/file_role_reading_test.go` (skipped unless
`FATHOM_RFM1_SUBJECT` is set) writes, per subject, every indexed file's served role and basis, its
convention `test` role, and the pre-RF-2 and post-RF-2 `search` flags of the pre-registered queries'
hits. `roles-score.py` beside this file joins that with the oracle's list and prints the tables below.

## Results (read 2026-10-10, classifier at e7708e01 and 1b193cfe)

RF-M1 verdict: CUT

| Subject | Graded | Convention test | Oracle | TP | FP | FN | Precision | Wilson lower | Recall | Unknown (of indexed) | Bars |
|---|---|---|---|---|---|---|---|---|---|---|---|
| spf13/cobra | 36 | 17 | 17 | 17 | 0 | 0 | 1.000 | 0.816 | 1.000 | 1 / 66 (1.5%) | fails Wilson (n = 17) |
| cli/cli | 736 | 292 | 288 | 288 | 4 | 0 | 0.986 | 0.965 | 1.000 | 1 / 896 (0.1%) | pass |
| prometheus/prometheus | 618 | 236 | 228 | 228 | 8 | 0 | 0.966 | 0.935 | 1.000 | 58 / 1181 (4.9%) | pass |
| colinhacks/zod | 409 | 170 | 170 | 170 | 0 | 0 | 1.000 | 0.978 | 1.000 | 35 / 576 (6.1%) | fails unknown |
| remix-run/react-router | 295 | 125 | 90 | 90 | 35 | 0 | 0.720 | 0.636 | 1.000 | 1 / 665 (0.2%) | fails precision |
| gcanti/fp-ts | 271 | 0 | unmeasured | | | | | | | 0 / 430 (0.0%) | unmeasured |
| fastapi/fastapi | 1175 | 339 | 329 | 329 | 10 | 0 | 0.971 | 0.947 | 1.000 | 2 / 2559 (0.1%) | pass |
| pallets/flask | 83 | 27 | 22 | 22 | 5 | 0 | 0.815 | 0.633 | 1.000 | 1 / 235 (0.4%) | fails precision |
| pypa/pip | 644 | 140 | 140 | 140 | 0 | 0 | 1.000 | 0.973 | 1.000 | 37 / 1037 (3.6%) | pass |
| laravel/framework | 2490 | 810 | 808 | 808 | 2 | 0 | 0.998 | 0.991 | 1.000 | 86 / 2777 (3.1%) | pass |
| guzzle/guzzle | 74 | 27 | 14 | 14 | 13 | 0 | 0.519 | 0.340 | 1.000 | 3 / 117 (2.6%) | fails precision |
| Seldaek/monolog | 217 | 92 | 92 | 92 | 0 | 0 | 1.000 | 0.960 | 1.000 | 6 / 248 (2.4%) | pass |
| google/guava | | | unmeasured | | | | | | | 11 / 3299 (0.3%) | unmeasured |
| square/retrofit | | | unmeasured | | | | | | | 4 / 312 (1.3%) | unmeasured |
| apache/maven | 1030 | 233 | 183 | 183 | 50 | 0 | 0.785 | 0.728 | 1.000 | 50 / 2114 (2.4%) | fails precision |

No oracle file lay outside fathom's indexed set, and none was in another language. By ecosystem: Go 2
of 3 pass, TypeScript 0 of 2 measured, Python 2 of 3, PHP 2 of 3, Java 0 of 1 measured. fathom itself
(not a subject) reads unknown on 116 of 5,342 indexed files (2.2%).

### What the disagreements are

Every one is a file the convention calls a test that the runner, run from the root under the
repository's own configuration, does not collect. No test file was missed (recall 1.000 everywhere
measured).

- **react-router (35)**: helpers inside `__tests__/` (`setup.ts`, `utils/`, `TestSequences/`,
  components). The pack follows jest's default `testMatch`, under which every file in `__tests__/` is
  a test; react-router's own config narrows it to `*-test.[jt]s?(x)`.
- **flask (5)** and **fastapi (10)**: the example apps' suites (`examples/*/tests/test_*.py`) and the
  tutorial's tests (`docs_src/**/test_main.py`), outside the root `testpaths`. They are tests of their
  own projects.
- **apache/maven (50)**: abstract bases (`AbstractArtifactComponentTest`, `*TestCase`), helpers whose
  name starts with `Test` under `src/test/java` (`TestFileWagon`, `TestFileManager`: the pack's
  `test_dir_prefixes` rule, after Surefire's `Test*` include), `*IT` classes the launcher's default
  class-name filter leaves to Failsafe, and test classes with no test methods.
- **cli/cli (4)** and **prometheus (8)**: build-tagged test files (`acceptance`, `integration`,
  `dedupelabels`, `slicelabels`, the fuzz file), one GOARCH-specific file (`template_amd64_test.go`; the
  oracle's union was over GOOS on an arm64 host), and 4 files of a nested example module whose offline
  `go list` failed. Prometheus still passes.
- **laravel/framework (2)**: production files named like tests: `CreatesMatchingTest.php` (a trait)
  and `ChainedBatchTruthTest.php` (a fake). The only disagreements that are convention errors on a
  passing subject.
- **guzzle (13)**: an oracle mapping gap, not a classifier error. PHPUnit lists all 27 classes, but 13
  declare a namespace that does not follow PSR-4 (`GuzzleHttp\Tests\CookieJar` in `tests/Cookie/`,
  `GuzzleHttp\Test\Handler`), so the optimized class map the header names omits them. Mapped by
  declaration, guzzle reads 27 of 27, precision 1.000, Wilson lower bound 0.875: it would still fail,
  on sample size.
- **cobra**: 17 of 17. The Wilson bar needs at least 35 convention tests at perfect precision
  (n / (n + 3.84) >= 0.90), so cobra and guzzle cannot pass it whatever the classifier does (O881).
- **zod's unknown (35)**: binary assets: 14 `.png`, 5 `.svg`, 4 `.pdf`, 4 `.jpg`, 4 `.ai`, an `.ico`, a
  `.webmanifest` and a `.txt`. The vocabulary has no asset role, and the pre-registered indexed set is
  the discovered set, which counts binaries the ingest never extracts (O882).

### Unmeasured, with why

- **fp-ts**: its vitest is 1.6.0, which has no `list` command (`vitest list` runs the suite filter
  "list" and finds nothing). Informational only, not graded: `vitest run --reporter=json` under the
  same `include` collects 80 files under `test/`, and the convention marks none of them (they carry no
  `.test.` infix, so it reads them as fixtures under a test directory): recall 0.
- **guava**: `mvn -DskipTests test-compile` fails: `guava-testlib`'s `module-info.java` needs
  `com.google.common`, which the reactor only provides once packaged.
- **retrofit**: parent-2.8.1 does not compile under JDK 21 (its compiler setup reads a Java 9 class
  file layout: "class file has wrong version 65.0, should be 53.0"), and the host has no JDK 8 to 11.
- The Java oracle ran on apache/maven exactly as pre-registered (JUnit Platform Console Launcher 1.13.4
  `discover --scan-classpath`, with `--details=verbose --disable-ansi-colors` so the class names print).

### The flag bar: RF-2 moved 1.05% of served flags, 0.18% without a labelled gain

The ten pre-registered queries over the 15 subjects and fathom: 6,639 served hits (the first 50 per
query) and 351,643 hits in all (a lower bound where ripgrep's 1 MiB output bound stopped a query, named
by the harness). Every changed hit, by the rule that changed it:

| Change | Served | All | Label |
|---|---|---|---|
| `+test`: a file in a `__tests__/` tree (react-router) | 57 | 2,758 | gain: jest's test files and their helpers, test material |
| `+test`: a fixture tree (`fixtures/`, `__snapshots__/`) | 1 | 115 | gain: test material |
| `-test`: `view.test.stub`, a source stub (laravel) | 0 | 4 | gain: no test |
| `+test`: production files named or placed like tests (guava-testlib, monolog `src/Monolog/Test/`, laravel's two) | 12 | 673 | not a gain |
| `-test`: `tsconfig.test.json` (zod) | 0 | 4 | not a gain: test configuration |
| **Total changed** | **70 (1.05%)** | **3,554 (1.01%)** | |
| **Without a labelled gain** | **12 (0.18%)** | **677 (0.19%)** | bar: at most 2% |

## What follows (the plan's preamble)

A facet's serving lands only with a KEEP reading, so RF-4 (`search`'s role and `role_unknown`,
`symbol_context`'s file block, 75ccae3b) is reverted (d75a878a) and marked refused. RF-2 stands: the ten
rules call the one classifier, and its own bar passes above. RF-1 and RF-3 (`Role`, its bases and
`ReadAttributes`) are built and tested but have no product entry without RF-4, so they stay open, not
done. What would reopen RF-4 is a new reading, not a re-reading of this one: one that names a
configuration-scoped oracle or a convention that reads the runner's configuration, says what a subject
under 35 tests is graded on, and counts unknown over the files the ingest extracts (O881, O882).

## Reproduce

Subjects at the commits above; scratch under `/Volumes/m2ext/tmp/fathom-build/w-rf1-roles/`.

```
# the harness: files.tsv and flags.tsv per subject
go test -c -o rfm1.test ./internal/mcp
FATHOM_RFM1_SUBJECT=<checkout> FATHOM_RFM1_OUT=<dir> ./rfm1.test -test.run '^TestRFM1RoleReading$'
# one oracle list per subject (the commands in Oracles above), then
python3 docs/bench/readings/repository-facts/roles-score.py subject <name> <go|py|ts|php|java> <dir> <oracle.txt>
python3 docs/bench/readings/repository-facts/roles-score.py flags <dir>...
```

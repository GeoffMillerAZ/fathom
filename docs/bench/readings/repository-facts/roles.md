# RF-M1: does the convention `test` role agree with each test runner's own collection?

**Status: pre-registered 2026-10-10, before any read.** This header names the subjects at pinned
commits, the oracle command per ecosystem (Java's runner included, committed alone before any Java
read, K19), the files graded and the bars. Nothing below it has been measured yet. The verdict line
is written once, after the reading, in the form `RF-M1 verdict: KEEP` or `RF-M1 verdict: CUT`.

RF-M1 verdict: pending (no read yet)

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

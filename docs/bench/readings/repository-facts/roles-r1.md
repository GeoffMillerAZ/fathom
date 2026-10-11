# RF-M8: RF-M1 re-read once under R1: does the convention `test` role, split from `test-support`, agree with each runner's own collection?

**Status: pre-registered 2026-10-11, before any read.** This header names the split, the pooled Wilson
rule, fifteen new subjects at pinned commits, each runner's exact collection command, the harness and
the scorer, and the bars. Nothing below it has been measured yet. The verdict line is written once,
after the reading, in the form `RF-M8 verdict: KEEP` or `RF-M8 verdict: CUT`, and nothing is tuned
after it, whichever way it reads.

## Why a re-read, and what changes

RF-M1 (`roles.md`, rf1) read CUT with recall 1.000 on every measured subject: every disagreement was a
file the convention called a test that the runner, run from the root under the repository's own
configuration, does not collect (O881). The owner-architect's ruling R1 (2026-10-11): that reading
measured two questions as one. "Test code" (a file whose role is test, helpers and bases included) is
not "a test the runner collects". This reading splits the role, pre-registers the split and a pooled
Wilson rule alone, and then reads once, on subjects no reading has used. RF-M1 stays `[x]` as rf1's
reading; this is row RF-M8.

Two things change, and only two: the vocabulary gains `test-support`, and a subject too small for the
Wilson bar is pooled with its ecosystem's other small subjects. Every bar is otherwise RF-M1's.

### The split: `test` and `test-support`

- **`test`**: a file the test runner collects.
- **`test-support`**: test code the runner does not collect: helpers, fixture code, abstract bases,
  example suites.

Why split rather than narrow: rf1's 127 false positives were test code by any reader's account
(react-router's `__tests__/` helpers, flask's and fastapi's example suites, maven's abstract bases and
`Test*` helpers). Calling them `source` would be wrong, and calling them `test` answers a question the
runner does not. The split keeps "is this test code?" (what the ten product callers ask) and adds "does
the runner collect it?" (what RF-M1 graded) as the narrower of the two.

The split is a partition of what the convention called `test` before it, and nothing else moves. A
file the convention read as `test` reads `test` or `test-support`; `fixture` (test data and test-tree
helpers that match no test pattern, `conftest.py`) is unchanged. So RF-2's delegates keep their
signatures and their answers for every path: `IsTest` means test code (`test` or `test-support`),
`IsTestMaterial` adds `fixture`, `IsTestEntry` reads `IsTest`, and `IsVendored` does not read a test
rule at all. `TestNoSecondTestOrVendorRuleInProductCode` stays green.

A file the convention would call `test` (the pack's test pattern, as before) reads `test-support`
instead when one of these holds. Each rule is a runner's documented default, or a name no runner
collects; none was read off a subject below.

1. **Example and documentation suites** (Python, TypeScript and JavaScript, PHP): a directory segment
   `examples`, `example`, `samples`, `sample`, `docs`, `doc` or `docs_src`. These are suites of their own
   projects; the root runner's configuration leaves them out (rf1: flask, fastapi). Not Go, whose
   oracle runs `go list` in every module, so an example module's tests are collected.
2. **A directory the runner skips by default**: Go, a segment or base name beginning with `_` or `.`
   (the go tool ignores them); Python, pytest's default `norecursedirs` (a segment beginning with `.`,
   `build`, `dist`, `venv`, `_darcs`, `CVS`, `{arch}`, or ending in `.egg`); TypeScript and JavaScript,
   `dist`, `cypress` and a segment beginning with `.` (vitest's default `exclude`).
3. **Abstract bases** (Java, PHP): a base name beginning with `Abstract`; and Java's `*TestCase.java`
   (a base-class name: JUnit's own default class filter, `^(Test.*|.+[.$]Test.*|.*Tests?)$`, leaves it
   out).
4. **Helpers named like tests** (Java): a file matched only by the `Test*` prefix rule under a test
   directory (`TestUtils`, `TestFileWagon`), not by a `*Test`, `*Tests` or `*IT` suffix.
5. **Outside a test tree** (Java, PHP): a file with no `test` or `tests` directory segment (any case):
   production code named like a test (rf1: laravel's `CreatesMatchingTest.php` trait, guava-testlib).
   Maven's and PHPUnit's suites name test directories; nothing collects `src/main`.
6. **`__tests__/` helpers** (TypeScript, JavaScript): a file matched only by the `__tests__` directory
   (no `.test.` or `.spec.` in its name) whose stem names a helper (`setup`, `setupTests`,
   `setup-tests`, `helpers`, `helper`, `utils`, `util`, `test-utils`, `testUtils`, `test-helpers`,
   `testHelpers`, `mocks`, `mock`, `fixtures`, `fixture`, `factories`, `factory`, `index`, `types`,
   `constants`, `globals`, `support`, `shared`, `common`, any stem beginning with `_`, or a `.d.ts`
   declaration; case-insensitive), or that lies in a subdirectory of `__tests__` named for helpers
   (`utils`, `util`, `helpers`, `helper`, `support`, `setup`, `mocks`, `__mocks__`, `shared`, `common`,
   `__helpers__`, `__utils__`, `test-utils`). Jest's default `testMatch` collects every file in
   `__tests__/`, so a project that keeps helpers there narrows it (rf1: react-router); a file directly
   in `__tests__/` with no helper name stays `test`, as jest's default reads it.

Where a rule is wrong for a subject, the subject reads it: no rule is added, removed or narrowed after
this header is committed.

## What is graded

Only the convention basis, and of it only `test` (a file the runner collects), the role an independent
oracle attests. The attribute, header, dialect and linguist bases are their source's answer and claim
no reading (RF-M1, vision 42 D2).

- **The convention `test` role**: `repofacts.ConventionRole(path) == "test"` (the core table plus
  `internal/repofacts/packs.json`, path only), at the commit that records the reading. A file that
  reads `test-support` is not graded `test`: it is a false negative if the runner collects it.
- **The files graded**: per subject, the files fathom indexes (what `connector.NewLocalConnector()`
  discovers, RF-M1's set) in the ecosystem's language: `.go`; `.py`; `.ts`, `.tsx`, `.mts`, `.cts`,
  `.js`, `.jsx`, `.mjs`, `.cjs`; `.php`; `.java`. An oracle file fathom does not index, or in another
  language, is counted and named apart, never silently dropped.
- **The oracle's set**: the files the ecosystem's runner collects under the subject's own
  configuration (below), as repo-relative paths.

## Bars

Exactly RF-M1's, per subject or pooled stratum:

- precision of the convention `test` role at least **0.95**, with the Wilson 95% lower bound (z = 1.96)
  at least **0.90**;
- recall at least **0.90**, per subject;
- `unknown` at most **5%** of indexed files, per subject, printed for every subject: the full
  `repofacts.Role` as RF-4 would serve it (every basis), over the discovered set, RF-M1's definition
  unchanged (O882 stays open: no ruling changed it);
- once, over every subject plus fathom itself: unifying the classifiers (RF-2) changes at most **2%** of
  `search` flags without a labelled gain, RF-M1's queries and labelling unchanged (`func `, `class `,
  `import `, `TODO`, `test`, `error`, `return`, `config`, `assert`, `New`; served first 50 and all hits).

### The pooled Wilson rule (O881)

Per ecosystem, a subject whose runner collects fewer than **35** test files (its oracle set) is pooled
with that ecosystem's other such subjects into one stratum, judged on the stratum's pooled precision
(sum of true positives over sum of convention `test` files) and that precision's Wilson lower bound. A
subject whose runner collects 35 or more is judged alone. Recall and unknown stay per subject.

- **Why 35.** At perfect precision the Wilson lower bound is n / (n + 3.8416), which reaches 0.90 only at
  n >= 34.6. Below 35 a subject fails the interval bar whatever the classifier does (rf1: cobra 17 of 17
  read 0.816, guzzle 27 of 27 by declaration 0.875). The bar then measures the subject's size, not the
  role.
- **Why pool, and only the small.** The interval bar exists to stop a lucky small sample passing; pooling
  an ecosystem's small subjects gives it the sample it needs while keeping their errors in the count. A
  subject large enough to be judged alone is, so a large subject's errors are never diluted by another
  subject's clean files, and a small one cannot hide behind a large one. A stratum of one subject is
  judged on that subject. The threshold reads the runner's count, not the convention's, so the
  classifier cannot move a subject between the two rules.

### Verdict rule

**KEEP** requires every subject judged alone and every pooled stratum to pass precision and the Wilson
bar, every measured subject to pass recall and unknown, the flag bar to pass, and each of the five
ecosystems to have at least one measured subject (a role served for an ecosystem with no measured
subject would be unread). Anything else is **CUT**. A subject whose oracle cannot run as written below
is reported unmeasured with why, never substituted, and leaves its stratum.

## Subjects (new, pinned)

Chosen before any read by one rule: per ecosystem, three widely used libraries whose own test runner is
the row's (`go test`; pytest; jest or vitest; PHPUnit; Maven), at their latest release tag on
2026-10-11, none one of rf1's fifteen (`roles.md`) and none a subject of any committed reading or the
calibration corpus: each candidate's owner and repository name was searched across
`docs/bench/readings/**` and `calibration/**` and found in neither (go.uber.org/zap appears only as a
dependency line in three oracle `go.sum` files and is no subject; tenacity's first fetched tag, the
stale `v1.3.3`, was replaced by its current release `9.2.1` before any read). Each was fetched at
depth 1 at its tag into the worker's scratch. Where the tag is an annotated tag object, the commit it
names is the one read. Licences were read from each checkout.

| Ecosystem | Subject | Tag | Commit read | Licence | Runner (as the subject invokes it) |
|---|---|---|---|---|---|
| Go | gin-gonic/gin | v1.12.0 | 73726dc606796a025971fe451f0aa6f1b9b847f6 | MIT | go test |
| Go | labstack/echo | v5.4.0 | fc410094b2f3415cf2bf5204082137b205bfd5af | MIT | go test |
| Go | uber-go/zap | v1.28.0 (tag 1e9c10b2) | 5b81b37b81b8e2ed447a6f57991e372ee4fa5c8f | MIT | go test |
| TypeScript | pmndrs/zustand | v5.0.15 | 2115efb9e270e73ad1d3472dfe0e0c7b8c6abcd4 | MIT | vitest (`vitest run`, vitest.config.mts) |
| TypeScript | pmndrs/jotai | v3.0.1 | 6abd0ae3365e02ab432fba4b6e8e6f00aafbf508 | MIT | vitest (`vitest run`, vitest.config.mts) |
| TypeScript | react-hook-form/react-hook-form | v7.90.0 (tag 0ae9461d) | 8ac7cc0bcc1f50f887230d50b1929d977b4d3fb4 | MIT | jest (`jest --config ./scripts/jest/jest.config.js`) |
| Python | Textualize/rich | v15.0.0 | 6ac483cbea39cab124dfd3483bba70ffafb71050 | MIT | pytest (`testpaths = ["tests"]`) |
| Python | marshmallow-code/marshmallow | 4.3.1 | c7b559a1fa3aba57ca6dba0ab336841c5038a782 | MIT | pytest |
| Python | jd/tenacity | 9.2.1 | 4410996dd9a96864a2733ce0778bcaa112b72811 | Apache-2.0 | pytest |
| PHP | briannesbitt/Carbon | 3.14.2 | 94cfc34887f07ab5c0efa07914c8f527544e3224 | MIT | PHPUnit (phpunit.xml.dist) |
| PHP | thephpleague/commonmark | 2.10.3 (tag 584ead0c) | 6efbd9c472b91db0a3350fcd601c8332c2382e1f | BSD-3-Clause | PHPUnit (phpunit.xml.dist) |
| PHP | ramsey/uuid | 4.9.4 (tag b3600cf7) | 75d73f48d02797c2c285a7e9f348fadc0102ffe2 | MIT | PHPUnit (phpunit.xml.dist) |
| Java | jhy/jsoup | jsoup-1.23.2 (tag 0bf57a30) | fbc7775c462f5b023d8db54c9a73d7ec17d53300 | MIT | Maven Surefire and Failsafe |
| Java | apache/commons-lang | rel/commons-lang-3.21.0 (tag 0fd96925) | 155db038d73fe0ea49612d42ae70f6e48ee3ebb9 | Apache-2.0 | Maven Surefire |
| Java | apache/commons-text | rel/commons-text-1.15.0 (tag 27189fba) | 04e937470d3679cc163df85d82d5b6d2e3e71128 | Apache-2.0 | Maven Surefire |

## Oracles (each runner's own collection, under the subject's own configuration)

Toolchains and caches live under the worker's scratch and `<build scratch>/toolcache`; one subject at a
time; a subject's `node_modules`, `vendor`, `target` and virtualenv are deleted after its read.
Dependency installs use the subject's own lockfile or declared test requirements and skip install
scripts where the tool allows. Each command runs from the subject's root.

- **Go**: in every directory holding a tracked `go.mod`,
  `go list -e -find -f '{{.Dir}}<TAB>{{join .TestGoFiles " "}} {{join .XTestGoFiles " "}}' ./...`
  with `GOWORK=off GOFLAGS=-mod=mod`, the union over `GOOS` linux, darwin and windows crossed with
  `GOARCH` amd64 and arm64 (rf1 crossed GOOS only, on an arm64 host, and so missed a GOARCH-suffixed
  test file it then had to name). No `-tags`: a build-tagged test the default run leaves out is not
  collected. `GOPROXY` default (downloads land in the shared module cache only).
- **Python**: a virtualenv from `uv` (Python 3.12) with the subject installed editable plus its own
  declared test requirements (its `test`/`tests`/`dev` extra or dependency group), then
  `python -m pytest --collect-only -q -p no:cacheprovider` with no path argument, under the subject's
  own pytest configuration. The set is the file of every collected node id plus every file a
  collection error names.
- **TypeScript**: dependencies from the subject's lockfile (`pnpm install --frozen-lockfile
  --ignore-scripts`); vitest subjects `npx vitest list --filesOnly` (the subject's `vitest.config.*`);
  react-hook-form `npx jest --config ./scripts/jest/jest.config.js --listTests` (its `test` script's
  configuration). The set is every listed file, made repo-relative.
- **PHP**: `composer install --no-interaction --no-scripts --ignore-platform-reqs`, then
  `vendor/bin/phpunit --list-tests-xml <out>` under the subject's own `phpunit.xml(.dist)`. Each listed
  test class is mapped to its file by declaration: the `namespace` and `class` declared in each of the
  subject's `.php` files (a PHP tokenizer run, `token_get_all`), so a class whose namespace does not
  follow PSR-4 is still found (rf1's guzzle gap). A class mapped to no file or to several is named.
- **Java**: Maven Surefire, and Failsafe where the POM binds it, as the POM configures them (their
  `includes`, `excludes` and profiles as the default build activates them): `mvn -B -fae verify
  -Dmaven.test.failure.ignore=true -Djunit.platform.execution.dryRun.enabled=true` with JDK 21 and
  `MAVEN_OPTS=-Xmx1g`, the non-test checks skipped by their own skip properties (`-Drat.skip
  -Djapicmp.skip -Dcheckstyle.skip -Dspotbugs.skip -Dpmd.skip -Dcpd.skip -Danimal.sniffer.skip
  -Dmaven.javadoc.skip -Djacoco.skip -Denforcer.skip -Dmoditect.skip -Dgpg.skip -Dcyclonedx.skip
  -Dbnd.baseline.skip`). JUnit Platform 1.12 and later honour the dry-run parameter and report every
  test without running it; an earlier platform runs the suite, and the set is the same. The set is the
  source file of every class named in `target/surefire-reports/TEST-*.xml` and
  `target/failsafe-reports/TEST-*.xml` (a nested class maps to its outer class's file under the test
  source roots). This replaces rf1's JUnit Console Launcher scan with its default class filter, which
  did not read the POM: for Java, as for the other four, the oracle is now the runner as the
  repository configures it. If a build cannot reach its test phase within 1 GiB of heap and 30 minutes,
  the subject is unmeasured.

## Harness and scorer

`TestRFM1RoleReading` in `internal/mcp/file_role_reading_test.go` (skipped unless `FATHOM_RFM1_SUBJECT`
is set) writes, per subject, every indexed file's served role and basis, `IsTest` (test code), its
convention role (`repofacts.ConventionRole`), and the pre-RF-2 and post-RF-2 `search` flags of the
queries' hits. `roles-r1-score.py` beside this file joins that with the oracle's list (`subject`),
pools and judges (`verdict`), and prints the tables below. Both are committed, with the split, before
any read.

## Results

Pending: nothing has been read.

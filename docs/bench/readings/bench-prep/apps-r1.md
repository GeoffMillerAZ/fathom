# Applications and services: round 1

Measured on 2026-10-26 (UTC), from the wave-3 integration checkout at `0666ccc70d12fb27bfba8427bb3104d2c927cad1`. This is one preparation round under D27: five public application/service candidates per language, with no task derivation, paid API calls or C4 run. A kept row means a preparation candidate, not an admitted BX-44 subject. Native macOS C3, the exact twelve-core condition, C4, fail-before/pass-after C5 replay, post-cutoff yield after Q2, controller split assignment and the live C7 fleet check remain unmeasured. No plan box changed.

[Machine-readable receipts](apps-r1.json) bind pins, public metadata, licence-blob hashes, full-history counts, C6 witnesses and every archived byte. The sole new archive is [bench-apps-r1-logs.tar.gz](raw/bench-apps-r1-logs.tar.gz), SHA-256 `0004caba338c0d9048cfe7d5c7bae1a16d58426498fcfe549975dca8849d62ab`.

| Language | Kept this round | Refused this round | Dev-forced kept | Combined BX-44 preparation pool | Short of 6 | Unassigned dev / held-out |
|---|---:|---:|---:|---:|---:|---|
| Go | 0 | 5 | 0 | 0 | 6 | 3 / 3 |
| Python | 0 | 5 | 0 | 1 | 5 | 3 / 3 |
| TypeScript | 1 | 4 | 0 | 2 | 4 | 3 / 3 |

The combined preparation pool carries forward pytest (Python) and redux-toolkit (TypeScript). Class-validator remains BX-M0-only because it fails C5 and is excluded from this BX-44 count. Formal admission is still short **six per language**, with **three dev and three held-out slots unfilled in each language**. Prior-read exclusions do not fill slots.

## Fifteen-candidate screen

| Language | Repository / pin | C2 lines / band | Product dirs | C5 primary fix co-changes | D26 qualifying symbols | Disposition |
|---|---|---:|---:|---:|---:|---|
| Go | charmbracelet/soft-serve / `52ba4c852667ff464675b0876fcb1ca2da257dce` | 31,687 / middle | 47 | 26 | 35 | C5: 26 primary-language fix-message/test co-changes; fewer than 30; C3: full offline warm-up failed; 3 timed trials recorded |
| Go | go-shiori/shiori / `8fa2e2b4fa1f83c534016024209d0053a765f80d` | 29,377 / middle | 20 | 9 | 22 | C5: 9 primary-language fix-message/test co-changes; fewer than 30; C3: full offline warm-up failed; 0 timed trials recorded |
| Go | derailed/k9s / `6382557188ed29cbdf0b0f690d308cc0789aa3eb` | 75,704 / large | 24 | 40 | 23 | C3: full offline warm-up failed; 0 timed trials recorded |
| Go | micro-editor/micro / `e69c000696906cd90340c54ef39648befe773fff` | 25,265 / middle | 15 | 2 | 18 | C5: 2 primary-language fix-message/test co-changes; fewer than 30 |
| Go | goreleaser/goreleaser / `0162ee4bbe06909cc13956e147a6adb01feb127d` | 104,755 / large | 130 | 265 | 87 | C3: full offline warm-up failed; 0 timed trials recorded |
| Python | pypa/hatch / `e7495de2cac0710df2698b650427c7593f012037` | 75,649 / large | 64 | 22 | 3 | C5: 22 primary-language fix-message/test co-changes; fewer than 30; C3: no upstream primary-language dependency lock; no unlocked setup substituted |
| Python | pdm-project/pdm / `301ca8c8745fd552a6b4df5734c5a5c071ced362` | 41,788 / middle | 25 | 103 | 19 | C3: full offline warm-up failed; 0 timed trials recorded |
| Python | sqlfluff/sqlfluff / `27e84221e56410432bed4843e0a0853fbd4622ac` | 198,917 / above ceiling | 38 | 266 | 29 | C2: primary-language lines exceed 150,000; C3: not attempted after C2 refusal |
| Python | getnikola/nikola / `17ee421fee1deb8bf86cddff0feb96f23be14c73` | 35,039 / middle | 20 | 2 | 9 | C5: 2 primary-language fix-message/test co-changes; fewer than 30; C3: no upstream primary-language dependency lock; no unlocked setup substituted |
| Python | python-poetry/poetry / `1fe5e0a57928572eb1d08f83b0cf0179043280dd` | 81,877 / large | 37 | 102 | 4 | C3: locked setup exited 132; warm-up and three trials unrun |
| TypeScript | changesets/changesets / `e2ee472cc6c901a0bb29781dac12603f6e4d5215` | 32,576 / middle | 57 | 22 | 3 | C5: 22 primary-language fix-message/test co-changes; fewer than 30; C3: full offline warm-up failed; 0 timed trials recorded |
| TypeScript | verdaccio/verdaccio / `377d8b15074bbf75040b7eef72e0862e2559c06c` | 84,769 / large | 150 | 93 | 6 | C3: full offline warm-up failed; 0 timed trials recorded |
| TypeScript | keystonejs/keystone / `987c2bc887877c3042411a8c5fd7141bd3e9867b` | 110,417 / large | 77 | 23 | 10 | C5: 23 primary-language fix-message/test co-changes; fewer than 30; C3: locked setup exited 1; warm-up and three trials unrun |
| TypeScript | slidevjs/slidev / `a42aff28dd75f26eb5abbf3d616ea022c3d08924` | 29,266 / middle | 41 | 31 | 7 | kept for preparation |
| TypeScript | yarnpkg/berry / `ef16bb2e5826f2b4c342f387e4ac50fea86d4a51` | 95,488 / large | 105 | 26 | 0 | C5: 26 primary-language fix-message/test co-changes; fewer than 30; C3: locked setup exited 1; warm-up and three trials unrun; C6: 0 qualifying symbols in the recorded resolver; fewer than three |

All fifteen are applications, services or monorepos of internal product packages and have at least ten tracked product directories. None is archived; pins are the latest default-branch commit before 2026-10-26T00:00:00Z. Public metadata was fetched read-only from the repository host; package metadata and setup fetches were also allowed. Redwood was replaced during preflight because its latest commit was in 2025.

## Methods and limits

C1 reads each root licence blob at the pin and records its SHA-256; all fifteen pass the allowlist. C2 counts every tracked `.go`, `.py`, or `.ts/.tsx/.mts/.cts` blob, including tests, fixtures, generated files and declarations. Bands are 5k–25k, 25k–75k and 75k–150k. `inventory.py` is used except for Nikola: its tracked `messages_cz.py` is a symlink, so archive regular-file membership is incomplete. The committed `static.py` direct-blob fallback counts it without dereferencing.

C5 uses full-history filtered clones, non-merge commits, committer dates in the fixed UTC window 2024-10-25 through 2026-10-26, and `\b(fix(es|ed)?|bugs?|regressions?)\b`. It requires a test/source co-change in the primary language. These are **surrogates, with zero replayed fixes**; a count below thirty refuses this preparation screen, while a larger count does not prove C5.

C6 uses the committed go/types resolver, CPython 3.12.15 AST resolver, or TypeScript 5.9.3 signature resolver, followed by `recount.py` under D26. Defining directories and test/support callers are excluded; Go external test packages merge by directory under D24. TypeScript final receipts use `c6_ts_project.cjs` with the subject configuration, after setup where available. Python direct import/alias resolution does not resolve dynamic dispatch; TypeScript/PnP and unavailable generated/dependency declarations may under-resolve. A missing witness is a refusal by this instrument, not a certified zero in the complete runtime graph. C4 stays unrun.

## C3: setup and offline processes

Clones use `git clone --filter=blob:none --no-checkout`, then detached checkout of the recorded pin, without shallow history. The committed `remeasure.py` mounts `/subject:ro`, fresh job-owned `/work:rw`, and `/bench-inputs:ro`, stages tracked source and Git metadata in `/work/source`, and gives `/work/tmp` executable Linux tmpfs. Setup alone may fetch packages. Warm-up and timed processes use `--network none --cpus 4`, explicit offline flags and empty proxy variables. A failed network probe accompanies each successful setup. Tests use the subject lock and native script; no passing subset replaces a failing full suite. Failed setup/warm-up stops before three timed trials, except Soft Serve, whose three failures were recorded.

The resumed harness edits (`tar --touch --no-same-owner` and shell `set -e`) were present before these measurements. After measurement, compound-command grouping was corrected so a failed staging or working-directory check cannot be masked by later semicolon commands. Both regression tests failed before that correction and pass afterward. The archive preserves the exact executed wrapper at `preparation/remeasure-executed.py`; executed and corrected hashes are recorded separately. Kept manifests use corrected boundary grouping with the measured successful command bodies. Setup, test and all required staging/environment are explicit in each kept manifest. A stopped preparation is a refusal **in this isolation setup**, not evidence of universal non-hermetic behavior.

| Repository | Setup seconds / exit | Warm-up seconds / exit | Three timed seconds / exits |
|---|---|---|---|
| charmbracelet/soft-serve | 29.332 / 0 | 105.894 / 1 | 64.305 / 1, 67.177 / 1, 64.925 / 1 |
| go-shiori/shiori | 42.196 / 0 | 64.774 / 1 | unrun |
| derailed/k9s | 154.987 / 0 | 213.534 / 2 | unrun |
| micro-editor/micro | 8.131 / 0 | 17.351 / 0 | 4.508 / 0, 4.595 / 0, 4.825 / 0 |
| goreleaser/goreleaser | 114.793 / 0 | 203.72 / 1 | unrun |
| pypa/hatch | unrun | unrun | unrun |
| pdm-project/pdm | 29.325 / 0 | 178.724 / 1 | unrun |
| sqlfluff/sqlfluff | unrun | unrun | unrun |
| getnikola/nikola | unrun | unrun | unrun |
| python-poetry/poetry | 34.957 / 132 | unrun | unrun |
| changesets/changesets | 57.817 / 0 | 12.131 / 1 | unrun |
| verdaccio/verdaccio | 115.596 / 0 | 145.499 / 1 | unrun |
| keystonejs/keystone | 150.564 / 1 | unrun | unrun |
| slidevjs/slidev | 93.797 / 0 | 8.36 / 0 | 6.624 / 0, 6.787 / 0, 6.791 / 0 |
| yarnpkg/berry | 88.447 / 1 | unrun | unrun |

Raw failure witnesses and preparation corrections follow. They do not replace C5/C6 refusals or imply a subject was patched.

- **charmbracelet/soft-serve**: `TestValidateGitRemoteResolvesHostnames` fails with network disabled; Git/server fixture tests also fail. Warm-up and all three timed trials exit 1. C5 independently falls below thirty. Logs retain the complete failures.
- **go-shiori/shiori**: The native `scripts/test.sh` command fails in `TestDownloadBookImage` and related image-download cases offline. Database URLs and build tags are explicit in the suite receipt. C5 independently has only nine co-changes.
- **derailed/k9s**: `TestIsValidNamespace/default-ns` expects true but gets false. The test inserts a namespace cache entry with a 1 ms expiry. This is a failed measured warm-up, not an assertion that the subject needs an external network. Three trials were not run after it.
- **micro-editor/micro**: Its native `make test` passes warm-up and three offline runs after `go clean -testcache`; the lock is unchanged. C5 has only two qualifying co-changes, despite eighteen D26 witnesses.
- **goreleaser/goreleaser**: The expanded native Taskfile test command fails `TestSetupGitignore/write_error` when root can write through permission bits, and `TestRunPipeWithProxiedRepo` because `github.com/goreleaser/test-mod@v0.1.1` is absent from the warmed module cache. These are preparation limitations in this root container, not proof of universal network dependence. No subject code was modified and no full timed triple is claimed.
- **pypa/hatch**: No upstream primary-language test dependency lock is tracked. The downstream backend requirements file is not a lock for the root suite. C3 was not attempted with a newly resolved graph; C5 independently has twenty-two co-changes.
- **pdm-project/pdm**: The first isolated build could not derive its SCM version. Storing the safe-directory setting in the job-owned Git configuration corrected setup without changing subject files or its lock. The complete offline warm-up then recorded 99 failures and six errors (1,520 passed), including package-resolution name-resolution failures in fresh test projects. No timed triple was run after that failure. Both setup attempts are retained.
- **sqlfluff/sqlfluff**: 198,917 tracked Python lines exceed C2's ceiling. C3 was not attempted after that refusal; the static Python oracle still provides twenty-nine D26 witnesses.
- **getnikola/nikola**: The only dependency locks found belong to JavaScript asset tooling, not the Python suite. No unlocked Python setup was substituted. C5 has two co-changes. The direct-blob C2 fallback accounts for the tracked Python symlink rather than dropping it.
- **python-poetry/poetry**: The locked setup exits 132 with `Illegal instruction` during `poetry sync --only main,test`. It repeats in a fresh tree with RapidFuzz forced to its Python implementation, so that hypothesis did not resolve the native-runtime crash. Both failed setup logs are retained; no test result, timed triple or universal portability claim is made.
- **changesets/changesets**: The warm-up reports eight failures, including Git fixture commits/tags, a formatter subprocess and two release snapshots. The initial recipe lacked a synthetic Git identity, so this C3 result is confounded by preparation and is not attributed solely to network isolation. C5's twenty-two co-changes already refuse it; no passing subset was rerun.
- **verdaccio/verdaccio**: The native recursive test command fails four `packages/api/test/integration/user.spec.ts` cases: package retrieval expects HTTP 200 and receives 404. The full root command exits 1; other workspace suites passing do not replace that result.
- **keystonejs/keystone**: The locked setup fails the better-sqlite3 native install: node-gyp header extraction reports `EACCES` on `fchown` on the shared mount. This is an environmental preparation refusal, with no timed suite claim. C5 independently has twenty-three co-changes. Later Node recipes place the header cache under native `/work/tmp`.
- **slidevjs/slidev**: Its first setup produced a completed build, but a disk-monitor race with pnpm's transient paths closed the harness stdout before a setup receipt was written. That attempt is invalid preparation, not a C3 result; the unreceipted log is retained separately and the build was deleted before fresh setup. Dependency install hooks are suppressed to avoid unrelated Git-hook and skill synchronization; the declared product build still runs.
- **yarnpkg/berry**: The native `yarn install --immutable` refuses the prepared tree with YN0064 because the committed `.pnp.*` checksum would change. The lock/PnP constraints were not relaxed. C5 has twenty-six co-changes. The compiler oracle does not implement Yarn PnP resolution, so its missing C6 witness is scoped to that resolver.

Preparation anomaly: the first local regression-test mock allowed two host-side connectivity probes. These were not subject trials and are not C3 evidence. The corrected mock never executes later phases; recorded subject warm-ups and trials use Docker network none.

## C6 witnesses

Each entry gives resolved call sites / distinct product caller units. Full site lists, excluded callers and every recount row are archived; the three-symbol threshold is not inferred from tests.

- **charmbracelet/soft-serve**: `github.com/charmbracelet/soft-serve/pkg/config.FromContext` 42 / 16 (for example `cmd/cmd.go:22:9`); `github.com/charmbracelet/soft-serve/pkg/ui/common.*Common.SetSize` 15 / 10 (for example `cmd/soft/browse/browse.go:88:2`); `github.com/charmbracelet/soft-serve/pkg/backend.FromContext` 58 / 9 (for example `cmd/soft/admin/admin.go:61:10`).
- **go-shiori/shiori**: `github.com/go-shiori/shiori/internal/model.Dependencies.Domains` 66 / 7 (for example `internal/core/ebook.go:28:5`); `github.com/go-shiori/shiori/internal/model.Dependencies.Logger` 53 / 6 (for example `internal/domains/bookmarks.go:305:5`); `github.com/go-shiori/shiori/internal/model.Dependencies.Config` 20 / 6 (for example `internal/core/ebook.go:63:32`).
- **derailed/k9s**: `github.com/derailed/k9s/internal/client.IsAllNamespaces` 11 / 7 (for example `internal/config/views.go:178:5`); `github.com/derailed/k9s/internal/client.*GVR.String` 33 / 6 (for example `internal/config/alias.go:103:51`); `github.com/derailed/k9s/internal/client.*GVR.R` 32 / 6 (for example `internal/dao/rbac.go:40:9`).
- **micro-editor/micro**: `github.com/micro-editor/micro/v2/internal/screen.TermMessage` 72 / 6 (for example `cmd/micro/micro.go:191:5`); `github.com/micro-editor/micro/v2/internal/config.GetGlobalOption` 27 / 6 (for example `cmd/micro/micro.go:390:27`); `github.com/micro-editor/micro/v2/internal/util.SafeWrite` 6 / 4 (for example `internal/action/bindings.go:28:9`).
- **goreleaser/goreleaser**: `github.com/goreleaser/goreleaser/v2/internal/tmpl.New` 227 / 70 (for example `internal/builders/base/options.go:16:14`); `github.com/goreleaser/goreleaser/v2/internal/tmpl.*Template.Apply` 184 / 65 (for example `internal/archivefiles/archivefiles.go:24:16`); `github.com/goreleaser/goreleaser/v2/internal/tmpl.*Template.Bool` 41 / 36 (for example `internal/builders/base/options.go:23:19`).
- **pypa/hatch**: `hatch.utils.fs.Path` 34 / 9 (for example `src/hatch/cli/__init__.py:139:32`); `hatch.utils.runner.ExecutionContext` 9 / 3 (for example `src/hatch/cli/application.py:136:27`); `hatch.env.utils.add_verbosity_flag` 5 / 3 (for example `src/hatch/env/lockers/pip.py:53:13`).
- **pdm-project/pdm**: `pdm.exceptions.PdmUsageError` 78 / 10 (for example `src/pdm/cli/actions.py:102:19`); `pdm.utils.normalize_name` 47 / 10 (for example `src/pdm/cli/commands/lock.py:31:16`); `pdm.models.requirements.parse_requirement` 13 / 8 (for example `src/pdm/builders/base.py:297:30`).
- **sqlfluff/sqlfluff**: `sqlfluff.core.rules.base.LintResult` 166 / 13 (for example `plugins/sqlfluff-plugin-example/src/sqlfluff_plugin_example/rules.py:59:24`); `sqlfluff.core.rules.crawlers.SegmentSeekerCrawler` 67 / 11 (for example `plugins/sqlfluff-plugin-example/src/sqlfluff_plugin_example/rules.py:44:23`); `sqlfluff.core.errors.SQLFluffUserError` 35 / 8 (for example `plugins/sqlfluff-templater-dbt/sqlfluff_templater_dbt/templater.py:584:19`).
- **getnikola/nikola**: `nikola.utils.makedirs` 30 / 8 (for example `nikola/plugins/basic_import.py:131:9`); `nikola.utils.req_missing` 17 / 8 (for example `nikola/plugins/command/auto/__init__.py:175:13`); `nikola.log.get_logger` 13 / 5 (for example `nikola/plugins/command/console.py:36:10`).
- **python-poetry/poetry**: `poetry.toml.file.TOMLFile` 6 / 5 (for example `src/poetry/config/config.py:440:27`); `poetry.utils.helpers.remove_directory` 11 / 4 (for example `src/poetry/console/commands/build.py:146:13`); `poetry.utils._compat.decode` 9 / 4 (for example `src/poetry/console/exceptions.py:107:38`).
- **changesets/changesets**: `packages/cli/src/commands/shared.ts#ensureChangesetFolder@7` 8 / 8 (for example `packages/cli/src/commands/add/index.ts:34:9`); `packages/cli/src/utils/read-config.ts#readConfig@7` 7 / 7 (for example `packages/cli/src/commands/add/index.ts:42:24`); `packages/cli/src/utils/cli-utilities.ts#importantWarning@23` 5 / 3 (for example `packages/cli/src/commands/add/createChangeset.ts:17:5`).
- **verdaccio/verdaccio**: `packages/ui-components/src/providers/AppConfigurationProvider/AppConfigurationProvider.tsx#useConfig@86` 27 / 15 (for example `packages/ui-components/src/Theme/ThemeProvider.tsx:26:29`); `packages/ui-components/src/configuration.ts#getConfiguration@15` 11 / 8 (for example `packages/ui-components/src/components/Package/Package.tsx:75:18`); `packages/ui-components/src/store/utils.ts#stripTrailingSlash@5` 8 / 6 (for example `packages/ui-components/src/providers/AuthProvider/AuthProvider.tsx:28:18`).
- **keystonejs/keystone**: `packages/core/src/types/next-fields.ts#fieldType@469` 21 / 18 (for example `packages/core/src/fields/types/bigInt/index.ts:125:12`); `packages/core/src/types/next-fields.ts#fieldTypeWrapper@472` 21 / 18 (for example `packages/core/src/fields/types/bigInt/index.ts:125:12`); `packages/core/src/fields/non-null-graphql.ts#makeValidateHook@18` 11 / 11 (for example `packages/core/src/fields/types/bigInt/index.ts:104:32`).
- **slidevjs/slidev**: `packages/vscode/src/utils/getSlidesTitle.ts#getSlidesTitle@4` 6 / 4 (for example `packages/vscode/src/commands.ts:106:42`); `packages/client/logic/utils.ts#makeId@3` 5 / 4 (for example `packages/client/builtin/VSwitch.ts:57:16`); `packages/slidev/node/resolver.ts#toAtFS@133` 8 / 3 (for example `packages/slidev/node/setups/indexHtml.ts:146:19`).
- **yarnpkg/berry**: no three-symbol witness in the recorded resolver.

## C7 and observed metadata

The preflight grepped `calibration/`, `testdata/scenariobench/` and `docs/bench/` before adding this reading. Caddy, Task, Mitmproxy and Astro remain excluded; Beets, Act, Alertmanager and Docker Compose were also excluded for existing corpus/pilot/reading hits. Every literal repository hit for these exclusions is retained as a prior-read path/line receipt in archived `receipts/preflight.json`, including all duplicate locations. Air was excluded for GPL-3.0. Candidate short-name hits are retained in `apps-r1.json` and classified below. Ordinary words, installer use and another repository’s filenames do not establish a source-tree read. No new candidate has a known prior subject read; none is dev-forced. The daemon was unavailable, so the live-fleet exclusion is unknown. There is at most one candidate per organization.

| Repository | Created (UTC) | Latest pinned commit (UTC) | Stars at lookup | Prior-read search hits / disposition |
|---|---|---|---:|---|
| charmbracelet/soft-serve | 2021-07-30T23:32:44Z | 2026-10-23T06:56:02Z | 7,269 | 0: No literal repository or distinctive-name hit. |
| go-shiori/shiori | 2018-01-23T13:57:11Z | 2026-10-24T13:40:56Z | 11,685 | 0: No literal repository or distinctive-name hit. |
| derailed/k9s | 2019-01-25T18:46:02Z | 2026-10-24T09:23:38Z | 34,879 | 0: No literal repository or distinctive-name hit. |
| micro-editor/micro | 2016-03-11T02:06:28Z | 2026-10-22T16:51:04Z | 29,763 | 2: Ordinary micro-dollars and a workflow path; no subject source read. |
| goreleaser/goreleaser | 2016-12-21T17:13:39Z | 2026-10-23T16:03:51Z | 16,124 | 5: Release-tool configuration and cue-lang/cue file paths, not GoReleaser source. |
| pypa/hatch | 2017-05-31T23:37:53Z | 2026-10-23T05:02:40Z | 7,245 | 4: Ordinary escape-hatch wording and a build-system example, not a Hatch source read. |
| pdm-project/pdm | 2019-12-27T03:50:57Z | 2026-10-25T01:02:03Z | 8,661 | 0: No literal repository or distinctive-name hit. |
| sqlfluff/sqlfluff | 2018-11-01T23:56:04Z | 2026-10-25T21:14:10Z | 9,993 | 0: No literal repository or distinctive-name hit. |
| getnikola/nikola | 2012-04-14T13:53:22Z | 2026-10-24T19:43:28Z | 2,750 | 0: No literal repository or distinctive-name hit. |
| python-poetry/poetry | 2018-02-28T15:23:47Z | 2026-10-25T13:03:14Z | 34,321 | 15: Lockfile, packaging metadata and installer usage, not a Poetry source read. |
| changesets/changesets | 2019-03-21T22:33:37Z | 2026-10-16T21:27:35Z | 12,510 | 0: No literal repository or distinctive-name hit. |
| verdaccio/verdaccio | 2016-04-15T16:21:12Z | 2026-10-25T18:03:40Z | 17,943 | 0: No literal repository or distinctive-name hit. |
| keystonejs/keystone | 2018-04-05T10:48:37Z | 2026-10-23T08:38:27Z | 9,981 | 3: A domain-model metaphor, not the Keystone repository. |
| slidevjs/slidev | 2021-04-24T01:25:23Z | 2026-10-23T06:44:44Z | 49,214 | 0: No literal repository or distinctive-name hit. |
| yarnpkg/berry | 2018-09-10T22:49:22Z | 2026-10-23T14:11:35Z | 8,117 | 1: A package-manager protocol description; no Berry source tree in a corpus or pilot. |

## Every shortfall

The fifteen-candidate selection has **no small-band subject**. Its middle/large/above-ceiling counts are Go: 2 large, 3 middle, Python: 1 above ceiling, 2 large, 2 middle, TypeScript: 3 large, 2 middle. There is no claim of a complete three-band spread.

- **Go**: missing kept bands small, middle, large; 6 preparation subjects still needed; all three dev and three held-out assignments remain pending.
- **Python**: missing kept bands small, middle; 5 preparation subjects still needed; all three dev and three held-out assignments remain pending.
- **TypeScript**: missing kept bands small; 4 preparation subjects still needed; all three dev and three held-out assignments remain pending.

D27 revisit: the Go application slice failed C3 preparation in four of five candidates, compared with two of six Go library candidates in the previous reading. This fires the C3 revisit condition for this isolation setup. Root permissions, fixture warming and a short cache TTL confound an intrinsic application-versus-library conclusion; the round does not reverse the application-first policy. C5’s below-thirty proportion is eight of fifteen, versus fifteen of twenty in `c5-r3.json`; its revisit condition does not fire on these surrogate counts.

The requested round screened exactly five candidates per language. Including the earlier six Go, six Python and eight TypeScript candidates, cumulative searches total eleven Go, eleven Python and thirteen TypeScript candidates. Search volume does not fill the enrollment or split shortfalls.

## Disk budget and retained evidence

Free space was checked with `df -g ~` at the start and before each candidate/build. No check fell below 20 GiB. All clones, source copies, dependencies, caches and outputs stayed in the job’s `workspace/`, with later suite orchestration guarded at 7 GiB below the 8 GiB hard cap. The largest recorded workspace check was below 5.5 GiB. Candidate clones and build trees were removed after the measured trials or refusal; only receipts and logs remain: 22,552 KiB at the final check, with 37 GiB free. Oracle dependencies and all orchestration caches were also removed. The archive includes cleanup and sampled-budget receipts. The raw archive was scanned before placement for local home paths, account/host identity, unsafe members and archive owner/group names; no author metadata was collected. Test fixture identities are synthetic.

## Verification

```sh
python3 calibration/bench/tools/verify_apps.py
python3 calibration/bench/tools/test_remeasure.py
python3 calibration/bench/tools/test_recount.py
cue vet -d '#Subject' ./schema/bench/schema.cue calibration/bench.d/slidevjs--slidev.yaml
task cue-vet
```

The verifier also rejected three deliberately invalid scratch archives: a wrong archive hash, a nonzero member owner id, and a test caller inserted into a D26 receipt with hashes recomputed. The wrapper regressions cover failed staging, failed working directory and a successful multi-command body. `command gofmt -l calibration/bench/tools/go-c6/main.go` and `git diff --check` pass; no Go file changed.

These checks bind the fifteen candidates, complete archive membership, every member SHA-256, explicit C3 phase/log links, product-only C6 counts and kept manifest hashes. Native macOS C3, C4, actual C5 replay, Q2 cutoff and split, and live-fleet C7 remain unverified. `task validate` was attempted in an isolated job-owned copy and was blocked before its checks: the sandbox denied the standard shared gate lease. The lease was not bypassed. The full gate remains unverified.

## Raw log hashes

| Archived raw log | SHA-256 |
|---|---|
| `logs/changesets--changesets-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/changesets--changesets-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/changesets--changesets-setup.log` | `a3962c9e314a051c0ea7e3e2c1d24e7f73992eb313b895ca310a814262fec1cf` |
| `logs/changesets--changesets-warmup.log` | `c6192d61c985effb0695413d758ac2b663dd6ffe60ed69be6aa8587a5fe082a2` |
| `logs/charmbracelet--soft-serve-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/charmbracelet--soft-serve-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/charmbracelet--soft-serve-run1.log` | `6222ca3a37036d533b22e53b615fb6178ea88427e874f51f54a46731a156995c` |
| `logs/charmbracelet--soft-serve-run2.log` | `7c069a1a90f73b529c9a110945a09b1c2430626d059a9216437a1909e4b390ca` |
| `logs/charmbracelet--soft-serve-run3.log` | `3786194226d58b4fb771e56eaa6023ec0c4566f628e5562e855068bfa221c141` |
| `logs/charmbracelet--soft-serve-setup.log` | `89097cdf07b76b2c4ea6ce2879fb9d849449c537c82b2b6422cbe1254f8e984c` |
| `logs/charmbracelet--soft-serve-warmup.log` | `7937ef1be90e8cc40949b4be4386c1477848bd374adc335cc72a831cc90e3c8c` |
| `logs/derailed--k9s-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/derailed--k9s-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/derailed--k9s-setup.log` | `89097cdf07b76b2c4ea6ce2879fb9d849449c537c82b2b6422cbe1254f8e984c` |
| `logs/derailed--k9s-warmup.log` | `de939e2cf65ba676e464aaf4f7319285ba300bdddec6d8abe479c1783ba7bc39` |
| `logs/go-shiori--shiori-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/go-shiori--shiori-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/go-shiori--shiori-setup.log` | `89097cdf07b76b2c4ea6ce2879fb9d849449c537c82b2b6422cbe1254f8e984c` |
| `logs/go-shiori--shiori-warmup.log` | `05ef31c23ca5205237edca5de760afafeb917c3a10da638abf65f8508ef05017` |
| `logs/goreleaser--goreleaser-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/goreleaser--goreleaser-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/goreleaser--goreleaser-setup.log` | `89097cdf07b76b2c4ea6ce2879fb9d849449c537c82b2b6422cbe1254f8e984c` |
| `logs/goreleaser--goreleaser-warmup.log` | `590694bb1b7535fd8c7fa6d52a8830d3b647aec31364b7b0621282c92ee52d87` |
| `logs/keystonejs--keystone-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/keystonejs--keystone-setup.log` | `3c5816b09e2f318812f75813e0b5f4ae321703fa0319e67d5966e6c593f6279b` |
| `logs/micro-editor--micro-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/micro-editor--micro-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/micro-editor--micro-run1.log` | `b4b7131255e80ef2892841e84460d000af6cf80b96585f702869a939e0f5bfc5` |
| `logs/micro-editor--micro-run2.log` | `18e605028af01f77dabcb1f7eaa917c80a14f63a8e6666f83297c44ee8b562f8` |
| `logs/micro-editor--micro-run3.log` | `4f02f2a0683bca84e04952088f48b94729527a542cecab9b8afc65d98d7eec40` |
| `logs/micro-editor--micro-setup.log` | `89097cdf07b76b2c4ea6ce2879fb9d849449c537c82b2b6422cbe1254f8e984c` |
| `logs/micro-editor--micro-warmup.log` | `4b92a8f9d75fe1bcc0e8ca7989b5b1cdd0ab4393e42e441ca5f45fd5e0107a73` |
| `logs/pdm-project--pdm-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/pdm-project--pdm-setup.log` | `aa0a433379c0bc2b5bfb99c81c30b67961e0b89c7fd5897328a2e8dd9a66befb` |
| `logs/pdm-project--pdm-warmup.log` | `5606cab698978128a2ff001c5e574934f362362ba0f0e562ce04d6bcb7af35e7` |
| `logs/python-poetry--poetry-setup.log` | `4f5186568efd4d786efd74b72536c475e36ae0112492b88096f740d89c8ee441` |
| `logs/slidevjs--slidev-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/slidevjs--slidev-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/slidevjs--slidev-run1.log` | `12529340ba588c92c088c61002dbc52504179918e92145fa44c6c6947df38283` |
| `logs/slidevjs--slidev-run2.log` | `c8d41f51c65c8f2018307c989b1cdd1059938a93b95fe328d03d4c953c8ea615` |
| `logs/slidevjs--slidev-run3.log` | `72cda8ae9fbc254da94a6d6d132dc64e0426df4c5a49d5a695a87ff8ccc27c93` |
| `logs/slidevjs--slidev-setup.log` | `9d6ed22e8f375fab1ce7acff85d78f5b58fdd831249b219b404866b9309a1c60` |
| `logs/slidevjs--slidev-warmup.log` | `b33530fbafeb3049557e775e5255e26bace6bd194643d4612c4a749b01f069e0` |
| `logs/verdaccio--verdaccio-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/verdaccio--verdaccio-network-probe.log` | `f9cf55b50dba65ca08df39180414f845e67c0f97277b46e404c346152653260d` |
| `logs/verdaccio--verdaccio-setup.log` | `c0ce39d1d9e9d0154ac1970997703b204ece4b19f85628b8d967ce62757ee22a` |
| `logs/verdaccio--verdaccio-warmup.log` | `a37264bc261cbc7ae87dea3459c80b443861f63908c58a641b417cecd9213fa2` |
| `logs/yarnpkg--berry-c6.stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `logs/yarnpkg--berry-setup.log` | `0ea191fde30cf8b17ac88c9dd677a42752503139b27251162b5e8eb91c15b540` |
| `preparation/pdm-project--pdm-setup-initial.log` | `293c9409d6301db37570380f44380c84a1e04fd1019fb0016cee09c1f751da71` |
| `preparation/python-poetry--poetry-setup-initial.log` | `4bddf4938e72d4ffffa0c6d7a9c6d9fa73d0509ff9eaa9f695437eb159886302` |
| `preparation/slidevjs--slidev-setup-unreceipted.log` | `dadf4d5ac3fb996d00846db45814289d80f2637088d8e9566b6bc70dce5715ca` |

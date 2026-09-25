# Scoped CLI invocation-exit migration

The owner authorized active app-kit/Bosun/Surveyor API, dependency, test and guidance
changes. [ADR-0070](../adr/0070-opt-in-cli-invocation-classification.md) preserves default
kit execution and explicit operational policy while selected framework invocation
refusals migrate from exit1 to2. This is working-tree adoption, not installed binaries,
an app-kit release, full Platform validation or operational-command execution.

## Delivered change

- App-kit adds `ExecuteWithClassifier` and `InvocationOriginCode`; unchanged `Execute`
  remains upstream-compatible. Explicit exits win, including zero and ordered joins.
  Only complete recognized framework-origin compositions select2; no text matching.
- Bosun and Surveyor explicitly enable origins in their entrypoints and select the
  shared classifier. Existing curated diagnostics and all internal command code remain
  unchanged. Tests now expect2 for unknown flag/command, while ordinary/mixed errors
  and a usage-looking operational error still expect1.
- Each consumer selects its own generated Cobra1.10.2/pflag1.0.10 patched sources
  through local replacements. The 156 source/license/test files per consumer retain
  their exact patch receipt. Cobra's copied `go.mod` additionally selects the sibling
  pflag at1.0.10 for independent module tests; no source is patched in the module cache.
- [Dependency delivery](fixtures/cli-consumers/cobra-origin/README.md) verifies the
  pinned original receipt and expected output, rather than trusting a self-authored
  manifest. Copied source changes require reviewed regeneration, not hand edits.

## Red, green and refusal evidence

Private root: `/Volumes/m2ext/appkit-cli-migration.PTQu01`.
Exact executed preparation/delivery/test runners, selected source/tests, original
consumer entrypoints/tests/modfiles and logs are retained. This is not a complete
archived dependency closure for the consumers. Their existing local app-kit replaces
remain; no standalone consumer release claim is added.

| Check | Result |
|---|---|
| Two independently generated dependency candidates | Identical manifests; original full upstream module sums verified |
| Delivery checker |14 expected outcomes, including source/manifest joint tamper, missing/extra/symlinked source, receipt tamper and overwrite refusal |
| New consumer expectations before migration |Exactly four usage subtests fail at1 instead of2; operational cases pass |
| Kit command race suite, ordinary upstream graph |92 passing test events, no failures/skips |
| Migrated Bosun and Surveyor entrypoint race suites |9 passing test events each, no failures/skips |
| Patched Cobra full upstream package-tree race suite |424 passing test events, no failures/skips |
| Patched pflag full upstream package-tree race suite |177 passing test events, no failures/skips |
| Actual patched-origin oracle plus unchanged kit compatibility |241 passing test events (149 origin +92 kit), no failures/skips |
| Each consumer's existing `internal/cmd` package race suite |Pass; synthetic adapter tests, no real CLI leaf invocation |
| Fresh CGO-free consumer builds |Both succeed; both dependency inventories still pass afterward |
| Unchanged guarded runtime matrix |13/13; help/version0, all four unknown-input cases2, expected diagnostics |

Test events include subtests and parents; these are not independent test-case counts.
The old consumer exit1 tests remain in `before/`; the new expected2 tests fail in
`red/` before the entrypoint/dependency migration and pass unchanged in `green/`.
Historical failures and the isolated adapter's legacy-expectation failures remain
separate evidence, not relabeled successes.

The fixed runtime probe first proves synthetic read, write and loopback-network
denial, then runs only help, version and deliberately invalid inputs from retained
binary copies. No operational leaf command, real dataset, installed binary, signing,
deployment, commit, publication or global configuration changes occur.
Both owned working directories stay empty; retained inputs match before and after.

## Identities

| Retained artifact under private root | SHA-256 |
|---|---|
| `preparation-result.json` | `47c1e379e0a89dffc11a176f0d7292e9fbfec9265b8e0e536449f7312bcda4e8` |
| Each source `receipt.json` | `4af4f21d8ee27f4bd403df24136825ac72edf995207e0c1f7b29aab942dbc7c5` |
| Each delivered `delivery.json` | `aeec70b8cc6d3c5135c426519e5ceee39a8e533c2d63d732456657e67222191d` |
| `delivery-test/result.json` | `cd678aa5a3a1b4dcc43ca489f7d3c0751b1f80f1a38dbbd8249915468b87248b` |
| `red/result.json` | `81b646dfa80069d4c81286d9f044c482f7bda6f54cdb0e1a170a15b3bb91eeb3` |
| `green/result.json` | `4912292c36520774aa02aa148b02537201a11cbbb5bd5d768365532ee96c4786` |
| `extended/result.json` | `b6676d7acade5f53e9fc6b839d0799afe56c7b283b39933373206f35b7341379` |
| `sandbox/run-receipt.json` | `2ca953f0e30b3bf0ac296b26247601fc50c192b3800eb65d4452db463ac46c0a` |
| `extended/bosun` | `a1a197f85a9dc0e9e2cdc3cbc18e88d6a3aa8567277176ecedd1852dcdf758b9` |
| `extended/surveyor` | `aff1bfdff04c18b71a76e31ebf22cd8dd7ee94f4ab12ab497603027f9080d691` |

The two additional `internal/cmd` runs use the same retained `green/source` test files
and isolated cache/temp settings, via `go test -p=2 -race -count=1 ./internal/cmd`;
their terminal outputs report2.419s (Bosun) and2.708s (Surveyor). They are not included
in `extended/result.json` or its event counts.

## Maintenance and remaining boundaries

Each adopter's `docs/app-kit.md` and paired agent guidance route to the selected policy,
dependency checks and contribution path. Do not silently migrate other fleet CLIs.
Shell callers must update usage-code expectations from1 to2; operational/explicit codes
and diagnostic acknowledgement remain unchanged. Default kit callers need no migration.
Root recursive Platform gates see the new nested modules; none was excluded or weakened.
Those full repository gates were not run by this slice. Installed-host and publication
evidence remain separate. This migration alone does not prove terminal interaction;
the [G5 acceptance matrix](2026-09-16-terminal-acceptance-matrix.md) combines it with
the reference's current-code interaction/machine evidence. No plan box was checked
from these counts.

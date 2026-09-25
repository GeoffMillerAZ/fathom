# Private Gallery candidate refresh

G4/G6 preparation under ADR-0045/0068 and I8/I9/I17/I19. This work creates private
reviewable artifacts, not a release, installation or permission to replace a running app.
The [known-good historical bundle](2026-09-16-private-bundle-runtime.md) remains intact;
its runtime result is not inherited by a new candidate.

## Source-selection refusal and repair

The first fresh root, `/Volumes/m2ext/appkit-current-candidate.Eimj5C1d/`, prepared
a 1,221-file candidate, then refused host source `cmd/bundle-launcher/exec_darwin.go`
before any server/native build. The host directory allowlist predated the private
packaging commands. `host-rehearsal/` remains empty; the failure and original sources
are retained rather than replaced by a successful result.

The repair admits only three exact directories: `cmd/bundle-launcher`,
`cmd/prepare-bundle` and `internal/macbundle`. Existing path, extension, symlink,
size/count and unknown-directory refusal rules remain. Two additive canaries fail on
the old code and pass with the repair: exact directory selection and a synthetic
Git-visible snapshot binding bytes/hashes while still rejecting a nested paid path.
The original tests are unchanged. Both preparer/rehearsal packages pass; their final
source snapshots and summarized results are retained at
`/Volumes/m2ext/appkit-host-source-repair.p1uxxFyW/` (`original/`, `red/`, `final/`).

Source retention does not imply execution: the host rehearsal still builds only its
existing server/native targets and runs its selected HTTP/hostwire/preparer tests.
Packaging command and native-control helper source in the snapshot gains no task pass.

Review also identified a pre-existing limit: `readHostSource` checks paths/types/sizes,
then reopens by pathname for `os.ReadFile`. It is not an atomic or adversarially
race-resistant reader: concurrent replacement/growth could precede its post-read checks.
This rehearsal freezes cooperative source and compares the final snapshot; that does
not detect every possible transient race. The new stable-file fixtures prove their
own byte inventory, not concurrent-file confinement. Descriptor-bound bounded reads
and replacement/growth canaries would be needed before making that stronger claim.

## Distinct prepared candidate

Fresh root: `/Volumes/m2ext/appkit-current-candidate-final.tHngq0rX/`.
The candidate contains 1,221 selected files / 10,355,531 bytes. Its full version is
`v0.0.0-rehearsal.h12d33fb44dc0787a00330059dd32760d1a66b57b4182a5bde31836e3cedd65e0`.
It captures reviewed working-tree bytes at preparation, not a clean commit or later
assessment edits. The native spike is separately captured by the host rehearsal;
it is deliberately outside the root module ZIP.

| Artifact | SHA-256 |
|---|---|
| First candidate receipt | `11f0d30ffcfb12d130bf16760ee796656a5e75e5cd59ec7073e620d8ec9b043b` |
| Repaired host selector | `2456323a62b47d16cb88b4a61a7f4cffe910cb93c7ffd5fc2aad9882349c5e7d` |
| Same-oracle additive tests | `5a4f7ec6b056f7a37e4afcf7f92601e9e7971a5a995d517b1c9d3b467d976fef` |
| Fresh candidate receipt | `ff1f19088f8c9ef9037cd531cf3a6b064163af25873a2e94a34eb2c186e83f61` |
| Fresh module ZIP | `9c2b74addd4436b81a200aa8a3be97e90c3a63f8f1f5c182f2b54fe32e002049` |

Keep the candidate private. Source allowlisting and ZIP integrity are not a license,
secret or publisher-authentication audit. The historical bundle, consumer checkouts,
dependency pins, installed binaries and global configuration are unchanged. Kit edits
are limited to private verification helpers, tests and documentation in this slice.

## Read-only native-workflow discovery prerequisite

A separate [Swift inspector](../../spikes/wails-gallery/native_workflow_discovery.swift)
and [fixture runner](../../spikes/wails-gallery/native_workflow_discovery_test.swift)
prepare for unchanged-bundle workflow testing. They do not modify the app or add a
privileged evaluation/control endpoint. The live executable accepts only a PID and
its exact observed absolute `appkit-native-companion-*/gallery-native` path, checks
existing Accessibility trust without requesting it, and reads only that app's windows.
It neither activates a window, changes a value, presses a control nor opens a browser.

The inspector reports ten named Profile/Activity controls, matching exact names and
roles. Hidden, duplicate, disabled, unsupported and unreadable controls are distinct;
an unreadable peer prevents a uniqueness claim. `unique-actionable` means the observed
metadata permits the requested kind of action, not that an action or user task passed.
Window ordinals apply only to that observation; they are not stable cross-call IDs.

Bounds are two windows, 512 recorded nodes, 64 children/node, depth32, 256 UTF-8 bytes
per inspected string and32KiB output, with visited-element cycle handling. The
three-second traversal deadline is soft between AX calls; any caller must enforce an
outer process deadline and independently bind executable hashes, ancestry and cleanup.
No AXURL is read. A successful process exit is not a workflow pass: callers must check
completeness and every required control's status. This preparation ran no live AX traversal.

Private stage: `/Volumes/m2ext/appkit-native-workflow-discovery.M5s4UN/`.
This is the initial inspector identity; the later [disclosure-structure preparation](2026-09-16-native-disclosure-scroll.md#structure-diagnostic-prepared-no-new-native-run)
adds44 offline checks with a distinct exact executable and no new native run.
Its subsequent [historical-bundle structure inspection](2026-09-16-native-disclosure-scroll.md#actual-read-only-structure-inspection)
now records a collapsed `AXSummary` and clean lifecycle; it neither opens the control nor tests this new candidate.
On macOS26.6.2 arm64 / Apple Swift6.4, production and fixture variants compile, and
the fixture executable passes27 checks. Exact pre-compile source copies still match
the workspace after testing. Two earlier compile-failure stages remain listed in the
receipt. Fixture selection/bounds tests do not exercise the live AX traversal or prove
real control availability, keyboard/pointer behavior or accessibility conformance.
Builds used `xcrun swiftc -parse-as-library -module-cache-path STAGE/cache`; the fixture
adds `-D DISCOVERY_SELF_TEST` and the test source. The inherited shell environment was
not overridden, so this is not a hermetic or independent-cold Swift build claim.

| Discovery artifact | SHA-256 |
|---|---|
| Source | `e17ff1094fd97715958a13e6fc6f42b89e9556367fa2ed2f275b80ff6ec2bd30` |
| Fixture source | `f1868291e74544765037e6bc3d99cb8088239581fc79993f4c031deb59cb6192` |
| Compiled inspector, not executed during preparation | `37afa1bbae1d871e63c1a57f3395a47357c1299e6b1c398f129aafe9ca3153dd` |
| `checks.json` | `8d3d0e7d45f1ff75cbc04bdf9b5f40cec3d2a9db68a35e6e34c9bab2bc1eecd2` |

A later [unchanged-bundle discovery test](2026-09-16-native-profile-workflow.md)
executes that exact inspector and retains the initial shallow tree plus populated
readiness observations. It provides no runtime pass for this new candidate.

## Completed independent host builds

`host-rehearsal/` under the fresh candidate root now passes both independent cold
builds. Its retained native source contains81 files /591,053 bytes, including the
unchanged discovery source/fixtures, which the Go rehearsal does not execute. The
final inventory is unchanged; root independently rehashed all81 source and retained
snapshot files plus all four delivered executables against the receipt.

Each root starts with empty Go build/module caches, uses the explicit existing local
dependency proxy, and denies network and both fleet source roots. Refusal canaries
pass. The separate loopback-only test policy passes the three selected actual host
tests (POST/SSE, canceled startup and native-free server dependencies), plus the
hostwire and prepare-wails package suites. Each root retains12 command records;
44 stdout/stderr log files and their manifest remain beside the build receipts.
No app, native window or browser was launched by this rehearsal.

Both server binaries match, and both contained native binaries match; each native
Mach-O declares minos14.0. This is linked-target evidence, not runtime-on-macOS14
support. Embedded Gallery CSS comes from the exact candidate; there is no new CSS
build or visual review in this run. Go1.26.3 darwin/arm64; no dependency upgrade.

| Independent-build artifact | SHA-256 |
|---|---|
| `host-rehearsal/host-receipt.json` | `3bb7d3c5633d77202dfa387bdacb28dd8819dffc1e27a01b73c03b33cb61cb8c` |
| Both server outputs | `d3536a443b9675cd0bc210a2fa85558f659552ddb03aa373aeea05a32b2b13f8` |
| Both native outputs | `852ce73ea035a42407d57298e564db4be5a28660148e4674ee909e8f59ec5d3c` |
| Preparer/rehearsal `go test -json` audit | `73a181071c37fa739e95ec73aafc00679ef222691aad1ab4610019e1934dec69` |

The preparatory JSON test audit retains actual stdout/stderr and source/configuration:
22 top-level tests /33 including subtests, two package passes, zero skips/failures and
empty stderr. These counts are not added to the native host test counts. The final
build handoff found no scoped leftover processes; the old bundle's hashes still match.

## Remaining proof

This independently built candidate has not been assembled into a bundle, launched or
used for shared native workflow, failed-start or manual rollback acceptance. Those
need separately retained exact artifacts and their applicable execution authority.
The known-good source identity remains `6b98f004a2a97fa0d7ea3b19d0e29ec2aba45b2d757a77e3ca8a3a108c0af705`;
it is not overwritten or relabeled as the new candidate.
Windows remains deferred; Cmd+N, media and physical-input limitations remain separate.

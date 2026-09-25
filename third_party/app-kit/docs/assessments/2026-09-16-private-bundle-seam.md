# Private macOS bundle assembly and check-only validation

G6 implementation under [ADR-0068](../adr/0068-prepare-private-unsigned-macos-delivery-first.md),
ADR-0047 and I8/I9/I17/I19. This pass did not execute the delivered Gallery server/native
binaries or request Finder/browser launch, installation, signing or OS-policy changes. Synthetic
server-module tests remain separate from actual app execution. Windows remains deferred.

## Implemented boundary

[Runbook](../../spikes/wails-gallery/BUNDLE.md): private `prepare-bundle` and
`bundle-launcher` commands plus `internal/macbundle`. The fixed bundle contains a
launcher, server, native companion, canonical manifest and generated Info.plist.
The launcher resolves its own location, verifies inputs and replaces itself with the
HTTP parent using only the paired native path/hash. Its `-check` mode does not execute
either app. The legacy plist's direct-native entrypoint is not used.

Preparation accepts only a fresh absolute real `.app` destination, explicit source
hashes and declared kit/minimum-OS metadata. Files are regular, nonempty, executable
and at most256MiB; metadata is at most64KiB. Type/mode/size precede opening, then
descriptor identity is rechecked. Hashing and copying share the same source descriptor,
bounded copies recheck bytes, and outputs are exclusive/private. Partial failures stay
for inspection rather than being overwritten or silently removed.

Quarantine on source leaves/ancestors and the bundle is refused without alteration.
Other OSes cannot silently pass macOS inspection. Bundle directories, their immediate
parent, executables and metadata reject group/world write. Relocation retains those
checks. This is not publisher authentication or hostile same-user race protection.

## Tests and review

- Focused Go race suites: three packages,18 top-level tests and35 subtests pass;
  scoped vet passes. The separate `-tags server` module run passes six packages,
  58 top-level tests and161 subtests. Counts overlap; do not add them together.
  No skips occurred in these selected builds; native-only tests are not their scope.
- Reviewer-requested ancestor-quarantine, staging-parent and manifest/argument-digest
  canaries fail on the retained pre-repair source, then pass unchanged after repair.
  Exact selected sources, tests, harness, commands/configuration and output are retained.
  FIFO, oversized sparse input, duplicate/trailing/unknown metadata, symlink, byte,
  collision and no-execution boundaries are also covered.
- The maintained compiled rehearsal uses inert distinct server/native fixtures. The
  final run passes57 assertions, including15 read-only quarantine inspections and
  ten preparer/check-only invocations. It proves byte/mode-identical assembly,
  relocation, altered/missing-file and metadata refusal, extra-flag refusal, and
  preservation of the known-good copy and occupied output.
- A prior26-assertion compiled run remains retained. Review then added quarantine
  inspection before copying supplied helper binaries; that earlier run used verified
  local builds, but did not prove the stronger helper-input refusal boundary.
  Two synthetic helper leaf/ancestor regressions fail against that retained runner,
  then pass against the current runner with the identical test oracle. They verify
  retained refusal evidence, no helper copies and zero preparer/check invocations.

## Actual retained Gallery artifacts

A separate18-assertion rehearsal reuses the exact
[independently built](2026-09-15-independent-gallery-host.md) server/native pair.
Their original receipt and both binary hashes match before and after assembly.
Two fresh bundles have identical full file/directory inventories, bytes and modes.
Both generated plists pass `plutil -lint`. One bundle is relocated and its real
launcher runs only `-check`, returning the exact paired paths/hash and kit selection.
The other staged bundle remains unchanged. Neither is yet a runtime-known-good bundle.

The new launcher and old server link with minimum macOS12.0; the native links with14.0,
so the actual bundle declares14.0. This is load-command reconciliation, not runtime
support on macOS14. Read-only `codesign -d` reports all three executables as embedded
ad-hoc/linker-signed, without a TeamIdentifier. No signature was created or modified;
private unsigned here means no Developer ID/notarized distribution claim.

Host: macOS26.6.2 build25G83, arm64; Node26.3.0. Helper builds use Go1.26.3, CGO=0,
trimpath/buildvcs=false and warm offline caches. The manifest's kit revision identifies
the historical host's908-file candidate, not today's whole dirty checkout. The launcher
has its own retained current source/build identity. Later UI work is not in those hosts.

## Retained evidence

Private root: `/Volumes/m2ext/appkit-bundle-seam.tHv3fmlT/`.
`red/` contains the executed pre-repair sources/oracle and three demonstrated failures;
`final/` contains the selected post-repair snapshots, logs and helper binaries.
`compiled-rehearsal/` is the earlier run; `compiled-replay/` is the reviewed replay.
`actual-artifacts/` retains its executed script/configuration, prior receipts, inspections,
complete inventories, `Gallery-A.app` and `Relocated.app`. No sources/corpus are copied
as bundle resources; the executable payloads are unchanged from the retained builds.

| Artifact | SHA-256 |
|---|---|
| Focused test/build `final/report.json` | `981efae43a07d28fb552abe33769a75b247fbcad825a6c45cc5b30bd5be34e1c` |
| Final compiled `compiled-replay/receipt.json` | `7d25f74213b9c7ccb6e91330d7627e63ee9ea6ac77f26970f938eb3b5aa0ff14` |
| Actual-artifact `actual-artifacts/report.json` | `7e00fa419723cd4bd6f1de266f776e811c562c22f80d0ce5b4747af2973ba963` |
| Helper-refusal red/green `probe-tests/report.json` | `fa64187e74ac070969ddf1a36bc62012595289d23b900e7cacd1e6c1531a3728` |
| Preparer | `fdbbc2cf0060d77c72f3c5e1921ee14f34b896a107236cc52250b5b3b1ee5ef2` |
| Launcher | `6a4d8c87f5b479b007393a61142bb593510e594e7bcdfeaa306c1799db4d7b13` |

The compiled receipt pins its exact runner/configuration and synthetic fixtures. The
actual-artifact receipt separately pins the real server/native and older kit identity.
These are local retained evidence, not an authenticated release or full transitive build
closure for the new helpers. Runbook/intent prose was updated after the test snapshot;
the executed Go source and binary identities remain unchanged.

## Still required

Apply the [private delivery matrix](../design/web-desktop-parity.md#private-unsigned-macos-delivery)
to a separately scoped real bundle launch: normal OS/Finder policy, relocated shared
workflow, close/reopen/shutdown, startup recovery and manual old/candidate rollback.
Preserve the earlier native Cmd+N/media failures. A check-only relocation is not a
relocated app launch, and synthetic package tests are not native workflow acceptance.
No consumer upgrade, release, dependency change or global configuration occurred.

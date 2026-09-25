# Independent Gallery server and macOS host delivery rehearsal

Date: 2026-09-15. G4/G6 progress under I8/I9/I15/I18/I19 and
ADR-0026/0035/0045/0047. The native experiment is not promoted. No consumer changes,
dependency upgrades, commits, publication, signing credentials, installation or native launch.

## Implemented and exercised

The [maintained rehearsal](../../tools/releasecandidate/README.md#opt-in-real-gallery-host-rehearsal)
now accepts `-host-source` to build the actual Gallery host, not a replacement fixture.
It freezes 58 Git-visible spike files (375,710 bytes) and the executed tool before builds.
The final source snapshot was unchanged at the end of execution and includes the spike
README's delivery update. Production/test Go source did not change afterward.

The spike preparer adds explicit `-appkit-version VERSION`. It requires an exact
private content-identified app-kit selection with no replacement, validates that selection
before and after preparation, skips sibling-checkout reads, and retains the existing Wails
pin/checksums and both verified patches. Its no-flag development behavior remains unchanged.
The caller validates the full kit receipt, ZIP inventory and both Go sums; the preparer
alone is not a candidate authenticity or license verifier.

Each fresh root stages only the known app-kit development replacement into an exact
candidate requirement. Explicit kit/Wails downloads and `go mod graph` resolve the local
cache prerequisites; ordinary offline Wails preparation then verifies its selected graph.
Builds keep public checksums enabled and deny networking and both owner fleet source roots.
Actual source-read and network-denial canaries pass. The HTTP test policy separately allows
localhost only; a non-loopback connection must still receive an explicit permission refusal.

Go build metadata preserves a replacement's absolute path even with `-trimpath`.
The original prepared stage/receipt are retained. A separate patched-Wails copy must match
its full directory hash; a separate `native.mod` changes only that sole verified replacement
to `../patched-wails`. This produces stable build metadata across independent roots without
changing the kit requirement, patch, original stage or byte-equality acceptance condition.

## Observed evidence

| Check | Result and boundary |
|---|---|
| Exact candidate | Previously frozen 908-file kit candidate; full receipt/ZIP/module identities validated, no sibling kit replacement |
| Two cold builds | Separate empty Go build/module caches; existing local dependency proxy and compilers; server and native outputs each byte-identical |
| Native inputs | Same Wails beta.21 pin and two patches; copied patched-source directory hashes and normalized native module bytes agree |
| Linked target | Both actual native binaries declare Mach-O `LC_BUILD_VERSION` minos14.0; no runtime-on-macOS14 support claim |
| Real HTTP task | Both runs pass `TestHostGalleryPOSTAndSSE`: guarded launch, initial profile, bad-CSRF refusal, save, two converging streams, stale-write refusal, reconnect/resnapshot, EOF and listener shutdown |
| Independence/lifecycle | Both runs pass `TestHostCancelledStartCloses` and `TestServerBuildDoesNotImportNativeRuntime`; server dependency closure contains neither Wails nor runtime/cgo |
| Additional tests | Both cold runs pass `internal/hostwire` and `cmd/prepare-wails`; scoped maintainer tool/preparer race tests and tool vet pass separately |
| Retention | Twelve command records per run; full output files except the source-read canary, which retains output hashes; exact executed binary and original spike source snapshot |

Cold host tests use CGO=0 without the race detector. Native output is compiled with
CGO=1, `appkit_contained`, `-trimpath`, `-buildvcs=false` and explicit linker flag
`-ldflags=-extldflags=-mmacosx-version-min=14.0`; `otool -l` must confirm minos14.0.
It is not launched. Go1.26.3 darwin/arm64. Final command logs have no compiler/linker warnings.
Gallery CSS comes embedded in the candidate; no fresh CSS compilation or browser visual
review occurred. Earlier browser/native runtime evidence remains separate.

## Frozen artifacts

Private root: `/Volumes/m2ext/appkit-independent-host.yubaf0m0/replay-v7/`.
Candidate version: `v0.0.0-rehearsal.h6b98f004a2a97fa0d7ea3b19d0e29ec2aba45b2d757a77e3ca8a3a108c0af705`.
Its source identity and private proxy are recorded in the [earlier preparation assessment](2026-09-15-private-candidate-rehearsal.md).

| Artifact | SHA-256 |
|---|---|
| `host-receipt.json` | `640e557e06dde2591c39e9ad4030f1b034396fc56dd27fd8a82cd42f75fcd309` |
| Executed rehearsal | `512d8ce1e22fdc4f940be89b3dd0ecc205a4c6ecd423ed7aa454aaac554258b5` |
| Server binary, both roots | `95cdc86056847a3beb18b9a9d7fa327041c39ad81adb9140b6f702a8b39e4e71` |
| Native binary, both roots | `98cf0b52661e82d0a51ca2230b7f0c3289252e487b1b798fe4c4583da3350f1b` |
| Normalized native module, both roots | `6a3eeba21b0dee0519d177fe224e9810a70a41fc04600003f616a5c17e65aeca` |

Patched Wails directory sum, both copies/roots:
`h1:PUl5hzOKDZHdWVNejVTkiY5PLS+uRE3fOg5ufQMgoB8=`.
Original prepared module hashes intentionally differ because those receipts retain
their random absolute staging paths. Their go.sum hashes agree.

## Retained failures and remaining acceptance

Sibling `replay-v1` refused missing `.info` metadata during an unnecessary all-module
download. `v2` refused uncached graph metadata during offline verification; `v3` showed
that `go list -m all` also requests unnecessary `.info` records. `go mod graph` supplies
the required metadata without archive/info requests for unused implementations. `v4`
compiled both actual binaries but rejected the runtime sandbox's numeric-host syntax;
the corrected policy uses explicit localhost inbound/outbound rules and an external-denial
canary. These failures remain failures, with their executed tools/sources/receipts retained.
The v5 executable was built but not used for a full replay. V6 proved byte equality and
the HTTP tasks, but review found its requested environment target14.0 differed from the
linked binary's minos11.0. V6 remains earlier limited evidence, not linked-target proof.
V7 adds an explicit linker minimum and checks the actual Mach-O load command, with
refusal tests for absent/env-only/wrong-minimum/sdk-only/duplicate metadata. Both cold
runs pass that stronger check and retain binary equality. Only v7 is the final proof.

The [2026-09-16 browser follow-up](2026-09-16-exact-gallery-browser.md) runs the exact server
through four task variants; the [native follow-up](2026-09-16-exact-native-delivery.md)
runs the exact server/native pair through AX menu/normal-shutdown tasks. This historical
build assessment itself launched neither graphical client.
The [current-source refresh](2026-09-16-current-gallery-candidate.md) retains a distinct
candidate and later source-selection repair; it does not change this historical proof.
No full G4/G6 completion: real-consumer upgrade/release acceptance, complete browser
quality, native shortcut/media capabilities, and authorized packaging/
installation/update recovery remain open. Windows stays owner-deferred. This closes the
specific sibling-source build dependency in a private rehearsal, not production delivery.

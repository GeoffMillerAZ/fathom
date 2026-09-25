# Private candidate preparation and standalone consumer replay

Date: 2026-09-15. G4 progress under I3/I8/I13/I15/I18 and
[ADR-0045](../adr/0045-exact-pins-and-private-candidate-preparation.md). No tag,
publication, consumer pin change, root/real-consumer dependency addition, deployment or global configuration change.

## Implemented and exercised

[Maintained preparer and migration guide](../../tools/releasecandidate/README.md):
bounded Git-visible source snapshots, stable tracked-deletion/nested-module exclusions,
unsafe-file and replacement refusal, content-identified private Go proxy, exclusive
output, exact ZIP replay and file/ZIP/module/go.mod hashes. It is not an atomic
concurrent-writer capture, license/secret scanner, clean-commit release or adoption certificate.

The maintained macOS rehearsal embeds its three-file synthetic consumer and verifies
the candidate inventory against every ZIP entry, both Go sums and the version/tree
identity. Tailwind must match its embedded platform pin. Subprocesses have five-minute
limits; outputs stay in fresh retained directories, without source or network fallback.

## Observed evidence

| Check | Result and boundary |
|---|---|
| Candidate replay | Three preparations of the final selected tree have identical ZIP/content/module/go.mod identities; 908 files, 7,581,309 source bytes |
| Independent consumer builds | Two empty module/build caches, exact candidate requirement, no replace, `-mod=readonly -trimpath -buildvcs=false`; identical binaries |
| Isolation | Both runs explicitly observe source-read refusal and socket permission denial; both fleet roots are denied by sandbox policy, but the read canary directly probes only the mounted app-kit path |
| Module resolution | Go resolves the exact private candidate inside each run's fresh module cache; Go download sums match the candidate receipt |
| Synthetic HTTP | Real `ServeMux`/`httptest` responses exercise `RenderShell`, `RenderNav`, `RenderBars` and `WriteCSV`: escaped labels, unsafe-link refusal, current navigation, 12.3 versus zero versus missing, wide SVG and explicit narrow table fallback |
| CSS | Pinned standalone compiler, explicit runtime-only sources, six exact required selectors, selected theme variables and poison-selector exclusion; identical 19,757-byte output |
| Verification | `go test ./...`, both tool packages with `-race -count=1`, and tool `go vet` pass |
| Intent structure | Focused doctor subset: 12 budgets, 215 relative links, 45 ADRs/index entries, matching intent stanzas and 59 Unreleased lines pass; not full host/configuration wiring |

Each replay records eight command invocations, not eight browser tasks. No actual
browser rendering, accessibility, human task timing, native host or real business data
was exercised by this harness. Existing browser/consumer assessments remain independent.
Go: `go1.26.3 darwin/arm64`. Tailwind: the existing `v4.3.0` platform pin.

## Frozen identities and retained artifacts

Final source HEAD: `63b3a4f01168b184f91d1e701414445820e24fdc` (working tree, not clean commit).
Candidate version: `v0.0.0-rehearsal.h6b98f004a2a97fa0d7ea3b19d0e29ec2aba45b2d757a77e3ca8a3a108c0af705`.
The source snapshot precedes this assessment and the final scope/changelog evidence links.

| Artifact | SHA-256 |
|---|---|
| Candidate ZIP | `a9d6047638e2357184dfd9abef725c02f9251dd59fea0895aac7cf5f69aa57bc` |
| Embedded consumer fixture | `28bdbdfa9b16af33d5ee56aa9ba4b64dc3433fc2463cdfec50b045c7f44cbe7f` |
| Consumer binary, both runs | `85925f12ee06cffaed4c3acc834e5dad3fca1976e490f9acf32abf123eaab467` |
| CSS, both runs | `30feeca8a4d04892094182cd2042e3eef5586849e95053adc10a8d79a10335e9` |
| Handler-result JSON, both runs | `4adcb246baae3bf1036ca1d2184620b82dd6f73649b8b19b29994caa86d2b041` |
| Final rehearsal receipt | `bc945b5e4cf1c5beaf7395eb8455df45108669c3c9f913aabaec279fa9f23eaa` |
| Preparer executable | `ca2918fee859e5e7a46a8ff558c71f303ed0008ab6fefcf45ccb2cc868998197` |
| Final rehearsal executable | `6b168484f2837b0ae781cede50e0b4bd2ddc8a4d0c53a6a52a48de8aedc8674e` |

Private durable base: `/Users/geoff/Library/Application Support/app-kit/private-candidates/`.
The `2026-09-15-h6b98f004a2a97fa0d7ea3b19d0e29ec2aba45b2d757a77e3ca8a3a108c0af705/`
directory holds the candidate proxy and source receipt. The sibling `-evidence/`
directory retains the final receipt, binary, CSS, consumer module/sums and first failed
replay receipt. These are private artifacts, not installed application/configuration files.
Scratch commands, intermediate candidates and both fresh build roots remain under
`/private/tmp/appkit-candidate-rehearsal.eTJv3s/`; no source, prior candidate or failed replay directory was removed or overwritten.

## Failures retained and corrected

The first real invocation refused `/tmp` versus canonical `/private/tmp` path identity;
the harness now resolves parent aliases without bypassing leaf/collision checks.
The first isolated build then refused a missing indirect requirement under readonly
mode. A same-sandbox `go mod tidy -diff` identified only `github.com/rivo/uniseg v0.4.7`;
the synthetic fixture now declares that already-selected version and retains its sums.
No real consumer or root dependency file was changed; the failing output is retained.

Review also added binary-boundary-safe tree identities, bounded reads during growth,
explicit deleted-file handling, independent ZIP-entry hashes, separate go.mod sums,
embedded fixtures, strict trailing-JSON refusal and exact selector tests. The final
two-root run used the strengthened checker; no compiler or CSS failure was waived.

## Remaining G4 work

This replaces the one-off preparation path with a maintained, replayed private handoff.
It does not close real-consumer profile/theme adoption, complete upgrade acceptance,
host loading, or clean tagged distribution. CA/GGB and Windows remain owner-deferred.
No G1–G6 completion checkbox or conversion certification changed.

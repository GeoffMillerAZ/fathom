# Authorized Bosun/Surveyor sandboxed CLI probe

This is a bounded, authorized macOS execution of freshly built, privately
retained binaries. It is G5 runtime evidence for exactly `--help`, `--version`,
one unknown flag, and one unknown command. It does not authorize an operational
leaf command, real data, an installed binary, a consumer source change, or a
usage classifier.

## Build and retention provenance

Both builds used Go `go1.26.3 darwin/arm64`, `-mod=readonly -trimpath`,
`GOPROXY=off`, `GOSUMDB=off`, `GONOSUMDB=*`, `GOTOOLCHAIN=local`, and
`GOWORK=off`. They reused `/Users/geoff/.dev-cache/go/modules` as the module
cache and used separate mode-0700 private build caches. The source worktree was
`platform-engineering` commit `7ff0104b5aa391b62a0ae1925f65161de222552f` with
the build metadata's `vcs.modified=true`; app-kit was local replacement input at
`63b3a4f01168b184f91d1e701414445820e24fdc`, also dirty. This is a fresh build
from those observed worktrees, not a release or installed-host provenance claim.

| binary | module/package | SHA-256 | bytes |
| --- | --- | --- | ---: |
| Bosun | `kits/env-kit/cmd/bosun` | `cc2cf2bb3a34bf3784e9c2ab5d4831c6ce9573fb33be2e65451d9d210c3cdb1d` | 26,776,562 |
| Surveyor | `kits/test-kit/cmd/surveyor` | `0fecfbf7a99c0ab4addf14900e068acbd5cffa4f0e4b2e8d95d2f42f064861ee` | 26,227,666 |

The retained-runner probe was copied before execution as
`dd8f116f043b0d8522b0630074ae6d512b4d058a8ebf3eaf277d5cbe7dafc918`
(14,308 bytes). Before and after checks matched all three retained inputs.
Private raw evidence, including Git status snapshots, direct module/source
digests, complete `go version -m` metadata, offline module graphs, build caches,
retained copies, bounded streams, and receipts, is at
`/Volumes/m2ext/appkit-cli-authorized-probe.SyAync/`. Its build-provenance file
hashes include `direct-source-module-sha256.txt`
`70726dae9de79396b721bdb6b49607b21355d6731b16e3f8c7ca1984675bdb41`,
`bosun.modules.json` `f7f5247d779e514503cfd2a6efcd733ea0d7bdafcb87d18ad8309bcb4d06d709`,
and `surveyor.modules.json` `da60dd1782e63ecd03759f68bf8aec95ff5b068581c13360d0a3a5cf2eb8775d`.

One initial combined build command correctly built Bosun but stopped before the
Surveyor build because it used env-kit's working directory for `./cmd/surveyor`:
`stat .../kits/env-kit/cmd/surveyor: directory not found`. Surveyor was then
built once from its correct module root using its separate private cache; no
consumer file was written and no consumer binary was launched during either
build step.

## Sandboxed fixed matrix

The retained runner copied the probe and both binaries into an empty, private
mode-0700 trial directory, then ran the three canaries *before* launching either
copy. Read and write denial passed; the parent loopback self-check returned 200,
the sandboxed curl made zero server connections, and its network-denial check
passed. The inner probe had a five-second/64-KiB-per-stream bound per invocation;
the outer owned group had a 70-second/64-KiB-per-stream bound. Neither timeout
nor truncation occurred.

| retained case | exit | stdout | stderr | reporting | exit policy / result |
| --- | ---: | ---: | ---: | --- | --- |
| Bosun `--help` | 0 | 1,690 | 0 | n/a | pass |
| Bosun `--version` | 0 | 25 | 0 | n/a | pass |
| Bosun unknown flag | 1 | 0 | 178 | pass | fail: requires 2 |
| Bosun unknown command | 1 | 0 | 178 | pass | fail: requires 2 |
| Surveyor `--help` | 0 | 1,601 | 0 | n/a | pass |
| Surveyor `--version` | 0 | 28 | 0 | n/a | pass |
| Surveyor unknown flag | 1 | 0 | 178 | pass | fail: requires 2 |
| Surveyor unknown command | 1 | 0 | 178 | pass | fail: requires 2 |

The receipt has 13 checks: three canaries, four passing help/version cases, two
empty-working-directory checks, and four failing usage cases. Its status is
`failed`, as required by the unchanged combined usage2 oracle; the outer runner
therefore exited 1 with no runner errors. The inner receipt is 20,065 bytes with
SHA-256 `effddc0dc73d87478218f1fe741750ae9e606593b4cf93e8217c2cfa2adc6d5e`.

## Limits

This proves only the listed retained binaries on this local macOS sandbox under
the fixed environment. It does not prove installed-host delivery, a clean or
released consumer source snapshot, all source bytes independently from the
dirty worktrees, terminal interaction, non-macOS behavior, an operational
consumer command, general descendant containment, or usage-classifier adoption.
The exit-1 behavior is reported as an expected preserved failure, not reclassified
as acceptance or a request to change the exit-2 oracle.

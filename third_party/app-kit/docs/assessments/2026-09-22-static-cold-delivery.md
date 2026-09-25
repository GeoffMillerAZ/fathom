# Static reading: independent cache-cold delivery

This owner-funded, bounded continuation tests the existing ADR-0071 exporter through
ADR-0045's private candidate mechanism. It does not change the exporter, dependencies,
consumer repositories, installed tools, host configuration or deployed content.

## Frozen source and dependency boundary

Private evidence: `/Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w`.
Source is clean app-kit `76d3ca6b908de01d7946f7ef0845cadb6a3d8a48`, selected before
these documentation changes: 1,445 files, 12,803,038 bytes. The maintained preparer
excludes paid `data/`, native spikes and global configuration by selection policy;
this is not a license audit or secret scanner. The private candidate is not a release.

| Identity | SHA-256 |
|---|---|
| Selected source tree | `857a37571ba6a1e1ed99b9514c3f35a53e6e6a1b3d7f4d076fdb9e669ea7de9d` |
| Module ZIP | `1cc993284296d66e9645e47722fc02b04f8193b431ede52a08cc5bc288373c9c` |
| Candidate receipt | `cba22e4231ad4416d334df44387dee39ca33e6a6400680ff636a012a0f8b2456` |
| Dependency closure receipt | `14116b1c12680956bb93c1f5412661c3664f8777e380efcd43874edba3f0f83b` |
| Independent output checker | `cb12464191aa298afcd24f1f330beb13ac6cddd724e810603a8bca5e12e904f5` |

Version is `v0.0.0-rehearsal.h` followed by the full source-tree digest. Its Go module
sum is `h1:bY3qCvnSauvULCDNZdsSGRQNzuGrUkGZyB9gpRL0WD0=`; go.mod sum is
`h1:szV2zNzr4OLW5OW4Tvk9GY91KpoiM657i//DgSAi1pE=`. The offline public closure contains
103 graph module metadata records and archives for the 15 actual build/test modules:
133 files, 19,078,226 bytes. Exact paths, sizes and hashes live in its receipt.

Go 1.26.3 darwin/arm64 uses its resolved Nix executable and recorded compiler/linker
identities. `CGO_ENABLED=0`, `GOTOOLCHAIN=local`, `GOWORK=off`, `GOENV=off`,
`-mod=readonly`, trimpath and disabled VCS stamping are explicit. The copied Tailwind
4.3.0 executable matches the existing CUE checksum. No dependency upgrade or network
fallback is permitted. The committed Go sums still govern dependency integrity even
with the network checksum service disabled for offline execution.

## Prospective test and retained preparation failures

The contract caps the batch at 60 minutes, two cold runs and five minutes per command,
with one explained preparation/collector repair. Both runs must start with distinct
empty caches, use only frozen file proxies for module resolution, and demonstrate
denial of the live source, shared cache, other run and network. Policies also deny
preparation caches and canonical cache aliases. Ordinary OS/toolchain reads remain
allowed: this is a filesystem denylist, not complete hermeticity or malicious-build
containment. A pre-execution review clarified that limit before acquisition.

The first dependency collector stopped with `incomplete module graph record` because
it assumed every pruned graph node carried a local `GoMod` path. No dependency files
had been copied. Its logs remain under `closure` and `preparation`, but its original
executed script was not saved before repair: those logs are non-replayable diagnostics
(INC-63), not reproducible failure proof. The single allowed repair retained the exact
`.mod` from the selected local file proxy for such nodes. `closure-v2` passed; its
unchanged successful source was saved after execution, not frozen beforehand.
Candidate preparation produced a complete passing receipt
and empty stderr, but its shell wrapper then used a reserved zsh variable; an exact
process-exit receipt was not captured. That limitation is not called a clean wrapper
pass, and the candidate command was not rerun.

Draft runner reviews required exact proxy inventories, resolved tool identities,
initial-cache assertions, seven named test passes and bounded owned-process cleanup.
Unexecuted drafts and superseded freeze manifests remain distinct from the final run.

## Acquisition result

**Failed preflight; no cold build or output verification ran.** On 2026-09-23 at
02:13 UTC (September 22 locally), the exact launcher admitted the pinned v5 manifest,
verified every listed file, and invoked the frozen runner once for `cold-1`.
The runner refused at line 82 with `dependency proxy inventory disagrees with closure
receipt`, before its first child command. Empty local cache/output-preparation
directories exist; there are no sandbox canary/build/export/test logs or run receipt.
`cold-2` contains only its peer sentinel and was not invoked.

Read-only diagnosis compared both inventories after sorting by path: all 133 records,
including scopes, sizes and hashes, are identical. The first ordering difference is
index 43: directory traversal emits `github.com/golang/glog/@v/v1.2.4.mod` before
`github.com/golang-jwt/jwt/v5/@v/v5.3.1.mod`, while flat path ordering reverses them.
This is an inventory-comparison defect, not missing dependencies or an exporter
failure. It is not permission to weaken byte/hash or exact-membership checks.

| Attempt artifact | SHA-256 |
|---|---|
| Executed launcher | `295282facec7e47f2baaa2fde7d26df01e91013646acb5ff20b4904e771f5f64` |
| Executed v5 runner | `2c2ca3ab91aa69f796d6d3793a4d499e4a632ba5c340fc447df5eccd107d7f41` |
| Whole-input manifest | `0d98d8afc6c480ba735d57f30517476a6bee74ac1f22ad5febbb75dc394bf7c1` |
| Driver stdout | `4ee439212fc88eefbc281db1e4aa48edcdc59354d72f62c5bacf0b1c2ee9bb98` |
| Driver stderr | `0823fc6c4df5fdc940af7ab4dbda1a9a4a79ee3eb94e8669b6afbac7b43610fc` |

The full manifest still verifies after the failure. The maintained direct-child
exit/pipe helper is `8283a22b3b11530e95745d1996a35bb1c5e9422ae2c5e97f6d310ffa5075f294`;
v5 adds owned detached-group cleanup. That cleanup was reviewed but **not exercised**
by this preflight failure. No timeout, sandbox, binary/CSS/site equality, seven-test
set or historical-output comparison has passed in this packet.

The one preparation repair was already used. Stop here: no changed runner, second
cold run or new packet was automatically acquired. Exact failed runner/launcher and
successful post-run-saved preparer are retained in the fixture; archives remain private.

## Exact next bounded task

Canonicalize copied inventories by `(scope, path)` before comparison and explicitly
reject duplicate keys. A pure fixture must accept the observed prefix-order collision
and reject changed SHA, byte count, duplicate, extra, missing and wrong-scope records.
Preserve v5 and this failure. Require a new reviewed source/dependency/runner manifest
and budget before another cold pair; do not reuse the occupied `cold-1` directory.
Only after successful cold execution run the independent output checker. Consumer
hosting, browser/assistive-technology and remaining quality acceptance stay separate.

## Scope and replay

The [small output checker](fixtures/static-cold/README.md) validates eleven exact
files, sorted manifests, native reading/FAQ content, selected CSS literals and CSP;
detached bad-hash, extra-file and wrong-size controls must fail. It pins its imported
helper, contract and historical receipt before using them. Historical equality is
informational and never counted as a fresh browser run.

This does not establish an independent-host, CGO-enabled, cross-platform, deployment,
browser/accessibility or full performance result. Consumer content and host policy
remain app-owned. `core.static-content` quality statuses must stay partial wherever
those requirements remain. No catalog promotion follows from a reproducible build.

Later continuation: the [separate corrected cold pair](2026-09-22-static-cold-replay.md)
passes within its narrower local-build scope. This original attempt remains failed.

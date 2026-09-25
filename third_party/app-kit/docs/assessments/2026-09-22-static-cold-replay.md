# Static reading: bounded offline cache-cold replay

**Result: the selected exporter builds reproducibly from frozen offline inputs in
two independently empty cache roots.** This closes the local cache-cold question, not
consumer hosting, a release, full accessibility or broader performance acceptance.
No application, dependency, consumer, installed-tool or deployment change was made.

## Scope and correction

The new goal continuation resumes the exact inventory-order defect from
[the failed preparation packet](2026-09-22-static-cold-delivery.md); that packet stays
failed. Private evidence: `/Volumes/m2ext/appkit-evidence/appkit-static-replay.ud7EOtjr`.
The prospective contract limits this packet to 30 minutes, two cold runs and one
explained checker repair. No browser or additional build matrix was acquired.

The comparator now normalizes copied records by ordinal `(scope,path)` ordering,
constructs a fixed property order and rejects duplicates, invalid records, changed
scope/size/hash, extra and missing entries. Eleven pre-frozen unit tests pass in
`/Volumes/m2ext/appkit-evidence/appkit-static-inventory-v2.psA4Tr`. The first ten-test snapshot remains
at `appkit-static-inventory.ViU2Ga`; review added object-key-order coverage before cold
execution. Neither locale nor object property insertion order determines equality.

Reuse is deliberate: the source is the same clean `76d3ca6` private candidate, with
1,445 selected files and tree digest `857a37571ba6a1e1ed99b9514c3f35a53e6e6a1b3d7f4d076fdb9e669ea7de9d`.
The original receipts and 133-file/19,078,226-byte dependency closure remain unchanged.
No warm collector was rerun. INC-63's historical missing source/post-run snapshot
limits remain; the new cold proof independently validates the frozen selected inputs.
It does not certify every later app-kit revision.

## Execution and observations

Both runs use Go 1.26.3 darwin/arm64, `CGO_ENABLED=0`, `GOTOOLCHAIN=local`,
`GOWORK=off`, `GOENV=off`, `-mod=readonly`, trimpath and `-buildvcs=false`.
Separate empty module/build caches, GOPATH, temporary and output directories precede
the first Go command. Only the two frozen file proxies resolve modules; the exact
candidate Go sums and committed dependency sums are retained. Tailwind 4.3.0 is the
existing checksum-pinned binary, not a download-capable resolver.

The launcher pins the entire 159-file manifest. All listed inputs verify before and
after execution; Go, compiler, linker and Tailwind identities also match afterward.
The sandbox denies network, both fleet source roots, shared cache aliases, old
preparation/run roots and the peer run. It admits writes only to the owned run root
and `/dev/null`. Selected existing-file source/cache/peer reads fail with `Operation
not permitted`; the network canary catches `PermissionError`, not connection refusal.
Ordinary OS/toolchain reads remain allowed: this is a denylist, not full hermeticity.

Each run completes nine commands. All owned groups are absent afterward, direct-child
exit and pipe capture finish cleanly, with no timeout, overflow, TERM/KILL or residual
cleanup. Seven named exporter test functions pass after the initial cold build:
28 test/subtest passes and two package passes per run, zero failures or skips.
These are CGO-disabled tests, not race-detector results.

Both 23,659,394-byte build-time exporter binaries match. Both 18,209-byte CSS assets
and all eleven delivered files match. The visitor needs only the exported HTML/CSS;
the Go binary is not a browser payload or runtime requirement.

## Retained output-checker failure and one correction

The first independent checker fails because it expects three total disclosures.
Frozen source defines three FAQ identities (`profile`, `source`, `quality`) plus the
table of contents. Both historical and current exports contain four. Independent
source review confirms the oracle error; no output, budget or application changed.
The corrected checker requires exactly four disclosures and exactly those three FAQ
IDs. The failed checker `cb124641…04f5` and its stderr remain in the original/new
private packets. This uses the one admitted checker repair; neither build is repeated.

The corrected checker passes both complete sites, validates sorted manifests and
hash/size/link/native-content/CSS/CSP checks, and rejects all three detached manifest
poisons for each site. All eleven files also match historical `prepared-4wDE2G` bytes.
That permits citing the old browser evidence at its original scope, not counting a
new browser run or upgrading accessibility/performance claims. CSS literal and HTML
checks remain selected smoke checks, not computed-style or arbitrary-content safety.

| Artifact | SHA-256 |
|---|---|
| Admitted whole-input manifest | `f7ce6d0ba4bac95edadf0479d7912f7e14b6043cae13a46a3d61016ce6b8e6de` |
| Executed runner | `fa801898c3becf05abba5bee987ba4bb2cb11f3d575bf2aa5bbdcc3ef63b6218` |
| Exact launcher | `207342a2c8a060812693d43e14886f12cc67985c0fed6b7e293167725a97d076` |
| Cold-1 receipt | `d752637034129f82974de904dbd51299accd3a9f7475cb2c95bf5aa124450eef` |
| Cold-2 receipt | `b42d40c40e6f781c010017575b9fa1100638e3d692aaaca3dfca2091d0a02696` |
| Pair report | `a919a4621c3436c8767ff501d08a180410e7296c05bb95ed885d56536bceb36c` |
| Corrected output checker | `a365da4fdf5d19b8034df782b86679a69beeefa81c15f5bf4a102b8c47d6d2df` |
| Output-check report | `535058508380d8eb60528af2da58bbe7e9b19654c410172d34e22d6310e91aa0` |
| Both exporter binaries | `ad409230e45da412a3d1cfca896c1cd7e75b4217653c0e970a8331d1932bf997` |
| Both CSS assets | `bf0b8e787aaecf1e4fdafe4a7e3db724b3ea8a787fa5e03f7959aabfc5ff22ce` |
| Both site manifests | `317301ea8b114aa59c4182611c3fee3c5a662ee1731782a4591fce4053f2352c` |

## Adoption and next boundary

The [fixture](fixtures/static-cold/README.md) retains source and exact replay inputs;
archives/caches remain private. A future source/toolchain change needs a newly reviewed
closure, not silent reuse of these results. This is local same-host, CGO-disabled
proof, not independent-host, cross-platform, hostile-build or deployment certification.
For a real adopter, prove its content/routes, CSS and static-host MIME/CSP/cache/404
policy. Keep consumer usability, assistive technology and remaining performance gates
open in `core.static-content`; this packet promotes no whole catalog or runtime profile.

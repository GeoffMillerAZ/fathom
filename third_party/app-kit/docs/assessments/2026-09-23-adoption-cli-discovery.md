# Host-independent adoption discovery

## Prospective contract

Owner-resumed adoption/core maintenance, baseline `4a65a7f`, under
[ADR-0082](../adr/0082-discover-adoption-capabilities-without-a-host.md).
The existing CLI drafts explicit pinned selections but requires another surface
for discovery. Add shared-catalog search and exact guarded retrieval without a
consumer root, model, network, purchased corpus or running server. This neither
repairs the Codex host nor promotes a catalog entry or G1/G4 acceptance.

Acceptance: shared catalog query/page/exact-entry/canonical-pin parity; valid empty
searches; unknown inputs, changed/stale cursors, malformed/stale guards, cancellation
and output failures refuse correctly. JSON-only bounded output. A synthetic operator
can search, review, draft, deliberately save and statically check a selection; drift
fails and restoration passes. Existing inspect/check/pins bytes remain unchanged.
No new dependencies, consumer writes, installed tools or global configuration.

Use an archived baseline plus frozen candidate files. Run the new tests against
the old dispatcher, then focused candidate package/race/vet checks and the retained
[actual-process probe](fixtures/adoption-catalog/probe.mjs). Each probe child has a
ten-second bound; no operational app command or model inference is run. Budget:
one candidate verification and at most one separately retained correctness repair;
do not relax the acceptance criteria to achieve a pass.

Private evidence: `/Volumes/m2ext/appkit-evidence/appkit-cli-discovery.SidIBITy`.
The initial baseline preparation used a wrong copy source and ran no new tests;
`preparation-no-tests.log` is not a pass. After copying the test by its absolute
path, the frozen baseline fails five top-level new-feature tests because the
commands are unavailable. Negative-input tests alone pass there and are not
feature proof. The synthetic walkthrough's success expectation was corrected by
source inspection to the existing `passed` outcome before candidate execution.

## Results

The frozen candidate passes all four selected packages with the race detector:
`cmd/appkit`, `appkit/catalog`, `internal/adoption` and `cmd/appkit-mcp`.
There are 1,013 passing test/subtest events (74 from the nine new discovery tests)
and zero failures. Seventeen existing
opt-in/private/installed-host or archive-unavailable checks skip; they are not
passes and do not establish commercial conversion, installed-host or binary
tracking coverage. New discovery tests run, including each current exact record's
output bound. Existing MCP/Gallery conformance tests and the new CLI tests compare
against the same shared catalog/pin semantics; this is not a fresh model-host run.

Scoped vet and both offline CGO-disabled CLI builds pass with Go1.26.3,
`GOWORK=off`, `GOPROXY=off`, `GOSUMDB=off` and readonly manifests.
The archive builds deliberately use `-buildvcs=false`: they are source-snapshot
proof, not stamped clean-VCS or installed-release proof.

The frozen actual-process probe passes **25 CLI invocations**. The baseline refuses
both new commands; candidate discovery is deterministic, exact retrieval preserves
the search revision, pagination advances, and empty compatible searches stay empty.
Changed cursors, unavailable profiles/IDs, malformed/stale guards and unsupported
flags refuse with exit2/no JSON. Help exits0 separately. An existing malformed
manifest is neither required nor overwritten by discovery or drafting.

The probe explicitly saves a reviewed synthetic draft; its metadata pin matches
the retrieved exact contract and its artifact pin matches the fixture bytes.
Existing inspect/pins/check JSON is byte-identical between baseline and candidate
for that fixture. Artifact drift produces a failed check; byte restoration passes.
All 25 child processes exit normally without timeout/signal or executable drift.
No real consumer, installed tool, configuration, dependency or service was changed.

| Retained artifact | SHA-256 |
|---|---|
| Baseline archive (`4a65a7f`) | `076f2283199e1408c33c0ec5e5f44a3be2504ae0a5426fbdc4280a6e80b72538` |
| Candidate source overlay, frozen before execution | `28fee4fe76d490f2dff79dc58093b11c77c0b5cdbc3946b95ebebd1a6cc012f0` |
| Baseline CLI | `addc496753c43f1313ba39e6abf9a0a5fff9bb309ea56dc903627d6578d06e8c` |
| Candidate CLI | `63cfa2fe0b25974d2475bfde4bc5b86d951935685a5f3dae58a6f76f86882e4e` |

`baseline/`, `candidate/`, `red.log`, `race.jsonl` and `process-probe/` retain the
source and observations; the latter also retains executed harness, binary identities,
all arguments, stdout/stderr and verdicts. Independent bounded review found no
high-impact issue. Final documentation records these results without changing the
tested implementation. Real consumer adoption, automatic instruction loading,
UI quality, catalog certification and broader G1/G4 acceptance remain open.

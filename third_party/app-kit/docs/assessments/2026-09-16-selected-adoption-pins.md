# Selected adoption metadata pins — 2026-09-16

G4 improvement under [ADR-0065](../adr/0065-pin-selected-adoption-capabilities.md),
not consumer migration, installed automation, release or runtime acceptance.

## Actual consumer observations

Frozen old and new `appkit` binaries ran explicit read-only `adoption inspect` and
`adoption check --json` against these existing roots and unchanged v1 manifests:

| Consumer | Root suffix under `/Volumes/m2ext/code/geoffmilleraz/` | Result |
|---|---|---|
| Resume as Code | `resume_as_code` | Inspect observed; check fails only whole-catalog revision. All20 selected artifacts, fit, module and routers pass. |
| Fathom | `sextant` | Inspect observed; check fails only whole-catalog revision. All11 selected artifacts, fit, module and routers pass. |

The checker catalog is `57da91591dca354d52c240db310e8f0cec6da035b23de09342d4a2a8a1d92fc5`.
RAC selects `7888d771…`; Fathom selects `2a98eb25…`. The new binary reproduces all
four old JSON reports **byte for byte**, including normalized selection identities;
exit codes and stderr also match. Reports equal the earlier retained invocation.
Selected-file hashes and read-only Git-status hashes match before/after each app's
invocations. This is a bounded observation, not an atomic whole-repository snapshot.
No consumer source, manifest, dependency, router or running app was changed.

Current `adoption pins --json` retrieves full selected metadata and candidate hashes
for both apps, returning `observed`, not adoption success. RAC selects `shell.layout`
and `work.approval`; Fathom selects `shell.layout`. Both strict v1 failures remain.

## Why pins, and what still needs review

The repeated global-revision review friction already appeared in the
[navigation upgrade](2026-09-14-navigation-delivery.md#prospective-catalog-re-selection).
It fires ADR-0024's per-entry evaluation trigger, but does **not** prove every current
selected record is unchanged. The retained `2a98eb25…` Fathom `shell.layout` record
differs from today's record by one added task phrase: “mobile application shell
navigation with current route and grouped links”. All other record fields match.
V2 would still invalidate this selected-metadata change. It is not a failure bypass.

The exact older RAC `7888d771…` catalog was not recovered in the bounded evidence
search. Fathom's intermediate catalog cannot substitute for it. An authorized future
RAC migration must identify its historical evidence or explicitly make a prospective
selection against current app intent and runtime evidence.

## Implemented contract and tests

V2 requires one complete metadata pin per selected capability, no missing/extra or
duplicate IDs; v1 retains its whole-catalog requirement. Both reject cross-version
fields. Hashes bind the canonical domain/version, catalog schema and every metadata
field, including ordered arrays. Metadata identity does not bind implementation bytes.
All existing profile/layer, artifact, module, router and bounded-reader gates remain.
`pins` reads only the explicit manifest and embedded catalog, never auto-writes pins.

Library/CLI tests cover canonical-envelope identity; every metadata field; schema,
sorting and empty-selection rules; unrelated-record independence; selected-record
drift; malformed manifests; missing/changed files; incompatible profiles/layers;
module/router refusal; unknown IDs; read limits/cancellation; and output/exit handling.
Full `internal/adoption` and `cmd/appkit` suites pass with the race detector; scoped
`go vet` passes. Earlier unretained broad-CLI failure had no named assertion and did
not recur in either the retained JSON suite or full race run; no root cause is claimed.

The actual compiled CLI additionally demonstrates this synthetic sequence:

1. Retrieve current candidates (`observed`, exit0).
2. Explicit synthetic v2 selection passes (exit0).
3. Frozen old checker refuses that v2 selection (exit1).
4. Changed selected pin fails (exit1, `capability-revision:1`).
5. Restored pin plus changed artifact fails (exit1, `artifact:1`).
6. Restored original bytes pass again (exit0).

An independent code review found no material issue. A second `-trimpath` build matches
the tested new binary exactly. This uses existing caches, not a fresh dependency
installation or clean consumer build. No whole-repository `task check` claim follows.
MCP current-document/ADR tests pass, including protocol retrieval of the updated guide.
A scoped audit resolves135 local Markdown targets, verifies the selected document
budgets and ADR index entries, and compares paired Fathom stanzas. This is not a full
intent doctor or proof that installed agent hosts loaded the new guidance.

## Retained evidence and reproduction

Private parent: `/Volumes/m2ext/appkit-adoption-audit.8JK5D5ft/`.
It retains pre-change sources/catalog and binary, both initial real-app reports,
the new binary, executed harness copies, scoped current sources, complete JSON/stderr
outputs and receipts. Current sources are checked unchanged after the run.

| Artifact | SHA-256 |
|---|---|
| Initial `receipt.json` | `3c41dadde83e4b187151c0b9e7aba07927eb9ca7a1a3bcf67523e06f81155bec` |
| Old binary `appkit` | `e76fd859170db4edfa982b73be9f86385593b68aed0bb82d658a242642a7b9fe` |
| New binary `appkit-v2` | `004b35b7dde5ebb3c31540f74a35ed60dd6f7fa8fdb2356aa09969077c249aaa` |
| Compatibility `verification-0NjlYs/receipt.json` | `1ca8ad0e618ef3e6489295bd10a67576990cbdea6a223f849d5be044aa5e55c2` |
| Executed `audit-v2.mjs` | `9c43cbecaf7b373cfe915f1662520001448e4ed92d5f788a176efa7803ec6de9` |
| Race/vet/rebuild `checks-GEeS6Q/receipt.json` | `8c77bb822a5396608740caf195715edf8739b8f6cd63bef21e677feaafab08f5` |

The initial compatibility attempt `verification-lf5Mz1` is retained as failed: its
harness incorrectly required empty stderr on a failed check. The revised harness
requires the existing exact diagnostic and compares old/new stderr; no checker,
manifest, result or historical output was changed to pass the comparison.

```sh
go test -race ./internal/adoption ./cmd/appkit -count=1
go vet ./internal/adoption ./cmd/appkit
go build -mod=readonly -trimpath -o /private/chosen/path/appkit ./cmd/appkit
```

Observed Go1.26.3, Node26.3.0, Darwin/arm64; `GOWORK=off GOPROXY=off GOSUMDB=off
GOENV=off GOTOOLCHAIN=local`, with existing external build and local module caches.
Private `audit-v2.mjs` and `checks-v2.mjs` create new exclusive evidence directories;
they retain the exact roots/binaries/commands rather than auto-discovering a fleet.
An old checker must be upgraded before separately authorized v2 adoption. G4's
runtime, host, reviewed propagation and release requirements remain open.

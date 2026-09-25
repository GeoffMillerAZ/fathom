# P02 private delivery and isolated consumer proof

The typed P02 adapter now exports a usable private package and checks its exact drift.
Two independent cold-cache consumer builds compiled and rendered both variants with
network, sibling-repo and original-export reads denied. Generated Go, test binaries
and rendered HTML were byte-identical. This advances G1/G4, not their completion.

## Delivered capability

[ADR-0034](../adr/0034-source-specific-private-component-delivery.md) defines the contract.
`appkit data assets-adapt-pricing -out NEW_DIRECTORY` creates six private files:
template, model, companion CSS, defaults, README and artifact JSON. The 29,243-byte
delivery has no app-kit runtime dependency. Existing JSON output remains unchanged.
No install, compiler, backend or catalog write runs inside the export command.

The existing parent and new leaf are explicit; directory/file creation is exclusive
0700/0600. Existing paths are refused, not overwritten. An interrupted export remains
incomplete for inspection; artifact-last ordering is not crash-atomic publication.
The owner controls the directory during the operation. POSIX modes do not audit ACLs.

`-check DIRECTORY` independently rereads the original archive, regenerates the current
artifact and compares every expected byte, inventory and exact mode. It rejects absent,
extra, edited, nonregular, symbolic/hard-linked and executable-mode files. macOS/Linux
use bounded non-following/nonblocking reads with descriptor identity checks; other
platforms report unsupported. Exit 0 means exported/matching, 1 observed drift, 2 invalid
request or could-not-check. Export/check success is silent. Neither grants a target badge.

Ordinary tests use synthetic markup. The explicit paid-source CLI test proves default
JSON determinism, two equal exports, exact checks/refusals and unchanged archive/catalog.
Independent review caught hard-link acceptance and non-exact mode checks; both were
fixed and covered for all six files, with a separate FIFO refusal test. Final rereview
found no remaining false pass within the stated trusted-owner/non-concurrent contract.

## Isolated consumer rehearsal

The [fixture](fixtures/pricing-adapter/delivery-proof.mjs) materializes one immutable
delivery, then copies it into two fresh private consumers before compiling. App wrappers
and generated code stay outside the checked export. It invokes no real consumer app.
The only direct dependency is templ v0.3.1020, also pinned as the compiler tool.
Go 1.26.3 darwin/arm64 generated/compiled it; its native binary identity is retained.

Both runs use empty module/build caches, cleared environment, GOENV/GOWORK off,
GOTOOLCHAIN local, CGO disabled, readonly module inputs and a file-only public ZIP proxy.
There are no replace directives or fleet modules. Compiler and consumer dependency
directories resolve only within their fresh cache or copied consumer. macOS sandbox-exec
denies network and reads under the fleet root and immutable export; canaries prove denial.

| Check | Observed result |
|---|---|
| Two actual templ generations / test-binary builds | pass; identical generated Go and binaries |
| Named synthetic runtime test, twice | pass; constructors, required app URL, rendered variants |
| httptest routes | GET component/destination 200; POST 405; missing route 404 |
| Original/sibling file reads, twice each | denied with `Operation not permitted` |
| Network request, twice | denied; same owned loopback server returns 200 outside sandbox |
| Generated code withheld, twice | compilation fails with undefined symbols; no output binary |
| Module checksum verification, twice | pass; go.mod/go.sum unchanged |
| Export exact check before/after | pass; original archive/catalog unchanged |
| HTML vs independently retained earlier probe | faithful and enhanced hashes match exactly |

The 43-line consumer go.sum is separate from app-kit's root sum file, which lacks 12
needed lower-version checksum lines. All 41 dependency lines match templ's own go.sum
inside its cached module ZIP; templ's two sums match the cached signed checksum record.
GOSUMDB remains enabled, but network is denied: this verifies ZIPs against pinned sums,
not a live checksum-service consultation or clean remote provisioning.

An initial inventory query, `go list -m -json all`, failed on uncached unused
`golang.org/x/telemetry` metadata after compile/runtime/module verification had passed.
That red run is retained. The final inventory explicitly queries dependencies used by
the actual consumer and compiler, checks every resolved directory, and does not claim
the complete transitive module graph is downloadable offline. No network fallback or
checksum bypass was enabled to make the final proof pass.

Review also tightened the receipt: executable/fixture identities are bound before use
and checked afterward; earlier HTML oracle paths/hashes are explicit; negative compile
checks assert output absence. The final tightened harness passed a fresh two-run proof.

## Retained evidence

Private packet: `data/design-assets/pricing-delivery.ShWEQt` (115 payloads,15,319,924 bytes).
Manifest SHA-256: `7ff8282716c3e0acd77464232e42271938756275c1b8988705371eaf0f36b22a`.
Final receipt: `622639feda61610b3b486f357feff76879e674846a015d0e62d73987ad496a9b`.
The packet retains logs, both binaries/renders, delivered inputs, 17 source identities
and the failed all-graph inventory. No build ran inside the private archive store.
Its module boundary prevents root Go discovery from compiling retained derivatives.

| Repeated output | Bytes | SHA-256 |
|---|---:|---|
| Generated Go | 56,636 | `863c1372a811e1a2a79f1679109910db5d85f048e9e8688b81c77fe48e56e0e1` |
| Faithful HTML | 4,744 | `b5bd30cf8544d5fd978727e5f13a22ed2d786105602aeaec423e8d3fe1a0695d` |
| Enhanced HTML | 4,775 | `891be428144fe1b9d30f70cd0d6516b3aa36f329712b4b7d97b017d962c69f93` |
| Consumer test binary | 7,487,666 | `55cc68d48f2773d9851099b5d503a34852a56638388a0f6703d8e3becb1980ad` |

## Gates and limits

Scoped Go tests, paid CLI replay, race tests, vet, pricing CUE validation and fresh MCP
guide/tool/resource parity pass. Root intent budgets, ADR membership/status checks and
identical AGENTS/CLAUDE files pass; this is not a fresh Claude/Codex host adoption test.
`task check` passes Go vet/tests, CUE/Node checks and generation, then fails its final
Git-diff gate on existing uncommitted token/theme changes. The full gate is **not green**;
those unrelated changes were not reset or hidden to obtain a passing exit.

No CSS compiler, browser, real app task, remote install, release, Windows or native-host
test is claimed here. Earlier [browser results and failures](2026-09-14-pricing-typed-adapter.md)
remain authoritative for their exact captured conditions. Conversion counts remain
439/657 syntax/compiler/DOM, one typed family candidate and zero certified targets.
RAC/Fathom remain the active consumer pair; Conjure, CA/GGB and Windows stay deferred.
Actual app use, a second useful integration, reviewed upgrades and the rest of G1–G6 remain open.

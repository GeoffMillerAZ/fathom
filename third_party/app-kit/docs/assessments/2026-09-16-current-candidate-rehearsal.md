# Current private candidate and CSV migration rehearsal

G4 progress under [ADR-0045](../adr/0045-exact-pins-and-private-candidate-preparation.md),
not a release, whole-module approval or real-consumer upgrade. Windows remains deferred.
Private evidence root: `/Volumes/m2ext/appkit-current-candidate.TeaBqLxK/`.

## Why another rehearsal was necessary

The previous frozen candidate predates newer shell, analytics, catalog and contribution
work. Preparation of the current selected tree succeeded, but its standalone consumer
panicked: `CSV data section missing or wrong size`. The fixture still required eight
columns after [ADR-0056](../adr/0056-preserve-analytics-role-meaning.md) appended `role`
and `role_label`. Successful preparer unit tests did not execute this `.go.txt` fixture.

The fixture is now `tools/releasecandidate/rehearsal/fixture/main.go`: the exact source
is compiled/executed by ordinary root Go tests and embedded as a standalone input.
`TestSyntheticConsumerContract` reproduced the old-schema failure before migration.
The migrated reader requires all ten header names in order and exact values, including
declared `Scheduled <work>` meaning in rendered, table-only and CSV outputs. Existing
12.3, measured-zero, missingness and escaping assertions remain; the kit API is unchanged.
`TestCSVContractRefusesSchemaAndMeaningDrift` rejects eight-column, renamed/reordered
header, wrong-role, lost-meaning, missing-as-zero, missing-row and extra-row mutations.
Fixture tests are explicitly excluded from the three embedded deployment inputs.

This is an explicit consumer-fixture schema migration, not a same-oracle renderer fix.
No column was silently discarded and no library contract was relaxed to make it pass.

## Inventory review and boundaries

Initial snapshot:1,194 files/10,139,917 bytes, tree
`1144be625df2bc75426930a75fc715b0fe56b140c61eae0420e3e3dcb2334f24`.
Two independent reviews matched all selected source paths/bytes/hashes to the receipt;
the ZIP inventory/hash also matched. No selected forbidden root, `.env`/key-like
filename, symlink/special file, nested module, replacement or exclusion was found.
Directory0700 and receipt/ZIP0600 were checked. Filename checks are not secret scanning.

The root `go.mod` changes only directness of already-pinned x/net, x/mod and uniseg;
versions and `go.sum` are unchanged. No dependency addition or tool install occurred.
Receipt selection includes dirty/untracked source deliberately; it is not a clean
commit. Private MatDash/Catalyst fixtures and local-path-bearing assessments remain
private, with no redistribution-rights claim. The narrow rehearsal does not verify
unexercised Receipts/design-adapter packages, generated theme freshness or browser UX.

## Cold consumer evidence

The repaired intermediate snapshot contains1,196 files/10,150,330 bytes, tree
`0fa77b07c6cd56d9f15c49f797dcdca5e17c3c1f171bf8db99f1d6d0a1413780`.
Version is `v0.0.0-rehearsal.h` followed by the complete tree digest.
Source HEAD is `63b3a4f01168b184f91d1e701414445820e24fdc`, with selected working-tree changes.

| Check | Observed result |
|---|---|
| Archive | Preparer generates two byte-identical ZIPs from retained source; exact file proxy and Go sums |
| Cold builds | Two empty build/module caches; exact version, no replace, readonly module graph, no sibling-source or network fallback |
| Refusal controls | Both runs observe source-read and socket permission denial before building; both fleet roots denied by policy, only app-kit path directly probed |
| Synthetic task | Actual ServeMux/httptest shell/navigation, chart/table fallback and CSV assertions pass, including meaning/zero/missing distinctions |
| CSS | Pinned Tailwind4.3.0; explicit runtime-only sources, six exact selectors, token declaration and fixture-poison exclusion |
| Identity | Both binaries, generated CSS and handler-result JSON match exactly; eight retained command records per run, not eight browser tasks |
| Root checks | Releasecandidate/rehearsal/fixture and analytics package race suites pass; eight schema/data corruption subcases pass |

Observed Go1.26.3, Darwin/arm64; existing offline dependencies. The outer tool build
uses existing caches; the consumer builds use their own empty caches. No installation,
browser/native launch, real business data, consumer mutation, commit or publication.

| Retained artifact | SHA-256 |
|---|---|
| Original rehearsal executable | `59a9b9ec2461fe6366a231be7a06545adb4021a5583f8a824d2229b204bc044e` |
| Original failed receipt | `d9207059d441f7f91d10484b55e9e3f88a02b8461cc16c870816d68602b25bb4` |
| Repaired rehearsal executable | `6ccef187694f5a4e36c7c7d26a17f2207b9c2612e28b3964d81afc1ed22c2b0f` |
| Repaired intermediate ZIP | `4284b5a8aeb79f7742731af1e6aacc31689b5b66264efe840bb19c70c39dea85` |
| Embedded fixture | `d8400ba9f3cec86c2700d4f55d6e4b7e2ab2fee80a5800127e066646090126df` |
| Both consumer binaries | `87cf7b12626eb28350829ed1e4b38263abeadbadece56cab23e21145657850f7` |
| Both CSS files | `19eccd51d7bac0c91c03908c6b7c03a707cf0885122b02e62c6deaeea5063f60` |
| Both handler results | `1a19e67feec7af10012dac703fe153f8bafc302420b8b671956320467d62cee3` |
| Repaired intermediate receipt | `d235a66ae96d8cd545dea941cbed64bc4c01b3cdcb81d8d11fcce110efd904ef` |

Original `candidate/`, `rehearsal-before` and `rehearsal-before-output/` retain failed
inputs/binary/consumer. `candidate-final/` and `rehearsal-final-output/` hold the repaired
intermediate experiment; these path names do not grant release approval.

## Documentation correction and migration handoff

The broad MCP suite caught a missing D68 summary-table row after the unsigned-macOS ADR
was added. The ADR index/body existed, but guidance still advertised D1–D67. The row was
corrected and exact document/index protocol tests pass. That correction and this report
postdate the intermediate snapshot above; do not claim its docs are the latest checkout.
Subsequent artifact identities must come from their own receipts, not renamed evidence.

The [maintained upgrade checklist](../../tools/releasecandidate/README.md#migration-review-for-an-authorized-consumer-upgrade)
now includes exact graph/sums, analytics CSV8→10 (Gallery31→33, Receipts33→35), schema
refusal, runtime-only CSS regeneration, search/cursor review, opt-in v2 selected metadata,
private capture-reader bounds and CLI diagnostic/exit-policy replay. Missing historical
metadata means prospective selection, not proved equivalence. No automatic pin change.
Native packaging follows the separately selected [ADR-0068](../adr/0068-prepare-private-unsigned-macos-delivery-first.md)
plan; this synthetic consumer does not implement or certify it.

## Corrected-index exact candidate

After the table correction, preparation froze1,197 files/10,158,050 bytes as
`v0.0.0-rehearsal.hb76a301f18d4e43f256490e72b54e2e50bf8269a8374d765bca707d84c87596d`.
`candidate-reviewed/receipt.json` binds the complete inventory. Relative to the prior
snapshot, only this assessment, CHANGELOG, decision table and incident record changed;
all1,197 selected source hashes were rechecked with zero drift before the final run.
This appended result and subsequent evidence links postdate that immutable snapshot.

`rehearsal-reviewed-output/` passes both empty-cache builds and the same eight-command
task/CSS/refusal sequence with the unchanged repaired fixture and retained executable.
Its CSS/handler hashes equal the intermediate experiment; the version-bound binary is
different and matches between its own two runs. A separate `go mod graph` invocation
under the retained source/network-denying environment passes and binds only the selected
app-kit version; its raw output is `reviewed-module-graph.log`. The module declaration
graph is not a claim that every dependency is linked or required at runtime.

| Final artifact | SHA-256 |
|---|---|
| ZIP | `730a6a0740ba0bd23e6b60fe644f627e0d58098e33071713893539ee1594b035` |
| Both consumer binaries | `46448b5bfd0db689391bfaa1e41bbe062fbd72e713e040af7b14e879f39ec5ea` |
| Rehearsal receipt | `7ff8d708a52c69610938b4ad124e33cd7330efcabaaeb55342d00560a9406d46` |
| Root test log | `65acfc13e0bd9fb01699ee6556b27a9f70c75dd67faad7e126755c3642fab578` |

Fresh `go test ./... -count=1` passes all70 tested root-module packages, including
MCP/index and the now-discovered consumer fixture; nested Wails remains outside this
command. This broader source-test result does not certify private converted targets,
generated CSS for every consumer, full browser/native behavior or real adoption.

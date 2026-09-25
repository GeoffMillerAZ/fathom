# Fathom shell acceptance: bounded recovery follow-up

Synthetic G2 follow-up, **not full consumer or G2 acceptance**. The consumer remains
unchanged. [Application-shell guidance](../design/application-shell.md), Fathom I38
and its ADR-027 require truthful degraded-data navigation, not an invented account
backend. The [actual-handler fixture](fixtures/fathom-handler/README.md) is the
existing delivery path; no new server framework or production test hook is selected.

## Gap and existing proof

RAC and Fathom independently adopted shared navigation under the
[navigation delivery](2026-09-14-navigation-delivery.md) record. RAC also adopted
[settings sections](2026-09-16-settings-section-recipe.md#resume-as-code-adoption-follow-up)
while preserving sensitive form authority. Fathom's
[reader](2026-09-15-fathom-handler-read-failure.md#authorized-reader-adoption) and
[display](2026-09-16-fathom-display-adoption.md) checks cover degraded read-only pages,
not the same browser journey through backend recovery. Historical receipts prove
their named revisions; source identity must be checked before calling them current.
Gallery is the kit reference, not a second independent consumer.

G2's exit test is the composed behavior and two consumers replacing their applicable
overlap with the named interaction checks. It does not require permanent promotion of
every experimental helper or a published release. The promotion rules in
[ADR-0063](../adr/0063-incubate-account-presentation-recipe.md) and
[ADR-0064](../adr/0064-incubate-responsive-settings-sections.md) remain binding for that
separate decision; distribution belongs to G4. Do not manufacture a second account or
settings backend to satisfy a requirement G2 does not make. Conversely, experimental
status does not waive G2's task, two-consumer or recovery evidence.

## Predeclared acceptance

- Use the canonical consumer's actual landing/fleet/service/CSS handlers and selected
  app-kit mirror, with `GOWORK=off`. Overlay only an absent synthetic test, never a
  production renderer, reader, cache implementation, dependency or generated CSS.
- Feed a long synthetic service name through the registry into actual rendered labels
  and escaped route links. Fathom's seven primary navigation labels are hard-coded;
  changing them or replacing browser DOM would not prove the current consumer. Shared
  `RenderNav` input stress and consumer service-label stress must be reported separately.
- Start with an existing test-owned directory as `FATHOM_CALL_LOG`: stat succeeds,
  reading fails, and real-log fallback is not selected. Require visible unreadability
  through actual keyboard navigation, not merely HTTP200 or a model-level assertion.
- Replace only that owned fixture input with bounded, valid synthetic log data. Keep
  the same mounted handlers and browser contexts. Declare exact input bytes, expected
  counts/units and visible absent-versus-measured states after reviewing the reader;
  those source-dependent details are pinned by the separate recovery contract below.
- **Do not reset the cache during recovery.** The inspected consumer's `obsCacheTTL`
  is30 seconds (`cmd/fathom/observatory.go`). Ordinary reload/navigation must eventually
  obtain recovered data through that real policy. A reset before initial fixture
  setup is not evidence of production invalidation. Record timing and set a bounded
  wait before acquisition; a timeout remains a failure, not a reason to force refresh.
- Prefer one controlled input transition shared by four independent contexts:
 390/1440×1000, light/dark. Preserve actual Tab/Enter, native mobile Menu, skip focus,
  current-page state, reachable service/home/fleet links, visible focus and narrow
  containment before and after recovery. Do not alter labels or markup in the browser.
- A test-only control may mutate only the exact owned fixture path. Its origin,
  action, request shape, input bounds and receipt must be fixed; it is not a product
  endpoint or an end-user recovery operation. Preserve all other route refusals.
- Require exact recovered measurements and removal of the obsolete unreadable reason;
  unrelated missing configuration stays missing. Negative controls must reject false
  zero, premature recovery, stale failure and missing expected data.
- Retain executed harness/config/source identities before execution, failed receipts,
  owned process/port cleanup and before/after consumer source/pin/dirty-inventory checks.
  Do not change the existing fixture's two-scenario oracle or historical receipts.

## Prepared fixture and limits

The separate `recovery_run.mjs`, `recovery_browser.mjs` and
`actual_handler_recovery_probe_test.go.txt` execute through the [retained runner](fixtures/fathom-handler/recovery_README.md).
The original runner/browser/Go test and its two scenarios stay unchanged. The new
[contract](fixtures/fathom-handler/recovery_contract.json) pins four lanes, the long
service name, exact three-row JSONL with two agent entries, and the native30s TTL.
Fixed timestamps support all-time counts only, not freshness or rolling-window proof.
The runner retains harness/configuration and selected consumer bytes, then executes
the retained runner; compilation overlays only absent test files. Existing offline
module/compiler caches are not cold-build or full transitive-closure evidence.

Ordinary source access succeeded for this implementation after intermittent earlier
`Operation not permitted` reads. No permission or macOS-policy workaround was used.
The live Fathom index now resolves the canonical `sextant` checkout at indexed commit
`e1841718fa07af964a3d5dc5c9d6f68eb26516c5`; the historical scratch-root warning does
not describe this refresh. No G2 completion is claimed.

The scoped pass below proves only this synthetic Chrome/read-only composition.
It does not prove live-daemon recovery, authentication/mutations, physical input,
assistive technology, complete accessibility, general performance, native delivery or
all G2 requirements. A joint requirement-to-evidence review remains necessary.

## Executed attempts and retention

Private roots are `/Volumes/m2ext/appkit-evidence/appkit-fathom-recovery-<id>`; each retains eight
harness/configuration files, sixteen selected consumer source files, the exact
synthetic log, run configuration, overlay, command outputs and result receipts.

| Attempt | Result |
|---|---|
| `MSeepM` | Ten retained Node canaries pass; Go compilation fails on a fixture-only `:=` redeclaration. No browser started. |
| `K2OfGy` | Corrected fixture compiles; browser fails on an ambiguous service-label selector after reaching narrow-light fleet/detail. Both h2s are real labels, not an app defect. |
| `rdpTpb` | Requiring both labels resolves the selector error; ten retained Node canaries, missing-frozen-root refusal, race-enabled Go test and four browser lanes pass. |
| `2wUD3G` | Extends current-route/structure assertions to every native destination and repeats skip focus after recovery; all prior requirements remain and pass. |

The subagent's earlier live Node checks lacked retained pre-execution bytes and are
excluded despite their reported green result. [INC-43](../intent/incidents.md) records
the violation. The Node entrypoint now requires its saved runner root plus matching
test/browser/contract hashes; retained execution proves missing-root refusal. This
does not recover old bytes or enforce every fleet test.

## Final scoped result

`2wUD3G/run-report.json` SHA-256:
`896d2232aad036d07645d21aedcae272560f9621666a08537d6222d164d48c0e`.
Its `reports/recovery-browser/browser-report.json` SHA-256:
`cf8648cf538bc5c7c6e06ff420181b4fa62fccec8759ff9ae934a52bc78b55d9`.
Go1.26.3/darwin-arm64 with `-race` passes the named test in32.95s. Chrome152.0.7977.84
passes four390/1440×1000 light/dark lanes:82 admitted requests,20 hash-checked captures,
no blocked requests or browser failures. Immediate observations retain the old cache
at2,428–2,872ms; all lanes recover at30,152ms from that cache's timestamp. Counts are
exactly3 calls/2 agent; service detail says2/3 and identifies its local-log source;
missing configuration remains not inspectable. The second transition returns409.
The27 HTTP/control/cleanup checks pass; both owned ports refuse after close and the
browser child exits0. Selected source/HEAD/dirty inventory, original fixtures,
snapshots, configuration and absent overlay paths are unchanged afterward.

Visual review of two failed-run narrow-light and four `rdpTpb` narrow-dark/wide-light
captures finds readable long labels and recovered values without observed clipping.
The failed-state absolute-path prose is truthful but repetitive and dominates the
narrow view. Concise summaries with inspectable details remain a consumer polish
follow-up, not permission to hide missing-data evidence or modify consumer source.
These screenshot observations are not complete accessibility or contrast proof.
Two final `2wUD3G` captures (390px dark fleet and1440px light service) were separately
reviewed: full labels, exact traffic/share and unknown configuration remain readable,
with no observed P0/P1 visual defect. This does not expand the screenshot scope above.

The CUE shell-integration entry now links this evidence while retaining `partial`.
The pinned `go tool cue vet -c -d '#Inventory'` passes against schema/inventory bytes
retained before validation in `/Volumes/m2ext/appkit-evidence/appkit-recovery-metadata.e8PGwG`.
Inventory SHA-256: `35fd71b6b23fa39e60b23885770539c73d9a49ae10e968c05262f8ef571c3035`.

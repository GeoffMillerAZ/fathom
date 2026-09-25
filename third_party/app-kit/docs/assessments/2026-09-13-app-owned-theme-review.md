# Local app-owned theme review evidence

Implementation: [guide](../design/app-owned-styleguide.md),
[ADR-0031](../adr/0031-local-app-owned-theme-review.md). Experimental G1/G4 progress,
not completion of browser review, real-consumer adoption or the six project goals.

## Implemented loop

`apps/styleguide` has no Gallery or private-corpus dependency. Its pure engine retains
raw CUE, evaluates bounded known-token overrides and validates a concrete candidate.
Its local-only handler mounts app-owned callbacks with ordinary HTTP forms, isolated
static previews, diff, selected contrast checks, reset and acknowledged ZIP export.
It does not write source, preferences or persistent drafts, and does not load fonts.
Trusted callbacks/source/styles are not sandboxed Go code or untrusted uploads.

Gallery opts in with `-theme-source`; its adapter invokes the existing dashboard,
component and form renderers rather than a second specimen catalog. The live source is
`cue/tokens/geoff-synthwave/geoff-synthwave.cue`, SHA-256
`228a32cb3be5943124020fd791858571112cbc546c20810cbb8fb44eb92d8f08`.
This is the kit's adapted CUE source, not the original personal website CSS hash.

## Automated evidence

- Eight pure-engine tests cover immutable options/bytes, bounded fields, fonts/radii,
  display inheritance, malformed/unknown/nonfinite contrast, deterministic replay and
  stale review identity after changed original comments, kit, CSS strategy or checks.
- Nine external-package tests mount a synthetic app's actual `stat.RenderBand` next
  to an ordinary app route on a real HTTP listener. They cover preview/export/reset,
  independent clients, exact ZIP identities, independent21:1/1:1 contrast cases,
  strict origin/TLS/form/query boundaries and capped callback output without partial
  data, including the `io.Copy` destination fast-path canary.
- Review caught a smaller preview-URL budget discarding otherwise valid edits. The
  permanent large-role fixture proves422 retains form/diff, shows the baseline and
  clears acknowledgement. Showing source cannot acknowledge newly edited candidates.
- Gallery adapter tests prove all3 actual renderers in both schemes, opt-in linking
  and unchanged ordinary routes. CLI tests bind source reads and actual executable
  identity. Catalog/MCP checks cover experimental discovery and exact guide routing.

Final scoped race run: styleguide3.176s, Gallery20.317s, Gallery command1.681s,
catalog1.993s, MCP18.286s. Independent final styleguide race3.446s; no skipped tests.
Scoped vet, catalog concrete-CUE validation and intent link/budget/index checks pass.
This is not a claim that the complete repository generation gate was run.

## Live compiled process

Local binary SHA-256:
`0fe8e7bf58b8aa89626232fc81f0a124eecddfbd1d1895f13971505343db698f`.
Served at `http://127.0.0.1:18766/theme-lab`; source is read once at startup.
The exported kit identity names that exact local executable, not a released module.

Live HTTP journey changed `radius.md` to0.75rem, `fonts.sans` to approved system-serif
and `colors.accent.light` to#ff7edb. The preview emitted all3 candidate values.
Two exports matched byte-for-byte, contained all5 expected files and exact original
source, and all8 selected checks passed. Bundle bytes16,340; SHA-256
`e534dd2cdbdf14503772704e37e1a74f59525822e6480d0d66034efe6cf65fe0`.
Review/receipt digest:
`8623290fb619afa1449557e49a72f5253a908efb95e4c53b26983c4054d2250f`.
Changing a value after acknowledgement returned409; making text equal its background
also blocked export409. Reset cleared acknowledgement; the source file stayed exact.
The existing `/themes`, `/catalog` and private pricing comparison remained200.
The test retained exports in memory only, not in the repository or user Downloads.

## Browser follow-up and remaining limits

At this initial review, the authorized in-app browser returned `Browser is not available: iab`; a later
[isolated Chrome smoke](2026-09-13-gallery-browser-smoke.md) is limited comparison/theme-page
evidence, not the complete editing/export journey. HTTP/CSP and token-output checks
do not establish real-browser containment, geometry, keyboard/error
recovery, font fidelity or useful task performance. The lab's no-font static profile
must not be sold as exact branded typography or a working application workflow.
No consumer repository was modified, migrated or added to a proven-consumer list.
The [2026-09-14 follow-up](2026-09-14-theme-lab-browser.md) subsequently reproduced and
fixed native POST refusal, added scoped geometry/error recovery, and passed107 browser
assertions including actual export and a no-script preview. It has a new executable
identity and URL; the historical18766 process above is not the repaired implementation.
Full accessibility/font/performance and consumer gates, and G1–G6, remain open.

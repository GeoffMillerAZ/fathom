# Fathom display repair adopted — 2026-09-16

Owner-authorized narrow consumer update, not deployment or completion of G3. The
[prepared candidate](2026-09-15-fathom-display-candidate.md) is applied to canonical
`/Volumes/m2ext/code/geoffmilleraz/sextant`, preserving paused work. The shared index
still points at a scratch checkout; canonical local source is the authority here.

## Change and exact identity

`observatory_render.go` requires nonempty complete known coverage for positive
freshness, names unknown/registry failure, and renders partial stale counts as lower
bounds with coverage/reasons. Key/value content wraps; absence prose is left-aligned.
`observatory_series.go` uses intrinsic256px grade bars instead of CSS-shrinking420px
geometry. Existing text size, token roles and caller escaping remain unchanged.
New durable `observatory_display_quality_test.go` began as the reviewed kit fixture:
13 freshness cases plus long/escaped diagnostics. A subsequent reviewer-requested
geometry regression pins256px width/viewBox and grade/value/summary content. Existing
I38/ADR-027 govern; no production source changed during the test-only follow-up.

| File in `cmd/fathom/` | SHA-256 |
|---|---|
| `observatory_render.go` | `c17dfae367276856579902d1184a56f2aa53a0a533b65d52d5d272870b99290b` |
| `observatory_series.go` | `7bf0e15b0e1503f2539bb781e303c7028967d2361580eb63ac7a31ed3c78d41e` |
| final `observatory_display_quality_test.go` | `018aef13fd6bbb7d7c4cfe7466ed826c4b293ac91e739ebf9858572449e58c20` |

Originals matched the candidate's base hashes before editing and remain in consumer
`workspace/tmp/appkit-display-adoption.1Tmo5v/`, alongside executed-source snapshots.
No dependency, generated CSS/theme or stamped mirror change was required.

## Current-source proof

The maintained handler runner used **current-consumer** mode: only an absent test
harness was overlaid, not production renderers or durable regression tests. Retained
harness/browser/config bytes drive real landing, fleet, service and CSS handlers
against synthetic unavailable/pathless inputs, never operational data or user logs.

- 88 HTTP and412 browser checks pass; no failures, unexpected traffic or browser errors.
  Eight keyboard journeys at390/1440×1000 in both themes; four bounded320px resize/
  restore observations, twelve captures. Main inspected320px dark pathless and1440px
  light unavailable captures: grade text contained, diagnostics wrapped, freshness absent.
- Race-enabled `TestObservatory`, `TestGradeDistribution`, `TestRenderGrade` and both
  new display tests pass, including required fleet/service/impact tests.
- `scripts/ui-css_test.sh`:9/9; `scripts/ui-css.sh --check`: current. Source/assets,
  module files and mirror stamp hashes unchanged after checks. Warm caches, offline
  modules and `GOWORK=off`; no cold-build claim.

Handler receipt: `workspace/tmp/appkit-fathom-handler-0K7BbS/run-report.json`, SHA-256
`0179997a5ad3edf1f4cae2765ac90618c27b91d8b4be9832ed6114b9d41b91c3`.
Selected sources, dirty inventory, fixtures and retained snapshots match afterward.
Scoped test/CSS receipt: `workspace/tmp/appkit-display-adoption.1Tmo5v/checks.json`,
SHA-256 `880db9c65457ee277dfabcfb575324c7ef8ecc0a08c4adc422b90e82266c204b`.
These are bounded observations, not immutable transitive build-closure proof.

The first test/CSS receipt retains the initial fixture hash `40fb1b42…78b99`.
Geometry follow-up: `workspace/tmp/appkit-grade-geometry.WLzM3x/`; retained original
series overlay fails exactly on420px width/viewBox, then all three actual-source
display tests pass with `-race`. Executed test, arguments, environment and logs are
bound by `SHA256SUMS`, SHA-256
`e2ded1c79c92c23f3dd1e11a0239035d4751f44d9b60ada758e076e3241ec52f`.
An initial relative-cache setup refusal is retained separately, not counted as RED.

## Consumer v2 metadata migration

The separately authorized adoption migration applies
[ADR-0065](../adr/0065-pin-selected-adoption-capabilities.md) to Fathom and
Resume-as-Code. Both manifests retain their consumer, profile, layers, capability IDs,
contract, module requirement, artifact hashes and agent-router requirement. Only
`schema_version: 1` plus the whole-catalog revision is replaced by
`schema_version: 2` plus exact selected metadata pins:

| Consumer | Reviewed metadata pins | Review disposition |
|---|---|---|
| Resume-as-Code | `shell.layout` = `34006292…59b10`; `work.approval` = `53121ab5…8b71` | Current records fit `go-web`/L3b, the real responsive shell and human-confirmation queue. Routes, authorization, execution, audit and résumé policy stay app-owned. The older `7888d771…bb6ce` catalog bytes were not recovered, so this is prospective selection, not an old-record equality claim. |
| Fathom | `shell.layout` = `34006292…59b10` | Current record fits `go-web`/L3b and the real observatory shell. Routes, permitted destinations, identity, authorization, content and evidence semantics stay app-owned. Relative to retained `2a98eb25…dc43c`, the record includes the already-reviewed mobile-navigation task phrase; this is prospective acceptance, not unchanged metadata. |

The retained rebuilt checker ran `adoption pins --json` and `adoption check --json`
against both real dirty roots after migration. A normalized comparison deletes only
the version/catalog-pin fields; every other selection field is byte-identical to the
retained v1 reports.

| Consumer | v2 selection SHA-256 | Result |
|---|---|---|
| Resume-as-Code | `da7e092ba8f4e093067a3584b3808d8344606c39b4dedbec9ab01b21a22012fb` | 32 pass, one expected whole-catalog not-applicable, zero failures |
| Fathom | `0e7e7ea766054a1982ed34ac4cfc7125425f9128c1ba4df50efed72a2c589a5c` | 21 pass, one expected whole-catalog not-applicable, zero failures |

All four consumer instruction files contain one canonical block with SHA-256
`73f1e0771eb7249b555de78449e90d62f4dff38756f0c5fe8af609b20fc30880`,
matching fresh `adoption router --contract docs/app-kit.md` output. The frozen v1
checker exits1 for both migrated manifests instead of accepting v2. The rebuilt checker
is `004b35b7dde5ebb3c31540f74a35ed60dd6f7fa8fdb2356aa09969077c249aaa`;
both reports identify embedded catalog
`57da91591dca354d52c240db310e8f0cec6da035b23de09342d4a2a8a1d92fc5`.

Private evidence:
`/Volumes/m2ext/appkit-adoption-audit.8JK5D5ft/consumer-migration.y2nbGO/`.
Its `SHA256SUMS` binds the checker, retained catalog source, v1 before reports, v2
pins/check reports, old-checker refusals and migrated manifests; SHA-256
`e454b28feed19d7587e359a283b5eb9219300e0830c5d32de246e66ce1284432`.
Metadata pins remain static metadata identity, not implementation bytes, transitive
source, license, release, effective module resolution, host invocation or runtime proof.

## Contribution guidance handoff

The consumer contracts and short notes outside the generated router now point to the
[contribution loop](../design/contribution-loop.md) accepted by
[ADR-0066](../adr/0066-close-consumer-contribution-loops.md). Shared bugs return with a
sanitized failing fixture or minimal regression; repeated workarounds and reusable
lessons are proposed upstream; one-app components incubate locally. Local/deferred
dispositions keep a reason and revisit condition. A returned fix still needs a
versioned reviewed upgrade in the originating app; a second consumer is promotion
authority only where the layer contract requires it.

No consumer ADR threshold was crossed: existing consumer decisions already assign
product/domain authority locally and route app-kit delivery through `docs/app-kit.md`.
No duplicate intent/config file, skill, hook or host permission was added. The source
MCP round-trip passes `go test ./cmd/appkit-mcp -run
'^TestIntentDocumentsThroughMCP$' -count=1`, proving current document bytes and the
`contribution-loop` ID/resource mapping. Already-running servers were not restarted;
fresh installed-host retrieval remains separate and unproven.

Review added stable `RAC-ADOPT-01`, `RAC-THEME-01`, `FATHOM-ADOPT-01` and
`FATHOM-UI-01` records with ownership/disposition/revisit and separate propagation
status. Project-local host files were inspected without recording permission values;
existing unrelated hooks remain unchanged. The new guide is not assumed present in
old mirrors. No candidate is lost if canonical retrieval must await a scoped refresh.
The full MCP package passes with `-race`; an initial ADR status-format failure was
corrected to the existing hyphenated header convention, without weakening its test.
A scoped check resolves247 links, keeps affected intent/ADR budgets and paired routing
blocks valid; this is not a full intent-doctor or consumer configuration audit.
Final full MCP race-run evidence and pre-execution source/document snapshots:
`/Volumes/m2ext/appkit-contribution-verification.T3An4Z/receipt.json`, SHA-256
`6a0feaaeb9cdcb0b72bbae247138c3b55682ddea383227d7fdb56f65f47600ee`;
exit0 and all selected hashes unchanged. The current-document protocol test checks
both exact tool retrieval and resource bytes, not a running external server reload.

## Return and limits

Disposition: **returned lesson/test; app-local composition repair**. Kit fixtures
preserve original RED/candidate evidence and now actual adoption. Do not move Fathom's
registry policy into a universal kit API. Owner: app-kit maintainer; revisit when a
second analytical consumer needs the same missingness or containment cases. That
second-consumer replay is unverified.

Long raw diagnostics remain verbose. The unavailable-registry view still contains
other pre-existing zero-count displays; this freshness repair does not certify every
metric's missingness semantics. Full dashboard acceptance, populated grade variants,
actual zoom, assistive technology, performance, real data and native proof remain open.
Consumer adoption/changelog notes hand the UI change back to the paused session.
No commits, restart, deployment or global changes. The separately authorized v2
metadata migration does not change these renderer/CSS identities.

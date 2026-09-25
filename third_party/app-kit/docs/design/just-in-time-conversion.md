# Starter patterns and just-in-time conversion

[ADR-0069](../adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md) bounds
the current milestone. Complete patterns and an excellent usable core come first;
the long tail is converted when consumed or in an explicitly selected capacity batch.
This does not relax [conversion evidence](conversion-pipeline.md), source restrictions
or the [contribution loop](contribution-loop.md).
Enhanced components follow [converted-core-v1](converted-component-standard.md): typed
configuration, themes, native interaction, honest states, bounded performance, safe
composition and reproducible delivery. Implementation, verification and adoption differ.

## Three linked records, three different jobs

| Record | Owns | Does not prove |
|---|---|---|
| `appkit/catalog/catalog.cue` | Available/experimental capabilities, IDs, profiles and adoption guidance | Every source is converted, or every gate passes |
| `internal/qualityplan/inventory.cue` | Starter priorities, tasks, requirements, authored status and next work | Execution, current artifact integrity or certification |
| `internal/designreceipts` / private target-evidence catalog | Exact contracts, artifacts, observed gate claims and retained failures | Authenticated semantic acceptance merely because hashes match |

Do not add a second inventory of all licensed files. Existing archive/member IDs and
the 657-input audit identify originals. Keep a work item only for an actual selected
task or family; link its original and exact target when chosen. Unselected inventory
is on-demand, not implicitly converted. A receipt is historical until its pins match.

## Bounded starter areas

| Area | Complete task to prove | Starting selection |
|---|---|---|
| Shell/settings | Navigate, inspect identity/settings, edit, save, fail and recover | Existing shell, appearance, forms, account and settings-section recipes |
| Analytics | Filter/compare, explain, inspect rows/detail and export consistent data | Existing analytics, viz, stat, table and source/quality disclosure |
| Static content | Read, navigate, disclose FAQ/content and follow real destinations | [Bounded plain-file reading export](static-content-delivery.md); deliberate source-native or adapted sections |
| Client-heavy boundary | Edit locally without losing draft/history during outer updates; dispose cleanly | Existing client-editor integration specimen, not a new universal editor |
| Themes/style tools | Select identity and scheme, preview real states, validate and export | Existing tokens/workbench/app guide plus three new light/dark families |

These are acceptance areas, not five new frameworks. Existing useful components can
satisfy them. Full fidelity certification of every P01–P12 commercial source is not a
starter prerequisite. An intentionally changed composition names its own baseline and
differences; it never inherits a source-equivalence claim. CLI/macOS remain separately
tracked under G5/G6. Full target certification still requires its complete contract.

## On-demand workflow

1. Search first-party capabilities and exact originals through the existing MCP/Gallery.
   Select the consumer task/profile; source-native use is valid when its costs fit.
2. Reuse an existing pattern. If there is a real gap, create one readiness work item:
   owner, capability/source IDs, variant, target, critical states and bounded scope.
3. Freeze all nine gate decisions and the exact source/fixture/runtime. Applicable
   gates need a workload and oracle; N/A needs a reviewed reason, not inconvenience.
   Ground asset closure, response statuses and visible control states in the selected
   renderer/handler before acquisition; a harness review alone cannot establish them.
4. Replay a supported converter. Otherwise adapt the smallest complete pattern, retain
   the original, and distinguish a source-faithful output from an enhanced companion.
5. Run structural checks first, then task/browser/visual and workload-budget checks.
   Repair the smallest failure without changing the expected behavior or hiding it.
6. Retain exact executed source, commands, assets, receipts and failed attempts. Attach
   revision-bound evidence; update current planning status without rewriting history.
7. With scoped authority, integrate in the consuming app and remove its obsolete fork.
   Verify CSS, runtime delivery and the real task. Return the test/rule/recipe through
   the contribution record; update affected intent, ADRs and agent routing.
8. Stop when the selected task and declared budgets pass. Do not expand into unrelated
   variants, redesigns, engines or microbenchmarks to finish a small adoption.

## Spare-capacity batches

Select named IDs and profiles plus a case/token/time budget before starting. Rank by
current consumer demand, breadth of reuse, serious known defects and repair cost.
Cheap structural triage may cover many inputs; expensive browser work stays family-
specific. Preserve partial results on interruption. Prefer one generalizable fix with
several regressions over many bespoke copies. No automatic publishing or fleet upgrade.

## Read and check the readiness manifest

```sh
go tool cue vet -c -d '#Inventory' internal/qualityplan/schema.cue internal/qualityplan/inventory.cue
go tool cue export -d '#Inventory' internal/qualityplan/schema.cue internal/qualityplan/inventory.cue --out json
go test ./internal/qualityplan
```

The manifest is an authored work queue, not a test runner. `partial` means a linked
assessment proves a narrower slice, not that the entire gate passes. `not-run` remains
unknown; `failed` needs its reason and next step. A declared `passed` needs revision-
bound evidence and review; schema validity still cannot authenticate that claim.
`accepted` work requires all seven standards implemented or justified N/A and all
nine gates passed or justified N/A. Deferred non-core items never enter the core
denominator. Adding a core item is an explicit scope decision.

The five `starter-area` records aggregate pattern work; they do not pretend that an
individual commercial conversion has been selected. A `candidate` remains queued or
deferred. When selecting an actual conversion, use `kind: "conversion"` and record
its exact original/target IDs and revision hashes, a pinned `acceptance_contract`,
the workload and named budgets before entering in-progress, blocked or accepted state.
This is a declared requirement, not a runner that measures the budget automatically.

Referenced evidence/reviews must resolve to regular files inside this repository;
directories and escaping symlinks are refused. Receipt and acceptance-contract pins
are checked against actual file bytes. An `adopted` consumer assertion also requires
a reviewed pinned receipt. Source/artifact, original/target and consumer revision
hashes remain authored identities: this checker does not compare them with fields
inside a receipt or resolve the referenced artifact tree. Matching receipt bytes do
not prove a consumer ran or certify the result; review those claims against retained
proof. Never derive a certified/ready count automatically from these authored states.

The source MCP exposes this guide, `converted-component-standard`, `quality-inventory`
and `quality-schema` through `get_kit_decision` and corresponding `kit://` resources.
CUE resources are plain text. Prefer `list_quality_work` for compact discovery: its
`scope` is `core` (default), `on-demand` (recorded non-core work), or `all`; optional
`capability_id` joins an exact first-party ID. The additive `selection` object echoes
the normalized scope and optional capability ID, reports `matched`, `filtered-out`
or `untracked`, and supplies `available_scopes` and `next_action`. Available scopes
are computed before filtering, in core/on-demand order. For filtered-out work, retry
one of those scopes or `all`. For untracked work, retrieve this guide and the contribution
loop; prepare a consumer-owned `docs/app-kit.md` section or existing issue for review.
Retain the original search query/profile, task, capability ID and both returned revisions;
the lookup does not know that original query or create records. Consumer writes need
their own authority. Empty results never mean completion. Unknown capability IDs fail.
`get_quality_work` takes the returned work-item `id` and
optional `expected_revision`; a mismatch refuses a silently changed selection.
Both tools also accept optional `expected_catalog_revision`: use the whole-catalog
`revision` from capability discovery for the list, then the list's `catalog_revision`
for detail. The existing `revision`/`expected_revision` still identifies only authored
inventory. Do not substitute a per-capability adoption pin. Either identity can change
independently; malformed or stale guards refuse before records are returned. Restart
capability discovery/review on catalog mismatch, never silently remove its guard.
Empty/omitted guards retain current-snapshot lookup; unknown coverage is not a pass.
Detail includes all authored standard/gate gaps, next steps, references and adoption,
plus standard definitions and backlog policy. No readiness percentage is computed.
These read-only tools use the binary's embedded CUE snapshot without source files or
licensed corpora. They do not recheck evidence files, hashes or runtime claims. Rebuild
to refresh; raw document resources from a selected kit root may be a newer revision.
Lists are capped at32 records/64KiB JSON; detail at128KiB, per text/structured
representation. Oversize results fail rather than truncate. Existing installed servers
need a reviewed refresh to receive selection guidance; its absence in an older binary
is unknown coverage, not acceptance. Rebuild for the additive `catalog_revision` and
catalog guard; old servers may reject that argument. Inventory revisions still identify
authored content, not response shape. Source protocol tests do not prove host adoption.

The Gallery exposes the same embedded snapshot at `/quality` and `/quality/{id}`,
linked from the capability catalog. HTML and matching `/api/quality` routes accept
optional `expected_catalog_revision`; detail also accepts inventory `expected_revision`.
Capability links, native filters, detail/Back and supplied theme guards retain catalog
identity. A stale catalog returns409 and requires fresh capability review; malformed,
duplicate or invalidly encoded queries return400. Empty matches are not passes
([Gallery handoff](../assessments/2026-09-23-gallery-quality-review.md)); rebuild Gallery, not consumer pins.
Detail separates standards, gates, next steps and consumer integration status;
references remain plain authored records, not an evidence-file server. Browser
verification and its limits are recorded in the [Gallery assessment](../assessments/2026-09-16-quality-gallery.md).

## Rehearse the handoff without inventing adoption

Maintainers can run the test-only [connected contribution example](../assessments/2026-09-17-jit-contribution-rehearsal.md):

```sh
go test ./internal/qualityplan -run '^TestJITContributionPathRehearsal$' -count=1 -v
```

It joins a synthetic selected task, the existing CUE rules, seven standard decisions,
nine gate decisions, contract/receipt bytes and a local contribution record. Missing
evidence and mismatched records must fail; a returned lesson with an unverified second
consumer remains legitimate. The authored revisions, reviewer and success claims are
test data, not observed task results. No live inventory entry is accepted or modified.
This is not a new production contribution format, consumer command or host gate.
For real adoption, retain the actual task/build evidence and consumer-owned record
described above; passing this example supplies neither propagation nor certification.

## Performance and stability without speculative machinery

Use [runtime selection](runtime-selection.md) for the profile and budgets. Measure
the actual task, not framework reputation. Start with these inexpensive defaults:

- Static: build-time semantic HTML, explicit CSS scans, local/pinned assets, image
  dimensions and deliberate fonts; no JavaScript for essential navigation by default.
- Ordinary apps: server authority with local focus/disclosure/drafts; cancel stale
  requests, bound queues, recover streams, and keep useful native HTTP behavior.
- Client-heavy: one renderer per subtree, typed boundaries, lazy loading and disposal;
  worker/virtualization/Wasm only when a named measured workload needs them.
- All: preserve missing/zero/error semantics, explicit versions, failure canaries,
  clean builds and rollback. Theme swaps must not alter metric meaning or state ownership.

Meet the agreed budgets before chasing a smaller already-acceptable payload. A budget
failure is not waived; decide explicitly whether to repair, select a fitting original
or alternative, or defer that target while preserving its failed receipt.

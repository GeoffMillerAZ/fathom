# Adoption and analytics: bounded resumption

Status: reviewed RAC/Fathom candidates preserved in `8b311b8`;
[RAC's approved working-tree integration](2026-09-22-analytics-adoption-integration.md)
now has scoped synthetic/native-browser proof. Fathom stays parked; another Codex
session owns Conjure adoption. Fathom's historical ZIP failure is retained alongside
a [scoped native-export pass](2026-09-22-fathom-native-export.md) on its frozen candidate;
active integration and owner-deferred human A/B remain open. G3/G4 are not complete.

## Resume contract

- **Packet/task:** R4, Resume as Code channel yield and Fathom Observatory traffic;
  R5, Fathom's opt-in app-owned theme guide. The owner explicitly prioritized
  finishing adoption, dashboards and analytics on 2026-09-21.
- **Need/deliverable:** chart, visible data and export must agree about values,
  scope and missingness. Fathom needs a synthetic real-renderer guide using an exact
  app-kit snapshot, preserving its default appearance. Return reusable lessons,
  checks and a reviewed propagation record to the kit.
- **Inputs:** app-kit `f7ab0fb`; Fathom `c58adaa2bf865e41c52eb00df92987cc96a089b1`
  (initially clean), mirror `bcfeb8e94ce4132e1198a69f4cf10dc6764bd353`;
  RAC `15f7306`, branch `kit-governed-adoption`, 33 modified and 122 untracked paths
  at entry. RAC selected working sources require separate hashes; HEAD alone is
  not their identity. Recheck all inputs before applying anything.
- **Authority:** kit implementation/documentation and isolated consumer candidates;
  the owner's subsequent Conjure selection admits its bounded dashboard repair.
  Do not touch Fathom's active checkout; Conjure now has a separate owning session.
  RAC's scoped CSV integration was subsequently approved and applied. No
  consumer commits/pushes, daemon restart, deployment, live data/logs, global
  configuration, or unrelated edits. Preserve paused work and all prior failures.
- **Non-goals:** restarting whole-catalog conversion, native/Windows work, replacing
  app domain calculations, redesigning either brand, or declaring G3/G4 complete
  from a count of tests. G4's admitted core/JIT and guide gates remain binding.
- **Execution cap:** this slice gets two economical implementation agents and one
  integration/review thread; at most one synthetic baseline/candidate acquisition
  per app and one correction replay per demonstrated defect. Additional acquisition
  needs a decision-changing hypothesis. No broad browser matrix. Stop this slice
  at 60 further minutes or an authority/contract blocker; retain a reproducible
  handoff rather than cycling through failures.
- **Evidence:** consumer-private ignored `workspace/` candidates; portable findings
  and commands here. No real application data or paid sources in public receipts.
  Maintainer owns review, integration and contribution disposition.

## Fixed acceptance for this slice

| Area | Required result | Not sufficient |
|---|---|---|
| RAC channel yield | Existing domain projection reused by chart/table and additive aggregate CSV; legacy application CSV unchanged; exact counts/rates, cohort floor, incomplete/no-data states, metadata and formula safety tested | A raw application export alongside an aggregate chart |
| Fathom traffic | Synthetic actual handler/renderers agree across fleet, service detail and exported data; measured zero differs from unavailable input; source/snapshot qualifications survive export; recovery and method boundaries tested | A static image or zeros without validity metadata |
| Guide propagation | Exact source closure and no unintended module/default-theme drift; synthetic real Fathom pages; preview/reset/refusal/export; retained tests and reviewed adoption pins/guidance | Newer guide files copied under the old mirror stamp |
| Usability | Retain existing Gallery A/B identities; offer the owner the fixed task protocol and record actual human time/errors when available | Automated clicks reported as human improvement |
| Cost | Explicit local synthetic payload/render budgets and recorded results; browser/asset cost disclosed separately | Handler timing called page load or production performance |

Prospective Fathom traffic probe budgets, fixed before measurement: three-service
synthetic HTML body at most 64 KiB per fleet/detail page, fleet JSON at most 32 KiB;
pure fleet-plus-detail render at most 5 ms/op and 512 KiB allocated/op in a local
benchmark. These exclude CSS, network and browser rendering, which must not inherit
a pass. Record actual values and machine/toolchain; do not silently raise a failed
limit. RAC's premeasurement contract fixed 20 fictional applications, five per
canonical channel: widget HTML at most 16 KiB, CSV at most 8 KiB, combined pure
widget/CSV render at most 5 ms/op and 512 KiB allocated/op.

## Starting observations

- RAC's shared Pipeline/People channel widget already uses `ops.ChannelYield`.
  Its existing CSV is one row per application, not the corresponding aggregate.
  The isolated candidate introduces `format=csv&view=channel-yield`, not a silent
  replacement of `pipeline.csv`.
- Fathom's old exact mirror does not contain `apps/styleguide/handler.go`. A full
  reviewed snapshot upgrade is necessary; dependency graph and token changes are
  review inputs, not changes to wave through. Preserve the old app-selected token
  baseline for the guide rather than changing Fathom's normal CSS.
- G3 still requires both selected apps and human baseline/improved time/error
  evidence. G4's original propagation checklist was already proven; this slice
  advances its guide extension without retiring the remaining core/JIT gates.

## Results

| Deliverable | Evidence and boundary |
|---|---|
| [RAC review packet](fixtures/adoption-analytics-2026-09-21/rac/README.md) | Four-file patch over separately hashed dirty sources; read-only applicability check passes. Independent review caught an absent empty-state link; its canary fails before correction and passes after. All four synthetic chart/table/CSV rows and metadata agree; legacy application CSV/JSON retained. No active application or auth-runtime proof. |
| [Fathom review packet](fixtures/adoption-analytics-2026-09-21/fathom/README.md) | Exact private mirror, app-owned baseline, opt-in guide, independent-source false-zero repairs, and additive service-JSON context. Focused synthetic tests and ordinary-manifest standalone build pass; old missing-checksum build failure retained. Patch applicability passes against the unchanged active checkout. |
| Default appearance preservation | Review found that a kit upgrade would otherwise switch Fathom's generated positive token from 0.55 to 0.51. Candidate CSS generation uses the same app-owned CUE as its guide and hashes it; 11 synthetic CSS checks pass, including mutation refusal and stale-source detection. The later [real-compiler follow-up](2026-09-22-fathom-css-rebuild.md) passes build/`--check`, preserving the exact theme and explaining 720 additive application-CSS bytes. Active integration and visual equivalence remain unproven. |
| Fathom browser tasks | Actual Chrome153 passes 13 assertions through light preview, dark/narrow partial-data rendering, visible validation refusal, reset and export readiness. Native ZIP download times out: **whole task run fails**, despite HTTP export tests passing. Earlier selector/startup failures remain retained. No browser certification or gate promotion. |
| Kit verification | MCP package tests pass. Explicit strict verification of all four existing plans reports 40 claimed / 40 verified / 0 failing, out of 56 total. This is static claim verification, not proof that the consumer candidates or original G3/G4 outcomes are complete. |

Local in-process measurements on Apple M4 (CSS/network/browser excluded):

| Fixed synthetic task | Serialized payload | Render cost |
|---|---|---|
| RAC 20 applications / four channel rows, Go1.26.2 | 5,365-byte widget; 1,870-byte CSV | 22,189 ns/op; 35,496 B/op; one benchmark reading |
| Fathom three-service fleet and selected detail, Go1.26.3 | Across seven truth-state cases: fleet at most18,805 bytes; detail13,653; fleet JSON2,047 | Three readings67,377–68,510 ns/op; at most306,812 B/op for combined full fleet/detail rendering |

These meet only the declared synthetic serialization/renderer ceilings. They are
not p95 page loading, update latency, production throughput or a UX improvement.
RAC's separate live page/export reads are not snapshot-bound; source freshness
remains explicitly unverified. Missingness and authorization cannot be inferred
from numeric agreement.

## Browser failure and bounded continuation

The [browser packet](fixtures/adoption-analytics-2026-09-21/fathom/browser-results.md)
retains the three acquisitions and a narrow export diagnostic. The first failed on
a harness selector; the second reached the server before it listened. The corrected
ready-server run reached export but observed no ZIP download. A separate valid-form
probe observed no export network request in its bounded window, so a server-side
409/403 is not established as the cause. The browser-side cause is still unknown.
Do not change download policy, relax Origin/CSP, or count HTTP ZIP success as a native
download. The next investigation needs one decision-changing input/event hypothesis,
not another broad matrix or an automatic restart of the old cancellation campaign.

## Next action and contribution disposition

1. Preserve the [Conjure handoff](2026-09-21-conjure-dashboard-validity.md) for its
   separate owning session. RAC's [approved integration](2026-09-22-analytics-adoption-integration.md)
   is applied with scoped proof; no consumer commit/push. Defer Fathom's exact mirror,
   reviewed module graph, truthful displays/service JSON and shared app-owned token
   source; consumer adoption pins and intent/guidance/changelog. No deployment,
   daemon restart, consumer commit/push or unrelated dirty work by implication.
2. Reuse the frozen candidate's [native-export](2026-09-22-fathom-native-export.md)
   and [real CSS rebuild](2026-09-22-fathom-css-rebuild.md) receipts within their exact
   identities; retain earlier failures without repeating the completed diagnostics.
   When Fathom is available, review/rebase and verify the actual integrated CSS/build,
   selected browser tasks and artifact pins against exact applied sources. Candidate
   successes do not transfer automatically to an active checkout.
3. The owner answered “Later; prepare the test and leave this gate open.” The existing
   A/B binaries, CSS and templates were hash-checked against their receipt on
   2026-09-21 and match. No new browser run or human result was inferred. Start only
   when the owner is available, using the fixed participant-safe protocol.

The reusable return is now in [consumer analytics checks](../design/analytics-journey.md#consumer-adoption-checks):
all-row and empty-state journeys, narrowed-export validity, independent read sources,
immutable-vs-live snapshot distinctions and scoped cost evidence. Source patches and
reproduction instructions are retained in git, not only in ignored workspaces.
No CUE inventory gate/status is promoted; remaining core/JIT, guide memory/cancellation,
full accessibility and other original G3/G4 criteria remain open.

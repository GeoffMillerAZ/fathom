# Stabilization and demand-driven resumption

## Current priority

[ADR-0072](../adr/0072-stabilize-and-resume-on-demand.md) ends the open-ended completion
campaign. Stabilize the current work, retain evidence and resume only a selected need.
Do not automatically resume all G1–G6 goals, certify the catalog or open new consumers.
Deferred is not passed. Historical failures are preserved. The ambitious
[vision](../../VISION.md) and [acceptance outcomes](../intent/scope.md) still govern.

The [bounded handoff was verified on 2026-09-17](../assessments/2026-09-17-stabilization-handoff.md).
No campaign work remains active merely because a packet below is incomplete.

The landing boundary is a coherent handoff, current agent/MCP routing, retained last
verification and focused checks of those changes. It is not a tagged release, full
quality certification or proof that every existing working-tree change is stable.
A reproducible immediate regression in a supported path must be fixed or isolated;
optional promotion/research gaps below do not block parking the campaign.

## What remains usable, and where truth lives

| Need | Source of truth / boundary |
|---|---|
| Find existing capabilities | [UI APIs](../ui-components.md), Gallery (`task gallery:css`, `task gallery:run`), existing appkit MCP catalog; availability is per record |
| Know requirements and gaps | [CUE inventory](../../internal/qualityplan/inventory.cue), [converted-core-v1](converted-component-standard.md); authored status is not executed proof |
| Adopt or enhance one capability | [Adoption](adoption.md), [JIT conversion](just-in-time-conversion.md), [contribution loop](contribution-loop.md); preserve originals and exact pins |
| Choose static, Go/Datastar or client-heavy delivery | [Runtime selection](runtime-selection.md), [static delivery](static-content-delivery.md); no universal rewrite or mandatory WASM/NATS |
| Reuse shell or CLI behavior | [Shell acceptance](../assessments/2026-09-16-shell-acceptance-matrix.md), [terminal acceptance](../assessments/2026-09-16-terminal-acceptance-matrix.md), [CLI-kit](cli-kit.md); G2/G5 accepted only within their fixed matrices |
| Reuse themes and local previews | [Theme workbench](theme-workbench.md), [app-owned guide](app-owned-styleguide.md); experimental, not full performance/accessibility or two-consumer certification |
| Native delivery | [Web/macOS parity](web-desktop-parity.md); private unsigned experiment, not installed/signed production support; Windows remains deferred |

Earlier approximately87–88% campaign figures were planning estimates, not a measured
weighted score: no durable category weights/formula were found in the [current review](../assessments/2026-09-22-analytics-adoption-integration.md#progress-accounting).
Deferral earns no credit. Report task acceptance and blockers, not invented precision.

## Resume packets

The owner resumed adoption/dashboards/analytics on 2026-09-21. Selected **R4/R5** work
is bounded by the [resumption assessment](../assessments/2026-09-21-adoption-analytics-resumption.md);
On 2026-09-22 the owner also resumed bounded reusable UI/theme and catalog/core work
(R1/R2/R3 when a named defect or useful acceptance step warrants it), without expanding
the fixed gates or making long-tail certification mandatory. Fathom is available for
the previously authorized guide scope. The [separated rebase](../assessments/2026-09-22-fathom-guide-only.md)
is now working-tree integrated with its authorized exact mirror/minimal module delta
and display repair. Current-source race/vet/build, real CSS build/check and bounded
light/wide/dark/narrow native-browser tasks pass. Service JSON, agentic-kit and running
services stay unchanged. [Large-value/zoom observations](../assessments/2026-09-22-fathom-large-zoom.md)
now pass geometry and a native keyboard journey. A [one-cell capture diagnosis](../assessments/2026-09-22-native-zoom-paint.md)
resolves light/200 chart paint without app changes; [dark chart inspection](../assessments/2026-09-23-fathom-dark-guide-paint.md)
also succeeds, but its combined run fails native Scheme selection. The [minimal control](../assessments/2026-09-23-native-select-guide-resume.md)
also fails: park this input lane until a materially different input comparison is available,
not more key permutations. Dark-guide, old-matrix and broader quality gates stay open.
Another Codex session owns Conjure adoption after its
[bounded browser handoff](../assessments/fixtures/adoption-analytics-2026-09-21/conjure/browser-results.md).
[RAC's approved CSV integration](../assessments/2026-09-22-analytics-adoption-integration.md)
passes scoped synthetic/native-browser checks in the active worktree, without a consumer
commit. Human A/B and the remaining guide/core/JIT gates stay open; native/CLI work
remains outside this resumed priority. Jev is [research only](../assessments/2026-09-22-jev-adoption-assessment.md),
not a new prerequisite for adoption or runtime rendering.
Owner: the app-kit maintainer with the named adopter's
owner. Read only the selected row's owning specs and evidence; do not replay the whole
session or crawl every assessment.

| Packet / activation trigger | Smallest next action and completion boundary |
|---|---|
| **R1: Needed catalog/core capability.** A real app lacks a required task, or the owner selects a finite maintenance batch. | Select the CUE inventory entry and profile; follow [JIT](just-in-time-conversion.md), preserve source/runtime identity and [conversion contract](conversion-pipeline.md). Fix the task's applicable gates, verify its consumer, record return/disposition. A syntax translation is not certification; unrelated variants stay parked. Personal-site series-navigation remains a separately scoped candidate in [sizing](../assessments/2026-09-16-personal-site-component-scope.md). |
| **R2: Theme-guide browser target diagnosis.** Unknown targets prevent a newly selected acceptance run. | Use the frozen four-run [submission-boundary protocol](../assessments/2026-09-17-theme-guide-submission-boundaries.md), after input identity and cleanup checks. Its implementation/unit/race preparation is complete; browser acquisition was not run. Done for this diagnostic means retained lineage and cleanup observations, not full cancellation or permission to weaken the guard. |
| **R3: Theme-guide acceptance/performance.** A consuming app needs a supported guide or promotion is explicitly funded. | Start from [guide design](app-owned-styleguide.md), [v1 contract](../assessments/fixtures/styleguide-performance/browser-contract.md), [failed evidence](../assessments/2026-09-17-theme-guide-memory.md) and prospective [memory-v2](../assessments/fixtures/styleguide-performance/memory-v2-contract.md). V2's pure metrics subcheck is not app attribution or an acquired memory pass; next qualify one cell's lifecycle and independent retention controls under its fixed cap before any full matrix. Keep v1 failures. Resolve R2 only if it blocks the selected run; cancellation/cache/memory/accessibility need independent evidence. |
| **R4: Analytics consumer value.** RAC/Fathom changes an analytical task or requests G3 promotion. | Read [analytics task evidence](../assessments/2026-09-17-analytics-truth-state-journeys.md) and G3 in [scope](../intent/scope.md). Bind a selected real renderer to synthetic known values, chart/table/export agreement, uncertainty/recovery and payload/render/update budgets. G3 completion still needs the two selected apps and human time/error comparison; canaries alone are insufficient. |
| **R5: Fathom guide and propagation.** Fathom needs its local guide, or the owner explicitly resumes the authorized feature. | Refresh `/Volumes/m2ext/code/geoffmilleraz/sextant` intent and paused edits. Use the authorized exact mirror/minimal module/display packet with actual Observatory renderers and synthetic complete/partial/missing/error states; preview/reset/refusal/export only. Add tests, source/artifact pins, adoption/intent/changelog and a contribution disposition. Preserve the default palette, service JSON, agentic-kit pin, live logs and daemon. No consumer commits/pushes/deployment/global changes. [RAC evidence](../assessments/2026-09-15-resume-theme-contrast-and-zoom.md) is a pattern, not Fathom proof. Broader G4 requires versioned propagation and independent consumer proof; the [JIT rehearsal](../assessments/2026-09-17-jit-contribution-rehearsal.md) is synthetic only. |
| **R6: Native macOS delivery.** A named task needs the native host or the owner resumes private-bundle work. | Select exact current candidate from [candidate handoff](../assessments/2026-09-16-current-gallery-candidate.md), compare [actual-bundle evidence](../assessments/2026-09-16-native-profile-workflow.md), then select one missing failure/retry/rollback or input/media lane. G6 completion remains the [parity matrix](web-desktop-parity.md) and ADR-0021, not historical success inherited by a new binary. Installation/signing/production distribution need separate authority. |

R3 now has an opt-in [raw lifecycle instrument](../assessments/2026-09-22-theme-lifecycle-instrument.md)
and a separate [independent structural join](../assessments/2026-09-22-theme-lifecycle-join.md)
with replay/hidden-membership refusal controls. Next qualify real navigation and
retained-allocation controls, then the one-cell workload;160 synthetic tests do not
establish v2 browser/memory acceptance. Instrumentation stays opt-in; v1 failures remain.
The [first real instrument control](../assessments/2026-09-22-theme-memory-controls.md)
stopped before app work on an unadmitted Chrome background page, with clean cleanup.
The separate startup profile passes50 samples. Its admitted positive control reaches20
native cycles/structural lifecycle checks but fails process classification and forced,
incomplete cleanup. Retention is unrun; resolve those recorded hypotheses, not a retry.
Explicit context disposal/capture pass both a no-memory20-action control and a later
failed positive; component-update disabling does not fix missing arguments. The latter
also exposes an early idle wake, now repaired with114 frozen tests. No memory qualification.

R5 now has frozen-candidate [native-export](../assessments/2026-09-22-fathom-native-export.md),
[real CSS rebuild](../assessments/2026-09-22-fathom-css-rebuild.md), and
[scoped race/vet/regression](../assessments/2026-09-22-fathom-candidate-regressions.md) passes.
Reuse these receipts within their exact source identities. The newer guide-only
rebase is integrated and passes its selected CSS/browser tasks; earlier CSS source-read
failures remain retained. Start from the current packet's remaining gates, not replayed setup.
Broader guide/propagation gates still apply. Do not replay
completed diagnostics or reinterpret historical failed runs as passes.

Other future choices and their promotion/drop triggers remain in
[futures](../intent/futures.md): CA/GGB, broader Conjure adoption, Windows/mobile, personal-site family,
WASM, high-volume renderers, NATS/synchronization and framework-native packages.
Existing approvals remain scoped; they are not instructions to execute every packet.

## Resume contract

Before code, record this small header in the task's existing assessment or contribution
record. Do not create a parallel backlog, server or broad new plan merely to resume.

```text
Packet and named consumer/task (or explicit owner-funded maintenance batch):
Observed need; smallest deliverable; non-goals:
Exact kit/consumer/source/runtime inputs; dirty work and prior receipt identities:
Authority and excluded operations:
Acceptance and applicable standards/gates; expected benefit or failed budget:
Token/time/tool-cost cap; maximum acquisitions and stop/escalation rule:
Next action, owner, evidence destination and contribution disposition:
```

1. Recheck source and receipt availability before reuse. Missing/stale evidence is
   unknown, not a fresh pass. Read the selected consumer's intent before its changes.
2. Use one bounded task. Delegate small searches/edits/reviews to economical models;
   reserve expensive reasoning for unresolved architecture or consequential review.
   Summarize by paths/hashes/results; do not repeatedly ingest full session histories.
3. Freeze acceptance and the acquisition limit before running. A repeated failure
   requires a new decision-changing hypothesis, not another identical costly matrix.
   At the cap, retain the result and stop; expanding scope/cost needs an explicit choice.
4. Verify the narrow change, preserve negative controls and independently verify each
   claimed consumer/target. Update CUE status only to the level the evidence supports.
5. Return the smallest reusable fix/test/lesson, update affected intent/ADRs and
   matching CLAUDE.md/AGENTS.md routes, then close the selected task. Do not chain into
   another packet automatically. No global configuration or fleet rollout by implication.

## Durability and retrieval

The owner separately authorized source cleanup and direct pushes to main on 2026-09-18,
including preservation of valuable branch/worktree changes; no PR or release is implied.
Source commits are not release certification. Preserve any overlapping dirty work and
never stage indiscriminately. Before sharing a pin, review the exact source closure, include required
untracked files, check private-source exclusions and reproduce a standalone consumer
build under [ADR-0045](../adr/0045-exact-pins-and-private-candidate-preparation.md).

Dated assessments retain exact commands, private receipt locations, hashes and limits.
External task packets are grouped under `/Volumes/m2ext/appkit-evidence/`; preserve
their contents and update the referenced paths when reorganizing them.
External-volume and temporary evidence may disappear; check availability, and retain
the selected packet outside temporary storage before deleting anything. An unavailable
receipt leaves the historical observation documented but cannot support fresh approval.

The 2026-09-24 landing audit is recorded in
[`2026-09-24-landing-audit.md`](../assessments/2026-09-24-landing-audit.md): main and
origin are aligned; the stat/font/radius delivery branch's behavior is already in main,
while its remaining palette delta would undo a later contrast correction; the older
component archive branch is intentionally retained as historical source, not merged
over the newer main tree. Only main is checked out. The old generated-artifact stash
is preserved.

MCP `get_kit_decision("stabilization")` serves this same document. The intent router and
both agent files route here. Fathom's ordinary index/thread continuity are helpful;
they do not replace source, tests or consumer evidence. Per [maintenance](fathom-maintenance.md),
no new semantic layer or LLM compilation is needed now. Reconsider only after a bounded
retrieval evaluation demonstrates missing value; keep mutable status in CUE and scope.

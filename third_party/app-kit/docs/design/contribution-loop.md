# Make each app improve the kit

[ADR-0066](../adr/0066-close-consumer-contribution-loops.md) makes the return step in
[adoption](adoption.md) explicit. This is a reviewed work practice, not installed
automation, permission to modify another repository, or a component-count target.
MCP: `get_kit_decision("contribution-loop")`; resource: `kit://contribution-loop`.

## When to assess reuse

Assess at discovery, before duplicating a workaround, and before handing off a material
UI/CLI change. Do not wait until the whole app is finished.
For converted components, check [converted-core-v1](converted-component-standard.md)
and the [JIT inventory/workflow](just-in-time-conversion.md). Return the standard/version,
exact implementation revision, independent gate evidence and consumer delivery status;
do not collapse implemented, verified and adopted into one checkbox.

| Signal | Minimum useful return |
|---|---|
| Shared defect, inaccessible interaction, misleading metric or performance regression | Sanitized failing fixture, expected behavior, affected kit revision and narrow fix proposal now; no second victim required. |
| Same workaround/divergence in two apps | One kit issue with both call sites and constraints; reopen the boundary under ADR-0017, not another silent fork. |
| Useful one-app design or interaction | Reusable lesson/specimen or explicitly experimental recipe; not a permanent shared API. |
| Missing/incompatible catalog or MCP guidance | Exact query, profile, returned ID/revision and incompatible recommendation or missing task. |
| Better testing, CSS-build, theme or delivery technique | Small deterministic fixture/check with failure proof and adoption instructions. |
| App-specific workflow, metrics, permissions or branding | Keep domain ownership local; document divergence only from an applicable kit contract. |

No contribution quota. A review finding nothing reusable records “no candidate” with
its scope; it does not invent a component. Strip real user data, secrets, identifying
logs and host paths from reusable fixtures. Paid references stay private and provenance-
bound; do not upload evidence by default or copy it into unrelated public packages.

## Choose the smallest owning layer

Search current first-party capabilities and source before adding one. Choose a hazard,
test, role token, portable artifact, pure renderer, composition or terminal helper under
ADR-0017/0018. Keep app routing, authentication, data semantics and identity local.
A composition error does not automatically justify a shared renderer change.

Existing promotion bars remain: lessons can return after one occurrence; L0–L3a use
their layer-specific tests/specimens; permanent L3b/L4 components need two consumers,
a Gallery page, an upgrade note and an accessibility contract. Existing shared bugs
can be repaired without waiting for another victim. Experimental incubation names
what is unproven and what earns promotion.

## One small contribution record

Use a section in the consumer's existing `docs/app-kit.md` or already-owned issue;
link larger assessments/fixtures instead of duplicating them. Give each candidate a
stable local ID and these fields:

```text
ID / originating app and user task:
Observed / expected result / affected kit and app revisions:
Evidence: synthetic reproduction, provenance, commands and retained results
Ownership: kit layer or app-specific; other actual consumer if known
Disposition: returned | incubating | local-divergence | deferred | no-candidate
Next action and owner; revisit condition for unresolved candidates:
Delivery: upstream change/revision, original fork removed or reason retained
Verification: originating-app result; second-consumer result or unverified
```

“Returned” means the kit has the linked fix, test or lesson; it proves neither
originating-app integration nor second-app propagation. Accepted proposals are not
implementation; static manifests are not runtime
proof. Record pending delivery separately and preserve failed receipts. Git keeps
history; close/prune resolved work rather than accumulating duplicate ledgers.

## Close the loop

Select one task under [stabilization and resumption](stabilization-and-resumption.md)
with acceptance and a stopping budget; returning a lesson does not reopen the entire
kit backlog or authorize an automatic second-consumer migration.

1. Reproduce the task/state with synthetic data; record the failure or benefit.
2. Classify ownership and check authority. Prepare a patch/handoff when destination
   writes are not authorized; do not lose the lesson or silently change another repo.
3. Fix the narrow shared seam, or return the reusable test/guidance. Preserve invariants,
   measured baselines and original licensed references.
4. Add applicable unit, browser/terminal, keyboard, narrow/theme, failure/recovery and
   performance evidence bound to the tested revision. Never weaken the oracle to pass.
5. Update affected kit API/catalog/design docs. Record an ADR for a durable boundary or
   tradeoff, not each bug. Prepare an exact reviewed upgrade and migration note.
6. Adopt in the originating app, remove the superseded fork, and rerun its task plus
   CSS/delivery checks. Do not hand-edit vendored mirrors or silently update pins.
7. With authority, verify a second applicable app. Until then, propagation is unverified.
   No automatic fleet upgrade; preserve each app's identity and constraints.

## Keep consumer instructions alive

At adoption/upgrade and handoff, review the contract, selection manifest, intent router,
applicable invariants/ADRs, `PRODUCT.md`, build/check commands and `CLAUDE.md`/`AGENTS.md`.
Update only changed truths. Keep the generated app-kit block identical in both agent
files; route extra local guidance through the contract. Do not fork the policy into
skills, hooks and MCP. New decisions belong in the owning repo, not competing copies.

Review project-local Claude/Codex registration before altering it. Do not widen
permissions, install hooks or change global settings as incidental hygiene. Unnecessary
config changes add no adoption value. Name missing registration; prove fresh-host
retrieval separately when in scope. Instructions guide until an invoked check has
demonstrated refusal; existence of a configuration file proves neither.

For UI work require a real task, deliberate visual reference, applicable states and
measured budgets. Excellent visuals and correct/useful interaction are separate review
questions; compilation proves neither. Analytics retain units, periods, coverage,
freshness and chart/table/export agreement. Use an app-owned style guide where one
exists; never imply that a planned guide is already available.

## Worked example

The [synthetic JIT rehearsal](../assessments/2026-09-17-jit-contribution-rehearsal.md)
connects the readiness, receipt and contribution fields with deterministic refusal
tests. Its local-only `returned` record does not claim second-app propagation or API
promotion. It is a test fixture, not a replacement for this consumer-owned record or
evidence that an actual app executed the task.

RAC's `RAC-NAV-01` records a complete scoped return-and-propagation loop: small-target
and active-text failures became shared kit `63b3a4f`, then reached RAC and Fathom with
independent browser/build evidence. The [reconciliation](../assessments/2026-09-16-propagation-reconciliation.md)
keeps that fact separate from current manifest drift, release pins and full-app quality.
Use this record's ownership, disposition and revision-bound verification fields; do not
copy another app's routes, identity or permissions.

RAC's `RAC-ADOPT-04` returns aggregate-selector refusal, missing-sent evidence and
all-row/native-export checks while keeping `ops.ChannelYield` and routes app-owned.
The [integration receipt](../assessments/2026-09-22-analytics-adoption-integration.md)
is pinned by `core.analytics` in the readiness inventory as `verified-scoped`.
Its artifact identity is the executed synthetic test binary, not a shipped app;
source identities, closure limits, task results and contribution disposition remain
in the linked receipt and RAC's contract. This real scoped return does not accept the starter area,
claim an `analytics.RenderBars` conversion, or complete second-app propagation.

Fathom's freshness/diagnostic/grade-layout repair belongs to its model composition,
not the stamped kit mirror. Its shared return is the
[synthetic regression and actual-handler browser fixture](../assessments/fixtures/fathom-handler/README.md)
and coverage-before-conclusion lesson. The
[adoption receipt](../assessments/2026-09-16-fathom-display-adoption.md) distinguishes
originating-app proof from unperformed second-consumer replay. This contributes value
without inventing a universal freshness API.

Fathom's `FATHOM-GUIDE-01` returns the source-bound local-guide integration to
`recipe.styleguide.local` and `core.themes-style-guide`, including the exact tested
binary and [scoped integration receipt](../assessments/2026-09-22-fathom-guide-only.md).
Consumer history is not promotion: the recipe stays experimental, integration
partial and performance failed. This catalog metadata correction changes its
ADR-0065 capability pin; consumer review and installed-MCP delivery are separate,
not silently applied. [Return regression](../assessments/2026-09-22-bar-resolution-and-guide-discovery.md).

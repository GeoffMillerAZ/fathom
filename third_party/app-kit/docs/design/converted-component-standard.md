# Converted component standard: converted-core-v1

This standard applies to deliberately enhanced kit components and recipes, not to
immutable imported originals. [ADR-0069](../adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md)
sets starter priority; [the JIT workflow](just-in-time-conversion.md) governs remaining
work. Source-faithful conversions stay separately identified from enhanced variants.

## Standards catalog

| ID | Requirement | Evidence and owning gates |
|---|---|---|
| `typed-config` | Profile-appropriate typed inputs; validated IDs/URLs/enums; useful defaults; explicit optional features and app-owned content slots. Invalid input fails before partial output. | Constructor/schema refusals, zero/empty/boundary cases and public example; structure, security, integration |
| `semantic-theme` | Semantic color/state roles and declared font/radius bindings; identity separate from scheme. No invented metric meaning from color. Declare supported schemes and effects; do not silently recolor source references. | Real forms/navigation/charts under selected schemes; text/control/focus contrast, print/forced-color/reduced-motion checks where relevant; visual, data, interaction |
| `native-interaction` | Prefer real links, forms, buttons, details and dialog; essential keyboard/pointer paths, visible focus and narrow/zoom layouts. One DOM owner, explicit mount/update/dispose, stable identity and preserved local input. | Actual tasks, long labels, repeated instances, focus return and lifecycle/cancellation; structure, interaction, lifecycle |
| `honest-states` | Declare applicable empty/loading/saving/success/error/permission/stale/partial/conflict states. Preserve drafts and useful recovery. Zero is not missing; demo actions never imply a real backend. | Synthetic failures and recovery, fixed expected values, chart/table/detail/export agreement; data, interaction, integration |
| `bounded-performance` | Named workload and budgets for complete assets, first useful view, input/update, memory and cancellation. Avoid mandatory hydration, fonts, stores or streams without a task need. | Cold/warm and degraded-network measurements with device/toolchain identity; performance, lifecycle |
| `safe-composition` | Escape untrusted content; review trusted slots; keep routes/auth/data policy app-owned. Explicit CSS/font/runtime dependency closure, no secrets or paid-source leakage. | Malicious inputs, CSP/origin/CSRF cases when applicable, multiple-instance tests and provenance review; security, structure, integration |
| `reproducible-delivery` | Versioned input/config/rules/patches/output, repeatable generation without an LLM, exact replay and a migration note. Consumer builds must not silently depend on a sibling checkout. | Hash/drift failure canaries, standalone build, unchanged oracle replay and authorized consumer upgrade; reproducibility, integration |

Configuration should remove repeated implementation work, not make every CSS property
a new public option. Prefer semantic variants and composition over boolean combinations.
Do not add a general form engine, cross-framework intermediate representation, theme
DSL, chart engine or plugin system for hypothetical consumers. Public configuration
and defaults are compatibility surfaces even while a recipe is experimental.

## Track four independent facts

1. **Implementation:** which standard/version the selected component implements, partly
   implements or has not addressed, with the precise remaining gap.
2. **Verification:** which applicable gates were exercised for which artifact/profile,
   using which fixture and exact retained evidence. A partial test is not a full pass.
3. **Adoption:** which consumer selected, integrated and verified that revision. A
   selection manifest alone does not establish runtime use or successful delivery.
4. **Promotion:** whether layer-specific reuse/consumer requirements justify a permanent
   shared API. One successful app may justify an experimental recipe, not a fleet promise.

`internal/qualityplan/inventory.cue` is the authored inventory against these standards.
MCP `list_quality_work` / `get_quality_work` expose compact discovery and one complete
work item with its gaps and next steps; they report the embedded plan, not live proof.
Gallery `/quality` presents that same snapshot with native filters and exact-item
detail; [the UI assessment](../assessments/2026-09-16-quality-gallery.md) keeps its
interaction evidence separate from the standards' authored status.
The nine existing gate names remain canonical in `internal/designreceipts/schema.cue`;
the planning tests check agreement, current capability/profile fit, repository-contained
file references and declared receipt/contract byte hashes. Those checks cannot
authenticate semantic claims or confer certification. All seven standards and nine
gates must be completed or reviewed N/A before a work item declares acceptance.
Keep both the current gap and older failed evidence visible. The
[implementation assessment](../assessments/2026-09-16-quality-plan.md) records current
coverage and negative/positive admission checks, not completed core patterns.

## Priority and realistic effort

The core uplift is incremental: shared tokens, explicit typed adapters, native forms,
private replay and many failure tests already exist. Start by applying this checklist
to the five starter patterns, reusing exact evidence where its subject and scope match.
Repair identified gaps, not every implementation merely because it is old.

- **Small:** semantic token fixes, clearer errors, missing IDs/labels, exposing an
  already-app-owned value, bounded overflow and a missing deterministic regression.
- **Medium:** extracting repeated composition, adding all meaningful recovery states,
  clean consumer delivery, multi-instance/lifecycle proof or a complete theme matrix.
- **Large / separate decision:** replacing a client renderer, offline reconciliation,
  new Wasm/NATS infrastructure, generalized virtualization or full native-platform support.

These are relative sizing judgments, not measured schedules. The converted core is a
moderate hardening program; the whole catalog would be large. Standards adoption does
not guarantee all unused variants eventually get work. Consumption drives demand;
owner-selected batches advance the rest without making them current blockers.

## Reuse and upgrade practice

New or materially modified converted components evaluate every standard. N/A requires
a specific reviewed reason. An adopter discovers the current gap, selects a bounded
repair, verifies its task, and returns a fixture/adapter/recipe through the contribution
loop. Only authorized writes update another app or its pin. Keep app-owned policies
out of the kit, and never edit archived originals to make a target test pass.

Performance improvements must preserve usefulness and stability: precompute static
work, scan only selected CSS sources, load optional assets lazily, keep immediate view
state local, cancel obsolete requests, and bound dataset/stream work. Compare a worker
before adding Wasm, and ordinary calls/transactions before adding a broker. Prefer the
simpler solution that meets the measured budget; no technology earns a pass by name.

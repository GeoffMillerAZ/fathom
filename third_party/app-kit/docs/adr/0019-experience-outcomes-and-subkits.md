# ADR-0019: Organize the kit around proven experience outcomes

- **Status:** Accepted
- **Type:** strategy
- **Date:** 2026-09-13

## Context

The owner asks for exceptional, reusable UI/UX across dashboards, common app surfaces,
CLIs and desktop. The [assessment](../assessments/2026-09-13-app-kit.md) finds actual
kit imports but repeated shell behavior, consumer-owned chart checks, and specimen
pages that do not demonstrate complete tasks. More components alone cannot close this.

## Decision

The product promise is cumulative improvement in application experience and delivery.
The reusable unit includes the decision, typed contract, working implementation,
behavioral states, proof and adoption path. Organize capabilities into foundation,
application-shell, analytics, interaction, CLI and optional host families within the
existing module. Do not create separate modules or servers merely to name a sub-kit.

Complete recipes compose existing primitives before creating permanent abstractions.
The first reference workflows are shell/navigation/appearance in Car Assistant and
Growing Gardens BI, analytics in GGB and Resume-as-Code with Throughline temporal
drill-down, and Charm interaction informed by Bosun/thumperctl.

Preserve ADR-0017's layer-specific promotion bar and ADR-0018's terminal contract.
The owner can admit a bounded experiment with one named need, a fixture, a task
comparison and a kill condition. This does not waive the two-consumer bar for
permanent opinionated components. Promote at the lowest useful layer.

A consumer chooses its own identity, constraints and selected capabilities. Stopping
at tokens/CSS is complete adoption. Reuse must not force every product into the same
visual composition, framework or runtime.

## Consequences

[Experience-system](../design/experience-system.md) defines craft, analytic semantics
and the recipe contract. [Scope](../intent/scope.md) owns exit tests. Shipping a recipe
requires evidence of task completion, meaningful states and a real consumer outcome.
Measure task success/error/time, integration effort, performance and improvements
propagated into a second app; import counts and catalog size are supporting inventory.

This is accepted direction, not a claim that new recipes or gates ship in this change.
The Gallery becomes the first place to inspect behavior; representative apps prove fit.

## Alternatives rejected

- More component packs alone: retrieval already works; missing behavior and propagation
  remain. Additional packs win only for a named unmet capability.
- One mandatory application framework: fails existing constrained and non-Go consumers.
- Bespoke design per app without return contribution: repeats verified failures.

## Revisit when

- Two consumers reject a recipe for the same central reason.
- A bounded experiment shows no task benefit or costs more to integrate than local code.
- A sub-kit develops a measured independent release/dependency boundary.

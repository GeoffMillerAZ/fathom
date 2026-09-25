# ADR-0023: Earn cross-target compatibility through a tested conversion pipeline

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-13
- **Partly supersedes:** ADR-0017's server-only L4/client-compute rules, Go-consumer
  toolchain ban and restriction of consumer-authored browser code to disposable polish
- **Extends:** ADR-0019, ADR-0020, ADR-0021 and ADR-0022

## Context

The owner wants reusable Go/templ/Datastar conversions, deterministic tests and agent repairs.
Static brochure sites and heavy client applications must benefit too. ADR-0017's
server-only clauses would forbid valid implementations of that request. The
[evaluation](../assessments/2026-09-13-conversion-strategy.md) distinguishes syntax
conversion from behavioral equivalence and potential benefits from measured results.

## Decision

Build a private, profile-specific conversion and verification pipeline, beginning with
the bounded pilot in [conversion-pipeline](../design/conversion-pipeline.md). Aim for
broad coverage by classifying the full source inventory, then converting supported
families in batches. Do not spend the whole investment porting every variant before
one complete workflow proves value. Unknown and unsupported remain visible outcomes.

Prefer official HTML inputs for Go/templ/static targets. Use narrow syntax transforms,
explicit typed data seams and reviewed interaction mappings; no universal UI IR or
arbitrary React-to-Go compiler. Original source remains immutable. An LLM may propose
a rule or source-specific patch; normal builds replay reviewed deterministic inputs
without an LLM. A repair may not weaken assertions or replace baselines to obtain green.

Separate source-faithful reference conversion from kit semantic-token adaptation.
Preserve structure, typography, density, color/state distinctions and behavior; record
intentional changes. Paid derivatives remain private. Compatibility is per source,
adapter, runtime pin and target, not a universal badge or inferred from compilation.
Originals and adaptations remain first-class linked choices. Source-native fallback
states its dependencies and target fit; no silent renderer switch or overwritten source.

Support three deliberate web strategies under [runtime-selection](../design/runtime-selection.md):
static HTML/CSS generated with Go/templ; Go/templ with Datastar when dynamic behavior
needs it; and client-owned applications/islands for measured local interaction or
offline needs. Browser DOM subtrees have one owner. Durable state has one declared
authority and reconciliation policy. Do not force server round-trips for local input.

L4 templ can render at build time or request time; dependencies determine suitability.
Client filtering/geometry is allowed behind an explicit contract and performance proof.
ADR-0017's layer boundaries, portable artifacts and layer-specific promotion rules stand.
Go/CUE build hygiene binds kit-core/portable artifacts, not every Go consumer repository.
A declared client profile may use a pinned client toolchain and correctness-bearing
local code, including in a Go-hosted app. No such toolchain becomes a kit-core dependency.

NATS remains optional for actual event topology; it is not a browser rendering layer.
Browser Wasm is an optional compute island, not an assumed DOM framework. wasm3 is not
the browser Wasm engine or a mandatory dependency. Select a host runtime only for a
named host need and measured pin. Existing WASM/mobile/distributed futures remain gated.

## Consequences

The catalog needs source lineage, target-specific proof and explicit unsupported cases;
Gallery and MCP must expose the same receipts when implemented. Automated certification
and conversion coverage are not shipped by this decision. Existing catalog/tool IDs
remain unchanged; schema changes require migration tests rather than inferred support.

We accept maintaining adapters, fixtures, private reference environments and vendor
update checks. Reuse must repay that work in integration effort and preserved quality.
No runtime migration, dependency install, global configuration or publication is implied.

## Alternatives rejected

- Rewrite every interaction in Datastar: duplicates mature behavior without a benefit.
  Wins for a family only when its replacement passes stronger task/security/cost tests.
- Adopt Wasm/NATS everywhere: added costs need a named task and measured benefit.

## Revisit when

- A pilot family cannot retain essential behavior or visual quality at its budget.
- Two comparable consumer integrations show no effort reduction after setup costs.
- A vendor update cannot replay a reviewed patch or invalidates a target proof.
- A named offline/interaction workload defeats the selected profile's latency/state model.
- A new target requires a contract the current narrow adapters cannot express cleanly.

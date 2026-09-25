# ADR-0073: Separate retained-memory metrics from app attribution

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-22
- **Extends:** ADR-0072; historical v1 contracts and results are unchanged

## Context

The owner selected bounded app-owned growth with total browser cost reported.
[Failed v1 measurements](../assessments/2026-09-17-theme-guide-memory.md) show that
even blank-browser controls exceed the absolute renderer RSS budget. Subtracting a
blank baseline would not establish app allocation. CDP heap usage is isolate-wide;
native navigation can replace preview targets, and detached isolates can become
inaccessible. Current acquisition does not prove complete frame/isolate ownership.

## Decision

Define a prospective [memory-v2 contract](../assessments/fixtures/styleguide-performance/memory-v2-contract.md).
Keep metric calculation, attribution, task correctness and full acceptance separate.
The first implementation is a pure metrics subcheck, not a new browser acquisition
or a replacement pass for an old receipt. It always reports attribution unverified
and memory/full-contract completion false.

After the fixed warm task and twenty native task cycles, compare matched settled
endpoints. Deduplicate observed isolates and bound growth independently at4MiB for
used JS heap, embedder GC heap and backing storage. Do not sum these domains: their
disjointness is not established. Record per-session DOM counters without adding
them or calling them frame-local allocations. Missing/invalid data is unqualified.

Report all owned Chrome process RSS, its renderer/non-renderer split and server RSS
separately, including endpoint changes and sampled peaks. No blank subtraction,
silent process exclusion, unique-physical-memory claim or v2 RSS acceptance ceiling.
Keep the100ms sampling target and200ms coverage limit. Total RSS remains visible
evidence requiring investigation when growth cannot be explained; it is not app cost.

The live-isolate metric does not exclude detached retention or native allocations.
A later acquisition must qualify frame/context/loader/session/isolate lifecycle and
independent controls before any app-attributed or full memory claim. It must retain
unknown target failures and cannot infer target destruction from session detachment.

## Consequences and alternatives

- Larger v1 RSS ceiling or blank subtraction: rejected; neither fixes attribution.
- Requiring identical preview IDs across navigation: rejected; legitimate replacement
  needs lifecycle evidence, not identity equality between different task endpoints.
- Shipping only a heap budget as full memory acceptance: rejected; leaves detached
  and non-heap retention unproven. Expose the missing evidence explicitly.
- No runtime API, consumer pin, guard policy or default appearance changes. Existing
  v1 tools remain usable; their failures and full G3/G4 criteria remain unchanged.

## Revisit when

- A frozen acquisition proves complete lifecycle attribution and independent controls.
- A protocol/browser change alters heap fields, their scope or process accounting.
- A reproducible named workload crosses a prospective limit or reveals unmeasured
  retention; review the method before executing a separately versioned replacement.

# Dependency edges for standalone readers

Status: accepted 2026-11-18, owner-architect ruling (ADR-114).
[Contract](../contracts/dependency-edges.md) · [Plan](../plans/dependency-edges.md).

A cache that omits an input may reuse a stale result. The CI kit is the first named
consumer. A standalone reader supplies a set of direct edges and a completeness verdict
for each file, including the reason and possible evidence source for every unknown.

## Intent check

- ADR-011 and I19 govern public text and fixtures. ADR-066 and I56 keep reads pure Go,
  offline, and free of toolchain execution. The test harness alone runs pinned oracles.
- ADR-063 O1 keeps package closure and hashing in the CI kit's dependency graph. This
  door reads each language's file-level dependency constructs. ADR-090 still governs
  observed reads. The named consumer fires TQ-38's trigger. DEP-8 owns the deferred audit.
- ADR-092 and I69 keep the standalone boundary and the `pkg/lang` closure at 80
  packages. The dependency door is a sibling feature over existing read evidence.
  ADR-112's builder direction supplies the precedent for a consumer conformance helper.
- The named consumer satisfies F99 for this door. Vision 46 §7 admits it: computable
  from bytes and a repository view, returns a set and verdict, rides the existing
  library and binary, and admits against independent tooling. No default tool is added.

## Sequence

1. Record the accepted boundary and preregistration (DEP-0).
2. Build the door, the Go control, and the binary and export (DEP-1 through DEP-3).
   Review the public API and serialized shape independently and obtain architect
   acceptance before integration.
3. Read TypeScript and JavaScript against the compiler's file list and resolution
   trace (DEP-M1), Python against per-entry standard-library module finding (DEP-M2),
   and Dockerfile against a pinned BuildKit parser (DEP-M3). If oracle rules
   preclude BuildKit, use a labelled sample and keep Dockerfile detect-only.
4. Add resolver, pack and declaration conformance with a runnable consumer example
   (DEP-7).
5. After the public door lands, wire the import boundary rules into the existing
   family import gate (BND-1 through BND-3). After DEP-7, two passing B1/B2 readings
   and architect acceptance permit moving the closure engine (DEP-P1a). Computed
   path filters follow that move (PF-1 through PF-3). The consumer switch, the
   under-keying audit and dialect breadth keep their plan triggers.

## Preregistered admission

Freeze public repositories, exact commits, deterministic file selection and oracle
versions before the first run. Use at least three repositories and 300 files per
dialect (30 files for Dockerfile). A smaller or ungraded population is inconclusive.
Run Go's `go list -deps -json` as the control. Test-only code may compute reachability.

| Bar | Rule |
|---|---|
| B1 | Zero oracle in-repository direct dependencies missing from files marked complete. |
| B2 | Zero wrong file targets among files marked complete. Count extra edges separately. |
| B3 | Byte-identical output on repeated runs and in a second checkout path. |
| B4 | Report complete-file share, unknown-edge share by code and extra-edge rate. Below 0.80 complete is “sound, low coverage” when the soundness bars pass. |
| B5 | A B1 or B2 failure leaves that dialect detect-only. |

No threshold changes after a run. Record per-repository and pooled counts, the
oracle's coverage gaps and the evidence that limits the verdict. A fresh reviewer
reruns each reading from a clean clone. Each dialect gets at most two correction
rounds before publishing the result at its measured ceiling.

## Library first uses

ADR-115 allows a same-change first use and dated keep-or-cut reading to satisfy
Vision 46 §7 clause 5 for library members. Boundary rules consume direct edges
and return violated, clean or unverified. The gate's first use covers this
repository's family boundaries and compares seeded violations with the Go import
graph. BND-M1 counts actual evaluations, pre-merge findings and outside use 30 days
after landing. It keeps the capability when a violation is caught more clearly
than by the older tests or an outside use is observed.

The engine move preserves the existing generic implementation. Its family
closure test enforces independent imports for the dependency graph and
understanding packages. Data crosses through the declared-edge export.
The separate consumer-switch phase owns changing the other repository.

Path filters read the workflow event patterns and join them with a declared map
from workflows or jobs to entry paths. They return missed, unverified or covered
and name unused patterns. The first use is a fixture with seeded misses and a
clean control. The CI kit's consumer switch supplies the first real wiring and
starts the 30-day PF-M1 window.

## Decision models

None applies. Dependency constructs, resolution, completeness and admission are
computed from repository inputs and compared with independent evidence. Provider
`none` covers the entire feature. Consumers opt in by invoking the door.

# 0011: Select workload solutions from scoped, versioned evidence

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-16

## Context

The owner asks for defensible model, retrieval and agent-architecture choices.
The kit already has a caller-owned model catalog, ladder, evaluator, journal,
cost accounting, retrieval metrics and statistics. They do not yet bind complete
solution configurations to comparable outcome measurements. See the
[assessment](../../assessments/2026-09-16-selection-evidence.md).

## Decision

- The unit of evaluation is a workload solution: models by role, prompts, context,
  retrieval, tools, workflow and operating policy. There is no universal model order.
- Keep provider claims, organization eligibility, measured results and recommendations
  separate. Catalog presence, a family label or a successful credential probe grants
  neither runtime access nor statistical independence.
- Hard authorization, safety and correctness constraints precede soft optimization.
  Unknown required evidence prevents selection. A rule trace, not generated prose,
  explains a policy decision; explicit human overrides remain visible.
- Extend `kits/eval-runner` experimentally with bounded manifest/trial accounting and
  exact paired comparisons. Reuse `record-fingerprint` and `reliability-stats`; retain
  `eval-journal` for judge calls and consumer-owned persistence. No second evaluator,
  provenance store, router, organization registry or mandatory platform.
- Legacy replay/regression remains an implementation check. Live quality claims need
  independent trials, relevant data, validated graders and uncertainty. A content hash
  establishes identity, not completeness, correctness or external freshness.
- Keep existing stable schemas compatible. Rich provider/deployment contracts require
  a separately reviewed schema evolution and consumer adapter, not silent defaults.
- Admit one bounded evidence-comparison incubation under 0004. First consumer design:
  resume_as_code's existing red-team workflow. First code: input binding, portable
  receipts, honest summaries and paired acceptance differences. No paid campaign or
  production model change is authorized by this record.

## Rationale

The missing product is a short path from a workload requirement to inspectable evidence
and a reproducible decision. More catalog rows and more agents do not establish value.
Detailed protocol, contract ownership, acceptance and pilot:
[benchmark evidence](../../designs/benchmark-evidence.md).

## Alternatives rejected

- A universal strongest-model ladder: roles, deployment constraints and workloads differ.
- An optimizer over public benchmark scores: external scores do not measure this solution.
- A new experiment service or autonomous model watcher: no consumer need for its operation.
- Family separation as proof of independence; repeated replay as additional quality trials.

## Deferred

Runtime selection, deployment discovery, new retrieval engines, judge panels and worker
execution remain outside this slice. Existing F1/F7/F8/F14 triggers are not satisfied by
the research request alone. Research recommendations are not implementation commitments.

## Revisit when

- 2026-10-16 or the next release: promote only with a real consumer adapter, coverage of
  failed work and reported collection cost; revise with new evidence or park otherwise.
- A receipt permits a stale, incomplete or unauthorized recommendation.
- Two consumers cannot represent the same material dependency using this contract.
- Stable catalog/journal evolution is needed by the pilot; record migration explicitly.

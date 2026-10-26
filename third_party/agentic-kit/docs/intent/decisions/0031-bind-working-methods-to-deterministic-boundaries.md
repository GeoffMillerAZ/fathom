# 0031: Bind working methods to deterministic boundaries

Status: accepted
Type: architecture
Date: 2026-10-22

## Context

The owner requests another excellence pass and asks where the quality-boundary
approach can improve other work. Decision 0030's executable/context binding revisit
conditions fired: a candidate Git shim could execute through PATH, and calibration
did not bind its full effective prompt and shared site cohort. HTTP error bodies
escaped the successful-response bound. These are repairable command-local gaps.
0026's model-authority rejection and F18's unpromoted lost-pin boundary remain intact.

## Decision

Extend the existing bounded reference, without a new runtime or implementation slot.
Canonical pattern: freeze inputs and trusted policy; enumerate required properties
independently of returned results; check computable properties; bound any judgment;
refuse missing evidence; enforce at the actual boundary; record outcome and regressions.
The [working-method designs](../../designs/deterministic-workflows.md) compose existing
authority, evidence, investigation, replay, host and learning contracts. They are
adoption blueprints, not claims that all compositions have shipped.

For `quality-gate`: resolve Git outside the candidate tree, support explicit trusted
`--git-bin`, preserve valid raw-OID push sources, bind calibration to the effective
question/state/confidence recipe and full configured site cohort, cap every HTTP body,
and default to one provider dispatch per invocation. A trusted policy can explicitly
raise `model.max_batches` to at most 32. Cache hits consume no provider dispatch.

Preflight hard rules, snapshots and required model inputs for all pushed refs before
any provider dispatch. Unknown shadow inputs remain advisory and cannot clear a required
floor. Enumerate required model sites
from trusted policy and refuse omitted, duplicate or unbound clear receipts. A boundary
without required rules/hold sites is unknown; shadow-only policy is observation.
Expose applicability reasons and changed paths outside selectors without claiming
those paths are independently known to need review. Use deterministic source/question
ordering and conservative confidence for provider probability rounding.

Sites in one policy deliberately share a bounded context. Any configured cohort
question/selector change invalidates their calibration. Separate policies/invocations
provide different disclosure scopes. This preserves batching without pretending that
prompt instructions isolate information. Existing calibration hashes must be renewed
from genuine evidence under the new recipe; copying new digests alone is not calibration.

## Rejected alternatives

- Turn all quality problems into LLM judgments: loses computable guarantees and cost
  control. Wins if: never for a computable required property.
- One global accuracy/confidence bar for every judgment: risk, class mix and error
  costs differ. Wins if: a named domain establishes a common risk contract. Policy
  bars remain explicit owner choices; permissive bars do not establish useful accuracy.
- Install a new universal workflow/policy engine or add OPA/CEL/Temporal dependencies:
  existing contracts suffice and I1 forbids dependencies. Wins if: separately admitted
  consumer integration demonstrates a missing seam; external tools can remain external.
- Automatic promotion from journals, model-written policy, or replayed successes:
  confuses observations with authority and inflates sample counts. Wins if: never
  without independent receiver-owned evidence and explicit trusted policy adoption.
- Make every shadow unknown block unrelated observations: defeats optional shadow
  collection. Wins if: a named profile requires full coverage and promotes those sites
  to required checks; advisory failures never waive required floors.
- Claim this pass closes every possible gap: finite review/tests cannot prove that.
  Wins if: an independently checked proof covers the exact declared finite property.

## Revisit when

0030's review date still applies. Reopen for a concrete bypass, missing required
coverage, policy/context or provider drift, measured false holds/clears, excessive
adoption cost, or two consumers needing the same missing contract. Build further
compositions only through the existing uplift admission and acceptance work order.

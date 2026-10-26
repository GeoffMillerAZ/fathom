# Command governance delivery

Implementation: `c5e21ecb8529bcdc16338d590f3909ff0f823a5d`, 2026-10-23.
Initial delivery `0c38e199` was followed by canonical assessment reuse and bounded
native model-reference support. The full gate and independent review cover both.
Authority: [0035](../../../intent/decisions/0035-command-safety-starts-with-offline-qualification.md).
Scope: CG-1..7 only; CG-8..16 are explicit deferred work in the
[work order](../../../plans/command-governance.md). No execution broker, scanner/model
installation, hook mutation, application-provider dispatch or consumer configuration change.

## Evidence

| Check | Observed result |
|---|---|
| Full `task validate` | Pass: formatting, vet, CUE/fixtures, script checks, constitution and complete race suite including nested modules. |
| Fresh focused race tests | Command lab and Assignment Kit/examples pass; full-gate caching does not replace the focused run. |
| Independent security review | [Initial findings and fixes](security-review.md), [final closure](final-review.md); mixed JS coverage, malformed native/normalized fields, duplicate JSON, exact physical bounds, CUE/Go capture agreement and Unicode wire integrity corrected. |
| Guide/skill trial | [Synthetic offline cases](skill-trial.md): direct source manifest 0/complete, JS manifest 2/incomplete with literal candidate retained; assess 0/advisory, traversal 1/blocked, missing required layer 2/incomplete. |
| Captured classifier | Valid pinned fixture is advisory; malformed identity/key/outcome/probability/confidence rejects. No dispatch occurs. |
| Skills | Both project-local skill metadata validators pass; guided trials do not prove implicit triggering in a fresh host. |
| Strict Fathom proof | 24 total CG/AS rows: all 15 claimed rows verified after the final AS8 claim, zero failing; nine CG rows remain deferred. Static proof is not live safety. |

The root subsequently verified a fresh built binary: direct audit 3/3 sources and
5 records completes; the JS sample has 2/3 complete sources, 5 records, one unsupported
wrapper and one search candidate. Counts concern proposals/candidates, not executions.
No raw source ID/path/command or arbitrary tool name is emitted in audit output.
Clean committed-source command and example binaries at `c5e21ec` subsequently report
`vcs.modified=false`; these are local build artifacts, not installed tools.

## Research and failed evidence retained

Today's scheduled Daily AI Release Brief and October 23 local-model addendum were
located after Pages connector searches did not expose them. Public primary-source
checks and adoption/rejection choices are in the [assessment](../../../assessments/2026-10-23-command-governance.md).
No Page/chat/schedule was edited and no private brief text/profile was copied here.
The [Astra consultation](astra-consultation.md) is one read-only Herdr consultation,
not a test or source for empirical quality.

INC-20 preserves the exploratory sampling failure: metadata discovery exceeded its
declared file bound; discovery bytes were unknown and vocabulary was open. Its counts
remain exploratory, not a reproducible bounded audit. The shipped explicit-source
reader, privacy regressions and skill remedy the reference path, not all agent reads.
O12 records a daemon refresh timeout; a later refresh returned successfully and
strict plan proof passed. No Fathom wiring or global configuration was changed.

## Boundaries

The command reads explicit regular files; it is not a filesystem sandbox and input
selectors must remain trusted against concurrent replacement. Classifier keys bind
the minimized projection and policy, not undisclosed raw target contents. A favorable
answer grants no permission. Passing fixtures does not prove scanner completeness,
model calibration, actual hook invocation, local inference residency, deployment
isolation, a complete historical command inventory or measured user benefit.

Shared contribution review retained app-kit's dependency-free CLI divergence; no
shared renderer defect or upstream UI mutation was justified. Consumer compatibility
for the accompanying new library is recorded in [Assignment Kit delivery](../../assignment-kit/2026-10-23/README.md).
The [contribution audit](contribution-audit.md) records every recoverable tip and
verified private preservation. [Harness friction](harness-friction.md) retains the
observed review-model classification refusal for later controlled diagnosis.

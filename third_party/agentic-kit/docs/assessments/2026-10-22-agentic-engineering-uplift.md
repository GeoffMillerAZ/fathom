# Agentic engineering uplift assessment, 2026-10-22

Baseline source: `531e9c0e47e591cbf8b62c1c3972dbb8d7c2dd34`.
Scope: contribution audit, primary-source research, code/test inventory, actual
offline evaluation and accepted design. No production migration or paid model run.
Direction: [program and request coverage](../designs/agentic-engineering/README.md).

## Decision

Invest in composed assurance paths before adding more disconnected primitives.
The source has strong local building blocks. Target readiness is **not established**:
the actual assessment returned required `fail`, with 11 scoped component passes,
2 known missing target capabilities and 5 composition/adoption unknowns.
`soft_score` is null and trust is `caller-declared`, not certification.

## Contribution inventory

Fresh `git pull --ff-only origin main` was already up to date. Independent audit
then fetched/pruned remote-tracking state and inspected local/remote refs,
registered worktrees, stashes, reflogs, unreachable commits, all 11 GitHub PRs
and immediate-parent sibling checkouts. Only main/origin-main at the baseline,
one worktree, no stashes and no open PRs were found. No new useful delta surfaced.

Unreachable `87003725767ffb010beeac285377803f1811aba3`'s exact scenario/output
binding is retained in `kits/eval-runner/runner.go` with missing/duplicate/foreign/
empty-ID/producer-mismatch regressions in `evalrunner_test.go`. It is integrated
history, not another patch to merge. Older draft material remains recoverable in
the prior verified backup. No new branches/worktrees needed pruning; no GC ran.
Detailed commands, coverage and backup reference: [contribution audit](../evidence/agentic-engineering/2026-10-22/contributions.md).

## Actual offline run

Executed existing `fitness-function-registry.AssessProfile` against 18 versioned
required definitions, subject/source digests and observed outcomes, then validated
Profile and Assessment with existing CUE contracts. Five uncached `go test -count=1`
groups passed. This is a deterministic projection of reviewed source/test evidence,
not an independent measurement of every semantic claim or consumer entrypoint.

| Criteria | Result and exact scope | Next proof |
|---|---|---|
| C01, C03 | Typed decision/refusal conformance and offline OpenAI Decisions wire pass | Live task-specific quality remains unmeasured |
| C02 | Unknown site-specific current calibration | Labeled holdout for an admitted site/deployment/question |
| C04 | Fail: no Google adapter in bounded provider inventory/search | Official wire fixtures, conformance and actual consumer path |
| C05, C06 | Evidence provenance rejection and exact catalog binding pass | Consumer source/definition lifecycle and independent scope |
| C07 | Remote ambiguity blocks further effects in local HITL tests | Actual destination reconciliation contract |
| C08 | Unknown consumer RBAC/effect decision coverage | Authenticated role/resource/purpose/tenant negative cases |
| C09 | Local encrypted vault/shred conformance passes | Actual durable consumer storage/disclosure/deletion boundaries |
| C10 | Strict sourced export rejects untrusted generation claims in local tests | Hierarchical rebuild, attended task, semantic quality |
| C11 | Unknown actual Review Kit exchange at a consumer revision | Typed finding/criterion/evidence exchange and domain refusal |
| C12 | Declared contradictory evidence raises investigation concern in local test | Conflict detection recall and useful escalation in a real task |
| C13, C14 | Local USD charge refusal and decision cache identity pass | All paid attempts and current permissions covered at the entrypoint |
| C15 | Unknown composed security screening effectiveness | Held-out misses/false holds plus actual prevented effects |
| C16 | Independent synthetic receiver adoption boundary passes | Prospective transfer and recurrence/cost reduction |
| C17 | Fail: no GitHub issue ingestion in bounded source inventory/search | Read-only cursor, edits/access-loss and dedup fixtures |
| C18 | Unknown fresh Claude/Codex/Antigravity negative-case parity | Versioned actual invocation probes; unsupported remains explicit |

Raw input, definitions/evidence, source hashes, assessment and command logs:
[offline report](../evidence/agentic-engineering/2026-10-22/offline/REPORT.md).
Both known-absence findings are bounded to the named source roots and searched
contracts, not claims that no unrelated code exists anywhere. Historical receipts
are not refreshed by this run. The profile's recorded evaluation time/validity
belongs to that historical observation; replay cannot make it fresh.

## Uplift choices and gaps

| ID | Finding / decision | Disposition |
|---|---|---|
| AE-F1 | Package breadth exceeds proved cross-kit workflows | Reuse existing seams; W1 starts with practice/evidence review and independent coverage |
| AE-F2 | Catalog identity is available; criteria completeness, applicability and evidence quality need domain proof | One pattern/antipattern criterion record; independent expected scope; Review Kit retains law |
| AE-F3 | Provider and host portability differ; Google and Antigravity coverage are missing | Capability/native-extension matrix, adapter conformance, explicit unsupported |
| AE-F4 | Probabilities and citations can look more authoritative than their evidence | 0026 rejects model-only permission/completeness; separate integrity/support/truth and calibration |
| AE-F5 | HITL/prose have transaction primitives but no complete accessible workbench/document graph | 0027 accepts one authority path, immutable projections and dependency-closure rebuild |
| AE-F6 | Security bins cover narrow guards, not comprehensive vulnerability scanning | Optional established-scanner adapters and honest coverage receipts; no gate changes |
| AE-F7 | Cache/cheap models/fanout may reduce or increase total work | Per-site baseline, shared budget, exact reuse and kill measurements before default routing |
| AE-F8 | Feedback needs source-specific reliable intake and receiver-owned promotion | Read-only GitHub adapter first; unsampled critical events, loss/existence accounting and review |
| AE-F9 | Goal/feature catalogs could create competing status truth | Project catalogs are projections over existing intent/TRACKING/plan/receipt sources |
| AE-F10 | app-kit has an approval presentation primitive, not the full proposed interaction | Pending reusable visual/test packets in docs/app-kit.md, no upstream acceptance claim |

## Intent review and research limits

Applicable: I1/I5/I13/I18-I21, P1/P4/P11-P13, 0004-0009,
0011-0014, 0017/0018, 0020 and 0023/0024. The 0011-0014 2026-10-16
review and older prose/lesson review dates fired. 0025 explicitly retains offline
delivery/maintenance and parks expansion until bounded wave admission. F5's second
self-auditing workload, batch economics F6, runtime routing F7, worker execution
F8 and panels F14 are not established by this request or by a passing local test.
No stable tier, dependency, public-release decision, consumer pin or global host
configuration changes. Q1/Q5/Q7/Q8 remain unanswered by this evidence.

[Research](../evidence/agentic-engineering/2026-10-22/research.md) draws on 31 dated
source entries, including vendor guidance, open workflow implementations, Inspect,
W3C, OWASP and NIST. Prescriptions, vendor experiments and observed product behavior
are labeled separately. Certification existence is confirmed for Foundations;
professional-syllabus claims were not established. No exam content was copied.
Successful public projects inform hypotheses, not portable performance promises.

Fathom returned stale/incomplete obligations and deferred freshness; later symbol
definitions named the baseline but cold call-graph queries explicitly withheld
callers/tests. Existing O11 covers resource-gated freshness. Bounded source reads
and executable tests supplied the evidence; no stale or cold result counted as
a passing static proof. Host hook-template drift is a separate unclosed DX gap.

`agent-hooks report` over the last seven days returned 204 spawn decisions,
21 denials and 21 recoveries within its 15-minute window, with zero allowed forks.
It also reported 838 historical subagent records with unknown spend. This is a
host-wide observation, not project cost or a 14/30-day reading for 0022's triggers.
Two correctly marked deep-critic requests in this run were still classified as
worker-tier violations; standard-tier independent review completed. Preserve that
routing defect for SH-03/SH-04 rather than weakening the tier gate.

## Delivery boundary

This change completes the requested vision/design phase: six goals, six detailed
specifications, 36 request mappings, a paired practice seed, durable decisions
including rejection, a sequenced work order and this actual baseline evaluation.
All new implementation tasks remain open in TRACKING/work order. Consumer usability,
production adoption, live calibration, model quality, savings and broad security
effectiveness remain unmeasured. Verification of this documentation change is
recorded in the [evidence README](../evidence/agentic-engineering/2026-10-22/README.md).

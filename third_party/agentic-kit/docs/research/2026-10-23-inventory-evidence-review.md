# Inventory and evidence uplift: sources, Astra and active-session handoff

Date: 2026-10-23. Outcome: accepted design [0037](../intent/decisions/0037-inventory-and-evidence-compose-with-domain-reviews.md),
[IER](../designs/inventory-evidence-review.md), [EDR](../designs/evidence-driven-reviews.md),
[IR work order](../plans/inventory-evidence-review.md). No runtime implementation.

## Evidence and ownership baseline

Agentic Kit baseline: `5af1c0b10ecb6b245b3f032b62cbe85ef2f71138`.
Platform Engineering inspected HEAD: `821c509035a8bbd98907c83b54372bec70116425`,
with concurrent dirty work. The [source pins](../evidence/inventory-evidence-review/2026-10-23/source-pins.json)
record exact inspected working-tree digests; new/uncommitted designs are not asserted
to exist at that HEAD. The platform checkout was read only and remains its owner's work.

| Source | Observed status and relevance |
|---|---|
| Agentic `kits/catalog-kit/README.md`, decisions 0023/0024 | Implemented definition/layer/stable-key composition and scan-go projection; no transferred domain authority. |
| Agentic evidence foundations, `kits/investigation/types.go` | Existing primitives and workflow; finding input evidence IDs are not independently assessed claim-specific support. |
| Agentic `foundations/telemetry/read-manifest/readmanifest.go` | Implemented considered-read health; empty/skipped reads can be complete without proving expected population coverage. |
| Agentic `foundations/quality/fitness-function-registry/profile.go`, 0036 | Existing caller-declared assessments and scoped Assignment Kit composition; wrappers cannot imply authenticated source truth. |
| Platform ADR-0096; `kits/review-consumer/internal/core/core.go`; `internal/inventory/inventory.go` | Accepted generated-inventory authority and implemented committed-file Tool/Practice/Inventory collection. Internal types are not a public cross-module API. |
| Platform `kits/review-consumer/README.md`, `internal/cli/sound.go` | Implemented `review-cli inventory` and `review-cli sound` producers and `docs/review` artifacts. A new producer's public ingress remains to be qualified. |
| Platform `docs/designs/platform-design-contracts.md` PDX-10/WB-9 | Proposed independent scope, source identity, coherent capture, incarnation and freshness joins; no universal attestation. |
| Platform `docs/designs/platform-assurance-atlas.md` ATC-1/2/6; delivery ATP-3/4 | Proposed native-owned promise/risk/control/evidence relationships and receiver-first dossiers; no general probability engine or runtime closure. |
| Platform design ledger IDEA-391/392 | Ownership determines app/infra lifecycle and repo layout; contracts remain pinned across layouts. |

## Two consultations, two different questions

The owner explicitly requested Astra through Herdr. One separate read-only Codex
pane, `wC:p4`, named `inventory-astra`, used Astra at high effort for exactly two
consultations. No subagent impersonation, source edits, application-provider calls, probes, installations
or effect authorization were requested. The temporary pane is closed after preserving
its bounded terminal captures; those captures are advice receipts, not runtime tests.
Retained privately at
`/Volumes/m2ext/backups/agentic-kit/design-reviews/2026-10-23-inventory-evidence/`;
each source-pin record includes its exact local artifact path and digest.

Consultation 1 addressed kit boundaries, missing behavior, typed contracts and staged
investment. It completed before the second prompt. Root then wrote the IER draft.
Consultation 2 reviewed that core draft for investigations and code/documentation/
well-architected review. EDR was being authored concurrently, so Astra's comment that
the linked workflow file was absent described its read-time state. Root subsequently
persisted the workflow contracts and incorporated the amendments below. Neither
consultation claims final runtime or visual acceptance.

| Reference | Advice | Owner disposition and durable location |
|---|---|---|
| AC1-1 | Do not put observed-state authority under Catalog. | Accepted, IER-1/0037. Existing catalog views and scan-go stay valid. |
| AC1-2 | Evidence Assessment should compose admission, claim support, conflicts and invalidation. | Accepted, IER-3. Profiles/adapters first; extraction needs proven independent reuse. |
| AC1-3 | Independent scope and negative-evidence capability cannot be inferred from collector success. | Accepted, IER-2 and EDR-A01/02/21/22. |
| AC1-4 | Keep evidence input, support, truth, confidence and authority distinct. | Accepted, IER-3/4 and all workflow completion contracts. |
| AC1-5 | Reject new stores, universal scores, autonomous responders and premature shared formats. | Accepted, 0037; F17/F19/F20 remain gated. |
| AC2-1 | Qualify absence with sensitivity, lag, retention, selectors and filtering; a canary proves one path. | Accepted, IER-2 and EDR-A21/22/23. |
| AC2-2 | Exact propositions require complete premises, not individually credible citations. | Accepted, IER-3; support relation remains separately assessed. |
| AC2-3 | Historical validity, assessment availability and present reuse are different temporal questions. | Accepted, IER-3 and EDR-A28. |
| AC2-4 | Redaction is a transformation; lost qualifiers and sensitive hashes matter. | Accepted, IER-6 and EDR-A17/24/27. |
| AC2-5 | Invalidation includes scope/policy/assumptions and new counterevidence. | Accepted, IER-3 and EDR-A09/25/26; independence must be demonstrated before retaining conclusions. |
| AC2-6 | Provisional ceilings need decoded-size/fan-out/cancellation qualification; measure misses and recall. | Accepted, IER-6/7. No capacity/security guarantee from chosen fixture sizes. |
| AC2-7 | Optional kits cannot make completion semantics optional. | Accepted, IER-1 and EDR's workflow-output table. |
| AC2-8 | Repo layout does not establish deployment atomicity or failure isolation. | Accepted, EDR-1/6 and EDR-A03/04/10/23. |
| AC2-9 | Separate code attribution/exposure; compare incremental/full; design mismatch need not imply a doc fix. | Accepted, EDR-3/4 and EDR-A11/12/29. |
| AC2-10 | Review completion differs from readiness; risk acceptance does not meet a floor. | Accepted, EDR-5. Existing domain dispositions remain authoritative. |
| AC2-11 | Probes are bounded proposals; small models cannot hide critical findings; reuse app-kit. | Accepted, IER-5/6, EDR-2/7, IR-11/12. |
| AC2-12 | Prove one native Review receiver before extraction. | Accepted, IER-7 and IR-01. Existing producer/read ports are not invented ingress support. |

## Primary-source cross-checks

[NIST OSCAL Assessment Results](https://pages.nist.gov/OSCAL/learn/concepts/layer/assessment/assessment-results/)
separates scope/subjects, actual assessment activities, observations, findings and
identified risks. Its assessment plan links avoid copying native records. This
supports explicit joins; it does not justify replacing native envelopes with a new
universal proof object. Any OSCAL mapping remains separately versioned and qualified.

[W3C PROV-DM](https://www.w3.org/TR/prov-dm/) distinguishes entities, activities,
responsible agents and derivations. Use that conceptual separation for lineage and
correlated sources. Provenance does not by itself establish semantic support or truth.
No PROV serialization or conformance is implemented by this design.

[AWS Well-Architected review process](https://docs.aws.amazon.com/wellarchitected/latest/framework/the-review-process.html)
emphasizes continual, lightweight, owner-participatory review, follow-up questions
for uncertainty and prioritized improvement actions. EDR adopts that workflow shape
while retaining domain-owned criteria and evidence. No vendor catalog is copied,
framework certification claimed or automatic cloud access configured.

These official sources were checked on the date above. They are design influences,
not evidence that this repository implements their standards or proves a consumer safe.

## Active-session handoff

The requested live Agentic Kit target is Herdr pane `wC:p1`. Its transcript shows
this very planning session. Root therefore applies this brief directly and records
it durably instead of injecting a duplicate task into its own prompt queue.

**Assignment delivered in this change:** decide ownership, record accepted/rejected/
deferred alternatives, specify contracts and workflow outputs, define adversarial
acceptance, update routed goals/tracking, and queue implementation honestly.

**Next admitted assignment:** take IR-01 only after receiver/slot admission. Qualify
one real native Review Kit receiving route and one synthetic source-correction dossier
under [0038's prose/HITLMAN anchor priority](../designs/agentic-engineering/prose-hitl-priority.md).
Independent receiver-authored scope must expose an omitted required item, an unrelated
citation and an incompatible source/version binding. Preserve native Test/Policy/Review
artifacts, raw findings and domain floors. Start in the consumer adapter/scenario;
do not create public Inventory/Evidence Assessment packages to make the demo compile.

Before admission, pin maintainer, exact receiver/profile, baseline, independent oracles,
permitted disclosure, corpus/attempt/storage/time budgets, zero live-effect boundary,
review date and promote/revise/retire tests. If the native ingress cannot preserve
these, record the gap with its owner; do not import `internal` packages or invent
interoperability. Keep app/infra source work read only unless separately authorized.

**Completion evidence later:** positive and adversarial entry-point tests, CUE/Go parity
for introduced shapes, full gate, affected downstream checks, strict plan proof after
claims and an exact scoped delivery receipt. Then qualify the next IR row. Retain
unknowns, observed misses and unsupported semantics. Do not mark whole G9..G14
complete from this design, a static parse or a synthetic dossier.

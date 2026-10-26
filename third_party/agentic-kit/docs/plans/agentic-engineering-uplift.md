# Agentic engineering uplift work order

Request (owner, 2026-10-22): "i want you to start by updating our vision and goals
and making sure all of these are durably recorded as visions and goals and
whatever. then get started on planning out designs."

Design: [program](../designs/agentic-engineering/README.md), decisions 0025-0028.
Status: W1's first bounded reference admitted by [0032](../intent/decisions/0032-admit-offline-practice-review-and-local-diagnostics.md); whole rows below remain open. Source contracts below name
existing integration seams confirmed by Fathom at baseline 531e9c0. They do not
claim the new behavior exists. Narrow each row's symbol to its actual new helper
when implemented, retaining the stable row ID and scenario acceptance references.
Static existence/reachability alone cannot close a row; the independent cases and
actual consumer receipts in the specification are required too.

## W1: practice and evidence reference

The [UP-3..6 reference specification](../designs/uplift-2026-10-23.md) implements a
bounded subset of AE-1/AE-2. Other typed families, actual Review Kit exchange, lifecycle
reuse and external adoption remain open; narrow reference tests cannot close them.
The [DL-1..6 utilities](../designs/decision-lab.md) extend this offline reference with
bound question packs and labeled studies under 0033. They do not close AE-7's routing
or independent live calibration exits, or start W2 before its prerequisites.
G15's [AAIF qualification work order](aaif-alignment.md) refines these existing
host/provider/evidence boundaries under 0034; it neither closes AE rows nor creates
a third slot. The new optional execution/evaluation/remote-agent paths stay deferred.

- [ ] Materialize paired practice definitions and typed family payloads with exact binding [PC-A01] [PC-A02] [AE-R04] {id: AE-1, symbol: PrepareDecision, package: kits/catalog-kit, ctx: docs/designs/agentic-engineering/practice-catalog.md}
- [ ] Detect omitted expected subjects and criteria before a complete assessment [PC-A03] [PC-A04] [PV-11] {id: AE-2, symbol: AssessProfile, package: foundations/quality/fitness-function-registry, needs: AE-1}
- [ ] Exchange an actual typed Review Kit criterion/finding/evidence packet without weakening domain law [PC-A08] [PC-A09] {id: AE-3, symbol: PrepareDecision, package: kits/catalog-kit, needs: AE-2}
- [ ] Bind lifecycle invalidation and access-safe evidence reuse in the review scenario [EL-A02] [EL-A04] [EL-A05] {id: AE-4, symbol: Cached.Decide, package: foundations/decision, needs: AE-2}
- [ ] Project authoritative goals and capabilities into evidence-bound stage views [PC-A13] [PC-A14] [PC-A15] [PC-A17] {id: AE-19, symbol: AssessProfile, package: foundations/quality/fitness-function-registry, needs: "AE-1, AE-2"}

## W2: portable decisions and bounded investigation

- [ ] Add declared generation capabilities and explicit native requirements including Google conformance [PV-05] [PV-06] [PV-07] [PV-08] [PV-17] {id: AE-5, symbol: LLMClient, package: foundations/llmclient, needs: AE-2}
- [ ] Exercise a bounded cheap-first review/investigation ladder without authority or budget escalation [PV-09] [PV-10] [PV-12] [PV-14] {id: AE-6, symbol: Cascade.Decide, package: foundations/decision, needs: AE-4}
- [ ] Bind application/model/question-specific calibration and unknown evidence to routing [PV-13] [PV-15] {id: AE-7, symbol: NewReading, package: foundations/decision, needs: AE-6}

## W3: human/prose vertical slice

Priority anchor under [0038/PX](../designs/agentic-engineering/prose-hitl-priority.md).
The existing AE-4/authority prerequisites apply; W3 does not wait for every W2 row or
IR-10. PX browser/attended proof supplements HW/PH, and none is closed by a mock.

- [ ] Apply consumer RBAC and attributes to shared human/agent proposal and commit paths [HW-01] [HW-04] [HW-06] [HW-09] [HW-15] [HW-16] [HW-17] [HW-21] [HW-22] [HW-23] {id: AE-8, symbol: Committer.Commit, package: kits/hitl, needs: AE-4}
- [ ] Bind accessible evidence/form workbench projections and required clarification plans [HW-10] [HW-11] [HW-13] [HW-14] [HW-18] [HW-19] [HW-20] [PX-01] [PX-02] [PX-06] [PX-10] {id: AE-9, symbol: Committer.Commit, package: kits/hitl, needs: AE-8, ctx: docs/app-kit.md}
- [ ] Add immutable hierarchical prose variants and deterministic dependency-closure rebuild [PH-04] [PH-05] [PH-07] [PH-20] [PH-21] [PH-22] [PH-24] [PH-26] {id: AE-10, symbol: SourcedPipeline.CheckDraft, package: kits/governed-prose-kit, needs: AE-8}
- [ ] Prove exact artifact publication and policy exceptions with independent change metrics [PH-10] [PH-12] [PH-13] [PH-16] [PH-19] [PH-23] [PH-25] [PH-27] {id: AE-11, symbol: SourcedPipeline.Export, package: kits/governed-prose-kit, needs: AE-10}

## W4: host/security/learning loop

- [ ] Add explicit established-scanner coverage adapters without treating unavailable scans as clean [SH-05] [SH-06] [SH-07] [SH-14] [SH-15] [SH-16] [SH-17] {id: AE-12, symbol: AssessProfile, package: foundations/quality/fitness-function-registry, needs: AE-2}
- [ ] Prove shared-rule host bindings and failure/existence journals on declared native versions [SH-08] [SH-09] [SH-10] [SH-11] {id: AE-13, symbol: run, package: cmd/kit-new, needs: AE-2}
- [ ] Deliver read-only issue intake to sanitized receiver-owned lessons with revocation [EL-A09] [EL-A10] [EL-A11] [EL-A13] {id: AE-14, symbol: Catalog.Admit, package: foundations/evidence/evidence-producer, needs: "AE-3, AE-4"}

## Conditional follow-through

- [-] Paid prospective quality/cost campaigns [PV-15] {id: AE-15, disposition: deferred, reason: "needs a current named deployment corpus and campaign authority", trigger: "offline path and explicit bounded live campaign are ready"}
- [-] Production Review Kit and other consumer migrations [AE-R34] {id: AE-16, disposition: deferred, reason: "consumer-owned acceptance follows the bounded exchange", trigger: "AE-3 and the destination owner accept an exact upgrade"}
- [-] Batch optimization and health/cost routing [AE-R16] {id: AE-17, disposition: deferred, reason: "F6 and F7 require measured consumer value", trigger: "asynchronous tolerance and baseline economics establish value"}
- [-] Worker executor and generic judge panels [AE-R36] {id: AE-18, disposition: deferred, reason: "I20 and F8/F14 remain binding", trigger: "their independently documented admission conditions fire"}

## Evidence before a checkbox

Run the row's independent negative fixtures at the actual entrypoint; retain
subject, definition, configuration, host/provider and test revision pins. For
exported code run full `task validate` and isolated `task downstream-check` with
each actual consumer result/skip. Refresh the source index through the daemon,
then run:

```sh
fathom verify --service agentic-kit --plan docs/plans/agentic-engineering-uplift.md --strict
```

Only checked rows are claims. Do not use `--require-all` until all admitted code
rows are actually done. Exit 2, cold graph or a stale definition is an operational
unknown, never proof. UI rows additionally need actual keyboard/narrow/error and
accessibility evidence plus attended-task measurement; source proof cannot supply it.
Historical evaluation in this change is a baseline, not completion of AE-1..AE-14 or AE-19.

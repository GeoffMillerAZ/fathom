# Inventory, evidence and review: queued implementation

Owner request, 2026-10-23: plan the next kit capabilities with two Astra
consultations, then persist investigation/code/documentation/well-architected review
uplifts for later implementation. Decision [0037](../intent/decisions/0037-inventory-and-evidence-compose-with-domain-reviews.md).
Specifications: [IER-1..7](../designs/inventory-evidence-review.md) and
[EDR-1..8 / EDR-A01..31](../designs/evidence-driven-reviews.md).

All implementation is deferred, not completed or admitted. No speculative symbol
contracts or test waivers are used. When admitting a row, change it to open, ground
its actual symbols/packages through Fathom and retain its stable ID. The phase
dependency order is S1: IR-01..04, S2: IR-05..09, S3: IR-10..11,
S4: IR-12..14. Within each phase, use the native owner and prior evidence before
generalizing. Admission needs a free 0004 slot, named maintainer/receiver, independent
scope oracle, baseline, fixed quality/cost bars, bounded corpus/attempts/retention and
dated promote/revise/retire conditions. A profile is not an incubation exemption.

[0038/PX](../designs/agentic-engineering/prose-hitl-priority.md) makes prose/HITLMAN
the anchor. IR-11 may accompany separately admitted W3 after its authority prerequisites,
without waiting for IR-10. Phase labels do not add a two-consumer gate to a local UI pilot.

- [-] Qualify one public native receiving route and bounded dossier fixture [IER-7] [EDR-1] {id: IR-01, disposition: deferred, reason: "no new receiver or implementation slot admitted", trigger: "receiver accepts S1 and proves a supported native artifact ingress without internal-package imports"}
- [-] Independent scope and coherent observed-inventory reconciliation [IER-2] [EDR-A01] [EDR-A02] [EDR-A21] [EDR-A22] [EDR-A23] {id: IR-02, disposition: deferred, reason: "S1 awaits admission and native receiver qualification", trigger: "IR-01 qualifies independent population and capture contracts"}
- [-] Purpose-bound admission and claim/premise-specific support [IER-3] [EDR-A05] [EDR-A06] [EDR-A07] {id: IR-03, disposition: deferred, reason: "no admitted claim-specific adapter", trigger: "IR-02 supplies qualified native evidence and independent positive/negative oracles"}
- [-] Temporal validity and authorized transformations plus dependency invalidation [IER-3] [IER-6] [EDR-A08] [EDR-A09] [EDR-A24] [EDR-A25] [EDR-A26] [EDR-A27] [EDR-A28] {id: IR-04, disposition: deferred, reason: "requires qualified evidence-use and dependency semantics", trigger: "IR-03 passes and the receiver owns correction/withdrawal/disclosure lifecycle"}
- [-] Bounded investigation plans and discriminating probe proposals [EDR-2] [EDR-A16] [EDR-A20] [EDR-A30] {id: IR-05, disposition: deferred, reason: "no investigation runtime or new probe execution admitted", trigger: "S1 passes and a receiver selects a bounded investigation with equivalent assignment semantics"}
- [-] Incremental code review with exact attribution and full-review comparison [EDR-3] [EDR-A11] [EDR-A29] {id: IR-06, disposition: deferred, reason: "code review adapter and impact closure unqualified", trigger: "S1 passes and a receiver supplies base/head plus seeded full-review oracle"}
- [-] Source-owned documentation correction and exact regeneration proposal [EDR-4] [EDR-A12] [EDR-A13] {id: IR-07, disposition: deferred, reason: "no correction/publication receiver admitted", trigger: "S1 passes and an owner accepts claim authority plus source/generator/consent bindings"}
- [-] Native well-architected review dossier and separate readiness reading [EDR-5] [EDR-A14] {id: IR-08, disposition: deferred, reason: "domain criterion/applicability and native finding ingress await qualification", trigger: "S1 passes and domain owners accept solution/control-plane criteria and dispositions"}
- [-] Conditional incident scenarios and readiness gaps without invented likelihood [EDR-6] [EDR-A03] [EDR-A04] [EDR-A15] [EDR-A30] [EDR-A31] {id: IR-09, disposition: deferred, reason: "incident runtime and numerical likelihood remain unadmitted", trigger: "S1 passes and receiver accepts a synthetic scenario with alternatives and honest support limits"}
- [-] Native adapter conformance and monorepo/split-repo equivalence [EDR-1] [EDR-A10] [EDR-A18] {id: IR-10, disposition: deferred, reason: "independent receiver commitments are unproven", trigger: "two owners provide native artifacts and compatible release/deployment fixtures after S2"}
- [-] Existing app-kit compact/full projections and accessible intervention tasks [EDR-7] [EDR-A17] [PX-01] [PX-02] [PX-06] [PX-10] {id: IR-11, disposition: deferred, reason: "actual UI runtime/pilot remains unadmitted", trigger: "named W3 consumer admits a bounded prose/HITLMAN task with authority prerequisites and app-kit browser/accessibility proof"}
- [-] Exact reuse and optional judgment qualification against a baseline [IER-5] [IER-7] [EDR-A18] [EDR-A19] [EDR-A20] {id: IR-12, disposition: deferred, reason: "no measured workload or live provider study admitted", trigger: "qualified receiver fixes false-pass/miss/false-hold and cost bars before comparison"}
- [-] Optional versioned native/OSCAL/PROV mappings with explicit loss [IER-3] [EDR-5] {id: IR-13, disposition: deferred, reason: "standards are design influences and no interchange receiver has committed", trigger: "two owners select exact format versions and qualify round-trip or explicit unsupported semantics"}
- [-] Extract a sibling Inventory Kit or Evidence Assessment Kit only on proven reuse [IER-1] [IER-7] {id: IR-14, disposition: deferred, reason: "no common public contract or measured duplication reduction established", trigger: "two independent owners satisfy S4 extraction and standalone adoption gates"}

## Proof and handoff

Start by reading [the exact handoff](../research/2026-10-23-inventory-evidence-review.md#active-session-handoff).
First candidate: one synthetic source-correction dossier, with inventory omission,
unrelated citation and changed-version negatives, feeding the later prose/HITLMAN task.
Keep the app/database case as a secondary fixture. Qualify the native Review route
before promising public interoperability. No production incident or paid model call.

Before claiming any admitted row, run the full gate, affected downstream checks and
strict plan proof after updating the checkbox:

```sh
fathom verify --service agentic-kit --plan docs/plans/inventory-evidence-review.md --strict --require-all
```

A successful parse with zero claimed rows proves no implementation. Runtime controls,
native host behavior, human usability and measured outcomes require their own receipts.
Review by 2026-11-21 or next release; keep F17/F19/F20 triggers separate.

# Agentic engineering design reviews

Latest pass: five further contract clarifications and a visual/reader pass against
`4b8c6e4`. See [third-pass findings](#third-pass-for-completeness-and-reader-clarity).
All identified request clauses retain dispositions; implementation stays open.

Second-pass baseline: `2d83c5a077a7a0fc58a4a22a893f56b5f83663b0`, 2026-10-22.
Owner asks whether every original request is addressed or durably rejected, and
requests another design pass plus three visual Pages.

## Second pass conclusion

The first pass covered the topics and major rejected directions, but its statement
that the request was fully specified was too broad. Two independent reviews found
13 concrete contract gaps. This change tightens the owning specifications, adds
the missing goal-view implementation row AE-19 and records decision 0028. It does
not turn those acceptance scenarios into executed tests or close runtime work.

The [clause audit](../designs/agentic-engineering/request-coverage.md) now splits
compound asks and links each identified obligation to acceptance or an explicit
limit/rejection. The [program](../designs/agentic-engineering/README.md) remains the
reader entrypoint. Fifteen runtime rows are open and four conditional rows deferred.

## Findings and design closure

DC findings came from the [completeness review](../evidence/agentic-engineering/2026-10-22/design-completeness-review.md).
DA findings came from the [adversarial review](../evidence/agentic-engineering/2026-10-22/design-adversarial-review.md).
These original reports describe the pre-change baseline, not current defects.
Both independent reviewers rechecked the amended owning contracts. DC1..6 and
DA1..7 are closed at design level, with no remaining substantive original-request
gap identified. This is a bounded review verdict, not proof of semantic perfection
or execution of the future fixtures. The work-order footer was reconciled with
the new AE-19 row. Concurrent token-economics documentation commit `0a0c5e2` was
preserved; it does not change the reviewed runtime baseline.

| ID | Missing or weak contract | Amended design and required future proof |
|---|---|---|
| DC1 | Default prose quality criteria and disable/override semantics | PH PQ-01..10, typed scope grants/merge, hard floors and visible optional-off/waivers; PH-19 |
| DC2 | Goal/capability/evidence dashboard joins and freshness reduction | PC authoritative keyed projection and stage-specific state; PC-A13..15, new AE-19 |
| DC3 | Simultaneous variants sharing a snapshot and partial readiness | PH ProjectionBatch, independent/all-required publish modes and honest partial remote delivery; PH-20/21 |
| DC4 | Concrete returns beyond app-kit | [Cross-kit register](../designs/agentic-engineering/contributions.md) gives Review/Test/Fathom/Catalog/host packets, owner and delivery conditions; no false upstream-delivered status |
| DC5 | Safe low-burden question suggestions | HW disclosure/permissions, known-answer freshness/dedupe, disputed premises and stop routes; HW-18..20 |
| DC6 | Buildable first security scan profile | SH opt-in bounded Go profile with required guard/vulnerability classes, build roots/configuration and explicit database-age choice; SH-14..17 |
| DA1 | Revocation between checks and effect | HW trusted atomic local fence and dispatch admission, explicit unobserved-source/remote limits; HW-16/17 |
| DA2 | Two-party approval bypass through delegated identities | HW attested controlling principal/chain, not two credential strings; HW-15 |
| DA3 | Uncited generation/review context omitted from dependencies | PH context manifest and conservative whole-exposed-snapshot binding; PH-24 |
| DA4 | Unicode/media/node selection bound to the wrong bytes | PH immutable canonical byte locators plus versioned rendered selection/extraction mapping; PH-22 |
| DA5 | Annotation/edit text treated as policy or authoritative facts | PH typed trust/intent and correction adjudication; PH-23 |
| DA6 | Superseded catalog policies reused by old profiles/approvals | PC reviewed domain migration map, unmapped hold and historical-only old exact bindings; PC-A15/16 |
| DA7 | Concept drift judged without intended-change reference | PH ChangeIntent and intended/unexplained/unknown deltas; PH-25 |

Additional precision: CUE document vocabulary explicitly includes headings and
executive summaries as section or separate output; partial streams are provisional;
EL names append-log, mutable poll, webhook, file/Git and batch-import source patterns.
Source/provider observations cannot establish global instant freshness or atomic
transactions across unrelated destinations. No new engine/framework is justified.

## Rejection and deferral check

Retained rejections: model-only safety/completeness authority; universal excellence
scores; certification as sole architectural authority; speculative central platform;
tokenization as authorization/erasure; blanket override of hard blockers; color-only
status; mandatory editor/framework first; guaranteed isolated regeneration despite
dependencies; cache identity that omits exposed context; arbitrary cross-destination
atomicity; Pages as a competing source of progress/policy. Reasons and wins-if/revisit
conditions live in 0025-0028 rather than a separate rejection log.

Retained conditional work: live calibration/quality/cost campaigns, production kit
migrations, measured async batching/routing and worker/panel expansion. Foundations
certification was researched; an unverified professional blueprint remains explicitly
unestablished, not silently treated as evidence. No new paid campaign is authorized.

## Visual reader views

The owner additionally requests three Pages: overall system design, HITLMAN and the
prose solution. They use native editable text, small diagrams, concrete task examples
and source links. Design status is visible; they do not claim a running workbench,
completed implementation or UI acceptance. The repository owns the specification.
[Saved Page links](../designs/agentic-engineering/visual-pages.md) retain the three
reader views. Save/readback verified their text and seven Mermaid diagram blocks;
browser inspection separately checks rendering, not application usability.

## Verification boundary

This pass changes documentation and future acceptance, not runtime library APIs,
schemas, dependencies, gates or consumers. Required before commit: relative-link,
clause/acceptance/work-order reconciliation; independent recheck of both finding sets;
`git diff --check`; full `task validate`; normal source-backed push hooks. Strict
work-order verification gates zero checked new implementation claims. The historical
18-criterion offline run remains unchanged: 11 scoped component passes, two target
gaps, five unknown gates and overall required fail. It is not re-dated by this design.

## Third pass for completeness and reader clarity

Baseline: `4b8c6e4e15bacd01eef70523c74e37d3f1a9a38e`. Two fresh read-only reviewers
found five further contract ambiguities; the owner accepts the corrections below.
This supersedes the earlier bounded no-remaining-gap verdict for those details,
not the historical reports or their baseline. Existing 74 clause identities remain
stable; owning acceptance and work-order references are extended without checking
off implementation rows.

| ID | Gap and smallest correction | Required counterexample |
|---|---|---|
| DP3-01 | Provider-hosted tool execution needs proved interception or enforceable preauthorization before the outer call; otherwise unsupported | PV-17 refuses a hosted disclosure that local tool execution could not intercept |
| DP3-02 | Expected capabilities need an independently authored scope-owner manifest, not a TRACKING-derived set | PC-A17 deletes a whole TRACKING row while expected scope stays pinned and detects the omission |
| DP3-03 | Distinguish requested purpose from trusted host-attested effective purpose | HW-21 refuses a caller's broader purpose for read/cache/approval/effect |
| DP3-04 | Approval must bind resolved destination/account, disclosure audience and publish mode | HW-22/PH-27 refuse private-to-public reuse of identical approved bytes |
| DP3-05 | Reconciliation cannot replenish single-send consent under 0008 | HW-23/PH-27 require fresh authority after confirmed non-delivery and never resend confirmed delivery |

Reader changes: the program now routes by desired outcome, separates evidence,
permission and observed effect, and traces one synthetic correction across the
compositions. Each owning specification gains a small workflow/ownership diagram.
The existing three Pages receive matching boundary clarifications, nine diagram
blocks and readable layouts. The visual review also corrected destination binding
to appear as input to the publication gate. These are explanations of the accepted
target, not UI prototypes, executed acceptance fixtures or new empirical results.
Both reviewers rechecked their contract findings as closed at design level; the
remaining diagram ambiguity was corrected and independently rechecked. No additional
framework or speculative feature was justified by the bounded review.

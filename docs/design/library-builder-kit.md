# The understanding library as a kit for scanner builders

Status: Proposed 2026-11-14 (architect ruling on owner direction). Records: ADR-112 and ADR-113 drafts in the handback. Plan: `docs/plans/library-builder-kit.md` (LBK). This adds builder support to the existing library roadmap, not another engine program.

Owner evidence: an outside consumer, attested by the owner. Evidence class: **attested, unseen**. No consumer code, performance result, identity, licence grant or requested API has been inspected. The requirements sheet accompanies the handback.

Intent before design: P2 favors deterministic checks, P20 requires a consumer and P21 requires an enforced boundary before splitting (`docs/intent/principles.md:12`, `:72`, `:75`). I69 keeps the door free of store, daemon and network dependencies, I56 forbids implicit fetches, and I62 keeps model judgments off proof surfaces (`docs/intent/invariants.md`, I69, I56, I62). The closure and values tests are the existing enforcement, not new evidence from this spec. ADR-092's no-import premise is superseded by the attestation. Its module/repository triggers and F100's process-helper trigger remain unproven. Preserve those options through existing boundaries without building future packaging.

## 1. Findings

- LK-F1 The existing door already has public types, an API snapshot, an outside-module build and import closure tests (`pkg/lang/types.go:21`, `pkg/lang/api_test.go:19`, `pkg/lang/outside_test.go:42`, `pkg/lang/closure_test.go:101`). Builder support extends these controls.
- LK-F2 The binary already serves extraction, resource reads, contract reads and catalog import. JSON has a schema version, additive fields and distinct success, failure and misuse exits (`cmd/fathom-lang/main.go:5`, `:27`, `:101`). LIB-7 owns the future hazard entry (`docs/plans/library-roadmap.md`, LIB-7).
- LK-F3 The current test kit grades dialect packs and their goldens, not consumer checks (`pkg/lang/langtest/langtest.go:1`, `:52`). HZ-16 owns kernel mutation operators, HZ-18 owns external agreement and LIB-10 owns draft acceptance (`docs/plans/change-hazards.md`, HZ-16, HZ-18, `docs/plans/library-roadmap.md`, LIB-10).
- LK-F4 The use census contains 67 judged uses. Its six primitives become eight with lifetime evidence and measured reading levels (`docs/assessments/2026-11-12-library-uses.md:9`, `docs/design/library-roadmap.md:48`). The gap survey below reuses those judgments.
- LK-F5 Earlier records say the outside consumer is unbuilt or absent (`docs/design/library-roadmap.md:22`, `:103`, `docs/assessments/2026-11-12-library-uses.md:32`). Owner attestation supersedes that premise only for the import condition. Consumer requirements and any measured benefit remain unknown.
- LK-F6 Existing kernel rows remain gated on OQ3, and LIB-10 is deferred (`docs/plans/change-hazards.md`, HZ-13 to HZ-16, HZ-18, `docs/plans/library-roadmap.md`, LIB-10). The new ruling changes scheduling authority, not their contracts.
- LK-F7 Whole-program lifetime joins belong in the product's typed pass. Only site facts enter the door (`ADR.md`, ADR-107 decisions 3 and 4). A daemon-free example must respect that limit.

## 2. Numbered design decisions

### LK-D1 Attestation meets the import condition, not every trigger

Record ADR-092's outside-import condition as met, class attested, unseen. Its no-import sunset on 2027-05-04 no longer describes the evidence. This permits continued work on the existing door and concrete capability requests. It proves no accuracy, speed, adoption scale, second consumer, independent release cadence or four-week gate trial.

F99 still requires a named capability and consumer before another public door (`docs/intent/futures/f98-f99.md:14`). Treat the examples below as explicit consumers of the specified kit work. Do not promote unrelated register entries from an unseen import. OQ4 remains open. Content with no redistribution licence stays unpublished, and IF-6 still needs IF-11 and its release conditions (`docs/vision/46-proof-at-the-moment-of-change.md:324`, `docs/plans/library-roadmap.md`, LP-3).

### LK-D2 Ship building blocks, not a scanner corpus

The library is a kit for scanner builders. It supplies the eight primitives, the check kernel, a consumer test kit and a cookbook. Consumers supply rules, baselines, severity, waiver policy and reporting. The examples are small teaching fixtures, never a default scan or shipped rule pack. The base never imports checks (`docs/design/library-roadmap.md:35`, `:38`).

Keep the free scanners named in `docs/design/change-hazards.md:69` to `:75` as comparison subjects and possible consumers. Competing on their rule counts is outside this design. No assertion about their current market coverage is made here.

### LK-D3 Schedule the existing kernel without expanding it

The architect answers OQ3 yes on the attested use, subject to the owner's one-line confirmation. Record confirmation as pending, not received. The handback sheet carries that line, without interrupting this spec round.

After HZ-3, HZ-4 and HZ-6, run HZ-13 to HZ-16, HZ-18 and LIB-10 exactly as written. Add no predicate, oracle, API, truth value or acceptance requirement to those rows. The architect reconciles their gate prose and LIB-10's disposition when recording the ruling. Kernel implementation remains subject to that confirmation. The existing fact rows and independent builder readings can proceed.

### LK-D4 Use three reference consumers until requirements arrive

The owner sheet asks what was built in one line, which primitives were used, what was missing, what was slow and what was wished for. Until answered, requirements come from existing hazard fixtures and these in-repository consumers. They prove the specified integration paths, not unseen consumer behavior.

| Consumer | Input and boundary | Existing owner | New work and acceptance |
|---|---|---|---|
| Terraform change hazard | Two source trees, class transitions, locally placed catalog only when needed. No values, credentials, plan or daemon in the consumer | HZ-3, HZ-4, HZ-6, HZ-10, LIB-7, LP-4 | An executable wrapper over the binary's existing hazard contract. Rename and covering `moved` fixtures plus incomplete input. Agreement with the pinned HZ fixtures, no unknown converted to clean |
| Go resource lifetime | Graded lifetime site facts only. The consumer applies one declared syntactic tenancy rule from the RL track | RL-2, RL-M0, RL-9 in realignment CHG-10, LP-5 | An example over the RL facts and rule data, without another analyzer. Cross-function ownership and escape remain unknown. No claim that syntax proves acquire-release pairing. RL-3 and RL-4 stay in the typed pass |
| Infrastructure policy authored in CUE | A caller-authored CUE check compiled to the kernel's canonical JSON, applied to infrastructure facts | HZ-13 to HZ-16, HZ-18, LIB-10, LP-7 | An example that rejects an unknown as a pass, reports reasons and handles empty scope. CUE authors data, the kernel evaluates it. No CUE unification evaluator and no CUE dependency in the base |

The Go example reads files directly using RL-2's door. It does not launch the typed pass. It demonstrates the boundary and points to RL-M1 and RL-M2 for stronger claims. The CUE example uses an explicitly invoked local authoring step outside the base, preserving D6 and D7 of `docs/design/change-hazards.md:104`. No new dependency is assumed.

### LK-D5 Rank deterministic gaps, then verify the market hypotheses

This is an editorial ranking, not a measured market result. Need is the consequence of a wrong answer for an agent or check author. Cost is relative implementation effort over the existing rows, not an estimate in money or days. Absence is always a hypothesis. LBK-1 verifies it against pinned primary documentation and runnable public examples, reusing HZ-0 for the scanner landscape.

Primitive numbers reuse `docs/design/library-roadmap.md:52`: 1 fact delta, 2 hazards and contracts, 3 upgrade impact, 4 input inventory, 5 configuration references, 6 fact JSON, 7 lifetime evidence, 8 measured reading levels. No ninth primitive is proposed.

| Rank | Candidate and determinism need | Absence hypothesis to verify | Cost and reuse | Current owner or gate |
|---|---|---|---|---|
| 1 | Value-free change hazards. Critical when a proposed edit destroys a resource | A reusable Terraform source-pair API without plans is underserved. CloudFormation has a comparison oracle already | Medium, 1/2/3/6 | U-3, LP-4, HZ-1, HZ-9, HZ-10. Source: census U-3 and hazard K2 |
| 2 | Three-valued verdicts with reasons. Critical because an unresolved input must not become a pass | Reasons plus a closed read boundary are less available than three-valued logic alone | Low on the planned kernel, 2/5/6 | HZ-3, HZ-14. Source: hazard K1 and D4, beyond-determinism BY-L3 |
| 3 | Read sets and digests as proof inputs and cache keys. Critical when a receipt is reused after an input moves | Builder APIs rarely expose enough dependencies to reproduce and invalidate a verdict | Medium, 1/2/6/8 | HZ-3, LIB-10, BEN-9. Source: hazard D4, beyond-determinism BY-L4 |
| 4 | Version-bound provider contract facts. Critical for replacement and upgrade decisions | Small offline catalogs with binding provenance are underserved as reusable data | Medium, 2/3/6 | HZ-4, IF-13, IF-17, LP-3. Publication waits on OQ4. Source: census U-7, U-53, U-62 |
| 5 | Go resource lifetime evidence. Critical when leaks survive local checks | Cross-function evidence might improve on the union of existing linters. No win is assumed | High for typed work, 7/8 | RL-2 first, RL-3/RL-4 on their cost and kill gates. Source: census U-12, beyond-determinism A-N3 |
| 6 | References resolved across infrastructure, code and pipelines. High for rename and change reach | A common, evidence-tiered reference boundary across formats is underserved | High, 1/4/5 | U-4, U-36, U-42, LREG-13, FCT-11. Census and typed-link triggers stay. Source: census U-42 and library E16 |
| 7 | Fact deltas with stable identity. High for agent edits and collateral-change checks | Reusable cross-format change sets with explicit incomplete reads are underserved | Medium, 1/6/8 | LIB-13, HZ-6. Source: census U-1, U-2, U-40 |
| 8 | API lifecycle evidence. High when deprecated or removed APIs drive a migration | A common contract-change and use join across code and infrastructure is underserved | Medium to high, 1/3/5 | U-8/U-9 remain on their triggers, U-10 uses SG-39, U-65 needs nothing new. Source: census U-8 to U-10, U-65 |
| 9 | Input inventories with pin state. High for reproducible agent-produced changes | Chart, pipeline and infrastructure inputs in one typed inventory are underserved | Medium, 4/5/6 | Actions through UND-1/UND-4, other inputs remain LREG-21. Source: census U-21 and its existing counterexamples |
| 10 | Consumer-owned check grading. High because deterministic repetition cannot prove a rule correct | A small reusable kit combining outside labels, unknown cases and mutation receipts is underserved | Low integration after existing acceptance work, 6/7/8 | HZ-16, HZ-18, LIB-10 plus LBK-6 below. Source: census U-39/U-54/U-60, beyond-determinism BY-L4 |

The survey schedules no new resolver, lifecycle engine, cache or inventory extractor. If a market hypothesis fails, publish the counterexample and prefer an adapter. A digest is not a complete cache key until check bytes, catalog version, reader version, options, declared context and the complete read set are bound. Partial reads remain unknown. Context outside that boundary never becomes a fact through hashing.

### LK-D6 Make the builder path stable and measurable

Reuse the API snapshots and binary goldens rather than inventing another protocol (`pkg/lang/api_test.go:19`, `cmd/fathom-lang/golden_test.go:25`). Document stable and provisional surfaces explicitly. Stable field removals, renames, retypes and meaning changes need an approved breaking release and migration note. Additions must preserve old readers. Provisional fields retain the existing minor-release warning until their readings pass (`docs/design/library-roadmap.md:39`).

Document JSON envelopes, schema versions, ordering guarantees, unknown versus incomplete, exits and value opt-in. Use LIB-7's hazard shape without changing it. Do not flatten the kernel's satisfied/violated vocabulary into the product's holds/violated vocabulary without an explicit adapter (hazard D4 and vision 46 section 14a).

Extend `langtest` with a generic consumer-case grader accepting a caller callback and expected verdict data. It compares caller-provided labels, counts unknown reasons, and checks repeatability under reordered inputs. It does not implement evaluation, derive new mutants or accept drafts. Feed existing HZ-16 mutant outputs and LIB-10 receipts through adapters. A false pass, dropped unknown or mismatched receipt fails. An unlabelled case is ungraded, never counted correct. Keep pack grading compatible and keep the base from importing the grader or kernel.

Publish a "build a check in a day" cookbook from the three executable examples: read facts, state scope, author check data, handle unknowns, grade with outside labels, apply existing mutants, bind the receipt, then integrate through JSON or Go. The title is a usability target, not a measured completion time. LBK-12 reads it with a second contributor and reports elapsed active time, failed steps and required private knowledge. The acceptance bar is completion within one working day with no unpublished steps. A miss fixes the guide before that promise is advertised.

Measure performance before setting new limits. LIB-11 owns the closure-bar ruling and LL-M1 owns its original size bar (`docs/plans/library-roadmap.md`, LIB-11, `docs/bench/readings/lang-library/m1-closure.md:3`). LBK-9 records cold/warm time, bytes, allocations, peak memory and input size for the examples, then freezes proposed budgets before LBK-10's independent reread. A new timing ceiling is unknown today. Passing existing read deadlines does not prove interactive speed.

No daemon, network access or ambient catalog download is required. Hostile-input examples use the existing supervised binary boundary, not an invented process helper (`cmd/fathom-lang/main.go:20`, `docs/intent/futures/f98-f99.md:23`). This does not assert that masking makes arbitrary source safe to share (ADR-092, Literals in a Result).

### LK-D7 Keep the tool family in one repository

ADR-113 names roles, not speculative brands. The existing product is `fathom`. The existing understanding binary is `fathom-lang`. Catalogs are data, typed records stay a leaf, and the bench driver becomes a separate tool only after BEN-1's merge. Choose other names when a split is actually justified.

| Member | Boundary to enforce | Reuse and disposition |
|---|---|---|
| Proof-layer product | May consume the understanding layer and typed records. Does not evaluate consumer resource rules or opt in to values | ADR-092 decision 7, ADR-063 O11, existing product values test noted in library LB-F9's source `docs/design/library-roadmap.md:23` |
| Understanding library and binary | No store, fusion, connector, MCP or network imports from the door. Base does not import checks. Hazards remain facts, kernel accepts caller checks | Existing closure/no-side-effect tests, HZ-14's own closure. RL-3/RL-4 remain typed-pass exceptions outside this closure |
| Provider catalogs | Versioned, digested data with licence provenance. No executing providers or fetching from the consumer | IF-6/IF-11 and OQ4. Catalog implementation remains where its existing rows own it |
| Typed records | No store dependency from its first implementation. Product adapters provide data through its port | `docs/design/typed-records.md:133`. No new package or public API in this lane |
| Bench driver | Command outside the product after merge. Production packages never depend on its driver. Benchmark subjects may import the public library | `docs/plans/proof-program.md`, BEN-1. No second driver before merge |

LBK-13 checks current import boundaries and negative cases. LBK-14 adds records and merged-driver edges after their owning rows land. No import test passes by skipping an expected member. Missing planned members are recorded as pending until that row is eligible. Tests should inspect imports transitively and distinguish test/benchmark subjects from production dependencies, rather than banning a whole shared directory by name.

No new repository, nested module, renamed command or licence change follows from this design. ADR-092's recorded triggers remain: a real version conflict for a nested module and a second outside consumer needing an independent release cadence for a repository. Neither is proven by the attestation. Keeping existing import boundaries preserves those options without prebuilding packaging.

## 3. Risks

- LK-R1 Attestation gets retold as a benchmark or detailed requirements. Preserve the unseen qualifier and leave the five sheet answers unknown until supplied.
- LK-R2 The kit grows a default rule corpus. Keep teaching checks under examples and make consumers select every check explicitly.
- LK-R3 A Go example overclaims whole-program correctness. Require explicit unknowns for escape and keep RL-3/RL-4's cost gates and kills intact.
- LK-R4 CUE authoring silently becomes another evaluator or expands closure. Compile outside the base and route evaluation through the existing kernel only.
- LK-R5 Performance limits become invented precision. Publish the preregistered budgets and independent reread, including failures, before claiming a speed target.
- LK-R6 Family naming drives premature splits. Enforce boundaries first and reopen repository strategy only on its recorded trigger.

## 4. Sequence

1. The architect records ADR-112/ADR-113, the attestation and pending OQ3 confirmation. Reconcile older absence claims without inventing consumer details. LBK-1 surveys gaps and LBK-2 records the requirements source.
2. Keep LIB-12, LIB-11, LIB-1 and LL-M7's release order. Complete HZ-3/HZ-4/HZ-6 on their own rows. Builder contracts and present-family boundary tests can run alongside the main roadmap.
3. On owner confirmation, run the existing kernel and acceptance rows unchanged. Build the consumer grading adapter and the Terraform, Go and CUE examples on their stated prerequisites.
4. Assemble the cookbook. Measure, freeze budgets, independently reread performance, and run the one-day builder trial. Publish misses as well as passes.
5. Extend family tests when typed records and the merged driver exist. Revisit further capabilities only on their original triggers and grounded consumer requests.

## 5. Decision models

No new judged site applies. Facts, callback grading, compatibility and import edges are deterministic. Provider `none` supports every specified path, and consumers opt out by not invoking examples or grading. Draft generation remains the caller's action under ADR-106, with no model resolving an unknown, accepting a check or judging a security property. The existing HZ-15/HZ-16/HZ-18 and LIB-10 measurements grade the kernel and drafts. LBK-1 can refute market hypotheses, and LBK-10/LBK-12 can refute performance and usability claims. No paid measurement runs in this spec round.

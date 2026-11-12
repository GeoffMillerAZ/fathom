# Change hazards: contract and change hazards that never read a value

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Grammar v2 (`docs/plans/README.md`). Prefix: HZ- (HZ-n work, HZ-Mn readings). Design: `docs/design/change-hazards.md` (codes H, V, E, D and M are its). Decision: ADR-099. Goal served: G49. Prerequisite rows live in `docs/plans/iac-facts.md` and are referenced by id, not duplicated: IF-13, IF-6, IF-11, IF-14, IF-17.

Rows HZ-0 to HZ-22 follow the design's phase order. The design's readings M1 to M7 are HZ-M1 to HZ-M7. HZ-1, HZ-15, HZ-16, HZ-17 and HZ-18 are themselves the readings HZ-M2, HZ-M3, HZ-M5, HZ-M4 and HZ-M6, so each opens its reading file with that verdict line. HZ-M1 and HZ-M7 are separate rows. M8 (agent outcome) is deferred in the design and has no row.

No store table, column, cache, memo file or background job. The product never reads a value (ADR-092 decision 7) and never evaluates a resource rule (ADR-063 O11). Readings live under `docs/bench/readings/change-hazards/`, one file per row, each opening with a verdict line. A row flips only through `fathom verify --service fathom --plan docs/plans/change-hazards.md`. Raw plans, exports and replay state stay under `workspace/`.

Owner gate: HZ-13 to HZ-18 start only on the owner's written ask for the check kernel (OQ3 in `docs/vision/46-proof-at-the-moment-of-change.md`, design D8). Until that ask exists they stay open and nothing in them is built. HZ-14 to HZ-18 wait on HZ-13 by `needs`.

Stop rules:

- If HZ-1 reads under its floor (the IF-CM3 floor of 0.85), fix that before any other row.
- If HZ-9 precision (HZ-M1) reads under 0.9, no agent surface for hazards.
- If HZ-17 shows Trivy decides far more at equal soundness, the kernel stays small and the design's R5 advice stands.

## Phase A: measure what exists, no new engine

- [ ] (op) Landscape table from primary sources: Terrascan and Datree status, licence wording, per-platform counts, no `[prior]` tag left in the design {id: HZ-0, oracle: lexical, path: docs/bench/readings/change-hazards/landscape.md, pattern: "^HZ-0 verdict:"}
- [ ] (op) HZ-M2: the CloudFormation change check against `cdk diff --template`, the independent oracle IF-CM3 is waiting for, at the IF-CM3 floor of 0.85 {id: HZ-1, oracle: lexical, path: docs/bench/readings/change-hazards/m2-cdk-diff.md, pattern: "^HZ-M2 verdict:"}
- [ ] `iac` is a seventh review dimension with `Checked` and `Reason`, so an absent catalog or an `as_of` review states why it is silent, output schema change with a change note {id: HZ-2, symbol: reviewIaCDimension, package: internal/mcp, reachable_from: reviewIaCFindings}

## Phase B: additive facts

- [ ] `UnknownReason` and a read set ride on the public `iac` result, so every unknown states its reason {id: HZ-3, symbol: ReadSet, package: pkg/lang/iac, library: true, reason: "the public result type is the deliverable and the first outside consumer is the owner's policy tool", needs: HZ-2}
- [ ] Structured blocks (variables, outputs, locals, provider requirements, `moved`, `removed`, `import`) and a public binding resolver from lockfile to catalog version {id: HZ-4, symbol: ResolveBinding, package: pkg/lang/iac, library: true, reason: "every Terraform contract check needs the resolver and a door consumer must not supply the provider version itself"}
- [ ] Class and span per member of a Terraform object or map, read in the LL-M3 style {id: HZ-5, symbol: MemberAttrs, package: pkg/lang/iac, library: true, reason: "the member class is the deliverable, consumed by H5 and the kernel", needs: HZ-3}
- [ ] `ChangeSet` in the door: two fact sets by address with class transitions, generalizing the CloudFormation diff, the IF-C4 tests pass through the port {id: HZ-6, symbol: DiffResources, package: pkg/lang/iac, library: true, reason: "the change set is the shared input of every H check", needs: HZ-4}
- [ ] Inline suppressions are facts (tool, id, reason, expiry, span) for `checkov:skip`, `tfsec:ignore` and `trivy:ignore`, with fixtures for the three syntaxes {id: HZ-7, symbol: ReadSuppressions, package: pkg/lang/iac, library: true, reason: "disposition stays with the review kit and the reader only emits the facts", needs: HZ-4}

## Phase C: the product's bet, the H checks

- [ ] The existing prerequisites are claimed in `docs/plans/iac-facts.md`: IF-13, IF-6 and IF-11 {id: HZ-8, oracle: lexical, path: docs/plans/iac-facts.md, pattern: "^- \[x\] .*\{id: IF-(13|6|11),", min: 3}
- [ ] H1 on Terraform: `isStatefulType` over the stateful type list, joined to IF-14's replacement flag in `review_diff` {id: HZ-9, symbol: isStatefulType, package: internal/iacfacts, needs: "HZ-6,HZ-8"}
- [ ] H2 and H4: `CheckRenameWithoutMoved` flags a renamed address with no `moved` block, and its sibling covers a `count` and `for_each` switch with no covering `moved` {id: HZ-10, symbol: CheckRenameWithoutMoved, package: internal/iacfacts, needs: "HZ-6,HZ-4"}
- [ ] (op) HZ-M1: hazard precision by replay for H1, H2 and H4 on Terraform against the plan of head over a state synthesized from base, with the fake-credential cost and its bias stated, bar 0.9 precision {id: HZ-M1, oracle: lexical, path: docs/bench/readings/change-hazards/m1-replay.md, pattern: "^HZ-M1 verdict:", needs: "HZ-9,HZ-10"}
- [ ] (op) H6 is IF-17 with fixtures: provider upgrade impact on attributes in use, carried as a hazard once IF-6 is claimed {id: HZ-11, oracle: lexical, path: docs/bench/readings/change-hazards/h6-upgrade-fixtures.md, pattern: "^HZ-11 verdict:", needs: HZ-8}
- [ ] H3, H5 and H8: `CheckProtectionChange` covers a protection attribute changed or removed on a stateful resource, a literal-to-expression transition is advice only, and a literal on a contract-sensitive attribute is checked against secret scanners in HZ-18 {id: HZ-12, symbol: CheckProtectionChange, package: internal/iacfacts, needs: "HZ-6,HZ-4"}

## Phase D: the check kernel (owner gate, OQ3)

- [ ] Verdict and check schemas are frozen as CUE with valid and invalid pairs, starts only on the owner's written ask {id: HZ-13, oracle: lexical, path: schema/check.cue, pattern: "^#Verdict:"}
- [ ] The kernel `Evaluate` in `pkg/lang/check` imports only `pkg/lang/iac`, ships no checks, and pins the truth tables (`all` over an empty set holds, `any` over an empty set does not) with its own closure test and API snapshot {id: HZ-14, symbol: Evaluate, package: pkg/lang/check, library: true, reason: "the kernel is a public library surface for the policy tool and has no product caller by design", needs: "HZ-13,HZ-3"}
- [ ] (op) HZ-M3: bench plan reader and kernel soundness against the plan, a definite static verdict equals the verdict on plan values, bar 1.000 on a literal basis and 0.99 on defaulted {id: HZ-15, oracle: lexical, path: docs/bench/readings/change-hazards/m3-soundness.md, pattern: "^HZ-M3 verdict:", needs: HZ-14}
- [ ] (op) HZ-M5: mutation operators derived from each check (flip the literal, delete the attribute, defaulted variable, undefaulted variable, function call, local), accepted at 100% kill on V1 to V5 {id: HZ-16, oracle: lexical, path: docs/bench/readings/change-hazards/m5-mutation.md, pattern: "^HZ-M5 verdict:", needs: HZ-14}
- [ ] (op) HZ-M4: decidability on a preregistered random sample stratified by module use, unknowns counted by reason, head to head with Trivy's resolution, ranks E2, E4 and E5 {id: HZ-17, oracle: lexical, path: docs/bench/readings/change-hazards/m4-decidability.md, pattern: "^HZ-M4 verdict:", needs: HZ-14}
- [ ] (op) HZ-M6: agreement on V checks against KICS positive and negative fixtures and IaC-Eval, executed and pinned by checksum with nothing copied in, confusion matrix and adjudication ledger {id: HZ-18, oracle: lexical, path: docs/bench/readings/change-hazards/m6-agreement.md, pattern: "^HZ-M6 verdict:", needs: "HZ-14,HZ-16"}

## Phase E: gated follow-ons

- [ ] (op) Pre-write delta labs card: base against proposed text, speaking only on a transition, value-free, inside the I35 320-token cap, with lower noise than the post-write hooks, gate is ADR-091 decision 7 and the IF-M6 trigger {id: HZ-19, oracle: lexical, path: docs/bench/readings/change-hazards/prewrite-card.md, pattern: "^HZ-19 verdict:"}
- [ ] (op) E4 (bounded pure-function tier) or E5 (local module expansion), whichever HZ-17 ranks first, with the decidability gain and a change note, each needs its own design amendment {id: HZ-20, oracle: lexical, path: docs/bench/readings/change-hazards/e4-e5-ranked.md, pattern: "^HZ-20 verdict:", needs: HZ-17}
- [ ] (op) Kubernetes and Helm in the door (E11): an OpenAPI-derived catalog checked with kubeconform, then HZ-M7 with `helm template` as oracle, sound only for charts with no `lookup` and no capability branches {id: HZ-21, oracle: lexical, path: docs/bench/readings/change-hazards/k8s-helm.md, pattern: "^HZ-21 verdict:", needs: HZ-6}
- [ ] (op) HZ-M7: Helm reads against `helm template`, charts with `lookup` or capability branches read unknown with a reason {id: HZ-M7, oracle: lexical, path: docs/bench/readings/change-hazards/m7-helm.md, pattern: "^HZ-M7 verdict:", needs: HZ-21}
- [ ] (op) E13, an IAM policy document model, gate is the F97 promote condition (IF-M2 read and a named adopter) {id: HZ-22, oracle: lexical, path: docs/bench/readings/change-hazards/e13-iam.md, pattern: "^HZ-22 verdict:"}

# Change hazards: the library as a hazard engine that never reads a value

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Decision: ADR-099 (the hazard engine). Plan: `docs/plans/change-hazards.md` (HZ-). Goal served: G49. Builds on `docs/design/iac-facts.md` (IF-) and `docs/design/lang-library.md` (LL-). Method: seven read-only inventory lanes, a draft, then four critics told to refute it (engine truth, intent records, a hostile market read, external format facts). The critics overturned five draft claims. Section 4 lists them, because the corrections are the most useful part of this design.

Tags: `[web]` means checked against a primary source during the review. `[prior]` means unverified.

Codes in this document: U uplift, F finding, K draft claim killed, T and N thesis holds or fails, D decision, H hazard check, V value check, E engine gap, M measurement, S spin-off, X record change, R risk. Owner questions OQ3 and OQ4 live in `docs/vision/46-proof-at-the-moment-of-change.md`.

The verdict is the last section.

## 1. Intent that applies

Named before designing. No revisit trigger on any record below has fired. Each change here is an owner reprioritization, as the CloudFormation amendment to ADR-091 was.

- **ADR-091 and G49:** IaC facts come from read-only catalogs joined at query time. This design serves G49 ("an agent changing infrastructure code knows what the change does before any plan runs") and adds hazards on top of the contracts ADR-091 already holds. ADR-091 decision 7 requires a labs card before any edit-time delivery (section 6, D15).
- **ADR-092:** the language and tool understanding library is a public door in the one module. Decision 7: the product never opts in to values. Decision 8: the closure test. Decision 10: evaluator, verdicts, baselines and crosswalk stay with the policy tool. That tool is the owner's own and sits outside this repository by the owner's written choice on 2026-11-04.
- **ADR-063 O11:** fathom "never evaluates a resource rule, in core or in a tech pack". This design keeps that line for the product. The one proposed change is a kernel in the library, and only on the owner's written ask (D8, X1).
- **ADR-067 decision 12:** a tech pack never carries a resource rule. Kept.
- **ADR-069 decision 5:** severity, pass or fail, findings, the defaults pack and admission stay in the review kit. Resource and security rules stay with the policy kit. Kept.
- **ADR-040:** SARIF was rejected as the interchange for three stated reasons. None has been answered, so no SARIF door opens (D11). Disposition belongs to the review kit (decision 1, F32).
- **ADR-088:** Well-Architected and compliance stay evidence production. Decision 4: each imported scanner gets a producer entry with its own tier cap.
- **ADR-089:** fathom never mitigates and builds no transformation engine.
- **I38:** no instrument reports a conclusion it could not earn. A review section that goes silent without a reason breaks this, which is why `iac` becomes a stated absence (F8, D9).
- **I56:** the product fetches nothing and reads no plan file. A plan reader exists only in the bench (E10).
- **I62:** no model resolves an unknown or judges a security property.
- **I35, I57, I68:** the 320-token edit cap, heuristic links never gate, a tech pack never governs. Kept as constraints (X5).
- **F90:** no best-practice linter. Kept. It is why code analyzers are refused (D12).
- **F97:** contract catalogs beyond Terraform, Kubernetes and Pulumi, including an IAM policy model, wait for IF-M2 and a named adopter (E13).
- **F99:** no public door without a consumer named in writing. This is why the kernel waits for the owner's ask (D8, R6).
- **Existing rows:** IF-13, IF-6, IF-11, IF-14 and IF-17 are prerequisites. The plan references them by id and does not duplicate them.

## 2. The uplifted ask

The ask: "strengthen static analysis and policy as code on the library". Taken literally it produces a scanner, and scanners are free, mature and already in the agent's write path. The better question:

> What does fathom hold that a scanner does not, and what is the smallest thing that turns it into verdicts nobody else can give?

The answer is not the rule engine. It is two data assets joined: versioned provider contracts (with replacement facts) and a diff of two classed fact sets. Things the ask did not name:

- U1 `unknown` is a reported result with a reason and a read set. Incumbents compute it and drop it.
- U2 Checks about the change, not only the state. Incidents come from changes.
- U3 The product can reason about classes, contracts and changes without ever reading a value.
- U4 A plan, `cdk diff --template` and `helm template` are local oracles. No labels, no cloud account.
- U5 Decidability (share of definite verdicts) is reported next to precision, head to head with Trivy.
- U6 Inline suppressions written for other scanners are facts the reader can emit.
- U7 The provider contract catalog is a spin-off as data, not code.
- U8 The `iac` section of `review_diff` goes silent when no catalog is installed. That absence carries no read set, which the product's own invariant I38 forbids.

## 3. Findings: what is true today

Each finding names the repository source it summarizes.

- F1 `pkg/lang/iac` returns resources whose attributes each carry a class: literal, defaulted, reference, expression, unknown (`pkg/lang/iac/types.go`). Values are opt-in (`WithValues`), never evaluated through functions, and "the product never opts in" (ADR-092 decision 7, enforced by a test).
- F2 Terraform object attributes are one `Attr` (a map value when all literal). There is no class or span per member. CloudFormation walks members into paths. No unknown carries a reason. No read set is returned (`pkg/lang/iac/read.go`).
- F3 A module call is never expanded, and the public docs promise that (`docs/design/lang-library.md`). Variables, outputs, locals, provider requirements, `moved` and `import` blocks are not in the public result. The lockfile-to-version resolver is internal, so a door consumer must supply the provider version itself.
- F4 Contracts carry required, optional, computed, sensitive, write-only, deprecated, item bounds and replacement (Terraform set). The Terraform schema export has no enum, default, validation or force-new field `[web]`, so Terraform catalogs carry no replacement facts until IF-13. CloudFormation catalogs carry replace and may-replace from `createOnlyProperties` and `conditionalCreateOnlyProperties`. They carry no sensitive flag. No catalog ships (IF-6, IF-11 open). Sources: `docs/design/iac-facts.md` sections 3, 4 and 13, `docs/plans/iac-facts.md`.
- F5 Reader accuracy on small pools: Terraform 36 roots, precision 1.0, recall 1.0, class agreement 0.9986. CloudFormation 30 templates, 1.0 across. In that Terraform reading 47% of attributes sat in `dynamic` blocks that the oracle drops, so they are ungraded (the reader does emit them). The separate unknown-rate reading (LL-M6): Terraform 10.0%, CloudFormation 6.1%, Pulumi 16.7%, on a model-chosen pool. Sources: `docs/bench/readings/lang-library/m3-terraform-reads.md`, `m4-cloudformation-reads.md`, `m6-unknown-rate.md`.
- F6 No verdict evaluator exists anywhere. The LL-M6 scorer reads the class at a rule path and nothing more. The product does run contract checks: unknown attribute, required unset, deprecated, sensitive value in an unmarked output (`internal/iacfacts/check.go`), and a plan-free CloudFormation change check that diffs base and head by logical id against replacement contracts (`CheckCloudFormationChange` in `internal/iacfacts/cfn_replace.go`, row IF-C4 checked). It compares span text, so "no value leaves this function".
- F7 The Terraform side of that is specified and unbuilt: IF-13 (replacement scan), IF-14 (review flags a change that forces replacement), IF-15 and IF-16 (cascade), IF-17 (upgrade impact across two catalog versions). Gates IF-M1, IF-M2, IF-CM1 and IF-CM3 are unread. ADR-091 records that CloudFormation replacement has no independent oracle.
- F8 `iac` in `review_diff` is a payload field, not one of the six review dimensions. It is nil with no catalog and on `as_of` reviews, with no `Checked:false` and no reason (`internal/mcp/iac_cfn.go`, `reviewCFNFindings`).
- F9 Every rule mechanism in the product is CUE-authored data, a closed enum of kinds and Go evaluation (ADR-087, ADR-074). No expression language runs over extracted facts.
- F10 Code facts outside Go are name-level. Typed call edges are Go-only without a supplied SCIP index. There is no control flow, def-use or lifetime fact for any language, and no syntax tree in `pkg/lang`.
- F11 Kubernetes, Helm and Kustomize are L3 dialects in `pkg/lang.Extract` with a few pod-spec facts. They are not in the `iac` door, have no schema catalog, and Helm is not rendered.
- F12 The bench has pinned tool oracles (tofu, cfn-lint, checkov, kyverno, opa, kubectl, helm), an offline plan run that resolved 98.1% using fake credentials, provider skip flags and seeded state, a pinned corpus and preregistration (`docs/bench/readings/iac-facts/p3-replacement.md`, `docs/assessments/2026-11-02-iac-facts-probe.md`). It has no rule-verdict oracle and no fixture mutation.
- F13 The records are explicit and recent. ADR-063 O11: fathom "never evaluates a resource rule, in core or in a tech pack". ADR-067 decision 12: "A tech pack never carries a resource rule". ADR-069 decision 5: severity, pass or fail, findings, the defaults pack and admission stay in the review kit, resource and security rules stay with the policy kit. ADR-092 decision 10: evaluator, verdicts, baselines and crosswalk stay with the policy tool. That tool is the owner's own, outside this repository by the owner's written choice on 2026-11-04. F99: no door without a consumer named in writing. F90: no best-practice linter. No revisit trigger on any of these has fired.

## 4. What the critique killed

- K1 "Nobody else exposes the class on a value." False. Trivy's Go API carries resolvable, explicit and default on every value. Checkov computes UNKNOWN internally (a pull request to report it is open). cfn-guard has PASS, FAIL and SKIP `[web]`. What is new is narrower: an unknown that is reported, with a reason and a read set.
- K2 "Nobody does plan-free change hazards." False for CloudFormation. `cdk diff --template` maps the resource specification to will-replace and may-replace offline `[web]`. No static equivalent was found for Terraform. So Terraform is the open ground and CDK diff is the missing CloudFormation oracle (F7).
- K3 "Scanning at the edit is ours." False. Semgrep and Snyk ship agent hooks and Trivy ships an MCP server `[web]`. `terraform-ls` flags deprecated attributes as you type `[web]`. Only a pre-write delta that speaks solely on a transition survives, and it must prove lower noise.
- K4 "A model drafting a check once is a new idea." False. Datadog and Semgrep ship prose-to-rule with example validation `[web]`.
- K5 "An evaluator already exists in fathom, so moving it in is a tidy-up." False (F6). Building one reverses a week-old owner decision. It needs an owner ask, not an inference.

Also corrected: Trivy expands local and remote modules statically and fathom does not, so on module-heavy code Trivy resolves more values today `[web]`. Checkov documents 750+ policies and KICS 2,000+ queries `[web]`. tflint's 700+ AWS rules take enums from AWS SDK models, not the provider schema `[web]`.

## 5. The thesis, tested

The thesis: the deterministic engine does these jobs better, cheaper and more reliably than real-time model review. Against a model, yes. Against deterministic scanners, only where fathom holds data they lack.

Holds:

- T1 Terraform change hazards with no plan: replacement of a stateful resource, a rename with no `moved` block, a provider upgrade that removes or deprecates attributes in use. This needs versioned contracts joined to a fact diff. Fathom has the first half built and the second half specified. No tool found does it.
- T2 Honest absence. Reported unknowns with reasons and read sets, and receipts on every verdict.
- T3 Value-free reasoning (U3). Class, contract and change are enough for the hazard set in section 8. The product never needs a value, so it never holds a secret.
- T4 Joins the index already owns: which changed resource a test reads, who calls what, what a diff reaches. These rank findings, including imported ones. They are fathom's alone.

Fails:

- N1 Breadth. Thousands of free checks exist. Matching them is drift toward the mean.
- N2 Value resolution on module-heavy Terraform. Trivy and a plan both beat fathom today.
- N3 Memory, lifetime and anti-patterns in code. With no control flow and Go-only typed edges, fathom cannot beat `go vet`, staticcheck, Clippy, the Clang analyzers or CodeQL. F90 is right. Keep it.
- N4 Cost. It needs prices and usage. Runtime truth (drift, used permissions) is equally out of reach.
- N5 Delivery at the edit and prose-to-rule are table stakes, not differentiators (K3, K4).
- N6 Many "obvious" state checks are undecidable from files alone because account-level defaults sit outside the read set. S3 blocks public access and encrypts by default, and a log retention policy can be set per account `[web]`. A scanner that fires on absence is guessing.

## 6. Decisions

- D1 Reshape, and narrower than asked. Fathom the product becomes a **change and contract hazard engine that never reads a value**. The library equips an outside policy tool with complete facts and, on the owner's written ask, a small verdict kernel. No scanner, no rule corpus, no rule packs in core, no code analyzers.
- D2 One line divides the work. Checks that need only class, contract and change (H1 to H9) belong to the product and fit the existing records, because the product already emits such findings (F6). Checks that need values (V1 to V7) belong to the policy tool. Fathom keeps them only as bench fixtures that calibrate the kernel.
- D3 The fact model, for both sides: `Resource` and `Attr` with class, references as one-hop relations inside a declared read set, the contract at the bound provider version, structured blocks, and a `ChangeSet` of two fact sets by address with class transitions. Code facts are never inputs to a check. They join afterwards as ranking and scope (T4).
- D4 Verdict shape. Three truth values under strong Kleene logic: violated, satisfied, unknown. "Not applicable" is a scope outcome decided before evaluation, not a truth value. `all` over an empty set holds and `any` over an empty set does not, both pinned in the truth-table tests. `Basis` is the weakest class that decided: literal, defaulted, absent, contract, relation. Expression and unknown never decide. Absent follows per-type semantics, and "a default set outside the read set" is an unknown reason (N6). Every verdict carries spans, unknowns with reasons, the check digest, the catalog digest and the read set.
- D5 Relation checks conclude only in a closed read set. An unexpanded module call in scope makes the verdict unknown with that reason.
- D6 The rule language is data in a closed, total predicate algebra: path selectors, comparisons, `present`, `class in`, `refers to type`, `exists related`, `all`, `any`, `not`, and over a change `was`, `becomes`, `replaced`, `address removed`, `address added`. Canonical form is JSON and its hash is the digest. CUE is the authoring and schema layer, which honors "CUE never an execution engine". Named Go builtins behind a closed enum cover what the algebra cannot say. This is the house pattern (F9), and it lets the engine read a check: the paths it needs, a type check against each catalog version (a known technique, OPA does it against JSON schemas), and its mutation operators.
- D7 Rejected for now, with wins-if:
  - Rego as the authoring language. Wins if an adopter brings a corpus they will not rewrite. Until then the fact set is already JSON and `conftest` can read it with no code from us.
  - CEL. Total, typed, with native unknowns and residual expressions `[web]`. It is the strongest alternative. Wins if a Kubernetes adopter wants admission policy expressions run locally unchanged.
  - CUE unification as the evaluator. Its lattice mirrors the classes, but it breaks a non-goal and the door's closure. Wins if over 20% of proposed checks cannot be said in the algebra.
  - A fathom query language. No.
- D8 Kernel location and condition. `pkg/lang/check`, importing only `pkg/lang/iac`, with its own closure test. It evaluates a caller's check over a caller's facts and returns verdicts. It ships no checks. It is built only when the owner, as owner of the policy tool, asks for it in writing, which satisfies F99. If the owner prefers the tool to own evaluation, fathom ships only the truth tables as conformance fixtures in `pkg/lang/langtest`. Recommendation: put it in the library. Class semantics without evaluation semantics are ambiguous, and they change together.
- D9 The product side adds no rule surface. H checks are code in `internal/iacfacts` beside the existing ones. `iac` becomes a seventh review dimension with `Checked` and `Reason`, so a missing catalog is a stated absence (U8). Findings keep their existing path. No new MCP tool.
- D10 Waivers are not fathom's. Disposition belongs to the review kit (ADR-040 decision 1, F32). The reader emits inline suppressions as facts: tool, id, reason, expiry, span. That covers `checkov:skip`, `tfsec:ignore` and `trivy:ignore` with its expiry, workspace and attribute suffixes `[web]`.
- D11 Scanner findings do not enter through a new SARIF door. ADR-040 rejected SARIF as the interchange for three stated reasons and none has been answered. The review kit ingests scanners. Each scanner gets a producer entry with its own tier cap (ADR-088 decision 4). Fathom's contribution is the join: rank an imported finding by what the changed code reaches and what no test reads.
- D12 Code concerns (memory, lifetime, anti-patterns) are refused as engine work. They arrive as imported findings (D11) and get T4 ranking.
- D13 Well-Architected and compliance stay evidence production (ADR-088). A pillar review is a roll-up the policy tool builds from verdicts, with unknowns listed as questions for a person. Fathom never mitigates (ADR-089) and builds no transformation engine. A verdict may carry the satisfying condition as data.
- D14 Decision models: none applies (see the Decision models section).
- D15 The pre-write delta (base against proposed text, speaking only on a transition, value-free, inside the I35 320-token cap) is a labs card candidate, not a build row. ADR-091 decision 7 requires the card first, IF-M6 is deferred, and its bar is now external: lower noise than the post-write hooks in K3.
- D16 First uplift: Terraform and OpenTofu with the AWS provider, for change hazards. That is the one place the thesis is unrefuted (T1). CloudFormation is the calibrated twin: its change check exists, and CDK diff gives it the independent oracle the record says it lacks. Kubernetes second, Helm third.

## 7. Hazard classes and what fathom does with each

The concern map. The word "concern" already has four meanings here (layer stratum, concern packs, the tech pack key, the custodian file), so this design says "check" and "hazard" and adds no fifth (X6).

| Class | Product, value-free (H) | Policy tool, with kernel (V) | Refuse |
|---|---|---|---|
| Incident precursors | replacement, rename, key churn, protection touched, upgrade impact | "is it production" | root cause, mitigation |
| Contract misuse | deprecated, removed, literal on sensitive, sensitive to output | none | none |
| Security | none beyond contract | exposure, ingress, wildcard IAM | vulnerability and entropy scanning |
| Reliability | none | backups, multi-zone, probes, replicas | capacity modelling |
| Cost and waste | none | unbounded retention, orphans, old generations | pricing, usage |
| Seams, code to infra | none until typed links exist (I57) | none | heuristic links as gates |
| Memory, lifetime, anti-patterns | import ranking only (D11) | none | building analyzers (F90) |
| Well-Architected | evidence | pillar roll-up | attestation, scoring |

Extension per tool: a check gains a tool by gaining a binding and fixtures. A new tool needs only a reader that emits the common fact shape. The kernel does not change.

## 8. The first check set

Product hazards. All read class, contract and change. None reads a value.

| Id | Hazard | Facts | State today |
|---|---|---|---|
| H1 | Stateful resource replaced by this change | ChangeSet, replacement contract, stateful type list | CloudFormation built (IF-C4), unmeasured. Terraform is IF-13 then IF-14 |
| H2 | Terraform address renamed with no `moved` block | ChangeSet, `moved` blocks | Needs E3, E7. CloudFormation renames are unknown, since a refactor happens outside the template `[web]` |
| H3 | Protection attribute changed or removed on a stateful resource (`prevent_destroy`, `deletion_protection`, `DeletionPolicy`, `UpdateReplacePolicy`) | ChangeSet, span text | Needs E7, E12. Direction of change is a V check |
| H4 | `count` and `for_each` switched, or key expression changed, with no covering `moved` | Multiplicity, span text, `moved` | Needs E3, E7 |
| H5 | Provability lost: an attribute went from literal to expression or unknown | class transition | Needs E7. Advice only |
| H6 | Provider upgrade impact on attributes in use | binding, two catalog versions | IF-17, needs IF-6 |
| H7 | Deprecated attribute in use | contract | Built. Table stakes (K3) |
| H8 | Literal on a contract-sensitive attribute, value never read | class, sensitive flag | Terraform only. Needs the public version binding (E3). Not found in any tool, on a negative search |
| H9 | Sensitive value reaches an unmarked output | refs, outputs | Terraform built (IF-8). CloudFormation needs E12 |

Replacement semantics for H1: create-only is a lower bound. Conditional create-only is may-replace, which maps to unknown. A type with no update handler always replaces. Custom provider diff logic is in no schema.

Value checks. Bench fixtures for the kernel and seed content for the policy tool. Each has an oracle.

| Id | Check | Note |
|---|---|---|
| V1 | Bucket made public by explicit false, public ACL or wildcard principal | Absence is not a violation (N6) |
| V2 | Database or volume not encrypted | S3 excluded, encrypted by default |
| V3 | Ingress open to the world on admin or database ports | Static blocks readable today. `dynamic` content reads expression |
| V4 | No backup or point-in-time recovery | Absent semantics per type |
| V5 | Log group with no retention | Unknown reason: account policy outside the read set |
| V6 | IAM statement allows all actions on all resources | Needs E13 |
| V7 | Protection turned off (the direction H3 cannot see) | |

Second wave, named only: Kubernetes workloads without limits or probes, mutable tags, privileged containers. kube-linter already has six dangling-reference checks `[web]`, so the only new ground is the code seam: a probe path no served route matches.

## 9. Engine gaps

- E1 Unknown reasons and a read set on the result. Small, and D4 and D5 depend on it.
- E2 Class and span per member of a Terraform object or map.
- E3 Structured blocks in the public result (variables, outputs, locals, provider requirements, `moved`, `removed`, `import`) and a public binding resolver from lockfile to catalog version. Every contract check on Terraform needs the resolver.
- E4 A bounded pure-function tier (`jsonencode`, `merge`, `concat`, conditionals over literals). The function library is already linked. It changes the published class table, so it needs a design amendment.
- E5 Local module expansion. It breaks the public promise "never expands a module", so it arrives behind an option with a change note, and only if M4 ranks it first.
- E6 Instance expansion of `count` and `for_each`: refused. Verdicts are per declaration.
- E7 `ChangeSet` in the door, generalizing the CloudFormation diff that already exists.
- E8 Existing rows as prerequisites: IF-13, IF-14, IF-17, IF-6, IF-11. Do not duplicate them.
- E9 Enum, default and pattern in contracts where the source has them. CloudFormation registry schemas do `[web]`. For Terraform the only source is AWS SDK models, pending a licence check.
- E10 A plan JSON reader, bench only. Plan files hold provider facts and plain-text secrets, so product use would need I56 amended. Not proposed.
- E11 Kubernetes in the `iac` door with an OpenAPI-derived catalog. Helm with the same classes: template literal is literal, a `.Values` read with a default is defaulted, any other action is expression.
- E12 CloudFormation `DeletionPolicy`, `UpdateReplacePolicy`, `DependsOn`, `Mappings`, `Outputs`.
- E13 An IAM policy document model. F97 gates it on IF-M2 and a named adopter.

## 10. Measurement

All local. None needs a cloud account. Sample sizes are preregistered. Readings live under `docs/bench/readings/change-hazards/`.

- M1 Hazard precision by replay. For H1, H2 and H4 on Terraform, replay public history and compare with the plan of head over a state synthesized from base. Ground truth is a replace action in either order, with `replace_paths` and the action reason `[web]`. It needs fake credentials, provider skip flags and seeded state. State that cost and its bias in the reading.
- M2 CloudFormation H1 against `cdk diff --template`. This is the independent oracle IF-CM3 is waiting for.
- M3 Kernel soundness against the plan. A definite static verdict must equal the verdict on plan values. Bars: 1.000 on a literal basis, 0.99 on defaulted.
- M4 Decidability on a preregistered random sample stratified by module use, with unknowns counted by reason, head to head with Trivy's resolution. It ranks E2, E4 and E5 before any is built.
- M5 Mutation. Operators derive from the check: flip the literal, delete the attribute, swap in a defaulted variable, an undefaulted variable, a function call, a local. Each mutant has a predicted transition. A check is accepted at a 100% kill rate.
- M6 Agreement on V checks against labeled data: KICS positive and negative fixtures and IaC-Eval (458 Terraform AWS scenarios, CC-BY-4.0) `[web]`. Execute only, pinned by checksum, nothing copied in.
- M7 Helm: `helm template` as oracle, sound only for charts with no `lookup` and no capability branches. Those read unknown with a reason.
- M8 Agent outcome: deferred. The agent bench is not reliable enough to tune by. Do not gate on it.

## 11. Spin-offs

- S1 The library, with the kernel if D8 goes that way. Stay in the module. No ADR-092 trigger has fired and the product does not count as the outside consumer. Split on the recorded triggers. Name-free framing: a plan-free reader for infrastructure code that reports what it cannot know.
- S2 The contract catalog as data: versioned, digested contracts with replacement facts and upgrade diffs. No record addresses it. It is the earliest thing with outside value and it rides IF-6 and IF-11.
- S3 Test selection by what tests read: the standalone assessment (`docs/assessments/2026-11-04-standalone-capabilities.md`) routes it to the CI kit. Decide after the TQ-M7 reading.
- S4 Guards and log templates: not yet, triggers stated in the assessment. The verify gate: a recorded never.
- S5 Licence. ADR-052 accepted the current licence and rejected a permissive one, and none of its triggers has fired. A standalone library competes with Apache-2.0 and MIT tools, so at split time a permissive licence for the library and catalog format is worth a decision. It is a partial reversal of ADR-052, it sits under Q4, and outside check contributions also wait on Q29.

## 12. Record changes

Stated plainly: no revisit trigger has fired. These are owner reprioritizations, as the CloudFormation amendment to ADR-091 was.

- X1 ADR-063 O11 and ADR-092 decision 10: amend only if D8 is approved. Split "evaluator": evaluation semantics may live in the library. Rules, baselines, severity, pass or fail and crosswalks stay outside. The product still evaluates no resource rule.
- X2 Library design: additive surface (E1, E2, E3, E7, suppressions as facts). E4 and E5 each need their own amendment because they change published class and expansion promises.
- X3 `review_diff`: `iac` becomes a dimension. Output schema change, with a change note.
- X4 ADR-091: record CDK diff as the independent CloudFormation replacement oracle.
- X5 Keep, and cite as design constraints: ADR-067 decision 12, ADR-069 decision 5, ADR-040, ADR-088, ADR-089, ADR-092 decisions 7 and 8, ADR-011, ADR-052 until a split, F90, F97, F99, I35, I38, I56, I57, I62, I68, "no transformation engine", "CUE never an execution engine".
- X6 The word "concern" already has four meanings here (layer stratum, concern packs, the tech pack key, the custodian file). This design uses "check" and "hazard" and adds no fifth.

The decision record for this design is ADR-099. The plan flags where each record change lands.

## 13. Risks

- R1 An incumbent closes T1 in a release. Trivy has the parts except versioned replacement contracts. Mitigation: the catalog (S2) is the moat. Finish IF-13 and IF-6 first.
- R2 Replacement facts are wrong where providers compute replacement in code. Mitigation: provenance and class already exist. Conditional maps to unknown. M1 reports precision before any agent surface.
- R3 Replay ground truth needs synthesized state, which biases toward simple roots. Mitigation: say so.
- R4 The kernel grows into a language. Mitigation: closed enum, a new operator needs two checks that need it.
- R5 Decidability on real code is low and Trivy beats it. Then the honest advice to the policy tool is to take values from Trivy or a plan and take hazards and contracts from fathom.
- R6 Public surface grows on a door with no outside import (revisit 2027-05-04). Mitigation: D8 requires a written ask. Phase 1 surface is what that consumer needs regardless.
- R7 Several landscape rows remain `[prior]`: Terrascan and Datree status, licence wording, per-platform counts. Mitigation: HZ-0.

## 14. Phases and stop rules

The plan is `docs/plans/change-hazards.md`. Rows inside a phase are independent lanes and each names its gate. The candidate rows are HZ-0 to HZ-22, and the readings M1 to M7 of this design are HZ-M1 to HZ-M7.

Order: HZ-0 to HZ-2 need no new engine and measure what exists. HZ-3 to HZ-7 are additive facts. HZ-8 to HZ-12 are the product's bet. HZ-13 to HZ-16 and HZ-18 start only on the owner's written ask (OQ3). HZ-17, the decidability reading against Trivy, is scheduled as a reading lane. HZ-19 to HZ-22 follow their own gates.

Stop rules:

- If HZ-1 is under its floor, fix that before anything else.
- If HZ-9 precision is under 0.9, no agent surface for hazards.
- If HZ-17 shows Trivy decides far more at equal soundness, the kernel stays small and R5's advice stands.

## 15. Questions for the owner

The owner questions are numbered OQ3 and OQ4 in `docs/vision/46-proof-at-the-moment-of-change.md`. This design needs three answers. OQ3 carries the first two, and OQ4 carries the third.

- OQ3, part 1. The kernel (D8): in the library on your written ask, or in your policy tool with fathom shipping only conformance fixtures? Recommendation: in the library. Until the ask exists, HZ-13 to HZ-16 and HZ-18 do not start.
- OQ3, part 2. Is "the product never reads a value" (D1, ADR-092 decision 7) the line you want? It keeps the records intact and costs the product every V check.
- OQ4. The catalog as a published data artifact (S2): yes or no, and under what licence. Default if unanswered: the current licence stays and the catalog is not published.

Defaults if unanswered: the kernel is not built (HZ-13 to HZ-16 and HZ-18 stay closed), the line stays where ADR-092 decision 7 draws it, the product emits H checks only, and the catalog is not published.

## Decision models

None applies. The product's H checks are class, contract and change joins with nothing to judge. A model must not resolve an unknown or judge a security property (I62), and the fallback with provider `none` is the whole feature. Opting out means installing no catalog, which makes the `iac` dimension a stated absence. Drafting a check from prose is generation, outside the port's predicate, choice and score kinds, and it belongs to the policy tool as an authoring aid. If a judged site appears later it follows ADR-073 as written, with its own kill measurement.

## 16. Verdict

**Reshape, narrower than the ask. Reject the policy engine. Build the hazard engine.**

Rejected outright: a scanner, a rule corpus, rule packs in core, code analyzers for memory and anti-patterns, a model in the verdict path. The market critique was decisive. The class on every value is not unique, delivery at the edit is table stakes, and Trivy resolves more values than fathom does.

What survives is smaller and stronger. Fathom holds two things together that no scanner does: versioned provider contracts and a diff of classed facts. From those it can say, with no plan, no credentials and no value read, that a change replaces a database, renames a resource into a destroy, or breaks on the next provider version. That work is already specified as IF-13, IF-14 and IF-17, already built for CloudFormation, and unmeasured. Finishing and measuring it beats starting an evaluator.

The library's unused potential is facts, not rules: unknowns with reasons, read sets, change sets, structured blocks, suppressions, and the same classes for Kubernetes and Helm. A verdict kernel is worth having in the library, but it is the policy tool's instrument and it waits for the owner's ask (OQ3).

The bet in one line: **what will this change destroy, proven from contracts, with every unknown stated**. First proof: measure the CloudFormation check against CDK diff (HZ-1), then land the Terraform replay reading (HZ-9, HZ-M1). If hazard precision is under 0.9, stop and say so.

# Benchmark environments: validity before a verdict

Status: Proposed 2026-11-14 (architect ruling on owner direction).
Records: ADR-108 and ADR-109 drafts. Plan: `docs/plans/bench-environments.md` (ENV).
Decision codes: BV-D for validity, BE-D for environments. All thresholds below are proposals, not observations.
The ENV plan and validity assessment remain held for fresh-reading premise ownership and are not included in this integration.

## 1. Findings

- BE-F1 The proof program already owns capability cards, seven task families, six scripted readiness harnesses,
  statistics, the delivery log and proof liveness. Reuse BEN-1 to BEN-9 rather than build another driver
  (`docs/plans/proof-program.md`, BEN-1 to BEN-9).
- BE-F2 Preparation receipts are not admissions. The latest application screen retains Gotenberg and Slidev,
  carries pytest and redux-toolkit, and leaves formal admission incomplete
  (`docs/bench/readings/bench-prep/apps-r2.md:3`, table Selected candidates and rescreens).
- BE-F3 A useful fleet seed already exists. Its answer key includes positive and negative cross-service links
  (`internal/connector/testdata/fauxtown/links.answer.json:3`). The cost harness copies six named services and
  adds bulk functions. That exercises growth, not a labelled multi-repository task bank
  (`internal/mcp/costtest/fleet.go:49`, `BuildFleet`).
- BE-F4 Old invalid readings cannot establish a win or a refusal. The audit is
  `docs/assessments/2026-11-14-benchmark-validity.md`, Tables 1 and 2.

Intent before design: P6 requires stamps, P7 existence checks, P8 a consumer population and P12 a counterfactual
(`docs/intent/principles.md`, P6 to P12). I43 requires live evidence, I54 measured language levels, I56 operator-named
inputs and I62 keeps model judgment out of proof (`docs/intent/invariants.md`, I43, I54, I56, I62).
ADR-085 owns independent anchors, contamination and power. ADR-102 owns harness parity.
The owner direction reopens benchmark-only cuts when their evidence is invalid. This is an evidence trigger,
not evidence that any feature wins. Independent boundary reasons remain in force. Record changes belong to the architect.

## 2. Numbered design decisions

### BV-D1 Reading validity is a conjunction

A reading is VALID only when all required fields and receipts pass:

| Dimension | Required evidence |
|---|---|
| Product | Exact commit and build digest, extractor epochs, schema digest, dependency and catalog pins |
| Version bound | At most 30 calendar days old at claim time, and no relevant reach-set or input change since measurement |
| Wiring | WPF-4 scripted preflight proves W0 to W3 and W6 before the agent acts, including registration and a successful scripted surface call |
| Environment | Pinned admitted environment, independent concept witnesses and passing coverage matrix for this card and stratum |
| Trials | Preregistered effective task and repository counts met, with paired seeds and stopping rule satisfied |
| Instrument | No unresolved defect affecting the cell, graders pass stubs, lost and abandoned cells counted |
| Model | Exact resolved model id, harness version, profile and budget identical except for the declared treatment |

Thirty days is a conservative review bound proposed here, not an estimate of product stability.
A relevant change stales a reading immediately, even within thirty days. An unchanged old reading needs a re-read,
not an author's freshness assertion. A model-free arm records model id `none` and proves no model was invoked.
Unknown is a failed prerequisite, never an inferred pass. Keep all failure flags. Primary class precedence is
VOID (instrument, model pin or wiring), STALE (version), UNFIT (environment), UNDERPOWERED (trials), then VALID.
A native binary arm declares a native profile under WPF. Its registration and call are process and invocation
receipts, never an invented daemon connection. ADR-105's daemon-free exception removes daemon checks only.
Classify paired contrasts from both arms, never from a passing treatment alone.
Preserve correction 1 and BX-40 (`docs/design/bench-prep.md:36`, Scoring by assignment at line 371).
Wiring is an environment property. WPF-4 preflight runs before treatment behavior and cannot depend on agent use.
A failed preflight voids the whole pair. For old readings, require evidence of registration and reachability,
not evidence that the agent chose a tool. W4 use and W5 acted-on are outcomes, never exclusions.
Keep non-use cells under their assigned arms. The assigned-arm estimate is the headline and ruling basis.
Report low W4/W5 in the delivery funnel as findings against the profile. A use-conditioned secondary is labelled
"biased toward fathom: selects on tool use" and never supports a headline or a ruling.

### BV-D2 Evidence symmetry reopens unsupported conclusions

Wins, refusals, cuts, non-goals and claims that determinism loses need VALID readings for their empirical premise.
An independent licence, ownership, safety boundary, existing free tool or cost reason can sustain the bounded ruling.
A benchmark-only ruling without VALID support becomes UNTESTED, with an ENV re-test link and its old reading retained.
It becomes neither a proven refusal nor a default-on feature. The audit separates factual observations from causal claims.
No new verdict overwrites a historic ledger row. BEN-9 owns the liveness fingerprint and release claim check.

### BE-D1 Three tiers, separate estimates

1. Pinned public repositories provide external outcome evidence and public history replay.
2. A constructed multi-repository fleet provides deterministic seeded truth and scale experiments.
3. Fathom and the owner fleet provide smoke runs only. They never count toward evidence, power or the two-environment rule.

Synthetic and public effects are reported separately. A constructed win cannot replace public confirmation.
The smoke tier is not exported. Public receipts contain roles and public repository ids only, per ADR-011.

### BE-D2 Inventory concepts before selecting tasks

Counts apply separately within every claimed language, scale, predicted-effect and predicted-null stratum.
A positive is an independent task or site, not duplicated generated padding. Every negative is a matched absent or
safe case. These are proposed admission floors, separate from the power analysis that sets paid trial counts.
Every concept requires at least 30 independent positive and 30 matched negative witnesses per eligible environment
and claimed stratum. The table adds concept-specific requirements and never lowers that common floor.

| Concept | Minimum per eligible environment and stratum | Independent checker | Cards, families and existing work reused |
|---|---|---|---|
| False plan claims | 30 false and 30 true rows, including absent symbols and unwired entry points | seed manifest plus compiler and test oracle | verify, PLAN, BEN-5 |
| Cross-file flow | 30 paths of depth at least 3 across at least 3 product files, plus 30 disconnected controls | go/types, SCIP or native compiler | graph, ASK, LOC, V-N1, FCT-1 |
| Cross-repository dependencies | 30 witnessed links across at least 3 repos and 3 services, plus 30 non-links | lockfiles, protocol contracts and independent link key | fleet standards, ASK, library primitive 5 |
| Fan-in and dispatch | 3 symbols each with 5 product call sites in 3 outside units, plus 30 dispatch or re-export sites | reuse D26 recount and compiler receipts | impact, FIX, REV, V-N1 |
| Infrastructure hazards | 30 hazardous and 30 safe changes for each claimed tool and hazard kind | base state to head plan or native diff, offline fixtures | HAZ, HZ-1, HZ-9, BEN-10 |
| Modules, unknowns and upgrades | 30 module or dynamic expressions, 30 missing-context cases, 30 provider-version transitions | native evaluator and provider contract diffs | A-N1, A-N2, A-N6, HZ-17, LIB-14 |
| Stale instructions and rationale | 30 stale and 30 live anchored instructions, 30 explicit governing record links | seed key or person-confirmed public record anchors | advice, ASK T3, V-N3, ADR-080 |
| Real failures and mutants | 30 fail-before/pass-after public fixes and 30 collateral mutants with killed/survived labels | subject tests and fix commits, never index truth | FIX, REV, suspect set, CHG-4 |
| Localization without shared names | 30 non-lexical and 30 lexical issues with base files present | D-a token rule and fix key, D-b diagnostic | LOC, anchors, V-N1, V-N6 |
| Landing conflicts | 30 conflicting and 30 clean lane pairs with overlap witnesses | replay actual merges and gate logs | LANE, CHG-M1, CHG-7 |
| Replay history | 200 reachable commits and 30 labelled relevant changes | git parent graph and oracle receipts, no merge-message proxy | what-if, CHG-1, CHG-9 |
| Generated and polyglot code | 30 generated/handwritten boundary sites, at least 3 languages with 30 sites each | generator inputs, compiler or native format oracles | V-N9, in-band outlines, library primitive 8 |
| Data and subprocess test reads | 30 runtime data reads and 30 subprocess reads, plus 30 uncovered changes | attested file-read logs and deliberate test failures | tests selected by reads, ADR-090 |
| Resource lifetime | 30 cross-function acquire/release pairs, 30 missing-release fixes, 30 escapes | SSA plus public parent/fixed tests and incumbent linters | A-N3, RL-M0, RL-M1 |
| Growth and shutdown bounds | 30 bounded and 30 unbounded reachable sites, 30 spawn/join or timer-loop cases | independent audit plus runtime leak profiles | RL-M2, RL-5, RL-6 |
| Shared read maps and orientation | 30 repeated multi-session tasks with independent gold anchors | transcript reads and held-out T2 anchors | shared map, XM-M2, V-N2, FTU |
| Guardrails and drafted rules | 30 declared and 30 undeclared deletions, 30 independent labels per rule kind | pipeline execution, incident fix and derived mutants | CHG-9 to CHG-12, V-N4, A-N5 |
| Pins, contracts and inventories | 30 pinned and 30 floating external inputs, 30 interface changes | lock parsers, native API diff and catalog versions | library primitives 2 to 4, U-21, U-25 |
| Attested foreign inputs | 30 valid and 30 stale/mismatched envelopes per claimed input format | producer output, commit and age checks | V-N8, V-N9, FCT-6, RL-1 |
| Runtime/account identity | 30 resolvable and 30 ambiguous bindings, with observed and declared values separated | operator-placed runtime export | A-N4, A-N6, RL-10, RL-11 |
| Output fidelity and judgments | 30 long/short outputs, 30 route/rank/table questions with person-confirmed anchors | lossless byte check, native evaluation or frozen gold key | V-N5, V-N7, ENV-R3, ENV-R4 |

Coverage closure joins BEN-3's entire capability catalog, BN-D4's PLAN/HAZ/FIX/LOC/REV/LANE/ASK,
proof-program section 4's eleven cards and added guardrail/lifetime cards, library primitives 1 to 8 and RL-1 to RL-12,
and every V-N/A-N target of `docs/design/beyond-determinism.md`, section 3.
A capability absent from this inventory fails closure until its required concepts and checkers are declared.
A future target can have a requirements row without being implemented or scheduled. Do not build runtime/account
joins, a kernel or other deferred features merely to fill the matrix.

### BE-D3 Adequacy is computed

ENV-1 defines typed requirements and ENV-2 implements the leaf coverage checker. ENV-C6 joins it to BEN-5 admission.
Reuse `LoadSubjectManifests` and `SplitHeldOut` (`internal/benchprep/manifest.go:80`, `internal/benchprep/split.go:160`).
Read independent receipts. Count unique anchors, deduplicate fix descendants and clones, verify pins, and emit
missing-concept and missing-count reasons for each card/environment/stratum. Unknown counts fail closed.
Negative fixtures must fail admission when a concept is removed, a count is one short, or a pin is changed.
Only passing matrix cells may enter trials. Empty or wholly ineligible claims read UNFIT, never zero effect.

### BE-D4 Scale is a vector with explicit claims

| Axis | S | M | L | XL |
|---|---|---|---|---|
| Tracked product source lines | 5,000 to 49,999 | 50,000 to 249,999 | 250,000 to 999,999 | 1,000,000 or more |
| Tracked product files | 50 to 499 | 500 to 2,499 | 2,500 to 9,999 | 10,000 or more |
| Repositories | 1 | 2 to 4 | 5 to 12 | 13 or more |
| Reachable history commits | 200 to 999 | 1,000 to 4,999 | 5,000 to 19,999 | 20,000 or more |

Below-floor subjects are tiny and make no S claim. Count lines/files excluding vendored, generated, test and build
output, with those exclusions and generated counts reported separately. Count reachable unique commits, not shallow
checkout length. A card declares its vector and claimed axes. No scalar maximum hides a weak axis.
Require two independent eligible environments per claimed class on every claimed axis, and two public repositories
for a public outcome claim. Replicas and different pins of one repository are one environment.
Report unsupported classes as untested and narrow the claim rather than filling them with smoke subjects.
Existing C2's 150,000-line ceiling conflicts with L/XL admission (`docs/bench/readings/bench-prep/apps-r2.md`, grype/pip rows).
Propose replacing that upper ceiling with this vector plus measured resource caps. Preserve C1 and C3 to C7.
No historic environment is retrospectively admitted under the amendment.

### BE-D5 Public portfolio and constructed fleet

The following twelve receipt-backed candidates span Go, Python and TypeScript. Size is the old primary-language
line count at the cited pin, not a new measurement under BE-D4. Licences are recorded receipt values.
Full pins and licence-blob digests stay in the cited JSON subject records. Every row remains a candidate.
Concept fits and new class assignments are hypotheses until ENV admission checks them.

| ENV row | Public repository | Recorded licence | Recorded lines / target line class | Proposed concepts and known admission issue | Receipt |
|---|---|---|---|---|---|
| ENV-A1 | gotenberg/gotenberg | MIT | 44,765 / S | service paths, resource lifetime, FIX. Preparation kept, formal admission incomplete | `docs/bench/readings/bench-prep/apps-r2.json`, subject gotenberg--gotenberg |
| ENV-A2 | slidevjs/slidev | MIT | 29,266 / S | generated/polyglot flow, orientation. Dev only from prior reads | `calibration/bench.d/slidevjs--slidev.yaml:2`, `apps-r2.md`, Slidev row |
| ENV-A3 | charmbracelet/soft-serve | MIT | 31,687 / S | service calls, auth boundaries, LANE. Existing C3/C5 refusal retained | `docs/bench/readings/bench-prep/apps-r1.json`, subject charmbracelet--soft-serve |
| ENV-A4 | pdm-project/pdm | MIT | 41,788 / S | dependencies and config, FIX. Existing C3 refusal retained | same JSON, subject pdm-project--pdm |
| ENV-A5 | changesets/changesets | MIT | 32,576 / S | package contracts and upgrades. Existing C3/C5 refusal retained | same JSON, subject changesets--changesets |
| ENV-A6 | derailed/k9s | Apache-2.0 | 75,704 / M | resource sites and config. Timing flake must be repaired by a new upstream pin | `docs/bench/readings/bench-prep/apps-r2.md`, k9s row. Licence: apps-r1.json, subject derailed--k9s |
| ENV-A7 | goreleaser/goreleaser | MIT | 104,755 / M | build inputs and packaging. Setup cap failure remains a refusal | same reading, GoReleaser row. Licence: apps-r1.json, subject goreleaser--goreleaser |
| ENV-A8 | restic/restic | BSD-2-Clause | 88,548 / M | cross-function lifetime and FIX. Missing preparation dependencies remain unresolved | `docs/bench/readings/bench-prep/apps-r2.json`, subject restic--restic |
| ENV-A9 | pypa/hatch | MIT | 75,649 / M | dependency/config contracts. No root lock and insufficient fix surrogate count | `docs/bench/readings/bench-prep/apps-r1.json`, subject pypa--hatch |
| ENV-A10 | python-poetry/poetry | MIT | 81,877 / M | dependency resolution and LOC. Setup exit remains unresolved | same JSON, subject python-poetry--poetry, and apps-r2.md Poetry row |
| ENV-A11 | sqlfluff/sqlfluff | MIT | 198,917 / M | polyglot/config reading. Old C2 refused it, new admission still required | same JSON, subject sqlfluff--sqlfluff |
| ENV-A12 | verdaccio/verdaccio | MIT | 84,769 / M | several packages, generated interfaces and failures. Existing login failures unresolved | same JSON, subject verdaccio--verdaccio, and apps-r2.md Verdaccio row |

Additional gap candidates below have no inspected checkout in this audit. Every licence, pin, size, file count,
history depth and claimed concept is **unknown, candidate, unverified**. The target class and purpose are proposals.
This roster is a search order, not permission to fetch or a statement that these projects pass existing admission.

| ENV row | Public candidate | Proposed target and purpose |
|---|---|---|
| ENV-A13 | opentofu/opentofu | L lines, Terraform module/evaluator and replacement-history questions |
| ENV-A14 | aws/aws-cdk | L lines, CloudFormation native-diff and generated/polyglot questions |
| ENV-A15 | kubernetes/kubernetes | XL lines/files, long-history and Go resource-lifetime questions |
| ENV-A16 | rust-lang/rust-analyzer | L lines, Rust typed graph and generated-code questions |
| ENV-A17 | spring-projects/spring-petclinic | S lines, Java service/config questions |
| ENV-A18 | llvm/llvm-project | XL lines/files, C/C++ cross-file flow and deep-history questions |

No L/XL pair, infrastructure pair or non-Go typed class is admitted today. Failure to find two fits narrows the claim.
Each admission row records pass or refusal and its cause within a day. Expensive setup becomes a separately approved
follow-up rather than an unbounded lane. Do not patch subject tests or relax a bar to retain a candidate.

ENV-3 extends fauxtown with a standalone deterministic generator, keeping existing fixture bytes and answer keys
compatible. Reuse `BuildFleet`'s copying and bulk-generation pattern without importing the MCP test harness into
the product (`internal/mcp/costtest/fleet.go`, BuildFleet).
Given seed, version and size vector, emit independent git repositories, protocol contracts, commit DAGs, false plan
rows, stale instructions, generated files, labelled fixes, lane conflicts, hazards and lifetime sites.
Truth is written from the seed manifest before ingest, then checked by native compilers, tests and evaluators.
Fathom's index never supplies truth. Include safe/unsafe and reachable/unreachable twins. Mutations remove each
concept independently. ENV-S1 and ENV-S2 admit development and held-out constructed topologies separately.
The held-out family uses different graph topologies and templates, with its seeds and keys sealed for confirmation.
Changing the random seed alone creates a replica, not another independent environment.
Generator padding never counts as extra tasks.

### BE-D6 Baseline parity and contamination

The headline control is an agent that already searches and reads with grep, runs native tests and has proven wiring.
Use the same exact model id, harness release, tool permissions, context, token/dollar/wall caps, environment pin and task
order policy. Paired seeds and interleaving are BEN-6's work. Ablations compare wired with wired minus one card.
ENV-5 proves parity and consumes ADR-110 receipts. A harness description is not a baseline proof.
Record the resolved model's documented cutoff as known or unknown. Prefer eligible fixes committed after it, using
committer dates, and publish before/after-cutoff strata. Unknown cutoff supports no post-cutoff claim.
Reuse prior-read exclusions and repository splits. These receipt-backed subjects were inspected during preparation,
so default them to development. A controller may seal fresh, unaudited repositories for held-out use.
Keep oracle keys out of agent mounts. Shakedown tasks leave the confirmatory set. Holdout repositories are read once.
A re-read or tuning access demotes the whole repository to development and requires a replacement before confirmation.

### BE-D7 Wiring has one owner

ADR-110 and `docs/design/wiring-proof.md` / `docs/plans/wiring-proof.md` (WPF) own conformance and profile experiments.
ENV consumes W0-W3/W6 preflight verdicts and separate W4/W5 outcome fields. No duplicate ladder or stamp implementation.
WPF-4 supplies the common receipt gate, with WPF-10 integrating scenario admission. ENV-5 depends on WPF-4 and
remains blocked until its receipt interface exists.

### BE-D8 Schedule from 2026-11-16

Free proofs start now, on November 14. Week numbers start on November 16, not at the first paid run.

| Week | Dates | Work and exit |
|---|---|---|
| W1 | November 16 to 22 | $0 begins immediately: schema contracts, audit import, independent concept counts, candidate admission and seeded fleet work. Use existing HAZ/LANE/native oracle scripts now where prerequisites exist. Start BEN-4 delivery logging and WPF experiments in their owning lanes |
| W2 | November 23 to 29 | Finish passing matrix cells and baseline parity. Run six BEN-2 scripted harnesses and free fact/delivery proofs. Review ENG-1 deployed proof and RT-M15. Rejected admissions remain rejected |
| W3 | November 30 to December 6 | Earliest paid A/A shakedown, only if every gate below passes and an approved spend row exists. Publish the power table, sign card preregistrations, then PLAN first. A missing gate slips the date, never the bar |
| W4 | December 7 to 13 | Earliest powered first-card confirmation after sizing, development tuning and held-out repository sealing. If delivery needs longer accumulation, remain on free work |

Paid gate list: ENG-1 deployed (`realignment.md#ENG-1`), engine presence/RT-M15 for daemon arms (`#ENG-5`),
all six BN-D12 harnesses (`BEN-2`), delivery log (`BEN-4`) with this card's funnel passing (`BEN-M1`),
admitted environments and passing coverage matrix (`ENV-2`, relevant ENV-A rows, ENV-4), per-arm ADR-110 conformance
(`ENV-5`), exact pins, independent oracle/grade checks, signed preregistration and an approved capped spend row.
BEN-10/11 keep the daemon-free exception for RT-M15 and daemon stamps, not environment or native-arm conformance.
ENG-1 remains the owner-directed paid scheduling prerequisite. This is stricter than the old daemon-free schedule.
Do not wait for ENG-2, every capability, every portfolio candidate, the full library roadmap, an outside adopter,
the record-syntax trial or a combined run. No paid run is authorized by this schedule.

### BE-D9 Typed data, derived validity

Extend typed-records CU-D2 data containers and BEN-3 capability records. ENV-1 defines CUE environment heads,
concept requirements, witnesses and card/environment/stratum coverage records. A head includes immutable id,
public repo URL and full commit, licence digest, toolchain and image pins, source policy, scale vector, split,
prior reads, oracle ids and receipt digests. Requirements declare minima, positive/null strata and checker version.
Reading heads reference the environment and matrix digests, exact model/harness/product pins, n, defects and WPF receipt.
Authored requirements and measured receipts are inputs. Adequacy, validity and freshness are computed at HEAD,
never authored as a passing status. The evaluator receives its clock. No scheduler or automatic downloads are added.

## 3. Risks

- BE-R1 Two environments per class may be unavailable. Publish the unsupported class and narrow the claim.
- BE-R2 Seeded truth flatters the implementation. Public confirmation stays separate and independently labelled.
- BE-R3 Thirty-day validity can queue costly re-reads. Withdraw stale claims and batch approved measurements, never waive age.
- BE-R4 Prepared candidates cannot supply a clean holdout. Seal fresh repository identities and report the gap.
- BE-R5 The roster repeats old setup failures. Bound each admission lane and retain refusals without calling them feature losses.

## 4. Sequence

Land ENV-1 contracts, then independent ENV-2 checker and ENV-3 generator APIs. ENV-C0 owns only dispatch.
ENV-C1 to ENV-C8 separately own fixtures, portfolio, baseline, audit, seed, candidate, seal and fresh-reading modes.
ENV-C9 owns native snapshot replay and ENV-C10 owns the capped model-run adapter.
Each consumer needs its mode implementation explicitly.
The generator has no checker dependency. ENV-C5 performs their joined negative-twin proof, avoiding a hidden cycle.
ENV-G1 to ENV-G7 own the separate concept presets, keeping generator implementation within one lane-day.
ENV-4 publishes the portfolio coverage matrix. ENV-6 applies the audit without editing historical readings.
Each re-test has a fresh measurement owner. Existing open BEN, FTU, CHG, HZ and RL reading rows supply fresh
results where explicitly named in needs. ENV-R3, ENV-R9, ENV-R10, ENV-R14, ENV-R17 and ENV-R18 own bounded free reruns.
ENV-R4/5/6 and other model reruns each need their own ENV-P preregistration and ENV-B capped spend receipt,
BEN-M7 sizing and BE-D8 gates. Historic VOID receipts stay audit inputs, never re-test inputs.

## 5. Decision models

No judged site decides validity, adequacy, a primary label or a ruling. Requirements, receipts and thresholds are
computed deterministically with provider `none`. Opt-out is complete because this design invokes no model.
Paid agents are experimental subjects under BEN-6, not graders. ADR-085's optional secondary grader stays secondary,
with its existing calibration and opt-out. Failure or absence of that grader never changes a primary result.

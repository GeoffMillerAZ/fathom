# The proof program: prove each capability alone, then together, then tune, and never let proof go stale

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Decision: ADR-101 (one proof program), with ADR-102
(every harness measured) and invariant I74 (a reading names its inputs). Plan: `docs/plans/proof-program.md` (BEN-).
Builds on ADR-031 to ADR-034, ADR-080, ADR-082, ADR-085. Sources it merges: `docs/bench/README.md`,
`docs/bench/scenarios-ledger.jsonl`, `docs/design/bench-pyramid.md`, `docs/design/bench-prep.md`, `docs/design/feature-labs.md`.

Codes: BN-F finding, BN-K kept practice, BN-D decision, BN-R risk. Public benchmarks (BN-D17, BN-D18) added 2026-11-12 with the library roadmap (`docs/plans/library-roadmap.md`). Owner questions are OQ rows in
`docs/vision/46-proof-at-the-moment-of-change.md`. The sequence is the last section.

Environment admission is proposed in [bench-environments.md](bench-environments.md). Its ENV plan and validity assessment remain held for fresh-reading premise coverage.
Wiring conformance is proposed in [wiring-proof.md](wiring-proof.md), with [WPF work orders](../plans/wiring-proof.md). Scripted preflight and assigned-arm analysis preserve correction 1 and BX-40.

## 1. Findings

- BN-F1 The free layers are excellent. Index answers are graded against go/types in the gate. Language reads are graded
  against the tools' own evaluators.
- BN-F2 The paid layer has not produced a number anyone could tune by. The scenario ledger holds 893 rows and about $486
  of spend. No row shows fathom ahead of grep on the registered criterion. About $183 sits on rows flagged with an
  instrument defect.
- BN-F3 The causes are instrument and design debt. 660 of 893 rows have two trials. The model is recorded as an alias.
  In the first round the wired arm ran with no tools connected. Verdicts flipped on re-read several times.
- BN-F4 The comparison is too coarse. "All of fathom against grep" mixes a proven gate, unproven advice and a cold engine.
  A null does not say which part failed.
- BN-F5 The engine contaminated the readings. Deadline abandons inflate tokens and failed cells.
- BN-F6 The confirmatory layer is unbuilt: three-outcome classification, a power table, a results renderer, the lab
  harness and its ledger. No subject is admitted. No preregistration is signed for the two primary hypotheses.
- BN-F7 Delivery cannot be replayed from what is logged today. Hook rows do not record the target they cited, the holdout
  log is unbuilt, and the hook log keeps about a week.
- BN-F8 Readings go stale in silence. A reading carries no fingerprint of what it measured.
- BN-F9 Bench code is about 120,000 lines with tests across six commands. Two programs proposed on the same day overlap,
  and nothing says how per-feature results combine.
- BN-F10 About 310 open plan rows are readings. Most are runnable and unassigned.
- BN-F11 Outside results set the bar: savings against a naive baseline are worthless, and structure has both helped and
  hurt agents depending on how it is delivered. Report success and tokens, per repository, against an agent that already
  greps.

## 2. Practices to keep

- BN-K1 Preregistration, with predicted wins and predicted no-effects written before spend.
- BN-K2 Oracles fathom does not own. Truth never comes from fathom's index.
- BN-K3 No model judge on any primary metric.
- BN-K4 Losses and nulls are published.
- BN-K5 No pilot is cited before its second reading.
- BN-K6 An incomplete run never reports a win.
- BN-K7 Cell keys and borrowed control cells.
- BN-K8 Firing is never counted as helping.
- BN-K9 A zero-effect holdout reopens policy. It never auto-disables a channel (ADR-080).

## 3. The design

### BN-D1 The unit is a capability card

A card is a CUE record: surface keys (verb, tool, hook), the claim in one sentence, the stratum where an effect is
predicted, the stratum where none is, and three proofs. A shipped surface with no card fails a closure check.

### BN-D2 Three proofs, in order, each gating the next

| Proof | Question | Cost | Oracle | Bar before the next |
|---|---|---|---|---|
| Fact proof | Is the answer right? | $0, in the gate | an outside tool or compiler, on a pinned corpus and a holdout | precision and recall per card |
| Delivery proof | Does it reach the agent at the right moment, in time, with something relevant? | $0 once logging exists | eligible, fired, delivered in deadline, acted on (the agent touches the cited file, symbol or test within k calls), against the base rate with the capability held out | a preregistered funnel per card |
| Outcome proof | Does the task end better? | paid, paired | tests, fix commits, rows false by construction, a plan | preregistered per card |

Delivery proof is forward logging, not replay of the past. Three things must be built first: the deterministic holdout
log ADR-080 designed, the cited target on every hook row, and an archive for the hook log. Readings then accumulate
weekly. Where old sessions do carry enough (failing test runs, sub-agent reads), replay is used and labelled.

No outcome spend is approved for a card whose delivery proof is under its bar. This is where pushed advice would have
been stopped for free.

### BN-D3 Outcome proof is an ablation inside the wired arm

For a card, the comparison is the product against the product minus that capability, on the stratum where the card
predicts an effect. The whole-product comparison against an agent with grep stays as one headline arm per release.

Outcomes, ordered by how well determinism can move them:

1. False-done: tasks the agent declared finished that fail the oracle.
2. Collateral damage: unrelated tests broken, conserved surface lost.
3. Task success.
4. Tokens to green and cost per success.
5. Minutes per landing.

### BN-D4 The task bank comes from history, with oracles built in

| Family | Task | Oracle | Proves |
|---|---|---|---|
| PLAN | finish a plan where some rows are false by construction | which rows the agent claims | false-done, the gate |
| HAZ | an infrastructure change with a real replacement, rename or upgrade hazard, from public history | the plan of head over a state built from base, `cdk diff` | the hazard engine, at $0 |
| FIX | reproduce a real fix from the commit before it | the fix's own tests | success, tokens |
| LOC | find the file when the issue shares no identifier with it | the fix commit | impact, in-band anchors |
| REV | review a change with seeded mutants | the mutant list | review evidence |
| LANE | replay two real lanes from merge history | did they conflict, and where | overlap forecast, at $0 |
| ASK | questions from the named-query set, with keys from a compiler or SCIP index | anchor match | question shapes, the thesis arm |

Subjects are public repositories admitted by the existing rules, applications before libraries, with a development
split and a held-out split. Fathom's own repository is a smoke run. The public benchmarks of ADR-085 judge the
understanding claim.

### BN-D5 Arms

- Per card: wired, and wired minus the card.
- Headline: wired against grep, once per release.
- In-band: wired plus anchors on the agent's own search and read results, against wired. This is the arm that tests case 2
  of the rule in `docs/vision/46-proof-at-the-moment-of-change.md` section 4.
- Thesis: the named-query answers against a query-time model tool, on the ASK family. Accuracy by anchor match, cost per
  question, and agreement between two runs. It adds conceptual questions with a hand key beside the enumerate-all questions.
- Review family: the same model with and without the evidence pack, reported side by side.
- Advice card: reports the share of prose firings that were compiled to checks (CHG-10).
- Every harness that builds fathom is an arm (ADR-102).

### BN-D6 Statistics that survive small n

- Paired by task, clustered by repository. Three outcomes: confirmed, refuted, inconclusive. Equivalence tests for
  predicted nulls. Holm across preregistered primaries.
- A noise floor first: one arm against itself. No effect under the floor is reported.
- One shakedown gives a power table that sets n. Runs are sequential with a preregistered stopping rule.
- Pins on every cell: exact model id, harness version, fathom commit, tool schema bytes.
- A validity gate per cell: a wire stamp, the surface called at least once in the wired arm, abandon rate under its bar. A
  failing cell is void.

### BN-D7 Together

When at least three cards have outcome proof, one combined run: all on, all off, and all on minus each card whose own
effect cleared the noise floor. The run is priced from the power table before it is approved. The interaction estimate has
several times the variance of a single delta, so it is reported as a finding to investigate, never as a headline. A full
factorial is refused on cost.

### BN-D8 Tuning

- Every knob is data: knob, card, stratum, metric, value, the reading that set it.
- Tune on the development split, on delivery proof first because it is free. Confirm once on held-out data. A second read
  of a held-out split demotes it.
- A text knob (a tool description, a stanza line) is a knob like any other. Firing is never the target.

### BN-D9 Proof liveness

Every reading is a record with an input fingerprint: corpus digest, fathom commit, the extractor epoch of the dialects it
used, harness and model pins, and the reach set of the capability. The evaluator derives:

- live: the fingerprint matches.
- stale: a commit changed something in the reach set, or a pin moved.
- void: a validity gate failed after the fact.

Free readings re-run in the gate when stale. Paid readings queue with a cost estimate. A release note or a guide may not
state a claim whose reading is stale. This is certificate liveness (invariant I43) applied to every number fathom
publishes, and it is invariant I74.

### BN-D10 The flywheel

1. An observation is filed, and can be closed with a verdict (fix, wontfix, duplicate, ceiling).
2. A fix carries its own red-to-green test commit.
3. The card's stale readings re-run.
4. The card's condition updates on the fleet view.

### BN-D11 One program

- The strengths bench, bench prep, feature labs, the context bench and the review bench merge. Labs become the delivery
  layer. Bench prep becomes the outcome layer. The pyramid's free layers are fact proof.
- One bench command outside the product binary. One ledger schema with a unique key per cell.
- The home bench stops being a source of claims.
- The ambient A/B, if the owner agrees (OQ6), is succeeded by ADR-080's deterministic holdouts, which measure production
  effect where paired cells cannot.

### BN-D12 Readiness before spend

No paid cell runs until the scripted harnesses pass: null, oracle, wrong patch, flake, scripted wired (surface called in
every wired cell), scripted grep. They are deterministic and would have saved the void spend.

### BN-D13 Reading lanes

The 310 open measurement rows are staffed as lanes. A reading lane runs one preregistered measurement and files it with
its verdict line.

### BN-D17 Hazard replay as a public benchmark (BEN-10)

The HAZ family becomes a public benchmark. It is the one infrastructure row with a consumer, an outside oracle and no competitor.

| Part | Rule |
|---|---|
| Subjects | public infrastructure history, admitted by the existing rules (BEN-5) |
| Labels | the plan of head over a state synthesized from base for Terraform, `cdk diff` for CloudFormation. Provider-bump commits label upgrade breaks. Published with the method |
| Stated bias | synthesized state favours simple roots. The labels and every reading say so |
| Arms | fathom through the library binary (it never calls the daemon), scanners, and a model reviewer |
| Gates | ENTRY: HZ-9 at its reading, and BEN-5. The scanner arms and the fathom arm are free. The model arm needs the paid gate, minus RT-M15 and the wire stamp, which a daemon-free reading is excused from |
| Preregistration | signed before any model arm. A zero or negative result is published (BN-K4) |

Scanners are a pair, not a rival: the benchmark states what each decides, and fathom's claim is only on class, contract and change hazards.

### BN-D18 A reading benchmark (BEN-11)

A second test of the thesis on ground fathom does not own: a model's reading of a configuration or infrastructure file, graded
against the tool's own evaluator. Three rules, without which it would test exhaustive listing, where an index wins by construction:

1. The questions are preregistered, and are questions an agent needs.
2. Only native-tier oracles grade (the tool's own evaluator).
3. No oracle is written by fathom's authors.

ENTRY: BEN-5, the paid gate, and the fixtures and oracles of library row LIB-14. Its fathom arm is the library's reading. It
never calls the daemon, so it is excused from RT-M15 and the wire stamp and still needs the readiness harnesses, a signed
preregistration and an approved spend row.

## 4. Card order

| Order | Card | First proof to run | Cost |
|---|---|---|---|
| 1 | Verify gate | PLAN family, false-done | first paid dollar |
| 2 | Hazard checks | HAZ replay against a plan and `cdk diff` | $0 |
| 3 | Landing bundle and overlap | LANE replay, minutes per landing from gate logs | $0 |
| 4 | Graph answers on the hook path | RT-M15 | $0 |
| 5 | Tests selected by reads | the 14-day boundary reading, once its open row lands | $0 |
| 6 | Suspect set on a red test | delivery logging, then its preregistered arm | $0, then paid |
| 7a | What-if verdicts, delta tier | replay cards, after the hook prerequisites (CHG-8) | $0, then paid |
| 7b | What-if verdicts, reach tier | replay cards, after CHG-8 and RT-M15 | $0, then paid |
| 8 | Shared read map | its sub-agent arm (XM-M2) | paid |
| 9 | Question shapes and `why` | the public understanding benchmarks, the thesis arm | paid |
| 10 | In-band anchors | replay, then the LOC arm | $0, then paid |
| 11 | Post-edit advice | AD-M1, then the holdout reading AD-M4 | $0 |

Two cards are added: guardrail deletion (CHG-9, a replay over history and a hand audit) and the lifetime rows (RL-M0 to RL-M2 of
the library roadmap, each compared with the incumbent). Both cost nothing for the first proof. Eight of the eleven first proofs
cost nothing, and so do the first proofs of the added cards.

## 5. Decision models

None on any primary metric (BN-K3). One existing candidate: a secondary grader on the understanding bench, off by default,
valid only at an agreement of 0.6 or better with person labels. Opt-out: leave the provider at `none`. Kill measurement:
the agreement against person labels falls under 0.6. With provider none the primary anchor match stands alone.

The model arms of BEN-10 and BEN-11 are subjects of the benchmark, never graders: oracles stay a plan, `cdk diff` and the tools'
own evaluators. Opt-out: run the scanner and fathom arms only. Kill measurement: a model arm whose spend row is not approved does
not run, and with no answer to OQ10 the hazard benchmark runs scanner arms only and the reading benchmark does not run.

## 6. Decisions

- BN-D1 to BN-D13 as above.
- BN-D14 Two gates. Free steps start at the H0 exit. Paid steps need a wire stamp, the readiness harnesses, RT-M15
  passing, a signed preregistration and an approved spend row.
- BN-D15 Budgets are per card, approved by row, with the cap in the driver.
- BN-D16 The admission test's fourth clause applies to fathom's own programs. Each names an oracle fathom does not own.
- BN-D17 and BN-D18 as in section 3. A daemon-free reading is excused from RT-M15 and the wire stamp, and from nothing else.

## 7. Risks

- BN-R1 The program becomes the product again. Mitigation: the merge deletes code, and the driver is frozen while the
  first cards read.
- BN-R2 The acted-on proxy is noisy. Mitigation: always against a held-out base rate.
- BN-R3 Small public subject pools. Mitigation: applications first, and inconclusive is a reported result.
- BN-R4 Harness releases move results. Mitigation: pins, and control cells re-run on a pin change.
- BN-R5 Proof liveness floods the queue. Mitigation: reach sets are per card, and paid re-runs batch per release.
- BN-R6 Delivery logging takes weeks to accumulate. Accepted. It starts in wave 1 so it is ready when the cards are.
- BN-R7 The headline arm reads null. The ablation still says which cards help on which strata.
- BN-R8 The hazard benchmark's synthesized state favours simple roots. Mitigation: the bias is stated on the labels and in every reading.
- BN-R9 The reading benchmark turns into an exhaustive-listing test where an index wins by construction. Mitigation: the three rules of BN-D18.

## 8. The sequence

1. Merge the programs and freeze the driver. Build the readiness harnesses. Start the delivery log. $0.
2. Cards and fact proof for every shipped surface. Closure check on. $0.
3. The $0 first proofs of section 4, as reading lanes. $0.
3b. The public benchmarks (BN-D17, BN-D18): scanner and fathom arms at $0 once HZ-9 reads. The model arms wait on the paid gate and OQ10.
4. Shakedown: noise floor and power table (OQ10: the architect recommends $120 for the shakedown, and with no answer there is no paid spend).
5. Outcome proof for the first three cards, PLAN family first.
6. The combined run, priced, when three cards have passed.
7. Tune on development data, confirm on held-out, record the knob map.
8. Proof liveness on, release claims bound to live readings.

Steps 1 to 3 cost nothing, need no ruling, and would have predicted most of the paid nulls on the ledger.

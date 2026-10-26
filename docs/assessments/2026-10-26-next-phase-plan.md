# Next phase: land what is built, make the bench honest in code, then prove fathom helps

**Verdict.** Fathom cannot yet prove that it helps a coding agent. Every claim is either self-graded on fathom's own
repository or unmeasured for six weeks:
- The newest home-bench row (2026-09-14, arm `full` against `grep`, 13 tasks) fails H2-3. Successes are 41 against 44,
  with a success-difference CI of [-0.19, +0.03] and an aggregate token ratio of 1.81×.
- The flywheel A/B (G2) has 0 admissible ON-arm units.

The plan runs in this order:
1. Land the built work, and first the accepted P0 fixes that would otherwise contaminate every measurement.
2. Move the bench's honesty rules from prose into code.
3. Run a small number of preregistered experiments whose predictions are written down here.

Fan-out workers keep doing most of the building. Their scheduling stays outside fathom (ADR-078).

Evidence caveat: the daemon timed out at host load 44 while this was read, so plan counts come from the plan files,
not from `fathom verify`. Work order NP-W2 re-derives them with `fathom plans status --store` on a scratch ingest.

## 1. What binds this plan

- **P12:** no savings claim stands without a counterfactual. Any controlled arm counts as one: the flywheel A/B, the
  bench arms, or a lab's plugin-minus-one arm.
- **ADR-078 (D5):** fathom does not schedule, place or monitor work across machines.
- **ADR-079 d2:** GDEV-01 to GDEV-08 land first, merged with the deadline-abandon work (O1014, O1446).
- **ADR-073:** decision models (section 9). **ADR-082:** the feature-labs arms and bars.
- **The 2026-10-26 spend lift** (`docs/intent/autonomy.md`): spend goes first to harness development, then a no-claim
  shakedown, then preregistered runs.
- **LAB-R1:** no model grades a primary metric. **EP-43:** no machine-written K0 annotation.

**Records that are stale, or triggers that have fired**
- **G7:** its Done-when ("either outcome closes the goal") is met, and the goal is still listed. Remove it.
- **G9:** met with a FAIL, which is the outcome the bet allowed. BP-41 to BP-43 are proven, and BP-44 records payload
  2.66× with H2-3 failing. Close G9, keep BP-44 ticked, and open a successor goal (NP-Q6).
- **RT-M1:** read FAIL a second time (`docs/bench/readings/tenancy/rt-m1-2026-10-26.md`). Refuse minutes were 140 to
  265 a day against a 60-minute bar. Fathom caused 0 of them; host swap caused all of them. The reading's own question
  is open (NP-Q1), and its preregistered floor-change row should ship and restart the window.
  - Development fan-out adds to the same host load, so the operator's local lane caps are tied to the RT-M1 window.
- **Elapsed review dates:** G2, G3, G4, G9, G11, G12 and G13 were due 2026-10-25, and G14 2026-10-26. No review record
  was found.
- **Stale spend-hold text** in G36 and G37. The 2026-10-26 lift supersedes it.
- **`docs/intent/scope/boundaries.md:39`** says no CI runner exists. Linux CI runs `task validate` on every push.
- **PR #45** (oracle-call scope, held for an HLSL replay) reached main inside #61's history.
  - SQL and CUDA are cleared: their producers declare no scope, and a no-scope test plus a stored-count regrade show
    no change.
  - The [public HLSL replay](https://github.com/GeoffMillerAZ/fathom/actions/runs/40407275332) returned on Ubuntu 24.04 with pinned DXC v1.9.2602: call precision .9656 -> .956, recall .9859 unchanged, and L1/L2/L3 grades unchanged. The fresh reading and [scope assessment](../bench/readings/oracle-call-scope-compatibility.md) record the result.
  - The integration rules now include a stacked-PR check (section 7).

## 2. Priorities

| Id | Work | Why first | Exit test |
|---|---|---|---|
| NP-P0 | Land the built work: the open drafts #72 to #84, GDEV-01 to GDEV-08 with O1014 and O1446, and the slices left from the game-development contribution series (of its 31 inputs, 20 are already on main in another form, 8 are adapted in slices, 1 lands unchanged and 2 are refused: `docs/assessments/2026-10-26-gdev-series-triage.md`) | Deadline abandons inflate tokens and failed cells, so every measurement before this fix would measure the bug | Rows proven by `fathom verify`; issue #9 closed; each series patch landed, superseded or refused with a reason |
| NP-P1 | Bench honesty in code (section 3) | Without it every number is prose | BX-M0 and BX-M1 exits met |
| NP-P2 | The flywheel A/B, OP-1 (G2), run as registered or under a dated amendment made before session 1 | The only experiment measured in real working sessions | At least 20 admissible units per arm, then keep or retract |
| NP-P3 | Responsible tenant: the RT-M1 ruling, the floor-change row, and the O1014 wave | Every table, cache and daemon-backed seam waits on it | RT-M1 PASS, or the ruled bar |
| NP-P4 | Feature labs: the LAB-37 no-claim shakedown, then K3 | Prices and powers every later experiment | Shakedown rows in the labs ledger; BX-70 sizing computed |
| NP-P5 | The test-run hook (G27, TR-M1) | The most direct "fathom helps" experiment | At least 20% fewer tokens-to-green with the paired CI excluding 0, or a published FAIL with the hook off |
| NP-P6 | Exploration maps (G30, XM-M2), after AD-M2 confirms delivery | XM-M1 read BUILD, and the slice is shipped | At least 25% fewer sub-agent tokens, or the shipped slice is cut |

Language work continues inside its share (section 8). Board honesty comes first: GAP1-1 and GAP1-2 (the board names
file kinds nothing dispatches), and the D19 evidence floor (#7) enforced in the harness.

## 3. Honest benchmarking: work orders bound to existing rows

Each order cites the bench-prep row (BX-) that `fathom verify` reads. A change of exit test is an amendment to
`docs/plans/bench-prep.md`, never a parallel list.

- **NP-B1, preregistration (BX-62, BX-63).** `LoadPrereg` and `ClassifyOutcome`, with one file per run under
  `docs/bench/prereg/`. The evaluator refuses a run whose prereg commit is newer than its first cell, or whose prereg
  hash differs from the hash on its row.
- **NP-B2, holdouts (BX-54, BX-29).** `SplitHeldOut` and `RecordHeldOutRead`. The 13-task home corpus is declared a dev set,
  and a second read of a held-out split is refused.
- **NP-B3, independent graders.** Grade with each subject's own tests, or with truth authored outside fathom.
  - An import test proves that no grader imports a fathom engine package.
  - A fixture that dumps every file must fail the grader.
- **NP-B4, exclusion budget (BX-85, in its existing Python harness).** Skips are counted. A run that excludes more than
  2% of `suite_tests` reads `indeterminate`.
- **NP-B5, pins and a clean tree (BX-14).** Every row carries the resolved model id, CLI version and binary hash. A row
  with the `sonnet` alias or `dirty_tree: true` is refused.
- **NP-B6, spend and schedule (BX-15, BX-16, BX-57, BX-17).** A spend cap that stops before the next cell, interleaved arm order, counted turn-cap
  stops, and a rule-based lost-cell classifier with no model.
- **NP-B7, isolation (BX-13, BX-55, BX-84, BX-86).** Egress denial and unprivileged native-arch preparation with a
  `BENCH_UID=` witness.
  - Agent cells run on helper Macs under the BX-74 lane script, never as a product job.
  - Linux CI runs only $0 fixture tests of the refusal logic, with no API keys in the public repository's Actions.
- **NP-B8, subjects (BX-32, BX-3, open in #73).** `LoadSubjectManifests` and `VetSubject`. Close today's admission
  shortfall (6 Go, 5 Python, 4 TypeScript subjects), with C1 to C7 receipts committed.
- **NP-B9, ledger key.** `docs/bench/ledger.jsonl` keys rows by `(run_id, criterion_id)`, enforced by a uniqueness
  test.
  - The two rows for run 20260913T002832Z are not duplicates. They score H2-1 and H2-3, both FAIL, and both stay.
  - Nothing is deleted.
- **NP-B10, nulls stay public.** "Where fathom stands" is already rendered from the ledger (`RenderVerdict`). Add a
  README-equals-render drift test, and an age banner when the newest row is older than 30 days.
- **NP-B11, a readout at every release.** Each bench layer's newest row, its age, its verdict and its `cost_usd` go into
  the tag's notes, or the notes say "unmeasured since <date>".
- **NP-B12, power (BX-26, BX-60, BX-70, BX-71).** `PowerTable` and `CheckArmParity`, fed by the shakedown. An unpowered prereg is
  refused.

The full third-party head-to-head (A3 against A1) waits for NP-B1 to NP-B8.

## 4. The experiment loop

- **Registry.** One prereg file per experiment (NP-B1). It holds:
  - the hypothesis, and a prediction with direction and size;
  - the arms, the unit, and n from the power table;
  - the instrument and the primary metric;
  - keep and kill thresholds, taken from the goal's own Done-when;
  - a spend cap, the grader, and a review date.
- **Ledger and verdicts.** Results go to the one labs ledger (`AppendLabResult`, `CheckLabClosure`). A verdict file's
  first line is read by `fathom verify`'s ledger oracle.
- **Arms.**
  - In a lab, the helped arm (the plugin against the plugin minus one feature, ADR-082 d5) is required.
  - "Fathom plus normal tools against normal tools" is bench prep's separate A3-against-A1 question.
- **Sample size.** $0 replays first. Then the shakedown sets n. An experiment that cannot reach n within its cap is
  declared unpowered and not run.
- **Cadence.** At most three live experiments, reviewed inside the weekly ADR-074 bounded review. Every experiment that
  closed in the cycle appears in the release readout.
- **Graders.** No model grades a primary metric. Human labels are the real bottleneck: one owner cannot be two blind
  labelers (NP-Q3).

**First experiments, with predictions committed now**

- **NP-E1, OP-1 flywheel A/B (`docs/plans/expert-suite-p.md` §5).**
  - **The registered criterion** is lower median tokens per session at equal-or-better self-rated outcome, across at
    least 20 sessions per arm, with retraction after 40.
  - **Its weaknesses.** A bare median comparison passes about half the time when there is no effect. The owner rates
    outcomes while knowing the arm. Twenty consecutive OFF sessions confound the arm with the week's work and the
    binary version.
  - **Recommended amendment**, dated before session 1:
    - arms assigned per session by `hash(salt, session, target, ordinal) mod 2` (ADR-080 d6);
    - the fathom binary pinned for the window, or the analysis stratified by version;
    - a CI on the per-session log-token ratio;
    - a checkable outcome (a commit landed with the gate green) in place of self-rating;
    - the 40-session kill kept.
  - **Prior, from paired estimates.**
    - Lookups (vs-pilot-6): paired token ratio 1.20× [0.76×, 1.75×].
    - Home bench 09-14 (arm `full`): aggregate token ratio 1.81×, median 2.97×.
  - **Prediction:** no token saving at equal outcome. P(keep) about 0.3.
  - **On a FAIL:** every savings claim is retracted, and fathom's case rests on proof, meaning the verify gate and
    receipts, where it already leads.
- **NP-E2, LAB-37 no-claim shakedown** (2 labs, 10 cases, 3 runs). Prediction: a discordant rate between 10% and 25%,
  and under $5 a run. Its output is a price and BX-70 sizing, not a verdict on fathom.
- **NP-E3, XM-M2.** Read AD-M2 first, or check per cell that the map id reached the sub-agent payload. Otherwise a null
  could mean the map was never delivered. Prediction: a saving of 10% to 20%, below the 25% bar. On a FAIL, cut the
  shipped slice.
- **NP-E4, K3 (tool and skill text).** It needs LAB-7 and LAB-9 (LAB-8 to LAB-11 are in #76). Prediction: changing the
  text moves verb choice on eligible turns by at least 15 points, with task success unchanged.

## 5. Spend

Every row records `cost_usd`.

- **Under the 2026-10-26 lift, now:** development sessions that build the bench and labs harnesses, then the NP-E2
  shakedown.
- **Claim runs** (NP-E1 sessions excepted, which are ordinary owner work) wait for NP-B1 to NP-B8.
- **Cell runs that need an approval line** (`docs/intent/autonomy.md`): TR-M1 ($90 to $360) and PR-M2 (about $76).

## 6. Owner questions

| Id | Question | Recommendation |
|---|---|---|
| NP-Q1 | RT-M1 asks whether the table freeze should gate on the tenant clauses alone. | Yes. Fathom caused 0 refuse minutes, so host swap should not freeze fathom's tables. Ship the floor-change row and restart the window. |
| NP-Q2 | Run OP-1 as registered, or under the amendment in NP-E1? | Amend before session 1. The registered criterion cannot tell a real effect from noise. |
| NP-Q3 | Labels: answer feature-labs Q4 (about 1,600 points labelled by the owner), or accept counts-only metrics? | Counts-only for the first labs. Label only K1's 40 points per class. |
| NP-Q4 | Existing question Q4: is fathom a force multiplier for its owner, or a product for others? | It decides what G2 is for. NP-E1 runs either way. |
| NP-Q5 | Language lanes: keep 5, or drop to 2 once in-flight items land? | Drop to 2 after NP-P0, and put the freed capacity into NP-B. |
| NP-Q6 | Open G44 (bench prep: exit BX-M0 and BX-M1) and G45 (labs: exit K1 to K6, kill 2027-01-31) through `intent-decide`? | Yes. Neither program has a goal today, and G42 is closed. |

## 7. Fan-out: what lives where

Lane placement, machine caps, load and disk watching, and critic assignment stay in the operator's private tooling,
outside fathom (ADR-078) and out of this public repository. The repository-level rules that any adopter's waves need
go into `docs/guides/agent-waves.md`:
- **Stacked work.** Before a wave, check whether any open PR head is an ancestor of another PR head. Stop if a held or
  unreviewed PR is carried. This is how #45 reached main.
- **ENGINE changes** land one per wave, with a regrade of every reading they can move.
- **Findings are relayed by the report's own count**, never by grepping headings. One blocker (7 tests red against
  main) was missed that way.
- **Merge commits carry an exact `Verified:` line:** clean, or the files resolved.
- **Cross-controller holds** go on the lane-protocol issue (#11). Local gates take `scripts/gate-lease.sh`.

## 8. Capacity split

| Share | Work |
|---|---|
| 40% | NP-B (bench honesty) and NP-E (experiments, labs harness) |
| 30% | NP-P0 and NP-P3 (P0 fixes, tenancy), NP-P5 subject, receipts wave 5 (G11) |
| 20% | Languages: board honesty and D19 first, then the gap list ranked by reach |
| 10% | Records hygiene: section 1's stale records, flaky tests, the ledger |

Revisit the split once `instrument --adoption` reads. If adopters' repositories need languages that are not covered,
the language share grows.

## 9. The first two weeks

| Id | Work | Lane | Exit | Needs | Spend |
|---|---|---|---|---|---|
| NP-W1 | Land wave D (#78, #79, #80, #82), the other controller's batch (#72, bench round two, plan flips), then v0.23.0 | local integrator | Linux CI green on the tag | none | $0 |
| NP-W2 | Records refresh: remove G7, close G9 met-with-FAIL, fix the spend text and boundaries line, hold the overdue reviews, re-derive counts with `--store` | local | `intent-doctor` clean, and one review entry per goal | NP-Q6 | $0 |
| NP-W3 | GDEV-01 to GDEV-08 with O1014 and O1446; the series' adapt slices | the other controller, ENGINE serialized | Rows proven; #9 closed | NP-W1 | $0 |
| NP-W4 | NP-B9 ledger key and NP-B10 drift test | helper | Uniqueness and drift tests green | none | $0 |
| NP-W5 | NP-B1 and NP-B5 | helper, Opus critic | Refusal fixtures pass | none | $0 |
| NP-W6 | NP-B6 | helper | Over-cap and interleave fixtures pass | none | $0 |
| NP-W7 | NP-B2, plus NP-B8 finishing #73 | helper, Opus critic | Second held-out read refused; first subject admitted | NP-W5 | $0 |
| NP-W8 | NP-B3 and NP-B4 | local, Opus critic | Import test and the dump fixture's failure | NP-W5 | $0 |
| NP-W9 | NP-B7 on helper Macs | helper | An egress attempt fails inside a cell; refusal fixtures run in CI at $0 | none | $0 |
| NP-W10 | LAB-7 decoder, then the NP-E2 shakedown | helper, then local | Shakedown rows plus BX-70 sizing | NP-W5, NP-W6, #76 | under the lift, capped at $75 |
| NP-W11 | NP-E1: OP-1 sessions under the answered NP-Q2 | owner sessions | At least 20 admissible units per arm | NP-Q2 | owner's ordinary sessions |
| NP-W12 | NP-E3 prereg after the AD-M2 read | local | Prereg committed and powered | NP-W5, NP-W10 | $0 |
| NP-W13 | Board honesty (GAP1-1, GAP1-2) and the D19 floor | helper language lane | Truthfulness test green; Kustomize re-read under the floor | #7 | $0 |
| NP-W14 | TR-M1 Go mutation-seeded subject (20 tasks) | helper | Subject passes C1 to C7 | NP-W7, NP-W9 | $0 to build; the run needs approval |

## 10. Decision models (ADR-073)

- **LAB-D6, the residue predicate.** Absent by default. It is built only if LAB-M1a shows more than 20% residue, and
  kept only at a Wilson lower bound of at least 0.90 against the K1 labels (ADR-082). LAB-9 stays deterministic.
- **Lost-cell classification:** no model. A model deciding exclusions is a hole in grader independence.
- **Grading:** no model on any primary metric (LAB-R1).
- **Lane placement and critic tier:** not a fathom question (ADR-078).

## 11. What not to do

- Do not run a claim run before NP-B1 to NP-B8, and do not spend expiring credit on claims. Spend it on harness
  development and the shakedown.
- Do not delete or reword a failed result. Nulls and FAILs stay in the ledger and in the README.
- Do not let a model grade or label a primary metric, and do not fake K0 annotations.
- Do not build scheduling, placement or load reading into fathom.
- Do not tag over red CI, and run one gate at a time.

**Specced, not implemented:** NP-B1 to NP-B12 (bench-prep rows), the labs harness rows, NP-W3's GDEV rows.
**Planned, not specced:** a fan-out instrument (lanes, rounds, outcome, wall time) in the operator's lane tooling.
Until it reads, "fan-out saved time" is a claim.

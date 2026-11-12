# The proof program: each capability proven alone, then together, with proof that cannot go stale

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Grammar v2 (`docs/plans/README.md`). Prefix: BEN-
(BEN-n work, BEN-Mn readings). Design: `docs/design/proof-program.md` (section codes BN-D, BN-R are its). Decision: ADR-101.
Related: ADR-031 to ADR-034, ADR-080, ADR-082, ADR-085, ADR-102, invariant I74.

Goal: each capability proven alone, then together, with proof that cannot go stale in silence. Done when the design's
sequence (section 8) is complete. The first paid dollar goes to false-done on tasks with rows that are false by
construction. This program deletes a bench program: the merge removes code (BN-R1).

Two gates (BN-D14). Free steps start at the H0 exit. Paid steps need a wire stamp, the readiness harnesses (BEN-2),
RT-M15 passing, a signed preregistration and an approved spend row, with the cap in the driver (BN-D15). Readings live
under `docs/bench/readings/proof-program/`, one file per row, each opening with a verdict line. A row flips only through
`fathom verify --service fathom --plan docs/plans/proof-program.md`. Every row is open.

Two exceptions to the paid gate: BEN-10 and BEN-11 are public benchmarks whose model arms never call the daemon, so those arms
are excused from RT-M15 and the wire stamp (`realignment.md#ENG-5`, `realignment.md#WIR-1`). They still need the readiness
harnesses (BEN-2), a signed preregistration and an approved spend row (OQ10). Their scanner and fathom arms cost nothing.

Readings that `docs/plans/realignment.md` also names are owned there and recorded here once. BEN-M3, BEN-M4, BEN-M6, BEN-M8 and
BEN-M9 write the proof program's ledger entry for a verdict that the realignment row reads, and wait on that row. No reading is
taken twice. Each paid row waits on `realignment.md#ENG-5` (RT-M15 passing) and `realignment.md#WIR-1` (a wire stamp).

## Phase 1: one program, readiness, cards, delivery log ($0)

- [ ] One bench command outside the product binary, one ledger schema, a unique key per cell, and the old driver frozen {id: BEN-1, symbol: CellKey, package: internal/benchledger}
- [ ] Readiness harnesses: null, oracle, wrong patch, flake, scripted wired (surface called in every wired cell), scripted grep. All six pass before any paid cell {id: BEN-2, symbol: RunReadiness, package: internal/benchready, needs: BEN-1}
- [ ] Capability cards as a CUE record (surface keys, one-sentence claim, predicted-effect stratum, predicted-null stratum, three proofs) and a closure check: a shipped surface with no card fails {id: BEN-3, symbol: CheckCardClosure, package: internal/capability, needs: BEN-1}
- [ ] Delivery proof by forward logging: the deterministic holdout log ADR-080 designed, the cited target id on every hook row, and an archive for the hook log. This plan owns the log, the wiring plan reads it {id: BEN-4, symbol: DeliveryFunnel, package: internal/labs, needs: BEN-1}
- [ ] (op) Weekly delivery funnel reading per card: eligible, fired, delivered in deadline, acted on within k calls, against the held-out base rate {id: BEN-M1, oracle: lexical, path: docs/bench/readings/proof-program/m1-delivery-funnel.md, pattern: "^BEN-M1 verdict:", needs: BEN-4}

## Phase 2: the $0 first proofs, as reading lanes

- [ ] (op) Hazard checks: HAZ replay against a plan and `cdk diff` {id: BEN-M2, oracle: lexical, path: docs/bench/readings/proof-program/m2-hazard-replay.md, pattern: "^BEN-M2 verdict:", needs: BEN-3}
- [ ] (op) Landing bundle and overlap: LANE replay, minutes per landing from gate logs. Records the verdict of CHG-M1 {id: BEN-M3, oracle: lexical, path: docs/bench/readings/proof-program/m3-lane-replay.md, pattern: "^BEN-M3 verdict:", needs: "BEN-3,realignment.md#CHG-M1"}
- [ ] (op) Graph answers on the hook path, which records the RT-M15 verdict that ENG-5 reads, and tests selected by reads (the 14-day boundary reading, once its open row lands) {id: BEN-M4, oracle: lexical, path: docs/bench/readings/proof-program/m4-hook-path-and-tests.md, pattern: "^BEN-M4 verdict:", needs: "BEN-3,realignment.md#ENG-5"}
- [ ] (op) Replay cards for suspect set on a red test, what-if verdicts and in-band anchors, labelled as replay where old sessions carry enough {id: BEN-M5, oracle: lexical, path: docs/bench/readings/proof-program/m5-replay-cards.md, pattern: "^BEN-M5 verdict:", needs: "BEN-3,BEN-M1"}
- [ ] (op) Post-edit advice: AD-M1, then the holdout reading AD-M4, which WIR-8 reads. This row records its verdict {id: BEN-M6, oracle: lexical, path: docs/bench/readings/proof-program/m6-post-edit-advice.md, pattern: "^BEN-M6 verdict:", needs: "BEN-M1,realignment.md#WIR-8"}

## Phase 3: subjects, task bank, statistics

- [ ] Subject admission rules per subject (applications before libraries, development and held-out splits) and the task bank of seven families: PLAN, HAZ, FIX, LOC, REV, LANE, ASK, each with an oracle fathom does not own. Gains false-by-construction rows for configuration dialects (development split only) and a lane of its own, since the benchmarks below wait on it {id: BEN-5, symbol: AdmitSubject, package: internal/benchledger, needs: BEN-1}
- [ ] Statistics: three outcomes (confirmed, refuted, inconclusive), equivalence tests for predicted nulls, Holm across primaries, a noise floor, a power table, and pins on every cell {id: BEN-6, symbol: ClassifyOutcome, package: internal/benchstats, needs: BEN-1}
- [ ] (op) A/A reading: one arm against itself sets the noise floor, and one shakedown gives the power table that sets n (spend cap per OQ10) {id: BEN-M7, oracle: lexical, path: docs/bench/readings/proof-program/m7-noise-floor.md, pattern: "^BEN-M7 verdict:", needs: "BEN-2,BEN-5,BEN-6,realignment.md#ENG-5,realignment.md#WIR-1"}

## Phase 4: outcome proof, together, liveness

- [ ] (op) Outcome proof for the first three cards, PLAN family first (verify gate, false-done), preregistered, each card as wired against wired minus the card {id: BEN-7, oracle: lexical, path: docs/bench/readings/proof-program/outcome-first-three.md, pattern: "^BEN-7 verdict:", needs: "BEN-M7,BEN-M1,BEN-M2,BEN-M3,realignment.md#ENG-5,realignment.md#WIR-1"}
- [ ] (op) Combined run priced from the power table: all on, all off, all on minus each card whose effect cleared the noise floor. The interaction estimate is a finding to investigate, never a headline {id: BEN-8, oracle: lexical, path: docs/bench/readings/proof-program/combined-run.md, pattern: "^BEN-8 verdict:", needs: "BEN-7,realignment.md#ENG-5,realignment.md#WIR-1"}
- [ ] Proof liveness: every reading carries an input fingerprint (corpus digest, fathom commit, extractor epochs, pins, reach set) and derives live, stale or void. A hazard fact also carries the catalog digest and version it was computed against (FCT-8). A stale reading blocks a claim line in a release note or guide {id: BEN-9, symbol: ReadingLiveness, package: internal/benchledger, needs: "BEN-1,BEN-3"}
- [ ] (op) Tuning: every knob as data (knob, card, stratum, metric, value, the reading that set it), tuned on development data and confirmed once on held-out data. The knob map is TUN-1 and the held-out confirmation is TUN-3, read there. This row records their verdict {id: BEN-M8, oracle: lexical, path: docs/bench/readings/proof-program/m8-knob-map.md, pattern: "^BEN-M8 verdict:", needs: "BEN-8,BEN-9,realignment.md#TUN-3"}
- [ ] (op) The public understanding benchmarks and the thesis arm on the ASK family (ADR-085), read as UND-M1 and UND-M2. This row records their verdict {id: BEN-M9, oracle: lexical, path: docs/bench/readings/proof-program/m9-understanding.md, pattern: "^BEN-M9 verdict:", needs: "BEN-5,BEN-6,BEN-M7,realignment.md#UND-M2,realignment.md#ENG-5,realignment.md#WIR-1"}

## Phase 5: public benchmarks on ground fathom does not own

- [ ] (op) Hazard replay as a public benchmark (the HAZ family made public): public infrastructure history labelled by a plan, three kinds of arm (fathom through the library binary, scanners, and a model reviewer), and the labels published with the bias of synthesized state stated. ENTRY: HZ-9 at its reading, and BEN-5 for subject admission. The scanner arms and the fathom arm cost nothing. The model arm needs the paid gate and never calls the daemon. Preregistered {id: BEN-10, oracle: lexical, path: docs/bench/readings/proof-program/ben-10-hazard-benchmark.md, pattern: "^BEN-10 verdict:", needs: "BEN-1,BEN-2,BEN-5,change-hazards.md#HZ-9,change-hazards.md#HZ-10,library-roadmap.md#LIB-7"}
- [ ] (op) A reading benchmark: a model's reading of a configuration or infrastructure file graded against the tool's own evaluator, under three rules: preregistered questions an agent needs, native-tier oracles only, and no oracle written by fathom's authors. ENTRY: BEN-5, the paid gate, and the fixtures and oracles of library row LIB-14. Preregistered {id: BEN-11, oracle: lexical, path: docs/bench/readings/proof-program/ben-11-reading-benchmark.md, pattern: "^BEN-11 verdict:", needs: "BEN-2,BEN-5,library-roadmap.md#LIB-14"}

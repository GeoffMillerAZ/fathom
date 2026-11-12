# Realignment: horizons and programs, ordered by dependency

Status: Proposed 2026-11-12 (architect ruling, owner review pending).

Grammar v2 (`docs/plans/README.md`). Prefixes: ENG- (engine presence), CHG- (change landing, the non-hazard half of proof at the
moment of change), RLN- (realignment records), WIR- (wiring), CUT- (subtraction), LIB- (library), FCT- (facts, wider), UND-
(understanding and organization), INB- (in-band structure), TUN- (default-on and tuning). ENG-Mn, UND-Mn and CHG-M1 are readings. Design:
`docs/vision/46-proof-at-the-moment-of-change.md` (its section 14 is the order below). Records: ADR-093 to ADR-104. This file
replaces the dated amendments to `docs/vision/38-proof-layer-roadmap.md` as the operative schedule (RM-D1).

Every row below is open. Symbols and packages name what the change will create, and the integrator binds each row to a symbol
that is new in its change. Rows owned by other plans are named by id and never copied: the hazard rows are `HZ-n` in
`docs/plans/change-hazards.md`, the typed-record rows are `REC-n` in `docs/plans/typed-records.md`, the proof-program rows are
`BEN-n` in `docs/plans/proof-program.md`, and the fleet rows are `FLT-n` in `docs/plans/fleet-picture.md`. Readings live under
`docs/bench/readings/realignment/`, one file per row, each opening with a verdict line. A row flips only through
`fathom verify --service fathom --plan docs/plans/realignment.md`.

Owner questions (OQ1 to OQ14) live in vision 46, section 12, each with its default if unanswered. This file names the ones that
gate a row.

## 1. The order, and where it differs from the ask

The ask: add capabilities, prove them, then wire and tune.

The order keeps that shape and moves two things forward.

- **Engine presence first.** Graph-backed answers are declined on the hook path. Every capability rides the same graph, and a
  measurement taken on it measures the fault. The fix is specified and small.
- **Plumbing for the proof channels from the start.** A wire stamp per session and parity for the claim gate on every harness
  that builds fathom. Tuning and default-on stay last.

Four programs are the core: PG-1 engine presence, PG-2 typed records, PG-3 proof at the moment of change, PG-4 the proof
program. The other eight start when their entry condition holds.

## 2. Horizons, by event

| Horizon | Waves | Exit events |
|---|---|---|
| H0 Realign and unblock | 0a, 0b, 0c | (1) the realignment records are landed and OQ7, OQ9 and OQ11 are answered or defaulted. OQ5 gates only the rows that need the freeze amendment, and OQ6 only the preference cuts. (2) RT-195 and RT-197 are deployed. (3) The hook-path harness is re-read. (4) Purge wave 1 is proven gone. (5) The lane kind, id allocation and the path check exist. (6) Wave 1 lanes have contracts and failing proofs. (7) A release tag contains the language library. |
| H1 Core capabilities | 1, 2 | each core program's done-when |
| H2 Proof | 3 and the paid readings | capability cards read, the first combined run, the understanding bench verdict |
| H3 Default-on, tuning, outside use | later | V-M1 to V-M8 of vision 46 |

No date is set for H0. Its limit is attention and rulings, not commit throughput: the engine fix has sat specified for seventeen
days and main has had no commit since 2026-11-06. Existing kill dates stay as anchors: 2027-01-31 (the old `ask` spine, mods,
concern-fact waves, labs), 2027-03-31 (the understanding program), 2027-05-04 (the library door sunset).

Two gates for proof work, stated once:

- Free steps (fact proofs, forward logging, readiness harnesses) start at the H0 exit.
- Paid steps need all of: a wire stamp on every cell, the readiness harnesses passing, RT-M15 reading PASS, a signed
  preregistration and an approved spend row.

## 3. Core programs

### PG-1 Engine presence

- Goal: graph-backed answers arrive inside the hook deadline, and every miss is named.
- The bar, stated once. First: RT-M15 reads PASS (at least half of hook-path graph calls answered, evictions without re-warm at
  most 10%), and pre-edit `blast_radius` itself answers at least half, since the existing bar can pass on `verify_plan` alone.
  Then: a seven-day reading on the build fleet with each hook-path tool reported apart, under 1% abandoned for every one
  including `obligations`, and the answered share raised by reading toward V-M2.
- Where it starts: the committed harness reading has 0% abandoned, 48.5% answered and `blast_radius` declined 50 of 50.
- Done when: the bar above is met, ENG-M1 reads PASS.
- Proof: the hook-path harness, the call log, the harness's own hook timeouts as the outside witness.
- Stop: per-RAM-class fit is RT-M10's question. No second store is added.
- Entry: none. ENG-1 and ENG-2 wait on no ruling (RM-D3). ENG-3, ENG-4 and ENG-6 need the freeze amendment (OQ5, ADR-095).
- Deletes (RM-D5): a patch path that differed from a rebuild (`Graph.ReplacePackage`, through RT-197).

The existing rows RT-195 to RT-199 carry the contracts. The rows below add the deploy and reading proof around them. Reading of
the defect: `docs/bench/readings/tenancy/graph-patch-2026-10-26.md`, design `docs/design/responsible-tenant.md` section 18.

- [ ] (op) RT-195 deployed: index constructor-scoped calls. A cold build goes from 17 to 25 seconds to under one, read on the deployed binary. Not frozen {id: ENG-1, oracle: lexical, path: docs/bench/readings/realignment/eng-1-cold-build.md, pattern: "^ENG-1 verdict:", needs: "responsible-tenant.md#RT-195"}
- [ ] (op) RT-197 deployed: a package patch that equals a rebuild, read as a patched snapshot equal to a fresh one on the deployed binary {id: ENG-2, oracle: lexical, path: docs/bench/readings/realignment/eng-2-patch-equals-rebuild.md, pattern: "^ENG-2 verdict:", needs: "responsible-tenant.md#RT-197,ENG-1"}
- [ ] (op) Bound abandonment of `obligations` on the edit hook (1,370 of 16,654 calls in 30 days). Find whether the cause is admission, the tree read or a cold claim load, then bound it. Proof: abandoned share per cause {id: ENG-11, oracle: lexical, path: docs/bench/readings/realignment/eng-11-obligations-abandon.md, pattern: "^ENG-11 verdict:"}
- [ ] (op) A fresh seven-day read of the tenant clauses, so the freeze amendment is argued from a current reading {id: ENG-M2, oracle: lexical, path: docs/bench/readings/tenancy/clauses-week-2026-11.md, pattern: "^ENG-M2 verdict:"}
- [ ] The freeze amendment in code: a table, cache or job without a cost row fails the lint {id: ENG-3, symbol: lintTenancyCostRow, package: internal/tenancy, needs: "ENG-M2"}
- [ ] (op) RT-196 deployed (re-warm what an eviction removed) and RT-199 landed (the wording that amends ADR-041 decision 3). The code rows stay in the tenancy plan, which waits on RT-M1 until ADR-095 is accepted and then on ENG-3 and ENG-M2. This row reads the deploy {id: ENG-4, oracle: lexical, path: docs/bench/readings/realignment/eng-4-rewarm-deployed.md, pattern: "^ENG-4 verdict:", needs: "ENG-3,responsible-tenant.md#RT-196,responsible-tenant.md#RT-199"}
- [ ] (op) A tenant-clause week after ENG-4 deploys, as the revert check, with the re-warm counters read {id: ENG-M3, oracle: lexical, path: docs/bench/readings/tenancy/rewarm-revert-check.md, pattern: "^ENG-M3 verdict:", needs: "ENG-4"}
- [ ] (op) RT-M15 under scripted commit traffic: at least ten watermark moves of the measured service during the hook-path harness run {id: ENG-5, oracle: lexical, path: docs/bench/readings/realignment/eng-5-rt-m15-scripted.md, pattern: "^ENG-5 verdict:", needs: "ENG-4"}
- [ ] (op) RT-198 row-level patch, built only if RT-M15 reads under half answered (its own kill clause K8). The reading records built or refused {id: ENG-6, oracle: lexical, path: docs/bench/readings/realignment/eng-6-row-patch.md, pattern: "^ENG-6 verdict:", needs: "ENG-5,responsible-tenant.md#RT-198"}
- [ ] Answered, declined by name and abandoned reported apart, per tool and caller. Proof: the panel equals the verb JSON {id: ENG-7, symbol: HookPathPresence, package: internal/metrics}
- [ ] One lookup verb on the hook path (`query_symbols` merges into `find_symbol` there), and a cheaper liveness probe. Proof: off-surface rows fall to zero {id: ENG-8, symbol: hookLookup, package: internal/mcp}
- [ ] Session and lane id on every call-log and hook row (20% carry a session today). Proof: attribution coverage at least 95% {id: ENG-9, symbol: stampRowAttribution, package: cmd/fathom}
- [ ] Gate-clone verify runs recorded under the checkout they verify (5,489 of 15,329 ledger rows sit outside one). Proof: the ledger reading {id: ENG-10, symbol: recordUnderCheckout, package: internal/verify}
- [ ] (op) Seven-day presence reading on the build fleet, each hook-path tool reported apart: the second step of the bar {id: ENG-M1, oracle: lexical, path: docs/bench/readings/realignment/eng-m1-presence-week.md, pattern: "^ENG-M1 verdict:", needs: "ENG-5,ENG-7,ENG-9,ENG-11"}

ENG-M2 and ENG-M3 are readings that the order needs (the fresh tenant-clause week and the revert check). They carry the ENG-M
prefix so the ENG-1 to ENG-11 ids stay as ruled.

Amendment wiring (ADR-095): when ADR-095 is accepted (OQ5), RT-196 and RT-198 in `docs/plans/responsible-tenant.md`
are re-pointed from RT-M1 to ENG-3 and ENG-M2. RT-M1 read FAIL twice on host-starved minutes that fathom does not own. Until
then, and if the owner answers no, they keep waiting on RT-M1.

### PG-2 Typed records, stage 1

- Goal: every record head is typed and vetted, conditions are computed, intent is queried.
- Done when: unparsed heads read zero for every kind, every condition has a date or a computable kind, `rec query` and `why`
  answer from typed links, and the syntax trial is read and recorded.
- Proof: the typed-records design. The outside witnesses are `cue vet` and agents on both harnesses in the trial.
- Stop: a kind that cannot reach zero unparsed without rewriting keeps a counted share.
- Rows: REC-1 to REC-12, REC-T, in `docs/plans/typed-records.md` (decision ADR-094, invariants I70, I72, I75). REC-1 is main
  thread, wave 0a. REC-T does not wait on RT-M15 because it measures authoring, not the engine, and its spend is its own cap (OQ10).
- Deletes (RM-D5): direct parse calls that bypass the dialect port.

### PG-3 Proof at the moment of change. The flagship.

- Goal: an edit is judged before it is written, and a landing carries its proof.
- Done when: at least two what-if verdicts pass replay and run inside the pre-edit deadline, the hazard engine reads on
  Terraform and CloudFormation, and lanes land on a bundle the integrator accepts without re-running an identical tree.
- Proof: false-done and collateral damage on the bench, hazard precision by replay against a plan and against `cdk diff`,
  minutes per landing from gate logs.
- Stop: a what-if verdict that fails replay is not built. If the CloudFormation check is under its floor against `cdk diff`
  (HZ-1), that is fixed before any other hazard row. If hazard precision on Terraform is under 0.9, hazards get no agent
  surface. These are the hazard plan's rules.
- Entry: none for the replay rows. No what-if verdict is trusted at the edit until RT-M15 reads PASS (ENG-5).

Hazard rows are not copied here:

- HZ-0 to HZ-7 and HZ-9 to HZ-12 are the hazard engine, in `docs/plans/change-hazards.md`. HZ-0 and HZ-1 are first and are
  reading lanes. HZ-9 waits on HZ-6 and HZ-8. HZ-8 is the single check that IF-13, IF-6 and IF-11 are claimed. LIB-2 and LIB-3 read the deployed result.
- HZ-13 to HZ-18 are the check kernel. The gate is the owner's written ask (OQ3).
- HZ-19 is the infrastructure replacement card that CHG-1 and CHG-2 use. It keeps its gate (ADR-091 decision 7 and the deferred
  agent reading).

- [ ] (op) Replay cards for what-if verdicts. Code cards: conserved surface lost, wiring break, duplicate at birth. Infrastructure card: replacement hazard (HZ-19). Proof: eligible, fired, right, and noise against the post-write signal for the same hazard, per card {id: CHG-1, oracle: lexical, path: docs/bench/readings/realignment/chg-1-replay-cards.md, pattern: "^CHG-1 verdict:"}
- [ ] Build the code verdicts that pass replay with lower noise than the post-write signal. Each speaks only on a transition and fits the payload cap of invariant I35. The infrastructure card is built only when HZ-19's gate opens. Proof: latency inside the 500 ms cap {id: CHG-2, symbol: evaluateWhatIf, package: internal/review, needs: "CHG-1,ENG-5"}
- [ ] Suspect set on a red test, and shell unwrap so wrapped test runs are seen (83% unseen in the baseline). Proof: replay over a week of sessions {id: CHG-3, symbol: SuspectSet, package: internal/testrun, needs: "ENG-9"}
- [ ] Strength floors for agent-written tests, through attested results. Fathom runs nothing. Proof: certificate fixtures {id: CHG-4, symbol: EnforceStrengthFloor, package: internal/certify}
- [ ] Landing bundle: exact-tree gate receipt, verify verdicts, affected-test set, overlap with main's reach. Proof: the bundle verifies on a clean checkout {id: CHG-5, symbol: BuildLandingBundle, package: internal/landing}
- [ ] Integrator accepts a bundle. CI samples full runs, and a sampled run that disagrees with a bundle is an incident {id: CHG-6, symbol: AcceptLandingBundle, package: internal/landing, needs: "CHG-5"}
- [ ] (op) Landing-evidence reading: minutes per landing and bundle acceptance, read as LE-M1. The proof program records this verdict as BEN-M3 {id: CHG-M1, oracle: lexical, path: docs/bench/readings/realignment/chg-6-landing-evidence.md, pattern: "^CHG-M1 verdict:", needs: "CHG-6,landing-evidence.md#LE-M1"}
- [ ] Lane overlap forecast, used to order merges. PG-3 owns it and the fleet view renders it. Proof: a 30-day replay, forecast recall and false alarms {id: CHG-7, symbol: ForecastOverlap, package: internal/landing, needs: "ENG-9"}

### PG-4 The proof program

- Goal: each capability proven alone, then together, with proof that cannot go stale in silence.
- Done when: the proof-program design's sequence is read.
- Proof: BEN-n readings. The first paid dollar goes to false-done on tasks with rows that are false by construction.
- Rows: BEN-1 to BEN-9 in `docs/plans/proof-program.md` (decision ADR-101, design `docs/design/proof-program.md`).
- Deletes (RM-D5): a bench program (the merge of strengths, prep, labs, context and review into one driver and one ledger).

### Thin slices that ride with the core

These start in H0. Each has a reason it cannot wait.

- [ ] The realignment records ADR-093 to ADR-104 (NA-1 to NA-12), each short, with typed revisit text. No lane may cite a decision that does not resolve {id: RLN-1, oracle: lexical, path: ADR.md, pattern: "^## ADR-104:"}
- [ ] Verdicts for the nine goals past review, and new goals G51, G52 and G53 each with an instrument and a bar. The intent doctrine refuses a goal without one {id: RLN-2, oracle: lexical, path: docs/intent/scope/goals-3.md, pattern: "^\*\*G5[123]\.", min: 3, needs: "RLN-1"}
- [ ] A wire stamp per session and a `wiring_firing` doctor row. No bench cell is valid without it {id: WIR-1, symbol: WriteWireStamp, package: internal/wiring}
- [ ] Purge wave 1, the dead packages: `concern`, `graph`, `lookup`, `journal`, `resultcache` and `scenariobenchwired` have no product importer. Less for every lane to read {id: CUT-1, gone: pattern "^package (concern|graph|lookup|journal|resultcache|scenariobenchwired)$", paths: "internal/concern/,internal/graph/,internal/lookup/,internal/journal/,internal/resultcache/,internal/scenariobenchwired/"}
- [ ] (op) A release tag that contains the library, with its closure budget re-read. The one open library reading cannot run without it {id: LIB-1, oracle: lexical, path: docs/bench/readings/realignment/lib-1-tag.md, pattern: "^LIB-1 verdict:"}

FLT-1 (the fleet freshness table and engine strip) lives in `docs/plans/fleet-picture.md`. It is PG-1's instrument and the first
thing the owner asked to see, so it rides with the core. It needs ENG-7, ENG-9 and ENG-10 from this file.

The rest of purge wave 1 is named in the disposition and bound by the integrator when each symbol is known: the lossy readout
rules (RO-M0 cut at 0.535% against 4%, RO-M19 stop at 0.40% against 1%), carry-across at compaction (CA-M1 stop), the `route`
and `rank_within_tier` decision sites (45.7% right at 0.9 confidence), `gen-status-cue`, the `gen-claude-md` alias and the three
one-time migration scripts, and designs for work already cut. Each lands under a `gone:` row with a reopening condition.

## 4. Programs with an entry condition

Nothing here starts before its entry condition holds. This is a queue, not a plan (RM-R1). Rows under these programs carry
`needs:` for the entry.

| Program | Goal | Entry |
|---|---|---|
| PG-5 Facts, wider | Proof verbs work beyond Go. Cross-service and dependency questions have answers. | None for FCT-1. The freeze amendment (ENG-3) for the table rows. |
| PG-6 Fleet picture | Progress, lanes and relations, with the age of every number | REC-8 landed (keys), ENG-9 landed (attribution) |
| PG-7 Understanding and organization | Language in, facts out. The fleet's standards are checked, not described. | REC-9 and REC-10 landed |
| PG-8 Wiring | Wired, firing and measured on every harness that builds fathom | WIR-1 landed |
| PG-9 Library and catalogs | The library is released and useful to a stranger | LIB-1 landed |
| PG-10 Subtraction | Less code, fewer surfaces | each cut's own evidence |
| PG-11 In-band structure | Stop repeated exploration, inside the agent's own actions | WIR-1 and BEN-4 landed |
| PG-12 Default-on and tuning | What won a reading is on. Knobs are set by readings. | at least three cards with outcome proof |

### PG-5 Facts, wider

- Goal: proof verbs work beyond Go. Cross-service and dependency questions have answers.
- Done when: `wired` and test reach answer for TypeScript and Python on the reference repositories. The system graph has tables
  and one verb. One dependency question is answerable.
- Proof: edge readings against SCIP or a compiler, the link oracle at its recorded bars, the upgrade-impact reading.
- Stop: a fact kind that grows the store past its cost gate without a reading that shows a use is cut.
- Entry: none for FCT-1. The freeze amendment (ENG-3, OQ5) for FCT-2 and FCT-4.
- Plans it draws on: `docs/plans/system-graph.md`, `docs/plans/dependency-licenses.md`, `docs/plans/repository-facts.md`.

- [ ] SCIP-tier edges on by default at init for TypeScript with JavaScript, and Python, with `wired` and test reach reading them {id: FCT-1, symbol: defaultIndexersForInit, package: internal/scip}
- [ ] System graph tables and refresh {id: FCT-2, symbol: RefreshSystemGraph, package: internal/syslink, needs: "ENG-3"}
- [ ] Links and system impact as parameters of existing verbs {id: FCT-3, symbol: systemImpactParams, package: internal/mcp, needs: "FCT-2"}
- [ ] Dependency plane from local caches {id: FCT-4, symbol: ReadLocalDependencyCaches, package: internal/depgraph, needs: "ENG-3"}
- [ ] Owners as facts: the owners parser that has no caller is wired {id: FCT-5, symbol: OwnerFacts, package: internal/ownership}
- [ ] Attested execution evidence flows into `wired` (ADR-067: static reach was wrong both ways in CV-M13) {id: FCT-6, symbol: WiredFromAttested, package: internal/testevidence}
- [ ] (op) Incumbent extractors retired at parity, one implementation per language {id: FCT-7, oracle: lexical, path: docs/bench/readings/realignment/fct-7-extractor-parity.md, pattern: "^FCT-7 verdict:", needs: "FCT-1"}
- [ ] Extractor epoch per dialect, so a bump no longer invalidates every cache {id: FCT-8, symbol: DialectEpoch, package: internal/extractor}
- [ ] The computed catalog of services, owners, dependencies and called services (ADR-051's U10) {id: FCT-9, symbol: BuildServiceCatalog, package: internal/syslink, needs: "FCT-2,FCT-5"}

### PG-6 Fleet picture

- Goal: progress, lanes and relations, with the age of every number.
- Done when: the fleet-picture build steps 1 to 4 are shipped.
- Proof: the fleet-picture design's readings.
- Stop: no view writes a phase (ADR-049 decision 4).
- Entry: REC-8 landed (keys), ENG-9 landed (attribution).
- Rows: FLT-0 to FLT-10 in `docs/plans/fleet-picture.md` (decision ADR-098). FLT-5 reads the forecast that CHG-7 computes.
  FLT-9 reads the page-view log and lists the console pages with no views in 30 days, which the cut then removes (ADR-104
  decision 5).

### PG-7 Understanding and organization

- Goal: language in, facts out. The fleet's standards are checked, not described.
- Done when: a closed set of question shapes answers through the router and existing tools, `why` walks typed links, and fleet
  conformance reads on the fleet.
- Proof: the public benchmarks of ADR-085 with their kill on 2027-03-31, a hand-audited conformance sample, the thesis arm.
- Stop: the kill on 2027-03-31 stands. If the understanding bench reads null, the product is the proof layer without the
  understanding claim, and that is planned for (V-R6).
- Entry: REC-9 and REC-10 landed.
- Plans it draws on: `docs/plans/project-understanding.md` (its lifecycle rows moved to PG-2, its trace, router and bench rows
  belong here).

- [ ] Named-query registry: the closed catalog of questions, as a CUE data kind {id: UND-1, symbol: NamedQueries, package: internal/query, needs: "typed-records.md#REC-10"}
- [ ] Question shapes and the router verb, by keyword and anchor match at provider none {id: UND-2, symbol: RouteQuestion, package: internal/query, needs: "UND-1"}
- [ ] `why`: walk typed links from a decision to its enforcement, and the intent trace {id: UND-3, symbol: WhyChain, package: internal/intent, needs: "typed-records.md#REC-9"}
- [ ] Fleet conformance: a standard is one typed check run over every indexed repository, with adoption share and receipts {id: UND-4, symbol: RunStandard, package: internal/custodian, needs: "typed-records.md#REC-10"}
- [ ] The organization view: the catalog of FCT-9 joined with conformance {id: UND-5, symbol: OrganizationView, package: internal/console, needs: "FCT-9,UND-4"}
- [ ] Shape classification as a decision site, off by default {id: UND-6, symbol: ClassifyShape, package: internal/decide, needs: "UND-2"}
- [ ] Bench preregistration and grader for the named-query question set {id: UND-7, symbol: GradeAnchorMatch, package: internal/benchprep, needs: "UND-2"}
- [ ] (op) The public reading, on the public benchmarks of ADR-085 {id: UND-M1, oracle: lexical, path: docs/bench/readings/realignment/und-m1-public.md, pattern: "^UND-M1 verdict:", needs: "UND-7,ENG-5,WIR-1"}
- [ ] (op) The thesis arm against a query-time model tool: accuracy by anchor match, cost per question, and the same answer on a second run (V-M6) {id: UND-M2, oracle: lexical, path: docs/bench/readings/realignment/und-m2-thesis-arm.md, pattern: "^UND-M2 verdict:", needs: "UND-7,UND-M1,ENG-5,WIR-1"}

### PG-8 Wiring

- Goal: wired, firing and measured on every harness that builds fathom.
- Done when: the second harness has parity for the proof channels. A stranger wires either harness on Linux or macOS in five
  minutes. Every session carries a wire stamp.
- Proof: the harness's own event log, the stranger reading, live fixtures.
- Stop: a channel with no parity on the second harness is listed by name and no paid cell uses it.
- Entry: WIR-1 landed.

- [ ] Second-harness parity for the claim gate, deny and test-run {id: WIR-2, symbol: ClaimGateParity, package: internal/wiring, needs: "WIR-1"}
- [ ] Managed daemon on Linux {id: WIR-3, symbol: LinuxServiceUnit, package: internal/wiring, needs: "WIR-1"}
- [ ] (op) One-step install, measured with a stranger on each host {id: WIR-4, oracle: lexical, path: docs/bench/readings/realignment/wir-4-install.md, pattern: "^WIR-4 verdict:", needs: "WIR-3"}
- [ ] Harness matrix rendered from the registry, so a demoted harness stops printing tier A {id: WIR-5, symbol: RenderHarnessMatrix, package: internal/wiring, needs: "WIR-1"}
- [ ] (op) Release provenance, read by a stranger: the release workflow already attests each archive and the checksums file, so this reading runs the documented `gh attestation verify` on the newest release archive on each host {id: WIR-6, oracle: lexical, path: docs/bench/readings/realignment/wir-6-attestation.md, pattern: "^WIR-6 verdict:", needs: "WIR-1"}
- [ ] The threat-model answer, and write tools (`ingest`, `compile_layer`, `remove_service`, `rename_service`) off the full endpoint and on the CLI only {id: WIR-7, symbol: FullEndpointToolSet, package: internal/mcp, needs: "WIR-1"}
- [ ] (op) The advice readings AD-M1 (after AD-7 lands) and AD-M4, on the holdout log that BEN-4 builds {id: WIR-8, oracle: lexical, path: docs/bench/readings/realignment/wir-8-advice-readings.md, pattern: "^WIR-8 verdict:", needs: "advice-delivery.md#AD-7,proof-program.md#BEN-4,WIR-1"}

### PG-9 Library and catalogs

- Goal: the library is released and useful to a stranger.
- Done when: a tag contains the door, its outside-module reading passes, replacement facts and published catalogs exist.
- Proof: the library readings and the infrastructure readings IF-M1 and IF-M2.
- Stop: the library door sunsets on 2027-05-04 without an outside consumer.
- Entry: LIB-1 landed. PG-3 depends on IF-13, IF-6 and IF-11 through HZ-8. PG-9 owns IF-13, IF-6, IF-11 and the Kubernetes door (RM-D9).
- Plans it draws on: `docs/plans/iac-facts.md`, `docs/plans/lang-library.md`.

- [ ] (op) Terraform replacement scan, the row IF-13 of the IaC facts plan, read for precision {id: LIB-2, oracle: lexical, path: docs/bench/readings/realignment/lib-2-replacement-scan.md, pattern: "^LIB-2 verdict:", needs: "LIB-1,iac-facts.md#IF-13"}
- [ ] (op) Published catalogs and the licence audit, the rows IF-6 and IF-11 {id: LIB-3, oracle: lexical, path: docs/bench/readings/realignment/lib-3-catalogs.md, pattern: "^LIB-3 verdict:", needs: "LIB-1,iac-facts.md#IF-6,iac-facts.md#IF-11"}
- [ ] The product reads through the door where it already needs those facts {id: LIB-4, symbol: factsViaDoor, package: internal/iacfacts, needs: "LIB-2"}
- [ ] (op) Kubernetes and Helm in the door (hazard plan row HZ-21) {id: LIB-5, oracle: lexical, path: docs/bench/readings/realignment/lib-5-kubernetes.md, pattern: "^LIB-5 verdict:", needs: "LIB-1,change-hazards.md#HZ-21"}
- [ ] The spec-tool check as packaging {id: LIB-6, oracle: lexical, path: docs/use-cases/spec-interop.md, pattern: "^## Maintained", needs: "LIB-1"}

### PG-10 Subtraction

- Goal: less code, fewer surfaces.
- Done when: every cut in the disposition is proven gone.
- Proof: `gone:` rows.
- Stop: a cut returns only on its written reopening condition.
- Entry: each cut's own evidence. CUT-1 is a fired kill. CUT-2 waits on OQ6 because it is preference, not a fired kill.
- Rule: PG-10 deletes more than it adds (RM-D5).

- [ ] Preference cuts after OQ6: the assess lanes and the hidden `assess` and `research` verbs, and goal G2's ambient A/B with `arm`, `rate`, `retro` and `flywheel` {id: CUT-2, gone: pattern "^func cmd(Assess|Research|Arm|Rate|Retro|Flywheel)[(]", paths: "cmd/fathom/", needs: "CUT-1"}
- [ ] (op) The second surface reckoning: eleven extended tools with no or almost no calls leave the extended surface and stay as CLI, read against the off-surface demand {id: CUT-3, oracle: lexical, path: docs/bench/readings/realignment/cut-3-surface-reckoning.md, pattern: "^CUT-3 verdict:", needs: "ENG-8"}
- [ ] Merge the five refresh triggers, the five export paths and the four offline transcript readers into one verb each, with modes {id: CUT-4, symbol: cmdRefreshModes, package: cmd/fathom, needs: "CUT-1"}
- [ ] (op) Archive plans with zero open rows and retire designs for cut work {id: CUT-5, oracle: lexical, path: docs/bench/readings/realignment/cut-5-archive.md, pattern: "^CUT-5 verdict:", needs: "CUT-1"}
- [ ] Render or test every number in README, overview and early-tester notes {id: CUT-6, symbol: renderDocCounts, package: cmd/loctruth}
- [ ] Verb packages by family with a row rewrite, on the trigger of future F98. Not before: a split unbinds 563 claimed rows {id: CUT-7, symbol: registerVerbFamily, package: cmd/fathom, needs: "CUT-4"}

### PG-11 In-band structure

- Goal: stop repeated exploration, inside the agent's own actions.
- Done when: the shared read map and the anchors each have a delivery reading and a paid arm, or are cut.
- Proof: XM-M2 and the in-band arm.
- Stop: fails admission clause 2 until a reading, since it offers context, not a verdict. Default off, by the rule's case 2.
- Entry: WIR-1 and BEN-4 landed.
- Plans it draws on: `docs/plans/exploration-maps.md`.

- [ ] A shared read map for sibling agents at sub-agent start. Its reading is XM-M2. Sibling re-reads are 69.9% of re-reads, parent-to-child 2.8% to 15% {id: INB-1, symbol: SharedReadMap, package: internal/explmap, needs: "WIR-1,proof-program.md#BEN-4"}
- [ ] Anchors on the agent's own search and read results (future F75), replay first {id: INB-2, symbol: AnchorOnResult, package: internal/explmap, needs: "INB-1"}
- [ ] (op) The in-band bench arm {id: INB-3, oracle: lexical, path: docs/bench/readings/realignment/inb-3-in-band-arm.md, pattern: "^INB-3 verdict:", needs: "INB-2,proof-program.md#BEN-6"}

### PG-12 Default-on and tuning

- Goal: what won a reading is on. Knobs are set by readings.
- Done when: V-M1 to V-M8 of vision 46 are read.
- Proof: the knob map with a reading per knob, the outside team's report.
- Stop: nothing is default-on without a card that has outcome proof.
- Entry: at least three cards with outcome proof (BEN-7).
- Owner questions: OQ1 (product or force multiplier), OQ2 (name one outside team by 2027-01-31, else "fleet only").
- These rows are provisional. PG-12 has a goal, an entry and a proof, and no authored row list yet.

- [ ] The knob map as a CUE data kind, one row per knob with its reading {id: TUN-1, oracle: lexical, path: docs/bench/readings/realignment/tun-1-knob-map.md, pattern: "^TUN-1 verdict:", needs: "typed-records.md#REC-10,proof-program.md#BEN-7"}
- [ ] Default-on for a capability only when its card has outcome proof {id: TUN-2, symbol: DefaultOnGate, package: internal/config, needs: "TUN-1"}
- [ ] (op) Held-out confirmation of each tuned knob {id: TUN-3, oracle: lexical, path: docs/bench/readings/realignment/tun-3-held-out.md, pattern: "^TUN-3 verdict:", needs: "TUN-2"}
- [ ] (op) One team outside the owner's fleet runs the gate for four weeks and files what broke (V-M7) {id: TUN-4, oracle: lexical, path: docs/bench/readings/realignment/tun-4-outside-team.md, pattern: "^TUN-4 verdict:", needs: "TUN-2"}

## 5. The admission test, applied to these programs

Vision 46, section 7, has five clauses: computable from held or attested facts, a verdict or a set, an existing channel, an
outside oracle, a consumer or a net deletion.

| Program | 1 to 3 | 4 Outside oracle | 5 Consumer or deletion |
|---|---|---|---|
| PG-1 | pass | the harness's own hook timeouts and kills | every hook-path capability |
| PG-2 | pass. Records are authored, conditions are computed. | `cue vet`, and agents on two harnesses in the trial | verify, the fleet view, `why` |
| PG-3 | pass. Clause 1 relies on attested test results for CHG-4. | tests, a plan, `cdk diff`, fix commits, CI | the build fleet |
| PG-4 | pass | every oracle in the proof program | every claim fathom publishes |
| PG-5 | pass | SCIP indexers and compilers | `wired` beyond Go, the catalog |
| PG-6 | clause 2 in part: a board shows sets and ages, not prose | git merge history for overlap. Page views are fathom's own and only cut pages. | the owner |
| PG-7 | pass at provider none | public benchmarks, a hand-audited conformance sample | unproven until the bench reads |
| PG-8 | pass | the harness's own event log | the proof program |
| PG-9 | pass | the tools' own evaluators | no outside consumer yet. Sunset 2027-05-04. |
| PG-10 | deletes | `gone:` rows | deletes more than it adds |
| PG-11 | fails clause 2 until a reading: it offers context, not a verdict | harness transcripts | default off, by the rule's case 2 |
| PG-12 | pass | the outside team | the outside team |

## 6. Capacity, as a starting split

Core programs take about 70%. Thin slices take 10%. Programs with an entry condition share 20% and only after entry. Language
breadth continues in data lanes outside this split, capped as `docs/design/parallel-lanes.md` describes. Reading lanes are
staffed from the first wave.

## 7. Decisions

- RM-D1 The program list replaces the amendment chain as the operative schedule.
- RM-D2 Four core programs. Nothing else starts without its entry condition.
- RM-D3 ENG-1 and ENG-2 are main-thread work before fan-out. They wait on no ruling.
- RM-D4 No paid proof step before the second gate in section 2.
- RM-D5 Every build program names what it deletes. PG-1 deletes a patch path that differed from a rebuild. PG-2 deletes direct
  parse calls. PG-4 deletes a bench program.
- RM-D6 A program past its review date gets a recorded verdict within seven days.
- RM-D7 Measurement rows are staffed. About 310 open rows are readings. Each is assigned to a reading lane, closed, or turned
  into a typed question.
- RM-D8 No program adds a tool to the default surface.
- RM-D9 The hazard plan's rows keep their numbers (HZ-n). PG-9 owns IF-13, IF-6, IF-11 and the Kubernetes door. PG-3 depends on
  them. HZ-20 (E4 or E5) and HZ-22 (E13) wait on their own gates, as the hazard plan states.

## 8. Risks

- RM-R1 Twelve programs still read as too many. Mitigation: only four are staffed, and the table in section 4 is a queue, not
  a plan.
- RM-R2 The freeze amendment waits on the owner and on a fresh read. Mitigation: ENG-1 and ENG-2 do not wait, and they carry
  most of the latency gain.
- RM-R3 What-if verdicts do not fit a 500 ms cap. Mitigation: replay measures latency before any build.
- RM-R4 Eight or more existing kill dates fall on 2027-01-31. Mitigation: the goal verdicts (RLN-2) close most of them on
  purpose before then.
- RM-R5 Proof arrives and reads null. The roadmap does not depend on a win (V-R6 of vision 46).
- RM-R6 The trial delays typed syntax by weeks. Accepted. Stage 1 does not wait on it.

## 9. The critical path

```
records and rulings (ADR-093 to ADR-104, OQ7, OQ9, OQ11)
  -> lanes can start: REC-1 lane kind, allocation, path check, contracts and failing proofs

ENG-1, ENG-2 (not frozen)
  -> hook-path harness re-read -> fan-out

OQ5 freeze amendment + fresh tenant-clause week (ENG-M2, runs beside wave 1)
  -> ENG-3 -> ENG-4 re-warm -> ENG-5 RT-M15 reading -> paid proof steps
  -> system graph tables (FCT-2), dependency plane (FCT-4)

REC-2 to REC-9 typed heads, conditions, queries -> fleet bars, why, named queries
WIR-1 wire stamp + BEN-2 readiness harnesses -> paid proof steps
```

Three gates decide what follows them.

1. **ENG-1.** One indexed lookup is 96% of the cold build. It is specified, measured and unbuilt, and it waits on nothing.
2. **The freeze amendment.** It unblocks the re-warm and every table program. The reading that failed asked the owner this
   question on 2026-10-26.
3. **RT-M15.** Until at least half of hook-path graph calls are answered, no paid proof is valid and no what-if verdict can be
   trusted at the edit.

If only one thing is done this month, it is ENG-1.

# The library roadmap: the understanding library and the checks that go with it

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Plan: `docs/plans/library-roadmap.md` (LP- programs, LIB- and RL- rows). Decisions: ADR-092 (the library), ADR-099 (the hazard engine), ADR-105 to ADR-107 (same round, ADR-107 is the record that opens the lifetime lane). Builds on `docs/design/change-hazards.md`, `docs/plans/change-hazards.md` (HZ-), `docs/plans/lang-library.md` (LL-), `docs/plans/iac-facts.md` (IF-) and `docs/vision/46-proof-at-the-moment-of-change.md`. Method: a capability review and a use census, then two critics told to refute the draft.

This is the separate roadmap for the standalone understanding library and the checks that go with it. It shows the library's whole potential, not only change hazards, and it schedules only what someone will use.

Codes in this document: LB-F finding, LB-D decision, LB-R risk. Programs are LP-1 to LP-7 and horizons are LH0 to LH3. RU-n are the rulings of section 12. Rows reuse the ids that exist: LL (the library plan), IF (the infrastructure facts plan) and HZ (the hazard plan). New library rows continue from LIB-7. RL-n are resource-lifetime rows, U-n are uses from the census, CAP-n are capabilities from the review, E-n are engine gaps (E1 to E13 in the hazard design, E14 to E25 here). Marks: NOW, ENTRY (adopt on an entry condition), FUTURE (a register entry, not a scheduled row). T-IaC is the trigger for infrastructure work: a fleet repository with Terraform or CloudFormation, or the policy tool importing the library, or an outside adopter named in writing. OQ3, OQ4 and OQ15 are owner questions (OQ3 and OQ4 in vision 46, OQ15 is new: whether an outside adopter is named in writing, which satisfies T-IaC). A hazard fact is what the library computes. A hazard finding is what the product delivers. The main roadmap is `docs/plans/realignment.md`.

Tags: `[web]` means checked against a primary source during the review. `[prior]` means unverified.

Reading order: the library's critical path is the last section.

Builder support is proposed in [library-builder-kit.md](library-builder-kit.md), with work orders in [the builder-kit plan](../plans/library-builder-kit.md).

## 1. Findings

- LB-F1 No release tag contains the library. The last tag is 2026-10-28 and the first door commit is 2026-11-04. The one open library reading (LL-M7) waits on a tag.
- LB-F2 The door is a set of public packages over internal code. `pkg/lang/iac` wraps `internal/iacfacts`. The product does not import the door today.
- LB-F3 The closure reading is stale and the budget is spent. The door links 80 packages against a budget of 80. A consumer links 81, since the read guarantee added two leaf packages after the reading. The repository states the bar two ways: the design calls 81 one over LL-M1's bar of 80, and the closure test's comment calls 81 the consumer bar. The design says LL-M1 is to be re-read and the bar decided. LL-M7 needs LL-M1.
- LB-F4 The door reads 121 dialects (12 at L4, 57 at L3) and gives value classes for three infrastructure tools. It holds no change set, no Kubernetes contract, no lifetime fact and no fixture mutation.
- LB-F5 The honest-unknown base is measured: unknown rates of 10.0% for Terraform and 6.1% for CloudFormation against a 30% bar. Pulumi reads 16.7% on 18 pairs, reported with no verdict (LL-M6).
- LB-F6 The distinctive facts (replacement, cascade, upgrade impact: IF-13 to IF-17) are unbuilt, and so is catalog publication. Every outside prerequisite of the Kubernetes rows (IF-18 to IF-20) is done.
- LB-F7 A CloudFormation replacement finding is already served by `review_diff` (IF-C4). Its floor against `cdk diff` (HZ-1) has never been read.
- LB-F8 Whether the fleet holds infrastructure code is unverified. This repository holds none outside test data and the policy tool is unbuilt. A census of the fleet through the fleet index settles it before anything else here acts on it (main UND-1). Note 2026-11-14: superseded 2026-11-14 by ADR-112: an outside consumer is attested, unseen. The fleet infrastructure census remains unverified.
- LB-F9 The test that keeps values out of the product already flags a product call that opts in through the door, and has a seeded case for it.

## 2. The boundary: are scanners part of the package

Yes at the package level. Not yet at the public API level. Never for rule corpora.

| Level of "package" | Holds | Scanners in it |
|---|---|---|
| The module, the implementation package and the binary | facts, contracts, the change set, hazard facts H1 to H9, lifetime site facts | Yes, now. One release, one binary (`fathom-lang`). |
| The public Go API (the door) | facts, resource reads with value classes, catalogs | Additive fields and types on the door that exists, marked provisional. No new door until a consumer is named in writing (F99). |
| Checks that need the typed whole-program graph (pairing, growth, detector coverage) | hosted in the product's typed pass, tracked on this roadmap | Yes, on their entry conditions. They cannot live in the door: they run the toolchain. |
| The check kernel, `pkg/lang/check` | evaluation of a caller's check, and the draft acceptance test | Location settled. Built on a yes to OQ3. |
| Outside fathom | rule corpora, baselines, severity, pass or fail, waivers, crosswalks | Never in the library. Value checks V1 to V7 exist only as fixtures. |

- LB-D1 A hazard is a derived fact about a change, not a rule and not a verdict on a caller's rule. It stays where it lives and ships in the binary. ADR-092 decision 10 gains one line that says so.
- LB-D2 The base never imports a check. Checks are not useful without the base. The reverse dependency does not exist.
- LB-D3 New fields and types on the door are marked provisional in the API snapshot until their reading passes. A provisional symbol may change in a minor release.
- LB-D4 Values are read only when a caller opts in, and the product never opts in (ADR-092 decision 7, kept, and already tested).
- LB-D5 This roadmap has two sizes. The programs in section 5 are scheduled whatever the owner answers. The register in section 7 opens on OQ3, on OQ15, or on each entry's own trigger.
- LB-D6 Clause 5 of the admission test (vision 46 section 7) is applied to every row, old and new. The consumer of the hazard rows is the replay reading and the public benchmark. A hazard no plan can label waits in the register.

## 3. The whole potential

The use census judged 67 possible uses across code review, refactoring, upgrades, best practices, security, supply chain, maps, docs, tests, incidents, cost, packaging and other engines. One is owed now, 12 were already planned, 14 are scheduled behind a condition, 22 wait in the register, 14 are refused with a wins-if or a re-read trigger, and 4 need nothing built.

### 3.1 Eight primitives

Most uses reduce to six primitives. This roadmap adds two that the census ranks high (7 and 8).

| Primitive | State today | Scheduled here | Planned or scheduled on the main roadmap | Register |
|---|---|---|---|---|
| 1. A fact delta between two versions of a file | An index-free delta exists for some Go kinds | LIB-13: change summaries, the touch set, a what-changed timeline (U-1, U-2, U-40) | edit-time verdicts (U-51), collateral damage and task construction on the bench (CAP-26, CAP-27) | the delta as a binary subcommand |
| 2. Hazard and contract facts | A CloudFormation replacement finding is live and unread. Terraform is specified. | LP-3 and LP-4: what a change destroys, upgrade breaks, the contract check inside the product (U-3, U-7, U-14 inside, CAP-25) | the review dimension and the benchmark | a literal on a sensitive attribute (U-17), the contract check outside the product (U-14), packaging (U-48) |
| 3. Upgrade impact between two versions of a contract | Unbuilt | HZ-11: provider upgrades (U-7) | uses of a deprecated library or API (U-10) | module and chart interface diffs (U-8), removed cluster API versions (U-9), the catalog diff (CAP-32), dependency lifetime contracts (RL-7) |
| 4. An inventory of external inputs with pin state | Only workflow actions carry a pin class | none | the actions half (U-21) with the census and fleet standards (CAP-4, CAP-8), the dependency licence seed (U-25) | base image pins, chart dependency versions, provider constraints, an estate inventory (U-58), declared build inputs (U-23) |
| 5. Reference resolution for configuration | Only one config family links across files | none | which job runs which target, dead configuration, `referenced` for verify rows (U-36, U-44, CAP-16) | the resolver itself (E16), rename safety (U-4, U-42) |
| 6. The fact JSON as input to someone else's engine | The JSON is already a pinned compatibility surface | catalogs as data (IF-6, U-53) | none | a schema document (U-52), fixtures for other engines (U-54), a grader (U-56), structure in place of text (U-64) |
| 7. Lifetime site facts, and resource checks over a typed whole-program graph | This repository's tenancy lint is the first instance, written by hand | LP-5: RL-2, then RL-5, RL-4, RL-3 (U-12) | analyzer and runtime output as graded facts, the lint as rule data, rules compiled once (U-13, RL-1, RL-6, RL-9, RL-8) | RL-7, sizing (RL-10), cloud and cost joins (RL-11) |
| 8. Measured reading levels | 121 dialect readings, each graded against an outside oracle | LIB-14: fixtures and oracles for the reading benchmark (U-55) | the benchmark itself (BEN-11) | a conformance kit (U-60), file classification by level (U-67) |

### 3.2 The census's ranking, and where this roadmap orders differently

| Census rank | Use | Consumer today | Row here | Order here |
|---|---|---|---|---|
| owed | A licence notice for the embedded grammars (U-24) | every archive already shipped | LIB-12 | first, before the tag |
| 1 | The fact delta (U-1, U-2, U-40) | the review verb, the incident procedure, the bench | LIB-13 | later than its rank. It is built inside the what-if work, which waits on the hook prerequisites (main CHG-8) |
| 2 | Change hazards through their public benchmark (U-3, CAP-25) | the proof program, and a finding that is already live | LP-4, main BEN-10 | earlier than its rank. HZ-1 reads a finding agents already see |
| 3 | Resource lifetime, starting from the tenancy lint (U-12, U-13) | this repository's gate | LP-5 | as ranked |
| 4 | The reading benchmark, under three rules (U-55) | the proof program | LIB-14, main BEN-11 | as ranked, behind the paid gate |
| 5 | The actions half of the input inventory (U-21) | fleet standards | main UND-1, UND-4 | as ranked |
| 6 | Configuration reach (U-36, U-44) | verify rows that bind config symbols | main FCT-11 | as ranked, on its census |
| 7 | The recurrence guard and derived mutants (U-41, U-39) | the incident procedure, the bench | main CHG-10 to CHG-12, BEN-5 | as ranked. The checks half of U-39 waits on OQ3 |
| 8 | The contract check outside the product (U-14) | none | register | on a named user |
| 9 | Module and chart interface diff (U-8) | none | register | on a named user |
| 10 | The catalog as data, with its diff (U-53, CAP-32) | reproducing the benchmark | IF-6 scheduled, the diff in the register | the diff on a named user |

The value order is the census's. The start order differs in two places for stated reasons: a live finding is read before anything new is built, and the delta cannot start before the hook can rebuild a proposed file.

### 3.3 Census uses that land on the main roadmap

They use library facts and are product work. Each has a home in `docs/plans/realignment.md`.

| Use | What it is | Mark | Home |
|---|---|---|---|
| U-21 | The actions half of the external input inventory, as a named query | ENTRY: UND-1 | UND-1, UND-4 |
| U-36, U-44 | Which pipeline job runs which task target, and configuration nothing references | ENTRY: the config census | FCT-11 |
| U-61 | Config value drift across services. The analyzer exists and only its own test imports it. | ENTRY: wire it under fleet standards, or cut it | UND-4, else PG-10 |
| U-41 | An incident becomes a rule that is accepted once and runs forever | ENTRY: one incident, CHG-11, CHG-12 | CHG-10 (RL-8) |
| U-30 | Import-direction rules beyond Go | ENTRY: typed edges for other languages | FCT-1 |
| U-13 | Analyzer and runtime output as graded, attested facts | ENTRY: FCT-6 | FCT-6 (RL-1, RL-6) |
| U-51 | A pre-write contract and hazard check on proposed text | ENTRY: CHG-8, a replay card | CHG-1, CHG-2, HZ-19 |
| U-66 | Per-file outlines for the agent's own reads | PLANNED, default off | PG-11 |

### 3.4 What already exists and needs nothing built

- Reading untrusted input safely (U-63). The binary reads in a supervised child under a hard deadline. That is why it can run on a pull request from a stranger, and it is the edge of the packaging entries in the register.
- Span-accurate targets for bulk edits of non-code files (U-5). It is what a read already returns.
- Breaking-change checks for exported Go API (U-65). The product has the surface delta.
- Insecure defaults (U-18). Nothing in the library. Scanners and the policy tool own them, and the library can feed them.

What takes this roadmap beyond change hazards is primitives 1, 7 and 8. What would take it much further is a named user for primitives 3, 4 and 6. Every outside use fails the consumer clause, and the three ranked wedges (ranks 8 to 10) fail nothing else.

## 4. Horizons, by event

| Horizon | Exit events |
|---|---|
| LH0 Released and measured | The archives carry their licence notice (LIB-12). LL-M1 is re-read and its bar decided (LIB-11). A tag contains the door (LIB-1). LL-M7 passes. HZ-1 and HZ-0 are read. |
| LH1 The first Terraform reading | HZ-3, HZ-4 and HZ-6 are in. Catalog-free hazards run in the binary (HZ-10, LIB-7). The M1 replay reads for H2 and H4. |
| LH2 The bet is read | IF-13 lands. HZ-9 reads at 0.9 or the stop rule fires. HZ-11 is in. The benchmark's labels are published (main BEN-10). IF-11 is recorded. |
| LH3 Beyond hazards, and the owner's answers | Lifetime site facts are in the door and match the tenancy lint. The fact delta feeds a what-if card. The reading benchmark has its fathom arm. Catalogs are published (IF-6). Register rows move on OQ3, OQ15 or their own triggers. ADR-092's revisit is read on 2027-05-04. |

No date is set for LH0 to LH2. The anchors are the dates in section 9.

## 5. Programs, scheduled rows only

Rows are in `docs/plans/library-roadmap.md`. Rows that already exist in the hazard, library and infrastructure plans are named by id and never copied.

### LP-1 Release and door health

- Goal: a stranger can fetch, build and run the library from a tag.
- Done when: a tag contains the door and LL-M7 passes.
- Proof: LL-M7 through the Go proxy, LL-M1 re-read, the API snapshots.

| Row | Title | Mark | Proof or gate |
|---|---|---|---|
| LIB-12 | A licence notice for what the binaries embed (U-24). Every release archive since v0.13.0 embeds grammar blobs (55 since v0.23.0, 54 under MIT and one under Apache-2.0) and ships only the product licence and the readme. The census dates this from v0.24.0. The first archive was earlier. Generate the notice from the grammar lock and add it to the archives. | NOW, before the next archive ships | the release check fails without it |
| LIB-11 | Re-read LL-M1 and decide the bar: 80 as written, or 81 with the two guard packages named. State whether ADR-092's revisit on LL-M1 has fired. | NOW, before the tag and before HZ-4 or HZ-6 add to the door | the reading, a recorded ruling |
| LIB-1 | A release tag that contains the library (main row `realignment.md#LIB-1`) | NOW. Waits on LIB-12, LIB-11 and a green wave on main | `git tag --contains` the first door commit |
| LL-M7 | An outside module builds and runs against the tag | ENTRY: LIB-1 | the existing reading |
| LL-43 | A literal on a contract-sensitive attribute stays withheld even when values are on. Its trigger ("a door binds a contract during a read") fires when HZ-4 binds one. | ENTRY: HZ-4 | reader test |
| LL-40, LL-41, LL-42 | Deferred, triggers unchanged | | |

LIB-8 (widen the values scan) is withdrawn. Its premise was false (LB-F9).

### LP-2 Facts for hazards

- Goal: the reader states everything a hazard needs, and every unknown has a reason and a next step.
- Done when: no unknown in the three infrastructure readers lacks a closed reason and a `decidable_by`, and LL-M6 is re-read under its bar.
- Proof: reader tests, the API snapshot, the binary's JSON goldens.

| Row | Title | Mark | Proof or gate |
|---|---|---|---|
| HZ-3 | E1: unknown reasons and the read set on the result. With E17: `decidable_by` names the cheapest action that would decide it, from a closed list (a plan, a render, a vendored module, a catalog version, a context file). | NOW | every unknown has both |
| HZ-4 | E3: structured blocks (variables, outputs, locals, provider requirements, `moved`, `removed`, `import`) and a public resolver from lockfile to catalog version | NOW | reader tests, API snapshot |
| HZ-6 | E7: the change set, two fact sets by address with class transitions. An additive type on the door. | NOW | it passes the IF-C4 tests |
| HZ-5 | E2: class and span per member of an object or map | ENTRY: a hazard needs it, or M4 ranks it first | reading |

### LP-3 Contracts and catalogs

- Goal: versioned contracts with replacement facts, and catalogs a stranger can verify.
- Done when: replacement facts are present, each provider has a recorded licence finding, and catalogs for the pinned lines are release assets with attestations.
- Proof: IF-M1 against `tofu validate -json`, attestations that verify, the licence audit.

| Row | Title | Mark | Proof or gate |
|---|---|---|---|
| IF-13 | The Terraform replacement scan | NOW, beside HZ-10 | existing |
| IF-11 | Licence audit per provider | NOW | a provider that fails stays operator-built |
| IF-12, IF-M1 | Docs, and facts 1 to 4 against the tool's own validator | NOW | existing |
| IF-6 | Catalogs published as release assets with attestations. Consumer: anyone reproducing the benchmark's fathom arm. | ENTRY: LIB-1, IF-11, signed releases (main WIR-6), OQ4 | attestations verify |
| LIB-9 | Stateful type candidates. A published list is tried first (linters ship one for CloudFormation `[prior]`). A model may propose more. A person accepts each into the list the catalog builder compiles, with its provenance. The list is a declared baseline, not a provider contract, so a hazard fact that rests on an entry carries the evidence class `declared context`. | ENTRY: with HZ-9 | agreement of the list with a published list or a hand key on a stated sample. Replay cannot grade this: a plan says replace, not data loss |

### LP-4 Change and hazards

- Goal: "what will this change destroy", from two versions of text and a catalog, with no plan, no credentials and no value read.
- Done when: H1, H2, H4 and H6 read on Terraform and H1 on CloudFormation at their floors, in the binary. The main roadmap's PG-3 closes on this.
- Proof: M1, replay against the plan of head over a state built from base, read through a bench plan reader. M2, against `cdk diff`.
- Consumer (LB-D6): the replay reading, the public benchmark, and the CloudFormation finding that is already live.
- Stop rules, per tool: if HZ-1 is under its 0.85 floor, the live CloudFormation finding drops to advice wording and is fixed before any other hazard row. If HZ-9 precision is under 0.9, Terraform hazards get no agent surface.

| Row | Title | Mark | Proof or gate |
|---|---|---|---|
| HZ-1 | M2: the live CloudFormation change check against `cdk diff` on template pairs. It needs a small harness that runs both and scores them, and none exists. It is a reading row of its own: it needs only the check that is built (IF-C4), not IF-13. | NOW, the first lane of the first wave, because its finding is live | the 0.85 floor |
| HZ-0 | Landscape table from primary sources | NOW, a reading lane | no unverified row left |
| HZ-10 | H2 and H4: rename with no `moved`, multiplicity switched. H5 (provability lost) moves here from HZ-12. None needs a catalog. | NOW, after HZ-4 and HZ-6, beside IF-13 | the first M1 reading |
| LIB-7 | A hazard entry in the library binary: change set and hazard facts as JSON for two trees. Its JSON shape is a contract set on the main roadmap. | NOW, with HZ-10. It enters on HZ-6 and HZ-9 as first drafted and moves up so the first reading can run through it. | JSON goldens. The replay harness and the benchmark read it |
| HZ-9 | H1 on Terraform: the computation and the M1 reading. The bench plan reader half of HZ-15 is built with the harness. | ENTRY: IF-13 | precision 0.9 or the stop rule |
| HZ-11 | H6 from IF-17: upgrade impact on attributes in use | ENTRY: two catalog versions on disk | existing proof and fixtures. The benchmark labels upgrade breaks |
| IF-15 | The replacement cascade | ENTRY: IF-13 | graded by IF-M2 |
| IF-M2 | Replacement and cascade against a plan. Its `needs` are amended to IF-13, IF-15 and LIB-7, so it reads the binary and not the product surface. | ENTRY: IF-13, IF-15, LIB-7 | existing |
| IF-M3 | Upgrade impact on provider-bump commits | ENTRY: IF-17, with HZ-11 | existing |
| IF-CM3 | Terraform and CloudFormation replacement facts against each other. Unchanged. A second independent read beside HZ-1. | ENTRY: IF-13 | its 0.85 agreement floor |
| HZ-17 | M4: the decidability sample, head to head with Trivy, with soundness read from plan values through the bench plan reader | ENTRY: HZ-3, the bench plan reader | it ranks E2, E4 and E5 before any is built |

The product rows that deliver these stay on the main roadmap: HZ-2, IF-14, IF-16, HZ-19.

### LP-5 Resource lifetime and best practices

Per-function best practice stays with linters. The library's ground is site facts for every language, and for Go the answers only a typed whole-program graph can give. The first instance already exists: this repository's gate runs a hand-written tenancy lint that flags bare `go` statements, package-level maps, commands started without a context, a background context on request paths and whole reads in handlers. The library version is the same detections as facts, so any repository can state its own rule. The lint is also the bar.

- Goal: say where a resource, a goroutine or a container can outlive its bound, and where the detectors are not looking.
- Done when: site facts are graded in the door, RL-5 reports its number, and RL-4 and RL-3 have each passed or met their kill.
- Proof: RL-M0 to RL-M2 below. Each compares with the incumbent, and each can lose.
- Consumer: this repository is Go, and its gate runs the tenancy lint and the race detector. Fathom's own daemon memory is a standing problem. Whether another fleet repository runs an equivalent lint is unknown.
- A record comes first for the two heavy rows. RL-3 and RL-4 open a lane that F90 and the hazard design refused, so they wait on ADR-107 and on a cost reading each.

| Row | Title | Mark | Proof or gate |
|---|---|---|---|
| RL-2 | Lifetime site facts in the door (E22): acquire candidates, release calls, `defer`, `go` statements, lock and unlock, the loops that contain them, scoped constructs. Go first, from the reader's existing function walk. Other languages as query-pack data lanes. A new fact kind needs a graded reading. | ENTRY: the lint's port (RL-9) or one of RL-5, RL-4, RL-3 is scheduled | RL-M0: site facts against the tenancy lint's own findings on this repository, and against SSA truth for Go |
| RL-5 | Leak-detector coverage: which spawn sites are reached by a test that ran under a leak detector. Race coverage is not a separate number, since the gate runs every test under the race detector. | ENTRY: RL-2, and the operator turns on the runtime's leak profile in tests. Fathom runs nothing. | mutants that remove a join or a stop: the share killed only under the detector. The number starts at zero, because no leak detector is in use today. It has no bar and no kill: it reports where the detector is not looking |
| RL-4 | Growth and bound hazards for Go: a request-reachable insert into a long-lived container that nothing deletes from, a request-reachable spawn with no visible bound, a `defer` or timer made in a loop, work started with nothing that can stop it. Computed at ingest, since "no delete anywhere" is a whole-program fact. | ENTRY: ADR-107, RL-2, a cost reading at ingest under the store's cost gate (main ENG-3) | RL-M2: it must beat the tenancy lint on this repository, and the runtime's own leak profile on the goroutine half. Labels: a public concurrency-bug suite and public fix commits. Kill: audit precision under 0.7, or no gain over the lint |
| RL-3 | Acquire and release pairing for Go across functions, three-valued (E23). Escape is an unknown with a reason, never a pass. | ENTRY: ADR-107, the engine presence bar (main PG-1), a cost reading for the typed pass, RL-M1 preregistered | RL-M1: public fix commits that add a missing release, against the union of the linters. Kill: not above the union |

Order: RL-2 with its reading and the lint's port, because that has a consumer and a bar today. Then RL-5. Then RL-4 and RL-3, each after the record and its cost reading. Only RL-2 is door surface. RL-3 to RL-5 are hosted in the product. None of this runs on the hook path. The rows that pair with them are on the main roadmap: RL-1 and RL-6 (analyzer and runtime output as graded, attested facts, with FCT-6), RL-9 and RL-8 (the lint's checks as rule data, then project rules compiled once, with CHG-10). Kubernetes limits and probes need nothing added for change: a delta for them exists and is judged elsewhere (RL-10).

### LP-6 Uses beyond hazards

The census's library rows that are not hazards or lifetime. Each has a consumer inside the product or the proof program.

- Goal: build each primitive once, where a reader exists, so the uses that reduce to it follow.
- Done when: the delta feeds a what-if card and the bench, and the reading benchmark has its fathom arm.
- Proof: per row.

| Row | Title | Mark | Proof or gate |
|---|---|---|---|
| LIB-13 | The fact delta: symbols and facts removed, added or changed between two versions of one file. Internal, code card kinds first (U-1, U-2, U-40). For infrastructure it is the change set (HZ-6). | ENTRY: a what-if card passes replay (main CHG-1). Built by the what-if lane as an internal package, tracked here | delta tests. The bench's collateral-damage reading uses it |
| LIB-14 | Fixtures and oracles for the reading benchmark (U-55), under its three rules: preregistered questions an agent needs, native-tier oracles only, no oracle written by fathom's authors | ENTRY: main BEN-5. BEN-11 then waits on this row and the paid gate | the benchmark's fathom arm |
| HZ-4, again | The binding resolver is itself a use: locked, constraint-matched, ambiguous or unbound, per block (U-62) | NOW, in LP-2 | reader tests, API snapshot |

The census's other scheduled uses are product rows on the main roadmap. Section 3.3 lists each with its mark and home.

### LP-7 Kernel and acceptance, on OQ3

Not scheduled. On a yes it starts after HZ-3, HZ-4 and HZ-6.

| Row | Title | Proof |
|---|---|---|
| HZ-13, HZ-14 | Verdict and check schemas frozen. The kernel with truth tables. | schema pairs, closure test, API snapshot |
| HZ-15 (M3 half), HZ-16, HZ-18 | Soundness against plan values. Mutation operators (M5). Agreement on value checks (M6). | 1.000 on a literal basis and 0.99 on defaulted, 100% kill, a confusion matrix |
| LIB-10 | The draft acceptance test. Fidelity against labels fathom did not derive. Strength by derived mutants. A receipt that binds the check, label and catalog digests. A person still accepts the check. | a seeded misread sentence is refused. A draft with no outside labels is advice only |

### Dependency edges (ADR-114)

The CI kit needs direct file dependencies and an explicit per-file completeness
verdict for cache decisions. The standalone door supplies both, with unknowns and
provenance, while the CI kit's dependency graph owns closure and hashing.
The [dependency-edges plan](../plans/dependency-edges.md) owns the program and its
deferred audit, closure move and remaining dialects. Done when the door, Go control,
binary, three dialect readings and extension example have proof.

## 6. Where a hazard row differs from the policy review

The policy review that preceded the hazard design used PK- ids. The hazard plan uses the same numbers as HZ-. This table lists where this roadmap differs from what the hazard plan carries today. The hazard plan has absorbed these amendments.

| Row | The review wrote | Here | Why |
|---|---|---|---|
| HZ-1 | Proof: the IF-CM3 reading at its 0.85 floor | A reading row of its own against `cdk diff`, with a small harness | IF-CM3 in the plan is a cross-check that needs the Terraform scan. The live finding cannot wait for that |
| HZ-8 | IF-13, IF-6 and IF-11 as one row | The IF ids are used directly | An alias |
| HZ-5 | An additive fact, scheduled | ENTRY | The review's own M4 ranks E2 before it is built |
| HZ-7 | Suppressions as facts, scheduled | Register | Its consumer is the review kit, which has not asked |
| HZ-9 | "IF-14 as H1 on Terraform", with the M1 reading | The computation and the reading. The `review_diff` flag stays IF-14 on main and opens at 0.9 | The stop rule needs the reading before the surface |
| HZ-10 | H2 and H4 | Gains H5. Runs beside IF-13 | Catalog-free, and it gives the first reading |
| HZ-11 | H6, "needs IF-6" | Entry: two catalog versions on disk | IF-17 needs only IF-8, which is done. Operators can build catalogs today |
| HZ-12 | H3, H5, H8 | H5 moved to HZ-10. H3 and H8 to the register | No plan labels them, and the reading that would grade H8 (M6) is unscheduled |
| HZ-15 | Bench plan reader and M3 | Split. The reader is built with the M1 harness. M3 waits on OQ3 | M1 and M4 both need the reader |
| HZ-17 | M4, in the kernel phase | A scheduled reading lane | It grades the reader, not the kernel |
| HZ-20 | E4 or E5 | Register, on M4's result | |

HZ-2, HZ-4 and HZ-6 are as the review wrote them.

## 7. The register: futures with their triggers

Not roadmap rows. Each moves when its trigger fires. OQ15 satisfies T-IaC. Where an entry names a second condition, it still needs that one. The plan carries each as a deferred row.

| Entry | Trigger |
|---|---|
| H3 and H8 (the rest of HZ-12), with E12 | T-IaC |
| Pre-commit hook and pull-request check on the binary (CAP-29, CAP-30) | HZ-9 at 0.9 and an outside repository named in writing. IF-6 for the pull-request check. |
| Kubernetes and Helm contracts (HZ-21, IF-18 to IF-20, IF-M4). The census marks this planned (U-47). It waits here by LB-D6. | the fleet census finds a repository with manifests or charts, or a named adopter |
| Sizing of requests and limits against attested recommendations (RL-10) | a fleet repository with manifests or an adopter |
| Pulumi catalog and uses (IF-21, IF-22, IF-M5) | after HZ-21, same trigger |
| An IAM policy document model (HZ-22, E13) | F97's promote condition |
| A plan summary with every index step and instance key hashed (CAP-11, E19) | T-IaC, and a record change for the plan summary |
| Evidence-class annotation on other scanners' findings, through a review-kit producer entry (CAP-31) | the review kit asks in writing, and a reading shows a precision gain against plan truth |
| Suppressions as facts (HZ-7) | the review kit names it as a consumer |
| A context source in the read set (E18) | T-IaC and a platform repository in the index |
| Catalog diff as data (CAP-32) | IF-6 and IF-17 shipped |
| Lifetime contracts for dependencies (RL-7) | the dependency API index exists and RL-3 passed |
| Cloud resource and cost joins (RL-11) | T-IaC and main FCT-6 |
| A class conformance suite (CAP-28) | the library is released and one outside reader asks |
| Mutants as data (CAP-18) | an adopter with a policy suite asks, and attested test results are live (main CHG-4) |
| A cross-file resolver for config dialects (E16) | main's census for FCT-11 shows use |
| E4 or E5 (HZ-20) | M4 ranks one first |
| Enum, default and pattern in contracts (E9) | a licence check for the Terraform source, and a check that needs them |
| The product reads through the door (LIB-4) | an outside importer exists, and the binary and LL-M7 have missed an API gap |
| A public Go API for hazard facts | a consumer named in writing (F99) |
| The contract check as a binary subcommand outside the product (U-14). Its edge is narrow: it needs a small pinned catalog and no installed provider. | catalogs published and audited (IF-6, IF-11) and a consumer |
| Interface diff for Terraform modules and Helm charts between two tags (U-8) | T-IaC, or a module maintainer asks |
| The fact delta as a binary subcommand | LIB-13 landed and a consumer outside the product asks |
| The rest of the input inventory: base image pins, chart dependency versions, provider constraints (U-21) | a consumer, and a ruling on constraints, which are values the door withholds |
| A JSON Schema document for the fact JSON, for other policy engines (U-52) | an outside engine or the policy tool asks |
| A validity grader for model-written infrastructure code (U-56). A grader is an evaluator, which ADR-092 decision 10 leaves outside. | the policy tool or an evaluation builder asks |
| An estate inventory inside an air gap, with file classification by measured level (U-58, U-67) | an outside adopter. A candidate vehicle for OQ2 |
| A conformance kit that grades other readers by the library's method (U-60), with CAP-28 | the library is released and one outside reader asks |
| Structure in place of text for a third-party model (U-64) | an adopter with a data-egress rule asks |
| Test selection for infrastructure changes by touched resource (U-59) | T-IaC |
| Question and answer sets generated from facts, for outside use only (U-57) | OQ4 is answered and someone asks |

The three outside wedges the census ranks highest are the contract check outside the product, the module and chart interface diff, and the catalog as data with its diff. Each fails only the consumer clause.

Refused: a default rule pack, a built-in best-practice corpus, own per-function analyzers, control flow for languages other than Go, a language server, a library MCP surface, workflow and migration linting, leases on resource addresses, transpiling open rules into the kernel's algebra. The wins-if lines are in section 12 (RL-12 and the corpus) and in `docs/design/beyond-determinism.md`, which also holds the wins-if for transpiling. The record change for infrastructure tools as sources for the system graph rides CAP-1 in the main register.

### Adjacent capabilities (ADR-114)

Register only. Each entry needs a named consumer and its stated trigger before work starts.

| ID | Capability | Verdict or set | Trigger |
|---|---|---|---|
| ADJ-1 | Cacheability verdict | Per unit, cacheable or the nondeterministic inputs that forbid it: network fetches, unpinned images or actions, clock, random, `latest`. | First consumer stores a result cache. |
| ADJ-2 | Affected set | Units, tests, images and deploy targets downstream of a diff. | DEP-P1 lands. |
| ADJ-3 | Generated-file staleness | Generated files whose generator inputs changed since generation. | A gate failure caused by a stale generated file. |
| ADJ-4 | Manifest truth | Declared but unused dependencies, and imports without declarations. | Two DEP readings pass. |
| ADJ-5 | Import boundary rules | Violations of declared package or directory layering across languages. | A second repository asks. |
| ADJ-6 | Computed path filters | CI path filters derived from edges and checked against the authored filters. | The CI kit wires DEP output. |
| ADJ-7 | Build context minimum | Files an image build can read and gaps in the ignore file. | DEP-M3 passes. |
| ADJ-8 | Task graph | Task, script and workflow job relationships, dead tasks and local/CI parity. | Configuration reference resolution (E16) starts. |
| ADJ-9 | Environment surface | Environment variables read by code and build files, compared with declarations. | ADJ-1 starts. |
| ADJ-10 | Toolchain version agreement | Versions demanded by module files, tool-version files, images and CI setup, and disagreements. | A gate failure caused by version drift. |
| ADJ-11 | Public API delta | Per language, exported surface added, changed or removed between versions. | LIB-13 lands. |
| ADJ-12 | Minimal checkout | The sparse file set required by a unit. | A consumer builds or tests from a sparse checkout. |

## 8. Spin-off steps, in order

1. The licence notice (LIB-12). No archive ships without it.
2. The LL-M1 re-read (LIB-11), the tag (LIB-1) and the outside build reading (LL-M7).
3. The hazard entry in the binary (LIB-7), and with the proof program the public hazard benchmark: labels, method and the stated bias of synthesized state (main BEN-10). This is the library's first public artifact with an audience.
4. The reading benchmark under its three rules (main BEN-11, LIB-14).
5. The licence audit (IF-11), then published catalogs (IF-6).
6. On their triggers, which OQ15 helps and does not replace: the pre-commit hook and the pull-request check (an outside repository named in writing), the contract check outside the product (published catalogs and a consumer), the interface diff, the catalog diff.
7. On OQ3: the kernel and the draft acceptance test.
8. On their triggers: the schema document for other engines, evidence-class annotation, the conformance kit, the estate inventory.
9. Licence and module split, at ADR-092's recorded triggers (OQ4).

## 9. Kill and sunset dates

| Date or event | Rule |
|---|---|
| HZ-1 under 0.85 | The live CloudFormation finding drops to advice wording. It is fixed before any other hazard row |
| HZ-9 under 0.9 | No agent surface for Terraform hazards. The binary and the benchmark still state the reading |
| HZ-17 shows Trivy decides far more at equal soundness | E4 and E5 are not built. The advice to the policy tool is to take values from Trivy or a plan and hazards from fathom |
| IF-11 fails for a provider | That catalog stays operator-built |
| RL-4 audit precision under 0.7 or no gain over the tenancy lint, or RL-3 not above the union of the linters | That check is cut and its reading is published |
| 2027-01-31 | OQ3 and OQ15 are read with OQ2. With no answer the kernel stays unbuilt and the entries that need T-IaC stay closed |
| 2027-03-31 | The four standing refusals (`docs/design/beyond-determinism.md`) are re-read against their wins-if lines |
| 2027-05-04 | ADR-092's revisit: with no outside importer, stop adding public surface and keep what ships. On the same date the hazard bet is read: if HZ-9 has not reached 0.9, hazard work stops at the rows that feed the reading, and the reading is published as it stands |
| 2027-06-30 | The refusals on control flow beyond Go and a best-practice corpus are re-read |
| 2028-01-01 | F99: a further door with no named consumer is dropped |

## 10. Risks

- LB-R1 The hazard bet has no user in the fleet. Mitigation: it is proven by replay first, the benchmark is public, the census and OQ15 look for a user, and section 9 sets a date.
- LB-R2 Replacement facts are wrong where providers compute replacement in code. Mitigation: conditional reads unknown, and M1 reports precision before any agent surface.
- LB-R3 An incumbent ships plan-free replacement first. Mitigation: the catalog is the moat. IF-13 comes before any breadth.
- LB-R4 Replay needs synthesized state, which favours simple roots. Mitigation: the reading and the benchmark state that bias.
- LB-R5 The binary's JSON becomes a public promise by another name. Mitigation: it carries a schema version, the hazard entry is provisional, its shape is a main-roadmap contract, and no Go API is promised.
- LB-R6 A whole-program lifetime check costs a lot and loses to the linters. Mitigation: site facts and detector coverage come first because they are cheap, and each heavy check has a kill.
- LB-R7 The door cannot grow: its budget is spent. Mitigation: LIB-11 decides the bar before HZ-4 or HZ-6 add to the door, and the typed checks are hosted outside it.

## 11. The library's critical path

1. **The licence notice, a re-read of LL-M1, then the tag.** The notice is owed on every archive already shipped. Nothing outside can use the library until a tag exists, and ADR-092's revisit date is under six months away. The tag waits on one green wave on main.
2. **Read HZ-1, first in the first wave.** A finding built on it is already served to agents, and nobody knows if it is right. It needs a small harness and no product code.
3. **HZ-3, HZ-4 and HZ-6, then HZ-10 with LIB-7, with IF-13 built beside them.** Three fact rows and the catalog-free hazards. This is the first Terraform replay reading.
4. **HZ-9 at 0.9.** This one number gates the Terraform surface and the public benchmark.
5. **IF-11, then IF-6.** Catalogs a stranger can verify.
6. **Beyond hazards: the fact delta, and site facts with the tenancy lint ported onto them.** Both are small, and both have a reader in this repository today. Leak-detector coverage and the reading benchmark follow.

Everything after that is decided by who will use it. The census says whether the fleet has infrastructure code. OQ15 names an outside adopter. OQ3 says whether the kernel is built.

The library's potential is wider than change hazards. Eight primitives serve 67 judged uses, from review summaries and incident timelines to resource lifetime, supply-chain inventory and two public benchmarks. This roadmap schedules the part that has a reader today: hazards, the fact delta, lifetime facts and the benchmarks. It holds every other use with a trigger and a date, ranked, so that one named user is enough to open it.

## 12. Resource management, best practices and anti-patterns

The owner noticed that change hazards were the only library capability proposed, and asked about best practices, memory, connections, files, goroutines, locks, cloud resources, Kubernetes requests and limits, cost waste and anti-patterns. The hazard design refused code analyzers (N3, D12). That refusal was too blunt. This section is the answer, as revised after critique. Program LP-5 schedules it.

- RU1 This opens a new lane and needs a record, which is ADR-107. F90 rejects "a general best-practice linter" with "Wins if: never" and names fathom's lane as cross-file and cross-repo evidence, with dead code, architecture conformance and risk as its examples. Whole-program resource evidence fits that description but is not among the examples. So RL-3 and RL-4 need a short record that amends F90's rejection and the hazard design's N3 and D12, with their readings (RL-M1, RL-M2) as its kills.
- RU2 Three tiers. **Ingest and grade** what analyzers and runtimes already report, for every language. **Compute** where a typed whole-program graph and reach decide the answer, for Go first. **Compile once** project rules that the agent drafts and fathom admits.
- RU3 Fathom already built the first instance. Its own gate runs a tenancy lint that flags bare `go` statements, package-level maps, commands started without a context, a background context on request paths and whole reads in handlers, with an allowlist that may only shrink. That is RL-4 written by hand for one repository. The library version is the same detections as facts, so any repository can state its own rule. It is also the bar: a computed version must beat that lint on fathom's own history.

The facts the library needs, cheapest first:

| Step | Facts | Where | Cost |
|---|---|---|---|
| L-a | Site facts from syntax: acquire candidates, release calls, `defer`, `go` statements, lock and unlock, loops that contain them, scoped constructs (`with`, try-with-resources, `using`) | The door. Go's reader already walks each function with `go/ast` and emits none of these. Other languages as query-pack data lanes | Small. No new dependency. New fact kinds need a graded reading (I54) |
| L-b | Typed pairing for Go: release methods found from method sets, then path-sensitive "released on every path, not on some path, or escapes" over SSA | The product's typed pass. Not the door: it needs `go/packages`, which runs the toolchain, and the door's budget is 80 of 80 | Unmeasured. The typed pass runs at ingest and keeps type information for root packages only. `go/ssa` ships in a module already required and only a bench tool uses it. A cost reading comes before entry |
| L-c | Whole-program joins for Go: long-lived containers with their writers and deleters, spawn sites with a visible bound, reach from routes and `main` (forward reach and route readers exist) | The product, at ingest, since "no delete anywhere" is a whole-program fact | Unmeasured. The store's cost gate applies |
| L-d | Runtime evidence as attested input: leak profiles, race reports, heap profiles, mapped from frames to symbols | The attested evidence path. The race flag is already attested. The frame parser reads pasted traces, not profiles | Small after FCT-6 |

For languages other than Go: L-a and ingest only. Escape is always an unknown with a reason, never a pass.

| Id | What | Path | What fathom adds that the incumbent lacks | Measured against the incumbent | Mark and entry |
|---|---|---|---|---|---|
| RL-1 | Diagnostic facts: output of existing analyzers as attested facts, graded per analyzer and rule by the share of its diagnostics that history shows were fixed, suppressed or left | 3 | Reach, tests that read the code, change scope by reachability and not only by line, a grade per rule, receipts | Precision at k against the analyzer's own order and against new-lines-only filtering, on fix commits. Kill: no gain | ENTRY: FCT-6. Main roadmap. The fleet's gate runs `go vet`, the race detector and the tenancy lint, and no general linter set. Adding one creates the input |
| RL-2 | Lifetime site facts in the door (L-a) | 1 | A per-language, graded fact any consumer can read. The tenancy lint becomes its first consumer | RL-M0: site facts against the tenancy lint's own findings on fathom, and against SSA truth for Go | ENTRY: one of RL-3 to RL-5 or the lint's port is scheduled. Library row |
| RL-3 | Acquire and release pairing for Go: files, connections, response bodies, rows, cancel functions, tickers, locks. Three-valued. A constructor that returns an open resource passes the duty to its caller along typed edges | 1 | Cross-function duty, honest escape. The common linters are per type and per function and admit false positives `[web]` | RL-M1: public fix commits that add a missing release are the labels. Recall on the parent at matched noise on the fixed head, against the union of the linters. No published result shows whole-program beating them `[web]`, so this can lose. Kill: not above the union | ENTRY: ADR-107, the engine presence bar (PG-1), a cost reading for L-b, RL-M1 preregistered. Library track, typed pass |
| RL-4 | Growth and bound hazards for Go (memory): a request-reachable insert into a long-lived container that nothing deletes from, a request-reachable spawn with no visible bound, a `defer` or timer made in a loop, work started with nothing that can stop it | 1 | The whole-program view and reach. The tenancy lint sees the site but not "no delete anywhere" or reach | RL-M2: must beat the tenancy lint on fathom, and the runtime's own leak profile on the goroutine half (experimental in Go 1.26 `[web]`). Labels: the blocking subset of a public concurrency-bug suite (103 kernels, 82 real bugs `[web]`) and public fix commits. Fathom's own history offers no labeled product case yet. Kill: audit precision under 0.7, or no gain over the lint | ENTRY: ADR-107, RL-2, a cost reading for L-c at ingest. Library track |
| RL-5 | Leak-detector coverage: which spawn sites are reached by a test that ran under a leak detector. Race coverage is not a separate number, because the gate runs every test under the race detector, so it equals test reach | 1 over 3 | Dynamic detection is what works in practice (one large fleet reports about 260 leaks blocked in a year by a test-time check `[web]`). The deterministic part is proving where it is not looking. No leak detector is in use in the fleet today, so the number starts at zero | Mutants that remove a join or a stop: the share killed only under the detector | ENTRY: RL-2, and the operator turns on the runtime's leak profile in tests. Fathom runs nothing |
| RL-6 | Runtime profile attribution: attested heap and leak profiles joined to symbols, diffed across two commits, attributed to the change and the owner | 3 | A profiler shows where. This shows which change and who | Hand audit on the fleet's own daemon, whose memory is a standing problem | ENTRY: FCT-6, profile input for the frame parser |
| RL-7 | Lifetime contracts for dependencies: per version, which calls acquire and what releases them. The same shape as a contract catalog | 1 | Covers every library in the closure, not a hand list of types | Agreement with the linters' built-in lists on the types they know | FUTURE: the dependency API index exists and RL-3 passed |
| RL-8 | Project rules compiled once, for anti-patterns and best practices. The agent drafts a rule from an incident or a review comment, for whichever engine fits. Fathom is the admission harness: the rule fires on the commit that caused the incident, stays silent over a replay of history, and kills derived mutants | 2 | Fidelity from a label fathom did not derive, noise from replay. Rule tests elsewhere are fixture annotations `[web]`, and no replay or bad-commit admission was found (search incomplete) | Rules admitted by the harness against rules with fixture tests only: false alarms on held-out history, recall on seeded mutants | ENTRY: one incident to start. Syntactic rules only at first, because today's replay parses without types on purpose (old trees may not load). Main roadmap |
| RL-9 | Call-shape rule kinds as named kinds in Go: forbidden call except in, companion call, must call before, requires argument. Start syntactic, as the tenancy lint does. Typed versions are gate-only, since ADR-038 refuses edit-time enforcement for kinds that need the full index | 1 | Project rules any repository can state, not one hand-written lint | Replay noise, and what the typed kind catches that syntax misses | ENTRY: port the tenancy lint's checks to rule data first. A typed kind needs two rules that syntax cannot express |
| RL-10 | Kubernetes requests, limits, probes and replicas. A change delta for these exists, carries before and after values, and already feeds a change-risk tierer that judges it. A fathom hazard on removal would be a resource rule, which the recorded design gives to the policy tool. Sizing: join attested recommendations to the manifest span | 3 | Declared against observed on one span | Against the recommender's own export | NONE to add for change. Sizing FUTURE: a fleet repository with manifests or an adopter. The owning program has a kill date of 2027-01-31 |
| RL-11 | Cloud resources and cost waste: join attested exports (resource hygiene, cost breakdowns, the common cost export format) by resource identity. Declared and idle, observed and undeclared | 3 | The join to declaration, owner and change | Hand audit of the join | FUTURE: T-IaC and FCT-6. Declared waste shapes stay V checks for the policy tool |
| RL-12 | Control flow and def-use for languages other than Go | 4 | | | REFUSE own build. Site facts and ingest instead. Wins if: typed edges are default for three languages and a consumer fleet is not Go. Re-read 2027-06-30 |
| | A built-in best-practice corpus | 4 | | | REFUSE (F90). Wins if: a fleet language has no analyzer worth ingesting. Re-read 2027-06-30 |

Lock order across call edges needs "locks held at a call", which RL-3's SSA pass can state. It is a FUTURE row behind RL-3.

How the owner's list maps: best practices and anti-patterns are RL-1, RL-8 and RL-9. Memory is RL-4 and RL-6. Connections and files are RL-3 and RL-7. Goroutines are RL-4, RL-5 and RL-6. Locks are RL-3. Kubernetes is RL-10. Cloud resources and cost are RL-11.

Order: RL-2 with its reading and the lint's port (RL-9), because that has a consumer and a bar today. Then RL-5. Then RL-4 and RL-3, each after the record and a cost reading. Only RL-2 is door surface. None of this runs on the hook path.

### Engine gaps added to E1 to E13

- E14 folded into E7 for infrastructure. For code it is the internal delta of CAP-14.
- E15 Key emitters from infrastructure reads into the system graph's source port.
- E16 Cross-file reference resolution for config dialects. Only the CUE family links today.
- E17 `decidable_by` as a closed list in the unknown reason.
- E18 A context source in the read set, with its digest in the receipt.
- E19 A plan summary with every index step and instance key hashed.
- E20 Rebuild the proposed file inside the hook from the edit fragments, for every harness's edit forms.
- E21 A cold-latency reading per tree-sitter dialect inside the hook budget, with no daemon.
- E22 Lifetime site facts from the Go reader's existing function walk, then query-pack captures per language.
- E23 An SSA pass for pairing in the product's typed pass, and whole-program site joins at ingest. Each needs a cost reading first.
- E24 Profile input for the frame parser, inside the attested evidence envelope.
- E25 Rule replay widened from Go import rules to any admitted rule.

## Decision models

None applies to the roadmap itself. It schedules deterministic work: facts, deltas, change sets and coverage numbers, with provider `none` as the whole feature and nothing to opt out of. Two rows touch a judged site and both keep a person in the loop. LIB-9 lets a model propose stateful type candidates, a person accepts each into the list the catalog builder compiles, and the fallback with provider `none` is the published list alone. LIB-10 and RL-8 let an agent draft a check or rule, which is generation outside the port's predicate, choice and score kinds. Fathom admits a draft only through a deterministic acceptance test, and a person still accepts it. A model never resolves an unknown or judges a security property (I62). If a judged site appears later it follows ADR-073 as written, with its own kill measurement. The measurement for LIB-9 is agreement of the accepted list with a published list or a hand key on a stated sample.

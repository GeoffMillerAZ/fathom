# Library uses: a census of every potential use of the standalone library

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Verdict: the library has more than sixty plausible uses and six primitives behind them. Almost every use with a consumer was already on a roadmap. The sweep's real finds are a licence notice that is owed today (U-24), an analyzer nobody calls (U-61), and a list of outside wedges that wait on one thing: a named user.

Links: vision `docs/vision/46-proof-at-the-moment-of-change.md`, library roadmap `docs/design/library-roadmap.md`, hazard engine `docs/design/change-hazards.md`, win paths `docs/design/beyond-determinism.md`.

The question: a wide sweep of uses for the language and tool understanding library (`pkg/lang`, `pkg/lang/iac`, catalogs, `fathom-lang`, `langtest`), inside fathom and outside it, before judging.

Method: five low-cost sweepers produced 90 candidate uses across 25 areas with no judging. They were merged and each was judged. One critic then tried to refute the result, corrected eleven rows and named eight uses the sweep missed. The census has 67 rows. Tags: `[code]` read in the repository, `[web]` checked at a source, `[prior]` unverified.

How each use is judged:

- **Case** is the four-case rule (vision 46): 1 a verdict or a set at a forced channel or verb, 2 structure inside the agent's own action, 3 an optional tool, 4 pushed prose.
- **Fails** names the admission clause that fails (vision 46): 1 computable, 2 verdict or set, 3 existing channel, 4 outside oracle, 5 a named consumer.
- **Path** is 1 deterministic, 2 hybrid, 3 pair, 4 refuse with a wins-if.
- **Mark**: NOW, PLANNED (already a row), ENTRY (scheduled behind a condition), FUTURE (register entry with a trigger), REFUSE (with a wins-if and a re-read date), NONE (nothing to build).
- **T-IaC**: a fleet repository with Terraform or CloudFormation, or the policy tool importing the library, or an outside adopter named in writing.

Ids from other documents: CAP and RL are in part 2 below and in the library roadmap. HZ is the hazard plan (`docs/plans/change-hazards.md`). H and V are the policy review's hazard and value checks. The rest are in the vision documents and plans. The ranked list and the roadmap rows are the last two sections of part 1.

# Part 1. The census

## 1. What the sweep shows before any judging

- F1 **Most uses reduce to six primitives.** A fact delta between two versions (9 swept uses). An
  inventory of external inputs with pin state (10). Hazard and contract checks (9). Upgrade impact between
  two versions of a contract (6). Reference resolution for configuration (6). The fact JSON as input to
  someone else's engine (5). Build a primitive once and most of the census follows.
- F2 **The library's edge is almost always the same three things**: offline with no credentials, one
  shape across many file kinds, and unknowns stated. It is almost never "smarter than the incumbent".
- F3 **Where a use needs a rule, a ranking or prose, the library loses.** Those go to pairing or refusal.
- F4 **Clause 5 fails for every outside use.** No outside consumer is named for any of them today.
- F5 **Several "new" edges were not edges.** SBOM tools already catalog workflow actions and locked
  providers. Small interface-diff tools exist for modules and charts. The tool's own validate matches the
  contract check once providers are installed `[web, code]`.

## 2. The census

### Code review

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-1 | A structural change summary of a diff: symbols, edges and dialect facts added, removed or changed. For the review verb and for a pull-request comment. | 1 | 1 | 5 outside | ENTRY: a what-if card passes replay | CAP-14 |
| U-2 | Contract-surface touch set: which rpc, operation, column or exported symbol a diff changes, to route review and feed the risk feature file | 1 | 1 | | ENTRY: with U-1 | ADR-088 decision 5 |
| U-3 | Infrastructure hazards as review findings | 1 | 1 | 5 in the fleet | PLANNED. The public benchmark is its consumer today | HZ-9 to HZ-12, CAP-25 |

### Refactoring and codemods

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-4 | Every reference site of a config identifier, before a rename | 1 | 1 | 5 weak | FUTURE: the config resolver exists | CAP-34, E16 |
| U-5 | Span-accurate targets for bulk edits of non-code files | 3 | 3 | 5 | NONE. It is what `Extract` already returns. Update bots and pattern tools own the edit | |
| U-6 | Candidate module cuts for splitting a codebase | | 4 | 2, 4 | REFUSE: judgment over name-level edges. Wins if: typed edges are default for three languages. Re-read 2027-06-30 | |

### Migrations and upgrades

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-7 | Provider upgrade impact on attributes in use | 1 | 1 | | PLANNED | IF-17 |
| U-8 | Interface diff for Terraform modules and Helm charts between two tags: variables removed or newly required, outputs removed, values keys removed. Small tools exist for each side `[web]`. None covers both in one offline shape. | 1 | 1 | 5 | FUTURE: T-IaC, or a module maintainer asks | E3, CAP-14 |
| U-9 | Kubernetes API versions removed in a target release | 1 | 3 | 5 | FUTURE. A free tool owns it `[web]` | CAP-3 |
| U-10 | Inventory of uses of a deprecated library or API, to size a migration | 1 | 1 | | PLANNED. Name-level outside Go | SG-39 |
| U-11 | Cross-provider or cross-tool migration map | | 4 | 1 | REFUSE: needs a cross-tool identity table, which F97 forbids. Re-read when IF-M2 reads | |

### Best practices and resource management

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-12 | Lifetime site facts, acquire and release pairing, growth hazards, leak-detector coverage. The fleet's own tenancy lint is the first instance `[code]`. | 1 | 1 | | ENTRY | RL-2 to RL-5, RL-9 |
| U-13 | Analyzer and runtime output as graded facts | 1 | 3 | | ENTRY: FCT-6 | RL-1, RL-6 |
| U-14 | A contract check of infrastructure code: attribute unknown to the provider, required and unset, deprecated, sensitive value in an unmarked output. It exists inside the product `[code]`. Outside, its edge is narrow: it needs a small pinned catalog instead of an installed provider, and the tool's own validate matched it 23 of 23 once installed `[code]`. | 1 | 1 | 5 outside | PLANNED inside (IF-M1). Outside FUTURE: catalogs published and audited (IF-6, IF-11) and a consumer | IF-M1 |
| U-15 | Resource hygiene reports (limits, probes, retention, lifecycle) | 1 | 3 | 5 | FUTURE. Value checks belong to the policy tool | RL-10, V checks |
| U-16 | Pipeline hygiene (timeouts, concurrency groups) | | 3 | 5 | REFUSE: a linter owns it `[prior]`. Wins if: a consumer fleet's CI dialect has no linter. Re-read 2027-06-30 | |

### Security and secrets

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-17 | Literal on a contract-sensitive attribute, value never read | 1 | 1 | | PLANNED | H8, HZ-12 |
| U-18 | Insecure defaults (public, open ingress, unencrypted, privileged) | 1 | 3 | | NONE in the library. Scanners and the policy tool own it | A-N1 |
| U-19 | Workflow supply-chain risks (privileged triggers, broad permissions) | | 3 | 5 | REFUSE: a security rule, and a free tool owns it for one CI system `[prior]`. Wins if: a consumer fleet uses a CI dialect with no such tool. Re-read 2027-06-30 | |
| U-20 | Attack-surface map: ports, ingresses and public endpoints joined across files | 1 | 1 | 4 | FUTURE: typed links exist. Name joins are advice only (I57) | CAP-1 |

### Supply chain, SBOM and licences

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-21 | One inventory of external inputs with pin class. Only workflow actions carry a pin class today (`pin_sha`, `pin_ref`) `[code]`. Base image tag or digest, chart dependency versions and provider constraints are new facts, and constraints are values the door withholds, so they need a ruling. SBOM tools already cover actions and locked providers `[web]`. What is left is chart dependencies, module sources and base image pins. | 1 | 1 and 3 | | ENTRY for the actions part: UND-1, through the fleet's standards. The rest FUTURE: a consumer | CAP-4, CAP-8 |
| U-22 | Pin fixes as edits | | 3 | 1 | REFUSE: needs a registry lookup and the library never dials. Update bots own it. Wins if: never | |
| U-23 | Declared build inputs to cross-check an attested build | 1 | 1 | 5 | FUTURE: signed releases with provenance exist (WIR-6) | |
| U-24 | **A notice file for what the binaries embed.** Every release archive since v0.13.0 embeds grammar blobs (55 since v0.23.0, 54 MIT and one Apache-2.0) and ships only the product licence and the readme `[code]`. The lock file already records each licence. Both licences require their notices to travel with the copy. | | | | **NOW.** Generate the notice from the lock and add it to the release archives | I55 |
| U-25 | Dependency licence inventory seed | 1 | 3 | | PLANNED: the dependency licences design (0 of 12 rows done `[code]`) | U-21 |
| U-26 | Licence header presence | | 3 | 5 | REFUSE: a free tool owns it `[prior]`. Wins if: never | |

### Maps, ownership and architecture

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-27 | Static maps: selector to workload, image to build file, resource reference graph, and gateway or proxy rule to service (oracles already grade that last join `[code]`) | 1 | 1 | | PLANNED and FUTURE | system graph rows, CAP-1 |
| U-28 | Alert to owner trace: from an alert rule or dashboard panel to the service and repository path | 1 | 1 | 4 | FUTURE: with the system graph | |
| U-29 | Owner at the symbol and resource level, and declared surface with no owner | 1 | 1 | | PLANNED | FCT-5, CAP-8 |
| U-30 | Import-direction rules beyond Go | 1 | 1 | 4 | ENTRY: typed edges for other languages (FCT-1). Heuristic edges cannot gate (I57) | layer rules |
| U-31 | Declared against actual service map, schema against code | 1 | 1 | 4 | FUTURE: the system graph's table and config sources | SG-19 to SG-21 |
| U-61 | Config value drift across services: a key whose value deviates from the fleet majority. An analyzer exists and nothing in the product calls it `[code]`. | 1 | 1 | | ENTRY: wire it under fleet standards (UND-4), or cut it in the subtraction program | CAP-8 |

### Documentation, diagrams and education

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-32 | Generated diagrams | 4 | 4 | 2 | REFUSE: a rendering, not a verdict or a set. Wins if: the fleet view needs one. Re-read with PG-6 | |
| U-33 | Docs drift: names in prose that no longer exist. The product has a docs-drift analyzer, wired `[code]`. The library widens the truth side to config dialects. | 1 | 1 | 5 weak | FUTURE: a custodian kind needs it | |
| U-34 | Tours and glossaries | 4 | 4 | 2, 5 | REFUSE: prose with no shown benefit (V-N2). Wins if: an educator asks in writing. Re-read 2027-06-30 | |

### Tests

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-35 | Static test selection by imports, any language | 1 | 3 | 4 | REFUSE own build: build tools own it and the product selects by what tests read. Wins if: the test-selection readings fail and a consumer fleet is not Go. Re-read after TQ-M7 | ADR-090 |
| U-36 | Which CI job runs which task target and script | 1 | 1 | | ENTRY: with the config census | CAP-16 |
| U-37 | Test skeletons | 4 | 4 | 2 | REFUSE: generation. The agent does it. Wins if: never | |
| U-38 | Declared surface with no test reference or no handler (operations, rpcs, tables) | 1 | 1 | 4 | FUTURE: name-level only today, so advice | |
| U-39 | Fixtures and mutants for readers and checks | 1 | 1 | | ENTRY | M5, CAP-27 |
| U-59 | Test selection for infrastructure changes by touched resource, not by path glob | 1 | 1 | 5 | FUTURE: T-IaC | CAP-19 |

### Incidents and postmortems

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-40 | A what-changed timeline between two commits, in facts: resources, images, replicas, limits, pipeline steps | 1 | 1 | | ENTRY: with U-1. The incident procedure is the consumer | CAP-14 |
| U-41 | Recurrence guard: an incident becomes a rule that is admitted once and runs forever | 1 | 2 | | ENTRY | RL-8 |
| U-42 | Everything that references a name a diff removes, across code, pipelines and charts | 1 | 1 | 4 | FUTURE | CAP-34 |

### Cost and compliance

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-43 | Cost drivers handed to a pricing tool | | 3 | 5 | REFUSE: the pricing tool reads the code itself. Join its export instead (RL-11). Wins if: never | |
| U-44 | Dead configuration as waste | 1 | 1 | | ENTRY: with the config census | CAP-16 |
| U-45 | Static pipeline cost reduction | | 4 | 4 | REFUSE: a guess without run data. Wins if: attested run durations are ingested. Re-read with FCT-6 | |
| U-46 | Evidence per control, with spans, class and read set. The mapping from control to fact is the policy tool's (ADR-088 decision 4). | 1 | 1 | 5 | FUTURE: T-IaC, and a yes to OQ3 | HZ-3 |

### Kubernetes, CI packaging, editors and agents

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-47 | Kubernetes and Helm in the infrastructure reader, with the same classes | 1 | 1 | | PLANNED | LIB-5 |
| U-48 | Hazards and the contract check as a pull-request check and a pre-commit hook | 1 | 1 | 3, 5 | FUTURE: packaging, on a named outside repository | CAP-29, CAP-30 |
| U-49 | A language server, an outline provider, inline class hints | 3 | 4 | 5 | REFUSE: the tool vendor's server owns the editor `[web]`. Wins if: an editor maintainer asks in writing. Re-read 2027-05-04 | |
| U-50 | A standalone MCP server for reads and contract lookups | 3 | 4 | 3 | REFUSE: an optional tool, and ADR-091 decision 7 says no new tool. Wins if: the deferred agent reading (IF-M6) shows contract lookups prevent errors | |
| U-51 | A pre-write contract and hazard check on proposed text | 1 | 1 | | ENTRY | CAP-15, HZ-19 |
| U-66 | Per-file outlines for the agent's own reads. The product already uses the library's extractors for this under a 150 ms deadline `[code]`. | 2 | 1 | | PLANNED. Default off until the in-band reading | PG-11 |

### Other engines, data and other organizations

| Id | Use | Case | Path | Fails | Mark | Maps to |
|---|---|---|---|---|---|---|
| U-52 | The fact JSON as input to other policy engines. The JSON is already a compatibility surface with goldens, a schema version and an additive-only rule `[code]`. A JSON Schema generated from the goldens would be a document, not new surface. | 1 | 3 | 5 | FUTURE: an outside engine or the policy tool asks. No new surface after 2027-05-04 without a consumer | A-N1 |
| U-53 | Replacement and write-only flags as data for other engines | 1 | 3 | 5 | PLANNED | the catalog spin-off |
| U-54 | Policy test fixtures for other engines | 1 | 3 | 5 | FUTURE | CAP-18 |
| U-55 | A reading benchmark: a model's reading of a config or infrastructure file graded against the tool's own evaluator. No public benchmark of this kind was found `[web]`. As the oracles stand it would test exhaustive listing, where an index wins by construction. To test the thesis it must use preregistered questions an agent actually needs, native-tier oracles only, and no oracle written by fathom's authors. | 1 | measure | | ENTRY: BEN-5, the paid gate, those three rules | V-M6 |
| U-56 | A validity grader for model-written infrastructure code, and teaching feedback on a learner's file. A grader is an evaluator, which ADR-092 decision 10 leaves outside. | 1 | 1 | 5 | FUTURE: the policy tool or an evaluation builder asks | U-14 |
| U-57 | Question and answer sets generated from facts. Inside: refused, because bench truth never comes from fathom's index. Outside: FUTURE | 1 | 1 | 4 inside, 5 outside | FUTURE: the licence question (OQ4) is answered and someone asks | |
| U-58 | An estate inventory inside an air gap, across many repositories | 1 | 1 | 5 | FUTURE: an outside adopter. A candidate vehicle for the outside-team question (OQ2) | U-21, CAP-4 |
| U-60 | A conformance kit that grades other readers by the library's own method | 1 | 1 | 5 | FUTURE | CAP-28 |
| U-62 | An offline provider binding resolver: locked, constraint-matched, ambiguous or unbound, per block. It exists inside and has no door `[code]`. Upgrade and pin tools could use it. | 1 | 1 | | PLANNED | HZ-4, E3 |
| U-63 | Reading untrusted input safely. The binary reads in a supervised child under a hard deadline `[code]`. That is why it can run on a pull request from a stranger. | | | | NONE. It exists. It is an edge for U-48 | |
| U-64 | Structure in place of text for a third-party model. The door drops secrets the source marks and credentials in URLs `[code]`, so a file's structure can be sent where its text must not go. | 2 | 1 | 5 | FUTURE: an adopter with a data-egress rule asks | |
| U-65 | Breaking-change checks for exported Go API | 1 | 3 | | NONE. The product has the surface delta. Outside, the language's own tool owns it `[prior]` | |
| U-67 | File classification with a measured level per dialect, for corpus and estate tools | 1 | 1 | 5 | FUTURE: with U-57 or U-58 | |


## 3. Counts

67 rows. NOW 1. PLANNED 12. ENTRY 14. FUTURE 22. REFUSE 14. NONE 4.

Thirteen of the fourteen ENTRY rows were already capabilities in part 2 or rows in the
vision documents. What this census added: one obligation that is live today (U-24), one wire-or-cut finding
(U-61), a sharper rule set for the reading benchmark (U-55), and nine register entries (U-8, U-52, U-56,
U-58, U-59, U-60, U-64, U-67 and the outside half of U-14).

## 4. Ranked

First, outside the ranking because it is an obligation: **U-24, the notice file.** Do it before the next
release archive ships.

Ranked by value, with a named consumer first (clause 5):

1. **The fact delta (U-1, U-2, U-40).** Nine swept uses reduce to it. Consumers: the review verb, the
   incident procedure, the bench's collateral-damage outcome.
2. **Change hazards through their public benchmark (U-3, CAP-25).** Still the one thing nobody else
   computes. The proof program is the consumer. The fleet has no infrastructure code of its own.
3. **Resource lifetime, starting from the tenancy lint (U-12, U-13).** The fleet is Go and already runs
   the first instance.
4. **The reading benchmark (U-55), under its three rules.** A test of the thesis on ground fathom does
   not own.
5. **The actions half of the input inventory (U-21).** Existing facts, consumed by fleet standards.
6. **Configuration reach (U-36, U-44).** Dead config, and which job runs which target.
7. **The recurrence guard and derived mutants (U-41, U-39).**

Then three wedges with value and no consumer yet. They are register entries, not roadmap rows:

8. **The contract check outside the product (U-14).** Narrow edge, real setting: a place where providers
   cannot be installed.
9. **Module and chart interface diff (U-8).**
10. **The catalog as data, with its diff (U-53, CAP-32).**

## 5. Which become library roadmap rows

Ids are minted when the rows land.

| Row | Mark and entry | Proof |
|---|---|---|
| A notice file generated from the grammar lock, in every release archive (U-24) | NOW | the release check fails without it |
| A delta of two library results, internal, code card kinds first (U-1) | ENTRY: a what-if card passes replay | delta tests, and the bench's collateral-damage reading uses it |
| A hazard entry in the library binary (U-3) | ENTRY: HZ-6, HZ-9 | the replay reading runs through it |
| Lifetime site facts, with the tenancy lint ported onto them (U-12) | ENTRY: per section 5a | RL-M0 against the lint's own findings |
| The public binding resolver (U-62) | PLANNED as HZ-4 | reader tests, API snapshot |
| Fixtures and oracles for the reading benchmark, under its three rules (U-55) | ENTRY: BEN-5 | the benchmark's fathom arm |

Main roadmap, not library rows: the actions inventory as a named query (U-21), configuration reach
(U-36, U-44), wire or cut the config drift analyzer (U-61), the recurrence guard (U-41).

Register only (FUTURE): U-4, U-8, U-9, U-15, U-20, U-23, U-28, U-31, U-33, U-38, U-42, U-46, U-48, U-52,
U-54, U-56, U-57, U-58, U-59, U-60, U-64, U-67, and the outside half of U-14.

In one line: the library has more than sixty plausible uses and six primitives
behind them. Almost every use with a consumer was already on a roadmap. The sweep's real finds are a
licence notice that is owed today, an analyzer nobody calls, and a list of outside wedges that wait on
one thing: a named user.

# Part 2. Capabilities by plane

The library roadmap holds the capability rows (CAP-1 to CAP-36). This part copies the patterns and the per-plane tables with their marks, so that this assessment can be read alone.

## 6. Three patterns

- P1 **Contract, classed diff, reach.** The hazard engine is one instance of a shape. The dependency
  plane's upgrade impact (SG-39) and API contract drift are others. Both the first two are unbuilt, so
  nothing is shared yet. Extract the shape when two instances ship (P20). Never a general contract
  registry (F97).
- P2 **A pure read needs no engine.** Graph answers are declined on the hook path. A library read needs
  no index and no daemon, and the hook's enforce path is already local text `[code]`. Narrowed by SC4 of the library roadmap: the
  native readers (Go, HCL) fit. Tree-sitter dialects need a cold-latency reading first. The proposed file
  must be rebuilt inside the hook.
- P3 **Unknowns are routable.** An unknown reason names the cheapest action that would decide it. The
  agent runs it. Fathom runs nothing.

One trigger recurs. **T-IaC**: a fleet repository with Terraform or CloudFormation, or the owner's policy
tool importing the library, or an outside adopter named in writing.

## 7. Capabilities by plane

### Facts

| Id | Capability | Clause 5 consumer | Mark | Needs |
|---|---|---|---|---|
| CAP-1 | Extend the system graph's env, config, image and deploy sources (SG-20, SG-21) to Terraform, CloudFormation and Pulumi. A same-file literal gives a literal-tier key that never gates (I57). Anything else is an unknown. | none today | FUTURE: T-IaC, the freeze amendment, SG-10 | E2, E11, E15 |
| CAP-2 | Hazard reach: a replaced table joined to the services and owners that use it. | none today | FUTURE: CAP-1, and topic and table links pass their oracle | HZ-9 |
| CAP-3 | One upgrade-impact shape for provider catalogs (IF-17) and dependency versions (SG-39). | none until both exist | FUTURE: both rows shipped. A Kubernetes API family later, with pluto as oracle `[web]` | |
| CAP-4 | Tool and dialect census as a named query: which tools, levels and provider bindings the fleet uses. It orders language lanes and catalog publishing by use. | the language and catalog lanes | ENTRY: UND-1 | E3 for bindings |
| CAP-5 | Deployables in the computed catalog | | PLANNED: FCT-9. Add workloads when E11 lands | |

### Intent

| Id | Capability | Clause 5 consumer | Mark | Needs |
|---|---|---|---|---|
| CAP-6 | Declared destruction. A hazard reads "declared" when a plan row already declares the removal. The plan grammar has the keys (`retires`, `surface_change`) and conservation reads them `[code]`. Make resource addresses inventory keys. No new key. The undeclared hazard is the alarm. | none today | FUTURE: HZ-9 at 0.9 and T-IaC | E7 |
| CAP-7 | Conservation for infrastructure refactors: only moves covered by `moved` blocks and no attribute change. Compare a normalized token digest computed inside the library and never served. Span text would flag a reformat and would carry values. | none today | FUTURE: HZ-4, HZ-6, T-IaC | E3, E7. An ADR-092 decision 7 note |
| CAP-8 | The computed half of fleet standards, from facts that exist: share of actions pinned by commit, lockfile and provider constraint present, owners coverage. Sets and shares. The review kit judges any security threshold. | the fleet uses Actions. UND-4 | ENTRY: REC-10, UND-4 | a fleet runner (the audit is per repository `[code]`) |
| CAP-9 | Lifecycle conditions that read the census: "a second repository uses this tool", "provider major at least N". | records with such revisit lines | ENTRY: REC-4, and a count that shows at least five such lines | CAP-4 |

### Work

| Id | Capability | Clause 5 consumer | Mark | Needs |
|---|---|---|---|---|
| CAP-10 | A hazard section in the landing bundle | none today | FUTURE: CHG-5, HZ-9, T-IaC | |
| CAP-11 | A lane's own attested plan grades the plan-free verdict. Replace paths carry map keys and set elements `[web]`, so the summary must hash every index step and instance key. | none today | FUTURE: T-IaC, and a record change for the plan summary | E19 |
| CAP-12 | Lane overlap by resource reach | none today | FUTURE: two concurrent infrastructure lanes exist | E3 |
| CAP-13 | Upgrade worklist grouped by file and owner | none today | FUTURE: IF-17, IF-6, T-IaC | |
| | Leases or locks on resource addresses | | REFUSE. Fathom reports and never acts. Wins if: never | |

### Proof

| Id | Capability | Clause 5 consumer | Mark | Needs |
|---|---|---|---|---|
| CAP-14 | A fact delta of two versions of one file. An index-free one exists for Go-centric kinds (`inventory.Extract`, `inventory.Diff`) `[code]`. Build it internal, inside the replay work for the what-if cards, for the code card kinds first. For infrastructure it is E7 (HZ-6). Not in the door. | CHG-1 | ENTRY: a CHG-1 card passes replay | E20, E21 |
| CAP-15 | Two tiers for what-if verdicts. Delta tier: needs the file or the module directory and no graph, so it does not wait on engine presence. Reach tier: needs the graph. Delta tier today: removed exported symbol or route (Go), and later H5. H2 and H4 need the directory and E3. H8 needs the binder and the operator's catalog. | CHG-1, CHG-2 | ENTRY: the replay card, with lower noise than post-write scanners. ADR-091 decision 7 for the infrastructure card | E3, E20, E21 |
| CAP-16 | A `referenced` criterion for configuration symbols. A plan row can already bind one, and `wired` reads "not applicable" outside Go `[code]`. `referenced` never satisfies `wired` and never gates on a configures link (I57). | verify, if rows bind config symbols | ENTRY: a census of such rows, then E16 | E16 |
| CAP-17 | Proof liveness for library outputs | | PLANNED: FCT-8 and BEN-9. Add catalog digest and version binding to what a hazard verdict binds | |
| CAP-18 | Mutants as data, so an adopter's own pipeline grades its scanners and tests. No such tool was found for policy suites `[web]`. | none today | FUTURE: an adopter with a policy suite asks, and CHG-4's attested path is live | |
| CAP-19 | Tests selected by reads, for infrastructure tests. Fixture reads inside a Go test process are seen today. A test that shells out is not (TQ-M8) `[code]`. | none today | FUTURE: a non-Go runner's read log, or TQ-40's own trigger | |

### Understanding and delivery

| Id | Capability | Clause 5 consumer | Mark | Needs |
|---|---|---|---|---|
| CAP-20 | Infrastructure question shapes as a family on the understanding bench: what does this change destroy, what breaks on upgrade to N. An outside oracle exists and a model must know per-version provider behavior. | the bench (UND-7, V-M6) | ENTRY: HZ-9 read, UND-7 | |
| CAP-21 | `decidable_by` on every unknown (P3), as a closed list in HZ-3 | every reader of a verdict | ENTRY: with HZ-3 | E1 |
| CAP-22 | Decidability as a fleet metric | none today. M4 covers the bench | FUTURE: T-IaC | |
| CAP-23 | Cause set on the agent's own plan run | none today | FUTURE: T-IaC, replay first (case 2) | |
| CAP-24 | Contract anchors pushed when the agent reads a file | | REFUSE: case 4 by another name. Re-read when INB-2 has a reading | |

### Bench

| Id | Capability | Clause 5 consumer | Mark | Needs |
|---|---|---|---|---|
| CAP-25 | Hazard replay as a public benchmark with a model arm. Public history labeled by a plan. Arms: fathom, scanners, a model reviewer. No public benchmark of model reviewers on replacement risk was found `[web]`. The HAZ family is already in BENCH at no cost. The model arm makes it the cleanest test of the owner's thesis, since fathom does not own the oracle. Publish the labels and state the bias of synthesized state. | the proof program | ENTRY: HZ-9, BEN-5. The model arm waits on the paid gate | a runnable hazard entry in the library binary |
| CAP-26 | Collateral damage as a fact delta | | PLANNED: BENCH outcome two. CAP-14 widens it past Go | |
| CAP-27 | False-by-construction rows for config dialects, by fact mutation. Development split only. | the task bank | ENTRY: BEN-5 | CAP-14 |
| CAP-28 | A class conformance suite for static readers | none today | FUTURE: the library is released and one outside reader asks | |

### Spin-offs, community and the code side

| Id | Capability | Clause 5 consumer | Mark | Needs |
|---|---|---|---|---|
| CAP-29 | A standalone catalog-free check and pre-commit hook (H2, H4, H5) | none today | FUTURE: HZ-9 at 0.9 and an outside repository named in writing. Packaging, in the class of LIB-6 | E3, E7 |
| CAP-30 | A plan-free check for pull requests. Fork requests get no secrets, but scanners already run plan-free there. The gap is content: no static detector of a rename without `moved` was found `[web]`. | none today | FUTURE: as CAP-29, plus IF-6 | |
| CAP-31 | Basis annotation on other scanners' findings. Annotate, never drop. It enters through a review-kit producer entry, because ADR-040 rejected a raw findings pipe for reasons that hold with or without a store. No tool was found that filters by resolvability `[web]`. | none today | FUTURE: the review kit asks in writing, and a reading shows a precision gain against plan truth | E1 |
| CAP-32 | Catalog diff as data | none today | FUTURE: IF-6 and IF-17 shipped. Rides the catalog spin-off | |
| CAP-33 | API contract drift joined to callers | | PLANNED: the system graph's drift row | |
| CAP-34 | Rename and delete safety for config identifiers | weak today | FUTURE: CAP-16's resolver exists and its census shows use | E16 |
| CAP-35 | Imported analyzer and runtime findings (vet, race and leak reports) joined to symbols through stack frames, ranked by reach, owner and tests | the fleet is Go | ENTRY: FCT-6, producer entries in the review kit | |
| CAP-36 | Incident replay over infrastructure history | none today | FUTURE: T-IaC. The bench's M1 is the same machinery | E7 |
| | A language server, a library MCP surface, a default rule pack, workflow and migration linting | | REFUSE. Free tools own each (F90). Wins if: a named consumer asks in writing. Memory and lifetime moved to section 5a | |

A FUTURE row is a register entry, not a roadmap row. Only NOW and ENTRY rows are
scheduled.


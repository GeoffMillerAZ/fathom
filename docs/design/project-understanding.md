# Project understanding: the evidence layer agents and people call to be right about a codebase

Status: accepted 2026-10-28 (owner-architect rulings on the design pass; ADR-084 to ADR-089). Plan:
`docs/plans/project-understanding.md` (prefix PU). Vision: `docs/vision/45-the-understanding-layer.md`. Roadmap:
`docs/vision/38-proof-layer-roadmap.md` section 9.17. Evidence: `docs/assessments/2026-10-28-project-understanding.md`.
The owner asked on 2026-10-28 for fathom to become the code and project understanding tool, far better for code,
Well-Architected, operational, compliance and risk reviews and for incident work, and asked again about CUE state
machines for intent. This document is the how; the records hold the decisions.

## 0. The answer in brief

1. Fathom competes as the evidence layer (ADR-084): deterministic answers, oracle-graded where a grade exists, each
   with a receipt, served through its own CLI verbs, MCP tools, files, SARIF and hooks to agents and people.
2. Measure first (ADR-085). The bench is built from public, independent benchmarks with a deterministic anchor match
   as the primary metric, powered per question type. No understanding claim precedes a won reading.
3. The first new surface is the intent trace, `fathom why <record>` (ADR-086). The symbol why card follows only on a
   measured gap; the project map is a future.
4. Review kinds share one evidence contract; incidents get evidence cards with grounded causal paths; fathom never
   mitigates (ADR-088, ADR-089).
5. Intent stays markdown. A CUE status vocabulary, three conditions at HEAD and two counts close every measured gap
   (ADR-087).
6. Channels are local in phases 0 to 2; hosted agents wait on Q5. The table freeze is not amended.

## 1. Intent read before designing

| Kind | Id | How the design stays inside it |
|---|---|---|
| Invariant | I4 | Every answer carries the store's staleness and a freshness word (current, stale, unverified, no store) |
| Invariant | I8 | Nothing calls `VerifyPlan` on a request path; verdicts are read from the verify ledger "as of" their commit |
| Invariant | I9, I35 | Nothing new runs in a hook or is injected at the edit; every new answer is pulled |
| Invariant | I11 | An additive tool parameter lands with `docs/mcp-tools.md` in the same change |
| Invariant | I14, I56 | No fetch and no egress; benchmark repositories are cloned by the bench binary from operator-named sources |
| Invariant | I38 | "No governing record linked (searched N units at commit C)", never "ungoverned"; a missing input reads `not computed` |
| Invariant | I42, I60, I62 | Every decision-port site below is off by default and never a fact |
| Invariant | I48, I49 | Point queries only; new parameters declare a cost class |
| Invariant | I54 | A language level is quoted only from its committed reading |
| Invariant | I57, I58 | Drift signals and risk features are information, never a gate; no author field anywhere |
| Invariant | I19 (ADR-011) | Public product names only; adopter detail stays in the operator's private pack |
| Principle | P1, P14 | Governed evidence, never generated prose |
| Principle | P4, law 4 | No new MCP tool |
| Principle | P11 | Conditions and signals advise; nothing new gates |
| Principle | P12, P22 | No understanding claim without a counterfactual arm |
| Principle | P17 | Status is computed from what authors write, never stored by fathom |
| Decision | ADR-018 | Noted 2026-10-28: a status vocabulary that parses is not a lifecycle that drives process |
| Decision | ADR-049 d2 | The table freeze holds; phases 0 to 2 need no table, column, cache or job |
| Decision | ADR-053 | Additive parameters only, inside the default-surface schema cap |
| Decision | ADR-069, ADR-055 | The evidence plane, the review kit's authority and the external tierer are reused unchanged |
| Decision | ADR-074 | Markdown stays the source; review stamps in the intent ledger stay rejected |
| Decision | ADR-077 | "What governs this file" stays the rule selector's answer; the why card links to it |
| Decision | ADR-082, LAB-R1 | Every new surface rides a lab arm; no model grades a primary metric |
| Decision | ADR-083 | Loopback only; a remote channel waits on Q5 |
| Non-goal | all | None crossed; one product-vision verdict reopened with evidence (ADR-084 decision 3) |

Fired or due triggers: ADR-074's K1 and K6 reading is due 2026-11-17 and ADR-067's RT-M1 date trigger on 2026-11-15;
neither is fired by this design. ADR-074 R1's CUE wins-if has not fired (assessment section 3). Goal reviews for G2,
G3, G4, G9, G11, G12, G13 and G14 are overdue with no record; ADR-087's ReviewRecorded condition will name them.

## 2. Position

The wedge is what no surveyed product combines: receipts a stranger re-derives with git; graded breadth across
languages and tool formats, including the config, IaC and build files where every localizer scores under 40%; intent
checked as data and joined to code; preregistered readings that publish their losses. The channel exists, since
reviewers and SRE agents opened MCP and skills slots in 2026.

Where fathom does not win, it pairs or stays out:
- **Meaning questions** ("where does feature X live"): semantic search helps there and only there. Fathom pairs with
  a third-party meaning-search tool and measures the pairing (arm A3); it builds no vector store (ADR-041).
- **Popularity against free graph MCP servers**: one-command install and honest benchmarks are table stakes (G16).
- **Being the reviewer or the RCA agent**: review-agent-only PRs merge less often, and general agents beat bespoke RCA
  agents. Fathom feeds both and is neither.

## 3. Capability layers

| Layer | Exists | Add here (small) | Layered from others | Not building |
|---|---|---|---|---|
| L1 facts | Symbols and edges L1 to L4, concern facts, the repository graph, the doc index, centrality, a CODEOWNERS parser (unwired) | The common-name guard on heuristic callers and doc-link targets (PU-C1, PU-C2); the TS and JS precision reading (PU-M11) | SCIP indexers the operator runs; scanner SARIF through the findings loop; coverage reports | Embeddings, model-built graphs, a second store |
| L2 intent | The parsed corpus, conditions, bets, the intent ledger, `intent adr-rules`, plan grammar v2 with verify | The status vocabulary, three HEAD conditions and two counts (PU-L rows) | Spec dialects already read; kit lifecycles as input | CUE-authored records, rendered copies, a workflow engine, model-written specs |
| L3 delivery at the edit | Obligations, guards, rule selection, advice delivery | Nothing | Client rule files | Overviews pushed into the prompt |
| L4 answers | `symbol_context`, `blast_radius`, `review_diff`, `localize`, receipts, the concern-delta file | The intent trace (PU-W1 to PU-W5), ID3 (PU-I1, PU-I2), the risk feature file (PU-R1); the why card on its trigger (PU-W6 to PU-W8) | n/a | Generated prose, a committed map, chat, a new MCP tool |
| L5 review and incident engines | Two skills specced (PLT-10, PLT-11); the review kit and policy kit judge outside fathom | Nothing | MCP-capable reviewers, AI SRE agents, portals, GRC tools, the adopter's change-risk tierer | Verdicts, severity, catalogs, a risk model, an RCA agent, mitigation |
| L6 measurement | Bench prep, feature labs, readings, the ledger oracle | The understanding bench (PU-B rows) and the readings (PU-M rows) | RepoProbe, ORCA-bench, GitHub ReviewBench, MULocBench, ApacheJIT | Self-written keys, model-judged primaries, self-reported speedups |

**Not building**, each with a `[-]` row in the plan so it is not re-proposed: a generated wiki or architecture prose;
a committed project-map file; an embeddings store; a chat surface; a reviewer that posts its own comments; an RCA agent
or model-ranked suspects; a risk model, band or gate; compliance catalogs, crosswalks, SSP or POA&M output, or scanners
of its own; mitigation of any kind; a new MCP tool; CUE-authored intent records; transition rows in the intent ledger;
a team glossary.

## 4. Per review kind and incident phase

Every kind shares the contract of ADR-088 decision 1: an anchor, a coverage statement and a receipt per finding or
fact, and a read set for every absence.

| Kind | User story | Fathom surface | Proven better how |
|---|---|---|---|
| Code review | An author's agent needs what a PR breaks or leaves inconsistent, with anchors, before a person reads it | `review_diff`, `fathom review --since B` with SARIF and the check payload, `blast_radius`, `test_impact`, concern deltas, ID3 for the range; delivered to the author or the author's agent | PU-M6 precision floor per finding kind; PU-M7 the same reviewer with and without fathom on GitHub ReviewBench. Precondition: RVU-1 to RVU-5 |
| Well-Architected and architecture | A reviewer or a Well-Architected skill needs the code and config half of each question at a commit | Concern facts, joins J1 to J3 and S1 to S6, fact and join receipts, `fathom-platform-review` (PLT-11), `arch_conformance`, `intent adr-rules` | WAE-M2 to WAE-M6 and PLT-M1 to PLT-M5, unchanged; labels per U3 |
| Operational readiness | A readiness reviewer needs a per-workload inventory with honest absence | PLT-7's per-subject fact receipt, CODEOWNERS once wired (IM-F1) | PLT-M3 and PLT-M4 (kill 2027-01-31), unchanged |
| Compliance | An auditor, GRC tool or policy kit needs clause-keyed evidence that is current | `trace export` and `trace verify` packs, fact and join receipts, governance SARIF from `verify`, `intent audit`, `config audit` and `obligations`, imported scanner findings at their tier, decision records with checked statuses | CX-M2 (a pack re-verifies byte-identically); no new bar until a named consumer exists (F93) |
| Risk | The adopter's tierer needs structural reasons a diff is risky that size and metadata miss | `fathom review --since B --risk-features FILE` (PU-R1) | PU-M8 capture of defect-inducing commits on ApacheJIT |

| Incident phase | Fathom surface | Evidence produced | Proven better how |
|---|---|---|---|
| Prevention | The PR ratchet (concern deltas), risk features, ID3, governing records touched | Anchored deltas and features as SARIF | WAE-M5's preregistered postmortem replay; PU-M8 |
| Detection to context | `localize`, alert-to-workload-to-handler joins, `read_source` at a ref, owners, the deployed commit as a declaration (ADR-069 d7), `fathom-investigate` (PLT-10) | Evidence cards: a `join` receipt whose anchors form the causal path | PU-M2 grounded-path ablation on ORCA-bench; PU-M9 latency before any "real-time" word |
| Mitigation support | `review --since previous..deployed`, `blast_radius` of each suspect, owners, config and flag facts | Receipts; every hypothesis labelled "not cause" until a person decides | IM-M0 decides whether even a deterministic suspect list is built; fathom never executes, proposes or records a mitigation |
| Post-incident learning | OutcomeResolves (section 7); the outcome is then served by obligations at the edit | A named condition in `intent audit` and SARIF | PU-M4 |

## 5. The intent trace and the narrowed why card

### 5.1 `fathom why <record>`

Input: a decision, invariant, goal, future or plan-row id. Output, in order, each line with its anchor:

1. The record: kind, status (normalized by the vocabulary of section 7), file and line, the commit read.
2. Plan rows that cite it: a row whose title or spec ids name the record, or whose plan's header names it. Each row
   carries its id, its checkbox state, its contract (`symbol`, `package`, `oracle`) and its last recorded verdict from
   the verify ledger with the commit it was recorded at. A row with no recorded verdict reads `not recorded`.
3. Symbols: each contract's symbol resolved at the index commit, with its file, line and staleness; a symbol the index
   lacks reads `claimed-but-absent`, a stale index reads `not computed`.
4. Rules: accepted `intent adr-rules` proposals and compiled rules that cite the record, with their globs.
5. Doc links, last and labelled: `doc_links` rows from the record's units, after the common-name guard and package
   qualification (PU-C2), each with link state (holding, drifted, dangling) and a confidence label (`exact` or
   `name-only`).
6. Receipts: the receipt command for each resolved symbol where a receipt kind exists (Go functions and methods today).
7. Absence: "no binding row, rule or link found (searched N plans, M units at commit C)".

The trace reads the corpus, the plans, the verify ledger and the store by point queries. It computes nothing that
`fathom verify` owns. `--json` carries the same lines. The observatory's intent tab renders it beside the audit (PU-W5).

Illustration (shape only):

```
$ fathom why ADR-018
ADR-018  decision  accepted (Amended)  ADR.md:<line>  read at <commit>
plan rows citing it (<n> in <m> plans)
  <plan>.md#<id>  [x]  symbol <Func>  <package>  verified as of <commit>
  <plan>.md#<id>  [ ]  symbol <Func>  <package>  not recorded
symbols: <Func>  <file>:<line>  current
rules: <n> compiled rules cite ADR-018
doc links (labelled): <n> exact, <n> name-only
receipt: fathom export receipt --kind symbol_location <Func>
```

### 5.2 The symbol why card, on its trigger

`symbol_context` gains `include: ["why"]`, opt-in and additive, inside the tool's byte cap. For a symbol it lists the
plan rows whose contract binds it, each with its last recorded verdict and commit; its certifying and covering tests;
and one pointer line: "what governs this file: `obligations` (ADR-077)". It never calls `VerifyPlan` and never re-runs
rule selection. Provenance (at most 3 commits touching the symbol's lines: sha, date, subject, ticket ids by a
configured pattern, never an author) is opt-in and one bounded git read.

It is built only when both hold: PU-M1 shows a T3 or T4 gap, meaning A0 and A1 miss items that a binding row or record
would answer, and PU-M0 (the link pre-reading) passes. ADR-053's default-surface schema cap is 9,000 bytes and measured
8,339 on 2026-09-29, so about 660 bytes remain. PU-W7 states the parameter's byte cost, and the card ships behind a lab
card (PU-W8) because description text moves tool use.

## 6. Drift signals and the deferred project map

| Signal | Definition | Served by | Status |
|---|---|---|---|
| ID1 drifted binding | An intent unit's link target changed since the link was written | `staleness_report`'s `doc_drift` (existing) | Reused; measured by KR-M2 |
| ID2 dangling binding | An intent unit names a symbol or path that no longer resolves | The same | Reused; KR-M2 |
| ID3 ungoverned sensitive change | A path the router's sensitive-surface table names changed in the range with no governing record linked | `review_diff` and `fathom intent audit` (PU-I1, PU-I2) | New; PU-M12 |
| Lifecycle conditions | SupersederResolves, OutcomeResolves, ReviewRecorded at HEAD | `fathom intent audit` | New; section 7 |
| ID5 goal-plan-code break | A goal's cited rows went from proven to unproven since a base | Verify history | Deferred (PU-X rows): the verify ledger admitted scratch-plan rows on 2026-10-28 |

**The project map is deferred (F94).** A computed map of components (from the repository graph), their central
symbols, governing-record counts, owners, concern coverage, language levels and unknown residue stays a future. It is
promoted only when the bench's T2 reading shows A1 losing or tying on T2 at adequate power with the misses attributable
to orientation (the answer names the wrong component). Component clustering is its later refinement, never first.

## 7. The CUE status vocabulary and the audit conditions

Sketch of `schema/intent/status.cue` (PU-L1), embedded byte-identical like `schema/intent/schema.cue`:

```cue
package intent

// The closed status vocabulary per intent-record kind. Markdown stays the source:
// this parses what authors already write and never rewrites it (ADR-087).
#RecordKind: "decision" | "incident" | "future" | "goal"

#KitLifecycle: {
	source: "agentic-kit docs/project-intent/README.md"
	commit: string // the vendored kit commit the decision states were taken from
}

#DecisionStatus: "proposed" | "accepted" | "rejected" | "deferred" | "superseded" | "deprecated"
#IncidentStatus: "open" | "outcome-pending" | "closed"
#FutureStatus:   "candidate" | "promoted" | "dropped"
#GoalStatus:     "active" | "kept" | "killed" | "done"

// Conditions are observations, never states (the Kubernetes conventions).
#Condition: {
	type:   "Amended" | "ReviewDue" | "SupersederResolves" | "OutcomeResolves" | "ReviewRecorded"
	status: "True" | "False" | "Unknown"
	reason: string
}
```

The normalizer (PU-L3) folds "Accepted", "Accepted.", "Accepted," and "Accepted;" to `accepted`, reads "amended
<date>" as the Amended condition, and names any line it cannot fold as `unparsed` with its file and line (PU-L4).

The three conditions (PU-L5 to PU-L7), each computed at the current commit only:

| Condition | True when | False names | Unknown when |
|---|---|---|---|
| SupersederResolves | A record whose status is superseded names a successor id that has a heading | The record and the missing id | The successor is a sibling-repository id |
| OutcomeResolves | A closed incident's Outcome names an invariant, test, decision or plan-row id that resolves | The incident and the unresolved text | The Outcome is prose with no id (never guessed) |
| ReviewRecorded | A goal whose bet review date has passed carries a dated keep or kill line on or after it | The goal, its review date | The bet block is refused |

`fathom intent audit --kind lifecycle` prints them; `--json` carries them with the unparsed-status count and the
structured-bytes share per kind (PU-L9); `--format sarif` gives each False condition a rule (PU-L8). Nothing gates.
Records written before the vocabulary's commit are listed once as not computed, never as violations.

**Edge judgment** (proposed then judged between git endpoints) waits on PU-M5: a $0 backtest that replays 12 months
of git history of `ADR.md` and `docs/intent/` and counts illegal transitions against the vocabulary. At 2 or more, a
new record designs it and answers ADR-074's rejected review stamps. **Kit lifecycles** (agentic-kit's workflow
engine, correction ledger and intake triage; a review kit's finding lifecycle) are read by their schema version as
input, an unknown version refused by name.

## 8. The understanding bench and the readings

ADR-085 fixes the rules. The arms are A0 (grep and the harness's tools), A1 (fathom's default surface), A2 (A1 plus
the intent trace and, once built, the why card) and A3 (A1 plus a named third-party meaning-search server). Primary
types: A1 against A0 on T4, A2 against A1 on T3, A3 against A1 on T1, Holm across the rest, typed-language stratum
primary. Every N comes from a power analysis on the LAB-37 shakedown's variance; a reading short of its minimum
detectable effect reads `underpowered`.

| Id | Measures | Baseline | KEEP bar (fixed in the prereg before data) | CUT or kill effect | Phase, cost |
|---|---|---|---|---|---|
| PU-M0 | Precision of 100 sampled live intent links; coverage on a seeded sample of product functions | About 8 of 25 sampled exact-tier links named common words | Wilson 95% lower bound of precision at least 0.85 | Doc links stay a labelled secondary; the why card waits | P0, $0, about 1 person-hour |
| PU-M1 | A1 against A0, RepoProbe slice plus MULocBench's config subset plus the owner gap bank | Home bench 41 against 44 at 1.81x tokens (not comprehension) | A1 above A0 on T4 at the prereg's MDE, Holm-adjusted, paired interval excluding 0, tokens at most 1.25x A0 | A lost type is published as a loss; G46 fails | P1, API on the owner's cap |
| PU-M2 | ORCA-bench code-bearing tasks, one agent with and without fathom | None | Anchor match on the benchmark's root-cause key above the without arm at the MDE | Evidence cards not positioned for RCA | P2, API |
| PU-M3 | A2 against A1 on T3; A3 against A1 on T1 | PU-M1 | A2 above A1 on T3 at the MDE; A3 above A1 on T1 promotes F95 | `include: why` leaves under a `gone:` row; program kill if PU-M1 also null | P2, API |
| PU-M4 | The three lifecycle conditions on first runs over fathom's and the vendored agentic-kit's corpora | 9 overdue goal reviews and 1 pending outcome found by hand | At least 2 person-confirmed real defects, false reports at most 20% | Conditions removed, vocabulary kept as documentation | P0 to P1, $0 |
| PU-M5 | Illegal status edges in 12 months of history | 1 accepted-to-superseded edge in 30 days, resolving | Reading only: 2 or more revives edge judgment | n/a | P0, $0 |
| PU-M6 | `review_diff` precision per finding kind on clean and seeded PRs | 5 of 42 historical PRs, no false findings | Per kind: Wilson lower bound of precision at least 0.80 and at most 0.1 false findings per clean PR | The kind is advisory by default | P2, $0 plus labels |
| PU-M7 | One reviewer with and without fathom's MCP server on GitHub ReviewBench (Martian's offline set secondary) | In-the-wild acceptance 36.4% | Anchor-matched precision above the without arm at the MDE, recall non-inferior by the prereg margin | Reviewer guidance says unproven | P2, API |
| PU-M8 | Capture of defect-inducing commits at the riskiest 10%, fathom features plus size against size alone, on ApacheJIT | Meta's own 27.9% (shape only) | Capture above size alone at the MDE on a held-out split | PU-R1 removed | P2, $0 |
| PU-M9 | Cold and warm p95 and abandons for `localize`, `fathom why`, `review --since` and `blast_radius` on a 1,000-file public repository, off the operator's working host, load recorded | `localize` hook 200 of 223 timeouts under load | Cold p95 at most 3 s, warm at most 1 s, 0 abandons | No "real-time" anywhere | P1, $0 |
| PU-M10 | Install plus cold ingest plus first answer on a hosted CI runner for a 1,000-file repository | Fathom's own cold ingest 299 s | Reading only, against the review's time budget | n/a | P1, $0 |
| PU-M11 | TS and JS call precision and recall: the no-SCIP fallback and imported SCIP against SCIP truth | TS .52, JS .19 | Reading only (I54) | n/a | P1, $0 |
| PU-M12 | ID3 on 10 sensitive-path edits and 20 unchanged paths | None | 9 of 10 found, 0 false on unchanged | ID3 removed | P1, $0 |

Validity gates before any agent reading: NP-P0's deadline-abandon work lands; NP-B1 to NP-B8 hold; the anchor grader
passes its validation (PU-B4). A reading under an abandon rate above its prereg's bar is void.

## 9. Channels: local first, Q5 as the remote gate

| Channel | Phase 0 to 2 | How |
|---|---|---|
| Agents on the same machine (Claude Code, Codex, Cursor) | Open | Loopback MCP, skills and hooks |
| CI | Open | The `fathom` CLI on the runner writes SARIF and receipts; PU-M10 reads its cost |
| Hosted reviewers (Copilot code review's hosted runner, Bugbot) | Gated on Q5 | Needs a reachable, authenticated fathom; ADR-083 keeps loopback only |
| Hosted AI SRE agents | Gated on Q5 | The same |

This is a dependency, recorded on Q5, not a plan row.

## 10. Phases

Vision 38 section 9.17 holds the phases, their exit tests and the program kill. In short: P0 records, the CUE rows
and the bench; P1 the intent trace and the baselines; P2 the decisive readings; P3 gated on RT-M1 PASS or NP-Q1.

## 11. Risks

| Id | Risk | Mitigation |
|---|---|---|
| K1 | The bench says fathom does not help comprehension (likely given 41 against 44) | Claims are per type; the kill fires only on a powered null; review, compliance and intent evidence stand on their own readings |
| K2 | Self-grading or contamination | Public benchmarks, independent oracles, person-confirmed keys, leak audit, preregistration |
| K3 | Person hours bind (key confirmation, grader validation, PU-M0, PU-M6 labels) | Public keys first; Q38 |
| K4 | Serving reliability makes readings measure the bug | NP-P0 first; abandon-rate voiding |
| K5 | Surface creep in `symbol_context` | Opt-in, byte cap, built only on a measured gap, behind a lab arm |
| K6 | TS and JS precision too low for honest answers | PU-M11; levels quoted from readings only |
| K7 | Status theater: authors avoid status edits | The conditions advise and never gate |
| K8 | Ledger pollution corrupts verdicts | Verdicts carry their commit; ID5 deferred until the ledger admits only registered roots |
| K9 | Record-id collisions from parallel writers | ADR-084 to ADR-089, G46 to G48, F93 to F96, Q38 to Q40 and prefix PU reserved |

## 12. Owner questions

The assessment's section 7 lists Q4, Q5, NP-Q1, U3, Q38, Q39 and Q40 with their proposed effects; Q38 to Q40 are in
`docs/intent/questions.md`. None is decided here.

## Decision models

Three typed questions could enter; none is built in this program, and every one is off by default (I60).
- **A secondary grader** (ADR-085): `predicate` "does this answer sentence assert checklist item I?". With provider
  `none`, grading is the anchor match alone. Opt-out: `[decide] provider = none`. Kept only at a Wilson lower bound of
  at least 0.85 for TPR and TNR on at least 100 person-labelled items; never changes a primary verdict.
- **Prose intent links** (ADR-086): `predicate` "does intent span S govern symbol X?". With `none`, plan-row contracts
  and exact links only. When on, a separate list labelled judgment (I62). Kept only at a Wilson lower bound of 0.90
  precision on 100 person-labelled pairs and an A2 gain on T3.
- **Prose outcomes** (ADR-087): `predicate` "does this Outcome line name a durable artifact?". With `none`, the
  condition reads Unknown on prose. Not built: stage 0's predicates measured at or below baseline.

Refused: a `choice` ranking incident suspects (the external agent ranks, ADR-089) and a risk score (the tierer owns
it, ADR-088). The bench's arms, graders, intervals and verdicts are computed with provider `none`.

## Two horizons

- **Now.** The records, the CUE vocabulary and three conditions, the bench and its validation, then the intent trace
  and the baselines. No table, no new tool, no hosted channel.
- **Three years out, kept open for free.** Receipts keep stable fact ids a compliance clause can key (F93); the trace's
  JSON is the shape a project map would extend (F94); the arms leave room for a meaning-search pairing (F95) and for
  verifying other reviewers' comments (F96); verdicts carry their commit, so cross-repository traces after RT-M1 PASS
  need no format change.
- **Not pre-built.** The project map, edge judgment, the why card before its trigger and every hosted channel.

**Specced, not implemented:** every PU row. **Planned, not specced:** none.

## Records

ADR-084 to ADR-089; the ADR-018 note; product-vision section 3 amended 2026-10-28; vision 45; vision 38 section 9.17;
goals G46 to G48 (`docs/intent/scope/goals-3.md`); futures F93 to F96 (`docs/intent/futures/f92-f97.md`); questions
Q38 to Q40 and the Q4 and Q5 notes; plan `docs/plans/project-understanding.md`.

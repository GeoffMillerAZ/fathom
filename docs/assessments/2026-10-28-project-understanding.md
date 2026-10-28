# Project understanding: where fathom can win, what the market and the literature prove, and the CUE question

Status: design-pass assessment, 2026-10-28, read against fathom at `487535f`. The owner asked on 2026-10-28 for fathom
to become the code and project understanding tool: far better for code, Well-Architected, operational, compliance and
risk reviews, and for incident prevention and response. The same ask returned to an older question: should vision,
goals and specs become CUE state machines? Five research reports (the owner's prior evaluations, fathom's state, the
CUE question, the market, the literature), one design, one red team and the owner-architect's rulings fed this
reading. The reports stay private; this file condenses them. Decisions it drove: ADR-084 to ADR-089, the ADR-018 note,
the product-vision amendment, vision 45, vision 38 section 9.17, goals G46 to G48, futures F93 to F96, questions Q38 to
Q40 with notes on Q4 and Q5, and the plan `docs/plans/project-understanding.md` (PU-).

Grades follow this directory's README: *measured*, *estimate*, *claimed by a vendor*, *hypothesis*, *unmeasured*. A
claim that reached us only through an aggregator or a comparison site is marked *secondary*.

## Verdict

Compete in project understanding, as the evidence layer that agents and people call when they need to be right about a
codebase. Do not become a wiki generator, a reviewer, an RCA agent, a chat surface or a semantic search engine. The
position is open: no surveyed product combines deterministic, oracle-graded facts, receipts a stranger can re-check and
intent checked against code. The claim is not earned yet. Fathom's newest home-bench reading is 41 successes against
grep's 44 at 1.81 times the tokens (measured 2026-09-14), and change tasks are a published null. So the first
deliverable is the benchmark fathom will be judged on, built from public, independent benchmarks with a deterministic
primary metric, and every new understanding surface lives or dies by its question type. The first new surface is the
intent trace, `fathom why <record>`, because joining decisions and plans to code is where fathom alone holds the data.
On CUE: markdown stays the source. A CUE vocabulary for record statuses and three checks that run at the current
commit cover every gap measured today. Judging status edges waits until a $0 backtest finds a real illegal one.

## 1. The owner's prior evaluations, condensed

Eighteen private evaluation pages, dated 2026-07-08 to 2026-10-28, were read as data. Their links are not reproduced.

| Date | Evaluation | Its verdict | What it asked of fathom |
|---|---|---|---|
| 2026-10-28 | Fathom beside Paparats (MIT, embeddings plus a syntax call graph, v2.16.0) | Neighbours: a search engine beside an evidence engine | Publish a with-and-without agent benchmark ("the biggest missing piece"); borrow type-aware TS and Ruby call rules (ideas), git history and ticket ids per answer, a common-name guard; consider a module map and a support endpoint |
| 2026-10-28 | A 52-feature matrix: fathom, Paparats, AWS AI-DLC | Fathom owns the trust features, Paparats find-and-remember, AI-DLC process | Gaps: meaning search, history per result, a glossary, one-command install, a verified second client |
| 2026-10-27 | Fathom and AI-DLC's "CodeKB" | Keep building; AI-DLC leaves a slot for a code-knowledge server | Fill the slot with a guide; a question budget in the stanza; a one-word freshness verdict |
| 2026-10-22 | Why advice gets ignored | Mostly delivery: 5.7% of 211 served rules were relevant to the edit | Became ADR-080 |
| 2026-10-20 | Semantic layers, rearchitect or not | Do not; fix rule selection | Became ADR-077 |
| 2026-10-20 | A review kit and policy kit design | One closure contract every engine meets | Fathom facts meet it once it is published |
| 2026-10-11 | The evidence plane | The center of the evidence, not of the platform | Became ADR-069 and vision 44 |
| 2026-10-02 | Roadmap and feature scorecard refresh | Weakest: ingest cost, call precision beyond Go (TS .52, JS .19), governance that does not bite | Language and delivery programs |
| 2026-09-27 to 09-29 | Landscape, Claude OS, features and review, semantic layers, exploration maps | Keep building at the proof gap; reach before new engines | ADR-049 to ADR-056 |
| 2026-07-08 | A policy-as-code coverage review of an adopter platform's controls (CMMC, 800-171, FIPS, STIG) | The platform side has no emitters yet | Fathom's compliance role left undefined |

Across them, the gaps cluster into reach (install, clients, a published benchmark), delivery of what exists, breadth of
facts (cross-file concern joins, service links, typed calls) and packaging for review and compliance. Few ask for a new
engine. Three pages want meaning search and two cite market evidence that embeddings lost to grep, LSP and FTS; the
consistent reading is FTS now and a third-party meaning-search tool measured beside fathom. Dispositions:

| Asked for | Disposition | Record |
|---|---|---|
| A with-and-without agent benchmark, per question type, losses included | Adopt now, as the understanding bench | ADR-085, PU-B and PU-M rows |
| Git history and ticket ids per answer | Adopt inside the why card, bounded and author-free, if the card is built | ADR-086, PU-W rows |
| Common-name guard | Adopt for heuristic callers and doc-link targets, set apart and counted, never dropped | PU-C1, PU-C2 |
| Type-aware TS and Ruby calls without SCIP | Measure first (TS .52, JS .19 today) | PU-M11, `docs/plans/language-breadth.md` |
| Meaning search | Layer as a third-party arm, never built | ADR-085 arm A3, F95 |
| Module map by clustering | Defer with the project map | F94 |
| AI-DLC code-knowledge plug-in, adoption guides for hosted reviewers | Wait: hosted channels are gated on Q5 | ADR-084 decision 6 |
| Question budget in the stanza, pre-written MCP prompts | Defer to a lab arm (every always-loaded text change is A/B tested) | PU-X rows |
| Clause catalogs, STIG and FIPS scanners, OSCAL output | Reject for fathom; import scanner results; OSCAL waits | ADR-088, F93 |
| Incident map, rollback safety | Defer to IM-M0 | ADR-089 |
| Team glossary, splitting big diffs, approval gates | Reject | PU-X rows, non-goals |

## 2. Fathom's state on 2026-10-28

- **Surfaces.** 8 default tools, 21 extended, 25 in the registry, about 57 CLI verbs; law 4 forbids net tool growth.
- **Breadth.** 121 committed language and format readings: L4 11, L3 56, L2 43, L1 7, none 4 (measured by counting
  reading files; tool formats included). Readings and `fathom support` hold the levels, never this file.
- **Understanding is a byproduct.** `repo_map` is orientation-only. Product vision recorded comprehension as "staffed
  never". The feature map has 0 of 14 rows done. F30 (an onboarding path) is deferred. The thesis tombstones
  pre-summarized code.
- **No measured help yet.** Home bench 41 against 44 at 1.81x tokens (2026-09-14). Lookups: level with the best lookup
  tools at 0.76x grep's tokens, ahead only on verification and receipts. Change tasks and Long Code Arena: published
  nulls. The review verb named a gap the fix later touched in 5 of 42 historical PRs, with no false findings.
- **Reliability.** `fathom plans status` abandoned 9 of 96 plans at the daemon deadline (measured 2026-10-28). The
  `localize` hook timed out 200 of 223 calls under load (measured earlier). NP-P0 fixes this first.
- **Review honesty.** Four outputs can read as "clean" on nothing; RVU-1 to RVU-5 are specced and open.
- **Evidence plane.** Kubernetes workload and alert facts, the concern-delta file and portable receipts for Go
  functions and methods ship. Fact and join receipts (WAE-10, PLT-7) and the cross-file joins (WAE-8, 9, 12) are open.
- **Incidents.** Incident maps are deferred (IM-M0 and IM-M1 unread), the CODEOWNERS parser is unwired, the
  `fathom-investigate` skill is unbuilt, and mitigation is refused.
- **Intent.** `fathom intent audit` reads 300 conditions (0 annotated), 164 prose triggers, 68 invariants and 55
  unproven claims (measured 2026-10-28). Plans are reconciled by `fathom verify`; goals are pre-registered bets.
- **Gates.** The table freeze holds (RT-M1 read FAIL twice; host swap caused every refuse minute). Several plans carry
  a 2027-01-31 kill date.
- **Records debt.** Goal reviews for G2, G3, G4, G9, G11, G12, G13 (2026-10-25) and G14 (2026-10-26) passed with no
  record; G7 and G9 are listed past their Done when; `scope/boundaries.md` still says no CI runner exists.
- **Structural leads, none of them a lookup feature.** Governed and cited claims, done means proven, receipts a
  stranger re-derives with git (50 of 50 re-derived, envelope 7.6% of payload), oracle-graded levels, an audited
  intent corpus, the honesty floor (I38) and a decision port that cannot contaminate proof.

## 3. The CUE state-machine question

- **The owner's memory is half right.** The pattern exists for runtime records: agentic-kit's workflow engine,
  correction ledger, intake triage and claim lifecycle, and a review kit's finding lifecycle (the most mature). Their
  common core: CUE declares the vocabulary, Go owns the transition law and refuses illegal edges, and state is an
  event fold or a derivation, never a trusted hand-edited field.
- **Nobody converted vision, goals or specs into CUE state machines.** No `.cue` file sits under `docs/intent/` in
  fathom, agentic-kit or an adopter's platform repository. The closest artifacts are that adopter's v0 scenario catalog,
  which authors goals, requirements and controls in CUE with literal state ceilings (about 5,000 lines, one commit,
  synthetic), and an experimental kit's goal type whose status is computed, never authored. Neither is a standard.
- **Asked twice, declined three times on the record.** The owner asked on 2026-10-17 and 2026-10-24. agentic-kit 0021,
  fathom ADR-074 R1 and the adopter repository's structured-intent decision each declined CUE as the authoring source
  with a wins-if. ADR-018 (2026-07-15) had rejected authored transition tables.
- **The wins-if has not fired** (measured 2026-10-28, against a bar of 50% structured bytes):

  | Kind | Structured share |
  |---|---|
  | Goals | 28% in bet blocks (8,347 of 29,514 bytes, 36 bets) |
  | Futures | 7.5% rating lines, 13.7% condition lines (105,989 bytes) |
  | Plan rows | 38% in annotation blocks (629,771 of 1,656,378 bytes, 3,804 rows in 97 plans) |

  The other condition, more than five field-vet failures a month, has no instrument: the gate refuses a bad field
  before commit and leaves no trace.
- **The real gaps, all checkable at the current commit** (measured): decision statuses in four spellings (48
  "Accepted", 24 "Accepted.", 6 "Accepted,", 2 "Accepted;") with superseded-by links never checked; 9 of 36 goal bets
  past review with no keep or kill recorded; 1 of 9 incident outcomes pending, the outcome rule unchecked.
- **Costs of CUE as the source.** Dagger dropped CUE-only configuration over learning cost. Fathom's own plan CUE copy
  lost data on 49 of 75 plans before a gate held it. Short records grow about three times. Format effects on model
  reading are small and model-dependent (arXiv 2411.10541). CUE checks one snapshot, so alone it cannot judge an edge.
  Kubernetes deprecated its phase enum: "conditions are observations", and the system is "level-based rather than
  edge-triggered".
- **Prior art that matches fathom's shape** keeps typed fields inside readable documents and checks them at build time:
  Kubernetes conditions; sphinx-needs (typed requirements inside docs, an enum status, violations as build failures);
  OpenSpec's folder lifecycle with `validate --strict` (secondary).
- **What changed between design and ruling.** The design proposed a lifecycle law judging status edges between git
  endpoints, with transition rows in the intent ledger. The red team found every measured gap checkable at HEAD, no
  edge-only violation in 30 days of `ADR.md` history (the one accepted-to-superseded edge had a resolving successor),
  and a collision with ADR-074's rejected "review stamps in the intent ledger". The ruling narrowed it to ADR-087.

## 4. The market, with proof levels

Read 2026-10-28 from vendor pages, changelogs, open benchmarks and preprints. No product was tested hands-on.

| Area | What 2026 showed | Grade and date |
|---|---|---|
| Channels | MCP and skills became the reviewer and SRE agent integration contract: Copilot code review skills and MCP GA 2026-07-29; Bugbot MCP 2026-04-08; Azure SRE Agent GA with MCP connectors 2026-03-10; Spotify Portal Soundcheck MCP actions 2026-04-21; AWS Well-Architected Agent with an MCP server, preview 2026-10-01 | claimed by a vendor |
| Context engines | Absorbed or dying: Cursor agreed to buy Graphite (2025-12-19), Tricentis is acquiring Tabnine (2026-07-30), Bloop shut down (2026-04-10) | claimed by a vendor, partly secondary |
| Graph MCPs | GitNexus about 47.7k stars, CodeGraph about 70k (aggregator snapshots, October 2026). Codebase-Memory: answer quality 0.83 against 0.92 for a file-exploring agent at about 10x fewer tokens (arXiv 2603.27277, its own authors) | secondary; measured by the authors |
| Structural tools | Roughly double grep-only localization for 4-8B models; the gain falls to 4-5 points at 32B; agents chain graph expansions in 12-14% of episodes (arXiv 2610.33958, 2026-10-25) | measured |
| Meaning search | Cursor: +12.5% QA accuracy, best combined with grep (2025-11-06). Paparats gains on meaning questions only | claimed by a vendor |
| Generated wikis | DeepWiki, Google Code Wiki (preview 2025-11-13), Windsurf Codemaps: model-written, no citation validation, no accuracy figure | unmeasured |
| Validated memory | Copilot Memory re-checks code-location citations before use and drops unvalidated ones after 28 days | claimed by a vendor |
| Code review | One large in-the-wild study: 36.4% of 31,073 review comments accepted, 56.3% rejected, for false positives, redundancy and misalignment with intent (arXiv 2607.03316). Martian's offline set changed its key three times (137, 136, 173 comments, February to August 2026). GitHub ReviewBench (2026-10-05): 219 PRs, 187 repositories, 19 languages, open data and runner; authored by GitHub, Copilot code review leads its leaderboard at 40.1% grounded F1, and its gold findings come partly from LLM reviewers and static tools, re-labelled by people at 96.6% agreement (GitHub blog, 2026-10-05). Vendor F1 claims cluster at 51 to 64 | measured; secondary; claimed by a vendor |
| Spec tooling | Spec Kit, Kiro, OpenSpec, Augment Intent and BMAD all verify with a model or with model-written tests. Per-line citations made 86 to 88% of hallucinations machine-detectable against 0% for Spec Kit and OpenSpec styles (arXiv 2606.30689). Gemara (OpenSSF, 2026-03-09) declares governance layers in CUE | measured; claimed by a vendor |
| Well-Architected | AWS's agent reads configuration, metrics and IaC, not application source, and says its output "may contain errors". Azure's resiliency library expresses checks as deterministic queries with a doc link | claimed by a vendor |
| Compliance | Vanta, Drata and Secureframe automate documents and integration state, not code facts. Chainloop stores signed evidence including SARIF. OSCAL tooling is active (compliance-trestle v5.2.0, 2026-10-05) | claimed by a vendor |
| Incidents | Funded and unproven: no commercial AI SRE has an independent accuracy figure. OpenRCA 2.0: agents name a right service in 76.0% of cases and ground it in a verified causal path in 61.5% (arXiv 2606.27154). ORCA-bench: 1,079 tasks; removing source access lowers accuracy and raises hallucination for every model (arXiv 2607.28545, revised 2026-09-29). Meta's internal ranker: root cause in the top five for 42% of backtested investigations (2024-06-24, first party) | measured; claimed by a vendor |

**White space.** Nobody emits a per-answer receipt over code facts, publishes per-language accuracy against an oracle,
joins code to IaC, manifests, proxies, alerts and IAM at a commit with coverage for absence, proves work against
intent without a model, or keeps a preregistered accuracy record in incident prevention. **Positioning.** The
recommended frame is the verifiable evidence layer under other people's agents; the wedges are proof under spec and
intent tooling and code-to-cloud architecture and compliance evidence; incident context is a gated bet; head-on
competition with free graph MCPs is table stakes (install, honest benchmarks, TS and JS precision), not the story.

## 5. The literature, ranked

Retrieved 2026-10-28. Many 2026 items are preprints. Strength grades the method.

1. **A structural index pays as a pull tool, mostly on multi-file typed work** (arXiv 2606.22417, n=80 paired, one
   model, vendor authors: resolve 50.4% against 41.9%, localization acc@5 84.5% against 44.3%; Go 47.1 against 29.9,
   Python 47.6 against 45.5; against a grep-capable harness +6.0 pp, not significant). Medium.
2. **Locations alone barely move repair; a failure diagnosis does** (arXiv 2610.25826, 16,248 trajectories: adding
   fault localization lowered pass@1 in 17 of 18 settings). Its oracle table used one model: gold file +0.8 pp, gold
   symbol +2.8 pp, bug cause +4.4 pp, both +6.6 pp. The "bug cause" was generated with the gold patch, so it supports
   review and incident evidence cards, not cards about design rationale. High for the negative result.
3. **Evidence first, then a general agent, wins RCA** (EviRCA 40.6 to 43.9% against 15.2%; PRAXIS up to 6.3x accuracy
   on 30 incidents; general agents now beat bespoke RCA agents). Medium.
4. **Precision decides review adoption** (SWR-Bench best F1 19.38%; review-agent-only PRs merged at 45.2% against
   68.4% for human review; Uber 65% addressed after suppressing low-value kinds; patches shown to reviewers slowed
   reviews over 5%, shown to authors only, not). High for the production telemetry.
5. **Structure should feed a calibrated risk tierer, not replace it** (Meta: logistic regression captured 27.9% of
   SEVs at the riskiest 10% of diffs; DRS-OSS ROC-AUC 0.895 on ApacheJIT). High for scale, medium for causality.
6. **Deterministic AST graphs beat LLM-extracted graphs, and non-code artifacts are where everyone is weak** (an LLM
   graph missed 377 files; MULocBench: config, docs and build localization under 40% Acc@5). Medium.
7. **Context narrow and just in time; repository overviews do not help** (the AGENTS.md study across agents: no
   success gain, over 20% cost; augmented tool descriptions +5.85 pp at +67% steps). Anchored claims with separate
   freshness beat prose notes (EA-Graph), but for a smaller model on generated repositories, the larger model at
   ceiling. High for the overview result.
8. **Conformance checking against a spec works; LLM-written formal specs do not yet** (best 8.6% semantically correct
   TLA+; AWS's PObserve checks production logs against a P spec; 71.4% of scanner-passing IaC repairs left the
   vulnerability, TerraProbe). High.
9. **Benchmarks overstate** (the buggy file named from the issue text up to 76% of the time; one agent 19.25% on
   SWE-bench Live against 43.20% on Verified; RepoProbe grades comprehension by a checklist, with a model judge). High.
10. **Intent extraction works for explicit, code-inferable links and fails on rationale; specs are checked mainly at
    completion, not continuously** (980 ADRs from 109 repositories; doc-to-code trace F1 about 80% with phantom
    links). Medium.

**Where the literature is thin, and fathom could define the benchmark:** Well-Architected and compliance review over
code and config with measured precision; function-level blast radius tied to incident capture; code-to-config-to-deploy
evidence for RCA; intent-to-code traceability at the goal, invariant and decision level with continuous re-checking.

## 6. What the red team changed

The owner-architect accepted the red team's findings on measurement, the CUE law, the why card and sequencing:

- Public, independent benchmarks first (RepoProbe, ORCA-bench, GitHub ReviewBench, MULocBench, ApacheJIT); an
  owner-built bank only for the gaps. The design's 20-question banks with a +0.10 bar could not exclude 0 (an estimate
  of a 95% half-width of 0.11 to 0.15 at n=20), so every prereg now carries a power analysis and Holm correction.
- A deterministic anchor match is the primary metric everywhere; a benchmark's own model judge is a labelled secondary.
- Intent links are sparse and noisy: live links from `ADR.md` and `docs/intent/` reach 587 distinct symbol names of
  about 71,000, and about 8 of 25 sampled exact-tier links named common words (measured 2026-10-28). So plan-row
  contracts lead the intent trace, and the symbol why card waits for a reading and never recomputes verify inline.
- The project map is deferred; drift reuses `staleness_report`; the CUE law shrinks to a vocabulary and three HEAD
  checks; goals carry thresholds a null fails; channels stay local until Q5; the table freeze is not amended.

## 7. Open owner questions

| Id | Question | Proposed effect |
|---|---|---|
| Q4 | Force multiplier or product for others? | "Product for others" keeps ADR-084's frame; "force multiplier only" narrows it to the owner's fleet |
| Q5 | The threat model for a non-loopback daemon | Gates every hosted channel (a hosted reviewer, a hosted SRE agent); ADR-083's loopback-only rule stands until then |
| NP-Q1 | Gate the table freeze on the tenant clauses alone? | Only phase 3 (the project map's later layers, overlays, cross-repo joins) needs it |
| U3 | Who labels WAE-M0? | Unchanged; the review readings reuse its labels rule |
| Q38 | Who confirms the key items of the owner-built gap bank? | No gap-bank item counts toward a primary metric until a person confirms it |
| Q39 | The API cap for the understanding, ORCA-bench and reviewer readings | One cap, the shakedown first; no claim from the shakedown |
| Q40 | May a guide name a third-party meaning-search tool if it wins T1? | Yes as documentation with its reading cited, never bundled (F95) |

## What this assessment does not claim

- No fathom reading exists on any public benchmark; every comprehension, review and RCA effect here is unmeasured.
- Vendor figures are vendor claims; star counts are aggregator snapshots of different dates.
- The licence, version and key provenance of each public benchmark were not read; the prereg records them before data.
- The market table summarizes; it is not a ranking of products.

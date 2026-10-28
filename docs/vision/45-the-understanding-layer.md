# 45 - The understanding layer: the evidence agents and people call to be right about a codebase

Status: accepted 2026-10-28 (ADR-084 to ADR-089, owner-architect rulings on the design pass). Design:
`docs/design/project-understanding.md`. Plan: `docs/plans/project-understanding.md` (PU-). Roadmap:
`docs/vision/38-proof-layer-roadmap.md` section 9.17. Assessment: `docs/assessments/2026-10-28-project-understanding.md`.
The owner's ask of 2026-10-28: make fathom the code and project understanding tool, far better for code,
Well-Architected, operational, compliance and risk reviews, for incident prevention and response, and the best at
mapping intent, vision, goals and specs; and look again at CUE state machines for that intent.

## 1. The question

Every coding tool now says it understands your codebase. Most of them generate prose about it, search it by meaning,
or review it with a model. None of them can say why its answer should be believed. Product vision answered the
comprehension question in July with "staffed never": no decision moment and no consumer. In 2026 both arrived.
Reviewers, SRE agents and portals opened MCP and skills slots, and the research started measuring understanding where
it pays: a repair, a review, an incident.

## 2. The answer

Fathom becomes the evidence layer agents and people call when they need to be right about a codebase. An answer from
fathom says what the code is, why it is that way, which record governs it, what a change or an incident reaches across
code, config and intent, and whether the work still matches the plan. Each answer is computed from deterministic facts,
graded against an oracle where a grade exists, and carries a receipt a stranger can re-check with git. It reaches people
and agents through fathom's own verbs, tools, files, SARIF and hooks, inside the agents they already run.

It does not become a wiki, a reviewer, an RCA agent, a chat surface or a semantic search engine, and it never
mitigates an incident. The wedge is the combination nobody ships: receipts, graded breadth across code and the config
and IaC files where every localizer is weak, and intent checked as data and joined to code.

## 3. Measure before claiming

Fathom cannot yet show that it helps an agent: its newest home-bench reading is 41 successes against grep's 44 at 1.81
times the tokens. So the first deliverable is the bench it will be judged on. Public, independent benchmarks supply the
questions and keys (RepoProbe, ORCA-bench, GitHub ReviewBench, MULocBench, ApacheJIT); fathom writes a question only
where none exists. The primary metric is a deterministic anchor match against an oracle fathom does not own. Every
reading is powered for the effect it claims and reports each question type separately, losses included. No
understanding claim appears anywhere until a reading wins; a surface that does not win its question type is cut.

## 4. What fathom adds, smallest first

- **The intent trace.** `fathom why ADR-018` walks a record to the plan rows that cite it, the symbols their contracts
  bind, the verdicts the gate last recorded and the receipts that re-check them. This is the vision to goal to plan to
  symbol chain, computed, and it is where fathom alone holds the data.
- **A status vocabulary and three checks for intent.** Markdown stays the source. CUE declares the closed set of
  statuses decisions, incidents, futures and goals may carry, and `fathom intent audit` checks, at the current commit,
  that a superseded record names a real successor, a closed incident names a durable outcome and a goal past review
  records its keep or kill.
- **One evidence contract for reviews.** Code review findings go to the author's agent and show only above a measured
  precision floor. Well-Architected and readiness evidence ride the plans that already own them. Compliance gets
  evidence, never verdicts. Risk gets a feature file for the adopter's tierer.
- **Evidence cards for incidents.** A receipt whose anchors form the causal path, so an external agent's diagnosis can
  be graded "grounded". "Real-time" is a latency figure fathom must earn first.

Later, only on evidence: the symbol why card inside `symbol_context` (once the bench shows a gap it can close) and a
computed project map (once the bench's orientation reading says overviews help here).

## 5. What this does not do

No generated prose about code. No committed map. No embeddings. No new MCP tool, no table, no hosted service. No
model-written intent and no CUE-authored records. No transition history written by fathom. Nothing new at the edit.
Hosted agents that need a reachable fathom wait for the owner's threat-model answer (Q5).

## 6. How we will know

The bench's baseline (PU-M1) decides G46. The lifecycle checks' first runs (PU-M4) decide G47. The reviewer reading on
GitHub ReviewBench (PU-M7) decides G48. If by 2027-03-31 the baseline and the arms read an adequately powered null on
every primary question type, fathom withdraws the understanding positioning and stays the evidence engine for review,
compliance and intent, where its gate and receipts already lead.

## 7. Records

ADR-084 (position and channels), ADR-085 (the bench), ADR-086 (the intent trace, the why card, drift, the deferred
map), ADR-087 (statuses in CUE, markdown as source), ADR-088 (review kinds), ADR-089 (incidents); goals G46 to G48;
futures F93 to F96; questions Q38 to Q40.

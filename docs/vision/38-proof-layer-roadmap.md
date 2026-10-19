# 38 - The proof layer, widened: the roadmap through 2027-01

Status: accepted 2026-09-27 (ADR-049, ADR-050, ADR-051). Owner decision memo of 2026-09-27
over the agentic-coding landscape scan of the same date (about 60 tools), the owner's notes
N1 to N8, and its binding addendum "What only fathom can do" (T1 to T8, U1 to U16). Designs:
`docs/design/system-graph.md`, `docs/design/spec-interop.md`,
`docs/design/knowledge-retrieval.md`, `docs/design/governance-learning.md`,
`docs/design/progress-and-economics.md`, `docs/design/reach.md`,
`docs/design/test-intelligence.md`, `docs/design/work-intelligence.md`,
`docs/design/change-and-compliance.md`. Plans: `docs/plans/system-graph.md` (SG-),
`docs/plans/spec-interop.md` (SI-), `docs/plans/knowledge-retrieval.md` (KR-),
`docs/plans/governance-learning.md` (GL-), `docs/plans/progress-and-economics.md` (PE-),
`docs/plans/reach.md` (RH-), `docs/plans/test-intelligence.md` (TQ-),
`docs/plans/work-intelligence.md` (WI-), `docs/plans/change-and-compliance.md` (CX-). Goals
G16 to G25, and G15's named M2 and M3, in `docs/intent/scope.md`. Promotes futures F20, the plugin part
of F2, F24, and the computable subset of F9; promotes F14's output format only.

## 1. The thesis stays; the ambition widens

Fathom is the proof layer for code: it computes what breaks, which rule governs, and whether
work is really done, and it cites the evidence. The 2026-09 market has many ways to find
code (grep, language servers, a dozen new graph tools) and many ways to describe work
(specs, memories, rules files). It has very few ways to prove things about code. Spec Kit's
converge step and OpenSpec's verify ask a model; review bots guess at impact; the new graph
tools read syntax inside one repository. That gap is fathom's lane, and review and
governance is where the year's money went.

The roadmap does not add engines for their own sake. It widens what fathom can prove, takes
the proof to where people already work, and makes it visible.

### 1.1 Where we stand (read from the tree, not argued)

| Fact | Evidence |
|---|---|
| A cross-service edge already exists: `service_ref`, a heuristic-tier join of Go HTTP client call sites to route registrations on a normalized (method, path template) key | `internal/analysis/serviceref` (VB-30) |
| Language-neutral inventory keys `route:`, `rpc:`, `env:`, `cli:` are extracted for conservation | `internal/analysis/inventory` (ADR-044) |
| `dep_drift` and `cross_repo_blast_radius` are registered with zero lifetime calls; `package_deps` was already deleted by the surface prune | `internal/mcp/extra_tools.go`; `internal/mcp/surface_prune_test.go` |
| The plugin directory is generated (Claude Code and Agent Plugins 1.0 manifests) but not published or install-tested in a second client | `fathom plugin render` (`cmd/fathom/pluginrender.go`, IO-25, RP-6) |
| Gate refusals, `intent audit` and `config audit` render SARIF 2.1.0 | `internal/verify/sarif.go` (PC-21) |
| Semantic claims carry a bi-temporal validity window and `obligations` answers `as_of` | `internal/core/claims.go` `ValidFrom`/`ValidTo` (SU-19, ADR-041 decision 7) |
| The rule compiler reads `CLAUDE.md` and `AGENTS.md` as instruction files, no other client's rules | `internal/semantic/compile.go` `instructionFileBases` |
| Cursor and Copilot are descriptor rows; Gemini CLI ships `unverified`; Claude Code and Codex are verified | `internal/harness/descriptors_data.go`; scope "Harnesses" |
| Every verify run appends one row per plan (counts, commit, provenance, origin) to `~/.fathom/verify-ledger.jsonl`, about 17,000 readings on the reference machine; readers take only the newest row per plan; rows carry no per-row verdicts yet | `cmd/fathom/verifyledger.go` (SD-1, schema 2) |
| The session ledger holds 3,359 sessions; the Stop hook folds each transcript incrementally | `~/.fathom/session-ledger.jsonl`; `cmd/fathom/transcript_offset.go` |
| `blast_radius` read precision 1.00 on 9 audited calls | feature audit, 2026-09-27 |
| The learning signals are empty: 0 memories, 0 human claim verdicts, 0 live certificate declarations on the reference machine | feature audit, 2026-09-27 |
| The requirement-to-test link already exists as a proof contract: `certified_by:` verifies only on a `live` certificate (newest attested run passed, bound to a clean commit, reach unchanged) | ADR-043, `docs/design/test-certifications.md` |
| The pure-Go SQLite driver supports FTS5 with `bm25()` and `snippet()` | probe on `modernc.org/sqlite` v1.52.0 (SQLite 3.53.2), 2026-09-27 |
| The agent surface is 25 tools: 19 lookups, five promoted verbs (ADR-035) and `safe_delete_check` | `internal/mcp/surface.go` `companionTools` |

## 2. The kitchen-sink gate

Every accepted item names which of these it serves and the instrument that proves it.
Anything that serves none is refused, however popular.

- **(a) Widen what fathom can prove**: more languages, more edge kinds, across repositories,
  services and dependencies.
- **(b) Deliver proof where people already work**: their clients, their spec tools, their
  rules files, their reviewers.
- **(c) Make the proof visible**: progress and economics computed from instruments.

An item that names a letter but no instrument is not accepted yet; it goes to
`futures.md` with a Promote-when.

## 3. The pillars

| Pillar | Serves | Goals | What it adds |
|---|---|---|---|
| **Widen the graph** | a | G15, G17, G18 | languages (running), the system graph (one link model over routes, RPCs, GraphQL, topics, tables, config, images and dependency APIs), dependency API indexes read from local caches, a service catalog that cannot rot (U10) |
| **Proof depth** | a | G23 | test intelligence: risk-weighted coverage, the test portfolio, coverage for every language, certifying-first selection, contract certificates on links, upgrade test reach (T1 to T8) |
| **Know the words** | a | G20 | full-text retrieval over docs, decisions, plans and memories; doc spans linked to code; doc drift as a staleness fact |
| **Deliver where work happens** | b | G16, G19, G21 | one-plugin install, a small default surface, every client's rules and hooks, spec-tool interop with a converge report, rules that learn and are measured (U4), SARIF and a check for reviewers |
| **Review and compliance** | b | G25 | the change risk receipt (U7), the traceability matrix and evidence pack (U8), the migration tracker (U9), the architecture drift radar (U11) |
| **Make proof visible** | c | G22 | proof history as a projection of the verify ledger, weighted progress views, outcome economics (cost per proven row) over the session ledger |
| **Work intelligence** | c | G24 | the joins of agent activity, code and proof: cost per proven outcome by zone (U1), the debt interest rate (U2), the friction map (U3), the model and harness scorecard (U5), task preflight (U6), proof regressions generalized (U12), hotspots with context (U13) |
| **Stay a good guest** (lane) | all | G14 | the tenancy budget every pillar lives inside |

## 4. What only fathom can do: the joins

Fathom holds eight assets, locally and deterministically:

1. a resolved, typed call graph across repositories (soon the system graph and the dependency plane);
2. governing claims with citations, served and enforced at the edit;
3. proof: verify of plan rows, certifications, conservation, and proof history;
4. receipts and evidence tiers on every answer, with drift and staleness;
5. the agent activity record: the MCP call log, the hook log, session transcripts, discoveries, threads;
6. the machine-readable intent corpus: invariants, decisions with revisit triggers, goals as bets;
7. the fleet on one machine;
8. git history joined to all of it (watermarks, co-change).

Every other tool holds one or two. The unique products are the **joins**, and each accepted
one is a projection and a view over assets that already exist, never a new engine. That is
this part's kitchen-sink guard: a join that needs a new engine is sent back.

**Test intelligence (T, plan TQ-, pillar Proof depth).** Certificates, liveness, the
`certify` verbs and Go coverage by layer shipped (vision 33, ADR-043, TC-1 to TC-24, RC-37
onward); TC-M2 to TC-M4 stay open in `docs/plans/test-certifications.md` and are referenced,
not duplicated. Added: T1 risk-weighted coverage (coverage times centrality, churn,
governance severity and incident history, ranked); T2 the test portfolio (orphan,
redundant, and critical-uncertified); T3 coverage ingest for every language (lcov,
Cobertura, JaCoCo, coverage.py JSON, Istanbul, llvm-cov); T4 certifying-first test selection
for a diff; T5 contract certificates on system-graph links (takes TC-25); T6
dependency-upgrade test reach; T7 flakiness and commit-bound receipts made measured (the TC
rows stand); T8 the requirement-linked tier, which is `certified_by:` (§10).

**The ranked products (U).**

| # | Product | Verdict | Joins (assets) | Plan |
|---|---|---|---|---|
| U1 | Cost per proven outcome, by plan, language and code zone, with CI minutes | accept (extends N3) | 5, 3, 1 | PE-, WI- |
| U2 | Technical-debt interest rate: measured agent effort per proven change, per zone | accept | 5, 3, 1, 8 | WI- |
| U3 | Agent friction map with computed remedies (proposals, never applied) | accept | 5, 2, 4 | WI- |
| U4 | Rule effectiveness: served, complied, violated, blocked, refuted per rule | accept, in governance learning | 2, 5 | GL- |
| U5 | Model and harness scorecard on your own code, descriptive, n and confounders shown | accept | 5, 3 | WI- |
| U6 | Task preflight: a difficulty profile and a recommended model tier, advisory | accept; the best Jev site, A/B'd | 1, 2, 3, 5 | WI- |
| U7 | Change risk receipt for a PR, every reason cited, suggested reviewers | accept | 1, 2, 3, 4, 8 | CX- |
| U8 | Traceability matrix and a dated evidence pack that re-verifies | accept (export on request) | 6, 3, 1 | CX- |
| U9 | Migration tracker computed from code, proven complete by conservation | accept (tracks, never transforms) | 1, 3, 8 | CX- |
| U10 | A service catalog that cannot rot, exportable to catalog-info on request | accept, in the system graph | 1, 3, 7 | SG- |
| U11 | Architecture drift radar: decisions compiled to rules, triggers evaluated | accept (promotes F24, F9's computable subset) | 6, 1, 8 | CX- |
| U12 | Proof regressions generalized, with the breaking commit | accept | 3, 8 | WI- over the verify ledger |
| U13 | Hotspots with context, a view over T1's data | accept as a view | 1, 3, 8 | WI- |
| U14 | Onboarding path | defer (F30) | | |
| U15 | Capability reach (network, filesystem, exec) | defer (F31) | | |
| U16 | Doc truth | covered by doc drift (G20) | | |

**A dependency the joins share.** The learning products (U4, N8.1) and parts of U3 read
signals that are empty today: 0 memories, 0 human claim verdicts, 0 live certificate
declarations on the reference machine. GL-10 builds the signal source (hook outcomes as
complied, violated and blocked events) before any threshold can move a rule, and GL-M2 is
not read until 30 days of signal exist.

## 5. Verdicts on the owner's notes

| Note | Verdict | Serves | Shape |
|---|---|---|---|
| N1 dependencies | accept, reshaped | a | never download; read what the toolchain already fetched; index each (package, version) public API once per machine, content-addressed, shared by every service, lazily and under budget; upgrade impact, usage map, vuln reach (wave 4), version-exact docs. The lockfile graph comes through a port; ci-kit owns the engine (vision 15 §3). Dependency API is the `pkg:` kind of the system graph (ADR-050) |
| N2 progress | accept as views | c | status is derived by verify, never set. Optional row `weight:`; proof history is a projection of the existing verify ledger (about 17,000 readings), which gains per-row verdicts additively (schema 3) so burn-up and regressions are computed; views listed in `docs/design/progress-and-economics.md` |
| N3 usage dashboard | accept narrowly | c | outcome economics only: cost per proven outcome, a projection joining the session ledger (3,359 sessions) and transcripts to verify readings. A generic token dashboard is refused |
| N4 spec formats | accept (promotes F20) | b | a spec-dialect port with plan grammar v2 as the canonical model; read in place; converge report; `weight:` and `accept:` added; the test-tied tier is the existing `certified_by:`; Jev proposes anchors for unanchored foreign tasks behind a calibration gate |
| N5 languages | accepted (ADR-048); ladder named | a | vision 37 §5's M2 and M3 named in G15 as the next bets (§9.3) |
| N6 cross-service | accept as the system graph | a | one provides/consumes model, one link table, three verbs, four evidence tiers; heuristic links never gate (I57) |
| N7 documentation search | accept | a | FTS5, section-aware, BM25 fused with concern ranking; doc-to-code links; doc drift; no embeddings (vision 31 stands) |
| N8 semantic layers | accept four upgrades | b | rules that learn under published thresholds (after a signal source exists); validity windows activated and served everywhere a fact ends (the columns exist); every client's rules files as sources; serve through every client's hooks; SARIF and a check for reviewers |

## 6. Verdicts on the landscape's Now, Next and Later

| Item | Verdict | Where |
|---|---|---|
| One-command plugin (Claude Code marketplace, Copilot Agent Plugins) | accept; publication stays the owner's Q4 call | reach, RH- |
| Small default surface (six to eight tools, rest opt-in) | accept | reach, RH- |
| Proof layer under Spec Kit and OpenSpec | accept | spec interop, SI- |
| Every client's rules files, with off/warn/error | accept | governance, GL- |
| Hooks for every client | accept | governance, GL- |
| Rules that learn from feedback | accept | governance, GL- |
| Invalidated facts with dates | accept (extends SU-19) | governance, GL- |
| SARIF and a GitHub check for reviewers | accept (output format of F14 only) | governance, GL- |
| Calls between services | accept | system graph, SG- |
| Tests tied to requirements | accept as `certified_by:` for foreign dialects | spec interop, SI- |
| Layered diffs by blast radius, no model | accept | governance, GL- |
| Cost of every answer | accept | progress and economics, PE- |
| Co-change scoring as an independent oracle for `blast_radius` | accept | system graph, SG- |
| Live updates | accept under budget, with an incremental-equals-full check | system graph, SG- (wave 4) |
| Full-text search | accept, pulled to wave 2 | knowledge retrieval, KR- |
| A shared team index snapshot | accepted as a design, parked; promoted only through F27's amended trigger (IXS-M1 KEEP), after parity is proven (IXS-M0b) | index snapshots, IXS- (ADR-069) |
| Answer events for background agents | defer | futures F28 |

## 7. What we refuse (recorded so it is not re-proposed)

- A model deciding done. Jev and every model propose; only a contract proves.
- Auto-capture without a budget.
- Redis, a graph database, a vector database, or any second store.
- A generic token-usage dashboard.
- Kanban write-back, or any status set by a drag or a click.
- Downloading dependencies, advisories or docs by default (I56).
- A tool per edge kind (law 4): one link model, three verbs.
- Embeddings before the vision 31 bench passes.
- Hosted anything (the scope non-goal stands).
- Rewriting a foreign spec file outside fathom-marked blocks (law 15).
- A productivity score for a human, or any ranking of people (I58). Per-author figures are
  agent identities (model, harness, effort), never persons.
- Predictive estimates ("AI will size your sprint") without a calibrated reading.
- Any auto-applied remedy: friction remedies and learned rules are proposals with receipts.

## 8. Stack, and the Rust verdict

**The stack stays.** One static Go binary, pure-Go SQLite in WAL mode, tree-sitter grammars
on wazero, no Node, Python, Docker or JVM runtime, one loopback daemon under an OS-measured
memory budget (ADR-047). The peers in the scan are mostly TypeScript or Python processes
with Chroma, Kuzu, Neo4j, Redis or Postgres beside them and one language server per
language; the scan records users reporting tens of gigabytes for some clients. Fathom's
stack is among the leanest and most predictable in the field. That is a reading of their
documentation, not a head-to-head benchmark, and RH-M5 measures it on one shared corpus
before anyone repeats it as a claim. Fathom's own costs are known: garbage-collected heap
growth on large graphs (ADR-047's waves) and wasm parser memory (RT-107).

**Rust: do not rewrite.** Go is not the bottleneck: parsing is tree-sitter compiled to wasm,
the store is SQLite. A Rust extraction worker would add a second toolchain, a process or FFI
seam and a second language on a bus factor of one, for gains the budget work already
delivers. **Revisit when** the typed-pass or extraction children cannot meet the 300 MiB
ingest target after the RT waves, or garbage-collection pauses show in hook p99 at fleet
scale (RT-M7's GC CPU reading above 25% below the refuse rung). The cheap experiment if
that fires is a Rust-compiled wasm grammar or query module run under wazero: no FFI, the
same sandbox, the same module cap (futures F29).

The peer-stack survey behind this section (about 30 peers, their resource complaints with issue numbers, the licence
screen, and Rust in peers, where no source published a Rust-versus-Go benchmark) is summarized in
`docs/assessments/2026-09-27-peer-stacks-and-rust.md`, with its data beside it.

## 9. Sequencing

### 9.1 Lanes (run throughout)

- **Lane A, tenancy (G14).** Finishing. It gates everything that adds load: no wave-2 row
  that adds a table, a background job or a cache starts until RT-M1's first week reads PASS.
- **Lane B, languages (G15).** Running on the ADR-048 queue; the system graph's framework
  extractors ride on its dialects.

### 9.2 Waves

| Wave | Theme | Items (plan rows) |
|---|---|---|
| 1 | Reach: cheap, high leverage | default surface and opt-in groups (RH-1..RH-4); plugin bundle install-tested in two clients (RH-5..RH-8); rules readers for six client formats (GL-1..GL-6); hooks for Cursor and Gemini verified (GL-7..GL-9); spec-dialect port with Spec Kit and OpenSpec adapters and the converge report (SI-1..SI-9); SARIF for obligations and review findings (GL-20..GL-21); cost in every answer (PE-20..PE-21) |
| 2 | Deepen | system graph core: the key grammar, the tables, the port, route/rpc/OpenAPI/proto extractors and HTTP clients, the three verbs (SG-1..SG-16); FTS5 docs and doc drift (KR-1..KR-10); proof history and progress views (PE-1..PE-12); the learning signal source and learning rules (GL-10..GL-14); the change risk receipt U7 (CX-1..CX-8); proof regressions generalized U12 (WI-20..WI-22); risk-weighted coverage T1 and the test portfolio T2 (TQ-1..TQ-8) |
| 3 | Widen | dependency API index and upgrade impact (SG-30..SG-40); topics, tables and config links (SG-17..SG-22); contract drift (SG-23..SG-25); validity windows beyond claims (GL-15..GL-17); outcome economics (PE-13..PE-19); co-change oracle (SG-26..SG-27); layered diffs (GL-22); U1 to U5 (WI-1..WI-15, GL-18..GL-19); coverage for every language T3 and certifying-first selection T4 (TQ-9..TQ-16); the migration tracker U9 (CX-9..CX-13); the service catalog U10 (SG-43..SG-45) |
| 4 | Complete | live updates under budget (SG-28..SG-29); Kiro, BMAD, Task Master and GitHub task-list adapters (SI-10..SI-13); the certified proof tier everywhere (SI-14..SI-15); Jev anchor proposals behind their calibration gate (SI-16..SI-17); vuln reach (SG-41..SG-42); dependency docs (KR-11..KR-12); task preflight U6 (WI-16..WI-19); the evidence pack U8 (CX-14..CX-18); the drift radar U11 (CX-19..CX-23); contract certificates T5 and upgrade test reach T6 (TQ-17..TQ-20) |

Each wave closes with its measurement rows (the `-M` rows of each plan) read and committed,
honest either way.

### 9.3 G15's ladder, named

N5 asked to raise the milestone ladder. Vision 37 §5 already defines M2 (45 languages at L1
with 35 excellent, 25 at L2, 14 at L3, 10 at L4; 30 tool formats) and M3 (60+ at L1, 35 at
L2, 20 at L3, 12 at L4; 40+ tool formats), with plan rows LB-M6-M2 and LB-M6-M3. Those stand
unchanged; the memo's indicative figures are not used. Scope G15 now names M2 and M3 as the
next bets after M1. Today no language has a committed reading and only Go (and partly SQL)
fill `Signature`, so no other language can prove L1 good until LB-F3 lands: every current
level is an unmeasured estimate.

### 9.4 Amended 2026-10-10 (ADR-067, ADR-068): the repository graph, the delivery repairs and portable receipts

Visions 42 and 43 add no pillar. Their work rides two pillars and one lane:

- *Widen the graph (a).* The role facet, `runs` edges that never gate from their first row, the fleet-wide reach
  guard after RF-M6's published count, `configures` facts, non-Go entry points (informational first) and non-Go test
  entries (`docs/plans/repository-facts.md`, test-intelligence TQ-27 and TQ-28), goal G36. Instrument: RF-M1 to
  RF-M3 and RF-M5, all $0, and the flip counts RF-M6 and RF-M7.
- *Deliver where work happens (b).* The rule-outcome writer and hook reasons (GL-24), the destination class and the
  clone fold (XM-36, XM-37), the pin line's HEAD stamp (PR-13), coverage into the risk rank (TQ-26), attested runs
  into `runtime_pass` (TC-29, I8 and ADR-023 amended), stored churn as an IM-M0 arm, then the file passport
  (RF-14). Portable receipts (vision 43) carry fathom's evidence to tools that never run fathom: SARIF provenance
  on all seven callers, then anchored receipts a stranger checks with git (`docs/plans/portable-receipts.md` waves
  1 and 2, fillers), the record after EVX-G3 and replay after EVX-M3. Instrument: GL-M6, XM-M8, the pin skip share,
  IM-M0, RF-M4, EVX-K1 and EVX-M1.
- *Lane B, languages.* The web, scripting and ops-format queue edits of vision 42 section 9.

Order, at most two waves at once and none started during an RT-M1 window while the swap volume has under 15 GiB
free: a hygiene wave for the delivery repairs and the unread-fact joins, then the role classifier, then language
wave lf5, then the guard, `runs` and entry points, then lf6, then `configures`, test entries, review dimensions and
the priced dry run, then lf7 (TypeScript TSX, shell, Vue, Svelte, Angular and Astro), then lf8 (ops formats, Kubernetes facts including
LB-F451, GitHub Actions, Helm, Jenkinsfile, observed-by TL-15), then the passport. Vision 43's waves 1
and 2 ride beside the role classifier and the guard waves. RT-M1 reading PASS remains the one gate in front of SG-2,
the dependency plane, the feature map and the runtime topology (vision 42 decision D14).

Corrected 2026-10-11: the ruling of 2026-10-11 moved the ops formats from lf7 to lf8, because the web frameworks
needed TypeScript and HTML first. The ruling itself sits in gitignored workspace notes; this correction is its
committed record.

Index snapshots (`docs/design/index-snapshots.md`, ADR-069, plan `docs/plans/index-snapshots.md`, IXS-) are
parked, not scheduled. war1 carried the only pieces: the IXS-M0 timing reading (read CUT 2026-10-16, cache-from
0.81x cold against the 0.6x bar, so phases 1 and 2 do not start), the IXS-0 machine-local fix, and WAE-1 to WAE-3,
which the design also uses; all landed in v0.12.0. Nothing is published. The rest starts when the owner prioritizes
it, and no earlier than war1 landing and RT-M1 reading (its first amended window closes 2026-10-19 00:00Z). Each
later phase also needs its own trigger: phase 1 (`export snapshot`, `--cache-from`, IXS-M0b) the owner's go and
IXS-M0 KEEP; phase 2 (the publish recipe, IXS-M5) the owner naming the first producing repositories; phase 3
(`--read-only`, the answer basis, a read-only sidecar) a named sidecar consumer; phase 4 (`import snapshot`)
IXS-M1 KEEP and a named cross-repo consumer; phase 5 (the environment daemon, F74) the owner's U1 answer, the
owner's ruling that a lineup change is not attach, IXS-M3 KEEP, phase 4 built and one named service with an
on-call owner; phase 6 (versioned cross-repo links) RT-M1 PASS and SG-2; phase 7 (the overlay store) IXS-M5
KEEP; phase 8 (shared compiled layers) the spend hold lifted. Every phase carries a kill that can fire after it
starts (the design, section 12). Its waves count against the two-wave limit and ride as fillers; language
excellence stays the main line. Deployment options and their costs: `docs/design/index-snapshots-deployment.md`.

Paused 2026-10-11 (credits). war1, PowerShell L3, lf8, the evidence-plane waves and the RT-M1 reading wait in
`docs/plans/resume-2026-10.md` (RES-), with the rulings a cloud session follows, so one session can take one item
from the repository alone.

### 9.5 Amended 2026-10-11 (ADR-069): evidence before the incident, and index snapshots

Vision 44 adds no pillar. Its fact rows ride lf8 (ops formats, Kubernetes facts, observed-by) as WAE wave 2,
after Review Kit commits the bundle's requirements; its joins and fact receipts follow WAE-M2 KEEP. Index
snapshots are parked under section 9.4's rule: only IXS-M0 and IXS-0 ride war1. Language excellence stays the
main line. Instrument: WAE-M0 to WAE-M7 (WAE-M7 now with a fail-open bar) and IXS-M0 to IXS-M9, all $0 in model
spend. RT-M1 PASS still gates SG-2 and with it F72. G37's kill date, 2027-01-31, covers the WAE waves only; index
snapshots get their own goal when the owner starts them.

### 9.6 Amended 2026-10-16 (ADR-071): readouts is the next priority program, then the Claude Code mods program

The owner set the sequence on 2026-10-16: readouts (`docs/design/readouts.md`, plan `docs/plans/readouts.md`, RO-,
goal G39) is the next priority program after the RT-M1 reading, and the Claude Code mods program follows it (planned
in section 9.7; F78's vehicle is a deferred row there). While a readouts row is runnable, no other program's wave
starts except the RT-M1 reading itself and defect fixes; language excellence, the evidence-plane waves and index
snapshots resume after readouts reaches its gates (RO-M0 and RO-M1 read, and, on KEEP, RO-M3 for the rules turned
on) or RO-M0 reads CUT. Phase 0 needs no table, cache, background job or model spend, so it starts after the RT-M1
reading whatever that reads; phase 3 waits on RT-M1 PASS, the tenancy budget fix and EW-76; phase 4 on the owner
lifting the budget hold. The two-wave limit and the swap-volume rule of section 9.4 still apply.

  - 2026-10-18 (owner-architect): phase 0's waves start before the RT-M1 reading. No readouts row touches what RT-M1 measures (the daemon's footprint, refuse windows, exit rungs, unclean exits), and RO-15's default mode off keeps every hook silent; RT-M1 still reads after 2026-10-19 00:00Z and, on FAIL or INCOMPLETE, the tenancy budget fix (O1073, O1014) joins as the second wave.

### 9.7 Amended 2026-10-16 (ADR-072): readouts, then Claude Code mods, then the tenancy program as RT-M1 reads

The priority order: readouts first (section 9.6, goal G39); then the Claude Code mods program
(`docs/design/claude-code-mods.md`, plan `docs/plans/claude-code-mods.md`, MOD-, goal G40); then the tenancy program
(`docs/plans/responsible-tenant.md`, RT-) as RT-M1 reads: PASS opens what it gates (the table freeze lifts; SG-2 and
F72; readouts phase 3 with the tenancy budget fix and EW-76), and FAIL or INCOMPLETE makes the tenancy budget fix
(O1073, O1014) the next wave. The RT-M1 reading itself (its amended window closes 2026-10-19 00:00Z) and defect
fixes run whenever they are due. Section 9.6's parked programs (language excellence, the evidence-plane waves, index
snapshots) keep its rule: they resume once readouts reaches its gates, as fillers within the two-wave limit behind
the mods program, unless the owner reorders.

  - 2026-10-18 (owner-architect): RO-M0 read CUT (`docs/bench/readings/readouts/corpus-replay.md`; the eligible classes
    remove 0.53% of Bash bytes against the 4% bar). Readouts phase 0 has landed (RO-3 to RO-18 verified, RO-M0 filed),
    so mods phase 1 starts and the mods program is the main line. The readouts rows that stay runnable (RO-21 and the
    seam rows) fill the second wave slot, and RO-M1's observe window runs beside them.
  - 2026-10-19 (owner-architect): RT-M1 read FAIL on availability, not on fathom's consumption
    (`docs/bench/readings/tenancy/rt-m1-2026-10-19.md`: 216 host-starved refuse minutes on 2026-10-13, every tenant
    clause held). The table freeze stays. The tenancy budget fix is the next wave as RT-188 to RT-192
    (`docs/plans/responsible-tenant.md` §G), designed with RT-M13's retune because shed on host pressure is what
    evicts the graph; it ships before 2026-10-20 00:00Z so RT-M1's restarted window, 2026-10-20 to 2026-10-27,
    certifies the retuned ladder. RT-193 (the unmanaged default) waits for RT-M1 PASS.
  - 2026-10-19 (owner-architect): readouts phase 2's rows may be built now, every line they add reachable only under
    `[readouts] mode = rich` (user-only, default off); rich mode turns on in any session only at RO-M1 PASS, which
    measures the hook's cost and safety before any line is injected, not whether the code exists
    (`docs/plans/readouts.md` phase 2's Start line).

Mods phase 1 (Go and markdown) starts when readouts phase 0 lands (RO-3 to RO-18 verified, RO-M0 filed). RO-M1's 14
days of observe need time, not a wave, so they run beside it: section 9.6's rule reads a readouts row that waits on
its own measurement window or gate as not runnable, and mods waves fill those windows. Mods phase 2 (the fathom-ui
module) starts after RH-6 reads (one ordinary owner session, ADR-072 decision 7) and, while a readouts row is
runnable, only when the owner names the wave. Mods is the main line once readouts reaches its gates (RO-M0 and RO-M1
read, and on KEEP RO-M3 for the rules turned on) or RO-M0 reads CUT. Every mods reading is $0 in model spend, and no
mods row needs a table, cache or background job. The two-wave limit and the swap-volume rule of section 9.4 still
apply.

### 9.8 Amended 2026-10-16 (the seam map): interception seams inside readouts, mods and index snapshots

The owner rated thirteen hook seams (`docs/design/readouts.md` §20, goal G41) and asked that every one rated Medium or
higher be implemented and working. They ride the programs sections 9.6 and 9.7 already order, so no program is added:
the RT-M1 reading comes first (section 9.6: readouts follows it); then wave ro1, readouts phase 0 with H1, H2, H6 and
H11; then the tenancy budget fix, which every daemon-backed hook needs; wave ro2 is H3, H4, H8, H9 and H10 with rich mode; after RT-M1 PASS come H7 and H5 (index
snapshots' worktree rows run ahead of that plan's parked phases); H12 rides the mods program after its phase 2. Each
seam ends in a $0 reading over at least 7 days of real sessions; a row waiting on its window is not runnable under
section 9.6's rule, so other waves fill it. The two-wave limit and the swap-volume rule of section 9.4 still apply.

### 9.9 Amended 2026-10-16 (ADR-073): decision models after readouts and mods; the security fix and stage 0 ride wave ro1

The order: readouts (section 9.6, G39), then the Claude Code mods program (section 9.7, G40), then the decision-model
items rated Medium or higher (`docs/design/decision-models.md` section 5): the `fathom ask` spine at provider `none`
(`docs/plans/agentic-decisions.md` AQ-1 to AQ-6), readouts' decision rows inside their own phases
(`docs/plans/readouts.md` RJ-1, RJ-M1 and RJ-4 with phase 2; RJ-2 and RJ-3 with RO-32 in phase 4), disclosure
(`docs/plans/judgment-foundations.md` JG-1 to JG-5, with readouts phase 1 and the mods), and the port's pins, caps,
cache and kill criteria (`docs/plans/decision-port.md` KS-8 to KS-13, with readouts). Then the tenancy program as
RT-M1 reads (section 9.7), with the tenancy budget fix pulled forward as the daemon-backed seams' prerequisite (section
9.8).

Two pieces run earlier. The consent and kill rows (`docs/plans/decision-port.md` KS-1 to KS-7, DMP-1) are a security
fix, since a repository can today make fathom spend the user's ambient key, so they ride readouts' first wave (ro1).
Stage 0 of the agentic experiments (EXP-1 to EXP-M4, at most $2 of the owner's $10 for Jev on OpenRouter, G42) runs
beside wave ro1, after pol1 lands, because it touches only a build-tagged harness and readings. Stage 1 (at most $8)
runs only when EXP-M4 reads OPEN, as a filler within the two-wave limit; stage 2 and provider calibration (DMP-4,
DMP-6) wait on the owner. The two-wave limit and the swap-volume rule of section 9.4 still apply. Stage 0 read CLOSED
on 2026-10-17 (`docs/bench/readings/decide/exp-stage-gate.md`, US$0.0272 spent): stages 1 and 2 stopped, and the
spine keeps its place in the order above under its own kill.

## 10. Corrections to the memo (the tree or the memo itself disagreed)

1. **`tests:` is taken.** Plan grammar v2 already has `tests:` (a `gone: callers` modifier,
   ADR-044, in the compatibility surface). The test-tied tier is the existing `certified_by:`
   (ADR-043), whose `live` state already means "the newest attested run passed at a clean
   commit and the reach has not moved". No new key; foreign dialects map their
   requirement-to-test references onto it, and a plan-level `Proof: certified` header makes
   it the default for a plan (spec interop §4).
2. **Three zero-call tools are two.** `package_deps` was deleted by the surface prune. The
   system graph's verbs replace `dep_drift` and `cross_repo_blast_radius`; tool count stays
   flat (law 4).
3. **Bi-temporal claims exist.** SU-19 shipped `valid_from`/`valid_to` on `semantic_claims`
   and an `as_of` read on `obligations`. N8.2 is activate and serve, not a schema change for
   claims: every path that ends a fact sets the window, `as_of` reaches every verb that reads
   claims, and memories, rules-file rules, learned-rule states and doc links gain the same
   window additively.
4. **SARIF exists for the gate.** The new work is SARIF for obligations, review findings and
   system-graph drift, plus a check-run payload; the emitter is reused.
5. **The plugin renders already.** The new work is the published bundle, the trimmed default
   surface inside it, and a timed install in two clients.
6. **M2 and M3 exist.** Vision 37 §5 and LB-M6-M2/M3 already define both milestones; they
   stand as written and the memo's indicative figures are not used (§9.3).
7. **Proof history and economics are projections.** The verify ledger (about 17,000
   readings) and the session ledger (3,359 sessions) exist. N2's "proof history ledger" is the
   verify ledger with per-row verdicts added additively (schema 3), and N3/U1 join the session
   ledger; no new ledger file.
8. **Claim honesty per author.** N2 lists claim honesty "per author/agent"; the addendum
   refuses any per-human score. Claim honesty and every per-author figure are computed per
   agent identity (model, harness, effort) only (I58).
9. **Churn and complexity analyzers are gone.** `internal/analysis/churn` and
   `internal/analysis/cognitive` were retired (`docs/plans/retire-unused-analyzers.md`). T1
   and U13 do not bring them back: churn is a git-derived fact computed at ingest through
   `gitro`, and symbol size from L1 ranges (ADR-048) stands in for complexity, named as a
   proxy wherever it is shown.
10. **T5 is TC-25.** Contract certificates on cross-service links were deferred in
   `docs/plans/test-certifications.md` (TC-25, trigger TC-M2 plus two services declaring port
   certificates). TQ-17 takes the work with TC-M2 as its `needs:`.
11. **Learning needs a signal first.** N8.1 and U4 assume corroborate/refute signals; the
   reference machine has none (0 human claim verdicts). The signal source is a plan row
   (GL-10) that lands before any threshold row.

## 11. How this fits what is decided

Law 4 holds (one link model, three verbs replacing two dead tools, the default surface
shrinks). Law 15 holds (foreign spec files and rules files are read, never rewritten outside
fathom's markers). ADR-011 holds (no fixture, key or doc names an organization). ADR-041
holds (SQLite is the record, no graph database, embeddings removed). ADR-042's route-site
calibration failure is why Jev has no role in link matching and a gated role in anchor
inference. ADR-047 bounds every new cache, table and background job. The dispatch non-goal
holds: fathom computes and shows; it never schedules, assigns or retries, and task preflight
(U6) recommends a model tier for the harness to accept or ignore. The Moderne non-goal holds:
the migration tracker (U9) counts and proves, never transforms.

# 31 — The substrate: relational facts, a graph in memory, embeddings on probation

Status: ACCEPTED 2026-09-19 (ADR-041) with the owner's answers: embeddings are
removed rather than parked (D5 rejected for now; §5 stays the revisit bar); typed
edges proceed Go, CUE, TypeScript, Python, then the delegate's order, tracked in
`docs/languages.md`; freshness is hook-triggered per file. Plan:
`docs/plans/substrate.md`. Sources are dated inline; every claim about our own code carries a
`path:line` pointer to the inventory that produced it.

**Verdict.** Our base architecture is the right shape and the wrong build. The
shape — a relational store of facts, a typed call graph walked in memory,
governing claims compiled from documents and served at the edit, everything
cited and re-derivable — is the shape the field converged on after trying the
alternatives: no serious code index runs on a graph database (Joern left Neo4j
twice, Glean is "facts, not a graph", Sourcegraph keeps precise data in
Postgres), the vendors with real numbers de-emphasized embeddings as the
primary retrieval layer, and a 2026 controlled study of a symbols-plus-call-graph
index found exactly the lift our own bench keeps looking for: localization at
five candidates 84.5 % against 44.3 % for grep-only, 7.9 points more issues
resolved, at lower cost. The build, however, has ten provable limits that have
nothing to do with the choice of database and everything to do with how we use
it: every navigation call loads a whole service, every hook call loads every
claim, fourteen tables carry two indexes, every ranking path counts substrings,
freshness watches git HEAD only, and fleet queries are a loop in Go. Those
limits are where the measured cost problem lives. So: keep SQLite as the system
of record and do not add a graph database; fix the substrate's hygiene first,
because that is measurable and cheap; import typed edges for other languages
instead of writing them; put embeddings on probation for exactly one job
(natural-language concept search), as a compile-time enrichment with a kill
criterion, never in the lookup or localization paths; and adopt the temporal
and provenance mechanics the memory field learned without adopting its storage.

## 0. The question, reframed

"Should we use relational, RAG, or a graph database" is the wrong axis. Fathom
does five jobs with different physics, and each substrate choice must be judged
per job:

| Job | What decides quality | What the evidence favours |
|---|---|---|
| **Exact facts** — where is X, who calls it, what breaks | precision of edges, freshness, cost per answer | typed/structural index, lexical exactness; embeddings add nothing |
| **Concept search** — "where is retry handled" | ranking of natural language against code | hybrid: lexical + graph seeds, a small reranker or embedder for NL-shaped queries only |
| **Localization** — from a symptom to the site | multi-hop expansion from seeds, query reformulation | graph-guided traversal (LocAgent, RepoGraph, the 2026 index study); never one-shot ranking, lexical or dense |
| **Governance at the edit** — which rules bind this file | scoped match, drift, cost per hook call | claims with anchors and a sweep; served by prefilter, not full scans |
| **Findings and precedent** — what was found, what closed it, what recurs | identity, lifecycle, provenance, temporal validity | anchored claims with invalidation edges and bi-temporal validity; no graph storage needed |

Cross-cutting: freshness (uncommitted edits), cost per call (the bench's
consistent loser on every non-lookup task), portability (one file per store),
and multi-repo.

## 1. What the field says (external evidence, dated)

- **Structural index beats grep-only, measurably.** "Code Isn't Memory"
  (arXiv 2606.22417, 2026): symbols + call graph on, localization acc@5 84.5 %
  vs 44.3 % off, resolve 50.4 % vs 41.9 %, $2.30 vs $2.92 per solved issue,
  p < .003. This is our bet, measured by someone else.
- **Localization lift comes from graph-guided multi-hop traversal or query
  reformulation, never from swapping lexical for dense.** LocAgent (ACL 2025,
  arXiv 2503.09089) 94 % file-level acc@5; RepoGraph's honest agentic gain is
  +2–2.3 points (arXiv 2410.14684); reformulated BM25 on Long Code Arena
  +31 % relative MAP@1 (arXiv 2512.07022); our own LCA null at 2× tokens is
  consistent: a one-shot ranking pass is the wrong lever.
- **Embeddings help one job.** NL-to-code concept queries (RepoBench-R,
  CodeRAG-Bench, Agent Retrieval Bench arXiv 2607.24882: no family universally
  wins; Qwen3-Embedding-4B best MRR, RepoMap best yield per 8K tokens). Generic
  rerankers *degrade* results in the 200–800 token range (CodeRAG-Bench); the
  reranker that won was trained on localization pairs (SweRank, arXiv
  2505.07849).
- **The vendors with numbers moved away from embeddings as the primary layer.**
  Sourcegraph deprecated Cody's embeddings for keyword + graph (2024-02) and
  Amp ships deterministic SCIP graph tools with none; Anthropic dropped Claude
  Code's vector store for agentic grep (2025, self-reported); Cline refuses to
  index by design (2025-05); GitHub's one controlled test found semantic search
  worth 2 % time with no quality change, used only as a fallback (2026-03-17);
  Cursor's real number is about sync, not vectors (Merkle sync: median
  time-to-first-query 7.87 s to 525 ms, 2026-01-27).
- **Nobody runs a graph database as the index of record.** Joern went
  Neo4j → OverflowDB → flatgraph (2024) for memory and traversal speed; Glean
  is RocksDB facts + a Datalog-like query language; Kythe is LevelDB entries
  plus derived serving tables; Sourcegraph's precise data is in Postgres;
  CodeQL is a custom relational store. Kùzu, the embedded graph DB we would
  have reached for, was archived upstream in October 2025 after its acquisition.
- **Recursive CTEs are fine at our scale.** The published worst case (100k
  nodes, 500k power-law edges, a hub with 11,460 in-edges) is 305 ms for two
  hops; we have ~30k typed edges per service and already walk them in memory.
- **Typed edges beyond Go are importable, not writable.** Glean imports
  SCIP/LSIF for Go, Java, Rust, TypeScript rather than writing indexers; SCIP
  exporters exist for TypeScript, Python, Java, C/C++ and rust-analyzer; GitHub's
  stack-graphs resolve names per file with no build step (2021, still the
  precise-nav engine).
- **Memory systems learned three mechanics worth stealing and three to avoid.**
  Steal: bi-temporal validity (Zep/Graphiti, arXiv 2501.13956), explicit
  invalidation edges instead of status flips, entity identity for anchors
  (TOKI, arXiv 2606.06240). Avoid: LLM-extracted facts without anchors
  (HaluMem, arXiv 2511.03506), vector-only recall, silent overwrite of the
  losing fact.

## 2. What we actually have (inventory, 2026-09-18)

The store is fourteen tables in pure-Go SQLite (WAL, one writer, many readers;
ADR-020/025 removed DuckDB for the single-writer ritual, CGO and binary weight),
26 MB for this repo, "mostly derivable". The parts that work: content-addressed
claims with cites/scope-fp links and a $0 drift sweep; typed edges replaced at
package granularity; hash-gated ingest; a portable single-file store spec.

The ten limits the inventory can prove, each the actual cause of a measured
problem:

1. Every navigation tool loads the whole service's symbols and typed edges
   before answering (`internal/mcp/resolved_nav.go:91-92`); a cold graph build
   is ~9.7 s on ~10.6k symbols and the cache holds eight services
   (`internal/analysis/callgraph/cache.go:16-58`). Cost per call, fleet thrash.
2. `obligationsCore` loads every claim for the service on every edit-hook call
   and scores in Go (`internal/semantic/serve.go:477-483`). Serve cost scales
   with claim count, not match count.
3. Two non-primary-key indexes across fourteen tables
   (`internal/store/sqlite.go:283,299`); every scope, layer, status match is a
   per-service scan.
4. Every ranking path is hand-tuned substring counting — `keywordScore*10 +
   tierRank` (`internal/query/engine.go:292-313`), `localize` constants 5.0/3.0
   (`internal/mcp/localize_tool.go:393-395`); no BM25, no length normalisation.
   The app-kit journal's "unrelated intent-check results" (O23) is this.
5. The vector path is dead code: `hybridRank` runs only when a `SymbolIndexer`
   is wired, and no production caller wires it (`internal/query/engine.go:57-61`,
   `internal/modulehost/federated.go:281`).
6. Freshness watches git HEAD only; uncommitted edits are invisible until an
   explicit ingest or `watch` (`internal/mcp/autorefresh.go:73-79`). The
   journal's O18/O20/O32/O33 are this.
7. Fleet queries are a Go loop over independently opened store files
   (`internal/store/platform_store.go:123-142`); the only federation plan
   (DuckDB ATTACH, vision 03) targets a backend ADR-025 deleted.
8. The typed pass costs ~3.3× a syntax-only ingest even fully cached and
   cannot reach its own ≤2× target on `go/packages`
   (`internal/fusion/typedcalls_perf_test.go:14-21,105-109`).
9. Response budgeting is per handler, not middleware; O90's 63k-token answer
   happened before its guard existed (`.fathom/observations.md:1403-1409`).
10. Selector-method and interface edges are still missing in places (O39/O40),
    which is the named lever behind the review verb's 6-of-42 recall.

None of these is "SQLite's fault". All of them are cheaper to fix than to
migrate around.

## 3. Decisions proposed

**D1 — SQLite stays the system of record; no graph database.** Revisit only
when claim-to-claim edges exceed ~10M or a recursive rule query cannot be
expressed as a CTE plus an in-memory walk. DuckDB does not return (its vector
index persistence is still marked not for production, 2026).

**D2 — Substrate hygiene before any new engine.** Indexes on
`(service_id, layer, status)`, `(service_id, scope)`, typed edges by callee and
caller; claims prefiltered in SQL before Go scoring; an incremental graph cache
invalidated per package (the granularity typed edges already use) instead of
per watermark; a dispatch-level response budget so no handler can regress O90;
a working-tree overlay (re-extract dirty files on demand, the way `watch`
polls, so uncommitted edits resolve); fleet federation through SQLite `ATTACH`
(125-file cap, atomic across files) replacing the Go loop; per-symbol change
detection so an edit re-ingests one file's symbols, not a package. Each is
measured by the free tier of the bench pyramid (ADR-032): latency and tokens
per call on the L0 oracle, before and after.

**D3 — Typed edges beyond Go by import, not by writing.** Accept SCIP indexes
from language-native exporters (scip-typescript, scip-python, scip-java,
rust-analyzer) into `typed_edges` with the same package-granular replacement,
and a stack-graphs-style per-file resolver for JS/TS/Python where no exporter
runs. Measured by the L0 precision/recall oracle per language (the Kotlin
near-empty index in the LCA run is the reminder).

**D4 — Localization walks the graph.** `localize` already seeds from
literals and concern terms; make it the LocAgent shape: seeds → typed
neighbourhood expansion to depth 2 → rank by graph distance and evidence,
with query reformulation when seeds are empty; never a dense one-shot. Then
rerun the LCA corpus with the instrument fixed (turn cap, Kotlin index,
payload cap). If the paired interval still includes zero, localization leaves
positioning the way freshness did.

**D5 — Embeddings on probation, one job, compile-time, kill criterion.** An
optional `embeddings` compile layer (like every other model-dependent compile:
budgeted, recorded in `compile_runs`, never in a serving path's critical
section) that embeds one chunk per tree-sitter symbol with a local open model
(Qwen3-Embedding-0.6B is the one with head-to-head evidence on an
agentic-coding retrieval bench; ~61 MB float32 or ~15 MB int8 for 15k symbols)
into `modernc.org/sqlite/vec` (pure Go, pinned pre-1.0). Used only to rerank
lexical + graph candidates for natural-language `query_symbols` /
`pack_context` queries. Kill criterion: a paired bench (n ≥ 3, committed
ledger) on a labelled concept-query set must beat lexical-only significantly
at ≤ 2× tokens, or the layer is deleted and the effort goes to query
reformulation. Never for `find_symbol`, `find_callers`, `blast_radius`, or
`localize`.

**D6 — The claims model takes the memory field's mechanics, not its storage.**
Bi-temporal validity on claims and findings (`valid_from/valid_to` beside
`compiled_at/expired_at`, so `as_of` a commit answers "was this rule true
then"); explicit `supersedes`/`invalidated_by` links (the `ClaimLink` rel
vocabulary already reserves `supersedes`) instead of a terminal status; anchors
bound to the resolved `SymbolKey` beside the line span so a rename re-anchors
instead of drifting; PROV-style activity linkage (which run, model, commit
asserted the claim — most fields exist, the link does not); derived package
summaries only as a non-citable convenience layer, if ever.

**D7 — What we deliberately do not do.** A graph database; DuckDB's return;
embeddings as the primary or default retrieval layer; LLM-extracted facts
without anchors; generic rerankers bolted on; a code property graph until a
taint or dataflow question is actually asked; whole-program analysis at
serve time (gopls' 10 GB monorepo failure mode is the cautionary tale).

**D8 — Findings and review: the loop is ahead on integrity and behind on
learning telemetry.** Independent data on AI review is sobering: 36.4 % of
31,073 AI review comments across 239 repositories were accepted and 56.3 %
rejected, rejection driven by validity rather than volume (arXiv 2607.03316,
2026); the best production reviewer scores F1 51 on the one open multi-vendor
benchmark (Martian Code Review Bench, 2026-03); 93 % of findings in a 679-finding
study were flagged by exactly one tool, and higher severity tiers carried
higher false-positive rates. That is the argument for what we built: producer
tier caps, deterministic identity, conflicting-replay refusal, per-dimension
`checked:false` instead of an aggregate score, and nothing inferred from prose.
No surveyed product exposes an evidence tier on a check; the review kit's
ladder is ahead of the category. What the best of them do that we do not yet:
(1) promotion with decay, ownership weight and a cadence cap, and a provenance
tag on the promoted rule (Qodo's rule miner) — `findings candidates` counts
sites and code-change resolutions only; (2) a typed disposition with an impact
preview ("would filter N") before it becomes a scoped exception (Semgrep
memories) — our annotations stay inert quotes, which is right, but a typed
disposition can still be previewed; (3) per-rule effectiveness telemetry after
promotion — served, touched, disputed, last fired — so a lifted rule can retire
(CodeRabbit, Greptile); (4) a single-utterance "remember this" that becomes a
scoped standing fact without waiting for recurrence (Cursor Bugbot) — ours has
no path from one human sentence to a claim outside an import; (5) a
reproducible third-party benchmark of accept/reject behaviour — our
`findings-loop` corpus is that instrument and has not been run. What we refuse
to copy: admin self-approval of exemptions (Cortex), org-wide unaudited
prose-to-behaviour learning and negative-only suppression (CodeRabbit's default
scope, Bito's three-dismissal auto-suppression, Greptile's tone parsing), and
LLM-judged dedup or a single summed score as the headline. For the
organizational side, borrow sequential floors, expiring human attestations,
retained certification history and exemptions with an approval trail into
marks and soundings; keep the portable journal as the record, never a vendor
store.

## 4. Sequencing, each round gated by a measurement

| Round | Content | Gate (free tier unless noted) |
|---|---|---|
| A. Hygiene | D2 in full; D6's `supersedes` links and SymbolKey anchors | L0 latency/tokens per verb before/after; edit-hook p50 under 150 ms; O90 class impossible by construction |
| B. Edges | D3 SCIP import + per-file resolver; O39/O40 closed | L0 precision/recall per language; review recall re-measured on the 42 PRs |
| C. Localization | D4 | LCA rerun on the fixed instrument (paid tier) |
| D. Probation | D5 | paired concept-query bench, kill criterion stated before the run |
| E. Temporal claims | rest of D6 alongside the findings loop's second round | `as_of` conformance fixtures; bench corpus for stale-rule handling |

## 5. What would change my mind

- A graph database: our claim graph growing genuinely graph-shaped (long
  supersession chains queried transitively at scale) or a fleet whose adjacency
  no longer fits in memory.
- Embeddings for lookups: a controlled study showing dense retrieval beating
  exact identifier match on symbol lookup. None exists today.
- A code property graph: the first real taint or dataflow question a user asks.
- DuckDB: its vector and graph extensions reaching production status and a
  workload that is analytical rather than serving.

## 6. Open questions for the owner

1. Is a local embedding model an acceptable optional dependency for a compile
   step (it is not a serving dependency), given the local-first stance?
2. Which non-Go languages matter first for typed edges: TypeScript and Python
   (SCIP exporters are mature) or Kotlin/Java (the LCA corpus)?
3. Should the working-tree overlay be default-on in the daemon (more CPU,
   always fresh) or hook-triggered (re-extract the file being edited)?

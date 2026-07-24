# Expert modules — the use-case spec

*Status: adopted 2026-07-07. Owner: the fathom program. Companion to
`semantic-layers.md` §5 (the verdict: **fathom IS the expert-module core**;
commitments D-EM1–D-EM5) and `product-vision.md` §3 (the north-star row and its
entry conditions). This document specifies the expert-module use cases
concretely: what a module is in code, the launch use cases with their query
patterns and excellence bars, the lifecycle, and what v1 implements now versus
what waits for named entry conditions.*

*Engine-assumption note (added post-SS-8, 2026-07-23, ADR-025): the module
store is SQLite (`.sqlite`), not DuckDB — DuckDB was deleted from this
codebase outright once the fleet finished migrating (ADR-020). The
entry-conditioned "DuckDB VSS" vector mention below (§ Entry-conditioned)
predates the removal and needs a technical re-check against SQLite before
it fires, not a silent port of the DuckDB-specific mechanism.*

## 1. What an expert module is (in code, not prose)

An **expert module** is a fathom store compiled over a corpus:

```
module = one .sqlite store containing
           ├─ deterministic index         (observations: sources, symbols, refs, facts)
           ├─ semantic layers             (semantic_claims + claim_links: governed,
           │                               cited, tiered, staleness-tracked claims)
           ├─ its own audit trail         (compile_runs ledger, claim_events)
           └─ its own identity            (service_meta: corpus root, watermark,
                                           layer versions — self-describing)
```

Portable by construction (ADR-017: repo-relative anchors + root remap), served
by the same engine (daemon for shared/live, embedded for standalone), queried
through the same path-scoped MCP surfaces. **The store file is the product**;
the binary is its reference reader. There is no second engine, no second
schema, no "expert edition" fork — that is commitment D-EM2, and it is what
makes federation (§6) an ATTACH rather than an integration project.

What distinguishes a module from "a chatbot with a folder of PDFs" (D-EM4):
every answer carries **why it should be trusted** (evidence tier, citation to
the corpus line), **when it stopped being sure** (per-claim status: fresh /
stale / contradicted / refuted), and **what it was compiled from** (corpus
watermark, compile ledger, eval scores).

## 2. The corpus model

A corpus is anything a Connector can walk (D-EM1: corpus-agnostic core —
`SourceRef`/`SourceKind` are open):

- **Code repo** (LocalConnector, git watermark) — the degenerate and founding
  case: the fleet index *is* an expert module about the fleet.
- **Document folder** (LocalConnector over a non-git tree, or a docs/ subtree):
  standards, runbooks, review findings, ADRs, postmortems. Watermark = corpus
  manifest hash (Watermark SPI, ADR-017) — freshness must not assume git.
- **Mixed** — the common real shape: a repo *plus* its governing docs, where
  claims cite docs and scope code.

The corpus stays outside the store; the store carries extracted observations,
compiled claims with line-anchored citations, and the watermark needed to
detect corpus drift. Recompile rights (corpus selectors + prompt versions in
the store) mean any holder of corpus + store can refresh honestly.

## 3. Launch use cases

Ordered by proximity to the dogfood loop (each has a real corpus we own today).
Every module follows the same pattern — **corpus → layers → decision-moment
queries → honesty mechanics** — which is the point: use case = configuration,
not code.

### 3.1 Engineering-standards module ("how far behind our standards is X?")

- **Corpus:** the org's standards/conventions docs (for the fleet: CLAUDE.md
  files, ADRs, design docs — ~15 slow-churn governing docs).
- **Layers:** `obligations` (the ADR-016 tracer bullet — this module and the
  coding companion's M5 injection are the *same compiled layer*, consumed at
  different moments).
- **Queries:** "what governs `internal/query/engine.go`?" (obligations by
  scope); "which rules went stale since their last compile?" (claim status
  scan); "where does code drift from standard S?" (obligations ∩
  `arch_conformance` — a join, not a new engine).
- **Excellence bar:** every rule served carries a resolvable citation; a
  standards-doc edit flips affected claims to `stale` within one ingest cycle;
  the four fleet violation classes trend to zero (shared bar with the
  companion, §7 of coding-agent-excellence.md).

### 3.2 Review-findings module (well-architected / code-review findings)

- **Corpus:** a findings folder — one file per finding (severity, area,
  affected paths, recommendation), the natural export of any review
  engagement. Non-git or git; watermark handles both.
- **Layers:** findings-as-claims: kind `finding`, tier honest to origin
  (`sme-stated` for human review conclusions — rank below code-verified,
  which the shipped tier ranking already enforces), `cites` → the finding doc,
  `scopes` → the affected paths.
- **Queries:** "open findings scoped to the file I'm editing" (same shape as
  obligations — injectable at M5); "findings whose scoped code has since
  changed" (finding ∩ data-staleness: *did remediation happen?* — the drift
  query no review-deliverable PDF can answer); "findings contradicted by
  current code structure" (claim status after sweep).
- **Excellence bar:** a finding's remediation status is answerable per-file
  with citations, on a store compiled before the remediation started.

### 3.3 Incident/runbook module (failure-mode playbook)

- **Corpus:** dogfood-observations.md, postmortems, runbooks — symptom →
  cause → check triples.
- **Layers:** `failure-mode` playbook (semantic-layers roadmap #2).
  **Entry-conditioned** (product-vision): the corpus must reach ≥15–20
  resolved findings before compiling on it can pass the precision gate —
  today it has ~5. The module *shape* ships now (a findings module is 3.2 with
  a different kind); the playbook layer waits for its corpus.
- **Queries at incident time:** "symptom S — known causes, ranked checks,
  precedent incidents with citations"; write-side: "is this new observation a
  duplicate of precedent?"
- **Excellence bar:** ≥70% of a 20-incident replay surfaces the true
  root-cause class in the top-3 checks.

### 3.4 Codebase-expert module (the founding case)

The fleet index already is one: deterministic symbols/edges/routes + (now)
obligations. Standalone value today; it is also the eval environment for every
mechanism above — nothing ships to an external module that didn't run here
first.

### 3.5 Federated multi-expert systems (H3, entry-conditioned)

"Chat with the platform's experts": N module stores attached read-only behind
one router; cross-module queries are the existing cross-service machinery
(`pkg/platform`) over attached stores. Cross-module *preprocessing* (layers
whose corpus selectors span modules) is admitted by the manifest shape when it
arrives. Entry conditions (unchanged from product-vision): ≥2 real external
module consumers, or ≥100 external installs + ≥60% internal fathom-share.
Until then, federation work is capped at keeping D-EM5 true (nothing in the
engine assumes single-corpus).

## 4. Conflict and drift mechanics (the differentiating queries)

The owner's defining requirements — "understand where the code conflicts with
the documentation" and "how far behind standards" — decompose onto shipped or
in-flight machinery; **none of them require new engines**:

| Question | Mechanism |
|---|---|
| Doc says X, doc changed since compile | citation sweep: cited lines re-hashed post-ingest → claim `stale`/`contradicted` |
| Doc says X about code, code moved away | scope re-verification: claim scopes matching nothing / drifted anchors flagged at query time |
| Code contradicts code | existing `contradiction_scan` over observations |
| Code lags standard S | obligations ∩ `arch_conformance`/`dep_drift` joins (per-file, per-rule, citation-bearing) |
| Finding F remediated? | finding scope ∩ per-file content-hash drift since compile watermark |
| Expert disagrees with expert (federated) | same claim/citation model per module; disagreement is a *ranked, tiered* result set, not a merge |

## 5. Lifecycle (one flow for every module)

1. **Create:** `fathom ingest --service <module> --path <corpus>` — any corpus,
   same command. Store = the module file (default under `workspace/db/`).
2. **Compile:** `fathom compile --layer <layer> --service <module>` —
   dry-run/estimate first (mandatory), digest-gated per section, real usage
   recorded in `compile_runs`.
3. **Verify:** compile-time anchor + scope verification (reject claims whose
   citations don't resolve); layer eval fixtures gate shipping (precision
   ≥0.90 — ADR-016 §4.6).
4. **Refresh:** watermark drift (git HEAD or manifest hash) → re-ingest →
   citation sweep → recompile only what drifted (cost scales with change size,
   never corpus size).
5. **Ship:** copy the store file. The receiving side gets intact citations
   + honest staleness with zero recompile (the ADR-017 round-trip test is the
   contract). SQLite (ADR-020) caveat for LIVE stores: a WAL-mode store that
   is open in a writer holds recent commits in a `-wal` sidecar — copy the
   `.sqlite` file only after the writer closes (which checkpoints), or copy
   all three files (`.sqlite`, `-wal`, `-shm`) together; a bare file copy of
   an in-use store silently drops un-checkpointed writes.
6. **Serve:** daemon (`serve-http`, path-scoped surfaces per audience) or
   embedded/CLI (`fathom query`). One module = one store = one writer (D-EM5).
7. **Federate (H3):** read-only ATTACH of N module stores behind the platform
   router.

## 6. Serving surface

Modules reuse the path-scoped surface mechanism (ADR-014): an **expert
surface** — the module-relevant read tools (`query_symbols`, `find_symbol`,
`references`, `outline`, `pack_context`, `staleness_report`, `list_services`,
`batch`, plus claim serving) at `POST /rpc/expert` — so a knowledge-consumer
audience never sees 86 code tools. The claim-serving tool is shared with the
companion (obligations by scope); kinds beyond `obligation` reuse the same
query shape. Chat-style consumption is an agent + this surface; fathom stays
the substrate, not the chatbot.

## 7. v1 scope (this session) vs entry-conditioned

**Now (proves D-EM1/D-EM2 in code, not prose):**

1. ADR-017 portability: repo-relative anchors, root remap,
   `TestStorePortability_RoundTrip` (build on path A, answer with citations on
   path B), Watermark SPI (git HEAD / manifest hash), `tier_profile` column.
2. Claims schema + obligations compile + citation sweep (shared with the
   companion tracer bullet — one implementation, two use cases).
3. **A real non-git module end-to-end:** a findings-style corpus ingested as
   its own service, claims compiled over it, queried standalone, passing the
   portability round-trip. This is the demo that a module is configuration.

**Entry-conditioned (named evidence, from product-vision):** failure-mode
playbook layer (corpus ≥15–20); published store-format spec v0 (H2, after the
round-trip holds); `fathom pack` as a thin serializer (H2); federation ATTACH
(H3); vectors as an in-store DuckDB VSS column (telemetry evidence); external
/ regulated-domain modules (never with fathom-built connectors — the standing
non-goal).

## 8. What would falsify this design

Kept here deliberately, per program discipline: (a) if the portability
round-trip cannot hold without breaking existing queries, D-EM2 needs a
migration story before any external module ships; (b) if the obligations
tracer bullet fails its precision gate on our *own* corpus, no external
module is credible and the layer program halts at deterministic claims;
(c) if serving claims from a second, non-code module demands engine special
cases, D-EM1 was wrong and must be redesigned *before* H2 publication — the
non-git module in v1 exists to force that answer early.

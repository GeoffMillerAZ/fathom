# Semantic layers — fathom's founding design

*Status: adopted 2026-07-07 (ADR-016). Owner-designed via a 4-reader / 4-architect /
3-judge design workflow; every load-bearing claim below was verified against this
codebase, agentic-kit, or external engineering writeups during that review.*

*Vocabulary note (added post-ADR-024, 2026-07-23): this design predates the
four-stratum pack taxonomy — see
[docs/design/pack-taxonomy.md](pack-taxonomy.md) for the authorship/trust
vocabulary ("dialect" / "provider" / "platform" / "concern") layered on top
of the machinery below. Every layer this document describes and ships
(`obligations`, `findings`, `adr-rules`, `spec-bindings`) is **concern**
stratum; `kind` (below, §4.1 — `obligation`, `finding`, `adr-rule`,
`spec-binding`) is an orthogonal axis (what a layer's claims are *about*),
never a synonym for stratum (who authored and eval-gated the pack).*

*Engine-assumption note (added post-SS-8, 2026-07-23, ADR-025): DuckDB was
deleted from this codebase outright once the fleet finished migrating to
pure-Go SQLite (ADR-020) — the store is SQLite-only now. This document's
"DuckDB VSS" / "in DuckDB (single file, embedded OLAP)" mentions below
describe an entry-conditioned future capability that hasn't fired and
predate the removal; they need a technical re-check against SQLite before
anyone executes them, not a silent port of the DuckDB-specific mechanism.*

## 1. The framing

Fathom's deterministic index answers **what the code is** — symbols, call edges,
routes, specs mentioned, complexity — cheaply, incrementally, and provably. A whole
class of knowledge is invisible to it because producing that knowledge requires
*reasoning*, not scraping: what a file is **obliged** to do (specs, standards,
decisions), **why** it is shaped this way (rationale, invariants), and **what
happened here before** (review findings, incident precedent).

A **semantic layer** is that knowledge, compiled ahead of time by LLM passes into
**governed claims** stored beside the deterministic index — with provenance,
staleness, citations, and evidence tiers — and served at **decision moments** in
token-economical payloads.

Three sentences that discipline everything else:

1. **Fathom compiles meaning.** A layer is a build target over a corpus, not a RAG
   pipeline: incremental (hash-gated per input unit), budgeted (estimate before,
   real usage after), versioned (prompt + model + inputs = reproducible identity).
2. **Reasoning at compile time, determinism at query time.** LLMs run when a layer
   compiles; a query is a deterministic join. Same store, same staleness honesty,
   same single-writer daemon.
3. **A claim is only as good as its anchors.** Every claim cites the source lines it
   restates and the code scope it governs; broken citations flag the claim, never
   silently serve it.

## 2. Is this a good idea? (the honest verdict)

Yes — with one important correction of emphasis. The external record (prior-art
survey, engineering postmortems) is unambiguous about how systems in this space die:

- **Embeddings-first context engines get retracted** (Sourcegraph publicly retreated
  from embeddings toward deterministic code context). The deterministic spine must
  stay primary; LLM output is enrichment on top. Fathom already has this right.
- **Freshness kills LLM-derived knowledge** (DeepWiki serves days-stale wikis with no
  indicator; specs drift silently). Nobody in the field attaches per-artifact
  staleness to compiled knowledge. Fathom's staleness machinery is the wedge.
- **Coverage-first ontologies die before delivering** (BI semantic layers demanding
  upfront modeling; GraphRAG's $33k index made cost the first buyer question). Value
  must arrive per-query from week one, and compilation must be costed and incremental.
- **Knowledge without a consumer stalls** (llms.txt: ~784 sites, no agent consumes
  it; AGENTS.md: 60k+ projects because agents demonstrably obey it). A layer ships
  WITH the tool that consumes it and the telemetry that proves consumption.
- **Even citation-bearing systems mis-cite** (~15–20% citation hallucination in
  frontier models on citation tasks). Compile-time anchor verification — reject any
  claim whose cited lines don't exist — is mandatory, not optional.

The correction of emphasis: the differentiator is **not** "we have semantic layers"
(everyone will have AI-preprocessed knowledge). It is **governance**: provenance
tiers on every answer, machine-checked contradiction against the deterministic
index, honest per-claim staleness, costed compilation, and telemetry-gated
existence. No surveyed system (DeepWiki, Unblocked, glean.com, GraphRAG) exposes
*why a claim should be trusted*. That is the moat, and fathom already owns the
primitives (evidence tiers, hash-gating, staleness reports, contradiction scan,
call-log telemetry).

## 3. The better questions

The design was steered by replacing "how do we add semantic layers?" with:

1. **What knowledge changes an agent's action at a decision moment?** Not "what can
   we index" — what, delivered at the moment of an edit/review/incident, makes the
   agent do something different? (Ranked inventory in §5.)
2. **What is the unit of semantic knowledge?** A *governed claim with anchors* — not
   an embedding, not a wiki page. Claims have provenance, tier, citations, scope,
   and a reproducible identity.
3. **How does compiled knowledge stay true under churn?** Two mechanisms, both
   required: a post-ingest sweep that re-verifies citations/hashes, and a query-time
   check so a drifted claim is *flagged in the payload*, never silently served.
4. **What does it cost, and what does a re-run cost?** Compile-time LLM spend must be
   estimable before, recorded after, and scale with *change size* not repo size.
5. **How does a layer earn the right to exist?** Eval fixtures before shipping
   (precision-gated), call-log telemetry after (generalize-or-kill), and explicit
   revival evidence for everything deferred.
6. **What happens when the LLM confidently links the wrong spec to a hot file?**
   (The truth-lens attack scenario.) Anchors point at the *doc*; the file is only a
   scope target — so code churn never invalidates the claim. Detection needs: scope
   verification at compile, precision-gated evals with deliberate scope traps, a
   per-claim human refute channel, and bounded blast radius (≤150-token payloads,
   citations always attached).

## 4. Architecture (the judged hybrid)

Four independent designs (governed-claims-first, knowledge-compiler, ontology-graph,
tracer-bullet) were scored by three judges (agent value, truth & governance,
feasibility & economics). All three converged on the same hybrid: **the tracer
bullet's scope and economics, on the compiler's claim-identity skeleton, with the
governance transplants that close each design's hole.** The grand substrates
(generic layer runtime, claim-lifecycle FSM ledger, knowledge graph ontology,
persistent vectors) are *deferred with named revival evidence*, not rejected.

### 4.1 Data model (additive, beside — never inside — observations)

Three tables via the existing idempotent-DDL path (`internal/store`), mirrored in
the memory fake:

- **`semantic_claims`** — one row per claim.
  `claim_id` = sha256(layer, kind, subject, repro_key) — stable, content-addressed;
  `layer`, `layer_version`, `kind` (v1: `obligation`), `subject` (open ref string),
  `rule` (the claim text), `scope` (glob(s) it governs), `tier` (v1: `doc-claimed`),
  `status` (`fresh` | `stale` | `contradicted` | `superseded` | `refuted`),
  provenance (flattened agentic-kit `aiprovenance.AIAuthor`: model, prompt version,
  params + `repro_key`), `input_digest` (per-source-section), `compiled_at_commit`.
  Superseded rows are kept (status flip), never deleted — audit history is cheap now
  and unrecoverable later.
- **`claim_links`** — typed edges with an *open* `rel` vocabulary; v1 materializes
  only `cites` (doc file + line range + `target_hash` of the cited lines) and
  `scopes` (the code globs). `supports`/`contradicts`/`supersedes` are reserved and
  added edge-kind by edge-kind as layers earn them.
- **`compile_runs`** — per-run ledger: layer, corpus fingerprint, model, **real**
  token usage + USD from llmclient responses (not heuristics), duration, claim
  counts. `estimate_cost`-style dry-run is mandatory before any spending compile.
- **`claim_events`** (minimal, append-only) — human/machine verdicts on claims
  (`refuted`, `corroborated`, `contradicted`, with author + note). This is the
  per-claim correction channel the truth lens demanded; the full hash-chained
  lifecycle FSM in agentic-kit is a non-destructive retrofit later because claim
  identity is stable. `refuted` and `contradicted` are deliberately DIFFERENT
  verdict strings for different judgments — see §4.3's callout — never merged
  into one, even though both pair a `claim_events` row with a `semantic_claims`
  status flip.

Claims deliberately do NOT live in `observations`: re-ingest purges observations
per (service, source_ref) destructively, and the naive contradiction scan would
false-positive summaries against signatures. Same store, separate tables, same
query engine on top.

### 4.2 Why claims are `doc-claimed` tier (for now)

V1 obligations are **citation-grounded restatements of governing docs** — the rule
text is the doc's content, verifiably anchored, which is exactly what `doc-claimed`
(rank 3, ceiling 2) means; the shipped ranking machinery already caps it below
code-verified facts. The LLM-inferred part is the *scope mapping*, which is why
scope gets compile-time verification, eval scope-traps, and the refute channel. A
distinct `ai-derived` tier (and the cross-kit taxonomy ADR it drags in) is deferred
until a layer emits claims that are NOT citation-grounded — that is its named
revival evidence.

### 4.3 Freshness (both mechanisms, because they cover different windows)

- **Post-ingest sweep** (token-free, in the daemon): after every ingest/auto-refresh,
  re-hash each claim's cited lines; a mismatch flips status → `stale`
  (`contradicted` if the citation no longer resolves at all). Statuses persist. A
  transition into `contradicted` also appends a `claim_events` row with
  `verdict:"contradicted", author:"sweep"` — the same append-only audit
  mechanism a human refutation uses, so the anchor-dead judgment leaves a trail
  too (D2-2, `docs/plans/expert-depth.md`).
- **Query-time check**: every payload carries the claim's status and
  `compiled_at_commit` watermark; stale claims are shown *flagged*, never silently
  served, and never silently dropped (an agent that saw a rule yesterday should see
  "that rule is stale" today, not absence).

**`contradicted` vs `refuted` — two words for two different judgments, never
merged (P1-9, `docs/plans/pulled-tools-excellence.md`).** `contradicted` is
this section's own machine-detected, anchor-dead state: a claim whose citation
no longer resolves at all. It is `ClaimStatus.Served()`-true — the claim stays
**served, flagged with its status**, because the underlying rule may still
hold even though its evidence anchor broke; the next recompile is what heals
or reconfirms it. `refuted` is a categorically different, *deliberate*
judgment — a human via `fathom claim refute`, or the obligations tool's
feedback action (§4.5) acting on a human's behalf — that sets status
`ClaimRefuted`, which `Served()` excludes: a refuted claim is **never served
again**. Both verdicts append a `claim_events` row through the same
mechanism, which is exactly why the verdict *string* the row carries must
never be shared between them: a `claim_events` row reading `verdict:"refuted"`
for a claim that is, in fact, still being served (because sweep only ever
produces the contradicted state) would make the audit log lie about whether
the claim is live.

Because v1 claims anchor to slow-churn governing docs and join to code by scope
glob at query time, **code commits cost $0 in recompilation** — the economics that
made the tracer bullet win the feasibility lens.

### 4.4 Compilation (v1: one pass, manifest-shaped but not manifest-general)

Per governing doc section: split → DLP gate (`llmsec.Redactor`) → LLM with
strict-JSON output → **compile-time verification**: reject any claim whose cited
lines fail to re-resolve or whose scope globs match nothing in the live index →
transactional store. Incremental by `input_digest` per section; deterministic tests
via the llmclient replay backend. The layer's definition (corpus selectors, prompt
version, budgets, output kind) is expressed as a CUE schema
(`schema/semantic/schema.cue` + valid/invalid testdata per repo convention) so the
*shape* of a manifest exists from day one — but the generic manifest *runtime* (layer
DAGs, arbitrary passes) is only extracted when layer 2 is built. Generalize from two
instances, not one.

### 4.5 Serving (decision-time delivery)

- **`obligations`** `{service, file, symbol?, budget_tokens?}` — companion-surface
  tool. Payload ≤150 tokens, 3–6 rules max, ordered by (scope specificity, rule
  text), each with `src: "CLAUDE.md:62"`-style citation and tier;
  **zero-output-on-clean**; `stale` array flags drifted rules; watermark:
  `compiled_at_commit`. The about-to-edit flow is `outline` + `obligations` in one
  `batch` call.
  *Amended by the product-vision panel (see `product-vision.md` §4 H1.4): the
  primary delivery channel is a PostToolUse(Edit|Write) hook that **injects** this
  payload — configuration-forced, not agent-chosen; the tool is the secondary pull
  path, and the §4.6 adoption gate measures **injection coverage**, not call-rate.*
- **`compile_layer`** `{layer, dry_run?}` — full-surface only (it spends money;
  estimate-first is enforced). CLI mirror: `fathom compile --layer obligations`.
- **`claim_feedback`** `{claim_id, verdict, note}` — the refute/corroborate
  write-path (appends to `claim_events`; refuted claims stop serving). Human confirm
  **never promotes tier** — corroboration is status, not evidence laundering.
- `staleness_report` gains a `layers` section (per-layer claim counts by status,
  last compile, cost).

### 4.6 Eval + telemetry gates (a layer earns existence)

- **Before shipping**: labeled fixtures in `testdata/fauxtown` (seeded governing
  docs with known scopes, including deliberate scope-traps and one behavioral rule
  that globs express poorly). Gate: **precision ≥ 0.90** on scoped claims (the
  every-edit exposure makes false positives trust-fatal — the Moderne lesson),
  recall ≥ 0.75, plus byte-golden serving tests against a replay backend.
- **After shipping (3-week generalize-or-kill gate)**: from `mcp-calls.jsonl` — the
  fraction of edits to governed files preceded by an obligations call, AND an
  outcome metric: the four default-violation classes this repo actually commits
  (`time.Now()` in query logic, unsorted outputs, org-specific names,
  non-`workspace/` output paths) trending to zero in review. Pass → build layer 2
  and extract the generic runtime. Fail on call-rate but not quality → fold
  obligations into `outline`/`symbol_context` as a capped rider (the zero-new-habit
  delivery). Fail on quality → kill, and the substrate cost was two tables.

### 4.7 Deferred, with revival evidence (not rejected)

| Deferred machinery | Revived by |
|---|---|
| Generic CUE layer runtime + layer DAGs | Gate pass + building layer 2 |
| `ai-derived` evidence tier (+ cross-kit taxonomy ADR) | First non-citation-grounded claim kind |
| Claim-lifecycle FSM, hash-chained ledger, CODEOWNERS review routing | Claim volume exceeding the dogfood O-class loop |
| Persisted vectors / claim embeddings (DuckDB VSS) | Telemetry showing intent-search misses that joins can't answer |
| Machine-checkable claim predicates auto-refuted vs call graph | First layer emitting behavioral claims (invariants) |
| Deterministic anchor pass (persist sdd/adr/commit links into `claim_links` at $0 LLM cost) | First post-gate addition — pure value, no spend |
| Knowledge-graph ref schemes, closed ontology | Never as a prerequisite; grown edge-kind by edge-kind |

### 4.8 The layer roadmap (ranked by frequency × behavior delta, from real usage)

1. **Obligations** (every-edit; the v1 tracer bullet) — specs/standards/decisions
   governing the file being edited.
2. **Failure-mode playbook** (per-incident; the largest recorded behavior deltas in
   the dogfood record) — symptoms → known causes → checks, distilled from
   dogfood-observations + journal. Layer 2 candidate; also the write-side assist
   (dedupe/classify a new observation against precedent).
3. **Invariant / intent layer** (per-review) — ADR + resolved-incident invariants
   checked at diff time; needs machine-checkable predicates (revives that machinery).
4. **Brief curation** (per-task; the docs call the pre-packed brief "the biggest
   tokenomics lever") — intent-conditioned selection of load-bearing signatures into
   `brief`, embedding only fresh claims verbatim.
5. **Extension recipes, semantic spec-satisfaction, naming precedent** — later;
   each ships only with its consumer tool + eval + gate.

## 5. Expert modules: the north star this design already serves

The owner's second vision: standalone, high-performance, queryable **expert
modules** — deep domain knowledge (concepts + code + docs + standards + review/
incident findings), usable alone or federated into multi-expert systems, aware of
code↔doc conflict and standards drift, useful in real-time incident work.

**Verdict: fathom IS the expert-module core.** Not "two products sharing
libraries" — an expert module *is* a fathom store with semantic layers compiled
over a corpus, packaged. Every requirement maps onto machinery this design already
specifies:

| Expert-module requirement | Fathom mechanism |
|---|---|
| Deep knowledge, fast queries | Compiled claims + deterministic index in DuckDB (single file, embedded OLAP) |
| "Where does code conflict with docs?" | Contradiction status + citation sweep + (later) predicate auto-refutation |
| "How far behind org standards?" | Standards corpus as a layer; `arch_conformance` + obligations drift = a scoring query |
| Well-architected / review findings ingested | A findings connector → claims at honest tiers with `cites`/`scopes` links — a *layer*, not a new engine |
| Real-time incident support | Failure-mode playbook layer (roadmap #2) + the existing incident surface |
| Standalone module | The store file is self-describing: layer versions, prompt versions, eval scores, compile ledger, corpus fingerprints all live IN the store |
| Chat with an expert | Any agent + the module's MCP surface (path-scoped surfaces already let one daemon expose per-audience toolsets) |
| Multi-expert federation | The platform layer (`pkg/platform`, cross-repo graph) generalized: a registry of module stores behind one router; cross-module queries are the existing cross-service machinery |
| Cross-module preprocessing | A layer whose corpus selectors span modules — the manifest shape admits it when federation arrives |

**Design commitments locked NOW so expert modules never require a restart:**

- **D-EM1 — corpus-agnostic core.** `Observation`/`SourceKind` are already open;
  claims use open ref strings (`subject`, `target_ref`) — never code-only types. A
  runbook, a finding, a standard is just another source kind + layer.
- **D-EM2 — the store is the module.** Everything needed to interpret, trust, and
  recompile a store travels inside it (claims, links, compile ledger, eval results,
  watermarks). A fathom `.duckdb` file + the engine = a portable expert module.
- **D-EM3 — vectors are an additive column, never a second store.** When embeddings
  earn their revival evidence, they land as DuckDB VSS over claim/summary text in
  the same file — module portability is non-negotiable.
- **D-EM4 — determinism + tiers are the product.** An expert module that can say
  *why* it believes something (tier, citation, freshness) and *when it stopped being
  sure* (status) is what distinguishes it from a chatbot with a folder of PDFs.
- **D-EM5 — federation lives above the engine.** One module = one store = one
  single-writer daemon (or embedded engine). Multi-module systems compose at the
  MCP/platform layer; the engine never grows multi-tenant complexity.

## 6. Cost model

V1 economics (verified against corpus sizes): the obligations compile reads ~15
slow-churn governing docs — **single-digit cents per full compile of this repo,
< $1 fleet-wide**, $0 marginal on code commits, pennies on doc edits (per-section
digest gating). Every compile records real usage; every spending path requires a
dry-run estimate first. This is the inverse of the GraphRAG cost shape, on purpose.
Costs must scale with change size, never repo size — that constraint is structural
(query-time scope joins), not aspirational.

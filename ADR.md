# Architecture Decision Records — fathom

Fathom is a code-intelligence + evidence engine: it ingests source across one or
many repos, extracts symbols/edges/facts, stores them as **evidence
observations**, and answers concern-scoped queries (for AI coding assistants,
cross-repo blast-radius, investigations, fleet health). It exposes a Go client,
a CLI, and an MCP server.

This file records the decisions that shape the implementation, each with the
"why" rooted in a design lesson or constraint. Status: `accepted` unless noted.

---

## ADR-001 — Three-layer architecture

**Decision.** Strictly separate three layers:

1. **Connector SPI** — abstract source (`Discover`/`Fetch`); produces `SourceRef`s.
2. **Source Fusion Pipeline** — `SourceRef → bytes → extractors → Observation` records in the store; hash-gating, parallel extraction, lazy mode.
3. **Knowledge Query Layer** — `store → typed results` per query/tool; concern-scoped filtering, staleness assessment, optional hybrid semantic search.

**Why.** Each layer is independently testable and replaceable. The query layer
never touches a network; the pipeline never knows where bytes came from.

## ADR-002 — Connector SPI with a deterministic Faux connector

**Decision.** `Connector{ Discover(ctx, ServiceRef) ([]SourceRef, error); Fetch(ctx, SourceRef) ([]byte, error) }` with three impls: `LocalConnector` (disk, no network), `GitHubConnector` (gh CLI), `FauxConnector` (wraps `testdata/fauxtown/`).

**Why.** Tying ingest to a live VCS API created dev friction and non-deterministic
tests. With Faux, the entire fusion + query stack is tested against fixed fixture
data — no network, no credentials, golden-file stable. **Faux is the backbone of
the TDD strategy.**

## ADR-003 — Storage: DuckDB (supersedes the original SQLite choice)

**Status.** Superseded by ADR-020 (SQLite default, 2026-07-22) and ADR-025 (DuckDB
deleted outright, 2026-07-23). Kept as written.

**Decision.** Persist observations and derived tables in **DuckDB** (`github.com/marcboeker/go-duckdb/v2`).

**Context / change.** The original design used `modernc.org/sqlite` (pure-Go,
CGO-free) specifically because the project had a no-CGO constraint. **That
constraint has been lifted.** DuckDB is a columnar analytical engine that fits
fathom's workload far better: large `observations` scans, group-bys for
hotspots/metrics/fleet rollups, and (future) Parquet export for the analytical
substrate. CGO build cost is accepted.

**Consequences.** Tests and CI run with `CGO_ENABLED=1`. The store package hides
DuckDB behind an interface (`Store`) so the engine is swappable and unit-testable
with an in-memory fake. Schema tables: `observations` (main evidence),
`knowledge_cards`, `cross_ref_index`, `doc_summaries`, `runbooks`; fingerprints
in a separate store (cheap to update / clear independently).

## ADR-004 — CGO-free tree-sitter via wazero/WASM; Go via native `go/parser`

**Status.** Superseded in mechanism by ADR-013 (CGO bindings); revived in part by
ADR-021 §4 (the wazero engine for Python/JS/TS/Ruby). Kept as written.

**Decision.** Multi-language extraction runs tree-sitter grammars compiled to
**WASM, executed in the pure-Go `wazero` runtime** (no CGO, no Docker, golden
testable). **Go** uses the stdlib `go/parser`/`go/ast` directly (fastest, most
complete). HCL, k8s YAML, and CUE use their native Go parsers.

**Why.** Even with CGO available for DuckDB, the WASM tree-sitter path keeps the
extractor layer deterministic and free of native grammar build chains. Go AST is
strictly better than tree-sitter for Go, so we don't pay WASM cost there.

## ADR-005 — Hash-gated, deterministic 3-pass fusion pipeline

**Decision.** Pipeline runs: **Pass 1 Filter** (sequential, ignore vendor/
testdata/generated), **Pass 2 Fetch** (N parallel workers into pre-allocated
slots, first error via atomic CAS, no mid-flight cancel), **Pass 3 post-fetch**
(sequential, in input order: SHA-256 fingerprint → hash-gate → purge stale →
budget gate → pending). Then bucket pending by language and extract.

**Why.** Re-ingesting a 15k-file repo where 14.9k files are unchanged must be
near-free (hash compares only). Determinism (same repo → same result) requires
the post-fetch pass to re-impose input order even though fetch was parallel.
Budget gating prevents runaway cost on unexpectedly large repos.

## ADR-006 — Mandatory concern-scoped queries

**Decision.** Every query carries a `concern_id`; the engine filters observations
to that concern, then scores by keyword/semantic overlap and truncates to TopN.

**Why.** Dumping all symbols for a service into an LLM context was expensive and
noisy (~measured 77% token reduction by scoping). Forcing a concern makes callers
specific and keeps results relevant and cheap.

**Amendment (2026-08-26, ADR-026 review).** Re-scoped to what the code enforces:
the concern is a required parameter of `Engine.Query` (`query_symbols`, `fathom
query`, `pkg/client.Query`) and becomes a keyword when the store is unscoped; tools
that never pass through the engine — 55 of 58 registered at the time, the whole
index-native navigation surface — are exempt. "Mandatory concern-scoped queries"
names the engine's contract, not a property of every tool.

**Revisit when.** A tool outside the engine returns result sets the call log shows
scoping would have cut by more than half; or a second concern-shaped filter is
proposed for the agent surface.

## ADR-007 — Evidence tiers reused from agentic-kit

**Decision.** Observations carry an **evidence tier** from agentic-kit's
`foundations/review/scoring` foundation: `verified` (code, ceiling 5),
`pre-scan` (4), `doc-claimed` (2), `sme-stated` (2), `assumed` (1). The query
engine respects ceilings when scoring — a doc claim never outranks a code fact.

**Why.** fathom is a downstream consumer of the agentic-kit platform; evidence
governance is exactly what that platform provides. We import it rather than
reinvent. See ADR-009.

## ADR-008 — Staleness is always reported

**Decision.** Every query result includes a staleness assessment (which sources,
when last ingested, whether re-ingest is recommended).

**Why.** Callers must distinguish "data is fresh" from "re-ingest before trusting
this." Silent staleness is a correctness trap for AI consumers.

## ADR-009 — Depend on agentic-kit as the platform substrate

**Decision.** fathom imports `github.com/geoffmilleraz/agentic-kit` (resolved
locally via `go.work` + a `replace` directive during co-development). Reuse:
`review/scoring` (evidence tiers), `evidence/*` (observation/contradiction
shapes), `telemetry`, `llmclient` (knowledge-card enrichment), `security`
(token redaction / DLP posture), `contracts`.

**Why.** Avoids duplicating governed-evidence primitives; keeps the two projects
coherent. Changes to agentic-kit are made there when a shared primitive needs to
evolve.

## ADR-010 — MCP server: stdio + HTTP, tool groups, profiles

**Decision.** Expose tools over MCP: `serve-mcp` (stdio JSON-RPC 2.0, zero ports,
for editor/agent clients) and `serve-http` (JSON-RPC + SSE, multi-client,
platform mode loads all stores at startup). Tools are grouped
(`coding`/`investigation`/`shared`/`platform`); clients select a profile via
`set_profile` to filter the surface. A `resultcache` memoizes expensive derived
graphs and is cleared on any ingest.

**Why.** Editors want a local stdio server with no ports; platforms want a shared
HTTP server. Tool groups keep the surface relevant to the task at hand.

**Amended by ADR-014:** `set_profile` is stdio-only; over HTTP the surface is
selected per request by the endpoint path.

## ADR-014 — Path-scoped tool surfaces on the shared HTTP daemon

**Decision.** Over HTTP, the tool surface is an **immutable per-request
selection carried in the request context**, chosen by the endpoint path:
`POST /rpc` is the full registry; `POST /rpc/{surface}` serves `companion` (a
curated coding-agent allowlist) or one of the four ADR-010 groups (group +
shared tools). A named surface filters `tools/list` **and hard-gates**
`tools/call` (including `batch` sub-calls) and hides canned prompts whose
workflows it gates. `set_profile` is rejected on every HTTP request; it remains
valid on stdio (where one client owns the process, the profile stays advisory —
list-filtering only), gaining `companion` as a value. Deliberately **not**
chosen: one `mcp.Server` instance per surface (duplicates stats/prompt/resource
state and re-creates split-brain risk with any future per-server cache), a
second daemon per use case (single-writer DuckDB), and a meta-tool gateway
(destroys per-tool schemas, permissions, and ToolSearch discovery).

**Why.** The daemon is shared by many concurrent sessions, so the ADR-010
profile — a process-global mutable — let any client flip every other client's
`tools/list`, and `listChanged:false` meant nobody re-fetched; meanwhile
`tools/call` ignored the profile entirely, so "hidden" tools were still
callable. Per-request scoping fixes both without forking the registry. Scoped
endpoints also give schema-loading MCP clients a hard-bounded, ~80%-smaller
surface, while schema-deferring clients (Claude Code) stay on `/rpc` where
deferral already makes the full registry cheap. The `serve-http --call-log`
JSONL (per-call tool/surface/latency, `via:"batch"` for sub-calls) makes
surface curation measurable instead of argued.

**Trade-off / reversibility.** Surface membership is a contract (pinned by
tests and `docs/mcp-tools.md`); the `companion` allowlist is a second place a
new hot-loop tool must be added. Purely additive over ADR-010 — removing the
scoped endpoints restores the prior behavior, except `set_profile`-over-HTTP,
which was judged a defect, not a feature.

## ADR-011 — Generalized / org-agnostic, open-source-ready

**Decision.** No organization-specific names, hosts, catalogs, or product
references anywhere in code, docs, tests, or fixtures. Anything org-specific is
configuration. The `testdata/fauxtown/` fixtures use invented, neutral names.

**Why.** The project is intended to be open-sourced and adopted by any org.

## ADR-012 — Test-driven development with golden fixtures

**Decision.** Build TDD: write the failing test (often against `FauxConnector` /
`testdata/fauxtown/`) before the implementation; extractor and query outputs are
asserted with golden files; determinism is an explicit, tested property.

**Why.** A code-intelligence engine is only trustworthy if its outputs are exact
and reproducible. Tests encode that contract.

## ADR-013 — Tree-sitter via CGO bindings, superseding ADR-004's wazero mechanism

**Decision.** General-language extraction (Python, JS/TS, Rust, C, C++, C#, Java,
Ruby, Kotlin, Scala, PHP, Swift, Lua, Bash, …) runs **tree-sitter via the official
CGO Go bindings** (`github.com/tree-sitter/go-tree-sitter` + per-grammar modules
like `tree-sitter-python`), not WASM/wazero. This supersedes the *mechanism* in
ADR-004 while keeping its intent. Go still uses `go/parser`; the Track A config/IaC
formats still use their native pure-Go parsers (HCL, yaml, proto, gqlparser, CUE,
goldmark, go-toml).

**Why.** ADR-004 chose wazero to avoid a native grammar build chain, but in
practice (a) there is no production-grade pure-Go tree-sitter-via-wazero runtime —
grammar `.wasm` blobs target the Emscripten/web-tree-sitter ABI, not bare wazero;
(b) the binary **already requires CGO** for DuckDB (ADR-003), so "CGO-free" is not a
real property of the build; and (c) the CGO grammar modules vendor their C source
and compile automatically under `go build` — there is no separate emscripten/Docker
step, so ADR-004's "free of native grammar build chains" goal is actually met, and
parsing stays deterministic and golden-testable. The cost is slower CGO builds and a
C toolchain at build time, both of which the DuckDB dependency already imposes. The
WASM path remains a viable future option if a pure-Go runtime matures.

**Trade-off / reversibility.** This adds one CGO-compiled dependency per language.
It was implemented incrementally (one language per commit) so any subset can be
reverted without affecting Track A or the Go extractor.

**Revisit when.** "The WASM path remains a viable future option if a pure-Go runtime
matures" — ADR-021 §4 adopted that path for Python/JS/TS/Ruby
(`internal/extractor/tsengine`); the nine languages still on the CGO bindings are
XT-6b (`docs/intent/futures.md` F1).

## ADR-015 — Rename to fathom; two strata: deterministic index + semantic layers

**Decision.** The project is renamed **fathom**. Its knowledge model is explicitly
two strata: the **deterministic index** (scraped: symbols, call edges, specs,
routes — cheap, incremental, provable) and **semantic layers** (reasoned:
LLM-compiled governed claims — purpose, obligations, precedent — with citations,
tiers, and staleness). The old name survives only in historical records
(CHANGELOG entries, USAGE_JOURNAL.md, dogfood-observations.md) and in field
back-compat (`.sextantignore` fallback, legacy telemetry prefixes, service-name
aliasing in session-hook and reingest tooling; see `docs/rename-migration.md`).

**Why.** A sextant navigates by surface sightings; a fathom measures depth. The
product's differentiation is no longer only navigation (where is the symbol) but
governed depth (what governs this file, why, and how sure are we). The rename
landed with the semantic-layers founding design so the brand and the architecture
say the same thing.

**Trade-off / reversibility.** One-time operator migration (daemon label, MCP
registration, fleet re-ingest); GitHub redirects the old repo URL. Back-compat
shims are removable once the fleet has migrated.

**Revisit when.** "Back-compat shims are removable once the fleet has migrated" —
the `.sextantignore` fallback, legacy telemetry prefixes, and service-name aliasing
go when no fleet repo carries the old name (`fathom fleet report`).

## ADR-016 — Semantic layers: governed claims beside the index, tracer-bullet first

**Decision.** Semantic layers are **LLM-compiled, governed claims** stored in
additive tables beside observations (`semantic_claims`, `claim_links`,
`compile_runs`, minimal `claim_events`) — never inside `observations` (re-ingest
purges destructively; contradiction scan is kind-blind). Claim identity is
content-addressed (layer, kind, subject, aiprovenance `repro_key`); superseded
claims are status-flipped, never deleted. V1 claims carry the existing
`doc-claimed` tier (citation-grounded doc restatements); a distinct `ai-derived`
tier waits for the first non-citable claim kind. Freshness is dual: a token-free
post-ingest citation sweep AND a query-time status check — stale claims are
served flagged, never silently. Compilation is incremental (per-section input
digests), DLP-gated, budget-gated (dry-run estimate mandatory; real usage
ledgered), and verified at compile time (claims with unresolvable citations or
scopes are rejected). The first layer is a **tracer bullet** — `obligations`
(governing docs → per-file rules, served ≤150 tokens on the companion surface,
zero-output-on-clean) — gated by eval fixtures (precision ≥0.90 with scope
traps) and a 3-week generalize-or-kill telemetry gate. The generic layer
runtime, lifecycle FSM/ledger, graph ontology, vectors, and new tiers are
deferred with named revival evidence. Full design: `docs/design/semantic-layers.md`.

**Why.** Judged synthesis of four competing architectures under three lenses.
The external record: deterministic spine first (embeddings-first engines got
retracted); freshness is the product-killer nobody solved for LLM artifacts;
coverage-first ontologies die before delivering; knowledge without a consuming
tool stalls; costs must scale with change size, not repo size. Fathom's edge is
governance — provenance, contradiction, staleness, costed compilation,
telemetry-gated existence — which no surveyed system exposes.

**Expert-module commitments (locked now so the north star needs no restart):**
corpus-agnostic core (open ref strings, never code-only types); the store is the
module (self-describing: claims, ledger, evals, watermarks travel inside it);
vectors are an additive in-store column (DuckDB VSS) when earned, never a second
store; determinism + tiers are the product; federation composes above the engine
(one module = one store), never inside it.

**Trade-off / reversibility.** If the tracer bullet fails its gate, the sunk cost
is three small tables and one compiler pass. Claim tables are additive; the
schema's stable IDs and repro keys make every deferred piece (ledger, tiers,
runtime) a non-destructive retrofit.

**Amendment (2026-07-16, M1).** `#Layer.tier` widens to admit `verified`
alongside `doc-claimed` and `sme-stated` (schema/semantic/schema.cue,
`internal/semantic/schema_test.go`): a claim kind whose truth is established
by a deterministic, Model-empty compile path — one of ADR-018's named
**oracle classes** (`index`, `cmd`, `lexical`; docs/design/sdd-reconciler.md
§5) rather than by LLM restatement (`doc-claimed`) or human corroboration
(`sme-stated`) — may declare a manifest layer without inventing a fourth tier
ad hoc. (Since 2026-07-23 the findings layer's deterministic review-verb
bridge — `internal/analysis/review`, docs/plans/review-verb.md C2-1 — is the
first `verified`-tier emitter; its `layer.cue` records the admission beside
the LLM-compiled corpus's own `sme-stated` tier.) `ai-derived` stays
deliberately unadmitted, still awaiting its own revival evidence (§4.2 above). This
amendment also retires, for the manifest LOADER specifically, the "generic
layer runtime … deferred with named revival evidence" clause above: M1
(`internal/semantic.LoadLayer`, docs/plans/m1-manifest.md) makes
`schema/semantic/schema.cue` a runtime artifact instead of documentation —
any layer definition the manifest declares is now loadable generically
(corpus globs, tier, budgets, serving caps decoded via `cuelang.org/go`, not
hand-mirrored Go constants), so the revival evidence for "the manifest
generalizes beyond one hand-coded layer" is satisfied by the loader
existing, independent of whether a second layer has shipped.

**Revisit when.** (1) "a distinct `ai-derived` tier waits for the first non-citable
claim kind" (`docs/intent/futures.md` F7). (2) The lifecycle FSM/ledger, graph
ontology, vectors, and new tiers are "deferred with named revival evidence" —
vectors were specified as "DuckDB VSS" and need a SQLite-era re-check before any
revival (F13). (3) "If the tracer bullet fails its gate" — the 3-week
generalize-or-kill telemetry gate on `obligations`.

**Amendment proposed 2026-09-02 (Q21): a measured obligation ceiling.**
*Status: proposed; the owner decides.* This ADR's 150-token serving ceiling
was set before the channel's cost was ever measured. HF-3 measured it
(`docs/measurements/2026-08-27-obligation-ceiling.md`, 2026-07-30 →
2026-08-26, one machine, one adopter, `tokens_estimate` = bytes/4, an
ESTIMATE): `post-edit` obligations payloads n=1,918, p50 131, p90 315, p95
329, max 378 tokens — **897 of 1,918 (47%) over 150**; `pre-edit-guard`
payloads n=370, p50 185, p90 198, p95 222, max 380 — **308 (83%) over 150**.
Rules per post-edit payload p50 3, p95 6. Enforcing 150 as written would
drop a median of two rules from roughly half of all edits, with no evidence
about which two (precision was not computable: the beat log records bytes
and rule counts, never text). Proposed: raise the ceiling to the measured
post-edit p90 (≈320 tokens) so invariant I35 stops being a permanently
VIOLATED row nobody can satisfy, and treat drop-ranking under any cap as
separate work with its own evidence (HF-4, Q13's options 1–3). Not
adopted by this paragraph: no code or invariant changes until the owner
ratifies; `docs/intent/questions.md` Q21 carries the decision.

## ADR-017 — Born-portable stores: repo-relative anchors, domain-neutral freshness

**Context.** D-EM2 ("the store is the module") was prose, not code: the local
connector recorded absolute machine-local paths as observation anchors, so a
store could not leave its build machine with citations intact — and freshness
assumed git, so non-git corpora (docs folders, findings exports — the expert-
module shape) could never say whether they were stale. Both are cheap at schema
time and migrations forever after, so the tax is paid **before** the ADR-016
claims DDL lands.

**Decision.**
1. **Repo-relative anchors.** Every connector emits slash-separated,
   root-relative `SourceRef.Path`s; `LocalConnector.Fetch` resolves them
   against the root recorded at Discover (absolute refs from legacy stores are
   still read as-is). The store never records a machine-local absolute path.
2. **Root remap.** Moving a corpus (or shipping a store) is one metadata
   update: `fathom remap --store … --service … --path …` rewrites
   `service_meta.path`; queries, source fetching, watermarks, and re-ingest
   resolve through it immediately.
3. **Watermark SPI.** `Connector.Watermark(ctx, svc)` returns an opaque
   freshness token: bare git HEAD SHA when available, else a content-addressed
   `manifest:`-prefixed hash over (relative path, content hash) pairs — stable
   across moves, changed iff content changes. The fusion pipeline records it as
   `IndexedCommit`; auto-refresh falls back to it when git HEAD is absent, so
   non-git corpora auto-refresh too.
4. **Self-heal.** An ingest that discovers relative refs against a store
   holding absolute-anchored fingerprints purges that service's content
   wholesale (observations, fingerprints, doc summaries; identity rows kept)
   and rebuilds — one-shot, reported via `Report.PurgedLegacyAnchors`. No
   operator memory required; `scripts/reingest-fleet.sh` remains the clean
   rebuild.
5. **Tier profiles (data now, machinery later).** The ADR-016 claims schema
   carries a `tier_profile` reference column from birth so tier vocabularies
   can be reinterpreted per corpus domain when a non-engineering module first
   needs it (its named revival evidence).

**Contract.** `TestStorePortability_RoundTrip` (pkg/client): build a store on
corpus A, move the corpus, remap the root — symbols answer, citations resolve,
the watermark reads fresh (not falsely drifted), re-ingest is fully hash-gated.
That test is the publishable meaning of "portable store" until the H2 format
spec exists.

**Trade-off / reversibility.** Manifest watermarks read the whole corpus (fine
for small doc corpora; git corpora use HEAD). Legacy stores keep working
read-only via the absolute-ref fallback; the first write-path ingest converts
them. Reverting to absolute anchors would only require connectors to stop
relativizing — nothing downstream depends on absoluteness anymore.

**Revisit when.** (1) Tier profiles: "machinery later … when a non-engineering
module first needs it (its named revival evidence)". (2) "Manifest watermarks read
the whole corpus" — a non-git corpus large enough that the manifest hash dominates
ingest time.

## ADR-018 — Plan grammar v2 + reconciler verbs: SDD state is derived, never driven

**Context.** The 2026-07 excellence review (docs/vision/09) examined "should
SDD be a deterministically driven state machine in CUE?" against the July-2026
landscape (every shipped SDD tool self-reports "done"; no tool checks task
state against a code index), the Dagger CUE post-mortem (CUE-as-authoring-UX
and CUE-as-engine both failed with a funded team), and the fleet's own live
programs, which hand-invented machine-invisible states (`[~]`, "honestly
refused", "deferred pending trigger", prose wave-advance rules) that the
verify gate cannot see. Meanwhile H1 (the handoff program) fixed the gate's
recall (method-call edges — O6 — plus dedup, `package:`, index hygiene), so
the evidence layer can finally carry process weight.

**Decision.** Adopt the reconciler shape, not a state machine
(docs/design/sdd-reconciler.md is normative):
1. **Grammar v2 (additive):** `[~]` in-progress; author-owned stable `id:`;
   typed dispositions on `[-]` (`disposition: refused|deferred` + required
   `reason:`, optional recorded-not-evaluated `trigger:`); `skip:`
   justification for `skip_test`; double-quoted annotation values.
   Conventions enforced by `verify.LintPlan` as warnings.
2. **Wave gate:** `fathom verify --require-all` (`verify.GateRequireAll`) —
   a wave advances only when every task is claimed-and-verified or
   explicitly skipped. The recorded exit-0 run is the transition record.
3. **CUE shadow:** `fathom gen-plan-cue` emits a generated-only `#Plan`
   document and proves lossless round-trip. **Plan** CUE is output, never
   authored — this constrains generated `#Plan` documents only, NOT the
   hand-authored `schema/**` packages every schema in this repo is written
   as (see CLAUDE.md's "New CUE schemas" convention, which mandates
   hand-authoring them with valid/invalid testdata).
4. **Stop hook:** `fathom stop-hook` — newly-claimed-this-session tasks that
   fail verification surface at session end; advisory by default,
   `--block` opt-in; silent on any operational failure.

**Rejected:** authored transition tables and lifecycle enums (declared state
that can lie — checkbox theater one level up); CUE as authoring surface or
execution engine (`tools/flow`); a separate SDD binary/repo/family (no
consumer without the index); machine-evaluated triggers in v0 (speculative
engine); persisted process state (each run stays a pure re-derivation).

**Consequences.** v1 plans parse unchanged; `md-NNN` positional IDs remain
the default (stable IDs are a lint convention for gated plans). The pilot
(agentic-kit handoff program + this repo's own plans) runs 4 weeks against
the five metrics / four kill criteria in the design doc §6; a kill reverts
the hook and generator while keeping the harmless grammar extensions.

**Revisit when.** (1) "The pilot … runs 4 weeks against the five metrics / four
kill criteria in the design doc §6; a kill reverts the hook and generator" — due
about 2026-08-12; the 2026-08-21 audit measured the gate PROVEN (180 of 436 runs
caught claimed > verified, `docs/vision/24-work-frontier.md` §7) but no keep/kill
record exists (`docs/intent/questions.md` Q9). (2) "machine-evaluated triggers in
v0" were rejected as a speculative engine — a v1 reopens them only on the condition
in `docs/intent/futures.md` F9.

**Pilot verdict (2026-08-26): KEEP.** The 2026-08-21 audit measured 180 of 436 gate
runs in five days catching claimed > verified (`docs/vision/24-work-frontier.md` §7);
none of the four kill criteria in `docs/design/sdd-reconciler.md` §6 tripped. The
stop hook and the CUE shadow stay. Condition (2) has NOT fired: plan `trigger:`
values across the three consumer repos are all unique prose (the design panel of
2026-08-26 checked), so F9 stays a future and plan triggers stay free text.
ADR-026 opens annotated conditions on a different corpus (intent docs and ADR
trailers) on a different ground: the annotation itself is the demand signal, and
its runtime is not built until ≥3 exist across ≥2 repos.

## Amendment to ADR-014 (2026-07-16, surfaces week-1)

The daemon's 8-day call log (docs/vision/05-surfaces.md) showed both agents
that ever reached for an off-surface tool bouncing off ADR-014's hard gate —
an error teaches abandonment, which looks identical to no-demand in the log,
so the gate was actively destroying the evidence it needed to justify itself.
Gate semantics change from hard-gate to **execute-and-flag**: an off-surface
`tools/call` or `batch` sub-call now executes normally, and the JSONL call log
records it with `off_surface:true` instead of returning a "not on the
surface" error, so demand is measurable rather than argued. Two facts this
makes explicit: surfaces are **ergonomics and measurement, never a security
boundary** — bare `/rpc` already serves the full, unauthenticated registry,
including `ingest` (writes) and `compile_layer` (LLM spend), so a scoped
surface was never doing access control; and therefore `serve-http`'s default
listen address changes from `:7575` (all interfaces) to `127.0.0.1:7575`
(loopback) in the same change — previously only the launchd plist override
made it loopback. The pre-amendment hard gate stays available behind
`Server.strictSurfaces` for any future multi-tenant shape.

`set_profile` is retired entirely (further amends ADR-010/014's stdio
clause, which had kept it valid there as advisory list-filtering): no
production caller — stdio or HTTP — reaches it any more, so stdio now always
serves the full registry. The `companion` allowlist swaps membership to the
15-tool **agent** surface (docs/vision/05-surfaces.md §3: companion 15, minus
`batch` and `brief`, plus `verify_plan` and `capabilities`, the top non-hook
tool and the discovery escape hatch respectively) and is served at the new
primary path `POST /rpc/agent`, with `POST /rpc/companion` kept as a
compatible alias. *(Amended 2026-07-24: `read_source` — the bounded raw-text
read, T5-4, docs/plans/trust-and-tokenomics.md — joined the allowlist on
measured H3-1 evidence; the surface is now 16 tools. docs/mcp-tools.md holds
the live membership.)* The four ADR-010 group surfaces (`coding`/`investigation`/
`shared`/`platform`) are unchanged and un-killed — zero observed traffic, kept
for taxonomy completeness pending the 30-day post-flip log.

**Revisit when.** (1) The four group surfaces: "kept for taxonomy completeness
pending the 30-day post-flip log" — the clock ran from 2026-07-16 with zero
observed traffic and no kill record exists (`docs/intent/questions.md` Q9).
(2) "The pre-amendment hard gate stays available behind `Server.strictSurfaces`
for any future multi-tenant shape" — a hosted or multi-tenant deployment
(`docs/intent/futures.md` F8) turns it back on.

**Amendment 2 (2026-08-26).** The four group surfaces (`coding`, `investigation`,
`shared`, `platform`) were reachable only by URL path after the flip — no client
registration, stanza, skill, or doc pointed at them — so their zero calls prove
exposure-zero, not demand-zero (law 8 forbids reading that as a kill clock). They
are deleted on law 4's ground instead: a surface no wired client can reach is
registry weight with no consumer. `agent` (+ the `companion` alias), `modules`, and
bare `/rpc` remain; `Server.strictSurfaces` remains. Executed by
`docs/plans/honesty-fixes.md` HF-5; the trailer's condition (1) is thereby decided.

## ADR-019 — In-tree mirror of agentic-kit for kit-free distribution

**Context.** fathom's only private dependency is
`github.com/geoffmilleraz/agentic-kit` (ADR-009); a cold clone of fathom by
someone without access to that private repo cannot resolve it. A full `go
mod vendor` of fathom is 565MB — 538MB of it DuckDB's static libs — which is
not something to commit; the kit's own tree is small enough (low
double-digit MB at worst) to commit directly.

**Decision.** `third_party/agentic-kit/` is a committed, full mirror of the
agentic-kit tree at **exactly** the commit `go.mod`'s pseudo-version pins,
produced by `git -C ../agentic-kit archive <commit> | tar -x` (the whole
tree minus `.git`; nothing else excluded — exactness is the point, since the
drift guard below does a byte-level content compare). `go.mod` carries
`replace github.com/geoffmilleraz/agentic-kit => ./third_party/agentic-kit`.
Precedence is layered, not switched: with `go.work` active (local
co-development, sibling `../agentic-kit` checked out), the workspace's `use`
directive wins over the `go.mod` replace, so day-to-day dev is unchanged;
with `GOWORK=off` (a cold clone has no `go.work` at all), the `go.mod`
replace resolves to the in-tree mirror. Both paths are proven by `go list -m
-f '{{.Dir}}' github.com/geoffmilleraz/agentic-kit` under each mode.

`scripts/sync-vendor.sh <name>` (`task sync-kit` runs it for both) re-archives
a sibling at the pinned commit into `third_party/<name>`; it refuses to run if
the sibling lacks that commit.

**Amended 2026-09-01.** This record described a guarantee the repo did not
keep. `app-kit` — the second private module, and one fathom's observatory does
not compile without — was never mirrored: it carried a bare
`replace => ../app-kit`, so a clone without private access failed outright.
`go.work` was also COMMITTED, listing both siblings, which defeated the
mechanism above before it could run and masked missing `go.sum` entries.
Both are fixed: app-kit is mirrored on identical terms, `go.work` is
untracked and gitignored, and each mirror records its source commit in
`third_party/<name>/.vendor-commit` (the stamp, not the pin, is authoritative
— app-kit has no published version to derive a pin from, and a stamp needs no
network for anyone, which is the point of a mirror). The guarantee is now
GATED rather than asserted: `task clean-clone-check`, a `task validate`
dependency, copies the tracked tree, drops the workspace and builds with
`GOWORK=off`. It exists because this failure was invisible to every machine
that could have caught it — all of them had the siblings. `scripts/vendor-check.sh` (`task vendor-check`,
a `task validate` dependency, so part of the pre-push lefthook gate) is the
drift guard: it re-derives the pinned commit's archive into a temp dir,
content-hashes both trees (sha256 per file keyed by relative path, not raw
tar bytes — `git archive` embeds the commit timestamp in every tar entry,
which would make identical content hash differently), and fails the push if
they disagree. It skips gracefully (exit 0, warning) when `../agentic-kit`
isn't present or lacks the pinned commit — the expected case for anyone
without kit access, so the gate never blocks a consumer who was never
supposed to have the sibling in the first place.

`third_party/agentic-kit/` is excluded from `task fmt-check` / `task
cue-vet`'s file scans (Taskfile `GOFILES`, the `cue-vet` `find`): it's
upstream's code and upstream's formatting; our gate owns drift detection
(`vendor-check`), not gofmt opinions on code we don't maintain.

For a fully offline build, `go mod vendor` now succeeds standalone (the one
module that used to require private-repo access resolves in-tree); the
resulting `vendor/` directory is the consumer's own local artifact, never
committed (`.gitignore`). See docs/distribution.md.

**Why.** Avoids duplicating the 565MB DuckDB vendor problem while still
letting fathom build with zero private-repo access — the smallest mirror
that makes a cold clone self-sufficient. Content-hash drift detection (not
a manual "remember to re-sync" convention) is what makes it safe: a kit pin
bump that forgets `task sync-kit` fails the push, not silently ships a
mismatched mirror.

**Trade-off / reversibility.** `third_party/agentic-kit` tracks the kit's
own commit history size, not just fathom's needs (the pinned commit's
archive measured ~13MB against an initial ~716KB estimate, including one
~4.1MB binary that appears to be an accidental upstream commit — see
docs/distribution.md's size note). Removable entirely by reverting to a
bare `replace ... => ../agentic-kit` if kit access becomes universal; the
sync/drift-guard scripts are additive and inert without a `third_party/`
directory to check.

**Revisit when.** "Removable entirely by reverting to a bare `replace ... =>
../agentic-kit` if kit access becomes universal" — the kit repository goes public.

## ADR-020: Store backend — pure-Go SQLite (WAL) replaces DuckDB (2026-07-22)

**Decision.** `internal/store` gains a SQLite backend (modernc.org/sqlite,
pure Go, WAL mode) as the default; DuckDB remains temporarily behind the
same `Store` interface for migration, then is removed. Backend selection
is extension-based in a single shared helper (`store.BackendForPath`),
replacing the four independent `.duckdb` hardcodes (store.Open,
PlatformStore.load, genmanifest, platformingest).

**Why.** (1) Concurrency: SQLite WAL gives many cross-process readers
beside one writer — hooks and CLI verbs read the live store while the
daemon writes; a wedged daemon degrades to "writes pause", not "fathom is
down". DuckDB's cross-process single-writer forced daemon-stop rituals
(claim, report, store-mode compile) and made daemon wedges total outages.
(2) Distribution: dropping DuckDB removes CGO entirely — cross-compiled
single-binary releases, no 538MB of platform static libs, no
GOTMPDIR/link/deployment-target pain. (3) Fit: the audited workload is
point lookups and joins over a 26MB store — no OLAP need; all five ON
CONFLICT upserts port verbatim (SQLite ≥3.24 semantics).

**Constraints.** Fresh final-column DDL for SQLite (no ALTER-IF-NOT-EXISTS
emulation; only DuckDB carries legacy on-disk migration). Connection
pragmas: journal_mode=WAL, busy_timeout, uncapped read pool for file
stores (the SetMaxOpenConns(1) serialization is DuckDB-specific and must
NOT carry over; the sole exception is the in-memory store, whose pool is
capped at one because each modernc ":memory:" connection is a private
database). PutObservations keeps its purge-then-insert contract — the doc
comment's implied DB-level idempotency is corrected, not "fixed" with a
constraint. Migration ports the eight non-derivable tables
(semantic_claims, claim_links, claim_events, compile_runs, service_meta,
commits, knowledge_cards, doc_summaries — the last two because they are
LLM-derived enrichment a re-ingest does not rebuild in practice, so
"derivable" would mean silently lost); only observations/fingerprints
rebuild by re-ingest.

## ADR-021: Extraction v2 — language truth, tool identity, typed edges, tree-sitter (2026-07-22)

**Decision.** Four connected corrections to how fathom understands source:
(1) One language registry: the extractor registry becomes the single
source of language identity; `langFromPath`'s parallel 10-language table
is deleted. Binary files are detected (NUL-byte sniff) and excluded from
line counting; unclassified files are surfaced, never silently bucketed.
(2) Tool identity: extractor dispatch moves from path-only lookup to a
pass-3 classifier — content + path + a per-ingest sibling index —
returning a ToolKind that keys extractor registration. Helm templates,
kustomizations, compose, k8s manifests, CUE modules, timoni bundles,
OpenAPI documents, and generated files are recognized as what they are.
(3) Typed Go call edges: an additive enrichment pass (go/packages +
go/types) resolves the ambiguous-method residue into edges with kinds —
direct, interface-dispatch (edge-set to implementers), unresolved —
carried on core.Edge, never mutating the existing syntax-derived Calls
contract; verify counts dispatch-sets honestly as "reached via any of N".
(4) Tree-sitter (wazero/WASM, never CGO — coherence with ADR-020) becomes
the extraction engine for new-language breadth, gated per language by the
calibration corpus.

**Why.** The index's authority is fathom's product. An index that
line-counts a Mach-O binary as 423K lines of code, renders 14 of 24
supported languages as "other", silently drops every Helm chart's
templates, and refuses call edges for any method name two types share is
not yet authoritative. Each correction removes a class of silent
wrongness; none adds runtime machinery.

## ADR-022: MCP server stays bespoke, not `kits/mcp-server` (2026-07-23)

**Decision.** `internal/mcp.Server` (ADR-010) remains a from-scratch JSON-RPC
2.0 dispatcher — its own `Handler` table, its own stdio `Serve` loop, its own
`toolEntry`/group bookkeeping — rather than adopting agentic-kit's
`kits/mcp-server` (`mcpserver.Server`, composed over
`foundations/agent/tool-registry`). Records the deliberate non-adoption from
Phase K (K4-1, `docs/plans/kit-uplift.md`).

**Why (read both implementations, not assumed).** The kit's v1 is
deliberately **tools-only**: `initialize`/`ping`/`tools/list`/`tools/call`
only — `prompts/list`, `resources/list`, etc. return `-32601` by design
(`kits/mcp-server/doc.go`), and it has no tool-group or surface concept at
all — one flat `toolregistry.Registry`. Fathom needs exactly what the kit
doesn't have and actively uses today: tool groups gated per-surface
(ADR-014's scoped surfaces — the former `set_profile` RPC was retired in the
surfaces redesign), plus canned `prompts/list`+`prompts/get` and
`resources/list`+`resources/read` (`internal/mcp/meta.go`). Fathom's ~80+
tool handlers are also directly coupled to its own `QueryInterface`
(store/call-graph backed) with hand-maintained JSON schemas; porting them
onto `toolregistry.Tool` wrappers would be a full-surface rewrite for zero
behavioral gain — the kit's dispatch core (framing, batch-array rejection,
oversized-message resync) is solid, but fathom would still have to build the
surface/group/prompts/resources layer *on top of it*, so adoption relocates
work rather than removing it. Fathom's call log also carries
`service`+`group`+`surface`+`origin` dimensions (`internal/mcp/calllog.go`)
that the adoption-measurement program depends on; the kit's call-log entry
(`ts, method, tool, duration_ms, is_error`) carries none of them.

**Honest trade-off.** Fathom forgoes, for free, engineering the kit has
already hardened: DNS-rebinding `Origin` validation on Streamable HTTP, W3C
`traceparent` propagation, a documented oversized-message resync policy, and
JSON-RPC batch-array rejection. `internal/mcp`'s HTTP path and resync
handling have not been audited to the same standard — accepted debt, not an
oversight. If the kit grows surface/group/prompts/resources support,
revisit; today adoption would cost a full rewrite to buy back functionality
fathom already has.

**Revisit when.** "If the kit grows surface/group/prompts/resources support,
revisit" (`docs/intent/futures.md` F11). The accepted debt — unaudited HTTP
`Origin` validation, `traceparent`, resync policy, batch-array rejection — is
reopened by any hosted deployment (law 9, F8).

## ADR-023: Evaluation stays bespoke — `calibration/` + `internal/verify`, not `kits/eval-runner`/`qa-harness` (2026-07-23)

**Decision.** Fathom's two evaluation instruments — `internal/verify` (the
`fathom verify` static proof-gate: `Task`/`Criterion`/`Expect` checked
against a code index — symbol presence, tier, test-reaches, contradiction,
spec coverage; explicitly STATIC, never asserts `runtime_pass`) and
`calibration/` (a bash harness measuring per-language extractor **symbol
recall** against a golden/proxy-oracle corpus) — stay bespoke rather than
adopting agentic-kit's `kits/eval-runner` / `foundations/quality/qa-harness`.
Records the deliberate non-adoption from Phase K (K4-1).

**Why (read both kit packages, not assumed).** The kit's eval unit of work
is a **scenario scored by an LLM judge** against a cassette-replayed corpus,
budget-guarded, with a regression gate against a committed baseline. That is
evaluation of **agent/LLM output quality**. Fathom's two instruments
evaluate neither: `internal/verify` mechanically checks whether claimed code
facts are backed by index evidence — no judge, no scenario, no cassette, a
deterministic pure function of `(Plan, Evidence)` — and `calibration/`
mechanically counts extractor symbol recall against a proxy oracle. Bolting
either onto the judge-scored scenario model would be a category error, not a
reuse win — there is no LLM call to judge in either.

**Honest trade-off.** `calibration/` is bash, not Go — no `-race`/`go test`
coverage, no cassette replay, no committed-baseline regression gate the way
`qa-harness` scenarios have; `internal/verify` has no budget-governed LLM
lane because it makes no LLM calls to govern. If fathom later adds an
LLM-judged evaluation lane (e.g. grading LLM-enriched knowledge cards),
*that* lane is the natural `qa-harness` adoption point — parked as a named
ask with a second-consumer trigger (`agentic-kit/docs/plans/
fathom-uplift-asks.md`, FA-12) rather than forced here.

**Revisit when.** "If fathom later adds an LLM-judged evaluation lane (e.g. grading
LLM-enriched knowledge cards), *that* lane is the natural `qa-harness` adoption
point" — FA-12's second-consumer trigger (`docs/intent/futures.md` F12).

## ADR-024: Pack taxonomy adopted as vocabulary, not machinery (2026-07-23)

**Decision.** Fathom's various pluggable, data-shaped things — extractors,
the layer catalog, a project's own `.fathom/` content, and a still-hypothetical
"explain this external technology" pack — are named as four **strata**, each
with its own authorship and trust model, recorded in full in
`docs/design/pack-taxonomy.md`:

| Stratum | Fathom realization today | Authored by | Trusted via |
|---|---|---|---|
| dialect | extractors + tool identity (code, not packs) | fathom | golden tests, deterministic |
| provider | does not exist — parked pack type | fathom or vendor | eval-gated, staleness vs. an UPSTREAM |
| platform | `.fathom/` project packs (an org's own obligations/rules/bindings) | the org | compiled from an org corpus; on-disk eval evidence required (D3) |
| concern | the layer catalog + rule packs + analyzers | fathom + org extensions | eval-gated embedded; entry-conditioned growth |

This is **vocabulary layered onto mechanisms that already exist** — no new
subsystem. `#Layer` (`schema/semantic/schema.cue`) gains one additive field,
`stratum: "concern" | "platform" | *"concern"`, decoded by
`internal/semantic.LoadLayerIn`/`ListLayersIn` exactly like every other
manifest field; the four shipped packs (`obligations`, `findings`,
`adr-rules`, `spec-bindings`) are all annotated `stratum: "concern"` —
including `obligations`, whose corpus (`CLAUDE.md`, `ADR.md`,
`docs/plans/README.md`) is as org-specific as content gets, but whose
pack (prompt, corpus-selector pattern, eval fixtures) is authored and
gated by fathom once and shipped identically to every adopting repo — see
`docs/design/pack-taxonomy.md`'s "The obligations call" for the full
reasoning distinguishing pack authorship from compiled-corpus content.
`fathom layers list`/`show` and `docs/layers.md`'s catalog display the
field; no enable/disable semantics change.

**Two pack types are parked with named triggers, not built:**
- **Provider pack type** (TX-3, `docs/plans/pack-taxonomy-adoption.md`) —
  explaining an external technology's resource semantics, the first pack
  type whose corpus would drift against an UPSTREAM rather than against
  this repo's own churn (a genuinely new watermark kind). **Trigger:** a
  named consumer plus one concrete provider whose resource semantics an
  indexed service actually needs.
- **Practice/concern pack library growth** beyond the shipped catalog
  (TX-4, same plan) — e.g. a well-architected review pack, a compliance
  pack. **Trigger, per pack:** a real consumer with a real corpus, eval
  fixtures, and a kill criterion — the playbook pack's documented "≥15
  findings" condition (ADR-016 §4.6's entry-conditioned-growth clause) is
  the model every future concern pack's admission case should cite.

**Why.** `adr-rules` and `spec-bindings` sat eval-perfect with zero
production claims for weeks (`docs/dogfood-observations.md`) — a pack
library, or a fourth pack type, built ahead of a named consumer is theater,
not progress. Naming the taxonomy now — while deliberately building only
the one-field schema addition it requires — lets every future pack-shaped
design question ("what kind of pack is this, who's allowed to author it,
how does it earn trust?") answer by lookup instead of re-derivation,
without pre-building either parked type's machinery on spec.

**Formalizes ADR-011.** The platform stratum is ADR-011's "organization-
specific content is configuration, never product" given a stable name and
the trust mechanism (`.fathom/` project packs, D3's live on-disk
`eval/RESULTS.md` check) that already implements it.

**Trade-off / reversibility.** Zero new machinery means zero new sunk cost
if the vocabulary doesn't stick — the only artifact that outlives a
taxonomy nobody uses is one additive, defaulted CUE field. Naming
`provider`/further `concern` packs ahead of their triggers would invert
this ADR's own reasoning, so both stay recorded-not-built until a named
consumer arrives.

**Amendment (phase O, 2026-07-29, `docs/design/platform-modules.md`).**
The platform stratum's only realization above (`.fathom/` project packs)
had a finding: N repos meant N copies of the same org knowledge, N
compiles, N independent drift clocks, zero shared freshness story. Phase O
adds the platform stratum's actual DISTRIBUTION vehicle — a platform
module (compiled once, centrally, portable, eval-gated) that a consumer
repo *binds* via `.fathom/modules.cue` (schema `schema/binding/`), served
through the same forced channels (edit-hook, `obligations` tool) the
excellence benches already proved, module-tagged and never silent on an
unmounted binding. `.fathom/` project packs remain correct for genuinely
repo-specific rule authoring; copying org-wide content into them is now
the anti-pattern this amendment steers away from. See
`docs/design/platform-modules.md` for the full design and
`docs/use-cases/platform-modules.md` for the adoption path.

**Revisit when.** (1) Provider pack type (TX-3): "a named consumer plus one
concrete provider whose resource semantics an indexed service actually needs"
(`docs/intent/futures.md` F5). (2) Concern pack growth (TX-4): "per pack: a real
consumer with a real corpus, eval fixtures, and a kill criterion" (F6). (3) A
`.fathom/` project pack found copying org-wide content — the amendment's named
anti-pattern — becomes a platform-module binding instead.

## ADR-025: DuckDB backend deleted outright — SS-8 executed (2026-07-23)

**Decision.** ADR-020 made SQLite the default store backend but kept the
DuckDB backend compiled in as `migrate-store`'s read source for the fleet
migration, deferring its removal (SS-8, `docs/plans/store-sqlite.md`)
behind an explicit trigger: *"the operator runs migrate-store on the fleet
DB and the daemon runs a full day green on sqlite."* That trigger fired —
the fleet store migrated 2026-07-23T01:56:17Z
(`workspace/logs/store-migration-stamp.txt` on the fleet checkout) and the
daemon ran green on SQLite through the day. This ADR records the execution:
the DuckDB store file under internal/store (DuckStore over the
go-duckdb/v2 driver), its test file, the non-derivable migration file
(MigrateNonDerivable), and the migrate-store CLI verb under cmd/fathom are
all deleted outright (historical names, none exists at HEAD;
`internal/store/backend.go` and `internal/store/sqlite.go` are the living
successors), along with the DuckDB
driver, its per-platform bindings, and every indirect dependency that
existed only to support them (`go mod tidy` also dropped `apache/
arrow-go`, `klauspost/compress`, `klauspost/cpuid/v2`, `pierrec/lz4/v4`,
`zeebo/xxh3`, `google/flatbuffers`, `go-viper/mapstructure/v2`,
`goccy/go-json`, `golang.org/x/exp`, `golang.org/x/telemetry`,
`golang.org/x/xerrors` — none of these served anything but DuckDB/Arrow).

**`migrate-store`'s fate, decided explicitly.** The plan's own text ties
CGO's continued presence entirely to `migrate-store` needing the DuckDB
driver as its read source (`docs/distribution.md`'s pre-SS-8 CGO note:
"still compiled in only because `migrate-store` needs it as the source
backend for the DuckDB→SQLite migration"). Once the fleet migrated, keeping
`migrate-store` — and therefore the DuckDB driver, and therefore CGO for
the store — would have defeated SS-8's entire point for a tool with no
remaining live purpose: a straggler store that still needs migrating can be
migrated with a pre-SS-8 build of fathom, which remains available via git
history/tags. The command is removed, not deprecated-in-place.

**`store.BackendForPath` stays loud, never silent.** `.duckdb` remains a
*recognized* extension in `storeExtensions` — so `ResolveStoreFiles` still
detects a same-stem collision between a leftover pre-SS-8 file and a
`.sqlite` sibling, and the `.sqlite` file still wins with a warning, exactly
as before — but `BackendForPath` now refuses to ever open a `.duckdb` path,
returning a typed error naming the file and pointing at a pre-SS-8 build.
`PlatformStore.load` therefore fails loudly (not silently, not by ignoring
the file) if a *lone* `.duckdb` file with no `.sqlite` sibling would win a
stem — the correct behavior for a store extension whose backend no longer
exists. The `""` in-memory convention (`store.Open("")`,
`BackendForPath("")`) now opens an in-memory `SQLiteStore` rather than an
in-memory `DuckStore` — a behavior change ModeFaux/session-mode callers do
not observe (both are process-local, discarded on `Close`).

**Consequence: fathom is pure Go except tree-sitter.** `internal/store`
builds and passes `go test -race` under `CGO_ENABLED=0` with zero
workarounds — proven, not asserted, as part of this change. `CGO_ENABLED=1`
and `MACOSX_DEPLOYMENT_TARGET=14.0` remain required for the full `fathom`
binary and `task validate`, but the reason has changed: it is now solely
the tree-sitter extractor bindings (ADR-013,
`internal/extractor/treesitter`, unconditionally imported by nine
language extractors) that need CGO. Moving those to the wazero/WASM engine
(ADR-004's documented future path, already how Python/JS/TS/Ruby extract
via `internal/extractor/tsengine`, ADR-021 §4) is XT-6b's separate,
not-yet-triggered slice — this ADR does not touch tree-sitter.

**Not touched.** The live fleet store, the always-on `serve-http` daemon,
and `workspace/db/fleet.duckdb` (the on-disk rollback file from the
migration) are all outside this change's scope by design — SS-8 is a
source-tree removal, not a fleet operation. `fleet.duckdb`'s deletion is
the operator's manual step to take once satisfied it is no longer needed.

**Revisit when.** (1) XT-6b: "Moving those to the wazero/WASM engine … is XT-6b's
separate, not-yet-triggered slice" — the sole remaining reason for `CGO_ENABLED=1`
(`docs/intent/futures.md` F1). (2) "`fleet.duckdb`'s deletion is the operator's
manual step" — done: `workspace/db/` held no `.duckdb` file on 2026-08-25.

## ADR-026: The evidence plane — intent corpus, unproven claims, change records (2026-08-26)

**Status.** Accepted. **Type.** Architecture / process. Design:
`docs/design/evidence-plane.md` (panel-reviewed; findings disposed there §12).
Plan: `docs/plans/evidence-plane.md`; the honesty fixes it forces:
`docs/plans/honesty-fixes.md`.

**Context.** Three repositories now carry Project Intent (`docs/intent/`, agentic-kit's
practice): human-authored living docs whose every entry carries an expiry condition
(`Done when`, `Promote when`, `Drop if`, `Revisit when`), invariants with an honest
enforcement column, and decision records with revisit trailers. Fathom already makes
binding documents checkable (obligations, adr-rules, the custodian, `verify`, the
sweep) and ignored this corpus. The owner asked whether fathom should absorb the
concepts, optionally; whether "honesty debt" should be tracked; whether fathom should
be more git-aware — without bloating agent configuration.

**Decision.**
1. *Corpus, not engine.* `internal/intent` parses the router, the living docs, and
   decision records deterministically; fathom never edits them, never evaluates
   prose, never reads `docs/vision/`. Capability `intent-corpus` is on when the
   router exists and off-able; off silences delivery, never the verb.
2. *Annotated conditions only.* A condition line may carry one `{oracle: …}` block
   (plan tokenizer; classes `index`, `plan`, `lexical`, `date`; `cmd` deferred);
   unannotated conditions are human-judged. Evaluation runs only in the verb, never
   in a hook. Every verdict transition is recorded first-seen in a separate
   `~/.fathom/intent-ledger.jsonl`; "decided" is derived from the record's own diff
   after that timestamp. The runtime is built only after ≥3 annotations exist across
   ≥2 repos (K0).
3. *Unproven claims, never a score.* `fathom intent audit` lists plan rows that fail
   or carry no contract (from the verify ledger at HEAD), `VIOLATED` invariants,
   invariants and `Done needs` naming tests or gates that do not exist, and
   fired-undecided conditions — each with a remedy, capped per kind, no total
   presented as a grade.
4. *No new channel.* No MCP tool; no edit-time injector (the pre-edit guard path is
   fed by `fathom intent export --guard`, a proposal a human applies); the session
   attention line is the custodian's, extended, under a new 3,200-byte whole-primer
   cap; the stanza's obligations sentence is amended, no line added; global agent
   config gains nothing.
5. *Change records.* Ship now: plan-archive absorption via git history (commits that
   touched non-plan paths), `commit` on thread and ledger rows, a shell commit-msg
   advisory in the init MANUAL block, and the custodian kind
   `adr_references_resolve`. Defer the Go change-record kinds and cut `changes impact`.

**Alternatives rejected.** The broad form (fathom owns propose/decide/close) — wins
if decision latency, not detection, ever measures as the bottleneck. A CUE intent
catalog (DP-1) — wins if a second tool consumes intent as data (`export --cue`).
Compiling intent docs through the LLM layer — wins if a measured precision gap
appears. An edit-hook intent block — wins never: the guard path already exists and
the payload ceiling has no residual. A second attention line — never.

**Consequences.** One verb with two subverbs, two doctor rows that never FAIL, one
ledger file added to the compatibility surface, `internal/pathglob` promoted,
`internal/verify.ParseAnnotation` exported, `internal/adr` extended, the reconciler's
forward registry gains `plan` and `date`. The practice's own rejections of lint tools
and generated registries are honored by construction: prose stays human, nothing
generated is edited.

**Preserves / forecloses.** Preserves F18 (federation) by taking a root, never a fleet;
preserves DP-1's CUE face as output. Forecloses nothing documented.

**Deferred.** `cmd` oracle; change-record custodian kinds; unproven sources U6–U10;
LLM compile of the corpus; `export --cue` — each with its trigger in the design §9.

**Revisit when.** (1) Any kill clock in the design §8 fires — K0 (fewer than three
annotations), K1 (annotated conditions decided no faster than human-judged), K3
(no verb use), K5 (the attention line ignored), K-plane (no session outside this
repo uses it). (2) agentic-kit records a decision refusing annotations in its
template docs (Q18). (3) The practice ships its own evaluator. (4) A second tool
wants intent as data (`export --cue`). (5) Plan `trigger:` vocabulary actually
repeats across two repos (the real F9). (6) Decision latency measures as the
bottleneck the broad form would remove.

## ADR-027: Coverage before conclusion — an instrument may not degrade toward "healthy" (2026-08-27)

**Status.** Accepted. **Type.** Architecture / process. Design:
`docs/design/reader-coverage.md`. Plan: `docs/plans/reader-coverage.md`.

**Context.** ADR-026 shipped a reporting layer whose job is honesty: an audit
that lists what a repository claims and what nothing proves. Measured the day
after it shipped, that layer failed at its own job. With
`docs/intent/invariants.md` made unreadable — the state a wedged macOS TCC
verdict produces for a whole process tree on this machine, twice during the
session that wrote ADR-026 — `fathom intent audit` reported **0 invariants
instead of 37 and 85 unproven claims instead of 169, naming nothing**. The
repository's most broken state was its healthiest-looking one.

The cause was eight readers doing `if err != nil { continue }`, which collapses
three different facts into one: the file is not there, the file cannot be read,
the file is empty. Only the middle is an instrument failure, and it was being
reported as the other two. Three further instances turned up in the same sitting:
`binary_fresh` reporting "go.mod does not declare `github.com/geoffmilleraz/fathom`"
against the repo whose go.mod declares exactly that on line 1; `guard_dead_scopes`
telling the reader to delete a working guard because the files it covers are not
INDEXED; and a router surface row (`internal/instrument*`, `internal/flywheel*`)
whose globs matched nothing and had silently stopped guarding the measurement code.

None was a coding mistake. Each was a check that had never been told it must
distinguish "false" from "could not look".

**Decision.**
1. *Coverage before conclusion.* Every read in the reporting layer goes through
   `internal/readset`, which records `read | absent | unreadable(errno) |
   skipped(category)`. Only `unreadable` makes a manifest incomplete;
   first-outcome-wins, so a retry cannot erase a recorded blindness.
2. *A count over an incomplete manifest is a lower bound and says so.*
   `≥ 85 (1 source(s) unreadable: docs/intent/invariants.md: permission denied)`
   — never a bare total. Blindness LEADS the NOT-COMPUTED list, because every
   other entry there is a bounded gap while this one silently lowers every
   number above it.
3. *No failure mode may make the repository look healthier than it is.* Each
   affected instrument carries a test that makes an input unreadable and
   asserts the output does not improve.
4. *This is not new doctrine.* IT-14/IT-15 decided it for `ingest` in v0.5.0,
   and `fathom verify` was already correct (an unreadable plan exits 2). The
   layer built last simply never got the same treatment. The vocabulary mirrors
   `fusion.ReadFailure` deliberately and does NOT import it: the ingest path's
   refusal semantics are its own, and the reporting layer wants the opposite —
   report honestly rather than refuse.

**Consequences.** The reporting layer does not refuse to run on partial coverage:
a `doctor` that dies because one file is unreadable is worse than one that reports
honestly, so the doctor rows keep their never-FAIL contract and go WARN with the
reason. Two new claim kinds: U13 (guard provenance) already existed; U14 names a
sensitive surface whose globs match nothing. Fixing the one U14 finding surfaced
nine genuinely unpinned rows in measurement code that U12 could not previously see.

**Alternatives rejected.** *Fail hard on any unreadable file* — wins if the
reporting layer were load-bearing for correctness; it is advisory, and hard
failure on a transient wedge trains users to disable it. *Log to stderr and carry
on* — the number in the report is what is read and believed; a warning elsewhere
does not change what the number claims.

**Revisit when.** After 60 days, no manifest has ever been incomplete in a real
run on any adopting repo (the discipline is theatre for a condition that does not
occur), or every incompleteness recorded had a single cause a narrower fix would
have covered. Measured by `incomplete_coverage` on the existing `intent-cli` and
`doctor` beats — no new channel.

## ADR-028: External agent memory is a peer process, never a dependency (2026-09-01)

**Status.** Accepted. **Type.** Dependency / strategy. Assessed subject:
OpenViking (`volcengine/OpenViking`, AGPL-3.0, Python/Rust, ~35k stars,
created 2026-01-05, v0.4.17.1 on 2026-08-31). Coexistence guide:
`docs/guides/openviking-side-by-side.md`.

**Context.** A ByteDance/Volcengine project has claimed the "agent memory"
territory at a scale fathom will not match: a context database that stores
memories, resources, and skills as a virtual filesystem (`viking://`),
retrieved by directory-recursive vector search over LLM-distilled L0/L1/L2
summaries, installed into Claude Code and Codex by one shell command. The
question put to this repo was whether to partner, borrow, or ignore it.

The overlap is narrow and real, and it is not the index. OpenViking's own
integration documentation is explicit that it stores conversational memory
and does not parse source, build symbol tables, or index call graphs — so
extraction, typed edges, blast radius, `verify`, and obligations are
untouched. What it does overlap is the continuity layer: threads, the
session brief, `memory_search`, the observation log. It occupies four of the
five Claude Code lifecycle hooks fathom already occupies (`SessionStart`,
`Stop`, `PreCompact`, `SessionEnd`) and injects up to 2,000 tokens per turn
at a 35% similarity threshold.

**Decision.**
1. *Never a dependency.* Not vendored, not linked, not imported, not
   embedded, and not a required component of any fathom surface. AGPL-3.0
   is the hard reason — this repo carries no root LICENSE, so its licensing
   options are all still open, and linking AGPL code would close them
   permanently in the one direction that cannot be undone. The soft reasons
   are sufficient on their own: fathom is a single signed Go binary with a
   deterministic index, and a Python server plus an embedding model plus an
   LLM provider is a new failure domain inside the determinism contract.
2. *Coexistence is permitted and documented.* Running OpenViking as a
   separate process alongside fathom creates no derivative work and no
   licensing exposure; AGPL constrains distribution and linking, not use.
   Adopters who want conversational recall may run both, and
   `docs/guides/openviking-side-by-side.md` says how.
3. *The authority boundary is the load-bearing clause.* Retrieved
   conversational memory may recall and may suggest; it may not assert a
   fact about code. Facts about code come from the index, with an anchor and
   a read-time freshness state. An unanchored, LLM-distilled memory asserted
   confidently into every prompt is precisely the failure the `suspect` tier
   exists to catch, and importing that failure would cost more than the
   recall is worth. Any future integration keeps fathom's claims
   authoritative over an external memory's.
4. *Not a partner.* There is no leverage and no shape for it. Offering the
   index as a `viking://` resource would make fathom a plugin in another
   project's context plane in exchange for nothing that is currently
   scarce.
5. *One idea is worth borrowing, and one is not.* The L0/L1/L2 tiering of
   `pack_context` (today flat, ranked by `Complexity+1`) is a genuine
   improvement — but `pack_context` is a near-zero-traffic surface in this
   repo's own call log, so demand comes before tiering, and the idea is
   recorded rather than built (`docs/vision/24-work-frontier.md` territory).
   The benchmark lesson is the one acted on now: see ADR-028's consequences.

**Consequences.** `docs/plans/bench-confidence.md` shipped in the same change:
per-trial repetition, paired per-task statistics, a machine-evaluable
criterion read from the committed corpus, a tri-state verdict where an
incomplete run is `indeterminate` rather than a win, and a JSONL ledger at
`docs/bench/ledger.jsonl` — created by the first run; as of 2026-09-02 it
holds one backfilled n=1 row (PB-2), no replicated row yet. The assessment's honest finding was that fathom's
benchmark *method* already exceeds the compared project's — pre-registration,
a published negative verdict, hand-verified ground truth — while its
*credibility* lagged on three counts fixed or named here: n=1 (fixed, floor
of 3 registered), nothing committed (fixed, the ledger), and no
third-party-reproducible corpus (named as BC-15, still open, and the real
remaining gap).

**Alternatives rejected.** *Adopt it for continuity and retire threads* —
would trade an anchored, falsifiable record for a similarity score, which is
the one trade this project exists to refuse. *Fork the Apache-2.0 `ov` CLI
subtree* — the carve-out covers the client, not the engine, so it buys a
protocol shim and inherits the maintenance. *Ignore it entirely* — the
benchmark lesson was real and the coexistence question will be asked again
by anyone who reads a star count.

**Revisit when.** OpenViking (or an equivalent) relicenses permissively, or
ships code indexing that overlaps extraction rather than continuity; or an
adopter runs both and reports a concrete conflict the guide does not cover;
or `pack_context` traffic rises enough for tiering to have a consumer; or
fathom adopts a root LICENSE, at which point clause 1's hard reason should
be restated in terms of that licence rather than the absence of one.

## ADR-029: Harness adapters are data behind one seam; a second harness's inject tier ships unverified (2026-09-02)

**Status.** Accepted under the delegated ownership of 2026-07-07 and the
owner's 2026-09-01 directive to decide, spec and implement; the owner may
reopen by editing this status. **Type.** Architecture / strategy. Design:
`docs/vision/25-harness-port.md`. Plans: `docs/plans/harness-port.md`,
`docs/plans/reach-parity.md`. Amends the sequencing rule of
`docs/vision/04-coding-agents.md` §5 and `docs/vision/24-work-frontier.md` §6.

**Context.** The owner asked for a hexagonal capability with adapters for the
major coding harnesses, kept DRY. A 14-agent evaluation (2026-09-01) found
that the harness extension surface collapsed onto four open standards
(`AGENTS.md`, Agent Skills, Agent Plugins 1.0, MCP over HTTP) plus Claude
Code's hook JSON as the de-facto wire format every other harness shims to;
that fathom's coupling is one hook table, four decode sites, four encode
sites, one `claude|codex` switch and nine files of path literals; and that
Codex now carries eleven Claude-shaped lifecycle events, which falsifies the
`domain.md` assumption "Claude Code and Codex … hook events keep their
shapes". The previous rule — pull-tier only for a second harness until the
Claude numbers are green — rested on a Codex without hooks and on the cost
of a second implementation. Neither premise holds.

**Decision.**
1. *One seam, in fathom.* `internal/harness` holds a descriptor registry
   (data: identity, detection, instruction files, hook files and event
   bindings with stdin/stdout field maps, `failMode`, scope-merge rule,
   skills dirs, MCP target, env contract, transcript availability,
   `verified_against`) and one codec per harness. Claude Code's registry
   entry reproduces `fathomHookSpecs` byte-identically; the existing goldens
   are the proof and any changed golden byte kills the change.
2. *Additive surface.* `--harness <id>` (default `claude-code`) on the hook
   verbs; `fathom init --harness=claude-code|codex|agents|none`;
   `fathom harness list|capabilities --json`; `schema/harness/schema.cue`
   validates the exported descriptor. Detection lives in init and doctor
   only. The compatibility surface gains these and changes nothing existing.
3. *Writes go through the wiring engine.* A hook or MCP file whose shape
   differs from Claude Code's settings gets a `wiring.KeyKind` with
   plan/apply/undo and a receipt (law 15, I13).
4. *Honesty by construction.* Every harness's capability table is generated
   from the registry and pinned by test; `docs/adoption/codex.md` §6 becomes
   a rendering of it; an absent vehicle prints as absent; an unverified
   descriptor prints every row as unverified and doctor never guesses a PASS.
5. *The sequencing rule is amended narrowly.* A second harness's inject tier
   may ship as descriptor + codec + fixtures marked `unverified`, written by
   `init --harness=<id>` only on that explicit flag (repo-scoped files only;
   user-global configuration stays ask-first), with no measurement claim. A
   codec is written only for a harness with a documented Claude-shaped
   contract and a local install to verify against (P20); other harnesses
   ship as data rows carrying their documented field maps. Scope G2 is untouched: no Claude instrument is repointed, no
   savings number is admissible for any harness, and only doctor's
   `hooks_firing` on a real install turns `unverified` into a dated
   `verified_against`.
6. *Measurement stays Claude-only.* `Descriptor.Transcript == ""` makes the
   session-ledger capability absent; law 12 forbids extrapolation. No
   `TranscriptReader` port until a second harness exposes transcripts.
7. *The kit does not get the port yet.* agentic-kit carries three hand-rolled
   harness touchpoints; a kit package lands when a second consumer deletes
   its copy onto the contract (D8). Ask FA-22 is filed with that trigger.

**Consequences.** Adapters become configuration; `fleet reconcile` and
`fleet undo` cover every new harness for free; the degradation ladder
(A hooks + inject, B block-only, C JS-only, D none) is a first-class output
and tiers C/D are documented rows, not code. `domain.md`'s assumption is
restated; `scope.md`'s Harnesses boundary widens only as descriptors are
verified. A descriptor still `unverified` 60 days after its codec lands is
demoted to tier D and its `init --harness` write refuses with the reason.

**Alternatives rejected.** *A hooks framework in agentic-kit* — single
consumer, "not a framework", ADR-022/023 precedent. *A neutral hook schema*
— Claude Code's envelope is the interchange; a third shape adds a
translation. *A marketplace* — F2 unpromoted; two manifests over one tree
reach seven clients. *JS plugin hosts for OpenCode/Amp/Cline* — no-Node
non-goal. *An ACP agent* — fathom is not an agent; MCP passthrough reaches
ACP editors with zero code. *Waiting for G2 before any second harness* —
the premise (a second implementation) is gone; only the measurement claim
is still gated, and this record keeps it gated.

**Revisit when.** A second consumer imports the descriptor contract (FA-22
fires); a harness exposes transcripts (the measurement clause); Claude Code
stops being the interchange envelope; a descriptor reaches 60 days
unverified; or `hooks_firing` verifies Codex on a real install (widen the
Harnesses boundary, then revisit whether the Codex lane earns an instrument).

## ADR-030: A lexical oracle proves non-code plan rows (2026-09-02)

**Status.** Accepted. **Type.** Process / architecture. Design:
`docs/vision/26-proof-credibility.md` §1.3. Plan: `docs/plans/proof-credibility.md`.
Compatibility surface: plan grammar v2, additive.

**Context.** Eighteen of fathom's claimed-unproven rows are `[x]` rows with a
`skip_test` reason and no symbol — docs, measurements, changelog lines,
operator actions — and the U2 remedy text promised a `skip:` reason lifts
the debt while the counter never honored it. The verify ledger shows an
external repo's plans at 9 claimed / 0 verified and 37 / 12: the second
puller for a deterministic non-code contract exists in evidence, not
assertion. The tokenizer already accepts the ADR-026 oracle vocabulary on
plan lines.

**Decision.** A plan row may carry `{oracle: lexical, path: <repo path>,
pattern: <regex>}`. `fathom verify` checks the path exists at HEAD, is
non-empty, and the pattern matches at least once; the row verifies and
leaves U2. `LintPlan` refuses a Markdown path without a `pattern` (existence
proves nothing about a claim). The evaluator is ADR-026's, not a second
one; the key set stays owned where ADR-026 put it (Q20 named, not decided).
The U2 remedy text is corrected to say what the counter does.

**Consequences.** `schema/plan/schema.cue` and its valid/invalid testdata
gain the keys; `docs/plans/README.md` gains the row; `CHANGELOG.md` gains
the line; scope G6 gets a real path. Kill: if more than 30% of lexical rows
in the fleet point under `docs/vision` or `docs/design`, the lint tightens
to measurements, changelog, tests and specs.

**Alternatives rejected.** *Keep `skip_test` as the escape* — it trains the
reflex T8 warns about and the counter cannot honor it without lying. *A
separate `proof:` key* — a parallel grammar to ADR-026's oracles is the
drift O4 exists to catch. *An LLM judge for prose rows* — no LLM in the gate
(scope non-goal).

**Revisit when.** Q20 moves the grammar to the kit; the kill fires; or a
second evaluator appears.

## ADR-031: Bench credibility — derived third-party truth, a memorisation floor, a precision term, and statistics owned by the kit (2026-09-02)

**Status.** Accepted. **Type.** Process / strategy. Design:
`docs/vision/26-proof-credibility.md` §2. Plan: `docs/plans/bench-public.md`.
Extends ADR-028's consequences (BC-14/15/16).

**Context.** The bench method exceeds the field; its credibility does not:
every ground truth is derived from this private repo, the home corpus is
false at HEAD, the ledger ADR-028 calls committed has no rows, the open
bench rows already pass the verify gate on pre-existing anchors, and the
runner hard-codes its subject. The field converged on gold-patch-derived
localization truth (Loc-Bench, SWE-PolyBench, SWE-Explore, Multi-SWE-bench)
that needs no LLM and no execution to derive, on a no-tools arm as the
memorisation floor, on a precision term for set-valued grading, and on
paired task-clustered statistics with effect sizes.

**Decision.**
1. *Truth at HEAD before spend.* A committed corpus re-sweep revision, the
   `29` substring hole fixed, `machine_criterion` untouched; the ledger
   wording corrected; BC-14/15/16 re-anchored to symbols that do not exist
   until the work does.
2. *The corpus is derived, never authored.* `contextbenchscore import`
   derives Go localization tasks from two pinned public datasets with no LLM
   — SWE-Explore's Go subset (line-level truth an outsider built) and
   Multi-SWE-bench (CC0 gold patches for file, function and callers truth) —
   under a pre-registered selection rule, with per-task derivation and
   `requires`; the derivation reproduces bit-for-bit from a clean clone. The
   runner ingests each instance's base commit into a scratch store, never
   the fleet store and never the post-fix tree.
3. *Method additions, pre-registered.* A `none` arm and a committed leak
   rule; a fenced answer block with `max_predicted` and precision/recall,
   registered as a new criterion H2-2 because it changes the success
   definition after H2-1 has data (H2-1's verdicts stand as history, T5);
   seeded task-clustered paired bootstrap CI, exact Wilcoxon, A12, the
   unbiased `pass@k` and `pass^k` — reported, never gating.
4. *Statistics live in agentic-kit.* `foundations/quality/reliability-stats`
   is consumed by fathom's bench and the kit's eval-runner: fathom hand-built
   paired statistics twice, so the kit owns them.
5. *Regression is free; runs are not.* `contextbenchscore regress` mirrors
   the kit's eval-runner `Regression` semantics and joins `validate` only
   after two same-corpus rows exist. Runs stay operator-initiated
   (autonomy: ask before LLM spend).

**Consequences.** `LedgerRow` gains `per_task`, `corpus_sha256`, `model_id`,
`cost_usd`, `dirty_tree` (additive). The first replicated home run and the
first public-corpus run are operator actions with their cost stated in the
plan. Publishing a third-party-reproducible number still needs the binary
in a stranger's hands (Q4, F2) — built now, published on that decision.
The follow-on is registering the fathom-equipped harness as an explorer in
SWE-Explore's Go subset, the one leaderboard whose truth an outsider built —
a registry listing, so it waits on F2.

**Alternatives rejected.** *Hand-author a public corpus* — permissively
licensed derived corpora exist; authoring is the self-derived-truth hazard
again. *Adopt an execution harness* — localization needs none. *Keep stats
in fathom* — twice-built. *Automate the run* — real spend; refused in
`bench-confidence.md` and here.

**Revisit when.** The none arm passes more than half the home corpus (the
corpus, not the tool, is the finding); a public number is requested before
the repo is public; SWE-Explore accepts the registration; or the kit's
stats package gains a third consumer.

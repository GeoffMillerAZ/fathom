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

**Status.** Accepted.

**Decision.** Strictly separate three layers:

1. **Connector SPI** — abstract source (`Discover`/`Fetch`); produces `SourceRef`s.
2. **Source Fusion Pipeline** — `SourceRef → bytes → extractors → Observation` records in the store; hash-gating, parallel extraction, lazy mode.
3. **Knowledge Query Layer** — `store → typed results` per query/tool; concern-scoped filtering, staleness assessment, optional hybrid semantic search.

**Why.** Each layer is independently testable and replaceable. The query layer
never touches a network; the pipeline never knows where bytes came from.

## ADR-002 — Connector SPI with a deterministic Faux connector

**Status.** Accepted.

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

**Status.** Accepted.

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

**Status.** Accepted, amended 2026-08-26.

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

**Status.** Accepted.

**Decision.** Observations carry an **evidence tier** from agentic-kit's
`foundations/review/scoring` foundation: `verified` (code, ceiling 5),
`pre-scan` (4), `doc-claimed` (2), `sme-stated` (2), `assumed` (1). The query
engine respects ceilings when scoring — a doc claim never outranks a code fact.

**Why.** fathom is a downstream consumer of the agentic-kit platform; evidence
governance is exactly what that platform provides. We import it rather than
reinvent. See ADR-009.

## ADR-008 — Staleness is always reported

**Status.** Accepted.

**Decision.** Every query result includes a staleness assessment (which sources,
when last ingested, whether re-ingest is recommended).

**Why.** Callers must distinguish "data is fresh" from "re-ingest before trusting
this." Silent staleness is a correctness trap for AI consumers.

## ADR-009 — Depend on agentic-kit as the platform substrate

**Status.** Accepted.

**Decision.** fathom imports `github.com/geoffmilleraz/agentic-kit` (resolved
locally via `go.work` + a `replace` directive during co-development). Reuse:
`review/scoring` (evidence tiers), `evidence/*` (observation/contradiction
shapes), `telemetry`, `llmclient` (knowledge-card enrichment), `security`
(token redaction / DLP posture), `contracts`.

**Why.** Avoids duplicating governed-evidence primitives; keeps the two projects
coherent. Changes to agentic-kit are made there when a shared primitive needs to
evolve.

## ADR-010 — MCP server: stdio + HTTP, tool groups, profiles

**Status.** Accepted, amended by ADR-014.

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

**Status.** Accepted, amended 2026-07-16, 2026-08-26 and 2026-09-27.

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

**Status.** Accepted.

**Decision.** No organization-specific names, hosts, catalogs, or product
references anywhere in code, docs, tests, or fixtures. Anything org-specific is
configuration. Mechanically enforced (`cmd/fathom/orgnames_test.go`'s
`TestNoOrgSpecificNames`, invariant I19): a host under `*.corp`, `*.internal`,
`*.intranet`, or `*.lan`, a non-neutral email domain, a real cloud
account/ARN, or a non-neutral storage bucket URI all fail the gate. The
`testdata/fauxtown/` fixtures' invented, neutral names are what a name must
be instead of an org's own — not a description of what the gate checks, so
an otherwise-plausible invented host still fails if it lands under one of
those four TLDs (O57 item 7).

**Why.** The project is intended to be open-sourced and adopted by any org.

## ADR-012 — Test-driven development with golden fixtures

**Status.** Accepted.

**Decision.** Build TDD: write the failing test (often against `FauxConnector` /
`testdata/fauxtown/`) before the implementation; extractor and query outputs are
asserted with golden files; determinism is an explicit, tested property.

**Why.** A code-intelligence engine is only trustworthy if its outputs are exact
and reproducible. Tests encode that contract.

## ADR-013 — Tree-sitter via CGO bindings, superseding ADR-004's wazero mechanism

**Status.** Superseded by ADR-066 (2026-10-10): the last CGO binding left `go.mod` with LB-F19; see ADR-066.

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

**Status.** Accepted.

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

**Status.** Accepted, amended 2026-07-16.

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

**Amendment (2026-09-24, Q21): the measured obligation ceiling, ratified.**
*Status: accepted* (proposed 2026-09-02; the owner delegated the decision
2026-09-24). This ADR's 150-token serving ceiling was set before the
channel's cost was measured. HF-3 measured it
(`docs/measurements/2026-08-27-obligation-ceiling.md`, 2026-07-30 →
2026-08-26, one machine, one adopter, `tokens_estimate` = bytes/4, an
ESTIMATE): `post-edit` payloads n=1,918, p50 131, p90 315, p95 329, max 378
tokens — 897 (47%) over 150; `pre-edit-guard` n=370, p50 185, p90 198, p95
222, max 380 — 308 (83%) over 150. A second read of the same log
(2026-09-14 → 2026-09-24, mixed organic and A/B-bench traffic, a different
population) went the same way, milder: post-edit p90 241, p95 273, max 932
(25% over 150, 2% over 320); pre-edit-guard p90 204, p95 267, max 319 (36%
over 150, none over 320).
**Decision.** Raise the whole-payload serving ceiling from 150 to 320 tokens
(HF-3's post-edit p90) — `specLineTokenBudget` in `cmd/fathom/edithook.go`,
pinned to this paragraph by `TestSpecLineTokenBudget_MatchesADR016`. The
constant gates only what is appended after the rules (spec-line refs,
anchored memories, findings); the rules block stays bounded by
`MaxObligations` (6) and the ≤28-word compile budget. It costs nothing on
the median edit (p50 62–131 tokens in both windows) and nothing on payloads
already over 320; it only lets payloads in the 150–320 band carry content
that was silently skipped before, at most 170 tokens more. The tail over
320 (HF-3's pre-edit max 380; the second window's 72 post-edit payloads up
to 932) is named here, not absorbed: any future truncation must disclose
the dropped count (P3).
**Not resolved by this amendment.** The guards pilots and the
decision-port A/B (`docs/bench/readings/decision-port-ab-2026-09-23.md`)
show the task's governing rule missing from the served set regardless of
ranking or headroom. That is a selection problem, not a budget problem; it stays open
as `docs/plans/verbs.md` VB-40 (carried by honesty-fixes HF-4 until 2026-09-25), and
`rank_within_tier = false` (ADR-042 amendment, 2026-09-23) is a mitigation, not a fix.
**Revisit when** a single-adopter window of 14 days or more without bench
traffic moves post-edit p90 more than 15% from 320, or the selection work
(`docs/plans/verbs.md` VB-40) lands (a better selector may let the ceiling come back down).

## ADR-017 — Born-portable stores: repo-relative anchors, domain-neutral freshness

**Status.** Accepted.

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

**Status.** Accepted; pilot KEEP 2026-08-26; amended 2026-09-28 (done means proven AND wired); note 2026-09-29 (the 41 waivers re-audited; the pre-push gate reads the pushed range).

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

**Amendment (2026-09-28): done means proven AND wired.** The verify gate is this record's
"done means proven", and it failed: it accepted claimed rows whose anchored function a named
test reached while no program ever ran it. A hand audit (`deadcode ./cmd/...`, commit
`0bcb04ac`) re-opened a first batch; the gate below found 105 more across 41 plans.
Decisions:
1. `fathom verify` (store and daemon alike) computes, over the same syntax+typed call graph
   `test_reaches` walks, whether a **product entry point** reaches a claimed row's function:
   every `func main` of a Go `package main` outside `testdata/` (bench binaries are products),
   and the `init` functions and package-level variable initializers of every package those
   link. Registered MCP tool handlers, CLI verbs, console routes and hook entries need no rule
   of their own: a handler registered by reference is a func-value edge (I23) from the binary
   that registers it. The typed pass keys a non-test `init()` and a non-test file's variable
   initializers to `<pkg>.init` (`TypedEdgeEpoch` 3), and `callgraph.AddTypedEdges` adds that
   one vertex when no `init()` was indexed — the only vertex it ever invents.
2. A function a test reaches and no entry point does is **`library-only`**: Unknown, so the
   default gate reports it and `--strict` (the pre-push gate) refuses it.
3. **`library: true`** waives it only with the row's `reason:` naming why the API itself is
   the deliverable — a split's library half naming its open wiring row, a foundation row whose
   open rows wire it, a gate harness whose consumer is a `go test` gate by design — and only
   when it predates the claim: O56's self-skip comparison applies to it, and `--since` reports
   it self-declared (I45).
4. Wiring is **n/a, named**, for a type, a non-Go symbol, test code, and a service with no
   binary at all (a library has no product to wire into). A row whose name resolves to several
   functions (one method on two implementations) is wired when any of them is.
5. The reach set is one O(V+E) walk per (store, service, watermark, graph kind), a memo group
   (`product_reach`, I51) dropped with the graph it was walked over.
**Rejected:** running `x/tools` deadcode (RTA) inside verify — it builds every binary, and it
never reports a package no binary links, which is where most of the 105 lived; treating a
signature (`http.ResponseWriter`, a cobra `RunE`) as an entry point — it wires a handler
nobody registers; a Fail in the default gate — library-only is a missing fact about the
product, the same strictness split `no-contract` already has. **Jev:** the decision port has
no role — wiring is a deterministic graph fact, and a model judging "is the API the
deliverable" would be the self-grading decision 3 refuses. **Consequences:** the 105 rows were
re-anchored on the function the product runs when the anchor was a test-only wrapper of it
(35), declared `library: true` (41: 11 split halves, 13 foundation rows with open wiring rows,
17 gate harnesses), or re-opened with a note (29).
**Revisit when.** (1) A claimed row reads `library-only` while the product demonstrably runs
its function (reflection, a build tag, cgo, a plugin) — fix the extractor's edges, never widen
the entry points. (2) `library: true` rows exceed a tenth of a plan's claimed symbol rows —
the waiver is becoming the default; require a wiring row per library row. (3) A non-Go service
with binaries adopts verify — its entry points need a rule of their own.

**Note (2026-09-29): the 41 waivers, re-audited; the pre-push gate reads the pushed range.** An
independent re-audit reviewed the 41 `library: true` waivers this amendment declared. Its check was
tip-only (does each waiver name why the API is the deliverable?), and it holds: they are gate-harness
test deliverables whose consumer is a `go test` gate by design, and split library halves, each naming
its open wiring row. The same audit showed the gap: `fathom verify --since 8a165059` reads all 41 as
self-declared, because each entered in the range that claimed its row, and the pre-push gate did not
read ranges, only O56's one-commit comparison. From push 79 the pre-push verify step runs
`--strict --since $(git merge-base HEAD origin/main)` (`lefthook.yml`, I17), so a waiver added
anywhere in the pushed range is self-declared and refused (I45), with or without a later commit
between it and the claim; with no merge-base the step verifies without `--since` and says so, and
exit 2 still fails open. It is not retroactive: the 41 are on main, so they predate every later
range. Plan rows: `docs/plans/efficiency-wave.md` EW-14 (this gate) and EW-15 (wired-check
hardening from the same audit). Amended by EW-16: `<base>` is the remote sha lefthook passes
on stdin (`use_stdin: true`), the ref the push replaces, so a stale local `origin/main` neither
blocks a legitimate push nor widens the range; a new branch or a sha this clone lacks falls back to
the merge-base and says so. Proof: `TestLefthook_PrePushVerifyReadsTheChangeRange` runs the
step's own body against a stub binary; `TestCmdVerify_StrictSince_LibraryWaiverInsideTheRangeFails`
shows a waiver committed before its claim, inside one range, passing without `--since` and failing
with it.

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

**Amendment (2026-09-27, ADR-053).** The surface reckoning shrinks the served agent
surface (25 tools: this record's base allowlist plus ADR-035's promoted verbs and
`safe_delete_check`) to a default of eight — `find_symbol`, `search`, `read_source`,
`symbol_context`, `obligations`, `verify_plan`, `blast_radius`, `review_diff`. A new
opt-in surface, `extended` (`POST /rpc/extended`, 21 tools), holds what survives beyond
the default; `/rpc/companion` stops aliasing `agent` and aliases `extended` instead, so a
client pinned to it keeps its prior tool set minus the reckoning's deletions.
Execute-and-flag (the 2026-07-16 amendment above) is unchanged — an off-surface call
still executes and logs `off_surface:true` — so SR-M3 can read fourteen days of
off-surface demand before anything returns to the default.

## ADR-019 — In-tree mirror of agentic-kit for kit-free distribution

**Status.** Accepted, amended 2026-09-01; amended 2026-10-16 by ADR-070 (the mirrors are packages of fathom's module by import-path rewrite: no kit require or replace in `go.mod`, `.vendor-commit` is the pin, vendor-check compares after reversing the rewrite).

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

**Status.** Accepted (2026-07-22).

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

**Status.** Accepted (2026-07-22).

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

**Addendum: Kotlin and Java as syntax-only languages (wave P2, 2026-09-17).**
The Long Code Arena bench run (`docs/bench/readings/lca-run-1-reading.md`)
had 12 Kotlin and 8 Java tasks of 40; wave P1 (O69) disclosed Kotlin's
missing extractor but left the question open for Java. Investigated and
resolved: **Java was already extracted**, not merely classified —
`extractor.NewJavaExtractor()` (CGO tree-sitter bindings, ADR-013) has been
registered in `fusion.New` since before this wave, covering classes,
interfaces, enums, records, and methods; wave P2 closed its remaining gap
(fields, imports — `KindJavaField`/`KindJavaImport`) without moving it off
CGO — that migration is XT-6b, a separate, not-yet-triggered slice this wave
does not pull forward. **Kotlin had no extractor at all** and is genuinely
new: `extractor.NewTSExtractor("kotlin")` (§4's wazero/WASM engine) is now
registered for `.kt`/`.kts`, backed by `internal/extractor/tsengine/
grammars/kotlin.wasm` — fwcd/tree-sitter-kotlin (not an official
tree-sitter-org grammar) pinned at tag 0.3.8, commit
`e1a2d5ad1f61f5740677183cd4125bb071cd2f30`, built by the same zig-cc/
wasm-opt recipe as the original four blobs (sha256 in
`internal/extractor/tsengine/grammars/PROVENANCE.md`). Unlike Python/JS/
Ruby, Kotlin has no CGO incumbent to calibrate against — §4's "new-language
breadth" clause applies directly, not the calibration-gate clause. Proven
live on two real subjects (shallow clones, deleted after): bumble-tech/appyx
(Kotlin, 549 files) — 6,725 symbols where the O69 finding was ~165; jhy/jsoup
(Java, 313 files) — 7,559 symbols including 806 fields and 1,912 imports
that predate this wave's Java coverage fix. Full account: `.fathom/
observations.md` O79.

## ADR-022: MCP server stays bespoke, not `kits/mcp-server` (2026-07-23)

**Status.** Accepted (2026-07-23).

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

**Status.** Accepted (2026-07-23).

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
point" — FA-12's second-consumer trigger (`docs/intent/futures.md` F12). **Fired 2026-09-25 by ADR-046 (the assess lanes). Owner's disposition: evaluation stays bespoke; `qa-harness` is evaluated as the assess lanes' corpus and baseline once M-A1's adjudicated sample exists, because before it there is nothing for the harness to replay or gate against.**

**Amendment (2026-10-10, ADR-067).** "Explicitly STATIC, never asserts `runtime_pass`" is narrowed to what static
protects: `fathom verify` never executes a test or a command. It reads attested evidence. `runtime_pass` reads the
newest attested run in `test_runs` for a row's certifying tests (TC-29): Pass or Fail with the run's commit and age,
Unknown when none exists. It stays informational and never changes a row's status. Invariant I8 is amended with it,
and the deferred `cmd` oracle D0-1 is retired as superseded.

## ADR-024: Pack taxonomy adopted as vocabulary, not machinery (2026-07-23)

**Status.** Accepted (2026-07-23), amended phase O (2026-07-29).

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

**Trigger (1) fired 2026-10-09 (owner decision D11).** A named consumer (fathom, which already pins app-kit and agentic-kit and adopts env-kit) and two concrete providers (app-kit, env-kit) exist. The provider stratum is renamed `tech` (the word collided with decision providers and infrastructure providers) and promoted by ADR-061 (proposed): a tech pack is deterministic data read from the repository's own pinned copy, so the upstream watermark this record parked is not needed (freshness is the pin). A cousin of trigger (3)'s anti-pattern, a kit's content copied into `.fathom/`, has a measured instance in this repository (`docs/design/fathom-dir.md` §3 a); ADR-061 answers it with the pin, not a module.

## ADR-025: DuckDB backend deleted outright — SS-8 executed (2026-07-23)

**Status.** Accepted (2026-07-23).

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

**Amended 2026-09-25 (custodian DP-3, DP-4): the doctrine plane's two computed views,
and decision 1's "never reads `docs/vision/`" narrowed to the corpus.** Vision 23 §2
accepted the vision promotion pipeline and goals-as-bets before this record, and the
custodian plan kept both rows open after it; decision 1's clause, read literally, forbade
the first. Its reason was compiling aspiration into rules. So: (1) the intent *corpus*
(conditions, invariants, surfaces, the audit and every U-kind) still never reads
`docs/vision/`; (2) `fathom intent doctrine` reads the vision series the router's docs
map names for its *promotion metadata only* — a header `Status:` line or a status the
index entry leads with, condition labels its author wrote, plan and decision pointers in
its header or index entry (plans resolved in `docs/plans` or `docs/plans/archive`, rows
counted by the plan parser, claimed not proven), relative markdown links, and dates —
and reports per vision a computed stage (`landed` only from the rows of the plans it
declares as its own, never from a status word or a plan it merely cites), broken links,
an undated or unstated promise, and a for-review promise with no plan past 30 days;
nothing is compiled, evaluated, edited or scored; (3) a scope goal is a pre-registered
bet — a `{bet, instrument, threshold, review}` block on its `Done when:`, tokenized by
the plan tokenizer, the instrument resolved to a registered `fathom` verb, a Taskfile
target or a repo script — or it is refused at load, and a passed review date is listed
as due for a human; a bet-only block never counts as an oracle annotation, so K0 cannot
be inflated by the surface that enumerates bets, and one outside a goal is named as
misplaced; (4) one doctor row, `doctrine`, warns and never fails; no MCP tool, no hook,
no injected channel. **Revisit when.** A vision finding is
reported as a rule, an obligation, or an unproven claim anywhere (the narrowing has
leaked); bet blocks are mistaken for oracle annotations in K0's count; or the kit's
`intent-prune`/`intent-decide` drop a bet block (the Q18 class).

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

2026-09-02: `internal/readset` deleted onto agentic-kit's
`foundations/telemetry/read-manifest` (FB-8) — the kit generalised the package
from it, so fathom keeps only the rendering adapter `internal/readcoverage`
and one shape of "what could I not see" survives.

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

**Amendment (2026-09-27, ADR-052).** The revisit trigger fired: fathom adopted FSL-1.1-ALv2.
Clause 1's hard reason is restated: AGPL-3.0 code cannot be vendored, linked or embedded
because its copyleft would require the combined work to be distributed under AGPL, which is
incompatible with FSL's Competing Use restriction. The soft reasons and clause 2 stand.

## ADR-029: Harness adapters are data behind one seam; a second harness's inject tier ships unverified (2026-09-02)

**Status.** Accepted under the delegated ownership of 2026-07-07 and the
owner's 2026-09-01 directive to decide, spec and implement; the owner may
reopen by editing this status. **Type.** Architecture / strategy. Design:
`docs/vision/25-harness-port.md`. Plans: `docs/plans/archive/harness-port.md`,
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

**Amendment (2026-09-28, PE-14).** The measurement clause fires in part: a Codex rollout
(`~/.codex/sessions`) carries cumulative token usage and every tool call, in one schema read
unchanged across codex-cli 0.45.0 to 0.154.x (327 files, read-only), and `harness.CodexSessionCost`
reads it for outcome economics (`docs/design/progress-and-economics.md` §5). It is an instrument
for cost only: the Codex `session-ledger` vehicle stays absent, no ON/OFF arm exists, and G2's rule
stands (no savings number is admissible for any harness). The descriptor note that called the
rollout format "not stable" is corrected; the rest of the record stands.

**Amended 2026-10-16 (ADR-072): a committed, self-hosted marketplace manifest.** The rejected alternative *A
marketplace* now covers a catalog listing only. The repository commits the manifest `fathom plugin render
--marketplace` writes, with the rendered plugin trees it names, so a reader can install from the repository itself;
a catalog listing, the brew tap and the MCP registry stay with F2 and RH-X1. The rest of the record stands.
**Revisit when** ADR-072's triggers fire.

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

## ADR-032: The bench pyramid — measure the index for free, spend on the agent only where it can change, and remember every cell by its key (2026-09-12)

**Status.** Accepted (delegated ownership 2026-07-07; owner directive 2026-09-12: make
the bench show fathom's strengths and keep the loop token-efficient). **Type.**
Process / architecture. Design: `docs/design/bench-pyramid.md` (proposed 2026-09-03 from
the stopped replicated run, O23–O26). Plan: `docs/plans/bench-pyramid.md`. Extends
ADR-031; H2-1's recorded verdicts stand as history.

**Context.** The first replicated home run (`run-20260902T211103Z`, stopped at 17 of 117
cells, ~1.5M tokens) produced four findings in nine minutes: the obligations tool served
six plan-row spec-bindings and displaced the one governing rule, and the fathom arm
passed the task anyway by reading the file grep's way — task-level success hid tool-level
failure (O23); Claude Code 2.1.x defers MCP tool schemas, so every fathom cell now opens
with one or two `ToolSearch` turns the grep arm never pays (O24); a ground truth sat
verbatim in the auto-loaded `CLAUDE.md`, discovered only by spending six tool-arm cells
(O25); and `total_tokens` is cache-read × turns — a turn counter in a token costume,
under which the payload fathom shrinks is invisible (O26). The single expensive bench
measured agent × harness × index with the harness term dominant, could test only what
grep can also do, sat near ceiling on thirteen tasks, and cost enough to run quarterly.

**Decision.**
1. *Six layers, one question each, cheapest first.* L0 index oracle (`cmd/loctruth`
   over the fixture fleet, precision and recall goldens), L1 payload replay (golden
   bytes, count and latency per read-only tool against a pinned store), L2 corpus gate
   (truth at HEAD, no gold token in any auto-loaded file, no prompt naming the governing
   file) run in `task validate` at zero LLM cost. L3 targeted agent runs re-run only the
   fathom arm for tasks whose `exercises` touch the tool handlers a diff changed. L4 is
   the full three-arm run at n≥3, on a release tag or a surface change. L5 is the ambient
   session instrument (verify-gate catches, the A/B arm).
2. *Every cell is remembered by its key.* A scored cell carries
   `cell_key = sha256(corpus_sha256 · task_id · arm · model_id · harness_version ·
   scoring · prompt_subject · arm_surface)`, where `arm_surface` is the fathom binary
   commit for the fathom arm and empty for grep and none. Cells live content-addressed
   under `workspace/artifacts/context-bench/cells/`; a run borrows any cell whose key
   matches, records `borrowed` with the source run id, and the evaluator accepts a
   borrowed cell only by key equality — a changed invariant produces a different key,
   never a silent reuse. Control-arm cells therefore survive every fathom change; a
   fathom change re-runs the cells its diff can move.
3. *H2-3 is the criterion the next run is scored on.* Same success bar as H2-1; the cost
   half reads `payload_bytes` (tool-result bytes returned to the model) and `cost_usd`;
   `toolsearch_turns` and `turns_excl_toolsearch` are reported as named lines so the
   harness tax is visible, never hidden; `gold_source_tool` names the tool whose result
   first carried a gold token, so a tool passes only when it is the source. H2-1 is
   never re-scored.
4. *The bench's product findings are fixed before the next spend.* Fresh rules rank
   before stale ones; layers with the role `governs` (obligations, adr-rules) rank
   before layers with the role `claims` (spec-bindings) and the latter fill only the
   budget the former leave; a spec-binding claim is scoped at serve time by the plan
   row's `{package, symbol}` contract parsed from its citation, and a row with no
   contract binds to nothing; the obligations tool inlines each rule's cited lines under
   a cap so the agent stops round-tripping `read_source`.
5. *The deferral tax is a fathom cost.* The agent-surface schema budget is held under the
   harness's deferral threshold when the threshold is documented, else measured per
   release; `toolsearch_turns` on the exploration ledger is the tripwire.

**Consequences.** `docs/bench/README.md` gains the pyramid and a strengths map (which
layer shows which of fathom's differentiated capabilities: done-means-proven via L5
catches, edit-time obligations via `gold_source_tool`, callers and blast-radius truth via
L0, discovery cost via `payload_bytes`). `task validate` grows by L0–L2 (seconds).
Scope G9 is restated in these terms. The next full run is one spend scored under H2-1
(history) and H2-3 offline from the same transcripts.

**Alternatives rejected.** *Run the same bench harder* — the four findings are structural.
*Drop the control arm* — L4 is the only layer that answers "versus grep" and stays.
*Hash by tree instead of key* — a tree hash invalidates control cells on every docs
commit; the key names the invariants that actually bind a cell.

**Amended 2026-09-12, same day, from the first L0 self-run.** The oracle found
`blast_radius` at precision 0.075 (heuristic edges over-connect) and ~10 s per query, and
`find_callers` at 0.65; the fix was resolved edges by default with heuristic edges opt-in
(`include_heuristic`), a watermark-keyed call-graph memo (I21 amended), and a repaired
bare-name query path — find_callers now 1.0/1.0 and blast_radius 1.0/0.98 on this repo. The
0.95 floor binds the tools the type checker can adjudicate (callers, the resolved blast
walk); `references` is scored as a LOWER BOUND (comment and string-literal mentions cannot be
told from identifier uses by a text search) and is reported, not gated. The same day's
daemon log showed the typed-edge pass failing under launchd (no HOME/GOPATH), serving
syntax-only edges with no doctor row saying so — a `typed_edges` row and an absolute Go
environment for the daemon close it (BP-35..37). The deferral tax (O24) did not reproduce on
Claude Code 2.1.270 (zero ToolSearch turns at 28.7k schema bytes); BP-22 is refused and the
`toolsearch_turns` tripwire stands.

**Amended 2026-09-13 (the run).** The first replicated home run (run-20260913T002832Z,
three arms, n=3, $7.90) is on the ledger under H2-1 and H2-3 and FAILS both on cost: 31/33
successes to grep's 28/33 (p=0.042, the CI crosses zero), tokens 1.444×, payload 1.901×,
cost 1.465×; two tasks leaked to the none arm and left the verdict (O31). §5 is corrected:
the deferral tax DID reproduce — every headless fathom cell paid at least one ToolSearch
turn, 11% of the arm's tokens (O32); the 2026-09-12 zero was a probe that never called a
fathom tool. Claude Code documents no size threshold: with `ENABLE_TOOL_SEARCH` unset every
MCP schema is deferred and only `alwaysLoad: true` exempts a server, so the remedy is that
bit on every entry fathom writes (BP-39, measured 1 → 0 turns), not a smaller schema; BP-22
stays refused for the corrected reason. The scorer now defaults to the newest registered
criterion (BP-38) and the runner may not overwrite the README (BP-40). The next spend is
gated on a targeted run over the losing tasks (BP-41..44): the bench did what §1 asked —
it named the product work (references and read_source payload, stop-when-answered) before
another dollar goes to L4.

**Revisit when.** A harness release removes MCP tool deferral (retire the line item); L0
reports precision or recall under 0.95 for an adjudicable tool on a pinned repo (the index is
the bottleneck; L4 must not run); the `none` arm passes more than half the corpus; two
same-corpus L4 rows exist and `regress` has compared them.

## ADR-033: The strengths matrix — every claim measured on a real change, against its alternative, by an oracle fathom does not own, with the prediction written first (2026-09-13)

**Status.** Accepted. Vision: `docs/vision/27-strengths-bench.md`. Plan: `docs/plans/strengths-bench.md`.

**Context.** The first replicated home run (ADR-032 as amended) measured a cold single lookup
with each arm locked to one tool and found fathom more accurate and more expensive. Every
claim fathom makes to an adopter — obligations at the edit, the verify gate, continuity,
packs for workers, freshness, fleet blast radius — is a claim about a *change*, and none was
on a ledger. The operator's lived experience (a large tokenomic effect in real sessions) and
the bench disagreed because they measured different things; the session-level A/B (MI-5)
never collected an OFF arm.

**Decision.**
1. *A matrix, not a bench.* Each claim is a row with fixed columns: versus, oracle, readings,
   predicted win, predicted no-effect. Rows M1–M9 are registered in vision 27 §1. A row may
   not be run without its no-effect prediction written; a run that visits only expected wins
   is refused as marketing.
2. *One driver, arms as data.* `cmd/scenariobench` runs scenarios (a change to a pinned
   subject under a session budget) with arms declared in the corpus — wired or stripped, MCP
   config, tools, instruction variant, notes file, competitor setup — never in code. The
   realistic arm (fathom plus Read/Grep/Bash) is the default treatment; the control is the
   same fork stripped of fathom with the same notes-file instruction.
3. *Oracles fathom does not own.* Success is the pinned PR's failing-to-passing and
   passing-to-passing tests; SDD reachability is a `go test -cover` profile; rule violations
   are the invariant's own detector; graph and reference correctness is `go/types`. Fathom's
   verify, obligations and index are never the judge of their own readings.
4. *Competitors run as documented.* A static graph or an LSP server is installed and invoked
   exactly as its own README says, at a pinned version named on the row; `docs/bench/
   competitors.md` carries the commands. The free edge comparison runs before any paid arm.
5. *Cells and keys as ADR-032.* Control and competitor cells are keyed by their own surface
   (empty, or the competitor's version) and survive every fathom change.
6. *The doctrine follows the data.* The stanza, skills and adoption guides route by question
   shape (index for where/who-calls/what-breaks/what-governs/what-was-known; grep for
   literal text); tool descriptions say when a result is the answer.
7. *Repeatable by a stranger.* Corpora derive from pinned public sources with per-task
   derivation; the runbook gives one command per layer and the cost per tier; the free layers
   join `task validate`.

**Consequences.** Paid runs stay operator-initiated within the tiers in vision 27 §9 (the
2026-09-13 directive authorises the targeted and pilot tiers; the credible tier is a
separate decision). M8 (fleet blast radius) and M9 (breadth) are registered and scheduled
after the pilots. A gold-patch coverage reading is a lower bound and is reported, never gated.

**Alternatives rejected.** Extending the home bench with more lookup tasks (measures the
wrong regime); a hand-written "fathom vs X" comparison (not reproducible, not pre-registered);
a session-level A/B on the operator's own work as the only instrument (admissible but not
repeatable by a stranger — it stays as L5 beside this).

**Revisit when.** M2 or M3 shows no effect on the cross-package and governance-bound
categories at n=3 (the SDD story is prose); M5's control overlap is under 30%; M6 shows an
LSP beating fathom on multi-package resolved callers (vision 21's typed edges are the fix,
not more bench); payload stays over 1.0× after wave 1 (replace the lookup tools with
answer-shaped tools); a competitor ships an MCP surface that makes the free comparison
one command (fold it into L0).

## ADR-034: Receipts — instrument truth on every bench cell, evidence anchors the gate verifies, a budget and a receipt on every answer, discoveries as fresh-resolved pointers, coverage as a code fact (2026-09-13)

**Status.** Accepted. Vision: `docs/vision/28-receipts.md`. Plan: `docs/plans/receipts.md`.

**Context.** The first strengths-matrix round (ADR-033) published readings for M1–M7 on
74 ledger rows. A second reading of the raw transcripts found the wired arm had run
without fathom's MCP server in all 72 sessions (the driver handed the harness the
un-rendered arm; `--strict-mcp-config` hid the worktree's own entry), the graphify server
had failed on a command that does not exist, gopls and Serena had connected and never been
called, every session had inherited the operator's global hooks (fathom's own primer on the
control arm; a Stop hook's plan snapshot answered in a home cell), and the thread brief had
crossed between two tasks that shared a worktree basename. The numbers were honest; what
they measured was prompt text. The home bench, where the tools were present, showed the
uncapped tools (`find_callers` 28 KB, `staleness_report` 46 KB) as a third of the payload
overshoot, a Read after 27 % of Greps, and one missing shape (implementations of an
interface method). Nothing carries a planner's discoveries to a worker except a plan row's
`ctx:` pointers pasted by hand.

**Decision.**
1. A bench cell carries its instrument truth: MCP server statuses, a tool-call histogram,
   `surface_calls`; a wired or surface arm whose server is not connected is LOST, and one
   whose surface was never called is flagged. Sessions run in a clean room (Claude Code's `--restricted` mode with the arm's
   own settings file, `--permission-mode default` under an allow-list, per-cell threads).
   A competitor arm gets a routing stanza of the same shape as fathom's.
2. A published reading is a plan row with an `oracle: ledger` anchor the verify gate
   resolves; it refuses a missing, LOST or `surface_unused` cell. Numbers in the README are
   rendered from the ledger. Every artifact opens with the verdict in plain language.
3. Every agent-surface tool renders under the 16 KB budget with a named refinement; a
   generic name asks for scope; `references` lists implementations on an interface method;
   search returns each hit with its enclosing symbol from the index (SB-48).
4. A discovery is a pointer with provenance, never rendered text: the call log records the
   answer's top pointers; SubagentStart renders the parent session's discoveries resolved
   fresh; the session-end note carries them to the next session; `routes` derives the
   recurring ones with decay; the thread scope is the repo's identity. Kill criteria are
   pre-registered in the vision (§4).
5. Test coverage is a code fact fathom asserts from a supplied profile joined to the index,
   by a repo-declared layer; judgments about test quality stay in the review kit.

**Consequences.** The 74 first-round rows carry `instrument_defect` and the readings are
restated as unmeasured; vision 27's §0, the bench README and scope G10 say so. Reruns stay
in the pilot tier (≈ $120). The agent surface grows by `search` and `discoveries`, each
with a kill criterion. `fathom verify` learns one oracle class.

**Alternatives rejected.** Re-scoring the old cells (they contain no tool calls to score).
An LLM-distilled hand-off (vision 21 §7 rejects it; ADR-028's boundary). A third store for
discoveries (threads hold pointers, memory holds anchored facts, both already verify on
read). Building test-quality judgment into fathom (the review kit's lane; ADR-011).

**Revisit when.** A reading published after this record is found on a LOST or
`surface_unused` cell; the discoveries kill criteria fire; the rerun with every tool present
still shows the lookup surface above 1.0× grep on payload at equal success (the token claim
is withdrawn, the tools kept for review and verify).

## ADR-035: Verbs, not lookups — the agent surface leads with the answers grep cannot give; governance compiles to tests; every answer is falsifiable without fathom (2026-09-14)

**Status.** Accepted. Vision: `docs/vision/29-verbs-not-lookups.md`. Plan: `docs/plans/verbs.md`.

**Context.** With the tools present (the receipts rerun) the readings and the literature agree:
agents grep first, five arms are indistinguishable on lookup accuracy, grep is cheapest, and
fathom's lead is on blast radius, concern search and verify-shaped questions — the shapes grep
cannot answer — while its own governance lookup served the wrong rules (O23). The index is
graded by go/types as level with gopls and ahead of Serena and graphify, so the asset is the
layer above the index, not the index. `blast_radius`, `review_diff`, `test_impact` and
`rename_impact` exist and are not on the agent surface. Advisory governance produced zero
violations on either arm twice. Typed edges are Go-only and carry no visible tier elsewhere.
Fleet blast radius cannot cross a process. The harness's own memory carried a code claim the
custodian never saw. BC-15 (a corpus fathom does not own) is still open.

**Decision.**
1. The agent surface leads with verbs: the four impact verbs join it, `safe_delete_check` is
   added, the opening moves name a verb first. A new agent-surface tool must claim precision,
   completeness, freshness, governance or falsifiability — never token savings versus grep.
2. `obligations` ranks by governing scope, splits human rules from plan-row claims, inlines
   the cited lines, and reports what it displaced by tier; measured on forty hand-labelled
   files by a free oracle before any positioning claim.
3. Governance that can compile does: a `layer-rule` kind is served at the edit and emitted as
   a generated test; advice stays for prose that cannot compile.
4. Every edge and answer carries a resolution tier and an anchor; `fathom receipt verify`
   re-derives a claim from a bare checkout without fathom.
5. `gold_source_tool` rides every bench cell; a tool is a strength only when it earned the
   pass. The L0 index oracle and an L1 payload replay run in `task validate`.
6. fathom enters a corpus it does not own (Long Code Arena bug localization) with instrument
   truth, publishes raw cells, and closes BC-15 on the reading either way.
7. The custodian audits code-fact sentences in every auto-loaded file, including harness
   memory. A `service_ref` edge kind and a SCIP export follow, one edge kind at a time.

**Consequences.** Ten waves with kill criteria in the plan; free gates first. Positioning
drops any lookup-cost claim until a two-arm payload reading exists. The do-not list in vision
29 §4 is doctrine: no mutating tools, no ontology or query DSL, no embeddings as primary
retrieval, no taint engine, no language-count headline, no voided citation.

**Alternatives rejected.** Another payload trim on the lookups (tried; payload rose). Adopting
Serena's mutating verbs and self-attestation checkpoints. A human-browsable graph as the
adoption wedge. A standalone memory product.

**Revisit when.** The promoted verbs are the gold source on under one in five of their tasks;
the repaired `obligations` misses the top-three bar; a compiled rule catches nothing in 200
commits and thirty days; re-derivation reproduces under 95 % of receipts; the external corpus
shows no lift.

**Amended 2026-09-27 (ADR-053, the surface reckoning).** `rename_impact` and
`safe_delete_check` fold into `blast_radius` (`kind: change|rename|delete`);
`promotedVerbs()` drops `rename_impact`. The five-verb lead becomes two on the default
agent surface (`blast_radius`, now also answering rename and delete, and `review_diff`),
with `test_impact` and `localize` moved to the `extended` opt-in surface.

## ADR-036: Prompt-scoped obligations before the first edit — supersedes vision 21's hold on UserPromptSubmit in one narrow case (2026-09-14)

**Status.** Accepted. Vision: `docs/vision/21-continuity-threads.md` (supersedes its held
UserPromptSubmit paragraph); `docs/vision/27-strengths-bench.md` (guards-pilot).

**Context.** guards-pilot-5's second reading (O57) found the served obligation never named
the governing rule before the relevant edit in any wired trial, and six wired trials made no
edit at all, so the edit hook could not fire; a refusal-shaped task needs the rule before any
edit.

**Decision.** `fathom prompt-hook` on UserPromptSubmit serves only the rules governing the
paths, packages and symbols the prompt names — never a per-prompt digest, never a general
injection — capped at the obligations cap and an 8-second deadline, printing nothing when the
prompt names nothing.

**Consequences.** Vision 21's schema-tax objection stands for the general case; this narrow
case is priced by the cap and measured by the M1 rerun on the repaired instrument
(guards-pilot-6).

**Revisit when.** The measured injection cost per prompt exceeds the edit hook's, or
guards-pilot-6 shows no change in rule_reached_agent.

## ADR-037: `obligations` degrades to raw guide sections, never to silence (2026-09-16)

**Status.** Accepted. Bench readings: vs-pilot-5 and guards-pilot-6 second readings
(docs/vision/29-verbs-not-lookups.md §2–3 wave B; O54, .fathom/observations.md).

**Context.** Both readings converge on the same product defect. vs-pilot-5: the obligations
tool never served the predicted rule in 0 of 4 trials built to test it — the compiled-rule
layer hit `budget_stopped` (14/60 sections), displacing the cited rule to root/parent tiers;
both trials passed only via `mcp__fathom__search` + Read, the same route grep took.
guards-pilot-6: obligations never surfaced the task-specific governing rule on 6 of 8 tasks'
wired trials, serving only a fixed generic boilerplate subset of `internal/CLAUDE.md`, capped
before the invariant (I2, I11, I19, I28, I31, I36) appeared; a brand-new file's PostToolUse
hook returned only "index is behind your working tree" and nothing else
(`obligations_degraded_stale_index`). O54 had already built `LiveLayerCoverage` and
`NearestUncompiledGuide` to NAME the gap (`layer_coverage`, a `note` pointing at
`fathom compile --continue`) but never to FILL it — the served answer was still silence plus a
pointer to a command the agent cannot run mid-edit. Meanwhile a plain grep finds the rule in
the nearest CLAUDE.md in one call.

**Decision.** When the compiled obligations layer's coverage for the edited path's own scope
is incomplete — a `budget_stopped` run, a recorded run whose corpus has grown past what it
saw, or no compile_runs row at all for a layer that also has zero claims of its own —
`obligations` (`internal/mcp/semantic_tools.go`'s `renderObligationReceipt`, via the new
`rawFallbackBackend` capability) ranks the layer's own UNCOMPILED sections by the identical
governing-scope proximity ranker compiled claims use (`rankByGoverningScope`, VB-8,
`internal/semantic/governingscope.go`) and serves up to `semantic.RawFallbackReserve` (2) of
the nearest as raw rows: `tier:"raw"`, a `why` note naming the coverage defect, the section's
own citation as `src`, prepended AHEAD of any farther-tier compiled row so a nested guide's raw
section is never buried below a root rule that merely happened to compile. A raw row only ever
ADDS — it never displaces a compiled row, never consumes a slot from the six-rule
`MaxObligations` cap, and never fires when a fresh compiled claim already answers the exact
section (the same digest/prompt-version freshness test `LiveLayerCoverage` already applies). A
store with existing claims but no recorded compile run (a seeded fixture, not a real compile —
`internal/indexbench`'s L1 payload-replay fixture is exactly this shape) is deliberately NOT
treated as a gap: guessing from bookkeeping absence alone would be a false positive on a
byte-pinned golden, not a real defect. The edit hook and the prompt hook need no code change —
`writeRuleBullet` already renders whatever `tier` a served rule carries.

**Consequences.** `internal/semantic/rawfallback.go` (`RawFallbackRules`), `pkg/client`'s
`RawFallbackObligations`, and the MCP wiring add one new optional backend capability, following
the exact degrade-on-absence pattern `layerCoverageBackend` already established: a backend that
omits it serves byte-identical payloads to before this ADR. No LLM call sits on this path — it
is a deterministic rank over sections already on disk. `internal/mcp/autorefresh.go` gained an
explicit forward for the new capability (the O7 forwarding class this file's own doc comment
names) so the capability survives serve-http's default auto-refresh wrapper instead of being
silently stripped the way `ObligationsLayerCoverage`/`ObligationsLayer`/`ClaimFeedback` still
are today — that pre-existing gap is out of this wave's scope and is named here so it is not
mistaken for fixed.

**Alternatives rejected.** Raising the compile budget alone (treats the symptom on one repo's
current corpus size, not the general case — a corpus always eventually outgrows any fixed
budget). Compiling on demand at serve time (violates "determinism at query time, reasoning at
compile time" and puts an LLM call on the decision-moment hot path). Serving the WHOLE
uncompiled guide verbatim (unbounded payload growth; the existing snippet/byte-budget
discipline already caps a compiled rule's citation, and a raw row must cost the same). Treating
every `!found` (no compile_runs row) as a gap unconditionally (breaks the L1 fixture, which
seeds claims directly without a compile run — a real false positive, not a real gap).

**Revisit when.** A raw row is the gold source on fewer than one in five of the tasks
vs-pilot/guards-pilot built around a known coverage gap; `RawFallbackReserve` (2) proves too
small or too large against a re-run of guards-pilot-6 on this fix; or the pre-existing
`ObligationsLayerCoverage`/`ObligationsLayer`/`ClaimFeedback` auto-refresh forwarding gap named
above still has not been closed by the time a daemon operator reports the layer split or D2
feedback silently not working in production.

**Revisit-when trigger fired, resolved (wave N15 item 5).** The forwarding gap named above was
still open: `internal/mcp/autorefresh.go` forwarded `RawFallbackObligations` (this ADR's own
capability) but not `ObligationsLayerCoverage`, `ObligationsLayer`, `RootObligationsLayer`, or
`ClaimFeedback` — so the operator daemon (serve-http's own auto-refresh-on default) never
emitted `layer_coverage` at all, regardless of O54 deliverable 2 having shipped, because
wrapping the client in `autoRefreshBackend` silently stripped all four before any request
reached them. Fixed: all four now have explicit forwards, following the same O7 forwarding
class every other optional capability in that file already documents.
`ObligationsLayer`/`RootObligationsLayer` needed one more step beyond a bare forward — the
`obligationLayers` caller (`semantic_tools.go`) detects support via a TYPE ASSERTION on the
served backend, which is the wrapper when auto-refresh is on, so the wrapper's own method
existing at all made that assertion always succeed; the wrapper's `ObligationsLayer`/
`RootObligationsLayer` now degrade to plain `Obligations`/`rootObligations` themselves when the
wrapped backend lacks the layer split, so a pre-VB-9 backend still degrades exactly as
documented instead of hard-failing purely because auto-refresh happens to be on. Verified:
`TestAutoRefresh_ForwardsObligationsLayerCapabilities` (a wrapped backend that implements all
four exposes every one of them through the wrapper) and
`TestAutoRefresh_ObligationsLayerCapabilitiesDegradeGracefully` (one that does not degrades
honestly, never panics); the full `internal/mcp` suite, including the two existing
auto-refresh-wrapper round-trip tests this change could have broken
(`TestObligationsTool_RawFallback_ThroughAutoRefreshWrapper`,
`TestSemanticTools_ThroughAutoRefreshWrapper`), stays green.

**Addendum: ranking within a tier (wave O2, 2026-09-17).** vs-pilot-6's second reading
(docs/bench/readings/vs-pilot-6-second-reading.md §5) found the raw fallback THIS ADR shipped
fired on 8 of 10 "what governs this" trials — new, up from never existing — but never once
served the rule the oracle wanted (0 of 10 from the raw row itself): every fired trial served
an unrelated decoy (internal/CLAUDE.md's "Store purity" section, or a root CLAUDE.md header).
Root cause: `rankByGoverningScope` (VB-8) gets a section to the right governing-scope TIER, but
never decided which member of a multi-section tier wins — both `RawFallbackRules`'s candidate
sort and `obligationsCore`'s compiled-claim `less()` fell straight to document order (a raw
section's `StartLine`) or a near-alphabetical tie-break (`specificity`/`Rule` text) once a tier
was fixed, so a nested guide's several same-tier sections were served in authoring order, never
by which one actually names the edited file. Fixed by `internal/semantic/relevance.go`: a
deterministic, no-LLM lexical relevance score (path-derived tokens, weighted ancestor <
package < stem, plus optional caller-resolved symbol/import tokens at stem weight; heading hits
weigh 2×; per-token hit count capped at 3; no length normalisation — an early length-normalised
version penalised exactly the multi-topic section this fix targets) inserted as a new tie-break
strictly INSIDE a tier, in both rankers, never overriding VB-8's own tier/depth primacy. Five-
task offline reproduction (`internal/semantic/h1governance_test.go`, diagnostic, not a hard
gate): H1-T01 and H1-T12 now pass; H1-T02/T08/T13 still miss a strict top-3 window — a real,
named limitation (a nearer tier's own several sections can still fill every slot before a
farther-tier answer competes; a root-wide convention with no lexical tie to any one file's own
name needs a different mechanism entirely), not a regression on what this ADR already shipped.
Full account: `.fathom/observations.md` O73.

**Addendum: cross-tier promotion and behaviour-token relevance (wave O3, 2026-09-17).**
The addendum above named two of its own three misses as structural, not fixable by a
within-tier tie-break alone. This wave built the two mechanisms it predicted and measured
exactly how far each one closes the gap — both real, deliberately narrow, no-LLM. (1)
**Cross-tier promotion** (`crossTierCloser`, `internal/semantic/governingscope.go`): a
farther-tier candidate whose relevance is STRONG (a hit on a packageWeight-or-above token —
the file's own package path, stem, or a caller-resolved symbol/behaviour name, never a mere
shared ancestor or extension; `relevanceStrength`, `internal/semantic/relevance.go`) is
promoted ahead of a nearer-tier candidate that is a TOTAL lexical blank (relevance == 0) —
proximity still wins outright the moment the nearer side has ANY hit, however weak. Wired into
both rankers exactly where `closerThan`/`a.governing != b.governing` used to decide alone
(`obligationsCore`'s `less()`, `RawFallbackRules`' candidate sort). (2) **Behaviour-token
relevance** (`BehaviorTokens`, `internal/semantic/relevance.go`): a root-wide convention that
names no file/package/symbol of its own (the workspace/ row, the Gate row) can never earn a
"strong" lexical tie via (1) — what ties it to a file is the file's own BEHAVIOUR, not its name.
A small fixed table of source-text signals (`os.WriteFile`, `os.Create`, `os.MkdirAll`, …  ->
`"workspace"`; `exec.Command("go"/"sh"/"bash", …)` -> `"gate"`/`"validate"`) is scanned against
the edited file's own real content — already fetched by `pkg/client.RawFallbackObligations`
to build the layer's corpus — and folded into the same `extraTokens` path a resolved symbol
name already uses.

Re-running the five-task offline reproduction against THIS repo's own real corpus at HEAD
(`internal/semantic/h1governance_test.go`) with both mechanisms wired: **unchanged** —
H1-T01/T12 still pass, H1-T02/T08/T13 still miss, and each miss is now a *proven*, not merely
suspected, structural limit rather than an unclosed gap: H1-T02 (ADR-021, strong on
Edge/EdgeKind) stays crowded out because internal/CLAUDE.md's five same-tier sections each
carry at least a weak hit (mostly the bare ancestor token "internal" in passing) — never a
true zero — so promotion's own cap correctly declines to fire; H1-T13 (ADR-014) stays a miss
because the FARTHER-tier candidate itself is the one with zero relevance (the content drifted
away from anything lexically tying it to model.go — promotion only ever promotes a farther
side that is itself strong, and there is nothing here to promote); H1-T08 (the workspace/ row)
stays a miss only because cmd/fathom/emit.go (a synthetic scenariobench path, never a real
file on disk in this repo) leaves `BehaviorTokens` nothing to read — the mechanism itself is proven to
close exactly this shape of gap on a real file: a synthetic corpus fixture
(`TestRawFallbackRules_BehaviorTokenPromotesUntiedRootConvention`) and a full client-level
proof reading real file content off disk
(`TestClient_RawFallbackObligations_BehaviorTokenFromFileContent`) both show the workspace/
convention winning its slot over a lexically louder decoy once the edited file's own
`os.WriteFile` call is scanned. `h1governance_test.go` itself is no longer purely diagnostic
(wave O3 item 3): H1-T01/T12 are now a HARD gate (a future regression here fails the build);
H1-T02/T08/T13 stay diagnostic-only, each carrying its own proven reason inline rather than a
bare MISS. VB-11 oracle (`go test ./internal/indexbench -run TestGoverningRuleOracle_Deterministic`)
and the L1 payload replay (`go test ./internal/indexbench -run TestPayloadReplay`) both stayed
byte-identical. Full account: `.fathom/observations.md` O75.

**Addendum: ancestor-only hits no longer block cross-tier promotion (wave O4, 2026-09-17).**
The wave O3 addendum above left H1-T02 and H1-T13 as proven, structural misses; item 1 of wave
O4 (docs/bench reading behind the same H1 reproduction) revisited `crossTierCloser`'s own cap:
it required the nearer-tier candidate to be a TOTAL lexical blank (`relevance == 0`) before a
farther, strong candidate could leapfrog it — but a bare ANCESTOR-DIRECTORY token ("internal",
"cmd", "docs") already registers as a nonzero hit under `relevance.go`'s own scoring purely
because every sibling file under a guide shares it, never because the guide names the edited
file at all. H1-T02 (`internal/core/model.go` vs. ADR-021) was exactly this shape:
internal/CLAUDE.md's other same-tier sections each carried only a weak "internal" mention. The
cap is narrowed from "the nearer candidate has ANY hit" to "the nearer candidate is itself
STRONG" — `relevanceStrength`'s own packageWeight-or-above bit (the file's own package path,
stem, or a caller-resolved symbol/behaviour token; never a mere ancestor or extension hit) —
which is exactly what the previous addendum's own text already named as the discriminator for
"strong"; this wave only narrows what BLOCKS promotion to match it, rather than introducing a
new signal. `crossTierCloser`'s signature dropped the now-unused relevance floats (aRel/bRel)
entirely, taking only the two `strong` bools that decide it. Re-running
`TestH1GovernanceTasks_RawFallbackOffline` against this repo's own real corpus: H1-T02 now
passes and is promoted to a HARD gate alongside H1-T01/T12. H1-T13 stays a miss, for its OWN,
different, already-diagnosed reason (ADR-014's own section is the farther-tier candidate here
and carries ZERO relevance itself — promotion only ever promotes a farther side that is itself
strong, and there is nothing here to promote); its `missWhy` is unchanged by this addendum.

A genuinely new finding surfaced re-running the fix against the real corpus rather than only
the synthetic fixtures built to isolate it: even with the ancestor-token exemption in place,
H1-T02 would NOT have passed if not for the fix ALSO clearing the field of several other
tier-2 sections that were purely ancestor-blocked — and the same real-corpus run surfaced that
ADR-037's OWN multi-wave addendum text (the paragraphs immediately above this one, which
document this very ranking mechanism using its target tokens — "Edge/EdgeKind", "core",
"model" — verbatim, because they narrate the H1-T02 reproduction itself) also crosses the
"strong" bar on ordinary domain vocabulary and, being the longest, most discursive single
entry in ADR.md, out-scores genuinely on-topic sections on raw hit-weighted magnitude within
the SAME tier (root, alongside every other ADR). This is a same-tier (wave O2) relevance-
ranking limitation, not this addendum's cross-tier one — wave O2's own design deliberately
rejected length-normalisation, so a long section touching many relevant words legitimately
outscores a short, focused one under the existing rules — and it is a limitation this repo's
OWN corpus keeps actively widening: every wave that narrates a governance-ranking gap using
the gap's own tokens measurably worsens the self-reference the next reproduction will
encounter. Proven correct in ISOLATION, free of that confound, on synthetic fixtures at both
the unit level (`TestCrossTierCloser_AncestorOnlyHitOnNearerDoesNotBlockPromotion`,
`TestCrossTierCloser_StrongHitOnNearerBlocksPromotion`) and the `RawFallbackRules` level
(`TestRawFallbackRules_CrossTierPromotion_AncestorOnlyHitDoesNotBlockPromotion`,
`TestRawFallbackRules_CrossTierPromotion_StrongHitOnNestedGuideBlocksPromotion`). VB-11 oracle
(`TestGoverningRuleOracle_Deterministic`) and the L1 payload replay (`TestPayloadReplay`,
`internal/indexbench`) both stayed byte-identical. Full account: `.fathom/observations.md` O80.

**Revisit when.** A future wave's own real-corpus H1 reproduction shows ADR.md's self-
referential growth (this addendum included) has crowded a genuinely on-topic answer out of a
top-3 window that previously worked, or when a same-tier length-normalisation redesign is
seriously proposed — the wave O2 addendum's own rejection of length-normalisation should be
re-read first, since the failure mode that rejection was built to avoid (penalising a
thorough, multi-topic section) and the failure mode this addendum just measured (a long
section winning on breadth alone) are the same trade-off pulling in opposite directions.

## ADR-038: Governance bites at the edit for checkable rules — a compiled guard or layer rule denies a PreToolUse violation, never only advises after it (2026-09-17)

**Status.** Accepted. Vision: `docs/vision/29-verbs-not-lookups.md` §2 thesis 2 ("a rule that
can be compiled to a failing test is served at the edit AND fails the gate"). Amends
`docs/design/guarded-edits.md` §2.2's "inject-only, never blocking" doctrine, under its own
named escalation trigger ("revisit only with a documented incident a warning demonstrably
failed to prevent"), and narrows `internal/harness`'s §5.3 inject-only contract the same way
ADR-036 narrowed vision 21's UserPromptSubmit hold: one specific, bounded case, everything else
unchanged.

**Context.** guards-pilot-6/8 (`docs/bench/readings/guards-pilot-8-second-reading.md`) found the
governing rule now reaches the agent before the relevant edit on 4 of 8 tasks (ADR-036/037's own
fixes), and the wired arm violated the rule anyway on two of those four — i19 (an org-specific
name landed in a new fixture) and i28 (a lenient DuckDB path shipped over ADR-025's loud
refusal); one wired session went on to rewrite its own pinned regression test rather than honor
the rule it had just been served. Violations tied the control arm 4-for-4. An obligation the
agent can read, note, and still contradict is not the same instrument as a check the tool call
itself cannot get past.

**Decision.**
1. `#Guard` and `#LayerRule` (`schema/guard/schema.cue`) gain `enforce: bool | *false`.
   `internal/guardspec.LoadGuards`/`LoadLayerRules` refuse to load an enforced rule with no
   deterministic check behind it, naming it: a note-only guard (no `forbidden_paths`, `checks`,
   or `command_scopes`), or a layer rule whose kind is `no_cycles`/`cross_service_direction`
   (both need the full index or fleet graph a single proposed file cannot supply).
2. `internal/guardspec/enforce.go` adds the deterministic checks, each a pure function over one
   proposed edit's own repo-relative path and text: `forbidden_paths` (guard, any write to a
   named path is itself the violation); `checks: ["org_token", "secret_literal",
   "workspace_output", "usecase_maintained"]` (guard, content scans — `org_token` reuses
   ADR-011's scanner, lifted into `internal/guardspec/orgcheck.go` so the hook and
   `TestNoOrgSpecificNames` share one rule set); `cgo_purity` and `import_direction`/`boundary`
   content scans (layer rule, over the proposed text's own textually-present imports, keyed to
   the rule's existing `from`/`to`/`allow`/`guarded` fields — no schema addition needed for cgo,
   since ANY cgo import under a `from` package violates a "stays pure Go" sentence regardless of
   its declared `to` list). A command guard's own `command_scopes` match IS its violation
   (design §2.5's shape already has no separate content to check).
3. `internal/harness.HookOutput` gains `Deny`/`DenyReason`; `claudeCodec`/`codexCodec` render the
   documented `hookSpecificOutput.permissionDecision:"deny"` shape (verbatim against
   `internal/harness/testdata/codecs/codex/pre-tool.deny.json`, whose header comment is updated — the shape fathom
   now emits, narrowly, not the shape it never emits) on `EventPreTool` only; Gemini's codec
   refuses `Deny` outright rather than silently downgrading it to advisory. `HookInput` gains
   `NewContent`/`OldContent`, decoded from Claude Code's Write `content` / Edit
   `old_string`+`new_string` / MultiEdit's `edits[]`, and from a Codex `apply_patch`'s `+`/`-`
   hunk lines (`ApplyPatchContent`). `cmd/fathom/enforce.go` wires this into `edithook_pre.go`:
   `cmdEditHookPreFile`/`cmdEditHookPreCommand` run the enforced checks BEFORE the advisory
   daemon round-trip (pure local text either way, so the "instant silence on a miss" contract
   §2.2 already established for the advisory half holds for enforcement too) and, on a
   violation, emit `Deny` with a single line — the rule, its citation, the remedy — never a wall
   of text; one beat (`edit-hook`/`denied`) records it. A codec that cannot deny falls back to
   the ordinary advisory path rather than losing the signal or crashing. Escape hatch:
   `FATHOM_ENFORCE=off` (env, checked first) or a project/user `[guards] enforce = false`
   (`internal/config`'s new `Guards` section, `schema/config/schema.cue` `#Guards`) turns every
   enforced rule back to advisory, repo-wide; both fail OPEN (enforcement stays live) on a config
   read error, the same direction `capabilityOffAt` already takes.
4. This repository's own `.fathom/guard.cue` turns `enforce: true` on the rules that map to
   guards-fathom.json's eight tasks where a deterministic pre-write check exists: `store-no-cgo`
   (i2, layer rule, `cgo_purity`), `internal/connector/testdata/**` `org_token` (i19),
   `cmd/fathom/*.go` `workspace_output` (i31), `docs/use-cases/*.md` `usecase_maintained` (i36),
   and a new command guard denying a raw `cp` onto the installed daemon path (i16's Bash half —
   `O25`). i1 (an unsorted output field) and i11 (a new MCP tool's doc line) have no structural
   pre-write signal and stay gate-only (`TestDeterminism`, `TestMcpToolsDocMatchesRegistry`); i28
   (a lenient DuckDB path) is a control-flow decision no structural check here can see either,
   and stays gated by `TestBackendForPath_DuckDBLoudlyRefused`; i16's OTHER half (a CLAUDE.md
   prose edit instructing a raw-cp fast path) also stays gate-only via `task config-audit` — a
   content-pattern check for "advises a raw cp" risks a false positive on the Taskfile's own
   legitimate `cp -f bin/fathom … .new` line.
5. `internal/scenariobench` records `enforced_refusals` per cell: `handleHookResponse` counts a
   `PreToolUse` `hook_response` whose own output carries the deny marker, under the synthetic
   `HooksFired["enforced_refusals"]` key (mirroring `obligations_injected`'s own pattern
   exactly); `assertWiredHooksPresent` now also requires the wired arm's `settings.json` to carry
   a `PreToolUse` group matching `"Edit|Write|MultiEdit"` — without it the enforced-deny path can
   never fire in a cell at all, so a reading claiming `enforced_refusals` coverage would be
   measuring nothing.

**Consequences.** The advisory channel (`renderPreEdit`, `obligations`, the PostToolUse path) is
completely unchanged for every guard and layer rule that stays `enforce: false` — the default,
and every rule authored before this wave. `internal/harness`'s own tests
(`TestClaudeCodecNeverEmitsDeny`, `TestCodecsRoundTripDocumentedFixtures`'s inject-only loop)
still hold for ordinary `Context`/`Block` outputs; a new, parallel set of tests
(`TestClaudeCodecEncodesDenyBytes`, `TestCodexCodecEncodesDenyBytes`) pins the amendment
narrowly rather than loosening the old guarantee. A pre-write check is deliberately conservative
— false negatives (a violation phrased in a shape the regex-based scan misses) stay recoverable
via the advisory obligation, the compiled generated test, and code review; a false positive
would deny a legitimate edit, so every check's own doc comment states its narrowing choice.

**Alternatives rejected.** Blocking on every guard, enforced or not (the whole point of `enforce`
as an opt-in is that a guard's author, not a global switch, decides which rules are load-bearing
enough to deny on — a note-only "read this first" guard has no business ever blocking). An LLM
judging whether the proposed content violates the rule (an LLM call on the decision-moment hot
path, and a proven-failing claim from an untrusted judge is worse than a missed one). Blocking
only in CI/the gate, never at the edit (already true — `fathom rules gen-test`'s generated test
is the gate half of thesis 2; the readings show the gate alone does not stop the token/time spent
on a violating edit before it is caught, only the deny does).

**Revisit when.** A false positive from any of the five checks denies a legitimate edit in real
use (name the check, narrow or drop it); `FATHOM_ENFORCE=off` sees material adoption, suggesting
the checks are too aggressive by default; a guards-pilot rerun on the enforced arm still shows a
violation the deny should have caught (name which check missed it and why); or `enforce: true`
proves to need a check kind this ADR's five do not cover, at which point the schema's `checks`
enum grows rather than each repo hand-rolling its own.


## ADR-039: Build-cache placement is explicit, portable, and bounded (2026-09-16)

**Status.** Accepted; amended 2026-09-29 (EW-4: the guard trims by age).

**Context.** The repository hardcoded one workstation's removable volume for Go scratch,
while an agent independently accumulated 15 GiB under another machine-specific
`fathom-gate` path. The first choice made a clone non-portable; the second proved that
agent habit is not a disk policy. Go has no repository-local environment file, and a
silent fallback from a missing removable volume would recreate pressure on the system
disk precisely when the operator believed the cache was external.

**Decision.** Supported build entrypoints (`task` and repository build scripts) share
`scripts/lib/build-cache.sh`. With `FATHOM_CACHE_ROOT` unset they preserve the caller's
native Go, temp, and uv environment. When set, it must name an absolute, pre-existing,
writable directory; the helper derives build, module, temp, and uv subdirectories below
it. `FATHOM_CACHE_MAX_MIB` bounds only the regenerable Go build cache (8192 by default,
`0` disables), trimming it by entry age before a supported Go invocation (amended
2026-09-29, below). A missing configured root refuses loudly and never falls back to
internal storage.

**Consequences.** Other machines need no configuration and retain standard tool
behavior. A workstation may place Fathom caches on removable storage without embedding
that path in the product. Raw `go` commands remain outside repository policy; the
documented and gated path is Taskfile/scripts. Module downloads are not automatically
deleted, avoiding surprise network cost.

**Alternatives rejected.** A hardcoded external path (not portable); a silent internal
fallback (violates the placement promise); global `go env -w` settings (affect unrelated
projects); automatic module-cache deletion (surprising and network-expensive); a Fathom
TOML field (the binary cannot configure the toolchain needed to build itself).

**Revisit when.** A portable project-local Go environment standard ships; a second
supported build runner cannot share the shell contract; or measured automatic pruning
materially harms iteration.

**Amended 2026-09-29 (EW-4, O124): the guard trims by age and never cleans the whole
cache.** The original prune ran `go clean -cache` behind a `mkdir` lock that kept two
pruners apart but not a pruner from a build, so a build that started during the clean read
a cache being deleted and the pre-push gate failed on it. The guard now deletes entries
by age in three passes (unused for 7 days, then 3, then 1) and stops at the first that
fits; an entry used within the last day is never removed, because Go stamps an entry's
mtime when a build uses it and a live build may hold it. A cache still over the limit
after the last pass is reported and the build goes on. The prune lock is gone: an age
trim is idempotent and safe to run twice at once. `scripts/build-cache_test.sh` holds a
builder reading an entry while the guard trims. An operator who wants the whole cache
gone runs `go clean -cache` by hand with no build running.

## ADR-040: The findings loop — external review findings are governed projections served as obligations; fathom is a projection adapter, never the organizational record (2026-09-18)

**Status.** Accepted. Vision: `docs/vision/30-findings-loop.md`. Plan:
`docs/plans/findings-loop.md`. Counterpart: platform-engineering ADR-0103 and
`kits/review-kit/docs/design/fathom.md`.

**Context.** The review kit (platform-engineering `kits/review-kit`) owns an
organization's finding identity, lifecycle and dispositions and already emits
a projection outbox for fathom that it marks "pending: governed external
importer not negotiated". fathom already stores finding-kind claims with
content-addressed IDs, drift anchors, failure classes, patterns, resolution
outcomes and a gated promotion rule, and serves governing rules at the edit.
Neither side could close the loop: findings never reached the agent at the
point of work, a recurring finding never became a rule, and the review kit's
uplift brief (P0–P4) named the missing contracts. The earlier decision that
fathom must not be the only authoritative findings record (ADR-0098) stands;
what was never decided is the projection.

**Decision.**

1. **Ownership is hexagonal.** The review kit owns identity, lifecycle,
   dispositions and the portable bundle behind a `ProjectionPort`; fathom is
   the first projection adapter. fathom never approves, disputes, tailors or
   closes a review-kit finding, and a review-kit acceptance is never
   translated into a fathom `refute`.
2. **One governed import** (`fathom findings import`, `pkg/client.ImportFindings`)
   accepts the review kit's versioned projection under a provisioned
   subject→service mapping, derives claim identity deterministically in a
   producer-namespaced subject, maps lifecycle to the existing status and
   event vocabulary (`fresh`, `superseded` with `external-closure` /
   `revoked`, status-preserving `annotated`), enforces monotonic source
   versions with a per-claim cursor (duplicate / stale / conflicting-replay
   are distinct receipt counts), and never writes on a batch schema failure.
3. **Native and external findings are distinct reconciliation domains**,
   discriminated by `PromptVersion` (`review-verb/v1` vs `external-import/v1`).
   Native review never supersedes an external claim; an import never touches a
   native one; a fathom-origin producer is refused on import (no circular
   proof).
4. **Provenance and confidence are checkable.** A producer catalog
   (`schema/semantic/producers.cue`) declares each producer's kind, origin and
   tier cap; served tier is `min(event tier, cap)`; tier never rises on
   import; corroboration counts only across independent producers; every
   served finding is source-linked (producer, versions, rule, source id,
   revision, anchor path/lines/symbol, anchor digest, `anchor_current`);
   disagreements between producers are served side by side and counted, never
   resolved by fathom; annotation prose is quoted, never interpreted.
5. **Findings are served at the point of work** as an advisory block after
   the governing rules in the pre-edit hook (capped, omitted count always
   shown, drifted anchors flagged), as a count in the prompt hook and the
   session brief, and never as an enforcement. Enforcement belongs to rules;
   a recurring finding becomes a rule only through the human-adopted
   candidate (`fathom findings candidates`), after which ADR-038 applies.
6. **The public review contract is versioned** (`schema_version`, explicit
   `mode: report|persist` with a store-state equality guarantee for `report`,
   an explicit changed-path manifest, idempotency keys, exact input
   fingerprints, per-dimension `checked`/reason, `truncated`/`stale_index`/
   `cancelled` never readable as clean), and `fathom review-key` binds the
   changed manifest, the typed transitive closure, rule packs, plans, tooling
   and index identity into one token with an explicit unknown set and a
   changed-since-prior diff. The review kit keeps its own reuse policy
   (ADR-0099) and consumes the key through its `Digester` port.
7. **The outbox syncs without a network**: with `[findings] outbox` set, the
   session hook imports a changed outbox once per session and reports the
   receipt; the explicit CLI form remains.
8. **Measured before believed.** A `findings-loop` scenario corpus with paired
   criteria (`finding_addressed`, `finding_cited`) and kill criteria is
   committed; no number is cited before a second reading.

**Consequences.** The agent-facing MCP surface does not grow (findings ride
`obligations layer: "all"`); the write surface is CLI/client, inside the
trusted local boundary. Two additive store tables (`external_findings`,
`review_runs`) and one additive event verdict (`annotated`). The review kit
gains a `ProjectionPort` with fathom, file and null adapters, a publish verb
with receipts, additive anchor lines/symbol on its schema, and consumption of
the review key. agentic-kit gains an `evidence-producer` primitive (catalog
with tier caps and origin) that fathom consumes. Restore, retention, access
isolation across trust boundaries and named operators remain review-kit
obligations; the receipts and cursors here make that review possible.

**Alternatives rejected.** A findings database or CRUD API in fathom
(duplicates the claims lifecycle; vision 03). Symmetric status writes
between the two systems (two authorities for one disposition). SARIF as the
interchange (a fine export later; it carries no lifecycle, no source
versions, no subject authorization — the review kit's event contract does,
and a SARIF exporter is one adapter away). Enforcing findings at the edit
(a finding is one producer's observation; only an adopted rule is policy).
Model-assisted import or dedup (ADR-0098: enrichment may aid retrieval,
never tier or status).

**Revisit when.** The bench shows no lift in `finding_addressed` at n ≥ 3;
a second external producer needs a projection shape the event contract
cannot carry; the review kit's ADR-0098 "revisit" fires and fathom is asked
to be the record — then the durability list in vision 30 §5 is the entry bar.

## ADR-041: The substrate round — relational facts stay, embeddings leave, typed edges arrive by import, freshness follows the edit (2026-09-19)

**Status.** Accepted. Accepts `docs/vision/31-substrate.md` with the owner's
answers to Q22–Q24 (2026-09-19). Plan: `docs/plans/substrate.md`. Language
tracker: `docs/languages.md`.

**Context.** Vision 31 asked whether relational-for-everything, no embeddings
and no graph database is the right substrate for fathom's five jobs, and
answered with seven readings: the field converged on our shape, and our build
has ten provable limits that are cheaper to fix than to migrate around. The
owner answered the three open questions: a feature must earn its place, so
embeddings are disabled and removed from guidance rather than kept for rare
cases; typed edges proceed Go, CUE, TypeScript, Python, then the delegate's
order, with a tracked table of done and needed; freshness is the delegate's
call.

**Decision.**

1. **SQLite stays the system of record. No graph database, no DuckDB.**
   Graph walks stay in memory over per-service adjacency; fleet composition
   stays in Go until a measured query needs SQL across store files, at which
   point SQLite `ATTACH` is the path (vision 03's DuckDB ATTACH plan is
   withdrawn).
2. **Embeddings are removed, not parked.** The dead vector adapter, the
   `WithVector` option and the hybrid-rank vector branch are deleted; the
   agentic-kit embedding client import goes with them; guidance that mentions
   semantic or hybrid search is corrected to describe the lexical and graph
   paths that exist. Revisit only through vision 31 §5: a controlled result on
   a labelled concept-query set that lexical plus graph cannot match at ≤ 2×
   tokens. Until then "semantic" in fathom means compiled claims, never vectors.
3. **Substrate hygiene precedes every new capability.** Indexes on the
   columns every serve path filters; claims prefiltered in SQL by layer,
   status and scope prefix before Go scoring; the call-graph cache invalidated
   per package instead of per watermark; per-file re-ingest of dirty files; a
   dispatch-level response budget so no handler can regress the 63k-token
   class; the prompt hook ignores system notifications. Each is measured on
   the free tier of the bench pyramid before and after.
4. **Freshness is hook-triggered, per file.** The pre-edit hook asks the
   daemon to refresh the file being edited when its content hash differs from
   the index; the daemon re-extracts that file's symbols and syntax edges in
   place (typed edges stay package-granular until the next ingest) and the
   hook answers from the refreshed index. `fathom watch` remains the opt-in
   daemon-wide mode. The staleness line names what was refreshed and what was
   not.
5. **Typed edges beyond Go arrive by import.** `fathom ingest --scip` (and
   auto-detected `index.scip` / `.fathom/scip/*.scip`) maps SCIP occurrences
   to `typed_edges` under the same package-granular replacement, with the
   producer's identity recorded. Order: Go (done, native), CUE (native, via
   `cue/load` — no exporter exists), TypeScript and JavaScript
   (scip-typescript), Python (scip-python), then Java/Kotlin/Scala (scip-java),
   Rust (rust-analyzer), C/C++ (scip-clang), Ruby (scip-ruby), C# (scip-dotnet);
   PHP, Swift, Lua, Bash and the configuration dialects keep syntax-level
   edges. Where no exporter runs, a per-file resolver (imports plus
   module-scope names, stack-graphs style) supplies edges labelled
   `file-resolved` at tier pre-scan, never presented as typed.
   `docs/languages.md` is the tracker every round updates.
6. **Localization walks the graph.** `localize` expands seeds through the
   typed neighbourhood to depth 2, ranks by graph distance and evidence,
   reformulates when seeds are empty, and caps its payload; the Long Code
   Arena corpus is re-run on that instrument before localization is
   positioned or retired.
7. **Claims gain time and identity.** Bi-temporal validity, explicit
   `supersedes` links, anchors bound to the resolved symbol key beside the
   line span, and a link to the run that asserted the claim; `obligations`
   answers `as_of` a commit.
8. **The findings loop gains learning telemetry without prose inference.**
   Promotion candidates decay on dispute, weight by ownership and cap their
   cadence; promoted rules carry served/touched/disputed/last-fired
   telemetry; a human can add one scoped claim directly (`fathom claim add`,
   producer `human.review`, tier capped at sme-stated); a disposition can be
   previewed before it is recorded. Nothing is inferred from annotation text.
9. **Benchmarks stay runnable and unrun.** A free-tier L0 oracle for typed
   edges per language against SCIP truth on the calibration subjects; a
   TypeScript and a Python lookups corpus on those subjects; every corpus
   passes `scenariobench plan`; the next-round order is guards, findings-loop,
   lookups (Go), then the new language corpora. No run in this round.
10. **Pushes are gated by construction.** After INC-20 the push script refuses
   to push unless the lefthook pre-push hook is present, and a push log
   shorter than the gate's output is treated as an ungated push to be
   re-validated.

**Consequences.** One fewer subsystem (vectors), one new dependency
(`github.com/sourcegraph/scip` bindings; protobuf was already present),
additive store columns and links, a new daemon route for per-file refresh,
and a language tracker that turns "what is left" into busywork anyone can
pick up. The agent-surface tool set does not grow.

**Alternatives rejected.** Keeping the vector path dormant (a feature that
has not earned its place costs guidance and surface); a graph database or
DuckDB (vision 31 §1); writing typed passes per language (SCIP exporters
exist and are the calibration corpus's own oracle); daemon-default polling
for freshness (CPU always on for edits that happen rarely; the hook knows
which file matters).

**Revisit when.** Vision 31 §5; a language without a SCIP exporter becomes a
primary target (then the per-file resolver graduates to a typed pass); the
per-file refresh is measured slower than 150 ms p50 at the hook.

## ADR-042: The decision port — typed fast decisions behind a flag, off by default (2026-09-22)

**Status.** Accepted. Accepts `docs/vision/32-decision-port.md` (owner-directed,
2026-09-22, after the assessment "Jev and the Strengths Bench"). Plan:
`docs/plans/decision-port.md`.

**Context.** Fathom's facts come from the index, but three decisions are still made
by heuristics or by a slow text model: which six of ~140 eligible rules to serve at
an edit, which verb an issue-shaped prompt wants first, and which guide sections
carry a binding rule at all (the compile: 61 sections, 5.1M tokens, $2.79 and 75
minutes per subject and commit on this repository). Each is a typed question with a
probability answer. A model class now exists for exactly that shape (TypeSafe AI's
Jev, in early access), and an ordinary LLM asked for JSON answers it too.

**Decision.**
1. agentic-kit gains `foundations/decision`: `choice`, `score`, `noul` questions over
   a state, one batched request, `confidence` = (k·max−1)/(k−1) over the probability
   spread, bands at 0.5 and 0.9. Adapters `none` and `llm`; `typesafe` is a future
   (F21) gated on access and an A/B.
2. Fathom consumes it behind `[decide] provider = none | llm` (default `none`: no
   call is made and every site behaves as before) at three sites: the compile
   section pre-filter (skipped sections counted, disclosed, and served raw by
   ADR-037's fallback), obligations re-ranking within a governing-scope tier only
   (ADR-035's tier order is untouchable by a model), and prompt routing through a
   loopback daemon endpoint the hook calls under its existing deadline.
3. The port ranks, routes and triages; it never asserts a code fact (the ADR-028
   authority boundary). A decision in the unsure band never changes served content.
   Every decision is logged to the call log and disclosed in the envelope that used it.
4. I14 is amended: the decision port is the second sanctioned off-host egress after
   compilation, armed only by operator config, never from a hook process. I42 pins
   the default-off and disclosure properties.
5. No new MCP tool. Default-on for any repo only after the site's A/B (vision 32 §4).

**Consequences.** One new kit foundation and one fathom package (`internal/decide`),
a `[decide]` config block, a doctor row, an additive `compile_runs` column, two
envelope fields (`ranked_by`, `relevance`), one loopback endpoint (`POST /route`),
and a `Verified against` obligation on the `llm` adapter's prompt golden.

**Alternatives rejected.** A `decide` MCP tool (no call-log justification; internal
to serving); provider calls from the hook (I14, fail-open in milliseconds); model
re-ranking across scope tiers (breaks the one predictable part of serving);
recording every decision as a claim (ephemeral ranking is instrument truth, not
durable evidence).

**Revisit when.** A site's A/B shows no gain at n ≥ 3 (turn that site off by
default and say so); a second provider ships (the port's kinds are the contract, the
adapter is data); egress policy of an adopter forbids any third-party call (the
`none` default already answers it); the compile pre-filter ever loses a served rule
in an A/B (the threshold is wrong, not the fallback).

**Amendment (2026-09-22): the `typesafe` adapter ships, superseding decision 1's
"future (F21) gated on access and an A/B" clause.** Fires this ADR's own "a second
provider ships" revisit trigger, on owner direction rather than after the A/B —
access turned out to be a key the operator supplies
(`[decide].api_key_env`/`api_key_cmd`, else `TYPESAFE_API_KEY`), not a blocker to
wait on. `[decide] provider = none | llm | typesafe`; `typesafe` calls TypeSafe
AI's `/v1/systemone` wire directly (agentic-kit `foundations/decision.NewTypeSafe`),
at the same three sites, under the same I14 fail-open and disclosure properties as
`llm` — a missing key, an unreachable endpoint, or any other open failure is one warn
line and a silent fallback to `none`, never a startup or compile failure. The A/B
this decision originally gated default-on on (vision 32 §4) still applies before
`typesafe` becomes any repo's default provider. `docs/intent/futures.md` F21: landed.

**Amended 2026-09-23: `rank_within_tier` defaults to false.** Fires this ADR's own
"a site's A/B shows no gain at n ≥ 3" revisit trigger early, on measured harm rather
than a null result — `docs/bench/readings/decision-port-ab-2026-09-23.md`, the
DP-13/DP-14 A/B (trials=2, three corpora): the wired arm against the same arm plus
`[decide] provider = typesafe` (Jev 1.13, live via OpenRouter). Routing measured a
real, small positive (lca localize gold-source rate 4.6%→23.4%, paired mean +0.047,
bootstrap 95% CI [0.000, 0.125], 2 tasks better/0 worse; lookups-vs null, mean 0.000,
CI [-0.107, 0.107]). The obligations within-tier re-rank measured **negative** on
governance at the edit: guards-jev-r1's paired mean was -0.19 (CI [-0.375, -0.062]),
3 tasks worse and 0 better at n=2 trials, with the mechanism traced in raw
transcripts — the LLM re-rank reordering the served rules for the touched file
relative to the deterministic default, in one traced case (`guards-i28-duckdb-path`)
leaving the governing rule unserved at the first, most consequential touch (worse
than the baseline arm's own position there) and pushing it to the last visible
slot on a later touch of the same file. **Change:** `internal/config`'s
`DefaultDecide.RankWithinTier` is now `false` — a decision provider configured
without further tuning no longer reranks obligations within a tier; routing and the
compile pre-filter are unaffected and keep their own results. A repo sets
`rank_within_tier = true` explicitly to opt back in. **Revisit when** a
guards-shaped run at n ≥ 3 trials shows the re-rank net positive, or a corpus with
two governing rules genuinely competing within one scope tier is measured (this
A/B's 8 guards tasks never had that contest to arbitrate).

**Amended 2026-09-24 (ADR-044 decision 8, CV-M11): the `route` site fails its own
calibration check, and no `route_confidence` value fixes it.** ADR-044 decision 8
requires "0.9 right nine times in ten on adjudicated cases" before a site's default
proceed band is trusted; `docs/bench/readings/decision-port-calibration-2026-09-24.md`
ran it against `/route`, joining every logged route decision in
`sextant-r3-workspace`'s live `lookups-vs-jev-r1` and `guards-jev-r1` runs (82
answered decisions, 22 worktrees) against each task's own empirically-observed
gold-source tool. At confidence ≥ 0.9, route's choice matched the gold tool only
45.7% of the time (16/35) — well under the bar. Unlike a simple under-confidence
gap, **raising the threshold does not recover it**: sweeping every cutoff in
`[0, 1]` never reaches 90% (the best any cutoff achieves is 58.5%), and the
highest-confidence slice (exactly 1.0) is measurably *worse* than the 0.9-and-above
band it sits inside (28.6% vs 45.7%) — confidence and correctness are not
positively correlated in this sample. Per this ADR's own contingency for a failed
calibration check, the proceed threshold would ordinarily rise to the smallest
passing value; none exists on this data, so **`internal/config.DefaultDecide.
RouteConfidence` is unchanged at 0.9** — raising it to any value would not satisfy
the check and would misrepresent an unfixed miscalibration as fixed. This does not
change decision 3's boundary (the port still never asserts a fact, and a wrong
route suggestion only misdirects an agent's first tool choice, never a claim) or
any other site's calibration (`certify_suggest`'s own spot-check stayed 5/5
correct, unaffected). **Revisit when** a larger or less `governance_bound`-heavy
sample is measured, or route's own prompt/model changes in a way that could plausibly
fix the non-monotonicity this reading found — a threshold change alone is not
expected to.

**Amended 2026-09-24 (ADR-042, wave 2 of the same CV-M11 finding above): the
`route` site's line is never phrased as a firm instruction, at any
confidence.** The threshold fix above (leaving `route_confidence` unchanged)
only stops a wrong threshold from misrepresenting an unfixed miscalibration as
fixed; it does not, by itself, stop the hook from ever asserting a firm
suggestion when confidence happens to clear 0.9. Since the same reading found
confidence uncorrelated with correctness on this site (the ≥0.9 slice's own
1.0-confidence tail scored *worse* than the band below it), a `proceed`-band
line is no more trustworthy than a `confirm`-band one — there is no confidence
level this site can point to as "earned." `cmd/fathom/prompthook.go`'s
`routeLine` (DP-10) now renders the SAME hedged wording for both the
`proceed` and `confirm` bands ("this looks X-shaped; Y is the likely opening
move (confidence N, unconfirmed)"), never the prior firm form ("open with Y"),
and still says nothing at all below the confirm floor (0.5) or on any
failure. This changes only what the line SAYS: `routeHandler`'s own
`applied` bookkeeping (a `proceed`-band answer at or above `route_confidence`
logged to `decisions.jsonl`) is unchanged, since that log is instrument truth
about what the port did, not a claim the hook renders to a reader. **Revisit
when** a routing change (a different prompt, model, or provider) passes the
calibration check (0.9 right nine times in ten) on a newly measured sample —
only then does a firm line for a high-confidence band become warranted again.

**Amended 2026-09-29 (ADR-056): calibration first, and the decision-port site list.** The bar stands as
ADR-044 decision 8 set it: a site's proceed band is trusted only where 0.9 is right nine times in ten on
adjudicated cases. ADR-056 rule R3 adds Wilson-interval bars: a calibrated hint needs 90 of 100 right at the
shown confidence, a judgment that prunes, gates or reorders a default list needs a 95 percent lower bound of at
least 0.90, and a free-form question is shown uncalibrated and never prunes or reorders. New decision-port sites,
all opt-in and none in `DefaultDecide.Sites`: `triage_failure` (a failure kind over a test run), `ask` (judgments
the agent directs, over spans the graph selects), `explore` (question-directed traversal), `compact_advice` (when
to compact) and the `judge_*` family (sampled judged telemetry, `judge_failure` first). Each is refused unless the
repository's own `[decide] sites` names it, even when a provider is set; `route` leaves the default list (the code
stays). Decision 3 gains a category, judgments the agent directs; decision 5 (no new MCP tool) stands; I14's
enumerated list widens to any explicitly invoked CLI verb whose site is listed in `[decide] sites` (never a hook
process directly). This amendment builds nothing: each site waits on its plan rows in
`docs/plans/judgment-foundations.md`, `docs/plans/test-run-hook.md`, `docs/plans/judged-predicates.md` and
`docs/plans/judged-telemetry.md`.

**Amended 2026-10-09 (owner direction): OpenAI's Decisions API beside Jev, keys by file, and
the key stays with its provider.** `[decide] provider = none | llm | typesafe | openai`;
`openai` is agentic-kit's `foundations/decision/openai` adapter (kit decision 0018: one
port, optional adapters), and `typesafe` gains `transport = "openrouter"` (Jev through
OpenRouter's Decisions router). Both are resolved in `internal/decide.Open` through the
kit's fallback composition, so a provider error or a refused question is one disclosed
line and the site's deterministic path (JF-6), and a refusal is never read as an answer.
Decision models stay optional: `none` is the default and the unconfigured output is
byte-identical (I42). Keys are named, never written: `api_key_file` (a chmod 600 file the
adapter reads at first use; the launchd daemon's only source, since it does not see the
shell's environment), `api_key_env`, `api_key_cmd`; a literal `api_key` is refused at load
naming `api_key_file`. New rule: a key goes only to the provider's own host, a loopback
host, or a host the user layer (`~/.fathom/config.toml`) names — a repository's own config
may not redirect it, and non-loopback plain http never carries it. Providers are
replaceable at the port, not interchangeable without calibration: thresholds were set
against one provider's answers, and `fathom doctor` says a new provider's need a reading.
First live smoke: `docs/bench/readings/decide/provider-smoke-2026-10-09.md`. **Revisit
when** OpenAI's API leaves beta or offers a second model (pinning and the default model
move), an adopter needs a key host beyond the provider's own from a repository's config,
or a calibration reading grades either provider on a site.

**Amended 2026-10-09 (SEC-1, the decision-provider review): credentials and the commands that
fetch them come only from user scope.** `api_key_cmd` (an argv fathom execs), `api_key_file` and
`api_key_env` in `[decide]` and every `[profiles.*]` were honored from a repository's own
`.fathom/config.toml`, so any cloned repository (a bench corpus, an adopter's repo) could make
fathom run a command of its choosing when it opened or doctored that repository. They are now
honored only from `~/.fathom/config.toml` plus the transport's default variable: `config.Load`
drops the field from a repository layer and records the file and field (never the value), `fathom
doctor`'s `key_source_scope` row warns naming them, and `fathom config` lists them as ignored. A
repository may still choose provider, model, thresholds and sites; a key stays bound to the
transport it was named for and the host allowlist stays user-scope (DP-32, DP-39). A successful
`api_key_cmd` runs at most once per process (in memory only, never written). Jev, weighed:
unaffected; the decision port's boundary and the Jev decision port are untouched, only where a
credential may be named moves. **Revisit when** an adopter needs a repository to vouch for a
credential helper (a trust-on-first-use prompt, not a silent read).

**Amended 2026-10-09 (SEC-2, the second security amendment after SEC-1): a repository's config chooses
behaviour only.** SEC-1 moved the key sources; the same review found the rule they broke was
wider. A repository's `.fathom/config.toml` may choose behaviour: provider, model, thresholds,
include lists and repository-relative paths. It may never name an executable fathom runs, name a
credential, name a path outside the repository, or switch on automatic execution. Every field
the loader accepts has one of three scope classes in `internal/config`'s registry (`scope.go`),
and `TestEveryConfigFieldHasAScopeClass` walks the embedded `#Config` schema against it, so a
field cannot land unclassified: **behaviour** (either file; the project file wins),
**repo-path** (a path inside the repository: every use site resolves it with
`config.RepoPath`, which refuses an absolute value, a `..` escape and a symlink escape by name,
whichever file set it: `tests.results_dir`, `tests.artifacts_dir`, `findings.outbox`,
`connector.include`, `work.ci_summaries[].path`), and **user-only** (read from
`~/.fathom/config.toml` only; `config.Load` drops it from a repository's layer and names file,
field and reason, never the value, in `fathom doctor`'s `key_source_scope` row, `fathom config`
and a daemon's startup log). User-only now covers, beside SEC-1's key sources:
`profiles.*.bin` (the codex-cli lane execs it at construction: a repository's `./script` ran at
serve-http and serve-mcp startup and in doctor's providers row), `profiles.*.endpoint` (the
openai lane sends the user's `OPENAI_API_KEY` there), `daemon.store`, `daemon.addr`,
`daemon.call_log`, `daemon.auto_refresh` and `daemon.memory_limit` (the machine's daemon: where
it writes, where it listens, whether it re-ingests on its own; the session hook's outbox sync
passed a repository's `daemon.store` to `findings import --store`), and `[tenancy]` (folded in
from RT-30). `hooks.test_run` and `tests.commands` stay behaviour on the evidence: the test-run
hook runs nothing, it reads the output of a command the harness already ran (TR-X2), and
`tests.commands` is only matched. **Which verbs may run a repository's own commands:** only an
explicit verb the user types with its flag (`fathom doctor --stranger` runs
`scripts/stranger-check.sh`; `fathom receipt verify --run-tests` runs `go test` for the receipt's
test). No hook, daemon path, ingest or prompt hook runs a command a repository names. Breaking:
a repository that set a user-only field has it ignored, and an absolute `work.ci_summaries` path
(documented before as allowed) is refused. Jev, weighed: unaffected; the decision port's
`decide.base_url` stays behaviour under DP-32/DP-39's key-host rule, and only where an
executable, a key host or a machine path may be named moves. **Revisit when** the test-run hook
gains any execution of its own (a re-run, a tier that runs a command: `hooks.test_run` then
becomes user-only), an adopter needs a repository to name a local model endpoint (a
loopback-only exception like DP-32's), or a field needs a fourth class.

**Amended 2026-10-16 (ADR-071): one opt-in site name, `readout_mode`.** A `choice{raw, lean, rich}` for rich-mode
readouts, asked only in the policy table's contested band, through a loopback daemon endpoint the readout hook calls
under its deadline (decision 2's `POST /route` precedent, I14), over features only and never the output text (I61);
provider `none` leaves the table's answer byte-identical (I42). Unlike the other opt-in names it is honoured only
from user scope, because readouts' mode is user-only, so I60 is amended in the same change for this one site: it is
refused unless the user's own `[decide] sites` names it, a repository's naming of it is dropped and named by doctor,
and a miss (a missing key, a timeout, a 429 or 529, refuse or safe mode) is disclosed in one receipt clause when a
readout is served, else in the hook-log row and `fathom readout report`, since a raw result carries no fathom text.
No call site exists until `docs/plans/readouts.md` phase 4 (the owner's budget lift, ADR-056 R3's bar, instrument C).
Nothing else in this record changes. Jev, weighed: ADR-071 carries it. **Revisit when** ADR-071's triggers fire.

## ADR-043: Tested means certified — a certifications layer over declared test evidence, judged elsewhere (2026-09-23)

**Status.** Accepted 2026-09-23. Vision: `docs/vision/33-test-certifications.md`. Design: `docs/design/test-certifications.md`. Plan: `docs/plans/test-certifications.md`.

**Context.** The owner asked whether fathom should track test coverage, test kinds and what tests are for, on a standard defined in platform-engineering. Three repositories already hold most of the pieces and agree on the boundary: test-kit declares suites and attests per-case results (ADR-0042 there); Review Kit judges requirements through evidence checks with adapters and enforcement; fathom knows which tests reach which symbols, which symbols each labelled layer covers and which mutants survive, and states "fathom asserts a fact. The review kit judges it." Nothing links a test to what it is for, and nothing computes which invariants, incidents, specs and ports have no certifying test.

**Decision.**
1. *A certification is a claim.* `test T, of kind K and strength S, certifies target X` is stored, tiered, swept and served exactly like every other claim: content-addressed, cited, `stale` when the test or a reached symbol moves, `contradicted` when the test is gone or the target does not resolve, `refuted` by a human. Layer `certifications`, role `claims`, compiled deterministically from declarations times joins; no model in the default path.
2. *The vocabulary is the platform standard's.* `#Certifies` and `#Suite.certifies` are an amendment to test-kit's ADR-0042; the per-test marker `// certifies: <kind> <id> (<strength>)` is comment-grammar fathom reads at ingest. fathom never invents a vocabulary of its own.
3. *fathom asserts liveness, never adequacy.* A certificate's state (`live`, `flaky`, `broken`, `unproven`, `unreachable`) and tier are facts. No threshold, percentage or "adequate" lives in fathom; a Review Kit requirement with a fathom evidence adapter decides and enforces.
4. *Served through the gate, the review and the list.* `fathom verify` gains `certified_by`; `review_diff` gains a `certifications` dimension; `fathom certify status|gaps|manifest|orphans` are verb-first CLI, no new MCP tool. The edit hook is not a site this round (Q21).
5. *No attested run, no live certificate.* fathom never runs tests; it ingests test-kit's attested envelopes. A reached test is not a passing test; a receipt says `pass: unverified (no run)` until the holder runs it.
6. *The decision port may suggest, never certify.* `certify suggest` asks a strength choice and an asserts noul per unregistered test; proceed-band answers are suggestions in a workspace artifact, never claims (ADR-042's boundary).

**Alternatives rejected.** A coverage percentage as the product (test-kit already declares floors; Review Kit judges; covered is not asserted). fathom running tests (non-goal: no dispatch loop). Model-inferred test intent as truth (tier discipline). A hand-maintained test registry (state is computed, P17). A test quality score (the advisor recommends; the gate decides, P11). A new MCP tool ahead of call-log justification (P4). A fathom-local vocabulary (one standard, three consumers).

**Consequences.** Three repositories change in one round: test-kit (schema, ADR amendment), fathom (this layer and its verbs), Review Kit (a requirement and an adapter). Every stage carries a kill criterion (plan rows TC-M1..M4). The certifications layer is the second deterministic claim layer after findings; if a third appears the deterministic-layer path is factored (ADR-040's "not a findings database" holds: fathom remains the projection, test-kit the record).

**Revisit when.** TC-M1 fails (the envelope needs a case-to-symbol contract first); TC-M2 fails (declaration is too heavy); a non-Go service ingests envelopes (TC-26 fires); Review Kit's requirement never fires deny in a month (TC-M3); or a fourth consumer wants the vocabulary outside the platform standard.

**Amended 2026-09-24 (ADR-044, O114): liveness binds to a commit, and the kill rule counts warn.** Two defects in this record's first round. (1) A certificate read `live` whenever the newest ingested run passed; runs carried no commit and the sweep staled a certificate only when its test FILE moved, so after any refactor every certificate stayed live on a run that predated the change (O114). Decision 1's "stale when the test or a reached symbol moves" and decision 5's "no attested run, no live certificate" now mean: a certificate is `live` only when a passing attested run is bound to a commit (test-kit's envelope `env.commit`, clean tree) and no file the certificate reaches (its test file plus the files of the symbols it reaches) changed between that commit and HEAD. A run without a commit leaves the certificate `unproven` with the reason named; a bound run whose reach changed since makes the claim `stale`. (2) The TC-M3 kill clause ("never fires deny in a month") combined with requirements shipped `enforcement: off` is dead on arrival: an off requirement cannot fire. The clause now reads "never fires at warn in a month on a repository where it is armed at warn", and the testing-certifications requirement is armed at warn on this repository.

## ADR-044: Refactored means conserved — fathom computes what a change must keep, proves it kept it, and catches faked preservation (2026-09-24)

**Status.** Accepted 2026-09-24 (the owner adopted the refactoring assessment in full). Vision: `docs/vision/34-refactored-means-conserved.md`. Design: `docs/design/refactor-conservation.md`. Plan: `docs/plans/refactor-conservation.md`.

**Context.** Agents and codemods now produce refactors cheaply; nothing proves that a large agent-written refactor kept what it had to keep. The field agrees (SWE Refactor Bench: 28 of 520 runs pass its three stages and agents copy the old implementation behind a new name, "Blindness"; RefactorBench: agents 22%, humans 87%; ChangeGuard: a project's own regression tests catch 7.6% of semantics-changing edits). fathom's own data shows the failure that matters: in guards-pilot-8 an agent renamed and inverted the test certifying invariant I28 and another checked off its own plan rows; three of today's pushes failed because line-pinned citations moved; the refactor verbs had two organic calls each in 85,949 while this repo's stanza routed refactors to the lowest-precision tool; and O114 showed certificates survive any code change. fathom already holds most of the facts (typed callers at 1.00 precision and recall on Go, `safe_delete_check`'s proven-zero receipt, certificates, receipts, `review_diff`), but nothing composes them into a statement about conservation, and nothing checks that the proof itself was not tampered with.

**Decision.**
1. *fathom proves; it never writes the change.* Scope's "No transformation or migration engine … Revisit when: never" stands. Agents, codemods, OpenRewrite and ast-grep write; test-kit runs; fathom computes facts and proofs; Review Kit judges adequacy and enforces; the decision port orders attention over the residue.
2. *A refactor is a change under a conservation law, computed from a base commit.* What must stay: the capability inventory (language-neutral keys: `api:`, `route:`, `rpc:`, `env:`, `json:`, `cli:` and configured string surfaces such as `tool:`), the certified behaviour on the touched surface, and the governance bound to the moved code. What must change: whatever the refactor retires is provably gone in the forms a compiler does not check (imports, module dependencies, string and config patterns, kept-but-unused symbols). Base-side facts come from the changed files only (`git show base:path`), so cost scales with the change (P19); there is no second full ingest.
3. *Proof integrity comes first and gates everything else.* A certifying, enforcing or surface-pinning test that was deleted, renamed, weakened (fewer assertions), skipped, build-tagged out, or had its golden data or certifies marker changed inside the range is a finding, after rename normalisation so a legitimate rename does not flag. Certificates bind to the commit their run exercised (ADR-043 amendment above).
4. *A declaration made inside the range never excuses a loss.* Any skip, disposition, `retires:`, `surface_change:`, loosened ratchet or deleted plan row added in base..head is reported as self-declared; the loss it would excuse is still listed, and Review Kit decides. This generalises O56's "an agent may not grade its own row" from one prior plan version to the whole range.
5. *Report planes, never a grade.* `fathom conserve` and the review's `integrity` and `conservation` dimensions report per-plane findings and which inputs were missing (index not at HEAD, failed typed packages, unbound runs, unsupported language). A plane with a missing input reads `unproven`, never `conserved` (I44). No ordinal rung, no score.
6. *Forced channels, not pulled tools.* Conservation rides the gate (`fathom verify` rows with `gone:`, `retires:`, `certified_by`), `fathom review --since`, `review_diff`, the pre-push hook as a warning, and CI as `--strict`; the stanza routes refactors to `rename_impact` and `safe_delete_check` first.
7. *Review Kit judges; the pack is armed.* A `refactoring-conservation` requirement pack with a `fathom-conserve` evidence adapter is armed at warn on this repository from day one. "Complete functional-test coverage" is not the target: every capability in the touched surface keeps a live, unedited, commit-bound certificate, or its loss is declared outside the range. Review Kit's typed-judgment lane (ADR-0104) answers the residue no test covers (satisfied, violated or unknown) and never passes a gate alone.
8. *The decision port gets a narrow, measured role* (ADR-042's boundary): ordering attention over a very large frontier after a deterministic split, and same-concept questions for renames in languages without typed edges, each only after its own A/B and after the port's calibration check (0.9 right nine times in ten on adjudicated cases). It never decides a move, a strength, a scope question or safety, and every site is an explicit egress opt-in because questions carry code off-host.
9. *Claims move with the code.* Citations re-anchor by their anchor text when lines move, so a refactor or a doc edit does not stale a claim whose text is intact.

**Alternatives rejected.** A transformation or campaign engine inside fathom (the Moderne trap, vision 01 §3). An ordinal evidence ladder or safety grade (invites the adequacy judgment I43 forbids). A full re-ingest of the base commit (cost scales with the repo, not the change). Absence proofs for what the compiler already proves (deleted Go symbols). Freezing certifying tests at edit time (legitimate renames must edit them). A model deciding that X moved to Y (would launder a loss). Deleting the `clone` package (the only deterministic Blindness detector). Off-by-default review packs with a deny-based kill rule.

**Consequences.** Three repositories change: test-kit (the envelope records the commit, tree cleanliness and race mode a run exercised), fathom (integrity, inventory, conservation, absence, programs, commit-bound certificates), Review Kit (a pack and an adapter). Existing certificates read `unproven` until runs are re-attested with commits; that is the honest state. Each stage carries a kill criterion (plan rows CV-M1..M8).

**Revisit when.** CV-M1 fails (the integrity check misses the i28 and i36 cells); CV-M2 fails (more than 10% of ordinary commits touching certified tests flag after rename normalisation); CV-M4 fails (the Go surface delta disagrees with `apidiff` below 0.95); the refactoring pack never fires at warn in a month; or a second language reaches measured typed edges (the tier caps in decision 5 then move).

**Amended 2026-09-24 (CV-M1/CV-M2/CV-M4/CV-M5 measured): the CV-M4 revisit trigger fires — the Go surface delta is a textual diff, not a type-identity one.** `docs/bench/readings/cv-m4-apidiff-calibration-2026-09-24.md` ran 38 of the requested 50 consecutive release pairs (cobra, cli/cli, prometheus) against `apidiff`: precision 0.615, recall 0.941, both under this decision's 0.95/0.95 bar. Three disagreement classes, not noise: fathom's `api:` signature `detail` is printed source text, so an `interface{}`↔`any` modernization sweep (zero apidiff-visible change) reads as `changed` on every affected signature (the dominant false-positive driver); a struct's printed field list is compared wholesale, so a compatible additive field or a `gofmt` realignment also reads `changed` where apidiff correctly says compatible; and a type-alias introduction (`type X T` → `type X = T`, a real, apidiff-flagged incompatible change touching every signature that names `X`) is invisible to a textual diff, the one measured false negative. Per this trigger's own terms, the fix is a type-identity-aware surface diff for these two classes (`go/types` comparison, or at minimum normalizing `any`/`interface{}` and applying Go's additive-struct-is-compatible rule) before the surface plane's `api:` findings can be trusted at the precision this decision requires; a later, tracked plan row is where that lands. `docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md` (not itself a named revisit trigger here) found a second, independent gap worth carrying alongside this one: `conserve.Resurrected`'s genuine-range false-positive rate on ADR-025's own DuckDB removal (24 spurious findings — two long-lived, independently-written, interface-conformant backend implementations mistaken for a copy) shows the same-file matching pool needs to require the matching head function itself be new-or-changed in the diff, not merely co-located with one. CV-M1 and CV-M2 both measured clean (design's own bar) and need no amendment.
**Amended 2026-09-24 (wave 3, gap-closing): the CV-M4 revisit trigger is resolved — the Go surface delta now agrees with apidiff at the required bar.** `internal/analysis/inventory/apicompat.go` gives every Go `api:` type declaration a `TypeDetail` (struct field set, interface method set, alias-vs-defined kind) built from the same per-file `go/ast` parse `Extract` already does — no `go/types` pass, no second module load, P19 unchanged — and `inventory.Diff` applies apidiff's own compatibility rules over it: an added exported struct field is compatible, only removing or retyping an existing one is not; an added interface method is compatible only when the base interface already carried an unexported method (apidiff's own asymmetric rule); a flip between a defined type and an alias is always a loss. `any`/`interface{}` are canonicalized at render time for every Go item, not only types. `docs/bench/readings/cv-m4-apidiff-full50-2026-09-24.md` re-ran the full 50 consecutive release pairs the bar asks for (23 cobra, 17 cli/cli, 10 prometheus, chosen to include every example pair the failing reading named): **precision 0.960, recall 1.000 — both at or above 0.95.** One residual false positive remains (cobra v1.9.1→v1.10.0: a dependency's own type alias, `pflag.ParseErrorsAllowlist` for `pflag.ParseErrorsWhitelist`, which needs that OTHER package's declaration resolved — out of this fix's stated scope, named rather than chased, and not enough to move the overall bar). The two extra gaps the full run surfaced beyond the three originally named classes — a `var`'s literal initializer promoted to a named constant, and the same `any`/`interface{}` collapse reaching into a func-literal-valued `var`'s parameter types — are fixed the same way and reproduced as fixtures alongside the original three.

**Amended 2026-09-24 (CV-M5 fixed and re-measured, wave 3): the gap this amendment named above is closed.** `conserve.Resurrected`'s head matching pool (`internal/conserve/resurrection.go`) now admits a head function as a resurrection candidate only when it is itself new in the range (absent at base, by name, in its own file) or its body hash changed in the range (present at base under that name with a different hash) — exactly the fix this amendment's own prior paragraph specified. Both migrations replayed clean per design §3.3's bar: DuckDB genuine 0 `body_resurrected` findings (was 24), vector genuine unchanged at 0; a seeded copy in either migration still flags as a loss (`docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md`'s 2026-09-24 update). The replay also surfaced an unrelated gap, filed as O117 (`.fathom/observations.md`, not fixed here): `fathom conserve`'s default plan discovery (the only invocation shape the pre-push gate uses) hands its plan reader absolute paths where repo-relative ones are expected, so `retires:`/`surface_change:` declarations may be silently invisible on the real repository without an explicit `--plan`.

**Amended 2026-09-25 (Q22, the reading set): fathom computes the residue decision 7 hands to Review Kit.** Decision 7 said Review Kit's typed-judgment lane "answers the residue no test covers" and decision 1 let the decision port "order attention over the residue", but nothing computed the residue: `refactoring.uncovered-behaviour` had no evidence, and Q22 ("what would a reviewer need to be shown to accept a 400-file agent refactor without reading the diff") had no instrument. `fathom conserve` now reports a reading set (design §3.4): every function added or modified in `base..head` in a changed, non-test, non-generated Go file, classed `attested` (a reaching test's latest run passed on a clean tree, bound to a commit after which neither the test's file nor the function's file changed, the O114 rule applied without a declared target), `unattested` (reached, no such run) or `unreached` (no test reaches it over resolved edges). It is counts and lists, never a percentage (I43); it is `unproven` with nothing classed on a stale or partial index (I44); files in languages without typed edges are named `unanalyzed`. It is not a plane: it changes no verdict and no `--strict` exit. The review `conservation` dimension carries it, and Review Kit's `fathom-conserve` adapter makes it the evidence for `refactoring.uncovered-behaviour`, which stays advisory. The decision port's role over the list stays CV-51's deferral (a reading set over 100 items and an A/B corpus); a model deciding whether a changed function's behaviour survived is refused under decision 8. **Kill criterion CV-M12:** on two real ranges of this repository with runs attested at each head, the reading set plus unanalyzed files must name fewer files than the range changed, and 20 hand-checked items must each be a genuinely changed function in the right class; else it comes out.

**Amended 2026-09-25 (CV-M13, execution evidence): the reading set prefers execution over static reach.** CV-M13 calibrated static-reach attestation against Go coverage (`docs/bench/readings/cv-m13-reading-set-calibration-2026-09-25.md`). Over one head of this repository, 3.7% of functions static reach called attested never executed, and 31 of 91 it called unreached executed through the standard library, the runtime or package initializers. The test named as voucher was always a hub. And a function that ran could still carry a changed line that never did (`runServeHTTP`'s O119 fix). So `task tests:attest` now writes a Go coverage profile from the same run, bound to the commit by `coverage.meta.json`. When that profile is usable (bound, clean, passing, and the function's and test's files unchanged since its commit; I44), the reading set classes each changed function by its changed blocks: `attested` when every changed block executed, `partial` with the unexecuted line ranges, or `unexecuted`. Without a usable profile it falls back to static reach and says so per item. Counts reconcile through `unanalyzed_functions`. Test cases join by package when a name is shared. Review Kit fails `refactoring.uncovered-behaviour` on any item that is not `attested`. No percentage is computed (I43), and the decision port has no role. **Kill criterion CV-M14:** on the CV-M13 ranges with coverage, nothing attested is unexecuted and nothing flagged executed, and the named cases class as predicted. Separately, a `-coverpkg` attest run costing more than twice the plain run makes the profile opt-in.

## ADR-045: The Observatory becomes a console — every panel renders a verb, every number carries its instrument, and fathom returns what the kit lacks (2026-09-25)

**Status.** Accepted 2026-09-25 (owner directive: "come up with the vision of what the UI/UX should be, be very ambitious … execute it to completion, adopting and contributing back to the app-kit"). Vision: `docs/vision/35-observatory-console.md`. Design: `docs/design/observatory-console.md`. Plan: `docs/plans/observatory-console.md`.

**Context.** Seven Observatory pages were each honest alone but showed almost none of what fathom knows. That includes 1,057 plan rows with proof states, 4,025 governed claims, 82 compile runs, 71 certification targets, the reading set, 88,980 logged calls and 41 doctor checks per repo, all of it CLI-only. The pages also contradicted each other: a wiring grade and a realization grade shared one name, "≥114" became "114", ledger rows were counted as sessions, colored zeros stood in for tools with no instrument, and call totals differed by directory because CLI verbs read a directory-relative log. Nothing explained how a rule reaches an edit or what a layer or dialect pack is.

**Decision.**
1. *The Observatory covers three jobs.* Steer the fleet and each project; understand fathom's machinery at work on your own code; operate fathom itself. Pages are grouped Now, Fleet, Proof, Knowledge, Evidence, System and Learn (vision 35 §3).
2. *Every panel renders a verb (P10).* Page projections live in `internal/console` and are shared with the CLI verbs. Each panel names its verb. Each page's JSON must equal the JSON of the verbs it names (`TestConsolePagesRenderTheirVerbs`).
3. *One projection per metric.* A metric shown on several pages is computed once. `TestConsoleMetricsAgreeAcrossPages` fails on any disagreement. Every value carries a truth state (present, zero, absent, stale, partial, error) with reason, source, denominator and time.
4. *The call log is resolved once.* `console.ResolveCallLog` prefers the daemon's configured log. Metrics verbs and pages both use it.
5. *Read-mostly, local only.* Wiring preview/apply/undo stays the only mutation, behind `requireLocalMutation`. Everything else shows the command that fixes a problem. Law 13 is unchanged.
6. *Live freshness is admitted.* It supersedes vision 17 §5's "not now" for live updates only: the fleet model is 297 KB and takes 2 s cold, and a meta-refresh would re-render it all. `/events` streams names and IDs over SSE, and panels show "updated" without moving content.
7. *App-kit fully, and just in time.* Fathom adopts the current mirror's shipped packages. It builds what the kit lacks — palette, layered graph, heatmap truth states, code excerpt, unified diff, live freshness, page heading, catalog entries — as the originating consumer under the kit's contribution loop (ADR-0066). Each stays experimental until a second consumer adopts it. Domain views (proof ledger, strata, serving explainer, dialect matrix, attention queue) stay fathom's.
8. *The decision port gets one hedged job.* It may suggest a view for a free-text palette query at the existing route site. The site is off by default and needs an egress opt-in, and it never answers or ranks. Kill: under one in three suggestions followed over 30 days.

**Alternatives rejected.**
- A client-side SPA or chart library: it adds a Node toolchain and a second state model, and violates P10.
- A separate console binary: it would duplicate the daemon's stores.
- Chat over the index: a standing non-goal.
- Auto-refreshing pages: they move content under the reader.
- Hand-rolled look-alikes of kit components: doctrine rule 1.

**Consequences.**
- `internal/console` becomes the projection layer.
- `cmd/fathom/console_*.go` hold the pages, and `cmd/fathom/ui/input.css` scans them by glob.
- Old `/observatory*` routes redirect. Their JSON (observatoryModel v1) is unchanged.
- `obligations` gains an additive `--explain`, and `intent audit` gains `--read-only`.
- App-kit gains experimental capabilities carrying fathom contribution records.

**Revisit when.**
- UX-M1 fails: an operator task needs more than three clicks from Now, or a page answer differs from its verb.
- UX-M4 finds pages nobody opened in 30 days of a real consumer population.
- A render budget is missed by the SVG renderers (then OC-61's trigger fires).

## ADR-046: The assess lanes — typed judgments of on-disk work through the decision port, spend kept out of every unit (2026-09-25)

**Status.** Accepted 2026-09-25. The shape (two lanes, never transcripts, explicitly invoked, priced, isolated from the benchmark) was owner-authorized on 2026-08-14 (`docs/design/continuity-threads.md` §4.1, `docs/design/intervention-outcomes.md` §1). This record settles what those documents left undesigned. Design: `docs/design/assess-lanes.md`. Rows: DO-3, IR-3, IR-4, IR-5 in `docs/plans/ambient-fathom.md`.

**Context.** `docs/intent/scope.md` G4 listed DO-3 and IR-3/4/5 as the undesigned LLM assess lanes. The mechanical lanes already existed: DO-1's derived outcome and IR-2's `fathom retro`. What was missing was the priced lane for the rows mechanics cannot decide, a meter that keeps judging spend out of the A/B and the continuity bet, and a sanitized route for the retro corpus into the research flywheel.

**Decision.**
1. *Every judgment is a typed decision-port question.* Binary properties are noul and levels or outcomes are choice. Nothing is a score, because the port's score answer carries no spread and would always band as proceed. No free-form judge exists, and no transcript is read (DF-4 stands).
2. *The only egress is the decision port* (`[decide] provider`, the new `assess` site). I14 is amended to name the two CLI verbs that open the port (`certify suggest`, `assess`), still never a hook. The `llm` provider is the existing llm profile machinery.
3. *Consent is a capability, `assess`, default off and fail-closed.* A malformed capabilities file refuses the run, the opposite of the safety capabilities, because an unreadable file is never consent to spend. A live run needs, in order: the capability, a pre-flight token cap, a provider, and the site. `--dry-run` needs none of them and spends nothing.
4. *Spend isolation is pre-registered as `IR-4@2026-09-25`, before any assess run.* Assess spend is metered on its own ledger (`~/.fathom/assess-ledger.jsonl`) and priced through the compile meter's table. The judge process inherits `FATHOM_CHANNELS=off` and `FATHOM_SPEND_LABEL`. A session-ledger row carrying a spend label is never an arm unit, a thread-bet unit, or an admissible flywheel arm row; each exclusion is counted and named. Invariant I46.
5. *Verdicts are banded and uncalibrated until measured.* Proceed is the verdict, confirm is leaning, unsure is undecided. The label lifts per question-set digest only after proceed-band agreement reaches at least 9 in 10 on at least 10 adjudicated units (ADR-044 decision 8's bar). Until then verdicts stay local and feed no figure.
6. *The retro corpus reaches the flywheel only through the bundle*, behind `--include-retros`, as a counts-only group over closed labels. This is additive to the closed CUE allowlist, and absent means not read.

**Alternatives rejected.**
- *A free-form LLM judge with a rationale.* It is unbandable, uncalibratable and needs a parser, and prose invites quoting as evidence (law 14). A rationale field is reconsidered only if adjudicators find more than half of proceed-band verdicts unusable without one.
- *A new egress beside the port* (an `--profile` client in the verb). It widens I14 for no capability the port lacks.
- *Metering assess inside `compile_runs`.* It puts a non-compile row into the store's compile ledger (a sensitive surface). The assess ledger reuses the meter's pricing and aggregation instead.
- *Adopting `qa-harness` now* (ADR-023's trigger). There is no corpus or baseline until the adjudicated sample exists.

**Consequences.**
- One new pure package, `internal/assess`, plus a `fathom assess` verb with a `spend` subverb.
- One capability, one `[decide]` site, and one additive session-ledger field (`spend_label`).
- One additive research-bundle group (`retros`).
- `buildRetroModel` is shared by `retro` and the intervention lane.
- ADR-023's "Revisit when" fires (an LLM-judged evaluation lane now exists). Disposed 2026-09-25 by the owner: it stays bespoke, and `qa-harness` adoption is evaluated at M-A1 (recorded on ADR-023).

**Revisit when.**
- A question set's proceed band fails M-A1 at every threshold: remove the question.
- `llm` and `typesafe` agree on less than 80% of proceed verdicts for the same units (M-A2): the question is provider-specific.
- The adjudicated sample exists (M-A1): ADR-023's `qa-harness` adoption point.
- IR-3's "guidance as served now" measurably differs from what was served at firing time: record the served set at IR-1.

**Amended 2026-09-27 (ADR-053, the surface reckoning).** Status becomes *Accepted —
frozen 2026-09-27*: both `fathom research` (decision 6, the retro corpus's only route)
and `fathom assess` leave `fathom help`, the `assess` capability stays default off, and
neither lane takes new rows until an owner pull supersedes this record.

## ADR-047: The responsible tenant — fathom declares a footprint budget measured by the OS, enforces it itself, and no request costs more than its answer (2026-09-26)

**Status.** Accepted 2026-09-26. Owner directive of 2026-09-26, after the incident recorded as INC-23: "put measures into place to keep fathom from being a problematic tenant again. put a lot of design effort and implement completely to make it much more responsible and safe." Vision: `docs/vision/36-responsible-tenant.md`. Design: `docs/design/responsible-tenant.md`. Plan: `docs/plans/responsible-tenant.md` (RT-1..). Invariants I47–I53; principle P23.

**Context.** On 2026-09-26 the launchd daemon reached about 29 GB resident on a 32 GB machine, filled 21 GB of swap and made macOS jetsam a dozen neighbouring processes; the operator force-quit it five times in 17 minutes and launchd relaunched it into the same storm each time. The heap profiles attribute it to request paths doing work proportional to repository or fleet size: in one 62-minute window request paths allocated 730.7 GB (obligations' two whole-tree reads per edit 468.6 GB, `ServiceSummaries` loading every symbol row of every service 133.8 GB, `fleetCallers` 31.1 GB), multiplied by concurrency (49 concurrent `list_services` at 05:24:08Z), by abandoned work (every RPC runs under `context.Background()`; calls ran 19 minutes after their 2-second hook clients left) and by inline re-ingest (a read that sees drift re-ingests the whole service, app-kit peaking at 5.6 GiB). The only bound was `runtime/debug.SetMemoryLimit(3 GiB)`, a GC hint the Go runtime documents as soft. The audit found 82 hazards across request paths, background work, process hygiene and observability; the 4200482a hotfix fixed one call site. The earlier O108 incident (2026-09-22) was closed as "mitigated-and-bounded" on a reading of `HeapSys`, a field that never falls.

**Decision.**
1. *A declared budget, measured on the OS quantity.* The managed daemon's budget is `phys_footprint` (Linux: RSS + swap), default `clamp(RAM/8, 1 GiB, 4 GiB)` — 4 GiB on the 32 GB reference machine — set only at user scope (`[tenancy]` in `~/.fathom/config.toml`; a repository's `.fathom/config.toml` cannot raise a machine's ceiling). `HeapSys` is never read as a footprint again; the soft memory limit stays, re-derived as a GC hint at the budget.
2. *A ladder the daemon enforces on itself,* sampled every 2 s from `phys_footprint`, `runtime/metrics`, host memory pressure, swap and volume free space: **shed** at 0.75 × budget or host pressure warn (drop caches, pause background work, serve stale with disclosure); **refuse** at 1.0 × budget or pressure critical (heavy classes answer a named 503, diagnostics captured); **exit** at `min(2 × budget, RAM/4)` for two samples (write the reason, exit 70). Hysteresis: 60 s below 0.6 × the rung to step down.
3. *Admission control by cost class.* Every tool, in-process console call, batch sub-call and console job is admitted by a cost class (`point`, `index`, `tree`, `fleet`, `ingest`) with slots and per-call byte ceilings scaled from the budget so the admitted envelope is ≈ 0.5 × budget; the wait depends on origin (a hook never waits for heavy work). Overload is HTTP 503 + `Retry-After` + JSON-RPC `-32050` naming the reason; a budget exhausted mid-work returns a partial result that names its lower bound (I38). Registration without a cost class fails at init.
4. *Read work dies with its caller.* Read tools run under `r.Context()` with a class deadline and the client's `X-Fathom-Deadline`; only ingest and single-flighted memo builds run detached, and a flight is cancelled when its last waiter leaves unless it is past half its expected cost. `serve-http` uses an `http.Server` with header, idle and per-route write timeouts.
5. *No request path is proportional to repository size (P19 made structural).* Query-side code cannot name a whole-tree read: `Sources` leaves `mcp.QueryInterface`; request code reads through `internal/corpus` (stat-capped single files, budgeted streaming scans). What a request needs about "the whole repository" is derived at ingest and stored per watermark (file stats, service summary, reverse call index, routes, imports). Discovery honours `.gitignore` in git worktrees and never reads a file over 2 MiB. A tenancy lint with an only-falling allowlist, per-tool allocation tests at 1× and 10× fleet scale, and cold-stampede tests keep it true.
6. *Ingest leaves the request path.* One refresh queue: one ingest per service (single-flight plus a dirty bit), one heavy slot machine-wide, demand-ordered, staggered after a start; reads answer from the current index with `index_stale` (I4/I7). Ingest streams through a 64 MiB in-flight byte window, reads only files whose stat changed, runs the typed and CUE passes only when a `.go`/`go.mod`/`go.sum`/`go.work` (`.cue`) file changed, and advances the watermark only at commit.
7. *One cache registry.* Every derived in-memory structure is a `memo` group: single-flighted with context-aware waits, byte-accounted under one budget (0.25 × budget), evicted on a new watermark, dropped on shed and after 30 minutes idle. I21's count bound becomes a byte bound.
8. *launchd posture and a breaker.* Plist schema 2: `KeepAlive {SuccessfulExit: false}`, `ThrottleInterval 60`, `ProcessType Background`, `LowPriorityIO`, `LowPriorityBackgroundIO`, `Nice 5`, `ExitTimeOut 20`, `--managed`. Exit 0 means "stay down"; self-restart adoption exits 75. A lifecycle ledger records every start and exit (an unclean predecessor is named at the next start); 3 counted exits in 30 minutes start in safe mode (no auto-refresh, no heavy classes), 5 in 60 minutes (or a safe-mode over-budget exit) writes `disabled.json` and exits 0. `fathom daemon install --reconcile` migrates existing plists, keeping the operator's label, paths and environment; doctor names drift.
9. *An operator kill switch that needs nothing working.* `fathom daemon stop|start|pause|resume`; `~/.fathom/daemon/disabled.json` is read before config, store or bind and makes the daemon exit 0 in under 50 ms; hooks `stat` it and stay silent.
10. *Scratch daemons cannot outlive their parent.* A daemon not started with `--managed` exits on parent death, after 20 idle minutes, after 8 hours, or when its executable or store is unlinked; it runs no auto-refresh, feeders or self-restart watcher, and refuses the managed daemon's store and call log. `fathom daemon ps|reap` is the census. Tests reach no operator daemon: a shared `TestMain`, a `testing.Testing()` URL backstop, and a 409 from the managed daemon to `test=1` callers.
11. *Hooks carry one deadline and share a breaker.* One monotonic deadline per hook process (the harness `TimeoutSec` minus max(1 s, 20%)), never a retry after a timeout, and a breaker file per daemon URL with a single half-open prober.
12. *Visible where the operator already looks, even when the daemon is down.* A persisted 24-hour resource ring and lifecycle ledger; the `fathom resources` verb; doctor rows judging `phys_footprint` from the ring (never SKIP into silence); a primer attention line only when not normal; a Tenancy panel on Operate (ADR-045: it renders the verb); `/statusz` and `/metrics` families; automatic heap, goroutine and flight-recorder capture on thresholds; pprof and `/debug/inflight` on a private unix socket; every TCP route requires a loopback Host.
13. *Disk has retention.* `fathom gc` (dry-run by default) rotates and gzips the call log and ledgers (evidence is archived, never deleted), ages out caches, stamps and dead daemon records, and reports bench artifacts; the WAL is capped by `journal_size_limit`.

**Alternatives rejected.**
- *Tune the soft limit instead of a budget.* The Go GC guide documents the limit as soft and advises against it where the environment is not controlled; the incident exceeded it by 10×.
- *Rely on the OS to cap the daemon* (`HardResourceLimits.ResidentSetSize`). launchd documents it as advisory on macOS; systemd's `MemoryHigh`/`MemoryMax` are mapped for Linux adopters, not relied on here.
- *Byte-weighted admission reservations from an EWMA of past calls* (proposal A). Per-call byte ceilings times slots already bound the admitted envelope deterministically; reservations add an estimator with no counterfactual. Reconsidered if RT-M3 shows the envelope exceeded with slots alone.
- *Cap the SQLite read pool.* ADR-020's uncapped pool stays; concurrency is admitted upstream and `PRAGMA soft_heap_limit` bounds SQLite's own memory. ADR-020's premise ("capping the pool would throttle concurrent readers") is refined, not reversed.
- *Idle-exit the managed daemon with socket activation.* A cold start is the stampede that caused the incident; idle *shrink* instead. Reconsidered once RT-M5 shows a relaunch under load stays inside the budget.
- *Only a watchdog* (the seatbelt without the brakes) or *only root-cause fixes* (brakes without a seatbelt). The first would exit monthly and hide the cause; the second leaves the next regression unbounded.
- *The decision port anywhere in admission, shedding, exit, eviction, refresh gating or hooks.* These must be deterministic, local and sub-millisecond (P2, P11, P18); I14 forbids the port in hook processes. The port is itself admitted and is closed at refuse and in safe mode.

**Consequences.**
- New packages: `internal/tenancy` (budget, sampler, ladder, gate, breaker, ring, lifecycle, kill switch), `internal/memo`, `internal/corpus`, `internal/refresh`, `internal/hookguard`, `internal/testisolate`, `internal/lint/tenancy`. `internal/resultcache` is retired. `golang.org/x/sync` becomes a direct dependency (already in the module graph).
- Breaking (each under `### Breaking` with a migration note in `docs/upgrades/`): self-restart exits 75; plist schema 2; `references` defaults to the caller's own service (`services: ["*"]` opts into the fleet); discovery honours `.gitignore` (`[connector] include` re-includes); unmanaged `serve-http` defaults; pprof moves to the unix socket; test callers get 409; JSON-RPC `-32050` and the 503 envelope join the compatibility surface.
- ADR-017 amended: a service whose stored watermark is a git SHA never takes the content-manifest path; a git failure is "freshness unknown", disclosed. Genuine non-git corpora use a capped stat manifest.
- I21 amended by I51 (byte bound, one budget). `docs/design/observability-surfaces.md` §7's "no alerting" is not breached: self-protection and its disclosure (a primer line, one rate-limited local notification on a breaker stop) are not alerting; nothing pages and nothing leaves the host.
- Corrected in the build waves, not left standing: O108's "mitigated-and-bounded", O112's "eliminated at the mechanism level", `CHANGELOG.md`'s "bounded at the peaks", `docs/guides/fleet-index.md`'s `heap_sys` advice, `memorylimit.go`'s comment and `docs/configuration.md`'s `memory_limit` paragraph.

**Revisit when.**
- The exit rung fires more than once a month on legitimate work after the request-path and streaming waves land (RT-M1): raise `auto` to RAM/6 or retract the exit rung to refuse-only.
- More than 5% of hook calls are refused outside shed/refuse windows (RT-M2): the slots are too tight.
- `ProcessType Background` raises post-edit hook p99 by more than 25% (RT-M6): revert to `Standard` with niced children.
- The GC CPU fraction exceeds 25% for 5 minutes below the refuse mark (RT-M7): move the GC hint to the exit mark.
- A non-macOS adopter needs the systemd mapping, or budget `auto` refuses a registered corpus's legitimate ingest after streaming ingest ships.

**Amended 2026-09-26 (design review, same day).** A review of the vision, design and plan found that the first rollout would not have stopped the next incident on its first deploy, and that several mechanisms would misfire on a healthy machine. The decisions above stand except as follows; the vision's §10 and the design carry the detail.
- *Decision 2 (the ladder).* The swap-used fraction is no longer a trigger (macOS grows swap files on demand; this machine read 80% used at pressure level 1): host triggers are pressure level, swap growth over 2 GiB in 5 minutes while fathom is past the shed mark, and swap-volume free space. An `Unknown` pressure or swap reading is disclosed and never a trigger; an unknown footprint disables the ladder, disclosed. The children's footprint (`go list`, git, rg) counts toward shed and refuse. Every byte producer reads the ladder's level before each file or 1,000 rows and halts (tree, fleet and ingest at refuse; all at the exit mark), because 2 s sampling cannot bound an I/O-bound stampede. Refuse cancels every in-flight tree and fleet request, youngest first. A sample more than 3× late counts as a second over-mark sample; the sampler does not allocate. At the exit mark an in-flight ingest is halted first and its service quarantined (never re-queued until an explicit `ingest`); the daemon exits 70 only if the next sample is still over, and without a drain. Shed trims the caches to half their budget with one rebuild flight; only refuse drops them. The GC hint defaults to the exit mark, not the budget.
- *Decision 3 (admission).* A tool's class reflects its current measured cost and moves down only in the commit that rewrites it, with its cost assertion. The ≈ 0.5 × budget envelope holds only from wave C; in waves A–B the cause fixes, the ladder and the producer governor bound memory. Admission defaults to `observe` until RT-M0 reads on the post-wave-C binary.
- *Decision 4 (cancellation).* Before read work follows `r.Context()`, `ensure()`'s inline ingest runs detached under one process-wide ingest ticket, records `seen` only after success, and is skipped at shed and above, in safe mode, during warm-up and for a quarantined service; the watermark advances only at the final commit (moved from wave D to wave B).
- *Decision 5 (request cost).* Wave A lands schema-free forms of the obligations, `list_services`, "is X indexed" and connector fixes, plus two process-wide `Sources` slots that hooks never wait for; the schema-backed forms replace them in wave C. `.gitignore` discovery keeps the built-in `skipDir` exclusions and flips its default only after a fleet-wide dry-run shows no service losing more than 1% of observations outside ignored directories. The new derived tables are backfilled by SQL, not by a fleet re-ingest. `references` without `services` and without a resolvable caller identity (agents over HTTP send none) is a named `-32602` listing the choices. The lint keeps only syntactic rules.
- *Decision 8 (lifecycle).* SIGTERM writes its exit row first, drains within 12 s in total and exits 0 (stays down). `task deploy` writes a deploy-intent row so its kickstart is never counted by the breaker, and fails when the signing identity is missing. Under an installed schema-1 plist, `stop` and the breaker say launchd will relaunch and print the `bootout` line. `disabled.json` and `paused.json` govern only the managed daemon. `serve-http --lifecycle-id` lets the stress harness exercise the breaker.
- *Decision 12 (visibility).* The diagnostics unix socket is deferred (pprof stays behind the loopback Host check, capped, one profile at a time) until the stress harness shows `/metrics` unreachable under load. Disk writes gain a declared budget (≤ 2 GiB/day outside explicit ingest), disclosed.
- *Scope.* Linux gets an RSS reader and `Unknown` for the rest; PSI, cgroup budgets and a systemd unit wait for a Linux adopter (the systemd mapping stays documentation).
- *Alternative weighed and deferred: ingest in a child worker process.* It would return ingest memory to the OS on exit (removing the O108 retention class), could be niced and budgeted separately, and killed without killing the daemon; SQLite's single writer is unchanged if the worker holds the daemon's ingest slot. Cost: a process per ingest and a second store opener. Streaming ingest and the producer governor bound the peak first; the measurement that settles it is RT-M4's ingest peak and the daemon's footprint 10 minutes after an ingest (trigger: above 1.25× its pre-ingest footprint, plan RT-D6).
- *The decision port.* Unchanged: excluded from every amended mechanism; the reviewers found its weighing sound.
- *Revisit when (added).* The daemon spends more than 2% of a week in shed or above, or enters a host-triggered rung at normal pressure (RT-M13); the typed pass does not fit a RAM class's exit mark (RT-M10); a service is quarantined twice in a month for a legitimate ingest; `/metrics` is unreachable under load (RT-D7).

## ADR-048: Language breadth and depth — four measured support levels, dialects as data on two engines, embedded compressed grammars with pinned packs, no language above its reading (2026-09-27)

**Status.** Accepted 2026-09-27. Owner directive of 2026-09-27: "our language support is mostly Go. I want to be very competitive: AST symbols and dialect packs for the tooling that exists out there … Keep pressing until we have excellent language and tooling support — efficient tokenomics, background tasks with appropriately sized models, most common first, until we have a very respectable number." Vision: `docs/vision/37-language-breadth.md`. Design and worker recipe: `docs/design/language-breadth.md`. Plan: `docs/plans/language-breadth.md` (LB-…); queue: `docs/plans/language-breadth.queue.json`. Promotes futures F1; answers Q6; un-defers XT-6, XT-6b, XT-7. Invariants I54, I55.

**Context.** Fathom extracts 17 languages and about 16 tool formats, but only Go and CUE carry native types and only five languages run on the generic tags-query engine (ADR-021 §4); nine run on hand-written Go walkers over CGO tree-sitter and three on regular expressions. The tags engine never fills `EndLine` or `Signature`. No symbol extractor has ever been measured against an independent oracle (the calibration harness compares to a grep proxy), and typed edges beyond Go and CUE are measured for TypeScript and Python only. The field advertises 40 to 370 languages, none of them with a published recall. The owner wants breadth and depth, most common first, executed by a long-running loop of cheaper workers.

**Decision.**
1. *Four support levels with thresholds and an oracle rule.* L1 symbols (kind, range, qualified name, package, signature), L2 syntax call edges, L3 imports and file-resolved edges, L4 typed edges (native or SCIP import). Excellent L1 is recall and precision ≥ 0.95 on the kind intersection with kind agreement ≥ 0.95, range agreement ≥ 0.90 and signature coverage ≥ 0.90; L2 excellent is call recall and precision ≥ 0.90; L3 excellent is import resolution ≥ 0.95 and file-edge precision ≥ 0.95 with recall ≥ 0.50; L4 excellent is definition mapping and typed recall ≥ 0.95; "good" thresholds sit below each (design §1.2). Tool formats use L1–L3 with exactness thresholds (≥ 0.98 for entities) and never claim L4. Oracles are tiered: A semantic (SCIP, native type checker, the tool's own evaluation), B independent syntactic (universal-ctags as a subprocess), C independent query over the same grammar, D grep proxy or author-labelled sample. Excellent needs A or B; C caps at good; D proves only "tolerated". Every level also requires faults ≤ 0.1% of files (each named), ≥ 1 MB/s/core, no file over 2 s, and module memory inside RT-107's cap.
2. *A level is a reading, not a registration.* The harness (`indexbench lang-oracle`) writes `docs/bench/readings/lang/<id>.json` with `level_met` computed from metrics and tiers; the tracker is generated from manifests, readings and the queue, and a test refuses a tracker level above the reading (I54).
3. *Two engines; dialects are data.* Syntax languages run on the one tags-query engine, grown with host-side predicates (`#eq?`, `#match?`, `#any-of?` and negations), ranges, `@body`-cut signatures, `@name`-captured references, package captures, one-level injections and generalized import captures, so upstream `tags.scm` and aider queries are usable nearly verbatim. Structured tool formats (YAML/JSON/TOML/XML) run on one document-path engine with declarative selectors, generalized from the six bespoke config extractors (P20). A dialect is a directory — manifest, queries or selectors, provenance, fixtures, goldens — and adding one needs no Go. Per-language Go is limited to a closed set of L3 module-resolution strategies (adding one is a foundation change) and the native typed passes.
4. *Heuristics retire on measured parity.* Each CGO walker and regex extractor has a port item; the dialect replaces the incumbent when every L1 metric is at least the incumbent's on the same oracle and subjects, keeping the incumbent's `SourceKind` strings, with an `ExtractorEpoch` bump. When the last walker goes, the CGO tree-sitter modules leave `go.mod` (F1 promoted; its Promote-when is this protocol).
5. *Grammar distribution: embedded, compressed, lazy; pinned packs for the long tail.* Every blob is built from pinned upstream source by the existing monolithic wasm32-wasi recipe, recorded in `grammars.lock.json` (repo, tag, commit, tarball sha256, blob sha256, sizes, SPDX), committed deflate-compressed (measured 4.9× smaller) and embedded, decompressed and compiled only on first use, with wazero's compilation cache on disk at user scope. Embedded compressed grammars are capped at 40 MiB; beyond it the least-demanded dialects ship as dialect packs — the same directory plus its blob — installed at user scope from a file or directory and loaded only when their sha256 is pinned by the binary's lock or listed in the user-scope trust list. A repository's config can neither install nor trust a pack. The daemon never fetches; a fetch verb waits for a public release host (F2).
6. *Licence and provenance rule.* Grammars, queries and fixtures only from MIT, Apache-2.0, BSD-2/3-Clause, ISC, Unlicense, CC0-1.0, Zlib or 0BSD sources (MPL-2.0 for unmodified files only); GPL-family, SSPL, EUPL, PolyForm, BUSL, FSL, Commons Clause and unlicensed sources are refused. Every borrowed file names its source, commit and licence; a test refuses a blob or query the lock does not describe (I55). GPL tools are used only as separate processes on the measurement path, never linked, vendored or shipped. GitNexus (PolyForm Noncommercial) is not read for code or queries.
7. *The tenancy guard.* Grammars load lazily; at most six parser modules live process-wide (LRU, idle-closed after two minutes); compiled grammar code is charged to the memo registry's byte budget (I51) and evicted at shed; RT-107's 64 MiB module cap, the 2 MiB file cap, the binary sniff and a new minified/generated sniff apply to every dialect; no dialect may raise the daemon's footprint budget, proven by a polyglot fixture test and an operator reading.
8. *One interleaved queue, most common first.* Languages and tool formats share one queue ordered by demand (survey usage × fleet census × cross-repo value ÷ expected cost), in waves that mix upgrades, new languages and tool formats. The queue and the plan rows are facts fathom serves; the loop that consumes them belongs to the harness (vision 24). Foundation rows and triage run on Opus, queue items on Sonnet with the recipe as their whole brief, re-measurement and tracker regeneration on Haiku.
9. *The decision port has no role in extraction.* Extraction must be deterministic (P2); ambiguous extensions resolve by deterministic content sniff with the ambiguous count disclosed. The port stays out of classification, extraction, measurement and the tracker.

**Alternatives rejected.**
- *Per-language Go extractors for new languages* (the CGO walkers' shape). Each costs code a cheap worker cannot write safely and a reviewer must read; query files are data, reviewable against a grammar and borrowable under permissive licences.
- *Downloadable grammars as the default.* Breaks offline and air-gapped use, adds a supply-chain step to every adopter, and saves only binary bytes: embedded compressed blobs are clean file-backed pages that cost no resident memory until a language is used. Kept as the overflow and private-dialect path.
- *Emscripten side-module grammars (npm tree-sitter-wasms, @vscode/tree-sitter-wasm).* ADR-021 §4 measured why: a dynamic linker and a JS-shaped callback shim to reimplement in Go. The monolithic build stays the only shape; prebuilt packages are provenance references, not inputs.
- *Language servers in the daemon (Serena's shape) or per-language type checkers.* Scope's "no IDE language services" non-goal and ADR-041's rejection of per-language typed passes stand; SCIP import is the typed path.
- *Counting grammars.* A bundled grammar without a reading is "tolerated", printed as such (law 3).
- *A general cross-file resolution engine (stack-graphs).* Archived upstream in 2025; a closed set of measured strategies reaches L3 for the languages that carry most code.
- *The decision port for ambiguous classification.* Non-deterministic by construction (decision 9).

**Consequences.**
- New: `internal/extractor/dialects/<id>/` (embedded), `internal/langbench` (reading schema, level computation, queue loader), `indexbench lang-oracle|lang-probe|lang-parity|lang-tracker`, `grammars.lock.json`, `schema/dialect/`, `calibration/corpus.d/`, `docs/bench/readings/lang/`, `fathom dialect list|install|verify`, the `[dialects]` user-scope block.
- Changed: `TSExtractor`'s registry comes from manifests (the five tags languages migrate with zero output diff); classification consults manifests; the unsupported-language table is generated from the queue; `docs/languages.md` becomes generated; `build.sh` builds one blob from the lock; blobs are committed as `.wasm.gz`.
- Removed at the end: the nine CGO walkers, the three regex extractors, the CGO tree-sitter modules in `go.mod`.
- ADR-024's dialect row is refined: a dialect is still fathom-authored and trusted by goldens and calibration, still not a pack in the taxonomy's sense; its representation becomes data and a pinned dialect pack may be installed at user scope.
- ADR-013 gains a closing condition (LB-F19).
- The compatibility surface grows by the reading JSON shape (`docs/bench/readings/lang/*.json`), the manifest schema and the `[dialects]` config block.

**Revisit when.**
- Embedded compressed grammars reach the 40 MiB cap (LB-M4): decide the pack host (F2) or raise the cap with a measured binary-size cost.
- A dialect's corpus run pushes the operator daemon's `phys_footprint` up by more than 64 MiB after a polyglot ingest (LB-M3), or the six-module pool measurably serializes ingest (> 1.5× wall time on the polyglot corpus).
- Three or more queue items end `deferred` on the same missing L3 strategy or oracle: the strategy set or the oracle rule is wrong.
- A fourth L3 strategy would be a parameterized copy of an existing one: consider a resolution DSL.
- Universal-ctags changes licence or JSON output, or the SCIP governance transition (announced 2026-03) retires a producer the tracker depends on.
- A language without a maintained SCIP producer becomes a primary fleet target (ADR-041's trigger, restated): its per-file resolver graduates to a typed pass.

**Amended 2026-09-27 (ADR-054, the support matrix).** Decision 5's "pinned packs for the
long tail" moves from planned to deferred: dialect packs wait for embedded compressed
grammars to near the 40 MiB cap, and the Support page's dialects view showing the running
total against the cap is the trigger.

**Amended 2026-09-28 (owner decisions, round 2; design §1.5).** D1: decision 1's
"≥ 1 MB/s/core" is judged on CPU time — bytes per CPU-second of the extraction process —
not wall clock, so contention from other workers never fails a reading.
D6: a queue item depending on another queue item is eligible once the parent has a
committed reading at L1 or better, not only once the parent's row is checked.
D2: where universal-ctags is demonstrably deficient for a language, the dialect is measured
against a stronger oracle (scip-typescript, scip-java or a JavaParser dump, scip-dotnet or a
Roslyn dump; SQL on the families its grammar supports); node, a JVM and .NET are bench
instruments only. D5: every language dialect gets an L2 calls oracle — SCIP references
(tier A) or the grammar's upstream `tags.scm` references in an independent runtime (tier C).
D3: a reading may carry an adjudicated set of dialect-only definitions the oracle is verified
to miss, each admitted by a deterministic file:line, name and scope check (never a model's
judgment), listed, and left out of `sym_precision`'s denominator. Only a real module-scope
definition the oracle failed to report qualifies (column 0, no emitted definition enclosing
it); a local, callback helper or object-literal key the oracle deliberately excludes never does.
D7: C#'s unaliased `using X;` is a wildcard import resolved through the namespace's declaring
files, and a dotted call through a wildcard-imported type resolves to its member.

**Amended 2026-09-28 (owner decision D4, LB-F30).** The "three items deferred on the same
missing L3 oracle" trigger fired (github-actions, package-json and dockerfile all waited on
tool-format xref scoring). Answer: the doc engine's cross-file references now reach the
store as file-tier `doc-xref` edges and are scored as `xref_precision`/`xref_recall`
against a tool oracle's `cross_references`; the doc-engine strategy set gains `repo-root`
(GitHub Actions' root-relative `uses:`) and `label-selector` (Kubernetes Service to
workload, a bounded join over declared labels). `repo-root` is `repo-path` with a different
base directory — the "parameterized copy" trigger is watched, not yet fired. A Dockerfile
local-image strategy is designed and deferred (plan row LB-X-docker-image-ref): its
evidence lives in three other formats.

**Amended 2026-09-29 (owner decision, D5 extension; LB-F46, LB-F52, LB-F50, LB-F51).** A
third-party definitions query is accepted as a tier-C L1 oracle when (a) it is not the source
the dialect was drafted from; (b) the tier-C grade cap applies; (c) it is fetched at measurement
time by pinned revision and never vendored into the repository; and (d) its licence is recorded
in the reading. A licence declared consistently across a grammar's package manifests counts when
there is no LICENSE file. AGPL grammars are refused. The instrument: `lang-oracle -tags-query
<scm> -tags-query-rev <40-hex> -tags-query-source "<owner/repo> <path>" -tags-query-licence
<SPDX>` runs a `tags.scm` (upstream, helix, aider, Sourcegraph), an nvim-treesitter `locals.scm`
or a Zed `outline.scm` through fathom's own engine on the dialect's pinned grammar and scores its
definitions as a neutral oracle file at tier C (L1 good at most). It refuses a query that a `;
source:` header of the dialect's own queries names (a), a revision that is not a 40-hex commit or
a query file inside the work tree outside `workspace/` (c), a missing licence (d), an AGPL
licence, and a predicate the engine does not support (named, never stripped). The reading's
oracle run records source, revision and licence, and `Reading.Validate` refuses a run that names
a source without a 40-hex revision and a licence, or above tier C. Definitions only: D5's calls
path, an independent runtime, is unchanged. Evidence, the 2026-09-30 reference-codebases
evaluation: 34 of the 37 oracle-less open tags-engine items have a third-party definitions
query; prolog's foxyseta grammar is AGPL-3.0, so Rukiza's ISC grammar is the alternative; abap's
kennyhml grammar has no LICENSE file and declares MIT in `package.json`, `Cargo.toml` and
`tree-sitter.json` alike, which the licence rule above admits. **Revisit when** a tags-query
reading and a tier-A or tier-B reading of the same subject differ by more than .05 on
`sym_recall` or `sym_precision` (the query is then no oracle for that dialect, and the tier-C
admission needs a calibration bar), or a dialect is found drafted from its oracle query without
a `; source:` header naming it (condition (a) then needs more than headers, e.g. PROVENANCE.md).

**Amendment (2026-10-01, owner-architect decisions D1 and D2).** Two rules for the excellence program (`docs/design/language-breadth.md` section 18). **D1, dispatch at parity.** A tags or doc port replaces the incumbent extractor in ingest once lang-parity proves it at least as good as the incumbent on every measured metric at its demonstrated level, whatever its target level; decision 4's replacement protocol no longer waits for the target. Reason: improvements must reach the dispatched path. The PHP dialect is measured, but ingest runs the CGO `phpextract.go`, so a better port that ingest never calls helps no agent. The LB-M5 cost guard governs the switch: ingest CPU per byte stood at 1.85x against the 1.5x bar (O208, new coverage plus a double parse of the undispatched C, C++, C#, Java, PHP and Rust); the switch must bring it back under the bar, or the `-rest` rows say why. **Revisit when** a dispatch switch lands with LB-M5 still over 1.5x and no `-rest` row naming the cause, or a port switched on parity regresses an agent-visible answer (callers, blast radius, file edges) on a calibration subject that its measured metrics did not show. **D2, concurrent epoch rule.** When two waves both change extraction, the one that ships renumbers to `ExtractorEpoch` = origin/main's + 1. Reason: two concurrent bumps collide on one number, and a store stamped with it would skip the re-extraction the second wave needs. **Revisit when** a collision still reaches main (two merged waves share an epoch), or extraction changes stop being carried by one integer (per-language epochs).

**Amendment (2026-10-02, wave pc: a native reader where no grammar reads the language; Rego, Cedar).** LB-Q-rego was refused on its grammar (tree-sitter-rego leaves an ERROR node in 74% of the files OPA accepts) and its refusal's trigger has not fired: upstream HEAD is still da2a1f63 (`git ls-remote`, 2026-10-01). Re-planning it is a new decision, not the trigger. Decision 3 limits per-language Go to the closed strategy set and the native typed passes; this amendment adds one admission. **A reader written in Go, with no dependency, may extract a language that no allowlisted grammar parses under the 5% line, when it is measured against a tier-A oracle that shares no code with it.** Rego is the first: two options were weighed on `CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build ./cmd/fathom` at eb900e7a (102,371,538 bytes). (a) OPA's parser, `github.com/open-policy-agent/opa/v1/ast` v1.18.2 (Apache-2.0): +6,450,288 bytes (108,821,826), it raises `golang.org/x/net` and `google.golang.org/protobuf` and brings gojsonschema, go-metrics, xxhash and a second yaml; it parses every file the oracle accepts by construction (StyraInc/regal v2.2.3 bundle/ 232 of 232, open-policy-agent/opa v1.18.2 138 of 181, the 43 others v0 syntax), and it is the tier-A oracle's own parser (calibration/oracles/opa-parse shells out to `opa parse`), so a reading would prove the mapping from its AST and never the parse. (b) A reader of its own (`internal/extractor/regoextract.go`: a tokenizer and a top-level statement reader, packages, rule and function heads named as OPA names them, calls by written name and data references): +69,744 bytes with the file-resolver hook (102,441,282), no dependency, 0 of the 370 OPA-accepted files needing a repair and 0 faults, and independent of the oracle, so its reading proves both. Chosen: (b). Its reading: L1 and L2 excellent, L3 good (LB-Q-rego-rest: file_edge_recall .3858, an import naming every file of a split package while an edge reaches only the file defining what is used). L3 took a strategy, `rego-package`, joining the closed set (`import data.x.y` to the files declaring package x.y, else package x with a rule y, as OPA reads it; LB-F132), and the file resolver now reads a native dialect's own import specs (`extractor.FileSpecExtractor`), so a native language reaches L3 through the same pass and the same harness the tags languages use. Cedar was measured against the same bar and refused (LB-Q-cedar-rest): chrnorm/tree-sitter-cedar 5092204d (MIT) leaves an ERROR node in 5 of the 42 policy files `cedar` 4.13.0 accepts (11.9%) and has no schema grammar; cedar-go v1.8.0 (Apache-2.0, +867,680 bytes) rejects 6 of the 42 (templates) and its schema AST carries no source position; a reader of Cedar's own was outside the item. **Revisit when** a second language asks for this admission (Cedar is the candidate): the rule then needs its own bar beside the 5% line, a size cap for the reader and a repair-rate line; or a grammar or library that parses Rego under the 5% line appears, when its cost is weighed against the reader's; or the reader's repair rate on a new subject passes 5%.

**Amendment (2026-10-02, owner-architect decisions).** Two decisions of the owner-architect on wave le's open questions (`docs/design/language-breadth.md` §1.4 and §1.6). **(a) The oracle-blind rule (LB-F175) is accepted, with a hard cap.** A fathom call pair the truth lacks whose every written call site the index wrote no occurrence at stays out of both L2 counts, counted (`calls_oracle_blind`) and disclosed, as §1.4's amendment states; but a reading whose oracle-blind exclusions exceed 10% of its fathom pairs claims no level from the rule: its L2 is graded as if the rule were off, and the reading says so. The cap was the rule's revisit trigger; it is now a rule the harness computes (item js implements it, LB-F203), and every exclusion stays counted in the reading. Reason: past that share a reading cannot tell the oracle's blind spot from the subject's, and a level claimed through the rule would rest on pairs nobody judged; JavaScript fired the trigger at birth (422 of 1,441 pairs, 29%; O442). **(b) Go's L2 grades the calls the product serves (§1.6) is accepted.** Which surface reads which graph is recorded (O475): GraphBoth (`callgraph.BuildBoth`) answers `symbol_context`'s callers, callees and related tests (since wave lg; before, the syntax graph alone), `blast_radius` with `include_heuristic`, `find_callers`' depth annotation, `references`, `review_diff`, `test_impact`, `fathom verify` and `verify_plan`, and the review engine; for a service with typed edges the default answer of `blast_radius` and `find_callers` is the resolved graph (the typed pass alone: option B, .9805 / .9180 on cobra and cli/cli); `find_callers` with `include_heuristic` lists the store's raw `Calls` matches. Wave lg made the graded set the product inside `internal/analysis/callgraph`, so every reader of `BuildBoth` and every package patch (`ReplacePackage`) gets it: a syntax-only edge into a Go method whose caller sits in a file the typed pass resolved is not served (E, LB-F199: a dependency's call bound by bare name, `assert.Equal` to a service's `Equal`), and a syntax-only edge to a Go type is a reference, `core.EdgeTypeRef`, kept in memory only, which the callers and callees views and the L2 pairs leave out and the reach walkers, `blast_radius` (labelled), review, the console and the findings neighborhoods still read (D, LB-F200), so a type's consumers are never lost. Go's L2 reads excellent: .9573 / .9219 against the servable truth (LB-F201; level_met L2, L3 unmet on file_edge_precision .5625). No store table, cache or background job was added: the table freeze holds until RT-M1. **Revisit when** a reading past the 10% cap gets an oracle that types the receivers it missed (the rule is then measured off against it); a served surface reads a graph other than O475's map names (the map and the grading are re-recorded); `core.Edge` gains a call site (E then applies per site rather than per file); or `find_callers`' heuristic rows move off the store's raw `Calls`.

**Amendment (2026-10-10, owner-architect decisions K17 and K18, wave lf2).** Two rules on what a language's definitions are, decided 2026-10-09 (K17 closes O606; K18 closes O527 and O607). **(K17) Go's members are symbols.** Go struct fields and interface methods are symbols named `Owner.name`, from a member pass in goextract, of kind `field` outside `oracle_kinds` where the L1 oracle does not tag them (Python's LB-F241/LB-F242 precedent; Rust, Java and C# already emit their members), and a field reference go/types resolves is a typed edge. Reason: who reads or writes `Config.Timeout`, and the blast radius of a field rename, are ordinary agent questions; go/types answers them exactly at no model cost; on cli/cli 28,084 of the 42,232 unmapped typed endpoints are fields. Cost gate, and its kill rule: fathom's self-ingest wall time may grow by at most 15% and its store by at most 20%, both measured before and after in the same wave; over either, fields are kept for exported structs only and re-measured, else the change is reverted and the ceiling written. The measured numbers are the go item's (LB-Q-go and its rows in `docs/plans/language-breadth.md` §F.43), not this record's. **Revisit when** the cost gate fails (the exported-only fallback or the revert is then recorded here), or a language whose L1 oracle does tag its fields asks for a member kind outside `oracle_kinds`. **(K18) An executing top level is a module symbol; a pure namespace is file granularity.** One rule across languages. A file whose top level executes code (Python, JS/TS, Ruby, Lua, shell) gets one minted per-file module symbol, line 1 to the file's last line, of a `module` kind outside `oracle_kinds`: it is the definition the file's top-level calls and imports are made from (O527: 214 references were attributed to an unrelated method, or to nothing) and the target of an import of the module. Its name is `<module>` (`core.ModuleSymbolName`), Python's own for a module's top-level code (a traceback's `in <module>`), which no identifier spells, so it never collides with or is mistaken for a definition of the file: scip-python names a module `<path>/__init__:`, and its `__init__` leaf had joined a class's own `X.__init__` 41 times and a line-1 definition 15 times on fastapi and flask (O527). It is a tags-engine capability a dialect manifest turns on (`module_symbol`, schema `#Dialect`, checked at load: a kind of its own, canonical `module`, neither container nor callable, outside `oracle_kinds`, the `dir` package rule), never per-language Go; `internal/scip` maps a module definition to it and to nothing else, and the file resolver makes an import of a module an edge to it. Python is first (lf2): every `.py` file qualifies, an empty `__init__.py` included, as scip-python defines one module per document (1,258 on fastapi and flask); JS/TS, Ruby, Lua and shell turn it on in their own waves with no engine work. A pure namespace (a Go package, a Java package, a C# namespace, a Rust `mod`) is not a symbol: it is file granularity, already graded at L3 by file edges, and L4 leaves namespace descriptors out of `def_mapping` and `typed_recall` as a counted, disclosed harness rule (the S1/LB-F239 shape for parameters), never an uncounted exclusion. Python's reading moved def_mapping from .8906 to .9985 and typed_recall from .952 to .9979, L4 excellent (LB-F349). **Revisit when** a dialect that claims more than one extension per file name, or a package rule other than `dir`, turns the module symbol on (two files could then share its key, which then needs the file's extension); a producer other than scip-python names a module (`scip.ModuleDefinition` learns its form in that wave); a namespace descriptor is left out of an L4 count without the count in the reading; or a language's file both executes at its top level and is a namespace other files name (an ES module read through its exports), when the rule must say which a reference to it means.

## ADR-049: The proof-layer roadmap: a kitchen-sink gate, pillars inside the tenancy lane, reach before depth, views never trackers, no Rust rewrite (2026-09-27)

**Status.** Accepted 2026-09-27. Owner decision memo of 2026-09-27 over the agentic-coding landscape scan of the same date and the owner's notes N1 to N8. Vision: `docs/vision/38-proof-layer-roadmap.md`. Designs: `docs/design/reach.md`, `docs/design/spec-interop.md`, `docs/design/knowledge-retrieval.md`, `docs/design/governance-learning.md`, `docs/design/progress-and-economics.md`; the system graph is ADR-050. Plans: `docs/plans/reach.md` (RH-), `docs/plans/spec-interop.md` (SI-), `docs/plans/knowledge-retrieval.md` (KR-), `docs/plans/governance-learning.md` (GL-), `docs/plans/progress-and-economics.md` (PE-), `docs/plans/system-graph.md` (SG-). Goals G16 to G22; G15 names vision 37's M2 and M3 as its next bets. Promotes futures F20 and the plugin part of F2; promotes F14's output format only; F13 stays gated by vision 31. Invariants I56, I57. Amended twice on 2026-09-29: an efficiency wave goes first, then the Claude OS comparison (the two amendments below, in that order).

**Context.** The 2026-09 market has many tools that find code and many that describe work; almost none prove things about code. Spec tools verify by asking a model; review bots guess at impact; the new graph tools read syntax inside one repository. Fathom's lead is computed, cited and gated answers, and its gaps are reach (install, surface size, other clients), breadth (languages, service boundaries, dependencies) and visibility. The owner's notes asked for dependencies, progress views, a usage dashboard, spec formats, languages, cross-service calls, documentation search and stronger semantic layers. Without a gate, that list becomes a kitchen sink.

**Decision.**
1. *The kitchen-sink gate.* Every accepted item names which of (a) widen what fathom can prove, (b) deliver proof where people already work, (c) make the proof visible it serves, and the instrument that proves it. An item with a letter but no instrument goes to `futures.md` with a Promote-when; an item with neither is refused.
2. *Pillars and a lane.* Widen the graph (a: G15, G17, G18), Know the words (a: G20), Deliver where work happens (b: G16, G19, G21), Make proof visible (c: G22); ADR-051 adds Proof depth (a: G23), Work intelligence (c: G24) and Review and compliance (b: G25); tenancy (G14) is the lane every pillar lives inside, and no row that adds a table, cache or background job starts before RT-M1's first week reads PASS.
3. *Reach first.* Wave 1 is the cheap, high-leverage delivery work: a default agent surface of at most eight tools with the rest opt-in by group, the rendered plugin bundle install-tested in Claude Code and one other client, rules read from every major client's files with off/warn/error levels, hooks verified in at least three clients, the spec-dialect port with Spec Kit and OpenSpec adapters, SARIF for obligations and review findings, and token cost in every receipt. Public release remains the owner's Q4 call (F2).
4. *Progress is a view, never a tracker.* Status is derived by `fathom verify`; no view writes a plan, a status or an assignment. An optional row `weight:` (integer, default 1) and proof history make weighted progress, burn-up, proof regressions, time-to-proof and claim honesty computable. Proof history is a projection of the existing verify ledger (`~/.fathom/verify-ledger.jsonl`, about 17,000 readings), which gains per-row verdicts additively (schema 3); no new ledger. Claim honesty is computed per agent identity, never per person (I58). The derived kanban is read-only.
5. *Economics is cost per proven outcome.* The existing session ledger (3,359 sessions) and the Stop hook's incremental transcript fold are joined to what the session proved; transcripts are read within the tenancy budget and nothing new is captured. A generic token-usage dashboard is refused (commodity; owned by the clients). Fathom-on versus fathom-off is shown only where G2 makes the comparison admissible.
6. *Spec interop through a port; our grammar stays canonical.* Foreign spec dialects (Spec Kit, OpenSpec, Kiro, BMAD, Task Master, GitHub task lists) are read in place and verified; a converge report lists claimed-unproven tasks and code nothing claims; fix-up tasks are written only inside fathom-marked blocks, only when asked. Plan grammar v2 gains `weight:` and `accept:` (EARS-style acceptance text, informational); the test-tied tier is the existing `certified_by:` (ADR-043), made a plan default by a `Proof: certified` header. The memo's `tests:` name is not used: that key already means a `gone: callers` modifier (ADR-044) and is in the compatibility surface.
7. *Jev's two roles.* The decision port proposes anchors for unanchored foreign tasks (a Choice over `find_symbol`/`pack_context` candidates), shown as a proposal with its calibration, never as proof, and only after a calibration check on adjudicated anchors reads right at least nine times in ten at the confidence it is shown at (ADR-042's route-site lesson). It has no role in link matching, rule learning, doc linking or progress.
8. *Knowledge retrieval without embeddings.* SQLite FTS5 (supported by the pure-Go driver: `modernc.org/sqlite` v1.52.0, SQLite 3.53.2, probed 2026-09-27) over docs, decisions, intent, plans, memories and comments, section-aware, fused with concern ranking; doc spans that name a symbol, path or contract key are linked, and a changed target flags the span possibly stale in `staleness_report`. Vision 31's kill criterion for embeddings stands.
9. *Governance that learns, visibly.* Rules are promoted and retired from corroborate/refute signals under thresholds published in the repository, and every promotion or retirement is announced, never silent. The signals are empty today (0 memories, 0 human claim verdicts, 0 live certificate declarations on the reference machine), so a signal source (hook outcomes recorded as served, complied, violated and blocked events) lands before any threshold may move a rule. Validity windows (SU-19: `valid_from`/`valid_to` already on `semantic_claims`; activate and serve, no schema change for claims) extend from claims to memories, rules-file rules, learned-rule states and doc links, and `as_of` reaches every verb that reads them. Rules files of every major client are sources; hooks are served through every client's adapter on the harness port (vision 25). Findings reach reviewers as SARIF and a check-run payload; review diffs gain layers ordered by blast radius without a model.
10. *Deferred.* A shared team index snapshot (F15's second user) and answer events for background agents (fathom would publish a verdict file or webhook payload only when asked, never schedule; the dispatch non-goal holds) go to `futures.md`.
11. *The stack stays; no Rust rewrite.* Go, pure-Go SQLite, wazero tree-sitter, one loopback daemon under an OS-measured budget. Parsing is wasm and the store is SQLite, so Go is not the bottleneck; a Rust worker would add a toolchain, a seam and a second language on a bus factor of one.
12. *Refused.* A model deciding done; auto-capture without a budget; Redis, a graph database, a vector database or any second store; a generic token dashboard; kanban write-back or status set by a drag; downloading dependencies, advisories or docs by default; a tool per edge kind; embeddings before the vision 31 bench passes; hosted anything.

**Alternatives rejected.**
- *Depth first (system graph and dependencies in wave 1).* The deepest engineering does not move adoption; the scan's fastest growth came from one-command installs and small surfaces, and depth adds load before tenancy closes.
- *Adopt Spec Kit's or Kiro's format as canonical.* Theirs are prose; ours is the only grammar whose rows carry anchors that make done computable. We borrow two fields and keep the model.
- *A kanban with write-back.* Makes fathom a tracker (scope non-goal) and lets a drag assert what only verify may compute.
- *A token dashboard like the clients'.* Commodity, owned by the clients; the only number nobody else can compute is cost per proven outcome.
- *Embeddings for documentation search.* Vision 31's bench has not run; FTS5 plus doc-to-code links answers the measured doc-question gap with no second store.
- *A Rust extraction worker or rewrite.* Decision 11; kept as a trigger-gated wasm experiment (F29).

**Consequences.**
- New plans RH-, SI-, KR-, GL-, PE- (and SG- under ADR-050); new designs as listed; scope goals G16 to G22; G15 names M2 and M3 as defined in vision 37 §5 (LB-M6-M2, LB-M6-M3, unchanged).
- Plan grammar v2 grows additively: `weight:`, `accept:` and the `Proof: certified` header join the compatibility surface when their rows land (SI-7, SI-8, SI-14).
- The agent surface shrinks to a default of at most eight tools; the rest stay registered and reachable by group opt-in (law 4: no tool is deleted by this decision).
- `~/.fathom/verify-ledger.jsonl` gains per-row verdicts (schema 3, additive; v1 and v2 rows still parse).
- The rule compiler reads more instruction files; the obligations corpus widens only to files a client actually loads, and every compiled rule still passes I27's citation and scope checks.
- Futures F2 (plugin part), F14 (output format), F20 promoted; F27 (team snapshot), F28 (answer events), F29 (Rust wasm module) added.

**Revisit when.**
- The default surface of eight measurably loses answers: RH-M3 shows a task class whose best tool is outside the default and is called by fewer than 1 in 20 sessions that needed it.
- Stranger time-to-first-proven-answer stays above 10 minutes after wave 1 (RH-M1): reach is not the constraint and the wave order is wrong.
- A spec tool ships a deterministic, anchor-bearing checklist format: F20's drop condition, re-weigh the adapters.
- Jev's anchor calibration fails (SI-M4 below nine in ten at the shown confidence): drop the proposal path, keep the port out of spec interop.
- The FTS5 slice misses grep's pass rate (KR-M1) after one tuning round: re-weigh vision 31's bench before any other retrieval work.
- A learned rule is promoted and then refuted by a human within 30 days more than once in 10 promotions (GL-M2): the thresholds are wrong.
- The Rust trigger in vision 38 §7 fires: run the wasm-module experiment (F29), not a rewrite.
- (Amendment 2026-09-29) EW-M1 misses its bar: the session tools are not where our cost is, so efficiency rows stop jumping the queue and wave 1 resumes.
- (Amendment 2026-09-29) EW-M3 puts the since-compaction share's interval above 15 percent: make per-child handoff the default map delivery.
- (Amendment 2026-09-29) A second orchestrator session with enough sub-agents reads XM-M1 below its bar: stop the maps slice at the rows already built.

**Amendment (2026-09-29): an efficiency wave goes first.** Owner direction of 2026-09-29: move up every row that makes our own sessions cheaper, reach a stopping point, redeploy, adopt, then build the rest faster. Plan: `docs/plans/efficiency-wave.md` (EW-). Decision 3's order stands after the wave; the wave is inserted ahead of it and replaces nothing.
1. *Sequence.* Stage 0 merges the four unmerged lines, keeping the push line's state for every row the wired audit reopened. Stage 1 removes per-call waste: the Bash-write hook's doubt path, which served six unrelated rules on 4,835 of 7,953 write-shaped Bash calls in 13 days (about 1.6M tokens, outside the payload-cost instrument); two wall-clock tests and a shared build-cache prune that fail gates, plus a sweep of the 38 wall-bound test assertions; wrapped annotations and deferred `needs:` targets that let dispatch offer blocked rows (G2-5, G2-6); scratch runs writing the real ledgers (PE-26 to PE-28); a stanza naming tools the registered surface lacks. Stage 2 adds the session tools: the test-run hook's tiers 0 and 1, docs search wired in production with its byte budget (KR-8, KR-9), and certifying-first test selection wired (TQ-15w, TQ-16w). Stop 1 pushes, runs `task deploy` and adopts. Stage 3 builds the maps slice (XM-1 to XM-9, XM-14); stop 2 adopts it.
2. *Why first.* Every later row is built by the same orchestrator and worker waves, so a lower cost per proven row compounds across the roadmap. The readings of 2026-09-28 and 2026-09-29 (statistics only): sub-agents re-read a file their parent had read before spawning on 36.1 percent of their file reads (7.58M tokens in 14 days), and a file any earlier agent in the session had read on 69.9 percent (15.9M tokens, 32 percent of all sub-agent tool_result volume), led by ADR.md, Taskfile.yml and CHANGELOG.md; 7 of 10 push attempts on 2026-09-28 failed the gate.
3. *What the readings decide.* XM-M1 reads BUILD on the row as written (36.1 percent, interval 32.4 to 39.7, against a 15 percent kill line), but the pass rests on stale parent knowledge and on one parent session: files the parent touched since its last compaction give 15.1 percent (interval 11.2 to 19.5), within the hour before spawning 2.8 percent. So the maps slice is built but adopted first as a wave map that every worker brief carries by id and appends to, aimed at the sibling pool; per-child handoff as the default waits on EW-M3, and XM-M2 stays the kill. CA-M1 reads STOP: a median of 0.5 pre-set files re-read after compaction against a bar of 2, no different from 94 control points (p 0.39), and at 0.64 compactions a day CA-M2's 300 would take about 470 days. CA-1 to CA-5 are not built and `cmdThreadBrief`'s silence on compact stands.
4. *The lane holds.* Nothing in the wave adds a table, cache or background job before RT-M1 reads PASS (decision 2, ADR-056 R5): maps live in thread fields, test-run rows in the hook log, docs search in the FTS5 index KR-1 created. The work-signal detectors (a daemon fold), rule retirement (30 days of signal first) and surface groups (RH-3 cannot be read from a 7-row call log) stay out.
5. *Adopt means configured and written down.* This repository turns `[hooks] test_run` on, and `docs/guides/agent-waves.md` becomes the worker-brief contract: `FATHOM_HOME` and a private build-cache root per worker, docs search or a row's context pack before a hot file is opened whole, `fathom tests select` before a package run, one map id per brief after stop 2, and `fathom verify --strict` with the integrated binary (whose wired check refuses a library-only claim) before one push per wave. The stanza names only tools the registered surface serves.
6. *Paid means measured.* EW-M1: sub-agent tool_result plus hook-injected tokens per proven row at least 15 percent below the 14-day baseline after stop 1 (about 74k tool_result tokens per row: 49.69M over 671 rows proven across the lines); EW-M4: at least 30 percent after stop 2, read beside XM-M2 on our own briefs; EW-M2: at most 1.5 push attempts per landed push (10 for 3 on 2026-09-28). XM-M2 is the only model-budget A/B in the wave and is asked for first.

**Amended 2026-09-29 (the Claude OS comparison of 2026-09-27; evidence `docs/assessments/2026-09-27-fathom-and-claude-os.md`, `docs/assessments/2026-09-27-agentic-coding-landscape.md` and `docs/assessments/2026-09-27-peer-stacks-and-rust.md`): neighbours, a sharper focus, two refusals named, and the borrow list decided.** The owner read Claude OS (a local memory and document-search tool for Claude) against fathom on 2026-09-27 and asked whether to keep going. (1) *Neighbours, not competitors.* The two share one small job, carrying context into a new session (about 25 percent overlap, an estimate); Claude OS has no call graph, no rules at the edit and no gate. Keep building, with a sharper focus: ease of first use is Claude OS's advantage and fathom's risk, so reach work (G16) outranks new engines, which decision 3 already says. (2) *Decision 12's refusal list gains two names.* A time-series database (Prometheus, InfluxDB): fathom keeps a 24-hour resource history itself and exports `/metrics` for anyone who already runs Prometheus, so a second store adds a process and no answer (it was covered before only by "any second store"). And polling: fathom waits on events and never checks a server on a timer (P23). The rule of thumb stands: one embedded file, many indexes, no extra servers; sqlite-vec stays F13 ("maybe, narrowly", only for free-form memory and only if a reading beats FTS). (3) *The eight borrows, each decided.* A "remember this" memory skill is **refused**: anchored memory was deleted for 0 memories in any repository (ADR-053 decision 2), `fathom thread note` already captures a note against files, and a choose-to-save feature repeats the delivery-not-lookup failure. `fathom init --dry-run` and `--demo` are **accepted** as reach rows RH-11 and RH-12 (a test holds dry-run's list equal to the real run's touched set; demo uses the fauxtown fixture fleet and downloads nothing; both are read inside RH-M1). A knowledge-health report is **merged** into one console panel over instruments that exist or are planned (GL-23; law 4, no new verb). Warnings inside answers are **answered**: every query result already carries a staleness report (I4, ADR-008) and receipts carry tier and drift (ADR-034). An end-of-session thread-note offer is **deferred** (SR-X1; trigger: SR-52 reads keep, because no prompts are added to a bet under measurement). "Ready now, deeper later" is **deferred** (RH-X4; trigger: RT-M1 PASS and RH-M1 showing cold ingest is at least half of stranger time, because it adds background work before tenancy closes, decision 2). README screenshots and MCP directory listings are **deferred** under F2 (publishing is the owner's Q4 call), and a browsable skills and packs library under F6 (it needs adopters). (4) *The partnership idea* (fathom as the code engine under an agent-memory tool) is **deferred as F55**: ADR-028 allows a peer process and never a dependency; the project has been quiet since 2026-02-06 and has no licence file. The landscape and stack assessments behind decisions 3, 8 and 11 are summarized in `docs/assessments/`.

## ADR-050: The system graph: one contract-key link model with evidence tiers, three verbs, and dependency APIs read from local caches through a port (2026-09-27)

**Status.** Accepted 2026-09-27. Owner decision memo of 2026-09-27, notes N1 (dependencies) and N6 (calls between services). Vision: `docs/vision/38-proof-layer-roadmap.md`. Design: `docs/design/system-graph.md`. Plan: `docs/plans/system-graph.md` (SG-). Goals G17, G18. Extends ADR-044's inventory keys; keeps vision 15 §3 (ci-kit owns the depgraph engine). Invariants I56, I57.

**Context.** A call graph stops at a process boundary (inside one, `blast_radius` read precision 1.00 on 9 audited calls, 2026-09-27). Fathom already joins Go HTTP client calls to route registrations (`service_ref`, heuristic tier, VB-30) and extracts `route:`, `rpc:`, `env:` and `cli:` inventory keys for conservation (ADR-044), but each relation is its own code path, only HTTP crosses services, and nothing grades a link's evidence. Dependencies are the same problem one level down: our code consumes a library's API the way one service consumes another's route. `dep_drift` and `cross_repo_blast_radius` have zero lifetime calls; `package_deps` was deleted by the surface prune. Peers link routes to handlers across repositories; none grades the evidence or gates on it.

**Decision.**
1. *One model.* A node (a symbol, a file, or a contract document) PROVIDES or CONSUMES a typed contract key. A link is a provides/consumes match on the same key. Kinds: `route:` (method and path template), `rpc:` (package.Service/Method), `gql:` (Type.field), `topic:` (event or message channel name), `table:` (table, optionally .column), `env:` and `config:` (environment variables, config keys, feature flags), `pkg:` (ecosystem/name@version#symbol, a library API), `image:` and `deploy:` (container image, cluster service or ingress host), `cli:` (command path). The key grammar is language-neutral and extends ADR-044's keys without changing them.
2. *Four evidence tiers, ordered.* `spec` (both ends bound through a contract document: OpenAPI, proto, GraphQL SDL, AsyncAPI, Avro, a migration) > `typed` (a typed pass or generated stub resolves the consumer to the provider's declaration) > `literal` (normalized literal key equality) > `template` (a key with holes matched to a template). A link carries its tier and its evidence (file:line at both ends, the extractor and its epoch). A link below `typed` never gates a verify row, a conservation plane or a review verdict; it informs (I57).
3. *One port, one link table, three verbs.* Extractors implement one contract-source port; keys go to one additive `contract_keys` table per service and links to one `system_links` table, refreshed for the keys an ingest changed. Three verbs answer every kind: `links` (what a node provides and consumes, and who is on the other side), `system_impact` (a change's reach across services and dependents, by kind and tier), and `contract_drift` (a producer's contract against its consumers' usage, and a diff's breaking changes). The two surviving zero-call tools are retired when the verbs land; no tool per edge kind (law 4). The verbs join the agent surface only on call-log justification.
4. *Dependencies are the `pkg:` kind, read locally.* Fathom never downloads a package, advisory or doc on its own authority (I56). It reads what the toolchain already fetched (the Go module cache, `node_modules` and the pnpm store, a project virtualenv and the uv cache, the Maven and Gradle caches, the Cargo registry sources), indexes each (ecosystem, name, version) public API surface once per machine, content-addressed by the package digest and shared by every service that uses it, lazily, in the background ingest class, under the memory budget. Upgrade impact is `contract_drift` between two versions of a `pkg:` provider; the usage map is `links`; vuln reach reads an OSV file the user supplies.
5. *The lockfile graph comes through a port.* `DepGraph(ecosystem, root) -> nodes, edges, lock digests` is the seam vision 15 §3 named; an in-tree adapter reads lockfiles only until ci-kit ships its engine, then is deleted. Fathom consumes answers; ci-kit owns the engine.
6. *Measured by a fixture fleet.* Fauxtown gains services that exercise every kind with an answer key; precision and recall are read per kind and tier; drift detection is read on seeded breaking changes; upgrade impact on a seeded-upgrade corpus. Co-change history from git is an independent oracle for `blast_radius` and `system_impact`, never an input to them.
7. *Jev has no role in matching.* The route-site calibration failed (45.7% right at confidence at least 0.9, 2026-09-24). Revisit only with a passing calibration sample on adjudicated links.
8. *Tenancy.* No link computation on a request path: the join is an indexed lookup on key; materialized links refresh at ingest for changed keys only; dependency indexing is lazy, per package, byte-capped, evictable, and a request for an unindexed package answers `pending` with a named reason.

**Alternatives rejected.**
- *A tool per edge kind* (`routes`, `topics`, `tables` …): the pruned-tool history (34 zero-call tools deleted) says agents do not find them; law 4.
- *A graph database for cross-service edges:* ADR-041; the join is a key equality over an indexed table.
- *Downloading packages or advisories to fill gaps:* egress is an admission cost the operator never granted, and a daemon that fetches is a supply-chain surface (I56).
- *Indexing full dependency sources:* private bodies are not the contract and cost memory; the public surface answers upgrade impact and usage.
- *Building the depgraph engine in fathom:* inverts ownership (vision 15 §3).
- *Letting heuristic links gate:* a template match is a hint; gating on it would make a false positive block a push.

**Consequences.**
- New: `internal/syslink` (key grammar, port, join, tiers), `internal/depplane` (the DepGraph port, the in-tree lockfile adapter, cache locators, the API indexer), additive store tables `contract_keys`, `system_links`, `dep_packages`, `dep_api`, the verbs `links`, `system_impact`, `contract_drift`, `indexbench link-oracle`, fauxtown services and an answer key.
- Changed: `service_ref` becomes the `literal`/`template` tiers of `route:` links; inventory extraction feeds `contract_keys`.
- Removed at the end: `dep_drift`, `cross_repo_blast_radius`, and the in-tree lockfile adapter once ci-kit's engine ships.
- The compatibility surface grows by the key grammar and the three verbs' `--json` envelopes when they land.

**Revisit when.**
- A kind's spec+typed tiers stay below precision 0.95 or recall 0.85 on the fixture fleet after two extractor rounds (SG-M1): drop or narrow that kind.
- A materialized-link refresh costs more than 10% of ingest wall time on the home fleet, or dependency indexes exceed 256 MiB on disk or push `phys_footprint` beyond budget (SG-M5): change the materialization or the cap.
- ci-kit ships the depgraph engine: delete the in-tree adapter (SG-31..SG-35).
- A consumer needs private dependency bodies (not the public surface) for a proven answer class.
- A calibration sample of adjudicated links passes nine in ten at a shown confidence: reconsider a proposal role for Jev in `template`-tier matching.

## ADR-051: What only fathom can do: the joins, test intelligence, work intelligence, and change and compliance receipts (2026-09-27)

**Status.** Accepted 2026-09-27. Owner decision memo of 2026-09-27, binding addendum "What only fathom can do" (T1 to T8, U1 to U16). Vision: `docs/vision/38-proof-layer-roadmap.md` §4. Designs: `docs/design/test-intelligence.md`, `docs/design/work-intelligence.md`, `docs/design/change-and-compliance.md`; U4 in `docs/design/governance-learning.md`; U10 in `docs/design/system-graph.md`; U12 over `docs/design/progress-and-economics.md`'s ledger projection. Plans: `docs/plans/test-intelligence.md` (TQ-), `docs/plans/work-intelligence.md` (WI-), `docs/plans/change-and-compliance.md` (CX-). Goals G23, G24, G25. Promotes futures F24 and the computable subset of F9; adds F30, F31. Invariant I58. Extends ADR-043 and vision 33 without duplicating their rows.

**Context.** Fathom holds eight assets that other tools hold one or two of: a typed call graph across repositories, governing claims served at the edit, proof (verify, certificates, conservation, history), receipts with tiers and drift, the agent activity record (call log, hook log, transcripts, discoveries, threads), a machine-readable intent corpus, the fleet on one machine, and git history joined to all of it. Products built from joins of these cannot be copied by a tool that holds one. The risk is the opposite of the kitchen sink: building a new engine per product.

**Decision.**
1. *The join rule.* Every accepted product is a projection and a view over existing assets. A product that needs a new engine, a new store or a new analyzer is sent back. Retired analyzers (churn, cognitive complexity) are not revived: churn is a git-derived fact computed at ingest, and symbol size from L1 ranges is a named proxy for complexity.
2. *Test intelligence (TQ-).* Shipped certification work stands (TC-1 to TC-24; TC-M2 to TC-M4 stay in their plan). Added: T1 risk-weighted coverage, T2 the test portfolio, T3 coverage ingest in six report formats into the existing block model, T4 certifying-first selection for a diff with CI time saved recorded, T5 contract certificates on system-graph links (takes TC-25 with its trigger), T6 dependency-upgrade test reach, T7 flakiness and commit-bound receipts measured. T8 is `certified_by:` (ADR-049 decision 6).
3. *Work intelligence (WI-).* U1 cost per proven outcome by plan, language and zone with CI minutes; U2 the debt interest rate, measured agent effort per proven change per zone; U3 the friction map with computed remedies shown as proposals; U5 a descriptive model and harness scorecard with n and confounders, causal claims only where G2's admissibility holds (law 12); U6 task preflight, a difficulty profile and a recommended model tier, advisory only, the harness decides; U12 proof regressions generalized to certificates, conservation and invariant enforcement, each with the breaking commit; U13 hotspots with context as a view over T1's data.
4. *Change and compliance (CX-).* U7 a change risk receipt per PR: blast radius, links touched, certificates affected, conservation, rows proven, rules touched, zone interest, uncertified contracts, a risk band with every reason cited, suggested reviewers from ownership of impacted symbols, emitted as SARIF and a check payload. U8 a traceability matrix (requirement, code, test, certificate, attested run, decision) and a dated evidence pack exported only on request that re-verifies byte-identically; standard-agnostic through a mapping file (ADR-011). U9 a migration tracker: declared from and to (symbols, imports, routes, config keys), remaining sites per repository, burn-down from proof history, completion proven by conservation; it never transforms. U11 an architecture drift radar: ADR decisions compiled to dependency rules (F24 promoted), layer violations over time, and machine evaluation of the computable subset of revisit triggers (F9 promoted for triggers whose condition names a fathom instrument and a threshold).
5. *U4 and U10 live elsewhere.* Rule effectiveness (served, complied, violated, blocked, refuted per rule) is part of governance learning, and needs its signal source first: the reference machine holds 0 memories, 0 human claim verdicts and 0 live certificate declarations. The service catalog is a system-graph view with an opt-in catalog-info export.
6. *Jev's site.* U6 is the best new decision-port site: a Score over deterministic features, A/B'd against the deterministic baseline, shown with its calibration, dropped if null. It never dispatches.
7. *Deferred.* U14 onboarding path (F30) and U15 capability reach (F31).
8. *Refused.* A productivity score for any human or any ranking of people (I58); per-author figures are agent identities only. Predictive estimates without a calibrated reading. Any auto-applied remedy.

**Alternatives rejected.**
- *A coverage product that counts lines.* Commodity (hosted coverage services); the join with centrality, governance and incidents is the only ranking nobody else can compute.
- *A debt estimate from static rules.* Estimates debt; fathom can measure the interest actually paid in tokens and minutes.
- *A people leaderboard.* Surveillance, and it would poison the honesty signals the scorecard depends on.
- *A migration engine.* The Moderne non-goal; fathom tracks and proves.
- *A hand-maintained service catalog export by default.* It is computed; export happens only when asked.

**Consequences.**
- New plans TQ-, WI-, CX-; goals G23 to G25; F24 and part of F9 promoted; F30, F31 added; I58 added.
- The review verbs gain the risk receipt; `review_diff`'s output grows additively.
- The session ledger and transcript fold become inputs to WI- projections; nothing new is captured without a budget.

**Revisit when.**
- A join cannot be built without a new engine or store: send it back or record a new decision.
- U2's interest rate cannot separate zones (the top zone's effort per proven change within 1.2x of the median) on three repositories with at least 30 proven rows each (WI-M2).
- U6's A/B is null or negative against the deterministic baseline (WI-M4): drop the Jev site.
- T4's selection misses a certificate a full run would have caught on the replayed month (TQ-M2): selection goes advisory.
- An adopter or regulator rejects the evidence pack format: revisit U8's mapping file shape.

## ADR-052: Fathom is source-available under FSL-1.1-ALv2, converting to Apache-2.0 after two years (2026-09-27)

**Status.** Accepted 2026-09-27 (owner decision). Licence text: root `LICENSE.md`, the canonical template from fsl.software fetched verbatim 2026-09-27 (sha256 of the unfilled template `36b6082235c0a2105174927fc57cc6ae9c41f45a08af2bdcaee18a8dace56177`, identical to the getsentry/fsl.software repository copy), with the notice filled as "Copyright 2026 Geoff Miller". Fires ADR-028's revisit trigger ("fathom adopts a root LICENSE"). Not legal advice.

**Context.** The owner wants the source public and wants no one to sell fathom, or a competing product built from it, without him. MIT and Apache-2.0 permit exactly that resale. AGPL-3.0 permits commercial resale as long as the source is shared. The Functional Source License forbids "Competing Use" (making the software available in a commercial product or service that substitutes for it or offers substantially similar functionality) for two years per release, then each release converts irrevocably to Apache-2.0, which keeps an eventual-open promise and a patent grant that adopters can rely on.

**Decision.**
1. *Licence.* Fathom is licensed under FSL-1.1-ALv2, copyright holder Geoff Miller, 2026. Each release converts to Apache-2.0 on its second anniversary.
2. *How it is described.* "Source-available, converts to Apache-2.0 after two years." Never "open source": FSL is not OSI-approved.
3. *Contributions.* Outside contributions need a CLA or a DCO plus an explicit licence grant, so dual or commercial licensing stays possible. Recorded as an open question (Q29); nothing is built until a first outside contribution is offered.
4. *Kit mirrors.* The in-tree mirrors must carry licences compatible with distribution inside an FSL work before any public release (F2's preserve list). Satisfied on 2026-09-27: agentic-kit was relicensed from MIT to FSL the same day, and app-kit is FSL; a pin bump that changes either licence re-opens the check.
5. *Third-party inputs keep their own licences.* ADR-048's allowlist (permissive sources only, GPL-family tools as separate processes) already fits an FSL work; an FSL-licensed third-party source stays refused as a grammar or query input, as ADR-048 says.
6. *ADR-028 clause 1 restated.* AGPL-3.0 code still cannot be vendored, linked or embedded: its copyleft would require the combined work to be distributed under AGPL, which is incompatible with FSL's Competing Use restriction. The absence of a licence is no longer the reason; the incompatibility is.
7. *The name.* Trademark on "Fathom" is a separate open question (Q30): other products use the name. The licence's Trademarks clause reserves our marks; it does not establish one.

**Alternatives rejected.**
- *MIT or Apache-2.0 now.* Permits a competitor to sell fathom or a hosted copy.
- *AGPL-3.0.* Permits commercial resale with source shared; closes dual licensing without a CLA.
- *BUSL (Business Source License).* Similar intent, but a per-release custom "additional use grant" and a change date of up to four years; FSL is shorter, standardized and pre-committed to Apache-2.0.
- *Closed source.* The owner wants the source public.

**Consequences.**
- `LICENSE.md` at the root; `README.md` gains a one-line License section.
- `docs/intent/questions.md` gains Q29 (CLA or DCO plus grant) and Q30 (trademark).
- F2's preserve list names the kit-mirror licence check (satisfied at today's pins).
- ADR-028's clause 1 reason is restated (above).

**Revisit when.**
- An adopter or contributor refuses FSL in writing.
- A commercial licence is requested.
- A first outside contribution is offered (answer Q29 first).
- The FSL steward publishes a new version, or a court or the OSI changes how FSL is treated.

**Amended 2026-09-29 (correction to decision 4; itself corrected by the next amendment).** Decision 4 and futures F2's preserve line said agentic-kit "was relicensed from MIT to FSL the same day". That was wrong for the code fathom mirrors: at the pinned commit the in-tree mirror's `third_party/agentic-kit/LICENSE` still reads MIT (copyright 2026 Geoff Miller); the relicense commit sits on the kit's local branch `license-fsl`, which the kit's own gate blocks (a failure that predates it). The compatibility check still holds, for a different reason: MIT is permissive and fine inside an FSL work as long as its notice travels with the mirror, which the per-file pin (I15) keeps. app-kit is FSL. Re-check when the kit's relicense lands and on any pin bump. F2's preserve line carries the corrected wording.

**Amended 2026-09-29, later (decision 4 re-checked at the new pins).** The note above is no longer true: it held at the `beb53a4` pin and does not at the current one. agentic-kit main is FSL-1.1-ALv2 as of `53024ff`: the relicense merged there (`86fb4e6`, `LICENSE` replaced by `LICENSE.md`, "Copyright 2026 Geoff Miller") and was pushed with the owner's OK, and `third_party/agentic-kit` now mirrors `53024ff` (go.mod pin `v0.7.1-0.20260929054046-53024ffa6343`), so the mirror carries FSL. app-kit is FSL from `d4512ca` (2026-09-27); `third_party/app-kit` now mirrors `9fa58e6`, which carries it, and is the first app-kit mirror with a licence file (`d9dd789` had none). Decision 4 holds with both mirrors under fathom's own licence. Re-check on any pin bump that changes either mirror's licence. F2's preserve line carries this wording.

## ADR-053: The surface reckoning — eight tools by default, zero-use tools deleted, every removal proven absent (2026-09-27)

**Status.** Accepted 2026-09-27 from the owner-accepted feature scorecard (86 features:
30 keep, 24 improve, 12 merge, 10 ditch, 10 new). Amends ADR-014 (surfaces), ADR-035 (the
promoted verbs), ADR-046 (frozen). Design: `docs/design/surface-reckoning.md`. Plan:
`docs/plans/surface-reckoning.md`.

**Context.** The registry serves 59 tools on bare `/rpc` (63 with the module host's four)
and 25 on the agent surface, 18,310 bytes of schema before an agent asks anything. The call
log (97,898 calls) shows the load concentrated on a dozen tools; 30-odd registrations have
0–5 lifetime calls, and the most accurate impact verbs (blast_radius: precision 1.00,
0.006 s) are among the least called. Law 4 forbids net growth; it has never forced a
shrink.

**Decision.**
1. *The default agent surface is eight tools*: `find_symbol`, `search`, `read_source`,
   `symbol_context`, `obligations`, `verify_plan`, `blast_radius`, `review_diff`, at
   `/rpc/agent`. A path-scoped `extended` surface (21 tools) is the opt-in; `/rpc/companion`
   aliases it. Surfaces stay execute-and-flag, so nothing an agent calls stops working;
   off-surface demand is read from the call log after fourteen days (SR-M3).
2. *Deleted*: `cross_repo_blast_radius`, `dep_drift`, `http_client_calls`, `fleet_drift`,
   `reachable`, `traverse`, `history`, `memory_search` with all of anchored memory, the
   MCP `brief` tool, the `narrate` and `brief` CLI verbs, the four module MCP tools with the
   `/rpc/modules` surface and hosted-modules mode, twelve legacy tools (`complexity`,
   `god_object`, `community`, `dead_code`, `cloc`, `secret_scan`, `shell_out`,
   `proto_conformance`, `traceability`, `evidence_summary`, `contradiction_scan`, `batch`)
   and the four canned prompts.
3. *Merged*: `find_callees` into `symbol_context`; `entrypoints` into `repo_map`'s header;
   `capabilities`, `server_stats`, `list_sources` into one `status` tool; `rename_impact` and
   `safe_delete_check` into `blast_radius` (`kind: change|rename|delete`); `diff_impact`
   into `review_diff`; the `api_diff` tool into `conserve`; `arch_conformance`,
   `spec_coverage`, `spec_refs` into review internals (spec references ride the
   `obligations` answer the edit hook already fetches); `stale_docs` into
   `staleness_report`'s `doc_drift`; the `discoveries` tool into the session brief. CLI:
   the metrics verbs under one `metrics`, `providers`/`smoke` as a doctor row, the CUE
   shadow verbs out of help.
4. *Frozen, not deleted*: `fathom research` (ADR-046 decision 6 routes the retro corpus
   through its bundle) and `fathom assess` (this record marks ADR-046 frozen) leave help and
   stay callable until an owner pull or a superseding record.
5. *Every removal is proven absent.* A `gone:` contract on the registration literal (or the
   package import), a pinned registry test, callers evidence in the commit, the exemption
   check re-run, dependent claimed plan rows re-anchored (capability survives) or retired
   to `[-] refused` (capability gone), and a `### Breaking` line with a migration note.
6. *Counts after the reckoning*: 25 tools on bare `/rpc` (from 59), 8 on the default
   surface (from 25), 0 prompts (from 4); the default's schema at most 9,000 bytes.

**Alternatives rejected.**
- *Keep the tools, hide them from the default surface only.* Zero-call tools still cost
  schema bytes on `/rpc`, docs, tests and review; law 4 applies to the registry.
- *A hard gate on the default surface.* An error teaches an agent to abandon the surface
  and looks like no demand (ADR-014 amendment 2026-07-16).
- *Deleting the Expert Module system with its tools.* The manifest, `modules.cue` and the
  pack format are on the compatibility surface and used by layers; only the MCP tools had
  zero use.
- *Deleting `research` and `assess` outright.* Both sit under an accepted ADR with claimed
  rows; a freeze keeps the record honest until it is superseded.

**Consequences.**
- Breaking: every removed tool name, the `module_query` wire contract v0, the default
  surface's membership, the stanza (it no longer names `ingest`). One migration note,
  `docs/upgrades/0.9.0-surface-reckoning.md`.
- The compatibility surface line "the 19-tool agent surface allowlist" is replaced (it was
  already stale at 25).
- ADR-035's promoted verbs lose `rename_impact` (folded into `blast_radius`); the five-verb
  lead becomes two on the default surface (`blast_radius`, which now also answers rename
  and delete, and `review_diff`), with `test_impact` and `localize` on `extended`.
- ADR-046: status becomes *Accepted — frozen 2026-09-27* (verb out of help, capability off
  by default, no new rows) until an owner pull.

**Revisit when.**
- SR-M3 shows a tool left off the default called off-surface by agents on at least 5% of
  sessions: move it to the default in place of the least-called default tool (the count
  stays at eight).
- SR-M2 shows agents on the default surface take more turns to a first answer than on the
  25-tool surface.
- A removed tool's replacement cannot answer a question the removed tool answered (a
  filed observation with the question): restore the capability inside the replacement,
  never as a new tool.

## ADR-054: The support matrix is a projection — declared targets, computed states, evidence links; no status is typed (2026-09-27)

**Status.** Accepted 2026-09-27. Owner decision (binding): track implementation quality
and progress for languages, tools and every other supported kind as a projection, never a
hand-maintained status list. Extends ADR-048 decision 2 from languages to every kind.
Design: `docs/design/support-matrix.md`. Plan: `docs/plans/support-matrix.md`. Schema:
`schema/support/`.

**Context.** Fathom now declares support along eight axes — languages, tool formats,
agent clients, spec dialects (N4), rules files (N8.3), coverage formats (T3), contract-key
kinds (N6), dependency ecosystems (N1). Each has, or is getting, a registry the code loads.
Hand-kept status tables in this repo have drifted within weeks (the scope file's "19-tool"
surface; `package_deps` still listed a month after deletion).

**Decision.**
1. *One projection over registries.* Each kind's registry exposes `(kind, id,
   capabilities)`; targets come from the language queue (languages, tool formats) or
   `docs/plans/support.targets.json` (the other six kinds); state is computed from
   committed readings and `fathom verify` of the owning plan row; every cell links its
   evidence.
2. *No reading, no level.* A registered entry without a reading is `tolerated`; a target
   without a registry entry is `declared`; a level is shown only when a reading computed it.
   The state vocabulary is closed and has no member a person sets.
3. *Closure is tested.* A registry entry without a target fails; a target without a
   registry entry fails once its plan row is claimed; a reading naming an unknown entry
   fails.
4. *History is a ledger.* Every reading write appends to
   `docs/bench/readings/support-ledger.jsonl`; burn-up and `regressed` are computed from it.
5. *Surfaces render the one projection*: `fathom support [--kind …] [--json]`
   (`fathom.support/v1`), a console Support page (heatmap, dialects, progress, what's next),
   the generated `docs/support.md` (LB-F17's tracker becomes its language and tool
   sections — one generator), and the roadmap artifact's data. No MCP tool (law 4).
6. *CUE validates, never authors* (ADR-018). `schema/support/schema.cue` vets targets,
   readings, ledger rows and the output; the entries stay JSON and Go data.
7. *Dialect packs are deferred* (owner, 2026-09-27) until embedded grammars near the
   40 MiB cap; the Support page's dialects view shows the running total, which is the
   trigger. This amends ADR-048 decision 5's "pinned packs for the long tail" from
   "planned" to "deferred with a measured trigger".

**Alternatives rejected.**
- *A hand-kept catalog (markdown or JSON statuses).* It rots, and a typed level with no
  reading is the claimed-unproven row `verify` exists to refuse.
- *CUE-authored entries.* ADR-018 rejected CUE as an authoring surface; the registries
  already live in Go and JSON where their owners work.
- *An MCP tool for support queries.* Law 4; the CLI and the generated doc answer it.
- *Client support from the descriptors' `VerifiedAgainst` string alone.* It is authored;
  a local live check (`fathom harness verify`) writes the reading it must agree with.

**Consequences.**
- New: `internal/support`, `fathom support`, `fathom harness verify`, `schema/support/`,
  `docs/plans/support.targets.json`, `docs/bench/readings/support/`, the readings ledger,
  `docs/support.md`, the `/support` console page.
- Changed: LB-F17 and LB-M6-M1..M3 point at `docs/support.md`; `docs/languages.md` is
  removed; `indexbench lang-oracle` appends to the ledger.
- Claude Code and Codex get client readings from existing live tests; Cursor, Copilot and
  Gemini render `tolerated` until verified.
- Compatibility surface grows by `fathom.support/v1` and the targets file shape.

**Revisit when.**
- A kind needs a state the closed vocabulary cannot express (add it to the schema with a
  derivation rule, never a hand-set status).
- Embedded grammars reach 36 MiB (90% of the cap): the packs deferral's trigger fires.
- The closure test is bypassed or loosened to land a registry entry without a target.

## ADR-055: The feature map is fathom's; review stays a family, not a merge (2026-09-28)

**Status.** Accepted 2026-09-28. Owner decision memo of 2026-09-28 (the owner delegated the
call). Inputs: the owner's questions (the feature scorecard as a fathom feature, overrides,
specs as the source, what maps to what, and whether fathom should become the code review tool
and absorb Review Kit); the owner's Feature Scorecard (86
features with owner verdicts, 2026-09-27; summarized in `docs/assessments/2026-09-27-feature-scorecard.md`); Review Kit (`platform-engineering/kits/review-kit`,
`leadsman`) and its fathom uplift brief (`docs/design/fathom-uplifts.md`). Vision:
`docs/vision/39-feature-map-and-review.md`. Design: `docs/design/feature-map.md`. Plan:
`docs/plans/feature-map.md` (FM-, wave 5). Goal G26. Extends ADR-040 (findings loop authority
boundary), ADR-044 (inventory keys), ADR-050 (system graph contract keys), ADR-053 (surface
reckoning), ADR-054 (support matrix as a projection, never a hand-kept status list).

**Context.** Review Kit already covers standards authored as CUE catalogs, soundings,
dispositions, admission with live forge approval, signed execution, system/security/practice
reviews, and a hosted view (DATUM), by explicit design working with no fathom installed.
Fathom's own invariant (ADR-040) is that it never approves, disputes, tailors or closes a
finding; the review kit stays the organizational record. The owner's instinct that review is
superpowered by fathom is right — fathom is the only tool holding the joined evidence (usage,
proof, tests, bench readings, cost) a review verdict needs — but that does not by itself argue
for one codebase.

**Decision.**
1. *The feature map is a fathom feature (D1), computed and evidence-bound.* A feature is a
   named capability anchored to surface keys (`cli:`, `mcp:`, `route:`, `rpc:`, `page:`, public
   API symbols; ADR-044/050 keys). Source of truth: an authored `docs/features.md`
   (plan-grammar-style rows: id, name, keys, owner, cited ADR/spec) plus auto-proposed
   candidates from the surface inventory; unmapped keys are reported by a closure check (like
   ADR-054). Fathom never decides the verdict.
2. *Every fact per feature is a join, never a new engine* (the join rule, ADR-051 decision 1):
   code reached, specs/plan rows citing it (proven/open/regressed), ADRs, tests and live
   certificates with risk-weighted coverage (T1/T2), usage where a source exists (fathom's call
   and hook logs; for a service, an imported access-log or analytics count file as evidence,
   otherwise "unknown", never guessed), cost and debt interest (U1/U2 per feature), churn,
   owners (CODEOWNERS + history), docs linked (KR), flags and config read (env:/config:),
   dependencies (pkg:), incidents citing it.
3. *Implementation health is a published rule table*, the same shape as the risk band (CX-2):
   the strongest matched factor wins, never a summed opaque score, every reason cited with its
   evidence. Value needs usage or a human; where no usage source exists it shows "not measured".
4. *Verdicts (keep, improve, merge, ditch)* are authored by a human as a cited claim in the
   features file, or proposed by the decision port as a proposal only, calibrated against the
   owner's 86 labeled verdicts of 2026-09-27 (agreement bar at least 0.8 at n at least 3, else
   the proposal path is dropped). Never auto-applied; a ditch proposal flows into the
   surface-reckoning pattern (callers proof, re-anchor or refuse the citing rows), never a
   direct delete.
5. *Overrides and criteria are a Standard*, which is Review Kit's job (a `product-portfolio`
   catalog). Fathom's local fallback is a small `.fathom/features.cue` rule table, the same
   shape as the risk table, with defaults in the binary. The spec standard is the source:
   `docs/features.md` rows use plan grammar v2 anchors, and ADRs stay the generic spec that
   features cite.
6. *Surfaces:* `fathom features [--json]`, a console page (value x implementation scatter with
   verdict shapes, a filterable table, an underused list, a ditch list), and an artifact-style
   export. The Feature Scorecard artifact becomes the first reading this reproduces.
7. *Do not merge Review Kit into fathom now (D3). One family, two tools, one contract.* Into
   fathom (code-grounded, per change, computed): the change risk receipt (U7), SARIF and
   checks, the feature map, conservation, certification facts, the architecture drift radar
   (U11); `fathom review` is the code-review surface. Stays in Review Kit (organizational
   assurance): standards and catalogs, requirement severity, dispositions (dispute, accept,
   tailor), admission with live forge approval, signed execution, and the non-code domains
   (infra, ops, well-architected, security posture, economics, practice, org controls) plus
   DATUM (hosted).
8. *Instead of merging:* (a) a versioned evidence contract (the uplift brief's P0) so every
   fathom lane Review Kit consumes is stable and receipted; (b) Review Kit's own code-parsing
   may optionally consume fathom when present, keeping its standalone path; (c) Review Kit
   concern packs reach the edit through the findings outbox (amended 2026-10-10, ADR-067: no
   compiler to obligations exists, see the amendment below); (d) a
   family bundle: one installer and one plugin that installs fathom plus leadsman, shared
   nautical naming (fathom, leadsman, sounding, datum), shared licence (FSL). Review Kit's
   eventual move into its own repository beside fathom is a decided future, not built now (the
   owner, 2026-09-28 — see Consequences).
9. *Sequencing.* After `docs/plans/system-graph.md#SG-2` (contract-key storage) and
   work-intelligence's U1/U2 rows; plan prefix FM-, wave 5. Measurement: re-derive fathom's own
   86-feature scorecard; evidence columns match the hand-built one on at least 90% of cells;
   verdict proposals at least 0.8 agreement or dropped.

**Alternatives rejected.**
- *Merge Review Kit into fathom outright.* Authority: fathom's invariant and ADR-040 say
  fathom never approves or closes; admission and dispositions are authority by design. Hosted
  and org features conflict with fathom's non-goals and tenancy. Review Kit's independence rule
  (works with no fathom) is what lets an organization adopt it without the index. Scope: fathom
  is shrinking its surface (ADR-053); absorbing roughly 170 Go files of a different domain
  undoes that. The valuable part of a merge — one install, one experience — is a distribution
  problem solvable without merging code.
- *A hand-maintained feature list.* Drifts within weeks like every hand-kept status table has
  (ADR-054's precedent).
- *Fathom deciding or auto-applying a verdict.* The same authority boundary (ADR-040) and the
  refusal of a model deciding done (ADR-049 decision 12).

**Consequences.**
- New: `internal/featuremap`, `docs/features.md` (authored, outside `docs/plans/`), a
  `.fathom/features.cue` local fallback, the `fathom features` verb, the console Features page,
  an artifact-style export, plan `docs/plans/feature-map.md` (FM-).
- Review Kit gains a versioned evidence contract to consume (its uplift brief's P0); its
  concern packs reach the edit through the findings outbox import (amended 2026-10-10, ADR-067).
- `docs/intent/futures.md` gains two entries: "Review Kit merge into fathom" (contingent on
  this record's Revisit-when below) and "Review Kit in its own repo beside fathom" — the
  owner answered 2026-09-28 that this is where Review Kit eventually lands, but not now:
  recorded as a decided future (confidence likely, horizon later), not an open question.
- `docs/intent/scope.md` gains goal G26, gated on the feature-map measurement instrument
  (`fathom plans status`).

**Revisit when.**
- At least 70% of Review Kit's requirements evaluated in a quarter depend on a fathom evidence
  lane.
- At least 5 adopters exist and every one installs both fathom and Review Kit.
- The evidence contract takes more than 3 breaking changes in a quarter (the seam costs more
  than it saves).
- The owner decides to sell one product (then bundle first, merge code last).

**Amended 2026-09-29 (owner decision, recorded with ADR-056): model verdict proposals are deferred, not refused.**
Decision 4's second path, a verdict proposed by the decision port, is switched off until it is earned. Verdicts
stay human: an authored, cited claim in `docs/features.md`. Model proposals return only after J0
(`docs/plans/judged-predicates.md` JP-M1) reads PASS and a calibration against the owner's 86 labeled verdicts of
2026-09-27 reaches the bar (agreement of at least 0.8 at n of at least 3). The plan rows FM-10 (the proposal),
FM-11 (a ditch proposal routes into the surface-reckoning pattern) and FM-M2 (the calibration reading) in
`docs/plans/feature-map.md` are `[-]` deferred with that trigger, and G26's done-when no longer waits on FM-M2.
Everything else in this record stands: a ditch proposal never auto-applies, and fathom never decides or applies a
verdict. ADR-056's research pass had first read the path as refused because 86 labels over four classes look
too few for the bar and a model "ditch" could anchor the decision ADR-053 requires to be evidence-driven; the
owner kept the option open instead, so the calibration decides. Future F51.

**Amendment (2026-10-10, ADR-067).** Decision 8(c) and the Consequences bullet that repeated it are corrected. No
compiler from Review Kit concern packs to fathom obligations exists in either repository. A pack's findings reach
the edit through the findings outbox import (ADR-040). A kit's guards and formats may ride tech packs once ADR-061 is
accepted. A resource-shaped rule stays policy-kit's (platform-engineering ADR-0095 decision 5, ADR-063 O11), so no
tech pack carries one. The compiler is not built until a pack consumer exists (P20).

## ADR-056: Judgment at the port, proof first: level 10 through fathom, optional per site, measured before it is built (2026-09-29)

**Status.** Accepted 2026-09-29. Owner directive of 2026-09-29: use level 10 of the ten-levels series (agentic Jev)
powerfully in fathom while keeping Jev optional, and land every decision from the research pass in the durable
records. Inputs: the design brief (five centerpieces, two designs, nineteen ideas, ten findings), an invariants
critic, a cost critic, the owner-architect notes of 2026-09-29 (binding where they apply), and the public level 10
source. Amends ADR-042 (decisions 3 and 5 gain a category and a per-site opt-in; I14's enumerated list widens) and
reads ADR-046, ADR-049 (decisions 2 and 7), ADR-053 and ADR-055 as constraints. Vision:
`docs/vision/40-judgment-and-exploration-maps.md`. Designs: `docs/design/judgment-foundations.md`,
`docs/design/test-run-hook.md`, `docs/design/judged-predicates.md`, `docs/design/judged-telemetry.md`,
`docs/design/exploration-maps.md`, `docs/design/carry-across.md`, `docs/design/work-signals.md`. Plans:
`docs/plans/judgment-foundations.md` (JF-), `docs/plans/test-run-hook.md` (TR-), `docs/plans/judged-predicates.md`
(JP-), `docs/plans/judged-telemetry.md` (JT-), `docs/plans/exploration-maps.md` (XM-), `docs/plans/carry-across.md`
(CA-), `docs/plans/work-signals.md` (WS-). Goals G27 to G32. Principle P24, invariants I60 to I62, futures F34 to
F51, questions Q31 to Q33. Amended the same day by the owner's decision on feature-map verdict proposals
(decision 13 no longer refuses them: deferred, futures F51; ADR-055 decision 4 carries the dated amendment).
Amended 2026-10-16 by ADR-071: decision 13's refusal of output rewriting by fathom no longer covers readouts' view
of a successful Bash result (ADR-071 decisions 2 and 3: lossless by default, lossy only with a recovery and a
receipt); command execution by fathom stays refused.

**Context.** Level 10 gives the agent one tool that runs a command, assembles one state and asks a fast typed model
the agent's own questions, hiding the output from the agent. Fathom already has the decision port (ADR-042), a
`typesafe` provider, and a call log that shows the tools an agent must choose to call die (about 98k MCP calls:
localize 40, test_impact 103, discoveries 13) while hooks and threads survive. Nobody has measured a typed
judgment model's accuracy on code. Four facts shaped the answer: most of what level 10 asks has an exact answer
fathom computes with no model; the `route` site failed calibration (45.7 percent right at confidence 0.9 or more,
non-monotonic); a site added the usual way switches on in every repository that sets a provider; and the
rule-outcome events governance learning reads measure scope persistence, not compliance.

**Decision.**
1. *The decision-model bet (principle P24).* Fast, cheap typed-judgment models (Jev is the first) will keep
   improving in accuracy, calibration, price and latency, and fathom rides that curve without redesign. Boundary:
   judgment never replaces proof for anything fathom can compute, and a model answer is never a receipt a stranger
   can re-derive, however good the model becomes. Five rules. (1) Every judgment is a typed decision-port site
   (schema plus provider: none, llm, typesafe or a future one), never an ad hoc model call. (2) Every site owns an
   adjudicated calibration set that grows (sampled judged telemetry plus one-key human verdicts), and `fathom
   decide calibrate --provider X` re-grades every site, so adopting a better model is a measurement and not a
   redesign (JF-12). (3) Features improve monotonically: deterministic with provider none, judgment used only in
   the confidence bands where calibration has passed, bands that widen automatically as calibration improves and
   narrow when a model moves (JF-23), and no feature whose value exists only if a future model arrives. (4)
   Judgments are cached by (question template, content hash), stamped with provider and model version and kept
   apart from proof, so a better model re-grades cleanly. (5) No vendor lock-in: the port's primitives (choice,
   score, abstain) are generic and any fast model plugs in as Jev did (decision 11 closes the remaining leaks).
   Futures follow the bet: one per site, "Promote when provider X passes calibration on this site".
2. *Program rules R1 to R8.* R1 structural routing: named operators or existing tools answer every computable
   question and a model never decides what is computable. R2 new sites never enter the default site list. R3 bars
   use Wilson intervals: a calibrated hint needs 90 of 100 right at the shown confidence (ADR-044 decision 8); a
   judgment that prunes, gates or reorders a default list needs a 95 percent lower bound of at least 0.90 (the O113
   lesson); a free-form question is shown to the asking agent labeled uncalibrated and never prunes or reorders.
   R4 human labels go to one template at a time. R5 no table, durable memo, sampler queue or background judge
   before RT-M1 reads PASS (ADR-049 decision 2). R6 no new MCP tool (ADR-042 decision 5, ADR-053); exposure only
   on call-log demand from at least 5 percent of sessions, folded into an existing extended tool. R7 a judgment
   never asserts a code fact, prunes silently, sets a status or blocks, and a model never prunes on a security
   property. R8 pin the model, log the resolved model, mark a site "recalibration due" on any move.
3. *Every new decision site is opt-in per repository, even when a provider is set (this is what makes Jev optional
   per feature).* `DefaultDecide.Sites` is frozen at today's list minus `route`; `decide.OptInSites` lists
   `triage_failure`, `ask`, `explore`, `compact_advice` and the `judge_*` family; `SiteAllowed` refuses an opt-in
   site unless the repository's own `[decide] sites` names it. A missing key, a timeout, a 429 or 529, refuse mode
   or safe mode each produce one disclosed line and the deterministic path. Provider `none` stays the default and
   stays byte-identical (I42); `llm` serves the CLI and background work only; `typesafe` may serve a hook through a
   daemon endpoint. Calibration is per (site, template@version, provider, resolved model).
4. *A new category: judgments the agent directs.* ADR-042 decision 3 covered judgments fathom makes while serving.
   A judgment the agent asks for (a free-form question over spans the graph selects) is allowed as a CLI verb and a
   lazily loaded skill, shown to the asking agent as uncalibrated, and it never prunes, reorders, sets a status or
   gates. Decision 5 stands: it adds no MCP tool.
5. *I14 amendment.* The decision port may open from the daemon, or from any explicitly invoked CLI verb whose site
   is listed in `[decide] sites`; it never opens from a hook process directly (a hook reaches a daemon endpoint).
6. *Fathom's level 10 is the test-run hook, tiers 0 and 1, provider none (accepted).* On the repository's declared
   test command (PostToolUse and PostToolUseFailure of Bash) it parses the run and computes, by localize and test
   impact over the working-tree diff, the changed symbols each failing test reaches, and injects one line, nothing on
   a pass. It never rewrites output, blocks or runs a command. Opt-in behind `[hooks] test_run`, default-on only
   after its bench cell: 20 mutation-seeded test-fix tasks on a repository fathom did not write, n = 3, arms
   control, the hook and raw level 10 (bench sandbox only). Bar: at least 20 percent fewer tokens-to-green than
   control, the paired CI excludes 0, correctness not lower. Kill: the line stays off; if raw level 10 beats the
   hook, publish that. Tier 2 (a failure-kind choice, site `triage_failure`) is deferred.
7. *J0 first (accepted): a one-way kill screen.* About 1,000 spans from fathom and two open source Go repositories;
   oracle templates the index computes; a lexical baseline arm; a 150-unit residue set labeled by a frontier pass
   with the owner adjudicating every disagreement plus 20 percent; bars: direct-visible AUROC at least 0.9 and recall
   at least 0.95 at a prune rate of at least 0.5, beats the lexical baseline by 0.05, context assembly lifts
   transitive recall by 20 points, residue precision at least 0.9. Any miss, or grep tying the model, means no
   code-reading site is built (`ask`, the traversal scorer, files at scale). Alongside it, an I3 masked-plan-row
   smoke test that never counts toward SI-M4.
8. *Corrections accepted.* The rule-outcome events are renamed `still_served` and `top_changed` and every reading
   is re-labeled (they measured scope persistence, not compliance); `route` leaves the default site list (the code
   stays, SR-44's precheck stays; re-admission needs 100 replayed prompts against R3); `certify_suggest` computes
   the strength levels the evidence permits and offers the model only those; calibration is a reading per (site,
   template, provider, resolved model), not a fitted map; two design notes that assumed Jev generates text are
   corrected, and conservation's reach is stated exactly (it does not catch an expected constant edited to match
   buggy output).
9. *Experiments with named bars and kills.* (a) Carry-across after compaction: verify SessionStart(source=compact),
   read 14 days of transcripts, stop if the median compaction has under 2 re-reads or re-reads are under 10 percent
   of post-compaction reads, else A/B by session hash (re-reads down 30 percent, CI excluding 0); a pass overturns
   `cmdThreadBrief`'s deliberate silence on compact and amends I12. The compaction advisor itself is not a fathom
   feature: a hook cannot compact. (b) Exploration maps shared from planner to sub-agents, measurement first: build
   only if at least 15 percent of sub-agent reads are re-derivation, then an A/B (25 percent fewer sub-agent tokens,
   no accuracy loss, under 5 percent stale entries). Binding design: entries anchored to symbol and content hash;
   maps compose by reference as a DAG (cycles refused, depth 4), the handoff carries one id and fathom mints a
   composite of references for several; live maps and semantic layers are complementary axes (what matters for a
   purpose versus what is true or binding), live maps query layers and scope expensive columns; maps arrive through
   handoff, location-relevant hook lines, the session brief and threads, never by an agent choosing to look; no new
   default tool. (c) Stall detection: deterministic detectors folded asynchronously by the daemon (never inside a hook), checked against
   abandoned sessions and later green runs, an optional typed classification of ambiguous episodes (productive
   iteration versus stuck: sampled, calibrated against the owner's labels, never required), an advisory nudge at
   most once per episode and opt-in, A/B by session hash (kept only if episodes shorten and cost per proven weight
   point drops), stall cost in reports per session, model, harness and zone, and fathom never switches models,
   respawns or dispatches (it may write a signal an orchestrator reads). Plan rows: `docs/plans/work-signals.md`
   WS-1 to WS-7. (d) The U6 preflight model against a fitted logistic baseline (bar 0.05 held-out AUROC).
10. *Estimated weights and cost per weight point (owner, accepted).* A deterministic estimator proposes row weights
    shown beside authored weights and never overwriting them; the author accepts one with one command; a model may
    refine an estimate only after it passes calibration, and a refinement is a proposal for the author, so progress
    computed by `fathom verify` stays model-free (ADR-049 decision 7 holds). Estimates are calibrated against actual
    effort (tokens, turns and wall time per proven row) and the error is published (WS-M3). Economics show cost per
    proven weight point per model, harness and zone, keyed by agent identity (I58), on the Progress and Economics
    pages (WS-10, PE-22). Plan rows: `docs/plans/work-signals.md` WS-8 to WS-11.
11. *Hexagonal hardening of the port (owner, accepted; implemented in agentic-kit).* A generic kind name with the
    vendor term as a wire alias, adapter-declared limits, adapters in subpackages, a `Capabilities()` declaration, a
    conformance kit, composition as decorators (cascade, fallback, cache, record and replay) and a calibration
    mapping in the core. It is recorded in agentic-kit's decision-port plan (`docs/plans/decision-port.md`, section
    "Hexagonal hardening", rows DP-12 to DP-24, and its decision 0017); fathom then bumps its vendored mirror and moves
    its call sites (JF-13, JF-14, JF-24).
12. *Deferred, each on a named trigger (futures F34 to F51):* `fathom ask` (J0 passes); tier 2 `failure_kind` (at
    least 100 failing runs later green, and J0 has run); question-directed traversal (the maps A/B passes); files at
    scale (J0, RT-M1 and a named consumer's own metric); when-to-compact advice (carry-across passes and route is
    re-admitted); fitted recalibration and auto-disable (100 labels for one site, provider and model); the
    judged-telemetry sampler, queue, background judge and table (RT-M1 PASS, plus the owner's call to unfreeze
    assess or supersede it); guard proposals from relabeled rule outcomes; MCP exposure of test-run or ask output
    (5 percent of sessions); a `--plan-only` export (a named consumer); map set algebra; ADR-to-rule pair
    judgments, doc-drift triage, an injection screen, skill suggestion, friction classification and product feedback
    (each its own trigger); durable live maps (F50); and model proposals for feature-map verdicts (F51: the owner
    decided 2026-09-29 that this is deferred, not refused, and verdicts stay human until J0 passes and a
    calibration against the owner's 86 labeled verdicts reaches the bar; FM-10, FM-11 and FM-M2 in
    `docs/plans/feature-map.md` are `[-]` deferred with that trigger). Graduating a judged template to claims at tier
    ai-derived needs its own ADR promoting F7.
13. *Refused.* A public `fathom triage` verb as the delivery, and `ask`'s computable-operator grammar as a new verb
    (tools agents must choose to call die and the operators duplicate existing tools); judged rule compliance now
    (contradicts governance-learning section 11 and ADR-049 decision 7, and no labels exist); Jev for answer
    usefulness, config custodian proposals and product digests (joins over structured events, and narrative is a
    lens, P14); computable labels as model work (generated files, config versus code, public API, ownership, I/O
    reach, TODO admission); model-judged security properties as a served signal (X6, ADR-051); a Stop-hook
    completion check judged by a model; commit classification; model inputs to the risk band; model gates on
    destructive commands or secret writes (the harness owns them, P11); generated text from Jev; output rewriting and
    command execution by fathom; a judged-telemetry lane parallel to assess.
14. *Labels and spend.* The one labeled template is `failure_kind`; J0's residue set (50 to 80 owner labels, once)
    is the only other human labeling; labels arrive in batches of ten through the one-key verdict with provenance
    (human, frontier, computed). Bench spend in order: J0 plus I3 (about $0.25 and 1 to 2 days), the level 10 cell
    ($90 to $360), transcript readings ($0), the `ask` and traversal benches only after their triggers ($110 to
    $720). Provider spend is under $1 across all of it.

**Alternatives rejected.**
- *Port level 10 as is: a tool the agent calls, with fathom running the command.* The agent must choose to call
  it, and fathom does not run commands or rewrite output; the exact questions have exact answers.
- *Let a model route computable versus judged questions.* The `route` site failed at exactly that shape.
- *Ship the whole judged-telemetry loop now.* Labels, not tokens, are scarce; the loop needs a table and a worker
  that RT-M1 blocks, and a parallel lane would rebuild assess's consent and spend isolation.
- *Record every judgment as a claim.* ADR-042 rejected it; judgments stay in a memo until a template graduates.
- *A fitted calibration map now.* It cannot repair a non-monotonic failure and a fit at n near 100 is noise.
- *Make `typesafe` a default provider, or leave new sites on whenever a provider is set.* It breaks optionality
  per feature.

**Consequences.**
- Seven plans, seven design documents, vision 40, goals G27 to G32, principle P24, invariants I60 to I62 (I14
  amended), questions Q31 to Q33, futures F34 to F51, a dated amendment line on ADR-042 and another on ADR-055
  (verdict proposals deferred, not refused).
- Code follows through JF, TR, JP, JT, XM, CA and WS rows, each gated by `fathom verify`; nothing here is built
  by this record.
- The tool count is unchanged; the default surface stays at eight tools (ADR-053).
- A judgment is labeled with its tier and calibration state wherever it is shown, and every removed row is listed.

**Revisit when.**
- The J0 reading lands: a kill closes `ask`, the traversal scorer and files at scale; a pass opens F34.
- The level 10 cell reads: arm B misses the 20 percent bar (the hook stays off) or raw level 10 beats it (publish
  and consider tier 2 sooner).
- Any site's calibration reading clears or fails R3's bars at n of at least 100 (open or drop that template).
- RT-M1 reads PASS (the memo, the sampler and files at scale may be reconsidered).
- A site is found on by default when a provider is set (the opt-in guard leaked).
- The maps re-derivation reading or the carry-across Reading 0 lands (build or stop).
- Hook or CLI use reaches 5 percent of sessions (MCP exposure).
- The provider situation changes: Jev signups were paused on 2026-09-22 and its OpenRouter access is alpha; the
  design assumes nothing depends on it, and a withdrawal only removes an optional provider.

## ADR-057: Served is not kept: a lost pin is a fact about the change (2026-09-29)

**Status.** Accepted 2026-09-29. Decisions 1 to 9 by the owner: the commit pin line in record mode (PR-1 to PR-7), the two ledger fixes that ride SR-61 (PR-10, PR-11), and JF-1 landing alone as the O120 fix. PR-8, that a silent pin loss blocks at pre-push, accepted the same day by the owner-architect and confirmed by the owner the same day (Q34 answered; the `PR-8 decision.` line below). Inputs: the serve-time promises design after two critics (one on value, one on cost), the `test_ran` base-rate reading (`docs/bench/readings/served-not-kept/test-ran-base-rate-2026-09-29.md`), pilot 8's i28 and i19 trials, and CV-M1 and CV-M2. Reads ADR-038, ADR-044, ADR-049 decision 9, ADR-053 and ADR-056 as constraints. Design: `docs/design/served-not-kept.md`. Plan: `docs/plans/served-not-kept.md` (PR-). Goal G33. Futures F53 and F54. Questions Q34 and Q32 (extended).

**Context.** Fathom measures whether it served a rule and never whether the rule was kept. In the second guards pilot the rule for task i28 was served before the edit in both trials, and both trials violated it: each rewrote or renamed the test that pinned the rule, after which `go test -run` on the old name matched nothing and reported `ok`. A first design answered with a promise system (an open promise set, ledger schema 2, a check registry, waivers, edit, turn and commit deadlines, five states, a reclassification of rule effectiveness, Jev annotations on prose promises). Two critics showed it was about ten times too big for what can be measured: in production every rule a per-edit check could evaluate is already denied before the edit (ADR-038; 8 denials in 15 days, all in the bench, and 2 of 837 pre-edit guard payloads from Claude Code on this repository); the guard evaluators are fragment-tuned and misfire on existing content (45 of 366 non-test `cmd/fathom` files fail `workspace_output` when checked whole) or read kept vacuously for an advisory rule; `test_ran` has an 85 to 94 percent base rate on code scopes and a false-kept hole on `[no tests to run]`; a per-edit pin check reads a moved test as deleted; and the rule-outcome ledger has never held a row. The one gap left is the pin. CV-M1 flagged both i28 trials it replayed as `deleted`, and CV-M2 found 0 false losses in 50 finished commits. *Corrected 2026-09-29 (O160):* CV-M1's replayed cells are the 2026-09-21 round's `guards-fathom-r2`, not pilot 8's, as CV-M1 had labeled them; its measurements stand, and pilot 8's own i28 grades are its reading's.

**Decision.**
1. *Three placement questions, in order.* Is there a deterministic check (if not, the rule is advice: serve it, do not promise it, count it as unmeasured and never as ignored)? What is the earliest moment the check sees everything it needs (the proposed edit for a fragment, where it denies; the commit for a rule that spans files; the end of the turn only if measurement shows a gap)? Can the agent still act at that moment (if yes, one line: a failed fact with a location; if not, a record or a gate, never feedback)?
2. *The corrected ladder.* Fragment-checkable rules go from advise straight to deny. Whole-change rules go from advise to the commit line to the pre-push gate. There is no permanent middle rung of "advise, then check after": ADR-038 and SR-45 say a checkable rule denies.
3. *The pinned test is the bridge from prose to a check.* A prose rule that names its test becomes checkable by protecting that test. The trigger is the source that already governs (an invariant's test column, a `// certifies:` marker, a `tests/*.cue` manifest entry, a scope evidence line, a guard's `done needs:`). No new trigger language, and promises are never parsed from agent prose.
4. *The commit pin line.* On a `git commit` seen by PostToolUse(Bash) the hook reads the sha from git's own `[branch sha]` summary line (no summary line, no check), runs `integrity.Check` from the parent to the commit over the changed Go files with the CV-7 guarded set and no reach (a hook opens no store), and adds one line per lost pin, at most three per commit. Loss kinds: `deleted`, `renamed`, `weakened`, `skipped`, `golden_changed`, `marker_moved`; `edited` is silent (ADR-056 decision 8). The check keeps no state; one `git cat-file --batch` process reads the blobs; a 250 ms sub-budget inside the hook's deadline (I53), and on a miss it skips silently and its beat says `skipped`, never kept. One `pin` beat per checked commit goes into `hooks.jsonl` with no source text. `[hooks] pin_check = off | record | feedback`, default `record` until PR-M1 passes; `off` and `record` leave the payload byte-identical; Claude Code first, other harnesses record-only until PR-V1. The line is under 240 bytes, a fact with a location and never the rule text, never a description of the detector and never a waiver hint; silent when nothing is lost; it notes when the same commit edits the source that names the pin. It runs whether or not the rule was served: a lost pin is a fact about the diff.
5. *Jev, weighed.* Jev never resolves kept or broken (R7, I62; judged compliance is refused, ADR-056 decision 13, and a hook never opens the port, I14). Its only credible role is upstream: `propose_check`, a typed choice over the closed check kinds for a prose rule, accepted by a human (F53, deferred; about 21 labeled rules against a bar of 50). With provider none, the default, everything here runs unchanged.
6. *Experiments with kill criteria.* PR-M0, a $0 replay of the guards-pilot 6 and 8 diffs through the check, before PR-3 is built. PR-M1, 30 days of record mode on owner traffic with the owner adjudicating every fire. PR-M2, the bench: the enforced-arm rerun ADR-038 still owes plus the pin line in feedback mode against the same config with the line off, oracle-graded, 5 trials per task per arm (Fisher two-sided p of about 0.02 for a drop from 8 of 10 to 2 of 10), about 40 cells and about $76, which needs the owner's approval (`docs/intent/autonomy.md`). Kill criteria K1 to K9 are in the design.
7. *Deferred, each on a named trigger (PR-D1 to PR-D10):* `test_ran`; a one-shot Stop block for turn-end feedback; `enforce: warn`; `fathom claim waive` with a review receipt; the open set, ledger schema 2 and a check registry; the GL-11 reclassification; `propose_check` (F53); orchestrator constraints on sub-agents (F54); unserved scanning as a receipt-only counter; per-agent keeping tables keyed by agent identity (I58).
8. *Refused (PR-X1 to PR-X11):* a model setting kept or broken; a bypass or waiver command inside the line; turn-end lines delivered on the next prompt; whole-file evaluation of fragment checks after each edit; a per-edit single-file pin check; `denied_retry`, `doc_touched` and `verify_row` promise rows; automatic suppression of a rule's checks at a waiver rate; moving the thread evaluators into a shared registry; promises parsed from agent prose; a new MCP tool or top-level verb; a pin inferred from test reach, a blast-radius trailer or nearby tests rather than named by a governing source (precision over coverage).
9. *Two ledger fixes and one rename.* The enforce deny path writes `blocked` to the rule-outcome ledger (PR-10), and a rule served by the pre hook still writes its `served` row on the paired post call (PR-11); both need SR-61's writer deployed. JF-1 lands alone and drops its clause that keeps `violated` "for a rule with a checkable condition": the tracker evaluates no check, checkable rules deny and record `blocked`, and pin losses have their own beat, so the name would have no writer and would invite O120 back (O120 carries the addendum).

**PR-8 decision.** accepted 2026-09-29 by the owner-architect and confirmed by the owner the same day (Q34): add `fathom conserve --strict-plane integrity`, exiting 1 only on a silent loss (the lost test is still named by its source at head; a pin retired on purpose, whose naming source was edited in the same range, is printed and does not fail), and have lefthook's `refactor-conserve` use it (PR-9). Evidence: CV-M1 flagged both i28 trials, CV-M2 found 0 false losses in 50 commits. It changes the advisory-only pre-push doctrine (`coding-agent-excellence.md` section 5.3) for the integrity plane only, and only for a loss whose naming source still names the test at head. Nothing blocks until PR-9 is built and proven.

**K4 fired (2026-09-29, O161).** PR-M0's replay found that 3 of the 5 oracle-violating i28 cells (guards-fathom-r1 control t1, guards-fathom-r2 control t2 and full t1) also rewrote I28's row to name their new test, so PR-9 read each as a retired pin and blocked 2 of 5; none added a decision naming the lost test (the r2 ADR-025 amendments name `BackendForPath`, the r1 cells only their own plan rows). The third Revisit-when clause's response is taken: incident INC-26, and PR-12 exempts a retirement at pre-push only when a line added in the range names the lost test in a decision record (`ADR.md`, `docs/adr/**`, `docs/intent/decisions/**`) or on a dated decision line (an ISO date and decision, decided, retire, retired or retires) in any changed file, never under `docs/plans/**`. On the same replay PR-12 blocks 5 of 5 and passes the clean cell; over the last 50 guarded commits of main it found no retirement, so it blocks nothing PR-9 passed (`docs/bench/readings/served-not-kept/pr-m0-replay.md`, addendum). The Revisit-when clauses stand.

**Alternatives rejected.**
- *The first design (PK-): the open promise set, ledger schema 2, the check registry, waivers, deadlines, five states, the GL-11 reclassification and Jev annotations on prose promises.* Every checkable per-edit rule already denies, `test_ran` has no headroom, a per-edit pin check misreads a moved test, and the ledger it would feed has never held a row. Its framing survives; its build does not.
- *`test_ran` at the turn deadline.* 85 to 94 percent base rate on code scopes (404 transcript and guard pairs), a false-kept hole on `[no tests to run]`, doc-scope tests that cannot be affected, and pre-push already runs every test.
- *A per-edit, single-file pin check, or the fragment evaluators over the whole file.* A moved test reads as deleted; fragment-tuned checks misfire on existing content.
- *A waiver or bypass in the line.* In i28 the agent took the cheapest exit; a sanctioned bypass beside the fact becomes the new cheapest exit.
- *A model judging compliance.* R7, I62 and ADR-056 decision 13.
- *Feedback delivered on the next prompt, or after the push.* It arrives after the agent reported done, which is the served-but-ignored channel again.

**Consequences.**
- New: `docs/design/served-not-kept.md`, `docs/plans/served-not-kept.md` (PR-; PR-8 decided, every build row open), goal G33 (a bet), futures F53 and F54, question Q34 and an extension of Q32 (PR-V1 to PR-V3), a bench reading (`docs/bench/readings/served-not-kept/test-ran-base-rate-2026-09-29.md`), observation O123, the vocabulary pin loss in `docs/intent/domain.md`, and PR-M2's spend line in `docs/intent/autonomy.md`.
- Nothing is built by this record. The tool count is unchanged (no new tool or verb); the one new config key is documented and given a CHANGELOG line when PR-6 lands.
- JF-1's text changes as decision 9 says; O120 carries the addendum.
- The pattern is project-agnostic and goes to agentic-kit's guidance ("Served is not kept: the promise pattern for agent harnesses").

**Revisit when.**
- PR-M0, PR-M1 or PR-M2 reads (K1 to K5 and K9 decide what stays).
- Any false block or a false fire is adjudicated by the owner (name the loss kind).
- An agent retires a pin by editing its source with no decision cite (K4): file an incident and require the cite for the pre-push exemption.
- Fewer than 20 pin checks a week on registered roots (K6): nothing deferred is built on volume grounds.
- PR-V1, PR-V2 or PR-V3 reads (non-Claude harnesses, what a Stop block does, sub-agent `agent_id`).
- The owner reverses PR-8: record the rejection on the `PR-8 decision.` line, reopen Q34 and mark PR-9 refused.

## ADR-058: Incident maps are committed declarations, not a map kind; the suspect list comes first; outside systems are reached by link and typed parameter, never by fathom (2026-09-29)

**Status.** Accepted 2026-09-29 as a direction, with the build deferred behind measured stages. Owner request of
2026-09-29: capture the incident-map idea durably, in particular pre-shaping incident knowledge before an incident and
a kind separate from live maps, and analyze connections to other data sources and CLI commands, with the
implementation deferred. The owner endorsed pre-shaping and the separate kind. The dispositions on outward connection
below (links and typed parameters accepted for the design, probes deferred, attach and connectors refused) are the
analysis's recommendation, recorded with reopen triggers and open to the owner's revision. Amends section 3c of
`docs/design/exploration-maps.md` and wave 4 of `docs/plans/exploration-maps.md` (XM-26 to XM-30, XM-M6 and XM-M7
move, their ids stay reserved) and reads ADR-056 as a constraint. Design: `docs/design/incident-maps.md`. Plan:
`docs/plans/incident-maps.md` (IM-). Futures F56 and F57. Numbered 058 because ADR-057 (served is not kept) and futures F53 to F55 were
recorded first.

**Context.** Section 3c made incident maps a kind of live map, gated on live maps (XM-D1, whose trigger is XM-M2 and
RT-M1). Two reviews of the first draft (an invariants critic and an SRE and security critic) found: the map model's
machinery is unused by a static footprint, so calling it a map only escaped F50's gate; XM-M2 (sub-agent handoff
tokens) gated incident maps only to work around plan-grammar defect G2-6; probes, MCP next queries and attach were a
command runner and a connector in effect, with no meter that could ever kill them; and the stored commit table is
the wrong source for a suspect list (500 commits of HEAD, author dates, no merge files, no renames). Call-log
evidence says pull-shaped features die here (`localize` 40 of about 98k calls, discoveries 13, anchored memory 0, the
`history` tool removed at 0 calls), while hooks and forced channels survive. Rare use is an incident map's success
state, so a usage kill clock is wrong for it and readiness is the liveness measure. The practice evidence is modest
and cited in the design: change is the first suspect, correlation is not cause, and prepared playbooks rot.

**Decision.**
1. *The kind.* An incident map is a declaration in the file .fathom/incidents.cue (not yet created), committed beside
   `.fathom/guard.cue`, reviewed in a PR and readable when fathom is down: symptoms (alert names, error literals), a
   footprint (symbols, paths, contract keys), a window (a duration or `since_tag`), an in-repository runbook path,
   links, and authored `blind_spots`. It is not a member of the map family and has no `map` verb; it may later name
   a live map as its `scope` (IM-D4). Users still call it an incident map.
2. *The core facet is the suspect list, computed and never stored.* At request time, from a bounded read-only git
   read through `internal/gitro` (committer dates, first-parent landing order, merge files, renames, a scan cap with
   the remainder counted, a timeout, reverts and bulk commits marked, a shallow clone named as a lower bound), with no
   new table (this resolves the RT-M1 collision: R5 holds). Rows carry no author field (I58). The stored 500-commit
   horizon is not used. Two defects in shipped code found on the way are fixed independently (IM-F1: the CODEOWNERS
   matcher misroutes leading-slash and any-depth patterns; IM-F2: gitro must set `GIT_NO_LAZY_FETCH=1` and
   `GIT_TERMINAL_PROMPT=0`).
3. *The answer never claims more than its inputs.* Every facet reads a value, "checked, none", or "not computed:
   reason" (I38, I44). The change facet is always titled "git-visible changes to this repo only, not cause" and
   prints the declaration's blind spots. With no matching declaration, the unprepared view over the seed files is
   the floor and works with no pre-shaping. Contributor-controlled text (commit subjects, finding quotes, pasted
   errors) is stripped, capped, labeled untrusted and never enters the hook line. Facets use only `point` and
   `index` work, are admitted separately (I49), and the latency target is cold, not warm (IM-M5).
4. *Outward connection in v1 is by link and by typed parameter.* Links are rendered, show their host, and are never
   fetched (I56): `https` only, no userinfo, no token-named query keys, window placeholders on any host, commit and
   key placeholders only on configured link hosts. A `--deployed <sha>` (and `--previous`) parameter, filled by the
   agent or human from their own deploy tool, turns the time window into the deployed range; `since_tag` covers
   release trains. No credential, no fetch, no egress path is added by fathom.
5. *Probes (command templates) and MCP next queries are deferred (IM-D1, IM-D2; F57), not refused.* They are a
   command runner in effect while bypass-permissions mode exists, they have no meter, and they duplicate runbooks.
   Promotion needs all of: an inject-only PostToolUse(Bash) sensor that can match a run to a rendered probe (after Q32
   answers how exit codes arrive), a consumer whose runbooks lack the commands, and, for next queries, a portable
   server identity. The rendering contract a promoted probe must meet (argv only, a verb-and-flag allowlist, always
   refused shells and credential or endpoint redirection, bound-value character limits, pinned targets, `empty_means`
   only on exit 0) is recorded in the design so the analysis is not lost.
6. *Refused.* Fathom-side connectors that call ops systems (IM-X1); attach, meaning external tools push deploy, flag
   or alert events into fathom, because it is a connector with the call moved out (IM-X2, with a reopen trigger
   below); fathom executing a probe, fetching a link or calling another MCP server (IM-X3); a command center's
   accretion: drills and drill receipts as product state, a last-drill time, a deploy, flag and alert timeline, an
   incident-tool export (IM-X4); a map-family kind (IM-X5); committers as owners (IM-X6); remote or fetched
   declarations, environment- or keychain-bound values, and a static `bind` copying indexed facts (IM-X7); hook
   auto-run, a change facet presented as complete, or a missing pointer presented as no map (IM-X8); a typed model
   naming a cause, picking the match, judging runtime data, setting severity or proposing mitigation (IM-X9); and the
   unqualified claim that the incident path has no egress, because the answer reaches the model provider through
   the agent (IM-X10).
7. *Delivery is by configuration (P16), with no new tool.* `localize` gains an incident section and optional
   `deployed` and `previous` arguments; a prompt-hook line names the window's commit count (stage 1) and the matched
   declaration (stage 2), once per session, inside I35, silent on failure, with the stanza saying silence is not
   evidence; one CLI verb (`fathom incident`, name open) serves the human path and, with `--offline`, the
   daemon-down path; a doctor row summarizes readiness. The default surface stays at eight tools (ADR-053).
8. *Readiness is facts, never grades.* Anchors resolve; a non-wildcard owner exists for the footprint and for the
   declaration file; the runbook's age against the footprint's churn (the computable proxy for rot); URL-only runbooks
   and unchecked symptoms read unverifiable; symptom collisions are listed. A degrading declaration is flagged, never
   deleted, and fathom never edits the file (P15).
9. *The gate changes.* XM-M2 and XM-D1 no longer gate incident maps. Stage 0 builds nothing in the product: IM-M0, a
   two-arm replay over at least 50 fast reverts in at least 3 public repositories (seed files only versus a
   mechanical or blind footprint, equal list length, the production git flags, bars fixed before the run), and
   IM-M1, the same replay over at least 10 change-caused incidents at a named consumer (IM-V1, P20). Stage 1 (the
   bounded read, the `localize` recency hop, the untrusted-text handling, the hook line) starts only when the owner
   ends the deferral and IM-M0 reads BUILD, and IM-M2 (pointer expansion at least 10 percent) keeps or cuts the
   pointer. Stage 2 (the declaration) needs IM-M2 KEEP and a named consumer. IM-M3 (the cause was in the list), IM-M4
   (a readiness gap mattered) and IM-M5 (cold p95 under 3 s, warm under 1 s) read after that. Fathom's own INC
   records cannot feed the replay or count as the consumer.
10. *Futures.* F4 is split: incident maps become F56 and probes and next queries F57, and F4 returns to precedent over
    a non-git corpus. The non-goals in `docs/intent/scope.md` (no domain connectors, no chat or incident command
    center) are unchanged and each carries a dated note that incident maps respect it. XM-D9 (a typed model orders
    the suspect list) stays deferred with its trigger re-pointed to IM-M3 and a calibration whose Wilson 95 percent
    lower bound reaches 0.90 (R3, I60, I62).
11. *A proposed invariant, not in force until stage 2 is built:* no incident path executes a command other than
    read-only git, fetches a link, or calls another MCP server; the answer uses only point and index work and names
    every facet it could not compute; the change facet always names its horizon and blind spots.

**Alternatives rejected.**
- *Incident maps as a kind of live map (the section 3c decision).* The static footprint uses none of the map
  machinery, and membership would inherit F50's gate or escape it while claiming the family.
- *Fathom-side connectors that pull deploys, flags and alerts.* Egress, credentials and vendor liability, and the
  domain-connectors, dispatch and no-hosted-backend non-goals; there is no consumer.
- *Attach (events pushed into fathom).* Fathom would still own the schema, storage, TTL, dedup and reconciliation, and
  the optional drop-file import is polling by proxy.
- *Probes and next queries now.* A command runner in effect under bypass permissions, no meter, so it could never be
  killed; the runbook already carries the commands.
- *Store a larger commit table for the suspect list.* It collides with RT-M1 and R5; a bounded request-time read
  needs no table.
- *Frequent committers as owners.* It selects humans by commit count (I58) for little value.
- *A usage kill clock on declarations.* Rare use is the success state; readiness measures liveness instead.

**Consequences.**
- One design (`docs/design/incident-maps.md`), one plan (`docs/plans/incident-maps.md`), futures F56 and F57, the
  incident rows of the exploration-maps plan moved, dated notes on F4 and on two non-goals, a pointer in vision 40
  and in the exploration-maps design; nothing here is built by this record.
- The two shipped defects (IM-F1, IM-F2) are independent bugs and may be fixed at any time.
- Code follows only through IM rows, each gated by `fathom verify`, after the readings in decision 9.
- The tool count is unchanged (ADR-053), and no new table exists before RT-M1 reads PASS.
- Reusable, project-agnostic guidance is written in the design (section 18) for agentic-kit.

**Revisit when.**
- IM-M0 or IM-M1 reads KILL or DROP (drop F56 and mark its waves refused).
- IM-M1 shows git-invisible causes at 20 percent or more of change-caused incidents that tag windows and deployed
  SHAs do not cover (reopen attach, through a new ADR with the contract recorded in the design section 6.5).
- Q32 is answered and an inject-only sensor can match a run to a rendered probe while a consumer asks for templates
  (reconsider probes, F57).
- XM-D1 ships (add the `scope` field, IM-D4).
- The domain-connectors trigger fires (an external party ships a conforming module with zero connectors from us):
  reconsider fathom-side connectors through an ADR and an I14 amendment.
- The owner ends the deferral, or a named production consumer is recorded (IM-V1).

## ADR-059: AGENTS.md is the one instruction file; a CLAUDE.md fathom meets carries only an import (2026-09-29)

**Status.** Accepted 2026-09-29 on the owner's decision of the same date ("AGENTS.md is the single instruction file in our repos; do not maintain CLAUDE.md"). **Type.** Product / architecture. This record corrects ADR-029's Claude Code descriptor data (instruction files, skills directories, `verified_against`) with no change to ADR-029's decisions; executes ADR-049's consequence "the obligations corpus widens only to files a client actually loads"; and supersedes the obligations layer's corpus note excluding AGENTS.md. Design: `docs/design/agents-md.md`. Plan: `docs/plans/agents-md.md` (AG-).

**Context.**
- *Claude Code.* Since 2.1.277 it reads AGENTS.md natively through `agents-md@builtin`. In the default `claude-md-or-agents-md` mode, a project with no `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` in the working directory or any parent loads its AGENTS.md files where CLAUDE.md would load; `~/.claude/CLAUDE.md` does not count. InstructionsLoaded does not fire for an AGENTS.md loaded that way. Only `.claude/skills` is discovered. The setting is honored only in user and managed settings.
- *Codex.* It reads AGENTS.md from the root down and skills from `.agents/skills`.
- *fathom before this record.* It wrote one stanza into two files and rendered skills twice, and the in-repo copies had already drifted (`fathom-plan` differs between `.claude/skills` and `.agents/skills`). Its obligations layer compiled CLAUDE.md and excluded AGENTS.md, so converting a repository would have emptied its governing corpus silently.

**Decision.**
1. *One stanza, in AGENTS.md.* Every write path puts the stanza region in AGENTS.md only: `init` (every `--harness`, `none` included), `fleet reconcile --apply` and the console apply. For Gemini CLI, init also sets `.gemini/settings.json` `context.fileName` to AGENTS.md and GEMINI.md through the wiring engine. No path creates a CLAUDE.md.
2. *CLAUDE.md is at most an import.* The first present of `CLAUDE.md`, `.claude/CLAUDE.md` and `CLAUDE.local.md` gets one fathom-marked region whose body is the import line (`@AGENTS.md`, or `@../AGENTS.md` from `.claude/`). It gets none when an unfenced import already exists, and then fathom removes its own region there (`Region.Remove`). It also gets none when it is `AGENTS.md` itself (a link either way): it already carries the stanza, and an import written through the link would replace it.
3. *Migration rewrites the region, never the file.* An existing CLAUDE.md stanza region is rewritten in place to the import body, with the stanza written into AGENTS.md in the same apply. It carries a receipt, and `fleet undo` restores it. A drifted region blocks (law 15). fathom never deletes an adopter's CLAUDE.md.
4. *Doctor proves the path.* `stanza_present` FAILs when AGENTS.md lacks a well-formed region. It WARNs, with the remedy, when Claude Code will not load AGENTS.md: the root CLAUDE file has no import; a legacy stanza copy remains; an ancestor CLAUDE file exists; the effective setting is `claude-md` or `managed-only`; the Claude Code version is older than 2.1.277. fathom never writes the setting. `stanza_loaded` SKIPs with a named reason where no CLAUDE file exists.
5. *Instruction files are first-class wherever fathom reads them.* The obligations corpus includes AGENTS.md at the root, under `.claude/` and nested, beside CLAUDE.md; fathom-owned regions are not compiled; byte-identical twin sections compile once; probes, console guide predicates, fleet notes and benches name AGENTS.md first.
6. *One skills copy.* `.claude/skills` holds the files, and its frontmatter is held to the Agent Skills fields by test. `.agents/skills` is a wiring-owned relative link to it, written only for a harness that reads `.agents/skills` and not `.claude/skills` (Codex, Gemini CLI, the generic AGENTS.md tier, bare `--skills`). No writer writes through the link. A real `.agents/skills` with foreign entries blocks.
7. *Our repos carry AGENTS.md only:* fathom, agentic-kit and app-kit. Living citations are re-pointed; history is left as written.

**Alternatives rejected.**
- *Keep both stanzas.* Two copies to keep identical, and under the default mode the CLAUDE.md copy also suppresses AGENTS.md. The owner refused this.
- *Create an import-only CLAUDE.md in every adopter.* It restores the InstructionsLoaded evidence, but it re-adds the retired file and switches off native AGENTS.md loading.
- *Write the agents-md setting.* User-global configuration (ask-first), and project settings ignore it.
- *Keep rendering two skill copies.* They drift, and the `fathom-plan` copies here already differ.
- *Delete a CLAUDE.md that carries only fathom's import.* fathom did not create it (law 15).
- *Follow `@` imports in the obligations compile.* No measured need; not built.

**Consequences.**
- Breaking: what init and reconcile write (compatibility surface: the stanza region); `gen-claude-md` becomes `gen-agents-md` (the old name runs with a deprecation line through 0.9.x). Migration note: `docs/upgrades/0.9.0-agents-md-one-instruction-file.md`.
- Changed: the harness descriptors and the `schema/harness` mode `import` (`mirror` retired); the skills layout; the obligations corpus; doctor `stanza_present`; the claude-code pack's check kind (`agents_md_reaches_claude`).
- Restated: invariant I37 (and I34 when this repository converts); the router's sensitive-surface row; the obligations-corpus assumption in `domain.md`; the router footer (Claude Code 2.1.284); the compatibility surface in `scope.md`.
- Evidence and history: `stanza_loaded` loses harness evidence in AGENTS.md-only repos; a converted bench subject is a new corpus revision, and historical cells are unchanged.
- Kits: agentic-kit's and app-kit's own products change in their own records, and fathom mirrors them by `task sync-kit`.
- Plan row IA-3 retires as refused under this record.

**Revisit when.**
- Claude Code changes the agents-md contract: its default mode, discovery of `.agents/skills`, or InstructionsLoaded for AGENTS.md.
- Codex reads `.claude/skills`, or is shown not to follow the link (AG-OP2).
- A supported harness reads CLAUDE.md but not AGENTS.md.
- An adopter asks in writing for a CLAUDE.md-only layout.
- `stanza_loaded` SKIPs on every row of our fleet for 30 days: retire or replace the row.

**Amendment 2026-10-02 (cloud sessions).** On the owner-architect cloud-session decisions C1–C4 of 2026-10-02
(`docs/adoption/cloud-sessions.md`, plan `docs/plans/cloud-sessions.md`), decision 7 changes for this repository
only: fathom carries a root `CLAUDE.md` whose whole content is `@AGENTS.md`, and one such file beside each nested
AGENTS.md (`internal/`, `docs/`, `cmd/fathom/`, `schema/`). Why: Claude Code cloud sessions are documented to load a
repository's CLAUDE.md, and some sessions read CLAUDE files only (a version before 2.1.277, a first session after
an upgrade, the agents-md plugin disabled); the import reaches all of them. The cost, and why the nested files: a
root CLAUDE.md puts Claude Code's default `claude-md-or-agents-md` mode on CLAUDE files for the whole tree, so a
nested AGENTS.md then loads only through a CLAUDE.md beside it. The concern `one_instruction_file` moves from
`allow_import: false` to the new `import_only: true`: each CLAUDE.md is exactly the import of the AGENTS.md beside
it, every nested AGENTS.md has one once the root exists, and `.claude/CLAUDE.md` and `CLAUDE.local.md` are refused;
I34 is restated. The rest stands: the stanza lives only in AGENTS.md, no fathom write path creates a CLAUDE.md,
adopters are unchanged (the rejected alternative still applies to them), and nothing is maintained in a CLAUDE file.
**Revisit when** (added): a real cloud session shows AGENTS.md loading natively there and the owner prefers the
AGENTS.md-only layout back; or Claude Code changes how a nested AGENTS.md loads under a root CLAUDE.md.

**Note 2026-10-09 (plugin id; K12).** Claude Code 2.1.285 renamed the agents-md plugin from `agents-md@builtin`
(2.1.277 through 2.1.284) to `cc-plugin-agents-md@builtin`. Doctor reads `pluginConfigs[…].options.instructionFiles`
under both ids, the new one first, and names a settings file whose two ids disagree; the descriptor note and the
docs name both with the version boundary (AG-27). The first "Revisit when" bullet (the agents-md contract) fired on
the id alone; the default mode, `.agents/skills` discovery and InstructionsLoaded are unchanged, so the decision
stands. On the owner's K12 decision of the same date, doctor's machine-side remedies name only the native path and
never prescribe creating a root CLAUDE.md or adding an import (AG-28); the import region for a CLAUDE file that
already exists stays, and so do this repository's five `@AGENTS.md` files until one cloud session proves native
loading (the 2026-10-02 trigger, owner half fired, cloud half pending).

## ADR-060: Stewardship facts: lines, leftovers, inheritance and promises are joins fathom reports and never acts on (2026-09-30)

**Status.** Accepted 2026-09-30 for decisions 1 to 8 (the owner delegated the calls to the owner-architect); amended
2026-09-30: decision 7's ordering accepted, and decision 6's licence boundary settled as default deny (the amendment
below); amended 2026-10-01: SV-M1 read CUT, so `plans lines` loses the claims-rows and touches-rows classes (the
second amendment below). The owner's wind-down of 2026-09-30 still holds every new wave until the owner lifts it; decision 7 orders the
work after that. **Type.** Product / architecture. Inputs: the owner's six notes of 2026-09-30 (line
lifecycle, upstream drift, temporary artifacts, SRE as code, minimal vendoring, prioritization), the 2026-09-29
consolidation backup manifest (local, `workspace/`), the 2026-09-29 feature scorecard, a census of this
workstation's repositories on 2026-09-30, and fathom's own `third_party/` mirrors. Vision:
`docs/vision/41-stewardship.md`. Plan: `docs/plans/stewardship.md` (SV-). Goal G34. Invariant I63. Futures F58 and
F59. Reads ADR-049 decision 1, ADR-050 decision 4, ADR-053, ADR-055 decisions 7 and 8, and ADR-058 as constraints.

**Context.** Around the code at HEAD lie work not landed (about 230 lines in the 2026-09-29 harvest, 2 of them
valuable; 46 worktrees and 53 branches removed by hand), files nothing owns, code we did not write (fathom's binary
imports 22 of app-kit's 93 non-test Go packages and 19 of agentic-kit's 242 while `third_party/` mirrors both whole,
1,806 and 1,602 files), and what a service promises (no alert-rule, SLO-spec or dashboard file on this workstation on
2026-09-30). Tools exist for each part (git, Renovate and Dependabot, `go mod vendor`, Prometheus and Grafana, Review
Kit); none knows what the code is for. `dep_drift`, which the owner's note assumed, was deleted under ADR-053
(scorecard F-18); `task vendor-check` (I15) proves a mirror equals its pin and says nothing about upstream.

**Decision.**
1. *The family rule.* A stewardship fact is a join of git or file state with a fact fathom already computes (plan
   rows, reach from product entry points, contract keys, conservation planes); never a new engine (ADR-051 decision
   1). Fathom reports and never acts: it never deletes, merges, squashes, rebases or checks out a branch, worktree,
   stash or clone it did not create, and names the command instead of running it (invariant I63, P15). Each fact is
   a CLI report run on the operator's ask; the only ambient channel is one count on the SessionStart attention line,
   after a reading earns it. No stewardship work runs in the Stop hook or around compaction.
2. *Lines: accepted.* `fathom plans lines` classifies one repository's branches, linked worktrees and stashes as
   landed, claims rows, touches rows, at risk or idle (SV-1, SV-2); a $0 replay over the 2026-09-29 bundles decides
   whether the row classes stay (SV-M1); the attention count waits on it (SV-3). Squash and re-summarize by default is
   refused as fathom behaviour. Would-prove by a scratch verify per line and fleet-wide or scratch-clone lines are
   deferred (SV-D2, SV-D3).
3. *Upstream drift: merged into the system graph.* "Did upstream change what we use" is ADR-050's upgrade impact
   (SG-39) restricted to the used closure, with a vendored mirror's "to" version read from its sibling checkout on
   disk. "Significant" is deterministic: a removed or re-signatured symbol the product reaches, a changed file inside
   the used closure, or a licence-file change. Polling and version discovery are refused (P23, I56; Renovate and
   Dependabot own them). First slice: the pin-lag reading SV-M4; the attention line is deferred (F58, SV-D1).
4. *Temporary artifacts: merged.* Report-only `gc` classes for strays outside declared homes and binaries in the
   tree (SV-4), `archivable` in `plans status` (SV-5), the owner's adjudication SV-M2. Stale documents stay with the
   knowledge-retrieval doc drift; uncited documents are deferred (SV-D4). Stop-hook and compaction nudges and a
   `Lifetime:` header are refused.
5. *SRE as code: promises as code; the fathom half deferred, the standards in Review Kit.* Fathom's part is the
   code-grounded chain (feature, route, the SLI metric emitted on the handler's reach, the SLO, alert and dashboard
   that consume it, the runbook declaration): a `metric:` contract-key kind, dangling promises through
   `contract_drift`, observability conservation at review, and readiness facts delivered to Review Kit through the
   evidence contract. Readiness levels, error-budget policy and the reliability Standard are Review Kit's (its
   "Reliability and operations" domain); live values, burn rates and alert state stay in the ops system (a rendered
   link at most, ADR-058); an organization-wide dashboard is hosted (DATUM, a fathom non-goal). A separate
   Reliability Kit is refused (P21). The build is deferred (F59, SV-D5): no consumer on 2026-09-30 (P20). Usable
   today: a Review Kit concern pack compiling a reliability convention into obligations at the edit.
6. *Minimal vendoring: accepted as the used closure and a licence boundary plane.* `UsedUnits` and `fathom report
   used` compute a dependency's closure from product entry points with a reason per unit: Go packages equal to `go
   list -deps` (file pruning inside a Go package is unsafe and refused), TypeScript and JavaScript modules by import
   edges, declared asset roots (SV-6, SV-7), read on fathom's own mirrors (SV-M3). The boundary plane is default
   deny: licensed sources (a catalog such as Tailwind Plus or Catalyst) never ship in a public repository, so it
   blocks at pre-push a tree holding a path the owner's licensed-paths declaration names, unless the owner has
   recorded an explicit allow for that path, the only exception. The declaration is a CUE file under the
   repository's fathom directory, committed by a repository that ships publicly (SV-8, after a named consumer,
   SV-V1); a repository without one is not checked, and fathom never infers visibility. This is app-kit's own
   delivery rule ("Licensed derivative: keep this directory and generated outputs private",
   `internal/designadapt/*_delivery.go`). Reached licensed units are listed as a fact, so an allow can be as narrow
   as the closure. Fathom never copies, prunes or vendors files (the copy stays `scripts/sync-vendor.sh`), never
   fetches, and never infers or judges what is licensed.
7. *Prioritization.* Accepted 2026-09-30 by the owner-architect. Ditch-then-merge has run: all 10 DITCH and all 12
   MERGE verdicts are on `main`. After the wind-down: finish the efficiency wave (6 open rows; EW-M1 is a 14-day
   wait), then language excellence most common first as the main line (G15), with the stewardship first slices as $0
   fillers, lines first.
8. *The decision port has no role* in any stewardship fact; a model-drafted landing summary is generation, the
   port's weak side, and licence questions are never judged.

**Alternatives rejected.**
- *A lines tracker that nudges and lands work.* A dispatch loop (non-goal) and writes fathom does not own.
- *Upstream watching inside fathom.* Polling and egress (P23, I56) to rebuild what Renovate and Dependabot do.
- *Fathom as a vendoring tool.* A package manager and the transformation-engine non-goal; `go mod vendor` is already
  exact for Go packages.
- *Content-similarity detection of derived licensed work.* A legal judgment with a costly false "fine to ship".
- *A Reliability Kit, or SRE features in fathom now.* No consumer, and the Standard belongs to Review Kit.
- *Reordering work by ditch-then-merge.* Already done; the rule's remaining force is "finish before building".
- *A per-path policy (`never`, `reached`) that lets a licensed file the product reaches ship.* Replaced on 2026-09-30
  by default deny and an explicit owner allow per path (the amendment below).

**Consequences.**
- New when built: `internal/worklines`, `internal/usedset`, `fathom plans lines`, `fathom report used`, `gc`
  report-only classes, a `boundary` conservation plane, a licensed-paths CUE declaration (SV-8); no MCP tool, no table, no cache, no
  background job.
- `docs/intent/`: goal G34, invariant I63 (stated until SV-2 and SV-4 land), futures F58 and F59, the decisions row.
- The compatibility surface grows by `plans lines --json`, `report used --json` and the licensed declaration when
  their rows land.

**Revisit when.**
- SV-M1, SV-M2 or SV-M3 reads CUT: remove what the reading killed (vision 41 section 7). (Fired 2026-10-01 for
  SV-M1: the rows classes are removed, the amendment of 2026-10-01 below; SV-M3 read KEEP, and SV-M2 has not read.)
- SV-M4 shows no reached upstream change in two quarters: drop F58.
- A named consumer commits alert rules, SLO specs or dashboards: promote F59.
- Review Kit asks for readiness facts through the evidence contract before F59's trigger fires.
- A line holding valuable work is lost although the report classed it landed or idle.
- The owner reverses a refusal or the default-deny boundary. (The trigger "The owner decides decision 7's sequence"
  fired 2026-09-30: accepted, and decision 7 stands.)

**Amendment (2026-09-30): decision 7 accepted; the licence boundary is default deny.** The owner-architect accepted
decision 7's ordering, so decisions 1 to 8 are all accepted. The same pass settled the reading decision 6 left open:
the boundary plane's per-path policy (`never`, `reached`) is replaced by one rule. Licensed sources (Tailwind Plus and
Catalyst are the cases) never ship in a public repository, and the plane blocks a pushed tree holding a declared
licensed path unless the owner has recorded an explicit allow for that path; that allow is the only exception, and
fathom never grants one. The rule matches app-kit's delivery rule for licensed derivatives ("Licensed derivative: keep
this directory and generated outputs private", `internal/designadapt/*_delivery.go`). Reached licensed units are still
listed as a fact, so an allow can be as narrow as the closure. Vision 41 section 4.5, the SV-8 and SV-V1 rows and the
decisions index follow; ids and contracts are unchanged.

**Amendment (2026-10-01): SV-M1 read CUT; the rows classes are removed.** The $0 replay over the 2026-09-29
consolidation (`docs/bench/readings/stewardship/lines-replay.md`) flagged 36 lines against the pre-registered bar of
25 (O293): 18 kit lines read touches rows because their diffs named a generic symbol under an open row's package,
and the landed test cannot see work that landed and was then edited on `main`. The owner-architect decided on
2026-10-01 to remove the killed classes, claims rows and touches rows, from `fathom plans lines`: the
`internal/worklines` classes and their matchers, the `--json` fields `claims`, `touches`, `tier` and `open_rows`, the
table's rows column and open-rows count, and the docs. Landed, at risk, idle and active stay, in that precedence.
Nothing then needs the plan files, so worklines reads none. The decision retires the test
TestClassifyLinesLandedClaimsTouchesAtRiskIdle, renamed TestClassifyLinesLandedAtRiskIdle without its claims and
touches fixtures; TestClassifyLinesClassesOnlyLandedAtRiskIdleActive pins the four classes and the absent plan read
(efficiency-wave EW-40). SV-1 and SV-2 keep their ids and symbols with the rows wording removed, and SV-1 is reached
from the renamed test; SV-3 was refused with the reading, SV-D2 (would prove instead of claims or touches) is refused
because its trigger needed SV-M1 KEEP, and SV-D3's trigger no longer waits on SV-3. A per-commit "already applied"
landed test (`git apply --reverse --check` of each commit with no equivalent on the integration branch) is recorded
as future F60 (EW-41); no rows class returns before its own pre-registered reading reads KEEP. The
`plans lines --json` shape was added after 0.9.0 and has not been released, so the change carries no Breaking line;
the compatibility surface (`docs/intent/scope.md`) and vision 41 sections 4.1 and 7 follow.

## ADR-061: The `.fathom/` directory in four zones with one keyed schema, and tech packs: one format, three authors, bound by the repository's own pin (2026-10-09)

**Status.** Proposed 2026-10-09. Owner decisions D11 (tech packs), K5 (projects describe themselves in `.fathom/`; committed index snapshots rejected), K9 (an authored, anchored orientation set in the project pack) and K10 (one noun, one format: the custodian's packs and app-kit's fathom-pack become tech packs) of 2026-10-09, and the owner's ask of the same day: "are some of the patterns or features that we already have better reworked into a .fathom dir paradigm … as we add new capabilities we don't sprawl the config and surface area further than it needs." Design: `docs/design/fathom-dir.md` (normative once accepted). Plan: `docs/plans/fathom-dir.md` (FD-…). Goal G35. Promotes F5 (TX-3); fires ADR-024 *Revisit when* (1); reframes F27; adds F62 to F65.

**Context.** An inventory of every file fathom reads or writes (design §2) found: a kit's guidance adopted by copy drifted inside this repository (three guards marked "app-kit fathom-pack v0.1" in `.fathom/guard.cue` while the pinned `third_party/app-kit/fathom-pack` is v0.2, and `.fathom/layers/ui-appkit` differing from the pinned layer in all three files); per-checkout state in three homes, with `owned.json` and `leases.jsonl` left in the committed `.fathom/` and no ignore written for adopters; the literal `.fathom` spelled by hand in 96 non-test lines across 65 files, one of which (`cmd/fathom/configpanel.go:69`) reads a binding file name nothing writes; two architecture-rule formats (`layer_rules`, `.fathom/rules/archconformance.cue`); two switchboards (`capabilities.cue` and four `config.toml` booleans); tuning tables (`risk.cue`, `learning.cue`) outside `config.toml`'s provenance; and growth by file, each capability adding a file name, schema, loader, doctor row and docs section, with five more designed. Meanwhile the parked provider stratum's trigger fired: fathom is a named consumer and app-kit and env-kit are concrete providers.

**Decision.**
1. *Four zones.* Z1 authored config (`config.toml`, `capabilities.cue`, `modules.cue`), Z2 authored knowledge, which is the project's own pack (`guard.cue`, the new `generated` and `formats` keys, `concerns.cue`, `layers/`, `trace/`, `observations/`), Z3 local and derived state (never committed: the state directory, and `.fathom/.gitignore` written by `fathom init` through `internal/wiring` for the legacy local names), Z4 user scope (unchanged). No derived index data is ever committed (K5).
2. *One schema entry point, extended by key.* `schema/project` `#Project` composes the existing schemas by top-level key; one fault-isolated loader (`internal/projectdir`: per-file parse, per-key validation, a failing key named without dropping the others; an unknown key a doctor warning) and one resolver for every project-scope name. A new capability adds a key, never a loader, schema directory, doctor row or file name fathom must know; a new switch adds a capability id, never a `config.toml` boolean.
3. *Merges* (design §6): kit guidance read from the pin, not pasted (H1); `.fathom/rules/archconformance.cue` into `layer_rules` kind `call_direction`, `risk.cue` and `learning.cue` into `config.toml` `[risk]` and `[learning]`, `owned.json`, `leases.jsonl` and `scip/` into the state directory, the four `config.toml` switches surfaced as capability ids (all H2, each old path read with one doctor deprecation line, nothing deleted by fathom); this repository's tenancy allow list leaves `.fathom/` (H1); the custodian's shipped packs (`claude-code`, `codex`, `lefthook`, `taskfile`) become fathom's built-in tech packs, `packs:` in `concerns.cue` still enabling them, `config audit` byte-identical (H1, K10).
4. *Tech packs v1.* The provider stratum is renamed `tech` and promoted. A tech pack is a directory of CUE data validated by `#Pack`: the `pack: {name, version, about}` header the custodian's packs already carry, `guards` (the existing `#Guard` shape: D11's rules section, already shipped by kits as `guard-pack.cue`), `generated` (output pattern, source patterns, a regenerate command rendered and never run), `formats` (globs mapped to a dialect id fathom ships or the user trusts), `concerns` (custodian contracts about the technology's own config, applied only when the repository names the pack in `packs:`, because they can fail `config audit`) and, in the project's pack only, `orientation` (K9: authored entries, an anchor plus why, plain text readable without fathom, each anchor verified by `config audit`; a map is promoted into it by ADR-056's reuse rule, never generated into it), with fixtures. The project's `.fathom/` is the same format (`#Project` is `#Pack` plus the config keys), so one format has three authors: the project, a kit (`fathom-pack/` in its pinned copy) and an org (a platform module, or a tech pack for an internal tool shipped as a pinned module). A kit pack may not carry `layer_rules`, `capabilities`, `bindings` or `exclude_packs`. Fathom's built-in packs are a fourth, embedded source.
5. *Adoption is the binding.* A kit pack applies when the repository's committed Go or CUE pin puts it on disk (a local `replace`, `vendor/`, or the module cache at the required version; never `go.work`, never the network) and it carries a `pack` header. The `tech-packs` capability turns discovery off; `exclude_packs` in `modules.cue` drops one pack. A binding is a committed, reviewable act, a `modules.cue` entry or a pin, so the phase-O doctrine holds.
6. *Gates.* Data only (no command, script or fetch); read from the pinned copy, so freshness is the pin and no upstream watermark is needed; a section is served only when its fixtures pass fathom's harness, tagged kit-supplied, with no level above that reading; at most 2 lines per pack inside I35, below the repository's rules and bound modules, displaced lines counted per source; advisory only (never verified, never verify-gating); a repository with no discovered pack is byte-identical; the kill criterion reads the existing rule-outcome log at zero spend (FD-M1: the share of generated-line fires followed in-session by an edit to the declared source).
7. *Surfaces.* No MCP tool and no new verb: `fathom rules fixtures <dir>` (a kit runs it in CI), `fathom layers strata` lists the tech stratum, one doctor row; `#Layer.stratum` gains `"tech"` (additive; `provider` never shipped as a value).

**Proposed invariants (numbered on acceptance).** (a) Fathom never executes a command or script a tech pack names, never fetches one, and never loads one no committed pin puts on disk; a repository with no discovered pack serves byte-identical edit-hook, obligations and ingest output. (b) A kit's pack never governs: it is capped per pack inside I35, ranked below the repository's rules and bound modules, never verify-gating, and a section without passing fixtures is not served and is disclosed.

**Alternatives rejected.** Committing derived index snapshots (K5: churn, staleness, bloat; a 72 s cold ingest the cloud snapshot already caches). One CUE file for everything under `.fathom/` (merge conflicts, one parse failure takes everything down, every adopter migrates). Moving `.fathomignore` into `.fathom/` or user-scope state under `~/.fathom/state/` (convention and compatibility cost, no reader gain). An explicit kit list in `modules.cue` (duplicates the pin and drifts from it; D11). Copy-and-adapt adoption (the measured drift). A pack registry, fetch verb or host (I56, F2). A `fathom pack` verb or MCP tool (laws 4 and 10). Kit packs carrying governance keys (law 13's shape). A `fathom migrate` verb (no compatibility item is removed while legacy reads stay).

**Consequences.**
- New: the `orientation` key and its anchor check, `internal/projectdir`, `schema/project`, `schema/techpack`, `internal/techpack` (discovery, fixtures harness, generated lookup), a `pack.cue` in this repository's `.fathom/`, the `tech-packs` capability, `exclude_packs`, the `next_edit_source` and `next_edit_output` rule outcomes, the `tech` stratum value.
- Changed: the guard and obligations path serves pack lines; `GeneratedHandEdits` reads declarations first; classification honors `formats` for known dialects; `LoadLayerIn`'s union reads pack layers (embedded, then tech, then project); `guardspec`, `bindspec`, `capability` and `custodian` read through the one loader, byte-identical.
- The compatibility surface grows by the `#Pack` and `#Project` keys, the `.fathom/.gitignore` region and `rules fixtures`' output (H1), and the `call_direction` kind and the `[risk]` and `[learning]` sections (H2). No breaking change: every existing file keeps its name, location and meaning.
- ADR-024's provider row becomes the tech row; D6 (kit dialect packs) is superseded: a kit's dialect need becomes its pack's `formats`, and the dialect itself stays fathom-authored or user-trusted (ADR-048, I55).
- ADR-055, ADR-058 and ADR-060's designed files (`features.cue`, `incidents.cue`, `licensed.cue`) land as `#Project` keys.

**Revisit when.**
- FD-M1 reads KILL: the generated-line serving is removed; the declarations stay as data for integrity.
- A kit ships its pack through an ecosystem other than Go or CUE modules (a third discovery source needs its own offline resolution rule).
- A repository needs to narrow a kit pack below excluding it (per-pack scopes or caps in `modules.cue`): add the field by evidence.
- Two files in one repository fight over a `#Project` key in practice, or the one-loader parse measurably slows a hook past I53's deadline.
- A release before 1.0 proposes dropping a legacy read (`.fathom/rules/archconformance.cue`, `risk.cue`, `learning.cue`, the in-tree state names): that is a breaking change with its own migration note, and a mechanical move verb is weighed then.
- F62, F63 or F64's promotion trigger fires.

## ADR-062: Telemetry links: a pasted frame, error payload or telemetry identity reaches the code that threw; Stage A is dialect work, Stage B waits for the census (2026-10-09)

**Status.** Accepted 2026-10-09 for Stage A, on the owner's wave C1 direction (K14: an adopter's observability and CI
stack as navigable code evidence; K15: identifying labels are data); Stage B is gated (decision 3). Records owner
decision D4 of 2026-10-02 ("build a CI/build format when a user needs it"; the excellence directive's decision list,
not language-breadth's D4 of 2026-09-28 on cross-file references), fired 2026-10-09 by an adopter's stack (GitHub
Actions run steps, Composer), and accepts tier T7. Design: `docs/design/telemetry-links.md`. Plan:
`docs/plans/telemetry-links.md` (TL-). Futures F66 and F67. Observation O615. Reads ADR-050 (the link model, its
interim divergence), ADR-058 (incident maps) and F59 (promises as code) as constraints.

**Context.** An adopter's services report failures through OpenTelemetry, Bugsnag, New Relic and AWS Lambda, alert
through Prometheus, and deploy through Compose, Serverless and CloudFormation. An agent handed one of those artifacts
had two gaps. `localize` read Go, generic `path:line` and Python frames only: V8's `at fn (path:l:c)` without its
function, PHP's `#N path(line): fn` not at all, a JVM frame by basename (the first sorted `Util.java` of two), and a
JSON payload as escaped text (a Bugsnag frame object carries no `path:line`). The TL-7 replay measured it: top-1 0.259
over 440 answers before this record, 0.984 after (`docs/bench/readings/telemetry/tl1-replay.md`). And telemetry names
a service by its identity (an OpenTelemetry `service.name`, a New Relic app name, a Lambda function name, a
Prometheus `job`), which is rarely the repository's service id. The richer idea, links from alerts, SLOs and
dashboards to the code that keeps them, is F59, whose promotion needs a named consumer's committed rules and SG-2;
neither holds.

**Decision.**
1. *Stage A is G15 dialect work: identity joins only.* An observability-as-code format is a dialect measured like any
   other (ADR-048), at tier T7, "observability as code" (accepted here; item prom lands `#Tier` T7 in
   `schema/langqueue`). A label matcher (a Prometheus selector, a mesh rule) maps to the existing ADR-050 `deploy:`
   keys; no new key kind is added and F59 is not promoted.
2. *F59 has not fired* and stays unamended until the census reading (TL-6) lands.
3. *Stage B (name joins: metric and span names to their emitters) is gated* on the census reading E (the share of
   emitters a static pass can name) at 25 percent or more AND an F59 amendment the owner records. If Stage B is built,
   its storage follows `internal/fusion/routeref.go`'s interim ADR-050 divergence: a file-tier typed edge under the
   typed_edges key `telemetry-ref`, tier pre-scan, a `literal` link that never gates (I57); a cross-repository link
   is a query-time join until SG-2 lands. F66 holds Stage B.
4. *Stage A's interim storage* is the route-ref typed edges (`routeref.go`, lines 14 to 24): a label join is an edge
   on the `deploy:` key it matched, written into no new table, cache or background job.
5. *Delivery.* `localize` stays on `/rpc/extended` (law 4; ADR-053's default eight are unchanged). The incident path
   therefore serves hook-routed sessions and extended sessions only: the prompt hook routes an issue-shaped prompt to
   `localize` whether or not the session's surface serves it (O615, open).
6. *A payload is parsed in the call and discarded.* This is the "pasted trace or alert" delivery of
   `docs/design/incident-maps.md` section 5.1 (line 123), so ADR-058 is cited, not amended: nothing is fetched,
   stored or retained. Frame tuples only (ADR-058 decision 3): a frame's path, line and function are read; message,
   breadcrumb, user and metaData text never seeds the walk, never appears in the answer and never enters the hook
   line.
7. *The payload shapes are data* (`internal/stackframe/shapes.json`, universal defaults): Bugsnag
   `events[].exceptions[].stacktrace[]` with `inProject`; OTLP JSON `exception.stacktrace` and the `code.*`
   attributes in both the older (`code.filepath`, `code.lineno`, `code.function`) and the current
   (`code.file.path`, `code.line.number`, `code.function.name`) names; the Lambda runtimes' `errorType` with
   `stackTrace`, `trace` or `stack`; New Relic `error.stack_trace`. Chained errors are read root cause first.
8. *The TL-5 identity contract.* A doc entity of source kind `TelemetryIdentity` (canonical `key`), named by the
   literal value and signed `<scheme>=<value>`, scheme one of `service.name`, `newrelic.app_name`,
   `aws.lambda.function_name`, `prometheus.job`; a `${`, `{{`, `$(`, `!Ref` or `!Sub` value is never guessed.
   Several dialects declare the one kind (the loader's `contractSourceKinds`). `localize` resolves additively: a
   `service` that names no indexed service but exactly one service's TelemetryIdentity answers for that service and
   says so; two or more are refused with the candidates. No new tool, no schema or surface change.
9. *K15: the identifying label set is data* with universal defaults (the shape registry, the dialects' selectors);
   a project override in `.fathom/` waits for ADR-061's zones.
10. *Not built* (no consumer, or no format worth a dialect yet): LogScale CQL, LogsQL, NRQL, CloudWatch EMF,
    VictoriaMetrics (MetricsQL, VMRule), the Istio Telemetry CRD, mirror and delegate rules, and Bugsnag as an
    extractor (its configuration identifies nothing the payload does not, K15).

**Alternatives rejected.**
- *A telemetry connector that fetches alerts or errors.* Egress and credentials (I56, ADR-058 decision 6); the paste is
  the integration.
- *A new `metric:` or `telemetry:` contract key kind now.* F59's preserve clause keeps it one additive kind away; the
  census has not shown the emitters can be named.
- *Promoting `localize` to the default surface for this path.* Law 4 and ADR-053's call evidence; the replay measures
  the answer, not the reach.
- *Identities guessed through interpolation.* An answer that names the wrong service is worse than none (I38).

**Consequences.**
- New: `internal/stackframe` (the one parser `localize` and the prompt hook share), the payload-shape registry, the
  `TelemetryIdentity` entities in the docker-compose, serverless and cloudformation dialects with their ceilings, the
  TL-7 replay harness (`indexbench localize-replay`) and its fixture apps under `calibration/incidents`.
- Changed: `localize` weights the innermost resolved frame as the throw site and resolves a telemetry identity; the
  prompt hook counts V8 and PHP frames and frame-carrying payloads as issue-shaped.
- Futures: F66 (Stage B name joins) and F67 (the log-template index, wave C2).

**Jev, weighed.** The decision port could order candidates when frames are ambiguous, or pick a service among several
identities; neither is built. Ordering is deterministic and measured (TL-7 at 0.984 leaves seven misses, none a ranking
choice), and an ambiguous identity is refused, never chosen (I62: a judgment never asserts a code fact). With provider
none, the default, everything here runs unchanged. The measurement that would admit Jev: a replay over real consumer
payloads where top-1 falls below 0.90 on ranking, not extraction.

**Revisit when.**
- The census reading (TL-6) lands: Stage B is weighed against decision 3's gate.
- F59 promotes or is amended.
- ADR-061's zones ship: the identifying label set gains its `.fathom/` override.

## ADR-063: Kit overlaps: each kit edge is delegate, merge, or keep with a stated boundary; fathom builds no second package graph, verifier or result format (2026-10-09)

**Status.** Accepted 2026-10-09 (owner decision K7, "wire in all ways", of 2026-10-09). Reads ADR-050 (the system
graph's dependency port), ADR-040 (the findings loop), ADR-043 (certifications), ADR-055 (review stays a family),
ADR-059 (one instruction file) and ADR-061 (tech packs, proposed) as constraints; amends none of them.

**Context.** K7 asked for one matrix of the owner's kits against fathom with three edges per cell (adopts: the
tooling fathom's own repository uses; feeds fathom: formats, provenance and evidence a tech pack could carry;
consumes fathom: readings, test selection, plan gates) and an overlap check before any new wiring. The reading
(2026-10-09) found three edges K7 treated as future already live: fathom vendors app-kit and agentic-kit
in-tree (`third_party/`, pinned by `.vendor-commit`, `go.mod` replace), mirrors the test-kit schema
(`schema/testkit/`, `task tests:attest`, which skips without a surveyor binary), and Review Kit consumes fathom
through its bridge (`fathom review`, `review-key`, `certify gaps`, `conserve --since`). Nothing consumes env-kit,
image-kit, docs-kit or ci-kit from fathom today. Ten overlaps were named; one is a true duplicate.

**Decision.** One verdict per overlap, each with the boundary that keeps it from coming back:
1. *O1, the dependency graph: delegate.* ci-kit's `depgraph` (its ADR-0071) is the package and module closure;
   fathom's graph is symbols and call edges. Fathom consumes `depgraph` through ADR-050 decision 5's port, or
   ci-kit's own drop-if fires; fathom never builds a package-level resolver.
2. *O2, review: keep.* `review_diff` and `fathom review` assert diff-scoped facts from the index; leadsman judges
   readiness and sufficiency over many lanes, fathom's among them. Review Kit's `internal/fathombridge` Collect is
   the only path between them, and Review Kit never re-derives a blast radius (ADR-055 decision 3).
3. *O3, plan gates: delegate.* A kit's plan rows are verified by `fathom verify`; no kit builds a second
   checkbox verifier.
4. *O4, test facts: keep.* The test-kit result envelope (and its attestation) is the source; fathom ingests it
   (`fathom tests ingest`, ADR-043) and joins it against the index. Fathom adds no parallel result format;
   coverage profiles stay fathom's, since test-kit carries none.
5. *O5, documentation coverage: delegate.* docs-kit's reference coverage reads the fathom docs and symbol index
   (its DK-64); docs-kit renders and lints, fathom retrieves and finds contradictions.
6. *O6, generated configuration: keep with a boundary.* env-kit generates (Taskfile, lefthook, devenv files from
   `env.cue`); the custodian audits claims about them. Generated files carry provenance (source and regen
   command), so a custodian rule fires once, on the source, never on every generated copy.
7. *O7, doctors: keep.* `fathom doctor` and the kits' doctors (`bosun`, `surveyor`, `purser`, `leadsman`) share
   the exit lineage (0 pass, 1 finding, 2 operational); no merge.
8. *O8, guards: supersede by tech packs.* Guard entries copied by hand from a kit's fathom-pack (with provenance
   comments) are replaced by the pack read in place (ADR-061, futures F62 to F64) once tech packs ship.
9. *O9, evidence vocabulary: keep, plus a naming audit.* agentic-kit's `foundations/evidence` and `review` and
   fathom's evidence plane share words (evidence, claim, pack) with separate types; fathom imports slices only.
   The names are audited under K10, not merged.
10. *O10, test-kit availability: a gap, not an overlap.* The optional `tests:attest` skip closes when surveyor
    ships as a release artifact (K1).
11. *O11, resource policy: delegate (added 2026-10-10, ADR-067).* policy-kit (platform-engineering ADR-0015) owns
    resource-shaped rules: the control, its CEL assert, its admission forms, its crosswalk and coverage. Review Kit
    refuses a resource-shaped requirement unless it joins policy-kit by canonId (ADR-0095 decision 5) and builds no
    infrastructure scanner (its F5). Fathom keeps resource facts and membership (`internal/fusion/policyref.go`:
    which Kyverno or Gatekeeper policy selects a workload) and never evaluates a resource rule, in core or in a
    tech pack. Fathom builds no resource-policy evaluator. Until policy-kit ships, membership reads the upstream
    Kyverno policy library a repository applies, its licence checked at use.

*Fathom's edges, summarized.* Adopts: app-kit (UI, `appkit.adoption.json`, `docs/app-kit.md`) and agentic-kit
(foundations, the project-intent practice and skills), both vendored and pinned; the test-kit schema mirror;
env-kit next (K4), replacing `scripts/cloud-setup.sh`'s hand-written setup. Feeds fathom: kit fathom-packs today
(guards and layers, by copy), tech packs after ADR-061 (formats, generated-file provenance, rules), test-kit
envelopes, Review Kit findings (`fathom findings import`). Consumes fathom: Review Kit (bridge), kit plan gates
(verify), docs-kit coverage (DK-64), ci-kit selection by blast radius (not specified; would follow O1's port).

**Alternatives rejected.**
- *Merging Review Kit into fathom:* ADR-055 decision 3 stands; the family shares a contract, not a binary.
- *A fathom-native package resolver beside ci-kit's:* two graphs of the same inputs diverge; ADR-050 already
  inverted that ownership.
- *A fathom test-result format:* two ingest paths for one fact.

**Consequences.**
- No code changes here; the verdicts constrain future rows. A row that builds a second package graph, plan
  verifier or result format cites this record or changes it first.
- AGENTS.md routes the kits (the Kits row), and this record is that row's decision pointer.

**Jev, weighed.** None of the overlaps is a judgment question: each verdict names who owns a fact, and a fact
is computed or read, never judged (I62). With provider none, the default, everything here holds unchanged. The
measurement that would admit Jev: a kit edge whose answer is a ranking over fathom's facts (test selection by
blast radius ranked by likely failure), measured against computed ranking before any model reads it.

**Revisit when.**
- A kit ships the duplicate path: ci-kit `depgraph` gains symbol edges, or a kit builds a plan verifier or a
  result format of its own.
- A named consumer needs a package graph from fathom that `depgraph` cannot answer.
- ADR-061's tech packs ship (O8 executes) or K10's naming audit lands (O9).

## ADR-064: Log templates are LogTemplate observations of their own file, not typed edges, until the system graph's tables open (2026-10-09)

**Status.** Accepted 2026-10-09 (wave C2, row TL-4; F67, the log-template index). Design:
`docs/design/telemetry-links.md` section 9. Plan: `docs/plans/telemetry-links.md` (TL-4, TL-10 to
TL-14). Reading: `docs/bench/readings/telemetry/tl4-replay.md`. Observations O700, O701, O702.
Reads ADR-049 decision 2 (the table freeze until RT-M1 reads PASS), ADR-050 and
`internal/fusion/routeref.go`'s interim divergence, ADR-058 decision 3 and ADR-062 decision 6 as
constraints.

**Context.** `localize` should take a pasted log line to the logger call that wrote it (F67). That
needs each logger call's template at query time: file, line, logger API, template text and its
placeholder spans, the enclosing symbol. The table freeze allows no new table, column, cache or
background job, so the facts must live in an existing table. Two were candidates. typed_edges
under a pass key, the route-ref precedent: but a key survives the Go typed pass's reconcile only
when `isNamespacedTypedKey` names it (O335's class), so in a repository with Go inputs the next Go
change would purge the templates, and the readers that count typed edges (meta's totals, the
evidence report's typed share, the console's tally, conserve's per-language tier) would read every
template as a call edge (O701). observations as a non-oracle kind, the TelemetryIdentity and
GHAPin node precedent: a file's observations are purged when it changes and when it is deleted,
and `ReingestFile` replaces them, so a template stored there follows its file for free.

**Decision.**
1. *A template fact is a node of source kind `LogTemplate` in its file's own observations*, named
   by the template text, at the call's line (`EndLine` equal), tier pre-scan, signed
   `<api>[ in=<enclosing>][ holes=<start>-<end>[i],…][ keys=<k>,…]` (`internal/logtemplate`'s
   `Signature`, read back by `FromNode`), its package the enclosing symbol's.
2. *The ingest hook* (`enrichLogTemplates`, run after the extract stream and timed inside it)
   reads this ingest's changed candidates again (Python, Java, JavaScript/TypeScript, PHP), only
   where the store holds that very content, and adds templates to a file that holds none; it
   sweeps every candidate once when no template pass has completed for the service (a pass-state
   row in the typed-edge fingerprint table, which no typed pass reconciles away) or when the
   previous ingest never committed. `ReingestFile` writes a file's templates with its symbols
   (parity). No new table, column, cache or background job.
3. *`LogTemplate` is in no dialect's `oracle_kinds`* and no dialect emits it: the reader is
   cross-language data (`apis.json`, universal defaults, K15), and the language readings measure
   each dialect's own extractor, so none moves (re-run on one subject a language, identical).
4. *`localize` never resolves a frame, a grep hit, a name or a template to a `LogTemplate` node*;
   other line-to-symbol readers meet the node at exactly its call's line (O700, open).
5. *The pasted line is parsed in the call and discarded* (ADR-058 decision 3, ADR-062 decision 6,
   cited not amended): a matched line seeds a typed `log_template` seed and nothing else, so only
   the template's own text and call site are cited; the match is deterministic (longest coverage,
   placeholders as wildcards, then literal length, then file and line), provider none.

**Alternatives rejected.**
- *typed_edges under a `log-template` key.* The Go reconcile and the counting readers above
  (O701); fixing either is a cross-cutting edit to shared storage for an interim home.
- *A new table.* The freeze (ADR-049 decision 2) until RT-M1 reads PASS.
- *Templates emitted by each language's dialect.* A dialect is measured against its oracle's
  kinds and owned per language; the logger list is one cross-language registry with a `.fathom/`
  override coming (ADR-061), and the matching needs the API, which a dialect's tags do not carry.
- *A log store, a fetch from a log system, or an index of rendered lines.* F67's do-not-build.

**Consequences.**
- Symbol counts grow by the logger calls; `find_symbol` can find a template by its text.
- A changed or cited logger line maps to its template in readers other than `localize` (O700).
- No `ExtractorEpoch` bump is required by this record (the sweep fills a store once); the wave's
  one bump re-extracts every file anyway.
- Measured: TL-4's replay reads 92 of 96 top-1 (0.958 in every language, 0.021 before), no echo;
  the reader costs 32 ms a pass over 387 Python files and the store 0.44 percent more.

**Jev, weighed.** The decision port could order candidates when a line matches identical
templates in two places, the one ranking choice left. It is not a site: ties are seeded at equal
weight and disclosed, never judged (I62), and the replay's misses are a missing template, not a
ranking choice. With provider none, the default, everything here runs unchanged. What would admit
Jev: an adopter's replay where top-1 falls below 0.90 because of ties.

**Revisit when.**
- RT-M1 reads PASS and the system graph's tables open: templates move to their own table (or
  system_links), out of the symbol rows, and O700 closes with the move.
- A reader is found to mistake a `LogTemplate` node for code (O700 widens).
- An interim pass registry exists (O701's fix): typed_edges is weighed again.
- ADR-061's zones ship: the logger API registry gains its `.fathom/` override.

## ADR-065: A document a Kubernetes object embeds is read through its own dialect, and a secret-bearing source is never stored or served (2026-10-10)

**Status.** Accepted 2026-10-10 (wave c3, item obs; synthesis rows 13 and 14, K14, K15, K19, K20). Design:
`docs/design/language-breadth.md` section 4 "Observability documents"; `docs/design/telemetry-links.md` section 10.
Plan: `docs/plans/language-breadth.md` F.47 (LB-F361 to LB-F367), LB-Q-alertmanager, LB-Q-grafana-dashboard.
Readings: `docs/bench/readings/lang/alertmanager.json`, `grafana-dashboard.json`. Observations O716 to O720. Reads
ADR-048 (language breadth), ADR-049 decision 2 (the table freeze), ADR-050's interim doc-xref edges, ADR-062 (Stage
A, Stage B gated) and K19 as constraints.

**Context.** Two facts met in one wave. First, observability configuration often ships inside a Kubernetes object,
not as a file of its own: kube-prometheus' 33 dashboards sit in a ConfigMapList, its Alertmanager configuration in a
Secret's `stringData`, its Grafana datasources in another Secret; a grafana-operator dashboard is a CRD's
`spec.json`. Read as Kubernetes, each is one object and its document is invisible. Second, these files are where
credentials live: an Alertmanager configuration holds SMTP passwords, Slack and webhook URLs, API keys and routing
keys (226 fields its config package types a secret), a Grafana provisioning file `secureJsonData`, basic-auth
passwords and contact-point tokens. An index that stores those, or a tool that serves them into an agent's context,
leaks them. symbol_context's snippet reads a fixed window from a definition's line, so even an entity that selects no
secret would serve the lines beside it.

**Decision.**
1. *An embedded document is read through its own dialect as a document of its own* (`fusion/k8sembed.go`): a
   ConfigMap's (or ConfigMapList item's) `.json` value that is a dashboard or `.yaml`/`.yml` value that is a Grafana
   provisioning file, a GrafanaDashboard's `spec.json`, a Secret's `alertmanager.yaml` (or a provisioning file) in
   `stringData` or base64 `data`, each only when its dialect's content sniff claims it. Its entities sit beside the
   Kubernetes object's in the manifest, qualified by the key (`alertmanager.yaml/route.routes[0]`), a document root
   named by its file renamed to its key, a dashboard keeping its uid; its lines map onto the manifest's at a block
   scalar's offset, onto the value's own line otherwise. The wrapper is deliberately no DocResulter, so the route and
   policy passes keep reading the manifest as the Kubernetes object it is; the doc-xref pass's hook reads the embedded
   documents. ReingestFile reads them as ingest does.
2. *A secret-bearing field is never selected*: a dialect's spec names no such field in an entity, signature,
   reference, fact or matcher; `signature_keys` signs an entity by the keys it writes, never their values (a
   receiver's integrations). Proven by data, not reviewed by eye: the oracle lists every secret-typed field by
   reflection over Alertmanager's config package, and a test plants a sentinel in each (and in Grafana's) and finds it
   in no row of any store table.
3. *A sensitive entity's source is never served*: a doc spec marks an entity `sensitive` when its file may hold
   secret values (every Alertmanager entity, every Grafana provisioning entity); symbol_context answers it without a
   snippet and says why (`snippet_withheld`). `read_source` is unchanged: it serves a file the caller names, under its
   own policy (`.env`-pattern files refused).

**Alternatives rejected.**
- *Embedded documents as derived files on disk, or a pre-ingest extraction step.* A second artifact to keep in step
  with the manifest, and a write the index must never make in an adopter's tree.
- *A DocResulter wrapper for the manifest.* The route and policy passes take a DocResulter's result as the file's
  whole truth and would lose the Kubernetes object's routes, selectors and labels.
- *Redacting secret values in stored text or snippets by key name.* A `url` is a secret in one integration and not
  in another; a list of names guesses, the type system knows (decision 2 reads Alertmanager's own types).
- *A new table or column for sensitive symbols.* The table freeze; a kind-level flag in the dialect's own data
  suffices.

**Consequences.**
- kube-prometheus' dashboards, Alertmanager configuration and Grafana provisioning are navigable symbols where the
  index held one Kubernetes object each; alertmanager reads L3 excellent and grafana-dashboard L3 good (tier C).
- The doc-xref pass reads a Kubernetes YAML file a second time when it runs (O718).
- A Kubernetes Secret object's own snippet still serves its data (O717): pre-existing, out of this decision's reach
  until the kubernetes dialect is dispatched.
- No `ExtractorEpoch` bump is required by this record beyond the wave's one.

**Jev, weighed.** No ranking or judgment site here: what an embedded value is, is its dialect's sniff and decode,
deterministic; whether a field is secret is a type fact. Jev would be a guess where the answer is known. What would
admit it: a format whose secret fields are not typed anywhere (a free-form plugin `settings` map), where a judged
redaction might help; then the decision port, never a heuristic in the extractor.

**Revisit when.**
- The kubernetes dialect replaces k8sextract.go at dispatch: embedded documents move into its DocResult and the
  second read (O718) goes; a Secret object's own snippet is withheld (O717).
- A dialect for a format whose secrets are untyped is proposed (decision 2 needs another proof).
- An adopter embeds a third document kind (an OpenTelemetry Collector configuration in a ConfigMap,
  LB-Q-otel-collector-rest): decision 1 extends by data.

**Amendment (2026-10-10, wave h3; SR-76, O767).** Decision 3's rule is the file's, at the answering layer
(`internal/mcp/secret_withhold.go`): a definition in a file holding an entity of a sensitive kind, or a core
Kubernetes Secret (`Secret/<name>`, apiVersion `v1`), is answered without its snippet (`snippet_withheld`), and
references serves an occurrence in such a file without its line text (`text_withheld`). The Secret object's own
snippet (O717) and an embedding ConfigMap's or Secret's are withheld before the kubernetes dialect's dispatch, so the
first Revisit trigger's second half is met. `read_source` and `search`'s literal grep mode stay raw-bytes readers
(`docs/mcp-tools.md` "Secrets"); every tool on `/rpc/extended` is asked by SR-76's test.

## ADR-066: fathom is pure Go: Linux builds CGO_ENABLED=0, and darwin keeps cgo only for cmd/fathom's two memory readers (2026-10-10)

**Status.** Accepted 2026-10-10 (wave h4, item cgofree; owner K25 ruling 1). Supersedes ADR-013. Plan rows:
`docs/plans/language-breadth.md` LB-F395 and LB-F19 (design `docs/design/language-breadth.md` §11 step 4);
`docs/plans/cloud-sessions.md` CS-9. Closes future F1 (XT-6b) and question Q6; fires ADR-025's Revisit trigger (1).

**Context.** ADR-013 put general-language extraction on the CGO tree-sitter bindings and accepted a C toolchain at
build time because DuckDB (ADR-003) already imposed one. Both reasons are gone. SS-8 deleted DuckDB (ADR-025);
ADR-021 §4 and ADR-048 moved every tree-sitter language onto WASM grammars run by wazero; wave lf3 deleted the CGO
walkers once lang-parity read their dialects MERGE (LB-F390 to LB-F394, LB-F400). The last importer of
`github.com/tree-sitter/go-tree-sitter`, `internal/extractor/treesitter/`, was a package nothing imported, with no
tests. ADR-013's successor had to be measured, not assumed (§11 step 4: whether `CGO_ENABLED=0` builds follow).
What is left, read by `go list -deps` over cmd/fathom with cgo on: for linux/amd64 and linux/arm64, no non-stdlib
package carries a C source; for darwin, only cmd/fathom's `host_darwin.go` and `rss_darwin.go`
(`//go:build darwin && cgo`): `proc_pid_rusage`, `proc_listchildpids` and the memory-pressure and swap sysctls the
daemon's memory governor (RT-32) and its live RSS (DG-16) read. golang.org/x/sys v0.47.0 wraps neither libproc call.
Without cgo they fall back to `host_other.go` and `rss_other.go`, which read Unknown, disclosed.

**Decision.**
1. *Linux builds are pure Go.* `task build` pins `CGO_ENABLED=0` off darwin whatever the shell exports, the
   Taskfile's default is 0 there, and `scripts/cloud-setup.sh` builds `bin/fathom` and its tools with 0. Building
   fathom on Linux needs Go alone, and the binary is static.
2. *darwin keeps cgo for cmd/fathom's two memory readers and nothing else.* `cmd/fathom/cgofree_test.go` (CS-9)
   fails the gate on any non-stdlib cgo, C, C++, ObjC or SWIG file in cmd/fathom's dependency set for linux/amd64
   and linux/arm64 (read with cgo on, because under `CGO_ENABLED=0` go list silently drops cgo files; the
   `CGO_ENABLED=0` load must also resolve), and on any darwin cgo file but those two. `MACOSX_DEPLOYMENT_TARGET=14.0`
   stays for a reason measured now: without it ld printed no warning, but the cgo (external) link stamps the
   binary's minimum macOS as the build host's (minos 27.0 here) instead of 14.0; a pure-Go darwin build reads 12.0.
3. *`-race` is the one Linux use of cgo.* Go's race detector needs cgo there, so `task test` exports
   `CGO_ENABLED=1` and `task validate` on Linux needs a C compiler; the build does not. The Linux test build differs
   from the shipped one only in the standard library's cgo paths (net's resolver, os/user).

**Measured (2026-10-10; darwin arm64 host, Go 1.26.2, machine load 10 to 17 from parallel waves).**
- `GOOS=linux CGO_ENABLED=0 go build ./cmd/fathom`: amd64 84,810,959 B and arm64 78,352,754 B, both static.
  linux/amd64 with cgo on (zig cc as the C compiler): 84,878,515 B, dynamically linked against glibc;
  `CGO_ENABLED=0` is 67,556 B (0.08%) smaller.
- darwin `task build`: 80,852,882 B at origin/main (fb7d9f3f) and at this change: 0 B, because the deleted package
  was never linked. `deadcode ./cmd/...` 502 before and after, the same list.
- `go.mod` loses 7 require lines (go-tree-sitter, the c, javascript, php, python and rust bindings, mattn/go-pointer)
  and `go.sum` 28 lines.
- Cold builds, a private empty GOCACHE wiped between runs, mean of two alternating runs, before → after:
  `go build ./...` CPU (user+sys) 171.4 s → 159.4 s (−12.1 s, −7%), wall 42.5 s → 41.8 s; `go build ./cmd/fathom`
  CPU 150.8 s → 148.0 s, wall 40.3 s → 36.2 s (noise: the package was never in its graph); `go vet ./...` after
  the build CPU 96.9 s → 97.2 s (unchanged). The CPU saving is tree-sitter's C core no longer compiling; wall times
  moved within the load's noise.

**Alternatives rejected.**
- *Keep `CGO_ENABLED=1` everywhere* (ADR-013's status quo): a C toolchain to build a binary with no C of its own left,
  and a glibc-dynamic Linux binary.
- *`CGO_ENABLED=0` on darwin too*: the governor would read Unknown and turn its ladder off on the platform its daemon
  runs supervised (launchd). Wins if: a pure-Go path to `proc_pid_rusage` and `proc_listchildpids` exists.
- *Keep `internal/extractor/treesitter/` for a later use*: nothing imported it, and its require kept tree-sitter's C
  core compiling in every `go build ./...` and `go vet ./...`.

**Consequences.**
- An adopter on Linux builds with Go alone; the cloud VM's build needs no compiler, its gate still does (-race).
- ADR-025's Revisit when (1), "the sole remaining reason for `CGO_ENABLED=1`", fired: its decision stands, the
  reason it named is gone. F1 and Q6 met their own condition ("deleted when LB-F19 is proven").
- No `ExtractorEpoch` bump: nothing dispatched changes.

**Jev, weighed.** Whether a package carries C is a `go list` fact; there is no ranking or judgment to port. None.

**Revisit when.**
- A dependency needs cgo on Linux (CS-9 fails): retire it, or amend this record with the measured reason.
- A pure-Go path to `proc_pid_rusage` and `proc_listchildpids` lands (golang.org/x/sys or libSystem trampolines):
  darwin can drop cgo too.
- Go's race detector runs without cgo on Linux: `task test` drops its `CGO_ENABLED=1` and the gate needs no C
  compiler.
- F2 ships prebuilt binaries: the darwin minimum macOS and the Linux static binary are then release properties.

## ADR-067: Every file accounted for: role, runner, configurer, entry point and test entry are computed facts of a repository graph, delivery is repaired before content is added, verify reads attested runs and stays static, and features, references and ops concerns join it without a new engine (2026-10-10)

**Status.** Accepted 2026-10-10 (owner-architect under the owner's delegation, after an adversarial review the same
day, with the rulings recorded below). Numbered after wave h4's ADR-066. Owner notes of 2026-10-10, N1 to N11.
Design: `docs/vision/42-every-file-accounted-for.md`. Plan: `docs/plans/repository-facts.md` (RF-1 to RF-17, RF-M1
to RF-M7), with rows merged into test-intelligence (TQ-26 to TQ-28), test-certifications (TC-29),
governance-learning (GL-24, GL-M6), exploration-maps (XM-36, XM-37, XM-M8, XM-D10), served-not-kept (PR-13), reach
(RH-13), judgment-foundations (JF-25), feature-map (FM-15, FM-4 amended), incident-maps (IM-M0 amended),
telemetry-links (TL-15, TL-16) and language-breadth (LB-F451 to LB-F458, queue edits). Goal G36. Futures F68 and
F69, F37 amended. Invariants: I8 amended, I57 extended to call edges, I65 stated. Amends ADR-023 and ADR-055
(decision 8(c) and its Consequences bullet). Adds ADR-063 O11. The owner rulings it cites (K22, K23, K26) and the
web rulings W1 to W4 are committed in `docs/design/language-breadth.md` section 1.8 by the language lane (LB-F455),
the only lane that edits the language-breadth files. Reads ADR-016, ADR-043, ADR-044, ADR-048, ADR-049 decisions 1
and 2, ADR-050, ADR-051 decision 1, ADR-056, ADR-061 (proposed), ADR-062, ADR-064, platform-engineering ADR-0015
and ADR-0095, and Review Kit F5 as constraints. The evidence-export design of the same day (N12) is ADR-068.

**Owner-architect rulings at acceptance (2026-10-10).**
- *W3 (O482).* The oracle-blind cap applies per subject, so a typed subject cannot lift untyped code past its own
  grade (decision 15).
- *The RF-M6 guard.* The "heuristic edges never gate" guard covers the new `runs` and `configures` links from their
  first row. It extends to every heuristic call edge in the fleet only after RF-M6 publishes its count. The count is
  published whatever it reads, and a nonzero count ships the fleet-wide guard with a `### Breaking` line and a
  migration note (decision 3).
- *I8 and ADR-023.* Amended as decision 11 says. D0-1 retires.
- *ADR-063 O11.* Accepted. The upstream Kyverno policies a repository applies, their licence checked at use, stand
  in until policy-kit ships. Fathom builds no resource-policy evaluator.
- *K22, K23 and K26.* Their wordings are reviewed by the owner-architect when the language lane commits them.
- *Open to the owner.* An ops consumer with production manifests (else the selector join waits for its 2027-09-30
  kill date). The CI or review sink for SARIF and JUnit is ADR-068's open input.

**Context.** The owner asked for standard layers describing tests, scripts and configuration, for config and
script association, feature-to-code maps, reference implementations linked to specifications, operational
excellence layers, a cheap way to populate all of it, ways for fathom to make agents more capable, and the web and
scripting languages at their highest practical tier. Read against the tree at fb7d9f3f:

- Fathom's proof verbs are Go-shaped. `wired` reads n/a for a non-Go symbol, product entry points are Go `main`
  and `init` only, a test is a symbol named `Test*`, test layers exist only for Go test files, the evidence join reads
  only Go `Test*` functions, and the `certifies:` marker is Go-only.
- Non-code files are invisible to impact. `blast_radius` on a script the pre-push gate runs returns 0 symbols. The
  run rule (`ghaRunRefs`) has two callers, GitHub Actions and Jenkins.
- Verify reach is tier-blind and `wired` gates `--strict`, while I57 is enforced only for syslink links. A new
  heuristic edge would gate on arrival.
- Eight product rules decide whether a file is a test and two decide whether it is vendored. tsconfig's scope is
  computed at resolve time and discarded.
- Delivery is unexplained. From 2026-10-03 to 2026-10-10 the edit hook emitted on 78 of 3,367 file-target calls
  (2.3%) and on 5 of 963 Edit and Write calls, and 21,247 hook answers carry no reason, so whether that is a leak is
  unknown. 1,704 Bash writes were silent as `foreign-target`, a class mixing worker clones, gitignored drops and
  scratch writes, logged without a target path. The pin line skipped 744 of 745 checks, and the rule-outcome writer
  has written nothing since 2026-10-01 although JF-1 and GL-10 are proven (INC-27).
- Computed facts go unread. No caller passes coverage to the risk rank, and verify's `runtime_pass` is hard-coded
  Unknown while attested test runs sit in `test_runs`. I8 and ADR-023 define verify as static with `runtime_pass`
  always Unknown, so wiring it is an amendment.
- A layer (ADR-016) compiles a document corpus into cited claims. ADR-056 R1 and JP-X2 already refuse computable
  labels as model work, and the `ai-derived` tier does not exist (F7).
- Resource-shaped rules are refused by Review Kit unless they join policy-kit by canonId (platform-engineering
  ADR-0095 decision 5), and Review Kit builds no infrastructure scanner or second resource-policy evaluator (its F5).
  policy-kit (ADR-0015) is unbuilt.
- The table freeze (ADR-049 decision 2) holds until RT-M1 reads PASS. RT-M1's last window failed on 107 minutes of
  swap-volume disk-floor refusals. The bench and model spend hold of 2026-09-29 holds.

**Decision.**
1. *Four existing kinds of record, no new container.* Every association is a fact (computed by the ingest), a claim
   (compiled from a governing document), a map entry (filed by a session) or an attested record (a producer's
   envelope, run, coverage report, imported finding or ledger row). No "semantic map" store, table or tool is added.
2. *The role facet.* One pure classifier, `repofacts.Role`, over a closed vocabulary (source, test, fixture, script,
   config, ci, build, iac, schema, migration, docs, generated, vendored, unknown) with the basis order `attribute`
   (`.gitattributes` `linguist-*`, root and nested) over `header` (Go's generated-code line and the ADR-063 O6 kit
   provenance header) over `dialect` over `linguist` (Linguist's vendor, documentation and generated data, pinned)
   over `convention`. The eight product test rules and two vendor classifiers call it before any new consumer does.
   Directory-walk skip lists, copies of a tool's own semantics and the bench graders are out of scope, the graders
   so that they stay independent of their subject. Computed on read, no store column while the freeze holds.
   `unknown` is counted and printed. Only convention roles are graded (RF-M1, the `test` role against each runner's
   own collection). The other bases are their source's answer and claim no reading.
3. *`runs` edges, guarded from their first row.* Verify's `reachable_from` and `wired` never pass on a `runs` or
   `configures` link, and I57 is extended to call edges, enforced by a test. The guard over every other heuristic
   call edge in the fleet (RF-5) waits on RF-M6, which counts at $0 the proven fleet rows whose reach passes only
   through a heuristic edge. The count is published whatever it reads, and a nonzero count ships RF-5 with a
   `### Breaking` line and a migration note. The run rule gains callers (Taskfile `cmds`, `package.json` and
   composer scripts, Makefile recipes, lefthook, pre-commit, GitLab CI `script:`, Dockerfile `RUN`, `ENTRYPOINT`
   and `CMD`, Kubernetes container `command` and `args`), command words (`go run`, `go test`, `node`, `python`,
   `python -m`, `npx`, `tsx`, `php`, `vendor/bin/*`, `pwsh -File`, `bash -c` once) and a file-level node for a
   script with no function. Heuristic tier in the existing edge tables, labelled. `blast_radius` and `test_impact`
   follow them and say so.
4. *`configures` facts.* A config file's declared scope (tsconfig `include`, `files`, `exclude`, `extends`,
   `references`, `.editorconfig` sections, workspace members, Dockerfile `COPY` sources, Nx and Turborepo inputs)
   is an observation of its own file (the ADR-064 precedent). Membership is computed at read time by
   nearest-ancestor resolution, never stored per governed file, and answered at the existing tier `heuristic`. No
   new tier. Tool configs such as eslint, jest and vite are tech-pack data after ADR-061 is accepted. Env and config
   provide-and-consume stays SG-20 and SG-21.
5. *Non-Go entry points and test entries, informational first.* `wired` reads product entry points from package
   manifests (including composer `bin`), Python main guards, Cargo targets, Java and C# mains, and scripts a `runs`
   edge reaches from CI or a task. The non-Go verdict is printed and never changes a row's status until RF-M7 has
   counted the fleet rows it would flip and a later change carries that count, a `### Breaking` line and a migration
   note. Test intelligence reads pytest, jest, vitest, mocha, JUnit, xUnit, NUnit, PHPUnit and Rust test entries,
   and the existing join `JoinCasesToSymbols` (TC-3) is extended past Go to them, never copied. A non-Go test reaches
   and is selected, and certifies nothing until TC-26 lands the marker for its language.
6. *No compiled "standard layers".* Tests, scripts and configuration are the facets of decisions 2 to 5, served by
   existing verbs. The model-shaped residue (purpose text, concept tags, test kind beyond signatures) merges into
   F37, amended. Invariant I65 adds one clause to ADR-056 R1: the `unknown` residue of a file-fact rule is counted
   in every reading and answer that uses the facet.
7. *Onboarding prints a priced plan and spends nothing, inside the existing dry run.* RH-11's `fathom init
   --dry-run` gains a priced section (RH-13): the deterministic facets at $0, each compile layer priced with IO-8's
   2-to-8 band (`internal/semantic/compile.go`), and each decision-port site "unpriced" until a per-site estimator
   (JF-25) exists. JF-25 prices through `internal/cost` (PE-16), never a second table. Consent is per line. No new
   flag.
8. *Features merge into the feature map (ADR-055).* Role, run-by and configured-by join FM-4's column list.
   Co-change clusters become an FM-3 candidate source that is never written (FM-15). No sequencing change.
9. *Reference implementations are a map kind, deferred (F68).* An exploration map whose anchors point into a service
   ingested from an operator- or agent-made clone, flagged `reference`, pinned at that service's commit, carrying a
   licence from a pin manifest that reuses the calibration corpus shard schema, linked to spec IDs, advisory only.
   Fathom never fetches (I56). Built when a $0 transcript scan (`git clone`, `gh repo clone`, WebFetch of
   `github.com` URLs) shows one upstream researched in at least 3 sessions within 30 days across at least 2
   repositories, or the owner names a consumer. Preserved now: a map entry keeps its service for both anchor forms
   (span paths carry none today, and `NormalizeSymbol` drops `svc::`).
10. *Delivery before content.* The joins land in this order: the rule-outcome writer restored and every silent hook
    answer given a reason (GL-24), a destination class on the edit hook's beat and then the clone and worktree fold
    (XM-36, XM-37), a HEAD stamp for the pin check (PR-13), coverage into the risk rank from a named source with no
    store (TQ-26), attested runs into `runtime_pass` (TC-29, decision 11). Stored churn reaches `localize` only as a
    third arm of IM-M0, never as its own row. Then the facets into impact, wiring and review, then the file passport:
    SR-60's pre-edit line (`editHookBlastLine`) extended at a session's first edit of a file with role, run by,
    configured by, certifying tests and map pointers, at most 60 of the edit hook's 320 payload tokens (ADR-016
    Q21), dropped before spec lines, never displacing a rule, and the same block from `symbol_context` on a file
    path. No new MCP tool. The passport's reading (RF-M4) is a lift over a pre-ship base rate. No adherence claim is
    made from the owner's wave traffic alone: the general case (one repository edited in place, git worktrees) is
    reported separately and claimed only at 100 first touches or more.
11. *Verify reads attested runs and stays static: I8 and ADR-023 amended.* Static means verify reads attested
    evidence and never executes a test or a command. `runtime_pass` reads the newest attested run in `test_runs`
    for a row's certifying tests, Pass or Fail with the run's commit and age, Unknown when none exists, informational,
    never changing a row's status. D0-1 (a `cmd` oracle that would run commands) is retired as superseded. The
    `TestCmdVerify_*` tests that pin Unknown are updated in the same change.
12. *Operational excellence.* Fathom supplies facts (Kubernetes per-container requests, limits, probes, init
    containers as their own fact, replicas, read from all three pod paths: `spec`, `spec.template.spec` and
    CronJob's `spec.jobTemplate.spec.template.spec`), membership (policies through `policyref.go`, monitors through
    the shipped selector joins, served as observed-by, TL-15), delivery at the edit and proof by certifying tests.
    Resource-shaped rules belong to policy-kit (ADR-0015) or, until it ships, the upstream Kyverno policy library a
    repository already applies, its licence checked at use. Review Kit owns review judgment, severity, dispositions
    and typed criteria, over practice, process and artifacts. A tech pack never carries a resource rule, and fathom
    builds no resource-policy evaluator. ADR-063 gains O11 (resource policy: delegate to policy-kit). Observability
    is the second ops arm: alert and metric reach a workload only through Prometheus' own entities and monitor
    selectors (K19), and metric-to-emitter stays F66. Only the per-container facts and observed-by are built now.
13. *Correction of ADR-055 decision 8(c) and its Consequences bullet.* Review Kit concern packs reach the edit
    through the findings outbox import (ADR-040). A kit's guards and formats may ride tech packs once ADR-061 is
    accepted. A resource-shaped rule stays policy-kit's. No pack-to-obligations compiler exists or is built until a
    pack consumer exists.
14. *Default-on is earned by a reading.* A deterministic facet becomes default-on, behind an opt-out key, when it is
    $0, egress-free, byte-stable, inside the freeze and the hook bounds, adds no tool, and its reading holds:
    precision at least 0.95 with a Wilson lower bound at least 0.90 across at least three ecosystems, ingest at most
    +3% wall and +2% store (or a K26 ruling), and its named consumer's metric holds (vision 42 section 8 names one per
    facet). Above +3% wall a facet is computed at query time. A judged site climbs off, opt-in, then offered by
    default at onboarding (one consent per repository), the last rung only after J0 PASS, a calibration lower bound
    at least 0.90, a measured cost at most $0.25 per 10,000 files, an estimator within 2 times, a positive consumer
    A/B, F7 if stored, and the spend hold lifted. "On whenever a provider is configured" is F69. A separate agent that
    generates decision-port inputs is refused.
15. *Web, scripting and ops formats.* The queue edits of vision 42 section 9 are adopted, landed by the language
    lane. Language rulings are named W1 to W4 (they collided with ADR-056 R1 to R8): W1 (HTML L2 is same-document
    IDREF references), W2 (lightningcss is tier A for what it attests), W3 (O482: the oracle-blind cap applies per
    subject) and W4 (a format's own compiler or linter is its reference parser) are accepted and committed in the
    language design before any holdout reading (K19). Lua, Perl, Blade, Astro and Jinja2 go to T1 (T2 is build,
    package and CI). Jenkinsfile goes to L3, and Kubernetes, GitHub Actions and Helm each get an
    excellent-or-ceiling row, ahead of the smaller languages. PHP's L2 ceiling is restated: scip-php is tier A, and
    excellent waits on its 1,869 unexplained pairs.
16. *Dependency mapping* stays as decided (ADR-050, ADR-063 O1, I56). RT-M1 reading PASS is named the critical path
    of the roadmap. At most two waves run at once, and none starts during an RT-M1 window while the swap volume has
    under 15 GiB free.
17. *Goal G36, every file accounted for*, is registered with $0 readings (RF-M1 to RF-M3, RF-M5). RF-M5's 30 files
    are a seeded random draw whose seed and subject SHAs are committed before the probe runs.

**Alternatives rejected.**
- *Compiled layers for tests, scripts and configuration.* A tier citing no document, per-repository cost, ADR-056
  R1 and JP-X2.
- *Model or Jev labels for roles, runners or configurers.* Computable. A model is kept for the counted residue only.
- *"Three containers, never a fourth."* Attested records were already a fourth kind.
- *Grading Linguist-derived roles against `github-linguist`.* Precision 1 by construction.
- *A `file_stats.role` column now.* The freeze. Computing on read costs one path match per answered file.
- *A `declared` resolution tier.* None exists. `heuristic` already never gates once the guard holds.
- *`runs` edges with no guard, and non-Go `wired` gating at once.* The first gates on a heuristic edge, the second
  breaks the pre-push for every non-Go row without notice.
- *One stored edge per file a config governs.* A root tsconfig would add thousands of rows per repository.
- *A new `init --plan` flag.* Duplicates RH-11's `--dry-run`.
- *`runtime_pass` wired without amending I8 and ADR-023.* A silent contradiction of accepted records.
- *Churn wired into `localize` on its own row.* It is IM-3's hop, gated on IM-M0, and 180-day churn is not recency.
- *A reference-implementation feature with its own verb, a `git:` contract key now, or fathom cloning upstreams.*
  Law 4, ADR-056 R6 and I56. A map kind covers it, and a key rides ADR-050's additive kinds if F68 promotes.
- *An ops kit, a fathom rule catalog, fathom running kube-linter or Polaris, or a resource predicate in a tech pack
  (the earlier F52 static-scope amendment).* SV-X7, ADR-055 decision 7, ADR-061's proposed invariant (a), Review Kit
  F5 and platform-engineering ADR-0095 decision 5.
- *A label join from an alert or metric to a workload.* K19's Stage A kill.
- *Building the pack-to-obligations compiler ADR-055 decision 8(c) assumed.* No pack consumer exists (P20).
- *Onboarding that compiles or judges by default.* O8 under-reported a compile 25 times, and spend is ask-first.
- *Default-on judged sites whenever a provider is configured.* Amends JF-4 and ADR-056 R2 without a single
  calibrated site.
- *Feature-map v0 on inventory keys now.* RT-M1 may read PASS on its next window. Reconsidered on 2026-11-15.
- *Adding passport content before delivery is explained.* The hook's silences carry no reason.
- *A 120-token passport bounded by the primer cap.* I12 bounds the primer. The edit payload is 320 tokens, and 120
  of them is 37.5%.

**Consequences.**
- Impact, wiring and test selection answer for scripts, CI, task files and the languages K23 puts first. Search
  flags and test detection change where the unified classifier disagrees with an old copy (bounded by RF-M1's 2%
  rule).
- `fathom verify` may change verdicts twice, each announced: when the fleet-wide RF-5 guard lands (rows reached
  only through a heuristic edge, counted by RF-M6) and, later, when non-Go `wired` gates (rows counted by RF-M7).
  Each carries a `### Breaking` line and a migration note if its count is nonzero. Until then the non-Go verdict is
  informational, and the new `runs` and `configures` links never gate from their first row.
- I8 now reads "static means reads attested evidence, never executes", and `runtime_pass` can read Pass or Fail.
  The `TestCmdVerify_*` tests change with it. D0-1 is retired.
- The edit hook's payload gains at most 60 tokens per file per session, inside the 320-token ceiling, never at a
  rule's expense.
- `review_diff` gains dimensions for config, script and CI diffs, heuristic and never gating.
- Ingest work grows by the run-rule callers and the config scope observations. The facets move to query time if
  ingest grows more than 3%.
- PR-M1's 30-day record clock restarts when PR-13 lands. GL-M2's clock starts when GL-24 lands. G21 (review
  2027-01-15) is at risk until then.
- Record defects are corrected in the same change: ADR-055 decision 8(c) and its Consequences bullet, the silent
  rule-outcome writer (INC-27 through `intent-incident`), the KR-11 and KR-12 inconsistency, and two checked boxes
  whose proof never existed, VB-19 and CX-17, found by ADR-068's reading and reopened.
- K22, K23 and K26 move from an operator note into `docs/design/language-breadth.md` section 1.8 when the language
  lane lands LB-F455, so every record that cites them resolves.
- No `ExtractorEpoch` bump is required by this record. The waves that build decisions 3 to 5 and 12 bump it once
  each, by the concurrent epoch rule.

**Jev, weighed.** No new site. Roles, runners, configurers, entry points and test entries are rules. Ordering an
overflowing passport would be a re-rank, and the obligations re-rank A/B read negative. Matching a reference to a
spec would be generation. The memory and leak residue is judgment in Review Kit's criteria lane. What would admit
Jev: RF-M1's unknown residue above 5% of files on a subject while J0 reads PASS, then F37's first template, "role
among declared layers", opt-in per repository.

**Revisit when.**
- RF-M1, RF-M2, RF-M3, RF-M4 or RF-M5 reads CUT: the row it gated is removed in the same change.
- RF-M6 or RF-M7 reads a nonzero count: the change that ships the guard or the gating carries the `### Breaking`
  line and the migration note.
- RT-M1 reads PASS: a role column is weighed if a consumer measured the cost of filtering without one, and SG-2,
  the feature map and the env and config links proceed. RT-M1 still FAIL on 2026-11-15: feature-map v0 on the
  ADR-044 inventory keys is weighed.
- ADR-061 is accepted: tool-config `configures` readers are weighed.
- J0 reads PASS and the spend hold lifts: F37 and F69 are weighed. J0 reads FAIL: both stay down.
- A consumer with production Kubernetes manifests is named, or policy-kit lands its first canonId join: the
  selector join is weighed, and membership reads policy-kit's controls instead of the upstream Kyverno library. No
  consumer by 2027-09-30: the selector join is dropped.
- Census E reaches 0.25 and the owner records an F59 amendment: F66's metric-to-emitter join is weighed beside
  observed-by.
- A non-Go service ingests a test-kit envelope: TC-26 fires and non-Go certifying tests are built.
- F68's baseline count fires, or 2027-01-31 passes without it.
- The owner reverses W1, W2, W3 or W4.
- Edit-hook emission on rule-scoped file edits stays under 5% after XM-37, read with GL-24's reasons: rule scoping
  is fixed before more delivery.

**Amendment (2026-10-11, wave rf2): two Revisit-when conditions fired.**
- *RF-M1 read CUT, and so did its one re-read RF-M8 (the owner-architect's ruling R1,
  `docs/bench/readings/repository-facts/roles-r1.md`).* Decision 2's classifier (`repofacts.Role`, its vocabulary
  and the `attribute`, `header`, `dialect` and `linguist` bases) never had a product entry, so it was deleted rather
  than kept unwired, and RF-1, RF-3 and RF-4 are refused. What stays is RF-2: the eight product test rules and the
  two vendor classifiers call one test and vendor rule in `internal/repofacts`. Serving a role again needs a new
  classifier and its own reading.
- *RF-M6 read 9.* RF-5 shipped with the `### Breaking` line and the migration note
  (`docs/bench/readings/repository-facts/reach-guard-flips.md`). Every call edge outside Go's typed pass is
  heuristic today, so a non-Go `reachable_from` no longer passes until its language gains a typed pass.

**Amendment (2026-10-11, ADR-069): decision 12's build limit widens.** "Only the per-container facts and observed-by
are built now" widens to preStop, terminationGracePeriodSeconds, strategy, minReadySeconds,
progressDeadlineSeconds, affinity and topology spread, env names and `valueFrom` references, and arg flag names.
Env values and arg values are withheld under ADR-065 decision 2, except two allow-lists of non-secret keys: the
retry keys `AWS_RETRY_MODE`, `AWS_MAX_ATTEMPTS` and `AWS_NEW_RETRIES_2026`, and heap flags such as `-Xmx`. A sentinel
planted in a fixture workload's env value and arg value appears in no row of any store table and in no receipt
(WAE-13). Owner-architect ruling D16 of 2026-10-11, flagged to the owner because it widens a decision 12 limit.
Resource rules stay policy-kit's; fathom grades nothing.

## ADR-068: Portable receipts: receipts grown up, so a stranger checks fathom's evidence with git and replays it without fathom's state, in formats other tools already read (2026-10-10)

**Status.** Accepted 2026-10-10 for waves 1 and 2 (owner-architect under the owner's delegation, ruling EVX D13,
after an adversarial review the same day). Waves 3 to 5 are decided in shape and gated: the in-toto record (wave 3)
waits on EVX-G3, and the recipe and replay (wave 4) and the projections and trace pack (wave 5) on EVX-M3. Owner
note N12 of 2026-10-10. Design: `docs/vision/43-portable-receipts.md`. Plan: `docs/plans/portable-receipts.md`
(EVX-, 51 rows). Serves goal G25 (CX-M1, CX-M2). Grows ADR-034 (receipts) and amends no record. Reads ADR-011,
ADR-035 decision 4, ADR-040 decision 6, ADR-043 decision 5, ADR-049 decision 2, ADR-051, ADR-063 (O4, O9) and
ADR-065 as constraints, and owner decisions QK1 and K5. Reopens VB-19 and CX-17, whose proof never existed.

**Context.** The owner asked whether fathom can gather evidence any third-party tool can use (review tools,
compliance, incident reviews): a standard record of the query so it can be run again, several export formats,
plain text among them, usable by a reader with no access to fathom's store, hashing or better so the evidence stays
linked to its source, and a priority call. Read against the tree on 2026-10-10:

- Part of it ships. `fathom export scip` (VB-32) writes the symbol and typed-edge mapping as a standard SCIP index.
  `core.Receipt` and `fathom receipt verify` re-derive callers, references, symbol location and certification reach
  for Go from a bare git checkout (VB-20, VB-21 read 50 of 50). `reviewkey.Key` digests review inputs. One SARIF
  emitter serves seven callers, and two risk converters duplicate each other.
- Nothing on the main path produces a receipt, no fact carries a byte-level anchor, and no record names its query
  with the parameters and inputs that shaped the answer.
- `receipt verify` needs the fathom binary but no daemon and no store, so the honest phrase is "without fathom's
  state". Its toolchain rung runs go/packages with `os.Environ()`, so a module missing from the cache is downloaded:
  an I56 defect on a live path.
- VB-19 is checked and claims per-fact anchors, while the wire carries only `resolution` and `index_commit`. CX-17
  is checked and claims `trace verify` recomputes at the pack's commit, while it reads the current store and working
  tree (`cmd/fathom/trace.go:296`).
- The git watermark is the bare HEAD while ADR-041 decision 4 re-extracts an edited file in place, so an answer can
  come from uncommitted bytes. The index stores `FingerprintHash` (raw working-tree bytes, epoch-salted), not git
  blob bytes, and the two differ under autocrlf, eol, ident, working-tree-encoding or LFS.
- Review Kit's bridge takes seven source-binding inputs, its `SourceFingerprints` are `FingerprintHash` values, and
  its `SpanRef` is shaped to be swapped for agentic-kit's `evidence-span`. MCP serves no per-request `_meta`, so an
  agent cannot request a recipe through it.
- No consumer outside the owner's own code is named. Kubernetes admission engines check attestations on image
  digests, not git blobs. Observability export is refused (IM-X4).

**Decision.**
1. *Grow what exists, no new subsystem.* ADR-034's receipt carries anchors and later a record, `reviewkey` carries
   the recipe and the replay explanation, `FingerprintHash` names each file's indexed state, and the one SARIF
   emitter gains provenance. `fathom export receipt` sits beside `fathom export scip`. Checking, replaying and
   rendering stay under `fathom receipt`. No new top-level verb, no MCP tool, nothing stored.
2. *The record is an in-toto Statement v1 with one fathom predicate, in wave 3.* ADR-063 forbids a second result
   format, so the fathom-owned part is a predicate inside an existing envelope, as test-kit does it. A general
   predicate needs a second consumer (P20), so it waits on EVX-G3. Its shape is fixed now: a `facts` list (not
   `claims`, an O9 word), the commit subject `{name: "gitCommit", digest: {gitCommit}}` that fathom's own inbound
   reader also reads, a CUE schema with valid and invalid pairs, RFC 8785 bytes with integers only, and a producer
   in agentic-kit's `evidence-producer` shape. The predicate URI is an owner call (Q-EVX-1), and it names no host or
   organization and never `urn:fathom:`.
3. *The evidence unit is a fact with git-native anchors.* An anchor carries the commit and object format, the
   repo-relative path and line range, the git blob id, the excerpt sha256 by Review Kit's `SpanRef.ContentHash`
   formula, a bounded excerpt and the file's `fathom_fingerprint`. Its state is `committed`, `uncommitted` (the
   index read other bytes than the commit holds), `filtered` (git's filters change the bytes, so no git rung) or
   `withheld` (a secret-bearing or suppressed file gets no excerpt and no digest, ADR-065). `--inclusion` adds the
   commit and tree objects along the path, so blob-at-path checks with no clone. Git is the Merkle tree, so fathom
   builds none.
4. *Every fact says who can re-derive it.* `rederive.level` comes from a closed table per fact kind, never the
   producer's say-so: `git`, `toolchain:<name>` (`go/types` and `go test` today) or `fathom@<version>`. A judgment
   never enters a receipt (I62). A cross-boundary link below the typed tier never renders as an error (I57). An
   absence fact carries its read set and renders as a lower bound when reads are missing (I38). Git proves the text
   is there, never that it is a definition.
5. *The recipe is a review key over the query's inputs (wave 4).* A `reviewkey.Key` over its six components plus
   `params` (the tool, every parameter with its default filled, `budget_bytes`, refs and relative times resolved),
   plus the inputs reviewkey misses (the service ignore file by role, store-only inputs by digest, one watermark per
   service for a cross-repo answer). An unresolvable input reads `Unknown`. The result digest covers the canonical
   structured result, never the envelope's load-dependent fields. No query language.
6. *Replay answers same, changed or cannot reproduce, each with why (wave 4).* Exit 0 same, 1 changed (explained by
   `reviewkey.Diff`, or unexplained as a named determinism defect) or invalid, 2 cannot reproduce with one reason
   from a closed list. At the recipe's commit it uses the live index. Another commit needs `--scratch`, a `git
   archive` into a temporary directory, never a worktree.
7. *Tamper evidence is layered, and the operator's signer signs.* The file's sha256, a sha256 per fact and git
   object ids give integrity. Authenticity is the operator's tool (`cosign attest-blob`, `cosign sign-blob`,
   `ssh-keygen -Y sign`), each with a key or KMS and no transparency log. Fathom holds no key, verifies no
   signature, makes no network call and writes no wall-clock time unless `SOURCE_DATE_EPOCH` or `--observed-at`
   sets one.
8. *Formats are one-way projections, each carrying the source's digest.* SARIF for findings-shaped facts only,
   Markdown and plain text, and a POSIX `checks` script that needs only git, `head`, `tail` and a sha256 tool. A
   JUnit projection of `fathom verify` is built only when EVX-V1 names a JUnit-reading sink. HTML and OSCAL wait for
   a named consumer.
9. *The first producers are the ones with a consumer.* Wave 1: all seven SARIF callers through one provenance-aware
   emitter, the duplicate risk converters merged. Wave 2: `fathom export receipt` for the four Go kinds. Wave 3:
   `fathom review` and the change risk receipt as records, which Review Kit binds. Wave 4: any agent-surface tool
   through the recipe. Wave 5: the trace pack.
10. *Nothing stored, no new table, no MCP tool, no new parameter, no wire change.* `fathom export receipt <tool>
    --args <json>` runs the tool through an in-process server over the store (the `instrument --adoption`
    precedent), and dispatch captures the recipe where the defaults are known.
11. *Privacy by construction.* A receipt names no host, URL, organization, absolute path or user name. The service
    is redacted by default (`--keep-service` opts in). Repository identity is optional. Excerpts are bounded to 40
    lines (`--no-excerpts` keeps digests only). One allowlist constructor builds every exported receipt, and a
    privacy-scan row checks the golden set.
12. *Call it a receipt, and its list `facts`.* "Receipt" has been fathom's word since ADR-034. "Evidence", "claim"
    and "pack" fall under O9 and K10. The anchor aligns with `SpanRef` and its planned `evidence-span` successor.
13. *Priority: two small waves now as fillers, then gated (owner-architect ruling).* Waves 1 and 2 (about 5.5
    worker-days, $0) run beside the repository-facts waves, and language excellence stays the main line. Wave 1
    also takes `receipt verify` offline (`GOPROXY=off`, `GOFLAGS=-mod=readonly`, EVX-6), closing the I56 defect.
    The record waits on EVX-G3, recipe and replay on EVX-M3, and the projections and trace pack on EVX-M3. A medium
    feature in slices, about 16.5 worker-days in all, not a big near-term bet.

**Alternatives rejected.**
- *A fathom bundle format.* ADR-063 forbids a second result format.
- *SLSA VSA as the predicate.* It attests SLSA levels about an artifact, not facts about code.
- *W3C PROV, CycloneDX or SPDX annotations, OpenVEX.* None fits a code fact.
- *A public transparency log or keyless signing by default.* QK1 and I56.
- *Fathom holding keys or verifying signatures.* The signer's tools own both (ADR-043 decision 5).
- *A query language or mapping DSL.* Vision 29's do-not list. SCIP already exports the mapping.
- *Exporting the store or an index snapshot as evidence.* K5.
- *A Merkle tree over facts now.* The flat fact-id list proves membership (EVX-D3).
- *Query results in SARIF.* Code scanning would show them as alerts.
- *An `export_evidence` MCP tool, a tool parameter, or a recipe through `_meta`.* P4, and agents cannot set
  `_meta`.
- *Wall-clock timestamps.* They break byte identity.
- *An incident-tool export, forge posting or a blocking conclusion.* IM-X4, ADR-040, CX-8.
- *The record in wave 2.* P20: a general predicate waits for a second consumer.
- *A Kubernetes admission consumer for the DSSE envelope.* Policy engines check image digests.

**Consequences.**
- VB-19 (`docs/plans/verbs.md`) and CX-17 (`docs/plans/change-and-compliance.md`) are reopened `[ ]` with their
  reasons in the same change: done means proven.
- `receipt verify` stops reaching the network (EVX-6), and a private dependency needs the operator's own module
  cache, documented on the receipts page.
- The seven SARIF outputs gain provenance and stable fingerprints, and the two risk converters become one.
- An anchor can read `uncommitted` or `filtered`, which `--require-clean` refuses: an answer from uncommitted bytes
  is named, not hidden behind HEAD.
- No table, column, cache, background job, MCP tool, parameter or wire field is added. No `ExtractorEpoch` bump.

**Jev, weighed.** No site. Every field is computed or read. A model-written summary in a receipt or its Markdown is
refused: generation is the port's weak side (ADR-056), narrative is a lens (P14), and a judgment never enters a proof
surface (I62). With provider none, everything here holds.

**Revisit when.**
- EVX-V1 names the CI or review sink that consumes SARIF or JUnit (an open owner input: GitHub code scanning needs a
  paid licence on private repositories, reviewdog, Jenkins Warnings NG, the check-run payload): a JUnit sink builds
  EVX-D2, and the named sink is EVX-K1's instrument.
- The owner picks the predicate URI (Q-EVX-1, an open owner input needed only at wave 3).
- EVX-G3 records two consumers, or one the owner did not write: wave 3 starts.
- EVX-M3 reads KEEP: waves 4 and 5 start. CUT: they move to futures and the record stays as `receipt verify`'s
  input.
- EVX-K1 reads CUT (CX-M1 not PASS within 60 days of wave 1): wave 5's SARIF render is refused, and the SARIF sink
  stops counting toward EVX-G3.
- EVX-M1 reads CUT: the "without fathom" claim and the `checks` projection go, anchors and SARIF stay.
- The median receipt for one `fathom review` on this repository exceeds 256 KB, or export exceeds 2 s warm:
  excerpts become opt-in.
- EVX-M2 reads CUT for a tool: replay for that tool is withdrawn and renders "replay unreliable".
- A kit ships a code-evidence predicate, or Review Kit asks for its own: fathom adopts it and retires its own
  (ADR-063's revisit).
- An adopter or regulator rejects the evidence pack format (ADR-051's trigger).

**Amendment (2026-10-11, ADR-069 decision 4): decision 4's closed kind table grows by two.** The kinds `fact` (a
concern fact, an observation of its own file) and `join` (a cross-file join computed at request time) are added,
both at `fathom@<version>`: git proves the cited text, never the structure a parser read from it. An absence fact
carries its read set and renders as a lower bound when a read is missing (I38). No new verb, tool or format:
`fathom export receipt --kind fact|join` sits beside the four Go kinds of EVX-9.

## ADR-069: The evidence plane: concern facts and joins serve review, compliance and incidents, deploy state is a declaration, fathom instances connect by shipping index snapshots and never by calling each other, and the core reads named paths and listens on loopback (2026-10-11)

**Status.** Accepted 2026-10-11 for decisions 1 to 10 (owner-architect rulings of 2026-10-11 on the evidence-plane
assessments: D4 to D10, D13, D15 to D17 and D19, and the answer to the owner's question of 21:40Z about live
per-service instances). Decisions 11 to 15, index snapshots, are decided in shape and parked: no phase starts
until the owner prioritizes it, and no earlier than war1 landing and RT-M1 reading. Their environment daemon (a
network-reachable `serve-http --read-only --lineup`) also waits on the owner's answer to U1, which narrows the
hosted non-goal and amends law 9; nothing here decides U1. Vision: `docs/vision/44-evidence-before-the-incident.md`.
Design: `docs/design/index-snapshots.md`. Plans: `docs/plans/well-architected-evidence.md` (WAE-),
`docs/plans/index-snapshots.md` (IXS-). Goal G37. Futures F70 to F74; F3, F4, F27, F32 and F59 amended. Amends
ADR-067 decision 12 and ADR-068 decision 4. Assessments: `docs/assessments/2026-10-11-reviews-and-incidents.md`,
`2026-10-11-index-snapshots.md`, `2026-10-11-making-the-vision-happen.md`. Reads ADR-011, ADR-017, ADR-040,
ADR-041 decision 4, ADR-050, ADR-053, ADR-055 decision 7, ADR-056, ADR-058, ADR-063, ADR-064, ADR-065 decision 2,
ADR-068, I30, I56, I62, P9, P20 and SR-17 as constraints.

**Context.** The owner asked for fathom as Review Kit's evidence engine for well-architected, compliance and
incident-prevention catalogs; for fathom in incident investigation, RCA and agent-led mitigation; for indexes kept
fresh by CI/CD and shared across teams; and then whether live per-service instances, networked to their
dependencies' deployed versions, would serve the CD gate and incident investigation. Read against the tree on
2026-10-11:

- Dialect facts exist but are consumed only inside passes (`internal/fusion/alertroute.go:397,515`). Prometheus
  facts are labels only; Alertmanager facts are receiver and continue. Kubernetes per-container facts are LB-F451,
  open. Third-party call sites are not stored and Go skips composite-literal keys. IAM, Kyverno, Gatekeeper, Rego
  and Azure Policy dialects exist but join nothing to workloads or code.
- Review Kit's reliability pack has three findings, all off, with no checks, and its fathom adapter answers only
  `verify-plan`. pe's pdk/risk already tiers Pulumi previews and manifest diffs.
- The findings importer records `SourceRevision` while the product hashes cited lines from the working tree.
- Stores are portable (ADR-017) and read-only mounting is proven (GKE-MH-1), but federation over mounted stores
  covers obligations only (GKE-MH-3); code tools read the daemon's own store. Admission is loopback-only; the auth
  row F8 waited on was refused and hosted-modules mode deleted for zero use (GKE-MH-7, SR-17).
- `strictSurfaces` is not a security boundary: bare `/rpc` serves the full registry, and a drifted read queues a
  re-ingest.
- The store spec names one machine-local field, but `ingest_state.path` and `file_stats` mtime and inode are
  machine-local too.
- `ExtractorEpoch` is 62 and changed in 29 commits between 2026-09-11 and 2026-10-11; every bump re-extracts every
  file.
- No code models a deployed commit; `--deployed` is designed for incident maps and unbuilt.

**Decision.**
1. *Concern facts are observations of their own file, served by existing tools.* No table while the freeze holds
   (the ADR-064 precedent). Every fact carries ADR-068 anchors and a coverage statement (`complete|incomplete`,
   extractor, tier, read set).
2. *Secret-bearing values are never selected* (ADR-065 decision 2). Env facts are names and `valueFrom`
   references, never literal values, except an allow-list of non-secret retry keys; arg facts are flag names with
   values withheld, except an allow-list of heap flags. A planted sentinel appears in no store row (WAE-13). This
   is ADR-067 decision 12's amendment.
3. *Joins are computed at request time, never stored* (ADR-058 decision 2): probe to handler to network callee;
   workload to monitor to alert to receiver; retry and timeout along a route chain; IAM grant to service account to
   workload; public route to handler; workload to the policies that select it.
4. *Facts and joins leave fathom as receipts.* ADR-068 decision 4's closed kind table gains `fact` and `join`,
   both re-derivable at `fathom@<version>`: git proves the text, never the structure. An absence fact carries its
   read set. No new verb, tool or format.
5. *Review Kit judges; pdk/risk tiers; policy-kit rules.* Catalog, severity, the library-defaults pack,
   pass/fail, findings and admission stay in Review Kit (ADR-040, ADR-055 decision 7, ADR-063 O2). Change-risk
   tiers stay in pe's pdk/risk, which may take fathom concern deltas as named reasons. Resource and security rules
   and framework crosswalks stay with policy-kit (O11). Fathom never grows a deploy-gate or readiness verb. The PR
   ratchet (concern deltas against the merge-base) is the default delivery.
6. *The deploy gate is fail-open on fathom.* A fathom outage, or a missing or stale index, admits with a recorded
   exception; a rollback to an evaluated commit and a pdk/risk T3 break-glass admit; the evaluation follows.
7. *Deploy state is a declaration.* A per-call `--deployed/--previous`, a GitOps repository indexed as a service,
   or an operator-named image provenance file (image-kit's bake path; the apko path records none). Nothing about a
   deploy is stored; attach stays refused (IM-X2). A lineup the operator's deploy job writes, read once at a
   daemon's start, is a further declaration in decision 14's shape; whether a lineup change stays clear of IM-X2 is
   the owner's ruling, and if not, phase 5 waits on a new ADR.
8. *Incidents.* Fathom is a read-only context source for an external investigating or mitigating agent through
   its existing tools. It never executes, proposes or records a mitigation (IM-X3, IM-X9). A rollback dimension of
   `review --since` extends incident maps 6.3's removed-commit list and waits on IM-M0 BUILD.
9. *Instances connect by shipping immutable commit-pinned index snapshots, never by query-time RPC between
   daemons, until one of F3's amended triggers fires.* An index snapshot is exported on merge, published by the
   operator, and named by a lineup on deploy; a lineup change rolls the environment daemon to a new pod on the new
   lineup. Cross-service joins run in the one process that holds the snapshots. Query-time federation between live
   daemons is reconsidered only when (a) a service's imported dependency closure exceeds one reader's measured
   budget (IXS-M7: daemon RSS or disk), (b) a dependency's owner refuses in writing to let its snapshot leave its
   boundary, or (c) an answer class needs on-demand compute that one process holding the imported snapshots cannot
   run, and a reading shows the precomputed fact sheet misses it. Even then: no service fathom operates, no
   multi-tenant or writable daemon, no auth inside fathom, no fetch, no stored deploy record, no route around the
   operator's proxy, and no control plane over the set of daemons.
10. *Ports and adapters: fathom's core reads operator-named paths and listens on loopback; every network,
    credential or signature adapter belongs to the operator's recipe.* The registry pull, the authenticating proxy,
    the signer and its verification, and the deploy job that writes a lineup are the operator's. This extends I56
    (no download on fathom's own authority) and ADR-068 decision 7 (the operator's signer signs) from dependencies
    and receipts to every index snapshot and every serving mode. Fathom never binds a non-loopback address, holds a
    key, verifies a signature or lists a registry.
11. *Index snapshots, not a hosted store* (parked). `fathom export snapshot` copies a store with `VACUUM INTO`,
    clears machine-local fields (IXS-0, a defect fix that rides war1), and records and prints its cache key
    (repository identity, commit, fathom version, ExtractorEpoch, TypedEdgeEpoch, format version, config digest,
    env.lock digest). An index snapshot is a conforming store, not an expert module and not a SCIP index. Fathom
    ingests with `--cache-from` a file the operator names in that invocation (I56), proves a cache-from ingest
    equals a cold one before any snapshot is published (IXS-M0b), and imports an operator-named snapshot for code
    reads (`import snapshot`, gated on IXS-M1 KEEP and a named consumer).
12. *Refs are `--cache-from` services until measured otherwise* (parked). A PR or long-lived branch is ingested
    from its merge-base's index snapshot; a worktree stays its own service (I30 unchanged). An overlay store keyed by
    ref waits on IXS-M5 (F70).
13. *Every answer carries a basis* (parked). The basis names each `service@commit` read, its cache key, and the
    lineup and digest when one is served, with one state: `exact`, `differs`, `unchecked` or `uncommitted`. A caller
    names an expectation by the `X-Fathom-Expect-Lineup` or `X-Fathom-Expect-Commit` header or the CLI's
    `--expect-commit`, never through `_meta` and never as a tool parameter. A gate accepts `exact`, or `differs`
    when its own per-service comparison passes. `behind N` is computed on the caller's side with an operator-named
    snapshot list; a daemon never lists a registry.
14. *Read-only serving* (parked). `serve-http --read-only` opens snapshots with `store.MountReadOnly`, turns
    auto-refresh off with no refresh queue, refuses every write tool, removes `/reload`, turns off the wiring apply
    and undo endpoints, holds the decision port at provider `none`, and hard-gates or disables bare `/rpc`. With
    `--lineup` it composes its own store from the lineup's files at start and has no control channel. On loopback
    this needs no owner answer (a read-only sidecar); a network-reachable environment daemon behind the operator's
    authenticating proxy waits on U1 and the law 9 amendment.
15. *Retention serves rollback and replay, never evidence* (parked). Deployed commits' index snapshots are kept
    under a cap (owner ruling D13: 90 days after a later lineup moved off them, then one per service per month) for
    rollback gates and incident replay. A compliance or point-in-time claim leaves as an ADR-068 receipt re-derived
    at that commit; keeping snapshots as evidence would need an ADR-068 amendment.

**Alternatives rejected.**
- *A hosted service operated by fathom.* Bus factor of one, tenancy and SLA (vision 07 R1).
- *Auth inside fathom (a bearer token).* Built and deleted for zero use (SR-17); the operator's proxy carries it.
- *Query-time RPC between live daemons.* Files travel; daemons do not call each other (decision 9). It fails when
  a dependency is degraded, which is when an incident needs it.
- *Federating code tools over mounted stores.* Re-adds a read path SR-17 deleted and touches every tool's fan-out;
  `import snapshot` keeps every tool unchanged.
- *An overlay store now.* A compatibility-surface change before the cache-from path is measured.
- *A deploy ledger fed by CD.* IM-X2's attach under another name.
- *A fathom change-risk gate or readiness verb.* pdk/risk ships tiers and floors; ADR-063 O2.
- *Kubernetes or security rules in a fathom tech pack.* ADR-063 O11.
- *Fathom as the findings record.* ADR-040 and Review Kit's ADR-0098 bars are unmet.
- *A new reliability finding class.* Review Kit tags rules with its reliability domain and maps them onto the
  existing ten classes.
- *"Fathom workspaces" as the name.* `workspace/` is fathom's scratch directory, and the word means a mutable place
  in Go, Terraform, npm and Bazel; "index snapshot" is already fathom's word (I56, F27, ADR-068).

**Consequences.**
- One store format, one evidence contract, no new table, no new MCP tool (ADR-053). When index snapshots start:
  two verb forms (`export snapshot`, `import snapshot`), one ingest flag (`--cache-from`) and one serve mode
  (`--read-only --lineup`), each gated by its phase.
- Cache sharing works only when producer and consumer lock the same fathom release (env-kit `env.lock`).
- Review Kit, pdk/risk, image-kit, env-kit and the operator's publish and deploy jobs carry the rest; fathom's value
  depends on Review Kit's reliability catalog landing.
- Declared state is not live state: drift stays an attested artifact, and unattested drift is `incomplete`.
- G37 covers the WAE waves and IXS-M0 only. Index snapshots get their own goal (provisionally G38) when the owner
  starts phase 1, so G37's kill never fires on parked work.
- The 2026-10-11 ruling moved ops formats from lf7 to lf8 (vision 44 section 6); WAE wave 2 rides lf8.
- No `ExtractorEpoch` bump is required by this record.

**Jev, weighed.** Helps at the judged rows only: hand-rolled retry loops, health-pool separation, alert
investigative value and route auth middleware, as opt-in judged sites under ADR-056 that propose and never decide.
Not at facts, joins, cache keys, the answer basis, parity, lineups or gates, which are computed. Band policy (P24):
a site serves only inside the calibrated band where its Wilson lower bound is at least 0.90; outside it the answer
is "not judged" and coverage `incomplete`. With provider none, the default, everything here holds and the judged
rows read `incomplete`; a read-only daemon holds the port at none. Cost: model spend, held since 2026-09-29.
Measurement: a Wilson lower bound of at least 0.90 against WAE-M0's labels, and cost per true positive under the
owner's bar; below either, the site stays unbuilt.

**Revisit when.**
- WAE-M2 or a WAE-M3 or WAE-M6 join reads CUT: that fact family or join is removed.
- WAE-M2 finds the sentinel in any row: the family is blocked until fixed.
- WAE-M1 reads BUILD: code client-construction facts move from F71 to the plan.
- Review Kit's reliability domain declines fathom evidence: WAE stops at wave 2.
- 2027-01-31 with WAE-M4 unread or CUT: WAE waves 3 to 4 are dropped (G37's kill).
- IM-M0 reads BUILD: the rollback dimension is built. IM-M1 shows git-invisible causes at 20% or more: attach is
  reconsidered through ADR-058's own trigger.
- Any of F3's amended triggers fires: query-time federation between daemons is reconsidered (decision 9).
- The owner answers U1: yes opens phase 5 under F74's Promote when; no keeps index snapshots to local daemons and
  read-only sidecars. The owner rules a lineup change is attach: phase 5 waits on a new ADR.
- The owner starts index-snapshot phase 1: its goal is opened with the next free goal id.
- IXS-M0 reads cache-from above 0.6x cold, or IXS-M0b reads a mismatch unfixed after two waves: `--cache-from` and
  `export snapshot` are deleted under a `gone:` row, and F27 stays deferred.
- IXS-M1 reads above 0.6x cold, or IXS-M2 under 0.2: sharing stops and F27 stays unpromoted. IXS-M2 under 0.5 with
  the release locked: per-extractor epochs (F73).
- IXS-M3 has no read-only sidecar session recorded with a law 6 stamp file within 60 days: `--read-only` and its
  recipe are deleted with a `gone:` row (the SR-17 precedent).
- IXS-M5 reads KEEP: an overlay-store ADR (F70). RT-M1 reads PASS: SG-2 and versioned cross-repo joins (F72).
- A second organization asks in writing to share one daemon: the multi-tenant non-goal is reconsidered.

## ADR-070: fathom is one self-contained module; the kit mirrors are internal packages by import-path rewrite (2026-10-16)

**Status.** Accepted 2026-10-16 (owner decision Q1, delegated to the owner-architect: "do what you recommend, as
long as you are not deleting any valuable work"). Amends ADR-019. Plan: `docs/plans/distribution.md` (DS-).

**Context.** fathom's `go.mod` required `github.com/geoffmilleraz/agentic-kit` and
`github.com/geoffmilleraz/app-kit`, both private, and replaced them with `./third_party/agentic-kit` and
`./third_party/app-kit` (ADR-019). That builds a clone, but nothing else: `go install
github.com/geoffmilleraz/fathom/cmd/fathom@<tag>` refuses any module whose `go.mod` carries a replace, and `go get
github.com/geoffmilleraz/fathom/pkg/client@<tag>` ignores fathom's replaces and asks the proxy for the private
paths, which 404. `pkg/client` itself takes kit types (`WithDecision(decision.Decider, ...)`,
`WithLLM(llmclient.LLMClient, ...)`). The kit source is already public inside fathom's tree, so making it part of
fathom's module exposes nothing new; making a kit repository public was not on the table.

**Decision.**

1. fathom is one module. Each mirror is a plain directory of it: the kit's `go.mod`, `go.sum`, `go.work` and
   `go.work.sum` are kept as `<name>.upstream` (content intact), so the directory is no longer a nested module. The
   kits' own nested modules (agentic-kit's `kits/docextract/{pdf,xlsx}`, app-kit's `data/design-assets` and
   `spikes/wails-gallery`, testdata modules) keep their `go.mod` and stay outside fathom's module and its zip.
2. Every import of a kit's upstream path is rewritten to `github.com/geoffmilleraz/fathom/third_party/<kit>/...`:
   in kit code and in fathom's code. `cmd/kitrewrite` (`internal/kitrewrite`) does it deterministically: only
   import path literals, found by `go/parser` in `.go` and `.templ` files, never `testdata/`, a nested module,
   a dot or underscore directory; nothing is reformatted, so `-reverse` restores the upstream form byte for byte
   (DS-2). Comments and strings that merely name a kit stay as upstream wrote them. First run: 802 kit files and
   201 fathom files, 1,912 import literals, 5 module files moved.
3. `go.mod` requires no kit and carries no replace or exclude. app-kit's requirements are merged at app-kit's own
   pins (never lowering a fathom pin; no existing fathom requirement moved): 37 new requirements, none linked by
   `./cmd/fathom` or `./pkg/client`, all needed by app-kit's CLIs, TUI and their tests, which are now packages of
   this module and must build.
4. The pin is each mirror's `.vendor-commit`, its single pin. `task sync-kit` archives a sibling at its HEAD (or
   `scripts/sync-vendor.sh <kit> <commit>` at a named commit), rewrites the copy, replaces the mirror and stamps
   the commit; `task vendor-check` copies each mirror, reverses the rewrite and compares content with the archive at
   `.vendor-commit`, and fails a mirror still holding its own `go.mod`. `scripts/lib/kit-pin.sh` (go.mod pin
   parsing) is removed.
5. fathom's gates never format, vet or test kit code: every `./...` in `Taskfile.yml` is
   `go list ./... | grep -v /third_party/`, and lefthook's gofmt skips `third_party/`. Kit code is built
   (`go build ./...`, `clean-clone-check`); kit tests run in the kit's own CI, as before.
6. `TestRepositoryIsSelfContained` (`internal/kitrewrite`, DS-1, in the gate) refuses a replace, an exclude, a
   require under fathom's own owner path or of a mirror's upstream path, a mirror with its own `go.mod`, and any Go
   or templ file importing a kit by its upstream path. `go run ./cmd/kitrewrite -check` prints the same.
7. Live co-development through a local `go.work` with `use ../agentic-kit` stops applying: fathom's imports no
   longer name the kit's module, so a workspace module of that path satisfies nothing. The path is the sync:
   commit in the sibling, `scripts/sync-vendor.sh`, build.

**What changes in ADR-019.** The `require` + `replace` pair and the `go.work`-over-replace precedence are gone; the
pin moves from `go.mod`'s pseudo-version to `.vendor-commit`; the mirror is the pinned archive in the rewritten
form, not byte-identical to it, and vendor-check compares after reversing the rewrite. **What stands.** Full
in-tree mirrors of the pinned commit, nothing excluded; the content-hash drift guard that skips without the
sibling; `clean-clone-check`; exclusion from fathom's fmt and cue-vet scans; `go mod vendor` standalone; never
making a kit public and never editing the kit repositories from fathom.

**Consequences.** `go list -deps ./cmd/fathom ./pkg/client` names no `geoffmilleraz/agentic-kit` or `app-kit`
path; a module zip check (`golang.org/x/mod/zip.CheckDir`) reads 10,346 files, no invalid path, 92.6 MiB of the
500 MiB limit, with every file `cmd/fathom` and `pkg/client` compile or embed included. A `pkg/client` consumer that
passed kit values imports them from fathom's path (Breaking, CHANGELOG). fathom's own ingest of itself now reads the
kit packages as packages of the root module. `usedset` accepts a mirror that is a plain directory of the consumer
module. Kit tests that assert their own upstream module path may fail if run from fathom; fathom does not run them.

**Rejected.** Publishing either kit (the owner keeps them private). A `go.work` or vendoring for consumers (`go
install pkg@version` reads neither). Copying kit packages under `internal/` (moves and renames kit code, so drift
detection against upstream breaks; the rewrite keeps every path and byte but the import literals). Rewriting with
sed (it would also rewrite strings and raw-string fixtures and could not be reversed exactly).

**Revisit when.** Either kit is published publicly: requiring its tagged module replaces the rewrite for that kit
(`-reverse` for its imports, drop the mirror or keep it as a plain archive). A third kit is mirrored
(`kitrewrite.MirrorNames` grows). DS-M1 reads FAIL: the install proof names what still blocks a stranger.

**Decision 8 (2026-10-16, DS-4): app-kit's mirror holds only fathom's build closure, computed by
`scripts/kit-closure.sh`; history is not rewritten.** The owner asked to clean the TailwindPlus material out of
fathom where that is not too difficult or risky. Since this ADR the mirror is part of fathom's module, so every
module zip the Go proxy caches (forever) would carry whatever the copy holds: app-kit's design-asset data (its
`data/tailwindplus` manifest, sha256 index and golden queries), its docs, spikes and other apps. The mirror now
keeps only what fathom builds, never a hand list: the app-kit packages in `go list -deps -test` of fathom's own
packages and in `go list -deps` of app-kit's `cmd/appkit` (which `scripts/ui-css.sh` builds), over five fixed
GOOS/GOARCH pairs, with each package's non-test files and `go:embed` targets; every mirror path a non-Go file
outside `third_party/` names (ui-css.sh's Tailwind sources, `appkit.adoption.json`, docs), a name the mirror lacks
failing; the identity and provenance files and `fathom-pack/`. The list is committed as `third_party/app-kit.keep`;
`task sync-kit` reduces a fresh copy to it (computed against the full rewritten copy, so a newly reached package
is found), `task vendor-check` compares only its paths in upstream form, and `task scripts-check` fails when the
manifest and the computed closure part or the mirror holds anything else. First run: 1,806 files to 317 (19 MB to
2.6 MB on disk), the module zip 92.7 MiB to 80.2 MiB, and `go mod tidy` dropped nine requirements only the removed
apps and CLIs used. Twelve kept paths still name TailwindPlus or MatDash: `cmd/appkit`'s own Go code
(`internal/twplus/{embed,index,normalize,parse,schema}`, its schema CUE, `internal/matdash/walk.go`), no data.
agentic-kit's mirror stays a full copy. This amends "What stands: full in-tree mirrors, nothing excluded" for
app-kit. History is not rewritten, for two reasons: the licensed TailwindPlus dump was never committed (the
largest blob in all history is 5.4 MB), so a rewrite would remove no licensed corpus; and a rewrite would
force-push public `main`, move every tag and release, and break the commit SHAs fathom's own evidence cites.
**Revisit when** app-kit offers a narrow CSS entry point (Tailwind build and token generation without the
design-asset tooling): `cmd/appkit`'s TailwindPlus and MatDash code then leaves the closure.

## ADR-071: Readouts: fathom shows the agent a command's output as a compact, enriched or raw readout with a receipt; a view of tool output, never a command runner (2026-10-16)

**Status.** Accepted 2026-10-16 (owner direction of 2026-10-16, "build a more powerful version of it ... extensible
by users ... different modes ... decide, plan, spec", decided by the owner-architect; Q-RO-1, Q-RO-3 and Q-RO-4, and
Q-RO-2 for the owner-architect's wave sessions, answered 2026-10-16 by the owner-architect under the owner's
delegation, `docs/intent/questions.md` Q35). Design: `docs/design/readouts.md`. Plan: `docs/plans/readouts.md` (RO-).
Goal G39. Amends ADR-042 by one opt-in site name and nothing else, and I60 for that site's scope and disclosure
(decision 8); narrows the harness port's §5.3 inject-only contract by one bounded output, the way ADR-038 did for
deny, and by the same output ADR-056 decision 13, which refused output rewriting and command execution by fathom
(R-12 in `docs/assessments/2026-09-28-jev-ten-levels.md`; named 2026-10-16 with the seam map,
`docs/design/readouts.md` §20.4): decision 2's view is allowed on decision 3's terms, lossless by default and lossy
only with a recovery and a receipt, and command execution by fathom stays refused. Reads ADR-063's overlap rule (applied here to an external tool), ADR-061 (proposed), ADR-042's SEC-1 and SEC-2
amendments, I8, I9, I14, I35, I38, I42, I53, I55 and I61 as constraints.

**Context.** Bash carries 92% of this repository's tool-result bytes over 14 days (F75's evidence,
`docs/intent/futures/f74-f79.md`). A 30-day reading of every project's local transcripts on the owner's machine
(a throwaway script, aggregates only, not committed; RO-M0 re-reads it with a committed verb), deduplicated by
tool-use id: 145,825 Bash calls and 334.9 MB of result text, each result capped near 30,000 characters by the
harness. Shell reads of a source file (`sed -n`, `cat`, `head`, `tail`) are 37.2% of those bytes and reads over 200
lines 10.6%; grep 16.5%; `ls` 6.8%; `git diff`, `show` and `log` 5.3%; failures 2.7% of calls and 1.2% of bytes;
exact repeats 0.03%. Six deterministic strategies applied without overlap remove at most 12.3% (41.2 MB), and 69%
of that rests on the unvalidated assumption that a long read needs only 60 lines; without it, about 3.9%. rtk
(rtk-ai/rtk; CLI Apache-2.0, engine and proxy BUSL-1.1) rewrites the agent's command to run inside its own wrapper
and filters the output; its `discover` figure for this machine (41.9M tokens) multiplies a static per-rule
percentage by 351,726 calls that include copies of resumed sessions, so it is an estimate, not a saving (P12). Its
open defects cluster where the wrapper runs the command: silent caps (#2204), masked exit codes (#3267, #3492),
failures summarized as benign (#2317), `;;` broken by the lexer (#3197), machine-bound output corrupted (#2487),
and a permission verdict computed on a command other than the one that ran (#3970); an independent evaluation read
16% more cost per passing run with it on (#3492). Fathom today rewrites nothing: `HookOutput` carries context,
block and deny only (`internal/harness/codec.go:92-112`), and the test-run hook is inject-only
(`cmd/fathom/testrunhook.go:17`). PostToolUse(Bash) carries the output text capped at 30,000 characters, larger
output persisted and named by `persistedOutputPath`, and no exit code; an unpiped failure arrives on
PostToolUseFailure led by `Exit code N` (`docs/bench/readings/test-run-hook/hook-facts.md`, Q32). A graph call on
the hook path is abandoned on this index today (EW-75: localize 385 of 437 at its 2 s cap,
`docs/plans/efficiency-wave.md`).

**Decision.**

1. *The capability is named readouts.* For a Bash command the harness already ran, fathom may give the agent a
   readout in place of the output: compact, enriched, or the raw output, always ending in a receipt line that names
   which, the rule and version, what was dropped by class, and how to see the raw. The rules are readout rules.
2. *A view, never a runner.* Readouts ride PostToolUse(Bash) and replace only what the agent is shown of a successful
   result (Claude Code's `updatedToolOutput`, carrying every field of Bash's output envelope, `stdout`, `stderr`,
   `interrupted` and `isImage`, copied from the input with only `stdout` and `stderr` replaced, because a value that
   does not match the schema is ignored and the original output used). The command, its input, its exit status, its
   permission verdict and every file stay untouched: no PreToolUse rewrite, no `updatedInput`, no wrapper verb that
   runs a command. A failure (PostToolUseFailure, or an interrupted run) is never replaced; rich mode may add one
   injected line through the existing vehicle, inside I35. The §5.3 inject-only contract gains this one bounded
   output: a `HookOutput` view field the Claude Code codec encodes on PostToolUse only; every other codec and event
   refuses it and the verb stays silent (ADR-038's deny pattern).
3. *One contract for every readout, whichever rule or scope made it (I38).* (a) Protected lines are never dropped
   from a diagnostic stream: stderr, and the stdout of every command outside the classifier's closed content table.
   There error, FAIL, panic, fatal and warning lines and a `file:line` frame are protected. The content table is
   fixed in code and never set by a rule: reads of a source file, `grep`, `rg` and `git grep` matches, `ls`, and `git
   log`, `show` and `diff` bodies. In their stdout a word such as `error` or a `path:line:` prefix is data, and
   dropping it is ordinary lossiness, counted in the receipt and recoverable. `Exit code N` and the harness's
   truncation or persisted-output notice are protected in every stream. A rule that would drop a protected line
   serves the raw output, with the trip named in the receipt. (b) Never worse: a lean readout that is not smaller
   than the raw is not served. (c) Lossiness is computed from the rule's stages (none, columns, lines, whole), never
   declared by its author; a lossy rule names a recovery or is refused at load: a repeat (the same command again
   within five Bash calls of its lean readout in the session is served raw, with no prefix), a `sed -n` span recipe,
   the harness's persisted-output path, or a saved file (Q-RO-1). The recovery is never a prefixed command: Claude
   Code's allow rules do not match past an assignment of another variable (its permissions documentation,
   "Wrappers"), so a `FATHOM_READOUT=raw <cmd>` recovery would prompt in Manual mode, the permission defect decision
   2 refuses a rewrite for. (d) Output bytes are deterministic (`internal/AGENTS.md`). (e) Machine-bound commands
   stay raw: a pipe into anything but a display-only tail, a redirect, a heredoc, a subshell or `bash -c`, a
   background job, `sudo`, and the flags `--porcelain`, `-z`, `--json`, `--name-only`, `-c`, `-l`, `-L` and `-o`. (f)
   The classifier refuses what it cannot read, and the command stays raw.
4. *Local and deterministic on the hook path.* Readouts make no daemon call in phases 0 to 2. Joins are a closed
   built-in table of in-process, read-only functions: an outline of one named file from fathom's own extractors,
   the HEAD stamp (`cmd/fathom/edithook_pinstamp.go`), the previous run's failing ids from the hook log, and the
   changed-file list through the read-only git wrapper. Daemon joins wait on EW-76 and the tenancy budget fix; a
   daemon join that misses its deadline reads "index: not consulted (deadline)" in the receipt, never as nothing
   found.
5. *Modes.* `off` (the default), `observe` (compute and log, emit nothing), `lean` (compact successes), `rich` (lean
   plus enrichment; the policy table chooses raw, lean or enriched per call, and an enriched readout may cost more
   tokens than the raw). A `FATHOM_READOUT=raw`, `lean` or `rich` prefix is honoured when a command carries it, but
   no receipt or document asks the agent to write one (decision 3c's permission reason). `[readouts] mode` and every
   per-rule mode are user-only (`internal/config/scope.go`): a repository cannot change what the agent is shown. A
   rule reaches lean only on its own evidence (decision 9); `fathom readout report` recommends, the user's config
   decides (P11).
6. *Rules: TOML authored, CUE validated, fixtures mandatory.* Built-in rules are embedded files in the user
   format, so the built-ins exercise the extension surface. User rules live in `~/.fathom/readouts.toml` and
   `~/.fathom/readouts.d/`. An embedded CUE schema validates them (CUE validates and never authors or executes:
   the non-goal, I17). A rule is a token matcher (argv0, subcommand, flags present or absent, and a compound shape:
   simple, a display-only pipe tail, or after a quiet prefix from a closed list the user may extend), conditions
   (bytes, lines), stages from a closed set (strip ANSI and carriage-return progress, collapse repeats, keep, drop,
   group, fields, cap, extract records, summarize records, join, outline) and inline fixtures. A rule whose
   fixtures fail is not served and is named by `fathom doctor`. Typed records (`test_failure`, `compile_error`,
   `match`, `read_span`, `diff_hunk`, `path`) are the extension point: an author maps output to records, and
   fathom supplies the joins and the rendering.
7. *No command calling from a rule, a map or a pack, at any scope.* No rule field names an executable, an argv,
   shell text or a URL; the schema is closed. A rule may render a command as text (a recovery or a next step) and
   never runs it (F62's pattern). Maps stay anchor maps (`internal/explmap/model.go`). The boundary is SEC-1 and
   SEC-2's (no hook runs a command a repository names; `cmd/fathom/sec2_scope_test.go`), I8, F62's do-not-build,
   ADR-061's proposed invariant (a), the dispatch-loop non-goal and I53's deadline. The owner's question, "should
   mappings support command calling", is answered no for this program, with a revisit trigger below.
8. *The decision port: one opt-in site, `readout_mode`.* A `choice{raw, lean, rich}` asked only in rich mode, only in
   the policy table's contested band, only through a loopback daemon endpoint the hook calls under its deadline (the
   `POST /route` precedent, I14), over features only: class, byte and line counts, record counts by kind, overlap
   with the session's edited files, the class's measured recovery rate, and whether the session is a sub-agent; never
   the output text (I61). With provider `none` the table decides, byte-identical (I42). The site is named in user
   scope only, unlike the repository-named opt-in sites, because readouts' mode is user-only; I60 is amended for it
   (a repository's `[decide] sites` naming it is dropped, and a miss is disclosed in one receipt clause, or in the
   hook-log row and the report when the result stays raw, since a raw result carries no fathom text). It has no call
   site until plan phase 4, which needs the owner's budget lift, ADR-056 R3's bar (a 95% Wilson lower bound of at
   least 0.90, labels computed from the logged outcomes) and instrument C's A/B.
9. *Evidence per rule, never a dashboard.* Each readout writes one row to the existing hook log on a `readout`
   channel (rule and version, class, mode, bytes in and out, dropped lines by class, protected trips, milliseconds;
   never output text), the test-run hook's precedent with no table. `fathom readout report` joins bytes removed to
   harm: the dropped-line reference rate (a later call in the same session, within five, names a path, a `path:line`
   or an identifier that appeared only in dropped lines; computed by replaying the row's call from the local
   transcript with the rule its `rule_sha` names, since the row holds no text, and a hit it cannot replay is counted
   apart, never as clean) and the recovery rate (a repeat of the command or a re-read within five). Bytes removed are
   bytes, not tokens saved; a token figure is bytes/4, labelled an estimate; a saving is claimed only from instrument
   C (P12). Promotion to lean: at least 200 hits and a 95% Wilson lower bound of at least 0.90 that no dropped line
   was referenced (R3's pruning bar). Demotion: a lean recovery rate whose Wilson lower bound exceeds 5%. Every
   reading over the hook log asserts its window from its first row and reads INCOMPLETE when short: the log is
   user-global and rotates at 64 MB keeping one prior generation (`cmd/fathom/beats.go:22-35`). A count over output
   the harness cut near 30,000 characters is taken from `persistedOutputPath` when present, else labelled as covering
   only the shown part, never a total.
10. *Repository and pack scopes later, add-only.* A repository's own readout rules and a pack's `readouts` key wait
    on ADR-061's acceptance and FD-1 to FD-6. When they come they may extract and add, never drop or replace, and
    they run in observe until the user lists the file's sha256 in user scope (I55's pattern).
11. *rtk: keep, separate, with a boundary (ADR-063's rule; no delegate, no merge).* Fathom does not adopt rtk, and
    never reads, writes, imports or ships rtk configuration (owner, 2026-10-16: the filters-file item is removed). A
    command that starts `rtk ` and an output carrying rtk's recall marker stay raw: one rewriter per command, and two
    lossy views are never stacked. Whether PostToolUse's `tool_input` carries a PreToolUse rewriter's command or the
    original is UNVERIFIED (RO-M2 reads it), so lean and rich emit nothing at all while `fathom doctor` sees another
    Bash rewriter wired; the `rtk ` guard is the second line, not the first. `fathom doctor`'s readouts row names
    another Bash output rewriter when one is wired. No rtk code is ported. The ideas the design credits (a
    never-worse guard, lossy and recovery declarations, trust by content hash) are re-derived from fathom's own
    specification; rtk's BUSL-1.1 engine and proxy are not read for design.
12. *Harnesses.* Replacement is Claude Code only. Codex and Cursor output replacement is unverified, so their
    codecs refuse the view and `fathom doctor` names them unsupported, never silently. The engine is a pure function
    over the command's tokens, stdout, stderr, the interrupted flag and the harness cap, so the Claude Code mods
    program (next after readouts) can host it behind a `tool.call` vehicle (F78) without a second engine.
13. *Priority.* Readouts is the next priority program after the RT-M1 reading; the Claude Code mods program follows
    it (owner sequence, 2026-10-16; `docs/vision/38-proof-layer-roadmap.md` §9.6).

**Non-goals.** None is amended; each is respected, and the three with a reading to record carry a dated note in
`docs/intent/scope/non-goals.md`.
- *No transformation or migration engine (Revisit when: never).* Does not fire. A readout changes what the agent
  is shown of a command's output; it never writes code, a file, a command or a command's input, and the trap the
  non-goal names (vision `01` §3) is a code-transformation engine. Q-RO-3 answered yes 2026-10-16 (the
  owner-architect, under the owner's delegation): readouts are a view of displayed output, not the code or
  migration transformation engine; a reversal stops the program at observe and inject-only enrichment.
- *No grep-blocking (≥30% of greps index-answerable, then only as a hint).* Does not fire: bare-identifier greps
  are 13.5% of 29,643 greps, an upper bound, not an index-answerable count. A grep always runs and its hits are
  shown, grouped losslessly and capped only with exact totals and a raw recipe; an index hint beside a grep stays
  F75.
- *No dispatch loop (never).* Respected by decision 7.
- *CUE is never an authoring surface or an execution engine (never).* Respected by decision 6.
- *No net tool growth (never).* CLI verbs only (`fathom readout replay|explain|test|report|learn`); no MCP tool.
- *No generic token-usage dashboard.* Respected by decision 9: bytes per rule joined to harm, no token totals.

**Jev, weighed.** It helps at one point: the contested band of rich mode's per-call choice (failures and long
reads, at most about 4.5% of Bash calls on the corpus), where a typed choice over features could beat the table.
Not in the hook process (I14), not over output text (I61), not on the default path (I42), not before the budget
lift. Cost: model spend per contested call (tokens per call UNVERIFIED) and latency inside the hook's 4 s deadline
(provider p50 UNVERIFIED; `docs/bench/readings/decide/provider-smoke-2026-10-09.md` is a smoke). The measurement
that proves or kills it: R3's calibration from logged outcomes, then instrument C's arms (provider `none` against
`typesafe`); ADR-042's "no gain at n ≥ 3" turns the site off.

**Owner questions, answered 2026-10-16 by the owner-architect under the owner's delegation
(`docs/intent/questions.md` Q35).** Q-RO-1, a count- and byte-bounded raw-output file under the state directory is
outside the table freeze: yes, when the hook process writes it (never the daemon), because the freeze guards daemon
memory; the test and build classes recover from it in phase 1 (RO-22), and RO-36 bounds the test-run hook's
existing saved outputs, which are unbounded today (sha-named files under `workspace/tmp/test-run`,
`cmd/fathom/testrunhook.go:63`, `:250-261`). Q-RO-2, lean in real sessions for the classes RO-M0 keeps: yes for the
owner-architect's wave sessions when phase 1's rows land (`[readouts] roots`, RO-20), so RO-M2 reads the channel from
those transcripts with no bench spend (lean may raise a session's own spend, as rtk #3492 read); the owner's own
interactive sessions stay an open yes or no for the owner. Q-RO-3, readouts are a view of displayed output and not
the transformation engine the non-goal refuses: yes. Q-RO-4, lift the budget hold for instrument C's readouts A/B:
no for now; the only model budget is up to $10 on OpenRouter for Jev, so phase 4 waits.

**Alternatives rejected.**
- *A PreToolUse rewrite to a wrapper that runs the command (rtk's design).* Fathom would run commands (SEC-2) and
  tests (ADR-043 decision 5); Claude Code evaluates permission rules against the rewritten input, so an allow rule
  for `git` stops matching; and the exit-code, stdin and lexer defect class comes with it. Revisit only for a class
  whose measured saving needs a flag injected.
- *Adopting rtk, or shipping rtk filters for fathom's own output.* Refused by the owner, 2026-10-16.
- *A model decision on every tool call.* I14, I53, the budget hold, and route's calibration (45.7% right at
  confidence ≥0.9, ADR-042's 2026-09-24 amendment).
- *Command-calling joins or map steps.* Decision 7.
- *A recall store (a table of full outputs).* The table freeze; recovery is a repeat, a span or the harness's own
  persisted path; a stored index waits on RT-M1 PASS and a measured need.
- *Exact-repeat dedupe.* 0.03% of bytes.
- *A regular expression over the raw command string.* rtk #3838 and #3197; a token matcher instead.
- *Reusing the `compact_advice` site.* It names context-compaction advice (F38), a different question.
- *Substituting a symbol answer for a grep.* The grep-blocking non-goal.

**Consequences.** One new hook verb (additive to the compatibility surface's hook verbs), one `HookOutput` field,
one `[readouts]` config block whose fields are user-only, one embedded CUE schema, the `fathom readout` CLI verbs
and one hook-log channel, plus one count- and byte-bounded saved-output directory under the state directory that
the hook process writes (Q-RO-1; never the daemon). No table, MCP tool, cache, background job or model spend before
plan phase 4. The
test-run hook folds into the readout hook as its rich stage when both are on, so PostToolUse(Bash) runs one fathom
process (ADR-063's merge, internal). Four shell splitters (`splitShellSegments`, `shellClauseFields`,
`bashObservedReadPath`, `firstCommandToken`) merge into one command lexer. While a readouts row is runnable, no
other program's wave starts except the RT-M1 reading and defect fixes (roadmap §9.6).

**Revisit when.**
- RO-M0 reads CUT on the program bar (the classes that pass the harm bar would remove under 4% of Bash bytes): the
  lossy track stops at lossless stages and rich enrichment, and the mods program becomes the main line (its phase 1
  starts once readouts phase 0 lands, whatever RO-M0 reads; roadmap §9.7).
- RO-M0 reads whole-file source reads over 200 lines under 3% of Bash bytes: the outline class is dropped.
- RO-M1 reads added hook p95 over 100 ms or fail-open over 1%: lean waits for the fix or for the mods vehicle.
- RO-M2 shows a replaced output losing its exit text or `is_error`, or the harness rejecting the field:
  replacement stops and rich stays inject-only.
- A rule's lean recovery rate fails decision 9's demotion bar: that rule returns to observe.
- Instrument C reads tokens per passing task not lower, or the pass rate lower (rtk #3492's result): lean and rich
  stay off by default for every adopter, and every saving line is retracted.
- Three written cases where no built-in join supplies the data a rule needs: a user-scope, fixed-argv,
  sha256-trusted, deadline-bounded exec join is reconsidered, never from a repository.
- A harness documents an output-replacement field and a live probe verifies it: its codec gains the view.
- ADR-061 is accepted: phase 5 (repository and pack rules, add-only) is scheduled.
- The owner reverses Q-RO-3: the program stops at observe and inject-only enrichment.
- The owner answers Q-RO-4 yes (lifts the budget hold for this A/B): phase 4 (calibration, then the A/B) may start.

## ADR-072: Claude Code mods: fathom shows the person its evidence inside Claude Code through an optional fathom-ui plugin, one dependency-free TypeScript module the Go render writes, display-only, never a gate, a second engine or a second store (2026-10-16)

**Status.** Accepted 2026-10-16 (the owner's delegation of 2026-10-16, "record your decisions on the mods", decided
by the owner-architect; Q-MOD-1, and Q-MOD-2 for the owner-architect's wave sessions, answered 2026-10-16 by the
owner-architect under the owner's delegation, `docs/intent/questions.md` Q36). Design: `docs/design/claude-code-mods.md`. Plan:
`docs/plans/claude-code-mods.md` (MOD-). Goal G40. Assessment: `docs/assessments/2026-10-16-claude-code-mods.md`.
Narrows the `No Node toolchain` non-goal by one carve-out, its text amended in this change
(`docs/intent/scope/non-goals.md`). Promotes F2 in part and narrows ADR-029's rejected alternative *A marketplace*
to a catalog listing (amendment at the end of ADR-029). Settles how RH-6 is proven (`docs/plans/reach.md`). Reads
ADR-071 (readouts own every view of tool output and the shell-read measurement), F75, F78, ADR-059, ADR-063, I9,
I14, I38, I53, I62, SEC-1 and SEC-2, the table freeze and the budget hold as constraints. Follows readouts in
priority (ADR-071 decision 13).

**Context.**
- *What a mod is.* Claude Code 2.1.287 added mods (changelog 2.1.287): a plugin whose hooks file names a
  TypeScript or JavaScript hooks module, run in process. Per Claude Code 2.1.301's plugin typings (early access,
  "may change between releases without notice", line 4): the module runs "in an environment of its own: no DOM, no
  Node", and its one import is type-only and empty at run time (lines 14-20). It may redraw 15 named components
  (line 9771): a tool's result block on every attached surface, where a rewrite is checked against the tool's
  output schema and the stored result is untouched (lines 10362-10369); the band above the prompt, terminal and
  desktop only (line 10673); a pane. `$.ui.log` appends a dim transcript line that is not sent to the model (lines
  2432-2435). `classic.<Event>` fires for every settings-hook event; the chain is managed hooks, then hooks modules,
  then the other settings hooks, and the result carries their `additionalContext`, one entry per hook (lines
  1264-1327). A hook has 10 s per dispatch, its `$` calls excluded (lines 5236-5251), and a hook that fails is
  skipped (lines 4052-4053): mods fail open. `$.mcp.call` reuses the engine's MCP connection (lines 2749-2768);
  `$.process.run` runs an argv with no shell (lines 3581-3598); `$.store` is a JSON file kept between sessions and
  `$.state` a session's values any plugin may read (lines 3428-3467). `claude plugin validate --json` lists each
  gating hook (changelog 2.1.290) and names a mod's hooks that are skipped where an organization manages Claude
  Code (changelog 2.1.299); `claude plugin test` runs a mod's cases per surface (typings lines 28-41).
- *Fathom today.* `fathom plugin render` writes one plugin tree from the binary's embedded sources
  (`cmd/fathom/pluginrender.go:20-37`), versioned by a template digest pin (`cmd/fathom/pluginrender.go:43`,
  `:120-132`); `--marketplace` writes a manifest that nothing commits (`cmd/fathom/pluginrender.go:381-394`). The
  install test RH-6 is open (`docs/plans/reach.md:17`) and the catalog listing RH-X1 is deferred
  (`docs/plans/reach.md:50`). Every hook starts `command -v fathom >/dev/null 2>&1 || exit 0`
  (`cmd/fathom/hooknoop_test.go:95`), so a missing binary is silent. The edit hook gives the agent the governing
  rules and a blast trailer (`cmd/fathom/edithook_pre.go`, `cmd/fathom/edithook_blast.go`), a fathom answer that can
  be stale says so (`index_stale`, `internal/mcp/outputschema.go:137`), `/statusz?format=json` carries each
  service's indexed commit (`cmd/fathom/statusz.go:70-73`), and the stop hook names an unproven claim
  (`cmd/fathom/stophook.go:40-49`). All of it reaches the agent; whether Claude Code also shows hook context to the
  person is UNVERIFIED (MOD-M3 reads it).
- *The research.* A pass of 2026-10-16 proposed fifteen surfaces, M1 to M15, and eleven rejections (the assessment
  above). The non-goal read "Revisit when: never", and ADR-029 had rejected only *JS plugin hosts for
  OpenCode/Amp/Cline* under it (`ADR.md:1392-1393`), so a mod needed this record.

**Decision.**

1. *Display-only.* The program shows the person what fathom already told the agent or the gate already knows, and
   changes nothing the agent sees, is allowed or runs: no `tool.call`, `tool.check` or `agent.spawn` hook; every
   classic hook returns the result it got from `next(e)` unchanged; no question to the person; no approval, ever.
   Facts stay computed in Go (the determinism contract, I62); the module formats them.
2. *Q1, the Node carve-out: yes.* `fathom plugin render` may write one dependency-free TypeScript hooks module, with
   the `claude plugin test` cases for it, embedded in the binary as text, that only Claude Code executes. Fathom's
   build, gate and CI install, run and compile no Node, npm or TypeScript; the module imports only Claude Code's
   type-only declarations, has no package manifest and is at most 150 lines. The `.fathom/guard.cue` refusal of
   `npm`, `npx`, `yarn`, `pnpm` and `bun` stays as it is. Why: the module runs in Claude Code's own environment, not
   in Node, and fathom's artifact stays one Go binary. Revisit when: the module needs a dependency, a build step, a
   second logic file or more lines.
3. *Q2, a separate optional plugin: yes.* Mods ship as `fathom-ui` beside the core `fathom` plugin, which keeps the
   skills, hooks, MCP registration, the scout and the subagent status line and works without it. Why: an
   organization that skips a mod's hooks (changelog 2.1.299; the managed-mods-only setting the research names is
   UNVERIFIED in the local changelog and typings) keeps fathom whole. Revisit when: Claude Code can turn off one
   plugin's hooks module apart from its other components.
4. *The allowlist, enforced at render.* Render refuses to write a module that references anything outside:
   registrations on `classic.PreToolUse`, `classic.PostToolUse`, `classic.Stop`, `turn.complete` and `ui.render`
   for the tool result block, the band and a pane; two zero-turn commands, `/fathom` and `/proof`; `$.state` under
   its own keys; `$.ui.resolve`, `$.ui.log` and `$.ui.open`; `$.http.fetch` to the rendered loopback base only
   (I14); `$.mcp.call` to the core plugin's fathom server for `verify_plan` only; and `$.process.run` with the one
   argv `fathom doctor --json` (fathom's own binary with a fixed argv, the hooks' own trust, never a command a
   repository names: SEC-1, SEC-2). Never `$.model`, `$.store`, `$.tool.register`, `$.agent`, `$.fs`, `$.ui.ask`,
   `$.ui.notify`, `$.process.spawn` or another argv. `claude plugin validate --json` must list no gating hook.
5. *The portfolio.* Phase 1, Go and markdown only: M8 (the install check), M6 (the fathom-scout subagent), M7 (the
   subagent status line and `fathom statusline`), M15 (the committed marketplace manifest). Phase 2, the module: M1
   (the host), M2 (evidence rows), M3 (the evidence band), M4 (the proof line and `/proof`), M5 (the `/fathom` setup
   check). Owned elsewhere: M9 and M10 (decision 8). Deferred with triggers: M11 (decision 9), M12 (tests to run),
   M13 (a symbol blast hold, which gates and so needs an amendment of decisions 1 and 4), M14 (a review pane).
6. *Q3, commit the rendered marketplace manifest: yes.* The repository commits the manifest at its root and the
   rendered plugin trees it names, regenerated by render and held equal to a fresh render by a test (a hand-edited
   plugin is the schema-copy bug class, `cmd/fathom/pluginrender.go:25-26`); the README carries the self-hosted
   install line. F2 is promoted in part; a catalog listing, the brew tap and the MCP registry stay with F2 and
   RH-X1. Why: the install line is the shortest path from the README to a first answer, and the plugin's name is
   already public through the module path and the release archives (Q-MOD-1 answered yes 2026-10-16: the manifest
   names nothing the public repository and its releases do not already name). Revisit when: the owner reverses
   Q-MOD-1, or a catalog listing is proposed (F2's Promote-when and Q30 decide it).
7. *Q4, RH-6 by one ordinary session: yes.* One owner-run session in a scratch HOME, on the rendered plugin
   installed by the documented path, that lists the tools and gets one answer satisfies RH-6. It runs no bench, so
   the budget hold (`docs/intent/autonomy.md`) does not block it. An install script prepares it and runs in no gate
   (the gate has no Claude Code). Revisit when: a second client's install test (RH-7) needs a different proof.
8. *Q5, the read-routing hint: yes, when it fires.* M10 ships only as the recorded firing of the `No grep-blocking`
   non-goal's condition (at least 30% of greps in indexed repositories index-answerable, then a hint only), which is
   F75's Promote-when. Readouts own the measurement (RO-29, and ADR-071's corpus reading: bare-identifier greps
   13.5% of 29,643, an upper bound), and this program adds no row for it. M9 is not built: a classifier under an
   upper bound below the bar cannot read the condition as fired.
9. *Q6, spawn briefs: later.* M11 (`agent.spawn` appending pre-resolved symbols to a subagent's prompt) changes what
   an agent sees, so it waits for a $0 reading that subagents start with shell discovery and for whether
   SubagentStart's input carries the task (then a Go hook does it, with no mod), and needs an amendment of
   decisions 1 and 4.
10. *Readouts own every view of tool output.* The module never replaces a tool result. A view of a command's
    output is readouts', behind ADR-071's contract; readouts behind a mod's `tool.call` (F78) is deferred in the
    plan with F78's trigger and needs an amendment of decisions 1 and 4.
11. *Readings from what exists.* No `$.store`, table, counter or background job (the table freeze); no model spend
    (the budget hold). Every reading reads natural sessions, existing logs, the observations and the verify
    ledger, and every mod has a kill (the design, section 10).
12. *Priority.* The mods program follows readouts. Phase 1 starts when readouts phase 0 lands (RO-3 to RO-18
    verified, RO-M0 filed); mods waves fill while every runnable readouts row waits on a measurement window or a
    gate, and become the main line when readouts reaches its gates; the tenancy program follows, as RT-M1 reads
    (`docs/vision/38-proof-layer-roadmap.md` section 9.7).

**Non-goals.**
- *No Node toolchain.* Amended by decision 2's carve-out; its Revisit-when stays "never" beyond it.
- *No grep-blocking.* Does not fire (decision 8).
- *No net tool growth.* No MCP tool and no `$.tool.register`; one CLI verb, `fathom statusline`.
- *No generic token-usage dashboard.* No HUD or cost meter.
- *No dispatch loop.* No timer, monitor or pushed event; the band refreshes on turn completion and after edits; no
  status-setting button.
- *No IDE language services.* No LSP plugin.

**Jev, weighed.** No site. The module formats facts Go computed: a typed judgment could order the rule lines in an
evidence row, but a strip shows counts, not choices, and a judgment never asserts a code fact (I62). With provider
`none` nothing changes. The measurement that would open a site: M11 built and its choice of which symbols to
pre-resolve contested on written cases; then ADR-056 R3's calibration comes first, and any A/B waits on the budget
lift.

**Owner questions, answered 2026-10-16 by the owner-architect under the owner's delegation
(`docs/intent/questions.md` Q36).** Q-MOD-1, a self-hosted marketplace manifest in the public repository (no catalog
listing) is outside Q30's "public naming": yes; it names nothing the already-public repository and its public
releases do not already name, and the official catalog listing and a brew tap stay blocked by Q30 (the trademark).
Q-MOD-2, fathom-ui in real sessions once MOD-M2 reads PASS, so MOD-M4 and MOD-M5 read from them with no added model
spend: yes for the owner-architect's wave sessions (loaded by `--plugin-dir` at launch); the owner's own interactive
sessions stay an open yes or no for the owner.

**Alternatives rejected** (recorded so they are not re-proposed).
- *Answering a call in place* (a grep answered from the index, Grep rewritten to `search`, an auto-approver): the
  grep-blocking non-goal; it corrupts instrument B's discovery count; a mod may approve calls, fathom's never does.
- *A HUD or cost meter:* the generic token-dashboard non-goal; two public status-line projects, claude-hud and
  ccstatusline, held about 28,600 and 13,300 GitHub stars when the research pass read them on 2026-10-16
  (`docs/assessments/2026-10-16-claude-code-mods.md`).
- *Fathom's verbs re-registered with `$.tool.register`:* no net tool growth.
- *`$.model` routers or summaries:* the budget hold and the determinism contract.
- *A monitor or channel pushing events:* the dispatch-loop non-goal.
- *A module's own view of tool output* (a summary of a whole-file read, any result replacement): readouts own
  output views behind ADR-071's contract, where a read of a file the session edited stays raw and the outline class
  is gated by RO-M0 and RO-M3; a second engine would bypass that contract.
- *An MCP App:* Claude Code does not draw one, and it needs public HTTPS and auth, against I14. Revisit when fathom
  has an authenticated remote surface.
- *A live published-page dashboard:* a published page cannot call a local MCP server, and the observatory owns
  dashboards (ADR-063).
- *An LSP plugin:* the IDE language-services non-goal.
- *Uncheck or status-setting buttons:* the dispatch-loop non-goal and ADR-049 decision 4.
- *`$.store` counters:* a second store of fathom facts, against the table freeze.

**Consequences.** A second rendered plugin; one embedded TypeScript module and its test file; one CLI verb
(`fathom statusline`); one additive `/statusz` query (one service by root, with its head, how far its index is
behind and whether its tree is dirty); a plugin version bump; a committed manifest and rendered trees; the README's install line. No MCP tool,
table, cache, background job or model spend. The core plugin works without fathom-ui.

**Revisit when.**
- MOD-M3 reads that Claude Code already shows hook context to the person, or that the core plugin's command hooks
  do not reach a module's classic result: M2 is refused in the same change.
- MOD-M1 reads the parent re-reading a cited file with no edit of it within five calls after more than 50% of
  scout spawns, or no less often than after Explore subagent spawns: the scout is removed.
- MOD-M4 or MOD-M5 reads CUT: that mod is removed in the same change.
- Claude Code changes the mod surface: MOD-M2 re-reads before the next render ships.
- A mod needs a dependency, a build step, a second logic file or a call outside the allowlist: decisions 2 or 4
  are amended first.
- F78's trigger fires, or M11's reading meets its bar: decisions 1 and 4 are amended for that vehicle.
- The owner reverses Q-MOD-1: the committed manifest is removed and the README keeps the `--plugin-dir` path.
- The grep-blocking condition is recorded as fired: M10 is planned under F75.

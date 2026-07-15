# 03 — Findings, connected experts, hosted, chat

**Verdict.** The findings backend you need is mostly shipped and you should
not build a new one: findings are **documents in a corpus, compiled into
the existing `semantic_claims` stratum as `Kind:"finding"`** — a slot your
own code already reserves (`internal/core/claims.go`), and whose full loop
(non-git corpus → compile → cite → sweep → remap) your expert-module test
already exercises — *under the obligations kind*, since `Kind:"finding"` is
still gated off. The honest missing ~20%: a parameterized compiler and
kind-aware serving (that **is** [02 §3 M1](02-shape.md) — findings-as-claims
is blocked on it), per-claim scope fingerprints for remediation drift, and
the tier-schema widening ([02 §3](02-shape.md)). The hosted expert backend
is the wrong build at the wrong time, but the **composition contract**
(portable store format + loaded manifest + read-only ATTACH) is cheap to
freeze now and is the actual durable asset. MCP already *is* your pluggable
chat interface.

## 1. The findings model: findings are documents; claims are their index

Two kinds of "finding" that must not share a storage strategy:

- **Recomputable findings** — output of the deterministic analyzers
  (secret_scan, arch_conformance, god_object, …). Pure functions of the
  current index, $0 to recompute. **Do not persist them.** Persisting
  recomputable facts manufactures staleness debt. Today none are persisted;
  that is correct — keep it.
- **Judgment findings** — human review conclusions, LLM-assisted review
  outputs, incident root-causes, well-architected verdicts. Not
  recomputable; the expensive artifact. **These are the findings backend**,
  and the backend is: *one markdown file per finding in a corpus directory,
  ingested as a service, compiled into claims with `Kind:"finding"`.*

This is `docs/design/expert-modules.md` §3.2 verbatim, and
`TestExpertModule_NonGitCorpus_EndToEnd` already proves the whole loop:
non-git findings folder → `manifest:` watermark → same compiler → claim
scoped to code → "what governs this file" answered with a citation →
finding edited → claim flips `stale` → corpus moved → one `remap`, zero
recompile. **The job is to productize the test, not design a system.**

Why files-plus-claims beats a findings database/API: findings stay
git-versionable, reviewable, portable, human-owned; the claim layer gives
scoping, citations, staleness, tiers, and audit for free; the store file
remains the product. A bespoke findings CRUD API would duplicate the entire
lifecycle machinery you just shipped.

## 2. Findings vs code versions (your open worry — already solved)

Shipped ADR-016/017 machinery answers it with a three-part anchor:

1. **`CompiledAtWatermark`** — git HEAD or `manifest:` content hash at
   compile time. Every served finding carries an honest "as of."
2. **`ClaimLink rel="cites"` + `TargetHash`** — sha256 of the exact cited
   lines of the finding document; the $0 sweep flips the claim `stale` if
   the finding doc changes.
3. **`ClaimLink rel="scopes"`** — globs over affected code paths, verified
   at compile time.

The one genuinely new mechanic findings need beyond obligations:
**remediation detection** — "has the *scoped code* (not the cited doc)
changed since `CompiledAtWatermark`?" Implement as a second sweep pass for
`Kind:"finding"` claims: compare current file fingerprints of scope-matched
files against their state at compile time and surface `code_drifted: true`
on the served payload. A *serving annotation* ("re-review possible
remediation"), not a status flip. Unstated cost, stated here: this requires
**persisting the scope-matched files' fingerprints at compile time** (for
`manifest:`-watermark corpora there is no git to diff against) — a small
store addition that belongs in the same change. Don't start with
symbol-level anchors — observations are destructively purged per-source on
re-ingest; path+hash anchors are the stable substrate.

## 3. Lifecycle and tiers: wire what exists, don't invent

Your proposed asserted→verified→stale→superseded conflates two orthogonal
axes the shipped model already separates:

- **Freshness axis** (machine-owned, automatic): `fresh / stale /
  contradicted / superseded` — shipped, swept on every ingest.
- **Verdict axis** (judgment, append-only): `claim_events` with
  `refuted / corroborated` — **fully modeled and completely dead**:
  `AppendClaimEvent` has zero non-test callers.

Map your states: *asserted* = claim exists at its origin tier; *verified* =
a `corroborated` ClaimEvent (which deliberately never promotes tier —
corroboration is provenance, not evidence-laundering; keep that);
*stale/superseded* = already automatic. **The build item is wiring the dead
machinery**: `fathom claim refute|corroborate <claim-id> --note` (CLI + one
MCP tool), with refute flipping status to `refuted`. That is the entire
triage system — suppression is refutation with a note.

**Refutation durability is part of this item, not optional.** Today
`PutClaims` upserts status and the compiler always emits `fresh`, so any
recompile that re-emits an unchanged rule (same ClaimID) would silently
flip a refuted claim back to fresh — the suppressed finding reappears with
no trace. The rule: on upsert, a claim whose ClaimID carries a `refuted`
ClaimEvent (and unchanged repro key) **retains** refuted status. One lookup
in `PutClaims` or the compile path; without it the triage system is
unsound, especially across the M1 flag-day recompile ([02 §3](02-shape.md)).

Tiers per finding source, using the existing vocabulary:

| Source | Tier | Provenance |
|---|---|---|
| Deterministic analyzer snapshot | `verified` | `Model:""` (deterministic passes are explicitly supported); `ReproKey` = analyzer+rule+anchor digest |
| Human review finding (doc) | `sme-stated` | citation into the finding doc |
| LLM-assisted review | `doc-claimed` if citation-grounded, else `assumed` | Model + PromptVersion recorded |

Prerequisite the schema forbids today: `#Layer.tier` allows only
`doc-claimed | sme-stated`, so the `verified` row is unrepresentable until
the tier widening lands with M1 ([02 §3](02-shape.md)).

The content-addressed ClaimID gives dedup and baselines for free:
re-running a deterministic snapshot emits identical ClaimIDs for unchanged
findings (upsert, no churn); "new since last scan" = ClaimIDs first seen
after watermark W.

**The deterministic bridge — `fathom report`.** For "analyzer results at
release X as a baseline," don't persist analyzer structs — add a verb that
writes an analyzer's output as a markdown finding document into a corpus
directory. It then flows through the normal pipeline (ingest → compile →
claims, tier `verified`, deterministic ReproKey — an LLM-free compile
path). One verb unifies the five convergent `Finding{File,Line,Rule,
Severity}` structs at the *document* boundary instead of forcing a Go-type
unification across 37 packages. Start with `security_rollup` or `verify`.

## 4. Connected experts, hosted, and chat

**The minimal composition contract is three artifacts, zero servers:**
1. **Store-format spec v0** — versioned, self-describing schema doc for the
   `.duckdb` file (tables, ClaimID computation, citation-hash contract,
   watermark semantics). Your vision's own "durable asset." Start the doc
   now; it costs prose and pays schema discipline.
2. **Loaded module manifest** — [02 §3 M1](02-shape.md).
3. **Cross-module link vocabulary** — `ClaimLink.Rel` already reserves
   `supports/contradicts/supersedes`; define in the spec (not code) that
   cross-module links address `<service>::<claim-id>`. Federated
   disagreement is then "a ranked, tiered result set, not a merge" — no
   orchestration engine needed.

**Federation = DuckDB ATTACH over conforming portable stores** (H3, entry-
conditioned, unchanged). A connected-experts product is a reader over N
module stores, buildable by anyone — including future-you — without
touching the kernel.

**Hosted: never-until-pulled.** Hosting is only required for multi-tenant
writes, authn/authz, and cross-org sharing without file copy. Every one
collides with load-bearing design (`SetMaxOpenConns(1)` single-writer
DuckDB, one daemon, all SPIs internal) — and adds the unpriced cost of
**data governance for other people's code intelligence** (breach target,
SOC2, DPAs) that a solo maintainer cannot carry. Born-portable stores are
the better strategy anyway: the file travels; you never hold the data.
*Evidence gate:* one external party runs local fathom for 4 weeks and asks
in writing for a shared/hosted view. Until then: zero lines of code.

**Chat: MCP already is the pluggable chat interface.** A "pluggable chat
interface specialized per use case" is exactly an MCP client + a
path-scoped surface + a module store + canned prompts — all shipped. The
learning platform and the incident command center are *the same module
served to different surfaces with different prompts*, not applications.
(The mechanism being cheap doesn't make the use case worth it: the learning
platform is rated TRAP in [06 §1](06-use-cases.md) — don't pursue it even
as configuration.)
The incident center's real v1 is a failure-mode layer (entry-conditioned at
≥15–20 resolved findings; you have ~5) + the `investigate_incident` prompt.
If a visual artifact is ever warranted it is a **read-only viewer over the
store** (`fathom render`: static HTML from a `.duckdb` — claims, findings,
staleness, compile ledger), which doubles as the compliance-probe sales
artifact. A weekend, not a platform — and it still waits for a consumer.

## 5. The build split and the sequencing rule

**Build now (small, on the H1→H2 path, in this order):**
- Finish H1's two open items first (fleet compile + injection coverage; O6).
- Unlock `Kind:"finding"` end-to-end. Honest scope: this is more than
  relaxing the two hard-rejects in `cmd/fathom/compile.go` +
  `internal/mcp/semantic_tools.go` — the compiler hardcodes
  layer/kind/tier/prompt and `ObligationsFor` serves only the obligations
  layer, so the real unlock is the compiler/serving parameterization that
  **is M1/M2** ([02 §3](02-shape.md)). Findings-as-claims is sequenced
  after M1, not parallel to it.
- Wire `claim_events`: `fathom claim refute|corroborate` + refute→status,
  **with the durability rule** (§3).
- Remediation-drift flag (§2, including compile-time scope fingerprints)
  and `fathom report` (§3).

**Contract now, build later:** store-format spec v0; `/rpc/expert` surface
spec; federation ATTACH semantics + link addressing (spec text only).

**Don't build:** hosted multi-tenant service; a connected-experts
orchestration engine; a findings CRUD API; chat products.

**The sequencing rule — adopt as program law:**

> *Nothing graduates from contract to code without a named consumer or a
> passed gate. Every capability advances one rung at a time: spec → library
> → CLI/MCP verb → surface → service — and a rung is climbed only when the
> rung below has a live consumer and its kill gate has run.*

Applied: findings-as-claims may become a CLI verb now (named consumers: the
expert-module test, the review-engagement recipe, this fleet). Federation
stays spec-only (its consumer doesn't exist). Hosting stays unwritten.

**"Am I reaching too far?" — no, and yes.** No, architecturally: every idea
has a designed, shipped-or-specced landing zone (findings → reserved Kind;
connected experts → ATTACH; versioning → watermarks + hashes; pluggable
interfaces → surfaces). That is the opposite of silly. Yes, in sequence:
injection coverage 0%, pull adoption 0%, verify gate live in 1 of the
required 2 repos, one user total. The rule above lets you keep designing
the whole cathedral while only laying stones someone is standing on.

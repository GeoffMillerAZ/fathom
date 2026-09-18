# 30 — The findings loop: review findings served as obligations, promoted into rules

**Verdict.** A review finding and a governing rule are the same shape of fact —
a claim about a place in the code, made by a named producer with a stated
method, that must be re-checked when that place moves — and fathom already
stores, drift-tracks, tiers and serves that shape. What is missing is not a
findings database; it is the **loop**: an external reviewer's findings enter
fathom through a governed import that preserves who found what, at which
revision, at which confidence; the open findings on a file are served to the
agent *before* it edits that file, beside the governing rules; the next review
closes them by evidence, never by silence; and a finding that keeps recurring
becomes a candidate rule a human lifts into `CLAUDE.md` or `guard.cue`, where
ADR-038 enforcement makes the lesson permanent. Review teaches the store, the
store governs the edit, enforcement retires the class of mistake. The
organizational record of findings stays where its authority lives — the
review kit's portable journal (platform-engineering ADR-0098) — and fathom is
its first projection adapter, never its only home. This document is the
fathom half of one contract; the review-kit half is
`kits/review-kit/docs/design/fathom.md` and ADR-0103 in that repository.

Status: ACCEPTED 2026-09-18 (ADR-040). Plan: `docs/plans/findings-loop.md`.
Supersedes nothing; it closes the trigger `docs/plans/findings-tracer.md`
FT-8 named ("first external consumer of a findings store") and delivers the
review kit's uplift brief P0–P4 (`kits/review-kit/docs/design/fathom-uplifts.md`).

## 1. What already exists — the loop is mostly substrate

| Side | Shipped | Where |
|---|---|---|
| fathom | finding-kind claims on the `findings` layer, content-addressed IDs, cites/scopes/scope-fp links, drift sweep, `code_drifted` serving annotation | `internal/core/claims.go`, `internal/semantic/serve.go`, FT-1..FT-7 |
| fathom | closed failure-class vocabulary, deterministic pattern derivation, resolution outcomes on supersede, one-tier promotion behind the layer's eval gate, call-graph neighborhood (`findings near`), entry-condition status | `internal/core/findingclass.go`, `internal/analysis/findings`, phase R (`docs/plans/expert-suite-r.md`) |
| fathom | native review findings persisted deterministically at tier `verified`, reconciled as `superseded` when a later review no longer emits them, refutation durable across re-review | `internal/analysis/review/claims.go`, `cmd/fathom/review.go` |
| fathom | governing rules served at PreToolUse/PostToolUse, prompt-scoped rules, session brief, enforcement of `enforce: true` rules | ADR-036, ADR-037, ADR-038 |
| review kit | portable finding identity `LogicalID(producer, subject, sourceId, ruleId)`, append-only journal, monotonic `sourceVersion`, event kinds observation/closure/annotation/revocation, exact-subject visibility, archive export/restore | `internal/findings`, `schema/findings.cue` |
| review kit | report-only bridge over `fathom review --no-persist --json`, native-finding import into the journal, a **pending projection outbox** waiting for a governed importer | `internal/fathombridge`, `leadsman fathom-review`, `schema/fathom_bridge.cue` |
| review kit | evidence adapters as ports (fathom is a shell-out adapter), a `Digester` port, ADR-0099 cache keys over all influential inputs | `internal/evidence/adapters.go` |
| agentic-kit | evidence span/locator/anchor, evidence grade with floor tier and independent-corroboration promotion, AI/human authorship provenance, read manifests | `foundations/evidence/*`, `foundations/provenance/ai-provenance`, `foundations/telemetry/read-manifest` |

The two sides already share one tier ladder — `verified > pre-scan >
doc-claimed > sme-stated > assumed` — and one subject model. Nothing below
introduces a second vocabulary for either.

## 2. Ownership: hexagonal, fathom is the first adapter

- **The review kit owns** finding identity, lifecycle (open / closed / unknown /
  revoked), dispositions (dispute / accept / tailor), the Standard, and the
  portable bundle. Its `ProjectionPort` publishes lifecycle events to any
  projection target and records a **receipt**; the fathom adapter is the first
  implementation, a file-archive adapter the second, a null adapter the
  control. A deployment without fathom loses nothing it owns.
- **fathom owns** code facts (symbols, edges, fingerprints, watermarks), the
  claims store as a *projection* of findings anchored to those facts, the
  drift machinery, decision-time serving, and the derived promotion report.
  fathom never approves, disputes, tailors, or closes a review-kit finding on
  its own authority; a review-kit acceptance is never translated into a
  fathom `refute`.
- **Authority never flows backwards.** A projected finding cannot re-enter
  the review kit as independent evidence, and a fathom-native finding that the
  review kit imported into its journal cannot be projected back into fathom
  (the importer refuses producers whose `origin` is `fathom`). This is the
  circular-proof rule both design docs already state; the producer catalog
  (§4) makes it a check instead of a convention.

## 3. The contracts (fathom side)

All five are versioned with an explicit `schema_version`; a consumer that
sees a version it does not know refuses, it never decodes unknown fields into
healthy zero values. Fixtures under `testdata/contracts/` are the executable
form of every table below and run through CLI, Go client and (where one
exists) the daemon.

### 3.1 P0 — the public review request/response

`fathom review` keeps its flags and gains an explicit contract:

| Request field | Meaning |
|---|---|
| `schema_version` | `1` |
| `service` | the indexed service; the subject the caller is authorized for |
| `since` **or** `manifest` | a git ref to diff, or an explicit changed-path manifest `[{path, sha256}]` (the review kit supplies the manifest; a manifest digest that disagrees with the working tree is a named refusal, never a silent re-read) |
| `mode` | `report` (default when `--no-persist`) or `persist` — `report` **never** writes claims, events, or semantic state; a store-state equality test pins it including error paths |
| `expect_index_watermark` | optional; a mismatch is `stale_index: true` in the response and no `persist` |
| `idempotency_key` | optional; a repeated key with the same payload digest is a no-op returning the recorded response, the same key with a different payload refuses |
| `limits` | `budget_seconds`, `max_bytes` |

| Response field | Meaning |
|---|---|
| `schema_version`, `producer` `{name:"fathom", version, build}` | who produced it |
| `inputs` | `{manifest_digest, source_watermark, index_watermark, rules_digest, plans_digest, extractor_versions}` — exact input fingerprints |
| `dimensions[]` | each with `checked: bool` and a `reason` when not — a budget skip, a missing rule pack, a stale index or a clipped output can never read as a clean review |
| `coverage` | `{complete: bool, unknown: [...]}` |
| `truncated`, `stale_index`, `cancelled` | explicit |
| `findings[]` | native findings with `class`, `tier`, `citation`, `claim_id` (persist mode only) |
| `test_impact` | unchanged |

Process subjects (a finding about a workflow, not a symbol) are accepted
without inventing symbol anchors: a finding may carry `anchor: {path}` only,
or no anchor at all with `unanchored: true`.

### 3.2 P1 — the review key: complete-input eligibility without a second hash API

`fathom review-key --service S (--since REF | --manifest FILE) [--prior KEY.json]`
computes one token over every input a review verdict depends on, and says
what it could not see:

```
{ "schema_version": 1,
  "key": "<sha256>",
  "complete": false,
  "unknown": ["dependency closure: 2 packages have no typed import edges (cgo)"],
  "components": {
    "manifest": "<sha256 of sorted path+digest>",
    "closure":  "<sha256 over the typed import/call closure of the changed packages, transitively, at index watermark W>",
    "rules":    "<sha256 of the project rule pack + guard.cue + enforced layer rules>",
    "plans":    "<sha256 over docs/plans/**>",
    "tooling":  "<fathom version + extractor grammar shas>",
    "index":    "<index build identity + watermark>" },
  "changed_since_prior": ["closure", "rules"] }
```

Rules: a missing or unreadable path never hashes as an empty file (it is an
`unknown` entry); an incomplete closure makes `complete: false` and the key
still computes, so the consumer may reuse *narrower* results or refuse — the
review kit's ADR-0099 policy decides, fathom only reports. Identical inputs
give an identical key across runs and machines. The review kit consumes it
through its existing `Digester` port for the code-closure component and
keeps its own key for non-code inputs; fathom's typed import graph is the
part no consumer can recompute cheaply.

### 3.3 P2 — the governed external import

`fathom findings import --service S --subject ORG/REPO/COMPONENT [--producer NAME] --file projection.json [--dry-run] [--json]` (`--producer` asserts every event's producer; omitted, each event's producer is checked against the catalog)

**Input** is the review kit's projection (`#FathomProjectionOutbox` events)
plus two additive anchor fields (`anchor.start`, `anchor.end`, and
`anchor.symbol`, all optional). The importer:

1. **Validates** schema version, subject equality with the provisioned
   mapping (`.fathom/config.toml [findings] subject = "org/repo/component"`;
   no mapping, no import), producer presence in the catalog (§4), event
   ordering, digests, relative paths, and that the producer named on the
   event matches `--producer`.
2. **Derives identity** deterministically: `ClaimID("findings", "finding",
   "ext:<producer>/<org>/<repo>/<component>/<sourceId>", findingId)`.
   `PromptVersion = "external-import/v1"` is the discriminator that keeps
   native review reconciliation (`StaleReviewClaims`, prompt version
   `review-verb/v1`) from ever touching an external claim, and vice versa.
3. **Maps lifecycle** to the existing vocabulary, never inventing a status:
   `observation/fail` → `fresh` (re-observation of the same id/version is a
   no-op; a higher `sourceVersion` refreshes the row and appends nothing
   destructive); `observation/pass` or `closure` → status `superseded` plus a
   `ClaimEvent{Verdict:"superseded", ResolutionKind:"external-closure"}`;
   `revocation` → `superseded` with `ResolutionKind:"revoked"` (a revoked
   source is not a refuted claim; refute stays a human verdict);
   `annotation` → `ClaimEvent{Verdict:"annotated"}`, status-preserving, and
   the note is stored, never served as rule text.
4. **Enforces order and idempotency** with a per-claim cursor row
   (`external_findings`: producer, source id, latest source version, payload
   digest, anchor digest, imported-at): an older `sourceVersion` than the
   cursor is `stale` (counted, not applied); the same version with a
   different payload digest is `rejected: conflicting-replay`; the same
   version and digest is `duplicate`.
5. **Caps tier** at the producer's declared cap and never above the event's
   own tier; the served claim's `Tier` is `min(event tier, producer cap)`.
6. **Anchors**: `scopes` = the anchor path; `scope-fp` = the store's current
   fingerprint (so `code_drifted` works exactly as for native findings);
   `cites` = `path#Lstart-Lend` only when lines are given and hash cleanly
   from the current file. The producer's own anchor digest is kept on the
   cursor row; `anchor_current` is recomputed at serve time by hashing the
   current file, so a finding whose code moved is served **flagged**, never
   dropped and never silently re-anchored.
7. **Receipt**: `{schema_version, delivered, duplicate, stale, rejected: [{finding_id, reason}], truncated, cursor: {producer, latest_source_version}}`.
   Nothing is written on any rejection of the batch's schema; per-event
   rejections are recorded and the rest apply.

Loss of the projection is repaired by re-running the import from the review
kit's archive; the receipt makes the repair auditable.

### 3.4 P3 — serving at the point of work

- **Pre-edit hook** (`fathom edit-hook --pre`): after the governing rules and
  the enforcement verdict, a bounded block `open findings on this file (N,
  showing k)`: for each, `class · producer · rule id · one-line rule ·
  remedy · path#Lstart-Lend · state · anchor_current`. Cap 3 findings and
  1,200 bytes; `omitted: N-k` always rendered; superseded and refuted never
  appear; a finding whose anchor drifted is marked `(code moved since
  observed)`. Findings are advisory here — enforcement belongs to rules, and
  a promoted rule enforces, not the finding.
- **Post-edit hook**: the same block only when the edited region overlaps a
  finding's cited lines (one beat: `finding_touched`).
- **Prompt hook**: one line, `N open findings across the files this prompt
  names`, inside the existing deadline.
- **Session brief**: `open findings: N (k producers; oldest 2026-09-02)` and,
  when an outbox is configured, the sync result (§3.7).
- **Isolation**: claims are service-scoped in the store; the import binds one
  provisioned subject to one service; a test proves a claim imported for
  service A is invisible to obligations, findings near, and the hooks for
  service B. The full `/rpc` registry stays a trusted local boundary (ADR-035
  posture); the import is a CLI/client write, not a network write.
- **Cost**: findings ride the existing obligations fetch (`layer: "all"`),
  one call, no compile, no model.

### 3.5 P4 — the consumer seam

`pkg/client` gains `ImportFindings(ctx, req) (Receipt, error)` and
`ReviewKey(ctx, req) (Key, error)`; both marshal the same JSON the CLI emits.
Conformance fixtures (`testdata/contracts/findings-import/*.json`,
`testdata/contracts/review-key/*.json`) are run by one test through the CLI
and the client; a review-kit fixture copy of the same bytes runs in that repo.
Unknown fields refuse; an unsupported `schema_version` refuses with the
supported list.

### 3.6 Promotion — the expert-system half of the loop

`fathom findings candidates [--min-sites 3] [--json]` derives **rule
candidates** from patterns (`DerivePatterns` over native *and* external
findings): a pattern with ≥ `min-sites` distinct sites emits one candidate
with the class, the normalized rule shape, the member citations, the
producers that saw it, how many members were resolved by a code change, and
a drafted rule sentence plus the guard-check family (ADR-038) that could
enforce it if a human lifts it into `CLAUDE.md` or `.fathom/guard.cue`.
Candidates are a report. fathom never writes a rule. `findings status`
gains the candidate count so the entry condition reads from data.

### 3.7 The outbox sync — no network, no daemon dependency

`.fathom/config.toml`:

```toml
[findings]
subject   = "geoffmilleraz/fathom/fathom"       # the provisioned mapping (§3.3)
outbox    = ".reviewkit/outbox/projection.json" # optional; repo-relative
producers = ["reviewkit.system-review", "reviewkit.security-review"]
```

When `outbox` is set, the session hook (SessionStart, SubagentStart) runs
`findings import` against it if the file's digest differs from the cursor
under the state dir, bounded to one attempt per session, and reports the
receipt in the brief. A CI review that commits its outbox, or a local
`leadsman review`, therefore reaches the next coding session without a
service in between. `fathom findings import` remains the explicit form.

## 4. Provenance and confidence — structural, not decorative

**Producer catalog** — `schema/semantic/producers.cue`, loaded by
`internal/semantic.LoadProducers`:

```cue
#Producer: {
  id:       string          // "reviewkit.system-review"
  kind:     "deterministic" | "model" | "human" | "external-tool"
  origin:   "fathom" | "external"
  tier_cap: #Tier           // the highest tier a claim from this producer may carry
  requires: [...string]     // event fields that must be present ("anchor", "rule.closure")
}
```

Rules the catalog makes checkable:

1. **Tier never rises on import.** `served tier = min(event tier, producer
   cap)`. A `model` producer caps at `sme-stated`; a `deterministic`
   producer caps at `pre-scan` unless its cap is explicitly `verified` with
   a stated oracle; a `human` producer caps at `sme-stated`.
2. **Corroboration counts only across independent producers.** Two events
   from the same producer are one voice; a pattern's promotion (phase R,
   unchanged) counts *resolutions*, not observations.
3. **A fathom-origin producer is refused** with the reason `circular:
   native finding already exists as <claim_id>`.
4. **Every served finding is source-linked**: producer, producer version,
   rule id and version, source id, source version, observed-at, revision,
   anchor path (+ lines, symbol when given), anchor digest, `anchor_current`.
   A finding with no anchor is served as `unanchored` process context, never
   attached to a file.
5. **Contradictions are shown, never resolved by fathom**: a `fail` from
   producer A and a `pass` from producer B on the same rule and anchor are
   both served, each with its producer, and the pair is counted in
   `findings status` as `disagreements`.
6. **Nothing is inferred from prose.** Annotation text is stored and shown
   as a quote with its author; it never becomes a rule, a scope, or a status.
7. **Read coverage is visible**: the importer and the candidates report use
   agentic-kit's read manifest; an unreadable anchor file is `unreadable`,
   never `absent`, never `anchor_current: false` by default (I38).

These are the agentic-kit evidence laws (span, grade floor tier,
independent corroboration, authorship provenance) applied to findings; where
agentic-kit lacks a primitive this needs — a producer catalog with tier caps
and an origin flag — the primitive is added to agentic-kit's evidence
foundation as `evidence-producer`, and fathom consumes it.

## 5. Honesty rules and non-goals

- A finding served is not a finding verified; the block says who observed
  it and at which revision, and `anchor_current` says whether the code moved.
- An import receipt is not organizational admission; a refusal is not a
  finding of no findings; an outbox that failed to sync is reported in the
  brief with the reason.
- fathom does not become the organizational record. Retention, revocation,
  restore drills, access isolation across trust boundaries, and named
  operational owners remain the review kit's (ADR-0098 "Revisit when") —
  the receipts and cursors here are what would let that review happen, not
  a claim that it has.
- No model in this path. Import, serve, candidates, review-key: all
  deterministic, all replayable from the review kit's archive.

## 6. Measurement — the loop is a bench claim until it is measured

Corpus `testdata/scenariobench/findings-loop.json` (not run in this round):
tasks where an open external finding sits on the file the task edits, arms
`full` (findings served) vs `control`, paired criteria: the finding's remedy
applied (`finding_addressed`), the finding cited by the agent
(`finding_cited`), no unrelated rewrite, tokens per task. Kill criteria: no
lift in `finding_addressed` at n ≥ 3; hook injection exceeding the ADR-036
budget; a disagreement or drifted finding served as current. Second reading
before any number is cited (docs/bench/README.md).

## 7. Adoption

`fathom init` scaffolds the `[findings]` section commented out and, when
`--harness codex`, the same edit-hook codec renders the block. The
`fathom-onboard` skill gains the loop as step 7 (review kit outbox →
import → served); a new `fathom-findings` skill covers import, inspect,
candidates, refute. `docs/adoption/findings-loop.md` is the end-to-end guide
(CI variant: the GitHub Action commits the outbox; local variant: `leadsman
review` then `fathom findings import`). `docs/use-cases/change-review.md`
gains the loop; `docs/mcp-tools.md` records that no agent-surface tool grew.

## 8. Deliverables

The plan rows in `docs/plans/findings-loop.md` (FL-1 … FL-24) derive from
§3, §4, §6 and §7 one-to-one; the review-kit rows (RP-1 … RP-9) live in
platform-engineering `docs/plans/findings-projection.md`.

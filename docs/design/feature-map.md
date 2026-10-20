# The feature map: computed evidence over authored features, verdicts never auto-applied

Status: accepted 2026-09-28 (ADR-055). Vision: `docs/vision/39-feature-map-and-review.md`.
Plan: `docs/plans/feature-map.md` (FM-, wave 5). Goal G26. Builds on the inventory keys
(ADR-044), the system graph's contract keys and links (ADR-050), test intelligence's
risk-weighted coverage (T1/T2, `docs/design/test-intelligence.md`), work intelligence's cost
and interest rate (U1/U2, `docs/design/work-intelligence.md`), and knowledge retrieval's
doc-to-code links (KR, `docs/design/knowledge-retrieval.md`). Constraints carried in: the join
rule (ADR-051 decision 1: a product that needs a new engine is sent back), CX-2's risk-band
shape (a rule table, strongest reason, never summed), the surface-reckoning pattern (ADR-053
decision 5: every removal proven absent), ADR-040 (fathom never approves, disputes, tailors or
closes), ADR-049 decision 12 (no model deciding done).

## 1. Problem

The owner's Feature Scorecard (86 features, owner verdicts, 2026-09-27;
summarized in `docs/assessments/2026-09-27-feature-scorecard.md`) is valuable for its evidence, not its verdict column, and fathom already holds
every fact that evidence is built from — usage, cost, proof, tests, bench readings — scattered
across separate verbs. Nothing joins them by feature. Building that join, and nothing past it,
is this design.

## 2. Decisions (ADR-055 carries the record)

1. A feature is a named capability anchored to surface keys; `docs/features.md` is the
   authored source of truth, plus auto-proposed candidates; a closure check reports the gap.
2. Every computed fact is a join over existing assets, never a new engine.
3. Implementation health is a published rule table (strongest reason, never summed).
4. Verdicts are authored or decision-port-proposed under calibration; never auto-applied.
5. Overrides are a Standard (Review Kit's job); a small local fallback ships in fathom.
6. Surfaces: a CLI verb, a console page, an artifact-style export.

## 3. The `docs/features.md` grammar

A row is plan-grammar-shaped but the file itself is not under `docs/plans/` and is never
gated by `fathom verify` — it is authored spec, read by the feature map the way a rules file
is read by obligations.

| Field | Meaning |
|---|---|
| `id` | stable, author-owned (`FM-feat-NNN` or a short mnemonic) |
| `name` | human title |
| `keys` | comma-separated surface keys the feature is anchored to (`cli:`, `mcp:`, `route:`, `rpc:`, `page:`, or a public API symbol) |
| `owner` | a CODEOWNERS-shaped name or team, or blank (the health band names an unowned feature) |
| `cited` | the ADR(s) or spec doc(s) that justify the feature's existence |

`ParseFeatureMap` refuses a row that carries no `keys` or an unparseable key by name — the
same "refuse anything else by name" discipline ADR-050's `ParseContractKey` uses.

## 4. The closure check

Every key ADR-044 and ADR-050 extract from the surface inventory must resolve to exactly one
feature; a key with no feature is reported **unmapped**. Every feature's keys must resolve to
a live inventory entry; a feature whose key was retired or renamed is reported **dangling**.
Both are reported, never silently dropped — the same closure discipline ADR-054 uses for the
support matrix, applied to features instead of registries.

Auto-proposed candidates (new inventory keys with no citing feature) are surfaced as
proposals only; nothing is ever written into `docs/features.md` on the feature map's own
authority.

## 5. Computed columns and their sources

Every column below is a join over an asset that already exists. None is computed twice: a
feature's numbers are the same numbers `blast_radius`, `test_impact`, `review_diff` and the
console already produce, re-keyed by feature instead of by file or zone.

| Column | Source |
|---|---|
| code reached | `blast_radius`/`system_impact` over the feature's keys |
| specs, plan rows, ADRs citing it | plan-grammar spec IDs and `needs:` graph; state read as proven, open or regressed |
| tests and live certificates | T1/T2 risk-weighted coverage over the reached code |
| usage | fathom's call and hook logs; for a service, an imported access-log or analytics count file bound to a commit; otherwise **"unknown"**, never guessed |
| cost and debt interest | U1/U2, re-zoned to the feature's keys instead of package zones |
| churn | git-derived churn computed at ingest (TQ-1's mechanism, reused) |
| owners | CODEOWNERS plus commit history over the reached code |
| docs linked | knowledge retrieval's doc-to-code links (KR-6), filtered to spans naming the feature's keys |
| flags and config read | the system graph's `env:`/`config:` keys |
| dependencies | the system graph's `pkg:` keys |
| incidents citing it | `docs/intent/incidents.md` entries that name the feature |

## 6. The health rule table

Implementation health is a band — `strong`, `adequate`, `weak` — computed the same way the
change-risk band is (CX-2): **the strongest matched factor wins; no score is summed; every
reason is listed with its evidence.**

| Factor | Signal | Weight in the band |
|---|---|---|
| test/certificate coverage | T1/T2 risk-weighted coverage over the feature's reached code | `weak` if the uncovered risk-weighted share is high |
| contract certification | T5 contract certificates on the feature's system-graph links | `weak` if a `spec`/`typed`-tier link carries no live certificate |
| ownership | CODEOWNERS plus commit history | `weak` if no CODEOWNERS entry and no committer in 180 days |
| proof state | plan rows and ADRs citing the feature | `weak` if a claimed citing row reads claimed-unproven or regressed |
| debt interest | U2's zone interest rate over the feature's keys | `weak` if materially above the fleet median |
| doc drift | KR doc-to-code links citing the feature | caps the band at `adequate` if a linked doc is flagged possibly stale |

A feature with no weak factor and no capping factor reads `strong`; one capping factor with
no weak factor reads `adequate`; any weak factor reads `weak`, whatever else is true of the
feature — matching CX-2's rule exactly, never a different one for a different domain.

**Value** is a separate axis: usage-derived where a usage source exists, and **"not
measured"** otherwise — never inferred from health, never guessed.

## 7. Verdict claims: authored and proposed

**Authored.** A human writes a cited verdict (`keep`, `improve`, `merge`, `ditch`) directly
into `docs/features.md`, next to the feature's row. This is read and served; the feature map
never computes it.

**Proposed (the decision port), deferred.** Owner decision 2026-09-29 (ADR-055 amendment, future
F51): the proposal path is deferred, not refused, and verdicts stay human until it is earned. When it
returns, it is a Choice over the four verdicts given a feature's computed evidence row (health,
value, cost, usage), calibrated against the owner's 86 labeled verdicts from 2026-09-27 exactly as
CV-M11 calibrated the route site. Its trigger is J0 (`docs/plans/judged-predicates.md` JP-M1)
reading PASS and a calibration against those 86 verdicts reaching the bar of section 8. The proposal
is shown with its calibration and is never treated as proof. A `ditch` proposal never deletes anything:
it routes into the surface-reckoning pattern (ADR-053 decision 5) — callers proved absent, the
citing plan rows either re-anchored (the capability survives under a different key) or
retired to `[-] refused` (the capability is actually gone).

## 8. Decision-port proposal calibration (deferred: FM-10, FM-11, FM-M2)

The calibration bar is the same one ADR-042's route site used and failed, applied here before
the proposal path is trusted rather than after: an agreement rate of at least 0.8 against the
owner's 86 adjudicated verdicts, at n at least 3. Below the bar, the proposal path is dropped
outright, not tuned or hedged — the route-site lesson was that a hedge on a miscalibrated
signal still misleads. Jev has no role in computing any evidence column or the health band
itself; both stay deterministic, exactly as ADR-050 kept Jev out of link matching.

The 86 labeled verdicts are committed: `docs/assessments/data/2026-09-27-feature-scorecard.json` (summarized in
`docs/assessments/2026-09-27-feature-scorecard.md`) is the label set this calibration would run against, and the
same file is FM-M1's hand-built oracle (section 12).

## 9. Overrides: the Standard versus the local fallback

What counts as value, the health table's exact weights and thresholds, and any extra
consideration beyond this design's defaults — that is a Standard, authored as a
`product-portfolio` catalog, and authoring Standards is explicitly Review Kit's job (D3).
Fathom ships a small `features` key of `#Project` (by convention `.fathom/features.cue`), read through the one loader (ADR-061 decision 2), as the local fallback, the same shape as `.fathom/risk.cue`,
with defaults compiled into the binary so a repo with no Review Kit installed still gets a
working feature map. `docs/features.md` itself — the authored rows and their citations — stays
canonical either way; a Standard can change how a feature is scored, never which features
exist or what they cite.

## 10. Surfaces

- `fathom features [--json]` — the computed table plus authored verdicts and any proposal.
- A console Features page: a value-by-implementation scatter with verdict shapes, a
  filterable table, an underused list, a ditch list — the same shapes the Feature Scorecard
  artifact used by hand.
- An artifact-style export reproducing the Feature Scorecard artifact's first reading, so the
  hand-built baseline (FM-M1, committed as `docs/assessments/data/2026-09-27-feature-scorecard.json`) has something
  to compare against.

No new MCP tool (law 4): the feature map is a CLI verb and a console page, like the support
matrix before it (ADR-054 decision 5).

## 11. Tenancy and cost

Every product here is a projection over ledgers, stores and indexes that already exist,
computed in the background after ingest or on a bounded read, the same discipline work
intelligence uses (§4 of `docs/design/work-intelligence.md`). No new capture. The lane rule
(ADR-049 decision 2) applies: no row that adds a table, a cache or a background job starts
before `docs/plans/responsible-tenant.md#RT-M1` reads PASS, and the feature map's own
dependency on system-graph storage (SG-2) and the work-intelligence cost rows (WI-2, WI-5)
means it cannot start before those do either.

## 12. Measurement

| Reading | Threshold |
|---|---|
| FM-M1 | fathom's own 86-feature scorecard re-derived; evidence columns match the hand-built one (the committed oracle `docs/assessments/data/2026-09-27-feature-scorecard.json`) on at least 90% of cells |
| FM-M2 (deferred, F51) | verdict-proposal agreement at least 0.8 at n at least 3 against the owner's 86 verdicts, or the proposal path is dropped; not read until J0 passes |

## 13. Refused, and not worth it now

- Feature-to-customer-ticket or CRM joins — the connector non-goal.
- Feature-to-revenue — no source exists.
- Feature-to-SLO/alerts/runbooks — Review Kit's ops domain; fathom supplies the code side only.
- Fathom deciding or auto-applying a verdict — verdicts are authored or proposed only
  (ADR-049 decision 12, ADR-040's authority boundary).
- A hand-maintained feature list or status column — the same drift every hand-kept table in
  this repository has shown (ADR-054's precedent); every rendered field traces to a computed
  join or an authored citation.
- Ranking owners by output — the owners column is CODEOWNERS plus a committer count, never a
  productivity figure keyed by a person (I58, carried over from work intelligence).

# 39 - The feature map, and whether review moves into fathom

Status: accepted 2026-09-28 (ADR-055). Owner decision memo of 2026-09-28 — the owner
delegated the call. Design: `docs/design/feature-map.md`. Plan: `docs/plans/feature-map.md`
(FM-, wave 5). Goal G26 in `docs/intent/scope.md`. Sequenced after
`docs/plans/system-graph.md#SG-2` (contract-key storage) and work-intelligence's U1/U2 rows.

## 1. The question

The owner published a Feature Scorecard: 86 of fathom's own features, each with a verdict
(keep, improve, merge, ditch) and the evidence behind it (2026-09-27;
summarized in `docs/assessments/2026-09-27-feature-scorecard.md`). Four questions followed: should the scorecard itself become a fathom feature;
who or what overrides its criteria; is the authored spec (ADRs, `docs/features.md`) the
source of truth; what maps to what, and is each mapping worth building. A fifth, bigger
question sat under all of them: should fathom become the code review tool and absorb Review
Kit (`platform-engineering/kits/review-kit`, internal name `leadsman`) — the sibling tool
that already does standards, dispositions, admission and signed execution, and works whether
or not fathom is installed.

## 2. The feature map: yes, as a fathom feature, bounded to evidence (D1)

The scorecard's power was never the verdict column. It is the joined evidence sitting
behind each verdict — usage, proof, tests, bench readings, cost — and fathom is the only
tool in the fleet that holds all of that in one place. So fathom computes the feature map.
It does not decide the verdict.

**What a feature is.** A named capability anchored to the same surface keys conservation
already uses: `cli:`, `mcp:`, `route:`, `rpc:`, `page:`, and public API symbols (ADR-044 and
ADR-050's key grammar). The source of truth is an authored `docs/features.md` — rows shaped
like a plan file's (id, name, keys, owner, the ADR or spec it cites) — plus candidates the
surface inventory proposes on its own. Every inventory key must land in exactly one feature
or one candidate; a closure check reports anything unmapped, the same discipline ADR-054
uses for the support matrix.

**What is computed, never authored.** For every feature: the code its keys reach; which
specs, plan rows and ADRs cite it, and whether those rows read proven, open or regressed;
its tests and live certificates, risk-weighted; usage, where a usage source exists (fathom's
own call and hook logs; for a service, an imported access-log or analytics file bound to a
commit) — and "unknown", never a guess, where none does; cost and the debt interest rate per
feature; churn; owners from CODEOWNERS and commit history; linked docs; the flags and config
it reads; its dependencies; and incidents that name it. None of this is a new engine — it is
the same joins ADR-051 already licensed, re-keyed by feature instead of by zone or file.

**How a health score is computed without becoming an opaque score.** Implementation health
comes from a published rule table, the same shape as the change-risk band (CX-2): the
**strongest** matched reason wins — a feature with no owner and no live certificate on a
spec-tier link is "weak" even if everything else about it looks fine — and every band is
shown with the reasons that produced it, never a number nobody can argue with. Value is a
different axis: it needs a usage signal or a human judgment, and where neither exists the
feature is marked "not measured," not silently scored zero.

**Who decides the verdict.** A human, writing a cited claim into `docs/features.md`. A decision-port
proposal, calibrated against the owner's own 86 labeled verdicts from 2026-09-27, is deferred by the
owner's decision of 2026-09-29 (not refused): it returns only after J0 passes and that calibration
reaches the bar (future F51). The calibration bar is an agreement rate of at least 0.8 at three or more
samples; below that, the proposal path is simply dropped, the same discipline that killed
the route-site calibration in ADR-042. Neither path auto-applies anything. A "ditch"
proposal does not delete code; it flows into the same surface-reckoning pattern ADR-053 used
— callers proved, citing rows re-anchored or explicitly refused.

**Who owns the overrides.** What counts as value, what the weights and thresholds are, and
what extra considerations apply — that is a Standard, and authoring Standards is Review
Kit's job (a `product-portfolio` catalog). Fathom ships a small local fallback,
`.fathom/features.cue`, the same shape as the risk table, with sane defaults in the binary.
The authored `docs/features.md` stays the spec of record either way.

**Where it surfaces.** `fathom features [--json]`, a console page (a value-by-implementation
scatter with verdict shapes, a filterable table, an underused list, a ditch list), and an
export that reproduces the Feature Scorecard artifact as its first reading.

**Sequencing.** Wave 5, after system-graph storage (SG-2, itself gated on the responsible-
tenant budget) and the work-intelligence cost rows (U1/U2), because the feature map's cost,
dependency and config columns are joins over exactly those tables.

## 3. What maps to what, and whether it is worth it (D2)

| Join | Worth it now? | Where the evidence comes from |
|---|---|---|
| feature to code | Yes | `blast_radius`/`system_impact` over the feature's keys |
| feature to spec, plan rows, ADRs | Yes | plan-grammar citations, ADR trailers |
| feature to tests and certificates | Yes | T1/T2 risk-weighted coverage |
| feature to cost and debt interest | Yes | U1/U2, re-zoned to the feature's keys |
| feature to usage | Yes, where a source exists | call/hook logs; an imported service access-log or analytics file; else "unknown" |
| feature to owners | Yes | CODEOWNERS plus commit history |
| feature to docs | Yes | knowledge-retrieval's doc-to-code links |
| feature to flags and config | Yes | system-graph `env:`/`config:` keys |
| feature to dependencies | Yes | system-graph `pkg:` keys |
| feature to incidents | Yes | `docs/intent/incidents.md` citations |
| spec to code | Planned elsewhere, reused | `fathom verify` |
| rule to code | Planned elsewhere, reused | `obligations` |
| test to behaviour | Planned elsewhere, reused | certificates |
| service to service | Planned elsewhere, reused | the system graph |
| decision to code | Planned elsewhere, reused | the architecture drift radar (U11) |
| feature to customer tickets or CRM | No | the connector non-goal |
| feature to revenue | No | no source exists |
| feature to SLOs, alerts, runbooks | No | Review Kit's ops domain; fathom supplies the code side only |

The value nothing else gives: a portfolio view (health by value by cost) in one place;
computed ditch candidates with their evidence attached instead of asserted; feature
regressions; release notes generated by feature instead of by commit; a list of features
nobody owns, tests or documents; and a per-feature read of maintenance interest instead of a
per-file one.

## 4. The review question: merge, stay separate, or a middle path (D3)

Three shapes were on the table.

**A. Merge Review Kit into fathom.** Absorb its standards, dispositions, admission, signed
execution and hosted view into fathom's own codebase, and answer "should fathom be the
review tool" with a straightforward yes.

- *Pros:* one install, one experience; the ask is answered literally.
- *Cons:* fathom's own invariant (ADR-040) is that it never approves, disputes, tailors or
  closes a finding — admission with live forge approval and dispositions like dispute/accept/
  tailor are exactly that authority, by design. Review Kit's hosted view and org-level
  features (DATUM, well-architected, security posture, economics, practice, org controls)
  collide with fathom's non-goals and its tenancy budget (ADR-047). Review Kit's own design
  rule — it works with no fathom installed — is what lets an organization adopt it without
  ever standing up the index; folding it into fathom breaks that promise. And fathom is in
  the middle of shrinking its own surface (ADR-053, eight tools by default); absorbing
  roughly 170 Go files of a different domain reverses exactly that discipline.

**B. Stay fully separate, no formal contract.** Leave the two tools to be used together
ad hoc, as today.

- *Pros:* cleanest authority boundary; zero new work.
- *Cons:* it leaves real value on the table. Fathom is the only tool holding the joined
  evidence — usage, cost, proof, tests per requirement — a review judgment needs, and without
  a stable contract Review Kit either re-implements code-aware capabilities fathom already
  has or does without them. It also does not answer the owner's real want, which is closer to
  "one thing to install," not "two unrelated tools."

**C. One family, two tools, one contract (chosen).** Fathom becomes the best evidence engine
for review and gets a first-class, code-grounded, per-change surface (`fathom review`); the
organizational assurance system — standards, dispositions, admission, signed execution, the
non-code domains, the hosted view — stays in Review Kit, consuming fathom through a versioned
contract.

- *Pros:* keeps fathom inside its own invariant and non-goals; keeps Review Kit's
  independence rule intact for adopters who never install fathom; gets most of "one install"
  through packaging (the family bundle below) without merging code; each tool keeps shrinking
  or growing inside its own lane instead of one growing at the other's expense.
- *Cons:* two things to install and keep compatible instead of one — mitigated by the bundle
  and tracked by a revisit trigger if the seam turns out to cost more than it saves.

**Why C.** Five reasons, in order: authority (ADR-040's admission/disposition boundary is not
negotiable), hosted-and-org conflict (fathom's non-goals and tenancy budget), the
independence rule (Review Kit's whole value to a fathom-less adopter), scope (fathom is
shrinking, not growing, ADR-053), and distribution (the actual thing the owner wants — one
install, one experience — is solvable by packaging, not by merging two different domains'
code).

## 5. What moves where

| Into fathom (code-grounded, per change, computed) | Stays in Review Kit (organizational assurance) |
|---|---|
| the change risk receipt (U7) | standards and catalogs |
| SARIF and check-run output | requirement severity |
| the feature map | dispositions (dispute, accept, tailor) |
| conservation | admission with live forge approval |
| certification facts | signed execution |
| the architecture drift radar (U11) | infra, ops, well-architected, security posture, economics, practice, org controls |
| `fathom review`, the code-review surface | DATUM, the hosted view |

## 6. The family-bundle idea

Instead of one codebase, one installer and one plugin that installs fathom plus leadsman
together, under shared nautical naming (fathom, leadsman, sounding, datum) and a shared
licence (FSL). It gets an adopter most of what a merge would have bought — one install, one
coherent story — without either tool taking on the other's authority or domain.

Alongside the bundle: a versioned evidence contract (the fathom uplift brief's P0) so every
lane Review Kit consumes from fathom is stable and receipted, not read off internals; Review
Kit's own code-parsing may optionally consume fathom when it is present, while keeping its
fathom-less path working; and Review Kit's concern packs keep compiling into fathom
obligations at the edit, which they already do.

## 7. Revisit triggers

The merge answer is "not now," not "never." Revisit it when any of these fires:

- At least 70% of Review Kit's requirements evaluated in a quarter depend on a fathom
  evidence lane.
- At least 5 adopters exist and every one of them installs both tools.
- The evidence contract breaks more than 3 times in a quarter (the seam is costing more than
  it saves).
- The owner decides to sell one product — and even then, bundle first, merge code last.

## 8. Jev, weighed

**Helps:** proposing a verdict (a Choice over keep/improve/merge/ditch) given a feature's
computed evidence row, calibrated on the owner's 86 labeled verdicts.
**Does not help:** computing any evidence column or the health score — those stay
deterministic, joins over existing facts, never a model's guess.
**Cost:** one decision-port call per feature, on demand, never in a request path.
**Measurement:** agreement at least 0.8 at n at least 3 (`FM-M2`); below that, the proposal
path is dropped, not tuned. Deferred 2026-09-29: FM-10, FM-11 and FM-M2 are `[-]` until J0 passes
and the calibration against the 86 verdicts can be read (F51).

## 9. The owner's follow-up: where Review Kit eventually lives

Asked separately whether Review Kit should move out of the `platform-engineering` monorepo
into its own repository beside fathom, the owner answered on 2026-09-28: yes, eventually, but
not now. This is recorded as a decided future rather than an open question —
`docs/intent/futures.md` F33, confidence likely, horizon later — promoted when the family
bundle ships, or Review Kit's release cadence diverges from platform-engineering's, or a
second adopter installs it standalone; dropped if it stays tied to monorepo-only consumers.

## 10. How this fits what is decided

The feature map is a projection, exactly like the system graph (ADR-050) and the support
matrix (ADR-054) before it — no new engine, evidence-bound, never a verdict authority. The
review decision extends, rather than revisits, ADR-040's authority boundary: fathom still
never approves, disputes, tailors or closes anything; it now also computes the feature map
and a first-class review surface without taking on Review Kit's job. Full record: ADR-055.
Mechanism: `docs/design/feature-map.md`. Work: `docs/plans/feature-map.md` (FM-), gated by
goal G26 in `docs/intent/scope.md`.

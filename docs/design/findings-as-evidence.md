# Findings as evidence — from review output to investigable precedent

**Status:** adopted design, owner-commissioned 2026-07-30 ("fathom would be a
good place for a well architected review findings storage and those findings
would actually add value to the expert system especially for realtime
investigations… same thing with code review findings"). Registered as phase R
in `docs/vision/13-expert-suite.md` §9. Repo verdict for this work:
`docs/handoff/repo-strategy.md` §2 — all of it is packages in fathom.

## 1. The finding this design starts from

The owner's intuition is right, and sharper than it first appears: **review
findings are the feedstock for the incident-investigation expert module.**

`docs/design/product-vision.md` §3 rates incident investigation "world-class
*later* — entry-conditioned: the corpus is ~5 resolved observations today;
compiling on it would fail our own precision gate," with entry at 15–20
resolved findings. Meanwhile the pre-push review hook runs `review_diff` on
every push and persists findings as governed claims. **The blocked module and
the running channel are looking at the same corpus, and nothing has said so.**

So the work is not "store findings" — that already happens (`kind:"finding"`
claims with line-hashed citations, supersede-on-fix reconciliation, the
`code_drifted` remediation flag). The work is the **promotion path** from a
finding to something worth consulting during an incident, and the **query**
that makes it worth consulting.

## 2. What is missing (each item is a deliverable)

**A. Findings are free text, so recurrence is invisible.** Five hundred
findings is a log; "this failure class appears at eleven sites, seven fixed,
four live" is a playbook. Without a typed class there is nothing to cluster
on, and clustering is where all the value is.

**B. Resolution outcome is not recorded.** Supersede fires when review runs
over the same area again — incidental, and it captures *that* a finding
stopped applying, never *how*. Precedent without a resolution is trivia.

**C. There is no tier story, and the layer is ungated.** `findings` is
`not-gated` in the catalog today. Compiling expert knowledge from ungated
LLM output is the exact "eval-perfect ≠ production" trap this program has
caught twice. Raw findings must stay low-tier.

**D. The query that only fathom can answer does not exist.** Everyone can
text-search past issues. Only fathom knows that the handler you are staring
at during an incident is two call-hops from a function that carried three
findings last quarter, one still active. **Prior art in the call-path
neighborhood** needs no new storage — just a traversal over data already in
the store.

**E. An unaudited counter sits under all of it.** `research report` printed
`review_active_findings = 251783` on a repo with ~250 real findings
(observation O20, open). Nothing load-bearing may be built on findings
storage until that number is explained, because it means nobody has audited
how findings are written or counted.

## 3. Design (normative)

### 3.1 Sequence: audit before building (E first)

O20's counter is resolved *before* any of the rest lands. If findings are
being double-written, re-counted per call, or never marked inactive, every
downstream number inherits it. The fix ships with a test pinning the count
against a fixture store of known size.

### 3.2 Typed failure classes (A)

Every finding carries a `class` from a small, closed, CUE-validated
vocabulary — deliberately coarse, because a taxonomy nobody can apply
consistently is worse than none. Starting set, revisable by evidence:

`correctness` · `contract-violation` · `resource-leak` · `error-handling` ·
`concurrency` · `security` · `config-drift` · `test-gap` · `performance` ·
`conformance` (an architectural rule broken)

The reviewer assigns the class at emit time; an unrecognized class is a
compile-time rejection (the `over-broad` precedent from O19's rejection
taxonomy), never a silent "other".

### 3.3 Patterns: recurrence as a first-class object (A)

A **pattern** is a derived, deterministic grouping of findings sharing
`(class, normalized-rule-shape)` across ≥2 distinct sites. It is computed,
never authored, and stores: the class, the member findings' citations, per-
member status (live / superseded / refuted), the resolving commits where
known, and first/last-seen watermarks. Patterns are re-derived on demand from
claims — no second source of truth, no drift.

### 3.4 Resolution outcomes (B)

When reconciliation supersedes a finding, record *what resolved it*: the
commit, and whether the resolution was a code change in the cited scope, a
refutation (a human said it was wrong), or scope disappearance (the code was
deleted). Three cases, distinguishable from data already in hand, and the
difference is the whole value: "fixed in a1b2c3d" is precedent, "the file was
deleted" is not.

### 3.5 Tier promotion — the only path upward (C)

Raw findings stay at their emitted tier (`sme-stated` at best; LLM-authored
ones lower). A **pattern** may be promoted exactly one tier when it has ≥3
members with recorded, independent code-change resolutions — because at that
point the claim "this failure class recurs and gets fixed this way" is
supported by resolved evidence rather than by a reviewer's opinion. Promotion
is deterministic and recomputed, never sticky: if members are refuted, the
tier drops back. **The findings layer must pass an eval gate before any
pattern serves anywhere** — the gate that does not exist today.

### 3.6 The neighborhood query (D) — the differentiating verb

`fathom findings near <symbol|file> [--hops N] [--include-superseded]`, and
the same shape as an MCP tool on the investigation surface: findings whose
cited scope lies within N call-graph hops of the subject, ranked by
proximity then recency, each carrying class, status, resolution, and pattern
membership. Reuses the existing traversal; adds no storage. This is the
answer to "has anyone seen this before?" that no competitor can give, and it
is the reason findings belong in this store rather than a tracker.

### 3.7 The playbook module's entry condition, made visible

`fathom findings status` reports the corpus against product-vision's entry
bar: resolved-finding count, distinct classes, patterns with ≥3 resolved
members. The blocked module gets a progress bar instead of a memory, and the
bar is read from data, not asserted.

## 4. Honesty rules

1. A finding is a claim about a moment; `code_drifted` stays an annotation,
   never a status flip (drifted code may mean *fixed*).
2. Patterns are derived and recomputed — never hand-authored, never cached
   as truth.
3. Promotion is evidence-gated (§3.5) and reversible.
4. `findings near` returns superseded findings only when asked, and always
   labeled — precedent includes what got fixed, but must never read as live.
5. No count ships without a test pinning it to a fixture of known size (§3.1).

## 5. Non-goals

Not an issue tracker (no assignees, no workflow states beyond the claim
lifecycle). No cross-repo finding propagation in v1 (a finding is repo-scoped
context; the platform-module binding path is the eventual mechanism, on
evidence). No LLM at query time — the reviewer is the only LLM step, exactly
as today. No auto-filing to a forge (that is the delivery surface in
`docs/handoff/repo-strategy.md` §2, trigger-gated).

## 6. Deliverables (phase-R plan rows derive from here)

1. O20 audit + fix + fixture-pinned count test (**first, blocking**).
2. `class` on findings: CUE vocabulary, reviewer emits it, unknown rejected.
3. Pattern derivation (deterministic, tested, recomputed on read).
4. Resolution outcomes recorded at reconciliation, three cases distinguished.
5. Eval fixtures + gate for the `findings` layer; promotion rule behind it.
6. `fathom findings near` + the investigation-surface MCP tool.
7. `fathom findings status` reporting the entry condition from data.
8. Docs: the incident-investigation use-case guide gains the neighborhood
   query; `docs/layers.md` records the findings layer's new gate status.

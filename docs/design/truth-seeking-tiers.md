# Truth-seeking tiers — confidence is capped by what actually verified it

**Status:** adopted design, owner-commissioned 2026-07-30 ("we should lean
towards truth seeking so make confidence score is capped if there is not hard
evidence — not docs — not a prompt. hard evidence. agree?"). Answer: agreed,
with three sharpenings and one live gap this review uncovered. Registered as
phase S in `docs/vision/13-expert-suite.md` §9; introduces law 14.

## 1. The gap that makes this urgent

The ceiling function **already exists** and fathom **never calls it.**

`foundations/review/scoring.ApplyCeiling(confidence, tier)` caps
doc-claimed and sme-stated at 2, assumed at 1. Searching fathom's own source
for callers returns only the vendored copy of the function itself — zero
call sites in fathom code. The tier ceilings are documented in
`internal/core/model.go`'s comments as if enforced ("ceiling 5", "ceiling 2",
"ceiling 1") and are enforced nowhere.

This is the program's recurring failure mode — built machinery that was never
switched on (the vision-11 finding, again). The doctrine below is not new
philosophy; it is turning on a cap the codebase already believes in.

## 2. Law 14 — confidence is capped by verification depth, not by assertion

**A claim's confidence may never exceed the ceiling of the weakest evidence
that produced it. Documents and model output are assertions, not
verifications, and no quantity of either raises a ceiling.**

Corollaries, each enforced in code:

- **Compilation cannot manufacture confidence.** An LLM-compiled claim's tier
  is bounded by its *input's* tier. A model reading a doc produces at best a
  doc-claimed claim, never a verified one. Aggregation does not launder:
  three doc-claimed sources agreeing is still doc-claimed.
- **Recompilation is not corroboration.** Running the same prompt again on
  the same corpus produces the same tier, not a higher one.
- **The cap is structural, not advisory.** Enforced at the write path, so a
  future caller cannot forget it (the precedent: `PromoteTier` structurally
  refuses whenever a layer is not eval-gated — it does not trust its
  callers).

## 3. The sharpening: two axes, because one tier scale is answering two questions

The existing five-tier scale conflates two genuinely different claim kinds,
which is why `doc-claimed` feels simultaneously too generous and too stingy:

**Factual claims** ("`Foo` calls `Bar`", "this file imports CGO") are about
what *is*. Their ceiling is verification depth:

| Evidence | Ceiling | Why |
|---|---|---|
| executed (CI green, runtime trace, a test that actually ran) | highest | reality confirmed it |
| compiler/type-resolved (typed call edges from a real type checker) | high | machine-checked, not pattern-matched |
| parsed (AST: the symbol exists, the call site exists) | medium | true of the source text |
| pattern-matched (heuristic extractors) | low | pre-scan, by construction |
| doc-claimed / model-asserted | capped low | nobody checked |

**Normative claims** ("`internal/store` stays pure Go", "CUE owns intent")
are about what *ought*. No execution proves an obligation — obligations are
*decisions*, and their evidence is that an authority wrote them down.
Ceiling them at doc-claimed forever would be epistemically tidy and
practically wrong: it would report low confidence that a rule *is* the rule,
which was never the question.

So normative claims are tiered by **authority + freshness + enforceability**,
and the top tier has a precise meaning that is *not* "a doc says so twice":

> A normative claim reaches the top tier only when a **machine check
> enforces it** — a test, a lint, a conformance rule, a gate that fails when
> the rule is broken.

That is the honest promotion path, and it is exactly this program's own
doctrine: forced channels beat assertions; a rule nothing enforces is a
wish. It also gives every governed rule a concrete ladder — *write it,
cite it, then gate it* — instead of a permanent ceiling.

## 4. What this changes in practice

- Obligations/adr-rules/spec-bindings claims: stay doc-claimed (correct
  today), and become promotable **only** by pointing at the check that
  enforces them.
- Findings: the phase-R pattern promotion already requires ≥3 recorded
  code-change resolutions — that is evidence of the harder kind, and it
  survives law 14. Its eval gate remains a precondition.
- Symbols and call edges: typed edges (from a real type checker) legitimately
  outrank AST-parsed ones; today both are flatly `verified`. Splitting them
  is a truth improvement the tier scale can already express.
- Any consumer rendering a confidence number must render the ceiling and the
  reason beside it. A capped score with no visible reason reads as a bug.

## 5. Non-goals

No new tier *names* (the five-value vocabulary is load-bearing across the
store, the kit, and the spec — changing it would be a migration, not an
improvement); the axes in §3 are how the existing values are *assigned*, not
replacements for them. No confidence on refusals (a refusal is not a weak
claim — it is the absence of one). No runtime-evidence ingestion in this
phase (`runtime_pass` stays `unknown` until CI evidence lands, exactly as the
spec-verification honesty boundary already states).

## 6. Deliverables (phase-S plan rows)

1. Call `ApplyCeiling` at every claim write path; a test proving a
   doc-claimed claim cannot be persisted above its ceiling regardless of what
   the caller passes.
2. A structural guard: the write path derives the ceiling from the claim's
   own tier rather than trusting a caller-supplied confidence.
3. Split parsed vs type-resolved evidence for symbol/edge facts so the
   stronger machine check is distinguishable from the weaker one.
4. The enforceability promotion for normative claims: a claim may record the
   check that enforces it; only then may it hold the top tier; removing the
   check drops it back (recomputed, never sticky).
5. Render the cap: every surface showing confidence shows the ceiling and
   why.
6. Docs: the tier table in `internal/core/model.go` stops claiming ceilings
   that nothing enforced; `docs/design/semantic-layers.md` records law 14.

// findings is the D3 review-findings tracer bullet (docs/plans/
// findings-tracer.md FT-1, docs/vision/03-findings-and-experts.md §3.2): one
// markdown file per review/audit finding in a corpus directory, compiled
// through the same engine as obligations but under kind "finding" and the
// honest sme-stated tier — a human review conclusion is not a code-derived
// fact, and the shipped tier ranking already ranks it below
// verified/doc-claimed evidence.
//
// LC-4 pack shape: plain data, unified against #Layer (schema/semantic/
// schema.cue) by internal/semantic's loader — see obligations/layer.cue's
// doc comment for the convention.
name:    "findings"
version: "v1"
kind:    "finding"
corpus: include: ["findings/**"]
// Advisory only — see #Layer.prompt_version's doc comment in schema.cue. The
// real identity is content-addressed at runtime (internal/semantic/prompts.go).
prompt_version: "findings/content-addressed"
prompt_file:    "schema/semantic/layers/findings/prompt.md"
// tier of claims the LLM engine compiles from the findings/** corpus: a
// human review conclusion restated by a model is sme-stated evidence.
//
// Verified-tier admission (ADR-016 M1 amendment / ADR-018 oracle classes):
// this layer ALSO carries deterministic-oracle claims written by the
// review-verb findings bridge (internal/analysis/review, PromptVersion
// "review-verb/v1", Model empty) — those are stamped tier "verified" at the
// source, because a deterministic analyzer's own output needs no human or
// model restatement. Mixed tiers on this layer are therefore deliberate and
// admitted here, not drift; TestClaimsVocabularyMatchesSemanticLayer
// (internal/analysis/review) pins this admission text against the bridge's
// constants.
tier: "sme-stated"
// concern: fathom-authored, eval-gated-embedded — see obligations/layer.cue
// for the full stratum reasoning (ADR-024, docs/design/pack-taxonomy.md).
stratum: "concern"
budgets: {
	max_tokens_per_run: 200000
	max_usd_per_run:    1.0
}
// A finding's remediation lives in scoped CODE, not just the cited finding
// document — remediation drift (docs/vision/03-findings-and-experts.md §2,
// FT-6) is the reason this layer exists.
serving: drift_tracked: true

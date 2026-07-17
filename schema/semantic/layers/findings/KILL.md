# findings — kill gate

**Status: graduated, not under active kill review.** Findings is the D3
review/audit-findings tracer bullet (docs/plans/findings-tracer.md FT-1,
docs/vision/03-findings-and-experts.md §3.2) and the first drift_tracked
layer (LC-2): remediation-drift anchoring (scope-fp links, FT-6) is proven
in production via `fathom report` + `fathom claim`
(docs/use-cases/change-review.md).

**Metric:** claims-compiled and drift-flag rate per service
(`ClaimsSummary`), and human refute/corroborate call volume (`fathom claim`)
— tracked via the store, not this file.

**Revival/kill trigger:** none scheduled. `eval/` fixtures for this layer are
not yet seeded (findings corpora vary per project — the built-in prompt has
not yet needed the same fixture-driven precision gate obligations required,
since findings restate a human's own review conclusion rather than infer
scope from open-ended prose); a future contributor adding
`eval/{labels.json,*.md}` here would let `TestObligationsEval`-style gating
extend to this layer too.

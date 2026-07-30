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

**Revival/kill trigger:** none scheduled. `eval/{labels.json,corpus.md}`
were seeded by docs/design/findings-as-evidence.md's own §6 item 5 (the
promotion rule behind the findings layer's eval gate, `fathom findings
status`'s entry-condition work) — 6 labeled findings across 6 distinct
classes plus 2 traps (a resolved finding, a background-context section)
that must produce no claim, mirroring adr-rules' own fixture shape
(`eval/RESULTS.md`). The gate has NOT yet been run against a real model
(`TestFindingsEval_Precision`, `internal/semantic/eval_test.go`, skipped
without `FATHOM_EVAL_LLM=1` — see `eval/RESULTS.md` for the exact command
and its current "not yet run" status); `layer.cue`'s `evaled` stays
unflipped (defaults false) until a real run passes.

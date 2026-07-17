# obligations — kill gate

**Status: graduated, not under active kill review.** Obligations is the v1
tracer bullet (ADR-016 §4.8 #1) and the admission-rule proof case
(docs/vision/02-shape.md §5): it already cleared its own eval gate
(precision ≥0.90 / recall ≥0.75, `eval/labels.json` + `eval/{standards,
security}.md`, run via `TestObligationsEval_Precision`) and has a wired
delivery channel (the PostToolUse edit-hook injection, ADR-016 §4.5).

**Metric:** injection coverage on governed-file edits (docs/design/
semantic-layers.md §4.6's post-shipping gate) — tracked via
`mcp-calls.jsonl` / hook telemetry, not this file.

**Revival/kill trigger:** none scheduled. A future kill would require the
3-week generalize-or-kill telemetry (§4.6) to show call-rate or quality
failure; see `docs/design/semantic-layers.md §4.6` for the exact bar.

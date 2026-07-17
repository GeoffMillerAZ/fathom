# adr-rules — kill gate

**Status: shipped 2026-07-17, entering the 3-week generalize-or-kill
window.** adr-rules is the layer-2 proof (docs/plans/layer-catalog.md LC-5,
docs/vision/02-shape.md, docs/vision/08-roadmap.md): the first layer no
prior code was written for by name. It cleared its shipping eval
(`eval/RESULTS.md`: precision 1.00 / recall 1.00 over 2 consecutive live
runs against the `claude-cli` subscription lane) and is admitted into the
embedded set (`internal/semantic/layers/adr-rules/`).

**Metric (post-shipping gate, docs/design/semantic-layers.md §4.6):** from
`mcp-calls.jsonl`/hook telemetry — the fraction of edits to ADR/
docs/design-governed files preceded by an obligations-channel call serving
an adr-rule claim, AND an outcome signal (review findings citing an ADR
constraint the layer served vs. missed).

**Window:** 3 weeks from first fleet compile. **Decision date:** 2026-08-07.

**Kill criterion:** fails on call-rate but not quality → fold into
`outline`/`symbol_context` as a capped rider (the zero-new-habit delivery
obligations itself falls back to). Fails on quality (precision/recall
regresses under real corpus scale, or false positives prove trust-fatal in
review) → kill; the substrate cost was one pack directory, no Go removed.

**Revival evidence for anything deferred alongside this pack:**
`producers.cue` deterministic producers (LC-10) revive at the first pack
whose corpus is deterministic analyzer output wanting `verified` tier — not
this one; every adr-rules claim stays `doc-claimed` (LLM-restated,
citation-grounded), by design.

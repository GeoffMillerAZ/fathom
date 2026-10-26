# css named ceiling routes (X-R1)

Measured baseline: [css reading](../bench/readings/lang/css.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Lightning CSS browser AST | 1.33.0 | The committed adapter/README records rule start locations but no full rule-end locations. range_agreement remains null. This is the measured API limit, not a claim about every browser parser. |
| Universal Ctags CSS parser | 6.2.1 | The existing ceiling records absent CSS rule ends. It cannot supply the missing range metric. |
| PostCSS independent parser | 8.5.28 | The locally available package has source.end. Under design §1.3 it is an independent parser library, tier C, so it cannot make L1 excellent. No owner tier promotion is inferred. |
| Chromium DevTools CSS.CSSStyle.range | CSS domain protocol; browser engine | Reports a style declaration range with endLine/endColumn. It may locate the rule closing brace; whether this browser-engine surface is tier A for CSS rule ends is an owner oracle-tier question. Not run on the pinned corpus. |
| Lightning CSS nesting traversal | 1.33.0 / O849 | The spent holdout includes withdrawn @nest syntax the adapter does not walk. A future oracle revision must walk it before another holdout; no post-holdout tuning is performed here. |

The existing end-location trigger has fired literally: PostCSS exposes spans. Its tier-C truth cannot lift L1 to excellent under §1.3. A tier-A end oracle remains the open route; a browser engine needs an owner tier ruling.

Governing row: `LB-Q-css-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

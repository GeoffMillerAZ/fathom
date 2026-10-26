# css named ceiling routes (X-R1)

The committed reading is unchanged: level L3, grades L1 good, L2 excellent, L3 excellent, L4 not-attempted.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Lightning CSS browser AST | 1.33.0 | The committed adapter/README records rule start locations but no full rule-end locations. range_agreement remains null. This is the measured API limit, not a claim about every browser parser. |
| Universal Ctags CSS parser | 6.2.1 | The existing ceiling records absent CSS rule ends. It cannot supply the missing range metric. |
| PostCSS independent parser | 8.5.28 | The locally available package has source.end. Under design1.3 it is an independent parser library, tier C, so it cannot make L1 excellent. No owner tier promotion is inferred. |
| Lightning CSS nesting traversal | 1.33.0 / O849 | The spent holdout includes withdrawn @nest syntax the adapter does not walk. A future oracle revision must walk it before another holdout; no post-holdout tuning is performed here. |

The named browser-end oracle trigger has not fired. PostCSS is a real range route, retained explicitly with its tier cap rather than declared absent.

Governing row: `LB-Q-css-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

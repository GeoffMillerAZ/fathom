# javascript named ceiling routes (X-R1)

Measured baseline: [javascript reading](../bench/readings/lang/javascript.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| scip-typescript | 0.4.0 | OpenLayers checkJs subject is measured at typed_recall .9376. About 1,061 of 1,278 missed references target object-literal properties left in the denominator by D2. |
| Module binding extraction | O749/O858 historical experiment | It exceeded the store gate on express (+21.5%). Its counted prospective typed recall was about .948, still below .95; it was not accepted as a new reading. |
| Rule changes to property references and costs | D2, O858; ruling C-R3 (#20, 2026-10-26T01:44:25Z) | Owner keeps the current denominator, cost gates and written ceiling. A different indexer cannot by itself create missing extractor endpoints or change those rules. |
| Untyped CommonJS receiver resolution | scip-typescript 0.4.0 / W3 | Existing tuning readings have oracle-blind receiver occurrences; axios call precision .5653 and express .2733. Typed OpenLayers proof is not generalized to untyped code. |

Ruling C-R3 (#20) retains the current ceiling. Retry only the existing rule/cost trigger on fresh independent validation.

Governing row: `LB-Q-javascript-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

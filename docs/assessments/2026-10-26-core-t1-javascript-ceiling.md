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

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / parser | [TypeScript compiler API](https://github.com/microsoft/TypeScript-wiki/blob/main/Using-the-Compiler-API.md) and [scip-typescript JS inferred config](https://github.com/sourcegraph/scip-typescript) were searched. They do not change C-R3 denominator/cost rules. |
| SCIP / LSIF | [scip-typescript 0.5.2](https://registry.npmjs.org/@sourcegraph/scip-typescript/latest), [Git head ad3ca370](https://github.com/sourcegraph/scip-typescript/commit/ad3ca370efb05d305714c4bc859237ea701e40a4), postdates the measured 0.4.0 lane; property/kind and legacy-range behavior is **LIVE UNMEASURED** on unchanged OpenLayers/express/axios scopes. [lsif-node](https://github.com/sourcegraph/lsif-node) is historical, not a fresh indexer. |
| Language server | [TypeScript service in JavaScript mode](https://github.com/microsoft/TypeScript/wiki/Using-the-Language-Service-API) exposes reference requests. **LIVE UNMEASURED** against the same object-property and untyped receiver denominators. |
| Registry AST | [TypeScript 7.0.2 registry](https://registry.npmjs.org/typescript/latest) names a current parser/typechecker, not an extractor fix. |
| Runtime tracing | [Node diagnostics](https://nodejs.org/api/diagnostics_channel.html) and module loading see executed CommonJS paths only; no complete static receiver truth. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.

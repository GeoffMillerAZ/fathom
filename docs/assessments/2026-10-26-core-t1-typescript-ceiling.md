# typescript named ceiling routes (X-R1)

Measured baseline: [typescript reading](../bench/readings/lang/typescript.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| scip-typescript | 0.4.0 | Existing tier-A reading already exposes missed rxjs object-literal methods. sym_recall .9358 is a real extractor gap, not missing oracle truth. |
| Object-literal methods query | TypeScript grammar v0.23.2 / f975a621; main 5b84aa5b4 | Not implemented in the shared tags query. It affects TS and hosted frameworks, requiring ENGINE, K26 cost proof and all affected K29 readings. No dialect-only shortcut is available in this wave. |
| Denominator or wrapper-component matching change | D2, D3, S3 owner rules | Object/type-literal references remain in typed recall; wrapper-call bindings lack D3 admission. Changing either rule requires an owner ruling. Existing written trigger has not fired. |

Keep the existing deferred row. Retry its named query or owner-rule routes with cost evidence and every affected reading. No assertion that all future implementations are impossible.

Governing row: `LB-Q-typescript-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / parser | [TypeScript compiler API](https://github.com/microsoft/TypeScript-wiki/blob/main/Using-the-Compiler-API.md), [language service](https://github.com/microsoft/TypeScript/wiki/Using-the-Language-Service-API), [registry TypeScript 7.0.2](https://registry.npmjs.org/typescript/latest): searched compiler AST and object-literal methods. A new compiler pin has no measured query/cost result. |
| SCIP / LSIF | [scip-typescript registry](https://registry.npmjs.org/@sourcegraph/scip-typescript/latest) now reports **0.5.2**, [Git head ad3ca370](https://github.com/sourcegraph/scip-typescript/commit/ad3ca370efb05d305714c4bc859237ea701e40a4); [indexer source](https://github.com/sourcegraph/scip-typescript) covers property/kind and legacy-range behavior. Existing lane measured 0.4.0. **LIVE UNMEASURED**: 0.5.2 on the same typed denominator and object-method gap; old 0.3.x release notes do not reopen it. [Kythe TypeScript indexer at 69141f02](https://github.com/kythe/kythe/tree/69141f022689a611e8a4a1d9b08a3783a2e8a9ed/kythe/typescript), repository updated 2026-09-18, is a separate maintained cross-reference route; **LIVE UNMEASURED** on the frozen subjects. |
| Language server | [TypeScript language service references](https://github.com/microsoft/TypeScript/wiki/Using-the-Language-Service-API) can answer references but has no frozen gap-specific or cost measurement here. **LIVE UNMEASURED**. |
| Registry AST | [npm TypeScript 7.0.2](https://registry.npmjs.org/typescript/latest) is a version hit, not evidence of a changed Fathom tag query or reference recall. |
| Runtime tracing | [Node tracing/coverage](https://nodejs.org/api/trace_events.html) records executions and module activity, not every object-literal declaration or call. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.

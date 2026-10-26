# svelte named ceiling routes (X-R1)

Measured baseline: [svelte reading](../bench/readings/lang/svelte.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Svelte own compiler parser | svelte/compiler 5.57.2 | Existing first-party reading already measures components: symbol precision .3816. It parses let declarations as variables while the shared dialect names constants under M1/S3a. |
| Shared JS/TS canonical kind change | M1/S3a/K19; ruling C-R1 (#20, 2026-10-26T01:44:25Z) | Owner keeps the current kind rules and ceiling. Canonical-variable changes would affect JS/TS plus all hosted frameworks and require their K29 regrades. |
| Component-specific kind admission | K19 current rule / C-R1 | Changing the intersection only for components is an owner-rule change. The owner has not authorized it. It is not hidden inside an oracle adapter. |
| Component-to-TS file universe | current hosted file resolver / O979 | The written limit for imports to .ts targets remains. The measured host-only file edges are not proof of ingest edges to those files. |

Ruling C-R1 (#20) retains the existing ceiling. The brief unmeasured label was wrong: the committed compiler reading already exists.

Governing row: `LB-Q-svelte-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / parser | [svelte/compiler](https://svelte.dev/docs/svelte/svelte-compiler), registry **5.57.2** ([npm](https://registry.npmjs.org/svelte/latest)), is the same first-party compiler measured by the current reading. The Svelte team's [svelte2tsx 0.7.62](https://www.npmjs.com/package/svelte2tsx) is its component-to-TSX transform behind svelte-check and the language server, **LIVE UNMEASURED** as an independent component-reference route. The C-R1 kind-policy limit remains. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-svelte` search](https://github.com/search?q=scip-svelte&type=repositories) yielded no measured separate indexer. Negative search does not exhaust future tools. |
| Language server | [Svelte language server](https://github.com/sveltejs/language-tools), registry **0.18.5** ([npm](https://registry.npmjs.org/svelte-language-server/latest)), has navigation services. **LIVE UNMEASURED** for source-aligned component references; it does not waive C-R1 canonical-kind policy. |
| Registry AST / scope analysis | [Compiler AST API](https://svelte.dev/docs/svelte/svelte-compiler) is already in the first-party reading; no new 5.57.2 AST result is claimed. [svelte-eslint-parser 1.10.0](https://www.npmjs.com/package/svelte-eslint-parser) provides references and variables, **LIVE UNMEASURED**, tier C under P-T1. It does not change C-R1 kind policy or C-R4 extractor-side misses. |
| Runtime tracing | [Svelte $inspect.trace](https://svelte.dev/docs/svelte/$inspect) observes executed reactive code, not the complete declaration/call graph. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.

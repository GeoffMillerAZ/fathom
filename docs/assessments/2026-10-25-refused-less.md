# Less: measured parser limit and X-R4 source audit

Checked 2026-10-25. Source inspection only, with no new corpus grade. These
searches distinguish usable compiler/runtime oracles from the product parser
gate. No corpus shard or plan checkbox changed.


**Existing product pins and measurement.** `jimliang/tree-sitter-less` `e5ae6245f841` (MIT) had ERROR/MISSING in 97/146 `.less` files; `rhino1998/tree-sitter-less` `2bd739e106a3` (MIT) had 42/146. Both exceed 5%; this only rejects those product grammars. Official `less/less.js` v4.9.1 is the named parser/import oracle candidate, **unrun** on these subjects.

| Required route | Source, exact query/list checked 2026-10-25 | Result and limitation |
|---|---|---|
| Own compiler/runtime AST/introspection | [Less programmatic usage](https://lesscss.org/usage/), query `less.render`, `output.imports`, `filename`; [Less parser issue with maintainer discussion](https://github.com/less/less.js/issues/2777), query `less.parse`, `processImports` | Pin Less.js v4.9.1. `less.parse` returns the native AST, while `less.render` resolves imports and returns imported filenames. This is a viable tier-A import oracle and potential definition/mixin AST oracle. AST shapes/positions and corpus acceptance require a run. |
| SCIP and LSIF indexers | Same [Sourcegraph SCIP list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer) and [Sourcegraph-maintained archived LSIF indexer list](https://lsif.dev/), queries `Less`, `CSS`; GitHub web searches `site:github.com "scip-less" indexer`, `"lsif-less"` | No Less indexer in either checked list; exact-name GitHub searches yielded no result. Search absence only. |
| Language server | [Microsoft vscode-css-languageservice](https://github.com/microsoft/vscode-css-languageservice), query `LESS`, `findDefinition`, `findReferences`; [LSP server registry](https://github.com/microsoft/language-server-protocol/blob/gh-pages/_implementors/servers.md), query `CSS/LESS/SASS`; [npm registry query](https://registry.npmjs.org/-/v1/search?text=vscode-css-languageservice&size=5) | Microsoft LESS service **6.3.10** offers validation, definition, references and hover. It is JS/TS and **unrun** on the 146 files. LSP registration does not imply SCIP output. |
| Registry parsers/typecheckers/AST dump | Actual npm registry queries [Less parser](https://registry.npmjs.org/-/v1/search?text=less%20parser&size=5), [Less](https://registry.npmjs.org/-/v1/search?text=less&size=3), [postcss-less](https://registry.npmjs.org/-/v1/search?text=postcss-less&size=3); checked `objects[].package.{name,version}`; [npm `postcss-less` readme](https://www.npmjs.com/package/postcss-less), query `AST`, `@import`, `mixins` | Results: official `less` **4.9.1**, `postcss-less` **6.0.0**, `@jesscss/less-parser` **2.0.0-alpha.7**, `postcss-less-engine` **0.6.2**. `postcss-less` exposes import filename/options and mixin/variable AST nodes but expressly is not a compiler. These are **unrun**; official Less.js remains authority for import resolution and evaluated semantics. |
| Runtime call/require tracing | [Less programmatic usage and plugin/file-manager API](https://lesscss.org/usage/), queries `output.imports`, `FileManager`; [Node module resolution](https://nodejs.org/api/modules.html), query `require.resolve` | `less.render(...).imports` directly reports processed Less imports; a Less file-manager plugin could observe load paths. Node `require` traces concern JS modules, not `.less` imports or mixin calls. Less rendering may exercise mixins but does not document a complete source-level call trace. Import oracle viable, **unrun**; call-trace route not established. |

**Action/admission:** The measured grammar failures already sustain refusal. If a WASM-admissible grammar or approved alternative emerges, regrade the 146; Less.js v4.9.1 supplies an unrun import/AST oracle for the later level evaluation. Do not transfer oracle success into a product-parser claim.

A P-R1 source patch is an authorized retry route. On 146 unchanged pinned files
it must leave at most floor(0.05 * 146) = 7 gate-error files. The best measured
grammar needs at least 42 - 7 = 35 repairs. No Less source patch was measured
in this audit. New compiler/LSP capabilities alone do not change that gate.
GitHub connector repository search `scip-less` returned unrelated JavaScript
lessons and SciPy examples, with no Less indexer identified in those results.

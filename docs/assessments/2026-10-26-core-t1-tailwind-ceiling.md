# tailwind named ceiling routes (X-R1)

Measured baseline: [tailwind reading](../bench/readings/lang/tailwind.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Tailwind design-system compiler API | tailwindcss 4.3.2 | The committed adapter uses __unstable__loadDesignSystem. Theme/utility/variant registries expose no rule ends and the public package exports no parser; range_agreement remains null. |
| Tailwind internal parser source | npm tailwindcss 4.3.2, exact bundle | Inspected dist/lib.js: internal Pe assigns rule src/dst end at the opening brace; its closing-brace branch only parses children and pops the stack. It never extends the parent span to the closing brace. Pe is unexported; patching/extracting it still requires adding full-span behavior. |
| PostCSS independent parser | 8.5.28 | It exposes source.end but is tier C under the current oracle rules. No compiler-tier range truth has been measured from it. |
| Lightning CSS browser AST | 1.33.0 | It lacks full rule ends and does not attest Tailwind compiler registrations. |
| Template candidate scanner | @tailwindcss/oxide 4.3.2 | Template utility candidates are outside the CSS-at-rule reading scope (O1480); they do not attest source rule-end spans. |

Retain the measured API/tier ceiling. The exact internal parser route was checked, not inferred from a different revision. Retry when a pinned first-party end-location API is demonstrated, or an owner changes oracle tiers.

Governing row: `LB-Q-tailwind-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

Exact package inspected: [npm tailwindcss 4.3.2 tarball](https://registry.npmjs.org/tailwindcss/-/tailwindcss-4.3.2.tgz), SHA256 `3673da9004404d12d4672bb5d002945319c2b3dea8c13ea022fdd33a3e260e62`. The export map and dist/lib.d.mts confirm compile returns CSS/source maps, compileAst takes caller-supplied AST, and the design system exposes no raw parser. Reconstructing closing spans with an adapter scanner would be adapter truth, not a new compiler span.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / parser | [Tailwind registry](https://registry.npmjs.org/tailwindcss/latest) reports **4.3.3** after the inspected 4.3.2; the checked 4.3.3 parser still exposes the opening-brace source position, not an independently attested closing range. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-tailwind` search](https://github.com/search?q=scip-tailwind&type=repositories) yielded no measured Tailwind indexer. |
| Language server | [First-party Tailwind LSP](https://github.com/tailwindlabs/tailwindcss-intellisense), registry **0.16.0** ([npm](https://registry.npmjs.org/@tailwindcss/language-server/latest)), advertises editor services, not a verified full at-rule end oracle. **LIVE UNMEASURED** for any range API. |
| Registry AST | [Tailwind CSS parser source](https://github.com/tailwindlabs/tailwindcss/blob/main/packages/tailwindcss/src/css-parser.ts) and [4.3.3 registry](https://registry.npmjs.org/tailwindcss/latest) were checked; neither supplies the required measured closing spans. |
| Runtime tracing | [Tailwind source-class detection](https://tailwindcss.com/docs/detecting-classes-in-source-files) and emitted CSS concern utility generation, not all raw at-rule ends. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.
